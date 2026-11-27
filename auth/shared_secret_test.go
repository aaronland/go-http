package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestSharedSecretAuthenticator(t *testing.T) {

	a, err := NewSharedSecretAuthenticator(context.Background(), "sharedsecret://hunter2")

	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		header string
		set    bool
		ok     bool
	}{
		{"correct", "hunter2", true, true},
		{"wrong", "hunter3", true, false},
		{"prefix of secret", "hunter", true, false},
		{"longer than secret", "hunter22", true, false},
		{"empty header", "", true, false},
		{"no header", "", false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			req := httptest.NewRequest("GET", "/", nil)

			if tc.set {
				req.Header.Set(SHARED_SECRET_HEADER, tc.header)
			}

			_, err := a.GetAccountForRequest(req)

			if tc.ok != (err == nil) {
				t.Fatalf("ok=%v but err=%v", tc.ok, err)
			}
		})
	}
}

func TestSharedSecretAuthenticatorWrapHandler(t *testing.T) {

	a, err := NewSharedSecretAuthenticator(context.Background(), "sharedsecret://hunter2")

	if err != nil {
		t.Fatal(err)
	}

	called := false

	h := a.WrapHandler(http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		called = true
	}))

	rsp := httptest.NewRecorder()
	h.ServeHTTP(rsp, httptest.NewRequest("GET", "/", nil))

	if rsp.Code != http.StatusForbidden || called {
		t.Fatalf("Expected 403, got %d", rsp.Code)
	}

	req := httptest.NewRequest("GET", "/", nil)
	req.Header.Set(SHARED_SECRET_HEADER, "hunter2")

	rsp = httptest.NewRecorder()
	h.ServeHTTP(rsp, req)

	if rsp.Code != http.StatusOK || !called {
		t.Fatalf("Expected next to be called, got %d", rsp.Code)
	}
}

func TestSharedSecretAuthenticatorMissingSecret(t *testing.T) {

	if _, err := NewSharedSecretAuthenticator(context.Background(), "sharedsecret://"); err == nil {
		t.Fatal("Expected error for missing secret")
	}
}
