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
	"time"

	"codeberg.org/aros/dpview.git/internal/api"
	"codeberg.org/aros/dpview.git/internal/files"
	"codeberg.org/aros/dpview.git/internal/render"
	"codeberg.org/aros/dpview.git/internal/state"
	"codeberg.org/aros/dpview.git/internal/watch"
)

type Service struct {
	files    *files.Service
	renderer *render.Service
	store    *state.Store

	mu             sync.Mutex
	watcherEnabled bool
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

func (s *Service) Health() api.HealthData {
	snap := s.store.Snapshot()
	return api.HealthData{
		Status:    "ok",
		Version:   snap.Version,
		EventID:   snap.EventID,
		Renderers: s.renderer.RendererStatuses(),
		Limits:    s.renderer.Limits(),
		Watcher:   api.WatcherStatus{Enabled: s.watcherEnabled},
	}
}

func (s *Service) SetCurrent(ctx context.Context, rel string) (api.CurrentData, int, *api.Error) {
	abs, info, apiErr, status := s.resolvePath(rel)
	if apiErr != nil {
		return api.CurrentData{}, status, apiErr
	}
	s.store.PublishRenderStarted(&info)
	preview := s.renderer.Render(ctx, info, abs, s.store.Snapshot().Settings.Settings)
	selectionChanged := s.currentPath() != info.Path
	return s.store.SetCurrent(&info, preview, "api", selectionChanged), http.StatusOK, nil
}

func (s *Service) Refresh(ctx context.Context) (api.CurrentData, int, *api.Error) {
	current := s.store.Snapshot().Current.File
	if current == nil {
		return api.CurrentData{}, http.StatusBadRequest, api.NewError("no_current_file", "No current file selected", "")
	}
	abs, info, apiErr, status := s.resolvePath(current.Path)
	if apiErr != nil {
		if apiErr.Code == "file_not_found" {
			cleared := s.store.ClearCurrent(api.NewError("file_not_found", "Current file no longer exists", current.Path), "watch")
			return cleared, status, apiErr
		}
		return api.CurrentData{}, status, apiErr
	}
	s.store.PublishRenderStarted(&info)
	preview := s.renderer.Render(ctx, info, abs, s.store.Snapshot().Settings.Settings)
	return s.store.SetCurrent(&info, preview, "refresh", false), http.StatusOK, nil
}

func (s *Service) ClearCurrent() api.CurrentData {
	return s.store.ClearCurrent(api.NewError("no_current_file", "Current file cleared", ""), "api")
}

func (s *Service) UpdateSettings(settings api.Settings) api.SettingsData {
	return s.store.UpdateSettings(settings)
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
			return
		}
	}
	if current != nil {
		if _, _, err := s.files.Resolve(current.Path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				s.store.ClearCurrent(api.NewError("file_not_found", "Current file no longer exists", current.Path), "watch")
				return
			}
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
	s.watcherEnabled = true
	return watcher, nil
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

func OpenBrowser(url string) error {
	cmd := exec.Command("xdg-open", url)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open browser: %w", err)
	}
	return nil
}
