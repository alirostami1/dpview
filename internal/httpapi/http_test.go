package httpapi

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"

	"codeberg.org/aros/dpview/internal/api"
	"codeberg.org/aros/dpview/internal/files"
	"codeberg.org/aros/dpview/internal/state"
)

func TestRoutesServeHealthFilesAndCurrent(t *testing.T) {
	store := state.NewStore()
	store.SetFiles([]files.FileInfo{{Path: "notes/test.md", Name: "test.md", Kind: files.KindMarkdown}}, []files.TreeNode{{Name: "notes", Children: []files.TreeNode{{Name: "test.md", Path: "notes/test.md", Kind: files.KindMarkdown}}}})
	file := files.FileInfo{Path: "notes/test.md", Name: "test.md", Kind: files.KindMarkdown}
	store.SetCurrent(&file, api.Preview{HTML: "<p>ok</p>", Status: api.RenderStatusReady}, "test", true)
	store.SetSeek(api.SeekData{Path: "notes/test.md", Line: 8, TopLine: 4, BottomLine: 12, FocusLine: 8}, "test")

	server, err := New(fakeApp{
		store:  store,
		health: api.HealthData{Status: "ok", Renderers: []api.RendererStatus{{Kind: files.KindTypst, Name: "Typst", Available: true}}},
	}, fstest.MapFS{
		"index.html":                  &fstest.MapFile{Data: []byte("ok")},
		"styles.css":                  &fstest.MapFile{Data: []byte("style")},
		"app.js":                      &fstest.MapFile{Data: []byte("app")},
		"themes/markdown/default.css": &fstest.MapFile{Data: []byte("theme")},
		"vendor/katex/katex.min.css":  &fstest.MapFile{Data: []byte("katex")},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	handler := server.Routes()

	resp := performRequest(t, handler, http.MethodGet, "/api/health", "")
	body := readBody(t, resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"ok":true`) || !strings.Contains(body, `"kind":"typst"`) {
		t.Fatalf("GET /api/health status=%d body=%s", resp.StatusCode, body)
	}
	if got := resp.Header.Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'self'") {
		t.Fatalf("GET /api/health missing CSP header: %q", got)
	}

	resp = performRequest(t, handler, http.MethodGet, "/api/files", "")
	body = readBody(t, resp.Body)
	if !strings.Contains(body, `"notes/test.md"`) {
		t.Fatalf("GET /api/files body=%s", body)
	}

	resp = performRequest(t, handler, http.MethodGet, "/api/current", "")
	body = readBody(t, resp.Body)
	if !strings.Contains(body, `\u003cp\u003eok\u003c/p\u003e`) || !strings.Contains(body, `"ok":true`) {
		t.Fatalf("GET /api/current body=%s", body)
	}

	resp = performRequest(t, handler, http.MethodGet, "/api/seek", "")
	body = readBody(t, resp.Body)
	if !strings.Contains(body, `"path":"notes/test.md"`) || !strings.Contains(body, `"focus_line":8`) {
		t.Fatalf("GET /api/seek body=%s", body)
	}

	resp = performRequest(t, handler, http.MethodGet, "/styles.css", "")
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusOK || body != "style" {
		t.Fatalf("GET /styles.css status=%d body=%s", resp.StatusCode, body)
	}

	resp = performRequest(t, handler, http.MethodGet, "/themes/markdown/default.css", "")
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusOK || body != "theme" {
		t.Fatalf("GET /themes/markdown/default.css status=%d body=%s", resp.StatusCode, body)
	}

	resp = performRequest(t, handler, http.MethodGet, "/vendor/katex/katex.min.css", "")
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusOK || body != "katex" {
		t.Fatalf("GET /vendor/katex/katex.min.css status=%d body=%s", resp.StatusCode, body)
	}

	resp = performRequest(t, handler, http.MethodGet, "/notes/test.md", "")
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusOK || body != "ok" {
		t.Fatalf("GET /notes/test.md status=%d body=%s", resp.StatusCode, body)
	}

	resp = performRequest(t, handler, http.MethodGet, "/missing", "")
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusNotFound || body != "ok" {
		t.Fatalf("GET /missing status=%d body=%s", resp.StatusCode, body)
	}

	resp = performRequest(t, handler, http.MethodGet, "/settings", "")
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusNotFound || body != "ok" {
		t.Fatalf("GET /settings status=%d body=%s", resp.StatusCode, body)
	}
}

func TestSetCurrentRefreshDeleteAndSettings(t *testing.T) {
	store := state.NewStore()
	server, err := New(fakeApp{
		store:         store,
		setErr:        api.NewError("invalid_path", "bad path", ""),
		setStatus:     http.StatusBadRequest,
		refreshErr:    api.NewError("no_current_file", "no current file selected", ""),
		refreshStatus: http.StatusBadRequest,
		clearCurrent:  api.CurrentData{Preview: api.Preview{Status: api.RenderStatusError, Error: api.NewError("no_current_file", "Current file cleared", "")}},
		seekErr:       api.NewError("current_mismatch", "seek path must match the current file", ""),
		seekStatus:    http.StatusConflict,
		seek:          api.SeekData{Path: "notes/test.md", Line: 12, TopLine: 8, BottomLine: 16, FocusLine: 12},
		settings:      api.SettingsData{Settings: api.Settings{AutoRefreshPaused: true, SidebarCollapsed: true, EditorFileSyncEnabled: false, SeekEnabled: false, TypstPreviewTheme: false, MarkdownFrontMatterVisible: true, MarkdownFrontMatterExpanded: false, MarkdownFrontMatterTitle: true, Theme: "dark", PreviewTheme: "github"}},
	}, fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("ok")},
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	handler := server.Routes()

	resp := performRequest(t, handler, http.MethodPost, "/api/current", `{"path":"../bad.md"}`)
	body := readBody(t, resp.Body)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(body, `"ok":false`) || !strings.Contains(body, `"code":"invalid_path"`) {
		t.Fatalf("POST /api/current status=%d body=%s", resp.StatusCode, body)
	}

	resp = performRequest(t, handler, http.MethodPost, "/api/current", `{`)
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(body, `"code":"invalid_json"`) {
		t.Fatalf("POST /api/current invalid json status=%d body=%s", resp.StatusCode, body)
	}

	resp = performRequest(t, handler, http.MethodPost, "/api/current", `{"path":"notes/test.md","extra":true}`)
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(body, `"code":"invalid_json"`) {
		t.Fatalf("POST /api/current unknown field status=%d body=%s", resp.StatusCode, body)
	}

	resp = performRequest(t, handler, http.MethodPost, "/api/seek", `{"path":"notes/test.md","line":12,"top_line":8,"bottom_line":16}`)
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusConflict || !strings.Contains(body, `"code":"current_mismatch"`) {
		t.Fatalf("POST /api/seek status=%d body=%s", resp.StatusCode, body)
	}

	resp = performRequest(t, handler, http.MethodPost, "/api/seek", `{`)
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(body, `"code":"invalid_json"`) {
		t.Fatalf("POST /api/seek invalid json status=%d body=%s", resp.StatusCode, body)
	}

	resp = performRequest(t, handler, http.MethodPost, "/api/refresh", "")
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(body, `"code":"no_current_file"`) {
		t.Fatalf("POST /api/refresh status=%d body=%s", resp.StatusCode, body)
	}

	resp = performRequest(t, handler, http.MethodDelete, "/api/current", "")
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"Current file cleared"`) {
		t.Fatalf("DELETE /api/current status=%d body=%s", resp.StatusCode, body)
	}

	resp = performRequest(t, handler, http.MethodPost, "/api/settings", `{"editor_file_sync_enabled":false,"seek_enabled":false,"preview_theme":"github"}`)
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"auto_refresh_paused":true`) || !strings.Contains(body, `"sidebar_collapsed":true`) || !strings.Contains(body, `"editor_file_sync_enabled":false`) || !strings.Contains(body, `"seek_enabled":false`) || !strings.Contains(body, `"typst_preview_theme":false`) || !strings.Contains(body, `"markdown_frontmatter_visible":true`) || !strings.Contains(body, `"markdown_frontmatter_expanded":false`) || !strings.Contains(body, `"markdown_frontmatter_title":true`) || !strings.Contains(body, `"preview_theme":"github"`) {
		t.Fatalf("POST /api/settings status=%d body=%s", resp.StatusCode, body)
	}

	resp = performRequest(t, handler, http.MethodPost, "/api/settings", `{"seek_enabled":true,"extra":true}`)
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusBadRequest || !strings.Contains(body, `"code":"invalid_json"`) {
		t.Fatalf("POST /api/settings unknown field status=%d body=%s", resp.StatusCode, body)
	}

	resp = performRequest(t, handler, http.MethodPost, "/api/settings", `{"preview_theme":"`+strings.Repeat("a", maxJSONBodyBytes)+`"}`)
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusRequestEntityTooLarge || !strings.Contains(body, `"code":"request_too_large"`) {
		t.Fatalf("POST /api/settings oversized body status=%d body=%s", resp.StatusCode, body)
	}
}

type fakeApp struct {
	store         *state.Store
	health        api.HealthData
	current       api.CurrentData
	clearCurrent  api.CurrentData
	seek          api.SeekData
	setErr        *api.Error
	setStatus     int
	seekErr       *api.Error
	seekStatus    int
	refreshErr    *api.Error
	refreshStatus int
	settings      api.SettingsData
}

func (f fakeApp) SetCurrent(context.Context, string, string) (api.CurrentData, int, *api.Error) {
	if f.setErr != nil {
		return api.CurrentData{}, f.setStatus, f.setErr
	}
	return f.current, http.StatusOK, nil
}

func (f fakeApp) Refresh(context.Context) (api.CurrentData, int, *api.Error) {
	if f.refreshErr != nil {
		return api.CurrentData{}, f.refreshStatus, f.refreshErr
	}
	return f.current, http.StatusOK, nil
}

func (f fakeApp) ClearCurrent() api.CurrentData {
	return f.clearCurrent
}

func (f fakeApp) SetSeek(context.Context, api.SeekData) (api.SeekData, int, *api.Error) {
	if f.seekErr != nil {
		return api.SeekData{}, f.seekStatus, f.seekErr
	}
	return f.seek, http.StatusOK, nil
}

func (f fakeApp) UpdateSettingsPatch(patch api.SettingsPatch) api.SettingsData {
	settings := f.settings.Settings
	if patch.AutoRefreshPaused != nil {
		settings.AutoRefreshPaused = *patch.AutoRefreshPaused
	}
	if patch.SidebarCollapsed != nil {
		settings.SidebarCollapsed = *patch.SidebarCollapsed
	}
	if patch.EditorFileSyncEnabled != nil {
		settings.EditorFileSyncEnabled = *patch.EditorFileSyncEnabled
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
	return api.SettingsData{Settings: settings}
}

func (f fakeApp) Snapshot() state.Snapshot {
	if f.store != nil {
		return f.store.Snapshot()
	}
	return state.Snapshot{Current: f.current, Seek: f.seek, Settings: f.settings}
}

func (f fakeApp) Subscribe() (<-chan api.Event, func()) {
	ch := make(chan api.Event)
	close(ch)
	return ch, func() {}
}

func (f fakeApp) Health() api.HealthData {
	return f.health
}

func performRequest(t *testing.T, handler http.Handler, method, path, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, path, strings.NewReader(body))
	if err != nil {
		t.Fatalf("NewRequest(%s %s) error = %v", method, path, err)
	}
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	recorder := httptest.NewRecorder()
	handler.ServeHTTP(recorder, req)
	return recorder.Result()
}

func readBody(t *testing.T, body io.ReadCloser) string {
	t.Helper()
	defer body.Close()
	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}
	return string(data)
}
