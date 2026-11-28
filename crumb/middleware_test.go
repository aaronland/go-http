package crumb

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"
)

func newTestEncryptedCrumb(t *testing.T, ttl int) *EncryptedCrumb {
	t.Helper()

	ctx := context.Background()

	uri, err := NewRandomEncryptedCrumbURI(ctx, ttl, "test")

	if err != nil {
		t.Fatal(err)
	}

	cr, err := NewEncryptedCrumb(ctx, uri)

	if err != nil {
		t.Fatal(err)
	}

	return cr.(*EncryptedCrumb)
}

func testReq(method string, target string, body string) *http.Request {

	var req *http.Request

	if body != "" {
		req = httptest.NewRequest(method, target, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req = httptest.NewRequest(method, target, nil)
	}

	req.Header.Set("User-Agent", "test-agent")
	return req
}

// The fault-wrapper variant writes the status itself so it can be exercised without an error handler.
func TestCrumbMiddlewareMethods(t *testing.T) {

	cr := newTestEncryptedCrumb(t, 60)

	token, err := cr.Generate(testReq("GET", "/", ""))

	if err != nil {
		t.Fatal(err)
	}

	form := url.Values{"crumb": {token}}.Encode()

	tests := []struct {
		name   string
		method string
		target string
		body   string
		header string
		want   int
	}{
		{"GET needs no crumb", "GET", "/", "", "", 200},
		{"HEAD needs no crumb", "HEAD", "/", "", "", 200},
		{"POST without crumb", "POST", "/", "a=b", "", 400},
		{"POST with crumb", "POST", "/", form, "", 200},
		{"POST with bad crumb", "POST", "/", "crumb=garbage", "", 400},
		{"PUT without crumb", "PUT", "/", "", "", 400},
		{"PUT with header", "PUT", "/", "", token, 200},
		{"PATCH without crumb", "PATCH", "/", "", "", 400},
		{"PATCH with header", "PATCH", "/", "", token, 200},
		{"DELETE without crumb", "DELETE", "/", "", "", 400},
		{"DELETE with header", "DELETE", "/", "", token, 200},
		{"DELETE with query crumb", "DELETE", "/?crumb=" + url.QueryEscape(token), "", "", 200},
		{"DELETE with bad header", "DELETE", "/", "", "garbage", 400},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			called := false

			next := http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
				called = true
				rsp.Header().Set("Content-Type", "text/plain")
			})

			h := EnsureCrumbHandlerWithFaultWrapper(cr, next)

			req := testReq(tc.method, tc.target, tc.body)

			if tc.header != "" {
				req.Header.Set(CrumbHeader, tc.header)
			}

			rsp := httptest.NewRecorder()
			h.ServeHTTP(rsp, req)

			if rsp.Code != tc.want {
				t.Fatalf("got %d, want %d", rsp.Code, tc.want)
			}

			if (tc.want == 200) != called {
				t.Fatalf("next called=%v for status %d", called, rsp.Code)
			}
		})
	}
}

func TestCrumbMiddlewareErrorHandlerBlocksNext(t *testing.T) {

	cr := newTestEncryptedCrumb(t, 60)

	next := http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		t.Error("next handler must not be called")
	})

	errCalls := 0

	errh := http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		errCalls++
		rsp.WriteHeader(http.StatusBadRequest)
	})

	h := EnsureCrumbHandlerWithErrorHandler(cr, next, errh)

	for _, method := range []string{"POST", "PUT", "PATCH", "DELETE"} {

		errCalls = 0

		rsp := httptest.NewRecorder()
		h.ServeHTTP(rsp, testReq(method, "/", ""))

		if errCalls != 1 || rsp.Code != http.StatusBadRequest {
			t.Errorf("%s: expected the error handler to run once, got %d calls (status %d)", method, errCalls, rsp.Code)
		}
	}
}

func TestCrumbMiddlewareInjectsCrumb(t *testing.T) {

	cr := newTestEncryptedCrumb(t, 60)

	const page = `<html><body><form method="post" action="/save"></form>` +
		`<form method="POST"></form>` +
		`<form method="get" action="/search"></form>` +
		`<form method="post" action="https://evil.example/steal"></form>` +
		`<form method="post" action="//evil.example/steal"></form></body></html>`

	next := http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		rsp.Header().Set("Content-Type", "text/html")
		rsp.Write([]byte(page))
	})

	h := EnsureCrumbHandlerWithFaultWrapper(cr, next)

	rsp := httptest.NewRecorder()
	h.ServeHTTP(rsp, testReq("GET", "/", ""))

	body := rsp.Body.String()

	if !strings.Contains(body, "data-crumb=") {
		t.Error("body is missing data-crumb attribute")
	}

	// Only the two same-origin POST forms receive the hidden input.
	if n := strings.Count(body, `name="crumb"`); n != 2 {
		t.Errorf("expected 2 injected crumb inputs, got %d\n%s", n, body)
	}

	if strings.Contains(body, "evil.example/steal\"><input") {
		t.Error("crumb leaked into a cross-origin form")
	}
}

func TestCrumbBindingIsExclusive(t *testing.T) {

	cr := newTestEncryptedCrumb(t, 60)
	req := testReq("GET", "/", "")

	bound, err := cr.Generate(req, "alice")

	if err != nil {
		t.Fatal(err)
	}

	unbound, err := cr.Generate(req)

	if err != nil {
		t.Fatal(err)
	}

	if ok, err := cr.Validate(req, bound, "alice"); !ok || err != nil {
		t.Errorf("bound crumb should validate for its own binding, ok=%v err=%v", ok, err)
	}

	if ok, _ := cr.Validate(req, bound, "bob"); ok {
		t.Error("bound crumb validated for a different binding")
	}

	if ok, _ := cr.Validate(req, ""); ok {
		t.Error("empty crumb validated")
	}

	if ok, _ := cr.Validate(req, bound); ok {
		t.Error("bound crumb validated without a binding")
	}

	if ok, _ := cr.Validate(req, unbound, "alice"); ok {
		t.Error("unbound crumb validated with a binding")
	}
}

func TestEncryptedCrumbRejectsTampering(t *testing.T) {

	cr := newTestEncryptedCrumb(t, 60)
	req := testReq("GET", "/", "")

	token, err := cr.Generate(req)

	if err != nil {
		t.Fatal(err)
	}

	// Flip a character at several positions.
	for _, i := range []int{0, len(token) / 2, len(token) - 2} {

		b := []byte(token)

		// The token is lowercase hex, so use characters that always differ from the original.
		if b[i] == '0' {
			b[i] = '1'
		} else {
			b[i] = '0'
		}

		ok, _ := cr.Validate(req, string(b))

		if ok {
			t.Errorf("tampered crumb (position %d) validated", i)
		}
	}

	if ok, _ := cr.Validate(req, token[:len(token)-4]); ok {
		t.Error("truncated crumb validated")
	}

	other := newTestEncryptedCrumb(t, 60)

	if ok, _ := other.Validate(req, token); ok {
		t.Error("crumb validated under a different key")
	}
}

func TestEncryptedCrumbVaryingInputs(t *testing.T) {

	cr := newTestEncryptedCrumb(t, 60)

	token, err := cr.Generate(testReq("GET", "/", ""))

	if err != nil {
		t.Fatal(err)
	}

	// The configured key ("test") means the path does not matter, but the user agent does.
	req := testReq("GET", "/elsewhere", "")

	if ok, err := cr.Validate(req, token); !ok || err != nil {
		t.Errorf("crumb should validate for the same key and user agent, ok=%v err=%v", ok, err)
	}

	req.Header.Set("User-Agent", "someone-else")

	if ok, _ := cr.Validate(req, token); ok {
		t.Error("crumb validated for a different user agent")
	}
}

func TestEncryptedCrumbExpiry(t *testing.T) {

	cr := newTestEncryptedCrumb(t, 5)
	req := testReq("GET", "/", "")

	base, err := cr.crumbBase(req)

	if err != nil {
		t.Fatal(err)
	}

	hash, err := cr.hashCrumb(base)

	if err != nil {
		t.Fatal(err)
	}

	make := func(age time.Duration) string {
		ts := strconv.FormatInt(time.Now().Add(-age).Unix(), 10)

		enc, err := cr.encryptCrumb(ts + cr.separator + hash)

		if err != nil {
			t.Fatal(err)
		}

		return enc
	}

	if ok, err := cr.Validate(req, make(0)); !ok || err != nil {
		t.Errorf("fresh crumb should validate, ok=%v err=%v", ok, err)
	}

	if ok, err := cr.Validate(req, make(time.Minute)); ok || err == nil {
		t.Errorf("old crumb should be rejected, ok=%v err=%v", ok, err)
	}
}

func TestNewEncryptedCrumbSecretLengths(t *testing.T) {

	ctx := context.Background()

	for n := 0; n <= 40; n++ {

		secret := strings.Repeat("a", n)

		uri := fmt.Sprintf("encrypted://?extra=x&separator=:&ttl=10&secret=%s", secret)

		_, err := NewEncryptedCrumb(ctx, uri)

		valid := n == 16 || n == 24 || n == 32

		if valid && err != nil {
			t.Errorf("length %d should be valid, %v", n, err)
		}

		if !valid && err == nil {
			t.Errorf("length %d should be rejected", n)
		}
	}
}

// Crumb validation must never panic, whatever the input.
func FuzzEncryptedCrumbValidate(f *testing.F) {

	ctx := context.Background()

	uri, err := NewRandomEncryptedCrumbURI(ctx, 60, "test")

	if err != nil {
		f.Fatal(err)
	}

	c, err := NewEncryptedCrumb(ctx, uri)

	if err != nil {
		f.Fatal(err)
	}

	req := testReq("GET", "/", "")

	good, _ := c.Generate(req)

	f.Add(good)
	f.Add("")
	f.Add("!!!!")
	f.Add(good[:len(good)/2])
	f.Add(strings.Repeat("A", 11))
	f.Add(strings.Repeat("A", 200))

	f.Fuzz(func(t *testing.T, s string) {
		c.Validate(req, s)
	})
}
