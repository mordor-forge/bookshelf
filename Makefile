.PHONY: build run tidy test image clean web

BIN     ?= bin/bookshelf
PKG     ?= ./cmd/bookshelf
TAG     ?= dev
IMAGE   ?= bookshelf:$(TAG)

# web builds the SPA. Vite writes directly into internal/web/dist for go:embed.
web:
	cd web && npm ci && npm run build

build: web
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o $(BIN) $(PKG)

run:
	go run $(PKG)

tidy:
	go mod tidy

test:
	go test ./...

image:
	docker build -t $(IMAGE) .

clean:
	rm -rf bin/
