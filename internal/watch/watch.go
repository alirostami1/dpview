package watch

import (
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

type Op string

const (
	OpCreate Op = "create"
	OpWrite  Op = "write"
	OpRemove Op = "remove"
	OpRename Op = "rename"
)

type Event struct {
	Path  string
	Op    Op
	IsDir bool
}

type Watcher struct {
	fs       *fsnotify.Watcher
	root     string
	debounce time.Duration
	onChange func([]Event)
	done     chan struct{}
	once     sync.Once
}

func New(root string, debounce time.Duration, onChange func([]Event)) (*Watcher, error) {
	fs, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	w := &Watcher{
		fs:       fs,
		root:     root,
		debounce: debounce,
		onChange: onChange,
		done:     make(chan struct{}),
	}
	if err := w.addRecursive(root); err != nil {
		fs.Close()
		return nil, err
	}
	go w.loop()
	return w, nil
}

func (w *Watcher) Close() error {
	var err error
	w.once.Do(func() {
		close(w.done)
		err = w.fs.Close()
	})
	return err
}

func (w *Watcher) loop() {
	var (
		timer   *time.Timer
		pending []Event
		mu      sync.Mutex
	)
	flush := func() {
		mu.Lock()
		events := append([]Event(nil), pending...)
		pending = pending[:0]
		mu.Unlock()
		if len(events) > 0 {
			w.onChange(events)
		}
	}
	queue := func(event Event) {
		mu.Lock()
		pending = append(pending, event)
		mu.Unlock()
		if timer != nil {
			timer.Stop()
		}
		timer = time.AfterFunc(w.debounce, flush)
	}
	for {
		select {
		case <-w.done:
			if timer != nil {
				timer.Stop()
			}
			return
		case event, ok := <-w.fs.Events:
			if !ok {
				return
			}
			for _, mapped := range mapEvent(event) {
				if mapped.Op == OpCreate && mapped.IsDir {
					if err := w.addRecursive(mapped.Path); err != nil {
						log.Printf("watch add: %v", err)
					}
				}
				queue(mapped)
			}
		case err, ok := <-w.fs.Errors:
			if !ok {
				return
			}
			log.Printf("watch error: %v", err)
		}
	}
}

func (w *Watcher) addRecursive(root string) error {
	return filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			return nil
		}
		return w.fs.Add(path)
	})
}

func mapEvent(event fsnotify.Event) []Event {
	out := make([]Event, 0, 4)
	info, err := os.Stat(event.Name)
	isDir := err == nil && info.IsDir()
	if event.Op&fsnotify.Create != 0 {
		out = append(out, Event{Path: event.Name, Op: OpCreate, IsDir: isDir})
	}
	if event.Op&fsnotify.Write != 0 {
		out = append(out, Event{Path: event.Name, Op: OpWrite, IsDir: isDir})
	}
	if event.Op&fsnotify.Remove != 0 {
		out = append(out, Event{Path: event.Name, Op: OpRemove, IsDir: isDir})
	}
	if event.Op&fsnotify.Rename != 0 {
		out = append(out, Event{Path: event.Name, Op: OpRename, IsDir: isDir})
	}
	return out
}
