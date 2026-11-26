package rollup

import (
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"path/filepath"
	"regexp"

	"github.com/tdewolff/minify/v2"
	"github.com/tdewolff/minify/v2/js"
)

// RollupJSHandlerOptions defines configuration options for the handler returned by RollupJSHandler.
type RollupJSHandlerOptions struct {
	// FS is the filesystem from which JavaScript files are read.
	FS fs.FS
	// Paths maps the file name of a rollup file to the ordered list of JavaScript paths in FS that it combines.
	Paths map[string][]string
}

// RollupJSHandler returns a handler that serves the JavaScript files listed in 'opts.Paths' for the requested file name,
// minified and concatenated in order. It responds with a 404 if the file name is not a key in 'opts.Paths'.
func RollupJSHandler(opts *RollupJSHandlerOptions) (http.Handler, error) {

	js_regexp, err := regexp.Compile("^(application|text)/(x-)?(java|ecma)script$")

	if err != nil {
		return nil, fmt.Errorf("Failed to compile JS pattern, %w", err)
	}

	m := minify.New()
	m.AddFuncRegexp(js_regexp, js.Minify)

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		path := req.URL.Path
		fname := filepath.Base(path)

		js_paths, exists := opts.Paths[fname]

		if !exists {
			http.Error(rsp, "Not found", http.StatusNotFound)
			return
		}

		rsp.Header().Set("Content-type", "text/javascript")

		for _, path := range js_paths {

			r, err := opts.FS.Open(path)

			if err != nil {
				slog.Error("Failed to open JavaScript file for reading", "path", path, "error", err)
				http.Error(rsp, err.Error(), http.StatusInternalServerError)
				return
			}

			defer r.Close()

			err = m.Minify("text/javascript", rsp, r)

			if err != nil {
				slog.Error("Failed to minify JavaScript file", "path", path, "error", err)
				http.Error(rsp, err.Error(), http.StatusInternalServerError)
				return
			}

			rsp.Write([]byte(`;`))
		}

		return
	}

	return http.HandlerFunc(fn), nil
}
