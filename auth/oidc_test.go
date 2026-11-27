package auth

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

func TestOIDCAuthenticatorFlow(t *testing.T) {

	key, err := rsa.GenerateKey(rand.Reader, 2048)

	if err != nil {
		t.Fatal(err)
	}

	var issuer, nonce, challenge string

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
		challenge = q.Get("code_challenge")
		http.Redirect(rsp, req, q.Get("redirect_uri")+"?code=abc&state="+url.QueryEscape(q.Get("state")), http.StatusFound)
	})

	mux.HandleFunc("/token", func(rsp http.ResponseWriter, req *http.Request) {
		req.ParseForm()
		h := sha256.Sum256([]byte(req.PostForm.Get("code_verifier")))
		if base64.RawURLEncoding.EncodeToString(h[:]) != challenge || req.PostForm.Get("code") != "abc" {
			http.Error(rsp, "bad", http.StatusBadRequest)
			return
		}
		tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"iss": issuer, "aud": "cid", "sub": "user-1", "name": "Alice", "nonce": nonce,
			"exp": time.Now().Add(time.Hour).Unix(),
		})
		tok.Header["kid"] = "k1"
		s, _ := tok.SignedString(key)
		json.NewEncoder(rsp).Encode(map[string]string{"id_token": s, "access_token": "at"})
	})

	idp := httptest.NewServer(mux)
	defer idp.Close()
	issuer = idp.URL

	rp := httptest.NewUnstartedServer(nil)
	rp.Start()
	defer rp.Close()

	uri := "oidc://?issuer=" + url.QueryEscape(issuer) + "&client_id=cid&redirect_uri=" + url.QueryEscape(rp.URL+"/callback") +
		"&cookie_secret=0123456789abcdef0123456789abcdef&cookie_salt=salt1234&secure_cookies=false"

	a, err := NewOIDCAuthenticator(context.Background(), uri)

	if err != nil {
		t.Fatal(err)
	}

	rp.Config.Handler = a.WrapHandler(http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		acc, _ := a.GetAccountForRequest(req)
		rsp.Write([]byte(acc.Id() + ":" + acc.Name()))
	}))

	jar, _ := cookiejar.New(nil)
	c := &http.Client{Jar: jar}

	rsp, err := c.Get(rp.URL + "/page?x=1")

	if err != nil {
		t.Fatal(err)
	}

	defer rsp.Body.Close()

	buf := make([]byte, 100)
	n, _ := rsp.Body.Read(buf)

	if got := string(buf[:n]); got != "user-1:Alice" {
		t.Fatalf("got %q (status %d, url %s)", got, rsp.StatusCode, rsp.Request.URL)
	}

	if rsp.Request.URL.RequestURI() != "/page?x=1" {
		t.Fatalf("not returned to original url: %s", rsp.Request.URL)
	}

	// Forged cookie must be rejected.
	req, _ := http.NewRequest("GET", rp.URL+"/", nil)
	req.AddCookie(&http.Cookie{Name: "oidc_session", Value: base64.URLEncoding.EncodeToString([]byte(`{"sub":"x","exp":9999999999}`))})

	if _, err := a.GetAccountForRequest(req); err == nil {
		t.Fatal("forged cookie accepted")
	}
}
