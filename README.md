# DPview

DPview is a local Go web app for browsing and previewing Markdown and Typst files.

## Features

- Single Go binary
- Markdown rendering with `goldmark`, plus HTML sanitization before browser display
- Typst rendering to SVG pages through the `typst` CLI
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
