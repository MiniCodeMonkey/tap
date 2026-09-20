package tui

import (
	"errors"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
)

// fakeRecorder stands in for the CLI's controller.
type fakeRecorder struct {
	displays    []recorder.Display
	report      recorder.Report
	startErr    error
	path        string
	available   bool
	recording   bool
	startCalls  int
	stopCalls   int
	testCalls   int
	lastDisplay int
}

func (f *fakeRecorder) Available() bool { return f.available }

func (f *fakeRecorder) Displays() ([]recorder.Display, error) { return f.displays, nil }

func (f *fakeRecorder) DefaultAudioInput() string { return "MacBook Pro Microphone" }

func (f *fakeRecorder) Preflight() recorder.Report { return f.report }

func (f *fakeRecorder) Start(display int) (string, error) {
	f.startCalls++
	f.lastDisplay = display
	if f.startErr != nil {
		return "", f.startErr
	}
	f.recording = true
	return f.path, nil
}

func (f *fakeRecorder) Stop() (recorder.Result, error) {
	f.stopCalls++
	f.recording = false
	return recorder.Result{Path: f.path, Duration: 90 * time.Second}, nil
}

func (f *fakeRecorder) Test(display int) error {
	f.testCalls++
	f.lastDisplay = display
	return nil
}

func (f *fakeRecorder) Recording() bool { return f.recording }

func (f *fakeRecorder) Elapsed() time.Duration { return 0 }

func oneDisplay() []recorder.Display {
	return []recorder.Display{{Index: 1, Name: "Color LCD", Resolution: "2294 x 1432", Main: true}}
}

func twoDisplays() []recorder.Display {
	return []recorder.Display{
		{Index: 1, Name: "Color LCD", Resolution: "2294 x 1432", Main: true},
		{Index: 2, Name: "DELL U2720Q", Resolution: "3840 x 2160"},
	}
}

func pressC(t *testing.T, m *DevModel) tea.Cmd {
	t.Helper()
	_, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'c'}})
	return cmd
}

func TestRecordKeyStartsOnASingleDisplay(t *testing.T) {
	fake := &fakeRecorder{available: true, displays: oneDisplay(), path: "recordings/talk.mov"}
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(fake)

	cmd := pressC(t, m)
	if cmd == nil {
		t.Fatal("pressing c returned no command, so nothing would start")
	}
	if m.showRecordPicker {
		t.Error("the picker opened for a single display, want it skipped")
	}

	msg, ok := cmd().(recordMsg)
	if !ok {
		t.Fatalf("command returned %T, want recordMsg", cmd())
	}
	if msg.err != nil {
		t.Fatalf("recordMsg carries %v", msg.err)
	}
	if fake.startCalls != 1 || fake.lastDisplay != 1 {
		t.Errorf("Start called %d times with display %d, want once with 1", fake.startCalls, fake.lastDisplay)
	}
}

func TestRecordKeyOpensThePickerForTwoDisplays(t *testing.T) {
	fake := &fakeRecorder{available: true, displays: twoDisplays()}
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(fake)

	pressC(t, m)

	if !m.showRecordPicker {
		t.Fatal("the picker did not open for two displays")
	}
	if fake.startCalls != 0 {
		t.Error("Start was called before a display was chosen")
	}
}

func TestRecordKeyStopsARunningRecording(t *testing.T) {
	fake := &fakeRecorder{available: true, displays: oneDisplay(), recording: true, path: "recordings/talk.mov"}
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(fake)
	m.recording = true

	cmd := pressC(t, m)
	if cmd == nil {
		t.Fatal("pressing c while recording returned no command")
	}

	msg, ok := cmd().(recordMsg)
	if !ok {
		t.Fatalf("command returned %T, want recordMsg", cmd())
	}
	if !msg.stopped {
		t.Error("recordMsg is not marked stopped")
	}
	if fake.stopCalls != 1 {
		t.Errorf("Stop called %d times, want once", fake.stopCalls)
	}
}

func TestRecordKeyRefusesWhenPreflightBlocks(t *testing.T) {
	fake := &fakeRecorder{
		available: true,
		displays:  oneDisplay(),
		report: recorder.Report{Findings: []recorder.Finding{{
			Message:  "Screen Recording permission is missing",
			Fix:      "Grant it in System Settings, then restart the terminal",
			Blocking: true,
		}}},
	}
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(fake)

	pressC(t, m)

	if fake.startCalls != 0 {
		t.Error("Start was called although the preflight blocked")
	}
	if !strings.Contains(eventText(m), "Screen Recording") {
		t.Errorf("the blocking finding is not in the events: %s", eventText(m))
	}
	if !strings.Contains(eventText(m), "restart the terminal") {
		t.Errorf("the fix is not in the events: %s", eventText(m))
	}
}

func TestRecordKeyIsInertWithoutAController(t *testing.T) {
	m := NewDevModel(DevConfig{})

	if cmd := pressC(t, m); cmd != nil {
		t.Error("pressing c without a controller returned a command")
	}
}

func TestRecordKeyReportsAnUnsupportedPlatform(t *testing.T) {
	fake := &fakeRecorder{available: false}
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(fake)

	pressC(t, m)

	if !strings.Contains(eventText(m), "macOS") {
		t.Errorf("the events do not say recording is macOS only: %s", eventText(m))
	}
}

func TestStatusShowsARunningRecording(t *testing.T) {
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(&fakeRecorder{available: true, recording: true})
	m.recording = true
	m.recordingPath = "recordings/talk.mov"
	m.recordingStartedAt = time.Now().Add(-92 * time.Second)

	status := m.viewStatus()
	if !strings.Contains(status, "1:32") {
		t.Errorf("the status block does not show the elapsed time: %s", status)
	}
	if !strings.Contains(status, "talk.mov") {
		t.Errorf("the status block does not show the output path: %s", status)
	}
}

func TestHelpMentionsTheRecordKey(t *testing.T) {
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(&fakeRecorder{available: true})

	if !strings.Contains(m.viewHelp(), "record") {
		t.Errorf("the help line does not mention recording: %s", m.viewHelp())
	}
}

func TestHelpOmitsTheRecordKeyWhenUnsupported(t *testing.T) {
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(&fakeRecorder{available: false})

	if strings.Contains(m.viewHelp(), "record") {
		t.Errorf("the help line offers recording on a platform that cannot: %s", m.viewHelp())
	}
}

func TestApplyRecordMsgReportsAFailedStart(t *testing.T) {
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(&fakeRecorder{available: true})

	m.applyRecordMsg(recordMsg{err: errors.New("starting the recorder: exec format error")})

	if m.recording {
		t.Error("the model believes it is recording after a failed start")
	}
	if !strings.Contains(eventText(m), "exec format error") {
		t.Errorf("the failure is not in the events: %s", eventText(m))
	}
}

// eventText joins the model's recent event messages for assertions.
func eventText(m *DevModel) string {
	var parts []string
	for _, event := range m.state.RecentEvents {
		parts = append(parts, event.Message)
	}
	return strings.Join(parts, "\n")
}
