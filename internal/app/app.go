package app

import (
	"context"
	"errors"
	"fmt"
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

	mu             sync.Mutex
	watcherEnabled atomic.Bool
}

func New(files *files.Service, renderer *render.Service, store *state.Store) *Service {
	return &Service{files: files, renderer: renderer, store: store}
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
	s.store.PublishRenderStarted(&info)
	preview := s.renderer.Render(ctx, info, abs, s.store.Snapshot().Settings.Settings)
	if preview.Error != nil {
		s.logPreviewError("render", info.Path, preview.Error)
	}
	selectionChanged := s.currentPath() != info.Path
	if origin == "" {
		origin = "api"
	}
	return s.store.SetCurrent(&info, preview, origin, selectionChanged), http.StatusOK, nil
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
	s.store.PublishRenderStarted(&info)
	preview := s.renderer.Render(ctx, info, abs, s.store.Snapshot().Settings.Settings)
	if preview.Error != nil {
		s.logPreviewError("render", info.Path, preview.Error)
	}
	return s.store.SetCurrent(&info, preview, "refresh", false), http.StatusOK, nil
}

func (s *Service) ClearCurrent() api.CurrentData {
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

	return s.store.SetSeek(normalizeSeek(info.Path, seek), "api"), http.StatusOK, nil
}

func (s *Service) UpdateSettingsPatch(patch api.SettingsPatch) api.SettingsData {
	previous := s.store.Snapshot().Settings.Settings
	data := s.store.PatchSettings(patch)
	if previous.SeekEnabled && patch.SeekEnabled != nil && !*patch.SeekEnabled {
		s.store.ClearSeek("settings")
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
				s.store.ClearCurrent(api.NewError("file_not_found", "Current file no longer exists", current.Path), "watch")
				return
			}
			s.logInternalError("watcher", "failed to resolve current file during watch", err, current.Path)
		}
	}
	if currentDirty && !snap.Settings.Settings.AutoRefreshPaused {
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
	if top > 0 && bottom > 0 && bottom < top {
		top, bottom = bottom, top
	}

	focus := 0
	switch {
	case top > 0 && bottom > 0:
		focus = top + (bottom-top)/2
	case line > 0:
		focus = line
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

func (s *Service) logAPIError(source string, err *api.Error, path string, context string) {
	if err == nil {
		return
	}
	s.store.AppendLog(api.LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     "error",
		Source:    source,
		Code:      err.Code,
		Message:   err.Message,
		Detail:    err.Detail,
		Path:      path,
		Context:   context,
	})
}

func (s *Service) logPreviewError(source, path string, err *api.Error) {
	s.logAPIError(source, err, path, "preview render failed")
}

func (s *Service) logInternalError(source, message string, err error, path string) {
	if err == nil {
		return
	}
	s.store.AppendLog(api.LogEntry{
		Timestamp: time.Now().UTC(),
		Level:     "error",
		Source:    source,
		Code:      "internal_error",
		Message:   message,
		Detail:    err.Error(),
		Path:      path,
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
