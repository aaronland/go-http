package crumb

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	_ "log"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/aaronland/go-string/random"
)

func init() {
	ctx := context.Background()
	RegisterCrumb(ctx, "encrypted", NewEncryptedCrumb)
}

// EncryptedCrumb implements the Crumb interface for crumb strings that are encrypted.
type EncryptedCrumb struct {
	Crumb
	extra     string
	separator string
	secret    string
	ttl       int64
	key       string
}

// NewRandomEncryptedCrumbSecret returns a random secret value suitable for `encrypted://` crumb URIs.
func NewRandomEncryptedCrumbSecret() (string, error) {

	r_opts := random.DefaultOptions()
	r_opts.AlphaNumeric = true

	secret, err := random.String(r_opts)

	if err != nil {
		return "", fmt.Errorf("Failed to generate secret, %w", err)
	}

	return secret, nil
}

// NewRandomEncryptedCrumbExtra returns a random extra value suitable for `encrypted://` crumb URIs.
func NewRandomEncryptedCrumbExtra() (string, error) {

	r_opts := random.DefaultOptions()
	r_opts.Length = 8
	r_opts.AlphaNumeric = true

	extra, err := random.String(r_opts)

	if err != nil {
		return "", fmt.Errorf("Failed to generate extra, %w", err)
	}

	return extra, nil
}

// NewRandomEncryptedCrumbURI returns a valid `aaronland/go-http-crumb` URI for an encrypted crumb
// whose key is 'key' and whose time to live is 'ttl'.
func NewRandomEncryptedCrumbURI(ctx context.Context, ttl int, key string) (string, error) {

	s, err := NewRandomEncryptedCrumbSecret()

	if err != nil {
		return "", err
	}

	e, err := NewRandomEncryptedCrumbExtra()

	if err != nil {
		return "", err
	}

	params := url.Values{}
	params.Set("extra", e)
	params.Set("separator", ":")
	params.Set("secret", s)
	params.Set("ttl", strconv.Itoa(ttl))
	params.Set("key", key)

	uri := fmt.Sprintf("encrypted://?%s", params.Encode())
	return uri, nil
}

// NewEncryptedCrumb returns a new `Crumb` instance for 'uri' which is expected to contain the "extra", "separator",
// "secret" and "ttl" query parameters and, optionally, "key".
func NewEncryptedCrumb(ctx context.Context, uri string) (Crumb, error) {

	u, err := url.Parse(uri)

	if err != nil {
		return nil, err
	}

	q := u.Query()

	extra := q.Get("extra")
	separator := q.Get("separator")
	secret := q.Get("secret")
	str_ttl := q.Get("ttl")

	if extra == "" {
		return nil, errors.New("Empty extra= key")
	}

	if separator == "" {
		return nil, errors.New("Empty separator= key")
	}

	if secret == "" {
		return nil, errors.New("Empty secret= key")
	}

	if str_ttl == "" {
		return nil, errors.New("Empty ttl= key")
	}

	switch len(secret) {
	case 16, 24, 32:
		// valid AES key length
	default:
		return nil, fmt.Errorf("Invalid secret= key length (%d), must be 16, 24 or 32 bytes", len(secret))
	}

	ttl, err := strconv.ParseInt(str_ttl, 10, 64)

	if err != nil {
		return nil, err
	}

	cr := &EncryptedCrumb{
		extra:     extra,
		separator: separator,
		secret:    secret,
		ttl:       ttl,
	}

	key := q.Get("key")

	if key != "" {
		cr.key = key
	}

	return cr, nil
}

// Generate returns a new crumb string for 'req'.
func (cr *EncryptedCrumb) Generate(req *http.Request, extra ...string) (string, error) {

	crumb_base, err := cr.crumbBase(req, extra...)

	if err != nil {
		return "", err
	}

	crumb_hash, err := cr.hashCrumb(crumb_base)

	if err != nil {
		return "", err
	}

	now := time.Now()
	ts := now.Unix()

	str_ts := strconv.FormatInt(ts, 10)

	crumb_parts := []string{
		str_ts,
		crumb_hash,
	}

	crumb_var := strings.Join(crumb_parts, cr.separator)

	enc_var, err := cr.encryptCrumb(crumb_var)

	if err != nil {
		return "", err
	}

	return enc_var, nil
}

// Validate returns true if 'enc_var' is a valid (and unexpired) crumb for 'req' and an error otherwise.
func (cr *EncryptedCrumb) Validate(req *http.Request, enc_var string, extra ...string) (bool, error) {

	crumb_var, err := cr.decryptCrumb(enc_var)

	if err != nil {
		return false, err
	}

	crumb_parts := strings.Split(crumb_var, cr.separator)

	if len(crumb_parts) != 2 {
		return false, errors.New("Invalid crumb")
	}

	crumb_ts := crumb_parts[0]
	crumb_test := crumb_parts[1]

	crumb_base, err := cr.crumbBase(req, extra...)

	if err != nil {
		return false, err
	}

	crumb_hash, err := cr.hashCrumb(crumb_base)

	if err != nil {
		return false, err
	}

	ok, err := cr.compareHashes(crumb_hash, crumb_test)

	if err != nil {
		return false, err
	}

	if !ok {
		return false, errors.New("Crumb mismatch")
	}

	if cr.ttl > 0 {

		then, err := strconv.ParseInt(crumb_ts, 10, 64)

		if err != nil {
			return false, err
		}

		now := time.Now()
		ts := now.Unix()

		if ts-then > cr.ttl {
			return false, errors.New("Crumb has expired")
		}
	}

	return true, nil
}

// Key returns the key that 'cr' was instantiated with or, if empty, the path for 'req'.
func (cr *EncryptedCrumb) Key(req *http.Request) string {

	switch cr.key {
	case "":
		return req.URL.Path
	default:
		return cr.key
	}
}

func (cr *EncryptedCrumb) crumbBase(req *http.Request, extra ...string) (string, error) {

	crumb_key := cr.Key(req)

	base := make([]string, 0)

	base = append(base, crumb_key)
	base = append(base, req.UserAgent())
	base = append(base, cr.extra)

	for _, e := range extra {
		base = append(base, e)
	}

	str_base := strings.Join(base, "-")
	return str_base, nil
}

func (cr *EncryptedCrumb) compareHashes(this_enc string, that_enc string) (bool, error) {

	match := subtle.ConstantTimeCompare([]byte(this_enc), []byte(that_enc)) == 1
	return match, nil
}

func (cr *EncryptedCrumb) hashCrumb(raw string) (string, error) {

	msg := []byte(raw)

	mac := sha256.New()
	mac.Write(msg)
	hash := mac.Sum(nil)

	enc := hex.EncodeToString(hash[:])
	return enc, nil
}

// encryptCrumb encrypts and authenticates 'text' using AES-GCM so that tampered crumbs are rejected.
func (cr *EncryptedCrumb) encryptCrumb(text string) (string, error) {

	aead, err := cr.newAEAD()

	if err != nil {
		return "", err
	}

	nonce := make([]byte, aead.NonceSize())

	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := aead.Seal(nonce, nonce, []byte(text), nil)

	return hex.EncodeToString(ciphertext), nil
}

func (cr *EncryptedCrumb) decryptCrumb(enc_crumb string) (string, error) {

	ciphertext, err := hex.DecodeString(enc_crumb)

	if err != nil {
		return "", err
	}

	aead, err := cr.newAEAD()

	if err != nil {
		return "", err
	}

	if byteLen := len(ciphertext); byteLen < aead.NonceSize() {
		return "", fmt.Errorf("invalid cipher size %d", byteLen)
	}

	nonce := ciphertext[:aead.NonceSize()]
	ciphertext = ciphertext[aead.NonceSize():]

	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)

	if err != nil {
		return "", errors.New("Invalid crumb")
	}

	return string(plaintext), nil
}

func (cr *EncryptedCrumb) newAEAD() (cipher.AEAD, error) {

	block, err := aes.NewCipher([]byte(cr.secret))

	if err != nil {
		return nil, err
	}

	return cipher.NewGCM(block)
}
