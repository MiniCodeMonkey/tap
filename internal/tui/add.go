// Package tui provides terminal user interface components using Bubble Tea.
package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/MiniCodeMonkey/tap/internal/deckedit"
	"github.com/MiniCodeMonkey/tap/internal/layouts"
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

// AvailableLayouts lists the layouts the wizard offers, built from the
// templates in internal/layouts.
var AvailableLayouts = buildAvailableLayouts()

func buildAvailableLayouts() []Layout {
	templates := layouts.Templates()
	result := make([]Layout, len(templates))
	for index, template := range templates {
		fields := make([]LayoutField, len(template.Fields))
		for fieldIndex, field := range template.Fields {
			fields[fieldIndex] = LayoutField{Name: field.Name, Placeholder: field.Placeholder, Multiline: field.Multiline}
		}
		result[index] = Layout{
			Name:        template.Name,
			Description: template.Description,
			ASCII:       layoutPreviews[template.Name],
			Fields:      fields,
		}
	}
	return result
}

// layoutPreviews is the ASCII sketch the wizard shows for each layout.
var layoutPreviews = map[string]string{
	"title": `
┌─────────────────────┐
│                     │
│      # Title        │
│      subtitle       │
│                     │
└─────────────────────┘`,
	"section": `
┌─────────────────────┐
│                     │
│                     │
│    ## Section       │
│                     │
│                     │
└─────────────────────┘`,
	"default": `
┌─────────────────────┐
│ ## Header           │
│                     │
│ - Point one         │
│ - Point two         │
│ - Point three       │
└─────────────────────┘`,
	"two-column": `
┌─────────────────────┐
│ ## Header           │
│          ┃          │
│  Left    ┃   Right  │
│  column  ┃   column │
│          ┃          │
└─────────────────────┘`,
	"code-focus": `
┌─────────────────────┐
│ ┌─────────────────┐ │
│ │ func main() {   │ │
│ │   // code here  │ │
│ │ }               │ │
│ └─────────────────┘ │
└─────────────────────┘`,
	"quote": `
┌─────────────────────┐
│                     │
│  "Quote text..."    │
│                     │
│        -- Author    │
│                     │
└─────────────────────┘`,
	"big-stat": `
┌─────────────────────┐
│                     │
│        99%          │
│                     │
│    of developers    │
│    love this tool   │
└─────────────────────┘`,
	"three-column": `
┌─────────────────────┐
│ ## Header           │
│      ┃       ┃      │
│ Left ┃Center ┃Right │
│      ┃       ┃      │
│      ┃       ┃      │
└─────────────────────┘`,
	"sidebar": `
┌─────────────────────┐
│ ## Header    ┃ Side │
│              ┃      │
│ Main content ┃ - a  │
│              ┃ - b  │
│              ┃      │
└─────────────────────┘`,
	"split-media": `
┌─────────────────────┐
│ ## Header ┃ ┌─────┐ │
│           ┃ │     │ │
│ Text      ┃ │ img │ │
│           ┃ │     │ │
│           ┃ └─────┘ │
└─────────────────────┘`,
	"cover": `
┌─────────────────────┐
│░░░░░░░░░░░░░░░░░░░░░│
│░░░░░░░░░░░░░░░░░░░░░│
│░░░░   # Title   ░░░░│
│░░░░░░░░░░░░░░░░░░░░░│
│░░░░░░░░░░░░░░░░░░░░░│
└─────────────────────┘`,
	"blank": `
┌─────────────────────┐
│                     │
│                     │
│      (empty)        │
│                     │
│                     │
└─────────────────────┘`,
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
	if m.filePath != "" {
		if err := deckedit.AppendSlide(m.filePath, m.slideBody()); err != nil {
			m.err = err
			m.step = addStepDone
			return m, tea.Quit
		}
	}

	m.done = true
	m.step = addStepDone
	return m, tea.Quit
}

// fieldValues returns the trimmed text of each field, in order.
func (m AddModel) fieldValues() []string {
	values := make([]string, len(m.textInputs))
	for index, input := range m.textInputs {
		values[index] = strings.TrimSpace(input.Value())
	}
	return values
}

// slideBody is the new slide's markdown, without the separator.
func (m AddModel) slideBody() string {
	body, _ := layouts.RenderSlide(AvailableLayouts[m.layoutIndex].Name, m.fieldValues())
	return body
}

func (m AddModel) generateMarkdown() string {
	return deckedit.SlideSeparator + m.slideBody()
}

// GenerateSlideMarkdown returns the text the wizard appends for a slide in
// layoutName: the slide separator, then the layout's template filled with
// values. An unknown layout gives the separator alone.
func GenerateSlideMarkdown(layoutName string, values []string) string {
	body, _ := layouts.RenderSlide(layoutName, values)
	return deckedit.SlideSeparator + body
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
