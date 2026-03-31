# dpview.nvim

Neovim integration for DPview.

The plugin starts DPview for the directory where Neovim was launched and keeps
the current Markdown or Typst buffer in sync with the browser preview.
Theme settings are passed to DPview as startup flags.

## Requirements

- Neovim 0.11+
- a `dpview` binary on `PATH` or a repo-local build such as `build/dpview` or `build/main`
- `typst` installed if you want Typst previews

The plugin resolves the DPview command in this order:

1. `binary` from `require("dpview").setup(...)` when it points to an executable
2. `dpview` found on `PATH`
3. `build/dpview` or `build/main` under the directory where Neovim was started
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
      latex_enabled = true,
      typst_preview_theme = false,
      markdown_frontmatter_visible = true,
      markdown_frontmatter_expanded = true,
      markdown_frontmatter_title = true,
      auto_start = true,
      auto_open_browser = false,
      log_level = "info",
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
      latex_enabled = true,
      typst_preview_theme = false,
      markdown_frontmatter_visible = true,
      markdown_frontmatter_expanded = true,
      markdown_frontmatter_title = true,
      auto_start = true,
      auto_open_browser = false,
      log_level = "info",
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
  latex_enabled = true,
  typst_preview_theme = false,
  markdown_frontmatter_visible = true,
  markdown_frontmatter_expanded = true,
  markdown_frontmatter_title = true,
  auto_start = true,
  auto_open_browser = false,
  log_level = "info",
})
```

## Setup

```lua
require("dpview").setup({
  binary = nil,
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
  latex_enabled = true,
  typst_preview_theme = true,
  markdown_frontmatter_visible = true,
  markdown_frontmatter_expanded = true,
  markdown_frontmatter_title = true,
  auto_start = true,
  auto_open_browser = false,
  log_level = "info",
  notify = true,
})
```

Options:

- `binary`: explicit path to the `dpview` executable
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
- `latex_enabled`: enable Markdown LaTeX math rendering and on-demand KaTeX loading in the web preview
- `log_level`: DPview server log level, one of `debug`, `info`, or `error`
- `typst_preview_theme`: when false, DPview renders Typst sources directly without injecting preview theme tokens
- `markdown_frontmatter_visible`: show parsed YAML front matter above Markdown previews
- `markdown_frontmatter_expanded`: start Markdown front matter panels expanded
- `markdown_frontmatter_title`: use front matter `title` as an H1 when the document has no H1
- `auto_start`: when false, the plugin never starts DPview automatically
- `auto_open_browser`: when true, open the browser once after the plugin starts DPview
- `notify`: enable or disable plugin notifications
- `open_cmd`: optional Lua function that receives the DPview URL; used by `:DPviewOpen` and `auto_open_browser`

## Commands

- `:DPviewStart`: start DPview and push the current supported buffer
- `:DPviewStop`: stop the DPview process started by the plugin
- `:DPviewOpen`: open the current DPview URL; starts DPview first if needed
- `:DPviewSync`: push the current supported buffer even when editor file sync is disabled
- `:DPviewStatus`: show the startup root, URL, launch method, and current plugin toggles
- `:DPviewSeekEnable`, `:DPviewSeekDisable`, `:DPviewSeekToggle`: control editor-to-preview seeking
- `:DPviewFileSyncEnable`, `:DPviewFileSyncDisable`, `:DPviewFileSyncToggle`: control active-buffer to preview-file following
- `:DPviewLivePreviewEnable`, `:DPviewLivePreviewDisable`, `:DPviewLivePreviewToggle`: control unsaved buffer preview updates

## Notes

- The DPview root is the directory where Neovim was started.
- Supported files are `.md`, `.markdown`, `.typ`, and `.typst`.
- Only files inside that startup root can be synced to DPview.
- Unnamed buffers, special buffers, and files outside the startup root are ignored.
- Switching to unsupported buffers leaves the last DPview preview visible.
- `BufEnter` syncs the current file, sends an immediate live-preview update when enabled, and updates seek state.
- `TextChanged` and `TextChangedI` drive live preview with `live_buffer_preview_debounce_ms`.
- `CursorMoved`, `CursorMovedI`, and `WinScrolled` drive seek updates with `cursor_seek_debounce_ms`.
- `auto_start = false` prevents automatic startup from those events, but `:DPviewStart`, `:DPviewOpen`, and `:DPviewSync` can still start or sync manually.
- Theme can be controlled from Neovim config through `theme` and `preview_theme`.
- File following can be controlled from Neovim config through `editor_file_sync` and at runtime through `:DPviewFileSyncEnable`, `:DPviewFileSyncDisable`, and `:DPviewFileSyncToggle`.
- Seeking can be controlled from Neovim config through `cursor_seek` and at runtime through `:DPviewSeekEnable`, `:DPviewSeekDisable`, and `:DPviewSeekToggle`.
- Live preview can be controlled from Neovim config through `live_buffer_preview` and at runtime through `:DPviewLivePreviewEnable`, `:DPviewLivePreviewDisable`, and `:DPviewLivePreviewToggle`.
- LaTeX math rendering can be controlled from Neovim config through `latex_enabled`.
- Typst preview theming can be disabled from Neovim config through `typst_preview_theme`.
- Markdown front matter behavior can be controlled from Neovim config through `markdown_frontmatter_visible`, `markdown_frontmatter_expanded`, and `markdown_frontmatter_title`.
- Sidebar state can be controlled from Neovim config through `sidebar_collapsed`.
- Sidebar, sync, and theme values are passed to DPview at startup with `--sidebar-closed`,
  `--editor-file-sync`, `--live-buffer-preview`, `--seek-enabled`, `--latex-enabled`, `--theme`,
  `--preview-theme`, `--typst-preview-theme`, `--markdown-frontmatter-visible`,
  `--markdown-frontmatter-expanded`, and `--markdown-frontmatter-title`.
