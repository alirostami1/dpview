import { apiFetch } from "./api";
import { requiredElement, requiredSelector } from "./dom";
import { renderMarkdownMath as renderLatexMath } from "./latex";
import { createLiveEventController } from "./live-events";
import { createNavigationController } from "./navigation";
import {
  applyCurrent as applyCurrentState,
  applyFiles as applyFilesState,
  applyHealth as applyHealthState,
  applyLogs as applyLogsState,
  applySeek as applySeekState,
  applySettings as applySettingsState,
  clearLastError,
  setConnectionState,
  setLastError,
  setPreviewThemePreference,
  setSearch,
  setSidebarCollapsed,
  setSidebarMode,
  setStatus,
  setThemePreference,
} from "./actions";
import {
  renderConnectionBanner,
  renderSidebarShell,
  renderStatus,
} from "./render";
import { createViewController } from "./view-controller";
import type { Elements, State } from "./model";
import {
  currentDataSchema,
  currentPathStorageSchema,
  expandedPathsStorageSchema,
  filesDataSchema,
  healthDataSchema,
  logDataSchema,
  previewThemeSchema,
  searchStorageSchema,
  seekDataSchema,
  settingsDataSchema,
  settingsPayloadSchema,
  storedThemeSchema,
} from "./contracts";
import { readStoredJSON, readStoredString, STORAGE } from "./storage";
import type {
  ApiResult,
  CurrentData,
  LogData,
  PreviewTheme,
  ResolvedTheme,
  SettingsData,
  SettingsPayload,
  StoredTheme,
} from "./types";

const systemThemeMedia =
  window.matchMedia?.("(prefers-color-scheme: dark)") || null;
const mobileLayoutMedia = window.matchMedia?.("(max-width: 800px)") || null;

const elements: Elements = {
  appEl: requiredSelector<HTMLDivElement>(".app"),
  sidebarEl: requiredElement<HTMLElement>("sidebar"),
  treeEl: requiredElement<HTMLElement>("tree"),
  previewEl: requiredElement<HTMLElement>("preview"),
  connectionBannerEl: requiredElement<HTMLElement>("connection-banner"),
  connectionMessageEl: requiredElement<HTMLElement>("connection-message"),
  retryConnectionButton: requiredElement<HTMLButtonElement>("retry-connection"),
  markdownThemeCSS: requiredElement<HTMLLinkElement>("markdown-theme-css"),
  statusEl: requiredElement<HTMLElement>("status"),
  healthEl: requiredElement<HTMLElement>("health"),
  logsEl: requiredElement<HTMLTextAreaElement>("logs"),
  copyLogsButton: requiredElement<HTMLButtonElement>("copy-logs"),
  searchInput: requiredElement<HTMLInputElement>("search"),
  sidebarFilesTab: requiredElement<HTMLButtonElement>("sidebar-files-tab"),
  sidebarSettingsTab: requiredElement<HTMLButtonElement>(
    "sidebar-settings-tab"
  ),
  sidebarFilesViewEl: requiredElement<HTMLElement>("sidebar-files-view"),
  sidebarSettingsViewEl: requiredElement<HTMLElement>("sidebar-settings-view"),
  pauseRefreshInput: requiredElement<HTMLInputElement>("pause-refresh"),
  themeSelect: requiredElement<HTMLSelectElement>("theme"),
  previewThemeSelect: requiredElement<HTMLSelectElement>("preview-theme"),
  typstPreviewThemeInput: requiredElement<HTMLInputElement>(
    "typst-preview-theme"
  ),
  editorFileSyncInput: requiredElement<HTMLInputElement>("editor-file-sync"),
  liveBufferPreviewInput: requiredElement<HTMLInputElement>(
    "live-buffer-preview"
  ),
  seekEnabledInput: requiredElement<HTMLInputElement>("seek-enabled"),
  latexEnabledInput: requiredElement<HTMLInputElement>("latex-enabled"),
  markdownFrontMatterVisibleInput: requiredElement<HTMLInputElement>(
    "markdown-frontmatter-visible"
  ),
  markdownFrontMatterExpandedInput: requiredElement<HTMLInputElement>(
    "markdown-frontmatter-expanded"
  ),
  markdownFrontMatterTitleInput: requiredElement<HTMLInputElement>(
    "markdown-frontmatter-title"
  ),
  clearLogsButton: requiredElement<HTMLButtonElement>("clear-logs"),
  toggleSidebarButton: requiredElement<HTMLButtonElement>("toggle-sidebar"),
  showSidebarButton: requiredElement<HTMLButtonElement>("show-sidebar"),
  goHomeButton: requiredElement<HTMLButtonElement>("go-home"),
  notFoundMessageEl: requiredElement<HTMLElement>("not-found-message"),
  fileViewEl: requiredElement<HTMLElement>("file-view"),
  notFoundViewEl: requiredElement<HTMLElement>("not-found-view"),
};

const initialTheme = readStoredString(
  STORAGE.theme,
  storedThemeSchema,
  "system"
);
const initialPreviewTheme = parsePreviewTheme(
  elements.previewThemeSelect.value
);

const state: State = {
  files: [],
  tree: [],
  current: null,
  lastSettledCurrent: null,
  settings: {
    auto_refresh_paused: false,
    sidebar_collapsed: false,
    editor_file_sync_enabled: true,
    live_buffer_preview_enabled: false,
    seek_enabled: true,
    latex_enabled: true,
    typst_preview_theme: true,
    markdown_frontmatter_visible: true,
    markdown_frontmatter_expanded: true,
    markdown_frontmatter_title: true,
    theme: "light",
    preview_theme: "default",
  },
  health: null,
  logs: null,
  seek: null,
  expanded: new Set(
    readStoredJSON(STORAGE.expanded, expandedPathsStorageSchema, [])
  ),
  search: readStoredString(STORAGE.search, searchStorageSchema, ""),
  theme: initialTheme,
  previewTheme: initialPreviewTheme,
  sidebarCollapsed: false,
  sidebarMode: "files",
  frontMatterExpanded: null,
  localSelectionInFlight: "",
  statusMessage: "",
  lastError: "",
  connectionStatus: "connecting",
  connectionAttempts: 0,
  reconnectAt: 0,
  bootstrapFailed: false,
  pendingSeekFrame: 0,
};

let lastLatexLoadError = "";
let forceMarkdownPreviewReplace = false;

const viewController = createViewController({
  elements,
  state,
  onSelectFile: (path) => {
    void setCurrent(path);
  },
  onRenderMarkdownMath: renderMarkdownMath,
});

const navigation = createNavigationController({
  elements,
  state,
  fileExists,
  onSelectFile: (path) => {
    void setCurrent(path);
  },
});

const liveEvents = createLiveEventController({
  elements,
  state,
  onFilesChanged: applyFiles,
  onCurrentChanged: applyCurrent,
  onPreviewUpdated: applyCurrent,
  onSeekChanged: applySeek,
  onLogsChanged: applyLogs,
  onRenderStarted: handleRenderStarted,
  onRenderFailed: applyCurrent,
  onSettingsChanged: applySettings,
  setClientError,
  clearClientError,
});

initializeUI();
bindUIEvents();
void bootstrap();

function initializeUI(): void {
  if (isMobileLayout()) {
    state.sidebarCollapsed = true;
    state.settings.sidebar_collapsed = true;
  }
  elements.searchInput.value = state.search;
  elements.themeSelect.value = state.theme;
  elements.previewThemeSelect.value = state.previewTheme;
  elements.statusEl.setAttribute("role", "status");
  elements.statusEl.setAttribute("aria-live", "polite");
  elements.showSidebarButton.setAttribute("aria-controls", "sidebar");
  elements.toggleSidebarButton.setAttribute("aria-controls", "sidebar");
  elements.sidebarFilesTab.setAttribute("aria-controls", "sidebar-files-view");
  elements.sidebarSettingsTab.setAttribute(
    "aria-controls",
    "sidebar-settings-view"
  );
  elements.previewEl.tabIndex = -1;
  applyTheme(state.theme);
  applyMarkdownTheme(state.previewTheme);
  renderStatus(elements, state);
  renderConnectionBanner(elements, state);
  viewController.renderSettingsUI();
  renderSidebarShell(elements, state);
  viewController.renderTreeUI();
}

function bindUIEvents(): void {
  navigation.bindWindowEvents();

  elements.searchInput.addEventListener("input", () => {
    setSearch(state, elements.searchInput.value.trim().toLowerCase());
    localStorage.setItem(STORAGE.search, state.search);
    viewController.renderTreeUI();
  });

  elements.sidebarFilesTab.addEventListener("click", () =>
    updateSidebarMode("files")
  );
  elements.sidebarSettingsTab.addEventListener("click", () =>
    updateSidebarMode("settings")
  );
  elements.goHomeButton.addEventListener("click", () =>
    navigation.navigateToCurrentFile()
  );
  elements.toggleSidebarButton.addEventListener("click", () =>
    updateSidebarCollapsed(true)
  );
  elements.showSidebarButton.addEventListener("click", () =>
    updateSidebarCollapsed(false)
  );
  elements.retryConnectionButton.addEventListener("click", () => {
    void retryApplicationState();
  });
  elements.clearLogsButton.addEventListener("click", async () => {
    const result = await apiFetch("/api/logs", logDataSchema, {
      method: "DELETE",
    });
    if (!result.ok) {
      setClientError(result.error.message);
      return;
    }
    clearClientError();
    applyLogs(result.data);
    updateStatus("Log cleared.");
  });
  elements.copyLogsButton.addEventListener("click", async () => {
    const text = elements.logsEl.value;
    if (!text) {
      updateStatus("No log to copy.");
      return;
    }
    try {
      await navigator.clipboard.writeText(text);
      updateStatus("Log copied.");
    } catch (error) {
      setClientError(
        error instanceof Error ? error.message : "Failed to copy log."
      );
    }
  });
  window.addEventListener("resize", () => {
    if (state.settings.seek_enabled) {
      viewController.queueApplySeek();
    }
  });
  window.addEventListener("keydown", (event) => {
    if (event.key === "Escape" && state.sidebarMode === "settings") {
      event.preventDefault();
      updateSidebarMode("files");
      return;
    }
    if (event.key === "/" && document.activeElement !== elements.searchInput) {
      const target = event.target;
      if (
        target instanceof HTMLInputElement ||
        target instanceof HTMLTextAreaElement ||
        target instanceof HTMLSelectElement
      ) {
        return;
      }
      event.preventDefault();
      updateSidebarCollapsed(false);
      requestAnimationFrame(() => {
        elements.searchInput.focus();
        elements.searchInput.select();
      });
    }
  });
  elements.searchInput.addEventListener("keydown", (event) => {
    if (event.key === "ArrowDown") {
      const firstItem = elements.treeEl.querySelector<HTMLButtonElement>(
        "[data-tree-item='true']"
      );
      if (firstItem) {
        event.preventDefault();
        firstItem.focus();
      }
    }
  });

  elements.pauseRefreshInput.addEventListener("change", async () => {
    const previous = state.settings.auto_refresh_paused;
    state.settings.auto_refresh_paused = elements.pauseRefreshInput.checked;
    const result = await syncSettings();
    if (!result.ok) {
      state.settings.auto_refresh_paused = previous;
      elements.pauseRefreshInput.checked = previous;
      return;
    }
    updateStatus("Settings updated.");
  });

  elements.themeSelect.addEventListener("change", async () => {
    const nextTheme = storedThemeSchema.safeParse(elements.themeSelect.value);
    if (!nextTheme.success) {
      updateStatus(`Unknown theme: ${elements.themeSelect.value}`);
      elements.themeSelect.value = state.theme;
      return;
    }
    setThemePreference(state, nextTheme.data);
    localStorage.setItem(STORAGE.theme, state.theme);
    applyTheme(state.theme);
    await syncSettings({ rerenderTypst: true });
  });

  elements.previewThemeSelect.addEventListener("change", async () => {
    const nextPreviewTheme = previewThemeSchema.safeParse(
      elements.previewThemeSelect.value
    );
    if (!nextPreviewTheme.success) {
      updateStatus(
        `Unknown preview theme: ${elements.previewThemeSelect.value}`
      );
      elements.previewThemeSelect.value = state.previewTheme;
      return;
    }
    setPreviewThemePreference(state, nextPreviewTheme.data);
    applyMarkdownTheme(state.previewTheme);
    viewController.renderPreviewUI();
    await syncSettings({ rerenderTypst: true });
  });

  elements.typstPreviewThemeInput.addEventListener("change", async () => {
    const previous = state.settings.typst_preview_theme;
    state.settings.typst_preview_theme =
      elements.typstPreviewThemeInput.checked;
    const result = await syncSettings({ rerenderTypst: true });
    if (!result.ok) {
      state.settings.typst_preview_theme = previous;
      elements.typstPreviewThemeInput.checked = previous;
      return;
    }
    updateStatus("Settings updated.");
  });

  elements.editorFileSyncInput.addEventListener("change", async () => {
    const previous = state.settings.editor_file_sync_enabled;
    state.settings.editor_file_sync_enabled =
      elements.editorFileSyncInput.checked;
    const result = await syncSettings();
    if (!result.ok) {
      state.settings.editor_file_sync_enabled = previous;
      elements.editorFileSyncInput.checked = previous;
      return;
    }
    updateStatus("Settings updated.");
  });

  elements.liveBufferPreviewInput.addEventListener("change", async () => {
    const previous = state.settings.live_buffer_preview_enabled;
    state.settings.live_buffer_preview_enabled =
      elements.liveBufferPreviewInput.checked;
    const result = await syncSettings({
      rerenderTypst:
        state.current?.transient === true &&
        state.current.file?.kind === "typst",
      rerenderMarkdown:
        state.current?.transient === true &&
        state.current.file?.kind === "markdown",
    });
    if (!result.ok) {
      state.settings.live_buffer_preview_enabled = previous;
      elements.liveBufferPreviewInput.checked = previous;
      return;
    }
    updateStatus("Settings updated.");
  });

  elements.seekEnabledInput.addEventListener("change", async () => {
    const previous = state.settings.seek_enabled;
    state.settings.seek_enabled = elements.seekEnabledInput.checked;
    const result = await syncSettings();
    if (!result.ok) {
      state.settings.seek_enabled = previous;
      elements.seekEnabledInput.checked = previous;
      return;
    }
    if (!state.settings.seek_enabled) {
      applySeekState(state, null);
    } else {
      viewController.queueApplySeek();
    }
    updateStatus("Settings updated.");
  });

  elements.latexEnabledInput.addEventListener("change", async () => {
    const previous = state.settings.latex_enabled;
    state.settings.latex_enabled = elements.latexEnabledInput.checked;
    forceMarkdownPreviewReplace = true;
    const result = await syncSettings({
      rerenderMarkdown: state.current?.file?.kind === "markdown",
    });
    if (!result.ok) {
      state.settings.latex_enabled = previous;
      elements.latexEnabledInput.checked = previous;
      forceMarkdownPreviewReplace = false;
      return;
    }
    updateStatus("Settings updated.");
  });

  elements.markdownFrontMatterVisibleInput.addEventListener(
    "change",
    async () => {
      const previous = state.settings.markdown_frontmatter_visible;
      state.settings.markdown_frontmatter_visible =
        elements.markdownFrontMatterVisibleInput.checked;
      const result = await syncSettings();
      if (!result.ok) {
        state.settings.markdown_frontmatter_visible = previous;
        elements.markdownFrontMatterVisibleInput.checked = previous;
        return;
      }
      viewController.renderPreviewUI();
      updateStatus("Settings updated.");
    }
  );

  elements.markdownFrontMatterExpandedInput.addEventListener(
    "change",
    async () => {
      const previous = state.settings.markdown_frontmatter_expanded;
      state.settings.markdown_frontmatter_expanded =
        elements.markdownFrontMatterExpandedInput.checked;
      const result = await syncSettings();
      if (!result.ok) {
        state.settings.markdown_frontmatter_expanded = previous;
        elements.markdownFrontMatterExpandedInput.checked = previous;
        return;
      }
      viewController.renderPreviewUI();
      updateStatus("Settings updated.");
    }
  );

  elements.markdownFrontMatterTitleInput.addEventListener(
    "change",
    async () => {
      const previous = state.settings.markdown_frontmatter_title;
      state.settings.markdown_frontmatter_title =
        elements.markdownFrontMatterTitleInput.checked;
      const result = await syncSettings({ rerenderMarkdown: true });
      if (!result.ok) {
        state.settings.markdown_frontmatter_title = previous;
        elements.markdownFrontMatterTitleInput.checked = previous;
        return;
      }
      updateStatus("Settings updated.");
    }
  );

  elements.treeEl.addEventListener("keydown", navigation.handleTreeKeydown);

  if (systemThemeMedia) {
    systemThemeMedia.addEventListener("change", async () => {
      if (state.theme === "system") {
        applyTheme("system");
        await syncSettings({ rerenderTypst: true });
      }
    });
  }
  if (mobileLayoutMedia) {
    mobileLayoutMedia.addEventListener("change", () => {
      if (!isMobileLayout()) {
        return;
      }
      updateSidebarCollapsedLocal(true);
    });
  }
}

async function bootstrap(): Promise<void> {
  try {
    state.bootstrapFailed = false;
    await loadInitialState();
    liveEvents.connect();
    renderConnectionBanner(elements, state);
  } catch (error) {
    const message =
      error instanceof Error
        ? error.message
        : "Failed to load application state";
    state.bootstrapFailed = true;
    setLastError(state, message);
    setConnectionState(state, "degraded");
    renderStatus(elements, state);
    renderConnectionBanner(elements, state);
    updateSidebarMode("settings");
  }
}

async function retryApplicationState(): Promise<void> {
  liveEvents.close();
  setConnectionState(state, "connecting", 0);
  renderStatus(elements, state);
  renderConnectionBanner(elements, state);
  await bootstrap();
}

function parsePreviewTheme(value: string): PreviewTheme {
  const result = previewThemeSchema.safeParse(value);
  return result.success ? result.data : "default";
}

function updateStatus(message: string): void {
  setStatus(state, message);
  renderStatus(elements, state);
  renderConnectionBanner(elements, state);
}

function setClientError(message: string): void {
  setLastError(state, message);
  renderStatus(elements, state);
  renderConnectionBanner(elements, state);
}

function clearClientError(): void {
  if (!state.lastError) {
    return;
  }
  clearLastError(state);
  renderStatus(elements, state);
  renderConnectionBanner(elements, state);
}

function resolveThemeMode(theme: StoredTheme): ResolvedTheme {
  return theme === "system"
    ? systemThemeMedia?.matches
      ? "dark"
      : "light"
    : theme;
}

function isMobileLayout(): boolean {
  return mobileLayoutMedia?.matches === true;
}

function updateSidebarCollapsedLocal(collapsed: boolean): void {
  const activeElement = document.activeElement;
  setSidebarCollapsed(state, collapsed);
  renderSidebarShell(elements, state);
  if (
    collapsed &&
    activeElement instanceof HTMLElement &&
    elements.sidebarEl.contains(activeElement)
  ) {
    requestAnimationFrame(() => elements.showSidebarButton.focus());
  }
}

function applyTheme(theme: StoredTheme): void {
  document.body.dataset.theme = resolveThemeMode(theme);
}

function applyMarkdownTheme(theme: string): void {
  elements.markdownThemeCSS.href = `/themes/markdown/${theme}.css`;
}

function updateSidebarCollapsed(collapsed: boolean): void {
  updateSidebarCollapsedLocal(collapsed);
  if (!collapsed) {
    requestAnimationFrame(() => {
      if (state.sidebarMode === "settings") {
        elements.sidebarSettingsTab.focus();
        return;
      }
      elements.searchInput.focus();
    });
  }
  void syncSettings();
}

function updateSidebarMode(mode: "files" | "settings"): void {
  if (state.sidebarMode === mode) {
    if (state.sidebarCollapsed) {
      updateSidebarCollapsed(false);
    }
    return;
  }
  setSidebarMode(state, mode);
  renderSidebarShell(elements, state);
  if (state.sidebarCollapsed) {
    updateSidebarCollapsed(false);
    return;
  }
  requestAnimationFrame(() => {
    if (mode === "settings") {
      elements.sidebarSettingsTab.focus();
      return;
    }
    elements.searchInput.focus();
  });
}

function renderMarkdownMath(container: Element | null): void {
  if (!container) {
    return;
  }
  void renderLatexMath(container, state.settings.latex_enabled).then(
    (result) => {
      if (result.kind !== "failed") {
        return;
      }
      if (result.error.message === lastLatexLoadError) {
        return;
      }
      lastLatexLoadError = result.error.message;
      setClientError(result.error.message);
    }
  );
}

function currentSettingsPayload(): SettingsPayload {
  return settingsPayloadSchema.parse({
    auto_refresh_paused: state.settings.auto_refresh_paused,
    sidebar_collapsed: state.sidebarCollapsed,
    editor_file_sync_enabled: state.settings.editor_file_sync_enabled,
    live_buffer_preview_enabled: state.settings.live_buffer_preview_enabled,
    seek_enabled: state.settings.seek_enabled,
    latex_enabled: state.settings.latex_enabled,
    typst_preview_theme: state.settings.typst_preview_theme,
    markdown_frontmatter_visible: state.settings.markdown_frontmatter_visible,
    markdown_frontmatter_expanded: state.settings.markdown_frontmatter_expanded,
    markdown_frontmatter_title: state.settings.markdown_frontmatter_title,
    theme: resolveThemeMode(state.theme),
    preview_theme: state.previewTheme,
  });
}

async function refreshCurrentPreview(): Promise<ApiResult<CurrentData>> {
  const result = await apiFetch("/api/refresh", currentDataSchema, {
    method: "POST",
  });
  if (!result.ok) {
    setClientError(result.error.message);
    return result;
  }
  applyCurrent(result.data);
  return result;
}

async function syncSettings(
  options: { rerenderTypst?: boolean; rerenderMarkdown?: boolean } = {}
): Promise<ApiResult<SettingsData> | ApiResult<CurrentData>> {
  const result = await apiFetch("/api/settings", settingsDataSchema, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(currentSettingsPayload()),
  });
  if (!result.ok) {
    setClientError(result.error.message);
    return result;
  }
  clearClientError();
  applySettings(result.data);
  if (options.rerenderTypst && state.current?.file?.kind === "typst") {
    return refreshCurrentPreview();
  }
  if (options.rerenderMarkdown && state.current?.file?.kind === "markdown") {
    return refreshCurrentPreview();
  }
  return result;
}

function applyFiles(data: typeof filesDataSchema._output): void {
  applyFilesState(state, data);
  viewController.renderTreeUI();
  updateStatus(
    `${state.files.length} file${state.files.length === 1 ? "" : "s"} indexed`
  );
}

function applyCurrent(data: CurrentData): void {
  const previousCurrent = state.current;
  const previousLocalSelection = state.localSelectionInFlight;
  applyCurrentState(state, data);
  const path = data.file?.path || "";
  localStorage.setItem(STORAGE.currentPath, path);
  navigation.syncLocationPath(path);
  viewController.renderTreeUI();
  // Reuse the existing Markdown DOM when the server only changed content inside
  // the preview shell. Full replacement is still required when feature toggles
  // or incompatible preview states make morphing unsafe.
  if (
    !forceMarkdownPreviewReplace &&
    viewController.morphMarkdownPreview(previousCurrent, state.current)
  ) {
    viewController.queueApplySeek();
  } else {
    forceMarkdownPreviewReplace = false;
    viewController.renderPreviewUI();
  }
  if (previousLocalSelection && previousLocalSelection !== path) {
    updateStatus(`Switched externally to ${path || "no file"}.`);
  } else if (!state.statusMessage || previousLocalSelection) {
    updateStatus(path ? `Selected ${path}` : "");
  }
}

function applySeek(data: typeof seekDataSchema._output | null): void {
  applySeekState(state, data);
  if (state.settings.seek_enabled) {
    viewController.queueApplySeek();
  }
}

function applySettings(data: SettingsData): void {
  const previousSidebarCollapsed = state.sidebarCollapsed;
  applySettingsState(
    state,
    data,
    readStoredString(STORAGE.theme, storedThemeSchema, "system")
  );
  if (
    isMobileLayout() &&
    data.settings.sidebar_collapsed === false &&
    previousSidebarCollapsed
  ) {
    state.sidebarCollapsed = true;
    state.settings.sidebar_collapsed = true;
  }
  state.bootstrapFailed = false;
  applyTheme(state.theme);
  applyMarkdownTheme(state.previewTheme);
  renderSidebarShell(elements, state);
  renderConnectionBanner(elements, state);
  viewController.renderSettingsUI();
  if (state.current?.file) {
    viewController.renderPreviewUI();
  }
}

function applyHealth(data: typeof healthDataSchema._output): void {
  applyHealthState(state, data);
  viewController.renderSettingsUI();
}

function applyLogs(data: LogData): void {
  applyLogsState(state, data);
  viewController.renderSettingsUI();
}

function handleRenderStarted(incoming: CurrentData): void {
  const previous = state.current;
  const keepExistingPreview =
    previous !== null &&
    incoming.transient === true &&
    previous.file?.path === incoming.file?.path &&
    Boolean(previous.preview?.html) &&
    previous.preview.status !== "error";

  // `render_started` arrives before the final preview payload. For transient
  // editor updates, keep showing the last good preview until the matching
  // settled event lands so the UI does not flicker to an empty shell.
  if (keepExistingPreview && previous) {
    state.current = {
      ...incoming,
      preview: previous.preview,
    };
  } else {
    state.current = incoming;
    viewController.renderPreviewUI();
  }
  updateStatus(`Rendering ${incoming.file?.path || "file"}...`);
}

async function setCurrent(path: string): Promise<void> {
  state.localSelectionInFlight = path;
  updateStatus(`Loading ${path}...`);
  const result = await apiFetch("/api/current", currentDataSchema, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ path, origin: "web" }),
  });
  if (!result.ok) {
    state.localSelectionInFlight = "";
    setClientError(result.error.message);
    return;
  }
  clearClientError();
  applyCurrent(result.data);
  if (isMobileLayout()) {
    updateSidebarCollapsed(true);
  }
  navigation.navigateToFilePath(path, { replace: true });
}

async function loadInitialState(): Promise<void> {
  const initialRoute = navigation.readCurrentRoute();
  const [health, files, current, logs, seek, settings] = await Promise.all([
    apiFetch("/api/health", healthDataSchema),
    apiFetch("/api/files", filesDataSchema),
    apiFetch("/api/current", currentDataSchema),
    apiFetch("/api/logs", logDataSchema),
    apiFetch("/api/seek", seekDataSchema),
    apiFetch("/api/settings", settingsDataSchema),
  ]);

  if (!health.ok) throw new Error(health.error.message);
  if (!files.ok) throw new Error(files.error.message);
  if (!current.ok) throw new Error(current.error.message);
  if (!logs.ok) throw new Error(logs.error.message);
  if (!seek.ok) throw new Error(seek.error.message);
  if (!settings.ok) throw new Error(settings.error.message);

  clearClientError();
  applyHealth(health.data);
  applyFiles(files.data);
  applyCurrent(current.data);
  applyLogs(logs.data);
  applySeek(seek.data);
  applySettings(settings.data);
  navigation.applyRoute(initialRoute);

  const initialSettings = settings.data.settings;
  if (
    resolveThemeMode(state.theme) !== initialSettings.theme ||
    state.previewTheme !== initialSettings.preview_theme ||
    state.settings.typst_preview_theme !== initialSettings.typst_preview_theme
  ) {
    await syncSettings({ rerenderTypst: current.data.file?.kind === "typst" });
  }

  const storedPath = readStoredString(
    STORAGE.currentPath,
    currentPathStorageSchema,
    ""
  );
  const routedPath = initialRoute.path;
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

function fileExists(path: string): boolean {
  return state.files.some((file) => file.path === path);
}
