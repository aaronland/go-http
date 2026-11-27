package auth

import (
	"context"
	"crypto/subtle"
	"fmt"
	"net/http"
	"net/url"

	"github.com/aaronland/go-http/v4/response"
)

// SHARED_SECRET_HEADER is the name of the HTTP header to check for "shared secret" authentication.
const SHARED_SECRET_HEADER string = "X-Shared-Secret"

// SHARED_SECRET_ACCOUNT_ID is the account ID used for `Account` instances when shared secret authentication validates.
const SHARED_SECRET_ACCOUNT_ID string = ""

// SHARED_SECRET_ACCOUNT_NAME is the account name used for `Account` instances when shared secret authentication validates.
const SHARED_SECRET_ACCOUNT_NAME string = "sharedsecret"

func init() {
	ctx := context.Background()
	RegisterAuthenticator(ctx, "sharedsecret", NewSharedSecretAuthenticator)
}

// SharedSecretAuthenticator implements the Authenticator interface to require a simple shared secret be passed
// with all requests. This is not a sophisticated handler. There are no nonces or hashing of requests or anything like
// that. It is a bare-bones supplementary authentication handler for environments that already implement their own
// measures of access control.
type SharedSecretAuthenticator struct {
	Authenticator
	secret string
}

// NewSharedSecretAuthenticator implements the Authenticator interface to ensure that requests contain a `X-Shared-Secret` HTTP
// header configured by 'uri' which is expected to take the form of:
//
//	sharedsecret://{SECRET}
//
// Where {SECRET} is expected to be the shared secret passed by HTTP requests.
func NewSharedSecretAuthenticator(ctx context.Context, uri string) (Authenticator, error) {

	u, err := url.Parse(uri)

	if err != nil {
		return nil, fmt.Errorf("Failed to parse URI, %w", err)
	}

	secret := u.Host

	if secret == "" {
		return nil, fmt.Errorf("Missing or invalid secret")
	}

	a := &SharedSecretAuthenticator{
		secret: secret,
	}

	return a, nil
}

// WrapHandler returns an `http.Handler` that wraps 'next' and responds with an HTTP "403 Forbidden" error
// if `GetAccountForRequest` fails for the request.
func (a *SharedSecretAuthenticator) WrapHandler(next http.Handler) http.Handler {

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		_, err := a.GetAccountForRequest(req)

		if err != nil {
			http.Error(rsp, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(rsp, req)
	}

	return http.HandlerFunc(fn)
}

// GetAccountForRequest returns a stub `Account` instance for requests that contain a valid `X-Shared-Secret` HTTP header.
func (a *SharedSecretAuthenticator) GetAccountForRequest(req *http.Request) (Account, error) {

	secret := req.Header.Get(SHARED_SECRET_HEADER)

	if secret == "" || subtle.ConstantTimeCompare([]byte(secret), []byte(a.secret)) != 1 {
		return nil, NotAuthorized{}
	}

	acct := NewAccount(SHARED_SECRET_ACCOUNT_ID, SHARED_SECRET_ACCOUNT_NAME)
	return acct, nil
}

// SigninHandler returns an `http.Handler` instance that returns an HTTP "501 Not implemented" error.
func (a *SharedSecretAuthenticator) SigninHandler() http.Handler {
	return response.NotImplementedHandler()
}

// SignoutHandler returns an `http.Handler` instance that returns an HTTP "501 Not implemented" error.
func (a *SharedSecretAuthenticator) SignoutHandler() http.Handler {
	return response.NotImplementedHandler()
}

// SignupHandler returns an `http.Handler` instance that returns an HTTP "501 Not implemented" error.
func (a *SharedSecretAuthenticator) SignupHandler() http.Handler {
	return response.NotImplementedHandler()
}
