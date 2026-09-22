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
	displays            []recorder.Display
	report              recorder.Report
	startErr            error
	path                string
	gitignoreSuggestion string
	available           bool
	recording           bool
	startCalls          int
	stopCalls           int
	testCalls           int
	lastDisplay         int
	gitignoreCalls      int
	// stopBlock, when set, makes Stop wait for it to close before
	// returning: the real controller's Stop blocks for up to killGrace
	// while screencapture finalizes the file, and every fake before this
	// one returned instantly, which hid the runaway-cap bug from every
	// test built on it.
	stopBlock chan struct{}
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
	if f.stopBlock != nil {
		<-f.stopBlock
	}
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

func (f *fakeRecorder) SuggestGitignore() string { return f.gitignoreSuggestion }

func (f *fakeRecorder) AddGitignoreEntry() error { f.gitignoreCalls++; return nil }

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

// readyMsg runs pressC's command (the preflight/displays/audio probe) and
// returns the recordPickerReadyMsg it produces, off the update loop exactly
// as tea would deliver it.
func readyMsg(t *testing.T, m *DevModel) recordPickerReadyMsg {
	t.Helper()

	cmd := pressC(t, m)
	if cmd == nil {
		t.Fatal("pressing c returned no command")
	}
	msg, ok := cmd().(recordPickerReadyMsg)
	if !ok {
		t.Fatalf("command returned %T, want recordPickerReadyMsg", cmd())
	}
	return msg
}

func TestRecordKeyStartsOnASingleDisplay(t *testing.T) {
	fake := &fakeRecorder{available: true, displays: oneDisplay(), path: "recordings/talk.mov"}
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(fake)

	msg := readyMsg(t, m)
	_, cmd := m.applyRecordPickerReady(msg)
	if cmd == nil {
		t.Fatal("the ready message returned no command, so nothing would start")
	}
	if m.showRecordPicker {
		t.Error("the picker opened for a single display, want it skipped")
	}

	recMsg, ok := cmd().(recordMsg)
	if !ok {
		t.Fatalf("command returned %T, want recordMsg", cmd())
	}
	if recMsg.err != nil {
		t.Fatalf("recordMsg carries %v", recMsg.err)
	}
	if fake.startCalls != 1 || fake.lastDisplay != 1 {
		t.Errorf("Start called %d times with display %d, want once with 1", fake.startCalls, fake.lastDisplay)
	}
}

func TestRecordKeyOpensThePickerForTwoDisplays(t *testing.T) {
	fake := &fakeRecorder{available: true, displays: twoDisplays()}
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(fake)

	msg := readyMsg(t, m)
	m.applyRecordPickerReady(msg)

	if !m.showRecordPicker {
		t.Fatal("the picker did not open for two displays")
	}
	if fake.startCalls != 0 {
		t.Error("Start was called before a display was chosen")
	}
}

func TestRecordKeyIsBusyUntilThePreflightReplies(t *testing.T) {
	fake := &fakeRecorder{available: true, displays: oneDisplay()}
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(fake)

	if cmd := pressC(t, m); cmd == nil {
		t.Fatal("the first c returned no command")
	}

	// A second C landing before the preflight's reply (recordPickerReadyMsg)
	// must not run the whole probe again: that used to relaunch three
	// subprocesses and, once it also reached Start, reset
	// recordingStartedAt.
	if cmd := pressC(t, m); cmd != nil {
		t.Error("a second c before the preflight replied issued another command")
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

	msg := readyMsg(t, m)
	m.applyRecordPickerReady(msg)

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

func recordingModel(t *testing.T, cfg DevConfig, fake *fakeRecorder) *DevModel {
	t.Helper()

	m := NewDevModel(cfg)
	m.SetRecorderController(fake)
	m.recording = true
	m.recordingPath = "recordings/talk.mov"
	m.recordingStartedAt = time.Now()
	return m
}

func TestQuitAsksWhileRecording(t *testing.T) {
	fake := &fakeRecorder{available: true, recording: true}
	m := recordingModel(t, DevConfig{}, fake)

	_, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if cmd != nil {
		t.Error("q quit immediately while recording")
	}
	if !m.showQuitConfirm {
		t.Fatal("q did not open the confirmation")
	}
	if m.quitting {
		t.Error("the model is quitting although the confirmation is open")
	}
	if !strings.Contains(m.View(), "Stop recording and quit?") {
		t.Errorf("the confirmation is not on screen:\n%s", m.View())
	}
}

func TestQuitConfirmNoReturnsToTheTUI(t *testing.T) {
	fake := &fakeRecorder{available: true, recording: true}
	m := recordingModel(t, DevConfig{}, fake)
	m.showQuitConfirm = true

	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if m.showQuitConfirm {
		t.Error("n left the confirmation open")
	}
	if m.quitting {
		t.Error("n quit anyway")
	}
	if fake.stopCalls != 0 {
		t.Error("n stopped the recording")
	}
}

func TestQuitConfirmYesStopsAndQuits(t *testing.T) {
	fake := &fakeRecorder{available: true, recording: true}
	m := recordingModel(t, DevConfig{}, fake)
	m.showQuitConfirm = true

	_, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if cmd == nil {
		t.Fatal("y returned no command, so nothing quits")
	}
	if fake.stopCalls != 1 {
		t.Errorf("Stop called %d times, want once", fake.stopCalls)
	}
	if !m.quitting {
		t.Error("the model is not quitting after y")
	}
}

func TestQuitConfirmCtrlCStopsAndQuitsWithoutAsking(t *testing.T) {
	fake := &fakeRecorder{available: true, recording: true}
	m := recordingModel(t, DevConfig{}, fake)
	m.showQuitConfirm = true

	_, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyCtrlC})

	if cmd == nil {
		t.Fatal("ctrl+c returned no command, so nothing quits")
	}
	if fake.stopCalls != 1 {
		t.Errorf("Stop called %d times, want once", fake.stopCalls)
	}
	if !m.quitting {
		t.Error("the model is not quitting after ctrl+c")
	}
	if m.showQuitConfirm {
		t.Error("the confirmation is still open after ctrl+c")
	}
}

func TestQuitDoesNotAskWhenNothingIsRecording(t *testing.T) {
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(&fakeRecorder{available: true})

	_, cmd := m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})

	if cmd == nil {
		t.Error("q did not quit with no recording running")
	}
	if m.showQuitConfirm {
		t.Error("q asked about a recording that is not running")
	}
}

func TestTickWarnsOnceAboutALongRecording(t *testing.T) {
	fake := &fakeRecorder{available: true, recording: true}
	m := recordingModel(t, DevConfig{RecordWarnAfter: time.Minute, RecordStopAfter: time.Hour}, fake)
	m.recordingStartedAt = time.Now().Add(-2 * time.Minute)

	m.recordingTick()
	m.recordingTick()

	warnings := strings.Count(eventText(m), "still recording")
	if warnings != 1 {
		t.Errorf("the long-recording warning appeared %d times, want once", warnings)
	}
}

func TestTickStopsARunawayRecording(t *testing.T) {
	fake := &fakeRecorder{available: true, recording: true}
	m := recordingModel(t, DevConfig{RecordWarnAfter: time.Minute, RecordStopAfter: 30 * time.Minute}, fake)
	m.recordingStartedAt = time.Now().Add(-31 * time.Minute)

	cmd := m.recordingTick()
	if cmd == nil {
		t.Fatal("the cap did not stop the recording")
	}
	cmd()

	if fake.stopCalls != 1 {
		t.Errorf("Stop called %d times, want once", fake.stopCalls)
	}
}

// TestRunawayCapIssuesExactlyOneStopWhileItIsSlow closes the structural gap
// that let the runaway cap fire a Stop every tick: every fake used
// elsewhere in this file returns from Stop instantly, so nothing exercised
// what happens while a real Stop is still finalizing the file. It reproduces
// that by blocking Stop on a channel and driving several ticks past the cap
// before letting it return, the way the TUI's own tick loop would while
// screencapture takes its time.
func TestRunawayCapIssuesExactlyOneStopWhileItIsSlow(t *testing.T) {
	block := make(chan struct{})
	fake := &fakeRecorder{available: true, recording: true, stopBlock: block, path: "recordings/talk.mov"}
	m := recordingModel(t, DevConfig{RecordWarnAfter: time.Minute, RecordStopAfter: 30 * time.Minute}, fake)
	m.recordingStartedAt = time.Now().Add(-31 * time.Minute)

	cmd := m.recordingTick()
	if cmd == nil {
		t.Fatal("the cap did not stop the recording")
	}

	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()

	// Several ticks land while the first Stop is still blocked finalizing
	// the file. Before recordBusy, each one saw m.recording still true and
	// issued another Stop.
	for i := 0; i < 5; i++ {
		if extra := m.recordingTick(); extra != nil {
			t.Fatalf("tick %d issued another Stop while one was still in flight", i)
		}
	}

	close(block)

	select {
	case msg := <-done:
		recMsg, ok := msg.(recordMsg)
		if !ok {
			t.Fatalf("command returned %T, want recordMsg", msg)
		}
		m.applyRecordMsg(recMsg)
	case <-time.After(5 * time.Second):
		t.Fatal("Stop never returned")
	}

	if fake.stopCalls != 1 {
		t.Errorf("Stop called %d times, want exactly one", fake.stopCalls)
	}
}

func TestTickLeavesTheCapOffWhenItIsZero(t *testing.T) {
	fake := &fakeRecorder{available: true, recording: true}
	m := recordingModel(t, DevConfig{RecordWarnAfter: time.Minute, RecordStopAfter: 0}, fake)
	m.recordingStartedAt = time.Now().Add(-10 * time.Hour)

	if cmd := m.recordingTick(); cmd != nil {
		t.Error("a disabled cap stopped the recording")
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

func openPicker(t *testing.T, fake *fakeRecorder) *DevModel {
	t.Helper()

	m := NewDevModel(DevConfig{})
	m.SetRecorderController(fake)
	msg := readyMsg(t, m)
	m.applyRecordPickerReady(msg)

	if !m.showRecordPicker {
		t.Fatal("the picker did not open")
	}
	return m
}

func pressInPicker(t *testing.T, m *DevModel, key string) tea.Cmd {
	t.Helper()

	var msg tea.KeyMsg
	switch key {
	case "enter":
		msg = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		msg = tea.KeyMsg{Type: tea.KeyEsc}
	case "down":
		msg = tea.KeyMsg{Type: tea.KeyDown}
	default:
		msg = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}

	_, cmd := m.handleKeyPress(msg)
	return cmd
}

func TestPickerStartsTheChosenDisplay(t *testing.T) {
	fake := &fakeRecorder{available: true, displays: twoDisplays(), path: "recordings/talk.mov"}
	m := openPicker(t, fake)

	pressInPicker(t, m, "down")
	cmd := pressInPicker(t, m, "enter")

	if cmd == nil {
		t.Fatal("enter in the picker returned no command")
	}
	if _, ok := cmd().(recordMsg); !ok {
		t.Fatalf("command returned %T, want recordMsg", cmd())
	}
	if fake.lastDisplay != 2 {
		t.Errorf("Start used display %d, want 2", fake.lastDisplay)
	}
	if m.showRecordPicker {
		t.Error("the picker is still open after starting")
	}
}

func TestPickerEscapeStartsNothing(t *testing.T) {
	fake := &fakeRecorder{available: true, displays: twoDisplays()}
	m := openPicker(t, fake)

	pressInPicker(t, m, "esc")

	if m.showRecordPicker {
		t.Error("escape left the picker open")
	}
	if fake.startCalls != 0 {
		t.Error("escape started a recording")
	}
}

func TestPickerTestKeyRunsATestAndStaysOpen(t *testing.T) {
	fake := &fakeRecorder{available: true, displays: twoDisplays()}
	m := openPicker(t, fake)

	pressInPicker(t, m, "down")
	cmd := pressInPicker(t, m, "t")

	if cmd == nil {
		t.Fatal("t in the picker returned no command")
	}
	cmd()

	if fake.testCalls != 1 {
		t.Errorf("Test called %d times, want once", fake.testCalls)
	}
	if fake.lastDisplay != 2 {
		t.Errorf("Test used display %d, want the selected 2", fake.lastDisplay)
	}
	if !m.showRecordPicker {
		t.Error("the picker closed after a test, want it open so the choice can change")
	}
	if fake.startCalls != 0 {
		t.Error("the test started a real recording")
	}
}

func TestPickerSelectionStopsAtTheEnds(t *testing.T) {
	m := openPicker(t, &fakeRecorder{available: true, displays: twoDisplays()})

	pressInPicker(t, m, "down")
	pressInPicker(t, m, "down")

	if m.recordPickerIndex != 1 {
		t.Errorf("recordPickerIndex = %d, want it held at the last entry", m.recordPickerIndex)
	}
}

func TestPickerShowsDisplaysAndTheAudioInput(t *testing.T) {
	m := openPicker(t, &fakeRecorder{available: true, displays: twoDisplays()})

	view := m.viewRecordPicker()
	for _, want := range []string{"Color LCD", "DELL U2720Q", "3840 x 2160", "main", "MacBook Pro Microphone", "System Settings"} {
		if !strings.Contains(view, want) {
			t.Errorf("the picker does not show %q:\n%s", want, view)
		}
	}
}

func TestPickerNamesDisplaysByIndexWhenNamesAreMissing(t *testing.T) {
	displays := []recorder.Display{{Index: 1, Main: true}, {Index: 2}}
	m := openPicker(t, &fakeRecorder{available: true, displays: displays})

	view := m.viewRecordPicker()
	if !strings.Contains(view, "Display 1") || !strings.Contains(view, "Display 2") {
		t.Errorf("the picker does not fall back to plain indexes:\n%s", view)
	}
}

func TestNoteRecordingEndedClearsTheState(t *testing.T) {
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(&fakeRecorder{available: true})
	m.recording = true
	m.recordingPath = "recordings/talk.mov"

	m.NoteRecordingEnded(errors.New("exit status 3"))

	msg := m.listenForEvents()()
	ended, ok := msg.(recordEndedMsg)
	if !ok {
		t.Fatalf("listenForEvents returned %T, want recordEndedMsg", msg)
	}

	m.Update(ended)

	if m.recording {
		t.Error("the model still believes it is recording")
	}
	if !strings.Contains(eventText(m), "exit status 3") {
		t.Errorf("the unexpected exit is not in the events: %s", eventText(m))
	}
}

func TestStoppingOffersTheGitignoreEntry(t *testing.T) {
	fake := &fakeRecorder{available: true, gitignoreSuggestion: "recordings/"}
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(fake)
	m.recording = true

	m.applyRecordMsg(recordMsg{stopped: true, result: recorder.Result{Path: "recordings/talk.mov"}})

	if !m.showGitignorePrompt {
		t.Fatal("stopping did not offer the .gitignore entry")
	}
	if !strings.Contains(m.View(), "recordings/") {
		t.Errorf("the prompt does not name the entry:\n%s", m.View())
	}
}

func TestStoppingIsQuietWhenNothingToIgnore(t *testing.T) {
	fake := &fakeRecorder{available: true, gitignoreSuggestion: ""}
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(fake)
	m.recording = true

	m.applyRecordMsg(recordMsg{stopped: true, result: recorder.Result{Path: "recordings/talk.mov"}})

	if m.showGitignorePrompt {
		t.Error("the prompt appeared outside a git repository")
	}
}

func TestGitignorePromptYesAddsTheEntry(t *testing.T) {
	fake := &fakeRecorder{available: true, gitignoreSuggestion: "recordings/"}
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(fake)
	m.showGitignorePrompt = true

	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if fake.gitignoreCalls != 1 {
		t.Errorf("AddGitignoreEntry called %d times, want once", fake.gitignoreCalls)
	}
	if m.showGitignorePrompt {
		t.Error("the prompt stayed open after y")
	}
}

func TestGitignorePromptMessageNamesTheActualEntry(t *testing.T) {
	fake := &fakeRecorder{available: true, gitignoreSuggestion: "captures/"}
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(fake)
	m.recording = true

	// The suggestion is captured when the prompt opens, the way the real
	// flow does it: SuggestGitignore may name "captures/" or
	// "talks/2026/recordings/", never the hardcoded "recordings/" the
	// confirmation message used to print regardless.
	m.applyRecordMsg(recordMsg{stopped: true, result: recorder.Result{Path: "captures/talk.mov"}})

	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})

	if !strings.Contains(eventText(m), "Added captures/ to .gitignore") {
		t.Errorf("the confirmation does not name the actual entry: %s", eventText(m))
	}
}

func TestDiskFullStopsTheRecordingInTheTUI(t *testing.T) {
	model := NewDevModel(DevConfig{})
	model.recording = true

	updated, _ := model.Update(diskLevelMsg{level: recorder.DiskFull})
	devModel := updated.(*DevModel)

	if devModel.recording {
		t.Error("the TUI still shows recording")
	}
	last := devModel.state.RecentEvents[len(devModel.state.RecentEvents)-1]
	if last.Type != "error" || !strings.Contains(last.Message, "Recording stopped: disk full") {
		t.Errorf("last event = %+v", last)
	}
}

func TestDiskLowWarnsInTheTUI(t *testing.T) {
	model := NewDevModel(DevConfig{})
	model.recording = true

	updated, _ := model.Update(diskLevelMsg{level: recorder.DiskLow})
	devModel := updated.(*DevModel)

	if !devModel.recording {
		t.Error("a low disk stopped the recording")
	}
	last := devModel.state.RecentEvents[len(devModel.state.RecentEvents)-1]
	if !strings.Contains(last.Message, "Disk almost full, recording stops at 1 GB") {
		t.Errorf("last event = %+v", last)
	}
}

func TestGitignorePromptNoChangesNothing(t *testing.T) {
	fake := &fakeRecorder{available: true, gitignoreSuggestion: "recordings/"}
	m := NewDevModel(DevConfig{})
	m.SetRecorderController(fake)
	m.showGitignorePrompt = true

	m.handleKeyPress(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})

	if fake.gitignoreCalls != 0 {
		t.Error("n wrote to .gitignore")
	}
	if m.showGitignorePrompt {
		t.Error("the prompt stayed open after n")
	}
}
