.PHONY: build checksums winres

VERSION ?= dev
LDFLAGS = -s -w -X main.version=$(VERSION)

build:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/dvpl-linux-amd64 ./cmd/dvpl
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/dvpl-linux-arm64 ./cmd/dvpl
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/dvpl-windows-amd64.exe ./cmd/dvpl
	CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/dvpl-windows-386.exe ./cmd/dvpl
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags purego -trimpath -ldflags "$(LDFLAGS)" -o dist/dvpl-linux-amd64-purego ./cmd/dvpl

checksums: build
	cd dist && sha256sum dvpl-linux-amd64 dvpl-linux-arm64 dvpl-windows-amd64.exe dvpl-windows-386.exe dvpl-linux-amd64-purego > SHA256SUMS

winres:
	go run ./tools/winres -version "$(VERSION)"
