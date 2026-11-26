package auth

import (
	"context"
	"net/http"

	"github.com/aaronland/go-http/v4/response"
)

func init() {
	ctx := context.Background()
	RegisterAuthenticator(ctx, "none", NewNoneAuthenticator)
}

// NoneAuthenticator implements the Authenticator interface that always returns a "not authorized" error.
type NoneAuthenticator struct {
	Authenticator
}

// NewNoneAuthenticator returns a new `NoneAuthenticator` that always returns a "not authorized" error.
// It is configured by 'uri' which is expected to take the form of:
//
//	none://
func NewNoneAuthenticator(ctx context.Context, uri string) (Authenticator, error) {
	a := &NoneAuthenticator{}
	return a, nil
}

// WrapHandler returns 'h' unchanged.
func (a *NoneAuthenticator) WrapHandler(h http.Handler) http.Handler {
	return h
}

// GetAccountForRequest always returns a `NotAuthorized` error.
func (a *NoneAuthenticator) GetAccountForRequest(req *http.Request) (Account, error) {
	return nil, NotAuthorized{}
}

// SigninHandler returns an `http.Handler` instance that returns an HTTP "501 Not implemented" error.
func (a *NoneAuthenticator) SigninHandler() http.Handler {
	return response.NotImplementedHandler()
}

// SignoutHandler returns an `http.Handler` instance that returns an HTTP "501 Not implemented" error.
func (a *NoneAuthenticator) SignoutHandler() http.Handler {
	return response.NotImplementedHandler()
}

// SignupHandler returns an `http.Handler` instance that returns an HTTP "501 Not implemented" error.
func (a *NoneAuthenticator) SignupHandler() http.Handler {
	return response.NotImplementedHandler()
}
