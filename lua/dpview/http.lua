local uv = vim.uv

local M = {}

local function parse_headers(header)
  local headers = {}
  for line in header:gmatch("[^\r\n]+") do
    local name, value = line:match("^([^:]+):%s*(.*)$")
    if name and value then
      headers[name:lower()] = value
    end
  end
  return headers
end

local function decode_chunked(body)
  local pos = 1
  local out = {}

  while true do
    local line_end = body:find("\r\n", pos, true)
    if not line_end then
      return nil, "invalid chunked response"
    end

    local size_line = body:sub(pos, line_end - 1)
    local size = tonumber(size_line:match("^[0-9a-fA-F]+"), 16)
    if not size then
      return nil, "invalid chunk size"
    end

    pos = line_end + 2
    if size == 0 then
      return table.concat(out), nil
    end

    local chunk_end = pos + size - 1
    if chunk_end > #body then
      return nil, "truncated chunked response"
    end

    out[#out + 1] = body:sub(pos, chunk_end)
    pos = chunk_end + 1

    if body:sub(pos, pos + 1) ~= "\r\n" then
      return nil, "invalid chunk terminator"
    end
    pos = pos + 2
  end
end

local function finalize(client, callback, err, response)
  if client and not client:is_closing() then
    client:close()
  end
  vim.schedule(function()
    callback(err, response)
  end)
end

local function parse_response(raw)
  local header_end = raw:find("\r\n\r\n", 1, true)
  if not header_end then
    return nil, "incomplete HTTP response"
  end

  local header = raw:sub(1, header_end - 1)
  local body = raw:sub(header_end + 4)
  local status_line = header:match("([^\r\n]+)")
  if not status_line then
    return nil, "missing status line"
  end

  local status = tonumber(status_line:match("^HTTP/%d%.%d%s+(%d%d%d)"))
  if not status then
    return nil, "invalid status line: " .. status_line
  end

  local headers = parse_headers(header)
  if headers["transfer-encoding"] and headers["transfer-encoding"]:lower():find("chunked", 1, true) then
    local decoded, chunk_err = decode_chunked(body)
    if chunk_err then
      return nil, chunk_err
    end
    body = decoded
  end

  return {
    status = status,
    headers = headers,
    body = body,
  }, nil
end

function M.request(opts, callback)
  local client = uv.new_tcp()
  if not client then
    callback("failed to allocate TCP client")
    return
  end

  local body = opts.body or ""
  local method = opts.method or "GET"
  local headers = {
    ("Host: %s:%d"):format(opts.host, opts.port),
    "Connection: close",
  }
  if body ~= "" then
    table.insert(headers, "Content-Type: application/json")
    table.insert(headers, ("Content-Length: %d"):format(#body))
  end

  local request = table.concat({
    ("%s %s HTTP/1.1"):format(method, opts.path),
    table.concat(headers, "\r\n"),
    "",
    body,
  }, "\r\n")

  local chunks = {}
  local finished = false
  local function done(err, response)
    if finished then
      return
    end
    finished = true
    finalize(client, callback, err, response)
  end

  client:connect(opts.host, opts.port, function(connect_err)
    if connect_err then
      done("connect failed: " .. tostring(connect_err))
      return
    end

    client:read_start(function(read_err, chunk)
      if read_err then
        done("read failed: " .. tostring(read_err))
        return
      end

      if chunk then
        chunks[#chunks + 1] = chunk
        return
      end

      local response, parse_err = parse_response(table.concat(chunks))
      if parse_err then
        done(parse_err)
        return
      end
      done(nil, response)
    end)

    client:write(request, function(write_err)
      if write_err then
        done("write failed: " .. tostring(write_err))
      end
    end)
  end)
end

function M.request_json(opts, callback)
  M.request(opts, function(err, response)
    if err then
      callback(err)
      return
    end

    local ok, payload = pcall(vim.json.decode, response.body)
    if not ok then
      callback("invalid JSON response", response)
      return
    end

    callback(nil, response, payload)
  end)
end

return M
