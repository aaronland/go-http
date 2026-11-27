package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/aaronland/go-http/v4/cookie"
	"github.com/golang-jwt/jwt/v5"
)

const oidcTransientTTL = 15 * time.Minute
const oidcDefaultSessionTTL = 24 * time.Hour

// OIDCAccount is the `Account` derived from the claims of an OpenID Connect ID token.
type OIDCAccount struct {
	// Subject is the "sub" claim.
	Subject string `json:"sub"`
	// EmailAddress is the "email" claim.
	EmailAddress string `json:"email,omitempty"`
	// ProfileName is the "name" claim.
	ProfileName string `json:"name,omitempty"`
	// Expires is the "exp" claim as a Unix timestamp.
	Expires int64 `json:"exp"`
}

// Id returns the "sub" claim.
func (a *OIDCAccount) Id() string {
	return a.Subject
}

// Name returns the "name" claim, falling back to "sub".
func (a *OIDCAccount) Name() string {
	if a.ProfileName != "" {
		return a.ProfileName
	}
	return a.Subject
}

// Email returns the "email" claim.
func (a *OIDCAccount) Email() string {
	return a.EmailAddress
}

type oidcDiscovery struct {
	Issuer                string `json:"issuer"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
	UserinfoEndpoint      string `json:"userinfo_endpoint"`
	JWKSURI               string `json:"jwks_uri"`
}

type oidcTransient struct {
	State    string `json:"s"`
	Verifier string `json:"v"`
	Nonce    string `json:"n"`
	Next     string `json:"r"`
}

// OIDCAuthenticator is an OpenID Connect relying party (authorization code flow with PKCE)
// that implements the `Authenticator` interface.
type OIDCAuthenticator struct {
	issuer       string
	clientID     string
	clientSecret string
	redirectURI  string
	callbackPath string
	scope        string
	signupURI    string
	signoutURI   string
	secure       bool
	sessionTTL   time.Duration

	session   cookie.Cookie
	transient cookie.Cookie
	client    *http.Client

	mu   sync.Mutex
	disc *oidcDiscovery
}

func init() {
	ctx := context.Background()
	RegisterAuthenticator(ctx, "oidc", NewOIDCAuthenticator)
}

// NewOIDCAuthenticator returns a new `OIDCAuthenticator` configured by 'uri' which is expected to take the form of:
//
//	oidc://?issuer={URL}&client_id={ID}&redirect_uri={URL}&cookie_secret={SECRET}&cookie_salt={SALT}
//
// Required parameters:
//   - issuer: the OpenID Connect issuer URL. Endpoints are found using {issuer}/.well-known/openid-configuration.
//   - client_id: the registered client ID.
//   - redirect_uri: the registered callback URL. Requests to its path are handled by this authenticator (see WrapHandler).
//   - cookie_secret, cookie_salt: used to encrypt the session cookies. The secret must be exactly 32 bytes
//     (see cookie.NewRandomEncryptedCookieSecret).
//
// Optional parameters:
//   - client_secret: the client secret, for confidential clients (omit for public clients).
//   - scope: space-separated scopes (default "openid profile email").
//   - cookie_name: the session cookie name (default "oidc_session"). Transient state is kept in "{name}_state".
//   - session_ttl: how long a session lasts, as a Go duration (default 24h).
//   - secure_cookies: set the "Secure" cookie flag (default true unless redirect_uri is http).
//   - signup_uri: where SignupHandler redirects to (default: HTTP 404).
//   - signout_uri: where SignoutHandler redirects to after clearing the session (default "/").
func NewOIDCAuthenticator(ctx context.Context, uri string) (Authenticator, error) {

	u, err := url.Parse(uri)

	if err != nil {
		return nil, fmt.Errorf("Failed to parse URI, %w", err)
	}

	q := u.Query()

	issuer := strings.TrimRight(q.Get("issuer"), "/")
	client_id := q.Get("client_id")
	redirect_uri := q.Get("redirect_uri")
	secret := q.Get("cookie_secret")
	salt := q.Get("cookie_salt")

	if issuer == "" || client_id == "" || redirect_uri == "" {
		return nil, fmt.Errorf("Missing issuer, client_id or redirect_uri parameter")
	}

	if secret == "" || salt == "" {
		return nil, fmt.Errorf("Missing cookie_secret or cookie_salt parameter")
	}

	// The encrypted cookie uses the secret directly as a 32-byte NaCl secretbox key.
	if len(secret) != 32 {
		return nil, fmt.Errorf("cookie_secret must be exactly 32 bytes")
	}

	ru, err := url.Parse(redirect_uri)

	if err != nil {
		return nil, fmt.Errorf("Invalid redirect_uri, %w", err)
	}

	callback_path := ru.Path

	if callback_path == "" {
		callback_path = "/"
	}

	name := q.Get("cookie_name")

	if name == "" {
		name = "oidc_session"
	}

	session_ttl := oidcDefaultSessionTTL

	if v := q.Get("session_ttl"); v != "" {

		d, err := time.ParseDuration(v)

		if err != nil || d <= 0 {
			return nil, fmt.Errorf("Invalid session_ttl")
		}

		session_ttl = d
	}

	secure := ru.Scheme == "https"

	if v := q.Get("secure_cookies"); v != "" {
		secure = v == "true" || v == "1"
	}

	scope := q.Get("scope")

	if scope == "" {
		scope = "openid profile email"
	}

	new_cookie := func(n string) (cookie.Cookie, error) {
		cu := url.Values{}
		cu.Set("name", n)
		cu.Set("secret", secret)
		cu.Set("salt", salt)
		return cookie.NewEncryptedCookie(ctx, "encrypted://?"+cu.Encode())
	}

	session, err := new_cookie(name)

	if err != nil {
		return nil, fmt.Errorf("Failed to create session cookie, %w", err)
	}

	transient, err := new_cookie(name + "_state")

	if err != nil {
		return nil, fmt.Errorf("Failed to create state cookie, %w", err)
	}

	signout_uri := q.Get("signout_uri")

	if signout_uri == "" {
		signout_uri = "/"
	}

	a := &OIDCAuthenticator{
		issuer:       issuer,
		clientID:     client_id,
		clientSecret: q.Get("client_secret"),
		redirectURI:  redirect_uri,
		callbackPath: callback_path,
		scope:        scope,
		signupURI:    q.Get("signup_uri"),
		signoutURI:   signout_uri,
		secure:       secure,
		sessionTTL:   session_ttl,
		session:      session,
		transient:    transient,
		client:       &http.Client{Timeout: 15 * time.Second},
	}

	return a, nil
}

// WrapHandler returns an `http.Handler` that serves the OIDC callback at the path of the configured redirect_uri
// and redirects unauthenticated requests to the provider; all other requests are passed to 'next'.
func (a *OIDCAuthenticator) WrapHandler(next http.Handler) http.Handler {

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		if req.URL.Path == a.callbackPath {
			a.callback(rsp, req)
			return
		}

		_, err := a.GetAccountForRequest(req)

		if err != nil {
			a.SigninHandler().ServeHTTP(rsp, req)
			return
		}

		next.ServeHTTP(rsp, req)
	}

	return http.HandlerFunc(fn)
}

// GetAccountForRequest returns the `Account` for the session cookie in 'req' or a `NotLoggedIn` error.
func (a *OIDCAuthenticator) GetAccountForRequest(req *http.Request) (Account, error) {

	v, err := readCookie(a.session, req)

	if err != nil {
		return nil, NotLoggedIn{}
	}

	var acc OIDCAccount

	if err := json.Unmarshal([]byte(v), &acc); err != nil {
		return nil, NotLoggedIn{}
	}

	if acc.Subject == "" || time.Now().Unix() >= acc.Expires {
		return nil, NotLoggedIn{}
	}

	return &acc, nil
}

// SigninHandler returns an `http.Handler` that starts the authorization code flow with PKCE.
func (a *OIDCAuthenticator) SigninHandler() http.Handler {

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		disc, err := a.discover(req.Context())

		if err != nil {
			http.Error(rsp, "Failed to contact identity provider", http.StatusBadGateway)
			return
		}

		next := "/"

		if req.URL.Path != a.callbackPath {
			next = req.URL.RequestURI()
		}

		t := oidcTransient{
			State:    randomToken(),
			Verifier: randomToken(),
			Nonce:    randomToken(),
			Next:     next,
		}

		body, _ := json.Marshal(t)

		ck := &http.Cookie{
			Path:     "/",
			Expires:  time.Now().Add(oidcTransientTTL),
			HttpOnly: true,
			Secure:   a.secure,
			SameSite: http.SameSiteLaxMode,
		}

		if err := a.transient.SetStringWithCookie(rsp, string(body), ck); err != nil {
			http.Error(rsp, "Failed to start sign-in", http.StatusInternalServerError)
			return
		}

		hash := sha256.Sum256([]byte(t.Verifier))

		params := url.Values{}
		params.Set("response_type", "code")
		params.Set("client_id", a.clientID)
		params.Set("redirect_uri", a.redirectURI)
		params.Set("scope", a.scope)
		params.Set("state", t.State)
		params.Set("nonce", t.Nonce)
		params.Set("code_challenge", base64.RawURLEncoding.EncodeToString(hash[:]))
		params.Set("code_challenge_method", "S256")

		sep := "?"

		if strings.Contains(disc.AuthorizationEndpoint, "?") {
			sep = "&"
		}

		http.Redirect(rsp, req, disc.AuthorizationEndpoint+sep+params.Encode(), http.StatusFound)
	}

	return http.HandlerFunc(fn)
}

// SignoutHandler returns an `http.Handler` that clears the local session. The provider's session is left intact.
func (a *OIDCAuthenticator) SignoutHandler() http.Handler {

	fn := func(rsp http.ResponseWriter, req *http.Request) {
		a.session.Delete(rsp)
		http.Redirect(rsp, req, a.signoutURI, http.StatusFound)
	}

	return http.HandlerFunc(fn)
}

// SignupHandler returns an `http.Handler` that redirects to the configured signup_uri (or returns a 404).
func (a *OIDCAuthenticator) SignupHandler() http.Handler {

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		if a.signupURI == "" {
			http.NotFound(rsp, req)
			return
		}

		http.Redirect(rsp, req, a.signupURI, http.StatusFound)
	}

	return http.HandlerFunc(fn)
}

func (a *OIDCAuthenticator) callback(rsp http.ResponseWriter, req *http.Request) {

	ctx := req.Context()
	q := req.URL.Query()

	v, err := readCookie(a.transient, req)

	// The transient state is single-use.
	a.transient.Delete(rsp)

	if err != nil {
		http.Error(rsp, "Missing or expired sign-in state", http.StatusBadRequest)
		return
	}

	var t oidcTransient

	if err := json.Unmarshal([]byte(v), &t); err != nil || t.State == "" || q.Get("state") != t.State {
		http.Error(rsp, "Invalid state", http.StatusBadRequest)
		return
	}

	if e := q.Get("error"); e != "" {
		// Don't reflect the provider-supplied (attacker controlled) value back to the client.
		slog.Error("OIDC sign-in failed", "error", e)
		http.Error(rsp, "Sign-in failed", http.StatusForbidden)
		return
	}

	code := q.Get("code")

	if code == "" {
		http.Error(rsp, "Missing code", http.StatusBadRequest)
		return
	}

	disc, err := a.discover(ctx)

	if err != nil {
		http.Error(rsp, "Failed to contact identity provider", http.StatusBadGateway)
		return
	}

	id_token, access_token, err := a.exchange(ctx, disc, code, t.Verifier)

	if err != nil {
		http.Error(rsp, "Failed to exchange code", http.StatusBadGateway)
		return
	}

	claims, err := a.verifyIDToken(ctx, disc, id_token, t.Nonce)

	if err != nil {
		http.Error(rsp, "Invalid ID token", http.StatusForbidden)
		return
	}

	acc := OIDCAccount{
		Subject:      claims.Subject,
		ProfileName:  claims.Name,
		EmailAddress: claims.Email,
		Expires:      time.Now().Add(a.sessionTTL).Unix(),
	}

	if (acc.ProfileName == "" || acc.EmailAddress == "") && disc.UserinfoEndpoint != "" && access_token != "" {

		ui, err := a.userinfo(ctx, disc, access_token)

		// The userinfo subject must match the ID token's.
		if err == nil && ui.Subject == acc.Subject {

			if acc.ProfileName == "" {
				acc.ProfileName = ui.Name
			}

			if acc.EmailAddress == "" {
				acc.EmailAddress = ui.Email
			}
		}
	}

	body, _ := json.Marshal(acc)

	ck := &http.Cookie{
		Path:     "/",
		Expires:  time.Unix(acc.Expires, 0),
		HttpOnly: true,
		Secure:   a.secure,
		SameSite: http.SameSiteLaxMode,
	}

	if err := a.session.SetStringWithCookie(rsp, string(body), ck); err != nil {
		http.Error(rsp, "Failed to create session", http.StatusInternalServerError)
		return
	}

	next := t.Next

	// Only allow local, relative redirects.
	if !strings.HasPrefix(next, "/") || strings.HasPrefix(next, "//") || strings.HasPrefix(next, "/\\") {
		next = "/"
	}

	http.Redirect(rsp, req, next, http.StatusFound)
}

func (a *OIDCAuthenticator) discover(ctx context.Context) (*oidcDiscovery, error) {

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.disc != nil {
		return a.disc, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.issuer+"/.well-known/openid-configuration", nil)

	if err != nil {
		return nil, err
	}

	var d oidcDiscovery

	if err := a.doJSON(req, &d); err != nil {
		return nil, fmt.Errorf("Failed to fetch discovery document, %w", err)
	}

	if d.Issuer != a.issuer {
		return nil, fmt.Errorf("Issuer mismatch: %q", d.Issuer)
	}

	if d.AuthorizationEndpoint == "" || d.TokenEndpoint == "" || d.JWKSURI == "" {
		return nil, fmt.Errorf("Incomplete discovery document")
	}

	a.disc = &d
	return a.disc, nil
}

func (a *OIDCAuthenticator) exchange(ctx context.Context, disc *oidcDiscovery, code string, verifier string) (string, string, error) {

	form := url.Values{}
	form.Set("grant_type", "authorization_code")
	form.Set("code", code)
	form.Set("redirect_uri", a.redirectURI)
	form.Set("code_verifier", verifier)

	if a.clientSecret == "" {
		form.Set("client_id", a.clientID)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, disc.TokenEndpoint, strings.NewReader(form.Encode()))

	if err != nil {
		return "", "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	if a.clientSecret != "" {
		req.SetBasicAuth(url.QueryEscape(a.clientID), url.QueryEscape(a.clientSecret))
	}

	var tr struct {
		IDToken     string `json:"id_token"`
		AccessToken string `json:"access_token"`
	}

	if err := a.doJSON(req, &tr); err != nil {
		return "", "", err
	}

	if tr.IDToken == "" {
		return "", "", fmt.Errorf("No id_token in response")
	}

	return tr.IDToken, tr.AccessToken, nil
}

type oidcClaims struct {
	Name  string `json:"name"`
	Email string `json:"email"`
	Nonce string `json:"nonce"`
	jwt.RegisteredClaims
}

func (a *OIDCAuthenticator) verifyIDToken(ctx context.Context, disc *oidcDiscovery, raw string, nonce string) (*oidcClaims, error) {

	keys, err := a.fetchKeys(ctx, disc)

	if err != nil {
		return nil, err
	}

	claims := &oidcClaims{}

	_, err = jwt.ParseWithClaims(raw, claims, func(t *jwt.Token) (interface{}, error) {

		kid, _ := t.Header["kid"].(string)

		if k, ok := keys[kid]; ok {
			return k, nil
		}

		if kid == "" && len(keys) == 1 {
			for _, k := range keys {
				return k, nil
			}
		}

		return nil, fmt.Errorf("Unknown signing key")
	},
		jwt.WithValidMethods([]string{"RS256"}),
		jwt.WithIssuer(disc.Issuer),
		jwt.WithAudience(a.clientID),
		jwt.WithExpirationRequired(),
		jwt.WithLeeway(30*time.Second),
	)

	if err != nil {
		return nil, err
	}

	if claims.Nonce != nonce || nonce == "" {
		return nil, fmt.Errorf("Nonce mismatch")
	}

	if claims.Subject == "" {
		return nil, fmt.Errorf("Missing sub claim")
	}

	return claims, nil
}

func (a *OIDCAuthenticator) fetchKeys(ctx context.Context, disc *oidcDiscovery) (map[string]*rsa.PublicKey, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, disc.JWKSURI, nil)

	if err != nil {
		return nil, err
	}

	var set struct {
		Keys []struct {
			Kty string `json:"kty"`
			Kid string `json:"kid"`
			Use string `json:"use"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}

	if err := a.doJSON(req, &set); err != nil {
		return nil, fmt.Errorf("Failed to fetch JWKS, %w", err)
	}

	keys := make(map[string]*rsa.PublicKey)

	for _, k := range set.Keys {

		if k.Kty != "RSA" || (k.Use != "" && k.Use != "sig") {
			continue
		}

		n, err := base64.RawURLEncoding.DecodeString(k.N)

		if err != nil {
			continue
		}

		e, err := base64.RawURLEncoding.DecodeString(k.E)

		if err != nil {
			continue
		}

		keys[k.Kid] = &rsa.PublicKey{
			N: new(big.Int).SetBytes(n),
			E: int(new(big.Int).SetBytes(e).Int64()),
		}
	}

	if len(keys) == 0 {
		return nil, fmt.Errorf("No usable signing keys")
	}

	return keys, nil
}

func (a *OIDCAuthenticator) userinfo(ctx context.Context, disc *oidcDiscovery, access_token string) (*oidcClaims, error) {

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, disc.UserinfoEndpoint, nil)

	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+access_token)

	var c oidcClaims

	if err := a.doJSON(req, &c); err != nil {
		return nil, err
	}

	return &c, nil
}

func (a *OIDCAuthenticator) doJSON(req *http.Request, v interface{}) error {

	rsp, err := a.client.Do(req)

	if err != nil {
		return err
	}

	defer rsp.Body.Close()

	if rsp.StatusCode != http.StatusOK {
		return fmt.Errorf("Unexpected status: %s", rsp.Status)
	}

	dec := json.NewDecoder(io.LimitReader(rsp.Body, 1<<20))
	return dec.Decode(v)
}

func randomToken() string {
	b := make([]byte, 32)
	// crypto/rand.Read never returns an error on supported platforms (Go 1.24+); panic otherwise.
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

// readCookie returns the decrypted value of 'c' in 'req'. The value is copied before the locked buffer is destroyed.
func readCookie(c cookie.Cookie, req *http.Request) (string, error) {

	ec, ok := c.(*cookie.EncryptedCookie)

	if !ok {
		return c.GetString(req)
	}

	buf, err := ec.Get(req)

	if err != nil {
		return "", err
	}

	defer buf.Destroy()

	return string(append([]byte(nil), buf.Bytes()...)), nil
}
