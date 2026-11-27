// Package server provides a common interface, and a registry of implementations, for HTTP servers
// that can be instantiated from a URI. Implementations are included for plain HTTP(S) servers,
// `mkcert` generated TLS certificates, and AWS Lambda (API Gateway and Function URL) contexts.
package server

import (
	"context"
	_ "log"
	"net/http"

	"github.com/aaronland/go-roster/v2"
)

// Server is an interface for creating server instances that serve requests using a `http.Handler` router.
type Server interface {
	// ListenAndServe starts the server and listens for requests using a `http.Handler` instance for routing.
	ListenAndServe(context.Context, http.Handler) error
	// Address returns the fully-qualified URI that the server is listening for requests on.
	Address() string
}

// ServerInitializeFunc is a function used to initialize an implementation of the `Server` interface.
type ServerInitializeFunc = roster.Factory[Server]

var servers = roster.NewRegistry[Server]()

// RegisterServer associates 'scheme' with 'f' in an internal list of available `Server` implementations.
func RegisterServer(ctx context.Context, scheme string, f ServerInitializeFunc) error {
	return servers.Register(ctx, scheme, f)
}

// NewServer returns a new instance of `Server` for the scheme associated with 'uri'. It is assumed that this scheme
// will have previously been "registered" with the `RegisterServer` function.
func NewServer(ctx context.Context, uri string) (Server, error) {
	return servers.New(ctx, uri)
}

// Schemes returns the sorted list of schemes (in the form "scheme://") that have been "registered".
func Schemes() []string {
	return servers.Schemes(context.Background())
}
