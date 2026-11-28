// Package rollup provides HTTP handlers that combine and minify multiple JavaScript or CSS files into a single response.
package rollup

import (
	"fmt"
	"io/fs"
	"net/http"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
)

// RollupCSSHandlerOptions defines configuration options for the handler returned by RollupCSSHandler.
type RollupCSSHandlerOptions struct {
	// FS is the filesystem from which CSS files are read.
	FS fs.FS
	// Paths maps the file name of a rollup file to the ordered list of CSS paths in FS that it combines.
	Paths map[string][]string
	// Cache indicates that each rollup file should be built once and then served from memory with an ETag.
	// Changes to the underlying files are not picked up. The default is to rebuild the file on every request.
	Cache bool
}

// RollupCSSHandler returns a handler that serves the CSS files listed in 'opts.Paths' for the requested file name,
// minified and concatenated in order. It responds with a 404 if the file name is not a key in 'opts.Paths' and with
// a 405 for any method other than GET or HEAD. It returns an error if 'opts' or 'opts.FS' is nil or if any path
// in 'opts.Paths' does not exist in 'opts.FS'.
func RollupCSSHandler(opts *RollupCSSHandlerOptions) (http.Handler, error) {

	if opts == nil {
		return nil, fmt.Errorf("Missing options")
	}

	m := minify.New()
	m.AddFunc("text/css", css.Minify)

	h, err := newRollupHandler("CSS", opts.FS, opts.Paths, "text/css", m, "", opts.Cache)

	if err != nil {
		return nil, err
	}

	return h, nil
}
