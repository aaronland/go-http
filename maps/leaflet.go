package maps

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"
)

// LEAFLET_OSM_TILE_URL is the URL template for OpenStreetMap's standard raster tile layer.
const LEAFLET_OSM_TILE_URL = "https://tile.openstreetmap.org/{z}/{x}/{y}.png"

// LeafletConfig defines configuration details for maps using Leaflet.
type LeafletConfig struct {
	// A `LeafletStyle` instance used to style geometries
	Style *LeafletStyle `json:"style,omitempty"`
	// A `LeafletStyle` instance used to style Point geometries
	PointStyle *LeafletStyle `json:"point_style,omitempty"`
	// Zero or more (GeoJSON Feature) properties to use to construct a label for a feature's popup menu when it is clicked on.
	LabelProperties []string `json:"label_properties,omitempty"`
	// A dictionary defining Leaflet label pane names and their z-index values.
	Panes map[string]int `json:"panes,omitempty"`
}

// LeafletStyle is a struct containing details for decorating GeoJSON features and markers.
type LeafletStyle struct {
	// Color is the stroke colour.
	Color string `json:"color,omitempty"`
	// FillColor is the fill colour.
	FillColor string `json:"fillColor,omitempty"`
	// Weight is the stroke width in pixels.
	Weight float64 `json:"weight,omitempty"`
	// Opacity is the stroke opacity.
	Opacity float64 `json:"opacity,omitempty"`
	// Radius is the radius of point markers.
	Radius float64 `json:"radius,omitempty"`
	// FillOpacity is the fill opacity.
	FillOpacity float64 `json:"fillOpacity,omitempty"`
	// Custom flags for runtime style configuration
	Custom any `json:"custom,omitempty"`
}

// UnmarshalLeafletStyle derives a `LeafletStyle` instance from 'raw'. If 'raw' starts with "{" then it is treated as
// a JSON-encoded string, otherwise it is treated as a local path on disk.
func UnmarshalLeafletStyle(raw string) (*LeafletStyle, error) {

	raw = strings.TrimSpace(raw)

	if len(raw) == 0 {
		return nil, fmt.Errorf("Empty style definition")
	}

	if string(raw[0]) == "{" {
		return UnmarshalStyleFromString(raw)
	}

	r, err := os.Open(raw)

	if err != nil {
		return nil, err
	}

	defer r.Close()

	return UnmarshalStyleFromReader(r)
}

// UnmarshalStyleFromString derives a `LeafletStyle` instance from 'raw'.
func UnmarshalStyleFromString(raw string) (*LeafletStyle, error) {

	var s *LeafletStyle

	err := json.Unmarshal([]byte(raw), &s)

	if err != nil {
		return nil, err
	}

	return s, nil
}

// UnmarshalStyleFromReader derives a `LeafletStyle` instance from the body of 'r'.
func UnmarshalStyleFromReader(r io.Reader) (*LeafletStyle, error) {

	var s *LeafletStyle

	dec := json.NewDecoder(r)
	err := dec.Decode(&s)

	if err != nil {
		return nil, err
	}

	return s, nil
}
