local server = require("dpview.server")
local sync = require("dpview.sync")
local http = require("dpview.http")

local M = {}

local defaults = {
  binary = nil,
  host = "127.0.0.1",
  port = nil,
  sidebar_collapsed = false,
  editor_file_sync = true,
  live_buffer_preview = false,
  latex_enabled = true,
  theme = nil,
  preview_theme = "default",
  typst_preview_theme = true,
  markdown_frontmatter_visible = true,
  markdown_frontmatter_expanded = true,
  markdown_frontmatter_title = true,
  cursor_seek = true,
  cursor_seek_debounce_ms = 80,
  live_buffer_preview_debounce_ms = 200,
  auto_start = true,
  auto_open_browser = false,
  log_level = "info",
  notify = true,
  open_cmd = nil,
}

local state = {
  startup_root = vim.uv.cwd() or vim.fn.getcwd(),
  config = vim.deepcopy(defaults),
  initialized = false,
  server = {
    job_id = nil,
    port = nil,
    running = false,
    starting = false,
    pending = {},
    browser_opened = false,
    launch_label = nil,
    last_error = nil,
    current_path = nil,
  },
  seek = {
    seq = 0,
  },
  live = {
    seq = 0,
  },
}

local function notify(level, message)
  if state.config.notify == false then
    return
  end
  vim.schedule(function()
    vim.notify(message, level, { title = "dpview.nvim" })
  end)
end

local function create_autocmds()
  local group = vim.api.nvim_create_augroup("dpview.nvim", { clear = true })

  vim.api.nvim_create_autocmd("BufEnter", {
    group = group,
    callback = function(args)
      require("dpview").sync_current_buffer({ bufnr = args.buf })
      require("dpview").sync_live_preview({ bufnr = args.buf, immediate = true })
      require("dpview").sync_view({ bufnr = args.buf })
    end,
  })

  vim.api.nvim_create_autocmd({ "CursorMoved", "CursorMovedI", "WinScrolled" }, {
    group = group,
    callback = function(args)
      require("dpview").sync_view({ bufnr = args.buf })
    end,
  })

  vim.api.nvim_create_autocmd({ "TextChanged", "TextChangedI" }, {
    group = group,
    callback = function(args)
      require("dpview").sync_live_preview({ bufnr = args.buf })
    end,
  })

  vim.api.nvim_create_autocmd("VimLeavePre", {
    group = group,
    callback = function()
      require("dpview").stop()
    end,
  })
end

local function create_commands()
  vim.api.nvim_create_user_command("DPviewStart", function()
    require("dpview").start()
  end, {})

  vim.api.nvim_create_user_command("DPviewStop", function()
    require("dpview").stop()
  end, {})

  vim.api.nvim_create_user_command("DPviewOpen", function()
    require("dpview").open()
  end, {})

  vim.api.nvim_create_user_command("DPviewSync", function()
    require("dpview").sync_current_buffer({ bufnr = 0, force_start = true, force_current = true })
  end, {})

  vim.api.nvim_create_user_command("DPviewStatus", function()
    require("dpview").status()
  end, {})

  vim.api.nvim_create_user_command("DPviewSeekEnable", function()
    require("dpview").set_seek_enabled(true)
  end, {})

  vim.api.nvim_create_user_command("DPviewSeekDisable", function()
    require("dpview").set_seek_enabled(false)
  end, {})

  vim.api.nvim_create_user_command("DPviewSeekToggle", function()
    require("dpview").set_seek_enabled(not state.config.cursor_seek)
  end, {})

  vim.api.nvim_create_user_command("DPviewFileSyncEnable", function()
    require("dpview").set_file_sync_enabled(true)
  end, {})

  vim.api.nvim_create_user_command("DPviewFileSyncDisable", function()
    require("dpview").set_file_sync_enabled(false)
  end, {})

  vim.api.nvim_create_user_command("DPviewFileSyncToggle", function()
    require("dpview").set_file_sync_enabled(not state.config.editor_file_sync)
  end, {})

  vim.api.nvim_create_user_command("DPviewLivePreviewEnable", function()
    require("dpview").set_live_buffer_preview_enabled(true)
  end, {})

  vim.api.nvim_create_user_command("DPviewLivePreviewDisable", function()
    require("dpview").set_live_buffer_preview_enabled(false)
  end, {})

  vim.api.nvim_create_user_command("DPviewLivePreviewToggle", function()
    require("dpview").set_live_buffer_preview_enabled(not state.config.live_buffer_preview)
  end, {})
end

local function current_settings_payload()
  return {
    sidebar_collapsed = state.config.sidebar_collapsed,
    editor_file_sync_enabled = state.config.editor_file_sync,
    live_buffer_preview_enabled = state.config.live_buffer_preview,
    seek_enabled = state.config.cursor_seek,
    latex_enabled = state.config.latex_enabled,
    typst_preview_theme = state.config.typst_preview_theme,
    markdown_frontmatter_visible = state.config.markdown_frontmatter_visible,
    markdown_frontmatter_expanded = state.config.markdown_frontmatter_expanded,
    markdown_frontmatter_title = state.config.markdown_frontmatter_title,
    theme = state.config.theme,
    preview_theme = state.config.preview_theme,
  }
end

local function sync_settings()
  if not server.is_running(state) then
    return
  end
  http.request_json({
    method = "POST",
    host = state.config.host,
    port = state.server.port,
    path = "/api/settings",
    body = vim.json.encode(current_settings_payload()),
  }, function(err, response, payload)
    if err then
      notify(vim.log.levels.ERROR, "failed to sync settings: " .. err)
      return
    end
    if response.status ~= 200 or not payload or payload.ok ~= true then
      notify(vim.log.levels.ERROR, "dpview rejected updated settings")
    end
  end)
end

function M._init_plugin()
  if state.initialized then
    return
  end
  create_commands()
  create_autocmds()
  state.initialized = true
end

function M.setup(opts)
  opts = opts or {}
  state.config = vim.tbl_deep_extend("force", vim.deepcopy(defaults), state.config, opts)
  if state.config.theme == nil then
    state.config.theme = vim.o.background == "light" and "light" or "dark"
  end
  M._init_plugin()
end

function M.sync_current_buffer(opts)
  sync.sync_current(state, opts)
end

function M.sync_view(opts)
  sync.sync_seek(state, opts)
end

function M.sync_live_preview(opts)
  sync.sync_live_preview(state, opts)
end

function M.start()
  server.start(state, function(ok, err)
    if not ok then
      if err then
        notify(vim.log.levels.ERROR, err)
      end
      return
    end
    sync.sync_current(state, { bufnr = 0, force_start = true, force_current = true })
    sync.sync_live_preview(state, { bufnr = 0, immediate = true })
  end)
end

function M.stop()
  server.stop(state)
end

function M.open()
  if server.is_running(state) then
    server.open_browser(state)
    return
  end

  server.start(state, function(ok, err)
    if not ok then
      if err then
        notify(vim.log.levels.ERROR, err)
      end
      return
    end
    server.open_browser(state)
    sync.sync_current(state, { bufnr = 0, force_start = true, force_current = true })
    sync.sync_live_preview(state, { bufnr = 0, immediate = true })
  end)
end

function M.status()
  local lines = {
    ("root: %s"):format(state.startup_root),
    ("url: %s"):format(server.get_url(state) or "not running"),
    ("running: %s"):format(server.is_running(state) and "yes" or "no"),
    ("auto_start: %s"):format(state.config.auto_start and "true" or "false"),
    ("sidebar_collapsed: %s"):format(state.config.sidebar_collapsed and "true" or "false"),
    ("editor_file_sync: %s"):format(state.config.editor_file_sync and "true" or "false"),
    ("live_buffer_preview: %s"):format(state.config.live_buffer_preview and "true" or "false"),
    ("latex_enabled: %s"):format(state.config.latex_enabled and "true" or "false"),
    ("cursor_seek: %s"):format(state.config.cursor_seek and "true" or "false"),
    ("live_buffer_preview_debounce_ms: %s"):format(tostring(state.config.live_buffer_preview_debounce_ms)),
    ("theme: %s"):format(state.config.theme or "unset"),
    ("preview_theme: %s"):format(state.config.preview_theme or "unset"),
    ("log_level: %s"):format(state.config.log_level or "info"),
    ("typst_preview_theme: %s"):format(state.config.typst_preview_theme and "true" or "false"),
    ("markdown_frontmatter_visible: %s"):format(state.config.markdown_frontmatter_visible and "true" or "false"),
    ("markdown_frontmatter_expanded: %s"):format(state.config.markdown_frontmatter_expanded and "true" or "false"),
    ("markdown_frontmatter_title: %s"):format(state.config.markdown_frontmatter_title and "true" or "false"),
    ("launch: %s"):format(state.server.launch_label or "not started"),
  }
  if state.server.last_error then
    lines[#lines + 1] = ("last_error: %s"):format(state.server.last_error)
  end
  vim.api.nvim_echo({ { table.concat(lines, "\n") } }, false, {})
end

function M.set_seek_enabled(enabled)
  state.config.cursor_seek = enabled and true or false
  sync_settings()
  notify(vim.log.levels.INFO, "DPview seeking " .. (state.config.cursor_seek and "enabled" or "disabled"))
end

function M.set_file_sync_enabled(enabled)
  state.config.editor_file_sync = enabled and true or false
  sync_settings()
  notify(vim.log.levels.INFO, "DPview editor file sync " .. (state.config.editor_file_sync and "enabled" or "disabled"))
end

function M.set_live_buffer_preview_enabled(enabled)
  state.config.live_buffer_preview = enabled and true or false
  sync_settings()
  notify(vim.log.levels.INFO, "DPview live buffer preview " .. (state.config.live_buffer_preview and "enabled" or "disabled"))
end

function M._state()
  return state
end

return M
