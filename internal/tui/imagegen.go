// Package tui provides terminal user interface components using Bubble Tea.
package tui

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// ImageGenStep represents the current step in the image generation workflow.
type ImageGenStep int

const (
	// ImageGenStepSlideSelect is the slide selection step.
	ImageGenStepSlideSelect ImageGenStep = iota
	// ImageGenStepPrompt is the prompt input step.
	ImageGenStepPrompt
	// ImageGenStepGenerating is the image generation step.
	ImageGenStepGenerating
	// ImageGenStepDone is the completion step.
	ImageGenStepDone
)

// SlideInfo contains information about a slide for display in the selector.
type SlideInfo struct {
	// Index is the zero-based slide index.
	Index int
	// Title is the slide title (first heading or first line).
	Title string
	// HasAIImages indicates if the slide has AI-generated images.
	HasAIImages bool
	// AIImageCount is the number of AI-generated images on the slide.
	AIImageCount int
}

// ImageGenModel is the Bubble Tea model for the image generation workflow.
type ImageGenModel struct {
	// Slides contains information about all slides.
	Slides []SlideInfo
	// SelectedIndex is the currently selected slide index.
	SelectedIndex int
	// Step is the current step in the workflow.
	Step ImageGenStep
	// Error holds any error message to display.
	Error string
	// MarkdownFile is the path to the markdown file being edited.
	MarkdownFile string
}

// NewImageGenModel creates a new ImageGenModel for image generation.
func NewImageGenModel(markdownFile string) (*ImageGenModel, error) {
	m := &ImageGenModel{
		MarkdownFile:  markdownFile,
		SelectedIndex: 0,
		Step:          ImageGenStepSlideSelect,
	}

	// Load slides from the markdown file
	if err := m.loadSlides(); err != nil {
		return nil, err
	}

	return m, nil
}

// loadSlides parses the markdown file and extracts slide information.
func (m *ImageGenModel) loadSlides() error {
	content, err := os.ReadFile(m.MarkdownFile)
	if err != nil {
		return fmt.Errorf("failed to read markdown file: %w", err)
	}

	// Parse slides
	m.Slides = parseSlides(string(content))
	return nil
}

// slideDelimiterRe matches "---" on its own line.
var slideDelimiterRe = regexp.MustCompile(`(?m)^---\s*$`)

// headingRe matches markdown headings (# Heading).
var headingRe = regexp.MustCompile(`(?m)^#+\s+(.+)$`)

// frontmatterRe matches YAML frontmatter at the start of a file.
var frontmatterRe = regexp.MustCompile(`(?s)^---\n.*?\n---\n?`)

// parseSlides extracts slide information from markdown content.
func parseSlides(content string) []SlideInfo {
	// Remove frontmatter if present
	content = frontmatterRe.ReplaceAllString(content, "")

	// Split on slide delimiter
	parts := slideDelimiterRe.Split(content, -1)

	slides := make([]SlideInfo, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}

		slide := SlideInfo{
			Index: len(slides),
			Title: extractSlideTitle(part),
		}

		slides = append(slides, slide)
	}

	return slides
}

// extractSlideTitle extracts the title from slide content.
// It looks for the first heading, or falls back to the first non-empty line.
func extractSlideTitle(content string) string {
	// Try to find a heading
	if match := headingRe.FindStringSubmatch(content); match != nil {
		title := strings.TrimSpace(match[1])
		// Truncate if too long
		if len(title) > 50 {
			title = title[:47] + "..."
		}
		return title
	}

	// Fall back to first non-empty line
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Skip directives (HTML comments at start)
		if strings.HasPrefix(line, "<!--") {
			continue
		}
		if line != "" {
			// Truncate if too long
			if len(line) > 50 {
				line = line[:47] + "..."
			}
			return line
		}
	}

	return "(empty slide)"
}

// Init implements tea.Model.
func (m *ImageGenModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m *ImageGenModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)
	}
	return m, nil
}

// handleKeyPress handles keyboard input for the image generator.
func (m *ImageGenModel) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.Step {
	case ImageGenStepSlideSelect:
		return m.handleSlideSelectKey(msg)
	}
	return m, nil
}

// handleSlideSelectKey handles keyboard input during slide selection.
func (m *ImageGenModel) handleSlideSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc", "q":
		// Return nil to signal cancellation to the parent
		return nil, nil

	case "up", "k":
		if m.SelectedIndex > 0 {
			m.SelectedIndex--
		}
		return m, nil

	case "down", "j":
		if m.SelectedIndex < len(m.Slides)-1 {
			m.SelectedIndex++
		}
		return m, nil

	case "enter":
		// Select the slide and proceed to next step
		// For now, just mark the slide as selected (next stories will add more steps)
		m.Step = ImageGenStepPrompt
		return m, nil
	}

	return m, nil
}

// View implements tea.Model.
func (m *ImageGenModel) View() string {
	switch m.Step {
	case ImageGenStepSlideSelect:
		return m.viewSlideSelect()
	default:
		return m.viewSlideSelect()
	}
}

// viewSlideSelect renders the slide selection view.
func (m *ImageGenModel) viewSlideSelect() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("🖼  Select Slide for Image"))
	b.WriteString("\n\n")

	// Slide list
	for i, slide := range m.Slides {
		slideNum := fmt.Sprintf("%2d.", slide.Index+1)

		if i == m.SelectedIndex {
			// Selected item
			selectedStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary)
			numStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

			b.WriteString(numStyle.Render("> "))
			b.WriteString(numStyle.Render(slideNum))
			b.WriteString(" ")
			b.WriteString(selectedStyle.Render(slide.Title))
		} else {
			// Unselected item
			unselectedStyle := lipgloss.NewStyle().
				Foreground(ColorWhite)
			numStyle := lipgloss.NewStyle().
				Foreground(ColorMuted)

			b.WriteString("  ")
			b.WriteString(numStyle.Render(slideNum))
			b.WriteString(" ")
			b.WriteString(unselectedStyle.Render(slide.Title))
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

// GetSelectedSlide returns the currently selected slide info.
func (m *ImageGenModel) GetSelectedSlide() *SlideInfo {
	if m.SelectedIndex >= 0 && m.SelectedIndex < len(m.Slides) {
		return &m.Slides[m.SelectedIndex]
	}
	return nil
}

// IsCancelled returns true if the user cancelled the workflow.
func (m *ImageGenModel) IsCancelled() bool {
	return m == nil
}
