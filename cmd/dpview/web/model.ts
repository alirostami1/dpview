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

export type ConnectionStatus = "connecting" | "live" | "degraded";

export type SidebarMode = "files" | "settings";

export type Page = "file" | "not-found";

export interface State {
  files: FileInfo[];
  tree: TreeNode[];
  current: CurrentData | null;
  // Preserve the last settled preview while transient updates are still rendering.
  lastSettledCurrent: CurrentData | null;
  settings: Settings;
  health: HealthData | null;
  logs: LogData | null;
  seek: SeekData | null;
  expanded: Set<string>;
  search: string;
  theme: StoredTheme;
  previewTheme: PreviewTheme;
  sidebarCollapsed: boolean;
  sidebarMode: SidebarMode;
  // `null` means "follow the current settings default" until the user toggles it.
  frontMatterExpanded: boolean | null;
  // Used to distinguish a local selection request from a later external file switch.
  localSelectionInFlight: string;
  statusMessage: string;
  lastError: string;
  connectionStatus: ConnectionStatus;
  connectionAttempts: number;
  reconnectAt: number;
  bootstrapFailed: boolean;
  // Coalesces rapid seek updates so preview scrolling only runs once per frame.
  pendingSeekFrame: number;
}

export interface SyncSettingsOptions {
  rerenderTypst?: boolean;
  rerenderMarkdown?: boolean;
}

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
  latexEnabledInput: HTMLInputElement;
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

export interface TreeHandlers {
  onToggleFolder: (path: string) => void;
  onSelectFile: (path: string) => void;
}

export interface PreviewRenderResult {
  serverContentEl: HTMLElement | null;
  markdownRoot: HTMLElement | null;
}
