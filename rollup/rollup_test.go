package rollup

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
)

func testFS() fstest.MapFS {
	return fstest.MapFS{
		"a.css":  {Data: []byte("body {  color : red;  }")},
		"b.css":  {Data: []byte("p { margin: 0 }")},
		"a.js":   {Data: []byte("var  a = 1;")},
		"b.js":   {Data: []byte("var  b = 2;")},
		"bad.js": {Data: []byte("var = = ;")},
	}
}

func get(h http.Handler, method, path string, hdr map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, nil)
	for k, v := range hdr {
		req.Header.Set(k, v)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

func TestRollupCSS(t *testing.T) {

	h, err := RollupCSSHandler(&RollupCSSHandlerOptions{FS: testFS(), Paths: map[string][]string{"all.css": {"a.css", "b.css"}}})

	if err != nil {
		t.Fatal(err)
	}

	rec := get(h, "GET", "/css/all.css", nil)

	if rec.Code != 200 || rec.Body.String() != "body{color:red}p{margin:0}" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}

	ct := rec.Header().Get("Content-Type")

	if ct != "text/css; charset=utf-8" {
		t.Errorf("content type %q", ct)
	}

	rec = get(h, "GET", "/css/none.css", nil)

	if rec.Code != 404 {
		t.Errorf("got %d", rec.Code)
	}

	rec = get(h, "POST", "/css/all.css", nil)

	if rec.Code != 405 || rec.Header().Get("Allow") == "" {
		t.Errorf("got %d", rec.Code)
	}

	rec = get(h, "HEAD", "/css/all.css", nil)

	if rec.Code != 200 || rec.Body.Len() != 0 {
		t.Errorf("head got %d %d", rec.Code, rec.Body.Len())
	}
}

func TestRollupJS(t *testing.T) {

	h, err := RollupJSHandler(&RollupJSHandlerOptions{FS: testFS(), Paths: map[string][]string{"all.js": {"a.js", "b.js"}}})

	if err != nil {
		t.Fatal(err)
	}

	rec := get(h, "GET", "/all.js", nil)

	if rec.Code != 200 || rec.Body.String() != "var a=1;var b=2;" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
}

func TestRollupErrorWritesNothingPartial(t *testing.T) {

	h, err := RollupJSHandler(&RollupJSHandlerOptions{FS: testFS(), Paths: map[string][]string{"all.js": {"a.js", "bad.js"}}})

	if err != nil {
		t.Fatal(err)
	}

	rec := get(h, "GET", "/all.js", nil)

	if rec.Code != 500 {
		t.Fatalf("got %d", rec.Code)
	}

	if strings.Contains(rec.Body.String(), "var a") || strings.Contains(rec.Body.String(), "bad.js") {
		t.Errorf("leaked partial output or path: %q", rec.Body.String())
	}
}

func TestRollupValidation(t *testing.T) {

	_, err := RollupCSSHandler(nil)

	if err == nil {
		t.Error("nil options")
	}

	_, err = RollupCSSHandler(&RollupCSSHandlerOptions{})

	if err == nil {
		t.Error("nil FS")
	}

	_, err = RollupJSHandler(&RollupJSHandlerOptions{FS: testFS(), Paths: map[string][]string{"x.js": {"missing.js"}}})

	if err == nil {
		t.Error("missing file")
	}
}

func TestRollupCache(t *testing.T) {

	fsys := testFS()

	h, err := RollupCSSHandler(&RollupCSSHandlerOptions{FS: fsys, Paths: map[string][]string{"all.css": {"a.css"}}, Cache: true})

	if err != nil {
		t.Fatal(err)
	}

	first := get(h, "GET", "/all.css", nil)
	etag := first.Header().Get("ETag")

	if etag == "" {
		t.Fatal("no etag")
	}

	fsys["a.css"].Data = []byte("body{color:blue}")

	second := get(h, "GET", "/all.css", nil)

	if second.Body.String() != first.Body.String() {
		t.Error("cache not used")
	}

	rec := get(h, "GET", "/all.css", map[string]string{"If-None-Match": etag})

	if rec.Code != 304 {
		t.Errorf("got %d", rec.Code)
	}
}

func TestRollupNoCacheRebuilds(t *testing.T) {

	fsys := testFS()

	h, _ := RollupCSSHandler(&RollupCSSHandlerOptions{FS: fsys, Paths: map[string][]string{"all.css": {"a.css"}}})

	get(h, "GET", "/all.css", nil)
	fsys["a.css"].Data = []byte("body{color:blue}")

	rec := get(h, "GET", "/all.css", nil)

	if !strings.Contains(rec.Body.String(), "blue") {
		t.Errorf("got %q", rec.Body.String())
	}

	rec = get(h, "GET", "/all.css", nil)

	if rec.Header().Get("ETag") != "" {
		t.Error("etag without cache")
	}
}
