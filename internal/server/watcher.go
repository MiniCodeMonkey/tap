package server

import (
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// Watcher watches files and directories for changes and triggers callbacks.
type Watcher struct {
	// Fields ordered by size for better memory alignment
	watcher      *fsnotify.Watcher
	onChange     func(path string)
	stopCh       chan struct{}
	doneCh       chan struct{}
	mdFile       string
	mdDir        string
	mu           sync.Mutex
	debounceTime time.Duration
	running      bool
}

// NewWatcher creates a new file watcher.
// mdFile is the main markdown file to watch.
// The watcher also watches the directory containing the markdown file for asset changes.
func NewWatcher(mdFile string) (*Watcher, error) {
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	// Resolve absolute paths
	absFile, err := filepath.Abs(mdFile)
	if err != nil {
		fsWatcher.Close()
		return nil, err
	}
	mdDir := filepath.Dir(absFile)

	w := &Watcher{
		watcher:      fsWatcher,
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
		debounceTime: 100 * time.Millisecond,
		mdFile:       absFile,
		mdDir:        mdDir,
	}

	return w, nil
}

// SetOnChange sets the callback function called when a file changes.
// The callback receives the path of the changed file.
func (w *Watcher) SetOnChange(fn func(path string)) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.onChange = fn
}

// SetDebounceTime sets the debounce window for rapid changes.
// Default is 100ms.
func (w *Watcher) SetDebounceTime(d time.Duration) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.debounceTime = d
}

// Start starts watching for file changes.
// The watcher runs in a goroutine and can be stopped with Stop().
func (w *Watcher) Start() error {
	w.mu.Lock()
	if w.running {
		w.mu.Unlock()
		return nil
	}
	w.running = true
	w.mu.Unlock()

	// Add the markdown file to the watcher
	if err := w.watcher.Add(w.mdFile); err != nil {
		w.mu.Lock()
		w.running = false
		w.mu.Unlock()
		return err
	}

	// Add the directory for asset changes (not fatal if it fails)
	_ = w.watcher.Add(w.mdDir)

	go w.run()
	return nil
}

// Stop stops the watcher and waits for it to finish.
func (w *Watcher) Stop() error {
	w.mu.Lock()
	if !w.running {
		w.mu.Unlock()
		return nil
	}
	w.mu.Unlock()

	// Signal stop
	close(w.stopCh)

	// Wait for the run loop to finish
	<-w.doneCh

	// Close the underlying watcher
	return w.watcher.Close()
}

// IsRunning returns whether the watcher is currently running.
func (w *Watcher) IsRunning() bool {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.running
}

// run is the main watch loop that handles events with debouncing.
func (w *Watcher) run() {
	defer close(w.doneCh)

	var (
		debounceTimer *time.Timer
		pendingPath   string
	)

	for {
		select {
		case <-w.stopCh:
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			w.mu.Lock()
			w.running = false
			w.mu.Unlock()
			return

		case event, ok := <-w.watcher.Events:
			if !ok {
				w.mu.Lock()
				w.running = false
				w.mu.Unlock()
				return
			}

			// Handle file rename/move - re-add the markdown file if it was renamed
			if event.Name == w.mdFile && (event.Has(fsnotify.Rename) || event.Has(fsnotify.Remove)) {
				// Try to re-add the file after a short delay (file might be recreated)
				go w.readdFile()
			}

			// Only trigger on Write, Create, or Remove operations
			if !event.Has(fsnotify.Write) && !event.Has(fsnotify.Create) && !event.Has(fsnotify.Remove) {
				continue
			}

			// Debounce: reset timer on each event
			w.mu.Lock()
			debounceTime := w.debounceTime
			w.mu.Unlock()

			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			pendingPath = event.Name
			debounceTimer = time.AfterFunc(debounceTime, func() {
				w.triggerOnChange(pendingPath)
			})

		case err, ok := <-w.watcher.Errors:
			if !ok {
				w.mu.Lock()
				w.running = false
				w.mu.Unlock()
				return
			}
			// Log error but continue watching
			_ = err
		}
	}
}

// readdFile attempts to re-add the markdown file to the watcher after a rename/move.
// This handles the case where an editor saves by writing to a temp file then renaming.
func (w *Watcher) readdFile() {
	// Wait a bit for the file to be recreated
	time.Sleep(50 * time.Millisecond)

	// Check if the file exists now
	if _, err := os.Stat(w.mdFile); err == nil {
		// File exists, try to re-add it
		_ = w.watcher.Add(w.mdFile)
	}
}

// triggerOnChange safely calls the onChange callback if set.
func (w *Watcher) triggerOnChange(path string) {
	w.mu.Lock()
	fn := w.onChange
	w.mu.Unlock()

	if fn != nil {
		fn(path)
	}
}

// WatchedFile returns the path of the main markdown file being watched.
func (w *Watcher) WatchedFile() string {
	return w.mdFile
}

// WatchedDir returns the path of the directory being watched for assets.
func (w *Watcher) WatchedDir() string {
	return w.mdDir
}
