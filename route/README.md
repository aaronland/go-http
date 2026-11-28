# go-http/v4/route

Go package providing an `http.Handler` that routes requests using `http.ServeMux` style patterns but defers initializing the handlers being routed to until they are first needed.

## Motivation

The reason this package exists is for web applications that:

1. Are deployed as AWS Lambda functions (with an API Gateway integration) using the `lambda://` implementation of the [server](../server) package and have more handlers than you need or want to initialize, but never use, for every request.
2. You don't want to refactor in to (n) atomic Lambda functions. That is, you want to be able to re-use the same code in both a plain-vanilla HTTP server configuration as well as a Lambda + API Gateway configuration.

## Example

_Error handling omitted for brevity._

```
package main

import (
	"context"
	"net/http"

	"github.com/aaronland/go-http/v4/route"
)

func main() {

	handlers := map[string]route.RouteHandlerFunc{
		"/ping":            pingHandlerFunc,
		"POST /things":     createThingHandlerFunc, // defined like pingHandlerFunc
		"GET /things/{id}": getThingHandlerFunc,
	}

	h, _ := route.RouteHandler(handlers)
	http.ListenAndServe("localhost:8080", h)
}

func pingHandlerFunc(ctx context.Context) (http.Handler, error) {
	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {
		rsp.Write([]byte("PONG"))
	}), nil
}
```

Each function is called at most once, the first time a request matches its pattern. If it returns an error (or a nil handler) the request receives an HTTP 500 response and the function will be called again on the next matching request.

## Patterns

Patterns take the form of `[METHOD ][HOST]/[PATH]`:

* A `GET` pattern also matches `HEAD` requests.
* `{name}` matches a single path segment and `{name...}` matches the remainder of the path. Values are available using `req.PathValue("name")`.
* `{$}` matches the end of the path.
* `{name...}` and `{$}` may only be the final segment of a pattern.
* The most specific pattern wins: patterns with a host beat those without, literal segments beat wildcards and longer patterns beat shorter ones.
* If a path matches only patterns restricted to other methods the response is an HTTP 405 with an `Allow` header. Otherwise unmatched requests receive an HTTP 404.
* Patterns are parsed and validated when the handler is created. Invalid patterns, unknown methods and nil handler functions are returned as errors.

### End anchoring

By default a pattern matches any path that _begins_ with it, so `/admin` also matches `/admin/users` and `/administrator`, and `/id/{id}` matches `/id/1/anything`. This is how earlier versions of this package behaved and is retained for backwards compatibility.

To match like `http.ServeMux` does (a pattern that does not end in a slash matches that exact path only, a pattern that ends in a slash matches everything beneath it) use `RouteHandlerWithOptions`:

```
h, err := route.RouteHandlerWithOptions(&route.RouteHandlerOptions{
	Handlers:    handlers,
	EndAnchored: true,
})
```

## Notes

This is a regular expression based implementation and not a clone of the one in `net/http`, so unusual patterns may behave differently.
