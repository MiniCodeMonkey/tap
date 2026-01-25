package tui

import (
	"fmt"
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
