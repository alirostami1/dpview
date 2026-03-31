package main

import (
	"errors"
	"io/fs"
	"testing"
	"testing/fstest"
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
		"index.html":          &fstest.MapFile{Data: []byte("index")},
		"styles.css":          &fstest.MapFile{Data: []byte("style")},
		"themes/site.css":     &fstest.MapFile{Data: []byte("theme")},
		"app.ts":              &fstest.MapFile{Data: []byte("source")},
		"generated/types.ts":  &fstest.MapFile{Data: []byte("types")},
		"katex-style.css":     &fstest.MapFile{Data: []byte("@import")},
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
