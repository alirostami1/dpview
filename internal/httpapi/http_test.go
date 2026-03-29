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

	server, err := New(fakeApp{
		store:  store,
		health: api.HealthData{Status: "ok", Renderers: []api.RendererStatus{{Kind: files.KindTypst, Name: "Typst", Available: true}}},
	}, fstest.MapFS{
		"index.html": &fstest.MapFile{Data: []byte("ok")},
		"styles.css": &fstest.MapFile{Data: []byte("")},
		"app.js":     &fstest.MapFile{Data: []byte("")},
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
		settings:      api.SettingsData{Settings: api.Settings{AutoRefreshPaused: true, SidebarCollapsed: true, TypstPreviewTheme: false, Theme: "dark", PreviewTheme: "github"}},
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

	resp = performRequest(t, handler, http.MethodPost, "/api/settings", `{"auto_refresh_paused":true,"sidebar_collapsed":true,"typst_preview_theme":false,"theme":"dark","preview_theme":"github"}`)
	body = readBody(t, resp.Body)
	if resp.StatusCode != http.StatusOK || !strings.Contains(body, `"auto_refresh_paused":true`) || !strings.Contains(body, `"sidebar_collapsed":true`) || !strings.Contains(body, `"typst_preview_theme":false`) || !strings.Contains(body, `"preview_theme":"github"`) {
		t.Fatalf("POST /api/settings status=%d body=%s", resp.StatusCode, body)
	}
}

type fakeApp struct {
	store         *state.Store
	health        api.HealthData
	current       api.CurrentData
	clearCurrent  api.CurrentData
	setErr        *api.Error
	setStatus     int
	refreshErr    *api.Error
	refreshStatus int
	settings      api.SettingsData
}

func (f fakeApp) SetCurrent(context.Context, string) (api.CurrentData, int, *api.Error) {
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

func (f fakeApp) UpdateSettings(api.Settings) api.SettingsData {
	return f.settings
}

func (f fakeApp) Snapshot() state.Snapshot {
	if f.store != nil {
		return f.store.Snapshot()
	}
	return state.Snapshot{Current: f.current, Settings: f.settings}
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
