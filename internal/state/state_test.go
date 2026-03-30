package state

import (
	"testing"

	"codeberg.org/aros/dpview/internal/api"
	"codeberg.org/aros/dpview/internal/files"
)

func TestPublishRenderStartedUsesMatchingEventMetadata(t *testing.T) {
	store := NewStore()
	info := &files.FileInfo{Path: "notes/test.md", Name: "test.md", Kind: files.KindMarkdown}

	event := store.PublishRenderStarted(info)

	if event.Type != api.EventRenderStarted {
		t.Fatalf("event type = %q", event.Type)
	}
	data, ok := event.Data.(api.CurrentData)
	if !ok {
		t.Fatalf("event data type = %T", event.Data)
	}
	if data.EventID != event.EventID {
		t.Fatalf("payload event id = %d, event id = %d", data.EventID, event.EventID)
	}
	if data.Version != event.Version {
		t.Fatalf("payload version = %d, event version = %d", data.Version, event.Version)
	}
	if data.File == nil || data.File.Path != info.Path {
		t.Fatalf("payload file = %+v", data.File)
	}
}

func TestPatchSettingsPreservesUnspecifiedValues(t *testing.T) {
	store := NewStore()
	dark := "dark"
	disabled := false

	store.UpdateSettings(api.Settings{
		AutoRefreshPaused:           true,
		SidebarCollapsed:            true,
		EditorFileSyncEnabled:       true,
		SeekEnabled:                 true,
		TypstPreviewTheme:           true,
		MarkdownFrontMatterVisible:  true,
		MarkdownFrontMatterExpanded: false,
		MarkdownFrontMatterTitle:    true,
		Theme:                       "light",
		PreviewTheme:                "default",
	})

	data := store.PatchSettings(api.SettingsPatch{
		SeekEnabled:  &disabled,
		PreviewTheme: &dark,
	})

	if !data.Settings.AutoRefreshPaused || !data.Settings.SidebarCollapsed {
		t.Fatalf("patch unexpectedly changed preserved settings: %+v", data.Settings)
	}
	if data.Settings.SeekEnabled {
		t.Fatalf("seek should be disabled: %+v", data.Settings)
	}
	if data.Settings.PreviewTheme != "dark" {
		t.Fatalf("preview theme = %q", data.Settings.PreviewTheme)
	}
}

func TestAppendAndClearLogs(t *testing.T) {
	store := NewStore()

	for i := 0; i < maxLogEntries+5; i++ {
		store.AppendLog(api.LogEntry{
			Level:   "error",
			Source:  "test",
			Code:    "boom",
			Message: "entry",
			Detail:  "detail",
			Context: "ctx",
		})
	}

	snap := store.Snapshot()
	if len(snap.Logs.Entries) != maxLogEntries {
		t.Fatalf("log entry count = %d", len(snap.Logs.Entries))
	}
	if snap.Logs.Entries[0].Timestamp.IsZero() {
		t.Fatalf("expected timestamp to be populated: %+v", snap.Logs.Entries[0])
	}

	cleared := store.ClearLogs()
	if len(cleared.Entries) != 0 {
		t.Fatalf("expected logs to be cleared: %+v", cleared)
	}
}
