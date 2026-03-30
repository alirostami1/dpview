package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"

	"codeberg.org/aros/dpview/internal/api"
	"codeberg.org/aros/dpview/internal/state"
)

type Application interface {
	SetCurrent(context.Context, string, string) (api.CurrentData, int, *api.Error)
	Refresh(context.Context) (api.CurrentData, int, *api.Error)
	ClearCurrent() api.CurrentData
	SetSeek(context.Context, api.SeekData) (api.SeekData, int, *api.Error)
	UpdateSettingsPatch(api.SettingsPatch) api.SettingsData
	ClearLogs() api.LogData
	Snapshot() state.Snapshot
	Subscribe() (<-chan api.Event, func())
	Health() api.HealthData
}

type Server struct {
	app    Application
	static fs.FS
}

type CurrentRequest struct {
	Path   string `json:"path"`
	Origin string `json:"origin"`
}

type SeekRequest struct {
	Path       string `json:"path"`
	Line       int    `json:"line"`
	Column     int    `json:"column"`
	TopLine    int    `json:"top_line"`
	BottomLine int    `json:"bottom_line"`
	FocusLine  int    `json:"focus_line"`
}

type SettingsRequest = api.SettingsPatch

const maxJSONBodyBytes = 1 << 20

func New(app Application, static fs.FS) (*Server, error) {
	if static == nil {
		return nil, fmt.Errorf("static assets: nil fs")
	}
	return &Server{app: app, static: static}, nil
}

func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.handleHealth)
	mux.HandleFunc("GET /api/files", s.handleFiles)
	mux.HandleFunc("GET /api/current", s.handleCurrent)
	mux.HandleFunc("POST /api/current", s.handleSetCurrent)
	mux.HandleFunc("DELETE /api/current", s.handleDeleteCurrent)
	mux.HandleFunc("GET /api/seek", s.handleSeek)
	mux.HandleFunc("POST /api/seek", s.handleSetSeek)
	mux.HandleFunc("POST /api/refresh", s.handleRefresh)
	mux.HandleFunc("GET /api/settings", s.handleSettings)
	mux.HandleFunc("POST /api/settings", s.handleSetSettings)
	mux.HandleFunc("GET /api/logs", s.handleLogs)
	mux.HandleFunc("DELETE /api/logs", s.handleDeleteLogs)
	mux.HandleFunc("GET /events", s.handleEvents)
	mux.HandleFunc("/", s.handleStatic)
	return s.withMiddleware(mux)
}

func (s *Server) withMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data: https: http:; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		if strings.HasPrefix(r.URL.Path, "/api/") {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.Header().Set("Cache-Control", "no-store")
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.OK(s.app.Health()))
}

func (s *Server) handleFiles(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.OK(s.app.Snapshot().Files))
}

func (s *Server) handleCurrent(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.OK(s.app.Snapshot().Current))
}

func (s *Server) handleSettings(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.OK(s.app.Snapshot().Settings))
}

func (s *Server) handleSeek(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.OK(s.app.Snapshot().Seek))
}

func (s *Server) handleLogs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.OK(s.app.Snapshot().Logs))
}

func (s *Server) handleSetCurrent(w http.ResponseWriter, r *http.Request) {
	var req CurrentRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeDecodeError(w, err)
		return
	}
	resp, status, err := s.app.SetCurrent(r.Context(), req.Path, req.Origin)
	if err != nil {
		writeJSON(w, status, api.Fail(err))
		return
	}
	writeJSON(w, status, api.OK(resp))
}

func (s *Server) handleDeleteCurrent(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.OK(s.app.ClearCurrent()))
}

func (s *Server) handleSetSeek(w http.ResponseWriter, r *http.Request) {
	var req SeekRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeDecodeError(w, err)
		return
	}
	resp, status, err := s.app.SetSeek(r.Context(), api.SeekData{
		Path:       req.Path,
		Line:       req.Line,
		Column:     req.Column,
		TopLine:    req.TopLine,
		BottomLine: req.BottomLine,
		FocusLine:  req.FocusLine,
	})
	if err != nil {
		writeJSON(w, status, api.Fail(err))
		return
	}
	writeJSON(w, status, api.OK(resp))
}

func (s *Server) handleRefresh(w http.ResponseWriter, r *http.Request) {
	resp, status, err := s.app.Refresh(r.Context())
	if err != nil {
		writeJSON(w, status, api.Fail(err))
		return
	}
	writeJSON(w, status, api.OK(resp))
}

func (s *Server) handleSetSettings(w http.ResponseWriter, r *http.Request) {
	var req SettingsRequest
	if err := decodeJSON(w, r, &req); err != nil {
		writeDecodeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, api.OK(s.app.UpdateSettingsPatch(req)))
}

func (s *Server) handleDeleteLogs(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.OK(s.app.ClearLogs()))
}

func (s *Server) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	events, cancel := s.app.Subscribe()
	defer cancel()

	snap := s.app.Snapshot()
	writeSSE(w, api.Event{Type: api.EventFilesChanged, EventID: snap.EventID, Version: snap.Version, Data: snap.Files})
	writeSSE(w, api.Event{Type: api.EventCurrentChanged, EventID: snap.EventID, Version: snap.Version, Data: snap.Current})
	writeSSE(w, api.Event{Type: api.EventSeekChanged, EventID: snap.EventID, Version: snap.Version, Data: snap.Seek})
	writeSSE(w, api.Event{Type: api.EventSettingsChanged, EventID: snap.EventID, Version: snap.Version, Data: snap.Settings})
	writeSSE(w, api.Event{Type: api.EventLogsChanged, EventID: snap.EventID, Version: snap.Version, Data: snap.Logs})
	flusher.Flush()

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-r.Context().Done():
			return
		case event, ok := <-events:
			if !ok {
				return
			}
			writeSSE(w, event)
			flusher.Flush()
		case <-ticker.C:
			fmt.Fprint(w, ": keep-alive\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.NotFound(w, r)
		return
	}

	name := strings.TrimPrefix(path.Clean(r.URL.Path), "/")
	if name == "." {
		name = ""
	}
	if name != "" {
		if _, err := fs.Stat(s.static, name); err == nil {
			http.FileServer(http.FS(s.static)).ServeHTTP(w, r)
			return
		}
	}

	status := http.StatusOK
	if !s.isAppRoute(name) {
		status = http.StatusNotFound
	}

	data, err := fs.ReadFile(s.static, "index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if r.Method == http.MethodHead {
		w.WriteHeader(status)
		return
	}
	w.WriteHeader(status)
	_, _ = w.Write(data)
}

func (s *Server) isAppRoute(name string) bool {
	if name == "" {
		return true
	}
	for _, file := range s.app.Snapshot().Files.Files {
		if file.Path == name {
			return true
		}
	}
	return false
}

func writeSSE(w http.ResponseWriter, event api.Event) {
	data, _ := json.Marshal(map[string]any{
		"event_id": event.EventID,
		"version":  event.Version,
		"data":     event.Data,
	})
	fmt.Fprintf(w, "id: %d\n", event.EventID)
	fmt.Fprintf(w, "event: %s\n", event.Type)
	fmt.Fprintf(w, "data: %s\n\n", data)
}

func writeJSON(w http.ResponseWriter, status int, payload api.Envelope) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) error {
	defer r.Body.Close()
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBodyBytes))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return fmt.Errorf("unexpected trailing data")
	}
	return nil
}

func writeDecodeError(w http.ResponseWriter, err error) {
	var maxErr *http.MaxBytesError
	if errors.As(err, &maxErr) {
		writeJSON(w, http.StatusRequestEntityTooLarge, api.Fail(api.NewError("request_too_large", "Request body is too large", err.Error())))
		return
	}
	writeJSON(w, http.StatusBadRequest, api.Fail(api.NewError("invalid_json", "Invalid JSON body", err.Error())))
}
