package cookie

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/senseyeio/duration"
)

// Config defines the properties used to create new `http.Cookie` instances.
//
// See also: https://github.com/golang/go/issues/25194
type Config struct {
	uri string
	// Name is the name of the cookie.
	Name string
	// Domain is the cookie's domain.
	Domain string
	// Path is the cookie's path.
	Path string
	// Secure indicates whether the cookie has the "Secure" attribute set.
	Secure bool
	// SameSite is the cookie's "SameSite" mode.
	SameSite http.SameSite
	// TTL is the optional lifetime of the cookie, used to derive its expiry time.
	TTL *duration.Duration
}

// NewConfig returns a new `Config` derived from 'uri', which is expected to take the form of:
//
//	https://example.com/?name=test&ttl=PT1H&samesite=strict
//
// The "name" parameter is required. The optional "ttl" parameter is an ISO 8601 duration and "samesite" may be one of
// "lax", "strict", "none" or "default". The "Secure" flag is set if the scheme is "https" and the host and path (if present)
// are used as the cookie's domain and path.
func NewConfig(ctx context.Context, uri string) (*Config, error) {

	u, err := url.Parse(uri)

	if err != nil {
		return nil, err
	}

	q := u.Query()

	name := q.Get("name")

	if name == "" {
		return nil, errors.New("Missing name parameter")
	}

	cfg := &Config{
		uri:      uri,
		Name:     name,
		SameSite: http.SameSiteDefaultMode,
	}

	str_ttl := q.Get("ttl")

	if str_ttl != "" {

		ttl, err := duration.ParseISO8601(str_ttl)

		if err != nil {
			return nil, err
		}

		cfg.TTL = &ttl
	}

	samesite := q.Get("samesite")

	if samesite != "" {

		switch strings.ToLower(samesite) {
		case "lax":
			cfg.SameSite = http.SameSiteLaxMode
		case "strict":
			cfg.SameSite = http.SameSiteStrictMode
		case "none":
			cfg.SameSite = http.SameSiteNoneMode
		case "default":
			cfg.SameSite = http.SameSiteDefaultMode
		default:
			return nil, errors.New("Invalid samesite parameter")
		}
	}

	if u.Scheme == "https" {
		cfg.Secure = true
	}

	if u.Host != "" {
		cfg.Domain = u.Host
	}

	if u.Path != "" {
		cfg.Path = u.Path
	}

	return cfg, nil
}

// NewCookie returns a new `http.Cookie` with 'value' and the properties defined in 'cfg'.
func (cfg *Config) NewCookie(ctx context.Context, value string) (*http.Cookie, error) {

	ck := &http.Cookie{
		Name:     cfg.Name,
		Value:    value,
		Secure:   cfg.Secure,
		SameSite: cfg.SameSite,
	}

	if cfg.TTL != nil {
		now := time.Now()
		expires := cfg.TTL.Shift(now)
		ck.Expires = expires
	}

	if cfg.Domain != "" {
		ck.Domain = cfg.Domain
	}

	if cfg.Path != "" {
		ck.Path = cfg.Path
	}

	if ck.String() == "" {
		return nil, errors.New("Invalid cookie")
	}

	return ck, nil
}
