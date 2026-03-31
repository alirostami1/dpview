import type { ApiError } from "./generated/contracts";

export type {
  Route,
  PreviewTheme,
  ResolvedTheme,
  SettingsPayload,
  StoredTheme,
} from "./local-types";
export {
  currentPathStorageSchema,
  expandedPathsStorageSchema,
  previewThemeSchema,
  searchStorageSchema,
  settingsPayloadSchema,
  storedThemeSchema,
} from "./local-types";

export type {
  CurrentData,
  FileInfo,
  FileKind,
  FilesData,
  FrontMatter,
  FrontMatterEntry,
  HealthData,
  Limits,
  LogData,
  LogEntry,
  Preview,
  RenderStatus,
  RendererStatus,
  SeekData,
  Settings,
  SettingsData,
  TreeNode,
  WatcherStatus,
} from "./generated/contracts";

export {
  apiErrorSchema,
  currentDataSchema,
  fileInfoSchema,
  fileKindSchema,
  filesDataSchema,
  frontMatterEntrySchema,
  frontMatterSchema,
  healthDataSchema,
  limitsSchema,
  logDataSchema,
  logEntrySchema,
  previewSchema,
  renderStatusSchema,
  rendererStatusSchema,
  seekDataSchema,
  settingsDataSchema,
  watcherStatusSchema,
} from "./generated/contracts";

export type ApiResult<T> =
  | { ok: true; data: T }
  | { ok: false; error: ApiError };
