package render

import (
	"encoding/json"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"codeberg.org/aros/dpview/internal/api"
)

const typstSeekLabel = "dpview-anchor"

type typstAnchorSpan struct {
	ID        int
	StartLine int
	EndLine   int
}

func buildTypstSeekShadow(source []byte) ([]byte, []typstAnchorSpan) {
	spans := parseTypstAnchorSpans(source)
	if len(spans) == 0 {
		return source, nil
	}

	text := string(source)
	lines := strings.Split(text, "\n")
	hadTrailingNewline := strings.HasSuffix(text, "\n")
	injections := make(map[int][]string, len(spans))
	for _, span := range spans {
		index := max(0, span.StartLine-1)
		injections[index] = append(injections[index], formatTypstSeekAnchor(span))
	}

	var out []string
	for i, line := range lines {
		if extra := injections[i]; len(extra) > 0 {
			out = append(out, extra...)
		}
		out = append(out, line)
	}

	shadow := strings.Join(out, "\n")
	if hadTrailingNewline && !strings.HasSuffix(shadow, "\n") {
		shadow += "\n"
	}
	return []byte(shadow), spans
}

func parseTypstAnchorSpans(source []byte) []typstAnchorSpan {
	lines := strings.Split(string(source), "\n")
	spans := make([]typstAnchorSpan, 0)
	nextID := 1

	addSpan := func(start, end int) {
		if start <= 0 || end < start {
			return
		}
		if len(spans) > 0 {
			last := spans[len(spans)-1]
			if last.StartLine == start && last.EndLine == end {
				return
			}
		}
		spans = append(spans, typstAnchorSpan{
			ID:        nextID,
			StartLine: start,
			EndLine:   end,
		})
		nextID++
	}

	for i := 0; i < len(lines); {
		lineNo := i + 1
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" {
			i++
			continue
		}

		if strings.HasPrefix(trimmed, "```") {
			start := lineNo
			i++
			for i < len(lines) && !strings.HasPrefix(strings.TrimSpace(lines[i]), "```") {
				i++
			}
			if i < len(lines) {
				i++
			}
			addSpan(start, i)
			continue
		}

		if trimmed == "$" {
			start := lineNo
			i++
			for i < len(lines) && strings.TrimSpace(lines[i]) != "$" {
				i++
			}
			if i < len(lines) {
				i++
			}
			addSpan(start, i)
			continue
		}

		if isStandaloneTypstBlockStart(trimmed) {
			start := lineNo
			i++
			for i < len(lines) {
				next := strings.TrimSpace(lines[i])
				if next == "" || isStandaloneTypstBlockStart(next) || strings.HasPrefix(next, "```") || next == "$" {
					break
				}
				i++
			}
			addSpan(start, i)
			continue
		}

		start := lineNo
		i++
		for i < len(lines) {
			next := strings.TrimSpace(lines[i])
			if next == "" || isStandaloneTypstBlockStart(next) || strings.HasPrefix(next, "```") || next == "$" {
				break
			}
			i++
		}
		addSpan(start, i)
	}

	return spans
}

func isStandaloneTypstBlockStart(trimmed string) bool {
	if trimmed == "" {
		return false
	}
	if strings.HasPrefix(trimmed, "=") || strings.HasPrefix(trimmed, "#") || strings.HasPrefix(trimmed, ">") {
		return true
	}
	if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "+ ") {
		return true
	}
	digitRun := 0
	for digitRun < len(trimmed) && trimmed[digitRun] >= '0' && trimmed[digitRun] <= '9' {
		digitRun++
	}
	return digitRun > 0 && digitRun+1 < len(trimmed) && trimmed[digitRun] == '.' && trimmed[digitRun+1] == ' '
}

func formatTypstSeekAnchor(anchor typstAnchorSpan) string {
	return fmt.Sprintf(
		`#context [#let pos = here().position(); #metadata((id: %d, start_line: %d, end_line: %d, page: pos.page, x: pos.x, y: pos.y)) <%s>]`,
		anchor.ID,
		anchor.StartLine,
		anchor.EndLine,
		typstSeekLabel,
	)
}

func parseTypstSeekQueryOutput(raw []byte) []api.TypstSeekAnchor {
	var payload []map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}

	anchors := make([]api.TypstSeekAnchor, 0, len(payload))
	for _, item := range payload {
		start, okStart := parseTypstSeekNumber(item["start_line"])
		end, okEnd := parseTypstSeekNumber(item["end_line"])
		page, okPage := parseTypstSeekNumber(item["page"])
		x, okX := parseTypstSeekNumber(item["x"])
		y, okY := parseTypstSeekNumber(item["y"])
		if !okStart || !okEnd || !okPage || !okX || !okY {
			continue
		}
		anchors = append(anchors, api.TypstSeekAnchor{
			StartLine: int(start),
			EndLine:   int(end),
			Page:      int(page),
			X:         x,
			Y:         y,
		})
	}

	slices.SortFunc(anchors, func(a, b api.TypstSeekAnchor) int {
		if a.StartLine != b.StartLine {
			return a.StartLine - b.StartLine
		}
		if a.Page != b.Page {
			return a.Page - b.Page
		}
		switch {
		case a.Y < b.Y:
			return -1
		case a.Y > b.Y:
			return 1
		default:
			return 0
		}
	})
	return anchors
}

func parseTypstSeekNumber(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case json.Number:
		number, err := typed.Float64()
		return number, err == nil
	case string:
		trimmed := strings.TrimSpace(strings.TrimSuffix(typed, "pt"))
		number, err := strconv.ParseFloat(trimmed, 64)
		return number, err == nil
	default:
		return 0, false
	}
}
