package tui

import (
	"fmt"
	"path/filepath"
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
