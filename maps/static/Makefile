LEAFLET_VERSION=1.9.4
PROTOMAPS_LEAFLET_VERSION4=4.0.1
PROTOMAPS_LEAFLET_VERSION5=5.1.0
PMTILES_VERSION=4.4.0
MAPLIBRE_VERSION=5.21.1

maplibre:
	curl -s -o www/css/maplibre-gl-$(MAPLIBRE_VERSION).css https://unpkg.com/maplibre-gl@$(MAPLIBRE_VERSION)/dist/maplibre-gl.css
	curl -s -o www/javascript/maplibre-gl-$(MAPLIBRE_VERSION).js https://unpkg.com/maplibre-gl@$(MAPLIBRE_VERSION)/dist/maplibre-gl.js

leaflet:
	curl -s -o www/css/leaflet-$(LEAFLET_VERSION).css https://unpkg.com/leaflet@$(LEAFLET_VERSION)/dist/leaflet.css
	curl -s -o www/javascript/leaflet-$(LEAFLET_VERSION).js https://unpkg.com/leaflet@$(LEAFLET_VERSION)/dist/leaflet.js

protomaps:
	curl -s -o www/javascript/protomaps-leaflet-$(PROTOMAPS_LEAFLET_VERSION4).js https://unpkg.com/protomaps-leaflet@$(PROTOMAPS_LEAFLET_VERSION4)/dist/protomaps-leaflet.js
	curl -s -o www/javascript/protomaps-leaflet-$(PROTOMAPS_LEAFLET_VERSION5).js https://unpkg.com/protomaps-leaflet@$(PROTOMAPS_LEAFLET_VERSION5)/dist/protomaps-leaflet.js

pmtiles:
	curl -s -o www/javascript/pmtiles-$(PMTILES_VERSION).js https://unpkg.com/pmtiles@$(PMTILES_VERSION)/dist/pmtiles.js
