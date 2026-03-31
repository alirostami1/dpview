package render

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"html"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"codeberg.org/aros/dpview/internal/api"
	"codeberg.org/aros/dpview/internal/files"
	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/renderer"
	gmhtml "github.com/yuin/goldmark/renderer/html"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
	"gopkg.in/yaml.v3"
)

type markdownRenderer struct {
	sanitize *bluemonday.Policy
}

var markdownParagraphPattern = regexp.MustCompile(`(?s)<p\b[^>]*>(.*?)</p>`)
var markdownReferenceDefinitionPattern = regexp.MustCompile(`(?m)^[ \t]{0,3}\[[^\]]+\]:`)
var markdownFootnoteDefinitionPattern = regexp.MustCompile(`(?m)^[ \t]{0,3}\[\^[^\]]+\]:`)
var markdownFencePattern = regexp.MustCompile("(?m)^[ \\t]{0,3}(```|~~~)")
var markdownATXH1Pattern = regexp.MustCompile(`(?m)^[ \t]{0,3}#[ \t]+`)
var markdownSetextH1Pattern = regexp.MustCompile(`(?m)^[^\n]+\n=+[ \t]*(?:\n|$)`)

func newMarkdownRenderer() *markdownRenderer {
	policy := bluemonday.UGCPolicy()
	policy.AllowAttrs("type", "checked", "disabled").OnElements("input")
	policy.AllowAttrs("class").OnElements("article")
	policy.AllowAttrs("class", "role", "data-latex").OnElements("a", "div")
	policy.AllowAttrs("id").OnElements("li", "sup")
	policy.AllowAttrs("data-source-start-line", "data-source-end-line").OnElements(
		"blockquote",
		"h1", "h2", "h3", "h4", "h5", "h6",
		"hr",
		"li",
		"ol", "ul",
		"p",
		"pre",
		"table", "tbody", "thead", "tr",
	)
	return &markdownRenderer{sanitize: policy}
}

func (r *markdownRenderer) markdown() goldmark.Markdown {
	return goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Table,
			extension.TaskList,
			extension.Linkify,
			extension.Footnote,
		),
		goldmark.WithRendererOptions(
			gmhtml.WithHardWraps(),
			gmhtml.WithXHTML(),
			renderer.WithNodeRenderers(
				util.Prioritized(markdownSourceHTMLRenderer{}, 100),
			),
		),
	)
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
	preview, err := r.renderPreparedMarkdown(req.Source, req.Started, req.Settings)
	if err != nil {
		return errPreview(req.Started, "internal_error", "Failed to render Markdown", err.Error())
	}
	return preview
}

func (r *markdownRenderer) renderPreparedMarkdown(source []byte, started time.Time, settings api.Settings) (api.Preview, error) {
	body, frontMatter, bodyLineOffset := preprocessMarkdownSource(source)
	md := r.markdown()
	doc := md.Parser().Parse(text.NewReader(body))
	if root, ok := doc.(*ast.Document); ok {
		annotateMarkdownSourceLines(root, body, bodyLineOffset)
	}

	titleHTML := ""
	if frontMatter != nil && settings.MarkdownFrontMatterTitle && frontMatter.Title != "" && !markdownHasH1(doc) {
		frontMatter.TitleUsed = true
		titleHTML = "<h1>" + html.EscapeString(frontMatter.Title) + "</h1>"
	}
	var out bytes.Buffer
	if err := md.Renderer().Render(&out, body, doc); err != nil {
		return api.Preview{}, err
	}
	rewritten := rewriteMarkdownDisplayMath(out.String())
	safe := r.sanitize.SanitizeBytes([]byte(rewritten))
	return api.Preview{
		HTML:             `<article class="markdown-theme">` + titleHTML + string(safe) + `</article>`,
		FrontMatter:      frontMatter,
		SourceLineCount:  countSourceLines(source),
		UpdatedAt:        time.Now().UTC(),
		RenderDurationMS: time.Since(started).Milliseconds(),
		Status:           api.RenderStatusReady,
	}, nil
}

func preprocessMarkdownSource(source []byte) ([]byte, *api.FrontMatter, int) {
	body := source
	frontMatter, strippedBody, ok, err := parseMarkdownFrontMatter(source)
	bodyLineOffset := 0
	if err != nil {
		return source, nil, 0
	}
	if ok {
		body = strippedBody
		bodyLineOffset = countLeadingRemovedLines(source, body)
	}
	return body, frontMatter, bodyLineOffset
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

func markdownHasH1(doc ast.Node) bool {
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

func annotateMarkdownSourceLines(doc *ast.Document, source []byte, lineOffset int) {
	lineStarts := sourceLineStarts(source)
	_ = ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || node.Type() != ast.TypeBlock {
			return ast.WalkContinue, nil
		}
		start, end, ok := nodeLineRange(node, lineStarts, lineOffset)
		if !ok {
			return ast.WalkContinue, nil
		}
		node.SetAttributeString("data-source-start-line", strconv.Itoa(start))
		node.SetAttributeString("data-source-end-line", strconv.Itoa(end))
		return ast.WalkContinue, nil
	})
}

func nodeLineRange(node ast.Node, lineStarts []int, lineOffset int) (int, int, bool) {
	lines := node.Lines()
	if lines != nil && lines.Len() > 0 {
		startLine := 0
		endLine := 0
		for i := 0; i < lines.Len(); i++ {
			segment := lines.At(i)
			if segment.Stop <= segment.Start {
				continue
			}
			segStart := offsetToLine(lineStarts, segment.Start) + lineOffset
			segEnd := offsetToLine(lineStarts, segment.Stop-1) + lineOffset
			if startLine == 0 || segStart < startLine {
				startLine = segStart
			}
			if segEnd > endLine {
				endLine = segEnd
			}
		}
		if startLine != 0 && endLine != 0 {
			return startLine, endLine, true
		}
	}

	startLine := 0
	endLine := 0
	for child := node.FirstChild(); child != nil; child = child.NextSibling() {
		childStart, childEnd, ok := nodeLineRange(child, lineStarts, lineOffset)
		if !ok {
			continue
		}
		if startLine == 0 || childStart < startLine {
			startLine = childStart
		}
		if childEnd > endLine {
			endLine = childEnd
		}
	}
	if startLine == 0 || endLine == 0 {
		return 0, 0, false
	}
	return startLine, endLine, true
}

func sourceLineStarts(source []byte) []int {
	starts := []int{0}
	for idx, b := range source {
		if b == '\n' {
			starts = append(starts, idx+1)
		}
	}
	return starts
}

func offsetToLine(lineStarts []int, offset int) int {
	if offset < 0 {
		return 1
	}
	index := sort.Search(len(lineStarts), func(i int) bool {
		return lineStarts[i] > offset
	}) - 1
	if index < 0 {
		return 1
	}
	return index + 1
}

func countLeadingRemovedLines(full, body []byte) int {
	if len(body) == 0 {
		return 0
	}
	index := bytes.Index(full, body)
	if index < 0 {
		return 0
	}
	return bytes.Count(full[:index], []byte("\n"))
}

func countSourceLines(source []byte) int {
	if len(source) == 0 {
		return 0
	}
	return bytes.Count(source, []byte("\n")) + 1
}

type markdownSourceHTMLRenderer struct{}

func (markdownSourceHTMLRenderer) RegisterFuncs(reg renderer.NodeRendererFuncRegisterer) {
	reg.Register(ast.KindCodeBlock, renderMarkdownCodeBlock)
	reg.Register(ast.KindFencedCodeBlock, renderMarkdownFencedCodeBlock)
}

func renderMarkdownCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	if entering {
		_, _ = w.WriteString("<pre")
		if node.Attributes() != nil {
			gmhtml.RenderAttributes(w, node, gmhtml.GlobalAttributeFilter)
		}
		_, _ = w.WriteString("><code>")
		writeMarkdownCodeLines(w, source, node)
	} else {
		_, _ = w.WriteString("</code></pre>\n")
	}
	return ast.WalkContinue, nil
}

func renderMarkdownFencedCodeBlock(w util.BufWriter, source []byte, node ast.Node, entering bool) (ast.WalkStatus, error) {
	n := node.(*ast.FencedCodeBlock)
	if entering {
		_, _ = w.WriteString("<pre")
		if node.Attributes() != nil {
			gmhtml.RenderAttributes(w, node, gmhtml.GlobalAttributeFilter)
		}
		_, _ = w.WriteString("><code")
		if language := n.Language(source); language != nil {
			_, _ = w.WriteString(" class=\"language-")
			gmhtml.DefaultWriter.Write(w, language)
			_ = w.WriteByte('"')
		}
		_ = w.WriteByte('>')
		writeMarkdownCodeLines(w, source, node)
	} else {
		_, _ = w.WriteString("</code></pre>\n")
	}
	return ast.WalkContinue, nil
}

func writeMarkdownCodeLines(w util.BufWriter, source []byte, node ast.Node) {
	lines := node.Lines()
	if lines == nil {
		return
	}
	for i := 0; i < lines.Len(); i++ {
		segment := lines.At(i)
		gmhtml.DefaultWriter.RawWrite(w, (&segment).Value(source))
	}
}

func rewriteMarkdownDisplayMath(rendered string) string {
	return markdownParagraphPattern.ReplaceAllStringFunc(rendered, func(block string) string {
		matches := markdownParagraphPattern.FindStringSubmatch(block)
		if len(matches) != 2 {
			return block
		}

		inner := strings.TrimSpace(matches[1])
		expr, ok := extractDisplayMath(inner)
		if !ok {
			return block
		}
		return `<div class="markdown-math-block" data-latex="` + html.EscapeString(expr) + `"></div>`
	})
}

func extractDisplayMath(content string) (string, bool) {
	normalized := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(content, "<br />", "\n"), "<br>", "\n"))
	for _, pair := range [][2]string{
		{"$$", "$$"},
		{`\[`, `\]`},
	} {
		if !strings.HasPrefix(normalized, pair[0]) || !strings.HasSuffix(normalized, pair[1]) {
			continue
		}
		expr := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(normalized, pair[0]), pair[1]))
		if expr == "" {
			return "", false
		}
		return html.UnescapeString(expr), true
	}
	return "", false
}
