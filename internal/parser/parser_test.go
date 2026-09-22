package parser

import (
	"os"
	"path/filepath"
	"strings"
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
theme: paper
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
	blocks := parseCodeBlocksFromMarkdown(content)

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

func TestParseCodeBlockMeta_HighlightLinesSingle(t *testing.T) {
	meta := parseCodeBlockMeta("3")
	if meta.HighlightLines != "3" {
		t.Errorf("expected highlight lines '3', got %q", meta.HighlightLines)
	}
	if meta.Driver != "" || meta.Connection != "" {
		t.Errorf("expected no driver/connection for a line spec, got %+v", meta)
	}
}

func TestParseCodeBlockMeta_HighlightLinesRange(t *testing.T) {
	meta := parseCodeBlockMeta("3-4")
	if meta.HighlightLines != "3-4" {
		t.Errorf("expected highlight lines '3-4', got %q", meta.HighlightLines)
	}
}

func TestParseCodeBlockMeta_HighlightLinesList(t *testing.T) {
	meta := parseCodeBlockMeta("1, 3-5")
	if meta.HighlightLines != "1,3-5" {
		t.Errorf("expected highlight lines '1,3-5' (whitespace stripped), got %q", meta.HighlightLines)
	}
}

func TestParseCodeBlocks_HighlightLines(t *testing.T) {
	content := "```php {3-4}\n<?php\necho 1;\necho 2;\necho 3;\n```"
	blocks := parseCodeBlocksFromMarkdown(content)

	if len(blocks) != 1 {
		t.Fatalf("expected 1 block, got %d", len(blocks))
	}
	if blocks[0].Language != "php" {
		t.Errorf("expected language 'php', got %q", blocks[0].Language)
	}
	if blocks[0].Meta.HighlightLines != "3-4" {
		t.Errorf("expected highlight lines '3-4', got %q", blocks[0].Meta.HighlightLines)
	}
}

func TestParse_HighlightLines_RendersDataAttribute(t *testing.T) {
	p := New()
	content := []byte("```js {3}\nconst a = 1;\nconst b = 2;\nconst c = 3;\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	html := pres.Slides[0].HTML
	if !strings.Contains(html, `class="language-js" data-highlight-lines="3"`) {
		t.Errorf("expected rendered HTML to carry data-highlight-lines, got: %s", html)
	}
}

func TestParse_HighlightLines_DoesNotAffectDriverBlocks(t *testing.T) {
	p := New()
	content := []byte("```sql {driver: sqlite, connection: demo}\nSELECT 1;\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	html := pres.Slides[0].HTML
	if strings.Contains(html, "data-highlight-lines") {
		t.Errorf("driver code block should not get a data-highlight-lines attribute, got: %s", html)
	}
	if pres.Slides[0].CodeBlocks[0].Meta.Driver != "sqlite" {
		t.Errorf("expected driver 'sqlite' to still parse, got %+v", pres.Slides[0].CodeBlocks[0].Meta)
	}
}

func TestParse_HighlightLines_InNamedSlot(t *testing.T) {
	p := New()
	content := []byte("::left\n" +
		"```go {1,3}\n" +
		"package main\n" +
		"\n" +
		"func main() {}\n" +
		"```\n")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	slot := pres.Slides[0].Slots["left"]
	if !strings.Contains(slot, `data-highlight-lines="1,3"`) {
		t.Errorf("expected named slot HTML to carry data-highlight-lines, got: %s", slot)
	}
}

// Fragment parsing tests

func TestParse_Fragments_SinglePause(t *testing.T) {
	p := New()
	content := []byte(`# Title

First content block.

<!-- pause -->

Second content block.`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	// The first content block is always visible; only the content after the pause is a fragment.
	if slide.FragmentCount != 1 {
		t.Fatalf("expected 1 fragment, got %d", slide.FragmentCount)
	}

	if !contains(slide.HTML, "First content block") {
		t.Error("HTML should contain 'First content block'")
	}
	if !contains(slide.HTML, `data-fragment-index="0"`) {
		t.Error("HTML should contain data-fragment-index=0")
	}
	if !contains(slide.HTML, "Second content block") {
		t.Error("HTML should contain 'Second content block'")
	}
}

func TestParse_Fragments_MultiplePauses(t *testing.T) {
	p := New()
	content := []byte(`# Incremental Reveal

- Item 1

<!-- pause -->

- Item 2

<!-- pause -->

- Item 3

<!-- pause -->

- Item 4`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	// Item 1 is always visible; Items 2-4 are fragments.
	if slide.FragmentCount != 3 {
		t.Fatalf("expected 3 fragments, got %d", slide.FragmentCount)
	}

	for i := 0; i < 3; i++ {
		if !contains(slide.HTML, `data-fragment-index="`+intToString(i)+`"`) {
			t.Errorf("HTML should contain data-fragment-index=%d", i)
		}
	}

	// Verify content
	expectations := []string{"Item 1", "Item 2", "Item 3", "Item 4"}
	for _, expected := range expectations {
		if !contains(slide.HTML, expected) {
			t.Errorf("HTML should contain %q", expected)
		}
	}
}

func TestParse_Fragments_NoPause(t *testing.T) {
	p := New()
	content := []byte(`# No Fragments

This slide has no pause markers.

All content appears at once.`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	// With no pause markers, there are no fragments; all content is visible.
	if slide.FragmentCount != 0 {
		t.Fatalf("expected 0 fragments (no pauses), got %d", slide.FragmentCount)
	}

	if !contains(slide.HTML, "No Fragments") {
		t.Error("HTML should contain all slide content")
	}
}

func TestParse_Fragments_PauseVariations(t *testing.T) {
	p := New()
	// Test different spacing variations of <!-- pause -->
	content := []byte(`# Pause Variations

Content 1

<!--pause-->

Content 2

<!-- pause-->

Content 3

<!--pause -->

Content 4

<!-- pause -->

Content 5`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	// Content 1 is always visible; Content 2-5 are fragments (all pause variations recognized).
	if slide.FragmentCount != 4 {
		t.Fatalf("expected 4 fragments, got %d", slide.FragmentCount)
	}

	for i := 1; i <= 5; i++ {
		expected := "Content " + string(rune('0'+i))
		if !contains(slide.HTML, expected) {
			t.Errorf("HTML should contain %q", expected)
		}
	}
}

func TestParse_Fragments_ConsecutivePauses(t *testing.T) {
	p := New()
	// Consecutive pauses should result in skipped empty fragments
	content := []byte(`# Test

Content 1

<!-- pause -->
<!-- pause -->

Content 2`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]
	// Content 1 is always visible; the empty part between the consecutive pauses is skipped,
	// leaving Content 2 as the only fragment.
	if slide.FragmentCount != 1 {
		t.Fatalf("expected 1 fragment (empty skipped), got %d", slide.FragmentCount)
	}
	if !contains(slide.HTML, `data-fragment-index="0"`) {
		t.Error("expected fragment index 0")
	}
}

func TestParse_Fragments_AcrossSlides(t *testing.T) {
	p := New()
	content := []byte(`# Slide 1

Part A

<!-- pause -->

Part B

---

# Slide 2

Part C

<!-- pause -->

Part D

<!-- pause -->

Part E`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(pres.Slides))
	}

	// Slide 1: Part A is always visible, Part B is a fragment.
	if pres.Slides[0].FragmentCount != 1 {
		t.Fatalf("slide 1: expected 1 fragment, got %d", pres.Slides[0].FragmentCount)
	}

	// Slide 2: Part C is always visible, Part D and Part E are fragments.
	if pres.Slides[1].FragmentCount != 2 {
		t.Fatalf("slide 2: expected 2 fragments, got %d", pres.Slides[1].FragmentCount)
	}

	// Each slide's fragment indices start independently at 0.
	if !contains(pres.Slides[0].HTML, `data-fragment-index="0"`) {
		t.Error("slide 1: expected fragment index 0")
	}
	if !contains(pres.Slides[1].HTML, `data-fragment-index="0"`) {
		t.Error("slide 2: expected fragment index 0")
	}
	if !contains(pres.Slides[1].HTML, `data-fragment-index="1"`) {
		t.Error("slide 2: expected fragment index 1")
	}
}

func TestParse_Fragments_WithCodeBlocks(t *testing.T) {
	p := New()
	content := []byte("# Code Demo\n\nFirst explanation.\n\n<!-- pause -->\n\n```sql {driver: mysql}\nSELECT * FROM users;\n```\n\n<!-- pause -->\n\nFinal thoughts.")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]

	// First explanation is always visible; the code block and final thoughts are fragments.
	if slide.FragmentCount != 2 {
		t.Fatalf("expected 2 fragments, got %d", slide.FragmentCount)
	}

	// Code block should still be parsed
	if len(slide.CodeBlocks) != 1 {
		t.Fatalf("expected 1 code block, got %d", len(slide.CodeBlocks))
	}

	// The rendered HTML should still contain the code block's content
	if !contains(slide.HTML, "SELECT * FROM users") {
		t.Error("HTML should contain the SQL code")
	}
}

func TestParse_Fragments_WithDirectives(t *testing.T) {
	p := New()
	content := []byte(`<!--
layout: default
fragments: true
-->
# Title

Content 1

<!-- pause -->

Content 2`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]

	// Directive should be parsed
	if slide.Directives.Layout != "default" {
		t.Errorf("expected layout 'default', got %q", slide.Directives.Layout)
	}
	if !slide.Directives.Fragments {
		t.Error("expected fragments directive to be true")
	}

	// Content 1 is always visible; Content 2 is a fragment.
	if slide.FragmentCount != 1 {
		t.Fatalf("expected 1 fragment, got %d", slide.FragmentCount)
	}

	// HTML should not contain directive content
	if contains(slide.HTML, "layout:") {
		t.Error("HTML should not contain directive content")
	}
}

func TestRenderSlot_Direct(t *testing.T) {
	p := New()
	content := "Part 1\n\n<!-- pause -->\n\nPart 2\n\n<!-- pause -->\n\nPart 3"
	html, nextIndex, _, _, _, _, err := p.renderSlot(content, 0, 0, 0, 1, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Part 1 is always visible; Part 2 and Part 3 are fragments.
	if nextIndex != 2 {
		t.Fatalf("expected next fragment index 2, got %d", nextIndex)
	}
	expectations := []string{"Part 1", "Part 2", "Part 3"}
	for _, expected := range expectations {
		if !contains(html, expected) {
			t.Errorf("html should contain %q, got %q", expected, html)
		}
	}
	if !contains(html, `data-fragment-index="0"`) || !contains(html, `data-fragment-index="1"`) {
		t.Errorf("html should contain fragment indices 0 and 1, got %q", html)
	}
}

func TestRenderSlot_EmptyContent(t *testing.T) {
	p := New()
	html, nextIndex, _, _, _, _, err := p.renderSlot("", 0, 0, 0, 1, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if html != "" {
		t.Errorf("expected empty html for empty content, got %q", html)
	}
	if nextIndex != 0 {
		t.Errorf("expected next fragment index 0, got %d", nextIndex)
	}
}

func TestRenderSlot_OnlyPauses(t *testing.T) {
	p := New()
	content := "<!-- pause -->\n<!-- pause -->\n<!-- pause -->"
	html, nextIndex, _, _, _, _, err := p.renderSlot(content, 0, 0, 0, 1, true)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// All parts are empty, so there is no content and no fragments.
	if html != "" {
		t.Errorf("expected empty html for only pause markers, got %q", html)
	}
	if nextIndex != 0 {
		t.Errorf("expected next fragment index 0, got %d", nextIndex)
	}
}

func TestAutoFragmentListItems(t *testing.T) {
	html := `<h1>Title</h1>
<ul>
<li>Item 1</li>
<li>Item 2</li>
<li>Item 3</li>
</ul>`

	result, count := autoFragmentListItems(html, 0)

	if count != 3 {
		t.Errorf("expected 3 list items, got %d", count)
	}

	// Check that fragment classes were added
	if !contains(result, `class="fragment fragment-hidden"`) {
		t.Error("expected fragment classes to be added to list items")
	}

	// Check that data-fragment-index attributes were added
	if !contains(result, `data-fragment-index="0"`) {
		t.Error("expected data-fragment-index=0")
	}
	if !contains(result, `data-fragment-index="1"`) {
		t.Error("expected data-fragment-index=1")
	}
	if !contains(result, `data-fragment-index="2"`) {
		t.Error("expected data-fragment-index=2")
	}

	// Check that heading is unchanged
	if !contains(result, "<h1>Title</h1>") {
		t.Error("heading should be unchanged")
	}
}

func TestAutoFragmentListItems_WithExistingClass(t *testing.T) {
	html := `<ul>
<li class="existing">Item 1</li>
</ul>`

	result, count := autoFragmentListItems(html, 0)

	if count != 1 {
		t.Errorf("expected 1 list item, got %d", count)
	}

	// Check that fragment class was merged with existing class
	if !contains(result, `class="fragment fragment-hidden existing"`) {
		t.Errorf("expected fragment class to be merged, got %s", result)
	}
}

func TestAutoFragmentListItems_NoListItems(t *testing.T) {
	html := `<h1>Title</h1>
<p>Just a paragraph</p>`

	result, count := autoFragmentListItems(html, 0)

	if count != 0 {
		t.Errorf("expected 0 list items, got %d", count)
	}

	// HTML should be unchanged
	if result != html {
		t.Error("HTML should be unchanged when no list items")
	}
}

func TestParse_FragmentsDirectiveAutoFragments(t *testing.T) {
	p := New()
	content := `---
title: Test
---

<!--
fragments: true
-->

# Title

- Item 1
- Item 2
- Item 3`

	pres, err := p.Parse([]byte(content))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide, got %d", len(pres.Slides))
	}

	slide := pres.Slides[0]

	// Directive should be parsed
	if !slide.Directives.Fragments {
		t.Error("expected fragments directive to be true")
	}

	// Should have 3 fragments (one for each list item)
	if slide.FragmentCount != 3 {
		t.Fatalf("expected 3 fragments for auto-fragmented list, got %d", slide.FragmentCount)
	}

	// HTML should contain fragment classes on list items
	if !contains(slide.HTML, `class="fragment fragment-hidden"`) {
		t.Error("HTML should contain fragment classes on list items")
	}
	if !contains(slide.HTML, `data-fragment-index="0"`) {
		t.Error("HTML should contain data-fragment-index attributes")
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

// Tests for code block-aware slide splitting

func TestParse_CodeBlockWithHorizontalRule(t *testing.T) {
	p := New()
	content := []byte(`# Slide One

` + "```yaml" + `
---
title: Example
---
` + "```" + `

---

# Slide Two

This is the second slide.`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(pres.Slides))
	}

	// First slide should contain the YAML code block content
	if !contains(pres.Slides[0].Content, "title: Example") {
		t.Error("first slide should contain the YAML code block content")
	}

	// Second slide should be "Slide Two"
	if !contains(pres.Slides[1].Content, "Slide Two") {
		t.Error("second slide should contain 'Slide Two'")
	}
}

func TestParse_QuadrupleBackticksWithHorizontalRule(t *testing.T) {
	p := New()
	content := []byte(`# Markdown Example

` + "````markdown" + `
---
This is inside quadruple backticks
---
` + "````" + `

---

# Next Slide`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(pres.Slides))
	}

	// First slide should contain the content inside quadruple backticks
	if !contains(pres.Slides[0].Content, "inside quadruple backticks") {
		t.Error("first slide should contain the code block content")
	}
}

func TestParse_NestedCodeBlocks(t *testing.T) {
	p := New()
	// Quadruple backticks containing triple backticks with ---
	content := []byte(`# Nested Code Blocks

` + "````markdown" + `
` + "```yaml" + `
---
title: Nested
---
` + "```" + `
` + "````" + `

---

# After Nested`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides (nested code blocks should be preserved), got %d", len(pres.Slides))
	}
}

func TestParse_MultipleCodeBlocksWithHorizontalRules(t *testing.T) {
	p := New()
	content := []byte(`# Multiple Code Blocks

` + "```yaml" + `
---
first: block
---
` + "```" + `

` + "```yaml" + `
---
second: block
---
` + "```" + `

---

# Next Slide`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides (multiple code blocks on one slide), got %d", len(pres.Slides))
	}

	// First slide should contain both code blocks
	if !contains(pres.Slides[0].Content, "first: block") {
		t.Error("first slide should contain first code block")
	}
	if !contains(pres.Slides[0].Content, "second: block") {
		t.Error("first slide should contain second code block")
	}
}

func TestParse_CodeBlockFollowedBySlideDelimiter(t *testing.T) {
	p := New()
	content := []byte(`# First

` + "```" + `
some code
` + "```" + `

---

# Second`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	if len(pres.Slides) != 2 {
		t.Fatalf("expected 2 slides (code block followed by real delimiter), got %d", len(pres.Slides))
	}
}

func TestParse_UnclosedCodeBlock(t *testing.T) {
	p := New()
	content := []byte(`# First

` + "```" + `
unclosed code block
---
this should not be a new slide
`)

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	// Unclosed code block should prevent splitting
	if len(pres.Slides) != 1 {
		t.Fatalf("expected 1 slide (unclosed code block should prevent splitting), got %d", len(pres.Slides))
	}
}

func TestSplitSlidesPreservingCodeBlocks_Direct(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{
			name:     "no code blocks",
			input:    "slide 1\n---\nslide 2",
			expected: 2,
		},
		{
			name:     "--- in code block",
			input:    "slide 1\n```\n---\n```\n---\nslide 2",
			expected: 2,
		},
		{
			name:     "--- in quadruple backtick code block",
			input:    "slide 1\n````\n---\n````\n---\nslide 2",
			expected: 2,
		},
		{
			name:     "nested code blocks",
			input:    "slide 1\n````\n```\n---\n```\n````\n---\nslide 2",
			expected: 2,
		},
		{
			name:     "multiple --- in code block",
			input:    "slide 1\n```\n---\n---\n---\n```\n---\nslide 2",
			expected: 2,
		},
		{
			name:     "unclosed code block",
			input:    "slide 1\n```\n---\nno close",
			expected: 1,
		},
		{
			name:     "code block with info string",
			input:    "slide 1\n```yaml\n---\n```\n---\nslide 2",
			expected: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitSlidesPreservingCodeBlocks(tt.input)
			// Filter empty slides (like the real parser does)
			nonEmpty := 0
			for _, s := range result {
				if len(s) > 0 && s != "" {
					trimmed := s
					for len(trimmed) > 0 && (trimmed[0] == ' ' || trimmed[0] == '\n' || trimmed[0] == '\t' || trimmed[0] == '\r') {
						trimmed = trimmed[1:]
					}
					if len(trimmed) > 0 {
						nonEmpty++
					}
				}
			}
			if nonEmpty != tt.expected {
				t.Errorf("expected %d non-empty slides, got %d (raw: %d)", tt.expected, nonEmpty, len(result))
			}
		})
	}
}

// TestParse_CodeBlockIndex_SlotOrderReversed verifies that CodeBlocks and
// their data-code-block-index attributes are assigned in source (document)
// order even when a slide writes ::right before ::left. A layout renders
// slots in whatever order it wants, so index-based pairing (not DOM
// position) is what lets the frontend find the right <pre> regardless of
// render order.
func TestParse_CodeBlockIndex_SlotOrderReversed(t *testing.T) {
	p := New()
	content := []byte("# Reordered\n\n::right\n\n```sql {driver: sqlite}\nSELECT 1;\n```\n\n::left\n\n```go\npackage main\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	slide := pres.Slides[0]
	if len(slide.CodeBlocks) != 2 {
		t.Fatalf("expected 2 code blocks, got %d", len(slide.CodeBlocks))
	}

	// Source order: sql block (in ::right) comes first, go block (in ::left) second.
	if slide.CodeBlocks[0].Language != "sql" {
		t.Errorf("expected CodeBlocks[0] to be the sql block, got language %q", slide.CodeBlocks[0].Language)
	}
	if slide.CodeBlocks[1].Language != "go" {
		t.Errorf("expected CodeBlocks[1] to be the go block, got language %q", slide.CodeBlocks[1].Language)
	}

	if !contains(slide.Slots["right"], `data-code-block-index="0"`) {
		t.Errorf("expected slot 'right' to carry data-code-block-index=\"0\", got %q", slide.Slots["right"])
	}
	if !contains(slide.Slots["left"], `data-code-block-index="1"`) {
		t.Errorf("expected slot 'left' to carry data-code-block-index=\"1\", got %q", slide.Slots["left"])
	}
}

// TestParse_CodeBlockIndex_FenceInsideListItem verifies that a fence
// indented inside a list item is still found and indexed. The old
// regex-based codeBlockPattern only matched a fence at column 0, so it
// missed this case entirely.
func TestParse_CodeBlockIndex_FenceInsideListItem(t *testing.T) {
	p := New()
	content := []byte("# Slide\n\n- Item one\n\n  ```go\n  fmt.Println(\"hi\")\n  ```\n\n- Item two")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	slide := pres.Slides[0]
	if len(slide.CodeBlocks) != 1 {
		t.Fatalf("expected 1 code block found inside the list item, got %d", len(slide.CodeBlocks))
	}
	if slide.CodeBlocks[0].Language != "go" {
		t.Errorf("expected language 'go', got %q", slide.CodeBlocks[0].Language)
	}
	if !contains(slide.HTML, `data-code-block-index="0"`) {
		t.Errorf("expected HTML to carry data-code-block-index=\"0\", got %q", slide.HTML)
	}
}

// TestParse_CodeBlockIndex_TildeFence verifies that a ~~~ fence is found
// and indexed the same way a ``` fence is.
func TestParse_CodeBlockIndex_TildeFence(t *testing.T) {
	p := New()
	content := []byte("# Slide\n\n~~~python\nprint('hi')\n~~~")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	slide := pres.Slides[0]
	if len(slide.CodeBlocks) != 1 {
		t.Fatalf("expected 1 code block, got %d", len(slide.CodeBlocks))
	}
	if slide.CodeBlocks[0].Language != "python" {
		t.Errorf("expected language 'python', got %q", slide.CodeBlocks[0].Language)
	}
	if !contains(slide.HTML, `data-code-block-index="0"`) {
		t.Errorf("expected HTML to carry data-code-block-index=\"0\", got %q", slide.HTML)
	}
}

// TestParse_CodeBlockIndex_TwoFencesOneSlot verifies that two fences in the
// same slot get sequential indices 0 and 1.
func TestParse_CodeBlockIndex_TwoFencesOneSlot(t *testing.T) {
	p := New()
	content := []byte("# Slide\n\n```js\nconsole.log(1);\n```\n\n```py\nprint(2)\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	slide := pres.Slides[0]
	if len(slide.CodeBlocks) != 2 {
		t.Fatalf("expected 2 code blocks, got %d", len(slide.CodeBlocks))
	}
	if !contains(slide.HTML, `data-code-block-index="0"`) || !contains(slide.HTML, `data-code-block-index="1"`) {
		t.Errorf("expected HTML to carry data-code-block-index 0 and 1, got %q", slide.HTML)
	}
}

// TestParse_CodeBlockIndex_FenceInsideFragment verifies that a fence after a
// <!-- pause --> marker (inside a fragment) still receives a running index
// that continues from fences before the pause, threaded the same way
// nextFragmentIndex is threaded across parts.
func TestParse_CodeBlockIndex_FenceInsideFragment(t *testing.T) {
	p := New()
	content := []byte("# Slide\n\n```go\npackage a\n```\n\n<!-- pause -->\n\n```go\npackage b\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	slide := pres.Slides[0]
	if len(slide.CodeBlocks) != 2 {
		t.Fatalf("expected 2 code blocks, got %d", len(slide.CodeBlocks))
	}
	if !contains(slide.CodeBlocks[0].Code, "package a") {
		t.Errorf("expected CodeBlocks[0] to be 'package a', got %q", slide.CodeBlocks[0].Code)
	}
	if !contains(slide.CodeBlocks[1].Code, "package b") {
		t.Errorf("expected CodeBlocks[1] to be 'package b', got %q", slide.CodeBlocks[1].Code)
	}
	if !contains(slide.HTML, `data-code-block-index="0"`) || !contains(slide.HTML, `data-code-block-index="1"`) {
		t.Errorf("expected HTML to carry data-code-block-index 0 and 1 across the fragment boundary, got %q", slide.HTML)
	}
}

// TestParse_CodeBlockIndex_MapAndDriverFence verifies that a slide with both
// a "map" fence and a driver fence gets distinct, correctly ordered indices
// for each, so the frontend can find each one by index instead of by
// counting <pre> elements (which is what let the map block delete the wrong
// one before this fix).
func TestParse_CodeBlockIndex_MapAndDriverFence(t *testing.T) {
	p := New()
	content := []byte("# Slide\n\n```map\ncenter: 40.7,-74.0\n```\n\n```sql {driver: sqlite}\nSELECT 1;\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	slide := pres.Slides[0]
	if len(slide.CodeBlocks) != 2 {
		t.Fatalf("expected 2 code blocks, got %d", len(slide.CodeBlocks))
	}
	if slide.CodeBlocks[0].Language != "map" {
		t.Errorf("expected CodeBlocks[0] to be the map block, got language %q", slide.CodeBlocks[0].Language)
	}
	if slide.CodeBlocks[1].Language != "sql" {
		t.Errorf("expected CodeBlocks[1] to be the sql block, got language %q", slide.CodeBlocks[1].Language)
	}
	if !contains(slide.HTML, `data-code-block-index="0"`) || !contains(slide.HTML, `data-code-block-index="1"`) {
		t.Errorf("expected HTML to carry data-code-block-index 0 and 1, got %q", slide.HTML)
	}
}

// TestParse_ComponentFence_Placeholder verifies that a ```component fence
// does not become a CodeBlock, gets no data-code-block-index, and renders a
// deck-component placeholder div carrying data-component-index instead.
func TestParse_ComponentFence_Placeholder(t *testing.T) {
	p := New()
	content := []byte("# Slide\n\n```component ./charts/LatencyDrop.jsx\n{ \"before\": 412, \"after\": 88 }\n```\n\n```go\npackage main\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	slide := pres.Slides[0]
	if len(slide.CodeBlocks) != 1 {
		t.Fatalf("expected 1 code block (the go fence), got %d", len(slide.CodeBlocks))
	}
	if slide.CodeBlocks[0].Language != "go" {
		t.Errorf("expected the go fence to be the only code block, got language %q", slide.CodeBlocks[0].Language)
	}
	if len(slide.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(slide.Components))
	}
	comp := slide.Components[0]
	if comp.Index != 0 {
		t.Errorf("expected component index 0, got %d", comp.Index)
	}
	if comp.Source != "./charts/LatencyDrop.jsx" {
		t.Errorf("expected component source %q, got %q", "./charts/LatencyDrop.jsx", comp.Source)
	}
	if string(comp.Props) != `{ "before": 412, "after": 88 }` {
		t.Errorf("expected component props to be the fence body, got %q", string(comp.Props))
	}
	if !contains(slide.HTML, `<div class="deck-component" data-component-index="0"></div>`) {
		t.Errorf("expected HTML to carry the component placeholder, got %q", slide.HTML)
	}
	if !contains(slide.HTML, `data-code-block-index="0"`) {
		t.Errorf("expected the go fence to carry data-code-block-index 0 (the component fence must not consume a code block index), got %q", slide.HTML)
	}
	if contains(slide.HTML, "data-code-block-index=\"1\"") {
		t.Errorf("expected the component fence to not carry a data-code-block-index, got %q", slide.HTML)
	}
}

// TestParse_ComponentFence_PathWithSpaces verifies that a component fence
// path containing spaces is captured whole instead of silently becoming an
// ordinary code block at the first space.
func TestParse_ComponentFence_PathWithSpaces(t *testing.T) {
	p := New()
	content := []byte("# Slide\n\n```component ./my slides/X.jsx\n{}\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	slide := pres.Slides[0]
	if len(slide.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(slide.Components))
	}
	if slide.Components[0].Source != "./my slides/X.jsx" {
		t.Errorf("expected component source %q, got %q", "./my slides/X.jsx", slide.Components[0].Source)
	}
}

// TestParse_ComponentFence_EmptyBodyDefaultsToEmptyObject verifies that a
// ```component fence with no body gets "{}" as its props.
func TestParse_ComponentFence_EmptyBodyDefaultsToEmptyObject(t *testing.T) {
	p := New()
	content := []byte("# Slide\n\n```component ./charts/LatencyDrop.jsx\n```")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	slide := pres.Slides[0]
	if len(slide.Components) != 1 {
		t.Fatalf("expected 1 component, got %d", len(slide.Components))
	}
	if string(slide.Components[0].Props) != "{}" {
		t.Errorf("expected empty props body to default to \"{}\", got %q", string(slide.Components[0].Props))
	}
}

// TestParse_ComponentFence_InvalidJSON verifies that invalid props JSON is a
// parse error naming the slide number and the line.
func TestParse_ComponentFence_InvalidJSON(t *testing.T) {
	p := New()
	content := []byte("# Slide\n\n```component ./charts/LatencyDrop.jsx\n{ not json\n```")

	_, err := p.Parse(content)
	if err == nil {
		t.Fatal("expected an error for invalid component props JSON, got nil")
	}
	if !contains(err.Error(), "slide 1") {
		t.Errorf("expected the error to name the slide number, got %q", err.Error())
	}
	if !contains(err.Error(), "line 3") {
		t.Errorf("expected the error to name the line, got %q", err.Error())
	}
}

// TestParse_ComponentFence_InvalidJSON_AfterPauseAndSlot verifies that a bad
// component fence's line number is the real file line even after a pause
// marker and a slot marker earlier in the slide have shifted it away from
// its position within the slide's own content.
func TestParse_ComponentFence_InvalidJSON_AfterPauseAndSlot(t *testing.T) {
	p := New()
	content := []byte(strings.Join([]string{
		"---",
		"title: Deck",
		"---",
		"",
		"# Slide",
		"",
		"<!-- pause -->",
		"",
		"::caption",
		"```component ./charts/Bad.jsx",
		"{ not json",
		"```",
	}, "\n"))

	_, err := p.Parse(content)
	if err == nil {
		t.Fatal("expected an error for invalid component props JSON, got nil")
	}
	if !contains(err.Error(), "slide 1") {
		t.Errorf("expected the error to name the slide number, got %q", err.Error())
	}
	if !contains(err.Error(), "line 10") {
		t.Errorf("expected the error to name the real file line (10), got %q", err.Error())
	}
}

// TestParse_ComponentFence_PropsMustBeObject verifies that a non-object
// top-level JSON value in the props body is a parse error.
func TestParse_ComponentFence_PropsMustBeObject(t *testing.T) {
	p := New()
	content := []byte("# Slide\n\n```component ./charts/LatencyDrop.jsx\n[1, 2, 3]\n```")

	_, err := p.Parse(content)
	if err == nil {
		t.Fatal("expected an error for non-object component props, got nil")
	}
	if !contains(err.Error(), "must be a JSON object") {
		t.Errorf("expected the error to say props must be a JSON object, got %q", err.Error())
	}
}

// TestParse_StepsDirective verifies that a "steps" directive is parsed into
// SlideDirectives.
func TestParse_StepsDirective(t *testing.T) {
	p := New()
	content := []byte("<!--\nlayout: ./slides/RollingDeploy.jsx\nsteps: 5\n-->\n\n# Zero-downtime deploys")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	slide := pres.Slides[0]
	if !slide.Directives.HasSteps {
		t.Fatal("expected HasSteps to be true")
	}
	if slide.Directives.Steps != 5 {
		t.Errorf("expected Steps 5, got %d", slide.Directives.Steps)
	}
	if slide.Directives.Layout != "./slides/RollingDeploy.jsx" {
		t.Errorf("expected layout %q, got %q", "./slides/RollingDeploy.jsx", slide.Directives.Layout)
	}
}

// TestParse_StepsDirectiveRejectsNegativeValue verifies that a negative
// "steps" value is ignored and flagged, rather than reaching the slide.
func TestParse_StepsDirectiveRejectsNegativeValue(t *testing.T) {
	p := New()
	content := []byte("<!--\nsteps: -1\n-->\n\n# Slide")

	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	slide := pres.Slides[0]
	if slide.Directives.HasSteps {
		t.Error("HasSteps = true, want false: a negative steps value must be ignored")
	}
	if !slide.Directives.StepsInvalid {
		t.Error("StepsInvalid = false, want true")
	}
}

func TestParse_SkipDirective(t *testing.T) {
	content := []byte("# One\n\n---\n\n<!--\nlayout: section\nskip: true\n-->\n\n# Two\n\n---\n\n<!-- skip: false -->\n\n# Three")

	pres, err := New().Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if len(pres.Slides) != 3 {
		t.Fatalf("got %d slides, want 3: a skipped slide stays in the parsed deck", len(pres.Slides))
	}
	for index, want := range []bool{false, true, false} {
		if got := pres.Slides[index].Directives.Skip; got != want {
			t.Errorf("slide %d: Skip = %v, want %v", index+1, got, want)
		}
	}
	if pres.Slides[1].Directives.Layout != "section" {
		t.Errorf("slide 2 layout = %q, want section: skip sits beside other directives", pres.Slides[1].Directives.Layout)
	}
	if strings.Contains(pres.Slides[1].Content, "skip") {
		t.Errorf("slide 2 content = %q, want the directive comment removed", pres.Slides[1].Content)
	}
}

func TestParse_SkipDirectiveIgnoresANonBoolean(t *testing.T) {
	pres, err := New().Parse([]byte("<!-- skip: maybe -->\n\n# Slide"))
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}
	if pres.Slides[0].Directives.Skip {
		t.Error("Skip = true for skip: maybe, want false")
	}
	if !pres.Slides[0].Directives.SkipInvalid {
		t.Error("SkipInvalid = false, want true: skip: maybe is silently ignored otherwise")
	}
}

// TestParse_SkipDirectiveFlagsYAMLTruthyStrings covers the value an author
// is most likely to type by mistake: YAML resolves "yes" (and an empty
// value) as a string, not a boolean, so a naive bool assertion leaves the
// slide presented with no error and no warning, while the author believes
// it is hidden.
func TestParse_SkipDirectiveFlagsYAMLTruthyStrings(t *testing.T) {
	for name, content := range map[string]string{
		"yes":   "<!-- skip: yes -->\n\n# Slide",
		"empty": "<!-- skip: -->\n\n# Slide",
	} {
		t.Run(name, func(t *testing.T) {
			pres, err := New().Parse([]byte(content))
			if err != nil {
				t.Fatalf("Parse() returned error: %v", err)
			}
			if pres.Slides[0].Directives.Skip {
				t.Error("Skip = true, want false: a non-boolean value must not skip the slide")
			}
			if !pres.Slides[0].Directives.SkipInvalid {
				t.Error("SkipInvalid = false, want true")
			}
		})
	}
}

func TestParse_SlideLineRangesOfTheConferenceTalk(t *testing.T) {
	content, err := os.ReadFile(filepath.Join("..", "..", "examples", "conference-talk.md"))
	if err != nil {
		t.Fatal(err)
	}
	pres, err := New().Parse(content)
	if err != nil {
		t.Fatalf("Parse() returned error: %v", err)
	}

	want := [][2]int{{10, 14}, {18, 20}, {24, 32}, {36, 46}, {50, 68}, {72, 80}, {84, 86}, {90, 94}, {98, 106}}
	if len(pres.Slides) != len(want) {
		t.Fatalf("got %d slides, want %d", len(pres.Slides), len(want))
	}
	for index, slide := range pres.Slides {
		if slide.StartLine != want[index][0] || slide.EndLine != want[index][1] {
			t.Errorf("slide %d: lines %d-%d, want %d-%d", index+1, slide.StartLine, slide.EndLine, want[index][0], want[index][1])
		}
	}
}

func TestParse_SlideLineRanges(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    [][2]int
	}{
		{"no frontmatter", "# One\n\n---\n\n\n# Two\nmore\n", [][2]int{{1, 1}, {6, 7}}},
		{"windows line endings", "---\r\ntitle: T\r\n---\r\n\r\n# One\r\n---\r\n# Two\r\n", [][2]int{{5, 5}, {7, 7}}},
		{"an empty chunk is no slide", "# One\n---\n\n---\n# Two", [][2]int{{1, 1}, {5, 5}}},
		{"a separator inside a fence", "# One\n\n```yaml\n---\n```\n\n---\n# Two", [][2]int{{1, 5}, {8, 8}}},
		{"blank lines before the frontmatter", "\n\n---\ntitle: T\n---\n# One", [][2]int{{6, 6}}},
		{"a directive comment is part of the slide", "<!--\nlayout: title\n-->\n\n# One\n", [][2]int{{1, 5}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pres, err := New().Parse([]byte(tt.content))
			if err != nil {
				t.Fatalf("Parse() returned error: %v", err)
			}
			if len(pres.Slides) != len(tt.want) {
				t.Fatalf("got %d slides, want %d", len(pres.Slides), len(tt.want))
			}
			for index, slide := range pres.Slides {
				if slide.StartLine != tt.want[index][0] || slide.EndLine != tt.want[index][1] {
					t.Errorf("slide %d: lines %d-%d, want %d-%d", index+1, slide.StartLine, slide.EndLine, tt.want[index][0], tt.want[index][1])
				}
			}
		})
	}
}

func TestParseKeepingErrorsKeepsTheBrokenSlide(t *testing.T) {
	content := []byte("# One\n\n---\n\n## Two\n\n```component ./Chart.jsx\n{not json}\n```\n\n---\n\n# Three")

	pres, slideErrors := New().ParseKeepingErrors(content)
	if len(pres.Slides) != 3 {
		t.Fatalf("got %d slides, want 3", len(pres.Slides))
	}
	if len(slideErrors) != 1 || slideErrors[1] == nil {
		t.Fatalf("slideErrors = %v, want one error for index 1", slideErrors)
	}
	if !strings.Contains(slideErrors[1].Error(), "invalid component props JSON") {
		t.Errorf("error = %v, want it to name the props JSON", slideErrors[1])
	}
	broken := pres.Slides[1]
	if broken.Index != 1 || broken.StartLine != 5 || broken.EndLine != 9 {
		t.Errorf("broken slide = index %d, lines %d-%d, want index 1, lines 5-9", broken.Index, broken.StartLine, broken.EndLine)
	}
	if !strings.Contains(pres.Slides[2].HTML, "Three") {
		t.Errorf("slide 3 HTML = %q, want the slides after a broken one parsed", pres.Slides[2].HTML)
	}

	if _, err := New().Parse(content); err == nil || !strings.HasPrefix(err.Error(), "slide 2: ") {
		t.Errorf("Parse() error = %v, want one that starts with \"slide 2: \"", err)
	}
}
