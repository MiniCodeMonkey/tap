package tui

import (
	"os"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewDevModel(t *testing.T) {
	cfg := DevConfig{
		AudienceURL:       "http://localhost:3000",
		PresenterURL:      "http://localhost:3000/presenter",
		Port:              3000,
		PresenterPassword: "secret",
		MarkdownFile:      "slides.md",
	}

	model := NewDevModel(cfg)

	if model.config.AudienceURL != cfg.AudienceURL {
		t.Errorf("expected AudienceURL %q, got %q", cfg.AudienceURL, model.config.AudienceURL)
	}
	if model.config.PresenterURL != cfg.PresenterURL {
		t.Errorf("expected PresenterURL %q, got %q", cfg.PresenterURL, model.config.PresenterURL)
	}
	if model.config.Port != cfg.Port {
		t.Errorf("expected Port %d, got %d", cfg.Port, model.config.Port)
	}
	if model.config.PresenterPassword != cfg.PresenterPassword {
		t.Errorf("expected PresenterPassword %q, got %q", cfg.PresenterPassword, model.config.PresenterPassword)
	}
	if model.config.MarkdownFile != cfg.MarkdownFile {
		t.Errorf("expected MarkdownFile %q, got %q", cfg.MarkdownFile, model.config.MarkdownFile)
	}
	if model.quitting {
		t.Error("model should not be quitting initially")
	}
}

func TestDevModel_HandleKeyPress_Quit(t *testing.T) {
	tests := []struct {
		name string
		key  string
	}{
		{"q key", "q"},
		{"ctrl+c", "ctrl+c"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := NewDevModel(DevConfig{})

			msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tt.key)}
			if tt.key == "ctrl+c" {
				msg = tea.KeyMsg{Type: tea.KeyCtrlC}
			}

			newModel, cmd := model.Update(msg)
			m := newModel.(*DevModel)

			if !m.quitting {
				t.Errorf("expected quitting to be true after %s", tt.key)
			}
			if cmd == nil {
				t.Error("expected quit command to be returned")
			}
		})
	}
}

func TestDevModel_HandleKeyPress_Open(t *testing.T) {
	model := NewDevModel(DevConfig{
		AudienceURL: "http://localhost:3000",
	})

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("o")}
	newModel, cmd := model.Update(msg)
	m := newModel.(*DevModel)

	if m.quitting {
		t.Error("model should not be quitting after 'o' key")
	}
	if cmd == nil {
		t.Error("expected open browser command to be returned")
	}

	// Check that an event was added
	m.mu.RLock()
	events := m.state.RecentEvents
	m.mu.RUnlock()

	found := false
	for _, e := range events {
		if strings.Contains(e.Message, "Opening browser") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Opening browser' event to be added")
	}
}

func TestDevModel_HandleKeyPress_Presenter(t *testing.T) {
	model := NewDevModel(DevConfig{
		PresenterURL: "http://localhost:3000/presenter",
	})

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p")}
	newModel, cmd := model.Update(msg)
	m := newModel.(*DevModel)

	if m.quitting {
		t.Error("model should not be quitting after 'p' key")
	}
	if cmd == nil {
		t.Error("expected open browser command to be returned")
	}

	// Check that an event was added
	m.mu.RLock()
	events := m.state.RecentEvents
	m.mu.RUnlock()

	found := false
	for _, e := range events {
		if strings.Contains(e.Message, "presenter view") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'presenter view' event to be added")
	}
}

func TestDevModel_HandleKeyPress_Add(t *testing.T) {
	model := NewDevModel(DevConfig{})

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")}
	newModel, _ := model.Update(msg)
	m := newModel.(*DevModel)

	if m.quitting {
		t.Error("model should not be quitting after 'a' key")
	}

	// Check that an event was added
	m.mu.RLock()
	events := m.state.RecentEvents
	m.mu.RUnlock()

	found := false
	for _, e := range events {
		if strings.Contains(e.Message, "slide builder") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'slide builder' event to be added")
	}
}

func TestDevModel_HandleKeyPress_Reload(t *testing.T) {
	model := NewDevModel(DevConfig{})

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")}
	newModel, _ := model.Update(msg)
	m := newModel.(*DevModel)

	if m.quitting {
		t.Error("model should not be quitting after 'r' key")
	}

	// Check that an event was added
	m.mu.RLock()
	events := m.state.RecentEvents
	m.mu.RUnlock()

	found := false
	for _, e := range events {
		if strings.Contains(e.Message, "Manual reload") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'Manual reload' event to be added")
	}
}

func TestDevModel_WindowSize(t *testing.T) {
	model := NewDevModel(DevConfig{})

	msg := tea.WindowSizeMsg{Width: 120, Height: 40}
	newModel, _ := model.Update(msg)
	m := newModel.(*DevModel)

	if m.windowWidth != 120 {
		t.Errorf("expected windowWidth 120, got %d", m.windowWidth)
	}
	if m.windowHeight != 40 {
		t.Errorf("expected windowHeight 40, got %d", m.windowHeight)
	}
}

func TestDevModel_AddEvent(t *testing.T) {
	model := NewDevModel(DevConfig{})

	// Add multiple events
	for i := 0; i < 10; i++ {
		model.addEvent(DevEvent{
			Timestamp: time.Now(),
			Type:      "test",
			Message:   "test event",
		})
	}

	// Should only keep last 5 events
	model.mu.RLock()
	eventCount := len(model.state.RecentEvents)
	model.mu.RUnlock()

	if eventCount != 5 {
		t.Errorf("expected 5 events, got %d", eventCount)
	}
}

func TestDevModel_View_Basic(t *testing.T) {
	model := NewDevModel(DevConfig{
		AudienceURL:  "http://localhost:3000",
		PresenterURL: "http://localhost:3000/presenter",
		MarkdownFile: "slides.md",
	})
	model.windowWidth = 80
	model.windowHeight = 24

	view := model.View()

	// Check that essential elements are present
	if !strings.Contains(view, "Dev Server") {
		t.Error("view should contain 'Dev Server'")
	}
	if !strings.Contains(view, "slides.md") {
		t.Error("view should contain the markdown file name")
	}
	if !strings.Contains(view, "localhost:3000") {
		t.Error("view should contain the server URL")
	}
	if !strings.Contains(view, "quit") {
		t.Error("view should contain help text")
	}
}

func TestDevModel_View_WithPassword(t *testing.T) {
	model := NewDevModel(DevConfig{
		AudienceURL:       "http://localhost:3000",
		PresenterURL:      "http://localhost:3000/presenter?key=secret",
		PresenterPassword: "secret",
		MarkdownFile:      "slides.md",
	})
	model.windowWidth = 80
	model.windowHeight = 24

	view := model.View()

	if !strings.Contains(view, "password protected") {
		t.Error("view should indicate password protection")
	}
}

func TestDevModel_View_WithQRCode(t *testing.T) {
	model := NewDevModel(DevConfig{
		AudienceURL:  "http://localhost:3000",
		PresenterURL: "http://localhost:3000/presenter",
		MarkdownFile: "slides.md",
		QRCodeASCII:  "██████\n██  ██\n██████",
	})
	model.windowWidth = 80
	model.windowHeight = 40 // Tall enough to show QR

	view := model.View()

	if !strings.Contains(view, "████") {
		t.Error("view should contain QR code when window is tall enough")
	}
}

func TestDevModel_View_Quitting(t *testing.T) {
	model := NewDevModel(DevConfig{})
	model.quitting = true

	view := model.View()

	if !strings.Contains(view, "Shutting down") {
		t.Error("view should show shutdown message when quitting")
	}
}

func TestDevModel_View_WithError(t *testing.T) {
	model := NewDevModel(DevConfig{
		MarkdownFile: "slides.md",
	})
	model.windowWidth = 80
	model.windowHeight = 24
	model.state.Error = &mockError{msg: "test error message"}

	view := model.View()

	if !strings.Contains(view, "test error message") {
		t.Error("view should display error message")
	}
}

type mockError struct {
	msg string
}

func (e *mockError) Error() string {
	return e.msg
}

func TestDevModel_View_WithEvents(t *testing.T) {
	model := NewDevModel(DevConfig{
		MarkdownFile: "slides.md",
	})
	model.windowWidth = 80
	model.windowHeight = 24
	model.addEvent(DevEvent{
		Timestamp: time.Now(),
		Type:      "reload",
		Message:   "File changed: slides.md",
	})

	view := model.View()

	if !strings.Contains(view, "File changed") {
		t.Error("view should display recent events")
	}
}

func TestDevModel_View_WithConnections(t *testing.T) {
	model := NewDevModel(DevConfig{
		MarkdownFile: "slides.md",
	})
	model.windowWidth = 80
	model.windowHeight = 24
	model.state.WebSocketClients = 3

	view := model.View()

	if !strings.Contains(view, "3 client") {
		t.Error("view should display connection count")
	}
}

func TestDevModel_View_WatcherStatus(t *testing.T) {
	model := NewDevModel(DevConfig{
		MarkdownFile: "slides.md",
	})
	model.windowWidth = 80
	model.windowHeight = 24
	model.state.WatcherRunning = true

	view := model.View()

	if !strings.Contains(view, "watching") {
		t.Error("view should show watcher as running")
	}
}

func TestDevModel_SendEvent(t *testing.T) {
	model := NewDevModel(DevConfig{})

	// Start a goroutine to consume the event
	done := make(chan bool)
	go func() {
		select {
		case event := <-model.eventsCh:
			if event.Type != "test" || event.Message != "test message" {
				t.Errorf("unexpected event: type=%q, msg=%q", event.Type, event.Message)
			}
			done <- true
		case <-time.After(time.Second):
			t.Error("timeout waiting for event")
			done <- false
		}
	}()

	model.SendEvent("test", "test message")

	<-done
}

func TestDevModel_SendReloadEvent(t *testing.T) {
	model := NewDevModel(DevConfig{})

	// Start a goroutine to consume the event
	done := make(chan bool)
	go func() {
		select {
		case event := <-model.eventsCh:
			if event.Type != "reload" {
				t.Errorf("expected reload event, got %q", event.Type)
			}
			if !strings.Contains(event.Message, "slides.md") {
				t.Errorf("expected message to contain path, got %q", event.Message)
			}
			done <- true
		case <-time.After(time.Second):
			t.Error("timeout waiting for event")
			done <- false
		}
	}()

	model.SendReloadEvent("slides.md")

	<-done
}

func TestDevModel_UpdateWebSocketCount(t *testing.T) {
	model := NewDevModel(DevConfig{})

	model.UpdateWebSocketCount(5)

	model.mu.RLock()
	count := model.state.WebSocketClients
	model.mu.RUnlock()

	if count != 5 {
		t.Errorf("expected WebSocket count 5, got %d", count)
	}
}

func TestDevModel_UpdateWatcherStatus(t *testing.T) {
	model := NewDevModel(DevConfig{})

	model.UpdateWatcherStatus(true)

	model.mu.RLock()
	running := model.state.WatcherRunning
	model.mu.RUnlock()

	if !running {
		t.Error("expected watcher to be running")
	}

	model.UpdateWatcherStatus(false)

	model.mu.RLock()
	running = model.state.WatcherRunning
	model.mu.RUnlock()

	if running {
		t.Error("expected watcher to not be running")
	}
}

func TestDevModel_SetError_ClearError(t *testing.T) {
	model := NewDevModel(DevConfig{})

	err := &mockError{msg: "test error"}
	model.SetError(err)

	model.mu.RLock()
	stateErr := model.state.Error
	model.mu.RUnlock()

	if stateErr == nil || stateErr.Error() != "test error" {
		t.Error("expected error to be set")
	}

	model.ClearError()

	model.mu.RLock()
	stateErr = model.state.Error
	model.mu.RUnlock()

	if stateErr != nil {
		t.Error("expected error to be cleared")
	}
}

func TestDevModel_FormatEvent_Types(t *testing.T) {
	model := NewDevModel(DevConfig{})

	tests := []struct {
		eventType    string
		expectedIcon string
	}{
		{"reload", "↻"},
		{"action", "→"},
		{"error", "✗"},
		{"other", "•"},
	}

	for _, tt := range tests {
		t.Run(tt.eventType, func(t *testing.T) {
			event := DevEvent{
				Timestamp: time.Now(),
				Type:      tt.eventType,
				Message:   "test",
			}
			formatted := model.formatEvent(event)
			if !strings.Contains(formatted, tt.expectedIcon) {
				t.Errorf("expected icon %q in formatted event, got %q", tt.expectedIcon, formatted)
			}
		})
	}
}

func TestDevModel_WasQuit(t *testing.T) {
	model := NewDevModel(DevConfig{})

	if model.WasQuit() {
		t.Error("model should not be quit initially")
	}

	model.quitting = true

	if !model.WasQuit() {
		t.Error("model should be quit after setting quitting=true")
	}
}

func TestDevModel_HandleKeyPress_Image_NoAPIKey(t *testing.T) {
	// Ensure GEMINI_API_KEY is not set
	originalKey := os.Getenv("GEMINI_API_KEY")
	os.Unsetenv("GEMINI_API_KEY")
	defer func() {
		if originalKey != "" {
			os.Setenv("GEMINI_API_KEY", originalKey)
		}
	}()

	model := NewDevModel(DevConfig{})

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")}
	newModel, _ := model.Update(msg)
	m := newModel.(*DevModel)

	// Should not show image generator
	if m.showImageGenerator {
		t.Error("showImageGenerator should be false when API key is missing")
	}

	// Should set an error
	m.mu.RLock()
	err := m.state.Error
	m.mu.RUnlock()

	if err == nil {
		t.Error("expected error to be set when API key is missing")
	}
	if !strings.Contains(err.Error(), "GEMINI_API_KEY") {
		t.Errorf("error message should mention GEMINI_API_KEY, got: %s", err.Error())
	}

	// Should add an error event
	m.mu.RLock()
	events := m.state.RecentEvents
	m.mu.RUnlock()

	found := false
	for _, e := range events {
		if e.Type == "error" && strings.Contains(e.Message, "GEMINI_API_KEY") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error event about missing GEMINI_API_KEY")
	}
}

func TestDevModel_HandleKeyPress_Image_WithAPIKey(t *testing.T) {
	// Set GEMINI_API_KEY
	originalKey := os.Getenv("GEMINI_API_KEY")
	os.Setenv("GEMINI_API_KEY", "test-api-key")
	defer func() {
		if originalKey != "" {
			os.Setenv("GEMINI_API_KEY", originalKey)
		} else {
			os.Unsetenv("GEMINI_API_KEY")
		}
	}()

	// Create a temporary markdown file for testing
	tmpDir := t.TempDir()
	mdFile := tmpDir + "/test.md"
	content := "# Test Slide\n\n---\n\n# Another Slide"
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model := NewDevModel(DevConfig{MarkdownFile: mdFile})

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")}
	newModel, _ := model.Update(msg)
	m := newModel.(*DevModel)

	// Should show image generator
	if !m.showImageGenerator {
		t.Error("showImageGenerator should be true when API key is present")
	}

	// Should not set an error
	m.mu.RLock()
	err := m.state.Error
	m.mu.RUnlock()

	if err != nil {
		t.Errorf("expected no error when API key is present, got: %s", err.Error())
	}

	// Should add an action event
	m.mu.RLock()
	events := m.state.RecentEvents
	m.mu.RUnlock()

	found := false
	for _, e := range events {
		if e.Type == "action" && strings.Contains(e.Message, "image generator") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected action event about opening image generator")
	}

	// Should have created the imageGenModel
	if m.imageGenModel == nil {
		t.Error("expected imageGenModel to be created")
	}

	// Verify slides were loaded
	if m.imageGenModel != nil && len(m.imageGenModel.Slides) != 2 {
		t.Errorf("expected 2 slides, got %d", len(m.imageGenModel.Slides))
	}
}

func TestDevModel_HandleKeyPress_Image_AlreadyGenerating(t *testing.T) {
	// Set GEMINI_API_KEY
	originalKey := os.Getenv("GEMINI_API_KEY")
	os.Setenv("GEMINI_API_KEY", "test-api-key")
	defer func() {
		if originalKey != "" {
			os.Setenv("GEMINI_API_KEY", originalKey)
		} else {
			os.Unsetenv("GEMINI_API_KEY")
		}
	}()

	model := NewDevModel(DevConfig{})
	model.showImageGenerator = true // Already showing

	// Clear any events
	model.mu.Lock()
	model.state.RecentEvents = nil
	model.mu.Unlock()

	msg := tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("i")}
	newModel, _ := model.Update(msg)
	m := newModel.(*DevModel)

	// Should still show image generator (no change)
	if !m.showImageGenerator {
		t.Error("showImageGenerator should remain true")
	}

	// Should not add any events (early return)
	m.mu.RLock()
	eventCount := len(m.state.RecentEvents)
	m.mu.RUnlock()

	if eventCount != 0 {
		t.Errorf("expected no new events when already generating, got %d", eventCount)
	}
}

func TestDevModel_ShowImageGenerator(t *testing.T) {
	model := NewDevModel(DevConfig{})

	if model.ShowImageGenerator() {
		t.Error("ShowImageGenerator should be false initially")
	}

	model.showImageGenerator = true

	if !model.ShowImageGenerator() {
		t.Error("ShowImageGenerator should be true after setting flag")
	}
}

func TestDevModel_ResetImageGenerator(t *testing.T) {
	model := NewDevModel(DevConfig{})
	model.showImageGenerator = true

	model.ResetImageGenerator()

	if model.ShowImageGenerator() {
		t.Error("ShowImageGenerator should be false after reset")
	}
}

func TestDevModel_View_HelpIncludesImage(t *testing.T) {
	model := NewDevModel(DevConfig{
		MarkdownFile: "slides.md",
	})
	model.windowWidth = 100
	model.windowHeight = 24

	view := model.View()

	if !strings.Contains(view, "i") || !strings.Contains(view, "image") {
		t.Error("help text should include 'i' shortcut for image generation")
	}
}
