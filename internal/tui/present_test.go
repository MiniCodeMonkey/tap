package tui

import (
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

type fakePresentRecorder struct {
	state          PresentRecordingState
	blocked        string
	gitignore      string
	started        bool
	leftFirstSlide bool
	toggles        int
}

func (f *fakePresentRecorder) State() PresentRecordingState { return f.state }
func (f *fakePresentRecorder) Elapsed() time.Duration       { return 724 * time.Second }
func (f *fakePresentRecorder) Toggle() error                { f.toggles++; return nil }
func (f *fakePresentRecorder) Started() bool                { return f.started }
func (f *fakePresentRecorder) LeftFirstSlide() bool         { return f.leftFirstSlide }
func (f *fakePresentRecorder) Blocked() string              { return f.blocked }
func (f *fakePresentRecorder) SuggestGitignore() string     { return f.gitignore }
func (f *fakePresentRecorder) AddGitignoreEntry() error     { return nil }

func presentModel(recorder *fakePresentRecorder) *DevModel {
	model := NewDevModel(DevConfig{Present: true, MarkdownFile: "talk.md"})
	model.SetPresentRecorder(recorder)
	return model
}

func press(model *DevModel, key string) (*DevModel, tea.Cmd) {
	var message tea.KeyMsg
	switch key {
	case "enter":
		message = tea.KeyMsg{Type: tea.KeyEnter}
	case "esc":
		message = tea.KeyMsg{Type: tea.KeyEsc}
	case "ctrl+c":
		message = tea.KeyMsg{Type: tea.KeyCtrlC}
	default:
		message = tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(key)}
	}
	updated, command := model.Update(message)
	return updated.(*DevModel), command
}

func TestPresentModeShowsTheRecordingStateLarge(t *testing.T) {
	cases := map[PresentRecordingState]string{
		PresentRecording:    "● REC 12:04",
		PresentNotRecording: "NOT RECORDING",
		PresentPaused:       "PAUSED, waiting for projector",
	}
	for state, want := range cases {
		view := presentModel(&fakePresentRecorder{state: state}).View()
		if !strings.Contains(view, want) {
			t.Errorf("state %v: view lacks %q", state, want)
		}
		if !strings.Contains(view, "PRESENTING") {
			t.Errorf("state %v: header lacks PRESENTING", state)
		}
	}
}

func TestPresentModeShowsWhyRecordingIsBlocked(t *testing.T) {
	view := presentModel(&fakePresentRecorder{blocked: "Screen Recording permission is missing"}).View()
	if !strings.Contains(view, "Screen Recording permission is missing") {
		t.Error("the blocking reason is not shown")
	}
}

func TestPresentModeUnbindsTheEditingKeys(t *testing.T) {
	for _, key := range []string{"a", "i", "t", "e"} {
		model, command := press(presentModel(&fakePresentRecorder{}), key)
		if model.showSlideBuilder || model.showImageGenerator || model.showThemePicker || model.exportingPDF || command != nil {
			t.Errorf("key %q did something in present mode", key)
		}
	}
}

func TestPresentModeHelpListsOnlyTheKeptKeys(t *testing.T) {
	help := presentModel(&fakePresentRecorder{}).viewHelp()
	for _, kept := range []string{"open slides", "presenter view", "reload", "tunnel", "quit"} {
		if !strings.Contains(help, kept) {
			t.Errorf("help lacks %q", kept)
		}
	}
	for _, dropped := range []string{"add slide", "image", "theme", "export pdf"} {
		if strings.Contains(help, dropped) {
			t.Errorf("help still lists %q", dropped)
		}
	}
}

func TestPresentModeCTogglesTheRecording(t *testing.T) {
	recorder := &fakePresentRecorder{}
	press(presentModel(recorder), "c")
	if recorder.toggles != 1 {
		t.Errorf("toggles = %d, want 1", recorder.toggles)
	}
}

func TestPresentQuitWithoutARealRunQuitsAtOnce(t *testing.T) {
	model, command := press(presentModel(&fakePresentRecorder{started: true}), "q")
	if model.showKeepPrompt || command == nil {
		t.Error("q asked about a run that never left the first slide")
	}
}

func TestPresentKeepPrompt(t *testing.T) {
	cases := []struct {
		key      string
		keep     bool
		quits    bool
		stillAsk bool
	}{
		{key: "enter", keep: true, quits: true},
		{key: "y", keep: true, quits: true},
		{key: "Y", keep: true, quits: true},
		{key: "ctrl+c", keep: true, quits: true},
		{key: "n", keep: false, quits: true},
		{key: "N", keep: false, quits: true},
		{key: "esc", keep: true, quits: false},
		{key: "x", keep: true, quits: false, stillAsk: true},
	}
	for _, testCase := range cases {
		model, _ := press(presentModel(&fakePresentRecorder{started: true, leftFirstSlide: true}), "q")
		if !model.showKeepPrompt || !strings.Contains(model.View(), "Keep this recording? (Y/n)") {
			t.Fatalf("q did not open the keep prompt")
		}
		model, command := press(model, testCase.key)
		if model.KeepRecording() != testCase.keep {
			t.Errorf("%s: keep = %v, want %v", testCase.key, model.KeepRecording(), testCase.keep)
		}
		if model.quitting != testCase.quits || (command != nil) != testCase.quits {
			t.Errorf("%s: quitting = %v, want %v", testCase.key, model.quitting, testCase.quits)
		}
		if model.showKeepPrompt != testCase.stillAsk {
			t.Errorf("%s: prompt open = %v, want %v", testCase.key, model.showKeepPrompt, testCase.stillAsk)
		}
	}
}

func TestPresentKeepThenGitignorePromptThenQuit(t *testing.T) {
	model, _ := press(presentModel(&fakePresentRecorder{started: true, leftFirstSlide: true, gitignore: "recordings/"}), "q")
	model, command := press(model, "y")
	if !model.showGitignorePrompt || command != nil {
		t.Fatal("keeping inside a git repository did not ask about .gitignore first")
	}
	model, command = press(model, "n")
	if !model.quitting || command == nil {
		t.Error("answering the .gitignore prompt did not quit")
	}
}

func TestReloadKeyCallsTheReloader(t *testing.T) {
	calls := 0
	model := presentModel(&fakePresentRecorder{})
	model.SetReloader(func() error { calls++; return nil })

	_, command := press(model, "r")
	if command == nil {
		t.Fatal("r returned no command")
	}
	command()
	if calls != 1 {
		t.Errorf("reloader calls = %d, want 1", calls)
	}
}
