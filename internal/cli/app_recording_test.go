package cli

import (
	"sync"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
	"github.com/MiniCodeMonkey/tap/internal/tui"
)

// fakePresent is a tap present run for tests: Toggle stops a recording, or
// starts the next segment.
type fakePresent struct {
	mu sync.Mutex
	// finishBlock, when set, holds Finish until it closes. It is not
	// taken under mu: a run whose Finish is slow does not lock out the
	// rest of the interface.
	finishBlock    chan struct{}
	toggleErr      error
	dir            string
	state          tui.PresentRecordingState
	segment        int
	started        bool
	leftFirstSlide bool
	finished       bool
	kept           bool
}

var _ appPresentControl = (*fakePresent)(nil)

func (present *fakePresent) State() tui.PresentRecordingState {
	present.mu.Lock()
	defer present.mu.Unlock()
	return present.state
}

func (present *fakePresent) Elapsed() time.Duration { return 3 * time.Second }

func (present *fakePresent) Toggle() error {
	present.mu.Lock()
	defer present.mu.Unlock()
	if present.toggleErr != nil {
		return present.toggleErr
	}
	if present.state != tui.PresentNotRecording {
		present.state = tui.PresentNotRecording
		return nil
	}
	present.state = tui.PresentRecording
	present.segment++
	present.started = true
	return nil
}

func (present *fakePresent) Started() bool {
	present.mu.Lock()
	defer present.mu.Unlock()
	return present.started
}

func (present *fakePresent) LeftFirstSlide() bool {
	present.mu.Lock()
	defer present.mu.Unlock()
	return present.leftFirstSlide
}

func (present *fakePresent) Blocked() string { return "" }

func (present *fakePresent) Segment() int {
	present.mu.Lock()
	defer present.mu.Unlock()
	return present.segment
}

func (present *fakePresent) Dir() string { return present.dir }

func (present *fakePresent) Finish(keep bool) (recorder.RunSummary, error) {
	if present.finishBlock != nil {
		<-present.finishBlock
	}
	present.mu.Lock()
	defer present.mu.Unlock()
	present.finished, present.kept = true, keep
	present.state = tui.PresentNotRecording
	return recorder.RunSummary{Dir: present.dir, Segments: present.segment, Deleted: !keep}, nil
}

func (present *fakePresent) set(change func(*fakePresent)) {
	present.mu.Lock()
	defer present.mu.Unlock()
	change(present)
}

func (present *fakePresent) finishedWith() (finished, kept bool) {
	present.mu.Lock()
	defer present.mu.Unlock()
	return present.finished, present.kept
}

func TestRecordingReporterReportsOnlyChanges(t *testing.T) {
	events, log := newTestEvents(t)
	present := &fakePresent{}
	disk := &appDiskStatus{}
	reporter := newRecordingReporter(present, disk.get, events)

	reporter.report()
	first := log.next(t, appEventRecording)
	if first["state"] != "stopped" || first["segment"] != float64(0) || first["disk"] != "ok" {
		t.Errorf("first = %v", first)
	}

	reporter.report()
	present.set(func(present *fakePresent) { present.state, present.segment = tui.PresentRecording, 1 })
	reporter.report()
	second := log.next(t, appEventRecording)
	if second["state"] != "recording" || second["segment"] != float64(1) || second["elapsed"] != float64(3) {
		t.Errorf("second = %v; an unchanged state must send nothing", second)
	}

	disk.set(recorder.DiskLow)
	reporter.report()
	if third := log.next(t, appEventRecording); third["disk"] != "low" {
		t.Errorf("third = %v, want disk low", third)
	}

	present.set(func(present *fakePresent) { present.state = tui.PresentPaused })
	reporter.report()
	if fourth := log.next(t, appEventRecording); fourth["state"] != "paused" {
		t.Errorf("fourth = %v, want paused", fourth)
	}
}

func TestAppDiskStatusNames(t *testing.T) {
	disk := &appDiskStatus{}
	for _, test := range []struct {
		level recorder.DiskLevel
		want  string
	}{
		{recorder.DiskFull, "full"},
		{recorder.DiskLow, "low"},
		{recorder.DiskOK, "ok"},
	} {
		disk.set(test.level)
		if got := disk.get(); got != test.want {
			t.Errorf("level %v: %q, want %q", test.level, got, test.want)
		}
	}
}
