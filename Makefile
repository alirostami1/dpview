.PHONY: \
	web contracts build test check run dev nvim-sample \
	setup hooks format format-go format-web format-lua \
	check-format check-format-go check-format-web check-format-lua \
	lint lint-go lint-web typecheck-web pre-commit

VERSION ?= dev
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo unknown)
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS = -X main.version=$(VERSION) -X main.commit=$(COMMIT) -X main.date=$(DATE)

setup:
	npm install
	go mod download
	$(MAKE) hooks

hooks:
	./scripts/install-git-hooks.sh

format:
	$(MAKE) format-go
	$(MAKE) format-web
	$(MAKE) format-lua

format-go:
	./scripts/format-go.sh

format-web:
	npm run format

format-lua:
	stylua lua

check-format:
	$(MAKE) check-format-go
	$(MAKE) check-format-web
	$(MAKE) check-format-lua

check-format-go:
	./scripts/check-go-format.sh

check-format-web:
	npm run check:format

check-format-lua:
	stylua --check lua

lint:
	$(MAKE) lint-go
	$(MAKE) lint-web

lint-go:
	go vet ./...

lint-web:
	npm run lint

typecheck-web:
	npm run typecheck

pre-commit:
	./scripts/pre-commit.sh

contracts:
	GOCACHE=/tmp/dpview-gocache GOMODCACHE=/tmp/dpview-gomodcache GOPATH=/tmp/dpview-gopath go generate ./internal/api

web:
	$(MAKE) contracts
	npm run build

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

nvim-sample: build
	nvim examples/sample.md \
		--cmd 'set runtimepath^=$(CURDIR)' \
		--cmd 'lua require("dpview").setup({ binary = "$(CURDIR)/build/dpview", port = 8484, sidebar_collapsed = false, editor_file_sync = true, live_buffer_preview = true, cursor_seek = true, typst_preview_theme = true, markdown_frontmatter_visible = true, markdown_frontmatter_expanded = true, markdown_frontmatter_title = true, auto_start = true, auto_open_browser = true, log_level = "debug", notify = true, preview_theme = "github" })' \
		-c 'DPviewOpen'
