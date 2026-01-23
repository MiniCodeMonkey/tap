package parser

import (
	"bytes"
	"fmt"
	"testing"
)

// generateLargePresentation creates a presentation with the specified number of slides.
// Each slide has realistic content including headings, text, code blocks, and fragments.
func generateLargePresentation(slideCount int) []byte {
	var buf bytes.Buffer

	// Add frontmatter
	buf.WriteString(`---
title: Performance Test Presentation
theme: minimal
author: Test Author
date: "2026-01-23"
aspectRatio: "16:9"
transition: fade
codeTheme: github-dark
fragments: true
drivers:
  shell:
    timeout: 30
  sqlite:
    connections:
      default:
        database: ":memory:"
---

`)

	// Generate slides
	for i := 0; i < slideCount; i++ {
		if i > 0 {
			buf.WriteString("\n---\n\n")
		}

		// Vary slide types
		switch i % 5 {
		case 0:
			// Title slide
			buf.WriteString(fmt.Sprintf(`<!-- layout: title -->

# Slide %d

Subtitle for slide %d

`, i+1, i+1))

		case 1:
			// Section slide
			buf.WriteString(fmt.Sprintf(`<!-- layout: section -->

## Section %d

`, i+1))

		case 2:
			// Default slide with fragments
			buf.WriteString(fmt.Sprintf(`<!--
layout: default
notes: |
  Speaker notes for slide %d.
  These are multiline notes.
-->

## Content Slide %d

Introduction paragraph.

<!-- pause -->

- Bullet point 1
- Bullet point 2
- Bullet point 3

<!-- pause -->

Final thoughts on this topic.

`, i+1, i+1))

		case 3:
			// Code-focus slide
			buf.WriteString(fmt.Sprintf("<!-- layout: code-focus -->\n\n## Code Example %d\n\n```go\npackage main\n\nimport \"fmt\"\n\nfunc example%d() {\n\tfmt.Println(\"Example %d\")\n}\n```\n\n", i+1, i+1, i+1))

		case 4:
			// Two-column slide
			buf.WriteString(fmt.Sprintf(`<!-- layout: two-column -->

## Two Column %d

|||

### Left Column

Content on the left side.

- Item 1
- Item 2

|||

### Right Column

Content on the right side.

- Item A
- Item B

`, i+1))
		}
	}

	return buf.Bytes()
}

// BenchmarkParse100Slides benchmarks parsing a 100-slide presentation.
// Target: <100ms for parsing a 100-slide presentation.
func BenchmarkParse100Slides(b *testing.B) {
	content := generateLargePresentation(100)
	p := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(content)
		if err != nil {
			b.Fatalf("Parse error: %v", err)
		}
	}
}

// BenchmarkParse50Slides benchmarks parsing a 50-slide presentation.
func BenchmarkParse50Slides(b *testing.B) {
	content := generateLargePresentation(50)
	p := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(content)
		if err != nil {
			b.Fatalf("Parse error: %v", err)
		}
	}
}

// BenchmarkParse200Slides benchmarks parsing a larger 200-slide presentation.
func BenchmarkParse200Slides(b *testing.B) {
	content := generateLargePresentation(200)
	p := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(content)
		if err != nil {
			b.Fatalf("Parse error: %v", err)
		}
	}
}

// BenchmarkParseWithManyCodeBlocks benchmarks parsing slides with many code blocks.
func BenchmarkParseWithManyCodeBlocks(b *testing.B) {
	var buf bytes.Buffer
	buf.WriteString("---\ntitle: Code Heavy\n---\n\n# Code Presentation\n\n")

	for i := 0; i < 100; i++ {
		if i > 0 {
			buf.WriteString("\n---\n\n")
		}
		buf.WriteString(fmt.Sprintf("## Slide %d\n\n```go {driver: shell}\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello from slide %d\")\n\tfor i := 0; i < 10; i++ {\n\t\tfmt.Println(i)\n\t}\n}\n```\n\n```sql {driver: sqlite, connection: default}\nSELECT * FROM users WHERE id = %d;\n```\n\n", i+1, i+1, i+1))
	}

	content := buf.Bytes()
	p := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(content)
		if err != nil {
			b.Fatalf("Parse error: %v", err)
		}
	}
}

// BenchmarkParseWithManyFragments benchmarks parsing slides with many fragments.
func BenchmarkParseWithManyFragments(b *testing.B) {
	var buf bytes.Buffer
	buf.WriteString("---\ntitle: Fragment Heavy\n---\n\n")

	for i := 0; i < 50; i++ {
		if i > 0 {
			buf.WriteString("\n---\n\n")
		}
		buf.WriteString(fmt.Sprintf("# Slide %d\n\n", i+1))
		// Add 10 fragments per slide
		for j := 0; j < 10; j++ {
			if j > 0 {
				buf.WriteString("\n<!-- pause -->\n\n")
			}
			buf.WriteString(fmt.Sprintf("- Fragment %d: This is some content that will be revealed incrementally.\n", j+1))
		}
	}

	content := buf.Bytes()
	p := New()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := p.Parse(content)
		if err != nil {
			b.Fatalf("Parse error: %v", err)
		}
	}
}

// BenchmarkParserNew benchmarks the parser creation.
func BenchmarkParserNew(b *testing.B) {
	for i := 0; i < b.N; i++ {
		_ = New()
	}
}

// BenchmarkParseDirectives benchmarks directive parsing in isolation.
func BenchmarkParseDirectives(b *testing.B) {
	content := `<!--
layout: two-column
transition: slide
background: "#ff0000"
notes: |
  Speaker notes here.
  Multiple lines of notes.
fragments: true
-->
# Title`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseDirectives(content)
	}
}

// BenchmarkParseCodeBlocks benchmarks code block parsing in isolation.
func BenchmarkParseCodeBlocks(b *testing.B) {
	content := "```go {driver: shell}\npackage main\n\nimport \"fmt\"\n\nfunc main() {\n\tfmt.Println(\"Hello\")\n}\n```\n\n```sql {driver: sqlite, connection: default}\nSELECT * FROM users WHERE id = 1;\n```\n\n```python {driver: python}\nprint('Hello, World!')\n```"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseCodeBlocks(content)
	}
}

// BenchmarkParseFragments benchmarks fragment parsing in isolation.
func BenchmarkParseFragments(b *testing.B) {
	content := "Part 1\n\n<!-- pause -->\n\nPart 2\n\n<!-- pause -->\n\nPart 3\n\n<!-- pause -->\n\nPart 4\n\n<!-- pause -->\n\nPart 5"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parseFragments(content)
	}
}

// TestBenchmarkParse100Slides_PerformanceTarget runs the benchmark once and verifies
// that parsing a 100-slide presentation completes in under 100ms.
func TestBenchmarkParse100Slides_PerformanceTarget(t *testing.T) {
	content := generateLargePresentation(100)
	p := New()

	result := testing.Benchmark(func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, _ = p.Parse(content)
		}
	})

	nsPerOp := result.NsPerOp()
	msPerOp := float64(nsPerOp) / 1e6

	t.Logf("Parse 100 slides: %.2f ms/op", msPerOp)

	// Target: <100ms
	if msPerOp > 100 {
		t.Errorf("Performance target missed: parsing 100 slides took %.2f ms (target: <100ms)", msPerOp)
	}
}
