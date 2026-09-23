package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
)

// appPresentRecorderForTest is a real tap present run with the fake
// recorder binary, a screen list and free space that the test sets, and
// its disk level reported to disk.
func appPresentRecorderForTest(t *testing.T, feed *screenFeed, free *atomic.Uint64, disk *appDiskStatus) *presentRecorder {
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
		FreeSpace:    func(string) (uint64, error) { return free.Load(), nil },
		OnDiskLevel:  disk.set,
		PollInterval: 20 * time.Millisecond,
		DiskInterval: 20 * time.Millisecond,
	})
}

// startPresentSession runs the --app control loop over present, which
// starts recording from launch, as a consent of yes does.
func startPresentSession(t *testing.T, present *presentRecorder, disk *appDiskStatus) *sessionHarness {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return startAppSessionForTest(t, func(options *appSessionOptions) {
		options.Present = present
		options.DiskStatus = disk.get
		options.Startup = func(context.Context) { present.Begin(ctx, true) }
	})
}

func recordingStateIs(state string, segment int) func(map[string]any) bool {
	return func(event map[string]any) bool {
		return event["state"] == state && event["segment"] == float64(segment)
	}
}

func TestAppPresentFollowsTheProjectorAndAsksToKeep(t *testing.T) {
	feed := &screenFeed{screens: projectorScreens}
	var free atomic.Uint64
	free.Store(50 << 30)
	disk := &appDiskStatus{}
	present := appPresentRecorderForTest(t, feed, &free, disk)
	harness := startPresentSession(t, present, disk)

	harness.log.nextWhere(t, appEventRecording, recordingStateIs("recording", 1))
	feed.set(laptopScreens)
	harness.log.nextWhere(t, appEventRecording, recordingStateIs("paused", 1))
	feed.set(projectorScreens)
	harness.log.nextWhere(t, appEventRecording, recordingStateIs("recording", 2))

	present.NoteSlideChange(1)
	harness.send(appCommand{Type: appCommandQuit})
	question := harness.log.next(t, appEventQuestion)
	payload, _ := question["payload"].(map[string]any)
	if question["kind"] != appQuestionKeepRecording || payload["directory"] != present.Dir() || payload["segments"] != float64(2) {
		t.Errorf("question = %v", question)
	}
	harness.answer(t, question, "true")
	harness.waitForEnd(t)
	if _, err := os.Stat(present.Dir()); err != nil {
		t.Errorf("the kept recording is gone: %v", err)
	}
	harness.log.nextWhere(t, appEventRecording, recordingStateIs("stopped", 2))
}

func TestAppPresentStopsRecordingWhenTheDiskFills(t *testing.T) {
	feed := &screenFeed{screens: laptopScreens}
	var free atomic.Uint64
	free.Store(50 << 30)
	disk := &appDiskStatus{}
	present := appPresentRecorderForTest(t, feed, &free, disk)
	harness := startPresentSession(t, present, disk)

	harness.log.nextWhere(t, appEventRecording, recordingStateIs("recording", 1))
	free.Store(100 << 20)
	harness.log.nextWhere(t, appEventRecording, func(event map[string]any) bool {
		return event["state"] == "stopped" && event["disk"] == "full"
	})

	harness.send(appCommand{Type: appCommandRecording, Action: "new-segment"})
	event := harness.log.next(t, appEventError)
	if event["code"] != appErrorRecordingFailed || !strings.Contains(event["message"].(string), "disk full") {
		t.Errorf("event = %v, want the disk guard's refusal", event)
	}
}

func TestAppPresentRecordingCommandsDriveTheRun(t *testing.T) {
	feed := &screenFeed{screens: laptopScreens}
	var free atomic.Uint64
	free.Store(50 << 30)
	disk := &appDiskStatus{}
	present := appPresentRecorderForTest(t, feed, &free, disk)
	harness := startPresentSession(t, present, disk)

	harness.log.nextWhere(t, appEventRecording, recordingStateIs("recording", 1))
	harness.send(appCommand{Type: appCommandRecording, Action: "stop"})
	harness.log.nextWhere(t, appEventRecording, recordingStateIs("stopped", 1))
	harness.send(appCommand{Type: appCommandRecording, Action: "new-segment"})
	harness.log.nextWhere(t, appEventRecording, recordingStateIs("recording", 2))

	present.NoteSlideChange(2)
	folder := present.Dir()
	harness.send(appCommand{Type: appCommandQuit})
	harness.answer(t, harness.log.next(t, appEventQuestion), "false")
	harness.waitForEnd(t)
	if _, err := os.Stat(folder); !os.IsNotExist(err) {
		t.Errorf("a recording the app said no to is still on disk (stat error %v)", err)
	}
}
