package render

import (
	"container/list"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"sync"
	"time"

	"codeberg.org/aros/dpview/internal/api"
	"codeberg.org/aros/dpview/internal/files"
)

type Renderer interface {
	Render(ctx context.Context, info files.FileInfo, absPath string) api.Preview
	Limits() api.Limits
	RendererStatuses() []api.RendererStatus
	RendererStatus(kind files.Kind) (api.RendererStatus, bool)
}

type Config struct {
	Root          string
	TypstBinary   string
	MaxFileSize   int64
	RenderTimeout time.Duration
}

type RenderRequest struct {
	Root      string
	Info      files.FileInfo
	AbsPath   string
	Source    []byte
	Transient bool
	Started   time.Time
	Limits    api.Limits
	Settings  api.Settings
}

type DocumentRenderer interface {
	Kind() files.Kind
	Render(ctx context.Context, req RenderRequest) api.Preview
}

type statusProvider interface {
	Status() api.RendererStatus
}

type runnerSetter interface {
	SetRunner(CommandRunner)
}

type Service struct {
	limits    api.Limits
	root      string
	renderers map[files.Kind]DocumentRenderer
	closers   []io.Closer

	mu        sync.RWMutex
	cache     map[string]*list.Element
	cacheList *list.List
}

type cacheEntry struct {
	key     string
	preview api.Preview
}

const maxCacheEntries = 128

func NewService(cfg Config) (*Service, error) {
	svc := &Service{
		limits: api.Limits{
			MaxFileSizeBytes: cfg.MaxFileSize,
			RenderTimeoutMS:  cfg.RenderTimeout.Milliseconds(),
		},
		root:      cfg.Root,
		renderers: make(map[files.Kind]DocumentRenderer),
		cache:     make(map[string]*list.Element),
		cacheList: list.New(),
	}

	svc.Register(newMarkdownRenderer())

	typstRenderer, err := newTypstRenderer(cfg.TypstBinary)
	if err != nil {
		return nil, err
	}
	svc.Register(typstRenderer)

	return svc, nil
}

func (s *Service) Register(renderer DocumentRenderer) {
	s.renderers[renderer.Kind()] = renderer
	if closer, ok := renderer.(io.Closer); ok {
		s.closers = append(s.closers, closer)
	}
}

func (s *Service) Close() error {
	var firstErr error
	for _, closer := range s.closers {
		if err := closer.Close(); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}

func (s *Service) Limits() api.Limits {
	return s.limits
}

func (s *Service) RendererStatuses() []api.RendererStatus {
	statuses := make([]api.RendererStatus, 0, len(s.renderers))
	for _, renderer := range s.renderers {
		if provider, ok := renderer.(statusProvider); ok {
			statuses = append(statuses, provider.Status())
		}
	}
	slices.SortFunc(statuses, func(a, b api.RendererStatus) int {
		return strings.Compare(string(a.Kind), string(b.Kind))
	})
	return statuses
}

func (s *Service) RendererStatus(kind files.Kind) (api.RendererStatus, bool) {
	renderer, ok := s.renderers[kind]
	if !ok {
		return api.RendererStatus{}, false
	}
	provider, ok := renderer.(statusProvider)
	if !ok {
		return api.RendererStatus{}, false
	}
	return provider.Status(), true
}

func (s *Service) SetRunner(runner CommandRunner) {
	for _, renderer := range s.renderers {
		if setter, ok := renderer.(runnerSetter); ok {
			setter.SetRunner(runner)
		}
	}
}

func (s *Service) Render(ctx context.Context, info files.FileInfo, absPath string, settings api.Settings) api.Preview {
	source, err := os.ReadFile(absPath)
	if err != nil {
		return errPreview(time.Now().UTC(), "internal_error", "Failed to read source file", err.Error())
	}
	return s.RenderSource(ctx, info, absPath, source, settings, false)
}

func (s *Service) RenderSource(
	ctx context.Context,
	info files.FileInfo,
	absPath string,
	source []byte,
	settings api.Settings,
	transient bool,
) api.Preview {
	start := time.Now().UTC()
	renderCtx, cancel := context.WithTimeout(ctx, time.Duration(s.limits.RenderTimeoutMS)*time.Millisecond)
	defer cancel()

	if int64(len(source)) > s.limits.MaxFileSizeBytes {
		return errPreview(start, "file_too_large", "File exceeds configured preview limit", fmt.Sprintf("%d > %d bytes", len(source), s.limits.MaxFileSizeBytes))
	}

	cacheKey := renderCacheKey(info, source, settings, transient)
	if cached, ok := s.loadCache(cacheKey); ok {
		cached.CacheHit = true
		return cached
	}

	renderer, ok := s.renderers[info.Kind]
	if !ok {
		return errPreview(start, "unsupported_file_type", "Unsupported file type", string(info.Kind))
	}

	preview := renderer.Render(renderCtx, RenderRequest{
		Root:      s.root,
		Info:      info,
		AbsPath:   absPath,
		Source:    source,
		Transient: transient,
		Started:   start,
		Limits:    s.limits,
		Settings:  settings,
	})
	if preview.Error == nil {
		s.storeCache(cacheKey, preview)
	}
	return preview
}

func (s *Service) loadCache(key string) (api.Preview, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	entry, ok := s.cache[key]
	if !ok {
		return api.Preview{}, false
	}
	s.cacheList.MoveToFront(entry)
	return entry.Value.(cacheEntry).preview, true
}

func (s *Service) storeCache(key string, preview api.Preview) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cache == nil {
		s.cache = make(map[string]*list.Element)
	}
	if s.cacheList == nil {
		s.cacheList = list.New()
	}
	if existing, ok := s.cache[key]; ok {
		preview.CacheHit = false
		existing.Value = cacheEntry{key: key, preview: preview}
		s.cacheList.MoveToFront(existing)
		return
	}
	preview.CacheHit = false
	elem := s.cacheList.PushFront(cacheEntry{key: key, preview: preview})
	s.cache[key] = elem
	if s.cacheList.Len() <= maxCacheEntries {
		return
	}
	tail := s.cacheList.Back()
	if tail == nil {
		return
	}
	evicted := tail.Value.(cacheEntry)
	delete(s.cache, evicted.key)
	s.cacheList.Remove(tail)
}

func renderCacheKey(info files.FileInfo, source []byte, settings api.Settings, transient bool) string {
	return strings.Join([]string{
		info.Path,
		info.ModTime.UTC().Format(time.RFC3339Nano),
		shortHash(source),
		fmt.Sprintf("%t", transient),
		fmt.Sprintf("%t", settings.LiveBufferPreviewEnabled),
		fmt.Sprintf("%t", settings.TypstPreviewTheme),
		fmt.Sprintf("%t", settings.MarkdownFrontMatterTitle),
		settings.Theme,
		settings.PreviewTheme,
	}, ":")
}

func shortHash(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:12])
}

func errPreview(start time.Time, code, message, detail string) api.Preview {
	return api.Preview{
		SourceLineCount:  0,
		UpdatedAt:        time.Now().UTC(),
		RenderDurationMS: time.Since(start).Milliseconds(),
		Status:           api.RenderStatusError,
		Error:            api.NewError(code, message, detail),
	}
}
