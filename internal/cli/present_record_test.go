package cli

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
	"github.com/MiniCodeMonkey/tap/internal/tui"
)

type screenFeed struct {
	mu      sync.Mutex
	screens []recorder.Screen
}

func (f *screenFeed) set(screens []recorder.Screen) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.screens = screens
}

func (f *screenFeed) list() ([]recorder.Screen, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.screens, nil
}

var (
	laptopScreens    = []recorder.Screen{{Name: "Color LCD", Index: 1, BuiltIn: true}}
	projectorScreens = []recorder.Screen{{Name: "Color LCD", Index: 1, BuiltIn: true}, {Name: "EPSON PJ", Index: 2}}
)

func testPresentRecorder(t *testing.T, feed *screenFeed, free uint64) *presentRecorder {
	t.Helper()
	outputDir := filepath.Join(t.TempDir(), "recordings")
	return newPresentRecorder(presentRecorderOptions{
		Run: recorder.RunOptions{
			Parent:    outputDir,
			DeckTitle: "My Talk",
			Base:      recorder.Options{Command: fakeRecorderBinary(t), NoAudio: true},
			Chapters:  true,
		},
		OutputDir:    outputDir,
		ListScreens:  feed.list,
		FreeSpace:    func(string) (uint64, error) { return free, nil },
		PollInterval: 20 * time.Millisecond,
		DiskInterval: 20 * time.Millisecond,
	})
}

func waitForState(t *testing.T, present *presentRecorder, want tui.PresentRecordingState) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if present.State() == want {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("state = %v, want %v", present.State(), want)
}

func TestPresentRecorderRecordsFromBegin(t *testing.T) {
	feed := &screenFeed{screens: laptopScreens}
	present := testPresentRecorder(t, feed, 50<<30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	present.Begin(ctx, true)
	waitForState(t, present, tui.PresentRecording)
	_, _ = present.Finish(true)
}

func TestPresentRecorderPausesWhileTheProjectorIsAway(t *testing.T) {
	feed := &screenFeed{screens: projectorScreens}
	present := testPresentRecorder(t, feed, 50<<30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	present.Begin(ctx, true)
	waitForState(t, present, tui.PresentRecording)

	feed.set(laptopScreens)
	waitForState(t, present, tui.PresentPaused)

	feed.set(projectorScreens)
	waitForState(t, present, tui.PresentRecording)

	present.NoteSlideChange(1)
	summary, err := present.Finish(true)
	if err != nil {
		t.Fatal(err)
	}
	if summary.Segments != 2 {
		t.Errorf("segments = %d, want 2 (before and after the swap)", summary.Segments)
	}
}

func TestPresentRecorderToggleStopsAndStartsSegments(t *testing.T) {
	feed := &screenFeed{screens: laptopScreens}
	present := testPresentRecorder(t, feed, 50<<30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	present.Begin(ctx, true)
	waitForState(t, present, tui.PresentRecording)

	if err := present.Toggle(); err != nil {
		t.Fatal(err)
	}
	if present.State() != tui.PresentNotRecording {
		t.Fatalf("state after c = %v, want not recording", present.State())
	}
	time.Sleep(100 * time.Millisecond)
	if present.State() != tui.PresentNotRecording {
		t.Fatal("the display poll restarted a recording the speaker stopped")
	}

	if err := present.Toggle(); err != nil {
		t.Fatal(err)
	}
	waitForState(t, present, tui.PresentRecording)
	_, _ = present.Finish(true)
}

func TestPresentRecorderStopsOnAFullDisk(t *testing.T) {
	levels := make(chan recorder.DiskLevel, 4)
	feed := &screenFeed{screens: laptopScreens}
	present := testPresentRecorder(t, feed, 512<<20)
	present.options.OnDiskLevel = func(level recorder.DiskLevel) { levels <- level }
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	present.Begin(ctx, true)
	select {
	case level := <-levels:
		if level != recorder.DiskFull {
			t.Fatalf("level = %v, want full", level)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("no disk level")
	}
	waitForState(t, present, tui.PresentNotRecording)
	time.Sleep(100 * time.Millisecond)
	if present.State() != tui.PresentNotRecording {
		t.Error("the display poll restarted the recording on a full disk")
	}
	_, _ = present.Finish(true)
}

func TestPresentRecorderBlockedNeverStarts(t *testing.T) {
	feed := &screenFeed{screens: laptopScreens}
	present := testPresentRecorder(t, feed, 50<<30)
	present.Block("Screen Recording permission is missing")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	present.Begin(ctx, true)
	time.Sleep(100 * time.Millisecond)
	if present.Started() || present.Blocked() == "" {
		t.Error("a blocked recorder started or forgot why it is blocked")
	}
}

func TestPresentRecorderDeletesAFirstSlideOnlyRun(t *testing.T) {
	feed := &screenFeed{screens: laptopScreens}
	present := testPresentRecorder(t, feed, 50<<30)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	present.Begin(ctx, true)
	waitForState(t, present, tui.PresentRecording)
	summary, err := present.Finish(true)
	if err != nil {
		t.Fatal(err)
	}
	if !summary.Deleted {
		t.Error("a run that never left the first slide was kept")
	}
	if _, err := os.Stat(summary.Dir); !os.IsNotExist(err) {
		t.Error("the run folder is still on disk")
	}
}

// fakeExitingRecorderBinary writes a script that exits at once with a
// nonzero status, the way a recorder whose Screen Recording permission was
// just revoked would.
func fakeExitingRecorderBinary(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fake-exiting-recorder")
	script := "#!/bin/sh\nexit 3\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestPresentRecorderHoldsAfterAnImmediateSegmentExit covers ruling 1: a
// segment that exits before it could plausibly have recorded anything must
// not be respawned in a hot loop. The run should end up NotRecording and
// held, and it must not have kept creating new segment files.
func TestPresentRecorderHoldsAfterAnImmediateSegmentExit(t *testing.T) {
	feed := &screenFeed{screens: laptopScreens}
	outputDir := filepath.Join(t.TempDir(), "recordings")
	events := make(chan string, 8)
	present := newPresentRecorder(presentRecorderOptions{
		Run: recorder.RunOptions{
			Parent:    outputDir,
			DeckTitle: "My Talk",
			Base:      recorder.Options{Command: fakeExitingRecorderBinary(t), NoAudio: true},
			Chapters:  true,
		},
		OutputDir:    outputDir,
		ListScreens:  feed.list,
		FreeSpace:    func(string) (uint64, error) { return 50 << 30, nil },
		OnEvent:      func(eventType, message string) { events <- eventType },
		PollInterval: 20 * time.Millisecond,
		DiskInterval: 20 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	present.Begin(ctx, true)
	waitForState(t, present, tui.PresentNotRecording)

	sawError := false
	deadline := time.After(time.Second)
loop:
	for {
		select {
		case eventType := <-events:
			if eventType == "error" {
				sawError = true
				break loop
			}
		case <-deadline:
			break loop
		}
	}
	if !sawError {
		t.Error("a recorder that exits immediately did not report why it stopped")
	}

	time.Sleep(500 * time.Millisecond)
	if present.State() != tui.PresentNotRecording {
		t.Fatal("the display poll respawned a recorder that exits immediately")
	}

	dir := present.run.Dir()
	if dir != "" {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatal(err)
		}
		count := 0
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".mov" {
				count++
			}
		}
		if count > 2 {
			t.Errorf("segment files = %d, want at most 2 (no hot loop)", count)
		}
	}
	_, _ = present.Finish(true)
}

// slowStoppingRecorderBinary writes a script that takes its time finalizing
// on SIGINT: long enough that a State() implementation which shares
// presentRecorder's own mutex with the segment switch would visibly block,
// but well under the recorder package's unexported kill grace (5s), which a
// cli-package test cannot reach or shorten.
func slowStoppingRecorderBinary(t *testing.T) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "fake-slow-recorder")
	script := "#!/bin/sh\nfor last in \"$@\"; do :; done\n" +
		"trap 'sleep 0.3; printf recorded > \"$last\"; exit 0' INT\nsleep 60 &\nwait $!\n"
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatal(err)
	}
	return path
}

// TestPresentRecorderStateReadsThroughASegmentSwitch covers ruling 2:
// State() and Blocked() must never wait behind a segment switch, since
// StartSegment/StopSegment stop the previous recorder synchronously and can
// take a while.
func TestPresentRecorderStateReadsThroughASegmentSwitch(t *testing.T) {
	feed := &screenFeed{screens: laptopScreens}
	outputDir := filepath.Join(t.TempDir(), "recordings")
	present := newPresentRecorder(presentRecorderOptions{
		Run: recorder.RunOptions{
			Parent:    outputDir,
			DeckTitle: "My Talk",
			Base:      recorder.Options{Command: slowStoppingRecorderBinary(t), NoAudio: true},
			Chapters:  true,
		},
		OutputDir:    outputDir,
		ListScreens:  feed.list,
		FreeSpace:    func(string) (uint64, error) { return 50 << 30, nil },
		PollInterval: 20 * time.Millisecond,
		DiskInterval: 20 * time.Millisecond,
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	present.Begin(ctx, true)
	waitForState(t, present, tui.PresentRecording)

	// Toggle stops the running segment, which this recorder takes 300ms to
	// finalize. State() must read through that switch instead of waiting
	// behind the same lock.
	go func() {
		_ = present.Toggle()
	}()
	time.Sleep(20 * time.Millisecond)

	done := make(chan tui.PresentRecordingState, 1)
	go func() { done <- present.State() }()

	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("State() did not return within 200ms of a segment switch in progress")
	}
	_, _ = present.Finish(true)
}
