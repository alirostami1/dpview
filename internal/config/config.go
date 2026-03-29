package config

import (
	"flag"
	"fmt"
	"path/filepath"
	"time"
)

type Config struct {
	Root              string
	Bind              string
	Port              int
	SidebarClosed     bool
	TypstPreviewTheme bool
	Theme             string
	PreviewTheme      string
	TypstBinary       string
	LogLevel          string
	OpenBrowser       bool
	MaxFileSize       int64
	RenderTimeout     time.Duration
}

func Parse() (Config, error) {
	cfg := Config{}
	flag.StringVar(&cfg.Root, "root", ".", "root folder to scan for previewable files")
	flag.StringVar(&cfg.Bind, "bind", "127.0.0.1", "bind address")
	flag.IntVar(&cfg.Port, "port", 8090, "port to listen on")
	flag.BoolVar(&cfg.SidebarClosed, "sidebar-closed", false, "start with the sidebar collapsed")
	flag.BoolVar(&cfg.TypstPreviewTheme, "typst-preview-theme", true, "apply DPview preview theming to Typst documents")
	flag.StringVar(&cfg.Theme, "theme", "light", "initial app theme: light or dark")
	flag.StringVar(&cfg.PreviewTheme, "preview-theme", "default", "initial preview theme id")
	flag.StringVar(&cfg.TypstBinary, "typst-binary", "", "path to Typst executable to use instead of PATH lookup")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "log level")
	flag.BoolVar(&cfg.OpenBrowser, "open-browser", false, "open the browser after startup")
	flag.Int64Var(&cfg.MaxFileSize, "max-file-size", 4<<20, "maximum previewable source size in bytes")
	flag.DurationVar(&cfg.RenderTimeout, "render-timeout", 5*time.Second, "per-render timeout")
	flag.Parse()

	root, err := filepath.Abs(cfg.Root)
	if err != nil {
		return Config{}, fmt.Errorf("resolve root: %w", err)
	}
	cfg.Root = root

	if cfg.Port < 1 || cfg.Port > 65535 {
		return Config{}, fmt.Errorf("invalid port %d", cfg.Port)
	}
	if cfg.Theme != "light" && cfg.Theme != "dark" {
		return Config{}, fmt.Errorf("invalid theme %q", cfg.Theme)
	}
	if cfg.PreviewTheme == "" {
		return Config{}, fmt.Errorf("preview theme must not be empty")
	}
	if cfg.MaxFileSize <= 0 {
		return Config{}, fmt.Errorf("invalid max file size %d", cfg.MaxFileSize)
	}
	if cfg.RenderTimeout <= 0 {
		return Config{}, fmt.Errorf("invalid render timeout %s", cfg.RenderTimeout)
	}

	return cfg, nil
}

func (c Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Bind, c.Port)
}
