package maps

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMapConfigFromOptionsInitialView(t *testing.T) {

	tests := []struct {
		name    string
		view    string
		wantErr string
		check   func(t *testing.T, cfg *MapConfig)
	}{
		{"empty", "", "", func(t *testing.T, cfg *MapConfig) {
			if cfg.InitialView != nil || cfg.InitialBounds != nil {
				t.Error("expected no view")
			}
		}},
		{"lon,lat", "-122.4,37.8", "", func(t *testing.T, cfg *MapConfig) {
			if cfg.InitialView == nil || cfg.InitialView[0] != -122.4 || cfg.InitialView[1] != 37.8 {
				t.Errorf("got %v", cfg.InitialView)
			}
		}},
		{"lon,lat,zoom", "-122.4,37.8,12", "", func(t *testing.T, cfg *MapConfig) {
			if cfg.InitialView == nil || cfg.InitialZoom != 12 {
				t.Errorf("got %v zoom %d", cfg.InitialView, cfg.InitialZoom)
			}
		}},
		{"bounds", "-123,37,-122,38", "", func(t *testing.T, cfg *MapConfig) {
			b := cfg.InitialBounds

			if b == nil || b[0] != -123 || b[1] != 37 || b[2] != -122 || b[3] != 38 {
				t.Errorf("got %v", b)
			}
		}},
		{"non-numeric longitude", "x,37", "longitude", nil},
		{"non-numeric latitude", "-122,y", "latitude", nil},
		{"non-integer zoom", "-122,37,1.5", "zoom", nil},
		{"bad minx", "a,1,2,3", "minx", nil},
		{"bad miny", "1,b,2,3", "miny", nil},
		{"bad maxx", "1,2,c,3", "maxx", nil},
		{"bad maxy", "1,2,3,d", "maxy", nil},
		{"one value", "1", "Invalid initial view", nil},
		{"five values", "1,2,3,4,5", "Invalid initial view", nil},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {

			cfg, err := MapConfigFromOptions(&AssignMapConfigHandlerOptions{
				MapProvider: "leaflet",
				InitialView: tc.view,
			})

			if tc.wantErr != "" {

				if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
					t.Fatalf("expected error containing %q, got %v", tc.wantErr, err)
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			tc.check(t, cfg)
		})
	}
}

func TestMapConfigFromOptionsLeaflet(t *testing.T) {

	cfg, err := MapConfigFromOptions(&AssignMapConfigHandlerOptions{
		MapProvider:            "leaflet",
		MapTileURI:             "https://tiles.example/{z}/{x}/{y}.png",
		LeafletLabelProperties: []string{"name"},
		LeafletPanes:           map[string]int{"top": 650},
	})

	if err != nil {
		t.Fatal(err)
	}

	if cfg.TileURL != "https://tiles.example/{z}/{x}/{y}.png" || cfg.Leaflet == nil {
		t.Fatalf("unexpected config %+v", cfg)
	}

	if cfg.Leaflet.LabelProperties[0] != "name" || cfg.Leaflet.Panes["top"] != 650 {
		t.Errorf("leaflet options lost: %+v", cfg.Leaflet)
	}

	if _, err := MapConfigFromOptions(&AssignMapConfigHandlerOptions{
		MapProvider:  "leaflet",
		LeafletStyle: "{not json",
	}); err == nil {
		t.Error("expected an error for an invalid leaflet style")
	}
}

func TestMapConfigFromOptionsProtomaps(t *testing.T) {

	cfg, err := MapConfigFromOptions(&AssignMapConfigHandlerOptions{
		MapProvider:          "protomaps",
		MapTileURI:           "api://abc123",
		ProtomapsTheme:       "dark",
		ProtomapsMaxDataZoom: 9,
	})

	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(cfg.TileURL, "abc123") || strings.Contains(cfg.TileURL, "{key}") {
		t.Errorf("API key not substituted: %q", cfg.TileURL)
	}

	if cfg.Protomaps == nil || cfg.Protomaps.Theme != "dark" || cfg.Protomaps.MaxDataZoom != 9 {
		t.Errorf("unexpected protomaps config %+v", cfg.Protomaps)
	}
}

func TestAssignMapConfigHandler(t *testing.T) {

	mux := http.NewServeMux()

	err := AssignMapConfigHandler(&AssignMapConfigHandlerOptions{
		MapProvider: "leaflet",
		InitialView: "-122.4,37.8,10",
	}, mux, "/map.json")

	if err != nil {
		t.Fatal(err)
	}

	rsp := httptest.NewRecorder()
	mux.ServeHTTP(rsp, httptest.NewRequest("GET", "/map.json", nil))

	if rsp.Code != 200 || !strings.HasPrefix(rsp.Header().Get("Content-Type"), "application/json") {
		t.Fatalf("got %d %q", rsp.Code, rsp.Header().Get("Content-Type"))
	}

	var cfg MapConfig

	if err := json.Unmarshal(rsp.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("response is not valid JSON, %v", err)
	}

	if cfg.InitialZoom != 10 {
		t.Errorf("got zoom %d", cfg.InitialZoom)
	}

	if err := AssignMapConfigHandler(&AssignMapConfigHandlerOptions{InitialView: "bad"}, http.NewServeMux(), "/x"); err == nil {
		t.Error("expected an error for an invalid initial view")
	}
}
