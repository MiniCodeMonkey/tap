// Package tui provides terminal user interface components using Bubble Tea.
package tui

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textarea"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/tapsh/tap/internal/gemini"
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

// ImageGenerateResult is the result of an image generation operation.
type ImageGenerateResult struct {
	// ImageData contains the raw image bytes on success.
	ImageData []byte
	// ContentType is the MIME type of the generated image (e.g., "image/png").
	ContentType string
	// Error holds any error that occurred during generation.
	Error error
}

// imageGenerateMsg is sent when image generation completes.
type imageGenerateMsg struct {
	result ImageGenerateResult
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
	// promptInput is the textarea model for prompt input.
	promptInput textarea.Model
	// spinner is the spinner model for the generating step.
	spinner spinner.Model
	// GeneratedImage holds the result of a successful image generation.
	GeneratedImage *ImageGenerateResult
	// IsGenerating indicates whether generation is in progress.
	IsGenerating bool
}

// NewImageGenModel creates a new ImageGenModel for image generation.
func NewImageGenModel(markdownFile string) (*ImageGenModel, error) {
	// Initialize textarea for prompt input
	ta := textarea.New()
	ta.Placeholder = "Describe the image you want to generate..."
	ta.CharLimit = 2000
	ta.SetWidth(60)
	ta.SetHeight(5)
	ta.ShowLineNumbers = false

	// Initialize spinner for generating step
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = lipgloss.NewStyle().Foreground(ColorPrimary)

	m := &ImageGenModel{
		MarkdownFile:  markdownFile,
		SelectedIndex: 0,
		Step:          ImageGenStepSlideSelect,
		promptInput:   ta,
		spinner:       s,
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

	case imageGenerateMsg:
		return m.handleImageGenerateResult(msg.result)

	case spinner.TickMsg:
		// Update spinner when in generating step
		if m.Step == ImageGenStepGenerating {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}

	default:
		// Pass other messages to the textarea when in prompt step
		if m.Step == ImageGenStepPrompt {
			var cmd tea.Cmd
			m.promptInput, cmd = m.promptInput.Update(msg)
			return m, cmd
		}
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
	case ImageGenStepPrompt:
		return m.handlePromptKey(msg)
	case ImageGenStepGenerating:
		return m.handleGeneratingKey(msg)
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
			m.SelectedImage = nil
			m.Prompt = ""
			m.promptInput.SetValue("")
			m.promptInput.Focus()
			m.Step = ImageGenStepPrompt
			return m, textarea.Blink
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
				m.promptInput.SetValue("")
			} else {
				// Regenerating existing image, pre-fill prompt
				m.SelectedImage = option.AIImage
				if option.AIImage != nil {
					m.Prompt = option.AIImage.Prompt
					m.promptInput.SetValue(option.AIImage.Prompt)
				}
			}
			m.promptInput.Focus()
			m.Step = ImageGenStepPrompt
		}
		return m, textarea.Blink
	}

	return m, nil
}

// handlePromptKey handles keyboard input during prompt input.
func (m *ImageGenModel) handlePromptKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		// Go back to previous step
		m.promptInput.Blur()
		slide := m.GetSelectedSlide()
		if slide != nil && slide.HasAIImages {
			// Go back to image select
			m.Step = ImageGenStepImageSelect
		} else {
			// Go back to slide select
			m.Step = ImageGenStepSlideSelect
		}
		return m, nil

	case "ctrl+d":
		// Submit the prompt
		return m.submitPrompt()
	}

	// Check for enter key - submit if not empty
	if msg.Type == tea.KeyEnter {
		// Submit the prompt
		return m.submitPrompt()
	}

	// Pass other keys to the textarea
	var cmd tea.Cmd
	m.promptInput, cmd = m.promptInput.Update(msg)
	return m, cmd
}

// submitPrompt validates and submits the prompt, moving to the generating step.
func (m *ImageGenModel) submitPrompt() (tea.Model, tea.Cmd) {
	prompt := strings.TrimSpace(m.promptInput.Value())
	if prompt == "" {
		m.Error = "Prompt cannot be empty"
		return m, nil
	}

	m.Prompt = prompt
	m.Error = ""
	m.promptInput.Blur()
	m.Step = ImageGenStepGenerating
	m.IsGenerating = true

	// Start spinner and image generation
	return m, tea.Batch(m.spinner.Tick, m.generateImageCmd())
}

// generateImageCmd returns a command that generates an image using the Gemini API.
func (m *ImageGenModel) generateImageCmd() tea.Cmd {
	prompt := m.Prompt
	return func() tea.Msg {
		client, err := gemini.NewClientFromEnv()
		if err != nil {
			return imageGenerateMsg{result: ImageGenerateResult{Error: err}}
		}

		result, err := client.GenerateImage(context.Background(), prompt)
		if err != nil {
			return imageGenerateMsg{result: ImageGenerateResult{Error: err}}
		}

		return imageGenerateMsg{result: ImageGenerateResult{
			ImageData:   result.Data,
			ContentType: result.ContentType,
		}}
	}
}

// handleGeneratingKey handles keyboard input during image generation.
func (m *ImageGenModel) handleGeneratingKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Only allow cancel when there's an error (not while generating)
	if m.Error != "" {
		switch msg.String() {
		case "r":
			// Retry generation
			m.Error = ""
			m.IsGenerating = true
			return m, tea.Batch(m.spinner.Tick, m.generateImageCmd())

		case "esc":
			// Go back to prompt step
			m.Error = ""
			m.IsGenerating = false
			m.promptInput.Focus()
			m.Step = ImageGenStepPrompt
			return m, textarea.Blink
		}
	}
	// While generating, ignore all key presses
	return m, nil
}

// handleImageGenerateResult handles the result of image generation.
func (m *ImageGenModel) handleImageGenerateResult(result ImageGenerateResult) (tea.Model, tea.Cmd) {
	m.IsGenerating = false

	if result.Error != nil {
		// Show user-friendly error message
		m.Error = formatAPIError(result.Error)
		return m, nil
	}

	// Success - store the result and proceed to done step
	m.GeneratedImage = &result
	m.Step = ImageGenStepDone
	return m, nil
}

// formatAPIError converts an API error to a user-friendly message.
func formatAPIError(err error) string {
	if err == nil {
		return ""
	}

	// Check if it's a Gemini API error
	if apiErr, ok := err.(*gemini.APIError); ok {
		switch apiErr.Type {
		case gemini.ErrorTypeAuth:
			return "Authentication failed. Please check your GEMINI_API_KEY."
		case gemini.ErrorTypeRateLimit:
			return "Rate limit exceeded. Please wait a moment and try again."
		case gemini.ErrorTypeContentPolicy:
			return "The prompt was blocked by content policy. Please try a different prompt."
		case gemini.ErrorTypeInvalidRequest:
			return "Invalid request. Please try a different prompt."
		case gemini.ErrorTypeNoImage:
			return "No image was generated. Please try a different prompt."
		case gemini.ErrorTypeNetwork:
			return "Network error. Please check your connection and try again."
		case gemini.ErrorTypeServer:
			return "Server error. Please try again later."
		}
	}

	// Generic error
	return fmt.Sprintf("Failed to generate image: %v", err)
}

// View implements tea.Model.
func (m *ImageGenModel) View() string {
	switch m.Step {
	case ImageGenStepSlideSelect:
		return m.viewSlideSelect()
	case ImageGenStepImageSelect:
		return m.viewImageSelect()
	case ImageGenStepPrompt:
		return m.viewPrompt()
	case ImageGenStepGenerating:
		return m.viewGenerating()
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

// viewPrompt renders the prompt input view.
func (m *ImageGenModel) viewPrompt() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("🖼  Enter Image Prompt"))
	b.WriteString("\n\n")

	// Show selected slide info
	slide := m.GetSelectedSlide()
	if slide != nil {
		slideInfoStyle := lipgloss.NewStyle().
			Foreground(ColorMuted).
			Italic(true)
		b.WriteString(slideInfoStyle.Render(fmt.Sprintf("Slide %d: %s", slide.Index+1, slide.Title)))
		b.WriteString("\n")

		// Show if regenerating
		if m.SelectedImage != nil {
			b.WriteString(slideInfoStyle.Render("(Regenerating existing image)"))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	// Show error if any
	if m.Error != "" {
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff5555"))
		b.WriteString(errorStyle.Render("Error: " + m.Error))
		b.WriteString("\n\n")
	}

	// Textarea
	b.WriteString(m.promptInput.View())
	b.WriteString("\n\n")

	// Help text
	helpStyle := lipgloss.NewStyle().
		Foreground(ColorMuted)

	keyStyle := lipgloss.NewStyle().
		Foreground(ColorPrimary).
		Bold(true)

	help := fmt.Sprintf(
		"%s submit • %s submit • %s back",
		keyStyle.Render("enter"),
		keyStyle.Render("ctrl+d"),
		keyStyle.Render("esc"),
	)
	b.WriteString(helpStyle.Render(help))

	return b.String()
}

// viewGenerating renders the generating progress view.
func (m *ImageGenModel) viewGenerating() string {
	var b strings.Builder

	// Title
	titleStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(ColorPrimary).
		MarginBottom(1)

	b.WriteString(titleStyle.Render("🖼  Generating Image"))
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

	// Show prompt being used
	promptStyle := lipgloss.NewStyle().
		Foreground(ColorWhite)
	b.WriteString(promptStyle.Render("Prompt: "))

	// Truncate long prompts for display
	displayPrompt := m.Prompt
	if len(displayPrompt) > 60 {
		displayPrompt = displayPrompt[:57] + "..."
	}
	promptValueStyle := lipgloss.NewStyle().
		Foreground(ColorSecondary).
		Italic(true)
	b.WriteString(promptValueStyle.Render(displayPrompt))
	b.WriteString("\n\n")

	// Show error or progress
	if m.Error != "" {
		// Show error with retry/cancel options
		errorStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color("#ff5555"))
		b.WriteString(errorStyle.Render("Error: " + m.Error))
		b.WriteString("\n\n")

		// Help text for error state
		helpStyle := lipgloss.NewStyle().
			Foreground(ColorMuted)

		keyStyle := lipgloss.NewStyle().
			Foreground(ColorPrimary).
			Bold(true)

		help := fmt.Sprintf(
			"%s retry • %s back to prompt",
			keyStyle.Render("r"),
			keyStyle.Render("esc"),
		)
		b.WriteString(helpStyle.Render(help))
	} else {
		// Show spinner while generating
		progressStyle := lipgloss.NewStyle().
			Foreground(ColorSecondary)
		b.WriteString(m.spinner.View())
		b.WriteString(" ")
		b.WriteString(progressStyle.Render("Generating image..."))
		b.WriteString("\n\n")

		// Help text while generating
		helpStyle := lipgloss.NewStyle().
			Foreground(ColorMuted)
		b.WriteString(helpStyle.Render("Please wait, this may take a moment..."))
	}

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
