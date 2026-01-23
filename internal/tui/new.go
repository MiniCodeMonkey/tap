// Package tui provides terminal user interface components using Bubble Tea.
package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// newStep represents the current step in the new presentation wizard.
type newStep int

const (
	stepTitle newStep = iota
	stepTheme
	stepFilename
	stepDone
)

// Theme represents a presentation theme option.
type Theme struct {
	Name        string
	Description string
}

// AvailableThemes lists the themes available for new presentations.
var AvailableThemes = []Theme{
	{Name: "minimal", Description: "Clean Apple-style aesthetics with white background"},
	{Name: "gradient", Description: "Modern colorful gradients with glassmorphism"},
	{Name: "terminal", Description: "Hacker aesthetic with dark background and green text"},
	{Name: "brutalist", Description: "Bold, geometric, high contrast design"},
	{Name: "keynote", Description: "Professional style with subtle shadows"},
}

// NewModel is the Bubble Tea model for creating new presentations.
type NewModel struct { //nolint:govet // textinput.Model has complex alignment
	// User inputs
	titleInput    textinput.Model
	filenameInput textinput.Model

	// Error state
	err error

	// State strings (ordered by size for alignment)
	outputPath        string
	prefilledTheme    string
	prefilledFilename string

	// State integers
	windowWidth  int
	windowHeight int
	themeIndex   int
	step         newStep

	// State booleans
	quitting bool
	done     bool
}

// NewModelResult contains the result of the new presentation wizard.
type NewModelResult struct {
	Title    string
	Theme    string
	Filename string
	Aborted  bool
}

// NewNewModel creates a new NewModel for the presentation wizard.
func NewNewModel(prefilledTheme, prefilledFilename string) NewModel {
	ti := textinput.New()
	ti.Placeholder = "My Awesome Presentation"
	ti.Focus()
	ti.CharLimit = 100
	ti.Width = 40

	fi := textinput.New()
	fi.Placeholder = "presentation.md"
	fi.CharLimit = 100
	fi.Width = 40

	return NewModel{
		titleInput:        ti,
		filenameInput:     fi,
		step:              stepTitle,
		prefilledTheme:    prefilledTheme,
		prefilledFilename: prefilledFilename,
	}
}

// Init implements tea.Model.
func (m NewModel) Init() tea.Cmd {
	return textinput.Blink
}

// Update implements tea.Model.
func (m NewModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
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
	case stepTitle:
		return m.updateTitle(msg)
	case stepTheme:
		return m.updateTheme(msg)
	case stepFilename:
		return m.updateFilename(msg)
	case stepDone:
		return m, tea.Quit
	}

	return m, nil
}

func (m NewModel) updateTitle(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			title := strings.TrimSpace(m.titleInput.Value())
			if title == "" {
				title = "My Presentation"
			}
			m.titleInput.SetValue(title)

			// Skip to filename if theme was prefilled
			if m.prefilledTheme != "" {
				m.themeIndex = m.findThemeIndex(m.prefilledTheme)
				m.step = stepFilename
				// Set default filename based on title
				m.setDefaultFilename()
				m.filenameInput.Focus()
				return m, textinput.Blink
			}

			m.step = stepTheme
			return m, nil
		}
	}

	var cmd tea.Cmd
	m.titleInput, cmd = m.titleInput.Update(msg)
	return m, cmd
}

func (m NewModel) updateTheme(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.themeIndex > 0 {
				m.themeIndex--
			}
			return m, nil
		case "down", "j":
			if m.themeIndex < len(AvailableThemes)-1 {
				m.themeIndex++
			}
			return m, nil
		case "enter":
			// Skip to done if filename was prefilled
			if m.prefilledFilename != "" {
				m.filenameInput.SetValue(m.prefilledFilename)
				return m.finalize()
			}

			m.step = stepFilename
			m.setDefaultFilename()
			m.filenameInput.Focus()
			return m, textinput.Blink
		}
	}

	return m, nil
}

func (m NewModel) updateFilename(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "enter":
			return m.finalize()
		}
	}

	var cmd tea.Cmd
	m.filenameInput, cmd = m.filenameInput.Update(msg)
	return m, cmd
}

func (m NewModel) finalize() (tea.Model, tea.Cmd) {
	filename := strings.TrimSpace(m.filenameInput.Value())
	if filename == "" {
		filename = m.generateDefaultFilename()
	}
	if !strings.HasSuffix(filename, ".md") {
		filename += ".md"
	}
	m.filenameInput.SetValue(filename)

	// Generate the markdown file
	content := m.generateMarkdown()
	err := os.WriteFile(filename, []byte(content), 0644)
	if err != nil {
		m.err = err
		m.step = stepDone
		return m, tea.Quit
	}

	m.outputPath = filename
	m.done = true
	m.step = stepDone
	return m, tea.Quit
}

func (m *NewModel) setDefaultFilename() {
	if m.prefilledFilename != "" {
		m.filenameInput.SetValue(m.prefilledFilename)
		return
	}
	m.filenameInput.SetValue(m.generateDefaultFilename())
}

func (m NewModel) generateDefaultFilename() string {
	title := m.titleInput.Value()
	if title == "" {
		title = "presentation"
	}
	// Convert title to filename-friendly format
	filename := strings.ToLower(title)
	filename = strings.ReplaceAll(filename, " ", "-")
	// Remove non-alphanumeric characters except hyphens
	var result strings.Builder
	for _, r := range filename {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			result.WriteRune(r)
		}
	}
	filename = result.String()
	// Collapse multiple hyphens
	for strings.Contains(filename, "--") {
		filename = strings.ReplaceAll(filename, "--", "-")
	}
	filename = strings.Trim(filename, "-")
	if filename == "" {
		filename = "presentation"
	}
	return filename + ".md"
}

func (m NewModel) findThemeIndex(themeName string) int {
	themeName = strings.ToLower(themeName)
	for i, t := range AvailableThemes {
		if strings.ToLower(t.Name) == themeName {
			return i
		}
	}
	return 0 // Default to first theme
}

// GetResult returns the result of the wizard.
func (m NewModel) GetResult() NewModelResult {
	return NewModelResult{
		Title:    m.titleInput.Value(),
		Theme:    AvailableThemes[m.themeIndex].Name,
		Filename: m.filenameInput.Value(),
		Aborted:  m.quitting,
	}
}

// GetOutputPath returns the path of the created file.
func (m NewModel) GetOutputPath() string {
	return m.outputPath
}

// GetError returns any error that occurred.
func (m NewModel) GetError() error {
	return m.err
}

// WasAborted returns true if the wizard was aborted.
func (m NewModel) WasAborted() bool {
	return m.quitting
}

// View implements tea.Model.
func (m NewModel) View() string {
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
	b.WriteString(RenderTitle("Create New Presentation"))
	b.WriteString("\n\n")

	switch m.step {
	case stepTitle:
		b.WriteString(m.viewTitleStep())
	case stepTheme:
		b.WriteString(m.viewThemeStep())
	case stepFilename:
		b.WriteString(m.viewFilenameStep())
	}

	// Help text
	b.WriteString("\n")
	b.WriteString(RenderHelp("Press Enter to continue, Esc to cancel"))

	return b.String()
}

func (m NewModel) viewTitleStep() string {
	var b strings.Builder
	b.WriteString(RenderSubtitle("What's the title of your presentation?"))
	b.WriteString("\n")
	b.WriteString(m.titleInput.View())
	return b.String()
}

func (m NewModel) viewThemeStep() string {
	var b strings.Builder
	b.WriteString(RenderSubtitle("Select a theme:"))
	b.WriteString("\n\n")

	for i, theme := range AvailableThemes {
		if i == m.themeIndex {
			b.WriteString(RenderSelected(theme.Name))
			b.WriteString("\n")
			// Show description for selected theme
			descStyle := lipgloss.NewStyle().
				Foreground(ColorMuted).
				PaddingLeft(4)
			b.WriteString(descStyle.Render(theme.Description))
		} else {
			b.WriteString(RenderUnselected(theme.Name))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(RenderHelp("Use ↑/↓ or j/k to navigate"))

	return b.String()
}

func (m NewModel) viewFilenameStep() string {
	var b strings.Builder
	b.WriteString(RenderSubtitle("Output filename:"))
	b.WriteString("\n")
	b.WriteString(m.filenameInput.View())
	return b.String()
}

func (m NewModel) viewSuccess() string {
	var b strings.Builder

	b.WriteString(RenderSuccess("Presentation created successfully!"))
	b.WriteString("\n\n")

	// File info
	absPath, _ := filepath.Abs(m.outputPath)
	b.WriteString(fmt.Sprintf("  File: %s\n", RenderHighlight(absPath)))
	b.WriteString(fmt.Sprintf("  Title: %s\n", m.titleInput.Value()))
	b.WriteString(fmt.Sprintf("  Theme: %s\n", AvailableThemes[m.themeIndex].Name))

	b.WriteString("\n")
	b.WriteString(RenderSubtitle("Next steps:"))
	b.WriteString("\n")
	b.WriteString(fmt.Sprintf("  1. Edit %s to add your content\n", m.outputPath))
	b.WriteString(fmt.Sprintf("  2. Run %s to preview\n", RenderHighlight(fmt.Sprintf("tap dev %s", m.outputPath))))
	b.WriteString(fmt.Sprintf("  3. Run %s to build static files\n", RenderHighlight(fmt.Sprintf("tap build %s", m.outputPath))))

	b.WriteString("\n")

	return b.String()
}

// generateMarkdown generates the starter markdown content.
func (m NewModel) generateMarkdown() string {
	title := m.titleInput.Value()
	theme := AvailableThemes[m.themeIndex].Name
	date := time.Now().Format("2006-01-02")

	return GenerateStarterMarkdown(title, theme, date, "Your Name")
}

// GenerateStarterMarkdown generates markdown content for a new presentation.
// Exported for testing.
func GenerateStarterMarkdown(title, theme, date, author string) string {
	return fmt.Sprintf(`---
title: "%s"
theme: %s
author: "%s"
date: "%s"
aspectRatio: "16:9"
transition: fade
---

# %s

%s

---

## Agenda

- Introduction
- Main Content
- Conclusion

<!-- pause -->

Take your time to go through each section.

---

## Key Points

1. First important point
2. Second important point
3. Third important point

---

<!--
layout: code-focus
-->

`+"```go"+`
package main

import "fmt"

func main() {
    fmt.Println("Hello, Tap!")
}
`+"```"+`

---

## Two Column Layout

|||

**Left Column**

- Point A
- Point B

|||

**Right Column**

- Point C
- Point D

---

<!--
layout: quote
-->

> "The best way to predict the future is to invent it."
>
> — Alan Kay

---

# Thank You!

Questions?

`, title, theme, author, date, title, author)
}

// RunNewWizard runs the new presentation wizard and returns the result.
func RunNewWizard(prefilledTheme, prefilledFilename string) (NewModelResult, error) {
	model := NewNewModel(prefilledTheme, prefilledFilename)
	p := tea.NewProgram(model)

	finalModel, err := p.Run()
	if err != nil {
		return NewModelResult{}, err
	}

	m, ok := finalModel.(NewModel)
	if !ok {
		return NewModelResult{}, fmt.Errorf("unexpected model type: %T", finalModel)
	}
	if m.err != nil {
		return NewModelResult{}, m.err
	}

	return m.GetResult(), nil
}
