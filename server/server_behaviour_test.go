package server

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/aws/aws-lambda-go/events"
)

func TestHTTPServerTLSParameters(t *testing.T) {

	ctx := context.Background()

	dir := t.TempDir()
	cert := filepath.Join(dir, "cert.pem")
	key := filepath.Join(dir, "key.pem")

	for _, p := range []string{cert, key} {
		if err := os.WriteFile(p, []byte("x"), 0600); err != nil {
			t.Fatal(err)
		}
	}

	tests := []struct {
		name    string
		uri     string
		wantErr string
		address string
	}{
		{"plain http", "http://localhost:8081", "", "http://localhost:8081"},
		{"https needs cert and key", "https://localhost:8081", "Missing TLS cert and key", ""},
		{"tls_cert and tls_key", "https://localhost:8081?tls_cert=" + cert + "&tls_key=" + key, "", "https://localhost:8081"},
		{"legacy cert and key", "https://localhost:8081?cert=" + cert + "&key=" + key, "", "https://localhost:8081"},
		{"http upgraded by cert and key", "http://localhost:8081?tls_cert=" + cert + "&tls_key=" + key, "", "https://localhost:8081"},
		{"cert without key", "https://localhost:8081?tls_cert=" + cert, "Missing TLS key", ""},
		{"key without cert", "https://localhost:8081?tls_key=" + key, "Missing TLS cert", ""},
		{"legacy cert without key", "https://localhost:8081?cert=" + cert, "Missing TLS key", ""},
		{"cert file missing", "https://localhost:8081?tls_cert=" + cert + ".nope&tls_key=" + key, "no such file", ""},
		{"bad timeout", "http://localhost:8081?read_timeout=abc", "invalid syntax", ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			s, err := NewHTTPServer(ctx, tc.uri)

			if tc.wantErr != "" {

				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			if s.Address() != tc.address {
				t.Fatalf("got address %q, want %q", s.Address(), tc.address)
			}
		})
	}
}

func TestHTTPServerTimeouts(t *testing.T) {

	s, err := NewHTTPServer(context.Background(), "http://localhost:8081?read_timeout=3&write_timeout=4&idle_timeout=5&header_timeout=6")

	if err != nil {
		t.Fatal(err)
	}

	srv := s.(*HTTPServer).http_server

	got := []int64{int64(srv.ReadTimeout.Seconds()), int64(srv.WriteTimeout.Seconds()), int64(srv.IdleTimeout.Seconds()), int64(srv.ReadHeaderTimeout.Seconds())}
	want := []int64{3, 4, 5, 6}

	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("timeouts: got %v, want %v", got, want)
		}
	}
}

func TestMkCertServerRejectsBadRoot(t *testing.T) {

	ctx := context.Background()

	if _, err := NewMkCertServer(ctx, "mkcert://localhost:8081?root="+filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Fatal("expected error for missing root")
	}

	f := filepath.Join(t.TempDir(), "file")

	if err := os.WriteFile(f, []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}

	if _, err := NewMkCertServer(ctx, "mkcert://localhost:8081?root="+f); err == nil {
		t.Fatal("expected error for non-directory root")
	}
}

func TestFunctionURLCookiesAndErrors(t *testing.T) {

	s := &LambdaFunctionURLServer{binaryContentTypes: map[string]bool{"image/png": true}}

	s.handler = http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {

		switch req.URL.Path {
		case "/cookies":
			http.SetCookie(rsp, &http.Cookie{Name: "a", Value: "1", Path: "/", MaxAge: 60})
			http.SetCookie(rsp, &http.Cookie{Name: "b", Value: "2", Path: "/"})
			rsp.Header().Add("X-Multi", "one")
			rsp.Header().Add("X-Multi", "two")
			rsp.Write([]byte("ok"))
		case "/binary":
			rsp.Header().Set("Content-Type", "image/png")
			rsp.Write([]byte{0x89, 'P', 'N', 'G'})
		}
	})

	newEvent := func(path string) events.LambdaFunctionURLRequest {
		ev := events.LambdaFunctionURLRequest{RawPath: path, Headers: map[string]string{"host": "example.com"}}
		ev.RequestContext.HTTP.Method = "GET"
		return ev
	}

	rsp, err := s.handleRequest(context.Background(), newEvent("/cookies"))

	if err != nil {
		t.Fatal(err)
	}

	if len(rsp.Cookies) != 2 {
		t.Fatalf("expected 2 cookies, got %v", rsp.Cookies)
	}

	for k := range rsp.Headers {
		if http.CanonicalHeaderKey(k) == "Set-Cookie" {
			t.Fatalf("Set-Cookie must not appear in Headers")
		}
	}

	if rsp.Headers["X-Multi"] != "one,two" {
		t.Fatalf("got X-Multi %q", rsp.Headers["X-Multi"])
	}

	rsp, err = s.handleRequest(context.Background(), newEvent("/binary"))

	if err != nil {
		t.Fatal(err)
	}

	if !rsp.IsBase64Encoded {
		t.Fatal("binary response should be base64 encoded")
	}

	bad := newEvent("/%zz")
	rsp, err = s.handleRequest(context.Background(), bad)

	if err != nil {
		t.Fatalf("handler errors must be reported in the response, got %v", err)
	}

	if rsp.StatusCode != 400 || strings.Contains(rsp.Body, "%zz") || strings.Contains(rsp.Body, "escape") {
		t.Fatalf("expected generic 400, got %d %q", rsp.StatusCode, rsp.Body)
	}
}
