package state

import (
	"slices"
	"sync"
	"time"

	"codeberg.org/aros/dpview.git/internal/api"
	"codeberg.org/aros/dpview.git/internal/files"
)

type Snapshot struct {
	Files    api.FilesData
	Current  api.CurrentData
	Settings api.SettingsData
	Version  int64
	EventID  int64
}

type Store struct {
	mu       sync.RWMutex
	version  int64
	eventID  int64
	files    []files.FileInfo
	tree     []files.TreeNode
	recent   []api.RecentFile
	current  *files.FileInfo
	preview  api.Preview
	origin   string
	settings api.Settings
	subs     map[chan api.Event]struct{}
}

func NewStore() *Store {
	return &Store{
		version: 1,
		preview: api.Preview{Status: api.RenderStatusIdle},
		subs:    make(map[chan api.Event]struct{}),
	}
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Snapshot{
		Files:    s.filesDataLocked(),
		Current:  s.currentDataLocked(),
		Settings: s.settingsDataLocked(),
		Version:  s.version,
		EventID:  s.eventID,
	}
}

func (s *Store) SetFiles(items []files.FileInfo, tree []files.TreeNode) api.FilesData {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++
	s.files = append([]files.FileInfo(nil), items...)
	s.tree = append([]files.TreeNode(nil), tree...)
	data := s.filesDataLocked()
	s.emitLocked(api.EventFilesChanged, data)
	return data
}

func (s *Store) SetCurrent(info *files.FileInfo, preview api.Preview, origin string, selectionChanged bool) api.CurrentData {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++
	if info != nil {
		copyInfo := *info
		s.current = &copyInfo
		s.touchRecentLocked(copyInfo)
	} else {
		s.current = nil
	}
	s.preview = preview
	s.origin = origin
	data := s.currentDataLocked()
	if selectionChanged {
		s.emitLocked(api.EventCurrentChanged, data)
	}
	if preview.Error != nil {
		s.emitLocked(api.EventRenderFailed, data)
	} else {
		s.emitLocked(api.EventPreviewUpdated, data)
	}
	return data
}

func (s *Store) ClearCurrent(err *api.Error, origin string) api.CurrentData {
	preview := api.Preview{
		Status:    api.RenderStatusError,
		UpdatedAt: time.Now().UTC(),
		Error:     err,
	}
	return s.SetCurrent(nil, preview, origin, true)
}

func (s *Store) PublishRenderStarted(info *files.FileInfo) api.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	preview := api.Preview{
		Status:    api.RenderStatusRendering,
		UpdatedAt: time.Now().UTC(),
	}
	if s.current != nil {
		preview.Source = s.preview.Source
	}
	payload := api.CurrentData{
		File:    cloneFileInfo(info),
		Preview: preview,
		Version: s.version,
		Current: info != nil,
		Origin:  s.origin,
	}
	return s.emitLocked(api.EventRenderStarted, payload)
}

func (s *Store) UpdateSettings(settings api.Settings) api.SettingsData {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++
	s.settings = settings
	data := s.settingsDataLocked()
	s.emitLocked(api.EventSettingsChanged, data)
	return data
}

func (s *Store) Subscribe() (<-chan api.Event, func()) {
	ch := make(chan api.Event, 16)
	s.mu.Lock()
	s.subs[ch] = struct{}{}
	s.mu.Unlock()
	cancel := func() {
		s.mu.Lock()
		if _, ok := s.subs[ch]; ok {
			delete(s.subs, ch)
			close(ch)
		}
		s.mu.Unlock()
	}
	return ch, cancel
}

func (s *Store) filesDataLocked() api.FilesData {
	return api.FilesData{
		Files:   append([]files.FileInfo(nil), s.files...),
		Tree:    append([]files.TreeNode(nil), s.tree...),
		Recent:  append([]api.RecentFile(nil), s.recent...),
		Version: s.version,
		EventID: s.eventID,
	}
}

func (s *Store) currentDataLocked() api.CurrentData {
	return api.CurrentData{
		File:    cloneFileInfo(s.current),
		Preview: s.preview,
		Version: s.version,
		EventID: s.eventID,
		Current: s.current != nil,
		Origin:  s.origin,
	}
}

func (s *Store) settingsDataLocked() api.SettingsData {
	return api.SettingsData{
		Settings: s.settings,
		Version:  s.version,
		EventID:  s.eventID,
	}
}

func (s *Store) emitLocked(eventType string, data any) api.Event {
	s.eventID++
	event := api.Event{
		Type:    eventType,
		EventID: s.eventID,
		Version: s.version,
		Data:    data,
	}
	for ch := range s.subs {
		select {
		case ch <- event:
		default:
		}
	}
	return event
}

func (s *Store) touchRecentLocked(info files.FileInfo) {
	now := time.Now().UTC()
	next := make([]api.RecentFile, 0, len(s.recent)+1)
	next = append(next, api.RecentFile{
		Path:       info.Path,
		Name:       info.Name,
		Kind:       info.Kind,
		AccessedAt: now,
	})
	for _, item := range s.recent {
		if item.Path == info.Path {
			continue
		}
		next = append(next, item)
	}
	if len(next) > 10 {
		next = next[:10]
	}
	slices.SortFunc(next, func(a, b api.RecentFile) int {
		if a.AccessedAt.After(b.AccessedAt) {
			return -1
		}
		if a.AccessedAt.Before(b.AccessedAt) {
			return 1
		}
		return 0
	})
	s.recent = next
}

func cloneFileInfo(info *files.FileInfo) *files.FileInfo {
	if info == nil {
		return nil
	}
	copyInfo := *info
	return &copyInfo
}
