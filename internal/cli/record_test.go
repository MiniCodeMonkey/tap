package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// fakeRecorderBinary writes a script that finalizes its output on SIGINT,
// the way screencapture does.
func fakeRecorderBinary(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fake-recorder")
	script := "#!/bin/sh\nfor last in \"$@\"; do :; done\n" +
		"trap 'printf recorded > \"$last\"; exit 0' INT\nsleep 60 &\nwait $!\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// fakeInstantRecorderBinary writes its output and exits immediately,
// instead of waiting for SIGINT. TestControllerTestCaptureOpensAFileAndRecordsNothingPermanent
// runs the recorder to completion rather than stopping it early, so a
// binary that traps SIGINT and sleeps would make it sit through the full
// test-capture timeout before finishing.
func fakeInstantRecorderBinary(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fake-instant-recorder")
	script := "#!/bin/sh\nfor last in \"$@\"; do :; done\n" +
		"printf recorded > \"$last\"\nexit 0\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

func testController(t *testing.T, outputDir string, currentSlide func() (int, bool)) *recordController {
	t.Helper()

	return newRecordController(recordControllerOptions{
		DeckTitle:    "My Talk",
		OutputDir:    outputDir,
		Chapters:     true,
		CommandName:  fakeRecorderBinary(t),
		CurrentSlide: currentSlide,
		TitleFor:     func(index int) string { return fmt.Sprintf("Slide title %d", index) },
		OpenFile:     func(string) error { return nil },
	})
}

func TestControllerStartSeedsTheChapterListFromTheCurrentSlide(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "recordings")
	controller := testController(t, dir, func() (int, bool) { return 4, true })

	path, err := controller.Start(1)
	if err != nil {
		t.Fatalf("Start() returned %v", err)
	}
	if !strings.HasSuffix(path, ".mov") {
		t.Errorf("Start() returned %q, want a .mov path", path)
	}

	result, err := controller.Stop()
	if err != nil {
		t.Fatalf("Stop() returned %v", err)
	}

	written, err := os.ReadFile(result.ChapterPath)
	if err != nil {
		t.Fatalf("the chapter list was not written: %v", err)
	}
	if !strings.HasPrefix(string(written), "0:00 Slide title 4\n") {
		t.Errorf("the chapter list starts with %q, want the slide showing at start", written)
	}
}

func TestControllerRecordsSlideChanges(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "recordings")
	controller := testController(t, dir, func() (int, bool) { return 0, true })

	if _, err := controller.Start(1); err != nil {
		t.Fatalf("Start() returned %v", err)
	}
	controller.NoteSlideChange(1)
	controller.NoteSlideChange(2)

	result, err := controller.Stop()
	if err != nil {
		t.Fatalf("Stop() returned %v", err)
	}

	written, _ := os.ReadFile(result.ChapterPath)
	if strings.Count(string(written), "\n") != 3 {
		t.Errorf("the chapter list holds %q, want three entries", written)
	}
}

func TestControllerIgnoresSlideChangesWhenNotRecording(t *testing.T) {
	controller := testController(t, t.TempDir(), func() (int, bool) { return 0, true })

	controller.NoteSlideChange(2)

	if controller.Recording() {
		t.Error("the controller believes it is recording")
	}
}

func TestControllerWritesNoChapterListWhenChaptersAreOff(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "recordings")
	controller := newRecordController(recordControllerOptions{
		DeckTitle:    "My Talk",
		OutputDir:    dir,
		Chapters:     false,
		CommandName:  fakeRecorderBinary(t),
		CurrentSlide: func() (int, bool) { return 0, true },
		TitleFor:     func(int) string { return "Title" },
		OpenFile:     func(string) error { return nil },
	})

	if _, err := controller.Start(1); err != nil {
		t.Fatalf("Start() returned %v", err)
	}
	result, err := controller.Stop()
	if err != nil {
		t.Fatalf("Stop() returned %v", err)
	}

	if result.ChapterPath != "" {
		t.Errorf("Result.ChapterPath = %q, want empty with chapters off", result.ChapterPath)
	}
}

func TestControllerTestCaptureOpensAFileAndRecordsNothingPermanent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "recordings")

	opened := make(chan string, 1)
	controller := newRecordController(recordControllerOptions{
		DeckTitle:    "My Talk",
		OutputDir:    dir,
		Chapters:     true,
		CommandName:  fakeInstantRecorderBinary(t),
		CurrentSlide: func() (int, bool) { return 0, true },
		TitleFor:     func(int) string { return "Title" },
		OpenFile:     func(path string) error { opened <- path; return nil },
	})

	if err := controller.Test(1); err != nil {
		t.Fatalf("Test() returned %v", err)
	}

	select {
	case path := <-opened:
		if strings.HasPrefix(path, dir) {
			t.Errorf("the test capture landed in %q, want the temp directory", path)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the test capture never opened a file")
	}

	if entries, err := os.ReadDir(dir); err == nil && len(entries) > 0 {
		t.Errorf("the test capture left %d files in the recordings directory", len(entries))
	}
}

func TestControllerStopTwiceReturnsTheSameResult(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "recordings")
	controller := testController(t, dir, func() (int, bool) { return 0, true })

	if _, err := controller.Start(1); err != nil {
		t.Fatalf("Start() returned %v", err)
	}

	first, err := controller.Stop()
	if err != nil {
		t.Fatalf("Stop() returned %v", err)
	}

	second, err := controller.Stop()
	if err != nil {
		t.Fatalf("second Stop() returned %v", err)
	}

	if second.Path != first.Path {
		t.Errorf("second Stop() returned path %q, want %q", second.Path, first.Path)
	}
	if second.ChapterPath != first.ChapterPath {
		t.Errorf("second Stop() returned chapter path %q, want %q", second.ChapterPath, first.ChapterPath)
	}
}

func TestControllerStopWithoutStart(t *testing.T) {
	controller := testController(t, t.TempDir(), func() (int, bool) { return 0, true })

	if _, err := controller.Stop(); err != nil {
		t.Errorf("Stop() with nothing running returned %v, want nil", err)
	}
}
