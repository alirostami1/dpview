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
    body = vim.json.encode({ path = relpath }),
  }, function(err, response, payload)
    if err then
      notify(state, vim.log.levels.ERROR, "failed to sync buffer: " .. err)
      return
    end

    if response.status ~= 200 or not payload or payload.ok ~= true then
      notify(state, vim.log.levels.ERROR, "dpview rejected the current buffer")
    end
  end)
end

function M.sync_current(state, opts)
  opts = opts or {}
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

return M
