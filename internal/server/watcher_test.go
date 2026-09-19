package server

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewWatcher(t *testing.T) {
	// Create a temp file to watch
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Stop()

	if w.WatchedFile() != mdFile {
		t.Errorf("WatchedFile() = %v, want %v", w.WatchedFile(), mdFile)
	}

	if w.WatchedDir() != tmpDir {
		t.Errorf("WatchedDir() = %v, want %v", w.WatchedDir(), tmpDir)
	}
}

func TestNewWatcher_ResolvesAbsolutePath(t *testing.T) {
	// Create a temp file
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Change to tmpDir and use relative path
	oldWd, _ := os.Getwd()
	os.Chdir(tmpDir)
	defer os.Chdir(oldWd)

	w, err := NewWatcher("test.md")
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Stop()

	// Should have resolved to absolute path
	if !filepath.IsAbs(w.WatchedFile()) {
		t.Errorf("WatchedFile() = %v, want absolute path", w.WatchedFile())
	}
}

func TestWatcher_Start_Stop(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	if w.IsRunning() {
		t.Error("IsRunning() = true before Start()")
	}

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	if !w.IsRunning() {
		t.Error("IsRunning() = false after Start()")
	}

	// Start again should be a no-op
	if err := w.Start(); err != nil {
		t.Fatalf("Start() again error = %v", err)
	}

	if err := w.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	if w.IsRunning() {
		t.Error("IsRunning() = true after Stop()")
	}
}

func TestWatcher_OnChange(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	// Use atomic for thread-safe counter
	var callCount atomic.Int32
	var lastPath string
	var mu sync.Mutex

	w.SetOnChange(func(path string) {
		callCount.Add(1)
		mu.Lock()
		lastPath = path
		mu.Unlock()
	})

	// Use shorter debounce for testing
	w.SetDebounceTime(10 * time.Millisecond)

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer w.Stop()

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Modify the file
	if err := os.WriteFile(mdFile, []byte("# Updated"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Wait for debounce and callback
	time.Sleep(100 * time.Millisecond)

	if callCount.Load() == 0 {
		t.Error("onChange callback was not called")
	}

	mu.Lock()
	if lastPath != mdFile {
		t.Errorf("onChange path = %v, want %v", lastPath, mdFile)
	}
	mu.Unlock()
}

func TestWatcher_Debounce(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	var callCount atomic.Int32

	w.SetOnChange(func(path string) {
		callCount.Add(1)
	})

	// Use longer debounce time
	w.SetDebounceTime(100 * time.Millisecond)

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer w.Stop()

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Fire a burst of writes back to back, with no sleep between them, so
	// the whole burst lands well inside one debounce window regardless of
	// how loaded the machine running the test is. A fixed inter-write sleep
	// close to the debounce window is what made this test flaky on slow
	// runners: the writes could spread past the window and split into more
	// than one debounce cycle even though the debounce logic itself was
	// fine.
	const writes = 10
	for i := 0; i < writes; i++ {
		if err := os.WriteFile(mdFile, []byte("# Update "+string(rune('0'+i))), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
	}

	// Wait well past the debounce window for it to fire.
	time.Sleep(300 * time.Millisecond)

	// A burst of writes inside one debounce window should collapse to far
	// fewer callbacks than writes, not zero (the burst must still trigger a
	// rebuild) and not one per write (debouncing must have done its job).
	count := callCount.Load()
	if count < 1 || count > 3 {
		t.Errorf("callCount = %d, want between 1 and 3 (debouncing should collapse a %d-write burst into a small number of callbacks)", count, writes)
	}
}

func TestWatcher_FileCreate(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	var callCount atomic.Int32
	var lastPath string
	var mu sync.Mutex

	w.SetOnChange(func(path string) {
		callCount.Add(1)
		mu.Lock()
		lastPath = path
		mu.Unlock()
	})
	w.SetDebounceTime(10 * time.Millisecond)

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer w.Stop()

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Create a new file in the directory
	newFile := filepath.Join(tmpDir, "image.png")
	if err := os.WriteFile(newFile, []byte("fake image"), 0644); err != nil {
		t.Fatalf("failed to create new file: %v", err)
	}

	// Wait for callback
	time.Sleep(100 * time.Millisecond)

	if callCount.Load() == 0 {
		t.Error("onChange was not called for new file")
	}

	mu.Lock()
	if lastPath != newFile {
		t.Errorf("onChange path = %v, want %v", lastPath, newFile)
	}
	mu.Unlock()
}

func TestWatcher_RecursiveSubdirectoryChange(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	slidesDir := filepath.Join(tmpDir, "slides")
	if err := os.MkdirAll(slidesDir, 0755); err != nil {
		t.Fatalf("failed to create slides dir: %v", err)
	}
	componentFile := filepath.Join(slidesDir, "RollingDeploy.jsx")
	if err := os.WriteFile(componentFile, []byte("export default function () {}"), 0644); err != nil {
		t.Fatalf("failed to create component file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	var callCount atomic.Int32
	w.SetOnChange(func(path string) { callCount.Add(1) })
	w.SetDebounceTime(10 * time.Millisecond)

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer w.Stop()

	time.Sleep(50 * time.Millisecond)

	if err := os.WriteFile(componentFile, []byte("export default function () { return null; }"), 0644); err != nil {
		t.Fatalf("failed to update component file: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if callCount.Load() == 0 {
		t.Error("onChange was not called for a change in a subdirectory of the deck directory")
	}
}

func TestWatcher_SkipsNodeModules(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	nodeModulesDir := filepath.Join(tmpDir, "node_modules", "some-lib")
	if err := os.MkdirAll(nodeModulesDir, 0755); err != nil {
		t.Fatalf("failed to create node_modules dir: %v", err)
	}
	libFile := filepath.Join(nodeModulesDir, "index.js")
	if err := os.WriteFile(libFile, []byte("module.exports = {}"), 0644); err != nil {
		t.Fatalf("failed to create lib file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	var callCount atomic.Int32
	w.SetOnChange(func(path string) { callCount.Add(1) })
	w.SetDebounceTime(10 * time.Millisecond)

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer w.Stop()

	time.Sleep(50 * time.Millisecond)

	if err := os.WriteFile(libFile, []byte("module.exports = { changed: true }"), 0644); err != nil {
		t.Fatalf("failed to update lib file: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if callCount.Load() != 0 {
		t.Error("onChange was called for a change inside node_modules, which should not be watched")
	}
}

func TestWatcher_SkipsDotDirectoriesAndDist(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	gitDir := filepath.Join(tmpDir, ".git", "objects")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("failed to create .git dir: %v", err)
	}
	gitFile := filepath.Join(gitDir, "pack")
	if err := os.WriteFile(gitFile, []byte("git internals"), 0644); err != nil {
		t.Fatalf("failed to create git file: %v", err)
	}

	distDir := filepath.Join(tmpDir, "dist")
	if err := os.MkdirAll(distDir, 0755); err != nil {
		t.Fatalf("failed to create dist dir: %v", err)
	}
	distFile := filepath.Join(distDir, "index.html")
	if err := os.WriteFile(distFile, []byte("<html></html>"), 0644); err != nil {
		t.Fatalf("failed to create dist file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	var callCount atomic.Int32
	w.SetOnChange(func(path string) { callCount.Add(1) })
	w.SetDebounceTime(10 * time.Millisecond)

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer w.Stop()

	time.Sleep(50 * time.Millisecond)

	if err := os.WriteFile(gitFile, []byte("changed"), 0644); err != nil {
		t.Fatalf("failed to update git file: %v", err)
	}
	if err := os.WriteFile(distFile, []byte("<html>changed</html>"), 0644); err != nil {
		t.Fatalf("failed to update dist file: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if callCount.Load() != 0 {
		t.Errorf("onChange was called %d time(s) for changes inside .git and dist, which should not be watched", callCount.Load())
	}

	// Creating a skipped directory itself while the watcher is running
	// (e.g. `git init` in the deck folder) must not trigger a rebuild.
	callCount.Store(0)
	freshGitDir := filepath.Join(tmpDir, ".git2")
	if err := os.Mkdir(freshGitDir, 0755); err != nil {
		t.Fatalf("failed to create fresh dot dir: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if callCount.Load() != 0 {
		t.Error("onChange was called for creating a skipped directory itself, which should not trigger a rebuild")
	}
}

func TestWatcher_DeckRootSkippableName_StillWatchesSubtree(t *testing.T) {
	for _, rootName := range []string{".drafts", "dist"} {
		t.Run(rootName, func(t *testing.T) {
			parentDir := t.TempDir()
			deckDir := filepath.Join(parentDir, rootName)
			if err := os.MkdirAll(deckDir, 0755); err != nil {
				t.Fatalf("failed to create deck dir: %v", err)
			}

			mdFile := filepath.Join(deckDir, "test.md")
			if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
				t.Fatalf("failed to create test file: %v", err)
			}

			slidesDir := filepath.Join(deckDir, "slides")
			if err := os.MkdirAll(slidesDir, 0755); err != nil {
				t.Fatalf("failed to create slides dir: %v", err)
			}
			componentFile := filepath.Join(slidesDir, "RollingDeploy.jsx")
			if err := os.WriteFile(componentFile, []byte("export default function () {}"), 0644); err != nil {
				t.Fatalf("failed to create component file: %v", err)
			}

			w, err := NewWatcher(mdFile)
			if err != nil {
				t.Fatalf("NewWatcher() error = %v", err)
			}

			var callCount atomic.Int32
			w.SetOnChange(func(path string) { callCount.Add(1) })
			w.SetDebounceTime(10 * time.Millisecond)

			if err := w.Start(); err != nil {
				t.Fatalf("Start() error = %v", err)
			}
			defer w.Stop()

			time.Sleep(50 * time.Millisecond)

			if err := os.WriteFile(componentFile, []byte("export default function () { return null; }"), 0644); err != nil {
				t.Fatalf("failed to update component file: %v", err)
			}

			time.Sleep(100 * time.Millisecond)

			if callCount.Load() == 0 {
				t.Errorf("onChange was not called for a change in a subfolder of a deck directory named %q, which must still be watched", rootName)
			}
		})
	}
}

func TestWatcher_FileRename(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	var callCount atomic.Int32

	w.SetOnChange(func(path string) {
		callCount.Add(1)
	})
	w.SetDebounceTime(10 * time.Millisecond)

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer w.Stop()

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Simulate atomic save: write to temp, rename over original
	tempFile := filepath.Join(tmpDir, "test.md.tmp")
	if err := os.WriteFile(tempFile, []byte("# Updated via rename"), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}
	if err := os.Rename(tempFile, mdFile); err != nil {
		t.Fatalf("failed to rename file: %v", err)
	}

	// Wait for watcher to re-add file and callback
	time.Sleep(200 * time.Millisecond)

	// The callback might be called for the delete/rename, or for re-watching
	// The key thing is the watcher should still be running
	if !w.IsRunning() {
		t.Error("Watcher stopped after file rename")
	}
}

func TestWatcher_StopWhileDebouncing(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	var callCount atomic.Int32

	w.SetOnChange(func(path string) {
		callCount.Add(1)
	})
	w.SetDebounceTime(500 * time.Millisecond)

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Trigger change
	if err := os.WriteFile(mdFile, []byte("# Updated"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Stop immediately (before debounce completes)
	time.Sleep(50 * time.Millisecond)
	if err := w.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}

	// Callback should not have been called (debounce not complete)
	if callCount.Load() != 0 {
		t.Error("Callback was called after Stop() but before debounce completed")
	}
}

func TestWatcher_NoCallback(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	// Don't set callback
	w.SetDebounceTime(10 * time.Millisecond)

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer w.Stop()

	// Give watcher time to start
	time.Sleep(50 * time.Millisecond)

	// Modify file - should not panic even without callback
	if err := os.WriteFile(mdFile, []byte("# Updated"), 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// Wait for debounce
	time.Sleep(100 * time.Millisecond)

	// Test passes if no panic occurred
}

// TestWatcher_TriggerOnChangeSerializesCallback verifies that two
// overlapping triggerOnChange calls never run the onChange callback
// concurrently: a slow rebuild must finish before the next one starts,
// so a faster later call can't have its result overtaken mid-flight.
func TestWatcher_TriggerOnChangeSerializesCallback(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	var active atomic.Int32
	var overlapped atomic.Bool
	w.SetOnChange(func(path string) {
		if active.Add(1) > 1 {
			overlapped.Store(true)
		}
		time.Sleep(20 * time.Millisecond)
		active.Add(-1)
	})

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		w.triggerOnChange("a")
	}()
	go func() {
		defer wg.Done()
		w.triggerOnChange("b")
	}()
	wg.Wait()

	if overlapped.Load() {
		t.Error("onChange ran concurrently for two triggerOnChange calls, want them serialized")
	}
}

func TestWatcher_DefaultDebounceTime(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}
	defer w.Stop()

	// Check default is 100ms
	if w.debounceTime != 100*time.Millisecond {
		t.Errorf("default debounceTime = %v, want %v", w.debounceTime, 100*time.Millisecond)
	}
}

func TestWatcher_Stop_WhenNotRunning(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	// Stop without starting should not error
	if err := w.Stop(); err != nil {
		t.Errorf("Stop() error = %v, want nil", err)
	}
}

func TestWatcher_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	w.SetDebounceTime(10 * time.Millisecond)

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer w.Stop()

	// Concurrent access to SetOnChange and SetDebounceTime
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			w.SetOnChange(func(path string) {})
		}()
		go func() {
			defer wg.Done()
			w.SetDebounceTime(20 * time.Millisecond)
		}()
	}
	wg.Wait()

	// Test passes if no race condition
}

// TestWatcher_AddExtraDirs_WatchesFileOutsideDeckTree verifies that a
// directory added through AddExtraDirs (used for a component's imports that
// live outside the deck directory tree, from esbuild's metafile inputs) is
// watched even though it is never reached by the recursive walk of the deck
// directory itself.
func TestWatcher_AddExtraDirs_WatchesFileOutsideDeckTree(t *testing.T) {
	rootDir := t.TempDir()
	deckDir := filepath.Join(rootDir, "deck")
	sharedDir := filepath.Join(rootDir, "shared")
	if err := os.MkdirAll(deckDir, 0755); err != nil {
		t.Fatalf("failed to create deck dir: %v", err)
	}
	if err := os.MkdirAll(sharedDir, 0755); err != nil {
		t.Fatalf("failed to create shared dir: %v", err)
	}

	mdFile := filepath.Join(deckDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	sharedFile := filepath.Join(sharedDir, "Thing.jsx")
	if err := os.WriteFile(sharedFile, []byte("export default function Thing() {}"), 0644); err != nil {
		t.Fatalf("failed to create shared file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	var callCount atomic.Int32
	w.SetOnChange(func(path string) { callCount.Add(1) })
	w.SetDebounceTime(10 * time.Millisecond)

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer w.Stop()

	w.AddExtraDirs([]string{sharedDir})

	time.Sleep(50 * time.Millisecond)

	if err := os.WriteFile(sharedFile, []byte("export default function Thing() { return null; }"), 0644); err != nil {
		t.Fatalf("failed to update shared file: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if callCount.Load() == 0 {
		t.Error("onChange was not called for a change to a file outside the deck tree added via AddExtraDirs")
	}
}

// TestWatcher_AddExtraDirs_SkipsNodeModules verifies that AddExtraDirs does
// not add a directory under node_modules, even when it lies outside the
// deck tree (a bare import can resolve into an ancestor's node_modules).
func TestWatcher_AddExtraDirs_SkipsNodeModules(t *testing.T) {
	rootDir := t.TempDir()
	deckDir := filepath.Join(rootDir, "deck")
	libDir := filepath.Join(rootDir, "node_modules", "some-lib")
	if err := os.MkdirAll(deckDir, 0755); err != nil {
		t.Fatalf("failed to create deck dir: %v", err)
	}
	if err := os.MkdirAll(libDir, 0755); err != nil {
		t.Fatalf("failed to create lib dir: %v", err)
	}

	mdFile := filepath.Join(deckDir, "test.md")
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}
	libFile := filepath.Join(libDir, "index.js")
	if err := os.WriteFile(libFile, []byte("module.exports = {}"), 0644); err != nil {
		t.Fatalf("failed to create lib file: %v", err)
	}

	w, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("NewWatcher() error = %v", err)
	}

	var callCount atomic.Int32
	w.SetOnChange(func(path string) { callCount.Add(1) })
	w.SetDebounceTime(10 * time.Millisecond)

	if err := w.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer w.Stop()

	w.AddExtraDirs([]string{libDir})

	time.Sleep(50 * time.Millisecond)

	if err := os.WriteFile(libFile, []byte("module.exports = { changed: true }"), 0644); err != nil {
		t.Fatalf("failed to update lib file: %v", err)
	}

	time.Sleep(100 * time.Millisecond)

	if callCount.Load() != 0 {
		t.Error("onChange was called for a change under node_modules added via AddExtraDirs, which should be skipped")
	}
}
