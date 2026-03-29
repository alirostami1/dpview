.PHONY: build test check run dev

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

build:
	go build -ldflags="$(LDFLAGS)" -o build/dpview ./cmd/dpview

test:
	 go test ./...

check:
	go build -ldflags="$(LDFLAGS)" ./...

run:
	go run -ldflags="$(LDFLAGS)" ./cmd/dpview --root .

dev:
	 go tool air -c .air.toml
