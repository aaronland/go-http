package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type oidcFixture struct {
	auth Authenticator
	rp   *httptest.Server
	// mutate lets a test alter the ID token claims issued by the provider.
	mutate func(claims jwt.MapClaims)
}

func newOIDCFixture(t *testing.T, extra string) *oidcFixture {

	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)

	if err != nil {
		t.Fatal(err)
	}

	f := &oidcFixture{}

	var issuer, nonce string

	mux := http.NewServeMux()

	mux.HandleFunc("/.well-known/openid-configuration", func(rsp http.ResponseWriter, req *http.Request) {
		json.NewEncoder(rsp).Encode(map[string]string{
			"issuer":                 issuer,
			"authorization_endpoint": issuer + "/authorize",
			"token_endpoint":         issuer + "/token",
			"jwks_uri":               issuer + "/keys",
		})
	})

	mux.HandleFunc("/keys", func(rsp http.ResponseWriter, req *http.Request) {
		json.NewEncoder(rsp).Encode(map[string]any{"keys": []map[string]string{{
			"kty": "RSA", "kid": "k1", "use": "sig",
			"n": base64.RawURLEncoding.EncodeToString(key.N.Bytes()),
			"e": base64.RawURLEncoding.EncodeToString(big.NewInt(int64(key.E)).Bytes()),
		}}})
	})

	mux.HandleFunc("/authorize", func(rsp http.ResponseWriter, req *http.Request) {
		q := req.URL.Query()
		nonce = q.Get("nonce")
		http.Redirect(rsp, req, q.Get("redirect_uri")+"?code=abc&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
	})

	mux.HandleFunc("/token", func(rsp http.ResponseWriter, req *http.Request) {

		claims := jwt.MapClaims{
			"iss": issuer, "aud": "cid", "sub": "user-1", "name": "Alice", "nonce": nonce,
			"exp": time.Now().Add(time.Hour).Unix(),
		}

		if f.mutate != nil {
			f.mutate(claims)
		}

		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
		tok.Header["kid"] = "k1"
		s, _ := tok.SignedString(key)
		json.NewEncoder(rsp).Encode(map[string]string{"id_token": s, "access_token": "at"})
	})

	idp := httptest.NewServer(mux)
	t.Cleanup(idp.Close)
	issuer = idp.URL

	f.rp = httptest.NewUnstartedServer(nil)
	f.rp.Start()
	t.Cleanup(f.rp.Close)

	uri := "oidc://?issuer=" + url.QueryEscape(issuer) + "&client_id=cid&redirect_uri=" + url.QueryEscape(f.rp.URL+"/callback") +
		"&cookie_secret=0123456789abcdef0123456789abcdef&cookie_salt=salt1234&secure_cookies=false" + extra

	a, err := NewOIDCAuthenticator(context.Background(), uri)

	if err != nil {
		t.Fatal(err)
	}

	f.auth = a

	mux2 := http.NewServeMux()
	mux2.Handle("/signout", a.SignoutHandler())
	mux2.Handle("/", http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		acc, _ := a.GetAccountForRequest(req)
		rsp.Write([]byte(acc.Id()))
	}))

	f.rp.Config.Handler = a.WrapHandler(mux2)
	return f
}

func (f *oidcFixture) client() *http.Client {
	jar, _ := cookiejar.New(nil)
	return &http.Client{Jar: jar}
}

func body(rsp *http.Response) string {
	defer rsp.Body.Close()
	var sb strings.Builder
	buf := make([]byte, 512)
	for {
		n, err := rsp.Body.Read(buf)
		sb.Write(buf[:n])
		if err != nil {
			return sb.String()
		}
	}
}

// A callback hit directly, with no sign-in in progress, must be refused.
func TestOIDCCallbackWithoutState(t *testing.T) {

	f := newOIDCFixture(t, "")

	rsp, err := f.client().Get(f.rp.URL + "/callback?code=abc&state=anything")

	if err != nil {
		t.Fatal(err)
	}

	if rsp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got %d", rsp.StatusCode)
	}
}

// Login CSRF: a valid transient cookie combined with an attacker's state must be refused.
func TestOIDCCallbackStateMismatch(t *testing.T) {

	f := newOIDCFixture(t, "")
	c := f.client()
	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Path == "/callback" {
			return http.ErrUseLastResponse
		}
		return nil
	}

	rsp, err := c.Get(f.rp.URL + "/page")

	if err != nil {
		t.Fatal(err)
	}

	if rsp.StatusCode != http.StatusFound || !strings.Contains(rsp.Header.Get("Location"), "state=") {
		t.Fatalf("expected the callback redirect, got %d %q", rsp.StatusCode, rsp.Header.Get("Location"))
	}

	rsp, err = c.Get(f.rp.URL + "/callback?code=abc&state=attacker")

	if err != nil {
		t.Fatal(err)
	}

	if rsp.StatusCode != http.StatusBadRequest {
		t.Fatalf("got %d", rsp.StatusCode)
	}

	// The transient cookie is single-use, so a second callback finds no sign-in state.
	rsp, _ = c.Get(f.rp.URL + "/callback?code=abc&state=attacker")

	if rsp.StatusCode != http.StatusBadRequest {
		t.Fatalf("replay got %d", rsp.StatusCode)
	}
}

func TestOIDCCallbackProviderErrorNotReflected(t *testing.T) {

	f := newOIDCFixture(t, "")
	c := f.client()

	var state string

	c.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Path == "/callback" {
			state = req.URL.Query().Get("state")
			return http.ErrUseLastResponse
		}
		return nil
	}

	if _, err := c.Get(f.rp.URL + "/page"); err != nil {
		t.Fatal(err)
	}

	payload := "<script>alert(1)</script>"

	rsp, err := c.Get(f.rp.URL + "/callback?state=" + url.QueryEscape(state) + "&error=" + url.QueryEscape(payload))

	if err != nil {
		t.Fatal(err)
	}

	if rsp.StatusCode != http.StatusForbidden {
		t.Fatalf("got %d", rsp.StatusCode)
	}

	if strings.Contains(body(rsp), "script") {
		t.Fatal("provider error was reflected into the response")
	}
}

// An ID token minted for a different sign-in (replayed nonce) must be rejected.
func TestOIDCCallbackRejectsBadIDToken(t *testing.T) {

	tests := map[string]func(claims jwt.MapClaims){
		"wrong nonce":    func(c jwt.MapClaims) { c["nonce"] = "someone-elses" },
		"wrong audience": func(c jwt.MapClaims) { c["aud"] = "other-client" },
		"wrong issuer":   func(c jwt.MapClaims) { c["iss"] = "https://evil.example" },
		"expired":        func(c jwt.MapClaims) { c["exp"] = time.Now().Add(-time.Hour).Unix() },
		"no subject":     func(c jwt.MapClaims) { delete(c, "sub") },
	}

	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {

			f := newOIDCFixture(t, "")
			f.mutate = mutate

			c := f.client()

			rsp, err := c.Get(f.rp.URL + "/page")

			if err != nil {
				t.Fatal(err)
			}

			if b := body(rsp); rsp.StatusCode == http.StatusOK && b == "user-1" {
				t.Fatalf("session established with a bad token (%s)", name)
			}

			if _, err := f.auth.GetAccountForRequest(requestWithJar(t, f, c)); err == nil {
				t.Fatalf("account available after a bad token (%s)", name)
			}
		})
	}
}

func requestWithJar(t *testing.T, f *oidcFixture, c *http.Client) *http.Request {
	t.Helper()
	req, _ := http.NewRequest("GET", f.rp.URL+"/", nil)
	u, _ := url.Parse(f.rp.URL)
	for _, ck := range c.Jar.Cookies(u) {
		req.AddCookie(ck)
	}
	return req
}

func TestOIDCSessionExpiry(t *testing.T) {

	f := newOIDCFixture(t, "&session_ttl=1s")
	c := f.client()

	rsp, err := c.Get(f.rp.URL + "/page")

	if err != nil || body(rsp) != "user-1" {
		t.Fatalf("sign-in failed, %v", err)
	}

	if _, err := f.auth.GetAccountForRequest(requestWithJar(t, f, c)); err != nil {
		t.Fatalf("session should be valid, %v", err)
	}

	time.Sleep(2100 * time.Millisecond)

	if _, err := f.auth.GetAccountForRequest(requestWithJar(t, f, c)); err == nil {
		t.Fatal("expired session still accepted")
	}
}

func TestOIDCSignout(t *testing.T) {

	f := newOIDCFixture(t, "")
	c := f.client()

	rsp, err := c.Get(f.rp.URL + "/page")

	if err != nil || body(rsp) != "user-1" {
		t.Fatalf("sign-in failed, %v", err)
	}

	c.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	rsp, err = c.Get(f.rp.URL + "/signout")

	if err != nil {
		t.Fatal(err)
	}

	if rsp.StatusCode != http.StatusFound || rsp.Header.Get("Location") != "/" {
		t.Fatalf("got %d %q", rsp.StatusCode, rsp.Header.Get("Location"))
	}

	if _, err := f.auth.GetAccountForRequest(requestWithJar(t, f, c)); err == nil {
		t.Fatal("session survived signout")
	}
}
