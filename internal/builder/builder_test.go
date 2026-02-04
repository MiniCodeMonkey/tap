package builder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
)

func TestNew(t *testing.T) {
	b := New()
	if b.outputDir != "dist" {
		t.Errorf("expected default output dir 'dist', got %q", b.outputDir)
	}
}

func TestNewWithOutput(t *testing.T) {
	b := NewWithOutput("build")
	if b.outputDir != "build" {
		t.Errorf("expected output dir 'build', got %q", b.outputDir)
	}
}

func TestSetOutputDir(t *testing.T) {
	b := New()
	b.SetOutputDir("custom")
	if b.outputDir != "custom" {
		t.Errorf("expected output dir 'custom', got %q", b.outputDir)
	}
}

func TestOutputDir(t *testing.T) {
	b := NewWithOutput("mydir")
	if b.OutputDir() != "mydir" {
		t.Errorf("expected OutputDir() to return 'mydir', got %q", b.OutputDir())
	}
}

func TestSetBaseDir(t *testing.T) {
	b := New()
	b.SetBaseDir("/path/to/presentation")
	if b.baseDir != "/path/to/presentation" {
		t.Errorf("expected base dir '/path/to/presentation', got %q", b.baseDir)
	}
}

func TestExtractImagePaths(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		expected []string
	}{
		{
			name:     "no images",
			html:     "<p>Hello world</p>",
			expected: nil,
		},
		{
			name:     "single image",
			html:     `<img src="image.png" alt="test">`,
			expected: []string{"image.png"},
		},
		{
			name:     "multiple images",
			html:     `<img src="a.png"><img src="b.jpg">`,
			expected: []string{"a.png", "b.jpg"},
		},
		{
			name:     "image with path",
			html:     `<img src="images/photo.jpg" alt="photo">`,
			expected: []string{"images/photo.jpg"},
		},
		{
			name:     "absolute URL",
			html:     `<img src="https://example.com/image.png">`,
			expected: []string{"https://example.com/image.png"},
		},
		{
			name:     "mixed quotes",
			html:     `<img src='single.png'><img src="double.png">`,
			expected: []string{"single.png", "double.png"},
		},
		{
			name:     "image with attributes",
			html:     `<img class="hero" src="hero.webp" width="100">`,
			expected: []string{"hero.webp"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := extractImagePaths(tt.html)
			if len(result) != len(tt.expected) {
				t.Errorf("expected %d paths, got %d", len(tt.expected), len(result))
				return
			}
			for i, path := range result {
				if path != tt.expected[i] {
					t.Errorf("path %d: expected %q, got %q", i, tt.expected[i], path)
				}
			}
		})
	}
}

func TestRewriteImagePaths(t *testing.T) {
	tests := []struct {
		name     string
		html     string
		mapping  map[string]string
		expected string
	}{
		{
			name:     "no mapping",
			html:     `<img src="image.png">`,
			mapping:  map[string]string{},
			expected: `<img src="image.png">`,
		},
		{
			name:     "single rewrite",
			html:     `<img src="image.png">`,
			mapping:  map[string]string{"image.png": "assets/image.abc12345.png"},
			expected: `<img src="assets/image.abc12345.png">`,
		},
		{
			name:     "multiple rewrites",
			html:     `<img src="a.png"><img src="b.jpg">`,
			mapping:  map[string]string{"a.png": "assets/a.111.png", "b.jpg": "assets/b.222.jpg"},
			expected: `<img src="assets/a.111.png"><img src="assets/b.222.jpg">`,
		},
		{
			name:     "partial mapping",
			html:     `<img src="a.png"><img src="b.png">`,
			mapping:  map[string]string{"a.png": "assets/a.hash.png"},
			expected: `<img src="assets/a.hash.png"><img src="b.png">`,
		},
		{
			name:     "preserves attributes",
			html:     `<img class="photo" src="img.png" alt="test">`,
			mapping:  map[string]string{"img.png": "assets/img.hash.png"},
			expected: `<img class="photo" src="assets/img.hash.png" alt="test">`,
		},
		{
			name:     "preserves surrounding content",
			html:     `<p>Before</p><img src="x.png"><p>After</p>`,
			mapping:  map[string]string{"x.png": "assets/x.h.png"},
			expected: `<p>Before</p><img src="assets/x.h.png"><p>After</p>`,
		},
		{
			name:     "path with subdirectory",
			html:     `<img src="images/photo.jpg">`,
			mapping:  map[string]string{"images/photo.jpg": "assets/photo.abc.jpg"},
			expected: `<img src="assets/photo.abc.jpg">`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := rewriteImagePaths(tt.html, tt.mapping)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestIsAbsoluteURL(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		{"http://example.com/img.png", true},
		{"https://example.com/img.png", true},
		{"HTTP://EXAMPLE.COM/IMG.PNG", true},
		{"HTTPS://example.com/img.png", true},
		{"image.png", false},
		{"images/photo.jpg", false},
		{"/absolute/path.png", false},
		{"ftp://files.example.com/img.png", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			result := isAbsoluteURL(tt.path)
			if result != tt.expected {
				t.Errorf("isAbsoluteURL(%q) = %v, expected %v", tt.path, result, tt.expected)
			}
		})
	}
}

func TestGenerateStaticHTML(t *testing.T) {
	presJSON := `{"config":{"title":"Test"},"slides":[]}`

	// Test with title
	html := generateStaticHTML("My Presentation", presJSON)
	if !strings.Contains(html, "<title>My Presentation</title>") {
		t.Error("expected title to be included in HTML")
	}
	if !strings.Contains(html, presJSON) {
		t.Error("expected presentation JSON to be embedded")
	}
	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("expected DOCTYPE declaration")
	}

	// Test with empty title
	html = generateStaticHTML("", presJSON)
	if !strings.Contains(html, "<title>Tap Presentation</title>") {
		t.Error("expected default title for empty string")
	}
}

func TestBuild_CreatesOutputDirectory(t *testing.T) {
	// Create temp directory for test
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "dist")

	b := NewWithOutput(outputDir)
	cfg := config.DefaultConfig()
	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{
				Index:   0,
				Content: "# Hello",
				HTML:    "<h1>Hello</h1>",
			},
		},
	}

	result, err := b.Build(cfg, pres)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Check output directory was created
	if _, err := os.Stat(outputDir); os.IsNotExist(err) {
		t.Error("output directory was not created")
	}

	// Check assets directory was created
	assetsDir := filepath.Join(outputDir, "assets")
	if _, err := os.Stat(assetsDir); os.IsNotExist(err) {
		t.Error("assets directory was not created")
	}

	// Check index.html was created
	indexPath := filepath.Join(outputDir, "index.html")
	if _, err := os.Stat(indexPath); os.IsNotExist(err) {
		t.Error("index.html was not created")
	}

	// Verify result
	if result.OutputDir != outputDir {
		t.Errorf("expected OutputDir %q, got %q", outputDir, result.OutputDir)
	}
	if result.FileCount < 1 {
		t.Error("expected at least 1 file (index.html)")
	}
	if result.TotalSize <= 0 {
		t.Error("expected positive TotalSize")
	}
	if result.BuildTime <= 0 {
		t.Error("expected positive BuildTime")
	}
}

func TestBuild_GeneratesValidHTML(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "dist")

	b := NewWithOutput(outputDir)
	cfg := config.DefaultConfig()
	cfg.Title = "Test Presentation"
	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{Index: 0, HTML: "<h1>Slide 1</h1>"},
			{Index: 1, HTML: "<h2>Slide 2</h2>"},
		},
	}

	_, err := b.Build(cfg, pres)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Read generated HTML
	indexPath := filepath.Join(outputDir, "index.html")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index.html: %v", err)
	}

	html := string(content)

	// Check essential elements
	if !strings.Contains(html, "<title>Test Presentation</title>") {
		t.Error("missing or incorrect title")
	}
	if !strings.Contains(html, `<script id="presentation-data"`) {
		t.Error("missing presentation data script")
	}
	if !strings.Contains(html, `"slides":[`) {
		t.Error("missing slides in embedded JSON")
	}
}

func TestBuild_EmbedsPresentationJSON(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "dist")

	b := NewWithOutput(outputDir)
	cfg := config.DefaultConfig()
	cfg.Title = "JSON Test"
	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{Index: 0, HTML: "<p>Content</p>"},
		},
	}

	_, err := b.Build(cfg, pres)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Read generated HTML
	indexPath := filepath.Join(outputDir, "index.html")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatalf("failed to read index.html: %v", err)
	}

	// Extract JSON from script tag
	html := string(content)
	startMarker := `<script id="presentation-data" type="application/json">`
	endMarker := `</script>`

	startIdx := strings.Index(html, startMarker)
	if startIdx == -1 {
		t.Fatal("presentation data script tag not found")
	}
	startIdx += len(startMarker)

	endIdx := strings.Index(html[startIdx:], endMarker)
	if endIdx == -1 {
		t.Fatal("closing script tag not found")
	}

	jsonStr := html[startIdx : startIdx+endIdx]

	// Verify JSON is valid
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &data); err != nil {
		t.Fatalf("embedded JSON is invalid: %v", err)
	}

	// Check structure
	if _, ok := data["config"]; !ok {
		t.Error("missing 'config' in embedded JSON")
	}
	if _, ok := data["slides"]; !ok {
		t.Error("missing 'slides' in embedded JSON")
	}
}

func TestBuild_CopiesImagesWithHash(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "dist")
	baseDir := filepath.Join(tmpDir, "presentation")

	// Create base directory and test image
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatal(err)
	}
	imgPath := filepath.Join(baseDir, "test.png")
	imgContent := []byte("fake png content for testing")
	if err := os.WriteFile(imgPath, imgContent, 0644); err != nil {
		t.Fatal(err)
	}

	b := NewWithOutput(outputDir)
	b.SetBaseDir(baseDir)
	cfg := config.DefaultConfig()
	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{Index: 0, HTML: `<img src="test.png">`},
		},
	}

	result, err := b.Build(cfg, pres)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should have 2 files: index.html + test.png with hash
	if result.FileCount != 2 {
		t.Errorf("expected 2 files, got %d", result.FileCount)
	}

	// Check that image was copied to assets
	assetsDir := filepath.Join(outputDir, "assets")
	entries, err := os.ReadDir(assetsDir)
	if err != nil {
		t.Fatal(err)
	}

	var foundImage bool
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "test.") && strings.HasSuffix(entry.Name(), ".png") {
			foundImage = true
			// Verify hash is in filename
			if entry.Name() == "test.png" {
				t.Error("image should have hash in filename")
			}
		}
	}
	if !foundImage {
		t.Error("test image was not copied to assets")
	}
}

func TestBuild_RewritesImagePathsInHTML(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "dist")
	baseDir := filepath.Join(tmpDir, "presentation")

	// Create base directory and test image
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatal(err)
	}
	imgPath := filepath.Join(baseDir, "photo.jpg")
	if err := os.WriteFile(imgPath, []byte("jpg content"), 0644); err != nil {
		t.Fatal(err)
	}

	b := NewWithOutput(outputDir)
	b.SetBaseDir(baseDir)
	cfg := config.DefaultConfig()
	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{Index: 0, HTML: `<p>Before</p><img src="photo.jpg"><p>After</p>`},
		},
	}

	_, err := b.Build(cfg, pres)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Read generated HTML
	indexPath := filepath.Join(outputDir, "index.html")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}

	html := string(content)

	// Should contain rewritten path with "assets/" prefix
	if !strings.Contains(html, `"assets/photo.`) {
		t.Error("image path should be rewritten to assets/")
	}
	// Should NOT contain original path
	if strings.Contains(html, `"photo.jpg"`) {
		t.Error("original image path should be replaced")
	}
}

func TestBuild_SkipsAbsoluteURLs(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "dist")

	b := NewWithOutput(outputDir)
	cfg := config.DefaultConfig()
	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{Index: 0, HTML: `<img src="https://example.com/image.png">`},
		},
	}

	result, err := b.Build(cfg, pres)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should only have index.html, no image copied
	if result.FileCount != 1 {
		t.Errorf("expected 1 file (index.html only), got %d", result.FileCount)
	}

	// Read generated HTML
	indexPath := filepath.Join(outputDir, "index.html")
	content, err := os.ReadFile(indexPath)
	if err != nil {
		t.Fatal(err)
	}

	html := string(content)
	// URL should be preserved unchanged
	if !strings.Contains(html, `https://example.com/image.png`) {
		t.Error("absolute URL should be preserved unchanged")
	}
}

func TestCopyWithHash(t *testing.T) {
	tmpDir := t.TempDir()

	// Create source file
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	srcPath := filepath.Join(srcDir, "image.png")
	content := []byte("test image content")
	if err := os.WriteFile(srcPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	// Create destination directory
	destDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	b := New()
	relPath, size, err := b.copyWithHash(srcPath, destDir)
	if err != nil {
		t.Fatalf("copyWithHash failed: %v", err)
	}

	// Check returned values
	if !strings.HasPrefix(relPath, "assets/image.") {
		t.Errorf("expected path to start with 'assets/image.', got %q", relPath)
	}
	if !strings.HasSuffix(relPath, ".png") {
		t.Errorf("expected path to end with '.png', got %q", relPath)
	}
	if size != int64(len(content)) {
		t.Errorf("expected size %d, got %d", len(content), size)
	}

	// Check file was created
	destPath := filepath.Join(tmpDir, relPath)
	if _, err := os.Stat(destPath); os.IsNotExist(err) {
		t.Error("destination file was not created")
	}

	// Verify content matches
	destContent, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(destContent) != string(content) {
		t.Error("copied file content does not match source")
	}
}

func TestCopyWithHash_SameContentSameHash(t *testing.T) {
	tmpDir := t.TempDir()

	// Create two source files with same content
	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	content := []byte("identical content")
	src1 := filepath.Join(srcDir, "file1.png")
	src2 := filepath.Join(srcDir, "file2.png")
	if err := os.WriteFile(src1, content, 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src2, content, 0644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	b := New()
	path1, _, _ := b.copyWithHash(src1, destDir)
	path2, _, _ := b.copyWithHash(src2, destDir)

	// Extract hashes from paths
	// Format: assets/filename.HASH.ext
	parts1 := strings.Split(filepath.Base(path1), ".")
	parts2 := strings.Split(filepath.Base(path2), ".")

	if len(parts1) < 3 || len(parts2) < 3 {
		t.Fatal("unexpected path format")
	}

	hash1 := parts1[1]
	hash2 := parts2[1]

	if hash1 != hash2 {
		t.Errorf("same content should produce same hash, got %q and %q", hash1, hash2)
	}
}

func TestCopyWithHash_DifferentContentDifferentHash(t *testing.T) {
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "src")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	src1 := filepath.Join(srcDir, "file1.png")
	src2 := filepath.Join(srcDir, "file2.png")
	if err := os.WriteFile(src1, []byte("content A"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(src2, []byte("content B"), 0644); err != nil {
		t.Fatal(err)
	}

	destDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	b := New()
	path1, _, _ := b.copyWithHash(src1, destDir)
	path2, _, _ := b.copyWithHash(src2, destDir)

	parts1 := strings.Split(filepath.Base(path1), ".")
	parts2 := strings.Split(filepath.Base(path2), ".")

	if len(parts1) < 3 || len(parts2) < 3 {
		t.Fatal("unexpected path format")
	}

	hash1 := parts1[1]
	hash2 := parts2[1]

	if hash1 == hash2 {
		t.Error("different content should produce different hashes")
	}
}

func TestCopyWithHash_NonExistentFile(t *testing.T) {
	tmpDir := t.TempDir()
	destDir := filepath.Join(tmpDir, "assets")
	if err := os.MkdirAll(destDir, 0755); err != nil {
		t.Fatal(err)
	}

	b := New()
	_, _, err := b.copyWithHash("/nonexistent/file.png", destDir)
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestBuildResult_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "dist")
	baseDir := filepath.Join(tmpDir, "presentation")

	// Create multiple images
	if err := os.MkdirAll(baseDir, 0755); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 3; i++ {
		imgPath := filepath.Join(baseDir, "img"+string(rune('a'+i))+".png")
		if err := os.WriteFile(imgPath, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	b := NewWithOutput(outputDir)
	b.SetBaseDir(baseDir)
	cfg := config.DefaultConfig()
	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{Index: 0, HTML: `<img src="imga.png"><img src="imgb.png">`},
			{Index: 1, HTML: `<img src="imgc.png">`},
		},
	}

	result, err := b.Build(cfg, pres)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	// Should have 4 files: index.html + 3 images
	if result.FileCount != 4 {
		t.Errorf("expected 4 files, got %d", result.FileCount)
	}

	// Total size should include all files
	if result.TotalSize <= 0 {
		t.Error("expected positive total size")
	}

	// Build time should be recorded
	if result.BuildTime <= 0 {
		t.Error("expected positive build time")
	}
}
