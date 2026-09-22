package recorder

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

type slideState struct {
	mu    sync.Mutex
	index int
	known bool
}

func (s *slideState) current() (int, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.index, s.known
}

func testRun(t *testing.T, command string, slides *slideState) *Run {
	t.Helper()
	// Each reading of the clock is one second after the last, so chapter
	// order never depends on how fast the test runs.
	var clockMu sync.Mutex
	clock := time.Date(2026, 9, 21, 19, 32, 0, 0, time.UTC)
	return NewRun(RunOptions{
		Now: func() time.Time {
			clockMu.Lock()
			defer clockMu.Unlock()
			clock = clock.Add(time.Second)
			return clock
		},
		Parent:       filepath.Join(t.TempDir(), "recordings"),
		DeckTitle:    "My Talk",
		Base:         Options{Command: command, NoAudio: true},
		Chapters:     true,
		CurrentSlide: slides.current,
		TitleFor:     func(slideIndex int) string { return []string{"Title", "Agenda", "Demo"}[slideIndex] },
	})
}

func readChapters(t *testing.T, run *Run) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(run.Dir(), "chapters.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}

func TestRunNumbersSegmentsInOneFolder(t *testing.T) {
	run := testRun(t, obedientRecorder(t), &slideState{known: true})

	first, err := run.StartSegment(1)
	if err != nil {
		t.Fatal(err)
	}
	waitForRecorderReady(t, first)
	second, err := run.StartSegment(2)
	if err != nil {
		t.Fatal(err)
	}
	waitForRecorderReady(t, second)

	if filepath.Base(first) != "01.mov" || filepath.Base(second) != "02.mov" {
		t.Errorf("segments = %s, %s; want 01.mov, 02.mov", filepath.Base(first), filepath.Base(second))
	}
	if filepath.Dir(first) != filepath.Dir(second) {
		t.Errorf("segments landed in different folders")
	}
	if raw, _ := os.ReadFile(first); string(raw) != "recorded" {
		t.Errorf("the first segment was not finalized when the second started")
	}
	if _, err := run.Finish(true); err != nil {
		t.Fatal(err)
	}
}

func TestRunChapterListHasASectionPerSegment(t *testing.T) {
	slides := &slideState{known: true}
	run := testRun(t, obedientRecorder(t), slides)

	first, _ := run.StartSegment(1)
	waitForRecorderReady(t, first)
	second, _ := run.StartSegment(2)
	waitForRecorderReady(t, second)
	run.NoteSlide(1)

	chapters := readChapters(t, run)
	if !strings.HasPrefix(chapters, "01.mov\n0:00 Title\n02.mov\n0:00 Title\n") {
		t.Errorf("chapters.txt =\n%s", chapters)
	}
	if !strings.Contains(chapters, "Talk starts\n") || !strings.HasSuffix(chapters, "Agenda\n") {
		t.Errorf("chapters.txt misses the talk start or the agenda:\n%s", chapters)
	}
	_, _ = run.Finish(true)
}

func TestRunSeedsTheFirstChapterOnTheTitleSlideWhenUnknown(t *testing.T) {
	slides := &slideState{}
	run := testRun(t, obedientRecorder(t), slides)

	first, _ := run.StartSegment(1)
	waitForRecorderReady(t, first)

	chapters := readChapters(t, run)
	if chapters != "01.mov\n0:00 Title\n" {
		t.Errorf("chapters.txt = %q, want a title chapter seeded from the unknown slide", chapters)
	}

	run.NoteSlide(1)
	chapters = readChapters(t, run)
	if !strings.Contains(chapters, "Talk starts\n") {
		t.Errorf("chapters.txt misses the talk start after an unknown slide:\n%s", chapters)
	}
	_, _ = run.Finish(true)
}

func TestRunMovesTheTalkStartToTheLastDeparture(t *testing.T) {
	slides := &slideState{known: true}
	run := testRun(t, obedientRecorder(t), slides)

	first, _ := run.StartSegment(1)
	waitForRecorderReady(t, first)
	run.NoteSlide(1)
	run.NoteSlide(0)
	run.NoteSlide(1)
	run.NoteSlide(2)

	chapters := readChapters(t, run)
	if strings.Count(chapters, "Talk starts") != 1 {
		t.Fatalf("want one talk start mark:\n%s", chapters)
	}
	lines := strings.Split(strings.TrimSpace(chapters), "\n")
	// 01.mov, Title, Agenda, Title, Talk starts, Agenda, Demo
	if len(lines) != 7 || !strings.HasSuffix(lines[4], "Talk starts") {
		t.Errorf("the mark is not at the last departure from the title:\n%s", chapters)
	}
	_, _ = run.Finish(true)
}

func TestRunDeletesARunThatNeverLeftTheFirstSlide(t *testing.T) {
	run := testRun(t, obedientRecorder(t), &slideState{known: true})

	path, _ := run.StartSegment(1)
	waitForRecorderReady(t, path)

	summary, err := run.Finish(true)
	if err != nil {
		t.Fatal(err)
	}
	if !summary.Deleted || !summary.NeverLeftFirstSlide {
		t.Errorf("summary = %+v, want deleted because it never left the first slide", summary)
	}
	if _, err := os.Stat(run.Dir()); !os.IsNotExist(err) {
		t.Errorf("the run folder still exists")
	}
}

func TestRunKeepsOrDeletesOnRequest(t *testing.T) {
	for _, keep := range []bool{true, false} {
		run := testRun(t, obedientRecorder(t), &slideState{known: true})
		path, _ := run.StartSegment(1)
		waitForRecorderReady(t, path)
		run.NoteSlide(1)

		summary, err := run.Finish(keep)
		if err != nil {
			t.Fatal(err)
		}
		_, statErr := os.Stat(run.Dir())
		if keep && (summary.Deleted || statErr != nil) {
			t.Errorf("keep=true deleted the run")
		}
		if !keep && (!summary.Deleted || !os.IsNotExist(statErr)) {
			t.Errorf("keep=false kept the run")
		}
	}
}

func TestRunFinishWithoutASegmentDoesNothing(t *testing.T) {
	run := testRun(t, obedientRecorder(t), &slideState{})

	summary, err := run.Finish(true)
	if err != nil || summary != (RunSummary{}) {
		t.Errorf("Finish = %+v, %v; want an empty summary", summary, err)
	}
}

type exitReport struct {
	err error
	ran time.Duration
}

func TestRunReportsASegmentThatStopsOnItsOwn(t *testing.T) {
	exited := make(chan exitReport, 1)
	run := NewRun(RunOptions{
		Parent:        t.TempDir(),
		DeckTitle:     "My Talk",
		Base:          Options{Command: writeFakeRecorder(t, "exit 3\n"), NoAudio: true},
		OnSegmentExit: func(err error, ran time.Duration) { exited <- exitReport{err, ran} },
	})

	if _, err := run.StartSegment(1); err != nil {
		t.Fatal(err)
	}
	select {
	case report := <-exited:
		if report.err == nil {
			t.Error("want the exit error")
		}
		if report.ran <= 0 || report.ran >= 5*time.Second {
			t.Errorf("ran = %v, want a positive duration under 5s", report.ran)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("OnSegmentExit was never called")
	}
	if run.Recording() {
		t.Error("the run still reports recording")
	}
}

func TestRunDoesNotReportAStopItAskedFor(t *testing.T) {
	exited := make(chan error, 1)
	run := NewRun(RunOptions{
		Parent:        t.TempDir(),
		DeckTitle:     "My Talk",
		Base:          Options{Command: obedientRecorder(t), NoAudio: true},
		OnSegmentExit: func(err error, ran time.Duration) { exited <- err },
	})

	path, _ := run.StartSegment(1)
	waitForRecorderReady(t, path)
	if err := run.StopSegment(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-exited:
		t.Error("OnSegmentExit fired for a requested stop")
	case <-time.After(200 * time.Millisecond):
	}
}

func TestNoteSlideDoesNotWaitForASlowStop(t *testing.T) {
	previousGrace := killGrace
	killGrace = 2 * time.Second
	t.Cleanup(func() { killGrace = previousGrace })

	// This recorder ignores SIGINT, so stopping it blocks for killGrace
	// while it is killed.
	stubborn := writeFakeRecorder(t, "trap '' INT\nprintf ready > \"$last.ready\"\nsleep 60 &\nwait $!\n")
	slides := &slideState{known: true}
	run := testRun(t, stubborn, slides)

	first, err := run.StartSegment(1)
	if err != nil {
		t.Fatal(err)
	}
	waitForRecorderReady(t, first)

	started := make(chan struct{})
	go func() {
		_, _ = run.StartSegment(2)
		close(started)
	}()

	// Give StartSegment(2) time to switch r.current and begin stopping the
	// slow first segment before NoteSlide is called.
	time.Sleep(50 * time.Millisecond)

	before := time.Now()
	run.NoteSlide(1)
	if elapsed := time.Since(before); elapsed >= time.Second {
		t.Errorf("NoteSlide took %v while a segment was stopping, want well under killGrace", elapsed)
	}

	<-started
	_, _ = run.Finish(false)
}

func TestRunCountsItsSegments(t *testing.T) {
	run := testRun(t, obedientRecorder(t), &slideState{known: true})
	if run.Segments() != 0 {
		t.Errorf("Segments() = %d before any segment, want 0", run.Segments())
	}
	first, err := run.StartSegment(1)
	if err != nil {
		t.Fatal(err)
	}
	waitForRecorderReady(t, first)
	second, err := run.StartSegment(1)
	if err != nil {
		t.Fatal(err)
	}
	waitForRecorderReady(t, second)
	if run.Segments() != 2 {
		t.Errorf("Segments() = %d, want 2", run.Segments())
	}
	if _, err := run.Finish(true); err != nil {
		t.Fatal(err)
	}
}
