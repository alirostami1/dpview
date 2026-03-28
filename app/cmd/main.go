package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"

	"codeberg.org/aros/dpview.git/internal/api"
	"codeberg.org/aros/dpview.git/internal/app"
	"codeberg.org/aros/dpview.git/internal/config"
	"codeberg.org/aros/dpview.git/internal/files"
	"codeberg.org/aros/dpview.git/internal/httpapi"
	"codeberg.org/aros/dpview.git/internal/render"
	"codeberg.org/aros/dpview.git/internal/state"
)

//go:embed web/*
var embedded embed.FS

func main() {
	cfg, err := config.Parse()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	fileService, err := files.NewService(cfg.Root)
	if err != nil {
		log.Fatalf("files: %v", err)
	}

	renderer, err := render.NewService(render.Config{
		TypstBinary:   cfg.TypstBinary,
		MaxFileSize:   cfg.MaxFileSize,
		RenderTimeout: cfg.RenderTimeout,
	})
	if err != nil {
		log.Fatalf("renderer: %v", err)
	}
	defer renderer.Close()

	store := state.NewStore()
	store.UpdateSettings(api.Settings{})
	application := app.New(fileService, renderer, store)
	if err := application.Rescan(); err != nil {
		log.Fatalf("scan root: %v", err)
	}

	watcher, err := application.StartWatcher()
	if err != nil {
		log.Printf("watcher disabled: %v", err)
	} else {
		defer watcher.Close()
	}

	staticFS, err := fs.Sub(embedded, "web")
	if err != nil {
		log.Fatalf("static assets: %v", err)
	}

	server, err := httpapi.New(application, staticFS)
	if err != nil {
		log.Fatalf("http: %v", err)
	}

	log.Printf("serving %s on http://%s (log-level=%s)", cfg.Root, cfg.Address(), cfg.LogLevel)
	if status, ok := renderer.RendererStatus(files.KindTypst); ok && !status.Available {
		log.Printf("typst unavailable: %s", status.Details["reason"])
	}
	if cfg.OpenBrowser {
		if err := app.OpenBrowser("http://" + cfg.Address()); err != nil {
			log.Printf("open browser: %v", err)
		}
	}

	httpServer := &http.Server{
		Addr:    cfg.Address(),
		Handler: server.Routes(),
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Printf("server error: %v", err)
		os.Exit(1)
	}
}
