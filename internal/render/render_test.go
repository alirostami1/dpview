package render

import (
	"container/list"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"codeberg.org/aros/dpview/internal/api"
	"codeberg.org/aros/dpview/internal/files"
)

func TestRenderMarkdownSupportsCommonFeatures(t *testing.T) {
	svc, err := NewService(Config{MaxFileSize: 1 << 20, RenderTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	defer svc.Close()

	root := t.TempDir()
	path := filepath.Join(root, "sample.md")
	content := strings.Join([]string{
		"# Heading",
		"",
		"- item",
		"- [x] done",
		"",
		"| a | b |",
		"| - | - |",
		"| 1 | 2 |",
		"",
		"```go",
		"println(\"hi\")",
		"```",
		"",
		"[link](https://example.com)",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	preview := svc.Render(context.Background(), files.FileInfo{Path: "sample.md", Kind: files.KindMarkdown}, path, api.Settings{LatexEnabled: true})
	if preview.Error != nil {
		t.Fatalf("Render() error = %+v", preview.Error)
	}

	checks := []string{"<article class=\"markdown-theme\">", "<h1 data-source-start-line=\"1\" data-source-end-line=\"1\">Heading</h1>", "<ul", "<table", "<pre", "type=\"checkbox\"", "href=\"https://example.com\""}
	for _, check := range checks {
		if !strings.Contains(preview.HTML, check) {
			t.Fatalf("Render() HTML missing %q", check)
		}
	}
	if preview.SourceLineCount != strings.Count(content, "\n")+1 {
		t.Fatalf("Render() source line count = %d", preview.SourceLineCount)
	}
	lineChecks := []string{
		`<h1 data-source-start-line="1" data-source-end-line="1">`,
		`<li data-source-start-line="3" data-source-end-line="3">item</li>`,
		`<li data-source-start-line="4" data-source-end-line="4"><input checked="" disabled="" type="checkbox"/> done</li>`,
		`<tr data-source-start-line="8" data-source-end-line="8">`,
		`<pre data-source-start-line="11" data-source-end-line="11"><code>`,
		`<p data-source-start-line="14" data-source-end-line="14"><a href="https://example.com"`,
	}
	for _, check := range lineChecks {
		if !strings.Contains(preview.HTML, check) {
			t.Fatalf("Render() HTML missing source anchor %q", check)
		}
	}
}

func TestRenderMarkdownParsesFrontMatterAndInjectsTitle(t *testing.T) {
	svc, err := NewService(Config{MaxFileSize: 1 << 20, RenderTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	defer svc.Close()

	root := t.TempDir()
	path := filepath.Join(root, "frontmatter.md")
	content := strings.Join([]string{
		"---",
		"title: Frontmatter Title",
		"tags:",
		"  - docs",
		"  - markdown",
		"author:",
		"  name: Aros",
		"---",
		"",
		"Body paragraph.",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	preview := svc.Render(context.Background(), files.FileInfo{Path: "frontmatter.md", Kind: files.KindMarkdown}, path, api.Settings{
		MarkdownFrontMatterTitle: true,
	})
	if preview.Error != nil {
		t.Fatalf("Render() error = %+v", preview.Error)
	}
	if strings.Contains(preview.HTML, "title: Frontmatter Title") {
		t.Fatalf("Render() HTML should not include raw front matter: %q", preview.HTML)
	}
	if !strings.Contains(preview.HTML, "<h1>Frontmatter Title</h1>") {
		t.Fatalf("Render() HTML missing injected title: %q", preview.HTML)
	}
	if !strings.Contains(preview.HTML, `<p data-source-start-line="10" data-source-end-line="10">Body paragraph.</p>`) {
		t.Fatalf("Render() HTML missing offset source line mapping: %q", preview.HTML)
	}
	if preview.FrontMatter == nil || preview.FrontMatter.Format != "yaml" || !preview.FrontMatter.TitleUsed {
		t.Fatalf("Render() front matter = %+v", preview.FrontMatter)
	}
	if len(preview.FrontMatter.Entries) != 3 {
		t.Fatalf("Render() front matter entries = %+v", preview.FrontMatter.Entries)
	}
	if preview.FrontMatter.Entries[1].Value != "[\"docs\",\"markdown\"]" {
		t.Fatalf("Render() tag entry = %+v", preview.FrontMatter.Entries[1])
	}
}

func TestRenderSourceMarkdownSupportsTransientContent(t *testing.T) {
	svc, err := NewService(Config{MaxFileSize: 1 << 20, RenderTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	defer svc.Close()

	root := t.TempDir()
	path := filepath.Join(root, "draft.md")
	if err := os.WriteFile(path, []byte("# Saved"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	preview := svc.RenderSource(
		context.Background(),
		files.FileInfo{Path: "draft.md", Name: "draft.md", Kind: files.KindMarkdown},
		path,
		[]byte("# Draft\n"),
		api.Settings{LatexEnabled: true},
		true,
	)
	if preview.Error != nil {
		t.Fatalf("RenderSource() error = %+v", preview.Error)
	}
	if !strings.Contains(preview.HTML, ">Draft</h1>") || strings.Contains(preview.HTML, ">Saved</h1>") {
		t.Fatalf("RenderSource() HTML = %q", preview.HTML)
	}
}

func TestRenderMarkdownFrontMatterTitleDoesNotOverrideExistingH1(t *testing.T) {
	svc, err := NewService(Config{MaxFileSize: 1 << 20, RenderTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	defer svc.Close()

	root := t.TempDir()
	path := filepath.Join(root, "frontmatter-h1.md")
	content := strings.Join([]string{
		"---",
		"title: Frontmatter Title",
		"---",
		"",
		"# Existing Title",
		"",
		"Body paragraph.",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	preview := svc.Render(context.Background(), files.FileInfo{Path: "frontmatter-h1.md", Kind: files.KindMarkdown}, path, api.Settings{
		MarkdownFrontMatterTitle: true,
	})
	if preview.Error != nil {
		t.Fatalf("Render() error = %+v", preview.Error)
	}
	if strings.Count(preview.HTML, "<h1") != 1 || !strings.Contains(preview.HTML, ">Existing Title</h1>") {
		t.Fatalf("Render() HTML = %q", preview.HTML)
	}
	if preview.FrontMatter == nil || preview.FrontMatter.TitleUsed {
		t.Fatalf("Render() front matter = %+v", preview.FrontMatter)
	}
}

func TestRenderMarkdownSanitizesUnsafeLinks(t *testing.T) {
	svc, err := NewService(Config{MaxFileSize: 1 << 20, RenderTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	defer svc.Close()

	root := t.TempDir()
	path := filepath.Join(root, "unsafe.md")
	if err := os.WriteFile(path, []byte(`[bad](javascript:alert(1))`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	preview := svc.Render(context.Background(), files.FileInfo{Path: "unsafe.md", Kind: files.KindMarkdown}, path, api.Settings{LatexEnabled: true})
	if preview.Error != nil {
		t.Fatalf("Render() error = %+v", preview.Error)
	}
	if strings.Contains(preview.HTML, "javascript:alert") {
		t.Fatalf("Render() HTML should sanitize javascript URLs: %q", preview.HTML)
	}
}

func TestRenderMarkdownSupportsFootnotes(t *testing.T) {
	svc, err := NewService(Config{MaxFileSize: 1 << 20, RenderTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	defer svc.Close()

	root := t.TempDir()
	path := filepath.Join(root, "footnotes.md")
	content := strings.Join([]string{
		"Paragraph with a footnote.[^1]",
		"",
		"[^1]: Footnote body.",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	preview := svc.Render(context.Background(), files.FileInfo{Path: "footnotes.md", Kind: files.KindMarkdown}, path, api.Settings{LatexEnabled: true})
	if preview.Error != nil {
		t.Fatalf("Render() error = %+v", preview.Error)
	}

	checks := []string{
		`<sup id="fnref:1">`,
		`href="#fn:1"`,
		`class="footnote-ref"`,
		`role="doc-noteref"`,
		`<div class="footnotes" role="doc-endnotes">`,
		`id="fn:1"`,
		`href="#fnref:1"`,
		`class="footnote-backref"`,
		`role="doc-backlink"`,
	}
	for _, check := range checks {
		if !strings.Contains(preview.HTML, check) {
			t.Fatalf("Render() HTML missing %q", check)
		}
	}
}

func TestRenderMarkdownRewritesDisplayMathBlocks(t *testing.T) {
	svc, err := NewService(Config{MaxFileSize: 1 << 20, RenderTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	defer svc.Close()

	root := t.TempDir()
	path := filepath.Join(root, "math.md")
	content := strings.Join([]string{
		"Before math.",
		"",
		"$$",
		`\\int_{-\\infty}^{\\infty} e^{-x^2} \\, dx = \\sqrt{\\pi}`,
		"$$",
		"",
		"After math.",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	preview := svc.Render(context.Background(), files.FileInfo{Path: "math.md", Kind: files.KindMarkdown}, path, api.Settings{LatexEnabled: true})
	if preview.Error != nil {
		t.Fatalf("Render() error = %+v", preview.Error)
	}
	if !strings.Contains(preview.HTML, `class="markdown-math-block"`) {
		t.Fatalf("Render() HTML missing math placeholder: %q", preview.HTML)
	}
	if !strings.Contains(preview.HTML, `data-latex="\int_{-\infty}^{\infty} e^{-x^2} \, dx = \sqrt{\pi}"`) {
		t.Fatalf("Render() HTML missing math expression: %q", preview.HTML)
	}
}

func TestRenderMarkdownLeavesMathRawWhenLatexDisabled(t *testing.T) {
	svc, err := NewService(Config{MaxFileSize: 1 << 20, RenderTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	defer svc.Close()

	root := t.TempDir()
	path := filepath.Join(root, "math-disabled.md")
	content := strings.Join([]string{
		"Before math.",
		"",
		"$$",
		`\\int_{-\\infty}^{\\infty} e^{-x^2} \\, dx = \\sqrt{\\pi}`,
		"$$",
		"",
		"After math.",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	preview := svc.Render(context.Background(), files.FileInfo{Path: "math-disabled.md", Kind: files.KindMarkdown}, path, api.Settings{LatexEnabled: false})
	if preview.Error != nil {
		t.Fatalf("Render() error = %+v", preview.Error)
	}
	if strings.Contains(preview.HTML, `class="markdown-math-block"`) {
		t.Fatalf("Render() HTML unexpectedly rewrote math placeholder: %q", preview.HTML)
	}
	if !strings.Contains(preview.HTML, `$$`) {
		t.Fatalf("Render() HTML should keep raw math delimiters: %q", preview.HTML)
	}
}

func TestRenderCacheSeparatesLatexEnabledSetting(t *testing.T) {
	svc, err := NewService(Config{MaxFileSize: 1 << 20, RenderTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	defer svc.Close()

	root := t.TempDir()
	path := filepath.Join(root, "math-cache.md")
	content := strings.Join([]string{
		"Before math.",
		"",
		"$$",
		`\\int_{-\\infty}^{\\infty} e^{-x^2} \\, dx = \\sqrt{\\pi}`,
		"$$",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	info := files.FileInfo{Path: "math-cache.md", Kind: files.KindMarkdown}

	enabled := svc.Render(context.Background(), info, path, api.Settings{LatexEnabled: true})
	if enabled.Error != nil {
		t.Fatalf("Render(enabled) error = %+v", enabled.Error)
	}
	if !strings.Contains(enabled.HTML, `class="markdown-math-block"`) {
		t.Fatalf("Render(enabled) HTML missing math placeholder: %q", enabled.HTML)
	}

	disabled := svc.Render(context.Background(), info, path, api.Settings{LatexEnabled: false})
	if disabled.Error != nil {
		t.Fatalf("Render(disabled) error = %+v", disabled.Error)
	}
	if strings.Contains(disabled.HTML, `class="markdown-math-block"`) {
		t.Fatalf("Render(disabled) HTML unexpectedly reused math placeholder: %q", disabled.HTML)
	}
	if !strings.Contains(disabled.HTML, `$$`) {
		t.Fatalf("Render(disabled) HTML should keep raw math delimiters: %q", disabled.HTML)
	}
}

func TestRenderMarkdownSupportsRepeatedFootnoteReferences(t *testing.T) {
	svc, err := NewService(Config{MaxFileSize: 1 << 20, RenderTimeout: 2 * time.Second})
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}
	defer svc.Close()

	root := t.TempDir()
	path := filepath.Join(root, "footnotes-repeat.md")
	content := strings.Join([]string{
		"One[^same]",
		"",
		"Two[^same]",
		"",
		"[^same]: Shared footnote.",
	}, "\n")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	preview := svc.Render(context.Background(), files.FileInfo{Path: "footnotes-repeat.md", Kind: files.KindMarkdown}, path, api.Settings{LatexEnabled: true})
	if preview.Error != nil {
		t.Fatalf("Render() error = %+v", preview.Error)
	}

	checks := []string{
		`<sup id="fnref:1">`,
		`<sup id="fnref1:1">`,
		`href="#fn:1"`,
		`href="#fnref:1"`,
		`href="#fnref1:1"`,
		`class="footnote-backref"`,
		`role="doc-backlink"`,
	}
	for _, check := range checks {
		if !strings.Contains(preview.HTML, check) {
			t.Fatalf("Render() HTML missing %q", check)
		}
	}
}

func TestRenderTypstMissingBinaryReturnsClearError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.typ")
	if err := os.WriteFile(path, []byte("= demo"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	svc := &Service{
		limits:    api.Limits{MaxFileSizeBytes: 1 << 20, RenderTimeoutMS: 2000},
		renderers: map[files.Kind]DocumentRenderer{files.KindTypst: &typstRenderer{}},
	}
	preview := svc.Render(context.Background(), files.FileInfo{Path: "demo.typ", Kind: files.KindTypst}, path, api.Settings{})
	if preview.Error == nil || preview.Error.Code != "typst_unavailable" {
		t.Fatalf("Render() error = %+v", preview.Error)
	}
}

func TestRenderTypstSuccessReadsSVGPages(t *testing.T) {
	tempRoot := t.TempDir()
	path := filepath.Join(tempRoot, "demo.typ")
	if err := os.WriteFile(path, []byte("= demo"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	var renderDir string
	var sawQuery bool
	typst := &typstRenderer{
		tempRoot: tempRoot,
		status:   api.RendererStatus{Kind: files.KindTypst, Name: "Typst", Available: true, Details: map[string]string{"path": "typst"}},
		runner: mockRunner(func(_ context.Context, _ string, args ...string) ([]byte, []byte, error) {
			switch args[0] {
			case "query":
				sawQuery = true
				return []byte(`[{"start_line":1,"end_line":1,"page":1,"x":"0pt","y":"24pt"}]`), nil, nil
			case "compile":
			default:
				return nil, nil, errors.New("unexpected typst command")
			}
			if args[2] != tempRoot {
				return nil, nil, errors.New("expected project root to be passed to typst")
			}
			wrapper, err := os.ReadFile(args[3])
			if err != nil {
				return nil, nil, err
			}
			if !strings.Contains(string(wrapper), `#let dpview-page = rgb("#0d1117")`) {
				return nil, nil, errors.New("wrapper missing theme tokens")
			}
			includeLine := ""
			for _, line := range strings.Split(string(wrapper), "\n") {
				if strings.HasPrefix(line, "#include ") {
					includeLine = line
					break
				}
			}
			if includeLine == "" {
				return nil, nil, errors.New("wrapper missing include path")
			}
			includePath, unquoteErr := strconv.Unquote(strings.TrimSpace(strings.TrimPrefix(includeLine, "#include ")))
			if unquoteErr != nil {
				return nil, nil, unquoteErr
			}
			seekSource, err := os.ReadFile(filepath.Join(tempRoot, includePath))
			if err != nil {
				return nil, nil, err
			}
			if !strings.Contains(string(seekSource), "<dpview-anchor>") || !strings.Contains(string(seekSource), "= demo") {
				return nil, nil, errors.New("seek shadow source missing anchors")
			}
			renderDir = filepath.Dir(args[4])
			pageOne := strings.ReplaceAll(args[4], "{p}", "1")
			pageTwo := strings.ReplaceAll(args[4], "{p}", "2")
			if err := os.WriteFile(pageOne, []byte("<svg><text>one</text></svg>"), 0o644); err != nil {
				return nil, nil, err
			}
			if err := os.WriteFile(pageTwo, []byte("<svg><text>two</text></svg>"), 0o644); err != nil {
				return nil, nil, err
			}
			return nil, nil, nil
		}),
	}
	svc := &Service{
		root:      tempRoot,
		limits:    api.Limits{MaxFileSizeBytes: 1 << 20, RenderTimeoutMS: 2000},
		renderers: map[files.Kind]DocumentRenderer{files.KindTypst: typst},
	}

	preview := svc.Render(context.Background(), files.FileInfo{Path: "demo.typ", Kind: files.KindTypst}, path, api.Settings{
		Theme:             "dark",
		PreviewTheme:      "github",
		TypstPreviewTheme: true,
	})
	if preview.Error != nil {
		t.Fatalf("Render() error = %+v", preview.Error)
	}
	if !strings.Contains(preview.HTML, "data-page=\"1\"") || !strings.Contains(preview.HTML, "<svg><text>two</text></svg>") {
		t.Fatalf("Render() HTML = %q", preview.HTML)
	}
	if !sawQuery {
		t.Fatalf("Render() did not query typst anchors")
	}
	if len(preview.TypstSeekAnchors) != 1 || preview.TypstSeekAnchors[0].Y != 24 {
		t.Fatalf("Render() typst seek anchors = %+v", preview.TypstSeekAnchors)
	}
	if preview.SourceLineCount != 1 {
		t.Fatalf("Render() source line count = %d", preview.SourceLineCount)
	}
	if _, err := os.Stat(renderDir); !os.IsNotExist(err) {
		t.Fatalf("expected render dir cleanup, stat err = %v", err)
	}
}

func TestRenderTypstWithoutPreviewThemeCompilesSourceDirectly(t *testing.T) {
	tempRoot := t.TempDir()
	path := filepath.Join(tempRoot, "demo.typ")
	if err := os.WriteFile(path, []byte("= demo"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	typst := &typstRenderer{
		tempRoot: tempRoot,
		status:   api.RendererStatus{Kind: files.KindTypst, Name: "Typst", Available: true, Details: map[string]string{"path": "typst"}},
		runner: mockRunner(func(_ context.Context, _ string, args ...string) ([]byte, []byte, error) {
			if args[0] == "query" {
				return []byte(`[]`), nil, nil
			}
			compiled, err := os.ReadFile(args[3])
			if err != nil {
				return nil, nil, err
			}
			if !strings.Contains(string(compiled), "= demo") || !strings.Contains(string(compiled), "<dpview-anchor>") {
				return nil, nil, errors.New("expected anchored typst source")
			}
			pageOne := strings.ReplaceAll(args[4], "{p}", "1")
			if err := os.WriteFile(pageOne, []byte("<svg><text>plain</text></svg>"), 0o644); err != nil {
				return nil, nil, err
			}
			return nil, nil, nil
		}),
	}
	svc := &Service{
		root:      tempRoot,
		limits:    api.Limits{MaxFileSizeBytes: 1 << 20, RenderTimeoutMS: 2000},
		renderers: map[files.Kind]DocumentRenderer{files.KindTypst: typst},
	}

	preview := svc.Render(context.Background(), files.FileInfo{Path: "demo.typ", Kind: files.KindTypst}, path, api.Settings{TypstPreviewTheme: false})
	if preview.Error != nil {
		t.Fatalf("Render() error = %+v", preview.Error)
	}
	if !strings.Contains(preview.HTML, "<svg><text>plain</text></svg>") {
		t.Fatalf("Render() HTML = %q", preview.HTML)
	}
}

func TestRenderTypstCompileFailureIncludesStderr(t *testing.T) {
	path := filepath.Join(t.TempDir(), "demo.typ")
	if err := os.WriteFile(path, []byte("= demo"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	typst := &typstRenderer{
		tempRoot: t.TempDir(),
		status:   api.RendererStatus{Kind: files.KindTypst, Name: "Typst", Available: true, Details: map[string]string{"path": "typst"}},
		runner: mockRunner(func(_ context.Context, _ string, _ ...string) ([]byte, []byte, error) {
			return nil, []byte("compile failed"), errors.New("exit status 1")
		}),
	}
	svc := &Service{
		limits:    api.Limits{MaxFileSizeBytes: 1 << 20, RenderTimeoutMS: 2000},
		renderers: map[files.Kind]DocumentRenderer{files.KindTypst: typst},
	}

	preview := svc.Render(context.Background(), files.FileInfo{Path: "demo.typ", Kind: files.KindTypst}, path, api.Settings{})
	if preview.Error == nil || preview.Error.Code != "typst_compile_failed" || !strings.Contains(preview.Error.Detail, "compile failed") {
		t.Fatalf("Render() error = %+v", preview.Error)
	}
}

type mockRunner func(context.Context, string, ...string) ([]byte, []byte, error)

func (m mockRunner) Run(ctx context.Context, name string, args ...string) ([]byte, []byte, error) {
	return m(ctx, name, args...)
}

func TestRenderCacheEvictsOldEntries(t *testing.T) {
	svc := &Service{
		cache:     make(map[string]*list.Element),
		cacheList: list.New(),
	}
	for i := 0; i < maxCacheEntries+5; i++ {
		svc.storeCache(string(rune('a'+(i%26)))+strconv.Itoa(i), api.Preview{Status: api.RenderStatusReady})
	}
	if svc.cacheList.Len() != maxCacheEntries {
		t.Fatalf("cache size = %d", svc.cacheList.Len())
	}
}
