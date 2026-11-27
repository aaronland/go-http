// Package crumb provides interfaces and implementations for generating and validating HTTP crumbs
// (CSRF tokens), along with middleware for assigning and checking them in HTTP handlers.
package crumb

import (
	"context"
	"net/http"
	"strings"

	"github.com/aaronland/go-roster/v2"
)

// Crumb is an interface for generating and validating HTTP crumb strings.
type Crumb interface {
	// Generate returns a new crumb string for an HTTP request.
	Generate(*http.Request, ...string) (string, error)
	// Validate validates a crumb string for an HTTP request.
	Validate(*http.Request, string, ...string) (bool, error)
	// Key returns the unique key used to generate a crumb.
	Key(*http.Request) string
	// Base returns the leading string used to generate a crumb.
	Base(*http.Request, ...string) (string, error)
}

// CrumbInitializeFunc is a function used to initialize packages that implement the `Crumb` interface.
type CrumbInitializeFunc = roster.Factory[Crumb]

var crumbs = roster.NewRegistry[Crumb]()

// RegisterCrumb registers 'scheme' with 'f' for URIs passed to the `NewCrumb` method.
func RegisterCrumb(ctx context.Context, scheme string, f CrumbInitializeFunc) error {
	return crumbs.Register(ctx, scheme, f)
}

// NewCrumb returns a new `Crumb` instance for 'uri'. It is assumed that the scheme of 'uri' has been registered
// using the `RegisterCrumb` method.
func NewCrumb(ctx context.Context, uri string) (Crumb, error) {
	return crumbs.New(ctx, uri)
}

// Schemes returns the list of schemes that have registered for use with the `NewCrumb` method.
func Schemes() []string {
	return crumbs.Schemes(context.Background())
}

// SchemesAsString returns the list of schemes that have registered for use with the `NewCrumb`
// method as a string.
func SchemesAsString() string {
	schemes := Schemes()
	return strings.Join(schemes, ",")
}
