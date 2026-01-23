package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestNewAddModel(t *testing.T) {
	m := NewAddModel("test.md")

	if m.filePath != "test.md" {
		t.Errorf("expected filePath 'test.md', got '%s'", m.filePath)
	}
	if m.step != addStepLayout {
		t.Errorf("expected step addStepLayout, got %d", m.step)
	}
	if m.layoutIndex != 0 {
		t.Errorf("expected layoutIndex 0, got %d", m.layoutIndex)
	}
	if m.quitting {
		t.Error("expected quitting to be false")
	}
	if m.done {
		t.Error("expected done to be false")
	}
}

func TestAddModel_Init(t *testing.T) {
	m := NewAddModel("")
	cmd := m.Init()

	if cmd != nil {
		t.Error("expected Init to return nil")
	}
}

func TestAddModel_LayoutNavigation(t *testing.T) {
	m := NewAddModel("")

	// Navigate down
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newModel.(AddModel)
	if m.layoutIndex != 1 {
		t.Errorf("expected layoutIndex 1 after 'j', got %d", m.layoutIndex)
	}

	// Navigate down with arrow
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(AddModel)
	if m.layoutIndex != 2 {
		t.Errorf("expected layoutIndex 2 after down, got %d", m.layoutIndex)
	}

	// Navigate up
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newModel.(AddModel)
	if m.layoutIndex != 1 {
		t.Errorf("expected layoutIndex 1 after 'k', got %d", m.layoutIndex)
	}

	// Navigate up with arrow
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(AddModel)
	if m.layoutIndex != 0 {
		t.Errorf("expected layoutIndex 0 after up, got %d", m.layoutIndex)
	}

	// Try to go below 0
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(AddModel)
	if m.layoutIndex != 0 {
		t.Errorf("expected layoutIndex to stay at 0, got %d", m.layoutIndex)
	}
}

func TestAddModel_LayoutSelectionBounds(t *testing.T) {
	m := NewAddModel("")

	// Navigate to the last layout
	for i := 0; i < len(AvailableLayouts)-1; i++ {
		newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = newModel.(AddModel)
	}

	if m.layoutIndex != len(AvailableLayouts)-1 {
		t.Errorf("expected layoutIndex %d, got %d", len(AvailableLayouts)-1, m.layoutIndex)
	}

	// Try to go past the last layout
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(AddModel)
	if m.layoutIndex != len(AvailableLayouts)-1 {
		t.Errorf("expected layoutIndex to stay at %d, got %d", len(AvailableLayouts)-1, m.layoutIndex)
	}
}

func TestAddModel_SelectLayout(t *testing.T) {
	m := NewAddModel("")

	// Select the title layout (index 0)
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(AddModel)

	if m.step != addStepContent {
		t.Errorf("expected step addStepContent, got %d", m.step)
	}

	// Check that text inputs were initialized
	if len(m.textInputs) != len(AvailableLayouts[0].Fields) {
		t.Errorf("expected %d text inputs, got %d", len(AvailableLayouts[0].Fields), len(m.textInputs))
	}
}

func TestAddModel_Abort(t *testing.T) {
	tests := []struct {
		name string
		key  tea.KeyMsg
	}{
		{"ctrl+c", tea.KeyMsg{Type: tea.KeyCtrlC}},
		{"esc", tea.KeyMsg{Type: tea.KeyEsc}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewAddModel("")
			newModel, cmd := m.Update(tt.key)
			m = newModel.(AddModel)

			if !m.quitting {
				t.Error("expected quitting to be true")
			}
			if cmd == nil {
				t.Error("expected quit command")
			}
		})
	}
}

func TestAddModel_WindowResize(t *testing.T) {
	m := NewAddModel("")
	newModel, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m = newModel.(AddModel)

	if m.windowWidth != 100 {
		t.Errorf("expected windowWidth 100, got %d", m.windowWidth)
	}
	if m.windowHeight != 50 {
		t.Errorf("expected windowHeight 50, got %d", m.windowHeight)
	}
}

func TestAddModel_View_LayoutStep(t *testing.T) {
	m := NewAddModel("")
	view := m.View()

	if !strings.Contains(view, "Add New Slide") {
		t.Error("expected view to contain 'Add New Slide'")
	}
	if !strings.Contains(view, "Select a layout") {
		t.Error("expected view to contain 'Select a layout'")
	}
	if !strings.Contains(view, "title") {
		t.Error("expected view to contain 'title' layout")
	}
}

func TestAddModel_View_Aborted(t *testing.T) {
	m := NewAddModel("")
	m.quitting = true
	view := m.View()

	if !strings.Contains(view, "Aborted") {
		t.Error("expected view to contain 'Aborted'")
	}
}

func TestGenerateSlideMarkdown_Title(t *testing.T) {
	markdown := GenerateSlideMarkdown("title", []string{"My Title", "My Subtitle"})

	if !strings.Contains(markdown, "---") {
		t.Error("expected markdown to contain slide separator")
	}
	if !strings.Contains(markdown, "# My Title") {
		t.Error("expected markdown to contain title heading")
	}
	if !strings.Contains(markdown, "My Subtitle") {
		t.Error("expected markdown to contain subtitle")
	}
}

func TestGenerateSlideMarkdown_TitleWithoutSubtitle(t *testing.T) {
	markdown := GenerateSlideMarkdown("title", []string{"My Title", ""})

	if !strings.Contains(markdown, "# My Title") {
		t.Error("expected markdown to contain title heading")
	}
	// Should not have extra empty lines
	lines := strings.Split(markdown, "\n")
	emptyLineCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) == "" {
			emptyLineCount++
		}
	}
	if emptyLineCount > 3 { // reasonable number for formatting
		t.Errorf("too many empty lines: %d", emptyLineCount)
	}
}

func TestGenerateSlideMarkdown_Section(t *testing.T) {
	markdown := GenerateSlideMarkdown("section", []string{"My Section"})

	if !strings.Contains(markdown, "## My Section") {
		t.Error("expected markdown to contain section heading")
	}
}

func TestGenerateSlideMarkdown_Default(t *testing.T) {
	markdown := GenerateSlideMarkdown("default", []string{"Header", "- Point one\n- Point two"})

	if !strings.Contains(markdown, "## Header") {
		t.Error("expected markdown to contain header")
	}
	if !strings.Contains(markdown, "- Point one") {
		t.Error("expected markdown to contain bullet points")
	}
}

func TestGenerateSlideMarkdown_TwoColumn(t *testing.T) {
	markdown := GenerateSlideMarkdown("two-column", []string{"Header", "Left", "Right"})

	if !strings.Contains(markdown, "## Header") {
		t.Error("expected markdown to contain header")
	}
	if !strings.Contains(markdown, "|||") {
		t.Error("expected markdown to contain column separator")
	}
	if !strings.Contains(markdown, "Left") {
		t.Error("expected markdown to contain left content")
	}
	if !strings.Contains(markdown, "Right") {
		t.Error("expected markdown to contain right content")
	}
}

func TestGenerateSlideMarkdown_TwoColumnNoHeader(t *testing.T) {
	markdown := GenerateSlideMarkdown("two-column", []string{"", "Left", "Right"})

	if strings.Contains(markdown, "## ") {
		t.Error("expected markdown to not contain header when empty")
	}
	if !strings.Contains(markdown, "|||") {
		t.Error("expected markdown to contain column separator")
	}
}

func TestGenerateSlideMarkdown_CodeFocus(t *testing.T) {
	markdown := GenerateSlideMarkdown("code-focus", []string{"go", "func main() {}"})

	if !strings.Contains(markdown, "layout: code-focus") {
		t.Error("expected markdown to contain layout directive")
	}
	if !strings.Contains(markdown, "```go") {
		t.Error("expected markdown to contain code block with language")
	}
	if !strings.Contains(markdown, "func main()") {
		t.Error("expected markdown to contain code")
	}
}

func TestGenerateSlideMarkdown_Quote(t *testing.T) {
	markdown := GenerateSlideMarkdown("quote", []string{"Life is short", "Me"})

	if !strings.Contains(markdown, "layout: quote") {
		t.Error("expected markdown to contain layout directive")
	}
	if !strings.Contains(markdown, "> \"Life is short\"") {
		t.Error("expected markdown to contain quoted text")
	}
	if !strings.Contains(markdown, "— Me") {
		t.Error("expected markdown to contain author")
	}
}

func TestGenerateSlideMarkdown_QuoteNoAuthor(t *testing.T) {
	markdown := GenerateSlideMarkdown("quote", []string{"Just a quote", ""})

	if !strings.Contains(markdown, "> \"Just a quote\"") {
		t.Error("expected markdown to contain quoted text")
	}
	if strings.Contains(markdown, "—") {
		t.Error("expected markdown to not contain author dash when empty")
	}
}

func TestGenerateSlideMarkdown_BigStat(t *testing.T) {
	markdown := GenerateSlideMarkdown("big-stat", []string{"99%", "of developers"})

	if !strings.Contains(markdown, "layout: big-stat") {
		t.Error("expected markdown to contain layout directive")
	}
	if !strings.Contains(markdown, "# 99%") {
		t.Error("expected markdown to contain statistic as heading")
	}
	if !strings.Contains(markdown, "of developers") {
		t.Error("expected markdown to contain description")
	}
}

func TestGenerateSlideMarkdown_DefaultValues(t *testing.T) {
	// Test with empty values - should use defaults
	markdown := GenerateSlideMarkdown("title", []string{})

	if !strings.Contains(markdown, "# Title") {
		t.Error("expected markdown to contain default title")
	}
}

func TestGetValueOrDefault(t *testing.T) {
	tests := []struct {
		name       string
		values     []string
		index      int
		defaultVal string
		expected   string
	}{
		{"value exists", []string{"hello"}, 0, "default", "hello"},
		{"value is empty", []string{""}, 0, "default", "default"},
		{"index out of range", []string{"a"}, 5, "default", "default"},
		{"empty slice", []string{}, 0, "default", "default"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getValueOrDefault(tt.values, tt.index, tt.defaultVal)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestFormatContent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"single line", "hello", "hello"},
		{"multiple lines", "line1\nline2", "line1\nline2"},
		{"with whitespace", "  line1  \n  line2  ", "line1\nline2"},
		{"empty lines", "line1\n\nline2", "line1\n\nline2"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatContent(tt.input)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestAppendToFile(t *testing.T) {
	// Create a temporary file
	dir := t.TempDir()
	filePath := filepath.Join(dir, "test.md")

	// Create initial content
	err := os.WriteFile(filePath, []byte("# Initial\n"), 0644)
	if err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	// Append content
	err = appendToFile(filePath, "\n---\n\n## New Slide\n")
	if err != nil {
		t.Fatalf("failed to append to file: %v", err)
	}

	// Read back and verify
	content, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("failed to read file: %v", err)
	}

	if !strings.Contains(string(content), "# Initial") {
		t.Error("expected file to contain initial content")
	}
	if !strings.Contains(string(content), "## New Slide") {
		t.Error("expected file to contain appended content")
	}
}

func TestAppendToFile_NonExistent(t *testing.T) {
	dir := t.TempDir()
	filePath := filepath.Join(dir, "nonexistent.md")

	err := appendToFile(filePath, "content")
	if err == nil {
		t.Error("expected error when appending to non-existent file")
	}
}

func TestAvailableLayouts(t *testing.T) {
	// Verify all required layouts are present
	requiredLayouts := []string{"title", "section", "default", "two-column", "code-focus", "quote", "big-stat"}

	for _, required := range requiredLayouts {
		found := false
		for _, layout := range AvailableLayouts {
			if layout.Name == required {
				found = true
				if layout.Description == "" {
					t.Errorf("layout '%s' has empty description", required)
				}
				if layout.ASCII == "" {
					t.Errorf("layout '%s' has empty ASCII preview", required)
				}
				if len(layout.Fields) == 0 {
					t.Errorf("layout '%s' has no fields", required)
				}
				break
			}
		}
		if !found {
			t.Errorf("required layout '%s' not found", required)
		}
	}
}

func TestAddModel_GetResult(t *testing.T) {
	m := NewAddModel("test.md")
	m.layoutIndex = 2 // default layout

	result := m.GetResult()

	if result.FilePath != "test.md" {
		t.Errorf("expected FilePath 'test.md', got '%s'", result.FilePath)
	}
	if result.Layout != "default" {
		t.Errorf("expected Layout 'default', got '%s'", result.Layout)
	}
	if result.Aborted {
		t.Error("expected Aborted to be false")
	}
}

func TestAddModel_GetResult_Aborted(t *testing.T) {
	m := NewAddModel("")
	m.quitting = true

	result := m.GetResult()

	if !result.Aborted {
		t.Error("expected Aborted to be true")
	}
}

func TestAddModel_WasAborted(t *testing.T) {
	m := NewAddModel("")

	if m.WasAborted() {
		t.Error("expected WasAborted to be false initially")
	}

	m.quitting = true
	if !m.WasAborted() {
		t.Error("expected WasAborted to be true after quitting")
	}
}

func TestAddModel_GetError(t *testing.T) {
	m := NewAddModel("")

	if m.GetError() != nil {
		t.Error("expected GetError to return nil initially")
	}
}

func TestAddModel_ContentNavigation(t *testing.T) {
	m := NewAddModel("")

	// Select a layout with multiple fields (default has 2)
	m.layoutIndex = 2 // default
	newModel, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(AddModel)

	// Verify we're on content step
	if m.step != addStepContent {
		t.Fatalf("expected step addStepContent, got %d", m.step)
	}

	// Initial field should be 0
	if m.fieldIndex != 0 {
		t.Errorf("expected fieldIndex 0, got %d", m.fieldIndex)
	}

	// Navigate to next field with tab
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = newModel.(AddModel)
	if m.fieldIndex != 1 {
		t.Errorf("expected fieldIndex 1 after tab, got %d", m.fieldIndex)
	}

	// Navigate back with shift+tab
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyShiftTab})
	m = newModel.(AddModel)
	if m.fieldIndex != 0 {
		t.Errorf("expected fieldIndex 0 after shift+tab, got %d", m.fieldIndex)
	}
}
