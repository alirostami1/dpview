import type { ConnectionStatus, SidebarMode, State } from "./model";
import { previewThemeSchema } from "./local-types";
import type {
  CurrentData,
  FilesData,
  HealthData,
  LogData,
  PreviewTheme,
  SeekData,
  SettingsData,
  StoredTheme,
} from "./types";

/** Applies a new file index snapshot to local state. */
export function applyFiles(state: State, data: FilesData): void {
  state.files = data.files;
  state.tree = data.tree;
}

/** Applies a new current-file snapshot to local state. */
export function applyCurrent(state: State, data: CurrentData): void {
  const previousPath = state.current?.file?.path || "";
  state.current = data;
  if (data.preview.status !== "rendering") {
    state.lastSettledCurrent = data;
  }
  if ((data.file?.path || "") !== previousPath) {
    state.frontMatterExpanded = null;
  }
  state.localSelectionInFlight = "";
}

/** Applies a new seek snapshot to local state. */
export function applySeek(state: State, data: SeekData | null): void {
  state.seek = data;
}

/** Applies a new settings snapshot to local state. */
export function applySettings(
  state: State,
  data: SettingsData,
  storedTheme: StoredTheme
): void {
  state.settings = data.settings;
  state.theme = storedTheme;
  state.previewTheme = previewThemeSchema.safeParse(data.settings.preview_theme)
    .success
    ? previewThemeSchema.parse(data.settings.preview_theme)
    : "default";
  state.sidebarCollapsed = data.settings.sidebar_collapsed;
  if (!state.settings.seek_enabled) {
    state.seek = null;
  }
}

/** Applies a new health snapshot to local state. */
export function applyHealth(state: State, data: HealthData): void {
  state.health = data;
}

/** Applies a new runtime log snapshot to local state. */
export function applyLogs(state: State, data: LogData): void {
  state.logs = data;
}

/** Updates the transient status message. */
export function setStatus(state: State, message: string): void {
  state.statusMessage = message;
}

/** Updates the current structured client-side error. */
export function setLastError(state: State, message: string): void {
  state.lastError = message;
}

/** Clears the current structured client-side error. */
export function clearLastError(state: State): void {
  state.lastError = "";
}

/** Updates live connection state. */
export function setConnectionState(
  state: State,
  connectionStatus: ConnectionStatus,
  attempts = state.connectionAttempts,
  reconnectAt = 0
): void {
  state.connectionStatus = connectionStatus;
  state.connectionAttempts = attempts;
  state.reconnectAt = reconnectAt;
}

/** Updates the sidebar search term. */
export function setSearch(state: State, search: string): void {
  state.search = search;
}

/** Updates the selected local app theme. */
export function setThemePreference(state: State, theme: StoredTheme): void {
  state.theme = theme;
}

/** Updates the selected preview theme. */
export function setPreviewThemePreference(
  state: State,
  previewTheme: PreviewTheme
): void {
  state.previewTheme = previewTheme;
}

/** Updates the sidebar collapsed flag. */
export function setSidebarCollapsed(state: State, collapsed: boolean): void {
  state.sidebarCollapsed = collapsed;
  state.settings.sidebar_collapsed = collapsed;
}

/** Updates the active sidebar mode. */
export function setSidebarMode(state: State, mode: SidebarMode): void {
  state.sidebarMode = mode;
}
