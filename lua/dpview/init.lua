local server = require("dpview.server")
local sync = require("dpview.sync")

local M = {}

local defaults = {
  binary = nil,
  go_run_fallback = true,
  host = "127.0.0.1",
  port = nil,
  sidebar_collapsed = false,
  theme = nil,
  preview_theme = "default",
  typst_preview_theme = true,
  auto_start = true,
  auto_open_browser = false,
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
    require("dpview").sync_current_buffer({ bufnr = 0, force_start = true })
  end, {})

  vim.api.nvim_create_user_command("DPviewStatus", function()
    require("dpview").status()
  end, {})
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

function M.start()
  server.start(state, function(ok, err)
    if not ok then
      if err then
        notify(vim.log.levels.ERROR, err)
      end
      return
    end
    sync.sync_current(state, { bufnr = 0, force_start = true })
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
    sync.sync_current(state, { bufnr = 0, force_start = true })
  end)
end

function M.status()
  local lines = {
    ("root: %s"):format(state.startup_root),
    ("url: %s"):format(server.get_url(state) or "not running"),
    ("running: %s"):format(server.is_running(state) and "yes" or "no"),
    ("auto_start: %s"):format(state.config.auto_start and "true" or "false"),
    ("sidebar_collapsed: %s"):format(state.config.sidebar_collapsed and "true" or "false"),
    ("theme: %s"):format(state.config.theme or "unset"),
    ("preview_theme: %s"):format(state.config.preview_theme or "unset"),
    ("typst_preview_theme: %s"):format(state.config.typst_preview_theme and "true" or "false"),
    ("launch: %s"):format(state.server.launch_label or "not started"),
  }
  if state.server.last_error then
    lines[#lines + 1] = ("last_error: %s"):format(state.server.last_error)
  end
  vim.api.nvim_echo({ { table.concat(lines, "\n") } }, false, {})
end

function M._state()
  return state
end

return M
