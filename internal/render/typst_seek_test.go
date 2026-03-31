package render

import (
	"strings"
	"testing"
)

func TestParseTypstAnchorSpans(t *testing.T) {
	source := []byte(strings.Join([]string{
		"= Heading",
		"",
		"Paragraph line one",
		"Paragraph line two",
		"",
		"- item one",
		"- item two",
		"",
		"```typ",
		"#let x = 1",
		"```",
		"",
		"$",
		"x + y",
		"$",
		"",
		"#pagebreak()",
		"#figure(rect())",
	}, "\n"))

	spans := parseTypstAnchorSpans(source)
	checks := []struct {
		start int
		end   int
	}{
		{1, 1},
		{3, 4},
		{6, 6},
		{7, 7},
		{9, 11},
		{13, 15},
		{17, 17},
		{18, 18},
	}
	for _, check := range checks {
		found := false
		for _, span := range spans {
			if span.StartLine == check.start && span.EndLine == check.end {
				found = true
				break
			}
		}
		if !found {
			t.Fatalf("missing span %+v in %+v", check, spans)
		}
	}
}

func TestBuildTypstSeekShadowInjectsMetadataAnchors(t *testing.T) {
	source := []byte("= Heading\n\nParagraph\n")
	shadow, spans := buildTypstSeekShadow(source)
	if len(spans) != 2 {
		t.Fatalf("buildTypstSeekShadow() spans = %+v", spans)
	}
	text := string(shadow)
	if strings.Count(text, "<"+typstSeekLabel+">") != 2 {
		t.Fatalf("shadow missing metadata anchors: %q", text)
	}
	if !strings.Contains(text, "start_line: 1") || !strings.Contains(text, "start_line: 3") {
		t.Fatalf("shadow missing line metadata: %q", text)
	}
}

func TestParseTypstSeekQueryOutput(t *testing.T) {
	raw := []byte(`[
		{"start_line":3,"end_line":4,"page":1,"x":"12pt","y":"40pt"},
		{"start_line":1,"end_line":1,"page":1,"x":0,"y":"10pt"}
	]`)
	anchors := parseTypstSeekQueryOutput(raw)
	if len(anchors) != 2 {
		t.Fatalf("parseTypstSeekQueryOutput() len = %d", len(anchors))
	}
	if anchors[0].StartLine != 1 || anchors[1].StartLine != 3 {
		t.Fatalf("anchors not sorted by source line: %+v", anchors)
	}
	if anchors[1].Y != 40 {
		t.Fatalf("anchor y = %v", anchors[1].Y)
	}
}
