package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
	"strings"

	"codeberg.org/aros/dpview/internal/api"
	"codeberg.org/aros/dpview/internal/app"
	"codeberg.org/aros/dpview/internal/config"
	"codeberg.org/aros/dpview/internal/files"
	"codeberg.org/aros/dpview/internal/httpapi"
	"codeberg.org/aros/dpview/internal/render"
	"codeberg.org/aros/dpview/internal/state"
)

//go:embed web build-web
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
		LiveBufferPreviewEnabled:    cfg.LiveBufferPreviewEnabled,
		SeekEnabled:                 cfg.SeekEnabled,
		LatexEnabled:                cfg.LatexEnabled,
		TypstPreviewTheme:           cfg.TypstPreviewTheme,
		MarkdownFrontMatterVisible:  cfg.MarkdownFrontMatterVisible,
		MarkdownFrontMatterExpanded: cfg.MarkdownFrontMatterExpanded,
		MarkdownFrontMatterTitle:    cfg.MarkdownFrontMatterTitle,
		Theme:                       cfg.Theme,
		PreviewTheme:                cfg.PreviewTheme,
	})
	application := app.New(fileService, renderer, store, cfg.LogLevel)
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

	staticSourceFS, err := fs.Sub(embedded, "web")
	if err != nil {
		log.Fatalf("static source assets: %v", err)
	}
	staticBuildFS, err := fs.Sub(embedded, "build-web")
	if err != nil {
		log.Fatalf("static build assets: %v", err)
	}

	server, err := httpapi.New(application, layeredFS{layers: []fs.FS{staticBuildFS, sourceAssetFS{base: staticSourceFS}}})
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

type layeredFS struct {
	layers []fs.FS
}

func (l layeredFS) Open(name string) (fs.File, error) {
	var firstErr error
	for _, layer := range l.layers {
		file, err := layer.Open(name)
		if err == nil {
			return file, nil
		}
		if errors.Is(err, fs.ErrNotExist) {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		return nil, err
	}
	if firstErr != nil {
		return nil, firstErr
	}
	return nil, fs.ErrNotExist
}

type sourceAssetFS struct {
	base fs.FS
}

func (s sourceAssetFS) Open(name string) (fs.File, error) {
	clean := path.Clean(strings.TrimPrefix(name, "/"))
	if clean == "." {
		clean = ""
	}
	if !isServedSourceAsset(clean) {
		return nil, fs.ErrNotExist
	}
	return s.base.Open(clean)
}

func isServedSourceAsset(name string) bool {
	switch {
	case name == "":
		return true
	case name == "index.html":
		return true
	case name == "styles.css":
		return true
	case strings.HasPrefix(name, "themes/"):
		return true
	default:
		return false
	}
}
