package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const testJWTSecret = "s3cr3t-for-tests"

func signToken(t *testing.T, method jwt.SigningMethod, key any, claims JWTAuthenticatorClaims) string {
	t.Helper()

	s, err := jwt.NewWithClaims(method, claims).SignedString(key)

	if err != nil {
		t.Fatalf("Failed to sign token, %v", err)
	}

	return s
}

func validClaims() JWTAuthenticatorClaims {
	return JWTAuthenticatorClaims{
		AccountId:   "123",
		AccountName: "test",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
}

func newTestJWTAuthenticator(t *testing.T, uri string) Authenticator {
	t.Helper()

	a, err := NewJWTAuthenticator(context.Background(), uri)

	if err != nil {
		t.Fatalf("Failed to create authenticator, %v", err)
	}

	return a
}

func TestJWTAuthenticatorHeaders(t *testing.T) {

	good := signToken(t, jwt.SigningMethodHS256, []byte(testJWTSecret), validClaims())

	tests := []struct {
		name    string
		uri     string
		headers map[string]string
		ok      bool
	}{
		{"authorization bearer", "jwt://" + testJWTSecret, map[string]string{"Authorization": "Bearer " + good}, true},
		{"legacy header fallback", "jwt://" + testJWTSecret, map[string]string{"Authentication": "Bearer " + good}, true},
		{"custom header", "jwt://" + testJWTSecret + "?authorization-header=X-Custom", map[string]string{"X-Custom": "Bearer " + good}, true},
		{"custom header ignores legacy", "jwt://" + testJWTSecret + "?authorization-header=X-Custom", map[string]string{"Authentication": "Bearer " + good}, false},
		{"missing header", "jwt://" + testJWTSecret, nil, false},
		{"no bearer prefix", "jwt://" + testJWTSecret, map[string]string{"Authorization": good}, false},
		{"wrong scheme", "jwt://" + testJWTSecret, map[string]string{"Authorization": "Basic " + good}, false},
		{"malformed token", "jwt://" + testJWTSecret, map[string]string{"Authorization": "Bearer abc.def"}, false},
		{"trailing junk", "jwt://" + testJWTSecret, map[string]string{"Authorization": "Bearer " + good + " extra"}, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			a := newTestJWTAuthenticator(t, tc.uri)

			req := httptest.NewRequest("GET", "/", nil)

			for k, v := range tc.headers {
				req.Header.Set(k, v)
			}

			acct, err := a.GetAccountForRequest(req)

			if tc.ok {
				if err != nil {
					t.Fatalf("Expected success, %v", err)
				}

				if acct.Id() != "123" || acct.Name() != "test" {
					t.Fatalf("Unexpected account %q %q", acct.Id(), acct.Name())
				}

				return
			}

			if err == nil {
				t.Fatal("Expected failure")
			}
		})
	}
}

func TestJWTAuthenticatorTokens(t *testing.T) {

	expired := validClaims()
	expired.ExpiresAt = jwt.NewNumericDate(time.Now().Add(-time.Hour))

	noExp := validClaims()
	noExp.ExpiresAt = nil

	noID := validClaims()
	noID.AccountId = ""

	noName := validClaims()
	noName.AccountName = ""

	// An unsigned ("alg=none") token must never be accepted.
	none, err := jwt.NewWithClaims(jwt.SigningMethodNone, validClaims()).SignedString(jwt.UnsafeAllowNoneSignatureType)

	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name  string
		token string
		ok    bool
	}{
		{"valid", signToken(t, jwt.SigningMethodHS256, []byte(testJWTSecret), validClaims()), true},
		{"wrong secret", signToken(t, jwt.SigningMethodHS256, []byte("other"), validClaims()), false},
		{"expired", signToken(t, jwt.SigningMethodHS256, []byte(testJWTSecret), expired), false},
		{"no expiration", signToken(t, jwt.SigningMethodHS256, []byte(testJWTSecret), noExp), false},
		{"missing account id", signToken(t, jwt.SigningMethodHS256, []byte(testJWTSecret), noID), false},
		{"missing account name", signToken(t, jwt.SigningMethodHS256, []byte(testJWTSecret), noName), false},
		{"HS512 rejected", signToken(t, jwt.SigningMethodHS512, []byte(testJWTSecret), validClaims()), false},
		{"alg none", none, false},
	}

	a := newTestJWTAuthenticator(t, "jwt://"+testJWTSecret)

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			req := httptest.NewRequest("GET", "/", nil)
			req.Header.Set("Authorization", "Bearer "+tc.token)

			_, err := a.GetAccountForRequest(req)

			if tc.ok && err != nil {
				t.Fatalf("Expected success, %v", err)
			}

			if !tc.ok && err == nil {
				t.Fatal("Expected failure")
			}
		})
	}
}

func TestJWTAuthenticatorWrapHandler(t *testing.T) {

	a := newTestJWTAuthenticator(t, "jwt://"+testJWTSecret)

	called := false

	h := a.WrapHandler(http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		called = true
	}))

	rsp := httptest.NewRecorder()
	h.ServeHTTP(rsp, httptest.NewRequest("GET", "/", nil))

	if rsp.Code != http.StatusForbidden || called {
		t.Fatalf("Expected 403 without calling next, got %d (called=%v)", rsp.Code, called)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set("Authorization", "Bearer "+signToken(t, jwt.SigningMethodHS256, []byte(testJWTSecret), validClaims()))

	rsp = httptest.NewRecorder()
	h.ServeHTTP(rsp, req)

	if rsp.Code != http.StatusOK || !called {
		t.Fatalf("Expected next to be called, got %d", rsp.Code)
	}
}

func TestNewJWTAuthenticatorMissingSecret(t *testing.T) {

	if _, err := NewJWTAuthenticator(context.Background(), "jwt://"); err == nil {
		t.Fatal("Expected error for missing secret")
	}
}
