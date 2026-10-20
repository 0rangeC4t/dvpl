.PHONY: build

build:
	mkdir -p dist
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/dvpl-linux-amd64 ./cmd/dvpl
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -o dist/dvpl-linux-arm64 ./cmd/dvpl
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-s -w" -o dist/dvpl-windows-amd64.exe ./cmd/dvpl
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -tags purego -o dist/dvpl-linux-amd64-purego ./cmd/dvpl
