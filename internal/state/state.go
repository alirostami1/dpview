package state

import (
	"sync"
	"time"

	"codeberg.org/aros/dpview/internal/api"
	"codeberg.org/aros/dpview/internal/files"
)

type Snapshot struct {
	Files    api.FilesData
	Current  api.CurrentData
	Seek     api.SeekData
	Settings api.SettingsData
	Logs     api.LogData
	Version  int64
	EventID  int64
}

type Store struct {
	mu            sync.RWMutex
	version       int64
	eventID       int64
	files         []files.FileInfo
	tree          []files.TreeNode
	current       *files.FileInfo
	preview       api.Preview
	origin        string
	transient     bool
	sourceVersion int64
	seek          api.SeekData
	settings      api.Settings
	logs          []api.LogEntry
	subs          map[chan api.Event]struct{}
}

const maxLogEntries = 100

func NewStore() *Store {
	return &Store{
		version: 1,
		preview: api.Preview{Status: api.RenderStatusIdle},
		settings: api.Settings{
			SidebarCollapsed:            false,
			EditorFileSyncEnabled:       true,
			LiveBufferPreviewEnabled:    false,
			SeekEnabled:                 true,
			TypstPreviewTheme:           true,
			MarkdownFrontMatterVisible:  true,
			MarkdownFrontMatterExpanded: true,
			MarkdownFrontMatterTitle:    true,
			Theme:                       "light",
			PreviewTheme:                "default",
		},
		subs: make(map[chan api.Event]struct{}),
	}
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return Snapshot{
		Files:    s.filesDataLocked(),
		Current:  s.currentDataLocked(),
		Seek:     s.seekDataLocked(),
		Settings: s.settingsDataLocked(),
		Logs:     s.logsDataLocked(),
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

func (s *Store) SetCurrent(info *files.FileInfo, preview api.Preview, origin string, selectionChanged bool, transient bool, sourceVersion int64) api.CurrentData {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++
	if info != nil {
		copyInfo := *info
		s.current = &copyInfo
	} else {
		s.current = nil
	}
	s.preview = preview
	s.origin = origin
	s.transient = transient
	s.sourceVersion = sourceVersion
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
	return s.SetCurrent(nil, preview, origin, true, false, 0)
}

func (s *Store) PublishRenderStarted(info *files.FileInfo, transient bool, sourceVersion int64) api.Event {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.eventID++
	preview := api.Preview{
		Status:    api.RenderStatusRendering,
		UpdatedAt: time.Now().UTC(),
	}
	payload := api.CurrentData{
		File:          cloneFileInfo(info),
		Preview:       preview,
		Version:       s.version,
		EventID:       s.eventID,
		Current:       info != nil,
		Origin:        s.origin,
		Transient:     transient,
		SourceVersion: sourceVersion,
	}
	event := api.Event{
		Type:    api.EventRenderStarted,
		EventID: s.eventID,
		Version: s.version,
		Data:    payload,
	}
	for ch := range s.subs {
		select {
		case ch <- event:
		default:
		}
	}
	return event
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

func (s *Store) PatchSettings(patch api.SettingsPatch) api.SettingsData {
	s.mu.Lock()
	defer s.mu.Unlock()
	applySettingsPatch(&s.settings, patch)
	s.version++
	data := s.settingsDataLocked()
	s.emitLocked(api.EventSettingsChanged, data)
	return data
}

func (s *Store) SetSeek(seek api.SeekData, origin string) api.SeekData {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++
	s.seek = api.SeekData{
		Path:       seek.Path,
		Line:       seek.Line,
		Column:     seek.Column,
		TopLine:    seek.TopLine,
		BottomLine: seek.BottomLine,
		FocusLine:  seek.FocusLine,
		Origin:     origin,
	}
	data := s.seekDataLocked()
	s.emitLocked(api.EventSeekChanged, data)
	return data
}

func (s *Store) AppendLog(entry api.LogEntry) api.LogData {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++
	if entry.Timestamp.IsZero() {
		entry.Timestamp = time.Now().UTC()
	}
	s.logs = append([]api.LogEntry{entry}, s.logs...)
	if len(s.logs) > maxLogEntries {
		s.logs = s.logs[:maxLogEntries]
	}
	data := s.logsDataLocked()
	s.emitLocked(api.EventLogsChanged, data)
	return data
}

func (s *Store) ClearLogs() api.LogData {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++
	s.logs = nil
	data := s.logsDataLocked()
	s.emitLocked(api.EventLogsChanged, data)
	return data
}

func (s *Store) ClearSeek(origin string) api.SeekData {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.version++
	s.seek = api.SeekData{Origin: origin}
	data := s.seekDataLocked()
	s.emitLocked(api.EventSeekChanged, data)
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
		Version: s.version,
		EventID: s.eventID,
	}
}

func (s *Store) currentDataLocked() api.CurrentData {
	return api.CurrentData{
		File:          cloneFileInfo(s.current),
		Preview:       s.preview,
		Version:       s.version,
		EventID:       s.eventID,
		Current:       s.current != nil,
		Origin:        s.origin,
		Transient:     s.transient,
		SourceVersion: s.sourceVersion,
	}
}

func (s *Store) settingsDataLocked() api.SettingsData {
	return api.SettingsData{
		Settings: s.settings,
		Version:  s.version,
		EventID:  s.eventID,
	}
}

func (s *Store) logsDataLocked() api.LogData {
	entries := append([]api.LogEntry{}, s.logs...)
	return api.LogData{
		Entries: entries,
		Version: s.version,
		EventID: s.eventID,
	}
}

func (s *Store) seekDataLocked() api.SeekData {
	data := s.seek
	data.Version = s.version
	data.EventID = s.eventID
	return data
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

func cloneFileInfo(info *files.FileInfo) *files.FileInfo {
	if info == nil {
		return nil
	}
	copyInfo := *info
	return &copyInfo
}

func applySettingsPatch(settings *api.Settings, patch api.SettingsPatch) {
	if patch.AutoRefreshPaused != nil {
		settings.AutoRefreshPaused = *patch.AutoRefreshPaused
	}
	if patch.SidebarCollapsed != nil {
		settings.SidebarCollapsed = *patch.SidebarCollapsed
	}
	if patch.EditorFileSyncEnabled != nil {
		settings.EditorFileSyncEnabled = *patch.EditorFileSyncEnabled
	}
	if patch.LiveBufferPreviewEnabled != nil {
		settings.LiveBufferPreviewEnabled = *patch.LiveBufferPreviewEnabled
	}
	if patch.SeekEnabled != nil {
		settings.SeekEnabled = *patch.SeekEnabled
	}
	if patch.TypstPreviewTheme != nil {
		settings.TypstPreviewTheme = *patch.TypstPreviewTheme
	}
	if patch.MarkdownFrontMatterVisible != nil {
		settings.MarkdownFrontMatterVisible = *patch.MarkdownFrontMatterVisible
	}
	if patch.MarkdownFrontMatterExpanded != nil {
		settings.MarkdownFrontMatterExpanded = *patch.MarkdownFrontMatterExpanded
	}
	if patch.MarkdownFrontMatterTitle != nil {
		settings.MarkdownFrontMatterTitle = *patch.MarkdownFrontMatterTitle
	}
	if patch.Theme != nil {
		settings.Theme = *patch.Theme
	}
	if patch.PreviewTheme != nil {
		settings.PreviewTheme = *patch.PreviewTheme
	}
}
