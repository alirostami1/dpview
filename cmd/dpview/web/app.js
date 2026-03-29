const STORAGE = {
  expanded: "dpview.expanded",
  currentPath: "dpview.currentPath",
  search: "dpview.search",
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
    editor_file_sync_enabled: true,
    seek_enabled: true,
    typst_preview_theme: true,
    markdown_frontmatter_visible: true,
    markdown_frontmatter_expanded: true,
    markdown_frontmatter_title: true,
    theme: "light",
    preview_theme: "default",
  },
  health: null,
  seek: null,
  expanded: new Set(JSON.parse(localStorage.getItem(STORAGE.expanded) || "[]")),
  search: localStorage.getItem(STORAGE.search) || "",
  theme: localStorage.getItem(STORAGE.theme) || "system",
  previewTheme: localStorage.getItem(STORAGE.previewTheme) || "default",
  sidebarCollapsed: false,
  frontMatterExpanded: null,
  localSelectionInFlight: "",
  statusMessage: "",
  pendingSeekFrame: 0,
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
const editorFileSyncInput = document.getElementById("editor-file-sync");
const seekEnabledInput = document.getElementById("seek-enabled");
const markdownFrontMatterVisibleInput = document.getElementById("markdown-frontmatter-visible");
const markdownFrontMatterExpandedInput = document.getElementById("markdown-frontmatter-expanded");
const markdownFrontMatterTitleInput = document.getElementById("markdown-frontmatter-title");
const openSettingsButton = document.getElementById("open-settings");
const closeSettingsButton = document.getElementById("close-settings");
const toggleSidebarButton = document.getElementById("toggle-sidebar");
const showSidebarButton = document.getElementById("show-sidebar");
const goHomeButton = document.getElementById("go-home");
const notFoundMessageEl = document.getElementById("not-found-message");
const fileViewEl = document.getElementById("file-view");
const settingsViewEl = document.getElementById("settings-view");
const notFoundViewEl = document.getElementById("not-found-view");

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

openSettingsButton.addEventListener("click", () => {
  if (isSettingsRoute()) {
    navigateToCurrentFile();
    return;
  }
  navigateToSettings();
});
closeSettingsButton.addEventListener("click", () => navigateToCurrentFile());
goHomeButton.addEventListener("click", () => navigateToCurrentFile());
toggleSidebarButton.addEventListener("click", () => setSidebarCollapsed(true));
showSidebarButton.addEventListener("click", () => setSidebarCollapsed(false));
window.addEventListener("popstate", () => {
  applyRoute(parseRoute(window.location.pathname, window.location.search));
});

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

editorFileSyncInput.addEventListener("change", async () => {
  const previous = state.settings.editor_file_sync_enabled;
  state.settings.editor_file_sync_enabled = editorFileSyncInput.checked;
  const result = await syncSettings();
  if (!result.ok) {
    state.settings.editor_file_sync_enabled = previous;
    editorFileSyncInput.checked = previous;
    return;
  }
  setStatus("Settings updated.");
});

seekEnabledInput.addEventListener("change", async () => {
  const previous = state.settings.seek_enabled;
  state.settings.seek_enabled = seekEnabledInput.checked;
  const result = await syncSettings();
  if (!result.ok) {
    state.settings.seek_enabled = previous;
    seekEnabledInput.checked = previous;
    return;
  }
  if (!state.settings.seek_enabled) {
    state.seek = null;
  } else {
    queueApplySeek();
  }
  setStatus("Settings updated.");
});

markdownFrontMatterVisibleInput.addEventListener("change", async () => {
  const previous = state.settings.markdown_frontmatter_visible;
  state.settings.markdown_frontmatter_visible = markdownFrontMatterVisibleInput.checked;
  const result = await syncSettings();
  if (!result.ok) {
    state.settings.markdown_frontmatter_visible = previous;
    markdownFrontMatterVisibleInput.checked = previous;
    return;
  }
  renderPreview();
  setStatus("Settings updated.");
});

markdownFrontMatterExpandedInput.addEventListener("change", async () => {
  const previous = state.settings.markdown_frontmatter_expanded;
  state.settings.markdown_frontmatter_expanded = markdownFrontMatterExpandedInput.checked;
  const result = await syncSettings();
  if (!result.ok) {
    state.settings.markdown_frontmatter_expanded = previous;
    markdownFrontMatterExpandedInput.checked = previous;
    return;
  }
  renderPreview();
  setStatus("Settings updated.");
});

markdownFrontMatterTitleInput.addEventListener("change", async () => {
  const previous = state.settings.markdown_frontmatter_title;
  state.settings.markdown_frontmatter_title = markdownFrontMatterTitleInput.checked;
  const result = await syncSettings({ rerenderMarkdown: true });
  if (!result.ok) {
    state.settings.markdown_frontmatter_title = previous;
    markdownFrontMatterTitleInput.checked = previous;
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
  fileViewEl.classList.toggle("hidden", page !== "file");
  settingsViewEl.classList.toggle("hidden", page !== "settings");
  notFoundViewEl.classList.toggle("hidden", page !== "not-found");
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
  rememberFrontMatterState();
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
    const frontMatterHTML =
      file?.kind === "markdown" && state.settings.markdown_frontmatter_visible
        ? renderFrontMatter(preview.frontmatter)
        : "";
    previewEl.innerHTML = `<div class="${previewWrapperClass}">${frontMatterHTML}${preview.html}</div>`;
    bindFrontMatterState();
    if (file?.kind === "markdown") {
      renderMarkdownMath(previewEl.querySelector(".markdown-preview"));
    }
    queueApplySeek();
    return;
  }

  previewEl.className = "preview empty";
  previewEl.textContent = file ? "No preview available." : "No file selected.";
}

function renderMarkdownMath(container) {
  if (!container) {
    return;
  }
  if (typeof katex?.render === "function") {
    for (const node of container.querySelectorAll(".markdown-math-block")) {
      katex.render(node.dataset.latex || "", node, {
        displayMode: true,
        throwOnError: false,
      });
    }
  }
  if (typeof renderMathInElement !== "function") {
    return;
  }
  renderMathInElement(container, {
    delimiters: [
      { left: "$$", right: "$$", display: true },
      { left: "$", right: "$", display: false },
      { left: "\\(", right: "\\)", display: false },
      { left: "\\[", right: "\\]", display: true },
    ],
    throwOnError: false,
  });
}

function renderFrontMatter(frontMatter) {
  if (!frontMatter?.entries?.length) {
    return "";
  }
  const open = (state.frontMatterExpanded ?? state.settings.markdown_frontmatter_expanded) ? " open" : "";
  const rows = frontMatter.entries.map((entry) => `
    <tr>
      <th>${escapeHTML(entry.key)}</th>
      <td><code>${escapeHTML(entry.value)}</code></td>
    </tr>
  `).join("");
  return `
    <details class="frontmatter-panel"${open}>
      <summary>
        Front matter
        <span class="frontmatter-meta">${escapeHTML(frontMatter.format || "yaml")}</span>
      </summary>
      <div class="frontmatter-table-wrap">
        <table class="frontmatter-table">
          <tbody>${rows}</tbody>
        </table>
      </div>
    </details>
  `;
}

function rememberFrontMatterState() {
  const panel = previewEl.querySelector(".frontmatter-panel");
  if (panel) {
    state.frontMatterExpanded = panel.open;
  }
}

function bindFrontMatterState() {
  const panel = previewEl.querySelector(".frontmatter-panel");
  if (!panel) {
    return;
  }
  state.frontMatterExpanded = panel.open;
  panel.addEventListener("toggle", () => {
    state.frontMatterExpanded = panel.open;
  });
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
  const previousPath = state.current?.file?.path || "";
  state.current = data;
  const path = data.file?.path || "";
  if (path !== previousPath) {
    state.frontMatterExpanded = null;
  }
  localStorage.setItem(STORAGE.currentPath, path);
  syncLocationPath(path);
  renderTree();
  renderPreview();
  if (state.localSelectionInFlight && state.localSelectionInFlight !== path) {
    setStatus(`Switched externally to ${path || "no file"}.`);
  } else if (!state.statusMessage || state.localSelectionInFlight) {
    setStatus(path ? `Selected ${path}` : "");
  }
  state.localSelectionInFlight = "";
}

function applySeek(data) {
  state.seek = data || null;
  if (!state.settings.seek_enabled) {
    return;
  }
  queueApplySeek();
}

function applySettings(data) {
  const settings = data.settings || {};
  const storedTheme = localStorage.getItem(STORAGE.theme);
  const storedPreviewTheme = localStorage.getItem(STORAGE.previewTheme);
  state.settings = {
    auto_refresh_paused: !!settings.auto_refresh_paused,
    sidebar_collapsed: !!settings.sidebar_collapsed,
    editor_file_sync_enabled: settings.editor_file_sync_enabled !== false,
    seek_enabled: settings.seek_enabled !== false,
    typst_preview_theme: settings.typst_preview_theme !== false,
    markdown_frontmatter_visible: settings.markdown_frontmatter_visible !== false,
    markdown_frontmatter_expanded: settings.markdown_frontmatter_expanded !== false,
    markdown_frontmatter_title: settings.markdown_frontmatter_title !== false,
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
  editorFileSyncInput.checked = !!state.settings.editor_file_sync_enabled;
  seekEnabledInput.checked = !!state.settings.seek_enabled;
  typstPreviewThemeInput.checked = !!state.settings.typst_preview_theme;
  markdownFrontMatterVisibleInput.checked = !!state.settings.markdown_frontmatter_visible;
  markdownFrontMatterExpandedInput.checked = !!state.settings.markdown_frontmatter_expanded;
  markdownFrontMatterTitleInput.checked = !!state.settings.markdown_frontmatter_title;
  if (!state.settings.seek_enabled) {
    state.seek = null;
  } else {
    queueApplySeek();
  }
  if (state.current?.file) {
    renderPreview();
  }
}

function currentSettingsPayload() {
  return {
    auto_refresh_paused: !!state.settings.auto_refresh_paused,
    sidebar_collapsed: !!state.sidebarCollapsed,
    editor_file_sync_enabled: !!state.settings.editor_file_sync_enabled,
    seek_enabled: !!state.settings.seek_enabled,
    typst_preview_theme: !!state.settings.typst_preview_theme,
    markdown_frontmatter_visible: !!state.settings.markdown_frontmatter_visible,
    markdown_frontmatter_expanded: !!state.settings.markdown_frontmatter_expanded,
    markdown_frontmatter_title: !!state.settings.markdown_frontmatter_title,
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
  if (options.rerenderMarkdown && state.current?.file?.kind === "markdown") {
    return refreshCurrentPreview();
  }
  return result;
}

function applyHealth(data) {
  state.health = data;
  renderHealth();
}

function queueApplySeek() {
  if (state.pendingSeekFrame) {
    cancelAnimationFrame(state.pendingSeekFrame);
  }
  state.pendingSeekFrame = requestAnimationFrame(() => {
    state.pendingSeekFrame = 0;
    applySeekToPreview();
  });
}

function applySeekToPreview() {
  const seek = state.seek;
  const file = state.current?.file;
  const preview = state.current?.preview;
  if (!state.settings.seek_enabled || !seek || !file || seek.path !== file.path || preview?.status !== "ready") {
    return;
  }

  if (file.kind === "markdown") {
    applyMarkdownSeek(seek);
    return;
  }
  if (file.kind === "typst") {
    applyTypstSeek(seek, preview);
  }
}

function applyMarkdownSeek(seek) {
  const focusLine = seek.focus_line || seek.line || seek.top_line || 0;
  if (!focusLine) {
    return;
  }
  const candidates = collectMarkdownSeekCandidates();
  if (!candidates.length) {
    return;
  }

  const containing = candidates.filter((candidate) => (
    focusLine >= candidate.start && focusLine <= candidate.end
  ));

  if (containing.length) {
    const target = containing.reduce((best, candidate) => {
      if (!best) {
        return candidate;
      }
      const bestSpan = best.end - best.start;
      const candidateSpan = candidate.end - candidate.start;
      if (candidateSpan !== bestSpan) {
        return candidateSpan < bestSpan ? candidate : best;
      }
      return candidate.depth > best.depth ? candidate : best;
    }, null);
    scrollPreviewToLine(target, focusLine);
    return;
  }

  const before = candidates
    .filter((candidate) => candidate.end < focusLine)
    .reduce((best, candidate) => (!best || candidate.end > best.end ? candidate : best), null);
  const after = candidates
    .filter((candidate) => candidate.start > focusLine)
    .reduce((best, candidate) => (!best || candidate.start < best.start ? candidate : best), null);

  if (before && after) {
    scrollPreviewBetweenCandidates(before, after, focusLine);
    return;
  }

  scrollPreviewToLine(before || after, focusLine);
}

function applyTypstSeek(seek, preview) {
  const focusLine = seek.focus_line || seek.line || seek.top_line || 0;
  const totalLines = Number(preview?.source_line_count || 0);
  if (!focusLine || !totalLines) {
    return;
  }
  const container = fileViewEl;
  const scrollRange = container.scrollHeight - container.clientHeight;
  if (scrollRange <= 0) {
    return;
  }
  const progress = totalLines <= 1 ? 0 : Math.max(0, Math.min(1, (focusLine - 1) / (totalLines - 1)));
  container.scrollTo({ top: progress * scrollRange, behavior: "auto" });
}

function collectMarkdownSeekCandidates() {
  return [...previewEl.querySelectorAll("[data-source-start-line][data-source-end-line]")]
    .map((node) => {
      const start = Number(node.dataset.sourceStartLine || 0);
      const end = Number(node.dataset.sourceEndLine || start);
      if (!start || !end) {
        return null;
      }
      return {
        node,
        start,
        end,
        depth: countNodeDepth(node),
      };
    })
    .filter(Boolean);
}

function countNodeDepth(node) {
  let depth = 0;
  for (let current = node.parentElement; current; current = current.parentElement) {
    depth += 1;
  }
  return depth;
}

function scrollPreviewToLine(candidate, line) {
  if (!candidate) {
    return;
  }
  const container = fileViewEl;
  const containerRect = container.getBoundingClientRect();
  const nodeRect = candidate.node.getBoundingClientRect();
  const span = Math.max(1, candidate.end - candidate.start);
  const progress = Math.max(0, Math.min(1, (line - candidate.start) / span));
  const targetPoint = nodeRect.top + (nodeRect.height * progress);
  const targetTop = container.scrollTop + (targetPoint - containerRect.top) - (container.clientHeight * 0.32);
  const maxTop = Math.max(0, container.scrollHeight - container.clientHeight);
  container.scrollTo({
    top: Math.max(0, Math.min(maxTop, targetTop)),
    behavior: "auto",
  });
}

function scrollPreviewBetweenCandidates(before, after, line) {
  const container = fileViewEl;
  const containerRect = container.getBoundingClientRect();
  const beforeRect = before.node.getBoundingClientRect();
  const afterRect = after.node.getBoundingClientRect();
  const lineSpan = Math.max(1, after.start - before.end);
  const progress = Math.max(0, Math.min(1, (line - before.end) / lineSpan));
  const beforePoint = beforeRect.top + beforeRect.height;
  const afterPoint = afterRect.top;
  const targetPoint = beforePoint + ((afterPoint - beforePoint) * progress);
  const targetTop = container.scrollTop + (targetPoint - containerRect.top) - (container.clientHeight * 0.32);
  const maxTop = Math.max(0, container.scrollHeight - container.clientHeight);
  container.scrollTo({
    top: Math.max(0, Math.min(maxTop, targetTop)),
    behavior: "auto",
  });
}

function encodeAppPath(path) {
  if (!path) {
    return "/";
  }
  return `/${path.split("/").map((part) => encodeURIComponent(part)).join("/")}`;
}

function locationDeepLinkPath() {
  const url = new URL(window.location.href);
  if (url.pathname === "/") {
    return "";
  }
  return url.pathname
    .split("/")
    .filter(Boolean)
    .map((part) => decodeURIComponent(part))
    .join("/");
}

async function setCurrent(path) {
  state.localSelectionInFlight = path;
  setStatus(`Loading ${path}...`);
  const result = await apiFetch("/api/current", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ path, origin: "web" }),
  });
  if (!result.ok) {
    setStatus(result.error.message);
    state.localSelectionInFlight = "";
    return;
  }
  applyCurrent(result.data);
  navigateToFilePath(path, { replace: true });
}

function syncLocationPath(path) {
  syncCurrentPath(path);
}

async function loadInitialState() {
  const initialRoute = parseRoute(window.location.pathname, window.location.search);
  const [health, files, current, seek, settings] = await Promise.all([
    apiFetch("/api/health"),
    apiFetch("/api/files"),
    apiFetch("/api/current"),
    apiFetch("/api/seek"),
    apiFetch("/api/settings"),
  ]);
  if (!health.ok) throw new Error(health.error.message);
  if (!files.ok) throw new Error(files.error.message);
  if (!current.ok) throw new Error(current.error.message);
  if (!seek.ok) throw new Error(seek.error.message);
  if (!settings.ok) throw new Error(settings.error.message);

  applyHealth(health.data);
  applyFiles(files.data);
  applyCurrent(current.data);
  applySeek(seek.data);
  applySettings(settings.data);
  applyRoute(initialRoute);

  const initialSettings = settings.data.settings || {};
  if (
    resolveThemeMode(state.theme) !== (initialSettings.theme || "light") ||
    state.previewTheme !== (initialSettings.preview_theme || "default") ||
    !!state.settings.typst_preview_theme !== (initialSettings.typst_preview_theme !== false)
  ) {
    await syncSettings({ rerenderTypst: current.data.file?.kind === "typst" });
  }

  const storedPath = localStorage.getItem(STORAGE.currentPath);
  const routedPath = initialRoute.kind === "file" ? initialRoute.path : "";
  const preferredPath =
    routedPath || (!current.data.current && storedPath ? storedPath : "");
  if (preferredPath && preferredPath !== current.data.file?.path && fileExists(preferredPath)) {
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
  events.addEventListener("seek_changed", (event) => {
    applySeek(parseEvent(event));
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
    applyRoute({ kind: "settings" });
  });

function fileExists(path) {
  return state.files.some((file) => file.path === path);
}

function parseRoute(pathname, search = "") {
  const params = new URLSearchParams(search);
  const settingsOpen = params.get("settings") === "open";
  const decodedPath = pathname
    .split("/")
    .filter(Boolean)
    .map((part) => decodeURIComponent(part));

  if (settingsOpen) {
    return { kind: "settings" };
  }
  if (decodedPath.length === 0) {
    return { kind: "file", path: "" };
  }
  const filePath = decodedPath.join("/");
  return { kind: "file", path: filePath };
}

function isSettingsRoute() {
  return parseRoute(window.location.pathname, window.location.search).kind === "settings";
}

function applyRoute(route) {
  if (route.kind === "settings") {
    setPage("settings");
    return;
  }
  if (route.kind === "file") {
    if (!route.path || fileExists(route.path) || state.files.length === 0) {
      setPage("file");
      if (route.path && state.current?.file?.path !== route.path && fileExists(route.path)) {
        void setCurrent(route.path);
      }
      return;
    }
    showNotFound(`No previewable file exists at "${route.path}".`);
    return;
  }
  showNotFound("That route does not exist in DPview.");
}

function showNotFound(message) {
  notFoundMessageEl.textContent = message;
  setPage("not-found");
}

function navigateToSettings(options = {}) {
  const { replace = false } = options;
  const url = new URL(window.location.href);
  url.searchParams.set("settings", "open");
  window.history[replace ? "replaceState" : "pushState"]({}, "", url);
  applyRoute(parseRoute(url.pathname, url.search));
}

function navigateToCurrentFile(options = {}) {
  navigateToFilePath(state.current?.file?.path || "", {
    ...options,
    preserveSettings: false,
  });
}

function navigateToFilePath(path, options = {}) {
  const { replace = false, apply = true, preserveSettings = false } = options;
  const url = new URL(window.location.href);
  url.pathname = encodeAppPath(path || "");
  if (!preserveSettings) {
    url.searchParams.delete("settings");
  }
  window.history[replace ? "replaceState" : "pushState"]({}, "", url);
  if (apply) {
    applyRoute(parseRoute(url.pathname, url.search));
  }
}

function syncCurrentPath(path) {
  const url = new URL(window.location.href);
  url.pathname = encodeAppPath(path || "");
  window.history.replaceState({}, "", url);
}
