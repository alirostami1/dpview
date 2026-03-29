package api

import (
	"time"

	"codeberg.org/aros/dpview.git/internal/files"
)

const (
	EventFilesChanged    = "files_changed"
	EventCurrentChanged  = "current_changed"
	EventPreviewUpdated  = "preview_updated"
	EventRenderStarted   = "render_started"
	EventRenderFailed    = "render_failed"
	EventSettingsChanged = "settings_changed"
)

type Envelope struct {
	OK    bool   `json:"ok"`
	Data  any    `json:"data,omitempty"`
	Error *Error `json:"error,omitempty"`
}

type Error struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Detail  string `json:"detail,omitempty"`
}

type RenderStatus string

const (
	RenderStatusIdle      RenderStatus = "idle"
	RenderStatusRendering RenderStatus = "rendering"
	RenderStatusReady     RenderStatus = "ready"
	RenderStatusError     RenderStatus = "error"
)

type Preview struct {
	HTML             string       `json:"html,omitempty"`
	UpdatedAt        time.Time    `json:"updated_at,omitempty"`
	RenderDurationMS int64        `json:"render_duration_ms,omitempty"`
	CacheHit         bool         `json:"cache_hit"`
	Status           RenderStatus `json:"status"`
	Error            *Error       `json:"error,omitempty"`
}

type CurrentData struct {
	File    *files.FileInfo `json:"file,omitempty"`
	Preview Preview         `json:"preview"`
	Version int64           `json:"version"`
	EventID int64           `json:"event_id"`
	Current bool            `json:"current"`
	Origin  string          `json:"origin,omitempty"`
}

type FilesData struct {
	Files   []files.FileInfo `json:"files"`
	Tree    []files.TreeNode `json:"tree"`
	Version int64            `json:"version"`
	EventID int64            `json:"event_id"`
}

type Settings struct {
	AutoRefreshPaused bool   `json:"auto_refresh_paused"`
	Theme             string `json:"theme"`
	PreviewTheme      string `json:"preview_theme"`
}

type SettingsData struct {
	Settings Settings `json:"settings"`
	Version  int64    `json:"version"`
	EventID  int64    `json:"event_id"`
}

type RendererStatus struct {
	Kind      files.Kind        `json:"kind"`
	Name      string            `json:"name"`
	Available bool              `json:"available"`
	Details   map[string]string `json:"details,omitempty"`
}

type Limits struct {
	MaxFileSizeBytes int64 `json:"max_file_size_bytes"`
	RenderTimeoutMS  int64 `json:"render_timeout_ms"`
}

type WatcherStatus struct {
	Enabled bool `json:"enabled"`
}

type HealthData struct {
	Status    string           `json:"status"`
	Version   int64            `json:"version"`
	EventID   int64            `json:"event_id"`
	Renderers []RendererStatus `json:"renderers"`
	Limits    Limits           `json:"limits"`
	Watcher   WatcherStatus    `json:"watcher"`
}

type Event struct {
	Type    string `json:"type"`
	EventID int64  `json:"event_id"`
	Version int64  `json:"version"`
	Data    any    `json:"data,omitempty"`
}

func OK(data any) Envelope {
	return Envelope{OK: true, Data: data}
}

func Fail(err *Error) Envelope {
	return Envelope{OK: false, Error: err}
}

func NewError(code, message, detail string) *Error {
	return &Error{Code: code, Message: message, Detail: detail}
}
