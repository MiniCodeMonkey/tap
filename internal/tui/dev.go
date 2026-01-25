// Package tui provides terminal user interface components using Bubble Tea.
package tui

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tapsh/tap/internal/gemini"
)

// ThemeBroadcaster is an interface for broadcasting theme changes via WebSocket.
type ThemeBroadcaster interface {
	BroadcastTheme(themeName string) error
}

// DevConfig holds configuration for the dev TUI.
// Fields ordered by size for memory alignment.
type DevConfig struct {
	AudienceURL       string
	PresenterURL      string
	QRCodeASCII       string
	PresenterPassword string
	MarkdownFile      string
	CurrentTheme      string
	Port              int
}

// DevState holds the current state of the dev server.
// Fields ordered by size for memory alignment.
type DevState struct {
	Error            error
	RecentEvents     []DevEvent
	WebSocketClients int
	WatcherRunning   bool
}

// DevEvent represents a hot reload or server event.
// Fields ordered by size for memory alignment.
type DevEvent struct {
	Timestamp time.Time
	Type      string
	Message   string
}

// devEventMsg is sent when a new event occurs.
type devEventMsg struct {
	event DevEvent
}

// wsCountMsg is sent when WebSocket client count changes.
type wsCountMsg struct {
	count int
}

// watcherStatusMsg is sent when watcher status changes.
type watcherStatusMsg struct {
	running bool
}

// errorMsg is sent when an error occurs.
type errorMsg struct {
	err error
}

// tickMsg is sent periodically to update the display.
type tickMsg struct{}

// DevModel is the Bubble Tea model for the dev server TUI.
type DevModel struct { //nolint:govet // embedded structs prevent optimal alignment
	config             DevConfig
	state              DevState
	eventsCh           chan DevEvent
	closeCh            chan struct{}
	themeBroadcaster   ThemeBroadcaster
	imageGenModel      *ImageGenModel
	mu                 sync.RWMutex
	windowWidth        int
	windowHeight       int
	currentTheme       string
	themePickerIndex   int
	quitting           bool
	showThemePicker    bool
	showImageGenerator bool
}

// NewDevModel creates a new DevModel for the dev server TUI.
func NewDevModel(cfg DevConfig) *DevModel {
	// Set default theme if not provided
	currentTheme := cfg.CurrentTheme
	if currentTheme == "" {
		currentTheme = "paper"
	}

	// Find the index of the current theme
	themeIndex := 0
	for i, t := range AvailableThemes {
		if t.Name == currentTheme {
			themeIndex = i
			break
		}
	}

	return &DevModel{
		config: cfg,
		state: DevState{
			RecentEvents: make([]DevEvent, 0, 10),
		},
		eventsCh:         make(chan DevEvent, 100),
		closeCh:          make(chan struct{}),
		currentTheme:     currentTheme,
		themePickerIndex: themeIndex,
	}
}

// SetThemeBroadcaster sets the theme broadcaster for WebSocket communication.
func (m *DevModel) SetThemeBroadcaster(tb ThemeBroadcaster) {
	m.themeBroadcaster = tb
}

// Init implements tea.Model.
func (m *DevModel) Init() tea.Cmd {
	return tea.Batch(
		m.listenForEvents(),
		tickCmd(),
	)
}

// listenForEvents returns a command that listens for external events.
func (m *DevModel) listenForEvents() tea.Cmd {
	return func() tea.Msg {
		select {
		case event := <-m.eventsCh:
			return devEventMsg{event: event}
		case <-m.closeCh:
			return nil
		}
	}
}

// tickCmd returns a command that sends periodic tick messages.
func tickCmd() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return tickMsg{}
	})
}

// Update implements tea.Model.
func (m *DevModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	// Forward non-key messages to image generator when active (for spinner animation, API results, etc.)
	if m.showImageGenerator && m.imageGenModel != nil {
		// Only forward certain message types to the image generator
		switch msg.(type) {
		case tea.KeyMsg:
			// Key messages are handled by handleKeyPress below
		default:
			// Forward spinner ticks and other messages to image generator
			newModel, cmd := m.imageGenModel.Update(msg)
			if newModel != nil {
				m.imageGenModel = newModel.(*ImageGenModel)
				// Check if generation completed successfully
				if m.imageGenModel.Step == ImageGenStepDone && m.imageGenModel.GeneratedImage != nil {
					// Generation successful, will be handled by subsequent stories
				}
			}
			return m, cmd
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
		return m, nil

	case devEventMsg:
		m.addEvent(msg.event)
		return m, m.listenForEvents()

	case wsCountMsg:
		m.state.WebSocketClients = msg.count
		return m, nil

	case watcherStatusMsg:
		m.state.WatcherRunning = msg.running
		return m, nil

	case errorMsg:
		m.state.Error = msg.err
		return m, nil

	case tickMsg:
		// Periodic tick - just redraw
		return m, tickCmd()
	}

	return m, nil
}

// handleKeyPress handles keyboard input.
func (m *DevModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle theme picker if it's open
	if m.showThemePicker {
		return m.handleThemePickerKey(msg)
	}

	// Handle image generator if it's open
	if m.showImageGenerator && m.imageGenModel != nil {
		return m.handleImageGeneratorKey(msg)
	}

	switch msg.String() {
	case "q", "ctrl+c":
		m.quitting = true
		return m, tea.Quit

	case "a":
		// Add slide - will trigger external action
		m.addEvent(DevEvent{
			Type:      "action",
			Message:   "Opening slide builder...",
			Timestamp: time.Now(),
		})
		return m, nil

	case "o":
		// Open in browser
		m.addEvent(DevEvent{
			Type:      "action",
			Message:   "Opening browser...",
			Timestamp: time.Now(),
		})
		return m, openBrowserCmd(m.config.AudienceURL)

	case "p":
		// Open presenter view
		m.addEvent(DevEvent{
			Type:      "action",
			Message:   "Opening presenter view...",
			Timestamp: time.Now(),
		})
		return m, openBrowserCmd(m.config.PresenterURL)

	case "r":
		// Manual reload
		m.addEvent(DevEvent{
			Type:      "reload",
			Message:   "Manual reload triggered",
			Timestamp: time.Now(),
		})
		return m, nil

	case "t":
		// Open theme picker
		m.showThemePicker = true
		// Set picker index to current theme
		for i, t := range AvailableThemes {
			if t.Name == m.currentTheme {
				m.themePickerIndex = i
				break
			}
		}
		return m, nil

	case "i":
		// Open image generator
		// Check if already showing image generator or generation is in progress
		if m.showImageGenerator {
			return m, nil
		}
		// Also check if there's an active image generation in progress
		if m.imageGenModel != nil && m.imageGenModel.IsGenerating {
			return m, nil
		}

		// Check for GEMINI_API_KEY
		if !gemini.HasAPIKey() {
			m.SetError(fmt.Errorf("GEMINI_API_KEY not set. Add it to your .env file to use AI image generation"))
			m.addEvent(DevEvent{
				Type:      "error",
				Message:   "Missing GEMINI_API_KEY environment variable",
				Timestamp: time.Now(),
			})
			return m, nil
		}

		// Create the image generator model
		imageGen, err := NewImageGenModel(m.config.MarkdownFile)
		if err != nil {
			m.SetError(fmt.Errorf("failed to load slides: %w", err))
			m.addEvent(DevEvent{
				Type:      "error",
				Message:   "Failed to load slides for image generator",
				Timestamp: time.Now(),
			})
			return m, nil
		}

		// API key is present, show image generator
		m.imageGenModel = imageGen
		m.showImageGenerator = true
		m.addEvent(DevEvent{
			Type:      "action",
			Message:   "Opening image generator...",
			Timestamp: time.Now(),
		})
		return m, nil
	}

	return m, nil
}

// handleThemePickerKey handles keyboard input when the theme picker is open.
func (m *DevModel) handleThemePickerKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		m.showThemePicker = false
		return m, nil

	case "up", "k":
		if m.themePickerIndex > 0 {
			m.themePickerIndex--
		}
		return m, nil

	case "down", "j":
		if m.themePickerIndex < len(AvailableThemes)-1 {
			m.themePickerIndex++
		}
		return m, nil

	case "enter":
		// Select theme and broadcast
		selectedTheme := AvailableThemes[m.themePickerIndex].Name
		m.currentTheme = selectedTheme
		m.showThemePicker = false

		// Broadcast theme change via WebSocket
		if m.themeBroadcaster != nil {
			_ = m.themeBroadcaster.BroadcastTheme(selectedTheme)
		}

		m.addEvent(DevEvent{
			Type:      "action",
			Message:   fmt.Sprintf("Theme changed to %s", selectedTheme),
			Timestamp: time.Now(),
		})
		return m, nil
	}

	return m, nil
}

// handleImageGeneratorKey handles keyboard input when the image generator is open.
func (m *DevModel) handleImageGeneratorKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Delegate to the image generator model
	newModel, cmd := m.imageGenModel.Update(msg)

	// Check if the user cancelled (returns nil)
	if newModel == nil {
		m.showImageGenerator = false
		m.imageGenModel = nil
		m.addEvent(DevEvent{
			Type:      "action",
			Message:   "Image generator cancelled",
			Timestamp: time.Now(),
		})
		return m, nil
	}

	// Update the image generator model
	m.imageGenModel = newModel.(*ImageGenModel)
	return m, cmd
}

// openBrowserCmd returns a command that opens a URL in the default browser.
func openBrowserCmd(url string) tea.Cmd {
	return func() tea.Msg {
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", url)
		case "linux":
			cmd = exec.Command("xdg-open", url)
		case "windows":
			cmd = exec.Command("cmd", "/c", "start", url)
		default:
			return nil
		}
		_ = cmd.Start()
		return nil
	}
}

// addEvent adds a new event to the recent events list.
func (m *DevModel) addEvent(event DevEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// Keep only the most recent 5 events
	m.state.RecentEvents = append(m.state.RecentEvents, event)
	if len(m.state.RecentEvents) > 5 {
		m.state.RecentEvents = m.state.RecentEvents[len(m.state.RecentEvents)-5:]
	}
}

// View implements tea.Model.
func (m *DevModel) View() string {
	if m.quitting {
		return RenderMuted("Shutting down server...\n")
	}

	// Show theme picker overlay if active
	if m.showThemePicker {
		return m.viewThemePicker()
	}

	// Show image generator overlay if active
	if m.showImageGenerator && m.imageGenModel != nil {
		return m.imageGenModel.View()
	}

	var b strings.Builder

	// Header
	b.WriteString(m.viewHeader())
	b.WriteString("\n")

	// Server URLs section
	b.WriteString(m.viewURLs())
	b.WriteString("\n")

	// Status section
	b.WriteString(m.viewStatus())
	b.WriteString("\n")

	// QR Code (if available and fits)
	if m.config.QRCodeASCII != "" && m.windowHeight > 30 {
		b.WriteString(m.viewQRCode())
		b.WriteString("\n")
	}

	// Recent events
	b.WriteString(m.viewEvents())
	b.WriteString("\n")

	// Error display
	if m.state.Error != nil {
		b.WriteString(m.viewError())
		b.WriteString("\n")
	}

	// Help/keyboard shortcuts
	b.WriteString(m.viewHelp())

	return b.String()
}

// viewHeader renders the header section.
func (m *DevModel) viewHeader() string {
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)

	fileStyle := lipgloss.NewStyle().
		Foreground(ColorMuted)

	title := titleStyle.Render("⚡ Tap Dev Server")
	file := fileStyle.Render(fmt.Sprintf("Serving: %s", m.config.MarkdownFile))

	return title + "\n" + file
}

// viewURLs renders the server URLs section.
func (m *DevModel) viewURLs() string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(18)

	urlStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Bold(true)

	b.WriteString("\n")
	b.WriteString(labelStyle.Render("Audience view:"))
	b.WriteString(urlStyle.Render(m.config.AudienceURL))
	b.WriteString("\n")

	b.WriteString(labelStyle.Render("Presenter view:"))
	b.WriteString(urlStyle.Render(m.config.PresenterURL))

	if m.config.PresenterPassword != "" {
		b.WriteString("\n")
		b.WriteString(labelStyle.Render(""))
		b.WriteString(RenderMuted("(password protected)"))
	}

	return b.String()
}

// viewStatus renders the status section.
func (m *DevModel) viewStatus() string {
	var b strings.Builder

	labelStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(18)

	b.WriteString("\n")

	// Current theme
	b.WriteString(labelStyle.Render("Theme:"))
	themeStyle := lipgloss.NewStyle().Foreground(ColorPrimary).Bold(true)
	b.WriteString(themeStyle.Render(m.currentTheme))
	b.WriteString("\n")

	// WebSocket connections
	b.WriteString(labelStyle.Render("Connections:"))
	connCount := m.state.WebSocketClients
	if connCount == 0 {
		b.WriteString(RenderMuted("none"))
	} else {
		connStyle := lipgloss.NewStyle().Foreground(ColorSecondary)
		b.WriteString(connStyle.Render(fmt.Sprintf("%d client(s)", connCount)))
	}
	b.WriteString("\n")

	// Watcher status
	b.WriteString(labelStyle.Render("File watcher:"))
	if m.state.WatcherRunning {
		b.WriteString(RenderSuccess("● watching"))
	} else {
		b.WriteString(RenderMuted("○ not running"))
	}

	return b.String()
}

// viewQRCode renders the QR code section.
func (m *DevModel) viewQRCode() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(RenderSubtitle("Scan to join:"))
	b.WriteString("\n")

	// Render QR code with reduced size if needed
	qrLines := strings.Split(m.config.QRCodeASCII, "\n")
	maxLines := 15
	if len(qrLines) > maxLines {
		// Take every other line for a smaller QR
		for i := 0; i < len(qrLines) && i/2 < maxLines; i += 2 {
			b.WriteString(qrLines[i])
			b.WriteString("\n")
		}
	} else {
		b.WriteString(m.config.QRCodeASCII)
	}

	return b.String()
}

// viewEvents renders the recent events section.
func (m *DevModel) viewEvents() string {
	m.mu.RLock()
	events := make([]DevEvent, len(m.state.RecentEvents))
	copy(events, m.state.RecentEvents)
	m.mu.RUnlock()

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(RenderSubtitle("Recent activity:"))
	b.WriteString("\n")

	if len(events) == 0 {
		b.WriteString(RenderMuted("  No activity yet"))
	} else {
		for _, event := range events {
			b.WriteString(m.formatEvent(event))
			b.WriteString("\n")
		}
	}

	return b.String()
}

// formatEvent formats a single event for display.
func (m *DevModel) formatEvent(event DevEvent) string {
	timeStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		Width(10)

	var msgStyle lipgloss.Style
	var icon string

	switch event.Type {
	case "reload":
		msgStyle = lipgloss.NewStyle().Foreground(ColorSecondary)
		icon = "↻"
	case "action":
		msgStyle = lipgloss.NewStyle().Foreground(ColorPrimary)
		icon = "→"
	case "error":
		msgStyle = lipgloss.NewStyle().Foreground(ColorError)
		icon = "✗"
	default:
		msgStyle = lipgloss.NewStyle().Foreground(ColorWhite)
		icon = "•"
	}

	timeStr := event.Timestamp.Format("15:04:05")
	return fmt.Sprintf("  %s %s %s",
		timeStyle.Render(timeStr),
		icon,
		msgStyle.Render(event.Message))
}

// viewError renders the error section.
func (m *DevModel) viewError() string {
	if m.state.Error == nil {
		return ""
	}

	errorBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorError).
		Padding(0, 1).
		MarginTop(1)

	return errorBox.Render(RenderError("Error: " + m.state.Error.Error()))
}

// viewHelp renders the keyboard shortcuts section.
func (m *DevModel) viewHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		MarginTop(1)

	keyStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	help := fmt.Sprintf(
		"%s open browser • %s presenter view • %s theme • %s add slide • %s image • %s reload • %s quit",
		keyStyle.Render("o"),
		keyStyle.Render("p"),
		keyStyle.Render("t"),
		keyStyle.Render("a"),
		keyStyle.Render("i"),
		keyStyle.Render("r"),
		keyStyle.Render("q"),
	)

	return helpStyle.Render(help)
}

// viewThemePicker renders the theme picker overlay.
func (m *DevModel) viewThemePicker() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("🎨 Select Theme"))
	b.WriteString("\n\n")

	// Theme list
	for i, theme := range AvailableThemes {
		// Check if this is the current theme (from frontmatter)
		isCurrent := theme.Name == m.currentTheme

		if i == m.themePickerIndex {
			// Selected item
			selectedStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary)
			b.WriteString(selectedStyle.Render("> " + theme.Name))
			if isCurrent {
				currentStyle := lipgloss.NewStyle().Foreground(ColorMuted)
				b.WriteString(currentStyle.Render(" (current)"))
			}
			b.WriteString("\n")

			// Show description for selected theme
			descStyle := lipgloss.NewStyle().
				Foreground(ColorMuted).
				PaddingLeft(4)
			b.WriteString(descStyle.Render(theme.Description))
		} else {
			// Unselected item
			unselectedStyle := lipgloss.NewStyle().
				Foreground(ColorWhite)
			b.WriteString(unselectedStyle.Render("  " + theme.Name))
			if isCurrent {
				currentStyle := lipgloss.NewStyle().Foreground(ColorMuted)
				b.WriteString(currentStyle.Render(" (current)"))
			}
		}
		b.WriteString("\n")
	}

	// Help text
	b.WriteString("\n")
	helpStyle := lipgloss.NewStyle().
		Foreground(ColorMuted)

	keyStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	help := fmt.Sprintf(
		"%s/%s navigate • %s select • %s cancel",
		keyStyle.Render("↑"),
		keyStyle.Render("↓"),
		keyStyle.Render("enter"),
		keyStyle.Render("esc"),
	)
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

// External update methods - these can be called from outside the TUI

// SendEvent sends an event to be displayed in the TUI.
func (m *DevModel) SendEvent(eventType, message string) {
	select {
	case m.eventsCh <- DevEvent{
		Type:      eventType,
		Message:   message,
		Timestamp: time.Now(),
	}:
	default:
		// Channel full, skip
	}
}

// SendReloadEvent sends a reload event.
func (m *DevModel) SendReloadEvent(path string) {
	m.SendEvent("reload", fmt.Sprintf("File changed: %s", path))
}

// UpdateWebSocketCount updates the WebSocket client count.
func (m *DevModel) UpdateWebSocketCount(count int) {
	m.mu.Lock()
	m.state.WebSocketClients = count
	m.mu.Unlock()
}

// UpdateWatcherStatus updates the file watcher status.
func (m *DevModel) UpdateWatcherStatus(running bool) {
	m.mu.Lock()
	m.state.WatcherRunning = running
	m.mu.Unlock()
}

// SetError sets an error to be displayed.
func (m *DevModel) SetError(err error) {
	m.mu.Lock()
	m.state.Error = err
	m.mu.Unlock()
}

// ClearError clears any displayed error.
func (m *DevModel) ClearError() {
	m.mu.Lock()
	m.state.Error = nil
	m.mu.Unlock()
}

// Close signals the model to stop listening for events.
func (m *DevModel) Close() {
	close(m.closeCh)
}

// WasQuit returns true if the user quit the TUI.
func (m *DevModel) WasQuit() bool {
	return m.quitting
}

// ShowImageGenerator returns true if the image generator should be shown.
func (m *DevModel) ShowImageGenerator() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.showImageGenerator
}

// ResetImageGenerator resets the image generator state.
func (m *DevModel) ResetImageGenerator() {
	m.mu.Lock()
	m.showImageGenerator = false
	m.mu.Unlock()
}

// GetEventChannel returns the events channel for testing.
func (m *DevModel) GetEventChannel() chan DevEvent {
	return m.eventsCh
}

// RunDevTUI runs the dev server TUI and returns when the user quits.
func RunDevTUI(cfg DevConfig) error {
	model := NewDevModel(cfg)
	p := tea.NewProgram(model, tea.WithAltScreen())

	_, err := p.Run()
	model.Close()
	return err
}

// RunDevTUIWithModel runs the dev server TUI with a pre-configured model.
func RunDevTUIWithModel(model *DevModel) error {
	p := tea.NewProgram(model, tea.WithAltScreen())

	_, err := p.Run()
	model.Close()
	return err
}
