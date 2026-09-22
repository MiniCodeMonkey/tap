package cli

import (
	"context"
	"sync/atomic"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
	"github.com/MiniCodeMonkey/tap/internal/tui"
)

// appRecordingInterval is how often --app mode samples the recording.
const appRecordingInterval = 500 * time.Millisecond

// appPresentControl is the part of a tap present run that --app mode
// reports and drives. *presentRecorder is the real one. Following the
// projector and guarding the disk stay inside it, so the app decides
// nothing about recording.
type appPresentControl interface {
	State() tui.PresentRecordingState
	Elapsed() time.Duration
	Toggle() error
	Started() bool
	LeftFirstSlide() bool
	Blocked() string
	Segment() int
	Dir() string
	Finish(keep bool) (recorder.RunSummary, error)
}

var _ appPresentControl = (*presentRecorder)(nil)

// appDiskStatus is the last disk level the run reported, by name.
type appDiskStatus struct {
	status atomic.Value
}

func (disk *appDiskStatus) set(level recorder.DiskLevel) {
	name := diskStatusName(level)
	if name == "" {
		name = "ok"
	}
	disk.status.Store(name)
}

// get is "ok", "low" or "full".
func (disk *appDiskStatus) get() string {
	if name, ok := disk.status.Load().(string); ok {
		return name
	}
	return "ok"
}

// recordingStateName names a recording state for the app.
func recordingStateName(state tui.PresentRecordingState) string {
	switch state {
	case tui.PresentRecording:
		return "recording"
	case tui.PresentPaused:
		return "paused"
	default:
		return "stopped"
	}
}

// sampleRecording is the recording event for the run as it is now.
func sampleRecording(present appPresentControl, diskStatus func() string) appRecordingEvent {
	return appRecordingEvent{
		Type:    appEventRecording,
		State:   recordingStateName(present.State()),
		Segment: present.Segment(),
		Elapsed: int(present.Elapsed() / time.Second),
		Disk:    diskStatus(),
	}
}

// recordingReporter sends a recording event whenever the state, the
// segment or the disk status changes. The elapsed time rides along, and
// the app counts it up between events. Only one goroutine calls report.
type recordingReporter struct {
	present    appPresentControl
	diskStatus func() string
	events     *appEventWriter
	last       appRecordingEvent
	reported   bool
}

func newRecordingReporter(present appPresentControl, diskStatus func() string, events *appEventWriter) *recordingReporter {
	return &recordingReporter{present: present, diskStatus: diskStatus, events: events}
}

// report sends the recording state when it changed since the last report.
func (reporter *recordingReporter) report() {
	current := sampleRecording(reporter.present, reporter.diskStatus)
	if reporter.reported && current.State == reporter.last.State && current.Segment == reporter.last.Segment && current.Disk == reporter.last.Disk {
		return
	}
	reporter.last, reporter.reported = current, true
	reporter.events.emit(current)
}

// run reports now, and then every interval until ctx ends.
func (reporter *recordingReporter) run(ctx context.Context, interval time.Duration) {
	reporter.report()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			reporter.report()
		}
	}
}
