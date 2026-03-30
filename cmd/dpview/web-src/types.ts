export type FileKind = "markdown" | "typst";

export type Route =
  | { kind: "settings" }
  | { kind: "file"; path: string };

export interface FileInfo {
  path: string;
  name: string;
  kind: FileKind;
}

export interface TreeNode {
  name: string;
  path: string;
  kind?: FileKind;
  children?: TreeNode[];
}

export interface ApiError {
  code?: string;
  message: string;
  detail?: string;
}

export type RenderStatus = "idle" | "rendering" | "ready" | "error";

export interface FrontMatterEntry {
  key: string;
  value: string;
}

export interface FrontMatter {
  format: string;
  title?: string;
  title_used: boolean;
  entries: FrontMatterEntry[];
}

export interface Preview {
  html?: string;
  frontmatter?: FrontMatter;
  source_line_count?: number;
  updated_at?: string;
  render_duration_ms?: number;
  cache_hit?: boolean;
  status?: RenderStatus;
  error?: ApiError;
}

export interface CurrentData {
  file?: FileInfo;
  preview: Preview;
  version: number;
  event_id: number;
  current: boolean;
  origin?: string;
}

export interface SeekData {
  path?: string;
  line?: number;
  column?: number;
  top_line?: number;
  bottom_line?: number;
  focus_line?: number;
  version?: number;
  event_id?: number;
  origin?: string;
}

export interface FilesData {
  files: FileInfo[];
  tree: TreeNode[];
  version: number;
  event_id: number;
}

export interface Settings {
  auto_refresh_paused: boolean;
  sidebar_collapsed: boolean;
  editor_file_sync_enabled: boolean;
  seek_enabled: boolean;
  typst_preview_theme: boolean;
  markdown_frontmatter_visible: boolean;
  markdown_frontmatter_expanded: boolean;
  markdown_frontmatter_title: boolean;
  theme: string;
  preview_theme: string;
}

export interface SettingsData {
  settings: Settings;
  version: number;
  event_id: number;
}

export interface RendererStatus {
  kind: string;
  name: string;
  available: boolean;
  details?: Record<string, string>;
}

export interface WatcherStatus {
  enabled: boolean;
}

export interface HealthData {
  status: string;
  version: number;
  event_id: number;
  renderers: RendererStatus[];
  watcher: WatcherStatus;
}

export type ApiResult<T> = { ok: true; data: T } | { ok: false; error: ApiError };
