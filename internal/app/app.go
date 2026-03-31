package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeberg.org/aros/dpview/internal/api"
	"codeberg.org/aros/dpview/internal/files"
	"codeberg.org/aros/dpview/internal/render"
	"codeberg.org/aros/dpview/internal/state"
	"codeberg.org/aros/dpview/internal/watch"
)

type Service struct {
	files    *files.Service
	renderer *render.Service
	store    *state.Store

	mu                sync.Mutex
	watcherEnabled    atomic.Bool
	activeLivePreview *livePreviewState
	latestLivePreview map[string]int64
	logLevel          logLevel
}

type logLevel int

const (
	logLevelDebug logLevel = iota
	logLevelInfo
	logLevelError
)

type livePreviewState struct {
	path          string
	content       []byte
	sourceVersion int64
}

type LivePreviewRequest struct {
	Path    string
	Origin  string
	Content []byte
	Edits   []api.TextEdit
	Version int64
}

func New(files *files.Service, renderer *render.Service, store *state.Store, level string) *Service {
	return &Service{
		files:             files,
		renderer:          renderer,
		store:             store,
		latestLivePreview: make(map[string]int64),
		logLevel:          parseLogLevel(level),
	}
}

func (s *Service) Snapshot() state.Snapshot {
	return s.store.Snapshot()
}

func (s *Service) Subscribe() (<-chan api.Event, func()) {
	return s.store.Subscribe()
}

func (s *Service) ClearLogs() api.LogData {
	return s.store.ClearLogs()
}

func (s *Service) Health() api.HealthData {
	snap := s.store.Snapshot()
	return api.HealthData{
		Status:    "ok",
		Version:   snap.Version,
		EventID:   snap.EventID,
		Renderers: s.renderer.RendererStatuses(),
		Limits:    s.renderer.Limits(),
		Watcher:   api.WatcherStatus{Enabled: s.watcherEnabled.Load()},
	}
}

func (s *Service) SetCurrent(ctx context.Context, rel, origin string) (api.CurrentData, int, *api.Error) {
	if origin == "editor" && !s.store.Snapshot().Settings.Settings.EditorFileSyncEnabled {
		err := api.NewError("editor_file_sync_disabled", "Editor-driven file sync is disabled", "")
		s.logAPIError("current", err, rel, "editor file sync disabled")
		return api.CurrentData{}, http.StatusConflict, err
	}
	abs, info, apiErr, status := s.resolvePath(rel)
	if apiErr != nil {
		s.logAPIError("current", apiErr, rel, "resolve current path")
		return api.CurrentData{}, status, apiErr
	}
	selectionChanged := s.currentPath() != info.Path
	if selectionChanged {
		s.clearActiveLivePreviewExcept(info.Path)
	}
	live := s.activeLivePreviewForPath(info.Path)
	transient := live != nil
	sourceVersion := int64(0)
	if live != nil {
		sourceVersion = live.sourceVersion
	}
	s.logDebug(
		"current",
		"select_file",
		"Selecting current file",
		fmt.Sprintf("origin=%s transient=%t source_version=%d selection_changed=%t", valueOrDefault(origin, "api"), transient, sourceVersion, selectionChanged),
		info.Path,
		"set current",
	)
	s.store.PublishRenderStarted(&info, transient, sourceVersion)
	var preview api.Preview
	if live != nil {
		preview = s.renderer.RenderSource(ctx, info, abs, live.content, s.store.Snapshot().Settings.Settings, true)
	} else {
		preview = s.renderer.Render(ctx, info, abs, s.store.Snapshot().Settings.Settings)
	}
	if preview.Error != nil {
		s.logPreviewError("render", info.Path, preview.Error)
	}
	if origin == "" {
		origin = "api"
	}
	s.logDebug(
		"current",
		"current_ready",
		"Current file preview ready",
		fmt.Sprintf("origin=%s transient=%t status=%s duration_ms=%d", origin, transient, preview.Status, preview.RenderDurationMS),
		info.Path,
		"set current",
	)
	return s.store.SetCurrent(&info, preview, origin, selectionChanged, transient, sourceVersion), http.StatusOK, nil
}

func (s *Service) SetLivePreview(ctx context.Context, req LivePreviewRequest) (api.CurrentData, int, *api.Error) {
	settings := s.store.Snapshot().Settings.Settings
	if !settings.EditorFileSyncEnabled {
		err := api.NewError("editor_file_sync_disabled", "Editor-driven file sync is disabled", "")
		s.logAPIError("live_preview", err, req.Path, "editor file sync disabled")
		return api.CurrentData{}, http.StatusConflict, err
	}
	if !settings.LiveBufferPreviewEnabled {
		err := api.NewError("live_buffer_preview_disabled", "Live buffer preview is disabled", "")
		s.logAPIError("live_preview", err, req.Path, "live buffer preview disabled")
		return api.CurrentData{}, http.StatusConflict, err
	}
	if req.Version <= 0 {
		err := api.NewError("invalid_version", "Live preview version must be greater than zero", "")
		s.logAPIError("live_preview", err, req.Path, "invalid live preview version")
		return api.CurrentData{}, http.StatusBadRequest, err
	}

	abs, info, apiErr, status := s.resolvePath(req.Path)
	if apiErr != nil {
		s.logAPIError("live_preview", apiErr, req.Path, "resolve live preview path")
		return api.CurrentData{}, status, apiErr
	}
	if !s.reserveLivePreviewVersion(info.Path, req.Version) {
		err := api.NewError("stale_live_preview", "A newer live preview update already exists", info.Path)
		return s.store.Snapshot().Current, http.StatusConflict, err
	}

	selectionChanged := s.currentPath() != info.Path
	s.logDebug(
		"live_preview",
		"render_started",
		"Processing live preview update",
		fmt.Sprintf("origin=%s version=%d edits=%d selection_changed=%t", valueOrDefault(req.Origin, "editor"), req.Version, len(req.Edits), selectionChanged),
		info.Path,
		"live preview",
	)
	s.store.PublishRenderStarted(&info, true, req.Version)
	prevLive := s.activeLivePreviewForPath(info.Path)
	prevSource := []byte(nil)
	if prevLive != nil {
		prevSource = append([]byte(nil), prevLive.content...)
	}
	nextSource, applyErr := resolveLivePreviewSource(prevSource, req.Content, req.Edits)
	if applyErr != nil {
		err := api.NewError("invalid_live_preview_edits", "Invalid live preview edits", applyErr.Error())
		s.logAPIError("live_preview", err, info.Path, "apply live preview edits")
		return api.CurrentData{}, http.StatusBadRequest, err
	}
	preview := s.renderer.RenderSource(ctx, info, abs, nextSource, settings, true)
	if preview.Error != nil {
		s.logPreviewError("live_preview", info.Path, preview.Error)
	}
	if info.Kind == files.KindMarkdown && len(req.Edits) > 0 {
		s.logDebug(
			"live_preview",
			"markdown_full_render",
			"Rendered full markdown preview update",
			fmt.Sprintf("edits=%d bytes=%d", len(req.Edits), len(nextSource)),
			info.Path,
			"full render",
		)
	}
	if !s.isLatestLivePreviewVersion(info.Path, req.Version) {
		err := api.NewError("stale_live_preview", "Discarded an outdated live preview update", info.Path)
		return s.store.Snapshot().Current, http.StatusConflict, err
	}
	s.setActiveLivePreview(info.Path, nextSource, req.Version)
	if req.Origin == "" {
		req.Origin = "editor"
	}
	return s.store.SetCurrent(&info, preview, req.Origin, selectionChanged, true, req.Version), http.StatusOK, nil
}

func resolveLivePreviewSource(prevSource, content []byte, edits []api.TextEdit) ([]byte, error) {
	if len(edits) == 0 {
		return append([]byte(nil), content...), nil
	}
	if len(prevSource) == 0 {
		return append([]byte(nil), content...), nil
	}
	return applyTextEdits(prevSource, edits)
}

func applyTextEdits(source []byte, edits []api.TextEdit) ([]byte, error) {
	if len(edits) == 0 {
		return append([]byte(nil), source...), nil
	}
	var out bytes.Buffer
	cursor := 0
	for _, edit := range edits {
		if edit.Start < cursor || edit.Start < 0 || edit.End < edit.Start || edit.End > len(source) {
			return nil, fmt.Errorf("invalid edit range %d:%d", edit.Start, edit.End)
		}
		out.Write(source[cursor:edit.Start])
		out.WriteString(edit.Text)
		cursor = edit.End
	}
	out.Write(source[cursor:])
	return out.Bytes(), nil
}

func (s *Service) Refresh(ctx context.Context) (api.CurrentData, int, *api.Error) {
	current := s.store.Snapshot().Current.File
	if current == nil {
		err := api.NewError("no_current_file", "No current file selected", "")
		s.logAPIError("refresh", err, "", "refresh with no current file")
		return api.CurrentData{}, http.StatusBadRequest, err
	}
	abs, info, apiErr, status := s.resolvePath(current.Path)
	if apiErr != nil {
		if apiErr.Code == "file_not_found" {
			s.logAPIError("refresh", apiErr, current.Path, "refresh target missing")
			cleared := s.store.ClearCurrent(api.NewError("file_not_found", "Current file no longer exists", current.Path), "watch")
			return cleared, status, apiErr
		}
		s.logAPIError("refresh", apiErr, current.Path, "refresh resolve failed")
		return api.CurrentData{}, status, apiErr
	}
	live := s.activeLivePreviewForPath(info.Path)
	transient := live != nil
	sourceVersion := int64(0)
	if live != nil {
		sourceVersion = live.sourceVersion
	}
	s.logDebug(
		"refresh",
		"render_started",
		"Refreshing current preview",
		fmt.Sprintf("transient=%t source_version=%d", transient, sourceVersion),
		info.Path,
		"refresh",
	)
	s.store.PublishRenderStarted(&info, transient, sourceVersion)
	var preview api.Preview
	if live != nil {
		preview = s.renderer.RenderSource(ctx, info, abs, live.content, s.store.Snapshot().Settings.Settings, true)
	} else {
		preview = s.renderer.Render(ctx, info, abs, s.store.Snapshot().Settings.Settings)
	}
	if preview.Error != nil {
		s.logPreviewError("render", info.Path, preview.Error)
	}
	s.logDebug(
		"refresh",
		"refresh_ready",
		"Refresh completed",
		fmt.Sprintf("transient=%t status=%s duration_ms=%d", transient, preview.Status, preview.RenderDurationMS),
		info.Path,
		"refresh",
	)
	return s.store.SetCurrent(&info, preview, "refresh", false, transient, sourceVersion), http.StatusOK, nil
}

func (s *Service) ClearCurrent() api.CurrentData {
	s.clearActiveLivePreviewExcept("")
	return s.store.ClearCurrent(api.NewError("no_current_file", "Current file cleared", ""), "api")
}

func (s *Service) SetSeek(_ context.Context, seek api.SeekData) (api.SeekData, int, *api.Error) {
	if !s.store.Snapshot().Settings.Settings.SeekEnabled {
		err := api.NewError("seek_disabled", "Seek synchronization is disabled", "")
		s.logAPIError("seek", err, seek.Path, "seek disabled")
		return api.SeekData{}, http.StatusConflict, err
	}
	rel := strings.TrimSpace(seek.Path)
	if rel == "" {
		err := api.NewError("invalid_path", "Path is required", "")
		s.logAPIError("seek", err, rel, "seek missing path")
		return api.SeekData{}, http.StatusBadRequest, err
	}
	_, info, apiErr, status := s.resolvePath(rel)
	if apiErr != nil {
		s.logAPIError("seek", apiErr, rel, "seek resolve failed")
		return api.SeekData{}, status, apiErr
	}

	current := s.store.Snapshot().Current.File
	if current == nil || current.Path != info.Path {
		err := api.NewError("current_mismatch", "Seek path must match the current file", info.Path)
		s.logAPIError("seek", err, info.Path, "seek/current mismatch")
		return api.SeekData{}, http.StatusConflict, err
	}

	normalized := normalizeSeek(info.Path, seek)
	s.logDebug(
		"seek",
		"seek_updated",
		"Applied seek update",
		fmt.Sprintf("line=%d column=%d top=%d bottom=%d focus=%d", normalized.Line, normalized.Column, normalized.TopLine, normalized.BottomLine, normalized.FocusLine),
		info.Path,
		"seek",
	)
	return s.store.SetSeek(normalized, "api"), http.StatusOK, nil
}

func (s *Service) UpdateSettingsPatch(patch api.SettingsPatch) api.SettingsData {
	previous := s.store.Snapshot().Settings.Settings
	data := s.store.PatchSettings(patch)
	s.logDebug(
		"settings",
		"settings_updated",
		"Applied settings patch",
		describeSettingsPatch(patch),
		"",
		"settings",
	)
	if previous.SeekEnabled && patch.SeekEnabled != nil && !*patch.SeekEnabled {
		s.logDebug(
			"settings",
			"seek_cleared",
			"Cleared seek state after disabling seek synchronization",
			"",
			"",
			"settings",
		)
		s.store.ClearSeek("settings")
	}
	if previous.LiveBufferPreviewEnabled && patch.LiveBufferPreviewEnabled != nil && !*patch.LiveBufferPreviewEnabled {
		s.logDebug(
			"settings",
			"live_preview_cleared",
			"Cleared transient live preview state after disabling live buffer preview",
			"",
			"",
			"settings",
		)
		s.clearActiveLivePreviewExcept("")
		if current := s.store.Snapshot().Current; current.Current && current.Transient {
			s.logDebug(
				"settings",
				"refresh_after_live_preview_disable",
				"Refreshing current file to reconcile transient preview state",
				"",
				pathOrEmpty(current.File),
				"settings",
			)
			_, _, _ = s.Refresh(context.Background())
		}
	}
	return data
}

func (s *Service) Rescan() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	items, err := s.files.List()
	if err != nil {
		return err
	}
	tree := files.BuildTree(items)
	s.store.SetFiles(items, tree)
	return nil
}

func (s *Service) HandleWatchEvents(events []watch.Event) {
	snap := s.store.Snapshot()
	current := snap.Current.File
	treeDirty := false
	currentDirty := false

	for _, event := range events {
		if event.IsDir {
			if event.Op == watch.OpCreate || event.Op == watch.OpRemove || event.Op == watch.OpRename {
				treeDirty = true
			}
			continue
		}

		if current != nil && samePath(event.Path, filepath.Join(s.files.Root(), filepath.FromSlash(current.Path))) {
			currentDirty = true
		}
		_, previewable := s.files.IsPreviewable(filepath.Base(event.Path))
		if previewable && (event.Op == watch.OpCreate || event.Op == watch.OpRemove || event.Op == watch.OpRename) {
			treeDirty = true
		}
	}
	s.logDebug(
		"watcher",
		"watch_batch",
		"Processing filesystem watch events",
		fmt.Sprintf("events=%d tree_dirty=%t current_dirty=%t", len(events), treeDirty, currentDirty),
		pathOrEmpty(current),
		"watch",
	)

	if treeDirty {
		if err := s.Rescan(); err != nil {
			s.logInternalError("watcher", "watch rescan failed", err, "")
			return
		}
	}
	if current != nil {
		if _, _, err := s.files.Resolve(current.Path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				s.logInternalError("watcher", "current file removed", err, current.Path)
				s.clearActiveLivePreviewExcept("")
				s.store.ClearCurrent(api.NewError("file_not_found", "Current file no longer exists", current.Path), "watch")
				return
			}
			s.logInternalError("watcher", "failed to resolve current file during watch", err, current.Path)
		}
	}
	if currentDirty && !snap.Settings.Settings.AutoRefreshPaused {
		s.logDebug(
			"watcher",
			"watch_refresh",
			"Refreshing current file after watch event",
			"auto_refresh_paused=false",
			current.Path,
			"watch",
		)
		_, _, _ = s.Refresh(context.Background())
	}
}

func (s *Service) StartWatcher() (*watch.Watcher, error) {
	watcher, err := watch.New(s.files.Root(), 200*time.Millisecond, s.HandleWatchEvents)
	if err != nil {
		return nil, err
	}
	s.watcherEnabled.Store(true)
	return watcher, nil
}

func (s *Service) RecordRuntimeError(source, message string, err error) {
	s.logInternalError(source, message, err, "")
}

func (s *Service) currentPath() string {
	if current := s.store.Snapshot().Current.File; current != nil {
		return current.Path
	}
	return ""
}

func (s *Service) resolvePath(rel string) (string, files.FileInfo, *api.Error, int) {
	abs, info, err := s.files.Resolve(rel)
	if err == nil {
		return abs, info, nil, http.StatusOK
	}
	switch {
	case errors.Is(err, os.ErrNotExist):
		return "", files.FileInfo{}, api.NewError("file_not_found", "File not found", rel), http.StatusNotFound
	case errors.Is(err, files.ErrPathRequired):
		return "", files.FileInfo{}, api.NewError("invalid_path", "Path is required", ""), http.StatusBadRequest
	case errors.Is(err, files.ErrAbsolutePath), errors.Is(err, files.ErrPathTraversal), errors.Is(err, files.ErrPathOutsideRoot):
		return "", files.FileInfo{}, api.NewError("path_outside_root", "Path must stay inside the configured root", err.Error()), http.StatusBadRequest
	case errors.Is(err, files.ErrPathIsDirectory):
		return "", files.FileInfo{}, api.NewError("invalid_path", "Path points to a directory", rel), http.StatusBadRequest
	case errors.Is(err, files.ErrUnsupportedFileType):
		return "", files.FileInfo{}, api.NewError("unsupported_file_type", "Unsupported file type", rel), http.StatusBadRequest
	default:
		return "", files.FileInfo{}, api.NewError("internal_error", "Failed to resolve file", err.Error()), http.StatusInternalServerError
	}
}

func samePath(a, b string) bool {
	return filepath.Clean(a) == filepath.Clean(b) || strings.EqualFold(filepath.Clean(a), filepath.Clean(b))
}

func normalizeSeek(path string, seek api.SeekData) api.SeekData {
	line := clampSeekValue(seek.Line)
	column := clampSeekValue(seek.Column)
	top := clampSeekValue(seek.TopLine)
	bottom := clampSeekValue(seek.BottomLine)
	focus := clampSeekValue(seek.FocusLine)
	if top > 0 && bottom > 0 && bottom < top {
		top, bottom = bottom, top
	}

	switch {
	case focus > 0:
		// Prefer the editor cursor line when it is available. This lets the
		// preview align around the actual cursor position instead of the center
		// of the current Neovim viewport.
	case line > 0:
		focus = line
	case top > 0 && bottom > 0:
		focus = top + (bottom-top)/2
	case top > 0:
		focus = top
	case bottom > 0:
		focus = bottom
	}

	return api.SeekData{
		Path:       path,
		Line:       line,
		Column:     column,
		TopLine:    top,
		BottomLine: bottom,
		FocusLine:  focus,
	}
}

func describeSettingsPatch(patch api.SettingsPatch) string {
	parts := make([]string, 0, 11)
	appendBool := func(name string, value *bool) {
		if value != nil {
			parts = append(parts, fmt.Sprintf("%s=%t", name, *value))
		}
	}
	appendString := func(name string, value *string) {
		if value != nil {
			parts = append(parts, fmt.Sprintf("%s=%s", name, *value))
		}
	}
	appendBool("auto_refresh_paused", patch.AutoRefreshPaused)
	appendBool("sidebar_collapsed", patch.SidebarCollapsed)
	appendBool("editor_file_sync_enabled", patch.EditorFileSyncEnabled)
	appendBool("live_buffer_preview_enabled", patch.LiveBufferPreviewEnabled)
	appendBool("seek_enabled", patch.SeekEnabled)
	appendBool("typst_preview_theme", patch.TypstPreviewTheme)
	appendBool("markdown_frontmatter_visible", patch.MarkdownFrontMatterVisible)
	appendBool("markdown_frontmatter_expanded", patch.MarkdownFrontMatterExpanded)
	appendBool("markdown_frontmatter_title", patch.MarkdownFrontMatterTitle)
	appendString("theme", patch.Theme)
	appendString("preview_theme", patch.PreviewTheme)
	if len(parts) == 0 {
		return "no changes"
	}
	return strings.Join(parts, " ")
}

func pathOrEmpty(info *files.FileInfo) string {
	if info == nil {
		return ""
	}
	return info.Path
}

func valueOrDefault(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func (s *Service) reserveLivePreviewVersion(path string, version int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	if current := s.latestLivePreview[path]; version <= current {
		return false
	}
	s.latestLivePreview[path] = version
	return true
}

func (s *Service) isLatestLivePreviewVersion(path string, version int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.latestLivePreview[path] == version
}

func (s *Service) activeLivePreviewForPath(path string) *livePreviewState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeLivePreview == nil || s.activeLivePreview.path != path {
		return nil
	}
	copyState := *s.activeLivePreview
	copyState.content = append([]byte(nil), s.activeLivePreview.content...)
	return &copyState
}

func (s *Service) setActiveLivePreview(path string, content []byte, version int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.activeLivePreview = &livePreviewState{
		path:          path,
		content:       append([]byte(nil), content...),
		sourceVersion: version,
	}
}

func (s *Service) clearActiveLivePreviewExcept(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.activeLivePreview == nil {
		return
	}
	if path != "" && s.activeLivePreview.path == path {
		return
	}
	s.activeLivePreview = nil
}

func parseLogLevel(level string) logLevel {
	switch strings.ToLower(strings.TrimSpace(level)) {
	case "debug":
		return logLevelDebug
	case "error":
		return logLevelError
	default:
		return logLevelInfo
	}
}

func (s *Service) shouldLog(level logLevel) bool {
	return level >= s.logLevel
}

func (s *Service) appendLog(level logLevel, entry api.LogEntry) {
	if !s.shouldLog(level) {
		return
	}
	entry.Timestamp = time.Now().UTC()
	s.store.AppendLog(entry)
	log.Printf("[%s] %s %s: %s", strings.ToUpper(entry.Level), entry.Source, entry.Code, entry.Message)
}

func (s *Service) logAPIError(source string, err *api.Error, path string, context string) {
	if err == nil {
		return
	}
	s.appendLog(logLevelError, api.LogEntry{
		Level:   "error",
		Source:  source,
		Code:    err.Code,
		Message: err.Message,
		Detail:  err.Detail,
		Path:    path,
		Context: context,
	})
}

func (s *Service) logPreviewError(source, path string, err *api.Error) {
	s.logAPIError(source, err, path, "preview render failed")
}

func (s *Service) logInternalError(source, message string, err error, path string) {
	if err == nil {
		return
	}
	s.appendLog(logLevelError, api.LogEntry{
		Level:   "error",
		Source:  source,
		Code:    "internal_error",
		Message: message,
		Detail:  err.Error(),
		Path:    path,
	})
}

func (s *Service) logDebug(source, code, message, detail, path, context string) {
	s.appendLog(logLevelDebug, api.LogEntry{
		Level:   "debug",
		Source:  source,
		Code:    code,
		Message: message,
		Detail:  detail,
		Path:    path,
		Context: context,
	})
}

func clampSeekValue(v int) int {
	if v < 0 {
		return 0
	}
	return v
}

func OpenBrowser(url string) error {
	cmd := exec.Command("xdg-open", url)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}
