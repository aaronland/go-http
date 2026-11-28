package rollup

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/tdewolff/minify/v2"
)

// rollupHandler serves minified and concatenated files, keyed by the base name of the requested URL path.
type rollupHandler struct {
	fs          fs.FS
	paths       map[string][]string
	mediatype   string
	contentType string
	separator   []byte
	minifier    *minify.M
	cache       bool

	mu      sync.Mutex
	entries map[string]*rollupEntry
}

type rollupEntry struct {
	body []byte
	etag string
}

func newRollupHandler(kind string, fsys fs.FS, paths map[string][]string, mediatype string, minifier *minify.M, separator string, cache bool) (*rollupHandler, error) {

	if fsys == nil {
		return nil, fmt.Errorf("Missing %s filesystem", kind)
	}

	for name, files := range paths {

		if len(files) == 0 {
			return nil, fmt.Errorf("No %s paths defined for '%s'", kind, name)
		}

		for _, p := range files {

			_, err := fs.Stat(fsys, p)

			if err != nil {
				return nil, fmt.Errorf("Failed to stat %s path '%s' for '%s', %w", kind, p, name, err)
			}
		}
	}

	h := &rollupHandler{
		fs:          fsys,
		paths:       paths,
		mediatype:   mediatype,
		contentType: mediatype + "; charset=utf-8",
		separator:   []byte(separator),
		minifier:    minifier,
		cache:       cache,
		entries:     make(map[string]*rollupEntry),
	}

	return h, nil
}

func (h *rollupHandler) ServeHTTP(rsp http.ResponseWriter, req *http.Request) {

	if req.Method != http.MethodGet && req.Method != http.MethodHead {
		rsp.Header().Set("Allow", "GET, HEAD")
		http.Error(rsp, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	fname := path.Base(req.URL.Path)

	files, exists := h.paths[fname]

	if !exists {
		http.Error(rsp, "Not found", http.StatusNotFound)
		return
	}

	entry, err := h.entry(fname, files)

	if err != nil {
		slog.Error("Failed to build rollup", "name", fname, "error", err)
		http.Error(rsp, "Internal server error", http.StatusInternalServerError)
		return
	}

	hdr := rsp.Header()
	hdr.Set("Content-Type", h.contentType)
	hdr.Set("X-Content-Type-Options", "nosniff")

	if h.cache {

		hdr.Set("ETag", entry.etag)

		match := req.Header.Get("If-None-Match")

		if match != "" && strings.Contains(match, entry.etag) {
			rsp.WriteHeader(http.StatusNotModified)
			return
		}
	}

	if req.Method == http.MethodHead {
		return
	}

	_, err = rsp.Write(entry.body)

	if err != nil {
		slog.Debug("Failed to write rollup", "name", fname, "error", err)
	}
}

func (h *rollupHandler) entry(fname string, files []string) (*rollupEntry, error) {

	if !h.cache {
		return h.build(files)
	}

	h.mu.Lock()
	defer h.mu.Unlock()

	e, ok := h.entries[fname]

	if ok {
		return e, nil
	}

	e, err := h.build(files)

	if err != nil {
		return nil, err
	}

	h.entries[fname] = e
	return e, nil
}

// build minifies each file in turn into a single buffer so that nothing is written to the client unless every file succeeds.
func (h *rollupHandler) build(files []string) (*rollupEntry, error) {

	var buf bytes.Buffer

	for _, p := range files {

		err := h.minifyFile(&buf, p)

		if err != nil {
			return nil, err
		}

		buf.Write(h.separator)
	}

	sum := sha256.Sum256(buf.Bytes())

	e := &rollupEntry{
		body: buf.Bytes(),
		etag: `"` + hex.EncodeToString(sum[:16]) + `"`,
	}

	return e, nil
}

func (h *rollupHandler) minifyFile(w io.Writer, p string) error {

	r, err := h.fs.Open(p)

	if err != nil {
		return fmt.Errorf("Failed to open '%s', %w", p, err)
	}

	defer r.Close()

	err = h.minifier.Minify(h.mediatype, w, r)

	if err != nil {
		return fmt.Errorf("Failed to minify '%s', %w", p, err)
	}

	return nil
}
