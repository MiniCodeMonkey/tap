package tui

import (
	"fmt"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/MiniCodeMonkey/tap/internal/recorder"
)

// RecorderController is the slice of recording the dev TUI needs. The CLI
// supplies the real one; the output paths, the chapter list and the
// screencapture process live there, so this file stays about keys and
// rendering.
type RecorderController interface {
	Available() bool
	Displays() ([]recorder.Display, error)
	DefaultAudioInput() string
	Preflight() recorder.Report
	Start(display int) (string, error)
	Stop() (recorder.Result, error)
	Test(display int) error
	Recording() bool
	Elapsed() time.Duration
}

// recordMsg reports the outcome of a start or stop to the update loop.
type recordMsg struct {
	path    string
	err     error
	result  recorder.Result
	stopped bool
}

// SetRecorderController gives the TUI a way to record the talk.
func (m *DevModel) SetRecorderController(controller RecorderController) {
	m.recorders = controller
}

// NoteRecordingEnded is called from outside the update loop when the
// recorder stopped without being asked to. The state it changes is read
// and written all over the update loop, so the change is handed to that
// loop as a message rather than written here.
func (m *DevModel) NoteRecordingEnded(err error) {
	select {
	case m.recordEndedCh <- err:
	default:
	}
}

// toggleRecording is the C key: start a recording, or stop the running one.
func (m *DevModel) toggleRecording() (*DevModel, tea.Cmd) {
	if m.recorders == nil {
		return m, nil
	}

	if !m.recorders.Available() {
		m.addEvent(DevEvent{
			Type:      "error",
			Message:   "Recording is macOS only",
			Timestamp: time.Now(),
		})
		return m, nil
	}

	if m.recording {
		return m, m.stopRecordingCmd()
	}

	report := m.recorders.Preflight()
	for _, finding := range report.Findings {
		eventType := "action"
		if finding.Blocking {
			eventType = "error"
		}
		message := finding.Message
		if finding.Fix != "" {
			message += ". " + finding.Fix
		}
		m.addEvent(DevEvent{Type: eventType, Message: message, Timestamp: time.Now()})
	}
	if report.Blocked() {
		return m, nil
	}

	displays, err := m.recorders.Displays()
	if err != nil {
		m.addEvent(DevEvent{
			Type:      "error",
			Message:   "Cannot list displays: " + err.Error(),
			Timestamp: time.Now(),
		})
		return m, nil
	}

	// One display is not a choice, so it is not worth a screen.
	if len(displays) <= 1 {
		return m, m.startRecordingCmd(1)
	}

	m.recordDisplays = displays
	m.recordPickerIndex = preselectedDisplayIndex(displays, m.config.RecordDisplay)
	m.showRecordPicker = true
	return m, nil
}

// preselectedDisplayIndex is where the picker opens: the deck's configured
// display when it exists, and the main display otherwise.
func preselectedDisplayIndex(displays []recorder.Display, configured int) int {
	for position, display := range displays {
		if display.Index == configured {
			return position
		}
	}
	return 0
}

// startRecordingCmd starts the recorder off the update loop.
func (m *DevModel) startRecordingCmd(display int) tea.Cmd {
	controller := m.recorders
	return func() tea.Msg {
		path, err := controller.Start(display)
		return recordMsg{path: path, err: err}
	}
}

// stopRecordingCmd stops the recorder off the update loop.
func (m *DevModel) stopRecordingCmd() tea.Cmd {
	controller := m.recorders
	return func() tea.Msg {
		result, err := controller.Stop()
		return recordMsg{result: result, err: err, stopped: true}
	}
}

// applyRecordMsg folds a start or stop outcome back into the model.
func (m *DevModel) applyRecordMsg(msg recordMsg) *DevModel {
	if msg.err != nil {
		m.recording = false
		m.addEvent(DevEvent{
			Type:      "error",
			Message:   "Recording failed: " + msg.err.Error(),
			Timestamp: time.Now(),
		})
		return m
	}

	if msg.stopped {
		m.recording = false
		m.recordWarned = false
		message := fmt.Sprintf("Recording saved → %s (%s)", msg.result.Path, formatElapsed(msg.result.Duration))
		if msg.result.Truncated {
			message += ", and it may be incomplete: the recorder had to be killed"
		}
		m.addEvent(DevEvent{Type: "action", Message: message, Timestamp: time.Now()})
		return m
	}

	m.recording = true
	m.recordWarned = false
	m.recordingPath = msg.path
	m.recordingStartedAt = time.Now()
	m.addEvent(DevEvent{
		Type:      "action",
		Message:   "Recording started → " + msg.path,
		Timestamp: time.Now(),
	})
	return m
}

// confirmQuitWhileRecording reports whether q should ask before quitting.
// screencapture cannot resume into a file it has closed, so a mis-keyed q
// during a talk would be unrecoverable.
func (m *DevModel) confirmQuitWhileRecording() bool {
	return m.recording
}

// handleQuitConfirmKey drives the confirmation q opens while recording.
func (m *DevModel) handleQuitConfirmKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y":
		m.showQuitConfirm = false
		m.quitting = true

		if _, err := m.recorders.Stop(); err != nil {
			m.addEvent(DevEvent{
				Type:      "error",
				Message:   "Recording failed to stop: " + err.Error(),
				Timestamp: time.Now(),
			})
		}
		m.recording = false

		return m, tea.Quit

	case "n", "N", "esc":
		m.showQuitConfirm = false
		return m, nil
	}

	return m, nil
}

// viewQuitConfirm renders the confirmation overlay.
func (m *DevModel) viewQuitConfirm() string {
	return "\n" + RenderTitle("Recording in progress") + "\n\n" +
		RenderMuted("  "+filepath.Base(m.recordingPath)+"  "+formatElapsed(time.Since(m.recordingStartedAt))) + "\n\n" +
		"  Stop recording and quit? (y/n)\n"
}

// recordingTick is the once-a-second check on a running recording: it
// warns about one that has run long, and stops one that has run away.
// Both thresholds come from the deck.
func (m *DevModel) recordingTick() tea.Cmd {
	if !m.recording {
		return nil
	}

	elapsed := time.Since(m.recordingStartedAt)

	if m.config.RecordStopAfter > 0 && elapsed >= m.config.RecordStopAfter {
		m.addEvent(DevEvent{
			Type:      "action",
			Message:   "Recording stopped at the " + formatElapsed(m.config.RecordStopAfter) + " cap",
			Timestamp: time.Now(),
		})
		return m.stopRecordingCmd()
	}

	if !m.recordWarned && m.config.RecordWarnAfter > 0 && elapsed >= m.config.RecordWarnAfter {
		m.recordWarned = true
		m.addEvent(DevEvent{
			Type:      "error",
			Message:   "Tap is still recording after " + formatElapsed(elapsed) + ". Press c to stop.",
			Timestamp: time.Now(),
		})
	}

	return nil
}

// viewRecordingStatus is the status block's recording line, or "" when
// nothing is being recorded.
func (m *DevModel) viewRecordingStatus() string {
	if !m.recording {
		return ""
	}
	return fmt.Sprintf("● %s  %s", formatElapsed(time.Since(m.recordingStartedAt)), filepath.Base(m.recordingPath))
}

// formatElapsed renders a duration as m:ss, or h:mm:ss past an hour.
func formatElapsed(elapsed time.Duration) string {
	totalSeconds := int(elapsed.Seconds())
	hours := totalSeconds / 3600
	minutes := (totalSeconds % 3600) / 60
	seconds := totalSeconds % 60

	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, seconds)
	}
	return fmt.Sprintf("%d:%02d", minutes, seconds)
}

// handleRecordPickerKey drives the record picker overlay.
func (m *DevModel) handleRecordPickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.showRecordPicker = false
		return m, nil

	case "up", "k":
		if m.recordPickerIndex > 0 {
			m.recordPickerIndex--
		}
		return m, nil

	case "down", "j":
		if m.recordPickerIndex < len(m.recordDisplays)-1 {
			m.recordPickerIndex++
		}
		return m, nil

	case "t":
		// A test stays in the picker, so a wrong screen can be corrected
		// and tested again without starting over.
		return m, m.testRecordingCmd(m.selectedDisplay())

	case "enter":
		display := m.selectedDisplay()
		m.showRecordPicker = false
		return m, m.startRecordingCmd(display)
	}

	return m, nil
}

// selectedDisplay is the screencapture index the picker is sitting on.
func (m *DevModel) selectedDisplay() int {
	if m.recordPickerIndex < 0 || m.recordPickerIndex >= len(m.recordDisplays) {
		return 1
	}
	return m.recordDisplays[m.recordPickerIndex].Index
}

// testRecordingCmd runs a short test capture off the update loop.
func (m *DevModel) testRecordingCmd(display int) tea.Cmd {
	controller := m.recorders
	return func() tea.Msg {
		if err := controller.Test(display); err != nil {
			return devEventMsg{event: DevEvent{
				Type:      "error",
				Message:   "Test capture failed: " + err.Error(),
				Timestamp: time.Now(),
			}}
		}
		return devEventMsg{event: DevEvent{
			Type:      "action",
			Message:   "Test capture opened. Watch it and listen for your microphone.",
			Timestamp: time.Now(),
		}}
	}
}

// displayLabel names a display for the picker. A display with no name is
// shown by index: an unlabeled screen is better than a wrongly labeled one.
func displayLabel(display recorder.Display) string {
	label := display.Name
	if label == "" {
		label = fmt.Sprintf("Display %d", display.Index)
	}
	if display.Resolution != "" {
		label += "   " + display.Resolution
	}
	if display.Main {
		label += "   main"
	}
	return label
}

// viewRecordPicker renders the record picker overlay.
func (m *DevModel) viewRecordPicker() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(RenderTitle("Record"))
	b.WriteString("\n\n")

	b.WriteString(RenderSubtitle("Display"))
	b.WriteString("\n")
	for position, display := range m.recordDisplays {
		if position == m.recordPickerIndex {
			b.WriteString(RenderSuccess("  > " + displayLabel(display)))
		} else {
			b.WriteString(RenderMuted("    " + displayLabel(display)))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(RenderSubtitle("Audio"))
	b.WriteString("\n")

	// The microphone is shown, not chosen: screencapture selects an input
	// by CoreAudio UID, and a pure-Go binary cannot enumerate UIDs. Seeing
	// the wrong microphone here is the point.
	input := m.recorders.DefaultAudioInput()
	if input == "" {
		input = "no input device"
	}
	b.WriteString(RenderMuted("    " + input + " (system default input)"))
	b.WriteString("\n")
	b.WriteString(RenderMuted("    Change it in System Settings or the menu bar."))
	b.WriteString("\n\n")

	b.WriteString(RenderMuted("  enter start    t test 5s    esc cancel"))
	b.WriteString("\n")

	return b.String()
}
