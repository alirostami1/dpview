const STORAGE = {
  expanded: "dpview.expanded",
  currentPath: "dpview.currentPath",
  search: "dpview.search",
  page: "dpview.page",
  theme: "dpview.theme",
  previewTheme: "dpview.previewTheme",
  sidebarCollapsed: "dpview.sidebarCollapsed",
};

const systemThemeMedia =
  window.matchMedia?.("(prefers-color-scheme: dark)") || null;

const state = {
  files: [],
  tree: [],
  current: null,
  settings: {
    auto_refresh_paused: false,
    sidebar_collapsed: false,
    typst_preview_theme: true,
    theme: "light",
    preview_theme: "default",
  },
  health: null,
  expanded: new Set(JSON.parse(localStorage.getItem(STORAGE.expanded) || "[]")),
  search: localStorage.getItem(STORAGE.search) || "",
  page: localStorage.getItem(STORAGE.page) || "file",
  theme: localStorage.getItem(STORAGE.theme) || "system",
  previewTheme: localStorage.getItem(STORAGE.previewTheme) || "default",
  sidebarCollapsed: false,
  localSelectionInFlight: "",
  statusMessage: "",
};

const appEl = document.querySelector(".app");
const sidebarEl = document.getElementById("sidebar");
const treeEl = document.getElementById("tree");
const previewEl = document.getElementById("preview");
const markdownThemeCSS = document.getElementById("markdown-theme-css");
const statusEl = document.getElementById("status");
const healthEl = document.getElementById("health");
const searchInput = document.getElementById("search");
const pauseRefreshInput = document.getElementById("pause-refresh");
const themeSelect = document.getElementById("theme");
const previewThemeSelect = document.getElementById("preview-theme");
const typstPreviewThemeInput = document.getElementById("typst-preview-theme");
const openSettingsButton = document.getElementById("open-settings");
const closeSettingsButton = document.getElementById("close-settings");
const toggleSidebarButton = document.getElementById("toggle-sidebar");
const showSidebarButton = document.getElementById("show-sidebar");
const fileViewEl = document.getElementById("file-view");
const settingsViewEl = document.getElementById("settings-view");

searchInput.value = state.search;
themeSelect.value = state.theme;
previewThemeSelect.value = state.previewTheme;
applyTheme(state.theme);
applyMarkdownTheme(state.previewTheme);
renderSidebar();

searchInput.addEventListener("input", () => {
  state.search = searchInput.value.trim().toLowerCase();
  localStorage.setItem(STORAGE.search, state.search);
  renderTree();
});

openSettingsButton.addEventListener("click", () => setPage("settings"));
closeSettingsButton.addEventListener("click", () => setPage("file"));
toggleSidebarButton.addEventListener("click", () => setSidebarCollapsed(true));
showSidebarButton.addEventListener("click", () => setSidebarCollapsed(false));

pauseRefreshInput.addEventListener("change", async () => {
  const previous = state.settings.auto_refresh_paused;
  state.settings.auto_refresh_paused = pauseRefreshInput.checked;
  const result = await syncSettings();
  if (!result.ok) {
    state.settings.auto_refresh_paused = previous;
    pauseRefreshInput.checked = previous;
    return;
  }
  setStatus("Settings updated.");
});

themeSelect.addEventListener("change", async () => {
  state.theme = themeSelect.value;
  localStorage.setItem(STORAGE.theme, state.theme);
  applyTheme(state.theme);
  await syncSettings({ rerenderTypst: true });
});

previewThemeSelect.addEventListener("change", async () => {
  state.previewTheme = previewThemeSelect.value;
  localStorage.setItem(STORAGE.previewTheme, state.previewTheme);
  applyMarkdownTheme(state.previewTheme);
  renderPreview();
  await syncSettings({ rerenderTypst: true });
});

typstPreviewThemeInput.addEventListener("change", async () => {
  const previous = state.settings.typst_preview_theme;
  state.settings.typst_preview_theme = typstPreviewThemeInput.checked;
  const result = await syncSettings({ rerenderTypst: true });
  if (!result.ok) {
    state.settings.typst_preview_theme = previous;
    typstPreviewThemeInput.checked = previous;
    return;
  }
  setStatus("Settings updated.");
});

function setStatus(message) {
  state.statusMessage = message || "";
  renderStatus();
}

function renderStatus() {
  statusEl.textContent = state.statusMessage;
}

function applyTheme(theme) {
  document.body.dataset.theme = resolveThemeMode(theme);
}

function resolveThemeMode(theme) {
  return theme === "system"
    ? (systemThemeMedia?.matches ? "dark" : "light")
    : theme;
}

function applyMarkdownTheme(theme) {
  markdownThemeCSS.href = `/themes/markdown/${theme}.css`;
}

function setSidebarCollapsed(collapsed) {
  state.sidebarCollapsed = collapsed;
  state.settings.sidebar_collapsed = collapsed;
  localStorage.setItem(STORAGE.sidebarCollapsed, String(collapsed));
  renderSidebar();
  void syncSettings();
}

function renderSidebar() {
  appEl.classList.toggle("sidebar-collapsed", state.sidebarCollapsed);
  showSidebarButton.classList.toggle("hidden", !state.sidebarCollapsed);
  toggleSidebarButton.textContent = state.sidebarCollapsed ? "Show" : "Hide";
  toggleSidebarButton.setAttribute(
    "aria-label",
    state.sidebarCollapsed ? "Show sidebar" : "Hide sidebar",
  );
  if (!state.sidebarCollapsed) {
    sidebarEl.removeAttribute("aria-hidden");
  } else {
    sidebarEl.setAttribute("aria-hidden", "true");
  }
}

function escapeHTML(value) {
  return (value || "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

if (systemThemeMedia) {
  systemThemeMedia.addEventListener("change", async () => {
    if (state.theme === "system") {
      applyTheme("system");
      await syncSettings({ rerenderTypst: true });
    }
  });
}

async function apiFetch(path, options = {}) {
  const response = await fetch(path, options);
  const payload = await response.json().catch(() => ({
    ok: false,
    error: { message: "Invalid server response" },
  }));
  if (!response.ok || !payload.ok) {
    return { ok: false, error: payload.error || { message: "Request failed" } };
  }
  return { ok: true, data: payload.data };
}

function setPage(page) {
  state.page = page;
  localStorage.setItem(STORAGE.page, page);
  fileViewEl.classList.toggle("hidden", page !== "file");
  settingsViewEl.classList.toggle("hidden", page !== "settings");
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
    treeEl.textContent = "No matching files.";
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
      toggle.className = "tree-button tree-folder";
      toggle.style.paddingLeft = `${0.5 + depth}rem`;
      toggle.textContent = `${open ? "▾" : "▸"} ${node.name}`;
      toggle.addEventListener("click", () => {
        if (state.expanded.has(node.path)) {
          state.expanded.delete(node.path);
        } else {
          state.expanded.add(node.path);
        }
        localStorage.setItem(
          STORAGE.expanded,
          JSON.stringify([...state.expanded]),
        );
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
      row.className = "tree-button";
      row.style.paddingLeft = `${0.5 + depth}rem`;
      row.textContent = node.name;
      row.title = node.path || node.name;
      if (node.path === state.current?.file?.path) {
        row.classList.add("selected");
      }
      row.addEventListener("click", () => setCurrent(node.path));
      wrapper.appendChild(row);
    }

    container.appendChild(wrapper);
  }
}

function renderPreview() {
  const current = state.current;
  const file = current?.file || null;
  const preview = current?.preview || {};
  const previewWrapperClass =
    file?.kind === "markdown"
      ? "preview-content markdown-preview"
      : file?.kind === "typst"
        ? "preview-content typst-preview"
        : "preview-content";

  if (preview.status === "rendering") {
    previewEl.className = "preview";
    previewEl.textContent = `Rendering ${file?.path || "file"}...`;
    return;
  }

  if (preview.error) {
    previewEl.className = "preview";
    previewEl.innerHTML = `
      <div class="${previewWrapperClass}">
        <strong>${escapeHTML(preview.error.message)}</strong>
        ${preview.error.detail ? `<pre>${escapeHTML(preview.error.detail)}</pre>` : ""}
      </div>
    `;
    return;
  }

  if (preview.html) {
    previewEl.className = "preview";
    previewEl.innerHTML = `<div class="${previewWrapperClass}">${preview.html}</div>`;
    return;
  }

  previewEl.className = "preview empty";
  previewEl.textContent = file ? "No preview available." : "No file selected.";
}

function renderHealth() {
  const health = state.health;
  if (!health) {
    healthEl.innerHTML = "";
    return;
  }

  const items = [];
  items.push(`<div class="status-item">Status: ${escapeHTML(health.status || "unknown")}</div>`);
  items.push(`<div class="status-item">Watcher: ${health.watcher?.enabled ? "enabled" : "disabled"}</div>`);

  for (const renderer of health.renderers || []) {
    const value = renderer.available ? "available" : "unavailable";
    items.push(
      `<div class="status-item">${escapeHTML(renderer.name || renderer.kind)}: ${value}</div>`,
    );
  }

  healthEl.innerHTML = items.join("");
}

function applyFiles(data) {
  state.files = data.files || [];
  state.tree = data.tree || [];
  renderTree();
  setStatus(
    `${state.files.length} file${state.files.length === 1 ? "" : "s"} indexed`,
  );
}

function applyCurrent(data) {
  state.current = data;
  const path = data.file?.path || "";
  localStorage.setItem(STORAGE.currentPath, path);
  syncQueryPath(path);
  renderTree();
  renderPreview();
  if (state.localSelectionInFlight && state.localSelectionInFlight !== path) {
    setStatus(`Switched externally to ${path || "no file"}.`);
  } else if (!state.statusMessage || state.localSelectionInFlight) {
    setStatus(path ? `Selected ${path}` : "");
  }
  state.localSelectionInFlight = "";
}

function applySettings(data) {
  const settings = data.settings || {};
  const storedTheme = localStorage.getItem(STORAGE.theme);
  const storedPreviewTheme = localStorage.getItem(STORAGE.previewTheme);
  state.settings = {
    auto_refresh_paused: !!settings.auto_refresh_paused,
    sidebar_collapsed: !!settings.sidebar_collapsed,
    typst_preview_theme: settings.typst_preview_theme !== false,
    theme: settings.theme || "light",
    preview_theme: settings.preview_theme || "default",
  };
  state.theme = storedTheme || state.theme || "system";
  state.previewTheme = storedPreviewTheme || state.settings.preview_theme;
  state.sidebarCollapsed = state.settings.sidebar_collapsed;
  themeSelect.value = state.theme;
  previewThemeSelect.value = state.previewTheme;
  applyTheme(state.theme);
  applyMarkdownTheme(state.previewTheme);
  localStorage.setItem(STORAGE.sidebarCollapsed, String(state.sidebarCollapsed));
  renderSidebar();
  pauseRefreshInput.checked = !!state.settings.auto_refresh_paused;
  typstPreviewThemeInput.checked = !!state.settings.typst_preview_theme;
}

function currentSettingsPayload() {
  return {
    auto_refresh_paused: !!state.settings.auto_refresh_paused,
    sidebar_collapsed: !!state.sidebarCollapsed,
    typst_preview_theme: !!state.settings.typst_preview_theme,
    theme: resolveThemeMode(state.theme),
    preview_theme: state.previewTheme,
  };
}

async function refreshCurrentPreview() {
  const result = await apiFetch("/api/refresh", { method: "POST" });
  if (!result.ok) {
    setStatus(result.error.message);
    return result;
  }
  applyCurrent(result.data);
  return result;
}

async function syncSettings(options = {}) {
  const result = await apiFetch("/api/settings", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(currentSettingsPayload()),
  });
  if (!result.ok) {
    setStatus(result.error.message);
    return result;
  }
  applySettings(result.data);
  if (options.rerenderTypst && state.current?.file?.kind === "typst") {
    return refreshCurrentPreview();
  }
  return result;
}

function applyHealth(data) {
  state.health = data;
  renderHealth();
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
  setPage("file");
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
  setPage(state.page);

  const initialSettings = settings.data.settings || {};
  if (
    resolveThemeMode(state.theme) !== (initialSettings.theme || "light") ||
    state.previewTheme !== (initialSettings.preview_theme || "default") ||
    !!state.settings.typst_preview_theme !== (initialSettings.typst_preview_theme !== false)
  ) {
    await syncSettings({ rerenderTypst: current.data.file?.kind === "typst" });
  }

  const deeplinkPath = new URL(window.location.href).searchParams.get("path");
  const storedPath = localStorage.getItem(STORAGE.currentPath);
  const preferredPath =
    deeplinkPath || (!current.data.current && storedPath ? storedPath : "");
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
    setStatus(`Rendering ${state.current?.file?.path || "file"}...`);
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
    setPage("settings");
  });
