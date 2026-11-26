package maps

import (
	"fmt"
	"net/http"
	"path/filepath"
)

// PROTOMAPS_API_TILE_URL is the URL template for the Protomaps hosted tiles API, where "{key}" is replaced by an API key.
const PROTOMAPS_API_TILE_URL = "https://api.protomaps.com/tiles/v3/{z}/{x}/{y}.mvt?key={key}"

// ProtomapsConfig defines configuration details for maps using Protomaps.
type ProtomapsConfig struct {
	// A valid Protomaps theme label
	Theme string `json:"theme"`
	// MaxDataZoom is the maximum zoom (tile) level for data in a PMTiles database
	MaxDataZoom int `json:"max_data_zoom,omitempty"`
	// PaintRules are optional custom Protomaps paint rules.
	PaintRules []map[string]any `json:"paintRules"`
	// LabelRules are optional custom Protomaps label rules.
	LabelRules []map[string]any `json:"labelRules"`
}

// ProtomapsFileHandlerFromPath takes a path and creates a http.FileServer handler
// instance for the files in its root directory. The handler is returned with
// a relative URI for the filename in 'path' to be assigned to a net/http
// ServeMux instance. If 'prefix' is not empty it is stripped from request paths
// and prepended to the returned URI.
func ProtomapsFileHandlerFromPath(path string, prefix string) (string, http.Handler, error) {

	abs_path, err := filepath.Abs(path)

	if err != nil {
		return "", nil, fmt.Errorf("Failed to determine absolute path for '%s', %v", path, err)
	}

	fname := filepath.Base(abs_path)
	root := filepath.Dir(abs_path)

	tile_dir := http.Dir(root)
	tile_handler := http.FileServer(tile_dir)

	tile_url := fmt.Sprintf("/%s", fname)

	if prefix != "" {
		tile_handler = http.StripPrefix(prefix, tile_handler)
		tile_url = filepath.Join(prefix, fname)
	}

	return tile_url, tile_handler, nil
}
