const STORAGE = {
  expanded: "dpview.expanded",
  currentPath: "dpview.currentPath",
  showSource: "dpview.showSource",
  sidebarWidth: "dpview.sidebarWidth",
  search: "dpview.search",
  sidebarCollapsed: "dpview.sidebarCollapsed",
};

const state = {
  files: [],
  tree: [],
  recent: [],
  current: null,
  settings: { auto_refresh_paused: false },
  health: null,
  expanded: new Set(JSON.parse(localStorage.getItem(STORAGE.expanded) || "[]")),
  showSource: localStorage.getItem(STORAGE.showSource) === "true",
  search: localStorage.getItem(STORAGE.search) || "",
  sidebarWidth: Number(localStorage.getItem(STORAGE.sidebarWidth) || 320),
  sidebarCollapsed: localStorage.getItem(STORAGE.sidebarCollapsed) === "true",
  localSelectionInFlight: "",
};

const appEl = document.getElementById("app");
const sidebarEl = document.getElementById("sidebar");
const treeEl = document.getElementById("tree");
const recentEl = document.getElementById("recent");
const previewEl = document.getElementById("preview");
const sourcePaneEl = document.getElementById("source-pane");
const sourceEl = document.getElementById("source");
const currentPathEl = document.getElementById("current-path");
const currentMetaEl = document.getElementById("current-meta");
const previewStateEl = document.getElementById("preview-state");
const statusEl = document.getElementById("status");
const healthEl = document.getElementById("health");
const refreshButton = document.getElementById("refresh");
const railRefreshButton = document.getElementById("rail-refresh");
const clearButton = document.getElementById("clear-current");
const searchInput = document.getElementById("search");
const pauseRefreshInput = document.getElementById("pause-refresh");
const showSourceInput = document.getElementById("show-source");
const railSourceButton = document.getElementById("rail-source");
const dividerEl = document.getElementById("divider");
const panesEl = document.getElementById("panes");
const sidebarToggleButton = document.getElementById("sidebar-toggle");
const sidebarOpenButton = document.getElementById("sidebar-open");
const railCurrentEl = document.getElementById("rail-current");

document.documentElement.style.setProperty("--sidebar-width", `${state.sidebarWidth}px`);
searchInput.value = state.search;
showSourceInput.checked = state.showSource;

refreshButton.addEventListener("click", refreshCurrent);
railRefreshButton.addEventListener("click", refreshCurrent);

clearButton.addEventListener("click", async () => {
  const result = await apiFetch("/api/current", { method: "DELETE" });
  if (!result.ok) {
    setStatus(result.error.message);
    return;
  }
  applyCurrent(result.data);
});

searchInput.addEventListener("input", () => {
  state.search = searchInput.value.trim().toLowerCase();
  localStorage.setItem(STORAGE.search, state.search);
  renderTree();
});

pauseRefreshInput.addEventListener("change", async () => {
  const result = await apiFetch("/api/settings", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ auto_refresh_paused: pauseRefreshInput.checked }),
  });
  if (!result.ok) {
    setStatus(result.error.message);
    pauseRefreshInput.checked = state.settings.auto_refresh_paused;
    return;
  }
  applySettings(result.data);
});

showSourceInput.addEventListener("change", () => {
  state.showSource = showSourceInput.checked;
  localStorage.setItem(STORAGE.showSource, String(state.showSource));
  renderSourceVisibility();
});

railSourceButton.addEventListener("click", () => {
  state.showSource = !state.showSource;
  localStorage.setItem(STORAGE.showSource, String(state.showSource));
  renderSourceVisibility();
});

sidebarToggleButton.addEventListener("click", () => setSidebarCollapsed(true));
sidebarOpenButton.addEventListener("click", () => setSidebarCollapsed(false));

setupResize();

function setStatus(message) {
  statusEl.textContent = message || "";
}

function escapeHTML(value) {
  return (value || "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

async function apiFetch(path, options = {}) {
  const response = await fetch(path, options);
  const payload = await response.json().catch(() => ({ ok: false, error: { message: "Invalid server response" } }));
  if (!response.ok || !payload.ok) {
    return { ok: false, error: payload.error || { message: "Request failed" } };
  }
  return { ok: true, data: payload.data };
}

async function refreshCurrent() {
  const result = await apiFetch("/api/refresh", { method: "POST" });
  if (!result.ok) {
    setStatus(result.error.message);
  }
}

function setSidebarCollapsed(collapsed) {
  state.sidebarCollapsed = collapsed;
  localStorage.setItem(STORAGE.sidebarCollapsed, String(collapsed));
  appEl.classList.toggle("is-collapsed", collapsed);
}

function renderRecent() {
  recentEl.innerHTML = "";
  if (!state.recent.length) {
    recentEl.innerHTML = `<div class="status">No recent files yet.</div>`;
    return;
  }
  for (const item of state.recent) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "recent-item";
    if (state.current?.file?.path === item.path) {
      button.classList.add("is-current");
    }
    button.textContent = item.path;
    button.title = item.path;
    button.addEventListener("click", () => setCurrent(item.path));
    recentEl.appendChild(button);
  }
}

function filteredNodes(nodes) {
  if (!state.search) {
    return nodes;
  }
  const matchNode = (node) => {
    const haystack = `${node.name} ${node.path || ""}`.toLowerCase();
    const isMatch = haystack.includes(state.search);
    if (node.children?.length) {
      const children = node.children.map(matchNode).filter(Boolean);
      if (isMatch || children.length) {
        return { ...node, children };
      }
      return null;
    }
    return isMatch ? node : null;
  };
  return nodes.map(matchNode).filter(Boolean);
}

function renderTree() {
  treeEl.innerHTML = "";
  const nodes = filteredNodes(state.tree);
  if (!nodes.length) {
    treeEl.innerHTML = `<div class="status">No matching previewable files.</div>`;
    return;
  }
  renderTreeNodes(nodes, treeEl, 0);
}

function renderTreeNodes(nodes, container, depth) {
  for (const node of nodes) {
    const wrapper = document.createElement("div");
    wrapper.className = "tree-node";
    if (node.children?.length) {
      const open = state.expanded.has(node.path) || Boolean(state.search);
      const toggle = document.createElement("button");
      toggle.type = "button";
      toggle.className = "tree-folder-toggle";
      toggle.setAttribute("aria-expanded", String(open));
      toggle.classList.toggle("is-folder-open", open);
      toggle.style.paddingLeft = `${10 + depth * 16}px`;
      toggle.innerHTML = `
        <span class="tree-chevron" aria-hidden="true">›</span>
        <span class="tree-label">${escapeHTML(node.name)}</span>
      `;
      toggle.addEventListener("click", () => {
        if (state.expanded.has(node.path)) {
          state.expanded.delete(node.path);
        } else {
          state.expanded.add(node.path);
        }
        localStorage.setItem(STORAGE.expanded, JSON.stringify([...state.expanded]));
        renderTree();
      });
      wrapper.appendChild(toggle);

      if (open) {
        const children = document.createElement("div");
        children.className = "tree-children";
        renderTreeNodes(node.children, children, depth + 1);
        wrapper.appendChild(children);
      }
    } else {
      const row = document.createElement("button");
      row.type = "button";
      row.className = "tree-row tree-file";
      row.style.paddingLeft = `${10 + depth * 16}px`;
      if (node.path === state.current?.file?.path) {
        row.classList.add("is-current");
      }
      const icon = node.kind === "typst" ? "◇" : "·";
      row.innerHTML = `
        <span class="tree-icon" aria-hidden="true">${icon}</span>
        <span class="tree-label">${escapeHTML(node.name)}</span>
      `;
      row.title = node.path || node.name;
      row.addEventListener("click", () => setCurrent(node.path));
      wrapper.appendChild(row);
    }
    container.appendChild(wrapper);
  }
}

function renderSourceVisibility() {
  showSourceInput.checked = state.showSource;
  if (state.showSource) {
    sourcePaneEl.classList.remove("hidden");
    panesEl.classList.add("with-source");
  } else {
    sourcePaneEl.classList.add("hidden");
    panesEl.classList.remove("with-source");
  }
}

function formatMeta(file, preview) {
  if (!file) {
    return "";
  }
  const parts = [];
  parts.push(file.kind);
  if (preview?.render_duration_ms) {
    parts.push(`${preview.render_duration_ms} ms`);
  }
  if (preview?.cache_hit) {
    parts.push("cache");
  }
  if (file.mtime) {
    parts.push(new Date(file.mtime).toLocaleString());
  }
  return parts.join(" · ");
}

function renderPreview() {
  const current = state.current;
  const file = current?.file || null;
  const preview = current?.preview || {};
  currentPathEl.textContent = file?.path || "No file selected";
  currentMetaEl.textContent = formatMeta(file, preview);
  previewStateEl.textContent = preview.status || "idle";
  sourceEl.textContent = preview.source || "";
  railCurrentEl.classList.toggle("has-current", Boolean(file));
  railCurrentEl.title = file?.path || "No file selected";

  if (preview.status === "rendering") {
    previewEl.className = "preview-shell preview-loading";
    previewEl.textContent = `Rendering ${file?.path || "preview"}...`;
    return;
  }
  if (preview.error) {
    previewEl.className = "preview-shell";
    previewEl.innerHTML = `
      <div class="preview-error">
        <strong>${escapeHTML(preview.error.message)}</strong>
        ${preview.error.detail ? `<pre>${escapeHTML(preview.error.detail)}</pre>` : ""}
      </div>
    `;
    return;
  }
  if (preview.html) {
    previewEl.className = "preview-shell";
    previewEl.innerHTML = preview.html;
    return;
  }
  previewEl.className = "preview-shell empty";
  previewEl.textContent = "Select a Markdown or Typst file to preview it.";
}

function applyFiles(data) {
  state.files = data.files || [];
  state.tree = data.tree || [];
  state.recent = data.recent || [];
  renderRecent();
  renderTree();
  setStatus(`${state.files.length} file${state.files.length === 1 ? "" : "s"} indexed`);
}

function applyCurrent(data) {
  state.current = data;
  const path = data.file?.path || "";
  localStorage.setItem(STORAGE.currentPath, path);
  syncQueryPath(path);
  renderRecent();
  renderTree();
  renderPreview();
  if (state.localSelectionInFlight && state.localSelectionInFlight !== path) {
    setStatus(`Switched externally to ${path || "no file"}.`);
  } else if (!path) {
    setStatus("");
  }
  state.localSelectionInFlight = "";
}

function applySettings(data) {
  state.settings = data.settings || { auto_refresh_paused: false };
  pauseRefreshInput.checked = !!state.settings.auto_refresh_paused;
}

function applyHealth(data) {
  state.health = data;
  const typst = (data.renderers || []).find((renderer) => renderer.kind === "typst");
  if (typst?.available) {
    healthEl.textContent = "Typst ready";
    return;
  }
  healthEl.textContent = "Markdown only";
}

async function setCurrent(path) {
  state.localSelectionInFlight = path;
  setStatus(`Loading ${path}...`);
  const result = await apiFetch("/api/current", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ path }),
  });
  if (!result.ok) {
    setStatus(result.error.message);
    state.localSelectionInFlight = "";
    return;
  }
  applyCurrent(result.data);
}

function syncQueryPath(path) {
  const url = new URL(window.location.href);
  if (path) {
    url.searchParams.set("path", path);
  } else {
    url.searchParams.delete("path");
  }
  window.history.replaceState({}, "", url);
}

function setupResize() {
  dividerEl.addEventListener("pointerdown", (event) => {
    if (state.sidebarCollapsed) {
      return;
    }
    event.preventDefault();
    const onMove = (moveEvent) => {
      const width = Math.max(260, Math.min(440, moveEvent.clientX));
      state.sidebarWidth = width;
      localStorage.setItem(STORAGE.sidebarWidth, String(width));
      document.documentElement.style.setProperty("--sidebar-width", `${width}px`);
    };
    const onUp = () => {
      window.removeEventListener("pointermove", onMove);
      window.removeEventListener("pointerup", onUp);
    };
    window.addEventListener("pointermove", onMove);
    window.addEventListener("pointerup", onUp);
  });
}

async function loadInitialState() {
  const [health, files, current, settings] = await Promise.all([
    apiFetch("/api/health"),
    apiFetch("/api/files"),
    apiFetch("/api/current"),
    apiFetch("/api/settings"),
  ]);
  if (!health.ok) throw new Error(health.error.message);
  if (!files.ok) throw new Error(files.error.message);
  if (!current.ok) throw new Error(current.error.message);
  if (!settings.ok) throw new Error(settings.error.message);

  applyHealth(health.data);
  applyFiles(files.data);
  applyCurrent(current.data);
  applySettings(settings.data);
  renderSourceVisibility();
  setSidebarCollapsed(state.sidebarCollapsed);

  const deeplinkPath = new URL(window.location.href).searchParams.get("path");
  const storedPath = localStorage.getItem(STORAGE.currentPath);
  const preferredPath = deeplinkPath || (!current.data.current && storedPath ? storedPath : "");
  if (preferredPath && preferredPath !== current.data.file?.path) {
    await setCurrent(preferredPath);
  }
}

function connectEvents() {
  const events = new EventSource("/events");
  const parseEvent = (event) => JSON.parse(event.data).data;

  events.addEventListener("files_changed", (event) => {
    applyFiles(parseEvent(event));
  });
  events.addEventListener("current_changed", (event) => {
    applyCurrent(parseEvent(event));
  });
  events.addEventListener("preview_updated", (event) => {
    applyCurrent(parseEvent(event));
  });
  events.addEventListener("render_started", (event) => {
    state.current = parseEvent(event);
    renderPreview();
    setStatus(`Rendering ${state.current?.file?.path || "preview"}...`);
  });
  events.addEventListener("render_failed", (event) => {
    applyCurrent(parseEvent(event));
  });
  events.addEventListener("settings_changed", (event) => {
    applySettings(parseEvent(event));
  });
  events.onerror = () => {
    setStatus("Event stream disconnected. Retrying...");
  };
}

loadInitialState()
  .then(connectEvents)
  .catch((error) => {
    setStatus(error.message || "Failed to load application state");
  });
