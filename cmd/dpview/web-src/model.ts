import type {
    CurrentData,
    FileInfo,
    HealthData,
    LogData,
    PreviewTheme,
    SeekData,
    Settings,
    StoredTheme,
    TreeNode,
} from "./types";

/** Current SSE/live-update connectivity state. */
export type ConnectionStatus = "connecting" | "live" | "degraded";

/** Active sidebar panel content. */
export type SidebarMode = "files" | "settings";

/** Main shell page currently visible in the app. */
export type Page = "file" | "not-found";

/** Central client-side state for the embedded DPview web app. */
export interface State {
    /** Flat file list returned by the backend. */
    files: FileInfo[];
    /** Sidebar tree returned by the backend. */
    tree: TreeNode[];
    /** Current file/preview snapshot. */
    current: CurrentData | null;
    /** Last non-rendering current-file snapshot kept to avoid preview flicker. */
    lastSettledCurrent: CurrentData | null;
    /** Current settings snapshot. */
    settings: Settings;
    /** Health snapshot shown on the settings screen. */
    health: HealthData | null;
    /** Recent structured Go-side runtime errors shown in settings. */
    logs: LogData | null;
    /** Current seek synchronization snapshot. */
    seek: SeekData | null;
    /** Expanded directory paths in the sidebar. */
    expanded: Set<string>;
    /** Current sidebar search term. */
    search: string;
    /** Local app theme preference, including `system`. */
    theme: StoredTheme;
    /** Current document preview theme. */
    previewTheme: PreviewTheme;
    /** Whether the sidebar is collapsed in the shell. */
    sidebarCollapsed: boolean;
    /** Active content shown in the sidebar. */
    sidebarMode: SidebarMode;
    /** Remembered front matter disclosure state for the active file. */
    frontMatterExpanded: boolean | null;
    /** Tracks user-initiated selection changes until the server confirms them. */
    localSelectionInFlight: string;
    /** Transient status message shown in settings. */
    statusMessage: string;
    /** Last structured client-side error. */
    lastError: string;
    /** Current live-update connection state. */
    connectionStatus: ConnectionStatus;
    /** Current reconnect attempt counter. */
    connectionAttempts: number;
    /** Whether the initial or latest full-state bootstrap failed. */
    bootstrapFailed: boolean;
    /** Pending animation frame used to coalesce seek updates. */
    pendingSeekFrame: number;
}

/** Optional behaviors when persisting settings to the backend. */
export interface SyncSettingsOptions {
    /** Trigger a rerender when the current file is Typst. */
    rerenderTypst?: boolean;
    /** Trigger a rerender when the current file is Markdown. */
    rerenderMarkdown?: boolean;
}

/** DOM references used by the app shell. */
export interface Elements {
    appEl: HTMLDivElement;
    sidebarEl: HTMLElement;
    treeEl: HTMLElement;
    previewEl: HTMLElement;
    connectionBannerEl: HTMLElement;
    connectionMessageEl: HTMLElement;
    retryConnectionButton: HTMLButtonElement;
    markdownThemeCSS: HTMLLinkElement;
    statusEl: HTMLElement;
    healthEl: HTMLElement;
    logsEl: HTMLTextAreaElement;
    copyLogsButton: HTMLButtonElement;
    clearLogsButton: HTMLButtonElement;
    searchInput: HTMLInputElement;
    sidebarFilesTab: HTMLButtonElement;
    sidebarSettingsTab: HTMLButtonElement;
    sidebarFilesViewEl: HTMLElement;
    sidebarSettingsViewEl: HTMLElement;
    pauseRefreshInput: HTMLInputElement;
    themeSelect: HTMLSelectElement;
    previewThemeSelect: HTMLSelectElement;
    typstPreviewThemeInput: HTMLInputElement;
    editorFileSyncInput: HTMLInputElement;
    liveBufferPreviewInput: HTMLInputElement;
    seekEnabledInput: HTMLInputElement;
    markdownFrontMatterVisibleInput: HTMLInputElement;
    markdownFrontMatterExpandedInput: HTMLInputElement;
    markdownFrontMatterTitleInput: HTMLInputElement;
    toggleSidebarButton: HTMLButtonElement;
    showSidebarButton: HTMLButtonElement;
    goHomeButton: HTMLButtonElement;
    notFoundMessageEl: HTMLElement;
    fileViewEl: HTMLElement;
    notFoundViewEl: HTMLElement;
}

/** Callbacks used by tree rendering. */
export interface TreeHandlers {
    onToggleFolder: (path: string) => void;
    onSelectFile: (path: string) => void;
}

/** Result of rendering preview shell content. */
export interface PreviewRenderResult {
    serverContentEl: HTMLElement | null;
    markdownRoot: HTMLElement | null;
}
