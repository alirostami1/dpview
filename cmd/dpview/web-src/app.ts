import { applyPreviewSeek } from "./seek";
import { encodeAppPath, parseRoute } from "./routes";
import { apiFetch } from "./api";
import { requiredElement, requiredSelector } from "./dom";
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
    bindFrontMatterState,
    renderHealth,
    renderLogs,
    renderNotFound,
    renderConnectionBanner,
    renderPreview,
    renderSidebarShell,
    renderStatus,
    renderTree,
    rememberFrontMatterState,
    setPage,
} from "./render";
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
import { parseEventData } from "./validation";
import type {
    ApiResult,
    CurrentData,
    FrontMatter,
    LogData,
    PreviewTheme,
    ResolvedTheme,
    SettingsData,
    SettingsPayload,
    StoredTheme,
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

const systemThemeMedia =
    window.matchMedia?.("(prefers-color-scheme: dark)") || null;

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
    logsEl: requiredElement<HTMLElement>("logs"),
    searchInput: requiredElement<HTMLInputElement>("search"),
    sidebarFilesTab: requiredElement<HTMLButtonElement>("sidebar-files-tab"),
    sidebarSettingsTab: requiredElement<HTMLButtonElement>("sidebar-settings-tab"),
    sidebarFilesViewEl: requiredElement<HTMLElement>("sidebar-files-view"),
    sidebarSettingsViewEl: requiredElement<HTMLElement>("sidebar-settings-view"),
    pauseRefreshInput: requiredElement<HTMLInputElement>("pause-refresh"),
    themeSelect: requiredElement<HTMLSelectElement>("theme"),
    previewThemeSelect: requiredElement<HTMLSelectElement>("preview-theme"),
    typstPreviewThemeInput: requiredElement<HTMLInputElement>("typst-preview-theme"),
    editorFileSyncInput: requiredElement<HTMLInputElement>("editor-file-sync"),
    liveBufferPreviewInput: requiredElement<HTMLInputElement>("live-buffer-preview"),
    seekEnabledInput: requiredElement<HTMLInputElement>("seek-enabled"),
    markdownFrontMatterVisibleInput: requiredElement<HTMLInputElement>("markdown-frontmatter-visible"),
    markdownFrontMatterExpandedInput: requiredElement<HTMLInputElement>("markdown-frontmatter-expanded"),
    markdownFrontMatterTitleInput: requiredElement<HTMLInputElement>("markdown-frontmatter-title"),
    clearLogsButton: requiredElement<HTMLButtonElement>("clear-logs"),
    toggleSidebarButton: requiredElement<HTMLButtonElement>("toggle-sidebar"),
    showSidebarButton: requiredElement<HTMLButtonElement>("show-sidebar"),
    goHomeButton: requiredElement<HTMLButtonElement>("go-home"),
    notFoundMessageEl: requiredElement<HTMLElement>("not-found-message"),
    fileViewEl: requiredElement<HTMLElement>("file-view"),
    notFoundViewEl: requiredElement<HTMLElement>("not-found-view"),
};

const initialTheme = readStoredString(STORAGE.theme, storedThemeSchema, "system");
const initialPreviewTheme = parsePreviewTheme(elements.previewThemeSelect.value);

const state: State = {
    files: [],
    tree: [],
    current: null,
    settings: {
        auto_refresh_paused: false,
        sidebar_collapsed: false,
        editor_file_sync_enabled: true,
        live_buffer_preview_enabled: false,
        seek_enabled: true,
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
    expanded: new Set(readStoredJSON(STORAGE.expanded, expandedPathsStorageSchema, [])),
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
    bootstrapFailed: false,
    pendingSeekFrame: 0,
};

let eventSource: EventSource | null = null;
let reconnectTimer = 0;

initializeUI();
bindUIEvents();
void bootstrap();

function initializeUI(): void {
    elements.searchInput.value = state.search;
    elements.themeSelect.value = state.theme;
    elements.previewThemeSelect.value = state.previewTheme;
    elements.statusEl.setAttribute("role", "status");
    elements.statusEl.setAttribute("aria-live", "polite");
    elements.showSidebarButton.setAttribute("aria-controls", "sidebar");
    elements.toggleSidebarButton.setAttribute("aria-controls", "sidebar");
    elements.sidebarFilesTab.setAttribute("aria-controls", "sidebar-files-view");
    elements.sidebarSettingsTab.setAttribute("aria-controls", "sidebar-settings-view");
    elements.previewEl.tabIndex = -1;
    applyTheme(state.theme);
    applyMarkdownTheme(state.previewTheme);
    renderStatus(elements, state);
    renderConnectionBanner(elements, state);
    renderHealth(elements, state);
    renderLogs(elements, state);
    renderSidebarShell(elements, state);
    renderTreeUI();
}

function bindUIEvents(): void {
    elements.searchInput.addEventListener("input", () => {
        setSearch(state, elements.searchInput.value.trim().toLowerCase());
        localStorage.setItem(STORAGE.search, state.search);
        renderTreeUI();
    });

    elements.sidebarFilesTab.addEventListener("click", () => updateSidebarMode("files"));
    elements.sidebarSettingsTab.addEventListener("click", () => updateSidebarMode("settings"));
    elements.goHomeButton.addEventListener("click", () => navigateToCurrentFile());
    elements.toggleSidebarButton.addEventListener("click", () => updateSidebarCollapsed(true));
    elements.showSidebarButton.addEventListener("click", () => updateSidebarCollapsed(false));
    elements.retryConnectionButton.addEventListener("click", () => {
        void retryApplicationState();
    });
    elements.clearLogsButton.addEventListener("click", async () => {
        const result = await apiFetch("/api/logs", logDataSchema, { method: "DELETE" });
        if (!result.ok) {
            setClientError(result.error.message);
            return;
        }
        clearClientError();
        applyLogs(result.data);
        updateStatus("Log cleared.");
    });
    window.addEventListener("popstate", () => {
        applyRoute(parseRoute(window.location.pathname));
    });
    window.addEventListener("resize", () => {
        if (state.settings.seek_enabled) {
            queueApplySeek();
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
            if (target instanceof HTMLInputElement || target instanceof HTMLTextAreaElement || target instanceof HTMLSelectElement) {
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
            const firstItem = elements.treeEl.querySelector<HTMLButtonElement>("[data-tree-item='true']");
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
        const nextPreviewTheme = previewThemeSchema.safeParse(elements.previewThemeSelect.value);
        if (!nextPreviewTheme.success) {
            updateStatus(`Unknown preview theme: ${elements.previewThemeSelect.value}`);
            elements.previewThemeSelect.value = state.previewTheme;
            return;
        }
        setPreviewThemePreference(state, nextPreviewTheme.data);
        applyMarkdownTheme(state.previewTheme);
        renderPreviewUI();
        await syncSettings({ rerenderTypst: true });
    });

    elements.typstPreviewThemeInput.addEventListener("change", async () => {
        const previous = state.settings.typst_preview_theme;
        state.settings.typst_preview_theme = elements.typstPreviewThemeInput.checked;
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
        state.settings.editor_file_sync_enabled = elements.editorFileSyncInput.checked;
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
        state.settings.live_buffer_preview_enabled = elements.liveBufferPreviewInput.checked;
        const result = await syncSettings({
            rerenderTypst: state.current?.transient === true && state.current.file?.kind === "typst",
            rerenderMarkdown: state.current?.transient === true && state.current.file?.kind === "markdown",
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
            queueApplySeek();
        }
        updateStatus("Settings updated.");
    });

    elements.markdownFrontMatterVisibleInput.addEventListener("change", async () => {
        const previous = state.settings.markdown_frontmatter_visible;
        state.settings.markdown_frontmatter_visible = elements.markdownFrontMatterVisibleInput.checked;
        const result = await syncSettings();
        if (!result.ok) {
            state.settings.markdown_frontmatter_visible = previous;
            elements.markdownFrontMatterVisibleInput.checked = previous;
            return;
        }
        renderPreviewUI();
        updateStatus("Settings updated.");
    });

    elements.markdownFrontMatterExpandedInput.addEventListener("change", async () => {
        const previous = state.settings.markdown_frontmatter_expanded;
        state.settings.markdown_frontmatter_expanded = elements.markdownFrontMatterExpandedInput.checked;
        const result = await syncSettings();
        if (!result.ok) {
            state.settings.markdown_frontmatter_expanded = previous;
            elements.markdownFrontMatterExpandedInput.checked = previous;
            return;
        }
        renderPreviewUI();
        updateStatus("Settings updated.");
    });

    elements.markdownFrontMatterTitleInput.addEventListener("change", async () => {
        const previous = state.settings.markdown_frontmatter_title;
        state.settings.markdown_frontmatter_title = elements.markdownFrontMatterTitleInput.checked;
        const result = await syncSettings({ rerenderMarkdown: true });
        if (!result.ok) {
            state.settings.markdown_frontmatter_title = previous;
            elements.markdownFrontMatterTitleInput.checked = previous;
            return;
        }
        updateStatus("Settings updated.");
    });

    elements.treeEl.addEventListener("keydown", handleTreeKeydown);

    if (systemThemeMedia) {
        systemThemeMedia.addEventListener("change", async () => {
            if (state.theme === "system") {
                applyTheme("system");
                await syncSettings({ rerenderTypst: true });
            }
        });
    }
}

async function bootstrap(): Promise<void> {
    try {
        state.bootstrapFailed = false;
        await loadInitialState();
        connectEvents();
        renderConnectionBanner(elements, state);
    } catch (error) {
        const message = error instanceof Error ? error.message : "Failed to load application state";
        state.bootstrapFailed = true;
        setLastError(state, message);
        setConnectionState(state, "degraded");
        renderStatus(elements, state);
        renderConnectionBanner(elements, state);
        updateSidebarMode("settings");
    }
}

async function retryApplicationState(): Promise<void> {
    closeEventStream();
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

function applyTheme(theme: StoredTheme): void {
    document.body.dataset.theme = resolveThemeMode(theme);
}

function applyMarkdownTheme(theme: string): void {
    elements.markdownThemeCSS.href = `/themes/markdown/${theme}.css`;
}

function updateSidebarCollapsed(collapsed: boolean): void {
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

function renderTreeUI(): void {
    renderTree(elements, state, {
        onToggleFolder: (path) => {
            if (state.expanded.has(path)) {
                state.expanded.delete(path);
            } else {
                state.expanded.add(path);
            }
            localStorage.setItem(STORAGE.expanded, JSON.stringify([...state.expanded]));
            renderTreeUI();
        },
        onSelectFile: (path) => {
            void setCurrent(path);
        },
    });
}

function renderPreviewUI(): void {
    rememberFrontMatterState(elements, state);
    const renderResult = renderPreview(elements, state);
    elements.previewEl.setAttribute(
        "aria-busy",
        String(state.current?.preview.status === "rendering"),
    );
    bindFrontMatterState(elements, state);
    if (renderResult.markdownRoot) {
        renderMarkdownMath(renderResult.markdownRoot);
    }
    if (renderResult.serverContentEl) {
        queueApplySeek();
    }
}

function renderSettingsUI(): void {
    renderStatus(elements, state);
    renderHealth(elements, state);
    renderLogs(elements, state);
    elements.themeSelect.value = state.theme;
    elements.previewThemeSelect.value = state.previewTheme;
    elements.pauseRefreshInput.checked = state.settings.auto_refresh_paused;
    elements.editorFileSyncInput.checked = state.settings.editor_file_sync_enabled;
    elements.liveBufferPreviewInput.checked = state.settings.live_buffer_preview_enabled;
    elements.seekEnabledInput.checked = state.settings.seek_enabled;
    elements.typstPreviewThemeInput.checked = state.settings.typst_preview_theme;
    elements.markdownFrontMatterVisibleInput.checked = state.settings.markdown_frontmatter_visible;
    elements.markdownFrontMatterExpandedInput.checked = state.settings.markdown_frontmatter_expanded;
    elements.markdownFrontMatterTitleInput.checked = state.settings.markdown_frontmatter_title;
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

function currentSettingsPayload(): SettingsPayload {
    return settingsPayloadSchema.parse({
        auto_refresh_paused: state.settings.auto_refresh_paused,
        sidebar_collapsed: state.sidebarCollapsed,
        editor_file_sync_enabled: state.settings.editor_file_sync_enabled,
        live_buffer_preview_enabled: state.settings.live_buffer_preview_enabled,
        seek_enabled: state.settings.seek_enabled,
        typst_preview_theme: state.settings.typst_preview_theme,
        markdown_frontmatter_visible: state.settings.markdown_frontmatter_visible,
        markdown_frontmatter_expanded: state.settings.markdown_frontmatter_expanded,
        markdown_frontmatter_title: state.settings.markdown_frontmatter_title,
        theme: resolveThemeMode(state.theme),
        preview_theme: state.previewTheme,
    });
}

async function refreshCurrentPreview(): Promise<ApiResult<CurrentData>> {
    const result = await apiFetch("/api/refresh", currentDataSchema, { method: "POST" });
    if (!result.ok) {
        setClientError(result.error.message);
        return result;
    }
    applyCurrent(result.data);
    return result;
}

async function syncSettings(
    options: { rerenderTypst?: boolean; rerenderMarkdown?: boolean } = {},
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
    renderTreeUI();
    updateStatus(`${state.files.length} file${state.files.length === 1 ? "" : "s"} indexed`);
}

function applyCurrent(data: CurrentData): void {
    const previousLocalSelection = state.localSelectionInFlight;
    applyCurrentState(state, data);
    const path = data.file?.path || "";
    localStorage.setItem(STORAGE.currentPath, path);
    syncLocationPath(path);
    renderTreeUI();
    renderPreviewUI();
    if (previousLocalSelection && previousLocalSelection !== path) {
        updateStatus(`Switched externally to ${path || "no file"}.`);
    } else if (!state.statusMessage || previousLocalSelection) {
        updateStatus(path ? `Selected ${path}` : "");
    }
}

function applySeek(data: typeof seekDataSchema._output | null): void {
    applySeekState(state, data);
    if (state.settings.seek_enabled) {
        queueApplySeek();
    }
}

function applySettings(data: SettingsData): void {
    applySettingsState(state, data, readStoredString(STORAGE.theme, storedThemeSchema, "system"));
    state.bootstrapFailed = false;
    applyTheme(state.theme);
    applyMarkdownTheme(state.previewTheme);
    renderSidebarShell(elements, state);
    renderConnectionBanner(elements, state);
    renderSettingsUI();
    if (state.current?.file) {
        renderPreviewUI();
    }
}

function applyHealth(data: typeof healthDataSchema._output): void {
    applyHealthState(state, data);
    renderHealth(elements, state);
}

function applyLogs(data: LogData): void {
    applyLogsState(state, data);
    renderLogs(elements, state);
}

function queueApplySeek(): void {
    if (state.pendingSeekFrame) {
        cancelAnimationFrame(state.pendingSeekFrame);
    }
    state.pendingSeekFrame = requestAnimationFrame(() => {
        state.pendingSeekFrame = 0;
        applyPreviewSeek(elements.fileViewEl, elements.previewEl, state.current, state.seek, state.settings);
    });
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
    navigateToFilePath(path, { replace: true });
}

async function loadInitialState(): Promise<void> {
    const initialRoute = parseRoute(window.location.pathname);
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
    applyRoute(initialRoute);

    const initialSettings = settings.data.settings;
    if (
        resolveThemeMode(state.theme) !== initialSettings.theme ||
        state.previewTheme !== initialSettings.preview_theme ||
        state.settings.typst_preview_theme !== initialSettings.typst_preview_theme
    ) {
        await syncSettings({ rerenderTypst: current.data.file?.kind === "typst" });
    }

    const storedPath = readStoredString(STORAGE.currentPath, currentPathStorageSchema, "");
    const routedPath = initialRoute.path;
    const preferredPath = routedPath || (!current.data.current && storedPath ? storedPath : "");
    if (preferredPath && preferredPath !== current.data.file?.path && fileExists(preferredPath)) {
        await setCurrent(preferredPath);
    }
}

function connectEvents(): void {
    closeEventStream();
    startEventStream(0);
}

function startEventStream(attempt: number): void {
    setConnectionState(state, "connecting", attempt);
    renderStatus(elements, state);

    const source = new EventSource("/events");
    eventSource = source;

    source.onopen = () => {
        if (eventSource !== source) {
            return;
        }
        state.bootstrapFailed = false;
        setConnectionState(state, "live", 0);
        clearClientError();
        renderStatus(elements, state);
        renderConnectionBanner(elements, state);
    };

    source.addEventListener("files_changed", (event) => {
        handleEvent(source, attempt, () => applyFiles(parseEventData(event, "files_changed", filesDataSchema)));
    });
    source.addEventListener("current_changed", (event) => {
        handleEvent(source, attempt, () => applyCurrent(parseEventData(event, "current_changed", currentDataSchema)));
    });
    source.addEventListener("preview_updated", (event) => {
        handleEvent(source, attempt, () => applyCurrent(parseEventData(event, "preview_updated", currentDataSchema)));
    });
    source.addEventListener("seek_changed", (event) => {
        handleEvent(source, attempt, () => applySeek(parseEventData(event, "seek_changed", seekDataSchema)));
    });
    source.addEventListener("logs_changed", (event) => {
        handleEvent(source, attempt, () => applyLogs(parseEventData(event, "logs_changed", logDataSchema)));
    });
    source.addEventListener("render_started", (event) => {
        handleEvent(source, attempt, () => {
            const incoming = parseEventData(event, "render_started", currentDataSchema);
            const previous = state.current;
            const keepExistingPreview =
                previous !== null &&
                incoming.transient === true &&
                previous?.file?.path === incoming.file?.path &&
                Boolean(previous?.preview?.html) &&
                previous.preview.status !== "error";

            if (keepExistingPreview && previous) {
                state.current = {
                    ...incoming,
                    preview: previous.preview,
                };
            } else {
                state.current = incoming;
                renderPreviewUI();
            }
            updateStatus(`Rendering ${incoming.file?.path || "file"}...`);
        });
    });
    source.addEventListener("render_failed", (event) => {
        handleEvent(source, attempt, () => applyCurrent(parseEventData(event, "render_failed", currentDataSchema)));
    });
    source.addEventListener("settings_changed", (event) => {
        handleEvent(source, attempt, () => applySettings(parseEventData(event, "settings_changed", settingsDataSchema)));
    });
    source.onerror = () => {
        if (eventSource !== source) {
            return;
        }
        setClientError("Live updates disconnected.");
        scheduleReconnect(source, attempt + 1);
    };
}

function handleEvent(source: EventSource, attempt: number, handler: () => void): void {
    if (eventSource !== source) {
        return;
    }
    try {
        handler();
    } catch (error) {
        const message = error instanceof Error ? error.message : "Invalid live update payload.";
        setClientError(message);
        scheduleReconnect(source, attempt + 1);
    }
}

function scheduleReconnect(source: EventSource, attempt: number): void {
    if (eventSource !== source) {
        return;
    }
    source.close();
    if (eventSource === source) {
        eventSource = null;
    }
    setConnectionState(state, "degraded", attempt);
    renderStatus(elements, state);
    renderConnectionBanner(elements, state);
    window.clearTimeout(reconnectTimer);
    const delay = Math.min(1000 * Math.max(1, attempt), 5000);
    reconnectTimer = window.setTimeout(() => {
        startEventStream(attempt);
    }, delay);
}

function closeEventStream(): void {
    if (eventSource) {
        eventSource.close();
        eventSource = null;
    }
    window.clearTimeout(reconnectTimer);
}

function fileExists(path: string): boolean {
    return state.files.some((file) => file.path === path);
}

function applyRoute(route: { kind: "file"; path: string }): void {
    if (!route.path || fileExists(route.path) || state.files.length === 0) {
        setPage(elements, "file");
        if (route.path && state.current?.file?.path !== route.path && fileExists(route.path)) {
            void setCurrent(route.path);
        } else {
            requestAnimationFrame(() => elements.previewEl.focus());
        }
        return;
    }
    renderNotFound(elements, `No previewable file exists at "${route.path}".`);
    setPage(elements, "not-found");
    requestAnimationFrame(() => elements.goHomeButton.focus());
}

function navigateToCurrentFile(options: { replace?: boolean } = {}): void {
    navigateToFilePath(state.current?.file?.path || "", options);
}

function navigateToFilePath(
    path: string,
    options: { replace?: boolean; apply?: boolean } = {},
): void {
    const { replace = false, apply = true } = options;
    const url = new URL(window.location.href);
    url.pathname = encodeAppPath(path || "");
    url.search = "";
    if (replace) {
        window.history.replaceState({}, "", url);
    } else {
        window.history.pushState({}, "", url);
    }
    if (apply) {
        applyRoute(parseRoute(url.pathname));
    }
}

function syncLocationPath(path: string): void {
    const url = new URL(window.location.href);
    url.pathname = encodeAppPath(path || "");
    url.search = "";
    window.history.replaceState({}, "", url);
}

function handleTreeKeydown(event: KeyboardEvent): void {
    const target = event.target;
    if (!(target instanceof HTMLButtonElement) || target.dataset.treeItem !== "true") {
        return;
    }

    const items = [...elements.treeEl.querySelectorAll<HTMLButtonElement>("[data-tree-item='true']")];
    const index = items.indexOf(target);
    if (index === -1) {
        return;
    }

    const focusAt = (nextIndex: number) => {
        const item = items[nextIndex];
        if (item) {
            for (const treeItem of items) {
                treeItem.tabIndex = -1;
            }
            item.tabIndex = 0;
            item.focus();
        }
    };

    switch (event.key) {
        case "ArrowDown":
            event.preventDefault();
            focusAt(Math.min(items.length - 1, index + 1));
            return;
        case "ArrowUp":
            event.preventDefault();
            focusAt(Math.max(0, index - 1));
            return;
        case "Home":
            event.preventDefault();
            focusAt(0);
            return;
        case "End":
            event.preventDefault();
            focusAt(items.length - 1);
            return;
        case "ArrowRight":
            if (target.dataset.treeKind === "folder" && target.dataset.treeOpen === "false") {
                event.preventDefault();
                target.click();
            }
            return;
        case "ArrowLeft":
            if (target.dataset.treeKind === "folder" && target.dataset.treeOpen === "true") {
                event.preventDefault();
                target.click();
                return;
            }
            if (target.dataset.treeParentPath) {
                const parent = elements.treeEl.querySelector<HTMLButtonElement>(
                    `[data-tree-kind='folder'][data-tree-path='${CSS.escape(target.dataset.treeParentPath)}']`,
                );
                if (parent) {
                    event.preventDefault();
                    parent.focus();
                }
            }
            return;
        case "Enter":
        case " ":
            event.preventDefault();
            target.click();
            return;
        default:
            return;
    }
}
