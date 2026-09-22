package tui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// PresentRecordingState is what a tap present run is doing right now.
type PresentRecordingState int

const (
	// PresentNotRecording: no segment is being written.
	PresentNotRecording PresentRecordingState = iota
	// PresentRecording: a segment is being written.
	PresentRecording
	// PresentPaused: the projector went away and the run waits for it.
	PresentPaused
)

// PresentRecorder is the slice of a tap present run the TUI needs. The CLI
// owns the run, the display poll and the disk guard.
type PresentRecorder interface {
	State() PresentRecordingState
	// Elapsed is how long the current segment has run.
	Elapsed() time.Duration
	// Toggle stops a recording or a pause, or starts a new segment.
	Toggle() error
	// Started reports whether the run has written anything.
	Started() bool
	LeftFirstSlide() bool
	// Blocked explains why recording cannot run at all, or is "".
	Blocked() string
	SuggestGitignore() string
	AddGitignoreEntry() error
}

// presentToggleMsg reports the outcome of c in present mode.
type presentToggleMsg struct {
	err error
}

// reloadMsg reports the outcome of r.
type reloadMsg struct {
	err error
}

// SetPresentRecorder gives the TUI the run to show and control.
func (m *DevModel) SetPresentRecorder(recorder PresentRecorder) {
	m.presentRecorder = recorder
}

// SetReloader gives r a way to reload the deck from disk.
func (m *DevModel) SetReloader(reload func() error) {
	m.reload = reload
}

// KeepRecording is the speaker's answer to the keep prompt. It is true
// unless they pressed n.
func (m *DevModel) KeepRecording() bool {
	return !m.discardRecording
}

// reloadCmd reloads off the update loop: loading a deck bundles its
// components and can take a moment.
func (m *DevModel) reloadCmd() tea.Cmd {
	reload := m.reload
	if reload == nil {
		return nil
	}
	return func() tea.Msg {
		return reloadMsg{err: reload()}
	}
}

// applyReloadMsg reports a manual reload.
func (m *DevModel) applyReloadMsg(msg reloadMsg) {
	if msg.err != nil {
		m.SetError(msg.err)
		return
	}
	m.ClearError()
	m.addEvent(DevEvent{Type: "reload", Message: "Reloaded the deck", Timestamp: time.Now()})
}

// togglePresentRecordingCmd runs c off the update loop: stopping a segment
// waits for screencapture to finalize the file, which can take a few
// seconds, and calling Toggle inside Update would freeze the TUI for that
// long.
func (m *DevModel) togglePresentRecordingCmd() tea.Cmd {
	recorder := m.presentRecorder
	if recorder == nil {
		return nil
	}
	return func() tea.Msg {
		return presentToggleMsg{err: recorder.Toggle()}
	}
}

// presentQuit is q in present mode: a run worth keeping gets the keep
// prompt; anything else quits, and the CLI deletes a run that never left
// the first slide.
func (m *DevModel) presentQuit() (tea.Model, tea.Cmd) {
	if m.presentRecorder != nil && m.presentRecorder.Started() && m.presentRecorder.LeftFirstSlide() {
		m.showKeepPrompt = true
		return m, nil
	}
	m.quitting = true
	return m, tea.Quit
}

// handleKeepPromptKey drives the keep prompt. Only n deletes; every other
// way out of the prompt that quits keeps the recording.
func (m *DevModel) handleKeepPromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter", "ctrl+c":
		m.showKeepPrompt = false
		m.discardRecording = false
		if msg.String() != "ctrl+c" && m.presentRecorder != nil {
			if entry := m.presentRecorder.SuggestGitignore(); entry != "" {
				m.gitignoreSuggestion = entry
				m.showGitignorePrompt = true
				m.quitAfterGitignore = true
				return m, nil
			}
		}
		m.quitting = true
		return m, tea.Quit

	case "n", "N":
		m.showKeepPrompt = false
		m.discardRecording = true
		m.quitting = true
		return m, tea.Quit

	case "esc":
		m.showKeepPrompt = false
		return m, nil
	}
	return m, nil
}

// viewKeepPrompt renders the keep prompt.
func (m *DevModel) viewKeepPrompt() string {
	return "\n" + RenderTitle("Quit tap present") + "\n\n" +
		"  Keep this recording? (Y/n)\n\n" +
		RenderMuted("  esc goes back to the talk") + "\n"
}

// viewPresentRecording is the recording state, drawn large enough to read
// at a glance while setting up.
func (m *DevModel) viewPresentRecording() string {
	block := lipgloss.NewStyle().Bold(true).Padding(1, 3).Border(lipgloss.ThickBorder())

	if m.presentRecorder == nil {
		return block.BorderForeground(ColorWarning).Foreground(ColorWarning).Render("NOT RECORDING")
	}
	if reason := m.presentRecorder.Blocked(); reason != "" {
		return block.BorderForeground(ColorError).Foreground(ColorError).Render("NOT RECORDING\n\n" + reason)
	}

	switch m.presentRecorder.State() {
	case PresentRecording:
		return block.BorderForeground(ColorError).Foreground(ColorError).
			Render(fmt.Sprintf("● REC %s", formatElapsed(m.presentRecorder.Elapsed())))
	case PresentPaused:
		return block.BorderForeground(ColorWarning).Foreground(ColorWarning).Render("PAUSED, waiting for projector")
	default:
		return block.BorderForeground(ColorWarning).Foreground(ColorWarning).Render("NOT RECORDING")
	}
}

// handlePresentKey is the present mode key map: the keys that are safe
// during a talk, and none that write to the deck.
func (m *DevModel) handlePresentKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q":
		return m.presentQuit()
	case "ctrl+c":
		m.quitting = true
		return m, tea.Quit
	case "o":
		m.addEvent(DevEvent{Type: "action", Message: "Opening the slides...", Timestamp: time.Now()})
		return m, openBrowserCmd(m.config.AudienceURL)
	case "p":
		m.addEvent(DevEvent{Type: "action", Message: "Opening presenter view...", Timestamp: time.Now()})
		return m, openBrowserCmd(m.config.PresenterURL)
	case "r":
		return m, m.reloadCmd()
	case "u":
		return m.toggleTunnel()
	case "c":
		return m, m.togglePresentRecordingCmd()
	}
	return m, nil
}
