package fileserver

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestRoot(t *testing.T) string {
	t.Helper()

	parent := t.TempDir()
	root := filepath.Join(parent, "root")

	if err := os.Mkdir(root, 0755); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(root, "hello.txt"), []byte("hello"), 0644); err != nil {
		t.Fatal(err)
	}

	// A file outside the served root that must never be reachable.
	if err := os.WriteFile(filepath.Join(parent, "secret.txt"), []byte("TOPSECRET"), 0644); err != nil {
		t.Fatal(err)
	}

	return root
}

func TestFileServerHandlerServesFiles(t *testing.T) {

	h, err := NewFileServerHandler(&FileServerOptions{Root: newTestRoot(t)})

	if err != nil {
		t.Fatal(err)
	}

	rsp := httptest.NewRecorder()
	h.ServeHTTP(rsp, httptest.NewRequest("GET", "/hello.txt", nil))

	if rsp.Code != http.StatusOK || rsp.Body.String() != "hello" {
		t.Fatalf("Unexpected response %d %q", rsp.Code, rsp.Body.String())
	}

	rsp = httptest.NewRecorder()
	h.ServeHTTP(rsp, httptest.NewRequest("GET", "/missing.txt", nil))

	if rsp.Code != http.StatusNotFound {
		t.Fatalf("Expected 404, got %d", rsp.Code)
	}
}

func TestFileServerHandlerTraversal(t *testing.T) {

	h, err := NewFileServerHandler(&FileServerOptions{Root: newTestRoot(t)})

	if err != nil {
		t.Fatal(err)
	}

	paths := []string{
		"/../secret.txt",
		"/..%2fsecret.txt",
		"/%2e%2e/secret.txt",
		"/%2e%2e%2fsecret.txt",
		"//../secret.txt",
		"/./../secret.txt",
		"/..\\secret.txt",
		"/hello.txt/../../secret.txt",
	}

	for _, p := range paths {
		t.Run(p, func(t *testing.T) {

			req := httptest.NewRequest("GET", "/", nil)
			req.URL.Path = p
			req.URL.RawPath = ""
			req.RequestURI = p

			if dec, err := parseRaw(p); err == nil {
				req.URL.Path = dec
			}

			rsp := httptest.NewRecorder()
			h.ServeHTTP(rsp, req)

			if strings.Contains(rsp.Body.String(), "TOPSECRET") {
				t.Fatalf("Path %q escaped the root", p)
			}
		})
	}
}

// parseRaw decodes percent-escapes in 'p' the way net/http would for a request line.
func parseRaw(p string) (string, error) {
	req, err := http.NewRequest("GET", "http://example.com"+p, nil)

	if err != nil {
		return "", err
	}

	return req.URL.Path, nil
}

func TestFileServerHandlerInvalidRoot(t *testing.T) {

	if _, err := NewFileServerHandler(&FileServerOptions{Root: filepath.Join(t.TempDir(), "nope")}); err == nil {
		t.Fatal("Expected error for missing root")
	}

	f := filepath.Join(t.TempDir(), "file")

	if err := os.WriteFile(f, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	if _, err := NewFileServerHandler(&FileServerOptions{Root: f}); err == nil {
		t.Fatal("Expected error for non-directory root")
	}
}

func TestFileServerHandlerCORS(t *testing.T) {

	h, err := NewFileServerHandler(&FileServerOptions{
		Root:        newTestRoot(t),
		EnableCORS:  true,
		CORSOrigins: []string{"https://allowed.example"},
	})

	if err != nil {
		t.Fatal(err)
	}

	for origin, want := range map[string]string{
		"https://allowed.example": "https://allowed.example",
		"https://evil.example":    "",
	} {
		req := httptest.NewRequest("GET", "/hello.txt", nil)
		req.Header.Set("Origin", origin)

		rsp := httptest.NewRecorder()
		h.ServeHTTP(rsp, req)

		if got := rsp.Header().Get("Access-Control-Allow-Origin"); got != want {
			t.Errorf("Origin %q: got ACAO %q, want %q", origin, got, want)
		}
	}
}

func TestContentTypeHandlerUsesPathNotQuery(t *testing.T) {

	next := http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {})

	h, err := NewContentTypeHandler(&ContentTypeOptions{
		Matches: map[string]string{".geojson": "application/geo+json"},
	}, next)

	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]string{
		"/a.geojson":            "application/geo+json",
		"/a.geojson?x=1":        "application/geo+json",
		"/a.txt?file=b.geojson": "",
		"/a.txt":                "",
	}

	for target, want := range tests {
		rsp := httptest.NewRecorder()
		h.ServeHTTP(rsp, httptest.NewRequest("GET", target, nil))

		if got := rsp.Header().Get("Content-Type"); got != want {
			t.Errorf("%s: got %q, want %q", target, got, want)
		}
	}
}

func TestWithHeadersHandler(t *testing.T) {

	next := http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {})

	h, err := NewWithHeadersHandler(map[string]string{"X-Test": "yes"}, next)

	if err != nil {
		t.Fatal(err)
	}

	rsp := httptest.NewRecorder()
	h.ServeHTTP(rsp, httptest.NewRequest("GET", "/", nil))

	if rsp.Header().Get("X-Test") != "yes" {
		t.Fatal("Missing header")
	}
}
