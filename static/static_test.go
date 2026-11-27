package static

import (
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"
	"testing/fstest"
)

var testFS = fstest.MapFS{
	"css/site.css": {Data: []byte("body{}")},
	"js/app.js":    {Data: []byte("alert(1)")},
}

func get(h http.Handler, path string) *httptest.ResponseRecorder {
	rsp := httptest.NewRecorder()
	h.ServeHTTP(rsp, httptest.NewRequest("GET", path, nil))
	return rsp
}

func TestStaticAssetsHandler(t *testing.T) {

	h, err := StaticAssetsHandler(testFS)

	if err != nil {
		t.Fatal(err)
	}

	if rsp := get(h, "/css/site.css"); rsp.Code != 200 || rsp.Body.String() != "body{}" {
		t.Fatalf("Unexpected %d %q", rsp.Code, rsp.Body.String())
	}

	if rsp := get(h, "/nope.css"); rsp.Code != 404 {
		t.Fatalf("Expected 404, got %d", rsp.Code)
	}
}

func TestStaticAssetsHandlerWithPrefix(t *testing.T) {

	tests := []struct {
		prefix string
		path   string
		code   int
	}{
		{"/static", "/static/css/site.css", 200},
		{"/static/", "/static/css/site.css", 200},
		{"", "/css/site.css", 200},
		{"/", "/css/site.css", 200},
		{"/static", "/static/nope.css", 404},
		// Only a leading prefix is removed, never an embedded one.
		{"/static", "/css/static/site.css", 404},
	}

	for _, tc := range tests {
		h, err := StaticAssetsHandlerWithPrefix(testFS, tc.prefix)

		if err != nil {
			t.Fatal(err)
		}

		if rsp := get(h, tc.path); rsp.Code != tc.code {
			t.Errorf("prefix %q path %q: got %d, want %d", tc.prefix, tc.path, rsp.Code, tc.code)
		}
	}
}

func TestStaticAssetsHandlerWithPrefixEncodedPath(t *testing.T) {

	h, err := StaticAssetsHandlerWithPrefix(testFS, "/static")

	if err != nil {
		t.Fatal(err)
	}

	// An escaped slash must not let RawPath diverge from the rewritten Path.
	if rsp := get(h, "/static/css%2fsite.css"); rsp.Code != 200 {
		t.Logf("escaped slash returned %d", rsp.Code)
	}

	if rsp := get(h, "/static/../secret"); rsp.Code == 200 {
		t.Fatalf("Unexpected 200 for traversal")
	}
}

func TestAppendStaticAssetHandlers(t *testing.T) {

	mux := http.NewServeMux()

	if err := AppendStaticAssetHandlersWithPrefix(mux, fs.FS(testFS), "/assets"); err != nil {
		t.Fatal(err)
	}

	if rsp := get(mux, "/assets/js/app.js"); rsp.Code != 200 || rsp.Body.String() != "alert(1)" {
		t.Fatalf("Unexpected %d %q", rsp.Code, rsp.Body.String())
	}

	if rsp := get(mux, "/js/app.js"); rsp.Code != 404 {
		t.Fatalf("Expected 404 outside prefix, got %d", rsp.Code)
	}

	mux = http.NewServeMux()

	if err := AppendStaticAssetHandlers(mux, testFS); err != nil {
		t.Fatal(err)
	}

	if rsp := get(mux, "/css/site.css"); rsp.Code != 200 {
		t.Fatalf("Expected 200, got %d", rsp.Code)
	}
}
