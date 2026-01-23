// Package tui provides terminal user interface components using Bubble Tea.
package tui

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// addStep represents the current step in the add slide wizard.
type addStep int

const (
	addStepLayout addStep = iota
	addStepContent
	addStepDone
)

// Layout represents a slide layout option.
type Layout struct {
	Name        string
	Description string
	ASCII       string
	Fields      []LayoutField
}

// LayoutField represents a field to prompt for in a layout.
type LayoutField struct {
	Name        string
	Placeholder string
	Multiline   bool
}

// AvailableLayouts lists the layouts available for new slides.
var AvailableLayouts = []Layout{
	{
		Name:        "title",
		Description: "Title slide with centered heading",
		ASCII: `
┌─────────────────────┐
│                     │
│      # Title        │
│      subtitle       │
│                     │
└─────────────────────┘`,
		Fields: []LayoutField{
			{Name: "Title", Placeholder: "My Title"},
			{Name: "Subtitle", Placeholder: "Optional subtitle"},
		},
	},
	{
		Name:        "section",
		Description: "Section header for topic transitions",
		ASCII: `
┌─────────────────────┐
│                     │
│                     │
│    ## Section       │
│                     │
│                     │
└─────────────────────┘`,
		Fields: []LayoutField{
			{Name: "Section Title", Placeholder: "Section Name"},
		},
	},
	{
		Name:        "default",
		Description: "Standard content slide",
		ASCII: `
┌─────────────────────┐
│ ## Header           │
│                     │
│ - Point one         │
│ - Point two         │
│ - Point three       │
└─────────────────────┘`,
		Fields: []LayoutField{
			{Name: "Header", Placeholder: "Slide Header"},
			{Name: "Content", Placeholder: "Bullet points or paragraphs", Multiline: true},
		},
	},
	{
		Name:        "two-column",
		Description: "Side-by-side content columns",
		ASCII: `
┌─────────────────────┐
│ ## Header           │
│          ┃          │
│  Left    ┃   Right  │
│  column  ┃   column │
│          ┃          │
└─────────────────────┘`,
		Fields: []LayoutField{
			{Name: "Header", Placeholder: "Optional Header"},
			{Name: "Left Column", Placeholder: "Left side content", Multiline: true},
			{Name: "Right Column", Placeholder: "Right side content", Multiline: true},
		},
	},
	{
		Name:        "code-focus",
		Description: "Full-width code block",
		ASCII: `
┌─────────────────────┐
│ ┌─────────────────┐ │
│ │ func main() {   │ │
│ │   // code here  │ │
│ │ }               │ │
│ └─────────────────┘ │
└─────────────────────┘`,
		Fields: []LayoutField{
			{Name: "Language", Placeholder: "go, python, javascript..."},
			{Name: "Code", Placeholder: "Your code here", Multiline: true},
		},
	},
	{
		Name:        "quote",
		Description: "Styled blockquote with attribution",
		ASCII: `
┌─────────────────────┐
│                     │
│  "Quote text..."    │
│                     │
│        — Author     │
│                     │
└─────────────────────┘`,
		Fields: []LayoutField{
			{Name: "Quote", Placeholder: "The quote text"},
			{Name: "Author", Placeholder: "Author name"},
		},
	},
	{
		Name:        "big-stat",
		Description: "Large number with description",
		ASCII: `
┌─────────────────────┐
│                     │
│        99%          │
│                     │
│    of developers    │
│    love this tool   │
└─────────────────────┘`,
		Fields: []LayoutField{
			{Name: "Statistic", Placeholder: "99%"},
			{Name: "Description", Placeholder: "Description of the statistic"},
		},
	},
}

// AddModel is the Bubble Tea model for adding slides interactively.
type AddModel struct { //nolint:govet // textinput.Model has complex alignment
	// Inputs
	textInputs []textinput.Model

	// State (errors first, then strings, then ints/bools for alignment)
	err error

	// File to append to
	filePath string

	// Window dimensions
	windowWidth  int
	windowHeight int

	// Current step and indices
	layoutIndex int
	fieldIndex  int
	step        addStep

	// Flags
	quitting bool
	done     bool
}

// AddModelResult contains the result of the add slide wizard.
type AddModelResult struct {
	FilePath string
	Layout   string
	Markdown string
	Aborted  bool
}

// NewAddModel creates a new AddModel for the slide builder.
func NewAddModel(filePath string) AddModel {
	return AddModel{
		filePath: filePath,
		step:     addStepLayout,
	}
}

// Init implements tea.Model.
func (m AddModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m AddModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			m.quitting = true
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
	}

	switch m.step {
	case addStepLayout:
		return m.updateLayout(msg)
	case addStepContent:
		return m.updateContent(msg)
	case addStepDone:
		return m, tea.Quit
	}

	return m, nil
}

func (m AddModel) updateLayout(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.layoutIndex > 0 {
				m.layoutIndex--
			}
			return m, nil
		case "down", "j":
			if m.layoutIndex < len(AvailableLayouts)-1 {
				m.layoutIndex++
			}
			return m, nil
		case "enter":
			m.step = addStepContent
			m.initTextInputs()
			if len(m.textInputs) > 0 {
				return m, textinput.Blink
			}
			return m, nil
		}
	}

	return m, nil
}

func (m *AddModel) initTextInputs() {
	layout := AvailableLayouts[m.layoutIndex]
	m.textInputs = make([]textinput.Model, len(layout.Fields))

	for i, field := range layout.Fields {
		ti := textinput.New()
		ti.Placeholder = field.Placeholder
		ti.CharLimit = 500
		ti.Width = 50
		if i == 0 {
			ti.Focus()
		}
		m.textInputs[i] = ti
	}
	m.fieldIndex = 0
}

func (m AddModel) updateContent(msg tea.Msg) (tea.Model, tea.Cmd) {
	layout := AvailableLayouts[m.layoutIndex]

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "tab", "down":
			// Move to next field
			if m.fieldIndex < len(m.textInputs)-1 {
				m.textInputs[m.fieldIndex].Blur()
				m.fieldIndex++
				m.textInputs[m.fieldIndex].Focus()
				return m, textinput.Blink
			}
			return m, nil
		case "shift+tab", "up":
			// Move to previous field
			if m.fieldIndex > 0 {
				m.textInputs[m.fieldIndex].Blur()
				m.fieldIndex--
				m.textInputs[m.fieldIndex].Focus()
				return m, textinput.Blink
			}
			return m, nil
		case "enter":
			// If it's a multiline field, allow enter
			if layout.Fields[m.fieldIndex].Multiline {
				// Add newline to the current input
				current := m.textInputs[m.fieldIndex].Value()
				m.textInputs[m.fieldIndex].SetValue(current + "\n")
				return m, nil
			}
			// If last field, finalize
			if m.fieldIndex == len(m.textInputs)-1 {
				return m.finalize()
			}
			// Move to next field
			m.textInputs[m.fieldIndex].Blur()
			m.fieldIndex++
			m.textInputs[m.fieldIndex].Focus()
			return m, textinput.Blink
		case "ctrl+d":
			// Finalize from any field
			return m.finalize()
		}
	}

	var cmd tea.Cmd
	m.textInputs[m.fieldIndex], cmd = m.textInputs[m.fieldIndex].Update(msg)
	return m, cmd
}

func (m AddModel) finalize() (tea.Model, tea.Cmd) {
	// Generate markdown
	markdown := m.generateMarkdown()

	// If we have a file path, append to it
	if m.filePath != "" {
		err := appendToFile(m.filePath, markdown)
		if err != nil {
			m.err = err
			m.step = addStepDone
			return m, tea.Quit
		}
	}

	m.done = true
	m.step = addStepDone
	return m, tea.Quit
}

func (m AddModel) generateMarkdown() string {
	layout := AvailableLayouts[m.layoutIndex]
	values := make([]string, len(m.textInputs))
	for i, ti := range m.textInputs {
		values[i] = strings.TrimSpace(ti.Value())
	}

	return GenerateSlideMarkdown(layout.Name, values)
}

// GenerateSlideMarkdown generates markdown for a slide based on layout and field values.
// Exported for testing.
func GenerateSlideMarkdown(layoutName string, values []string) string {
	var b strings.Builder

	// Start with slide separator
	b.WriteString("\n---\n\n")

	switch layoutName {
	case "title":
		title := getValueOrDefault(values, 0, "Title")
		subtitle := getValueOrDefault(values, 1, "")
		b.WriteString(fmt.Sprintf("# %s\n", title))
		if subtitle != "" {
			b.WriteString(fmt.Sprintf("\n%s\n", subtitle))
		}

	case "section":
		section := getValueOrDefault(values, 0, "Section")
		b.WriteString(fmt.Sprintf("## %s\n", section))

	case "default":
		header := getValueOrDefault(values, 0, "Header")
		content := getValueOrDefault(values, 1, "- Point one\n- Point two")
		b.WriteString(fmt.Sprintf("## %s\n\n", header))
		b.WriteString(formatContent(content))
		b.WriteString("\n")

	case "two-column":
		header := getValueOrDefault(values, 0, "")
		left := getValueOrDefault(values, 1, "Left content")
		right := getValueOrDefault(values, 2, "Right content")
		if header != "" {
			b.WriteString(fmt.Sprintf("## %s\n\n", header))
		}
		b.WriteString("|||\n\n")
		b.WriteString(formatContent(left))
		b.WriteString("\n\n|||\n\n")
		b.WriteString(formatContent(right))
		b.WriteString("\n")

	case "code-focus":
		b.WriteString("<!--\nlayout: code-focus\n-->\n\n")
		language := getValueOrDefault(values, 0, "")
		code := getValueOrDefault(values, 1, "// Your code here")
		b.WriteString(fmt.Sprintf("```%s\n%s\n```\n", language, code))

	case "quote":
		b.WriteString("<!--\nlayout: quote\n-->\n\n")
		quote := getValueOrDefault(values, 0, "Your quote here")
		author := getValueOrDefault(values, 1, "")
		b.WriteString(fmt.Sprintf("> %q\n", quote))
		if author != "" {
			b.WriteString(fmt.Sprintf(">\n> — %s\n", author))
		}

	case "big-stat":
		b.WriteString("<!--\nlayout: big-stat\n-->\n\n")
		stat := getValueOrDefault(values, 0, "100%")
		description := getValueOrDefault(values, 1, "Description")
		b.WriteString(fmt.Sprintf("# %s\n\n%s\n", stat, description))
	}

	return b.String()
}

func getValueOrDefault(values []string, index int, defaultVal string) string {
	if index < len(values) && values[index] != "" {
		return values[index]
	}
	return defaultVal
}

func formatContent(content string) string {
	// Ensure each line is properly formatted
	lines := strings.Split(content, "\n")
	var result strings.Builder
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			result.WriteString(line)
		}
		if i < len(lines)-1 {
			result.WriteString("\n")
		}
	}
	return result.String()
}

func appendToFile(filePath, content string) error {
	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	_, err = f.WriteString(content)
	if err != nil {
		return fmt.Errorf("failed to write to file: %w", err)
	}

	return nil
}

// GetResult returns the result of the wizard.
func (m AddModel) GetResult() AddModelResult {
	return AddModelResult{
		FilePath: m.filePath,
		Layout:   AvailableLayouts[m.layoutIndex].Name,
		Markdown: m.generateMarkdown(),
		Aborted:  m.quitting,
	}
}

// GetError returns any error that occurred.
func (m AddModel) GetError() error {
	return m.err
}

// WasAborted returns true if the wizard was aborted.
func (m AddModel) WasAborted() bool {
	return m.quitting
}

// View implements tea.Model.
func (m AddModel) View() string {
	if m.quitting {
		return RenderMuted("Aborted.\n")
	}

	if m.done {
		return m.viewSuccess()
	}

	if m.err != nil {
		return RenderError(fmt.Sprintf("Error: %v\n", m.err))
	}

	var b strings.Builder

	// Title
	b.WriteString(RenderTitle("Add New Slide"))
	b.WriteString("\n\n")

	switch m.step {
	case addStepLayout:
		b.WriteString(m.viewLayoutStep())
	case addStepContent:
		b.WriteString(m.viewContentStep())
	}

	return b.String()
}

func (m AddModel) viewLayoutStep() string {
	var b strings.Builder
	b.WriteString(RenderSubtitle("Select a layout:"))
	b.WriteString("\n\n")

	for i, layout := range AvailableLayouts {
		if i == m.layoutIndex {
			b.WriteString(RenderSelected(layout.Name))
			b.WriteString("\n")
			// Show description for selected layout
			descStyle := lipgloss.NewStyle().
				Foreground(ColorMuted).
				PaddingLeft(4)
			b.WriteString(descStyle.Render(layout.Description))
			b.WriteString("\n")
			// Show ASCII preview for selected layout
			previewStyle := lipgloss.NewStyle().
				Foreground(ColorPrimary).
				PaddingLeft(4)
			b.WriteString(previewStyle.Render(layout.ASCII))
		} else {
			b.WriteString(RenderUnselected(layout.Name))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(RenderHelp("Use ↑/↓ or j/k to navigate, Enter to select, Esc to cancel"))

	return b.String()
}

func (m AddModel) viewContentStep() string {
	var b strings.Builder
	layout := AvailableLayouts[m.layoutIndex]

	b.WriteString(RenderSubtitle(fmt.Sprintf("Enter content for %s slide:", layout.Name)))
	b.WriteString("\n\n")

	for i, field := range layout.Fields {
		// Field label
		labelStyle := lipgloss.NewStyle().Bold(true)
		if i == m.fieldIndex {
			labelStyle = labelStyle.Foreground(ColorPrimary)
		} else {
			labelStyle = labelStyle.Foreground(ColorMuted)
		}
		b.WriteString(labelStyle.Render(field.Name + ":"))
		b.WriteString("\n")

		// Text input
		if i < len(m.textInputs) {
			b.WriteString(m.textInputs[i].View())
		}
		b.WriteString("\n")

		// Show multiline hint if applicable
		if field.Multiline && i == m.fieldIndex {
			b.WriteString(RenderMuted("  (Enter adds newline, Ctrl+D to finish)"))
			b.WriteString("\n")
		}
		b.WriteString("\n")
	}

	b.WriteString(RenderHelp("Tab/↓ next field, Shift+Tab/↑ previous, Ctrl+D to finish, Esc to cancel"))

	return b.String()
}

func (m AddModel) viewSuccess() string {
	var b strings.Builder

	b.WriteString(RenderSuccess("Slide added successfully!"))
	b.WriteString("\n\n")

	layout := AvailableLayouts[m.layoutIndex]
	b.WriteString(fmt.Sprintf("  Layout: %s\n", RenderHighlight(layout.Name)))
	if m.filePath != "" {
		b.WriteString(fmt.Sprintf("  File: %s\n", RenderHighlight(m.filePath)))
	}

	b.WriteString("\n")
	b.WriteString(RenderSubtitle("Generated markdown:"))
	b.WriteString("\n")

	// Show preview of generated markdown
	markdown := m.generateMarkdown()
	previewStyle := lipgloss.NewStyle().
		Foreground(ColorMuted).
		PaddingLeft(2)
	// Truncate if too long
	lines := strings.Split(markdown, "\n")
	if len(lines) > 15 {
		lines = append(lines[:15], "...")
	}
	b.WriteString(previewStyle.Render(strings.Join(lines, "\n")))

	b.WriteString("\n\n")

	return b.String()
}

// RunAddWizard runs the add slide wizard and returns the result.
func RunAddWizard(filePath string) (AddModelResult, error) {
	model := NewAddModel(filePath)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		return AddModelResult{}, err
	}

	m, ok := finalModel.(AddModel)
	if !ok {
		return AddModelResult{}, fmt.Errorf("unexpected model type: %T", finalModel)
	}
	if m.err != nil {
		return AddModelResult{}, m.err
	}

	return m.GetResult(), nil
}
