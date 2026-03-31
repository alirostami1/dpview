package app

import (
	"bytes"
	"testing"

	"codeberg.org/aros/dpview/internal/api"
)

func TestApplyTextEdits(t *testing.T) {
	source := []byte("alpha\nbeta\ngamma\n")
	edits := []api.TextEdit{
		{Start: 6, End: 10, Text: "BETA"},
		{Start: 11, End: 11, Text: "delta\n"},
	}

	got, err := applyTextEdits(source, edits)
	if err != nil {
		t.Fatalf("applyTextEdits() error = %v", err)
	}
	want := []byte("alpha\nBETA\ndelta\ngamma\n")
	if !bytes.Equal(got, want) {
		t.Fatalf("applyTextEdits() = %q, want %q", got, want)
	}
}

func TestResolveLivePreviewSourceFallsBackToContent(t *testing.T) {
	content := []byte("next")
	got, err := resolveLivePreviewSource(nil, content, []api.TextEdit{{Start: 0, End: 0, Text: "ignored"}})
	if err != nil {
		t.Fatalf("resolveLivePreviewSource() error = %v", err)
	}
	if !bytes.Equal(got, content) {
		t.Fatalf("resolveLivePreviewSource() = %q, want %q", got, content)
	}
}
