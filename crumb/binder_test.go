package crumb

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestEnsureCrumbHandlerWithBinder(t *testing.T) {

	ctx := context.Background()

	uri, err := NewRandomEncryptedCrumbURI(ctx, 10, "test")

	if err != nil {
		t.Fatal(err)
	}

	cr, err := NewCrumb(ctx, uri)

	if err != nil {
		t.Fatal(err)
	}

	bind := func(req *http.Request) string {
		return req.Header.Get("X-Session")
	}

	next := http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		rsp.Header().Set("Content-Type", "text/plain")
		rsp.WriteHeader(http.StatusOK)
	})

	h := EnsureCrumbHandlerWithBinder(cr, next, bind)

	alice, _ := http.NewRequest("GET", "/", nil)
	alice.Header.Set("X-Session", "alice")

	token, err := cr.Generate(alice, "alice")

	if err != nil {
		t.Fatal(err)
	}

	post := func(session string) int {
		body := url.Values{"crumb": {token}}.Encode()
		req := httptest.NewRequest("POST", "/", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		req.Header.Set("X-Session", session)
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}

	if code := post("alice"); code != http.StatusOK {
		t.Fatalf("Expected 200 for bound session, got %d", code)
	}

	if code := post("mallory"); code == http.StatusOK {
		t.Fatalf("Crumb for alice accepted for another session")
	}
}
