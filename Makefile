GOMOD=$(shell test -f "go.work" && echo "readonly" || echo "vendor")
LDFLAGS=-s -w

cli:
	go build -mod $(GOMOD) -ldflags="$(LDFLAGS)" -o bin/jwt-secret cmd/auth/jwt-secret/main.go
	go build -mod $(GOMOD) -ldflags="$(LDFLAGS)" -o bin/encrypted-cookie cmd/cookie/encrypted-cookie/main.go
	go build -mod $(GOMOD) -ldflags="$(LDFLAGS)" -o bin/encrypted-crumb cmd/crumb/encrypted-crumb/main.go
	go build -mod $(GOMOD) -ldflags="$(LDFLAGS)" -o bin/fileserver cmd/fileserver/fileserver/main.go

