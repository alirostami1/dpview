package render

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/aros/dpview/internal/api"
	"codeberg.org/aros/dpview/internal/files"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, []byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

type typstRenderer struct {
	runner   CommandRunner
	tempRoot string
	status   api.RendererStatus
}

type typstTheme struct {
	Page     string
	Text     string
	Heading  string
	Link     string
	Quote    string
	Code     string
	CodeFill string
	Border   string
}

func newTypstRenderer(binary string) (*typstRenderer, error) {
	tempRoot, err := os.MkdirTemp("", "dpview-typst-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}

	renderer := &typstRenderer{
		runner:   ExecRunner{},
		tempRoot: tempRoot,
	}

	if binary != "" {
		if abs, err := filepath.Abs(binary); err == nil {
			if _, statErr := os.Stat(abs); statErr == nil {
				renderer.status = api.RendererStatus{Kind: files.KindTypst, Name: "Typst", Available: true, Details: map[string]string{"path": abs}}
			} else {
				renderer.status = api.RendererStatus{Kind: files.KindTypst, Name: "Typst", Available: false, Details: map[string]string{"path": abs, "reason": statErr.Error()}}
			}
		}
	} else if typstPath, err := exec.LookPath("typst"); err == nil {
		renderer.status = api.RendererStatus{Kind: files.KindTypst, Name: "Typst", Available: true, Details: map[string]string{"path": typstPath}}
	} else {
		renderer.status = api.RendererStatus{Kind: files.KindTypst, Name: "Typst", Available: false, Details: map[string]string{"reason": "typst binary not found on PATH"}}
	}

	return renderer, nil
}

func (r *typstRenderer) Kind() files.Kind {
	return files.KindTypst
}

func (r *typstRenderer) Status() api.RendererStatus {
	return r.status
}

func (r *typstRenderer) SetRunner(runner CommandRunner) {
	r.runner = runner
}

func (r *typstRenderer) Close() error {
	return os.RemoveAll(r.tempRoot)
}

func (r *typstRenderer) Render(ctx context.Context, req RenderRequest) api.Preview {
	if !r.status.Available {
		return errPreview(req.Started, "typst_unavailable", "Typst CLI is not available", r.status.Details["reason"])
	}

	renderDir, err := os.MkdirTemp(r.tempRoot, "render-*")
	if err != nil {
		return errPreview(req.Started, "internal_error", "Failed to create Typst render directory", err.Error())
	}
	defer os.RemoveAll(renderDir)

	pattern := filepath.Join(renderDir, "page-{p}.svg")
	compileSource := req.AbsPath
	root := req.Root
	if root == "" {
		root = filepath.Dir(req.AbsPath)
	}
	if req.Settings.TypstPreviewTheme {
		wrapperDir, err := os.MkdirTemp(root, ".dpview-wrapper-*")
		if err != nil {
			return errPreview(req.Started, "internal_error", "Failed to create Typst wrapper directory", err.Error())
		}
		defer os.RemoveAll(wrapperDir)
		wrapperPath := filepath.Join(wrapperDir, "dpview-wrapper.typ")
		if err := os.WriteFile(wrapperPath, []byte(buildTypstWrapper(req.Info.Path, req.Settings)), 0o644); err != nil {
			return errPreview(req.Started, "internal_error", "Failed to prepare Typst theme wrapper", err.Error())
		}
		compileSource = wrapperPath
	}
	_, stderr, err := r.runner.Run(ctx, r.status.Details["path"], "compile", "--root", root, compileSource, pattern)
	if err != nil {
		code := "typst_compile_failed"
		msg := "Failed to render Typst document"
		if ctx.Err() == context.DeadlineExceeded {
			code = "render_timeout"
			msg = "Typst render timed out"
		}
		detail := strings.TrimSpace(string(stderr))
		if detail == "" {
			detail = err.Error()
		}
		return errPreview(req.Started, code, msg, detail)
	}

	entries, err := os.ReadDir(renderDir)
	if err != nil {
		return errPreview(req.Started, "internal_error", "Failed to read Typst render output", err.Error())
	}
	pages := make([]string, 0)
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".svg" {
			continue
		}
		pages = append(pages, filepath.Join(renderDir, entry.Name()))
	}
	slices.Sort(pages)
	if len(pages) == 0 {
		return errPreview(req.Started, "typst_compile_failed", "Typst produced no SVG output", req.Info.Path)
	}

	var htmlOut strings.Builder
	htmlOut.WriteString(`<div class="typst-pages">`)
	for i, page := range pages {
		data, err := os.ReadFile(page)
		if err != nil {
			return errPreview(req.Started, "internal_error", "Failed to read Typst SVG output", err.Error())
		}
		htmlOut.WriteString(`<section class="typst-page" data-page="`)
		htmlOut.WriteString(fmt.Sprintf("%d", i+1))
		htmlOut.WriteString(`">`)
		htmlOut.Write(data)
		htmlOut.WriteString(`</section>`)
	}
	htmlOut.WriteString(`</div>`)

	return api.Preview{
		HTML:             htmlOut.String(),
		SourceLineCount:  countSourceLines(req.Source),
		UpdatedAt:        time.Now().UTC(),
		RenderDurationMS: time.Since(req.Started).Milliseconds(),
		Status:           api.RenderStatusReady,
	}
}

func buildTypstWrapper(sourcePath string, settings api.Settings) string {
	theme := resolveTypstTheme(settings)
	source := "/" + strings.TrimPrefix(filepath.ToSlash(sourcePath), "/")
	return strings.Join([]string{
		fmt.Sprintf("#let dpview-page = rgb(%q)", theme.Page),
		fmt.Sprintf("#let dpview-text = rgb(%q)", theme.Text),
		fmt.Sprintf("#let dpview-heading = rgb(%q)", theme.Heading),
		fmt.Sprintf("#let dpview-link = rgb(%q)", theme.Link),
		fmt.Sprintf("#let dpview-quote = rgb(%q)", theme.Quote),
		fmt.Sprintf("#let dpview-code = rgb(%q)", theme.Code),
		fmt.Sprintf("#let dpview-code-fill = rgb(%q)", theme.CodeFill),
		fmt.Sprintf("#let dpview-border = rgb(%q)", theme.Border),
		"#set page(fill: dpview-page)",
		"#set text(fill: dpview-text)",
		"#show heading: set text(fill: dpview-heading)",
		"#show link: set text(fill: dpview-link)",
		"#show raw: set text(fill: dpview-code)",
		"#show raw.where(block: true): set block(fill: dpview-code-fill, stroke: (paint: dpview-border, thickness: 0.6pt), inset: 10pt, radius: 6pt)",
		"#show quote: set block(stroke: (left: (paint: dpview-quote, thickness: 2pt)), inset: (left: 10pt))",
		fmt.Sprintf("#include %s", strconv.Quote(source)),
		"",
	}, "\n")
}

func resolveTypstTheme(settings api.Settings) typstTheme {
	mode := settings.Theme
	if mode == "" || mode == "system" {
		mode = "light"
	}
	preview := settings.PreviewTheme
	if preview == "" {
		preview = "default"
	}

	switch preview {
	case "github":
		if mode == "dark" {
			return typstTheme{
				Page:     "#0d1117",
				Text:     "#c9d1d9",
				Heading:  "#f0f6fc",
				Link:     "#58a6ff",
				Quote:    "#3fb950",
				Code:     "#c9d1d9",
				CodeFill: "#161b22",
				Border:   "#30363d",
			}
		}
		return typstTheme{
			Page:     "#ffffff",
			Text:     "#24292f",
			Heading:  "#1f2328",
			Link:     "#0969da",
			Quote:    "#1f883d",
			Code:     "#24292f",
			CodeFill: "#f6f8fa",
			Border:   "#d0d7de",
		}
	case "notion":
		if mode == "dark" {
			return typstTheme{
				Page:     "#191919",
				Text:     "#e8e8e8",
				Heading:  "#ffffff",
				Link:     "#7cc4ff",
				Quote:    "#8f8f8f",
				Code:     "#f5f5f5",
				CodeFill: "#242424",
				Border:   "#333333",
			}
		}
		return typstTheme{
			Page:     "#ffffff",
			Text:     "#37352f",
			Heading:  "#2f3437",
			Link:     "#0b6e99",
			Quote:    "#9b9a97",
			Code:     "#24292f",
			CodeFill: "#f7f6f3",
			Border:   "#e9e9e7",
		}
	case "paper":
		if mode == "dark" {
			return typstTheme{
				Page:     "#1a1816",
				Text:     "#e8decf",
				Heading:  "#f7efe2",
				Link:     "#d9b36c",
				Quote:    "#b08968",
				Code:     "#f2e9dc",
				CodeFill: "#24201d",
				Border:   "#4a4037",
			}
		}
		return typstTheme{
			Page:     "#f6f1e8",
			Text:     "#2d241d",
			Heading:  "#1f1712",
			Link:     "#8c4f24",
			Quote:    "#a26a3d",
			Code:     "#2d241d",
			CodeFill: "#efe5d6",
			Border:   "#d2c1a8",
		}
	default:
		if mode == "dark" {
			return typstTheme{
				Page:     "#111111",
				Text:     "#e8e8e8",
				Heading:  "#ffffff",
				Link:     "#8ab4f8",
				Quote:    "#8f8f8f",
				Code:     "#f3f3f3",
				CodeFill: "#1b1b1b",
				Border:   "#303030",
			}
		}
		return typstTheme{
			Page:     "#ffffff",
			Text:     "#111111",
			Heading:  "#111111",
			Link:     "#0b57d0",
			Quote:    "#666666",
			Code:     "#111111",
			CodeFill: "#f5f5f5",
			Border:   "#d0d0d0",
		}
	}
}
