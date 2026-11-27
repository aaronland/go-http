// Package rollup provides HTTP handlers that combine and minify multiple JavaScript or CSS files into a single response.
package rollup

import (
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/css"
)

// RollupCSSHandlerOptions defines configuration options for the handler returned by RollupCSSHandler.
type RollupCSSHandlerOptions struct {
	// FS is the filesystem from which CSS files are read.
	FS fs.FS
	// Paths maps the file name of a rollup file to the ordered list of CSS paths in FS that it combines.
	Paths map[string][]string
}

// RollupCSSHandler returns a handler that serves the CSS files listed in 'opts.Paths' for the requested file name,
// minified and concatenated in order. It responds with a 404 if the file name is not a key in 'opts.Paths'.
func RollupCSSHandler(opts *RollupCSSHandlerOptions) (http.Handler, error) {

	m := minify.New()
	m.AddFunc("text/css", css.Minify)

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		path := req.URL.Path
		fname := filepath.Base(path)

		js_paths, exists := opts.Paths[fname]

		if !exists {
			http.Error(rsp, "Not found", http.StatusNotFound)
			return
		}

		rsp.Header().Set("Content-type", "text/css")

		for _, path := range js_paths {

			r, err := opts.FS.Open(path)

			if err != nil {
				slog.Error("Failed to open CSS path for reading", "path", path, "error", err)
				http.Error(rsp, err.Error(), http.StatusInternalServerError)
				return
			}

			defer r.Close()

			err = m.Minify("text/css", rsp, r)

			if err != nil {
				slog.Error("Failed to minify CSS", "path", path, "error", err)
				http.Error(rsp, err.Error(), http.StatusInternalServerError)
				return
			}
		}

	}

	return http.HandlerFunc(fn), nil
}
