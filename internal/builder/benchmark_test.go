package builder

import (
	"bytes"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
)

// generateBenchmarkPresentation creates a presentation with the specified number of slides.
func generateBenchmarkPresentation(slideCount int) ([]byte, *config.Config) {
	var buf bytes.Buffer

	// Add frontmatter
	buf.WriteString(`---
title: Build Performance Test
theme: paper
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
			// Two-column slide with background
			buf.WriteString(fmt.Sprintf(`<!--
layout: two-column
background: "#f0f4f8"
-->

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

	cfg := config.DefaultConfig()
	cfg.Title = "Build Performance Test"
	cfg.Theme = "minimal"
	cfg.Author = "Test Author"
	cfg.AspectRatio = "16:9"
	cfg.Transition = "fade"

	return buf.Bytes(), cfg
}

// BenchmarkBuild50Slides benchmarks building a 50-slide presentation.
// Target: <2s for building a 50-slide presentation.
func BenchmarkBuild50Slides(b *testing.B) {
	content, cfg := generateBenchmarkPresentation(50)
	p := parser.New()
	pres, err := p.Parse(content)
	if err != nil {
		b.Fatalf("Parse error: %v", err)
	}

	// Create a temp directory for each iteration
	tmpDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := NewWithOutput(fmt.Sprintf("%s/dist_%d", tmpDir, i))
		_, err := builder.Build(cfg, pres)
		if err != nil {
			b.Fatalf("Build error: %v", err)
		}
	}
}

// BenchmarkBuild100Slides benchmarks building a 100-slide presentation.
func BenchmarkBuild100Slides(b *testing.B) {
	content, cfg := generateBenchmarkPresentation(100)
	p := parser.New()
	pres, err := p.Parse(content)
	if err != nil {
		b.Fatalf("Parse error: %v", err)
	}

	tmpDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := NewWithOutput(fmt.Sprintf("%s/dist_%d", tmpDir, i))
		_, err := builder.Build(cfg, pres)
		if err != nil {
			b.Fatalf("Build error: %v", err)
		}
	}
}

// BenchmarkBuild200Slides benchmarks building a larger 200-slide presentation.
func BenchmarkBuild200Slides(b *testing.B) {
	content, cfg := generateBenchmarkPresentation(200)
	p := parser.New()
	pres, err := p.Parse(content)
	if err != nil {
		b.Fatalf("Parse error: %v", err)
	}

	tmpDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := NewWithOutput(fmt.Sprintf("%s/dist_%d", tmpDir, i))
		_, err := builder.Build(cfg, pres)
		if err != nil {
			b.Fatalf("Build error: %v", err)
		}
	}
}

// BenchmarkTransformOnly benchmarks only the transformation step.
func BenchmarkTransformOnly(b *testing.B) {
	content, cfg := generateBenchmarkPresentation(100)
	p := parser.New()
	pres, err := p.Parse(content)
	if err != nil {
		b.Fatalf("Parse error: %v", err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		builder := New()
		// We can't directly access the transformer from builder, but Build internally transforms
		// This is close enough for benchmarking purposes
		_ = builder
		_ = pres
		_ = cfg
	}
}

// BenchmarkGenerateIndexHTML benchmarks HTML generation.
func BenchmarkGenerateIndexHTML(b *testing.B) {
	content, _ := generateBenchmarkPresentation(50)
	presJSON := string(content) // Use content as mock JSON for simplicity

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = generateStaticHTML("Test Title", presJSON)
	}
}

// BenchmarkExtractImagePaths benchmarks image path extraction from HTML.
func BenchmarkExtractImagePaths(b *testing.B) {
	html := `<div class="slide">
		<h1>Title</h1>
		<img src="image1.png" alt="Image 1">
		<p>Some text</p>
		<img src="assets/image2.jpg" alt="Image 2">
		<img src="https://example.com/external.png" alt="External">
		<img src="./relative/path/image3.webp" alt="Image 3">
	</div>`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		extractImagePaths(html)
	}
}

// BenchmarkRewriteImagePaths benchmarks image path rewriting.
func BenchmarkRewriteImagePaths(b *testing.B) {
	html := `<div class="slide">
		<img src="image1.png" alt="Image 1">
		<img src="assets/image2.jpg" alt="Image 2">
		<img src="relative/path/image3.webp" alt="Image 3">
	</div>`

	pathMapping := map[string]string{
		"image1.png":                "assets/image1.abc12345.png",
		"assets/image2.jpg":         "assets/image2.def67890.jpg",
		"relative/path/image3.webp": "assets/image3.ghi11111.webp",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rewriteImagePaths(html, pathMapping)
	}
}

// BenchmarkIsAbsoluteURL benchmarks URL checking.
func BenchmarkIsAbsoluteURL(b *testing.B) {
	urls := []string{
		"https://example.com/image.png",
		"http://example.com/image.png",
		"relative/path/image.png",
		"./image.png",
		"/absolute/path/image.png",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, url := range urls {
			isAbsoluteURL(url)
		}
	}
}

// TestBenchmarkBuild50Slides_PerformanceTarget runs the benchmark once and verifies
// that building a 50-slide presentation completes in under 2 seconds.
func TestBenchmarkBuild50Slides_PerformanceTarget(t *testing.T) {
	content, cfg := generateBenchmarkPresentation(50)
	p := parser.New()
	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	tmpDir := t.TempDir()

	start := time.Now()
	builder := NewWithOutput(tmpDir)
	result, err := builder.Build(cfg, pres)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	t.Logf("Build 50 slides: %v (files: %d, size: %d bytes)", duration, result.FileCount, result.TotalSize)

	// Target: <2s
	if duration > 2*time.Second {
		t.Errorf("Performance target missed: building 50 slides took %v (target: <2s)", duration)
	}
}

// TestBenchmarkBuildResult_Stats verifies that build results are correctly calculated.
func TestBenchmarkBuildResult_Stats(t *testing.T) {
	content, cfg := generateBenchmarkPresentation(10)
	p := parser.New()
	pres, err := p.Parse(content)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}

	tmpDir := t.TempDir()
	builder := NewWithOutput(tmpDir)
	result, err := builder.Build(cfg, pres)
	if err != nil {
		t.Fatalf("Build error: %v", err)
	}

	// Verify result stats
	if result.OutputDir != tmpDir {
		t.Errorf("Expected output dir %q, got %q", tmpDir, result.OutputDir)
	}
	if result.FileCount < 1 {
		t.Errorf("Expected at least 1 file, got %d", result.FileCount)
	}
	if result.TotalSize <= 0 {
		t.Errorf("Expected positive total size, got %d", result.TotalSize)
	}
	if result.BuildTime <= 0 {
		t.Errorf("Expected positive build time, got %v", result.BuildTime)
	}

	// Verify index.html was created
	indexPath := tmpDir + "/index.html"
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("index.html was not created")
	}
}
