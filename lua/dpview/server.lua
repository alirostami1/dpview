local uv = vim.uv
local http = require("dpview.http")

local M = {}

local function join(...)
  return table.concat({ ... }, "/")
end

local function path_exists(path)
  return path and uv.fs_stat(path) ~= nil
end

local function is_file(path)
  local stat = path and uv.fs_stat(path) or nil
  return stat and stat.type == "file" or false
end

local function is_executable(path)
  return path and vim.fn.executable(path) == 1
end

local function pick_free_port(host)
  local _ = host
  if not M._seeded_random then
    math.randomseed(uv.hrtime())
    M._seeded_random = true
  end
  return math.random(49152, 65535)
end

local function repo_go_fallback(root)
  return path_exists(join(root, "go.mod")) and path_exists(join(root, "app/cmd/main.go"))
end

function M.resolve_command(state)
  local root = state.startup_root
  local config = state.config

  if config.binary and is_executable(config.binary) then
    return { config.binary }, "configured binary"
  end

  local on_path = vim.fn.exepath("dpview")
  if on_path ~= "" then
    return { on_path }, "PATH binary"
  end

  for _, candidate in ipairs({
    join(root, "build", "dpview"),
    join(root, "build", "main"),
  }) do
    if is_executable(candidate) then
      return { candidate }, "repo build"
    end
  end

  if config.go_run_fallback and repo_go_fallback(root) then
    return { "go", "run", "./app/cmd" }, "go run fallback"
  end

  return nil, "dpview executable not found"
end

function M.get_url(state)
  local port = state.server.port
  if not port then
    return nil
  end
  return ("http://%s:%d"):format(state.config.host, port)
end

local function notify(state, level, message)
  if state.config.notify == false then
    return
  end
  vim.schedule(function()
    vim.notify(message, level, { title = "dpview.nvim" })
  end)
end

local function finish_start(state, ok, err)
  local callbacks = state.server.pending
  state.server.pending = {}
  state.server.starting = false

  if not ok then
    state.server.running = false
    if err then
      state.server.last_error = err
    end
  end

  for _, callback in ipairs(callbacks) do
    callback(ok, err)
  end
end

function M.is_running(state)
  return state.server.running and state.server.job_id ~= nil
end

local function wait_for_ready(state, callback)
  local attempts = 0
  local timer = uv.new_timer()
  if not timer then
    callback(false, "failed to allocate readiness timer")
    return
  end

  local function stop(ok, err)
    if not timer:is_closing() then
      timer:stop()
      timer:close()
    end
    callback(ok, err)
  end

  timer:start(0, 150, function()
    attempts = attempts + 1

    if state.server.job_id == nil then
      stop(false, "dpview exited before becoming ready")
      return
    end

    http.request_json({
      method = "GET",
      host = state.config.host,
      port = state.server.port,
      path = "/api/health",
    }, function(err, response, payload)
      if err then
        if attempts >= 50 then
          stop(false, "dpview did not become ready: " .. err)
        end
        return
      end

      if response.status ~= 200 or not payload or payload.ok ~= true then
        if attempts >= 50 then
          stop(false, "dpview returned an unexpected health response")
        end
        return
      end

      stop(true)
    end)
  end)
end

function M.start(state, callback)
  callback = callback or function() end

  if M.is_running(state) then
    callback(true)
    return
  end

  if state.server.starting then
    table.insert(state.server.pending, callback)
    return
  end

  local command, label = M.resolve_command(state)
  if not command then
    callback(false, label)
    return
  end

  local port = state.config.port
  if port == nil then
    local picked, err = pick_free_port(state.config.host)
    if not picked then
      callback(false, err)
      return
    end
    port = picked
  end

  local args = vim.list_extend(vim.deepcopy(command), {
    "--root",
    state.startup_root,
    "--bind",
    state.config.host,
    "--port",
    tostring(port),
    "--sidebar-closed=" .. tostring(state.config.sidebar_collapsed),
    "--theme",
    state.config.theme,
    "--preview-theme",
    state.config.preview_theme,
    "--typst-preview-theme=" .. tostring(state.config.typst_preview_theme),
  })

  state.server.starting = true
  state.server.pending = { callback }
  state.server.port = port
  state.server.launch_label = label
  state.server.last_error = nil

  local job_id = vim.fn.jobstart(args, {
    cwd = state.startup_root,
    detach = false,
    on_exit = function()
      state.server.running = false
      state.server.starting = false
      state.server.job_id = nil
    end,
  })

  if job_id <= 0 then
    finish_start(state, false, "failed to start dpview")
    return
  end

  state.server.job_id = job_id
  wait_for_ready(state, function(ok, err)
    if not ok then
      if state.server.job_id then
        vim.fn.jobstop(state.server.job_id)
      end
      finish_start(state, false, err)
      return
    end

    state.server.running = true
    if state.config.notify ~= false then
      notify(state, vim.log.levels.INFO, "DPview started at " .. M.get_url(state))
    end
    if state.config.auto_open_browser and not state.server.browser_opened then
      M.open_browser(state)
      state.server.browser_opened = true
    end
    finish_start(state, true)
  end)
end

function M.stop(state)
  if state.server.job_id then
    vim.fn.jobstop(state.server.job_id)
  end
  state.server.running = false
  state.server.starting = false
  state.server.job_id = nil
end

function M.open_browser(state)
  local url = M.get_url(state)
  if not url then
    notify(state, vim.log.levels.WARN, "dpview is not running")
    return false
  end

  if type(state.config.open_cmd) == "function" then
    state.config.open_cmd(url)
    return true
  end

  local uname = uv.os_uname()
  local cmd
  if uname.sysname == "Darwin" then
    cmd = { "open", url }
  elseif uname.sysname == "Windows_NT" then
    cmd = { "cmd", "/c", "start", "", url }
  else
    cmd = { "xdg-open", url }
  end
  vim.fn.jobstart(cmd, { detach = true })
  return true
end

return M
