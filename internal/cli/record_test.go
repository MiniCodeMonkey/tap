package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
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

func TestCloseRemovesTestCaptures(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "recordings")

	opened := make(chan string, 1)
	controller := newRecordController(recordControllerOptions{
		DeckTitle:    "My Talk",
		OutputDir:    dir,
		CommandName:  fakeInstantRecorderBinary(t),
		CurrentSlide: func() (int, bool) { return 0, true },
		TitleFor:     func(int) string { return "Title" },
		OpenFile:     func(path string) error { opened <- path; return nil },
	})

	if err := controller.Test(1); err != nil {
		t.Fatalf("Test() returned %v", err)
	}

	var path string
	select {
	case path = <-opened:
	case <-time.After(10 * time.Second):
		t.Fatal("the test capture never opened a file")
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("the test capture is missing before Close(): %v", err)
	}

	controller.Close()

	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("Close() left the test capture behind: stat returned %v", err)
	}
}

func TestControllerStopWithoutStart(t *testing.T) {
	controller := testController(t, t.TempDir(), func() (int, bool) { return 0, true })

	if _, err := controller.Stop(); err != nil {
		t.Errorf("Stop() with nothing running returned %v, want nil", err)
	}
}

func TestControllerReportsARecorderThatDiesOnItsOwn(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "recordings")

	quitter := filepath.Join(t.TempDir(), "quitting-recorder")
	if err := os.WriteFile(quitter, []byte("#!/bin/sh\nexit 3\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	died := make(chan error, 1)
	controller := newRecordController(recordControllerOptions{
		DeckTitle:        "My Talk",
		OutputDir:        dir,
		CommandName:      quitter,
		CurrentSlide:     func() (int, bool) { return 0, false },
		TitleFor:         func(int) string { return "Title" },
		OpenFile:         func(string) error { return nil },
		OnUnexpectedExit: func(err error) { died <- err },
	})

	if _, err := controller.Start(1); err != nil {
		t.Fatalf("Start() returned %v", err)
	}

	select {
	case err := <-died:
		if err == nil {
			t.Error("the callback fired with no error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the controller never reported the recorder exiting")
	}

	if controller.Recording() {
		t.Error("the controller still believes it is recording")
	}
}

func TestControllerDoesNotReportAnOrdinaryStop(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "recordings")

	died := make(chan error, 1)
	controller := newRecordController(recordControllerOptions{
		DeckTitle:        "My Talk",
		OutputDir:        dir,
		CommandName:      fakeRecorderBinary(t),
		CurrentSlide:     func() (int, bool) { return 0, false },
		TitleFor:         func(int) string { return "Title" },
		OpenFile:         func(string) error { return nil },
		OnUnexpectedExit: func(err error) { died <- err },
	})

	if _, err := controller.Start(1); err != nil {
		t.Fatalf("Start() returned %v", err)
	}
	if _, err := controller.Stop(); err != nil {
		t.Fatalf("Stop() returned %v", err)
	}

	select {
	case err := <-died:
		t.Fatalf("a deliberate stop reported an unexpected exit: %v", err)
	case <-time.After(500 * time.Millisecond):
	}
}

func TestControllerCrashSetsLastResultWithTheChapterPath(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "recordings")

	quitter := filepath.Join(t.TempDir(), "quitting-recorder")
	if err := os.WriteFile(quitter, []byte("#!/bin/sh\nexit 3\n"), 0o700); err != nil {
		t.Fatal(err)
	}

	died := make(chan error, 1)
	controller := newRecordController(recordControllerOptions{
		DeckTitle:        "My Talk",
		OutputDir:        dir,
		Chapters:         true,
		CommandName:      quitter,
		CurrentSlide:     func() (int, bool) { return 0, true },
		TitleFor:         func(int) string { return "Title" },
		OpenFile:         func(string) error { return nil },
		OnUnexpectedExit: func(err error) { died <- err },
	})

	if _, err := controller.Start(1); err != nil {
		t.Fatalf("Start() returned %v", err)
	}

	select {
	case <-died:
	case <-time.After(5 * time.Second):
		t.Fatal("the controller never reported the recorder exiting")
	}

	// The crash happened well before anyone called Stop, so the next Stop
	// (runDevServer's deferred one, say) must still be able to say where
	// the file and its chapter list went.
	result, err := controller.Stop()
	if err != nil {
		t.Fatalf("Stop() after a crash returned %v", err)
	}
	if result.Path == "" {
		t.Error("Stop() after a crash returned no path")
	}
	if result.ChapterPath == "" {
		t.Error("Stop() after a crash returned no chapter path, so the crash summary prints nothing")
	}
}

func TestGitignoreEntryFollowsTheOutputDirectory(t *testing.T) {
	root := gitRepo(t)
	controller := testController(t, filepath.Join(root, "captures"), func() (int, bool) { return 0, true })

	if got := controller.SuggestGitignore(); got != "captures/" {
		t.Errorf("SuggestGitignore() = %q, want captures/", got)
	}
}

func TestGitignoreEntryIsEmptyOutsideARepository(t *testing.T) {
	controller := testController(t, filepath.Join(t.TempDir(), "recordings"), func() (int, bool) { return 0, true })

	if got := controller.SuggestGitignore(); got != "" {
		t.Errorf("SuggestGitignore() = %q, want an empty string outside a repository", got)
	}
}

func TestGitignoreEntryHandlesANestedOutputDirectory(t *testing.T) {
	root := gitRepo(t)
	controller := testController(t, filepath.Join(root, "talks", "2026", "recordings"), func() (int, bool) { return 0, true })

	if got := controller.SuggestGitignore(); got != "talks/2026/recordings/" {
		t.Errorf("SuggestGitignore() = %q, want talks/2026/recordings/", got)
	}
}

func TestControllerStopsWhenTheDiskFills(t *testing.T) {
	levels := make(chan recorder.DiskLevel, 4)
	controller := newRecordController(recordControllerOptions{
		DeckTitle:   "My Talk",
		OutputDir:   t.TempDir(),
		CommandName: fakeRecorderBinary(t),
		FreeSpace:   func(string) (uint64, error) { return 512 << 20, nil },
		OnDiskLevel: func(level recorder.DiskLevel) { levels <- level },
	})

	if _, err := controller.Start(1); err != nil {
		t.Fatal(err)
	}
	select {
	case level := <-levels:
		if level != recorder.DiskFull {
			t.Fatalf("level = %v, want DiskFull", level)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no disk level reported")
	}
	if controller.Recording() {
		t.Error("the controller is still recording on a full disk")
	}
}

// TestControllerReportsDiskOKAfterAnOrdinaryStopFollowingLowDisk covers the
// ruling that extends the brief: a normal Stop (not the DiskFull stop) after
// the watch last reported DiskLow reports DiskOK, so the "almost full" badge
// does not stay up once recording ends.
func TestControllerReportsDiskOKAfterAnOrdinaryStopFollowingLowDisk(t *testing.T) {
	levels := make(chan recorder.DiskLevel, 4)
	free := uint64(4 << 30)
	controller := newRecordController(recordControllerOptions{
		DeckTitle:   "My Talk",
		OutputDir:   t.TempDir(),
		CommandName: fakeRecorderBinary(t),
		FreeSpace:   func(string) (uint64, error) { return free, nil },
		OnDiskLevel: func(level recorder.DiskLevel) { levels <- level },
	})

	if _, err := controller.Start(1); err != nil {
		t.Fatal(err)
	}
	select {
	case level := <-levels:
		if level != recorder.DiskLow {
			t.Fatalf("level = %v, want DiskLow", level)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no disk level reported")
	}

	if _, err := controller.Stop(); err != nil {
		t.Fatalf("Stop() returned %v", err)
	}

	select {
	case level := <-levels:
		if level != recorder.DiskOK {
			t.Fatalf("level = %v, want DiskOK after an ordinary stop", level)
		}
	case <-time.After(time.Second):
		t.Fatal("no DiskOK reported after stopping")
	}
}
