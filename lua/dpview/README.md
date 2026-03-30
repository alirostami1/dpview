# dpview.nvim

Neovim integration for DPview.

The plugin starts DPview for the directory where Neovim was launched and keeps
the current Markdown or Typst buffer in sync with the browser preview.
Theme settings are passed to DPview as startup flags.

## Requirements

- Neovim 0.11+
- a `dpview` binary on `PATH`, a repo-local build such as `build/dpview` or `build/main`, or this repo available for `go run ./cmd/dpview`
- `typst` installed if you want Typst previews

## Installation

### lazy.nvim

```lua
{
  "aros/dpview",
  config = function()
    require("dpview").setup({
      sidebar_collapsed = true,
      editor_file_sync = true,
      theme = "dark",
      preview_theme = "github",
      cursor_seek = true,
      typst_preview_theme = false,
      markdown_frontmatter_visible = true,
      markdown_frontmatter_expanded = true,
      markdown_frontmatter_title = true,
      auto_start = true,
      auto_open_browser = false,
    })
  end,
}
```

### packer.nvim

```lua
use({
  "aros/dpview",
  config = function()
    require("dpview").setup({
      sidebar_collapsed = true,
      editor_file_sync = true,
      theme = "dark",
      preview_theme = "github",
      cursor_seek = true,
      typst_preview_theme = false,
      markdown_frontmatter_visible = true,
      markdown_frontmatter_expanded = true,
      markdown_frontmatter_title = true,
      auto_start = true,
      auto_open_browser = false,
    })
  end,
})
```

### vim-plug

```lua
vim.call("plug#begin")
vim.call("plug#", "aros/dpview")
vim.call("plug#end")

require("dpview").setup({
  sidebar_collapsed = true,
  theme = "dark",
  preview_theme = "github",
  typst_preview_theme = false,
  markdown_frontmatter_visible = true,
  markdown_frontmatter_expanded = true,
  markdown_frontmatter_title = true,
  auto_start = true,
  auto_open_browser = false,
})
```

## Setup

```lua
require("dpview").setup({
  binary = nil,
  go_run_fallback = true,
  host = "127.0.0.1",
  port = nil,
  sidebar_collapsed = false,
  editor_file_sync = true,
  theme = "dark",
  preview_theme = "github",
  cursor_seek = true,
  cursor_seek_debounce_ms = 80,
  live_buffer_preview = false,
  live_buffer_preview_debounce_ms = 200,
  typst_preview_theme = true,
  markdown_frontmatter_visible = true,
  markdown_frontmatter_expanded = true,
  markdown_frontmatter_title = true,
  auto_start = true,
  auto_open_browser = false,
  notify = true,
})
```

Options:

- `binary`: explicit path to the `dpview` executable
- `go_run_fallback`: allow `go run ./cmd/dpview` when the startup directory is this repo
- `host`: bind address for the local DPview server
- `port`: fixed port, or `nil` to let the plugin choose a high local port
- `sidebar_collapsed`: start DPview with the sidebar collapsed
- `editor_file_sync`: allow Neovim to switch the DPview preview to the active supported buffer
- `theme`: DPview app theme, `light` or `dark`; defaults to Neovim's current `background`
- `preview_theme`: DPview preview theme id such as `default`, `github`, `notion`, or `paper`
- `cursor_seek`: enable editor-to-preview seeking updates
- `cursor_seek_debounce_ms`: debounce delay for cursor/viewport seek updates
- `live_buffer_preview`: enable unsaved buffer preview updates while typing
- `live_buffer_preview_debounce_ms`: idle delay before sending a live buffer preview update; defaults to `200`
- `typst_preview_theme`: when false, DPview renders Typst sources directly without injecting preview theme tokens
- `markdown_frontmatter_visible`: show parsed YAML front matter above Markdown previews
- `markdown_frontmatter_expanded`: start Markdown front matter panels expanded
- `markdown_frontmatter_title`: use front matter `title` as an H1 when the document has no H1
- `auto_start`: when false, the plugin never starts DPview automatically
- `auto_open_browser`: when true, open the browser once after the plugin starts DPview
- `notify`: enable or disable plugin notifications
- `open_cmd`: optional Lua function that receives the DPview URL

## Commands

- `:DPviewStart`
- `:DPviewStop`
- `:DPviewOpen`
- `:DPviewSync`
- `:DPviewStatus`
- `:DPviewSeekEnable`
- `:DPviewSeekDisable`
- `:DPviewSeekToggle`
- `:DPviewFileSyncEnable`
- `:DPviewFileSyncDisable`
- `:DPviewFileSyncToggle`
- `:DPviewLivePreviewEnable`
- `:DPviewLivePreviewDisable`
- `:DPviewLivePreviewToggle`

## Notes

- The DPview root is the directory where Neovim was started.
- Supported files are `.md`, `.markdown`, `.typ`, and `.typst`.
- Switching to unsupported buffers leaves the last DPview preview visible.
- Theme can be controlled from Neovim config through `theme` and `preview_theme`.
- File following can be controlled from Neovim config through `editor_file_sync` and at runtime through `:DPviewFileSyncEnable`, `:DPviewFileSyncDisable`, and `:DPviewFileSyncToggle`.
- Seeking can be controlled from Neovim config through `cursor_seek` and at runtime through `:DPviewSeekEnable`, `:DPviewSeekDisable`, and `:DPviewSeekToggle`.
- Typst preview theming can be disabled from Neovim config through `typst_preview_theme`.
- Markdown front matter behavior can be controlled from Neovim config through `markdown_frontmatter_visible`, `markdown_frontmatter_expanded`, and `markdown_frontmatter_title`.
- Sidebar state can be controlled from Neovim config through `sidebar_collapsed`.
- Sidebar, sync, and theme values are passed to DPview at startup with `--sidebar-closed`,
  `--editor-file-sync`, `--seek-enabled`, `--theme`, `--preview-theme`, `--typst-preview-theme`,
  `--markdown-frontmatter-visible`, `--markdown-frontmatter-expanded`, and
  `--markdown-frontmatter-title`.
