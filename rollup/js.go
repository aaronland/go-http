package rollup

import (
	"fmt"
	"io/fs"
	"net/http"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/js"
)

// RollupJSHandlerOptions defines configuration options for the handler returned by RollupJSHandler.
type RollupJSHandlerOptions struct {
	// FS is the filesystem from which JavaScript files are read.
	FS fs.FS
	// Paths maps the file name of a rollup file to the ordered list of JavaScript paths in FS that it combines.
	Paths map[string][]string
	// Cache indicates that each rollup file should be built once and then served from memory with an ETag.
	// Changes to the underlying files are not picked up. The default is to rebuild the file on every request.
	Cache bool
}

// RollupJSHandler returns a handler that serves the JavaScript files listed in 'opts.Paths' for the requested file name,
// minified and concatenated in order. It responds with a 404 if the file name is not a key in 'opts.Paths' and with
// a 405 for any method other than GET or HEAD. It returns an error if 'opts' or 'opts.FS' is nil or if any path
// in 'opts.Paths' does not exist in 'opts.FS'.
func RollupJSHandler(opts *RollupJSHandlerOptions) (http.Handler, error) {

	if opts == nil {
		return nil, fmt.Errorf("Missing options")
	}

	m := minify.New()
	m.AddFunc("text/javascript", js.Minify)

	h, err := newRollupHandler("JavaScript", opts.FS, opts.Paths, "text/javascript", m, ";", opts.Cache)

	if err != nil {
		return nil, err
	}

	return h, nil
}
