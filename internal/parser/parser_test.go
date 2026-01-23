package parser

import (
	"testing"
)

func TestNew(t *testing.T) {
	p := New()
	if p == nil {
		t.Fatal("New() returned nil")
	}
	if p.Markdown() == nil {
		t.Fatal("Markdown() returned nil")
	}
}

func TestParse_SingleSlide(t *testing.T) {
	p := New()
	content := []byte(`# Hello World

This is a single slide.`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	if slide.Index != 0 {
		t.Errorf("expected slide index 0, got %d", slide.Index)
	}
	if slide.Content == "" {
		t.Error("slide content is empty")
	}
	if slide.HTML == "" {
		t.Error("slide HTML is empty")
	}
	if !contains(slide.HTML, "<h1") {
		t.Error("HTML should contain h1 tag")
	}
}

func TestParse_MultipleSlides(t *testing.T) {
	p := New()
	content := []byte(`# Slide One

First slide content.

---

# Slide Two

Second slide content.

---

# Slide Three

Third slide content.`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(pres.Slides))
	}

	// Verify each slide
	expectations := []struct {
		contains string
		index    int
	}{
		{"Slide One", 0},
		{"Slide Two", 1},
		{"Slide Three", 2},
	}

	for i, exp := range expectations {
		slide := pres.Slides[i]
		if slide.Index != exp.index {
			t.Errorf("slide %d: expected index %d, got %d", i, exp.index, slide.Index)
		}
		if !contains(slide.Content, exp.contains) {
			t.Errorf("slide %d: expected content to contain %q", i, exp.contains)
		}
		if !contains(slide.HTML, exp.contains) {
			t.Errorf("slide %d: expected HTML to contain %q", i, exp.contains)
		}
	}
}

func TestParse_WithFrontmatter(t *testing.T) {
	p := New()
	content := []byte(`---
title: My Presentation
theme: minimal
---

# Slide One

First slide after frontmatter.

---

# Slide Two

Second slide.`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(pres.Slides))
	}

	// First slide should be "Slide One", not frontmatter
	if contains(pres.Slides[0].Content, "title:") {
		t.Error("first slide should not contain frontmatter")
	}
	if !contains(pres.Slides[0].Content, "Slide One") {
		t.Error("first slide should contain 'Slide One'")
	}
}

func TestParse_EmptyContent(t *testing.T) {
	p := New()
	content := []byte(``)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 0 {
		t.Errorf("expected 0 slides for empty content, got %d", len(pres.Slides))
	}
}

func TestParse_EmptySlidesSkipped(t *testing.T) {
	p := New()
	content := []byte(`# Slide One

---

---

# Slide Two`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	// Empty slides between delimiters should be skipped
	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides (empty skipped), got %d", len(pres.Slides))
	}

	if !contains(pres.Slides[0].Content, "Slide One") {
		t.Error("first slide should contain 'Slide One'")
	}
	if !contains(pres.Slides[1].Content, "Slide Two") {
		t.Error("second slide should contain 'Slide Two'")
	}
}

func TestParse_HTMLRendering(t *testing.T) {
	p := New()
	content := []byte(`# Heading

Some **bold** and *italic* text.

- List item 1
- List item 2`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	html := pres.Slides[0].HTML
	if !contains(html, "<h1") {
		t.Error("HTML should contain h1 tag")
	}
	if !contains(html, "<strong>bold</strong>") {
		t.Error("HTML should contain bold text")
	}
	if !contains(html, "<em>italic</em>") {
		t.Error("HTML should contain italic text")
	}
	if !contains(html, "<li>") {
		t.Error("HTML should contain list items")
	}
}

func TestParse_SlideIndexPreserved(t *testing.T) {
	p := New()
	content := []byte(`# First

---

# Second

---

# Third`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	for i, slide := range pres.Slides {
		if slide.Index != i {
			t.Errorf("slide %d has incorrect index: expected %d, got %d", i, i, slide.Index)
		}
	}
}

func TestParse_DelimiterWithWhitespace(t *testing.T) {
	p := New()
	// Delimiter with trailing spaces/tabs should still work
	content := []byte("# Slide One\n\n---   \n\n# Slide Two")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(pres.Slides))
	}
}

func TestParse_NoDelimiter(t *testing.T) {
	p := New()
	content := []byte(`# Only One Slide

All content in a single slide without any delimiter.

## Section

More content here.`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	if !contains(pres.Slides[0].Content, "Section") {
		t.Error("slide should contain all content")
	}
}

// helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && findSubstring(s, substr)))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
