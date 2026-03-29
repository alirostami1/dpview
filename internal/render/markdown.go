package render

import (
	"bytes"
	"context"
	"time"

	"codeberg.org/aros/dpview/internal/api"
	"codeberg.org/aros/dpview/internal/files"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

type markdownRenderer struct {
	md       goldmark.Markdown
	sanitize *bluemonday.Policy
}

func newMarkdownRenderer() *markdownRenderer {
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.TaskList,
			extension.Linkify,
		),
		goldmark.WithRendererOptions(
			gmhtml.WithHardWraps(),
			gmhtml.WithXHTML(),
		),
	)
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("type", "checked", "disabled").OnElements("input")
	policy.AllowAttrs("class").OnElements("article")
	return &markdownRenderer{md: md, sanitize: policy}
}

func (r *markdownRenderer) Kind() files.Kind {
	return files.KindMarkdown
}

func (r *markdownRenderer) Status() api.RendererStatus {
	return api.RendererStatus{
		Kind:      files.KindMarkdown,
		Name:      "Markdown",
		Available: true,
	}
}

func (r *markdownRenderer) Render(_ context.Context, req RenderRequest) api.Preview {
	var out bytes.Buffer
	if err := r.md.Convert(req.Source, &out); err != nil {
		return errPreview(req.Started, "internal_error", "Failed to render Markdown", err.Error())
	}
	safe := r.sanitize.SanitizeBytes(out.Bytes())
	return api.Preview{
		HTML:             `<article class="markdown-theme">` + string(safe) + `</article>`,
		UpdatedAt:        time.Now().UTC(),
		RenderDurationMS: time.Since(req.Started).Milliseconds(),
		Status:           api.RenderStatusReady,
	}
}
