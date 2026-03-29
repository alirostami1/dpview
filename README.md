# DPview

DPview is a local Go web app for browsing and previewing Markdown and Typst files.

## Features

- Single Go binary
- Markdown rendering with `goldmark`, YAML front matter support, and HTML sanitization before browser display
- Typst rendering to themed SVG pages through the `typst` CLI
- Live updates via SSE when the current file changes or rerenders
- Optional seek synchronization between editor viewport updates and the preview

## Requirements

- Go 1.25+
- typst

Markdown previews still work when Typst is unavailable.

## Build

```bash
make build
```

## Install

```bash
go install codeberg.org/aros/dpview/cmd/dpview@latest
```

## Run

```bash
dpview --root /path/to/docs --bind 127.0.0.1 --port 8090
```

Open `http://127.0.0.1:8090`.

## Neovim Plugin

Neovim plugin can start DPview for the directory
where Neovim was launched and sync the current Markdown or Typst buffer.

Installation examples and plugin-specific docs live in
[`lua/dpview/README.md`](lua/dpview/README.md).

See [`doc/dpview.txt`](doc/dpview.txt) for the full option list.
