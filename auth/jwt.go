package auth

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"github.com/aaronland/go-http/v4/response"
	"github.com/aaronland/gocloud/runtimevar"
	"github.com/golang-jwt/jwt/v5"
)

// AUTHORIZATION_HEADER is the default name of the HTTP header that is checked for a bearer JWT.
const AUTHORIZATION_HEADER string = "Authorization"

// LEGACY_AUTHORIZATION_HEADER is the non-standard header name that was the default in earlier releases.
// It is still accepted as a fallback when the default header is in use.
const LEGACY_AUTHORIZATION_HEADER string = "Authentication"

var re_auth = regexp.MustCompile(`^Bearer\s+((?:[_\-A-Za-z0-9+/]*={0,3})\.(?:[_\-A-Za-z0-9+/]*={0,3})\.([_\-A-Za-z0-9+/]*={0,3}))$`)

func init() {
	ctx := context.Background()
	RegisterAuthenticator(ctx, "jwt", NewJWTAuthenticator)
}

// JWTAuthenticatorClaims are the custom claims for Authorization requests.
type JWTAuthenticatorClaims struct {
	// AccountId is the unique ID associated with this account.
	AccountId string `json:"account_id"`
	// AccountName is the name associated with this account.
	AccountName string `json:"account_name"`
	jwt.RegisteredClaims
}

// JWTAuthenticator implements the Authenticator interface to require a valid JSON Web Token (JWT) be passed
// with all requests.
type JWTAuthenticator struct {
	Authenticator
	secret               string
	authorization_header string
}

// NewJWTAuthenticator implements the Authenticator interface to ensure that requests contain a `Authorization: Bearer {JWT_TOKEN}` HTTP
// header configured by 'uri' which is expected to take the form of:
//
//	jwt://{SECRET}
//
// Where {SECRET} is expected to be the shared JWT signing secret passed by HTTP requests. Or:
//
//	jwt://runtimevar?runtimevar-uri={GOCLOUD_DEV_RUNTIMEVAR_URI}
//
// Where {GOCLOUD_DEV_RUNTIMEVAR_URI} is a valid `gocloud.dev/runtimevar` URI used to dereference the JWT signing secret.
// Under the hood this method uses the `github.com/aaronland/gocloud/runtimevar.StringVar` method to dereference runtimevar URIs.
//
// By default a `JWTAuthenticator` instance looks for JWT Bearer tokens in the HTTP "Authorization" header (falling back to the legacy "Authentication" header). This behaviour
// can be customized by passing an "authorization-header" query parameter in 'uri'. For example:
//
//	jwt://?authorization-header=X-Custom-AuthHeader
func NewJWTAuthenticator(ctx context.Context, uri string) (Authenticator, error) {

	u, err := url.Parse(uri)

	if err != nil {
		return nil, fmt.Errorf("Failed to parse URI, %w", err)
	}

	q := u.Query()

	secret := u.Host

	if secret == "runtimevar" {

		runtimevar_uri := q.Get("runtimevar-uri")

		s, err := runtimevar.StringVar(ctx, runtimevar_uri)

		if err != nil {
			return nil, fmt.Errorf("Failed to derive secret from runtimevar, %w", err)
		}

		secret = strings.TrimSpace(s)
	}

	if secret == "" {
		return nil, fmt.Errorf("Missing or invalid secret")
	}

	authorization_header := AUTHORIZATION_HEADER

	if q.Has("authorization-header") {
		authorization_header = q.Get("authorization-header")
	}

	a := &JWTAuthenticator{
		secret:               secret,
		authorization_header: authorization_header,
	}

	return a, nil
}

// WrapHandler returns an `http.Handler` that wraps 'next' and responds with an HTTP "403 Forbidden" error
// if `GetAccountForRequest` fails for the request.
func (a *JWTAuthenticator) WrapHandler(next http.Handler) http.Handler {

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		_, err := a.GetAccountForRequest(req)

		if err != nil {
			slog.Error("Failed to derive account", "error", err)
			http.Error(rsp, "Forbidden", http.StatusForbidden)
			return
		}

		next.ServeHTTP(rsp, req)
	}

	return http.HandlerFunc(fn)
}

// GetAccountForRequest returns a stub `Account` instance for requests that contain a valid `Authorization: Bearer {JWT_TOKEN}` HTTP header (or a custom header if defined in the `JWTAuthenticator` constructor URI).
func (a *JWTAuthenticator) GetAccountForRequest(req *http.Request) (Account, error) {

	var acct Account

	auth_header := req.Header.Get(a.authorization_header)

	// Fall back to the legacy header name when using the default header.
	if auth_header == "" && a.authorization_header == AUTHORIZATION_HEADER {
		auth_header = req.Header.Get(LEGACY_AUTHORIZATION_HEADER)
	}

	if !re_auth.MatchString(auth_header) {
		// Never log the header value since it may contain a (malformed) credential.
		slog.Error("Authorization header mismatch", "header", a.authorization_header)
		return nil, fmt.Errorf("Invalid auth header")
	}

	m := re_auth.FindStringSubmatch(auth_header)
	str_token := m[1]

	parse_func := func(token *jwt.Token) (any, error) {
		return []byte(a.secret), nil
	}

	parse_opts := []jwt.ParserOption{
		jwt.WithValidMethods([]string{
			jwt.SigningMethodHS256.Alg(),
		}),
		jwt.WithExpirationRequired(),
		jwt.WithPaddingAllowed(),
	}

	token, err := jwt.ParseWithClaims(str_token, &JWTAuthenticatorClaims{}, parse_func, parse_opts...)

	if err != nil {
		return nil, err
	} else if claims, ok := token.Claims.(*JWTAuthenticatorClaims); ok {
		acct = NewAccount(claims.AccountId, claims.AccountName)
	} else {
		return nil, fmt.Errorf("Unknown claims type, cannot proceed")
	}

	if acct.Id() == "" {
		return nil, fmt.Errorf("Missing account ID")
	}

	if acct.Name() == "" {
		return nil, fmt.Errorf("Missing account name")
	}

	return acct, nil
}

// SigninHandler returns an `http.Handler` instance that returns an HTTP "501 Not implemented" error.
func (a *JWTAuthenticator) SigninHandler() http.Handler {
	return response.NotImplementedHandler()
}

// SignoutHandler returns an `http.Handler` instance that returns an HTTP "501 Not implemented" error.
func (a *JWTAuthenticator) SignoutHandler() http.Handler {
	return response.NotImplementedHandler()
}

// SignupHandler returns an `http.Handler` instance that returns an HTTP "501 Not implemented" error.
func (a *JWTAuthenticator) SignupHandler() http.Handler {
	return response.NotImplementedHandler()
}
