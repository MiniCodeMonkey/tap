// Package tui provides terminal user interface components using Bubble Tea.
package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/deckedit"
	"github.com/MiniCodeMonkey/tap/internal/gemini"
	"github.com/MiniCodeMonkey/tap/internal/recorder"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ThemeBroadcaster is an interface for broadcasting theme changes via WebSocket.
type ThemeBroadcaster interface {
	BroadcastTheme(themeName string) error
}

// DevConfig holds configuration for the dev TUI.
// Fields ordered by size for memory alignment.
type DevConfig struct {
	AudienceURL  string
	PresenterURL string
	// NetworkURL is the presenter URL on this machine's LAN address. It is
	// set only when the server listens on the network (--lan).
	NetworkURL        string
	QRCodeASCII       string
	PresenterPassword string
	MarkdownFile      string
	CurrentTheme      string
	TunnelURL         string
	// Version is the tap version shown next to the title, for example
	// "v2.0.0-beta.2", or "dev" for a local build.
	Version string
	Port    int
	// RecordWarnAfter is how long a recording runs before the TUI warns.
	RecordWarnAfter time.Duration
	// RecordStopAfter is how long a recording runs before it stops itself.
	// Zero means no cap.
	RecordStopAfter time.Duration
	// RecordDisplay preselects an entry in the record picker.
	RecordDisplay int
	// Present runs the TUI for tap present: no editing keys, no file
	// watcher, and the recording state shown large.
	Present bool
}

// DevState holds the current state of the dev server.
// Fields ordered by size for memory alignment.
type DevState struct {
	Error            error
	RecentEvents     []DevEvent
	Warnings         []string
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

// recordEndedMsg is sent when the recorder stopped without being asked.
type recordEndedMsg struct {
	err error
}

// diskLevelMsg carries a change of the recordings disk level.
type diskLevelMsg struct {
	level recorder.DiskLevel
}

// recordPickerReadyMsg carries the result of running the preflight, listing
// displays and reading the default audio input off the update loop: all
// three shell out (screencapture or system_profiler), so none of them may
// run inside Update itself without freezing the TUI for the second or two
// they take.
type recordPickerReadyMsg struct {
	report      recorder.Report
	displays    []recorder.Display
	displaysErr error
	audioInput  string
}

// pdfExportMsg is sent when a PDF export completes.
type pdfExportMsg struct {
	outputPath string
	err        error
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
	recordEndedCh      chan error
	diskLevelCh        chan recorder.DiskLevel
	closeCh            chan struct{}
	themeBroadcaster   ThemeBroadcaster
	tunnels            TunnelController
	tunnelURL          string
	tunnelQR           string
	tunnelStarting     bool
	imageGenModel      *ImageGenModel
	addModel           *AddModel
	recorders          RecorderController
	recordDisplays     []recorder.Display
	recordingPath      string
	recordingStartedAt time.Time
	// recordPickerAudioInput is the default microphone name, read once
	// when the picker opens rather than on every render.
	recordPickerAudioInput string
	// gitignoreSuggestion is the ignore entry offered by the prompt shown
	// after a recording is saved, captured once when the prompt opens.
	gitignoreSuggestion string
	mu                  sync.RWMutex
	windowWidth         int
	windowHeight        int
	currentTheme        string
	themePickerIndex    int
	recordPickerIndex   int
	quitting            bool
	showThemePicker     bool
	showImageGenerator  bool
	showSlideBuilder    bool
	exportingPDF        bool
	recording           bool
	recordWarned        bool
	// recordBusy is set while a start or stop is in flight (including the
	// preflight and display probe that precede a start), and cleared once
	// its result lands. It stops the tick loop and a repeated C from
	// issuing another Start or Stop while one is still running: without
	// it, a Stop that takes a few seconds to finalize the file gets one
	// extra call per tick until the first reply arrives.
	recordBusy          bool
	showRecordPicker    bool
	showQuitConfirm     bool
	showGitignorePrompt bool
	presentRecorder     PresentRecorder
	reload              func() error
	showKeepPrompt      bool
	discardRecording    bool
	quitAfterGitignore  bool
}

// NewDevModel creates a new DevModel for the dev server TUI.
func NewDevModel(cfg DevConfig) *DevModel {
	// Set default theme if not provided
	currentTheme := cfg.CurrentTheme
	if currentTheme == "" {
		currentTheme = "base"
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
		config:    cfg,
		tunnelURL: cfg.TunnelURL,
		tunnelQR:  tunnelQRCode(PresenterTarget(cfg.TunnelURL, cfg.PresenterPassword)),
		state: DevState{
			RecentEvents: make([]DevEvent, 0, 10),
		},
		eventsCh:         make(chan DevEvent, 100),
		recordEndedCh:    make(chan error, 1),
		diskLevelCh:      make(chan recorder.DiskLevel, 4),
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
	commands := []tea.Cmd{m.listenForEvents(), tickCmd()}
	if m.config.Present {
		commands = append(commands, openBrowserCmd(m.config.AudienceURL))
	}
	return tea.Batch(commands...)
}

// listenForEvents returns a command that listens for external events.
func (m *DevModel) listenForEvents() tea.Cmd {
	return func() tea.Msg {
		select {
		case event := <-m.eventsCh:
			return devEventMsg{event: event}
		case err := <-m.recordEndedCh:
			return recordEndedMsg{err: err}
		case level := <-m.diskLevelCh:
			return diskLevelMsg{level: level}
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
			if igm, ok := newModel.(*ImageGenModel); ok {
				m.imageGenModel = igm
				// Check if generation completed successfully - save image and update markdown
				if m.imageGenModel.Step == ImageGenStepDone && m.imageGenModel.GeneratedImage != nil && m.imageGenModel.SavedImagePath == "" {
					placed, err := m.imageGenModel.PlaceImage()
					if err != nil {
						m.SetError(err)
						m.addEvent(DevEvent{
							Type:      "error",
							Message:   "Failed to add the generated image to the deck",
							Timestamp: time.Now(),
						})
						return m, cmd
					}
					savedPath := placed.Path
					m.imageGenModel.SavedImagePath = savedPath
					if placed.DeleteError != nil {
						m.addEvent(DevEvent{
							Type:      "error",
							Message:   "Failed to delete old image (non-fatal)",
							Timestamp: time.Now(),
						})
					}

					// Send reload event
					m.addEvent(DevEvent{
						Type:      "reload",
						Message:   fmt.Sprintf("Generated image: %s", savedPath),
						Timestamp: time.Now(),
					})
				}
			}
			return m, cmd
		}
	}

	// Forward non-key messages to slide builder when active (for textinput blink, etc.)
	if m.showSlideBuilder && m.addModel != nil {
		switch msg.(type) {
		case tea.KeyMsg:
			// Key messages are handled by handleKeyPress below
		default:
			// Forward blink and other messages to add model
			newModel, cmd := m.addModel.Update(msg)
			if addModel, ok := newModel.(AddModel); ok {
				m.addModel = &addModel
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

	case recordEndedMsg:
		m.recording = false
		m.recordWarned = false
		m.addEvent(DevEvent{
			Type:      "error",
			Message:   "Recording stopped unexpectedly: " + msg.err.Error(),
			Timestamp: time.Now(),
		})
		return m, m.listenForEvents()

	case diskLevelMsg:
		m.applyDiskLevel(msg.level)
		return m, m.listenForEvents()

	case tunnelMsg:
		return m.applyTunnelMsg(msg), nil

	case recordMsg:
		return m.applyRecordMsg(msg), nil

	case recordPickerReadyMsg:
		return m.applyRecordPickerReady(msg)

	case wsCountMsg:
		m.state.WebSocketClients = msg.count
		return m, nil

	case watcherStatusMsg:
		m.state.WatcherRunning = msg.running
		return m, nil

	case errorMsg:
		m.state.Error = msg.err
		return m, nil

	case pdfExportMsg:
		m.exportingPDF = false
		if msg.err != nil {
			m.SetError(msg.err)
			m.addEvent(DevEvent{
				Type:      "error",
				Message:   "PDF export failed",
				Timestamp: time.Now(),
			})
		} else {
			m.addEvent(DevEvent{
				Type:      "action",
				Message:   fmt.Sprintf("PDF exported → %s", msg.outputPath),
				Timestamp: time.Now(),
			})
		}
		return m, nil

	case tickMsg:
		// Periodic tick - redraw, and check on any running recording
		return m, tea.Batch(tickCmd(), m.recordingTick())

	case presentToggleMsg:
		if msg.err != nil {
			m.addEvent(DevEvent{Type: "error", Message: "Recording failed: " + msg.err.Error(), Timestamp: time.Now()})
		}
		return m, nil

	case reloadMsg:
		m.applyReloadMsg(msg)
		return m, nil
	}

	return m, nil
}

// handleKeyPress handles keyboard input.
func (m *DevModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle theme picker if it's open
	if m.showThemePicker {
		return m.handleThemePickerKey(msg)
	}

	// Handle record picker if it's open
	if m.showRecordPicker {
		return m.handleRecordPickerKey(msg)
	}

	// Handle the quit confirmation if it's open
	if m.showQuitConfirm {
		return m.handleQuitConfirmKey(msg)
	}

	// Handle the .gitignore prompt if it's open
	if m.showGitignorePrompt {
		return m.handleGitignoreKey(msg)
	}

	// Handle the keep prompt if it's open
	if m.showKeepPrompt {
		return m.handleKeepPromptKey(msg)
	}

	// Handle image generator if it's open
	if m.showImageGenerator && m.imageGenModel != nil {
		return m.handleImageGeneratorKey(msg)
	}

	// Handle slide builder if it's open
	if m.showSlideBuilder && m.addModel != nil {
		return m.handleSlideBuilderKey(msg)
	}

	// Present mode has its own, narrower key map: none of the editing keys
	// below are safe during a talk.
	if m.config.Present {
		return m.handlePresentKey(msg)
	}

	switch msg.String() {
	case "q":
		if m.confirmQuitWhileRecording() {
			m.showQuitConfirm = true
			return m, nil
		}
		m.quitting = true
		return m, tea.Quit

	case "ctrl+c":
		// Not always a deliberate keystroke, and the file must be
		// finalized either way, so this one never asks.
		m.quitting = true
		return m, tea.Quit

	case "a":
		// Add slide - open the slide builder overlay
		if m.showSlideBuilder {
			return m, nil
		}

		// Get absolute path to markdown file
		absPath, err := filepath.Abs(m.config.MarkdownFile)
		if err != nil {
			m.SetError(fmt.Errorf("failed to resolve file path: %w", err))
			return m, nil
		}

		// Create the add model
		addModel := NewAddModel(absPath)
		m.addModel = &addModel
		m.showSlideBuilder = true

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
		return m, m.reloadCmd()

	case "u":
		// Start or stop the public tunnel
		return m.toggleTunnel()

	case "c":
		// Start or stop a recording of the talk
		return m.toggleRecording()

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

	case "e":
		// Export to PDF
		if m.exportingPDF {
			return m, nil
		}
		m.exportingPDF = true
		m.addEvent(DevEvent{
			Type:      "action",
			Message:   "Exporting PDF...",
			Timestamp: time.Now(),
		})
		return m, m.exportPDFCmd()

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

		// Load a .env file next to the deck before checking for the key, the
		// same as tap image generate does, so a deck with no frontmatter
		// still picks up a key kept there instead of being told to add one
		// it already added.
		_ = config.LoadEnv(filepath.Dir(m.config.MarkdownFile))
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

		// Persist theme change to markdown file
		if m.config.MarkdownFile != "" {
			absPath, err := filepath.Abs(m.config.MarkdownFile)
			if err == nil {
				if err := deckedit.SetTheme(absPath, selectedTheme); err != nil {
					m.addEvent(DevEvent{
						Type:      "error",
						Message:   fmt.Sprintf("Failed to save theme: %v", err),
						Timestamp: time.Now(),
					})
				}
			}
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
	// Check if we're in the Done step - save the saved path before delegating
	wasInDoneStep := m.imageGenModel.Step == ImageGenStepDone
	savedPath := m.imageGenModel.SavedImagePath

	// Delegate to the image generator model
	newModel, cmd := m.imageGenModel.Update(msg)

	// Check if the user completed or cancelled (returns nil)
	if newModel == nil {
		m.showImageGenerator = false
		m.imageGenModel = nil

		// Check if this was a successful completion (was in Done step with saved image)
		if wasInDoneStep && savedPath != "" {
			m.addEvent(DevEvent{
				Type:      "action",
				Message:   fmt.Sprintf("Image generation complete: %s", savedPath),
				Timestamp: time.Now(),
			})
		} else {
			m.addEvent(DevEvent{
				Type:      "action",
				Message:   "Image generator cancelled",
				Timestamp: time.Now(),
			})
		}
		return m, nil
	}

	// Update the image generator model
	if igm, ok := newModel.(*ImageGenModel); ok {
		m.imageGenModel = igm
	}
	return m, cmd
}

// handleSlideBuilderKey handles keyboard input when the slide builder is open.
func (m *DevModel) handleSlideBuilderKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Check if we're in the Done step before delegating
	wasInDoneStep := m.addModel.step == addStepDone

	// Delegate to the add model
	newModel, cmd := m.addModel.Update(msg)

	// Check if it's the updated AddModel
	if addModel, ok := newModel.(AddModel); ok {
		m.addModel = &addModel

		// Check if the user completed or cancelled
		if addModel.quitting {
			m.showSlideBuilder = false
			m.addModel = nil
			m.addEvent(DevEvent{
				Type:      "action",
				Message:   "Slide builder cancelled",
				Timestamp: time.Now(),
			})
			return m, nil
		}

		// Check if done (success)
		if addModel.done || wasInDoneStep {
			m.showSlideBuilder = false
			m.addModel = nil
			m.addEvent(DevEvent{
				Type:      "reload",
				Message:   "Slide added successfully",
				Timestamp: time.Now(),
			})
			return m, nil
		}

		return m, cmd
	}

	return m, cmd
}

// openBrowserCmd returns a command that opens a URL in the default browser.
// exportPDFCmd runs `tap export pdf <file>` as a background command.
func (m *DevModel) exportPDFCmd() tea.Cmd {
	file := m.config.MarkdownFile
	ext := filepath.Ext(file)
	outputPath := strings.TrimSuffix(file, ext) + ".pdf"

	return func() tea.Msg {
		// Use the current binary to run tap export pdf
		binary, err := os.Executable()
		if err != nil {
			return pdfExportMsg{err: fmt.Errorf("failed to find executable: %w", err)}
		}

		cmd := exec.Command(binary, pdfExportArgs(file)...)
		if output, err := cmd.CombinedOutput(); err != nil {
			return pdfExportMsg{err: fmt.Errorf("PDF export failed: %s", strings.TrimSpace(string(output)))}
		}

		return pdfExportMsg{outputPath: outputPath}
	}
}

// pdfExportArgs is the tap command line that exports file to a PDF.
func pdfExportArgs(file string) []string {
	return []string{"export", "pdf", file}
}

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
	// Show the quit confirmation if it's open
	if m.showQuitConfirm {
		return m.viewQuitConfirm()
	}

	// Show the .gitignore prompt if it's open
	if m.showGitignorePrompt {
		return m.viewGitignorePrompt()
	}

	// Show the keep prompt if it's open
	if m.showKeepPrompt {
		return m.viewKeepPrompt()
	}

	if m.quitting {
		return RenderMuted("Shutting down server...\n")
	}

	// Show theme picker overlay if active
	if m.showThemePicker {
		return m.viewThemePicker()
	}

	// Show record picker overlay if active
	if m.showRecordPicker {
		return m.viewRecordPicker()
	}

	// Show image generator overlay if active
	if m.showImageGenerator && m.imageGenModel != nil {
		return m.imageGenModel.View()
	}

	// Show slide builder overlay if active
	if m.showSlideBuilder && m.addModel != nil {
		return m.addModel.View()
	}

	var b strings.Builder

	// Header
	b.WriteString(m.viewHeader())
	b.WriteString("\n")

	if m.config.Present {
		b.WriteString(m.viewPresentRecording())
		b.WriteString("\n")
	}

	// Server URLs section
	b.WriteString(m.viewURLs())
	b.WriteString("\n")

	// Status section
	b.WriteString(m.viewStatus())
	b.WriteString("\n")

	// QR Code (if available and fits)
	if m.qrCode() != "" && m.windowHeight > qrMinimumHeight(m.qrCode()) {
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

	// Warning display
	if len(m.state.Warnings) > 0 {
		b.WriteString(m.viewWarnings())
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
		Foreground(ColorPrimary)

	mutedStyle := lipgloss.NewStyle().
		Foreground(ColorMuted)

	titleText := "⚡ Tap Dev Server"
	if m.config.Present {
		titleText = "● PRESENTING"
	}
	title := titleStyle.Render(titleText)
	if m.config.Version != "" {
		title += " " + mutedStyle.Render(m.config.Version)
	}
	file := mutedStyle.Render(fmt.Sprintf("Serving: %s", m.config.MarkdownFile))

	return title + "\n\n" + file
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

	if m.config.NetworkURL != "" {
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("Network:"))
		b.WriteString(urlStyle.Render(m.config.NetworkURL))
	}

	switch {
	case m.tunnelStarting:
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("Tunnel:"))
		b.WriteString(RenderMuted("starting..."))
	case m.tunnelURL != "":
		b.WriteString("\n")
		b.WriteString(labelStyle.Render("Tunnel:"))
		b.WriteString(urlStyle.Render(m.tunnelURL))
		b.WriteString("\n")
		b.WriteString(labelStyle.Render(""))
		b.WriteString(RenderMuted("publicly accessible"))
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
	switch {
	case m.config.Present:
		b.WriteString(RenderMuted("○ off (r reloads)"))
	case m.state.WatcherRunning:
		b.WriteString(RenderSuccess("● watching"))
	default:
		b.WriteString(RenderMuted("○ not running"))
	}

	// Recording
	if !m.config.Present {
		if status := m.viewRecordingStatus(); status != "" {
			b.WriteString("\n")
			b.WriteString(labelStyle.Render("Recording:"))
			b.WriteString(RenderError(status))
		}
	}

	return b.String()
}

// viewQRCode renders the QR code section.
func (m *DevModel) viewQRCode() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(RenderSubtitle("Scan for the presenter view:"))
	b.WriteString("\n")

	// Every row, always. Dropping rows to make it fit leaves something
	// that still looks like a QR code and cannot be scanned; the caller
	// decides whether there is room for one at all (see the height check
	// in View).
	b.WriteString(m.qrCode())

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

// viewWarnings renders the warnings section - bundler warnings from the
// last reload (see ClearWarnings), shown next to any error but never
// fatal on their own.
func (m *DevModel) viewWarnings() string {
	if len(m.state.Warnings) == 0 {
		return ""
	}

	warningBox := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(ColorWarning).
		Padding(0, 1).
		MarginTop(1)

	return warningBox.Render(RenderWarning(strings.Join(m.state.Warnings, "\n")))
}

// viewHelp renders the keyboard shortcuts section.
func (m *DevModel) viewHelp() string {
	helpStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		MarginTop(1)

	keyStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	if m.config.Present {
		keys := []string{
			keyStyle.Render("o") + " open slides",
			keyStyle.Render("p") + " presenter view",
			keyStyle.Render("r") + " reload",
			keyStyle.Render("u") + " tunnel",
			keyStyle.Render("c") + " record",
			keyStyle.Render("q") + " quit",
		}
		return helpStyle.Render(strings.Join(keys, " • "))
	}

	keys := []string{
		keyStyle.Render("o") + " open browser",
		keyStyle.Render("p") + " presenter view",
		keyStyle.Render("u") + " tunnel",
		keyStyle.Render("t") + " theme",
		keyStyle.Render("a") + " add slide",
		keyStyle.Render("i") + " image",
		keyStyle.Render("e") + " export pdf",
		keyStyle.Render("r") + " reload",
	}
	if m.recorders != nil && m.recorders.Available() {
		label := " record"
		if m.recording {
			label = " stop recording"
		}
		keys = append(keys, keyStyle.Render("c")+label)
	}
	keys = append(keys, keyStyle.Render("q")+" quit")

	return helpStyle.Render(strings.Join(keys, " • "))
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

// SetWarnings sets the warnings to display next to any error - a bundler
// warning from the last reload (an esbuild warning on a component bundle,
// for example), not fatal enough to be an error but still worth showing.
// An empty slice clears the display, the same as ClearWarnings.
func (m *DevModel) SetWarnings(warnings []string) {
	m.mu.Lock()
	m.state.Warnings = warnings
	m.mu.Unlock()
}

// ClearWarnings clears any displayed warnings. Call this on every clean
// reload (one with nothing to warn about), so a warning from an earlier
// reload does not linger once it no longer applies.
func (m *DevModel) ClearWarnings() {
	m.mu.Lock()
	m.state.Warnings = nil
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
