package files

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

var supportedExts = map[string]string{
	".md":       "markdown",
	".markdown": "markdown",
	".typ":      "typst",
	".typst":    "typst",
}

type Service struct {
	root string
}

var (
	ErrPathRequired        = errors.New("path is required")
	ErrAbsolutePath        = errors.New("absolute paths are not allowed")
	ErrPathTraversal       = errors.New("path traversal is not allowed")
	ErrPathOutsideRoot     = errors.New("path escapes configured root")
	ErrPathIsDirectory     = errors.New("path is a directory")
	ErrUnsupportedFileType = errors.New("unsupported file type")
)

type Kind string

const (
	KindMarkdown Kind = "markdown"
	KindTypst    Kind = "typst"
)

type FileInfo struct {
	Path    string    `json:"path"`
	Name    string    `json:"name"`
	Kind    Kind      `json:"kind"`
	Size    int64     `json:"size"`
	Ext     string    `json:"ext"`
	ModTime time.Time `json:"mtime"`
}

type TreeNode struct {
	Name     string     `json:"name"`
	Path     string     `json:"path,omitempty"`
	Kind     Kind       `json:"kind,omitempty"`
	Children []TreeNode `json:"children,omitempty"`
}

func NewService(root string) (*Service, error) {
	info, err := os.Stat(root)
	if err != nil {
		return nil, fmt.Errorf("stat root: %w", err)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("root is not a directory: %s", root)
	}
	return &Service{root: filepath.Clean(root)}, nil
}

func (s *Service) Root() string {
	return s.root
}

func (s *Service) IsPreviewable(name string) (Kind, bool) {
	ext := strings.ToLower(filepath.Ext(name))
	kind, ok := supportedExts[ext]
	if !ok {
		return "", false
	}
	return Kind(kind), true
}

func (s *Service) List() ([]FileInfo, error) {
	items := make([]FileInfo, 0)
	err := filepath.WalkDir(s.root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		kind, ok := s.IsPreviewable(d.Name())
		if !ok {
			return nil
		}

		rel, err := filepath.Rel(s.root, p)
		if err != nil {
			return err
		}
		stat, err := d.Info()
		if err != nil {
			return err
		}
		items = append(items, FileInfo{
			Path:    filepath.ToSlash(rel),
			Name:    d.Name(),
			Kind:    kind,
			Size:    stat.Size(),
			Ext:     strings.ToLower(filepath.Ext(d.Name())),
			ModTime: stat.ModTime().UTC(),
		})
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("scan files: %w", err)
	}

	slices.SortFunc(items, func(a, b FileInfo) int {
		return strings.Compare(a.Path, b.Path)
	})
	return items, nil
}

func BuildTree(items []FileInfo) []TreeNode {
	root := &treeNode{Name: "", Children: make(map[string]*treeNode)}
	for _, item := range items {
		parts := strings.Split(item.Path, "/")
		cursor := root
		prefix := ""
		for idx, part := range parts {
			if prefix == "" {
				prefix = part
			} else {
				prefix += "/" + part
			}
			child, ok := cursor.Children[part]
			if !ok {
				child = &treeNode{
					Name:     part,
					Path:     prefix,
					Children: make(map[string]*treeNode),
				}
				cursor.Children[part] = child
			}
			if idx == len(parts)-1 {
				child.Path = item.Path
				child.Kind = item.Kind
			}
			cursor = child
		}
	}
	return collapseTree(root.Children)
}

type treeNode struct {
	Name     string
	Path     string
	Kind     Kind
	Children map[string]*treeNode
}

func collapseTree(nodes map[string]*treeNode) []TreeNode {
	out := make([]TreeNode, 0, len(nodes))
	for _, node := range nodes {
		converted := TreeNode{
			Name: node.Name,
			Path: node.Path,
			Kind: node.Kind,
		}
		if len(node.Children) > 0 {
			converted.Children = collapseTree(node.Children)
		}
		out = append(out, converted)
	}
	slices.SortFunc(out, func(a, b TreeNode) int {
		aDir := len(a.Children) > 0
		bDir := len(b.Children) > 0
		if aDir != bDir {
			if aDir {
				return -1
			}
			return 1
		}
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})
	return out
}

func (s *Service) Resolve(rel string) (string, FileInfo, error) {
	clean, err := cleanRelative(rel)
	if err != nil {
		return "", FileInfo{}, err
	}
	abs := filepath.Join(s.root, filepath.FromSlash(clean))
	abs, err = filepath.Abs(abs)
	if err != nil {
		return "", FileInfo{}, fmt.Errorf("resolve path: %w", err)
	}
	if !isWithinRoot(s.root, abs) {
		return "", FileInfo{}, ErrPathOutsideRoot
	}
	info, err := os.Stat(abs)
	if err != nil {
		return "", FileInfo{}, err
	}
	if info.IsDir() {
		return "", FileInfo{}, ErrPathIsDirectory
	}
	kind, ok := s.IsPreviewable(info.Name())
	if !ok {
		return "", FileInfo{}, ErrUnsupportedFileType
	}
	return abs, FileInfo{
		Path:    clean,
		Name:    info.Name(),
		Kind:    kind,
		Size:    info.Size(),
		Ext:     strings.ToLower(filepath.Ext(info.Name())),
		ModTime: info.ModTime().UTC(),
	}, nil
}

func cleanRelative(rel string) (string, error) {
	if rel == "" {
		return "", ErrPathRequired
	}
	if filepath.IsAbs(rel) || strings.HasPrefix(rel, "/") {
		return "", ErrAbsolutePath
	}
	clean := path.Clean(filepath.ToSlash(rel))
	if clean == "." || clean == "" {
		return "", ErrPathRequired
	}
	if strings.HasPrefix(clean, "../") || clean == ".." {
		return "", ErrPathTraversal
	}
	return clean, nil
}

func isWithinRoot(root, candidate string) bool {
	rel, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
