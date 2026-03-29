package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"strings"
	"time"

	"codeberg.org/aros/dpview/internal/api"
	"codeberg.org/aros/dpview/internal/files"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"gopkg.in/yaml.v3"
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
	body := req.Source
	frontMatter, strippedBody, ok, err := parseMarkdownFrontMatter(req.Source)
	if err != nil {
		body = req.Source
		frontMatter = nil
	} else if ok {
		body = strippedBody
	}

	titleHTML := ""
	if frontMatter != nil && req.Settings.MarkdownFrontMatterTitle && frontMatter.Title != "" && !markdownHasH1(r.md, body) {
		frontMatter.TitleUsed = true
		titleHTML = "<h1>" + html.EscapeString(frontMatter.Title) + "</h1>"
	}

	var out bytes.Buffer
	if err := r.md.Convert(body, &out); err != nil {
		return errPreview(req.Started, "internal_error", "Failed to render Markdown", err.Error())
	}
	safe := r.sanitize.SanitizeBytes(out.Bytes())
	return api.Preview{
		HTML:             `<article class="markdown-theme">` + titleHTML + string(safe) + `</article>`,
		FrontMatter:      frontMatter,
		UpdatedAt:        time.Now().UTC(),
		RenderDurationMS: time.Since(req.Started).Milliseconds(),
		Status:           api.RenderStatusReady,
	}
}

func parseMarkdownFrontMatter(source []byte) (*api.FrontMatter, []byte, bool, error) {
	content := strings.ReplaceAll(string(bytes.TrimPrefix(source, []byte("\xef\xbb\xbf"))), "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return nil, source, false, nil
	}

	end := strings.Index(content[4:], "\n---\n")
	if end < 0 {
		return nil, source, false, nil
	}
	end += 4
	raw := content[4:end]
	body := content[end+5:]
	if strings.HasPrefix(body, "\n") {
		body = body[1:]
	}

	var doc yaml.Node
	if err := yaml.Unmarshal([]byte(raw), &doc); err != nil {
		return nil, source, false, err
	}
	if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
		return nil, source, false, fmt.Errorf("front matter must be a YAML mapping")
	}

	meta := &api.FrontMatter{
		Format:  "yaml",
		Entries: make([]api.FrontMatterEntry, 0, len(doc.Content[0].Content)/2),
	}
	for i := 0; i+1 < len(doc.Content[0].Content); i += 2 {
		keyNode := doc.Content[0].Content[i]
		valueNode := doc.Content[0].Content[i+1]
		value, err := yamlNodeDisplayValue(valueNode)
		if err != nil {
			return nil, source, false, err
		}
		meta.Entries = append(meta.Entries, api.FrontMatterEntry{
			Key:   keyNode.Value,
			Value: value,
		})
		if keyNode.Value == "title" && meta.Title == "" {
			var title string
			if err := valueNode.Decode(&title); err == nil {
				meta.Title = strings.TrimSpace(title)
			}
		}
	}

	return meta, []byte(body), true, nil
}

func yamlNodeDisplayValue(node *yaml.Node) (string, error) {
	var value any
	if err := node.Decode(&value); err != nil {
		return "", err
	}
	switch typed := value.(type) {
	case nil:
		return "null", nil
	case string:
		return typed, nil
	case bool:
		if typed {
			return "true", nil
		}
		return "false", nil
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return fmt.Sprint(typed), nil
	default:
		data, err := json.Marshal(value)
		if err != nil {
			return "", err
		}
		return string(data), nil
	}
}

func markdownHasH1(md goldmark.Markdown, source []byte) bool {
	doc := md.Parser().Parse(text.NewReader(source))
	found := false
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		heading, ok := node.(*ast.Heading)
		if ok && heading.Level == 1 {
			found = true
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	return found
}
