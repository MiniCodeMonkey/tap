// Package tui provides terminal user interface components using Bubble Tea.
package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// FilePickerModel is a Bubble Tea model for selecting markdown files.
type FilePickerModel struct {
	files        []string
	cursor       int
	selected     string
	quitting     bool
	windowWidth  int
	windowHeight int
}

// FilePickerResult contains the result of the file picker.
type FilePickerResult struct {
	File    string
	Aborted bool
}

// NewFilePickerModel creates a new file picker model.
// It searches for .md files in the current directory.
func NewFilePickerModel() FilePickerModel {
	files := findMarkdownFiles()
	return FilePickerModel{
		files:  files,
		cursor: 0,
	}
}

// findMarkdownFiles returns a list of markdown files in the current directory,
// sorted by modification time (most recent first).
func findMarkdownFiles() []string {
	entries, err := os.ReadDir(".")
	if err != nil {
		return nil
	}

	type fileInfo struct {
		name    string
		modTime int64
	}

	var mdFiles []fileInfo
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(strings.ToLower(name), ".md") {
			info, err := entry.Info()
			if err != nil {
				continue
			}
			mdFiles = append(mdFiles, fileInfo{
				name:    name,
				modTime: info.ModTime().UnixNano(),
			})
		}
	}

	// Sort by modification time, most recent first
	sort.Slice(mdFiles, func(i, j int) bool {
		return mdFiles[i].modTime > mdFiles[j].modTime
	})

	result := make([]string, len(mdFiles))
	for i, f := range mdFiles {
		result[i] = f.name
	}
	return result
}

// HasFiles returns true if there are files to select from.
func (m FilePickerModel) HasFiles() bool {
	return len(m.files) > 0
}

// Init implements tea.Model.
func (m FilePickerModel) Init() tea.Cmd {
	return nil
}

// Update implements tea.Model.
func (m FilePickerModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc", "q":
			m.quitting = true
			return m, tea.Quit
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.files)-1 {
				m.cursor++
			}
		case "enter":
			if len(m.files) > 0 {
				m.selected = m.files[m.cursor]
			}
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.windowWidth = msg.Width
		m.windowHeight = msg.Height
	}

	return m, nil
}

// View implements tea.Model.
func (m FilePickerModel) View() string {
	if m.quitting {
		return ""
	}

	if m.selected != "" {
		return ""
	}

	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(RenderTitle("Select a presentation"))
	b.WriteString("\n\n")

	// Show files
	maxVisible := 10
	start := 0
	if m.cursor >= maxVisible {
		start = m.cursor - maxVisible + 1
	}
	end := start + maxVisible
	if end > len(m.files) {
		end = len(m.files)
	}

	// Show scroll indicator if needed
	if start > 0 {
		b.WriteString(RenderMuted("  ↑ more files above\n"))
	}

	for i := start; i < end; i++ {
		file := m.files[i]
		if i == m.cursor {
			b.WriteString(RenderSelected(file))
		} else {
			b.WriteString(RenderUnselected(file))
		}
		b.WriteString("\n")
	}

	// Show scroll indicator if needed
	if end < len(m.files) {
		b.WriteString(RenderMuted("  ↓ more files below\n"))
	}

	b.WriteString("\n")
	b.WriteString(RenderHelp("↑/↓ navigate • enter select • esc cancel"))
	b.WriteString("\n")

	return b.String()
}

// GetResult returns the selected file or empty if aborted.
func (m FilePickerModel) GetResult() FilePickerResult {
	return FilePickerResult{
		File:    m.selected,
		Aborted: m.quitting || m.selected == "",
	}
}

// RunFilePicker runs the file picker and returns the selected file.
// Returns empty result if no files found or user cancelled.
func RunFilePicker() (FilePickerResult, error) {
	model := NewFilePickerModel()

	if !model.HasFiles() {
		return FilePickerResult{Aborted: true}, nil
	}

	p := tea.NewProgram(model)
	finalModel, err := p.Run()
	if err != nil {
		return FilePickerResult{}, err
	}

	m, ok := finalModel.(FilePickerModel)
	if !ok {
		return FilePickerResult{Aborted: true}, nil
	}

	return m.GetResult(), nil
}

// RenderNoFilesError returns a formatted error message when no markdown files are found.
func RenderNoFilesError() string {
	var b strings.Builder

	b.WriteString("\n")
	b.WriteString(RenderError("No markdown files found"))
	b.WriteString("\n\n")

	b.WriteString(RenderMuted("  To get started:\n\n"))
	b.WriteString("  1. Create a new presentation:\n")
	b.WriteString(RenderHighlight("     tap new"))
	b.WriteString("\n\n")
	b.WriteString("  2. Or specify a file directly:\n")
	b.WriteString(RenderHighlight("     tap dev slides.md"))
	b.WriteString("\n\n")

	cwd, _ := filepath.Abs(".")
	b.WriteString(RenderMuted("  Current directory: " + cwd))
	b.WriteString("\n\n")

	return b.String()
}
