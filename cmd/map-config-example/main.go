// Command map-config-example is an example web server that serves a map configuration
// (at /map.json) and an embedded web application for displaying a map using that configuration.
package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"

	"github.com/aaronland/go-http/v4/maps"
	"github.com/aaronland/go-http/v4/maps/static/www"
	"github.com/sfomuseum/go-flags/multi"
)

func main() {

	var verbose bool

	var host string
	var port int

	var initial_view string

	var map_provider string
	var map_tile_uri string
	var protomaps_theme string
	var protomaps_max_data_zoom int

	var leaflet_style string
	var leaflet_point_style string
	var leaflet_label_properties multi.MultiString
	var leaflet_panes multi.KeyValueInt64

	flag.StringVar(&map_provider, "map-provider", "leaflet", "Valid options are: leaflet, protomaps")
	flag.StringVar(&map_tile_uri, "map-tile-uri", maps.LEAFLET_OSM_TILE_URL, "A valid Leaflet tile layer URI. See documentation for special-case (interpolated tile) URIs.")
	flag.StringVar(&protomaps_theme, "protomaps-theme", "light", "A valid Protomaps theme label.")
	flag.IntVar(&protomaps_max_data_zoom, "protomaps-max-data-zoom", 0, "The maximum zoom (tile) level for data in a PMTiles database")
	flag.StringVar(&leaflet_style, "leaflet-style", "", "A custom Leaflet style definition for geometries. This may either be a JSON-encoded string or a path on disk.")
	flag.StringVar(&leaflet_point_style, "leaflet-point-style", "", "A custom Leaflet style definition for points. This may either be a JSON-encoded string or a path on disk.")

	flag.Var(&leaflet_label_properties, "leaflet-label-property", "Zero or more (GeoJSON Feature) properties to use to construct a label for a feature's popup menu when it is clicked on.")

	flag.Var(&leaflet_panes, "leaflet-pane", "Zero or more {LABEL}={Z_INDEX} pairs used to define Leaflet pane information.")

	flag.StringVar(&initial_view, "initial-view", "", "A comma-separated string indicating the map's initial view. Valid options are: 'LON,LAT', 'LON,LAT,ZOOM' or 'MINX,MINY,MAXX,MAXY'.")

	flag.StringVar(&host, "host", "localhost", "The host to listen for requests on")
	flag.IntVar(&port, "port", 8080, "The port number to listen for requests on")
	flag.BoolVar(&verbose, "verbose", false, "Enable verbose (debug) logging.")

	flag.Parse()

	if verbose {
		slog.SetLogLoggerLevel(slog.LevelDebug)
		slog.Debug("Verbose logging enabled")
	}

	mux := http.NewServeMux()

	opts := &maps.AssignMapConfigHandlerOptions{
		MapProvider:            map_provider,
		MapTileURI:             map_tile_uri,
		InitialView:            initial_view,
		LeafletStyle:           leaflet_style,
		LeafletPointStyle:      leaflet_point_style,
		LeafletLabelProperties: leaflet_label_properties,
		ProtomapsTheme:         protomaps_theme,
		ProtomapsMaxDataZoom:   protomaps_max_data_zoom,
	}

	if len(leaflet_panes) > 0 {

		panes := make(map[string]int)

		for _, fl := range leaflet_panes {
			panes[fl.Key()] = int(fl.Value().(int64))
		}

		opts.LeafletPanes = panes
	}

	err := maps.AssignMapConfigHandler(opts, mux, "/map.json")

	if err != nil {
		log.Fatalf("Failed to assign map config handler, %v", err)
	}

	www_fs := http.FS(www.FS)
	www_handler := http.FileServer(www_fs)

	mux.Handle("/", www_handler)

	addr := fmt.Sprintf("%s:%d", host, port)
	slog.Info("Listening for requests", "address", addr)

	err = http.ListenAndServe(addr, mux)

	if err != nil {
		log.Fatalf("Failed to serve requests, %v", err)
	}

}
