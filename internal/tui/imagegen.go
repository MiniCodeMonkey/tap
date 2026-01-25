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
	// ImageGenStepImageSelect is the image selection step (add new or regenerate).
	ImageGenStepImageSelect
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
	// AIImages contains info about each AI-generated image on the slide.
	AIImages []AIImageInfo
}

// ImageSelectOption represents an option in the image selection step.
type ImageSelectOption struct {
	// IsAddNew indicates if this is the "Add new image" option.
	IsAddNew bool
	// AIImage is the AI image info for regenerate options (nil for add new).
	AIImage *AIImageInfo
	// Label is the display label for this option.
	Label string
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
	// ImageOptions contains the options for image selection (add new or regenerate).
	ImageOptions []ImageSelectOption
	// ImageOptionIndex is the currently selected image option index.
	ImageOptionIndex int
	// SelectedImage is the AI image being regenerated (nil if adding new).
	SelectedImage *AIImageInfo
	// Prompt is the prompt text for image generation.
	Prompt string
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

// aiPromptRe matches AI prompt comments: <!-- ai-prompt: ... -->
// It captures the prompt text in group 1.
var aiPromptRe = regexp.MustCompile(`<!--\s*ai-prompt:\s*(.+?)\s*-->`)

// aiImageRe matches AI prompt comments followed by an image on the next line.
// Group 1: prompt text, Group 2: image path
// Only matches if the image is directly on the next line (possibly with leading spaces, but no blank lines).
var aiImageRe = regexp.MustCompile(`<!--\s*ai-prompt:\s*(.+?)\s*-->\n[ \t]*!\[\]\(([^)]+)\)`)

// AIImageInfo contains information about an AI-generated image.
type AIImageInfo struct {
	// Prompt is the AI prompt used to generate the image.
	Prompt string
	// ImagePath is the path to the generated image file.
	ImagePath string
}

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

		aiImages := parseAIImages(part)
		slide := SlideInfo{
			Index:        len(slides),
			Title:        extractSlideTitle(part),
			AIImages:     aiImages,
			HasAIImages:  len(aiImages) > 0,
			AIImageCount: len(aiImages),
		}

		slides = append(slides, slide)
	}

	return slides
}

// parseAIImages extracts AI-generated image info from slide content.
// It looks for <!-- ai-prompt: ... --> comments followed by image references.
func parseAIImages(content string) []AIImageInfo {
	matches := aiImageRe.FindAllStringSubmatch(content, -1)
	if matches == nil {
		return nil
	}

	images := make([]AIImageInfo, 0, len(matches))
	for _, match := range matches {
		if len(match) >= 3 {
			images = append(images, AIImageInfo{
				Prompt:    match[1],
				ImagePath: match[2],
			})
		}
	}

	return images
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
	case ImageGenStepImageSelect:
		return m.handleImageSelectKey(msg)
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
		slide := m.GetSelectedSlide()
		if slide != nil && slide.HasAIImages {
			// Slide has AI images, show add/regenerate options
			m.buildImageOptions()
			m.Step = ImageGenStepImageSelect
		} else {
			// No AI images, go directly to prompt input
			m.Step = ImageGenStepPrompt
		}
		return m, nil
	}

	return m, nil
}

// handleImageSelectKey handles keyboard input during image selection (add new or regenerate).
func (m *ImageGenModel) handleImageSelectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Go back to slide selection
		m.Step = ImageGenStepSlideSelect
		m.ImageOptions = nil
		m.ImageOptionIndex = 0
		return m, nil

	case "up", "k":
		if m.ImageOptionIndex > 0 {
			m.ImageOptionIndex--
		}
		return m, nil

	case "down", "j":
		if m.ImageOptionIndex < len(m.ImageOptions)-1 {
			m.ImageOptionIndex++
		}
		return m, nil

	case "enter":
		// Select the option and proceed
		if m.ImageOptionIndex >= 0 && m.ImageOptionIndex < len(m.ImageOptions) {
			option := m.ImageOptions[m.ImageOptionIndex]
			if option.IsAddNew {
				// Adding new image, proceed to prompt input
				m.SelectedImage = nil
				m.Prompt = ""
			} else {
				// Regenerating existing image, pre-fill prompt
				m.SelectedImage = option.AIImage
				if option.AIImage != nil {
					m.Prompt = option.AIImage.Prompt
				}
			}
			m.Step = ImageGenStepPrompt
		}
		return m, nil
	}

	return m, nil
}

// View implements tea.Model.
func (m *ImageGenModel) View() string {
	switch m.Step {
	case ImageGenStepSlideSelect:
		return m.viewSlideSelect()
	case ImageGenStepImageSelect:
		return m.viewImageSelect()
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

		// Build AI image indicator if slide has AI images
		aiIndicator := ""
		if slide.HasAIImages {
			if slide.AIImageCount == 1 {
				aiIndicator = " [has 1 AI image]"
			} else {
				aiIndicator = fmt.Sprintf(" [has %d AI images]", slide.AIImageCount)
			}
		}

		if i == m.SelectedIndex {
			// Selected item
			selectedStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary)
			numStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)
			indicatorStyle := lipgloss.NewStyle().
				Foreground(ColorMuted).
				Italic(true)

			b.WriteString(numStyle.Render("> "))
			b.WriteString(numStyle.Render(slideNum))
			b.WriteString(" ")
			b.WriteString(selectedStyle.Render(slide.Title))
			if aiIndicator != "" {
				b.WriteString(indicatorStyle.Render(aiIndicator))
			}
		} else {
			// Unselected item
			unselectedStyle := lipgloss.NewStyle().
				Foreground(ColorWhite)
			numStyle := lipgloss.NewStyle().
				Foreground(ColorMuted)
			indicatorStyle := lipgloss.NewStyle().
				Foreground(ColorMuted).
				Italic(true)

			b.WriteString("  ")
			b.WriteString(numStyle.Render(slideNum))
			b.WriteString(" ")
			b.WriteString(unselectedStyle.Render(slide.Title))
			if aiIndicator != "" {
				b.WriteString(indicatorStyle.Render(aiIndicator))
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

// buildImageOptions creates the image selection options for the selected slide.
func (m *ImageGenModel) buildImageOptions() {
	slide := m.GetSelectedSlide()
	if slide == nil {
		m.ImageOptions = nil
		return
	}

	options := make([]ImageSelectOption, 0, len(slide.AIImages)+1)

	// Always add "Add new image" as the first option
	options = append(options, ImageSelectOption{
		IsAddNew: true,
		Label:    "Add new image",
	})

	// Add regenerate options for each existing AI image
	for i := range slide.AIImages {
		prompt := slide.AIImages[i].Prompt
		// Truncate long prompts for display
		displayPrompt := prompt
		if len(displayPrompt) > 40 {
			displayPrompt = displayPrompt[:37] + "..."
		}
		options = append(options, ImageSelectOption{
			IsAddNew: false,
			AIImage:  &slide.AIImages[i],
			Label:    fmt.Sprintf("Regenerate: %s", displayPrompt),
		})
	}

	m.ImageOptions = options
	m.ImageOptionIndex = 0
}

// viewImageSelect renders the image selection view (add new or regenerate).
func (m *ImageGenModel) viewImageSelect() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("🖼  Select Action"))
	b.WriteString("\n\n")

	// Show selected slide info
	slide := m.GetSelectedSlide()
	if slide != nil {
		slideInfoStyle := lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)
		b.WriteString(slideInfoStyle.Render(fmt.Sprintf("Slide %d: %s", slide.Index+1, slide.Title)))
		b.WriteString("\n\n")
	}

	// Option list
	for i, option := range m.ImageOptions {
		if i == m.ImageOptionIndex {
			// Selected item
			selectedStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorSecondary)
			indicatorStyle := lipgloss.NewStyle().
				Bold(true).
				Foreground(ColorPrimary)

			b.WriteString(indicatorStyle.Render("> "))
			b.WriteString(selectedStyle.Render(option.Label))
		} else {
			// Unselected item
			unselectedStyle := lipgloss.NewStyle().
				Foreground(ColorWhite)

			b.WriteString("  ")
			b.WriteString(unselectedStyle.Render(option.Label))
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
		"%s/%s navigate • %s select • %s back",
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
