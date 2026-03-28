GOCACHE ?= /tmp/dpview-gocache
GOMODCACHE ?= /tmp/dpview-gomodcache
GOPATH ?= /tmp/dpview-gopath

GO_ENV = GOCACHE=$(GOCACHE) GOMODCACHE=$(GOMODCACHE) GOPATH=$(GOPATH)

.PHONY: build test check run

build:
	$(GO_ENV) go build -o dpview ./app/cmd

test:
	$(GO_ENV) go test ./...

check:
	$(GO_ENV) go build ./...

run:
	$(GO_ENV) go run ./app/cmd --root .
