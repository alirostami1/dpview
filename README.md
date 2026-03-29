# DPview

DPview is a local Go web app for browsing and previewing Markdown and Typst files.

## Features

- Single Go binary
- Markdown rendering with `goldmark`, plus HTML sanitization before browser display
- Typst rendering to themed SVG pages through the `typst` CLI
- Live updates via SSE when the current file changes or rerenders

## Requirements

- Go 1.25+
- typst

Markdown previews still work when Typst is unavailable.

## Build

```bash
make build
```

## Run

```bash
./dpview --root /path/to/docs --bind 127.0.0.1 --port 8090
```

Open `http://127.0.0.1:8090`.

For a quick theming check in this repo, run against [`examples/`](examples/) as the root:

```bash
./dpview --root ./examples --bind 127.0.0.1 --port 8090
```

## Preview Theming

Markdown theme files use a shared selector contract documented in [`docs/markdown-theming.md`](docs/markdown-theming.md).
Typst previews use the same preview theme ids and a Typst wrapper contract documented in [`docs/typst-theming.md`](docs/typst-theming.md).
