// Package maps provides configuration types and HTTP handlers for web maps using Leaflet and Protomaps.
package maps

import (
	"fmt"
	"net/http"
)

// InitialView is the initial (longitude, latitude) centre point of a map.
type InitialView [2]float64

// String returns the view as a comma-separated "lon,lat" string.
func (v *InitialView) String() string {
	return fmt.Sprintf("%f,%f", v[0], v[1])
}

// InitialBounds is the initial (minx, miny, maxx, maxy) bounding box of a map.
type InitialBounds [4]float64

// String returns the bounds as a comma-separated "minx,miny,maxx,maxy" string.
func (v *InitialBounds) String() string {
	return fmt.Sprintf("%f,%f,%f,%f", v[0], v[1], v[2], v[3])
}

// MapConfig defines common configuration details for maps.
type MapConfig struct {
	// A valid map provider label.
	Provider string `json:"provider"`
	// A valid Leaflet tile layer URI.
	TileURL string `json:"tile_url"`
	// A custom http.Handler to use with `TileURL`. Used for PMTiles-based tiles.
	TileURLHandler http.Handler `json:"tile_url_handler,omitempty"`
	// Optional Protomaps configuration details
	Protomaps *ProtomapsConfig `json:"protomaps,omitempty"`
	// Optional Leaflet configuration details
	Leaflet *LeafletConfig `json:"leaflet,omitempty"`
	// The initial view (lon, lat) for the map (optional).
	InitialView *InitialView `json:"initial_view,omitempty"`
	// The initial zoom level for the map (optional).
	InitialZoom int `json:"initial_zoom,omitempty"`
	// The initial bounds (minx, miny, maxx, maxy) for the map (optional).
	InitialBounds *InitialBounds `json:"initial_bounds,omitempty"`
}
