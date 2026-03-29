# dpview.nvim

Neovim integration for DPview.

The plugin starts DPview for the directory where Neovim was launched and keeps
the current Markdown or Typst buffer in sync with the browser preview.
Theme settings are passed to DPview as startup flags.

## Requirements

- Neovim 0.11+
- a `dpview` binary on `PATH`, a repo-local build such as `build/dpview` or `build/main`, or this repo available for `go run ./app/cmd`
- `typst` installed if you want Typst previews

## Installation

### lazy.nvim

```lua
{
  "aros/dpview",
  config = function()
    require("dpview").setup({
      sidebar_collapsed = true,
      theme = "dark",
      preview_theme = "github",
      typst_preview_theme = false,
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
      theme = "dark",
      preview_theme = "github",
      typst_preview_theme = false,
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
  theme = "dark",
  preview_theme = "github",
  typst_preview_theme = true,
  auto_start = true,
  auto_open_browser = false,
  notify = true,
})
```

Options:

- `binary`: explicit path to the `dpview` executable
- `go_run_fallback`: allow `go run ./app/cmd` when the startup directory is this repo
- `host`: bind address for the local DPview server
- `port`: fixed port, or `nil` to let the plugin choose a high local port
- `sidebar_collapsed`: start DPview with the sidebar collapsed
- `theme`: DPview app theme, `light` or `dark`; defaults to Neovim's current `background`
- `preview_theme`: DPview preview theme id such as `default`, `github`, `notion`, or `paper`
- `typst_preview_theme`: when false, DPview renders Typst sources directly without injecting preview theme tokens
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

## Notes

- The DPview root is the directory where Neovim was started.
- Supported files are `.md`, `.markdown`, `.typ`, and `.typst`.
- Switching to unsupported buffers leaves the last DPview preview visible.
- Theme can be controlled from Neovim config through `theme` and `preview_theme`.
- Typst preview theming can be disabled from Neovim config through `typst_preview_theme`.
- Sidebar state can be controlled from Neovim config through `sidebar_collapsed`.
- Sidebar and theme values are passed to DPview at startup with `--sidebar-closed`,
  `--theme`, `--preview-theme`, and `--typst-preview-theme`.
