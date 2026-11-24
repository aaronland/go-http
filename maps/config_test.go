package maps

import (
	"encoding/json"
	"testing"
)

func TestMapConfig(t *testing.T) {

	cfg := MapConfig{
		Provider: "leaflet",
		TileURL:  "https://tile.openstreetmap.org/{z}/{x}/{y}.png",
		InitialView: &InitialView{
			-122.384292,
			37.621131,
		},
		InitialZoom: 13,
	}

	enc_cfg, err := json.Marshal(cfg)

	if err != nil {
		t.Fatalf("Failed to marshal config, %v", err)
	}

	var cfg2 MapConfig

	err = json.Unmarshal(enc_cfg, &cfg2)

	if err != nil {
		t.Fatalf("Failed to unmarshal config, %v", err)
	}

	if cfg2.InitialView.String() != cfg.InitialView.String() {
		t.Fatalf("Invalid roundt trip for initial view")
	}

}
