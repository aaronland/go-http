package route

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
)

func tagHandler(tag string) RouteHandlerFunc {
	return func(ctx context.Context) (http.Handler, error) {
		return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
			rsp.Write([]byte(tag))
		}), nil
	}
}

func anchored(handlers map[string]RouteHandlerFunc) (http.Handler, error) {
	return RouteHandlerWithOptions(&RouteHandlerOptions{Handlers: handlers, EndAnchored: true})
}

func serve(t *testing.T, h http.Handler, method string, host string, path string) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, nil)

	if host != "" {
		req.Host = host
	}

	rsp := httptest.NewRecorder()
	h.ServeHTTP(rsp, req)
	return rsp
}

func TestRouteMethodAndHostConstraints(t *testing.T) {

	h, err := RouteHandler(map[string]RouteHandlerFunc{
		"POST /submit":              tagHandler("submit"),
		"example.com/hosted":        tagHandler("hosted"),
		"GET example.com/combined":  tagHandler("combined"),
		"/open":                     tagHandler("open"),
		"/things/{id}/children/{c}": tagHandler("things"),
	})

	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		method string
		host   string
		path   string
		code   int
		body   string
	}{
		{"method matches", "POST", "", "/submit", 200, "submit"},
		{"method mismatch", "GET", "", "/submit", 405, ""},
		{"host matches", "GET", "example.com", "/hosted", 200, "hosted"},
		{"host mismatch", "GET", "other.example", "/hosted", 404, ""},
		{"method and host match", "GET", "example.com", "/combined", 200, "combined"},
		{"method and host, bad host", "GET", "other.example", "/combined", 404, ""},
		{"method and host, bad method", "POST", "example.com", "/combined", 405, ""},
		{"unconstrained", "DELETE", "", "/open", 200, "open"},
		{"unknown path", "GET", "", "/missing", 404, ""},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			rsp := serve(t, h, tc.method, tc.host, tc.path)

			if rsp.Code != tc.code {
				t.Fatalf("got %d, want %d (%q)", rsp.Code, tc.code, rsp.Body.String())
			}

			if tc.body != "" && rsp.Body.String() != tc.body {
				t.Fatalf("got body %q, want %q", rsp.Body.String(), tc.body)
			}
		})
	}
}

// Constraints on one route must not leak into the matching of the next (regression for loop-variable reuse).
func TestRouteConstraintsDoNotLeakBetweenPatterns(t *testing.T) {

	h, err := RouteHandler(map[string]RouteHandlerFunc{
		"POST /aaaaaaaa/{x}": tagHandler("post-wildcard"),
		"/b":                 tagHandler("plain"),
	})

	if err != nil {
		t.Fatal(err)
	}

	rsp := serve(t, h, "GET", "", "/b")

	if rsp.Code != 200 || rsp.Body.String() != "plain" {
		t.Fatalf("got %d %q", rsp.Code, rsp.Body.String())
	}
}

func TestRoutePathValues(t *testing.T) {

	var gotID, gotChild string

	h, err := RouteHandler(map[string]RouteHandlerFunc{
		"/things/{id}/children/{c}": func(ctx context.Context) (http.Handler, error) {
			return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
				gotID = req.PathValue("id")
				gotChild = req.PathValue("c")
			}), nil
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	rsp := serve(t, h, "GET", "", "/things/42/children/abc")

	if rsp.Code != 200 {
		t.Fatalf("got %d", rsp.Code)
	}

	if gotID != "42" || gotChild != "abc" {
		t.Fatalf("got id=%q child=%q", gotID, gotChild)
	}

	// The wildcard pattern is anchored at the start so an unrelated prefix must not match.
	rsp = serve(t, h, "GET", "", "/other/things/42/children/abc")

	if rsp.Code != 404 {
		t.Fatalf("expected 404 for unanchored path, got %d", rsp.Code)
	}
}

func TestRouteLongestPatternWins(t *testing.T) {

	h, err := RouteHandler(map[string]RouteHandlerFunc{
		"/api/":        tagHandler("short"),
		"/api/special": tagHandler("long"),
	})

	if err != nil {
		t.Fatal(err)
	}

	rsp := serve(t, h, "GET", "", "/api/special")

	if rsp.Body.String() != "long" {
		t.Fatalf("got %q", rsp.Body.String())
	}

	rsp = serve(t, h, "GET", "", "/api/other")

	if rsp.Body.String() != "short" {
		t.Fatalf("got %q", rsp.Body.String())
	}
}

// A pattern without a trailing slash is an exact match, one with it is a subtree. Matching is by
// path segment so "/admin" can never capture "/administrator".
func TestRouteExactAndSubtree(t *testing.T) {

	h, err := anchored(map[string]RouteHandlerFunc{
		"/admin":  tagHandler("exact"),
		"/files/": tagHandler("subtree"),
		"/{$}":    tagHandler("root"),
	})

	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		code int
		body string
	}{
		"/admin":         {200, "exact"},
		"/admin/users":   {404, ""},
		"/administrator": {404, ""},
		"/files/":        {200, "subtree"},
		"/files/a/b":     {200, "subtree"},
		"/filesystem":    {404, ""},
		"/":              {200, "root"},
		"/other":         {404, ""},
	}

	for path, want := range tests {

		rsp := serve(t, h, "GET", "", path)

		if rsp.Code != want.code || (want.body != "" && rsp.Body.String() != want.body) {
			t.Errorf("%s: got %d %q, want %d %q", path, rsp.Code, rsp.Body.String(), want.code, want.body)
		}
	}
}

func TestRouteSamePathDifferentMethods(t *testing.T) {

	h, err := RouteHandler(map[string]RouteHandlerFunc{
		"GET /a/{id}":    tagHandler("get"),
		"POST /a/{id}":   tagHandler("post"),
		"PATCH /p":       tagHandler("patch"),
		"OPTIONS /o":     tagHandler("options"),
		"DELETE /a/{id}": tagHandler("delete"),
	})

	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct{ method, path, body string }{
		{"GET", "/a/1", "get"},
		{"HEAD", "/a/1", ""},
		{"POST", "/a/1", "post"},
		{"DELETE", "/a/1", "delete"},
		{"PATCH", "/p", "patch"},
		{"OPTIONS", "/o", "options"},
	} {

		rsp := serve(t, h, tc.method, "", tc.path)

		if rsp.Code != 200 || (tc.method != "HEAD" && rsp.Body.String() != tc.body) {
			t.Errorf("%s %s: got %d %q", tc.method, tc.path, rsp.Code, rsp.Body.String())
		}
	}

	rsp := serve(t, h, "PUT", "", "/a/1")

	if rsp.Code != 405 || rsp.Header().Get("Allow") != "DELETE, GET, HEAD, POST" {
		t.Errorf("got %d Allow=%q", rsp.Code, rsp.Header().Get("Allow"))
	}

	rsp = serve(t, h, "GET", "", "/nope")

	if rsp.Code != 404 {
		t.Errorf("got %d", rsp.Code)
	}
}

func TestRouteSpecificity(t *testing.T) {

	h, err := anchored(map[string]RouteHandlerFunc{
		"/id/{id}":      tagHandler("wild"),
		"/id/abc":       tagHandler("literal"),
		"/id/{rest...}": tagHandler("rest"),
		"/x/{a}/{b}":    tagHandler("ab"),
		"/x/{a}/fixed":  tagHandler("a-fixed"),
		"example.com/h": tagHandler("host"),
		"/h":            tagHandler("nohost"),
	})

	if err != nil {
		t.Fatal(err)
	}

	// Run repeatedly: ordering must not depend on map iteration.
	for i := 0; i < 50; i++ {

		for path, want := range map[string]string{
			"/id/abc":    "literal",
			"/id/other":  "wild",
			"/id/a/b":    "rest",
			"/x/1/fixed": "a-fixed",
			"/x/1/2":     "ab",
		} {

			rsp := serve(t, h, "GET", "", path)

			if rsp.Body.String() != want {
				t.Fatalf("%s: got %q, want %q", path, rsp.Body.String(), want)
			}
		}

		rsp := serve(t, h, "GET", "example.com:8080", "/h")

		if rsp.Body.String() != "host" {
			t.Fatalf("got %q", rsp.Body.String())
		}

		rsp = serve(t, h, "GET", "other.com", "/h")

		if rsp.Body.String() != "nohost" {
			t.Fatalf("got %q", rsp.Body.String())
		}
	}
}

func TestRouteEscapedSlashStaysInSegment(t *testing.T) {

	var got string

	h, _ := anchored(map[string]RouteHandlerFunc{
		"/u/{id}": func(ctx context.Context) (http.Handler, error) {
			return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
				got = req.PathValue("id")
			}), nil
		},
	})

	rsp := serve(t, h, "GET", "", "/u/a%2Fb")

	if rsp.Code != 200 || got != "a/b" {
		t.Fatalf("got %d %q", rsp.Code, got)
	}

	rsp = serve(t, h, "GET", "", "/u/a/b")

	if rsp.Code != 404 {
		t.Fatalf("got %d", rsp.Code)
	}
}

func TestRouteRestWildcard(t *testing.T) {

	var got string

	h, _ := RouteHandler(map[string]RouteHandlerFunc{
		"/files/{path...}": func(ctx context.Context) (http.Handler, error) {
			return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
				got = req.PathValue("path")
			}), nil
		},
	})

	rsp := serve(t, h, "GET", "", "/files/a/b.txt")

	if rsp.Code != 200 || got != "a/b.txt" {
		t.Fatalf("got %d %q", rsp.Code, got)
	}
}

func TestRouteInvalidPatterns(t *testing.T) {

	for _, p := range []string{
		"foo",
		"FETCH /x",
		"/x/{",
		"/x/}",
		"/x/{}",
		"/x/a{id}",
		"/x/{id}b",
		"/x/{a}/{a}",
		"/x/{rest...}/more",
		"/x/{$}/more",
	} {
		_, err := RouteHandler(map[string]RouteHandlerFunc{p: tagHandler("x")})

		if err == nil {
			t.Errorf("expected an error for pattern %q", p)
		}
	}

	_, err := RouteHandler(map[string]RouteHandlerFunc{"/x": nil})

	if err == nil {
		t.Error("expected an error for a nil handler func")
	}

	_, err = RouteHandlerWithOptions(nil)

	if err == nil {
		t.Error("expected an error for nil options")
	}
}

func TestRouteNilHandlerIs500(t *testing.T) {

	h, _ := RouteHandler(map[string]RouteHandlerFunc{
		"/nil": func(ctx context.Context) (http.Handler, error) { return nil, nil },
	})

	rsp := serve(t, h, "GET", "", "/nil")

	if rsp.Code != 500 {
		t.Fatalf("got %d", rsp.Code)
	}
}

// The handler is built once even under concurrent first requests, and a failed build is retried.
func TestRouteHandlerInitializedOnce(t *testing.T) {

	var calls atomic.Int32
	var fail atomic.Bool
	fail.Store(true)

	h, _ := RouteHandler(map[string]RouteHandlerFunc{
		"/once": func(ctx context.Context) (http.Handler, error) {
			calls.Add(1)

			if fail.Load() {
				return nil, http.ErrAbortHandler
			}

			return tagHandler("ok")(ctx)
		},
	})

	rsp := serve(t, h, "GET", "", "/once")

	if rsp.Code != 500 {
		t.Fatalf("got %d", rsp.Code)
	}

	fail.Store(false)
	calls.Store(0)

	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			rsp := serve(t, h, "GET", "", "/once")

			if rsp.Code != 200 {
				t.Errorf("got %d", rsp.Code)
			}
		}()
	}

	wg.Wait()

	n := calls.Load()

	if n != 1 {
		t.Fatalf("handler built %d times", n)
	}
}

func TestRouteHandlerFactoryError(t *testing.T) {

	h, err := RouteHandler(map[string]RouteHandlerFunc{
		"/broken": func(ctx context.Context) (http.Handler, error) {
			return nil, http.ErrAbortHandler
		},
	})

	if err != nil {
		t.Fatal(err)
	}

	rsp := serve(t, h, "GET", "", "/broken")

	if rsp.Code != 500 {
		t.Fatalf("expected 500, got %d", rsp.Code)
	}
}

// The default is to match by prefix, as earlier versions did.
func TestRouteDefaultIsPrefixMatching(t *testing.T) {

	h, err := RouteHandler(map[string]RouteHandlerFunc{
		"/admin":         tagHandler("admin"),
		"/id/{id}":       tagHandler("id"),
		"POST /foo/post": tagHandler("post"),
		"/foo":           tagHandler("foo"),
	})

	if err != nil {
		t.Fatal(err)
	}

	for path, want := range map[string]int{
		"/admin":         200,
		"/admin/users":   200,
		"/administrator": 200,
		"/id/1/extra":    200,
		"/foo/":          200,
		"/foo/other":     200,
		"/nope":          404,
	} {
		rsp := serve(t, h, "GET", "", path)

		if rsp.Code != want {
			t.Errorf("%s: got %d, want %d", path, rsp.Code, want)
		}
	}

	// A method mismatch on the most specific pattern is a 405, not a fallback to "/foo".
	rsp := serve(t, h, "GET", "", "/foo/post")

	if rsp.Code != 405 {
		t.Errorf("got %d", rsp.Code)
	}

	// Longer patterns still win over shorter prefixes.
	h2, _ := RouteHandler(map[string]RouteHandlerFunc{
		"/admin":         tagHandler("short"),
		"/administrator": tagHandler("long"),
	})

	rsp = serve(t, h2, "GET", "", "/administrator/x")

	if rsp.Body.String() != "long" {
		t.Errorf("got %q", rsp.Body.String())
	}
}
