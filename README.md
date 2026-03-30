# DPview

DPview is a local Go web app for browsing and previewing Markdown and Typst files.

## Features

- Single Go binary
- Markdown rendering with `goldmark`, YAML front matter support, and HTML sanitization before browser display
- Typst rendering to themed SVG pages through the `typst` CLI
- Live updates via SSE when the current file changes or rerenders
- Optional editor file following and editor position synchronization

## Requirements

- Go 1.25+
- Node.js with npm
- typst

Markdown previews still work when Typst is unavailable.

## Build

```bash
npm install
make build
```

## Install

```bash
git clone https://codeberg.org/aros/dpview.git
cd dpview
npm install
make build
```

To install the latest GitHub release for your current OS and architecture:

```bash
curl -fsSL https://raw.githubusercontent.com/alirostami1/dpview/main/scripts/install.sh | sh
```

The installer places `dpview` in `~/.local/bin` by default and creates that
directory if it does not exist. Override the target directory like this:

```bash
curl -fsSL https://raw.githubusercontent.com/alirostami1/dpview/main/scripts/install.sh | \
  DPVIEW_INSTALL_DIR="$HOME/bin" sh
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

See [`doc/dpview.txt`](doc/dpview.txt) or `:help dpview` for the Neovim help files.
