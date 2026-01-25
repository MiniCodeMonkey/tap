package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
)

func TestParseSlides(t *testing.T) {
	tests := []struct {
		name           string
		content        string
		expectedCount  int
		expectedTitles []string
	}{
		{
			name:           "empty content",
			content:        "",
			expectedCount:  0,
			expectedTitles: []string{},
		},
		{
			name:           "single slide no delimiter",
			content:        "# Hello World\n\nSome content",
			expectedCount:  1,
			expectedTitles: []string{"Hello World"},
		},
		{
			name:           "two slides with headings",
			content:        "# First Slide\n\nContent\n\n---\n\n# Second Slide\n\nMore content",
			expectedCount:  2,
			expectedTitles: []string{"First Slide", "Second Slide"},
		},
		{
			name:           "slide without heading",
			content:        "Just some text without a heading",
			expectedCount:  1,
			expectedTitles: []string{"Just some text without a heading"},
		},
		{
			name:           "slides with frontmatter",
			content:        "---\ntitle: Test\ntheme: paper\n---\n\n# First Slide\n\n---\n\n# Second Slide",
			expectedCount:  2,
			expectedTitles: []string{"First Slide", "Second Slide"},
		},
		{
			name:           "slide with h2 heading",
			content:        "## Second Level Heading\n\nContent",
			expectedCount:  1,
			expectedTitles: []string{"Second Level Heading"},
		},
		{
			name:           "slide with directive comment",
			content:        "<!-- layout: title -->\n\n# Title Slide",
			expectedCount:  1,
			expectedTitles: []string{"Title Slide"},
		},
		{
			name:           "empty slide skipped",
			content:        "# First\n\n---\n\n   \n\n---\n\n# Third",
			expectedCount:  2,
			expectedTitles: []string{"First", "Third"},
		},
		{
			name:           "long title truncated",
			content:        "# This is a very long title that should be truncated because it exceeds fifty characters",
			expectedCount:  1,
			expectedTitles: []string{"This is a very long title that should be trunca..."},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			slides := parseSlides(tt.content)

			if len(slides) != tt.expectedCount {
				t.Errorf("expected %d slides, got %d", tt.expectedCount, len(slides))
			}

			for i, expectedTitle := range tt.expectedTitles {
				if i >= len(slides) {
					break
				}
				if slides[i].Title != expectedTitle {
					t.Errorf("slide %d: expected title %q, got %q", i, expectedTitle, slides[i].Title)
				}
			}
		})
	}
}

func TestExtractSlideTitle(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{
			name:     "h1 heading",
			content:  "# Hello World",
			expected: "Hello World",
		},
		{
			name:     "h2 heading",
			content:  "## Second Level",
			expected: "Second Level",
		},
		{
			name:     "h3 heading",
			content:  "### Third Level",
			expected: "Third Level",
		},
		{
			name:     "heading after empty lines",
			content:  "\n\n# Title\n\nContent",
			expected: "Title",
		},
		{
			name:     "no heading - first line",
			content:  "First line of text\n\nMore text",
			expected: "First line of text",
		},
		{
			name:     "empty content",
			content:  "",
			expected: "(empty slide)",
		},
		{
			name:     "only whitespace",
			content:  "   \n\n   ",
			expected: "(empty slide)",
		},
		{
			name:     "skip directive comment",
			content:  "<!-- layout: title -->\n# Actual Title",
			expected: "Actual Title",
		},
		{
			name:     "long title truncated",
			content:  "This is a very very very very very long first line that exceeds fifty characters",
			expected: "This is a very very very very very long first l...",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractSlideTitle(tt.content)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestNewImageGenModel(t *testing.T) {
	// Create a temporary markdown file
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `---
title: Test Presentation
theme: paper
---

# First Slide

Content here

---

# Second Slide

More content

---

## Third Slide

Even more content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	if len(model.Slides) != 3 {
		t.Errorf("expected 3 slides, got %d", len(model.Slides))
	}

	if model.SelectedIndex != 0 {
		t.Errorf("expected SelectedIndex 0, got %d", model.SelectedIndex)
	}

	if model.Step != ImageGenStepSlideSelect {
		t.Errorf("expected step ImageGenStepSlideSelect, got %d", model.Step)
	}
}

func TestNewImageGenModel_FileNotFound(t *testing.T) {
	_, err := NewImageGenModel("/nonexistent/file.md")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestImageGenModel_Navigation(t *testing.T) {
	// Create a temporary markdown file with 5 slides
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := "# Slide 1\n\n---\n\n# Slide 2\n\n---\n\n# Slide 3\n\n---\n\n# Slide 4\n\n---\n\n# Slide 5"
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Test navigation down with 'j'
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m := newModel.(*ImageGenModel)
	if m.SelectedIndex != 1 {
		t.Errorf("expected SelectedIndex 1 after 'j', got %d", m.SelectedIndex)
	}

	// Test navigation down with 'down'
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(*ImageGenModel)
	if m.SelectedIndex != 2 {
		t.Errorf("expected SelectedIndex 2 after 'down', got %d", m.SelectedIndex)
	}

	// Test navigation up with 'k'
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newModel.(*ImageGenModel)
	if m.SelectedIndex != 1 {
		t.Errorf("expected SelectedIndex 1 after 'k', got %d", m.SelectedIndex)
	}

	// Test navigation up with 'up'
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(*ImageGenModel)
	if m.SelectedIndex != 0 {
		t.Errorf("expected SelectedIndex 0 after 'up', got %d", m.SelectedIndex)
	}

	// Test boundary - can't go below 0
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(*ImageGenModel)
	if m.SelectedIndex != 0 {
		t.Errorf("expected SelectedIndex 0 at boundary, got %d", m.SelectedIndex)
	}

	// Navigate to last slide
	for i := 0; i < 4; i++ {
		newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = newModel.(*ImageGenModel)
	}
	if m.SelectedIndex != 4 {
		t.Errorf("expected SelectedIndex 4, got %d", m.SelectedIndex)
	}

	// Test boundary - can't go past last slide
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(*ImageGenModel)
	if m.SelectedIndex != 4 {
		t.Errorf("expected SelectedIndex 4 at boundary, got %d", m.SelectedIndex)
	}
}

func TestImageGenModel_Cancel(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := "# Slide 1\n\n---\n\n# Slide 2"
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	tests := []struct {
		name string
		key  tea.KeyMsg
	}{
		{"esc key", tea.KeyMsg{Type: tea.KeyEsc}},
		{"q key", tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("q")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model, err := NewImageGenModel(mdFile)
			if err != nil {
				t.Fatalf("failed to create model: %v", err)
			}

			newModel, _ := model.Update(tt.key)
			if newModel != nil {
				t.Error("expected nil model after cancel")
			}
		})
	}
}

func TestImageGenModel_SelectSlide(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := "# Slide 1\n\n---\n\n# Slide 2\n\n---\n\n# Slide 3"
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Navigate to second slide
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	m := newModel.(*ImageGenModel)

	// Press enter to select
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*ImageGenModel)

	if m.Step != ImageGenStepPrompt {
		t.Errorf("expected step ImageGenStepPrompt after select, got %d", m.Step)
	}

	if m.SelectedIndex != 1 {
		t.Errorf("expected SelectedIndex 1 after select, got %d", m.SelectedIndex)
	}
}

func TestImageGenModel_GetSelectedSlide(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := "# First Slide\n\n---\n\n# Second Slide\n\n---\n\n# Third Slide"
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Get first slide
	slide := model.GetSelectedSlide()
	if slide == nil {
		t.Fatal("expected slide, got nil")
	}
	if slide.Title != "First Slide" {
		t.Errorf("expected title 'First Slide', got %q", slide.Title)
	}
	if slide.Index != 0 {
		t.Errorf("expected index 0, got %d", slide.Index)
	}

	// Navigate and check again
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	m := newModel.(*ImageGenModel)

	slide = m.GetSelectedSlide()
	if slide == nil {
		t.Fatal("expected slide, got nil")
	}
	if slide.Title != "Second Slide" {
		t.Errorf("expected title 'Second Slide', got %q", slide.Title)
	}
	if slide.Index != 1 {
		t.Errorf("expected index 1, got %d", slide.Index)
	}
}

func TestImageGenModel_View_SlideSelect(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := "# First Slide\n\n---\n\n# Second Slide\n\n---\n\n# Third Slide"
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	view := model.View()

	// Check title is present
	if !strings.Contains(view, "Select Slide") {
		t.Error("view should contain 'Select Slide'")
	}

	// Check all slides are listed
	if !strings.Contains(view, "First Slide") {
		t.Error("view should contain 'First Slide'")
	}
	if !strings.Contains(view, "Second Slide") {
		t.Error("view should contain 'Second Slide'")
	}
	if !strings.Contains(view, "Third Slide") {
		t.Error("view should contain 'Third Slide'")
	}

	// Check slide numbers are present
	if !strings.Contains(view, "1.") {
		t.Error("view should contain slide number '1.'")
	}
	if !strings.Contains(view, "2.") {
		t.Error("view should contain slide number '2.'")
	}
	if !strings.Contains(view, "3.") {
		t.Error("view should contain slide number '3.'")
	}

	// Check selection indicator is present (> for selected)
	if !strings.Contains(view, ">") {
		t.Error("view should contain '>' selection indicator")
	}

	// Check help text is present
	if !strings.Contains(view, "navigate") {
		t.Error("view should contain help text")
	}
	if !strings.Contains(view, "select") {
		t.Error("view should contain 'select' in help text")
	}
	if !strings.Contains(view, "cancel") {
		t.Error("view should contain 'cancel' in help text")
	}
}

func TestImageGenModel_Init(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := "# Test Slide"
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	cmd := model.Init()
	if cmd != nil {
		t.Error("Init should return nil cmd")
	}
}

func TestSlideInfo_SlideIndex(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := "# First\n\n---\n\n# Second\n\n---\n\n# Third"
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	for i, slide := range model.Slides {
		if slide.Index != i {
			t.Errorf("slide %d: expected Index %d, got %d", i, i, slide.Index)
		}
	}
}
