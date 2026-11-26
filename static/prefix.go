package static

import (
	"path/filepath"
	"strings"
)

// appendPrefix returns 'path' joined to 'prefix' (ignoring any trailing slashes on 'prefix'). If 'prefix' is empty 'path' is returned unchanged.
func appendPrefix(prefix string, path string) string {

	prefix = strings.TrimRight(prefix, "/")

	if prefix != "" {
		path = strings.TrimLeft(path, "/")
		path = filepath.Join(prefix, path)
	}

	return path
}
