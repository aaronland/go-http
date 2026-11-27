// Package auth provides a common interface for enforcing authentication in HTTP handlers, along with
// several implementations (JWT, OpenID Connect, shared secret, null and none).
package auth

import (
	"context"
	"fmt"
	"net/http"

	"github.com/aaronland/go-roster/v2"
)

// Authenticator is a simple interface for enforcing authentication in HTTP handlers.
type Authenticator interface {
	// WrapHandler wraps a `http.Handler` with any implementation-specific middleware.
	WrapHandler(http.Handler) http.Handler
	// GetAccountForRequest returns an `Account` instance for an HTTP request.
	GetAccountForRequest(*http.Request) (Account, error)
	// SigninHandler returns a `http.Handler` for implementing account signin.
	SigninHandler() http.Handler
	// SignoutHandler returns a `http.Handler` for implementing account signout.
	SignoutHandler() http.Handler
	// SignupHandler returns a `http.Handler` for implementing account signups.
	SignupHandler() http.Handler
}

// AuthenticatorInitializationFunc is a function defined by individual authenticator packages and used to create
// an instance of that authenticator.
type AuthenticatorInitializationFunc = roster.Factory[Authenticator]

var authenticator_roster = roster.NewRegistry[Authenticator]()

// MustRegisterAuthenticator calls `RegisterAuthenticator` and panics if registration fails.
func MustRegisterAuthenticator(ctx context.Context, scheme string, init_func AuthenticatorInitializationFunc) {

	err := RegisterAuthenticator(ctx, scheme, init_func)

	if err != nil {
		panic(fmt.Sprintf("Failed to register %s scheme, %v", scheme, err))
	}
}

// RegisterAuthenticator registers 'scheme' as a key pointing to 'init_func' in an internal lookup table
// used to create new `Authenticator` instances by the `NewAuthenticator` method.
func RegisterAuthenticator(ctx context.Context, scheme string, init_func AuthenticatorInitializationFunc) error {
	return authenticator_roster.Register(ctx, scheme, init_func)
}

// NewAuthenticator returns a new `Authenticator` instance configured by 'uri'. The value of 'uri' is parsed
// as a `url.URL` and its scheme is used as the key for a corresponding `AuthenticatorInitializationFunc`
// function used to instantiate the new `Authenticator`. It is assumed that the scheme (and initialization
// function) have been registered by the `RegisterAuthenticator` method.
func NewAuthenticator(ctx context.Context, uri string) (Authenticator, error) {
	return authenticator_roster.New(ctx, uri)
}

// Schemes returns the list of schemes that have been registered.
func Schemes() []string {
	return authenticator_roster.Schemes(context.Background())
}
