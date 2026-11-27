package cookie

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/aaronland/go-string/random"
	"github.com/awnumar/memguard"
	"golang.org/x/crypto/nacl/secretbox"
)

// secretKeySize is the required length (in bytes) of the secret used to encrypt cookies.
const secretKeySize = 32

const nonceSize = 24

func init() {
	ctx := context.Background()
	RegisterCookie(ctx, "encrypted", NewEncryptedCookie)
	memguard.CatchInterrupt()
}

// EncryptedCookie implements the `Cookie` interface for working with cookies whose values have been encrypted.
type EncryptedCookie struct {
	Cookie
	name   string
	secret *memguard.Enclave
	salt   string
}

// NewRandomEncryptedCookieSecret returns a random secret value suitable for `encrypted://` cookie URIs.
func NewRandomEncryptedCookieSecret() (string, error) {

	r_opts := random.DefaultOptions()
	r_opts.AlphaNumeric = true

	secret, err := random.String(r_opts)

	if err != nil {
		return "", fmt.Errorf("Failed to generate secret, %w", err)
	}

	return secret, nil
}

// NewRandomEncryptedCookieSalt returns a random salt value suitable for `encrypted://` cookie URIs.
func NewRandomEncryptedCookieSalt() (string, error) {

	r_opts := random.DefaultOptions()
	r_opts.AlphaNumeric = true

	salt, err := random.String(r_opts)

	if err != nil {
		return "", fmt.Errorf("Failed to generate salt, %w", err)
	}

	return salt, nil
}

// NewRandomEncryptedCookieURI returns a new URI for creating an `EncryptedCookie` instance for 'name' with random values for the required secret and salt.
func NewRandomEncryptedCookieURI(name string) (string, error) {

	secret, err := NewRandomEncryptedCookieSecret()

	if err != nil {
		return "", fmt.Errorf("Failed to generate secret, %w", err)
	}

	salt, err := NewRandomEncryptedCookieSalt()

	if err != nil {
		return "", fmt.Errorf("Failed to generate salyt, %w", err)
	}

	cookie_uri := fmt.Sprintf("encrypted://?name=%s&secret=%s&salt=%s", name, secret, salt)
	return cookie_uri, nil
}

// NewEncryptedCookie returns a new `EncryptedCookie` instance derived from 'uri' which is expected to take the form of:
//
//	encrypted://?name={NAME}&secret={SECRET}&salt={SALT}
//
// Where `{NAME}` is the name of the cookie for all subsequent operations; `{SECRET}` is the secret key used to encrypt the value
// of the cookie and must be exactly 32 bytes long. `{SALT}` is required for backwards compatibility but is not used: the secret is
// used directly as a NaCl secretbox key.
func NewEncryptedCookie(ctx context.Context, uri string) (Cookie, error) {

	u, err := url.Parse(uri)

	if err != nil {
		return nil, fmt.Errorf("Failed to parse URL, %w", err)
	}

	q := u.Query()

	name := q.Get("name")
	secret := q.Get("secret")
	salt := q.Get("salt")

	if name == "" {
		return nil, fmt.Errorf("Invalid name value (empty)")
	}

	if secret == "" {
		return nil, fmt.Errorf("Invalid secret value (empty)")
	}

	if salt == "" {
		return nil, fmt.Errorf("Invalid salt value (empty)")
	}

	if len(secret) != secretKeySize {
		return nil, fmt.Errorf("Invalid secret value (must be exactly %d bytes)", secretKeySize)
	}

	secret_key := memguard.NewEnclave([]byte(secret))

	c := EncryptedCookie{
		name:   name,
		secret: secret_key,
		salt:   salt,
	}

	return &c, nil
}

// GetString returns the value of the cookie associated with 'c' in 'req' as an unencrypted string.
func (c *EncryptedCookie) GetString(req *http.Request) (string, error) {

	buf, err := c.Get(req)

	if err != nil {
		return "", fmt.Errorf("Failed to get cookie, %w", err)
	}

	defer buf.Destroy()

	// buf.String() aliases the locked memory, which is freed by Destroy, so copy the bytes.
	return string(buf.Bytes()), nil
}

// Get returns the value of the cookie associated with 'c' in 'req' as an unencrypted `memguard.LockedBuffer` instance.
func (c *EncryptedCookie) Get(req *http.Request) (*memguard.LockedBuffer, error) {

	http_cookie, err := req.Cookie(c.name)

	if err != nil {
		return nil, fmt.Errorf("Failed to retrieve cookie, %w", err)
	}

	return c.unlock(http_cookie.Value)
}

// SetString assigns 'value' as an encrypted string to a cookie associated with 'c' to 'rsp'.
func (c *EncryptedCookie) SetString(rsp http.ResponseWriter, value string) error {

	buf := memguard.NewBufferFromBytes([]byte(value))
	defer buf.Destroy()

	return c.Set(rsp, buf)
}

// Set assigns 'buf' as an encrypted string to a cookie associated with 'c' to 'rsp'.
func (c *EncryptedCookie) Set(rsp http.ResponseWriter, buf *memguard.LockedBuffer) error {

	http_cookie := &http.Cookie{
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	}

	return c.SetWithCookie(rsp, buf, http_cookie)
}

// SetStringWithCookie assigns 'value' as an encrypted string to 'http_cookie' which is assigned to 'rsp'.
func (c *EncryptedCookie) SetStringWithCookie(rsp http.ResponseWriter, value string, http_cookie *http.Cookie) error {

	buf := memguard.NewBufferFromBytes([]byte(value))
	defer buf.Destroy()

	return c.SetWithCookie(rsp, buf, http_cookie)
}

// SetWithCookie assigns 'buf' as an encrypted string to 'http_cookie' which is assigned to 'rsp'.
func (c *EncryptedCookie) SetWithCookie(rsp http.ResponseWriter, buf *memguard.LockedBuffer, http_cookie *http.Cookie) error {

	if http_cookie.Name != "" {
		return fmt.Errorf("Cookie name already set")
	}

	enc, err := c.lock(buf)

	if err != nil {
		return fmt.Errorf("Failed to lock buffer, %w", err)
	}

	http_cookie.Name = c.name
	http_cookie.Value = enc

	http.SetCookie(rsp, http_cookie)
	return nil
}

// Delete removes the cookie associated with 'c' from 'rsp'. The expiring cookie uses the path "/" so it will only clear
// cookies that were assigned using `Set` or `SetString` (or a `http.Cookie` with the same path).
func (c *EncryptedCookie) Delete(rsp http.ResponseWriter) error {

	http_cookie := http.Cookie{
		Name:     c.name,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	}

	http.SetCookie(rsp, &http_cookie)
	return nil
}

// lock encrypts 'buf' with a random nonce and returns the base64-encoded nonce and ciphertext.
func (c *EncryptedCookie) lock(buf *memguard.LockedBuffer) (string, error) {

	var nonce [nonceSize]byte

	if _, err := io.ReadFull(rand.Reader, nonce[:]); err != nil {
		return "", fmt.Errorf("Failed to create nonce, %w", err)
	}

	key, err := c.secret.Open()

	if err != nil {
		return "", fmt.Errorf("Failed to open enclave, %w", err)
	}

	defer key.Destroy()

	enc := secretbox.Seal(nonce[:], buf.Bytes(), &nonce, key.ByteArray32())
	return base64.StdEncoding.EncodeToString(enc), nil
}

// unlock decrypts a value produced by `lock`.
func (c *EncryptedCookie) unlock(value string) (*memguard.LockedBuffer, error) {

	body, err := base64.StdEncoding.DecodeString(value)

	if err != nil {
		return nil, fmt.Errorf("Failed to decode string, %w", err)
	}

	if len(body) < nonceSize+secretbox.Overhead {
		return nil, fmt.Errorf("Invalid encrypted value")
	}

	var nonce [nonceSize]byte
	copy(nonce[:], body[:nonceSize])

	key, err := c.secret.Open()

	if err != nil {
		return nil, fmt.Errorf("Failed to open enclave, %w", err)
	}

	defer key.Destroy()

	out, ok := secretbox.Open(nil, body[nonceSize:], &nonce, key.ByteArray32())

	if !ok {
		return nil, fmt.Errorf("Unable to open secretbox")
	}

	return memguard.NewBufferFromBytes(out), nil
}
