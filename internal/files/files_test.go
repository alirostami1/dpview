package files

import (
	"os"
	"path/filepath"
	"testing"
)

func TestListResolveAndTree(t *testing.T) {
	root := t.TempDir()
	mustWriteFile(t, filepath.Join(root, "notes", "todo.md"), "# todo")
	mustWriteFile(t, filepath.Join(root, "notes", "draft.typ"), "#set page(width: 100pt)")
	mustWriteFile(t, filepath.Join(root, "skip.txt"), "ignore")
	mustWriteFile(t, filepath.Join(root, "unicode", "spaced name ü.md"), "hello")

	svc, err := NewService(root)
	if err != nil {
		t.Fatalf("NewService() error = %v", err)
	}

	items, err := svc.List()
	if err != nil {
		t.Fatalf("List() error = %v", err)
	}
	if len(items) != 3 {
		t.Fatalf("List() len = %d, want 3", len(items))
	}

	abs, info, err := svc.Resolve("unicode/spaced name ü.md")
	if err != nil {
		t.Fatalf("Resolve() error = %v", err)
	}
	if filepath.Base(abs) != "spaced name ü.md" {
		t.Fatalf("Resolve() absolute path = %q", abs)
	}
	if info.Kind != KindMarkdown {
		t.Fatalf("Resolve() kind = %q, want %q", info.Kind, KindMarkdown)
	}

	if _, _, err := svc.Resolve("../etc/passwd"); err == nil {
		t.Fatal("Resolve() expected traversal error")
	}

	tree := BuildTree(items)
	if len(tree) != 2 {
		t.Fatalf("BuildTree() top-level nodes = %d, want 2", len(tree))
	}
	if tree[0].Name != "notes" {
		t.Fatalf("BuildTree() first node = %q, want notes", tree[0].Name)
	}
	if len(tree[0].Children) != 2 {
		t.Fatalf("BuildTree() notes children = %d, want 2", len(tree[0].Children))
	}
}

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}
