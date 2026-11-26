GOMOD=$(shell test -f "go.work" && echo "readonly" || echo "vendor")
LDFLAGS=-s -w

vuln:
	govulncheck -show verbose ./...

cli:
	go build -mod $(GOMOD) -ldflags="$(LDFLAGS)" -o bin/jwt-secret cmd/auth/jwt-secret/main.go
	go build -mod $(GOMOD) -ldflags="$(LDFLAGS)" -o bin/encrypted-cookie cmd/cookie/encrypted-cookie/main.go
	go build -mod $(GOMOD) -ldflags="$(LDFLAGS)" -o bin/encrypted-crumb cmd/crumb/encrypted-crumb/main.go
	go build -mod $(GOMOD) -ldflags="$(LDFLAGS)" -o bin/fileserver cmd/fileserver/fileserver/main.go

INITIAL_VIEW=-122.384292,37.621131,13

map-example:
	go run cmd/map-config-example/main.go \
		-leaflet-pane hello=100 \
		-leaflet-pane world=200 \
		-initial-view '$(INITIAL_VIEW)'

map-example-pm:
	go run cmd/map-config-example/main.go \
		-initial-view '$(INITIAL_VIEW)' \
		-map-provider protomaps \
		-protomaps-max-data-zoom 14 \
		-map-tile-uri 'file://$(CWD)/fixtures/sfo.pmtiles'

map-example-pm-paint:
	go run cmd/map-config-example/main.go \
		-initial-view '$(INITIAL_VIEW)' \
		-map-provider protomaps-paint \
		-protomaps-max-data-zoom 14 \
		-map-tile-uri 'file://$(CWD)/fixtures/sfo.pmtiles'

map-example-pm-raster:
	go run cmd/map-config-example/main.go \
		-initial-view '$(INITIAL_VIEW)' \
		-map-provider protomaps-raster \
		-protomaps-max-data-zoom 14 \
		-map-tile-uri https://static.sfomuseum.org/aerial/1936.pmtiles

map-example-pm-ml:
	go run cmd/map-config-example/main.go \
		-initial-view '$(INITIAL_VIEW)' \
		-map-provider protomaps-ml \
		-protomaps-max-data-zoom 14 \
		-map-tile-uri 'file://$(CWD)/fixtures/sfo.pmtiles'

bump-version:
	perl -i -p -e 's/github.com\/aaronland\/go-http\/$(OLD)/github.com\/aaronland\/go-http\/$(NEW)/g' go.mod
	perl -i -p -e 's/github.com\/aaronland\/go-http\/$(OLD)/github.com\/aaronland\/go-http\/$(NEW)/g' README.md
	find . -name '*.go' | xargs perl -i -p -e 's/github.com\/aaronland\/go-http\/$(OLD)/github.com\/aaronland\/go-http\/$(NEW)/g'
