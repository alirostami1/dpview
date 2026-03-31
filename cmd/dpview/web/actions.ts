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

export function applyFiles(state: State, data: FilesData): void {
  state.files = data.files;
  state.tree = data.tree;
}

export function applyCurrent(state: State, data: CurrentData): void {
  const previousPath = state.current?.file?.path || "";
  state.current = data;
  // Rendering updates arrive before the final preview payload, so only promote
  // settled snapshots that can safely replace the cached preview.
  if (data.preview.status !== "rendering") {
    state.lastSettledCurrent = data;
  }
  if ((data.file?.path || "") !== previousPath) {
    state.frontMatterExpanded = null;
  }
  state.localSelectionInFlight = "";
}

export function applySeek(state: State, data: SeekData | null): void {
  state.seek = data;
}

export function applySettings(
  state: State,
  data: SettingsData,
  storedTheme: StoredTheme
): void {
  state.settings = data.settings;
  state.theme = storedTheme;
  // Keep a valid local preview theme even if an older server payload returns an
  // unexpected value during rollout mismatches.
  state.previewTheme = previewThemeSchema.safeParse(data.settings.preview_theme)
    .success
    ? previewThemeSchema.parse(data.settings.preview_theme)
    : "default";
  state.sidebarCollapsed = data.settings.sidebar_collapsed;
  if (!state.settings.seek_enabled) {
    state.seek = null;
  }
}

export function applyHealth(state: State, data: HealthData): void {
  state.health = data;
}

export function applyLogs(state: State, data: LogData): void {
  state.logs = data;
}

export function setStatus(state: State, message: string): void {
  state.statusMessage = message;
}

export function setLastError(state: State, message: string): void {
  state.lastError = message;
}

export function clearLastError(state: State): void {
  state.lastError = "";
}

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

export function setSearch(state: State, search: string): void {
  state.search = search;
}

export function setThemePreference(state: State, theme: StoredTheme): void {
  state.theme = theme;
}

export function setPreviewThemePreference(
  state: State,
  previewTheme: PreviewTheme
): void {
  state.previewTheme = previewTheme;
}

export function setSidebarCollapsed(state: State, collapsed: boolean): void {
  state.sidebarCollapsed = collapsed;
  state.settings.sidebar_collapsed = collapsed;
}

export function setSidebarMode(state: State, mode: SidebarMode): void {
  state.sidebarMode = mode;
}
