package tui

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/MiniCodeMonkey/tap/internal/gemini"
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

func TestParseAIImages(t *testing.T) {
	tests := []struct {
		name          string
		content       string
		expectedCount int
		expectedItems []AIImageInfo
	}{
		{
			name:          "no AI images",
			content:       "# Slide Title\n\nSome content here",
			expectedCount: 0,
			expectedItems: nil,
		},
		{
			name:          "single AI image",
			content:       "# Slide Title\n\n<!-- ai-prompt: a beautiful sunset -->\n![](images/generated-abc123.png)",
			expectedCount: 1,
			expectedItems: []AIImageInfo{
				{Prompt: "a beautiful sunset", ImagePath: "images/generated-abc123.png"},
			},
		},
		{
			name:          "multiple AI images",
			content:       "# Slide Title\n\n<!-- ai-prompt: first image -->\n![](images/first.png)\n\nSome text\n\n<!-- ai-prompt: second image -->\n![](images/second.png)",
			expectedCount: 2,
			expectedItems: []AIImageInfo{
				{Prompt: "first image", ImagePath: "images/first.png"},
				{Prompt: "second image", ImagePath: "images/second.png"},
			},
		},
		{
			name:          "AI prompt without image",
			content:       "# Slide Title\n\n<!-- ai-prompt: orphan prompt -->\n\nSome text but no image",
			expectedCount: 0,
			expectedItems: nil,
		},
		{
			name:          "regular image without AI prompt",
			content:       "# Slide Title\n\n![Alt text](images/regular.png)",
			expectedCount: 0,
			expectedItems: nil,
		},
		{
			name:          "AI prompt with spaces",
			content:       "<!--   ai-prompt:   a detailed prompt with spaces   -->\n![](images/test.png)",
			expectedCount: 1,
			expectedItems: []AIImageInfo{
				{Prompt: "a detailed prompt with spaces", ImagePath: "images/test.png"},
			},
		},
		{
			name:          "AI image with relative path",
			content:       "<!-- ai-prompt: test -->\n![](./images/local.png)",
			expectedCount: 1,
			expectedItems: []AIImageInfo{
				{Prompt: "test", ImagePath: "./images/local.png"},
			},
		},
		{
			name:          "AI image with whitespace between comment and image",
			content:       "<!-- ai-prompt: test -->\n   \n![](images/test.png)",
			expectedCount: 0,
			expectedItems: nil, // Should not match if there's extra whitespace (blank line)
		},
		{
			name:          "AI image with newline and leading spaces",
			content:       "<!-- ai-prompt: test -->\n  ![](images/test.png)",
			expectedCount: 1,
			expectedItems: []AIImageInfo{
				{Prompt: "test", ImagePath: "images/test.png"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			images := parseAIImages(tt.content)

			if len(images) != tt.expectedCount {
				t.Errorf("expected %d AI images, got %d", tt.expectedCount, len(images))
			}

			for i, expected := range tt.expectedItems {
				if i >= len(images) {
					break
				}
				if images[i].Prompt != expected.Prompt {
					t.Errorf("image %d: expected prompt %q, got %q", i, expected.Prompt, images[i].Prompt)
				}
				if images[i].ImagePath != expected.ImagePath {
					t.Errorf("image %d: expected path %q, got %q", i, expected.ImagePath, images[i].ImagePath)
				}
			}
		})
	}
}

func TestParseSlides_WithAIImages(t *testing.T) {
	content := `# First Slide

Some content

---

# Second Slide

<!-- ai-prompt: a mountain landscape -->
![](images/mountain.png)

---

# Third Slide

<!-- ai-prompt: image one -->
![](images/one.png)

Some text

<!-- ai-prompt: image two -->
![](images/two.png)
`

	slides := parseSlides(content)

	if len(slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(slides))
	}

	// First slide - no AI images
	if slides[0].HasAIImages {
		t.Error("slide 0: should not have AI images")
	}
	if slides[0].AIImageCount != 0 {
		t.Errorf("slide 0: expected AIImageCount 0, got %d", slides[0].AIImageCount)
	}

	// Second slide - one AI image
	if !slides[1].HasAIImages {
		t.Error("slide 1: should have AI images")
	}
	if slides[1].AIImageCount != 1 {
		t.Errorf("slide 1: expected AIImageCount 1, got %d", slides[1].AIImageCount)
	}
	if len(slides[1].AIImages) != 1 {
		t.Fatalf("slide 1: expected 1 AIImage, got %d", len(slides[1].AIImages))
	}
	if slides[1].AIImages[0].Prompt != "a mountain landscape" {
		t.Errorf("slide 1: expected prompt 'a mountain landscape', got %q", slides[1].AIImages[0].Prompt)
	}

	// Third slide - two AI images
	if !slides[2].HasAIImages {
		t.Error("slide 2: should have AI images")
	}
	if slides[2].AIImageCount != 2 {
		t.Errorf("slide 2: expected AIImageCount 2, got %d", slides[2].AIImageCount)
	}
	if len(slides[2].AIImages) != 2 {
		t.Fatalf("slide 2: expected 2 AIImages, got %d", len(slides[2].AIImages))
	}
	if slides[2].AIImages[0].Prompt != "image one" {
		t.Errorf("slide 2 image 0: expected prompt 'image one', got %q", slides[2].AIImages[0].Prompt)
	}
	if slides[2].AIImages[1].Prompt != "image two" {
		t.Errorf("slide 2 image 1: expected prompt 'image two', got %q", slides[2].AIImages[1].Prompt)
	}
}

func TestImageGenModel_View_WithAIImageIndicator(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# First Slide

No images here

---

# Second Slide

<!-- ai-prompt: test prompt -->
![](images/test.png)

---

# Third Slide

<!-- ai-prompt: prompt one -->
![](images/one.png)

<!-- ai-prompt: prompt two -->
![](images/two.png)
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	view := model.View()

	// First slide should not have indicator
	// (we check by ensuring the indicator appears in the right places)

	// Second slide should show "[has 1 AI image]"
	if !strings.Contains(view, "[has 1 AI image]") {
		t.Error("view should contain '[has 1 AI image]' for slide with 1 AI image")
	}

	// Third slide should show "[has 2 AI images]"
	if !strings.Contains(view, "[has 2 AI images]") {
		t.Error("view should contain '[has 2 AI images]' for slide with 2 AI images")
	}
}

func TestImageGenModel_SelectSlideWithAIImages_ShowsOptions(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# First Slide

No images

---

# Second Slide

<!-- ai-prompt: a beautiful sunset -->
![](images/sunset.png)
`
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

	// Select the slide (press enter)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*ImageGenModel)

	// Should be in image select step
	if m.Step != ImageGenStepImageSelect {
		t.Errorf("expected ImageGenStepImageSelect, got %d", m.Step)
	}

	// Should have 2 options: add new + 1 regenerate
	if len(m.ImageOptions) != 2 {
		t.Errorf("expected 2 image options, got %d", len(m.ImageOptions))
	}

	// First option should be "Add new image"
	if !m.ImageOptions[0].IsAddNew {
		t.Error("first option should be IsAddNew=true")
	}
	if m.ImageOptions[0].Label != "Add new image" {
		t.Errorf("expected label 'Add new image', got %q", m.ImageOptions[0].Label)
	}

	// Second option should be regenerate
	if m.ImageOptions[1].IsAddNew {
		t.Error("second option should be IsAddNew=false")
	}
	if !strings.Contains(m.ImageOptions[1].Label, "Regenerate:") {
		t.Errorf("expected label to contain 'Regenerate:', got %q", m.ImageOptions[1].Label)
	}
	if !strings.Contains(m.ImageOptions[1].Label, "a beautiful sunset") {
		t.Errorf("expected label to contain prompt preview, got %q", m.ImageOptions[1].Label)
	}
}

func TestImageGenModel_SelectSlideWithoutAIImages_SkipsImageSelect(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# First Slide

No images here
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Select the slide (press enter)
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	// Should skip directly to prompt step (no AI images to choose from)
	if m.Step != ImageGenStepPrompt {
		t.Errorf("expected ImageGenStepPrompt, got %d", m.Step)
	}
}

func TestImageGenModel_ImageSelectNavigation(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Slide

<!-- ai-prompt: first image -->
![](images/first.png)

<!-- ai-prompt: second image -->
![](images/second.png)

<!-- ai-prompt: third image -->
![](images/third.png)
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Select slide
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	// Should have 4 options: add new + 3 regenerate
	if len(m.ImageOptions) != 4 {
		t.Fatalf("expected 4 image options, got %d", len(m.ImageOptions))
	}

	// Initial selection should be 0
	if m.ImageOptionIndex != 0 {
		t.Errorf("expected ImageOptionIndex 0, got %d", m.ImageOptionIndex)
	}

	// Navigate down with 'j'
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = newModel.(*ImageGenModel)
	if m.ImageOptionIndex != 1 {
		t.Errorf("expected ImageOptionIndex 1 after 'j', got %d", m.ImageOptionIndex)
	}

	// Navigate down with 'down'
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(*ImageGenModel)
	if m.ImageOptionIndex != 2 {
		t.Errorf("expected ImageOptionIndex 2 after 'down', got %d", m.ImageOptionIndex)
	}

	// Navigate up with 'k'
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	m = newModel.(*ImageGenModel)
	if m.ImageOptionIndex != 1 {
		t.Errorf("expected ImageOptionIndex 1 after 'k', got %d", m.ImageOptionIndex)
	}

	// Navigate up with 'up'
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(*ImageGenModel)
	if m.ImageOptionIndex != 0 {
		t.Errorf("expected ImageOptionIndex 0 after 'up', got %d", m.ImageOptionIndex)
	}

	// Can't go below 0
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyUp})
	m = newModel.(*ImageGenModel)
	if m.ImageOptionIndex != 0 {
		t.Errorf("expected ImageOptionIndex 0 at boundary, got %d", m.ImageOptionIndex)
	}

	// Navigate to last option
	for i := 0; i < 3; i++ {
		newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
		m = newModel.(*ImageGenModel)
	}
	if m.ImageOptionIndex != 3 {
		t.Errorf("expected ImageOptionIndex 3, got %d", m.ImageOptionIndex)
	}

	// Can't go past last
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(*ImageGenModel)
	if m.ImageOptionIndex != 3 {
		t.Errorf("expected ImageOptionIndex 3 at boundary, got %d", m.ImageOptionIndex)
	}
}

func TestImageGenModel_ImageSelectAddNew(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Slide

<!-- ai-prompt: existing prompt -->
![](images/existing.png)
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Select slide
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	// Select "Add new image" (first option, index 0)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*ImageGenModel)

	// Should be in prompt step
	if m.Step != ImageGenStepPrompt {
		t.Errorf("expected ImageGenStepPrompt, got %d", m.Step)
	}

	// SelectedImage should be nil (adding new)
	if m.SelectedImage != nil {
		t.Error("SelectedImage should be nil when adding new")
	}

	// Prompt should be empty
	if m.Prompt != "" {
		t.Errorf("expected empty prompt, got %q", m.Prompt)
	}
}

func TestImageGenModel_ImageSelectRegenerate(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Slide

<!-- ai-prompt: existing prompt for testing -->
![](images/existing.png)
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Select slide
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	// Navigate to regenerate option (index 1)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(*ImageGenModel)

	// Select regenerate option
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*ImageGenModel)

	// Should be in prompt step
	if m.Step != ImageGenStepPrompt {
		t.Errorf("expected ImageGenStepPrompt, got %d", m.Step)
	}

	// SelectedImage should be set
	if m.SelectedImage == nil {
		t.Fatal("SelectedImage should not be nil when regenerating")
	}

	// Prompt should be pre-filled
	if m.Prompt != "existing prompt for testing" {
		t.Errorf("expected prompt 'existing prompt for testing', got %q", m.Prompt)
	}

	// ImagePath should be set
	if m.SelectedImage.ImagePath != "images/existing.png" {
		t.Errorf("expected ImagePath 'images/existing.png', got %q", m.SelectedImage.ImagePath)
	}
}

func TestImageGenModel_ImageSelectEscapeGoesBack(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Slide

<!-- ai-prompt: test -->
![](images/test.png)
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Select slide to go to image select
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	if m.Step != ImageGenStepImageSelect {
		t.Fatalf("expected ImageGenStepImageSelect, got %d", m.Step)
	}

	// Press escape to go back
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(*ImageGenModel)

	// Should be back in slide select
	if m.Step != ImageGenStepSlideSelect {
		t.Errorf("expected ImageGenStepSlideSelect after esc, got %d", m.Step)
	}

	// ImageOptions should be cleared
	if m.ImageOptions != nil {
		t.Error("ImageOptions should be nil after going back")
	}
}

func TestImageGenModel_ImageSelectView(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# My Test Slide

<!-- ai-prompt: first prompt -->
![](images/first.png)

<!-- ai-prompt: second prompt -->
![](images/second.png)
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Select slide to go to image select
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	view := m.View()

	// Should contain title
	if !strings.Contains(view, "Select Action") {
		t.Error("view should contain 'Select Action'")
	}

	// Should show slide info
	if !strings.Contains(view, "My Test Slide") {
		t.Error("view should contain slide title")
	}

	// Should show "Add new image" option
	if !strings.Contains(view, "Add new image") {
		t.Error("view should contain 'Add new image'")
	}

	// Should show regenerate options with prompt previews
	if !strings.Contains(view, "Regenerate:") {
		t.Error("view should contain 'Regenerate:'")
	}
	if !strings.Contains(view, "first prompt") {
		t.Error("view should contain 'first prompt'")
	}
	if !strings.Contains(view, "second prompt") {
		t.Error("view should contain 'second prompt'")
	}

	// Should have selection indicator
	if !strings.Contains(view, ">") {
		t.Error("view should contain '>' selection indicator")
	}

	// Should have help text
	if !strings.Contains(view, "navigate") {
		t.Error("view should contain 'navigate' in help text")
	}
	if !strings.Contains(view, "back") {
		t.Error("view should contain 'back' in help text")
	}
}

func TestImageGenModel_MultipleAIImagesShowAllOptions(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Slide

<!-- ai-prompt: image one -->
![](images/one.png)

<!-- ai-prompt: image two -->
![](images/two.png)

<!-- ai-prompt: image three -->
![](images/three.png)
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Select slide
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	// Should have 4 options: 1 add new + 3 regenerate
	if len(m.ImageOptions) != 4 {
		t.Fatalf("expected 4 image options, got %d", len(m.ImageOptions))
	}

	// Verify first option is add new
	if !m.ImageOptions[0].IsAddNew {
		t.Error("first option should be IsAddNew")
	}

	// Verify regenerate options
	expectedPrompts := []string{"image one", "image two", "image three"}
	for i, expected := range expectedPrompts {
		optionIdx := i + 1 // offset by 1 for "add new"
		if m.ImageOptions[optionIdx].IsAddNew {
			t.Errorf("option %d should not be IsAddNew", optionIdx)
		}
		if m.ImageOptions[optionIdx].AIImage == nil {
			t.Errorf("option %d AIImage should not be nil", optionIdx)
			continue
		}
		if m.ImageOptions[optionIdx].AIImage.Prompt != expected {
			t.Errorf("option %d: expected prompt %q, got %q", optionIdx, expected, m.ImageOptions[optionIdx].AIImage.Prompt)
		}
	}
}

func TestImageGenModel_LongPromptTruncated(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	longPrompt := "This is a very long prompt that should be truncated because it exceeds forty characters"
	content := fmt.Sprintf(`# Slide

<!-- ai-prompt: %s -->
![](images/long.png)
`, longPrompt)
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Select slide
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	// Check the regenerate option label is truncated
	if len(m.ImageOptions) < 2 {
		t.Fatal("expected at least 2 options")
	}

	label := m.ImageOptions[1].Label
	if !strings.Contains(label, "...") {
		t.Error("long prompt should be truncated with '...'")
	}

	// Original prompt should still be preserved in AIImage
	if m.ImageOptions[1].AIImage.Prompt != longPrompt {
		t.Error("original prompt should be preserved in AIImage")
	}
}

// Tests for prompt input step (US-013)

func TestImageGenModel_PromptInputStep(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Some content here
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Select slide (goes directly to prompt since no AI images)
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	if m.Step != ImageGenStepPrompt {
		t.Errorf("expected ImageGenStepPrompt, got %d", m.Step)
	}

	// Prompt should be empty for new image
	if m.Prompt != "" {
		t.Errorf("expected empty prompt for new image, got %q", m.Prompt)
	}

	// SelectedImage should be nil for new image
	if m.SelectedImage != nil {
		t.Error("SelectedImage should be nil for new image")
	}
}

func TestImageGenModel_PromptInputView(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# My Test Slide

Some content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Go to prompt step
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	view := m.View()

	// Check title
	if !strings.Contains(view, "Enter Image Prompt") {
		t.Error("view should contain 'Enter Image Prompt'")
	}

	// Check slide info is shown
	if !strings.Contains(view, "My Test Slide") {
		t.Error("view should contain slide title")
	}

	// Check help text
	if !strings.Contains(view, "enter") {
		t.Error("view should contain 'enter' in help text")
	}
	if !strings.Contains(view, "ctrl+d") {
		t.Error("view should contain 'ctrl+d' in help text")
	}
	if !strings.Contains(view, "esc") {
		t.Error("view should contain 'esc' in help text")
	}
}

func TestImageGenModel_PromptInputEscGoesBackToSlideSelect(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

No AI images
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Go to prompt step (no AI images, so directly from slide select)
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	if m.Step != ImageGenStepPrompt {
		t.Fatalf("expected ImageGenStepPrompt, got %d", m.Step)
	}

	// Press esc to go back
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(*ImageGenModel)

	// Should go back to slide select (no AI images to go back to image select)
	if m.Step != ImageGenStepSlideSelect {
		t.Errorf("expected ImageGenStepSlideSelect after esc, got %d", m.Step)
	}
}

func TestImageGenModel_PromptInputEscGoesBackToImageSelect(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

<!-- ai-prompt: existing -->
![](images/existing.png)
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Select slide (goes to image select)
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	if m.Step != ImageGenStepImageSelect {
		t.Fatalf("expected ImageGenStepImageSelect, got %d", m.Step)
	}

	// Select "Add new image" to go to prompt
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*ImageGenModel)

	if m.Step != ImageGenStepPrompt {
		t.Fatalf("expected ImageGenStepPrompt, got %d", m.Step)
	}

	// Press esc to go back
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = newModel.(*ImageGenModel)

	// Should go back to image select (slide has AI images)
	if m.Step != ImageGenStepImageSelect {
		t.Errorf("expected ImageGenStepImageSelect after esc, got %d", m.Step)
	}
}

func TestImageGenModel_PromptInputSubmitWithEnter(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Go to prompt step
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	// Type a prompt (simulate by setting the value directly since we can't easily send key events)
	m.promptInput.SetValue("A beautiful mountain landscape")

	// Submit with enter
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*ImageGenModel)

	// Should be in generating step
	if m.Step != ImageGenStepGenerating {
		t.Errorf("expected ImageGenStepGenerating, got %d", m.Step)
	}

	// Prompt should be captured
	if m.Prompt != "A beautiful mountain landscape" {
		t.Errorf("expected prompt 'A beautiful mountain landscape', got %q", m.Prompt)
	}
}

func TestImageGenModel_PromptInputSubmitWithCtrlD(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Go to prompt step
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	// Type a prompt
	m.promptInput.SetValue("A sunset over the ocean")

	// Submit with ctrl+d
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{}, Alt: false})
	// Simulate ctrl+d key message
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyCtrlD})
	m = newModel.(*ImageGenModel)

	// Should be in generating step
	if m.Step != ImageGenStepGenerating {
		t.Errorf("expected ImageGenStepGenerating, got %d", m.Step)
	}

	// Prompt should be captured
	if m.Prompt != "A sunset over the ocean" {
		t.Errorf("expected prompt 'A sunset over the ocean', got %q", m.Prompt)
	}
}

func TestImageGenModel_PromptInputEmptyPromptShowsError(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Go to prompt step
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	// Try to submit empty prompt
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*ImageGenModel)

	// Should stay in prompt step
	if m.Step != ImageGenStepPrompt {
		t.Errorf("expected to stay in ImageGenStepPrompt with empty prompt, got %d", m.Step)
	}

	// Should have error
	if m.Error == "" {
		t.Error("expected error for empty prompt")
	}
	if !strings.Contains(m.Error, "empty") {
		t.Errorf("expected error about empty prompt, got %q", m.Error)
	}
}

func TestImageGenModel_PromptInputWhitespaceOnlyShowsError(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Go to prompt step
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	// Set whitespace-only prompt
	m.promptInput.SetValue("   \n  \t  ")

	// Try to submit
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*ImageGenModel)

	// Should stay in prompt step
	if m.Step != ImageGenStepPrompt {
		t.Errorf("expected to stay in ImageGenStepPrompt with whitespace-only prompt, got %d", m.Step)
	}

	// Should have error
	if m.Error == "" {
		t.Error("expected error for whitespace-only prompt")
	}
}

func TestImageGenModel_PromptPrefilledWhenRegenerating(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

<!-- ai-prompt: A cat sitting on a windowsill -->
![](images/cat.png)
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Select slide (goes to image select)
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	// Navigate to regenerate option (index 1)
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyDown})
	m = newModel.(*ImageGenModel)

	// Select regenerate option
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*ImageGenModel)

	// Should be in prompt step
	if m.Step != ImageGenStepPrompt {
		t.Errorf("expected ImageGenStepPrompt, got %d", m.Step)
	}

	// Prompt should be pre-filled
	promptValue := m.promptInput.Value()
	if promptValue != "A cat sitting on a windowsill" {
		t.Errorf("expected prompt to be pre-filled with 'A cat sitting on a windowsill', got %q", promptValue)
	}

	// SelectedImage should be set
	if m.SelectedImage == nil {
		t.Error("SelectedImage should not be nil when regenerating")
	}

	// View should indicate regenerating
	view := m.View()
	if !strings.Contains(view, "Regenerating") {
		t.Error("view should indicate regenerating existing image")
	}
}

func TestImageGenModel_PromptInputViewShowsError(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Go to prompt step
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	// Try to submit empty prompt to trigger error
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*ImageGenModel)

	// View should show error
	view := m.View()
	if !strings.Contains(view, "Error") {
		t.Error("view should show error message")
	}
}

// Tests for generating step (US-014)

func TestImageGenModel_GeneratingStep(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content here
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Go to prompt step
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m := newModel.(*ImageGenModel)

	// Set a prompt and submit
	m.promptInput.SetValue("A test image prompt")
	newModel, cmd := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = newModel.(*ImageGenModel)

	// Should be in generating step
	if m.Step != ImageGenStepGenerating {
		t.Errorf("expected ImageGenStepGenerating, got %d", m.Step)
	}

	// IsGenerating should be true
	if !m.IsGenerating {
		t.Error("IsGenerating should be true")
	}

	// Prompt should be captured
	if m.Prompt != "A test image prompt" {
		t.Errorf("expected prompt 'A test image prompt', got %q", m.Prompt)
	}

	// Should return a command (for spinner and generation)
	if cmd == nil {
		t.Error("expected non-nil cmd for generation")
	}
}

func TestImageGenModel_GeneratingViewShowsSpinner(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# My Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Manually set to generating step
	model.Step = ImageGenStepGenerating
	model.IsGenerating = true
	model.Prompt = "A beautiful landscape"

	view := model.View()

	// Should contain title
	if !strings.Contains(view, "Generating Image") {
		t.Error("view should contain 'Generating Image'")
	}

	// Should show slide info
	if !strings.Contains(view, "My Test Slide") {
		t.Error("view should contain slide title")
	}

	// Should show prompt
	if !strings.Contains(view, "Prompt:") {
		t.Error("view should show prompt label")
	}
	if !strings.Contains(view, "A beautiful landscape") {
		t.Error("view should show the prompt text")
	}

	// Should show generating message
	if !strings.Contains(view, "Generating image") {
		t.Error("view should contain 'Generating image'")
	}
}

func TestImageGenModel_GeneratingViewTruncatesLongPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set a very long prompt
	longPrompt := "This is a very very very long prompt that should be truncated because it exceeds sixty characters in length"
	model.Step = ImageGenStepGenerating
	model.IsGenerating = true
	model.Prompt = longPrompt

	view := model.View()

	// Should contain truncated prompt with "..."
	if !strings.Contains(view, "...") {
		t.Error("long prompt should be truncated with '...'")
	}

	// Should not contain the full prompt
	if strings.Contains(view, longPrompt) {
		t.Error("view should not contain the full long prompt")
	}
}

func TestImageGenModel_HandleImageGenerateSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set to generating step
	model.Step = ImageGenStepGenerating
	model.IsGenerating = true
	model.Prompt = "Test prompt"

	// Simulate successful generation
	result := ImageGenerateResult{
		ImageData:   []byte("fake image data"),
		ContentType: "image/png",
	}
	newModel, _ := model.Update(imageGenerateMsg{result: result})
	m := newModel.(*ImageGenModel)

	// Should be in done step
	if m.Step != ImageGenStepDone {
		t.Errorf("expected ImageGenStepDone, got %d", m.Step)
	}

	// IsGenerating should be false
	if m.IsGenerating {
		t.Error("IsGenerating should be false after success")
	}

	// GeneratedImage should be set
	if m.GeneratedImage == nil {
		t.Fatal("GeneratedImage should not be nil after success")
	}

	if string(m.GeneratedImage.ImageData) != "fake image data" {
		t.Error("image data should match")
	}

	if m.GeneratedImage.ContentType != "image/png" {
		t.Errorf("expected content type 'image/png', got %q", m.GeneratedImage.ContentType)
	}

	// No error
	if m.Error != "" {
		t.Errorf("expected no error, got %q", m.Error)
	}
}

func TestImageGenModel_HandleImageGenerateError(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set to generating step
	model.Step = ImageGenStepGenerating
	model.IsGenerating = true
	model.Prompt = "Test prompt"

	// Simulate error
	result := ImageGenerateResult{
		Error: errors.New("generation failed"),
	}
	newModel, _ := model.Update(imageGenerateMsg{result: result})
	m := newModel.(*ImageGenModel)

	// Should stay in generating step (to show error)
	if m.Step != ImageGenStepGenerating {
		t.Errorf("expected ImageGenStepGenerating after error, got %d", m.Step)
	}

	// IsGenerating should be false
	if m.IsGenerating {
		t.Error("IsGenerating should be false after error")
	}

	// Error should be set
	if m.Error == "" {
		t.Error("Error should be set after generation failure")
	}

	// GeneratedImage should be nil
	if m.GeneratedImage != nil {
		t.Error("GeneratedImage should be nil after error")
	}
}

func TestImageGenModel_GeneratingViewShowsError(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set to generating step with error
	model.Step = ImageGenStepGenerating
	model.IsGenerating = false
	model.Prompt = "Test prompt"
	model.Error = "Network error occurred"

	view := model.View()

	// Should show error message
	if !strings.Contains(view, "Error:") {
		t.Error("view should show 'Error:'")
	}
	if !strings.Contains(view, "Network error") {
		t.Error("view should show the error message")
	}

	// Should show retry option
	if !strings.Contains(view, "retry") {
		t.Error("view should show retry option")
	}

	// Should show back option
	if !strings.Contains(view, "back") || !strings.Contains(view, "esc") {
		t.Error("view should show back/esc option")
	}
}

func TestImageGenModel_GeneratingRetry(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set to generating step with error (so retry is allowed)
	model.Step = ImageGenStepGenerating
	model.IsGenerating = false
	model.Prompt = "Test prompt"
	model.Error = "Previous error"

	// Press 'r' to retry
	newModel, cmd := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m := newModel.(*ImageGenModel)

	// Should still be in generating step
	if m.Step != ImageGenStepGenerating {
		t.Errorf("expected ImageGenStepGenerating after retry, got %d", m.Step)
	}

	// IsGenerating should be true again
	if !m.IsGenerating {
		t.Error("IsGenerating should be true after retry")
	}

	// Error should be cleared
	if m.Error != "" {
		t.Errorf("Error should be cleared after retry, got %q", m.Error)
	}

	// Should return a command for the new generation
	if cmd == nil {
		t.Error("expected non-nil cmd after retry")
	}
}

func TestImageGenModel_GeneratingEscapeGoesBackToPrompt(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set to generating step with error (so esc is allowed)
	model.Step = ImageGenStepGenerating
	model.IsGenerating = false
	model.Prompt = "Test prompt"
	model.Error = "Previous error"

	// Press esc to go back
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m := newModel.(*ImageGenModel)

	// Should go back to prompt step
	if m.Step != ImageGenStepPrompt {
		t.Errorf("expected ImageGenStepPrompt after esc, got %d", m.Step)
	}

	// Error should be cleared
	if m.Error != "" {
		t.Errorf("Error should be cleared after going back, got %q", m.Error)
	}

	// IsGenerating should be false
	if m.IsGenerating {
		t.Error("IsGenerating should be false after going back")
	}
}

func TestImageGenModel_GeneratingIgnoresKeysWhileActive(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set to generating step (actively generating, no error)
	model.Step = ImageGenStepGenerating
	model.IsGenerating = true
	model.Prompt = "Test prompt"
	model.Error = "" // No error, so keys should be ignored

	// Try to press esc - should be ignored while generating
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m := newModel.(*ImageGenModel)

	// Should still be in generating step
	if m.Step != ImageGenStepGenerating {
		t.Errorf("expected to stay in ImageGenStepGenerating while generating, got %d", m.Step)
	}

	// Try to press 'r' - should be ignored while generating
	newModel, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("r")})
	m = newModel.(*ImageGenModel)

	// Should still be in generating step
	if m.Step != ImageGenStepGenerating {
		t.Errorf("expected to stay in ImageGenStepGenerating while generating, got %d", m.Step)
	}
}

func TestImageGenModel_SpinnerTickUpdate(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set to generating step
	model.Step = ImageGenStepGenerating
	model.IsGenerating = true
	model.Prompt = "Test prompt"

	// Send spinner tick message
	newModel, cmd := model.Update(spinner.TickMsg{})
	m := newModel.(*ImageGenModel)

	// Should still be in generating step
	if m.Step != ImageGenStepGenerating {
		t.Errorf("expected ImageGenStepGenerating after spinner tick, got %d", m.Step)
	}

	// Should return a command (for next spinner tick)
	if cmd == nil {
		t.Error("expected non-nil cmd for next spinner tick")
	}
}

func TestFormatAPIError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		contains string
	}{
		{
			name:     "nil error",
			err:      nil,
			contains: "",
		},
		{
			name:     "auth error",
			err:      &gemini.APIError{Type: gemini.ErrorTypeAuth, Message: "invalid key"},
			contains: "Authentication",
		},
		{
			name:     "rate limit error",
			err:      &gemini.APIError{Type: gemini.ErrorTypeRateLimit, Message: "too many requests"},
			contains: "Rate limit",
		},
		{
			name:     "content policy error",
			err:      &gemini.APIError{Type: gemini.ErrorTypeContentPolicy, Message: "blocked"},
			contains: "content policy",
		},
		{
			name:     "invalid request error",
			err:      &gemini.APIError{Type: gemini.ErrorTypeInvalidRequest, Message: "bad request"},
			contains: "Invalid request",
		},
		{
			name:     "no image error",
			err:      &gemini.APIError{Type: gemini.ErrorTypeNoImage, Message: "no image"},
			contains: "No image was generated",
		},
		{
			name:     "network error",
			err:      &gemini.APIError{Type: gemini.ErrorTypeNetwork, Message: "connection failed"},
			contains: "Network error",
		},
		{
			name:     "server error",
			err:      &gemini.APIError{Type: gemini.ErrorTypeServer, Message: "internal error"},
			contains: "Server error",
		},
		{
			name:     "generic error",
			err:      errors.New("something went wrong"),
			contains: "Failed to generate image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := formatAPIError(tt.err)
			if tt.contains == "" {
				if result != "" {
					t.Errorf("expected empty string, got %q", result)
				}
			} else {
				if !strings.Contains(result, tt.contains) {
					t.Errorf("expected result to contain %q, got %q", tt.contains, result)
				}
			}
		})
	}
}

func TestImageGenModel_NewModelHasSpinner(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Spinner should be initialized
	view := model.spinner.View()
	if view == "" {
		t.Error("spinner should be initialized and have a view")
	}
}

func TestImageGenModel_GeneratedImageFieldInitiallyNil(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	if model.GeneratedImage != nil {
		t.Error("GeneratedImage should be nil initially")
	}

	if model.IsGenerating {
		t.Error("IsGenerating should be false initially")
	}
}

// Tests for images directory creation (US-015)

func TestImageGenModel_GetImagesDir(t *testing.T) {
	tests := []struct {
		name           string
		markdownPath   string
		expectedSuffix string
	}{
		{
			name:           "markdown in root directory",
			markdownPath:   "/presentations/slides.md",
			expectedSuffix: "/presentations/images",
		},
		{
			name:           "markdown in nested directory",
			markdownPath:   "/home/user/projects/docs/talk.md",
			expectedSuffix: "/home/user/projects/docs/images",
		},
		{
			name:           "markdown with relative path",
			markdownPath:   "docs/presentation.md",
			expectedSuffix: "docs/images",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			model := &ImageGenModel{MarkdownFile: tt.markdownPath}
			result := model.GetImagesDir()

			if result != tt.expectedSuffix {
				t.Errorf("expected %q, got %q", tt.expectedSuffix, result)
			}
		})
	}
}

func TestImageGenModel_EnsureImagesDir_CreatesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "slides.md")

	content := `# Test Slide`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Images directory should not exist yet
	imagesDir := filepath.Join(tmpDir, "images")
	if _, err := os.Stat(imagesDir); err == nil {
		t.Fatal("images directory should not exist yet")
	}

	// Call EnsureImagesDir
	resultDir, err := model.EnsureImagesDir()
	if err != nil {
		t.Fatalf("EnsureImagesDir failed: %v", err)
	}

	// Check returned path is correct
	if resultDir != imagesDir {
		t.Errorf("expected %q, got %q", imagesDir, resultDir)
	}

	// Check directory was created
	info, err := os.Stat(imagesDir)
	if err != nil {
		t.Fatalf("images directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("images path should be a directory")
	}
}

func TestImageGenModel_EnsureImagesDir_DirectoryAlreadyExists(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "slides.md")

	content := `# Test Slide`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Pre-create the images directory
	imagesDir := filepath.Join(tmpDir, "images")
	if err := os.Mkdir(imagesDir, 0755); err != nil {
		t.Fatalf("failed to create images directory: %v", err)
	}

	// Create a file in it to verify it's not replaced
	testFile := filepath.Join(imagesDir, "existing.png")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Call EnsureImagesDir
	resultDir, err := model.EnsureImagesDir()
	if err != nil {
		t.Fatalf("EnsureImagesDir failed: %v", err)
	}

	// Check returned path is correct
	if resultDir != imagesDir {
		t.Errorf("expected %q, got %q", imagesDir, resultDir)
	}

	// Check existing file is still there
	if _, err := os.Stat(testFile); err != nil {
		t.Error("existing file in images directory should not be affected")
	}
}

func TestImageGenModel_EnsureImagesDir_PathIsFile(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "slides.md")

	content := `# Test Slide`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	// Create "images" as a file instead of directory
	imagesPath := filepath.Join(tmpDir, "images")
	if err := os.WriteFile(imagesPath, []byte("not a directory"), 0644); err != nil {
		t.Fatalf("failed to write images file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Call EnsureImagesDir - should fail
	_, err = model.EnsureImagesDir()
	if err == nil {
		t.Error("EnsureImagesDir should fail when images path is a file")
	}
	if !strings.Contains(err.Error(), "not a directory") {
		t.Errorf("error should mention 'not a directory', got: %v", err)
	}
}

func TestImageGenModel_EnsureImagesDir_NestedDirectory(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a nested directory structure for the markdown file
	nestedDir := filepath.Join(tmpDir, "projects", "presentations", "2024")
	if err := os.MkdirAll(nestedDir, 0755); err != nil {
		t.Fatalf("failed to create nested directory: %v", err)
	}

	mdFile := filepath.Join(nestedDir, "talk.md")
	content := `# Test Slide`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Call EnsureImagesDir
	resultDir, err := model.EnsureImagesDir()
	if err != nil {
		t.Fatalf("EnsureImagesDir failed: %v", err)
	}

	// Check returned path is correct
	expectedDir := filepath.Join(nestedDir, "images")
	if resultDir != expectedDir {
		t.Errorf("expected %q, got %q", expectedDir, resultDir)
	}

	// Check directory was created
	info, err := os.Stat(expectedDir)
	if err != nil {
		t.Fatalf("images directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("images path should be a directory")
	}
}

func TestImageGenModel_EnsureImagesDir_Idempotent(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "slides.md")

	content := `# Test Slide`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Call EnsureImagesDir multiple times
	dir1, err1 := model.EnsureImagesDir()
	dir2, err2 := model.EnsureImagesDir()
	dir3, err3 := model.EnsureImagesDir()

	// All calls should succeed
	if err1 != nil {
		t.Errorf("first call failed: %v", err1)
	}
	if err2 != nil {
		t.Errorf("second call failed: %v", err2)
	}
	if err3 != nil {
		t.Errorf("third call failed: %v", err3)
	}

	// All calls should return the same path
	if dir1 != dir2 || dir2 != dir3 {
		t.Errorf("paths should be identical: %q, %q, %q", dir1, dir2, dir3)
	}
}

func TestImageGenModel_EnsureImagesDir_AbsolutePath(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "slides.md")

	content := `# Test Slide`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Call EnsureImagesDir
	resultDir, err := model.EnsureImagesDir()
	if err != nil {
		t.Fatalf("EnsureImagesDir failed: %v", err)
	}

	// Result should be an absolute path since mdFile was absolute
	if !filepath.IsAbs(resultDir) {
		t.Errorf("expected absolute path, got %q", resultDir)
	}
}

// Tests for image saving (US-016)

func TestGenerateImageFilename(t *testing.T) {
	tests := []struct {
		name           string
		imageData      []byte
		contentType    string
		expectedExt    string
		expectedPrefix string
	}{
		{
			name:           "PNG image",
			imageData:      []byte("fake png data"),
			contentType:    "image/png",
			expectedExt:    ".png",
			expectedPrefix: "generated-",
		},
		{
			name:           "JPEG image",
			imageData:      []byte("fake jpeg data"),
			contentType:    "image/jpeg",
			expectedExt:    ".jpg",
			expectedPrefix: "generated-",
		},
		{
			name:           "JPG content type",
			imageData:      []byte("fake jpg data"),
			contentType:    "image/jpg",
			expectedExt:    ".jpg",
			expectedPrefix: "generated-",
		},
		{
			name:           "GIF image",
			imageData:      []byte("fake gif data"),
			contentType:    "image/gif",
			expectedExt:    ".gif",
			expectedPrefix: "generated-",
		},
		{
			name:           "WebP image",
			imageData:      []byte("fake webp data"),
			contentType:    "image/webp",
			expectedExt:    ".webp",
			expectedPrefix: "generated-",
		},
		{
			name:           "Unknown content type defaults to PNG",
			imageData:      []byte("unknown data"),
			contentType:    "application/octet-stream",
			expectedExt:    ".png",
			expectedPrefix: "generated-",
		},
		{
			name:           "Empty content type defaults to PNG",
			imageData:      []byte("empty type data"),
			contentType:    "",
			expectedExt:    ".png",
			expectedPrefix: "generated-",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filename := GenerateImageFilename(tt.imageData, tt.contentType)

			// Check prefix
			if !strings.HasPrefix(filename, tt.expectedPrefix) {
				t.Errorf("expected filename to start with %q, got %q", tt.expectedPrefix, filename)
			}

			// Check extension
			if !strings.HasSuffix(filename, tt.expectedExt) {
				t.Errorf("expected filename to end with %q, got %q", tt.expectedExt, filename)
			}

			// Check hash length (8 characters)
			// Format is "generated-{8 chars}.{ext}"
			hashStart := len("generated-")
			hashEnd := strings.LastIndex(filename, ".")
			if hashEnd-hashStart != 8 {
				t.Errorf("expected 8-character hash, got %d characters", hashEnd-hashStart)
			}

			// Check hash is valid hex
			hash := filename[hashStart:hashEnd]
			for _, c := range hash {
				if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
					t.Errorf("hash contains invalid hex character: %c", c)
				}
			}
		})
	}
}

func TestGenerateImageFilename_DifferentDataDifferentHash(t *testing.T) {
	data1 := []byte("first image data")
	data2 := []byte("second image data")

	filename1 := GenerateImageFilename(data1, "image/png")
	filename2 := GenerateImageFilename(data2, "image/png")

	if filename1 == filename2 {
		t.Error("different data should produce different filenames")
	}
}

func TestGenerateImageFilename_SameDataSameHash(t *testing.T) {
	data := []byte("identical image data")

	filename1 := GenerateImageFilename(data, "image/png")
	filename2 := GenerateImageFilename(data, "image/png")

	if filename1 != filename2 {
		t.Errorf("same data should produce same filename: %q != %q", filename1, filename2)
	}
}

func TestGetExtensionFromContentType(t *testing.T) {
	tests := []struct {
		contentType string
		expected    string
	}{
		{"image/png", "png"},
		{"image/jpeg", "jpg"},
		{"image/jpg", "jpg"},
		{"image/gif", "gif"},
		{"image/webp", "webp"},
		{"application/octet-stream", "png"},
		{"", "png"},
		{"text/html", "png"},
	}

	for _, tt := range tests {
		t.Run(tt.contentType, func(t *testing.T) {
			result := GetExtensionFromContentType(tt.contentType)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestImageGenModel_SaveGeneratedImage(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "slides.md")

	content := `# Test Slide`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set up generated image
	imageData := []byte("fake PNG image data for testing")
	model.GeneratedImage = &ImageGenerateResult{
		ImageData:   imageData,
		ContentType: "image/png",
	}

	// Save the image
	relativePath, err := model.SaveGeneratedImage()
	if err != nil {
		t.Fatalf("SaveGeneratedImage failed: %v", err)
	}

	// Check relative path format
	if !strings.HasPrefix(relativePath, "images/") {
		t.Errorf("expected relative path to start with 'images/', got %q", relativePath)
	}
	if !strings.HasSuffix(relativePath, ".png") {
		t.Errorf("expected relative path to end with '.png', got %q", relativePath)
	}

	// Check file was created
	fullPath := filepath.Join(tmpDir, relativePath)
	savedData, err := os.ReadFile(fullPath)
	if err != nil {
		t.Fatalf("failed to read saved file: %v", err)
	}

	// Check file content matches
	if string(savedData) != string(imageData) {
		t.Error("saved file content does not match original image data")
	}
}

func TestImageGenModel_SaveGeneratedImage_JPEG(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "slides.md")

	content := `# Test Slide`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set up generated JPEG image
	model.GeneratedImage = &ImageGenerateResult{
		ImageData:   []byte("fake JPEG image data"),
		ContentType: "image/jpeg",
	}

	// Save the image
	relativePath, err := model.SaveGeneratedImage()
	if err != nil {
		t.Fatalf("SaveGeneratedImage failed: %v", err)
	}

	// Check extension is jpg
	if !strings.HasSuffix(relativePath, ".jpg") {
		t.Errorf("expected relative path to end with '.jpg', got %q", relativePath)
	}
}

func TestImageGenModel_SaveGeneratedImage_NoGeneratedImage(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "slides.md")

	content := `# Test Slide`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Don't set GeneratedImage (should be nil)

	// Try to save - should fail
	_, err = model.SaveGeneratedImage()
	if err == nil {
		t.Error("SaveGeneratedImage should fail when GeneratedImage is nil")
	}
	if !strings.Contains(err.Error(), "no generated image") {
		t.Errorf("error should mention 'no generated image', got: %v", err)
	}
}

func TestImageGenModel_SaveGeneratedImage_CreatesImagesDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "slides.md")

	content := `# Test Slide`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Verify images dir doesn't exist
	imagesDir := filepath.Join(tmpDir, "images")
	if _, err := os.Stat(imagesDir); err == nil {
		t.Fatal("images directory should not exist yet")
	}

	// Set up generated image
	model.GeneratedImage = &ImageGenerateResult{
		ImageData:   []byte("test image data"),
		ContentType: "image/png",
	}

	// Save the image
	_, err = model.SaveGeneratedImage()
	if err != nil {
		t.Fatalf("SaveGeneratedImage failed: %v", err)
	}

	// Verify images directory was created
	info, err := os.Stat(imagesDir)
	if err != nil {
		t.Fatalf("images directory should exist: %v", err)
	}
	if !info.IsDir() {
		t.Error("images should be a directory")
	}
}

func TestImageGenModel_SaveGeneratedImage_FilePermissions(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "slides.md")

	content := `# Test Slide`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	model.GeneratedImage = &ImageGenerateResult{
		ImageData:   []byte("test image data"),
		ContentType: "image/png",
	}

	relativePath, err := model.SaveGeneratedImage()
	if err != nil {
		t.Fatalf("SaveGeneratedImage failed: %v", err)
	}

	// Check file permissions (0644)
	fullPath := filepath.Join(tmpDir, relativePath)
	info, err := os.Stat(fullPath)
	if err != nil {
		t.Fatalf("failed to stat file: %v", err)
	}

	// Check file is readable
	perm := info.Mode().Perm()
	if perm&0400 == 0 {
		t.Error("file should be readable by owner")
	}
	if perm&0200 == 0 {
		t.Error("file should be writable by owner")
	}
}

func TestImageGenModel_SaveGeneratedImage_ReturnsRelativePath(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "slides.md")

	content := `# Test Slide`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	model.GeneratedImage = &ImageGenerateResult{
		ImageData:   []byte("test image data"),
		ContentType: "image/png",
	}

	relativePath, err := model.SaveGeneratedImage()
	if err != nil {
		t.Fatalf("SaveGeneratedImage failed: %v", err)
	}

	// Path should be relative (not absolute)
	if filepath.IsAbs(relativePath) {
		t.Errorf("expected relative path, got absolute path: %q", relativePath)
	}

	// Path should start with "images/"
	if !strings.HasPrefix(relativePath, "images/") {
		t.Errorf("expected path to start with 'images/', got: %q", relativePath)
	}

	// Filename should start with "generated-"
	filename := filepath.Base(relativePath)
	if !strings.HasPrefix(filename, "generated-") {
		t.Errorf("expected filename to start with 'generated-', got: %q", filename)
	}
}

// Tests for markdown insertion (US-017)

func TestInsertImageIntoSlide_SingleSlide(t *testing.T) {
	content := `# My Slide

Some content here`

	result, err := insertImageIntoSlide(content, 0, "A test prompt", "images/generated-abc123.png")
	if err != nil {
		t.Fatalf("insertImageIntoSlide failed: %v", err)
	}

	// Should contain the AI prompt comment
	if !strings.Contains(result, "<!-- ai-prompt: A test prompt -->") {
		t.Error("result should contain AI prompt comment")
	}

	// Should contain the image reference
	if !strings.Contains(result, "![](images/generated-abc123.png)") {
		t.Error("result should contain image reference")
	}

	// AI prompt should come before image
	commentIdx := strings.Index(result, "<!-- ai-prompt:")
	imageIdx := strings.Index(result, "![](images/")
	if commentIdx >= imageIdx {
		t.Error("AI prompt comment should come before image reference")
	}

	// Original content should be preserved
	if !strings.Contains(result, "# My Slide") {
		t.Error("original heading should be preserved")
	}
	if !strings.Contains(result, "Some content here") {
		t.Error("original content should be preserved")
	}
}

func TestInsertImageIntoSlide_MultipleSlides(t *testing.T) {
	content := `# First Slide

Content one

---

# Second Slide

Content two

---

# Third Slide

Content three`

	// Insert into second slide
	result, err := insertImageIntoSlide(content, 1, "Second slide image", "images/second.png")
	if err != nil {
		t.Fatalf("insertImageIntoSlide failed: %v", err)
	}

	// Should contain the AI prompt comment
	if !strings.Contains(result, "<!-- ai-prompt: Second slide image -->") {
		t.Error("result should contain AI prompt comment")
	}

	// All slides should still be present
	if !strings.Contains(result, "# First Slide") {
		t.Error("first slide should be preserved")
	}
	if !strings.Contains(result, "# Second Slide") {
		t.Error("second slide should be preserved")
	}
	if !strings.Contains(result, "# Third Slide") {
		t.Error("third slide should be preserved")
	}

	// Separators should still be present
	if strings.Count(result, "---") != 2 {
		t.Errorf("expected 2 slide separators, got %d", strings.Count(result, "---"))
	}

	// Image should be in the second slide section (between first and second ---)
	parts := strings.Split(result, "---")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}
	if !strings.Contains(parts[1], "![](images/second.png)") {
		t.Error("image should be in second slide section")
	}
	if strings.Contains(parts[0], "![](images/second.png)") {
		t.Error("image should not be in first slide section")
	}
	if strings.Contains(parts[2], "![](images/second.png)") {
		t.Error("image should not be in third slide section")
	}
}

func TestInsertImageIntoSlide_WithFrontmatter(t *testing.T) {
	content := `---
title: My Presentation
theme: paper
---

# First Slide

Content here

---

# Second Slide

More content`

	result, err := insertImageIntoSlide(content, 0, "First slide prompt", "images/first.png")
	if err != nil {
		t.Fatalf("insertImageIntoSlide failed: %v", err)
	}

	// Frontmatter should be preserved
	if !strings.Contains(result, "title: My Presentation") {
		t.Error("frontmatter title should be preserved")
	}
	if !strings.Contains(result, "theme: paper") {
		t.Error("frontmatter theme should be preserved")
	}

	// Image should be added to first slide
	if !strings.Contains(result, "<!-- ai-prompt: First slide prompt -->") {
		t.Error("AI prompt comment should be present")
	}
	if !strings.Contains(result, "![](images/first.png)") {
		t.Error("image reference should be present")
	}
}

func TestInsertImageIntoSlide_InvalidSlideIndex(t *testing.T) {
	content := `# Only Slide

Content`

	// Try to insert into non-existent slide
	_, err := insertImageIntoSlide(content, 5, "prompt", "images/test.png")
	if err == nil {
		t.Error("expected error for invalid slide index")
	}
	if !strings.Contains(err.Error(), "invalid slide index") {
		t.Errorf("error should mention 'invalid slide index', got: %v", err)
	}

	// Try negative index
	_, err = insertImageIntoSlide(content, -1, "prompt", "images/test.png")
	if err == nil {
		t.Error("expected error for negative slide index")
	}
}

func TestInsertImageIntoSlide_EmptySlidesSkipped(t *testing.T) {
	content := `# First Slide

Content

---



---

# Third Slide

More content`

	// Empty slide (index 1 would be empty, but it's skipped)
	// So slide index 1 should be "Third Slide"
	result, err := insertImageIntoSlide(content, 1, "Third slide image", "images/third.png")
	if err != nil {
		t.Fatalf("insertImageIntoSlide failed: %v", err)
	}

	// Image should be associated with "Third Slide"
	parts := strings.Split(result, "---")
	// Third part should contain the image
	found := false
	for _, part := range parts {
		if strings.Contains(part, "# Third Slide") && strings.Contains(part, "![](images/third.png)") {
			found = true
			break
		}
	}
	if !found {
		t.Error("image should be in the Third Slide section")
	}
}

func TestInsertImageIntoSlide_PreservesExistingImages(t *testing.T) {
	content := `# Slide With Images

Some text

<!-- ai-prompt: existing prompt -->
![](images/existing.png)

More text`

	result, err := insertImageIntoSlide(content, 0, "new prompt", "images/new.png")
	if err != nil {
		t.Fatalf("insertImageIntoSlide failed: %v", err)
	}

	// Existing image should be preserved
	if !strings.Contains(result, "![](images/existing.png)") {
		t.Error("existing image should be preserved")
	}
	if !strings.Contains(result, "<!-- ai-prompt: existing prompt -->") {
		t.Error("existing AI prompt should be preserved")
	}

	// New image should be added
	if !strings.Contains(result, "![](images/new.png)") {
		t.Error("new image should be added")
	}
	if !strings.Contains(result, "<!-- ai-prompt: new prompt -->") {
		t.Error("new AI prompt should be added")
	}
}

func TestInsertImageIntoSlide_LastSlide(t *testing.T) {
	content := `# First

Content

---

# Second

Content

---

# Last Slide

Final content`

	result, err := insertImageIntoSlide(content, 2, "last prompt", "images/last.png")
	if err != nil {
		t.Fatalf("insertImageIntoSlide failed: %v", err)
	}

	// Image should be in last slide
	if !strings.Contains(result, "<!-- ai-prompt: last prompt -->") {
		t.Error("AI prompt should be present")
	}
	if !strings.Contains(result, "![](images/last.png)") {
		t.Error("image reference should be present")
	}

	// Should not create extra separators
	if strings.Count(result, "---") != 2 {
		t.Errorf("expected 2 slide separators, got %d", strings.Count(result, "---"))
	}
}

func TestInsertImageIntoSlide_SpecialCharactersInPrompt(t *testing.T) {
	content := `# Slide

Content`

	// Prompt with special characters
	prompt := "A beautiful sunset with \"quotes\" and special chars: <>&"
	result, err := insertImageIntoSlide(content, 0, prompt, "images/special.png")
	if err != nil {
		t.Fatalf("insertImageIntoSlide failed: %v", err)
	}

	// Prompt should be preserved exactly
	expectedComment := "<!-- ai-prompt: A beautiful sunset with \"quotes\" and special chars: <>&"
	if !strings.Contains(result, expectedComment) {
		t.Errorf("expected prompt with special characters to be preserved, got:\n%s", result)
	}
}

func TestImageGenModel_InsertImageIntoMarkdown(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "slides.md")

	content := `# Test Slide

Some content here

---

# Second Slide

More content`

	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set up model state
	model.SelectedIndex = 0
	model.Prompt = "A test image"

	// Insert image
	err = model.InsertImageIntoMarkdown("images/test-image.png")
	if err != nil {
		t.Fatalf("InsertImageIntoMarkdown failed: %v", err)
	}

	// Read back the file
	updatedContent, err := os.ReadFile(mdFile)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}

	// Verify content was updated
	if !strings.Contains(string(updatedContent), "<!-- ai-prompt: A test image -->") {
		t.Error("updated content should contain AI prompt comment")
	}
	if !strings.Contains(string(updatedContent), "![](images/test-image.png)") {
		t.Error("updated content should contain image reference")
	}

	// Verify original content preserved
	if !strings.Contains(string(updatedContent), "# Test Slide") {
		t.Error("original content should be preserved")
	}
	if !strings.Contains(string(updatedContent), "# Second Slide") {
		t.Error("second slide should be preserved")
	}
}

func TestImageGenModel_InsertImageIntoMarkdown_FileNotFound(t *testing.T) {
	model := &ImageGenModel{
		MarkdownFile:  "/nonexistent/path/slides.md",
		SelectedIndex: 0,
		Prompt:        "test",
	}

	err := model.InsertImageIntoMarkdown("images/test.png")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

func TestImageGenModel_InsertImageIntoMarkdown_SecondSlide(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "slides.md")

	content := `# First Slide

Content one

---

# Second Slide

Content two

---

# Third Slide

Content three`

	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Select second slide
	model.SelectedIndex = 1
	model.Prompt = "Second slide image"

	// Insert image
	err = model.InsertImageIntoMarkdown("images/second-slide.png")
	if err != nil {
		t.Fatalf("InsertImageIntoMarkdown failed: %v", err)
	}

	// Read back the file
	updatedContent, err := os.ReadFile(mdFile)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}

	// Image should be in second slide section
	parts := strings.Split(string(updatedContent), "---")
	if len(parts) != 3 {
		t.Fatalf("expected 3 parts, got %d", len(parts))
	}

	// Second part should contain the image
	if !strings.Contains(parts[1], "![](images/second-slide.png)") {
		t.Error("image should be in second slide section")
	}

	// Other parts should not contain the image
	if strings.Contains(parts[0], "second-slide.png") {
		t.Error("image should not be in first slide section")
	}
	if strings.Contains(parts[2], "second-slide.png") {
		t.Error("image should not be in third slide section")
	}
}

// Tests for US-018: Handle image regeneration

func TestDeleteOldImage(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	imagesDir := filepath.Join(tmpDir, "images")

	// Create images directory and an old image
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatalf("failed to create images directory: %v", err)
	}

	oldImagePath := filepath.Join(imagesDir, "old-image.png")
	if err := os.WriteFile(oldImagePath, []byte("fake image data"), 0644); err != nil {
		t.Fatalf("failed to create old image file: %v", err)
	}

	content := `# Test Slide

<!-- ai-prompt: old prompt -->
![](images/old-image.png)
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set up the selected image (simulating regeneration workflow)
	model.SelectedImage = &AIImageInfo{
		Prompt:    "old prompt",
		ImagePath: "images/old-image.png",
	}

	// Verify the file exists before deletion
	if _, err := os.Stat(oldImagePath); os.IsNotExist(err) {
		t.Fatal("old image file should exist before deletion")
	}

	// Delete the old image
	err = model.DeleteOldImage()
	if err != nil {
		t.Fatalf("DeleteOldImage failed: %v", err)
	}

	// Verify the file was deleted
	if _, err := os.Stat(oldImagePath); !os.IsNotExist(err) {
		t.Error("old image file should be deleted")
	}
}

func TestDeleteOldImage_FileDoesNotExist(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

<!-- ai-prompt: old prompt -->
![](images/nonexistent.png)
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set up the selected image pointing to a non-existent file
	model.SelectedImage = &AIImageInfo{
		Prompt:    "old prompt",
		ImagePath: "images/nonexistent.png",
	}

	// Should not error when file doesn't exist
	err = model.DeleteOldImage()
	if err != nil {
		t.Errorf("DeleteOldImage should not error when file doesn't exist: %v", err)
	}
}

func TestDeleteOldImage_NoSelectedImage(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// SelectedImage is nil (adding new image, not regenerating)
	model.SelectedImage = nil

	// Should not error when no selected image
	err = model.DeleteOldImage()
	if err != nil {
		t.Errorf("DeleteOldImage should not error when SelectedImage is nil: %v", err)
	}
}

func TestReplaceImageInContent(t *testing.T) {
	tests := []struct {
		name         string
		content      string
		oldPrompt    string
		oldImagePath string
		newPrompt    string
		newImagePath string
		expected     string
		wantErr      bool
	}{
		{
			name: "simple replacement",
			content: `# Test Slide

<!-- ai-prompt: old prompt -->
![](images/old.png)

More content`,
			oldPrompt:    "old prompt",
			oldImagePath: "images/old.png",
			newPrompt:    "new prompt",
			newImagePath: "images/new.png",
			expected: `# Test Slide

<!-- ai-prompt: new prompt -->
![](images/new.png)

More content`,
			wantErr: false,
		},
		{
			name: "replacement with leading spaces on image line",
			content: `# Test Slide

<!-- ai-prompt: test prompt -->
  ![](images/test.png)

Content`,
			oldPrompt:    "test prompt",
			oldImagePath: "images/test.png",
			newPrompt:    "updated prompt",
			newImagePath: "images/updated.png",
			expected: `# Test Slide

<!-- ai-prompt: updated prompt -->
![](images/updated.png)

Content`,
			wantErr: false,
		},
		{
			name: "replacement in multi-slide markdown",
			content: `---
theme: paper
---

# Slide 1

Content one

---

# Slide 2

<!-- ai-prompt: middle slide image -->
![](images/middle.png)

---

# Slide 3

Content three`,
			oldPrompt:    "middle slide image",
			oldImagePath: "images/middle.png",
			newPrompt:    "regenerated image",
			newImagePath: "images/regen.png",
			expected: `---
theme: paper
---

# Slide 1

Content one

---

# Slide 2

<!-- ai-prompt: regenerated image -->
![](images/regen.png)

---

# Slide 3

Content three`,
			wantErr: false,
		},
		{
			name: "prompt not found",
			content: `# Test Slide

<!-- ai-prompt: different prompt -->
![](images/test.png)`,
			oldPrompt:    "nonexistent prompt",
			oldImagePath: "images/test.png",
			newPrompt:    "new prompt",
			newImagePath: "images/new.png",
			expected:     "",
			wantErr:      true,
		},
		{
			name: "image path not found",
			content: `# Test Slide

<!-- ai-prompt: test prompt -->
![](images/test.png)`,
			oldPrompt:    "test prompt",
			oldImagePath: "images/different.png",
			newPrompt:    "new prompt",
			newImagePath: "images/new.png",
			expected:     "",
			wantErr:      true,
		},
		{
			name: "replacement with special characters in prompt",
			content: `# Test Slide

<!-- ai-prompt: a (special) prompt with [brackets] and $symbols -->
![](images/special.png)`,
			oldPrompt:    "a (special) prompt with [brackets] and $symbols",
			oldImagePath: "images/special.png",
			newPrompt:    "a new (special) prompt",
			newImagePath: "images/new-special.png",
			expected: `# Test Slide

<!-- ai-prompt: a new (special) prompt -->
![](images/new-special.png)`,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := replaceImageInContent(tt.content, tt.oldPrompt, tt.oldImagePath, tt.newPrompt, tt.newImagePath)
			if (err != nil) != tt.wantErr {
				t.Errorf("replaceImageInContent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && result != tt.expected {
				t.Errorf("replaceImageInContent() got:\n%s\n\nwant:\n%s", result, tt.expected)
			}
		})
	}
}

func TestReplaceImageInMarkdown_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	imagesDir := filepath.Join(tmpDir, "images")

	// Create images directory
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatalf("failed to create images directory: %v", err)
	}

	// Create old and new image files
	oldImagePath := filepath.Join(imagesDir, "old.png")
	if err := os.WriteFile(oldImagePath, []byte("old image data"), 0644); err != nil {
		t.Fatalf("failed to create old image file: %v", err)
	}

	content := `# Test Slide

Some content before

<!-- ai-prompt: generate a sunset -->
![](images/old.png)

Some content after
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set up for regeneration
	model.SelectedImage = &AIImageInfo{
		Prompt:    "generate a sunset",
		ImagePath: "images/old.png",
	}
	model.Prompt = "generate a beautiful sunset"

	// Replace the image
	err = model.ReplaceImageInMarkdown("images/new.png")
	if err != nil {
		t.Fatalf("ReplaceImageInMarkdown failed: %v", err)
	}

	// Read back and verify
	updatedContent, err := os.ReadFile(mdFile)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}

	// Should contain new prompt
	if !strings.Contains(string(updatedContent), "<!-- ai-prompt: generate a beautiful sunset -->") {
		t.Error("should contain new prompt comment")
	}

	// Should contain new image path
	if !strings.Contains(string(updatedContent), "![](images/new.png)") {
		t.Error("should contain new image path")
	}

	// Should not contain old prompt
	if strings.Contains(string(updatedContent), "<!-- ai-prompt: generate a sunset -->") {
		t.Error("should not contain old prompt comment")
	}

	// Should not contain old image path
	if strings.Contains(string(updatedContent), "![](images/old.png)") {
		t.Error("should not contain old image path")
	}

	// Should preserve content before and after
	if !strings.Contains(string(updatedContent), "Some content before") {
		t.Error("should preserve content before image")
	}
	if !strings.Contains(string(updatedContent), "Some content after") {
		t.Error("should preserve content after image")
	}
}

func TestReplaceImageInMarkdown_PreservesPosition(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	imagesDir := filepath.Join(tmpDir, "images")

	// Create images directory
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatalf("failed to create images directory: %v", err)
	}

	// Content with image in the middle of the slide
	content := `# Test Slide

Paragraph one.

<!-- ai-prompt: middle image -->
![](images/middle.png)

Paragraph two.

End of slide.
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set up for regeneration
	model.SelectedImage = &AIImageInfo{
		Prompt:    "middle image",
		ImagePath: "images/middle.png",
	}
	model.Prompt = "regenerated middle image"

	// Replace the image
	err = model.ReplaceImageInMarkdown("images/new-middle.png")
	if err != nil {
		t.Fatalf("ReplaceImageInMarkdown failed: %v", err)
	}

	// Read back and verify position is preserved
	updatedContent, err := os.ReadFile(mdFile)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}

	contentStr := string(updatedContent)

	// Find positions
	paragraphOnePos := strings.Index(contentStr, "Paragraph one.")
	imagePos := strings.Index(contentStr, "![](images/new-middle.png)")
	paragraphTwoPos := strings.Index(contentStr, "Paragraph two.")
	endPos := strings.Index(contentStr, "End of slide.")

	// Verify ordering is preserved
	if paragraphOnePos >= imagePos {
		t.Error("Paragraph one should come before the image")
	}
	if imagePos >= paragraphTwoPos {
		t.Error("Image should come before Paragraph two")
	}
	if paragraphTwoPos >= endPos {
		t.Error("Paragraph two should come before End of slide")
	}
}

func TestReplaceImageInMarkdown_NoSelectedImage(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// SelectedImage is nil
	model.SelectedImage = nil
	model.Prompt = "some prompt"

	// Should error when no selected image
	err = model.ReplaceImageInMarkdown("images/new.png")
	if err == nil {
		t.Error("ReplaceImageInMarkdown should error when SelectedImage is nil")
	}
	if !strings.Contains(err.Error(), "no selected image") {
		t.Errorf("error should mention 'no selected image', got: %v", err)
	}
}

func TestImageRegeneration_FullWorkflow_Integration(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")
	imagesDir := filepath.Join(tmpDir, "images")

	// Create images directory and old image
	if err := os.MkdirAll(imagesDir, 0755); err != nil {
		t.Fatalf("failed to create images directory: %v", err)
	}

	oldImagePath := filepath.Join(imagesDir, "generated-abc12345.png")
	oldImageData := []byte("old image content")
	if err := os.WriteFile(oldImagePath, oldImageData, 0644); err != nil {
		t.Fatalf("failed to create old image file: %v", err)
	}

	content := `---
theme: paper
---

# Test Slide

Some content here

<!-- ai-prompt: original prompt -->
![](images/generated-abc12345.png)

More content below
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Simulate the full regeneration workflow

	// 1. Set up selected image (from image selection step)
	model.SelectedImage = &AIImageInfo{
		Prompt:    "original prompt",
		ImagePath: "images/generated-abc12345.png",
	}
	model.Prompt = "updated prompt for regeneration"

	// 2. Simulate successful image generation
	newImageData := []byte("new image content for testing")
	model.GeneratedImage = &ImageGenerateResult{
		ImageData:   newImageData,
		ContentType: "image/png",
	}

	// 3. Save the new image
	newImageRelPath, err := model.SaveGeneratedImage()
	if err != nil {
		t.Fatalf("SaveGeneratedImage failed: %v", err)
	}

	// Verify new image was saved
	newImageFullPath := filepath.Join(tmpDir, newImageRelPath)
	if _, err := os.Stat(newImageFullPath); os.IsNotExist(err) {
		t.Error("new image file should exist")
	}

	// 4. Replace the image in markdown
	err = model.ReplaceImageInMarkdown(newImageRelPath)
	if err != nil {
		t.Fatalf("ReplaceImageInMarkdown failed: %v", err)
	}

	// 5. Delete the old image
	err = model.DeleteOldImage()
	if err != nil {
		t.Fatalf("DeleteOldImage failed: %v", err)
	}

	// Verify old image was deleted
	if _, err := os.Stat(oldImagePath); !os.IsNotExist(err) {
		t.Error("old image file should be deleted")
	}

	// Verify markdown was updated correctly
	updatedContent, err := os.ReadFile(mdFile)
	if err != nil {
		t.Fatalf("failed to read updated file: %v", err)
	}

	contentStr := string(updatedContent)

	// Should have new prompt and path
	if !strings.Contains(contentStr, "<!-- ai-prompt: updated prompt for regeneration -->") {
		t.Error("markdown should contain new prompt")
	}
	if !strings.Contains(contentStr, fmt.Sprintf("![](%s)", newImageRelPath)) {
		t.Errorf("markdown should contain new image path: %s", newImageRelPath)
	}

	// Should not have old prompt and path
	if strings.Contains(contentStr, "<!-- ai-prompt: original prompt -->") {
		t.Error("markdown should not contain old prompt")
	}
	if strings.Contains(contentStr, "![](images/generated-abc12345.png)") {
		t.Error("markdown should not contain old image path")
	}

	// Should preserve other content
	if !strings.Contains(contentStr, "Some content here") {
		t.Error("should preserve content before image")
	}
	if !strings.Contains(contentStr, "More content below") {
		t.Error("should preserve content after image")
	}
	if !strings.Contains(contentStr, "theme: paper") {
		t.Error("should preserve frontmatter")
	}
}

func TestImageGenModel_DoneStep(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set to done step with saved image
	model.Step = ImageGenStepDone
	model.SavedImagePath = "images/generated-abc12345.png"
	model.Prompt = "Test prompt"

	// Should be in done step
	if model.Step != ImageGenStepDone {
		t.Errorf("expected ImageGenStepDone, got %d", model.Step)
	}
}

func TestImageGenModel_DoneViewShowsSuccessMessage(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set to done step with saved image
	model.Step = ImageGenStepDone
	model.SavedImagePath = "images/generated-abc12345.png"
	model.Prompt = "Test prompt"

	view := model.View()

	// Should show success message
	if !strings.Contains(view, "Successfully") {
		t.Error("view should show success message")
	}

	// Should show saved image path
	if !strings.Contains(view, "images/generated-abc12345.png") {
		t.Error("view should show saved image path")
	}

	// Should show "Added new image" for new images
	if !strings.Contains(view, "Added new image") {
		t.Error("view should indicate new image was added")
	}
}

func TestImageGenModel_DoneViewShowsRegenerateMessage(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set to done step with selected image (regenerating)
	model.Step = ImageGenStepDone
	model.SavedImagePath = "images/generated-newpath.png"
	model.SelectedImage = &AIImageInfo{
		Prompt:    "old prompt",
		ImagePath: "images/generated-oldpath.png",
	}

	view := model.View()

	// Should show regenerate message
	if !strings.Contains(view, "Regenerated existing image") {
		t.Error("view should indicate existing image was regenerated")
	}
}

func TestImageGenModel_DoneEnterKeyReturnsNil(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set to done step
	model.Step = ImageGenStepDone
	model.SavedImagePath = "images/generated-abc12345.png"

	// Press enter - should return nil to signal completion
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEnter})

	if newModel != nil {
		t.Error("pressing enter in done step should return nil to signal completion")
	}
}

func TestImageGenModel_DoneEscKeyReturnsNil(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set to done step
	model.Step = ImageGenStepDone
	model.SavedImagePath = "images/generated-abc12345.png"

	// Press esc - should return nil to signal completion
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyEsc})

	if newModel != nil {
		t.Error("pressing esc in done step should return nil to signal completion")
	}
}

func TestImageGenModel_DoneSpaceKeyReturnsNil(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set to done step
	model.Step = ImageGenStepDone
	model.SavedImagePath = "images/generated-abc12345.png"

	// Press space - should return nil to signal completion
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(" ")})

	if newModel != nil {
		t.Error("pressing space in done step should return nil to signal completion")
	}
}

func TestImageGenModel_DoneOtherKeyIgnored(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Set to done step
	model.Step = ImageGenStepDone
	model.SavedImagePath = "images/generated-abc12345.png"

	// Press random key - should be ignored (stay in done step)
	newModel, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("x")})

	if newModel == nil {
		t.Error("pressing random key in done step should not close (should return model, not nil)")
	}

	m := newModel.(*ImageGenModel)
	if m.Step != ImageGenStepDone {
		t.Errorf("expected to stay in ImageGenStepDone, got %d", m.Step)
	}
}

func TestImageGenModel_DoneViewShowsSlideInfo(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# First Slide

Content

---

# Target Slide

More content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	// Select second slide
	model.SelectedIndex = 1
	model.Step = ImageGenStepDone
	model.SavedImagePath = "images/generated-abc12345.png"

	view := model.View()

	// Should show slide info
	if !strings.Contains(view, "Slide 2") {
		t.Error("view should show slide number")
	}
	if !strings.Contains(view, "Target Slide") {
		t.Error("view should show slide title")
	}
}

func TestImageGenModel_DoneViewShowsHelpText(t *testing.T) {
	tmpDir := t.TempDir()
	mdFile := filepath.Join(tmpDir, "test.md")

	content := `# Test Slide

Content
`
	if err := os.WriteFile(mdFile, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	model, err := NewImageGenModel(mdFile)
	if err != nil {
		t.Fatalf("failed to create model: %v", err)
	}

	model.Step = ImageGenStepDone
	model.SavedImagePath = "images/generated-abc12345.png"

	view := model.View()

	// Should show help text for dismissing
	if !strings.Contains(view, "enter") && !strings.Contains(view, "esc") {
		t.Error("view should show help text for dismissing")
	}
	if !strings.Contains(view, "continue") {
		t.Error("view should mention 'continue' in help text")
	}
}
