package main

import (
	"embed"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"

	"codeberg.org/aros/dpview/internal/api"
	"codeberg.org/aros/dpview/internal/app"
	"codeberg.org/aros/dpview/internal/config"
	"codeberg.org/aros/dpview/internal/files"
	"codeberg.org/aros/dpview/internal/httpapi"
	"codeberg.org/aros/dpview/internal/render"
	"codeberg.org/aros/dpview/internal/state"
)

//go:embed web
var embedded embed.FS

var (
	version = "dev"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	cfg, err := config.Parse()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if cfg.ShowVersion {
		fmt.Println(versionString())
		return
	}

	fileService, err := files.NewService(cfg.Root)
	if err != nil {
		log.Fatalf("files: %v", err)
	}

	renderer, err := render.NewService(render.Config{
		Root:          cfg.Root,
		TypstBinary:   cfg.TypstBinary,
		MaxFileSize:   cfg.MaxFileSize,
		RenderTimeout: cfg.RenderTimeout,
	})
	if err != nil {
		log.Fatalf("renderer: %v", err)
	}
	defer renderer.Close()

	store := state.NewStore()
	store.UpdateSettings(api.Settings{
		SidebarCollapsed:            cfg.SidebarClosed,
		EditorFileSyncEnabled:       cfg.EditorFileSync,
		SeekEnabled:                 cfg.SeekEnabled,
		TypstPreviewTheme:           cfg.TypstPreviewTheme,
		MarkdownFrontMatterVisible:  cfg.MarkdownFrontMatterVisible,
		MarkdownFrontMatterExpanded: cfg.MarkdownFrontMatterExpanded,
		MarkdownFrontMatterTitle:    cfg.MarkdownFrontMatterTitle,
		Theme:                       cfg.Theme,
		PreviewTheme:                cfg.PreviewTheme,
	})
	application := app.New(fileService, renderer, store)
	if err := application.Rescan(); err != nil {
		log.Fatalf("scan root: %v", err)
	}

	watcher, err := application.StartWatcher()
	if err != nil {
		application.RecordRuntimeError("startup", "watcher disabled", err)
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

	log.Printf("serving %s on http://%s", cfg.Root, cfg.Address())
	if status, ok := renderer.RendererStatus(files.KindTypst); ok && !status.Available {
		log.Printf("typst unavailable: %s", status.Details["reason"])
	}
	if cfg.OpenBrowser {
		if err := app.OpenBrowser("http://" + cfg.Address()); err != nil {
			application.RecordRuntimeError("startup", "open browser failed", err)
			log.Printf("open browser: %v", err)
		}
	}

	httpServer := &http.Server{
		Addr:    cfg.Address(),
		Handler: server.Routes(),
	}

	if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		application.RecordRuntimeError("server", "server error", err)
		log.Printf("server error: %v", err)
		os.Exit(1)
	}
}

func versionString() string {
	if commit == "unknown" && date == "unknown" {
		return fmt.Sprintf("dpview %s", version)
	}
	return fmt.Sprintf("dpview %s (%s, %s)", version, commit, date)
}
