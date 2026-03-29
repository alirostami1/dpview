package render

import (
	"context"
	"errors"
	"os"
	"path/filepath"
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

	preview := svc.Render(context.Background(), files.FileInfo{Path: "sample.md", Kind: files.KindMarkdown}, path, api.Settings{})
	if preview.Error != nil {
		t.Fatalf("Render() error = %+v", preview.Error)
	}

	checks := []string{"<article class=\"markdown-theme\">", "<h1>Heading</h1>", "<ul>", "<table>", "<pre><code", "type=\"checkbox\"", "href=\"https://example.com\""}
	for _, check := range checks {
		if !strings.Contains(preview.HTML, check) {
			t.Fatalf("Render() HTML missing %q", check)
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
	if strings.Count(preview.HTML, "<h1>") != 1 || !strings.Contains(preview.HTML, "<h1>Existing Title</h1>") {
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

	preview := svc.Render(context.Background(), files.FileInfo{Path: "unsafe.md", Kind: files.KindMarkdown}, path, api.Settings{})
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

	preview := svc.Render(context.Background(), files.FileInfo{Path: "footnotes.md", Kind: files.KindMarkdown}, path, api.Settings{})
	if preview.Error != nil {
		t.Fatalf("Render() error = %+v", preview.Error)
	}

	checks := []string{
		`<sup id="fnref:1">`,
		`href="#fn:1"`,
		`class="footnote-ref"`,
		`role="doc-noteref"`,
		`<div class="footnotes" role="doc-endnotes">`,
		`<li id="fn:1">`,
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

	preview := svc.Render(context.Background(), files.FileInfo{Path: "footnotes-repeat.md", Kind: files.KindMarkdown}, path, api.Settings{})
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
	typst := &typstRenderer{
		tempRoot: tempRoot,
		status:   api.RendererStatus{Kind: files.KindTypst, Name: "Typst", Available: true, Details: map[string]string{"path": "typst"}},
		runner: mockRunner(func(_ context.Context, _ string, args ...string) ([]byte, []byte, error) {
			wrapper, err := os.ReadFile(args[3])
			if err != nil {
				return nil, nil, err
			}
			if !strings.Contains(string(wrapper), `#include `) || !strings.Contains(string(wrapper), `#let dpview-page = rgb("#0d1117")`) {
				return nil, nil, errors.New("wrapper missing theme tokens")
			}
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
			if args[3] != path {
				return nil, nil, errors.New("expected direct typst source compile")
			}
			pageOne := strings.ReplaceAll(args[4], "{p}", "1")
			if err := os.WriteFile(pageOne, []byte("<svg><text>plain</text></svg>"), 0o644); err != nil {
				return nil, nil, err
			}
			return nil, nil, nil
		}),
	}
	svc := &Service{
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
