package config

import (
	"flag"
	"fmt"
	"path/filepath"
	"time"
)

type Config struct {
	ShowVersion                 bool
	Root                        string
	Bind                        string
	Port                        int
	SidebarClosed               bool
	EditorFileSync              bool
	LiveBufferPreviewEnabled    bool
	SeekEnabled                 bool
	TypstPreviewTheme           bool
	MarkdownFrontMatterVisible  bool
	MarkdownFrontMatterExpanded bool
	MarkdownFrontMatterTitle    bool
	Theme                       string
	PreviewTheme                string
	TypstBinary                 string
	OpenBrowser                 bool
	MaxFileSize                 int64
	RenderTimeout               time.Duration
}

func Parse() (Config, error) {
	cfg := Config{}
	flag.BoolVar(&cfg.ShowVersion, "version", false, "print version and exit")
	flag.StringVar(&cfg.Root, "root", ".", "root folder to scan for previewable files")
	flag.StringVar(&cfg.Bind, "bind", "127.0.0.1", "bind address")
	flag.IntVar(&cfg.Port, "port", 8090, "port to listen on")
	flag.BoolVar(&cfg.SidebarClosed, "sidebar-closed", false, "start with the sidebar collapsed")
	flag.BoolVar(&cfg.EditorFileSync, "editor-file-sync", true, "allow editor integrations to switch the current preview file")
	flag.BoolVar(&cfg.LiveBufferPreviewEnabled, "live-buffer-preview", false, "allow editor integrations to push unsaved buffer content for preview")
	flag.BoolVar(&cfg.SeekEnabled, "seek-enabled", true, "enable source seeking between editor integrations and the preview")
	flag.BoolVar(&cfg.TypstPreviewTheme, "typst-preview-theme", true, "apply DPview preview theming to Typst documents")
	flag.BoolVar(&cfg.MarkdownFrontMatterVisible, "markdown-frontmatter-visible", true, "show parsed Markdown front matter above the preview")
	flag.BoolVar(&cfg.MarkdownFrontMatterExpanded, "markdown-frontmatter-expanded", true, "start Markdown front matter panels expanded")
	flag.BoolVar(&cfg.MarkdownFrontMatterTitle, "markdown-frontmatter-title", true, "use Markdown front matter title as an H1 when the document has no H1")
	flag.StringVar(&cfg.Theme, "theme", "light", "initial app theme: light or dark")
	flag.StringVar(&cfg.PreviewTheme, "preview-theme", "default", "initial preview theme id")
	flag.StringVar(&cfg.TypstBinary, "typst-binary", "", "path to Typst executable to use instead of PATH lookup")
	flag.BoolVar(&cfg.OpenBrowser, "open-browser", false, "open the browser after startup")
	flag.Int64Var(&cfg.MaxFileSize, "max-file-size", 4<<20, "maximum previewable source size in bytes")
	flag.DurationVar(&cfg.RenderTimeout, "render-timeout", 5*time.Second, "per-render timeout")
	flag.Parse()

	if cfg.ShowVersion {
		return cfg, nil
	}

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
