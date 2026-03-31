package config

import (
	"flag"
	"fmt"
	"path/filepath"
	"time"

	"codeberg.org/aros/dpview/internal/validation"
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
	LatexEnabled                bool
	TypstPreviewTheme           bool
	MarkdownFrontMatterVisible  bool
	MarkdownFrontMatterExpanded bool
	MarkdownFrontMatterTitle    bool
	Theme                       string
	PreviewTheme                string
	TypstBinary                 string
	OpenBrowser                 bool
	LogLevel                    string
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
	flag.BoolVar(&cfg.LatexEnabled, "latex-enabled", true, "enable LaTeX math handling in Markdown previews")
	flag.BoolVar(&cfg.TypstPreviewTheme, "typst-preview-theme", true, "apply DPview preview theming to Typst documents")
	flag.BoolVar(&cfg.MarkdownFrontMatterVisible, "markdown-frontmatter-visible", true, "show parsed Markdown front matter above the preview")
	flag.BoolVar(&cfg.MarkdownFrontMatterExpanded, "markdown-frontmatter-expanded", true, "start Markdown front matter panels expanded")
	flag.BoolVar(&cfg.MarkdownFrontMatterTitle, "markdown-frontmatter-title", true, "use Markdown front matter title as an H1 when the document has no H1")
	flag.StringVar(&cfg.Theme, "theme", "light", "initial app theme: light or dark")
	flag.StringVar(&cfg.PreviewTheme, "preview-theme", "default", "initial preview theme id")
	flag.StringVar(&cfg.TypstBinary, "typst-binary", "", "path to Typst executable to use instead of PATH lookup")
	flag.BoolVar(&cfg.OpenBrowser, "open-browser", false, "open the browser after startup")
	flag.StringVar(&cfg.LogLevel, "log-level", "info", "log level: debug, info, or error")
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

	if err := validation.ValidateRuntimeConfig(validation.RuntimeConfig{
		Port:          cfg.Port,
		Theme:         cfg.Theme,
		PreviewTheme:  cfg.PreviewTheme,
		LogLevel:      cfg.LogLevel,
		MaxFileSize:   cfg.MaxFileSize,
		RenderTimeout: cfg.RenderTimeout,
	}); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func (c Config) Address() string {
	return fmt.Sprintf("%s:%d", c.Bind, c.Port)
}
