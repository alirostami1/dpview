local http = require("dpview.http")
local server = require("dpview.server")

local M = {}

local supported = {
  [".md"] = true,
  [".markdown"] = true,
  [".typ"] = true,
  [".typst"] = true,
}

local function normalize(path)
  return vim.fs.normalize(path)
end

local function notify(state, level, message)
  if state.config.notify == false then
    return
  end
  vim.schedule(function()
    vim.notify(message, level, { title = "dpview.nvim" })
  end)
end

local function current_path(bufnr)
  local name = vim.api.nvim_buf_get_name(bufnr)
  if name == "" then
    return nil
  end
  return normalize(name)
end

local function relative_to_root(root, path)
  root = normalize(root)
  path = normalize(path)
  if path == root then
    return nil
  end
  local prefix = root .. "/"
  if not vim.startswith(path, prefix) then
    return nil
  end
  return path:sub(#prefix + 1)
end

function M.previewable_path(state, bufnr)
  local path = current_path(bufnr)
  if not path then
    return nil
  end

  if vim.bo[bufnr].buftype ~= "" then
    return nil
  end

  local ext = vim.fn.fnamemodify(path, ":e")
  ext = ext == "" and "" or "." .. ext:lower()
  if not supported[ext] then
    return nil
  end

  return relative_to_root(state.startup_root, path)
end

local function post_current(state, relpath)
  http.request_json({
    method = "POST",
    host = state.config.host,
    port = state.server.port,
    path = "/api/current",
    body = vim.json.encode({ path = relpath, origin = "editor" }),
  }, function(err, response, payload)
    if err then
      notify(state, vim.log.levels.ERROR, "failed to sync buffer: " .. err)
      return
    end

    if response.status ~= 200 or not payload or payload.ok ~= true then
      notify(state, vim.log.levels.ERROR, "dpview rejected the current buffer")
      return
    end
    state.server.current_path = relpath
  end)
end

local function buffer_content(bufnr)
  local lines = vim.api.nvim_buf_get_lines(bufnr, 0, -1, true)
  local content = table.concat(lines, "\n")
  if vim.bo[bufnr].endofline then
    content = content .. "\n"
  end
  return content
end

local function post_live_preview(state, relpath, content, version)
  http.request_json({
    method = "POST",
    host = state.config.host,
    port = state.server.port,
    path = "/api/live-preview",
    body = vim.json.encode({
      path = relpath,
      origin = "editor",
      content = content,
      version = version,
    }),
  }, function(err, response, payload)
    if err then
      notify(state, vim.log.levels.ERROR, "failed to sync live preview: " .. err)
      return
    end

    if response.status == 409 and payload and payload.error and payload.error.code == "live_buffer_preview_disabled" then
      state.config.live_buffer_preview = false
      return
    end
    if response.status == 409 and payload and payload.error and payload.error.code == "stale_live_preview" then
      return
    end
    if response.status ~= 200 or not payload or payload.ok ~= true then
      notify(state, vim.log.levels.ERROR, "dpview rejected the live buffer preview")
      return
    end
    state.server.current_path = relpath
  end)
end

local function current_seek_payload(state, bufnr)
  if state.config.cursor_seek == false then
    return nil
  end
  local relpath = M.previewable_path(state, bufnr)
  if not relpath then
    return nil
  end
  local win = vim.api.nvim_get_current_win()
  if vim.api.nvim_win_get_buf(win) ~= bufnr then
    return nil
  end
  local cursor = vim.api.nvim_win_get_cursor(win)
  return {
    path = relpath,
    line = cursor[1],
    column = cursor[2] + 1,
    top_line = vim.fn.line("w0", win),
    bottom_line = vim.fn.line("w$", win),
    focus_line = cursor[1],
  }
end

local function post_seek(state, payload)
  http.request_json({
    method = "POST",
    host = state.config.host,
    port = state.server.port,
    path = "/api/seek",
    body = vim.json.encode(payload),
  }, function(err, response, payload)
    if err then
      notify(state, vim.log.levels.ERROR, "failed to sync cursor: " .. err)
      return
    end

    if response.status == 409 then
      return
    end
    if response.status ~= 200 or not payload or payload.ok ~= true then
      notify(state, vim.log.levels.ERROR, "dpview rejected the cursor position")
    end
  end)
end

function M.sync_current(state, opts)
  opts = opts or {}
  if state.config.editor_file_sync == false and not opts.force_current then
    return
  end
  local relpath = M.previewable_path(state, opts.bufnr or 0)
  if not relpath then
    return
  end

  if server.is_running(state) then
    post_current(state, relpath)
    return
  end

  if not state.config.auto_start and not opts.force_start then
    return
  end

  server.start(state, function(ok, err)
    if not ok then
      if err then
        notify(state, vim.log.levels.ERROR, err)
      end
      return
    end
    post_current(state, relpath)
  end)
end

function M.sync_seek(state, opts)
  opts = opts or {}
  local bufnr = opts.bufnr or 0
  local payload = current_seek_payload(state, bufnr)
  if not payload then
    return
  end
  if not server.is_running(state) then
    return
  end

  state.seek.seq = (state.seek.seq or 0) + 1
  local seq = state.seek.seq
  vim.defer_fn(function()
    if seq ~= state.seek.seq then
      return
    end
    if state.server.current_path ~= payload.path then
      M.sync_current(state, { bufnr = bufnr, force_start = false })
      return
    end
    post_seek(state, payload)
  end, state.config.cursor_seek_debounce_ms or 80)
end

function M.sync_live_preview(state, opts)
  opts = opts or {}
  if state.config.editor_file_sync == false or state.config.live_buffer_preview == false then
    return
  end

  local bufnr = opts.bufnr or 0
  local relpath = M.previewable_path(state, bufnr)
  if not relpath then
    return
  end

  local content = buffer_content(bufnr)
  state.live.seq = (state.live.seq or 0) + 1
  local version = state.live.seq
  local function send_if_current()
    if version ~= state.live.seq then
      return
    end

    local function send_preview()
      post_live_preview(state, relpath, content, version)
    end

    if server.is_running(state) then
      send_preview()
      return
    end
    if not state.config.auto_start then
      return
    end

    server.start(state, function(ok, err)
      if not ok then
        if err then
          notify(state, vim.log.levels.ERROR, err)
        end
        return
      end
      send_preview()
    end)
  end

  if opts.immediate then
    send_if_current()
    return
  end

  vim.defer_fn(send_if_current, state.config.live_buffer_preview_debounce_ms or 200)
end

return M
