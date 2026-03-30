.PHONY: web contracts build test check run dev

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

contracts:
	GOCACHE=/tmp/dpview-gocache GOMODCACHE=/tmp/dpview-gomodcache GOPATH=/tmp/dpview-gopath go generate ./internal/api

web:
	$(MAKE) contracts
	npm run build:web

build:
	$(MAKE) web
	go build -ldflags="$(LDFLAGS)" -o build/dpview ./cmd/dpview

test:
	 go test ./...

check:
	$(MAKE) web
	go build -ldflags="$(LDFLAGS)" ./...

run:
	$(MAKE) web
	go run -ldflags="$(LDFLAGS)" ./cmd/dpview --root .

dev:
	$(MAKE) web
	 go tool air -c .air.toml
