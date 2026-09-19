package server

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
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
	// callbackMu serializes onChange invocations, so a rebuild slower than
	// the debounce window can never run concurrently with the next one and
	// have its (now stale) result land after it; see triggerOnChange.
	callbackMu   sync.Mutex
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

	// Add the deck directory tree for asset and component changes (not
	// fatal if it fails): a component file, or a file it imports, can live
	// in any subfolder of the deck directory, not just next to the
	// markdown file. node_modules, VCS/dotfile directories, and the build
	// output directory are skipped within the tree (see shouldSkipDir),
	// but never for the deck directory itself - see addTree's isRoot
	// parameter: a deck whose own folder happens to be named "dist" or to
	// start with "." (e.g. "~/talks/.drafts") must still be watched.
	_ = w.addTree(w.mdDir, true)

	go w.run()
	return nil
}

// shouldSkipDir reports whether a directory should never be watched:
// node_modules (can be enormous, never relevant), any directory whose name
// starts with "." such as .git (version control internals, editor
// swapfiles), and a directory literally named "dist" (tap build's default
// output directory).
func (w *Watcher) shouldSkipDir(path string, name string) bool {
	return name == "node_modules" || name == "dist" || strings.HasPrefix(name, ".")
}

// addTree adds root and every subdirectory under it (skipping directories
// shouldSkipDir rejects) to the underlying fsnotify watcher, since fsnotify
// does not watch subdirectories on its own. isRoot must be true only when
// root is the deck directory passed from Start: filepath.WalkDir stops the
// entire walk the moment its callback returns SkipDir for the root path
// itself, so applying shouldSkipDir to the deck directory's own name would
// silently leave a deck folder named "dist", or one starting with ".",
// completely unwatched. isRoot must be false for a directory discovered
// while the watcher is running (see the Create case in run): a newly
// created directory is exactly the thing the skip rule needs to keep out.
func (w *Watcher) addTree(root string, isRoot bool) error {
	return filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			// A directory that vanished mid-walk, or one we can't read, is
			// not fatal to the rest of the tree.
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		isWalkRoot := isRoot && path == root
		if !isWalkRoot && w.shouldSkipDir(path, entry.Name()) {
			return filepath.SkipDir
		}
		_ = w.watcher.Add(path)
		return nil
	})
}

// addDir adds a single directory (not its subtree) to the underlying
// fsnotify watcher, unless shouldSkipDir rejects it. Used to watch a
// component's imported file that lives outside the deck directory tree
// (see internal/cli's use of Bundle.Inputs), where walking the whole
// external tree would be both unnecessary and, for an arbitrary ancestor
// directory, unwelcome.
func (w *Watcher) addDir(dir string) {
	if w.shouldSkipDir(dir, filepath.Base(dir)) {
		return
	}
	_ = w.watcher.Add(dir)
}

// AddExtraDirs adds directories outside the deck directory tree to the
// watcher, without walking their subtrees: used for the directories of
// files a component imports from outside the deck folder (esbuild's
// metafile inputs), which the deck-tree walk in Start/addTree never
// reaches. Directories already watched, or that shouldSkipDir rejects
// (including anything under a node_modules directory, since a bare-import
// resolution can land there), are silently skipped. Safe to call whether or
// not the watcher has been started yet.
func (w *Watcher) AddExtraDirs(directories []string) {
	for _, dir := range directories {
		if dir == "" || strings.Contains(dir, string(filepath.Separator)+"node_modules"+string(filepath.Separator)) || strings.HasSuffix(dir, string(filepath.Separator)+"node_modules") {
			continue
		}
		w.addDir(dir)
	}
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

			// A newly created directory (e.g. `npm install` populating a
			// fresh node_modules, or a new components/ subfolder) is not
			// automatically watched by fsnotify; add its tree so files
			// inside it are seen too. shouldSkipDir applies to the new
			// directory itself here (unlike the deck root in Start/addTree),
			// so a freshly created .git or dist is skipped, and creating it
			// causes no rebuild (e.g. `git init` in the deck folder).
			if event.Has(fsnotify.Create) {
				if info, statErr := os.Stat(event.Name); statErr == nil && info.IsDir() {
					if w.shouldSkipDir(event.Name, filepath.Base(event.Name)) {
						continue
					}
					_ = w.addTree(event.Name, false)
				}
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
			// path is a fresh variable each iteration, so the closure below
			// captures this event's path by value; pendingPath itself is
			// read and written by this goroutine only; see AfterFunc's own
			// goroutine, which never touches it.
			path := pendingPath
			debounceTimer = time.AfterFunc(debounceTime, func() {
				w.triggerOnChange(path)
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

// triggerOnChange safely calls the onChange callback if set. callbackMu
// serializes calls, so an onChange that takes longer than the debounce
// window (a slow rebuild) always finishes before the next one starts,
// instead of the two racing and possibly landing out of order.
func (w *Watcher) triggerOnChange(path string) {
	w.mu.Lock()
	fn := w.onChange
	w.mu.Unlock()

	if fn == nil {
		return
	}

	w.callbackMu.Lock()
	defer w.callbackMu.Unlock()
	fn(path)
}

// WatchedFile returns the path of the main markdown file being watched.
func (w *Watcher) WatchedFile() string {
	return w.mdFile
}

// WatchedDir returns the path of the directory being watched for assets.
func (w *Watcher) WatchedDir() string {
	return w.mdDir
}
