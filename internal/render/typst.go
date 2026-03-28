package render

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"codeberg.org/aros/dpview.git/internal/api"
	"codeberg.org/aros/dpview.git/internal/files"
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

	fingerprint := shortHash(req.Source)
	renderDir := filepath.Join(r.tempRoot, fingerprint)
	if err := os.MkdirAll(renderDir, 0o755); err != nil {
		return errPreview(req.Started, "internal_error", "Failed to create Typst render directory", err.Error())
	}

	pattern := filepath.Join(renderDir, "page-{p}.svg")
	cleanupOldSVG(renderDir)
	_, stderr, err := r.runner.Run(ctx, r.status.Details["path"], "compile", req.AbsPath, pattern)
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
		Source:           string(req.Source),
		UpdatedAt:        time.Now().UTC(),
		RenderDurationMS: time.Since(req.Started).Milliseconds(),
		Status:           api.RenderStatusReady,
	}
}

func cleanupOldSVG(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if filepath.Ext(entry.Name()) == ".svg" {
			_ = os.Remove(filepath.Join(dir, entry.Name()))
		}
	}
}
