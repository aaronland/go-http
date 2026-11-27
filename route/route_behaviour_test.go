package route

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func tagHandler(tag string) RouteHandlerFunc {
	return func(ctx context.Context) (http.Handler, error) {
		return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
			rsp.Write([]byte(tag))
		}), nil
	}
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

	if rsp := serve(t, h, "GET", "", "/b"); rsp.Code != 200 || rsp.Body.String() != "plain" {
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

	if rsp := serve(t, h, "GET", "", "/things/42/children/abc"); rsp.Code != 200 {
		t.Fatalf("got %d", rsp.Code)
	}

	if gotID != "42" || gotChild != "abc" {
		t.Fatalf("got id=%q child=%q", gotID, gotChild)
	}

	// The wildcard pattern is anchored at the start so an unrelated prefix must not match.
	if rsp := serve(t, h, "GET", "", "/other/things/42/children/abc"); rsp.Code != 404 {
		t.Fatalf("expected 404 for unanchored path, got %d", rsp.Code)
	}
}

func TestRouteLongestPatternWins(t *testing.T) {

	h, err := RouteHandler(map[string]RouteHandlerFunc{
		"/api":         tagHandler("short"),
		"/api/special": tagHandler("long"),
	})

	if err != nil {
		t.Fatal(err)
	}

	if rsp := serve(t, h, "GET", "", "/api/special/x"); rsp.Body.String() != "long" {
		t.Fatalf("got %q", rsp.Body.String())
	}

	if rsp := serve(t, h, "GET", "", "/api/other"); rsp.Body.String() != "short" {
		t.Fatalf("got %q", rsp.Body.String())
	}
}

// Patterns are matched as string prefixes, not path segments. This test pins that behaviour
// so a change to it is a conscious one.
func TestRoutePrefixMatchingIsNotSegmentAware(t *testing.T) {

	h, err := RouteHandler(map[string]RouteHandlerFunc{
		"/admin": tagHandler("admin"),
	})

	if err != nil {
		t.Fatal(err)
	}

	if rsp := serve(t, h, "GET", "", "/admin/users"); rsp.Code != 200 {
		t.Fatalf("expected /admin/users to match, got %d", rsp.Code)
	}

	if rsp := serve(t, h, "GET", "", "/administrator"); rsp.Code != 200 {
		t.Fatalf("/administrator no longer matches /admin (got %d); update this test and the docs if that is intended", rsp.Code)
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

	if rsp := serve(t, h, "GET", "", "/broken"); rsp.Code != 500 {
		t.Fatalf("expected 500, got %d", rsp.Code)
	}
}
