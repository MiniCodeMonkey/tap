package builder

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/components"
	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
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

func TestGenerateIndexHTML(t *testing.T) {
	tmpDir := t.TempDir()
	b := NewWithOutput(tmpDir)

	// Test with title
	pres := &transformer.TransformedPresentation{
		Config: config.Config{Title: "My Presentation"},
		Slides: []transformer.TransformedSlide{},
	}
	path := filepath.Join(tmpDir, "index.html")
	size, err := b.generateIndexHTML(path, pres)
	if err != nil {
		t.Fatalf("generateIndexHTML failed: %v", err)
	}
	if size <= 0 {
		t.Error("expected positive file size")
	}

	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	html := string(content)

	if !strings.Contains(html, "<title>My Presentation</title>") {
		t.Error("expected title to be included in HTML")
	}
	if !strings.Contains(html, `<script id="presentation-data"`) {
		t.Error("expected presentation data script tag")
	}
	if !strings.Contains(html, "<!doctype html>") && !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("expected DOCTYPE declaration")
	}

	// Test with empty title (should use default)
	pres2 := &transformer.TransformedPresentation{
		Config: config.Config{Title: ""},
		Slides: []transformer.TransformedSlide{},
	}
	path2 := filepath.Join(tmpDir, "index2.html")
	_, err = b.generateIndexHTML(path2, pres2)
	if err != nil {
		t.Fatalf("generateIndexHTML failed: %v", err)
	}
	content2, _ := os.ReadFile(path2)
	if !strings.Contains(string(content2), "<title>Tap Presentation</title>") {
		t.Error("expected default title for empty string")
	}
}

// TestGenerateIndexHTML_NeverEmbedsDriverOrConnectionSettings covers the
// same leak /api/presentation closes, for a static tap build: the
// exported index.html embeds the presentation as a script tag anyone who
// downloads the file can read, so it must never carry a driver's command,
// arguments or timeout, nor a connection's host, user, password, database,
// path or port, literal or ${NAME}-referenced alike.
func TestGenerateIndexHTML_NeverEmbedsDriverOrConnectionSettings(t *testing.T) {
	tmpDir := t.TempDir()
	b := NewWithOutput(tmpDir)

	pres := &transformer.TransformedPresentation{
		Config: config.Config{
			// Deliberately contains "port" and "user" as ordinary English
			// inside other words, so a leak check that sweeps the body for
			// those substrings would fail on the title alone.
			Title: "Import and Export",
			Drivers: map[string]config.DriverConfig{
				"postgres": {
					Command: "psql",
					Args:    []string{"--quiet"},
					Timeout: 5,
					Connections: map[string]config.ConnectionConfig{
						"prod": {
							Host:     "db.internal.example.com",
							User:     "admin",
							Password: "hunter2literal",
							Database: "billing",
							Port:     5432,
						},
					},
				},
			},
		},
		Slides: []transformer.TransformedSlide{
			{
				Index:  0,
				Layout: "default",
				CodeBlocks: []transformer.TransformedCodeBlock{
					{Language: "sql", Code: "select 1", Driver: "postgres", Connection: "prod", Block: 1},
				},
			},
		},
	}

	path := filepath.Join(tmpDir, "index.html")
	if _, err := b.generateIndexHTML(path, pres); err != nil {
		t.Fatalf("generateIndexHTML failed: %v", err)
	}
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("failed to read generated file: %v", err)
	}
	html := string(content)

	// Field names (drivers/connections/command/args/timeout/host/user/
	// password/database/path/port) are proven absent structurally below,
	// by decoding the embedded config object's keys; a substring sweep
	// for those words over the whole page would risk failing on a deck
	// title that happens to contain one of them (see the Title comment
	// above). This checks only the specific secret values, which no real
	// deck's own wording could produce.
	for _, secret := range []string{
		"hunter2literal", "db.internal.example.com", "billing", "5432", "psql", "--quiet",
	} {
		if strings.Contains(html, secret) {
			t.Errorf("generated index.html contains %q, want it absent entirely", secret)
		}
	}
	if !strings.Contains(html, `"driver":"postgres"`) || !strings.Contains(html, `"connection":"prod"`) {
		t.Error("generated index.html dropped the driver/connection names the Run button needs")
	}

	startMarker := `<script id="presentation-data" type="application/json">`
	startIdx := strings.Index(html, startMarker)
	if startIdx == -1 {
		t.Fatal("presentation data script tag not found")
	}
	startIdx += len(startMarker)
	endIdx := strings.Index(html[startIdx:], "</script>")
	if endIdx == -1 {
		t.Fatal("closing script tag not found")
	}
	var decoded struct {
		Config map[string]json.RawMessage `json:"config"`
	}
	if err := json.Unmarshal([]byte(html[startIdx:startIdx+endIdx]), &decoded); err != nil {
		t.Fatalf("embedded JSON is invalid: %v", err)
	}
	allowedKeys := map[string]bool{
		"title": true, "theme": true, "customTheme": true, "aspectRatio": true,
		"transition": true, "themeColors": true, "slideNumbers": true, "presenterLayout": true,
	}
	for key := range decoded.Config {
		if !allowedKeys[key] {
			t.Errorf("config carries unexpected key %q; a driver or connection setting may have reached the export: %v", key, decoded.Config)
		}
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

func TestBuild_WritesComponentBundlesAndRelativeURLs(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "dist")

	b := NewWithOutput(outputDir)
	b.SetComponents(map[string]components.Result{
		"./slides/RollingDeploy.jsx": {
			Bundle: &components.Bundle{
				Name:       "RollingDeploy",
				Hash:       "abc123",
				JavaScript: []byte("export default 1;"),
				CSS:        []byte("body{color:red}"),
			},
		},
	})

	cfg := config.DefaultConfig()
	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{Index: 0, Directives: parser.SlideDirectives{Layout: "./slides/RollingDeploy.jsx"}},
		},
	}

	result, err := b.Build(cfg, pres)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	jsPath := filepath.Join(outputDir, "components", "RollingDeploy-abc123.js")
	jsContent, err := os.ReadFile(jsPath)
	if err != nil {
		t.Fatalf("expected component JS file to be written: %v", err)
	}
	if string(jsContent) != "export default 1;" {
		t.Errorf("unexpected JS content: %q", jsContent)
	}

	cssPath := filepath.Join(outputDir, "components", "RollingDeploy-abc123.css")
	cssContent, err := os.ReadFile(cssPath)
	if err != nil {
		t.Fatalf("expected component CSS file to be written: %v", err)
	}
	if string(cssContent) != "body{color:red}" {
		t.Errorf("unexpected CSS content: %q", cssContent)
	}

	indexContent, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatalf("failed to read index.html: %v", err)
	}
	if !strings.Contains(string(indexContent), `"url":"components/RollingDeploy-abc123.js"`) {
		t.Errorf("expected a relative component url in the embedded JSON, got: %s", indexContent)
	}
	if strings.Contains(string(indexContent), `"url":"/components/`) {
		t.Errorf("expected the component url to be relative (no leading slash), got: %s", indexContent)
	}

	if result.FileCount < 2 {
		t.Errorf("expected FileCount to include the component files, got %d", result.FileCount)
	}
}

// TestBuild_WritesComponentAssets checks that a bundle's emitted (not
// inlined) assets - an imported image or font at or above the 100 KB
// inline threshold - land in dist/components/ next to the JS and CSS, by
// the same name the JavaScript already references.
func TestBuild_WritesComponentAssets(t *testing.T) {
	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "dist")

	b := NewWithOutput(outputDir)
	b.SetComponents(map[string]components.Result{
		"./slides/RollingDeploy.jsx": {
			Bundle: &components.Bundle{
				Name:       "RollingDeploy",
				Hash:       "abc123",
				JavaScript: []byte(`export default "components/photo-xyz.png";`),
				Assets: []components.Asset{
					{Name: "photo-xyz.png", Content: []byte("fake-png-bytes"), ContentType: "image/png"},
				},
			},
		},
	})

	cfg := config.DefaultConfig()
	pres := &parser.Presentation{
		Slides: []parser.Slide{
			{Index: 0, Directives: parser.SlideDirectives{Layout: "./slides/RollingDeploy.jsx"}},
		},
	}

	result, err := b.Build(cfg, pres)
	if err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	assetPath := filepath.Join(outputDir, "components", "photo-xyz.png")
	assetContent, err := os.ReadFile(assetPath)
	if err != nil {
		t.Fatalf("expected the emitted asset to be written: %v", err)
	}
	if string(assetContent) != "fake-png-bytes" {
		t.Errorf("unexpected asset content: %q", assetContent)
	}
	if result.FileCount < 3 {
		t.Errorf("expected FileCount to include the asset file, got %d", result.FileCount)
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

	// Should have embedded assets + index.html + test.png with hash
	// At minimum, we expect more than just 2 files since embedded frontend assets are included
	if result.FileCount < 2 {
		t.Errorf("expected at least 2 files, got %d", result.FileCount)
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

	// Should have embedded assets + index.html, no image copied
	// The embedded frontend assets are included, so count will be > 1
	if result.FileCount < 1 {
		t.Errorf("expected at least 1 file, got %d", result.FileCount)
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

	// Should have embedded assets + index.html + 3 images
	// Count the embedded assets as a baseline
	baselineResult, _ := NewWithOutput(filepath.Join(t.TempDir(), "baseline")).Build(config.DefaultConfig(), &parser.Presentation{
		Slides: []parser.Slide{{Index: 0, HTML: "<p>empty</p>"}},
	})
	expectedFiles := baselineResult.FileCount + 3 // 3 additional images
	if result.FileCount != expectedFiles {
		t.Errorf("expected %d files (baseline %d + 3 images), got %d", expectedFiles, baselineResult.FileCount, result.FileCount)
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

func TestBuild_LeavesOutSkippedSlides(t *testing.T) {
	outputDir := filepath.Join(t.TempDir(), "dist")
	pres := &parser.Presentation{Slides: []parser.Slide{
		{Index: 0, HTML: "<p>Kept first</p>"},
		{Index: 1, HTML: "<p>Left out of the build</p>", Directives: parser.SlideDirectives{Skip: true}},
		{Index: 2, HTML: "<p>Kept second</p>"},
	}}

	if _, err := NewWithOutput(outputDir).Build(config.DefaultConfig(), pres); err != nil {
		t.Fatalf("Build failed: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(outputDir, "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	html := string(content)
	if strings.Contains(html, "Left out of the build") {
		t.Error("index.html holds the skipped slide's text")
	}

	startMarker := `<script id="presentation-data" type="application/json">`
	start := strings.Index(html, startMarker)
	if start == -1 {
		t.Fatal("presentation data script tag not found")
	}
	start += len(startMarker)
	end := strings.Index(html[start:], "</script>")
	var data struct {
		Slides []transformer.TransformedSlide `json:"slides"`
	}
	if err := json.Unmarshal([]byte(html[start:start+end]), &data); err != nil {
		t.Fatalf("embedded JSON is invalid: %v", err)
	}
	if len(data.Slides) != 2 || data.Slides[1].Index != 1 || !strings.Contains(data.Slides[1].HTML, "Kept second") {
		t.Errorf("embedded slides = %+v, want two slides indexed 0 and 1", data.Slides)
	}
}
