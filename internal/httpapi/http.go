package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"codeberg.org/aros/dpview.git/internal/api"
	"codeberg.org/aros/dpview.git/internal/state"
)

type Application interface {
	SetCurrent(context.Context, string) (api.CurrentData, int, *api.Error)
	Refresh(context.Context) (api.CurrentData, int, *api.Error)
	ClearCurrent() api.CurrentData
	UpdateSettings(api.Settings) api.SettingsData
	Snapshot() state.Snapshot
	Subscribe() (<-chan api.Event, func())
	Health() api.HealthData
}

type Server struct {
	app    Application
	static fs.FS
}

type CurrentRequest struct {
	Path string `json:"path"`
}

type SettingsRequest struct {
	AutoRefreshPaused bool   `json:"auto_refresh_paused"`
	Theme             string `json:"theme"`
	PreviewTheme      string `json:"preview_theme"`
}

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
	mux.HandleFunc("POST /api/refresh", s.handleRefresh)
	mux.HandleFunc("GET /api/settings", s.handleSettings)
	mux.HandleFunc("POST /api/settings", s.handleSetSettings)
	mux.HandleFunc("GET /events", s.handleEvents)
	fileServer := http.FileServer(http.FS(s.static))
	mux.Handle("/", fileServer)
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

func (s *Server) handleSetCurrent(w http.ResponseWriter, r *http.Request) {
	defer r.Body.Close()
	var req CurrentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, api.Fail(api.NewError("invalid_json", "Invalid JSON body", err.Error())))
		return
	}
	resp, status, err := s.app.SetCurrent(r.Context(), req.Path)
	if err != nil {
		writeJSON(w, status, api.Fail(err))
		return
	}
	writeJSON(w, status, api.OK(resp))
}

func (s *Server) handleDeleteCurrent(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, api.OK(s.app.ClearCurrent()))
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
	defer r.Body.Close()
	var req SettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, api.Fail(api.NewError("invalid_json", "Invalid JSON body", err.Error())))
		return
	}
	writeJSON(w, http.StatusOK, api.OK(s.app.UpdateSettings(api.Settings{
		AutoRefreshPaused: req.AutoRefreshPaused,
		Theme:             req.Theme,
		PreviewTheme:      req.PreviewTheme,
	})))
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
	writeSSE(w, api.Event{Type: api.EventSettingsChanged, EventID: snap.EventID, Version: snap.Version, Data: snap.Settings})
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
