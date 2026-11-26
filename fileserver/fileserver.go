// Package fileserver provides HTTP handlers and middleware for serving files from a local directory.
package fileserver

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/NYTimes/gziphandler"
	"github.com/rs/cors"
)

// FileServerOptions defines configuration options for the handler returned by NewFileServerHandler.
type FileServerOptions struct {
	// Root is the path of the directory to serve files from.
	Root string
	// EnableCORS is a boolean flag to enable CORS support.
	EnableCORS bool
	// CORSOrigins is the list of origins allowed by CORS requests.
	CORSOrigins []string
	// EnableGzip is a boolean flag to enable gzip compression of responses.
	EnableGzip bool
}

// ContentTypeOptions defines configuration options for the handler returned by NewContentTypeHandler.
type ContentTypeOptions struct {
	// Matches maps file extensions (including the leading ".") to the Content-Type header values to set.
	Matches map[string]string
}

// NewContentTypeHandler returns a middleware handler that sets the "Content-Type" header for requests whose
// URI file extension is a key in 'opts.Matches', before invoking 'next'.
func NewContentTypeHandler(opts *ContentTypeOptions, next http.Handler) (http.Handler, error) {

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		uri := req.RequestURI
		ext := filepath.Ext(uri)

		for re_ext, content_type := range opts.Matches {

			if ext == re_ext {
				rsp.Header().Set("Content-Type", content_type)
				break
			}
		}

		next.ServeHTTP(rsp, req)
	}

	return http.HandlerFunc(fn), nil
}

// NewWithHeadersHandler returns a middleware handler that sets each of the key and value pairs in 'headers'
// as response headers before invoking 'next'.
func NewWithHeadersHandler(headers map[string]string, next http.Handler) (http.Handler, error) {

	fn := func(rsp http.ResponseWriter, req *http.Request) {

		for k, v := range headers {
			rsp.Header().Set(k, v)
		}

		next.ServeHTTP(rsp, req)
	}

	return http.HandlerFunc(fn), nil
}

// NewFileServerHandler returns a handler that serves files from the directory 'opts.Root', optionally
// wrapped with gzip compression and CORS support. It returns an error if 'opts.Root' is not a directory.
func NewFileServerHandler(opts *FileServerOptions) (http.Handler, error) {

	abs_root, err := filepath.Abs(opts.Root)

	if err != nil {
		return nil, err
	}

	info, err := os.Stat(abs_root)

	if err != nil {
		return nil, err
	}

	if !info.IsDir() {
		msg := fmt.Sprintf("Root (%s) is not a directory", abs_root)
		return nil, errors.New(msg)
	}

	http_root := http.Dir(abs_root)
	fs_handler := http.FileServer(http_root)

	if opts.EnableGzip {
		fs_handler = gziphandler.GzipHandler(fs_handler)
	}

	if opts.EnableCORS {

		c := cors.New(cors.Options{
			AllowedOrigins: opts.CORSOrigins,
		})

		fs_handler = c.Handler(fs_handler)
	}

	return fs_handler, nil
}
