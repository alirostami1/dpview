package main

import (
	"context"
	"errors"
	"io/fs"
	"net/http"
	"testing"
	"testing/fstest"
	"time"
)

func TestVersionStringVersionOnly(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	t.Cleanup(func() {
		version = oldVersion
		commit = oldCommit
		date = oldDate
	})

	version = "v1.5.0"
	commit = "unknown"
	date = "unknown"

	if got := versionString(); got != "dpview v1.5.0" {
		t.Fatalf("versionString() = %q", got)
	}
}

func TestVersionStringWithMetadata(t *testing.T) {
	oldVersion, oldCommit, oldDate := version, commit, date
	t.Cleanup(func() {
		version = oldVersion
		commit = oldCommit
		date = oldDate
	})

	version = "v1.5.0"
	commit = "abc1234"
	date = "2026-03-29T00:00:00Z"

	if got := versionString(); got != "dpview v1.5.0 (abc1234, 2026-03-29T00:00:00Z)" {
		t.Fatalf("versionString() = %q", got)
	}
}

func TestLayeredFSBuildAssetsOverrideSourceAssets(t *testing.T) {
	build := fstest.MapFS{
		"app.js": &fstest.MapFile{Data: []byte("build-app")},
	}
	source := fstest.MapFS{
		"app.js":     &fstest.MapFile{Data: []byte("source-app")},
		"index.html": &fstest.MapFile{Data: []byte("index")},
	}

	merged := layeredFS{layers: []fs.FS{build, source}}

	appJS, err := fs.ReadFile(merged, "app.js")
	if err != nil {
		t.Fatalf("ReadFile(app.js) error = %v", err)
	}
	if string(appJS) != "build-app" {
		t.Fatalf("ReadFile(app.js) = %q", appJS)
	}

	indexHTML, err := fs.ReadFile(merged, "index.html")
	if err != nil {
		t.Fatalf("ReadFile(index.html) error = %v", err)
	}
	if string(indexHTML) != "index" {
		t.Fatalf("ReadFile(index.html) = %q", indexHTML)
	}
}

func TestSourceAssetFSOnlyServesPublicShellAssets(t *testing.T) {
	source := sourceAssetFS{base: fstest.MapFS{
		"index.html":         &fstest.MapFile{Data: []byte("index")},
		"styles.css":         &fstest.MapFile{Data: []byte("style")},
		"themes/site.css":    &fstest.MapFile{Data: []byte("theme")},
		"app.ts":             &fstest.MapFile{Data: []byte("source")},
		"generated/types.ts": &fstest.MapFile{Data: []byte("types")},
		"katex-style.css":    &fstest.MapFile{Data: []byte("@import")},
	}}

	if _, err := fs.ReadFile(source, "index.html"); err != nil {
		t.Fatalf("ReadFile(index.html) error = %v", err)
	}
	if _, err := fs.ReadFile(source, "styles.css"); err != nil {
		t.Fatalf("ReadFile(styles.css) error = %v", err)
	}
	if _, err := fs.ReadFile(source, "themes/site.css"); err != nil {
		t.Fatalf("ReadFile(themes/site.css) error = %v", err)
	}
	if _, err := fs.ReadFile(source, "app.ts"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReadFile(app.ts) error = %v, want fs.ErrNotExist", err)
	}
	if _, err := fs.ReadFile(source, "generated/types.ts"); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("ReadFile(generated/types.ts) error = %v, want fs.ErrNotExist", err)
	}
}

func TestNewHTTPServerSetsTimeouts(t *testing.T) {
	handler := http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})
	server := newHTTPServer("127.0.0.1:8080", handler)

	if server.Addr != "127.0.0.1:8080" {
		t.Fatalf("Addr = %q", server.Addr)
	}
	if server.Handler == nil {
		t.Fatal("Handler was not assigned")
	}
	if server.ReadHeaderTimeout != serverReadHeaderTimeout {
		t.Fatalf("ReadHeaderTimeout = %v", server.ReadHeaderTimeout)
	}
	if server.ReadTimeout != serverReadTimeout {
		t.Fatalf("ReadTimeout = %v", server.ReadTimeout)
	}
	if server.WriteTimeout != serverWriteTimeout {
		t.Fatalf("WriteTimeout = %v", server.WriteTimeout)
	}
	if server.IdleTimeout != serverIdleTimeout {
		t.Fatalf("IdleTimeout = %v", server.IdleTimeout)
	}
}

func TestServeWithGracefulShutdownCancelsCleanly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	server := &fakeGracefulServer{listenResult: make(chan error, 1)}

	done := make(chan error, 1)
	go func() {
		done <- serveWithGracefulShutdown(ctx, server)
	}()

	cancel()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("serveWithGracefulShutdown() error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("serveWithGracefulShutdown() did not return")
	}

	if server.shutdownCalls != 1 {
		t.Fatalf("Shutdown calls = %d", server.shutdownCalls)
	}
	if server.shutdownCtx == nil {
		t.Fatal("Shutdown context was not passed")
	}
	if _, ok := server.shutdownCtx.Deadline(); !ok {
		t.Fatal("Shutdown context did not include a deadline")
	}
}

func TestServeWithGracefulShutdownReturnsServeError(t *testing.T) {
	wantErr := errors.New("boom")
	server := &fakeGracefulServer{listenResult: make(chan error, 1)}
	server.listenResult <- wantErr

	err := serveWithGracefulShutdown(context.Background(), server)
	if !errors.Is(err, wantErr) {
		t.Fatalf("serveWithGracefulShutdown() error = %v, want %v", err, wantErr)
	}
	if server.shutdownCalls != 0 {
		t.Fatalf("Shutdown calls = %d, want 0", server.shutdownCalls)
	}
}

type fakeGracefulServer struct {
	listenResult  chan error
	shutdownErr   error
	shutdownCalls int
	shutdownCtx   context.Context
}

func (f *fakeGracefulServer) ListenAndServe() error {
	return <-f.listenResult
}

func (f *fakeGracefulServer) Shutdown(ctx context.Context) error {
	f.shutdownCalls++
	f.shutdownCtx = ctx
	if f.shutdownErr != nil {
		return f.shutdownErr
	}
	select {
	case f.listenResult <- http.ErrServerClosed:
	default:
	}
	return nil
}
