# go-http

Opinionated package of wrapper, helper and utility functions for working with HTTP-related services.

## Documentation

[![Go Reference](https://pkg.go.dev/badge/github.com/aaronland/go-http.svg)](https://pkg.go.dev/github.com/aaronland/go-http/v4)

Documentation remains incomplete despite this being a "v4" release. Documentation, such as it is, can be found in the individual sub-packages.

## Packages

Top-level packages. Each has its own README with more detail.

| Package | Description |
| --- | --- |
| [auth](auth) | A common interface for enforcing authentication in HTTP handlers, with JWT, OpenID Connect, shared secret, `null` (always unauthenticated) and `none` (always allowed) implementations. |
| [bootstrap](bootstrap) | Middleware and handlers for serving embedded [Bootstrap](https://getbootstrap.com) assets and adding references to them to HTML output. |
| [cookie](cookie) | A common interface for reading and writing HTTP cookies, including encrypted cookies. |
| [crumb](crumb) | Generating and validating HTTP crumbs (CSRF tokens), with middleware for assigning and checking them. |
| [fault](fault) | Handlers and middleware for reporting errors in web applications, optionally rendered with a custom HTML template. |
| [fileserver](fileserver) | Handlers and middleware for serving files from a local directory, and a `fileserver` command line tool. |
| [maps](maps) | Configuration types and handlers for web maps using Leaflet and Protomaps. |
| [opensearch](opensearch) | Types and handlers for publishing OpenSearch description documents and advertising them in HTML output. |
| [response](response) | Simple handlers for common responses: null, ping, not implemented and disabled. |
| [rewrite](rewrite) | Middleware for rewriting HTTP requests and the HTML content of HTTP responses. |
| [rollup](rollup) | Handlers that bundle and minify multiple JavaScript or CSS files in to a single response. |
| [route](route) | An `http.Handler` that routes requests using `http.ServeMux` style patterns and initializes the routed handlers lazily. |
| [sanitize](sanitize) | Helpers for extracting, sanitising and type-converting query, form and header parameters from HTTP requests. |
| [server](server) | A common interface, and registry of implementations, for HTTP servers created from a URI (HTTP(S), `mkcert` and AWS Lambda). |
| [slog](slog) | Helpers for deriving `log/slog` loggers decorated with details of HTTP requests. |
| [static](static) | Handlers for serving static assets from an `fs.FS` and for appending references to those assets to HTML output. |
| [wellknown](wellknown) | Packages for working with well-known URIs. Currently [webfinger](wellknown/webfinger), which defines WebFinger response types. |

The [cmd](cmd) directory contains example programs and command line tools. Build the command line tools with `make cli`.

## Motivation

There are many wrapper libraries for working with HTTP-related services. This one is mine. It is NOT comprehensive and instead reflects those services which occupy my time or have done so in the past. This is also NOT a "framework". It is a collection of packages and methods designed with "composability" in mind.
