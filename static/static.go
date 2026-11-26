// Package static provides handlers for serving static assets from an `fs.FS` instance and for
// appending references to those assets to HTML output.
package static

import (
	"fmt"
	"io/fs"
	"net/http"
	"strings"

	"github.com/aaronland/go-http/v4/rewrite"
)

// StaticAssetsHandler returns an `http.Handler` that serves the files in 'static_fs'.
func StaticAssetsHandler(static_fs fs.FS) (http.Handler, error) {

	http_fs := http.FS(static_fs)
	return http.FileServer(http_fs), nil
}

// StaticAssetsHandlerWithPrefix returns an `http.Handler` that serves the files in 'static_fs' after removing 'prefix' from the request path.
func StaticAssetsHandlerWithPrefix(static_fs fs.FS, prefix string) (http.Handler, error) {

	fs_handler, err := StaticAssetsHandler(static_fs)

	if err != nil {
		return nil, err
	}

	prefix = strings.TrimRight(prefix, "/")

	if prefix == "" {
		return fs_handler, nil
	}

	rewrite_func := func(req *http.Request) (*http.Request, error) {
		req.URL.Path = strings.Replace(req.URL.Path, prefix, "", 1)
		return req, nil
	}

	rewrite_handler := rewrite.RewriteRequestHandler(fs_handler, rewrite_func)
	return rewrite_handler, nil
}

// AppendStaticAssetHandlers registers a handler on 'mux' for each file in 'static_fs'.
func AppendStaticAssetHandlers(mux *http.ServeMux, static_fs fs.FS) error {
	return AppendStaticAssetHandlersWithPrefix(mux, static_fs, "")
}

// AppendStaticAssetHandlersWithPrefix registers a handler on 'mux' for each file in 'static_fs', with 'prefix' prepended to the path of each file.
func AppendStaticAssetHandlersWithPrefix(mux *http.ServeMux, static_fs fs.FS, prefix string) error {

	asset_handler, err := StaticAssetsHandlerWithPrefix(static_fs, prefix)

	if err != nil {
		return nil
	}

	walk_func := func(path string, info fs.DirEntry, err error) error {

		// log.Println("WALK", path)

		if path == "." {
			return nil
		}

		if info.IsDir() {
			return nil
		}

		if prefix != "" {
			path = appendPrefix(prefix, path)
		}

		if !strings.HasPrefix(path, "/") {
			path = fmt.Sprintf("/%s", path)
		}

		// log.Printf("APPEND '%s'\n", path)

		mux.Handle(path, asset_handler)
		return nil
	}

	return fs.WalkDir(static_fs, ".", walk_func)
}
