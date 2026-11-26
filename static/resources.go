package static

import (
	"net/http"

	"github.com/aaronland/go-http/v4/rewrite"
)

// ResourcesOptions provides a list of JavaScript and CSS links, and data attributes, to include with HTML output.
type ResourcesOptions struct {
	// JS is a list of JavaScript URIs to append as resources in HTML output.
	JS []string
	// CSS is a list of CSS URIs to append as resources in HTML output.
	CSS []string
	// DataAttributes is a map of key and value pairs to append to the HTML <body> element as `data-{KEY}` attributes.
	DataAttributes map[string]string
	// AppendJavaScriptAtEOF is a boolean flag to append JavaScript markup at the end of an HTML document
	// rather than in the <head> HTML element. Default is false.
	AppendJavaScriptAtEOF bool
}

// DefaultResourcesOptions returns a *ResourcesOptions struct with default paths and URIs.
func DefaultResourcesOptions() *ResourcesOptions {

	opts := &ResourcesOptions{
		CSS:            []string{},
		JS:             []string{},
		DataAttributes: make(map[string]string),
	}

	return opts
}

// AppendResourcesHandler rewrites any HTML produced by 'next' to include the JavaScript, CSS and data attributes defined in 'opts'.
func AppendResourcesHandler(next http.Handler, opts *ResourcesOptions) http.Handler {
	return AppendResourcesHandlerWithPrefix(next, opts, "")
}

// AppendResourcesHandlerWithPrefix is like AppendResourcesHandler but prepends 'prefix' to each of the JavaScript and CSS URIs in 'opts'.
func AppendResourcesHandlerWithPrefix(next http.Handler, opts *ResourcesOptions, prefix string) http.Handler {

	js := make([]string, len(opts.JS))
	css := make([]string, len(opts.CSS))

	for i, path := range opts.JS {
		js[i] = appendPrefix(prefix, path)
	}

	for i, path := range opts.CSS {
		css[i] = appendPrefix(prefix, path)
	}

	ext_opts := &rewrite.AppendResourcesOptions{
		JavaScript:            js,
		Stylesheets:           css,
		DataAttributes:        opts.DataAttributes,
		AppendJavaScriptAtEOF: opts.AppendJavaScriptAtEOF,
	}

	return rewrite.AppendResourcesHandler(next, ext_opts)
}
