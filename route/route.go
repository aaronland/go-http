// Package route provides an `http.Handler` that routes requests using `http.ServeMux` style patterns
// but defers initializing the handlers being routed to until they are needed.
package route

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Regular expression to match "{label}" style wildcards in URL patterns.
var re_braces = regexp.MustCompile(`\{([^\}]+)\}`)

// Regular expression to match Go 1.22 "[METHOD ][HOST]/[PATH]" URL patterns.
var re_route = regexp.MustCompile(`^(?:([A-Z]+)\s+)?([^/\s]*)(/.*)$`)

var valid_methods = map[string]bool{
	http.MethodGet:     true,
	http.MethodHead:    true,
	http.MethodPost:    true,
	http.MethodPut:     true,
	http.MethodPatch:   true,
	http.MethodDelete:  true,
	http.MethodOptions: true,
	http.MethodConnect: true,
	http.MethodTrace:   true,
}

// slash_placeholder stands in for an escaped slash ("%2F") inside a path segment so that a wildcard
// does not match across segments. It can never appear in a pattern.
const slash_placeholder = "\x00"

// RouteHandlerFunc returns an `http.Handler` instance.
type RouteHandlerFunc func(context.Context) (http.Handler, error)

// RouteHandlerOptions is a struct that contains configuration settings
// for use with the RouteHandlerWithOptions function.
type RouteHandlerOptions struct {
	// Handlers is a map whose keys are `http.ServeMux` style routing patterns and whose values
	// are functions that when invoked return `http.Handler` instances.
	Handlers map[string]RouteHandlerFunc
	// EndAnchored, if true, makes patterns match like `http.ServeMux` does: a pattern that does not end
	// in a slash matches only that exact path and one that ends in a slash matches that path and everything
	// beneath it. If false (the default) a pattern matches any path that begins with it, so "/admin" will
	// also match "/admin/users" and "/administrator".
	EndAnchored bool
}

// RouteHandler creates a new `http.Handler` instance that will serve requests using handlers defined in 'handlers'.
func RouteHandler(handlers map[string]RouteHandlerFunc) (http.Handler, error) {

	opts := &RouteHandlerOptions{
		Handlers: handlers,
	}

	return RouteHandlerWithOptions(opts)
}

// route is a single parsed pattern and its lazily initialized handler.
type route struct {
	pattern string
	method  string
	host    string
	re      *regexp.Regexp
	keys    []string
	ranks   []int
	path    string
	pathLen int
	factory RouteHandlerFunc

	mu      sync.Mutex
	handler atomic.Pointer[http.Handler]
}

// get returns the route's handler, initializing it on first use. A failed initialization is not cached.
func (r *route) get(ctx context.Context) (http.Handler, error) {

	if h := r.handler.Load(); h != nil {
		return *h, nil
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if h := r.handler.Load(); h != nil {
		return *h, nil
	}

	// A canceled first request must not poison the handler that every later request will share.
	h, err := r.factory(context.WithoutCancel(ctx))

	if err != nil {
		return nil, fmt.Errorf("Failed to instantiate handler func for '%s', %w", r.pattern, err)
	}

	if h == nil {
		return nil, fmt.Errorf("Handler func for '%s' returned a nil handler", r.pattern)
	}

	r.handler.Store(&h)
	return h, nil
}

// allows returns true if a request using 'method' may be served by the route. A "GET" route also serves "HEAD".
func (r *route) allows(method string) bool {

	if r.method == "" || r.method == method {
		return true
	}

	return r.method == http.MethodGet && method == http.MethodHead
}

// RouteHandlerWithOptions creates a new `http.Handler` instance that will serve requests using handlers defined
// in 'opts.Handlers'. This is essentially a "middleware" handler that does all the same routing that the default
// `http.ServeMux` handler does but defers initiating the handlers being routed to until they are invoked at runtime.
// Only one handler is initialized (and then cached) for any given pattern.
//
// The reason this handler exists is for web applications that:
//
//  1. Are deployed as AWS Lambda functions (with an API Gateway integration) using the "lambda://" `server.Server`
//     implementation that have more handlers than you need or want to initiate, but never use, for every request.
//  2. You don't want to refactor in to (n) atomic Lambda functions. That is you want to be able to re-use the same
//     code in both a plain-vanilla HTTP server configuration as well as Lambda + API Gateway configuration.
//
// Patterns follow Go 1.22's `http.ServeMux` syntax: "[METHOD ][HOST]/[PATH]". By default a pattern matches any
// path that begins with it. If `RouteHandlerOptions.EndAnchored` is true a pattern that does not end in a
// slash matches only that exact path, and one that ends in a slash matches that path and everything beneath it.
// Wildcards are "{name}" (one path segment), "{name...}" (the remainder of the path, final segment only) and
// "{$}" (the end of the path, final segment only). A "GET" pattern also matches "HEAD" requests.
//
// When more than one pattern matches a request the most specific one wins: patterns with a host beat those
// without, literal segments beat wildcards and longer patterns beat shorter ones. If a path matches only patterns
// restricted to other methods the response is a 405 with an "Allow" header. This is a regular expression based
// implementation, not a clone of the one in [net/http], so unusual patterns may behave differently. Patterns are
// parsed and validated once, when the handler is created, and an invalid pattern is returned as an error.
func RouteHandlerWithOptions(opts *RouteHandlerOptions) (http.Handler, error) {

	if opts == nil {
		return nil, fmt.Errorf("Missing options")
	}

	end_anchored := opts.EndAnchored
	routes := make([]*route, 0, len(opts.Handlers))

	for pattern, factory := range opts.Handlers {

		if factory == nil {
			return nil, fmt.Errorf("Missing handler func for pattern '%s'", pattern)
		}

		r, err := parsePattern(pattern, opts.EndAnchored)

		if err != nil {
			return nil, fmt.Errorf("Invalid pattern '%s', %w", pattern, err)
		}

		r.factory = factory
		routes = append(routes, r)
	}

	sort.Slice(routes, func(i, j int) bool {
		return moreSpecific(routes[i], routes[j])
	})

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		logger := slog.Default().With("path", req.URL.Path)

		path := normalizePath(req)
		host := stripPort(req.Host)

		allowed := make(map[string]bool)

		// Without end anchoring the first (most specific) pattern whose path matches owns the request, so a
		// method mismatch is a 405 rather than a reason to fall back to a shorter prefix.
		blocked_path := ""
		blocked := false

		for _, r := range routes {

			if r.host != "" && r.host != host {
				continue
			}

			m := r.re.FindStringSubmatch(path)

			if m == nil {
				continue
			}

			if blocked && r.path != blocked_path {
				continue
			}

			if !r.allows(req.Method) {

				if !end_anchored {
					blocked = true
					blocked_path = r.path
				}

				allowed[r.method] = true

				if r.method == http.MethodGet {
					allowed[http.MethodHead] = true
				}

				continue
			}

			h, err := r.get(req.Context())

			if err != nil {
				logger.Error("Failed to derive handler", "error", err)
				http.Error(rsp, "Internal server error", http.StatusInternalServerError)
				return
			}

			for i, key := range r.keys {
				req.SetPathValue(key, strings.ReplaceAll(m[i+1], slash_placeholder, "/"))
			}

			logger.Debug("Serve derived handler", "pattern", r.pattern)
			h.ServeHTTP(rsp, req)
			return
		}

		if len(allowed) > 0 {

			methods := make([]string, 0, len(allowed))

			for m := range allowed {
				methods = append(methods, m)
			}

			sort.Strings(methods)

			logger.Debug("Invalid method for route handler", "method", req.Method, "allow", methods)
			rsp.Header().Set("Allow", strings.Join(methods, ", "))
			http.Error(rsp, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		logger.Debug("Route handler not found")
		http.Error(rsp, "Not found", http.StatusNotFound)
	}

	return http.HandlerFunc(fn), nil
}

// parsePattern parses and validates 'pattern' and compiles its path in to a regular expression.
func parsePattern(pattern string, end_anchored bool) (*route, error) {

	m := re_route.FindStringSubmatch(pattern)

	if m == nil {
		return nil, fmt.Errorf("Expected [METHOD ][HOST]/[PATH]")
	}

	method, host, path := m[1], m[2], m[3]

	if method != "" && !valid_methods[method] {
		return nil, fmt.Errorf("Unknown method '%s'", method)
	}

	re, keys, err := compilePath(path, end_anchored)

	if err != nil {
		return nil, err
	}

	return &route{
		pattern: pattern,
		method:  method,
		host:    host,
		re:      re,
		keys:    keys,
		ranks:   segmentRanks(path),
		path:    path,
		pathLen: len(path),
	}, nil
}

// compilePath converts the path portion of a pattern in to a regular expression (anchored at the end if 'end_anchored' is true or the pattern ends in "{$}" or "{name...}") and the
// ordered list of wildcard names that its capture groups correspond to.
func compilePath(path string, end_anchored bool) (*regexp.Regexp, []string, error) {

	var sb strings.Builder
	sb.WriteString("(?s)^")

	keys := make([]string, 0)
	seen := make(map[string]bool)

	pos := 0
	open_ended := false

	for _, idx := range re_braces.FindAllStringSubmatchIndex(path, -1) {

		lit := path[pos:idx[0]]

		if strings.ContainsAny(lit, "{}") {
			return nil, nil, fmt.Errorf("Unbalanced brace")
		}

		next := idx[1]

		if !strings.HasSuffix(lit, "/") || (next < len(path) && path[next] != '/') {
			return nil, nil, fmt.Errorf("Wildcards must be a whole path segment")
		}

		sb.WriteString(regexp.QuoteMeta(lit))

		name := path[idx[2]:idx[3]]

		switch {
		case name == "$":

			if next != len(path) {
				return nil, nil, fmt.Errorf("{$} must be the final segment")
			}

			open_ended = true

		case strings.HasSuffix(name, "..."):

			if next != len(path) {
				return nil, nil, fmt.Errorf("{...} wildcards must be the final segment")
			}

			name = strings.TrimSuffix(name, "...")
			sb.WriteString("(.*)")
			open_ended = true

			if err := addKey(&keys, seen, name); err != nil {
				return nil, nil, err
			}

		default:

			sb.WriteString("([^/]+)")

			if err := addKey(&keys, seen, name); err != nil {
				return nil, nil, err
			}
		}

		pos = next
	}

	tail := path[pos:]

	if strings.ContainsAny(tail, "{}") {
		return nil, nil, fmt.Errorf("Unbalanced brace")
	}

	sb.WriteString(regexp.QuoteMeta(tail))

	switch {
	case open_ended || end_anchored:

		if !open_ended && strings.HasSuffix(path, "/") {
			sb.WriteString(".*")
		}

		sb.WriteString("$")
	}

	re, err := regexp.Compile(sb.String())

	if err != nil {
		return nil, nil, fmt.Errorf("Failed to compile wildcard regexp, %w", err)
	}

	return re, keys, nil
}

func addKey(keys *[]string, seen map[string]bool, name string) error {

	if name == "" {
		return fmt.Errorf("Empty wildcard name")
	}

	if seen[name] {
		return fmt.Errorf("Duplicate wildcard name '%s'", name)
	}

	seen[name] = true
	*keys = append(*keys, name)
	return nil
}

// segmentRanks scores each path segment: 2 for a literal (or "{$}"), 1 for "{name}", 0 for "{name...}" and
// for the open-ended tail of a pattern that ends in a slash.
func segmentRanks(path string) []int {

	segments := strings.Split(path, "/")
	ranks := make([]int, len(segments))

	for i, s := range segments {

		switch {
		case s == "" && i == len(segments)-1 && i > 0:
			ranks[i] = 0
		case s == "{$}":
			ranks[i] = 2
		case strings.HasPrefix(s, "{") && strings.HasSuffix(s, "...}"):
			ranks[i] = 0
		case strings.HasPrefix(s, "{") && strings.HasSuffix(s, "}"):
			ranks[i] = 1
		default:
			ranks[i] = 2
		}
	}

	return ranks
}

// moreSpecific returns true if 'a' should be tried before 'b'. The ordering is total so that routing
// does not depend on map iteration order.
func moreSpecific(a, b *route) bool {

	if (a.host != "") != (b.host != "") {
		return a.host != ""
	}

	for i := 0; i < len(a.ranks) && i < len(b.ranks); i++ {

		if a.ranks[i] != b.ranks[i] {
			return a.ranks[i] > b.ranks[i]
		}
	}

	if len(a.ranks) != len(b.ranks) {
		return len(a.ranks) > len(b.ranks)
	}

	if a.pathLen != b.pathLen {
		return a.pathLen > b.pathLen
	}

	if (a.method != "") != (b.method != "") {
		return a.method != ""
	}

	return a.pattern < b.pattern
}

// normalizePath returns the request path with each segment unescaped, except that an escaped slash is
// replaced by a placeholder so that it can't be mistaken for a segment boundary.
func normalizePath(req *http.Request) string {

	segments := strings.Split(req.URL.EscapedPath(), "/")

	for i, s := range segments {

		v, err := url.PathUnescape(s)

		if err != nil {
			v = s
		}

		segments[i] = strings.ReplaceAll(v, "/", slash_placeholder)
	}

	return strings.Join(segments, "/")
}

func stripPort(host string) string {

	if h, _, err := net.SplitHostPort(host); err == nil {
		return h
	}

	return host
}
