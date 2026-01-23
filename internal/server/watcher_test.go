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

	// Make multiple rapid changes
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(mdFile, []byte("# Update "+string(rune('0'+i))), 0644); err != nil {
			t.Fatalf("failed to write file: %v", err)
		}
		time.Sleep(20 * time.Millisecond) // Less than debounce time
	}

	// Wait for debounce to complete
	time.Sleep(200 * time.Millisecond)

	// Should have only one callback due to debouncing
	count := callCount.Load()
	if count > 2 {
		t.Errorf("callCount = %d, want <= 2 (debouncing should reduce calls)", count)
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
