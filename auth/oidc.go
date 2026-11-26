package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type OIDCAccount struct {
	Account
	Subject      string `json:"sub"`
	EmailAddress string `json:"email"`
	ProfileName  string `json:"name"`
}

func (a *OIDCAccount) Id() string {
	return a.Subject
}

func (a *OIDCAccount) Name() string {
	return a.ProfileName
}

// OIDCAuthenticator implements auth.Authenticator interface
type OIDCAuthenticator struct {
	providerURL  string
	clientID     string
	redirectURI  string
	cookieSecret string
}

func init() {
	MustRegisterAuthenticator(context.Background(), "oidc", NewOIDCAuthenticator)
}

// NewOIDCAuthenticator initializes parameters from parsed configuration strings
// Example URI: oidc://localhost:9998?client_id=my-client&redirect_uri=http://localhost:3000/callback
func NewOIDCAuthenticator(ctx context.Context, uri string) (Authenticator, error) {

	u, err := url.Parse(uri)

	if err != nil {
		return nil, fmt.Errorf("failed to parse OIDC URI: %w", err)
	}

	q := u.Query()

	clientID := q.Get("client_id")
	redirectURI := q.Get("redirect_uri")

	if clientID == "" || redirectURI == "" {
		return nil, fmt.Errorf("missing client_id or redirect_uri query parameters")
	}

	// Reconstruct the base HTTP/HTTPS path for your target provider

	providerHost := u.Host
	scheme := "https"

	if strings.HasSuffix(u.Path, "insecure") || providerHost == "localhost" || strings.HasPrefix(providerHost, "localhost:") {
		scheme = "http"
	}

	return &OIDCAuthenticator{
		providerURL:  fmt.Sprintf("%s://%s", scheme, providerHost),
		clientID:     clientID,
		redirectURI:  redirectURI,
		cookieSecret: "secure-session-encryption-key", // use go-http/cookie
	}, nil
}

// 1. Enforces user session validation rules around application entry points
func (o *OIDCAuthenticator) WrapHandler(next http.Handler) http.Handler {

	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {

		_, err := o.GetAccountForRequest(req)

		if err != nil {
			// If not authenticated, bounce directly to the providers authorization gateway
			o.SigninHandler().ServeHTTP(rsp, req)
			return
		}

		next.ServeHTTP(rsp, req)
	})
}

// 2. Looks up active cookie keys to determine current user identity profiles
func (o *OIDCAuthenticator) GetAccountForRequest(req *http.Request) (Account, error) {

	cookie, err := req.Cookie("oidc_session")

	if err != nil {
		return nil, fmt.Errorf("no active session found: %w", err)
	}

	// Decrypt/Decode profile fields stored in local transaction cookie storage
	rawVal, err := base64.URLEncoding.DecodeString(cookie.Value)

	if err != nil {
		return nil, err
	}

	var acc OIDCAccount

	if err := json.Unmarshal(rawVal, &acc); err != nil {
		return nil, err
	}

	return &acc, nil
}

// 3. Implements Authorization Code Flow with PKCE challenges
func (o *OIDCAuthenticator) SigninHandler() http.Handler {

	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {

		state := generateRandomToken()
		verifier := generateRandomToken()

		// Create secure PKCE Code Challenge (S256 hashing format)
		hash := sha256.Sum256([]byte(verifier))
		challenge := base64.URLEncoding.EncodeToString(hash[:])
		challenge = strings.NewReplacer("+", "-", "/", "_", "=", "").Replace(challenge)

		// Set transient cookies to capture parameters across window redirections
		setSecureCookie(rsp, "oidc_state", state, 15*time.Minute)
		setSecureCookie(rsp, "oidc_verifier", verifier, 15*time.Minute)

		providerAuthURL := fmt.Sprintf("%s/oauth/v2/authorize?response_type=code&client_id=%s&redirect_uri=%s&scope=openid+profile+email&state=%s&code_challenge=%s&code_challenge_method=S256",
			o.providerURL,
			url.QueryEscape(o.clientID),
			url.QueryEscape(o.redirectURI),
			state,
			challenge,
		)

		http.Redirect(rsp, req, providerAuthURL, http.StatusFound)
	})
}

// 4. Redirects to Provider endpoint or signs out locally by wiping active cookie keys
func (o *OIDCAuthenticator) SignoutHandler() http.Handler {

	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {

		// Clear local storage tracks
		setSecureCookie(rsp, "oidc_session", "", -1*time.Hour)

		// If the server supports standard OpenID end_session endpoints, redirect there

		logoutURL := fmt.Sprintf("%s/oauth/v2/logout?client_id=%s&post_logout_redirect_uri=%s",
			o.providerURL, url.QueryEscape(o.clientID), url.QueryEscape(o.redirectURI))

		http.Redirect(rsp, req, logoutURL, http.StatusFound)
	})
}

// 5. Explicitly maps account creation options back to alternative provider entry paths
func (o *OIDCAuthenticator) SignupHandler() http.Handler {

	return http.HandlerFunc(func(rsp http.ResponseWriter, req *http.Request) {

		signupURL := fmt.Sprintf("%s/signup?client_id=%s&redirect_uri=%s",
			o.providerURL, url.QueryEscape(o.clientID), url.QueryEscape(o.redirectURI))

		http.Redirect(rsp, req, signupURL, http.StatusFound)
	})
}

// --- Cryptographic & Cookie Helpers ---
func generateRandomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}

func setSecureCookie(rsp http.ResponseWriter, name string, value string, duration time.Duration) {

	ck := &http.Cookie{
		Name:     name,
		Value:    value,
		Path:     "/",
		Expires:  time.Now().Add(duration),
		HttpOnly: true,
		Secure:   false, // Set to true in production with valid HTTPS TLS configurations
		SameSite: http.SameSiteLaxMode,
	}

	http.SetCookie(rsp, ck)
}
