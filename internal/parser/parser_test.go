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

func TestParse_SlideDirectives(t *testing.T) {
	p := New()
	content := []byte(`<!--
layout: title
transition: slide
background: "#ff0000"
notes: "Speaker notes here"
fragments: true
-->
# Welcome

This is the title slide.

---

# Second Slide

No directives here.`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(pres.Slides))
	}

	// First slide should have directives
	slide1 := pres.Slides[0]
	if slide1.Directives.Layout != "title" {
		t.Errorf("expected layout 'title', got %q", slide1.Directives.Layout)
	}
	if slide1.Directives.Transition != "slide" {
		t.Errorf("expected transition 'slide', got %q", slide1.Directives.Transition)
	}
	if slide1.Directives.Background != "#ff0000" {
		t.Errorf("expected background '#ff0000', got %q", slide1.Directives.Background)
	}
	if slide1.Directives.Notes != "Speaker notes here" {
		t.Errorf("expected notes 'Speaker notes here', got %q", slide1.Directives.Notes)
	}
	if !slide1.Directives.Fragments {
		t.Error("expected fragments to be true")
	}
	// Content should not contain the directive comment
	if contains(slide1.Content, "layout:") {
		t.Error("slide content should not contain directive comment")
	}
	if !contains(slide1.Content, "Welcome") {
		t.Error("slide content should contain 'Welcome'")
	}

	// Second slide should have empty directives
	slide2 := pres.Slides[1]
	if slide2.Directives.Layout != "" {
		t.Errorf("expected empty layout, got %q", slide2.Directives.Layout)
	}
	if slide2.Directives.Transition != "" {
		t.Errorf("expected empty transition, got %q", slide2.Directives.Transition)
	}
}

func TestParse_DirectivesPartial(t *testing.T) {
	p := New()
	content := []byte(`<!-- layout: section -->
# Section Header`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	if slide.Directives.Layout != "section" {
		t.Errorf("expected layout 'section', got %q", slide.Directives.Layout)
	}
	// Other directives should be empty/false
	if slide.Directives.Transition != "" {
		t.Errorf("expected empty transition, got %q", slide.Directives.Transition)
	}
	if slide.Directives.Fragments {
		t.Error("expected fragments to be false")
	}
}

func TestParse_DirectivesNotAtStart(t *testing.T) {
	p := New()
	// Directive comment not at the start should not be parsed as directives
	content := []byte(`# Title

<!-- layout: title -->

Some content.`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	// Directive should not be parsed because it's not at the start
	if slide.Directives.Layout != "" {
		t.Errorf("expected empty layout (directive not at start), got %q", slide.Directives.Layout)
	}
	// The comment should remain in the content
	if !contains(slide.Content, "layout:") {
		t.Error("non-directive comment should remain in content")
	}
}

func TestParse_InvalidYAMLDirective(t *testing.T) {
	p := New()
	// Invalid YAML should not crash, just pass through
	content := []byte(`<!-- not: valid: yaml: : : -->
# Title`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	// With invalid YAML, directives should be empty and comment remains
	slide := pres.Slides[0]
	if slide.Directives.Layout != "" {
		t.Errorf("expected empty layout for invalid YAML, got %q", slide.Directives.Layout)
	}
}

func TestParse_NonDirectiveComment(t *testing.T) {
	p := New()
	// A regular HTML comment (not YAML) should pass through
	content := []byte(`<!-- This is just a regular comment -->
# Title`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	// Regular comments are valid YAML (empty map), so they get parsed
	// but result in empty directives
	slide := pres.Slides[0]
	if slide.Directives.Layout != "" {
		t.Errorf("expected empty layout, got %q", slide.Directives.Layout)
	}
}

func TestParse_DirectivesMultiline(t *testing.T) {
	p := New()
	content := []byte(`<!--
layout: two-column
notes: |
  These are multiline
  speaker notes that
  span multiple lines.
-->
# Content`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	if slide.Directives.Layout != "two-column" {
		t.Errorf("expected layout 'two-column', got %q", slide.Directives.Layout)
	}
	if !contains(slide.Directives.Notes, "multiline") {
		t.Errorf("expected notes to contain 'multiline', got %q", slide.Directives.Notes)
	}
	if !contains(slide.Directives.Notes, "span multiple lines") {
		t.Errorf("expected notes to contain 'span multiple lines', got %q", slide.Directives.Notes)
	}
}

func TestParse_CodeBlocks_Simple(t *testing.T) {
	p := New()
	content := []byte("# Slide\n\n```sql\nSELECT * FROM users;\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	if len(slide.CodeBlocks) != 1 {
		t.Fatalf("expected 1 code block, got %d", len(slide.CodeBlocks))
	}

	block := slide.CodeBlocks[0]
	if block.Language != "sql" {
		t.Errorf("expected language 'sql', got %q", block.Language)
	}
	if block.Code != "SELECT * FROM users;" {
		t.Errorf("expected code 'SELECT * FROM users;', got %q", block.Code)
	}
	if block.Meta.Driver != "" {
		t.Errorf("expected empty driver, got %q", block.Meta.Driver)
	}
}

func TestParse_CodeBlocks_WithDriver(t *testing.T) {
	p := New()
	content := []byte("# SQL Demo\n\n```sql {driver: mysql}\nSELECT * FROM products;\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	if len(slide.CodeBlocks) != 1 {
		t.Fatalf("expected 1 code block, got %d", len(slide.CodeBlocks))
	}

	block := slide.CodeBlocks[0]
	if block.Language != "sql" {
		t.Errorf("expected language 'sql', got %q", block.Language)
	}
	if block.Meta.Driver != "mysql" {
		t.Errorf("expected driver 'mysql', got %q", block.Meta.Driver)
	}
}

func TestParse_CodeBlocks_WithDriverAndConnection(t *testing.T) {
	p := New()
	content := []byte("# SQL Demo\n\n```sql {driver: mysql, connection: production}\nSELECT * FROM orders;\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	if len(slide.CodeBlocks) != 1 {
		t.Fatalf("expected 1 code block, got %d", len(slide.CodeBlocks))
	}

	block := slide.CodeBlocks[0]
	if block.Language != "sql" {
		t.Errorf("expected language 'sql', got %q", block.Language)
	}
	if block.Meta.Driver != "mysql" {
		t.Errorf("expected driver 'mysql', got %q", block.Meta.Driver)
	}
	if block.Meta.Connection != "production" {
		t.Errorf("expected connection 'production', got %q", block.Meta.Connection)
	}
}

func TestParse_CodeBlocks_MultipleBlocks(t *testing.T) {
	p := New()
	content := []byte(`# Multiple Code Blocks

` + "```javascript\nconsole.log('hello');\n```" + `

Some text in between.

` + "```python {driver: python}\nprint('world')\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	if len(slide.CodeBlocks) != 2 {
		t.Fatalf("expected 2 code blocks, got %d", len(slide.CodeBlocks))
	}

	// First block - javascript without driver
	if slide.CodeBlocks[0].Language != "javascript" {
		t.Errorf("expected first block language 'javascript', got %q", slide.CodeBlocks[0].Language)
	}
	if slide.CodeBlocks[0].Meta.Driver != "" {
		t.Errorf("expected first block empty driver, got %q", slide.CodeBlocks[0].Meta.Driver)
	}

	// Second block - python with driver
	if slide.CodeBlocks[1].Language != "python" {
		t.Errorf("expected second block language 'python', got %q", slide.CodeBlocks[1].Language)
	}
	if slide.CodeBlocks[1].Meta.Driver != "python" {
		t.Errorf("expected second block driver 'python', got %q", slide.CodeBlocks[1].Meta.Driver)
	}
}

func TestParse_CodeBlocks_NoLanguage(t *testing.T) {
	p := New()
	content := []byte("# Slide\n\n```\nplain text code\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	if len(slide.CodeBlocks) != 1 {
		t.Fatalf("expected 1 code block, got %d", len(slide.CodeBlocks))
	}

	block := slide.CodeBlocks[0]
	if block.Language != "" {
		t.Errorf("expected empty language, got %q", block.Language)
	}
	if block.Code != "plain text code" {
		t.Errorf("expected code 'plain text code', got %q", block.Code)
	}
}

func TestParse_CodeBlocks_MultilineCode(t *testing.T) {
	p := New()
	content := []byte("# Slide\n\n```go\npackage main\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	if len(slide.CodeBlocks) != 1 {
		t.Fatalf("expected 1 code block, got %d", len(slide.CodeBlocks))
	}

	block := slide.CodeBlocks[0]
	if block.Language != "go" {
		t.Errorf("expected language 'go', got %q", block.Language)
	}
	if !contains(block.Code, "package main") {
		t.Error("code should contain 'package main'")
	}
	if !contains(block.Code, "func main()") {
		t.Error("code should contain 'func main()'")
	}
}

func TestParse_CodeBlocks_AcrossSlides(t *testing.T) {
	p := New()
	content := []byte("# Slide 1\n\n```sql {driver: sqlite}\nSELECT 1;\n```\n\n---\n\n# Slide 2\n\n```bash {driver: shell}\necho hello\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(pres.Slides))
	}

	// First slide
	if len(pres.Slides[0].CodeBlocks) != 1 {
		t.Fatalf("expected 1 code block in slide 1, got %d", len(pres.Slides[0].CodeBlocks))
	}
	if pres.Slides[0].CodeBlocks[0].Meta.Driver != "sqlite" {
		t.Errorf("expected driver 'sqlite' in slide 1, got %q", pres.Slides[0].CodeBlocks[0].Meta.Driver)
	}

	// Second slide
	if len(pres.Slides[1].CodeBlocks) != 1 {
		t.Fatalf("expected 1 code block in slide 2, got %d", len(pres.Slides[1].CodeBlocks))
	}
	if pres.Slides[1].CodeBlocks[0].Meta.Driver != "shell" {
		t.Errorf("expected driver 'shell' in slide 2, got %q", pres.Slides[1].CodeBlocks[0].Meta.Driver)
	}
}

func TestParseCodeBlockMeta_YAMLFormat(t *testing.T) {
	meta := parseCodeBlockMeta("driver: mysql, connection: prod")
	if meta.Driver != "mysql" {
		t.Errorf("expected driver 'mysql', got %q", meta.Driver)
	}
	if meta.Connection != "prod" {
		t.Errorf("expected connection 'prod', got %q", meta.Connection)
	}
}

func TestParseCodeBlockMeta_OnlyDriver(t *testing.T) {
	meta := parseCodeBlockMeta("driver: postgres")
	if meta.Driver != "postgres" {
		t.Errorf("expected driver 'postgres', got %q", meta.Driver)
	}
	if meta.Connection != "" {
		t.Errorf("expected empty connection, got %q", meta.Connection)
	}
}

func TestParseCodeBlockMeta_Empty(t *testing.T) {
	meta := parseCodeBlockMeta("")
	if meta.Driver != "" {
		t.Errorf("expected empty driver, got %q", meta.Driver)
	}
	if meta.Connection != "" {
		t.Errorf("expected empty connection, got %q", meta.Connection)
	}
}

func TestParseCodeBlocks_Direct(t *testing.T) {
	content := "```sql {driver: mysql}\nSELECT * FROM users;\n```"
	blocks := parseCodeBlocks(content)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}

	if blocks[0].Language != "sql" {
		t.Errorf("expected language 'sql', got %q", blocks[0].Language)
	}
	if blocks[0].Meta.Driver != "mysql" {
		t.Errorf("expected driver 'mysql', got %q", blocks[0].Meta.Driver)
	}
	if blocks[0].Code != "SELECT * FROM users;" {
		t.Errorf("expected code 'SELECT * FROM users;', got %q", blocks[0].Code)
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
