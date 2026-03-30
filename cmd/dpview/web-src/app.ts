import { applyPreviewSeek } from "./seek";
import { encodeAppPath, isSettingsRoute, parseRoute } from "./routes";
import type {
  ApiError,
  ApiResult,
  CurrentData,
  FileInfo,
  FilesData,
  FrontMatter,
  HealthData,
  Route,
  SeekData,
  Settings,
  SettingsData,
  TreeNode,
} from "./types";

declare const katex:
  | {
      render: (
        expression: string,
        element: Element,
        options: { displayMode: boolean; throwOnError: boolean },
      ) => void;
    }
  | undefined;

declare const renderMathInElement:
  | ((
      element: Element,
      options: {
        delimiters: Array<{ left: string; right: string; display: boolean }>;
        throwOnError: boolean;
      },
    ) => void)
  | undefined;

const STORAGE = {
  expanded: "dpview.expanded",
  currentPath: "dpview.currentPath",
  search: "dpview.search",
  theme: "dpview.theme",
} as const;

interface State {
  files: FileInfo[];
  tree: TreeNode[];
  current: CurrentData | null;
  settings: Settings;
  health: HealthData | null;
  seek: SeekData | null;
  expanded: Set<string>;
  search: string;
  theme: string;
  previewTheme: string;
  sidebarCollapsed: boolean;
  frontMatterExpanded: boolean | null;
  localSelectionInFlight: string;
  statusMessage: string;
  pendingSeekFrame: number;
}

interface SyncSettingsOptions {
  rerenderTypst?: boolean;
  rerenderMarkdown?: boolean;
}

type Page = "file" | "settings" | "not-found";

const systemThemeMedia =
  window.matchMedia?.("(prefers-color-scheme: dark)") || null;

const state: State = {
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
    theme: "system",
    preview_theme: "default",
  },
  health: null,
  seek: null,
  expanded: loadExpandedPaths(),
  search: localStorage.getItem(STORAGE.search) || "",
  theme: localStorage.getItem(STORAGE.theme) || "system",
  previewTheme: "default",
  sidebarCollapsed: false,
  frontMatterExpanded: null,
  localSelectionInFlight: "",
  statusMessage: "",
  pendingSeekFrame: 0,
};

const appEl = requiredSelector<HTMLDivElement>(".app");
const sidebarEl = requiredElement<HTMLElement>("sidebar");
const treeEl = requiredElement<HTMLElement>("tree");
const previewEl = requiredElement<HTMLElement>("preview");
const markdownThemeCSS = requiredElement<HTMLLinkElement>("markdown-theme-css");
const statusEl = requiredElement<HTMLElement>("status");
const healthEl = requiredElement<HTMLElement>("health");
const searchInput = requiredElement<HTMLInputElement>("search");
const pauseRefreshInput = requiredElement<HTMLInputElement>("pause-refresh");
const themeSelect = requiredElement<HTMLSelectElement>("theme");
const previewThemeSelect = requiredElement<HTMLSelectElement>("preview-theme");
const typstPreviewThemeInput =
  requiredElement<HTMLInputElement>("typst-preview-theme");
const editorFileSyncInput = requiredElement<HTMLInputElement>("editor-file-sync");
const seekEnabledInput = requiredElement<HTMLInputElement>("seek-enabled");
const markdownFrontMatterVisibleInput = requiredElement<HTMLInputElement>(
  "markdown-frontmatter-visible",
);
const markdownFrontMatterExpandedInput = requiredElement<HTMLInputElement>(
  "markdown-frontmatter-expanded",
);
const markdownFrontMatterTitleInput = requiredElement<HTMLInputElement>(
  "markdown-frontmatter-title",
);
const openSettingsButton = requiredElement<HTMLButtonElement>("open-settings");
const closeSettingsButton = requiredElement<HTMLButtonElement>("close-settings");
const toggleSidebarButton = requiredElement<HTMLButtonElement>("toggle-sidebar");
const showSidebarButton = requiredElement<HTMLButtonElement>("show-sidebar");
const goHomeButton = requiredElement<HTMLButtonElement>("go-home");
const notFoundMessageEl = requiredElement<HTMLElement>("not-found-message");
const fileViewEl = requiredElement<HTMLElement>("file-view");
const settingsViewEl = requiredElement<HTMLElement>("settings-view");
const notFoundViewEl = requiredElement<HTMLElement>("not-found-view");

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
  state.settings.markdown_frontmatter_visible =
    markdownFrontMatterVisibleInput.checked;
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
  state.settings.markdown_frontmatter_expanded =
    markdownFrontMatterExpandedInput.checked;
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
  state.settings.markdown_frontmatter_title =
    markdownFrontMatterTitleInput.checked;
  const result = await syncSettings({ rerenderMarkdown: true });
  if (!result.ok) {
    state.settings.markdown_frontmatter_title = previous;
    markdownFrontMatterTitleInput.checked = previous;
    return;
  }
  setStatus("Settings updated.");
});

function requiredElement<T extends HTMLElement>(id: string): T {
  const element = document.getElementById(id);
  if (!element) {
    throw new Error(`Missing required element #${id}`);
  }
  return element as T;
}

function requiredSelector<T extends Element>(selector: string): T {
  const element = document.querySelector(selector);
  if (!element) {
    throw new Error(`Missing required element ${selector}`);
  }
  return element as T;
}

function loadExpandedPaths(): Set<string> {
  try {
    const raw = localStorage.getItem(STORAGE.expanded);
    if (!raw) {
      return new Set();
    }
    const values = JSON.parse(raw);
    return Array.isArray(values)
      ? new Set(values.filter((value): value is string => typeof value === "string"))
      : new Set();
  } catch {
    return new Set();
  }
}

function setStatus(message: string): void {
  state.statusMessage = message || "";
  renderStatus();
}

function renderStatus(): void {
  statusEl.textContent = state.statusMessage;
}

function applyTheme(theme: string): void {
  document.body.dataset.theme = resolveThemeMode(theme);
}

function resolveThemeMode(theme: string): string {
  return theme === "system"
    ? systemThemeMedia?.matches
      ? "dark"
      : "light"
    : theme;
}

function applyMarkdownTheme(theme: string): void {
  markdownThemeCSS.href = `/themes/markdown/${theme}.css`;
}

function setSidebarCollapsed(collapsed: boolean): void {
  state.sidebarCollapsed = collapsed;
  state.settings.sidebar_collapsed = collapsed;
  renderSidebar();
  void syncSettings();
}

function renderSidebar(): void {
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

function escapeHTML(value?: string): string {
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

async function apiFetch<T>(
  path: string,
  options: RequestInit = {},
): Promise<ApiResult<T>> {
  const response = await fetch(path, options);
  const payload = (await response.json().catch(() => ({
    ok: false,
    error: { message: "Invalid server response" },
  }))) as { ok?: boolean; data?: T; error?: ApiError };
  if (!response.ok || !payload.ok) {
    return {
      ok: false,
      error: payload.error || { message: "Request failed" },
    };
  }
  return { ok: true, data: payload.data as T };
}

function setPage(page: Page): void {
  fileViewEl.classList.toggle("hidden", page !== "file");
  settingsViewEl.classList.toggle("hidden", page !== "settings");
  notFoundViewEl.classList.toggle("hidden", page !== "not-found");
}

function filteredNodes(nodes: TreeNode[]): TreeNode[] {
  if (!state.search) {
    return nodes;
  }
  const matchNode = (node: TreeNode): TreeNode | null => {
    const haystack = `${node.name} ${node.path || ""}`.toLowerCase();
    const isMatch = haystack.includes(state.search);
    if (node.children?.length) {
      const children = node.children.map(matchNode).filter(isDefined);
      if (isMatch || children.length) {
        return { ...node, children };
      }
      return null;
    }
    return isMatch ? node : null;
  };
  return nodes.map(matchNode).filter(isDefined);
}

function isDefined<T>(value: T | null): value is T {
  return value !== null;
}

function renderTree(): void {
  treeEl.innerHTML = "";
  const nodes = filteredNodes(state.tree);
  if (!nodes.length) {
    treeEl.textContent = "No matching files.";
    return;
  }
  renderTreeNodes(nodes, treeEl, 0);
}

function renderTreeNodes(
  nodes: TreeNode[],
  container: HTMLElement,
  depth: number,
): void {
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
      row.addEventListener("click", () => void setCurrent(node.path));
      wrapper.appendChild(row);
    }

    container.appendChild(wrapper);
  }
}

function renderPreview(): void {
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

function renderMarkdownMath(container: Element | null): void {
  if (!container) {
    return;
  }
  if (typeof katex?.render === "function") {
    for (const node of container.querySelectorAll(".markdown-math-block")) {
      katex.render(node.getAttribute("data-latex") || "", node, {
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

function renderFrontMatter(frontMatter?: FrontMatter): string {
  if (!frontMatter?.entries?.length) {
    return "";
  }
  const open =
    (state.frontMatterExpanded ?? state.settings.markdown_frontmatter_expanded)
      ? " open"
      : "";
  const rows = frontMatter.entries
    .map(
      (entry) => `
    <tr>
      <th>${escapeHTML(entry.key)}</th>
      <td><code>${escapeHTML(entry.value)}</code></td>
    </tr>
  `,
    )
    .join("");
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

function rememberFrontMatterState(): void {
  const panel = previewEl.querySelector<HTMLDetailsElement>(".frontmatter-panel");
  if (panel) {
    state.frontMatterExpanded = panel.open;
  }
}

function bindFrontMatterState(): void {
  const panel = previewEl.querySelector<HTMLDetailsElement>(".frontmatter-panel");
  if (!panel) {
    return;
  }
  state.frontMatterExpanded = panel.open;
  panel.addEventListener("toggle", () => {
    state.frontMatterExpanded = panel.open;
  });
}

function renderHealth(): void {
  const health = state.health;
  if (!health) {
    healthEl.innerHTML = "";
    return;
  }

  const items: string[] = [];
  items.push(
    `<div class="status-item">Status: ${escapeHTML(health.status || "unknown")}</div>`,
  );
  items.push(
    `<div class="status-item">Watcher: ${health.watcher?.enabled ? "enabled" : "disabled"}</div>`,
  );

  for (const renderer of health.renderers || []) {
    const value = renderer.available ? "available" : "unavailable";
    items.push(
      `<div class="status-item">${escapeHTML(renderer.name || renderer.kind)}: ${value}</div>`,
    );
  }

  healthEl.innerHTML = items.join("");
}

function applyFiles(data: FilesData): void {
  state.files = data.files || [];
  state.tree = data.tree || [];
  renderTree();
  setStatus(
    `${state.files.length} file${state.files.length === 1 ? "" : "s"} indexed`,
  );
}

function applyCurrent(data: CurrentData): void {
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

function applySeek(data: SeekData | null): void {
  state.seek = data || null;
  if (!state.settings.seek_enabled) {
    return;
  }
  queueApplySeek();
}

function applySettings(data: SettingsData): void {
  const settings = data.settings || state.settings;
  const storedTheme = localStorage.getItem(STORAGE.theme);
  state.settings = {
    auto_refresh_paused: !!settings.auto_refresh_paused,
    sidebar_collapsed: !!settings.sidebar_collapsed,
    editor_file_sync_enabled: settings.editor_file_sync_enabled !== false,
    seek_enabled: settings.seek_enabled !== false,
    typst_preview_theme: settings.typst_preview_theme !== false,
    markdown_frontmatter_visible:
      settings.markdown_frontmatter_visible !== false,
    markdown_frontmatter_expanded:
      settings.markdown_frontmatter_expanded !== false,
    markdown_frontmatter_title: settings.markdown_frontmatter_title !== false,
    theme: settings.theme || "light",
    preview_theme: settings.preview_theme || "default",
  };
  state.theme = storedTheme || state.theme || "system";
  state.previewTheme = state.settings.preview_theme;
  state.sidebarCollapsed = state.settings.sidebar_collapsed;
  themeSelect.value = state.theme;
  previewThemeSelect.value = state.previewTheme;
  applyTheme(state.theme);
  applyMarkdownTheme(state.previewTheme);
  renderSidebar();
  pauseRefreshInput.checked = !!state.settings.auto_refresh_paused;
  editorFileSyncInput.checked = !!state.settings.editor_file_sync_enabled;
  seekEnabledInput.checked = !!state.settings.seek_enabled;
  typstPreviewThemeInput.checked = !!state.settings.typst_preview_theme;
  markdownFrontMatterVisibleInput.checked =
    !!state.settings.markdown_frontmatter_visible;
  markdownFrontMatterExpandedInput.checked =
    !!state.settings.markdown_frontmatter_expanded;
  markdownFrontMatterTitleInput.checked =
    !!state.settings.markdown_frontmatter_title;
  if (!state.settings.seek_enabled) {
    state.seek = null;
  } else {
    queueApplySeek();
  }
  if (state.current?.file) {
    renderPreview();
  }
}

function currentSettingsPayload(): Settings {
  return {
    auto_refresh_paused: !!state.settings.auto_refresh_paused,
    sidebar_collapsed: !!state.sidebarCollapsed,
    editor_file_sync_enabled: !!state.settings.editor_file_sync_enabled,
    seek_enabled: !!state.settings.seek_enabled,
    typst_preview_theme: !!state.settings.typst_preview_theme,
    markdown_frontmatter_visible: !!state.settings.markdown_frontmatter_visible,
    markdown_frontmatter_expanded:
      !!state.settings.markdown_frontmatter_expanded,
    markdown_frontmatter_title: !!state.settings.markdown_frontmatter_title,
    theme: resolveThemeMode(state.theme),
    preview_theme: state.previewTheme,
  };
}

async function refreshCurrentPreview(): Promise<ApiResult<CurrentData>> {
  const result = await apiFetch<CurrentData>("/api/refresh", { method: "POST" });
  if (!result.ok) {
    setStatus(result.error.message);
    return result;
  }
  applyCurrent(result.data);
  return result;
}

async function syncSettings(
  options: SyncSettingsOptions = {},
): Promise<ApiResult<SettingsData> | ApiResult<CurrentData>> {
  const result = await apiFetch<SettingsData>("/api/settings", {
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

function applyHealth(data: HealthData): void {
  state.health = data;
  renderHealth();
}

function queueApplySeek(): void {
  if (state.pendingSeekFrame) {
    cancelAnimationFrame(state.pendingSeekFrame);
  }
  state.pendingSeekFrame = requestAnimationFrame(() => {
    state.pendingSeekFrame = 0;
    applySeekToPreview();
  });
}

function applySeekToPreview(): void {
  applyPreviewSeek(
    fileViewEl,
    previewEl,
    state.current,
    state.seek,
    state.settings,
  );
}

async function setCurrent(path: string): Promise<void> {
  state.localSelectionInFlight = path;
  setStatus(`Loading ${path}...`);
  const result = await apiFetch<CurrentData>("/api/current", {
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

function syncLocationPath(path: string): void {
  syncCurrentPath(path);
}

async function loadInitialState(): Promise<void> {
  const initialRoute = parseRoute(
    window.location.pathname,
    window.location.search,
  );
  const [health, files, current, seek, settings] = await Promise.all([
    apiFetch<HealthData>("/api/health"),
    apiFetch<FilesData>("/api/files"),
    apiFetch<CurrentData>("/api/current"),
    apiFetch<SeekData>("/api/seek"),
    apiFetch<SettingsData>("/api/settings"),
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

  const initialSettings = settings.data.settings || state.settings;
  if (
    resolveThemeMode(state.theme) !== (initialSettings.theme || "light") ||
    state.previewTheme !== (initialSettings.preview_theme || "default") ||
    !!state.settings.typst_preview_theme !==
      (initialSettings.typst_preview_theme !== false)
  ) {
    await syncSettings({ rerenderTypst: current.data.file?.kind === "typst" });
  }

  const storedPath = localStorage.getItem(STORAGE.currentPath);
  const routedPath = initialRoute.kind === "file" ? initialRoute.path : "";
  const preferredPath =
    routedPath || (!current.data.current && storedPath ? storedPath : "");
  if (
    preferredPath &&
    preferredPath !== current.data.file?.path &&
    fileExists(preferredPath)
  ) {
    await setCurrent(preferredPath);
  }
}

function parseEventData<T>(event: MessageEvent<string>): T {
  return (JSON.parse(event.data) as { data: T }).data;
}

function connectEvents(): void {
  const events = new EventSource("/events");

  events.addEventListener("files_changed", (event) => {
    applyFiles(parseEventData<FilesData>(event as MessageEvent<string>));
  });
  events.addEventListener("current_changed", (event) => {
    applyCurrent(parseEventData<CurrentData>(event as MessageEvent<string>));
  });
  events.addEventListener("preview_updated", (event) => {
    applyCurrent(parseEventData<CurrentData>(event as MessageEvent<string>));
  });
  events.addEventListener("seek_changed", (event) => {
    applySeek(parseEventData<SeekData>(event as MessageEvent<string>));
  });
  events.addEventListener("render_started", (event) => {
    state.current = parseEventData<CurrentData>(event as MessageEvent<string>);
    renderPreview();
    setStatus(`Rendering ${state.current?.file?.path || "file"}...`);
  });
  events.addEventListener("render_failed", (event) => {
    applyCurrent(parseEventData<CurrentData>(event as MessageEvent<string>));
  });
  events.addEventListener("settings_changed", (event) => {
    applySettings(parseEventData<SettingsData>(event as MessageEvent<string>));
  });
  events.onerror = () => {
    setStatus("Event stream disconnected. Retrying...");
  };
}

loadInitialState()
  .then(connectEvents)
  .catch((error: unknown) => {
    setStatus(
      error instanceof Error
        ? error.message
        : "Failed to load application state",
    );
    applyRoute({ kind: "settings" });
  });

function fileExists(path: string): boolean {
  return state.files.some((file) => file.path === path);
}

function applyRoute(route: Route): void {
  if (route.kind === "settings") {
    setPage("settings");
    return;
  }
  if (!route.path || fileExists(route.path) || state.files.length === 0) {
    setPage("file");
    if (
      route.path &&
      state.current?.file?.path !== route.path &&
      fileExists(route.path)
    ) {
      void setCurrent(route.path);
    }
    return;
  }
  showNotFound(`No previewable file exists at "${route.path}".`);
}

function showNotFound(message: string): void {
  notFoundMessageEl.textContent = message;
  setPage("not-found");
}

function navigateToSettings(options: { replace?: boolean } = {}): void {
  const { replace = false } = options;
  const url = new URL(window.location.href);
  url.searchParams.set("settings", "open");
  if (replace) {
    window.history.replaceState({}, "", url);
  } else {
    window.history.pushState({}, "", url);
  }
  applyRoute(parseRoute(url.pathname, url.search));
}

function navigateToCurrentFile(options: { replace?: boolean } = {}): void {
  navigateToFilePath(state.current?.file?.path || "", {
    ...options,
    preserveSettings: false,
  });
}

function navigateToFilePath(
  path: string,
  options: { replace?: boolean; apply?: boolean; preserveSettings?: boolean } = {},
): void {
  const { replace = false, apply = true, preserveSettings = false } = options;
  const url = new URL(window.location.href);
  url.pathname = encodeAppPath(path || "");
  if (!preserveSettings) {
    url.searchParams.delete("settings");
  }
  if (replace) {
    window.history.replaceState({}, "", url);
  } else {
    window.history.pushState({}, "", url);
  }
  if (apply) {
    applyRoute(parseRoute(url.pathname, url.search));
  }
}

function syncCurrentPath(path: string): void {
  const url = new URL(window.location.href);
  url.pathname = encodeAppPath(path || "");
  window.history.replaceState({}, "", url);
}
