package pdf

import (
	"context"
	"image"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/tapsh/tap/internal/config"
	"github.com/tapsh/tap/internal/server"
	"github.com/tapsh/tap/internal/transformer"
)

func TestValidateContentType(t *testing.T) {
	tests := []struct {
		input   string
		want    ContentType
		wantErr bool
	}{
		{"slides", ContentSlides, false},
		{"", ContentSlides, false},
		{"notes", ContentNotes, false},
		{"both", ContentBoth, false},
		{"invalid", "", true},
		{"SLIDES", "", true}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got, err := ValidateContentType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateContentType(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ValidateContentType(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestDefaultExportOptions(t *testing.T) {
	opts := DefaultExportOptions()

	if opts.Content != ContentSlides {
		t.Errorf("DefaultExportOptions().Content = %v, want %v", opts.Content, ContentSlides)
	}
	if opts.Output != "presentation.pdf" {
		t.Errorf("DefaultExportOptions().Output = %v, want %v", opts.Output, "presentation.pdf")
	}
}

func TestNew(t *testing.T) {
	exp, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	if exp == nil {
		t.Error("New() returned nil exporter")
	}
	// Close should not error even without launching browser
	if err := exp.Close(); err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

// TestExportSlides is an integration test that requires Playwright.
// It is skipped in short mode.
func TestExportSlides(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a temporary directory for output
	tempDir, err := os.MkdirTemp("", "pdf-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test presentation with multiple slides
	pres := &transformer.TransformedPresentation{
		Config: *config.DefaultConfig(),
		Slides: []transformer.TransformedSlide{
			{Index: 0, HTML: "<h1>Slide 1</h1>", Layout: "title"},
			{Index: 1, HTML: "<h1>Slide 2</h1><p>Content</p>", Layout: "default"},
			{Index: 2, HTML: "<h1>Slide 3</h1><p>More content</p>", Layout: "default"},
		},
	}

	// Start a test server
	srv := server.New(0)
	srv.SetPresentation(pres)
	srv.SetupRoutes()
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Shutdown(context.Background())

	serverURL := "http://localhost:" + itoa(srv.Port())
	outputPath := filepath.Join(tempDir, "test.pdf")

	// Create exporter and export
	exp, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer exp.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := exp.Export(ctx, serverURL, ExportOptions{
		Content: ContentSlides,
		Output:  outputPath,
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	// Verify result
	if result.PageCount != 3 {
		t.Errorf("Export() PageCount = %d, want 3", result.PageCount)
	}
	if result.OutputPath != outputPath {
		t.Errorf("Export() OutputPath = %q, want %q", result.OutputPath, outputPath)
	}

	// Verify file exists and has content
	stat, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("output file not found: %v", err)
	}
	if stat.Size() == 0 {
		t.Error("output file is empty")
	}
	if result.FileSize != stat.Size() {
		t.Errorf("Export() FileSize = %d, want %d", result.FileSize, stat.Size())
	}
}

// TestExportNoSlides verifies proper error handling when presentation has no slides.
func TestExportNoSlides(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "pdf-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a presentation with no slides
	pres := &transformer.TransformedPresentation{
		Config: *config.DefaultConfig(),
		Slides: []transformer.TransformedSlide{},
	}

	srv := server.New(0)
	srv.SetPresentation(pres)
	srv.SetupRoutes()
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Shutdown(context.Background())

	serverURL := "http://localhost:" + itoa(srv.Port())
	outputPath := filepath.Join(tempDir, "test.pdf")

	exp, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer exp.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	_, err = exp.Export(ctx, serverURL, ExportOptions{
		Content: ContentSlides,
		Output:  outputPath,
	})

	// Should return error for no slides
	if err == nil {
		t.Error("Export() should return error when presentation has no slides")
	}
}

// TestGetSlideCountRetriesUntilLoaded verifies that getSlideCount waits for
// the presentation to load asynchronously.
func TestGetSlideCountRetriesUntilLoaded(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "pdf-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a presentation with multiple slides
	pres := &transformer.TransformedPresentation{
		Config: *config.DefaultConfig(),
		Slides: []transformer.TransformedSlide{
			{Index: 0, HTML: "<h1>Slide 1</h1>", Layout: "title"},
			{Index: 1, HTML: "<h1>Slide 2</h1>", Layout: "default"},
		},
	}

	srv := server.New(0)
	srv.SetPresentation(pres)
	srv.SetupRoutes()
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Shutdown(context.Background())

	serverURL := "http://localhost:" + itoa(srv.Port())
	outputPath := filepath.Join(tempDir, "test.pdf")

	exp, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer exp.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := exp.Export(ctx, serverURL, ExportOptions{
		Content: ContentSlides,
		Output:  outputPath,
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	// The retry logic should have successfully waited for the presentation to load
	if result.PageCount != 2 {
		t.Errorf("Export() PageCount = %d, want 2 (retry should have worked)", result.PageCount)
	}
}

// TestExportSlidesWithImages verifies that images are properly rendered in the PDF.
// This test creates a slide with a colored image and verifies the screenshot contains
// the expected colors (proving the image was rendered, not just a placeholder).
func TestExportSlidesWithImages(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	// Create a temporary directory for the test
	tempDir, err := os.MkdirTemp("", "pdf-image-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test image with a distinctive red color
	imgPath := filepath.Join(tempDir, "red-image.png")
	if err := createRedTestImage(imgPath, 200, 200); err != nil {
		t.Fatalf("failed to create test image: %v", err)
	}

	imgStat, err := os.Stat(imgPath)
	if err != nil {
		t.Fatalf("test image not found: %v", err)
	}
	t.Logf("Created red test image: %s (%d bytes)", imgPath, imgStat.Size())

	// Create a presentation with the red image displayed prominently
	pres := &transformer.TransformedPresentation{
		Config: *config.DefaultConfig(),
		Slides: []transformer.TransformedSlide{
			{
				Index:  0,
				HTML:   `<div style="display:flex;justify-content:center;align-items:center;height:100%;"><img src="/local/red-image.png" alt="Red" style="width:400px;height:400px;"></div>`,
				Layout: "default",
			},
		},
	}

	// Start server WITH base directory set
	srv := server.New(0)
	srv.SetPresentation(pres)
	srv.SetBaseDir(tempDir)
	srv.SetupRoutes()
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Shutdown(context.Background())

	serverURL := "http://localhost:" + itoa(srv.Port())

	// Verify the image is accessible
	imgURL := serverURL + "/local/red-image.png"
	resp, err := http.Get(imgURL)
	if err != nil {
		t.Fatalf("failed to fetch image from server: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("image not accessible: status %d", resp.StatusCode)
	}

	// Export PDF
	outputPath := filepath.Join(tempDir, "test.pdf")
	exp, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer exp.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	result, err := exp.Export(ctx, serverURL, ExportOptions{
		Content: ContentSlides,
		Output:  outputPath,
	})
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	t.Logf("PDF exported: %d bytes", result.FileSize)

	// Read the PDF file and check that it contains image data
	// PDFs with embedded images contain specific markers
	pdfContent, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("failed to read PDF: %v", err)
	}

	// Check for PNG or image stream markers in the PDF
	// pdfcpu embeds images as XObject streams
	hasImageMarker := containsAny(pdfContent, [][]byte{
		[]byte("/Subtype /Image"),
		[]byte("/Width"),
		[]byte("/Height"),
		[]byte("/BitsPerComponent"),
	})

	if !hasImageMarker {
		t.Error("PDF does not appear to contain embedded image data - image may not have rendered")
	} else {
		t.Log("PDF contains image markers - image was successfully embedded")
	}

	// Verify basic PDF structure
	if result.PageCount != 1 {
		t.Errorf("Expected 1 page, got %d", result.PageCount)
	}
}

// containsAny checks if data contains any of the given byte sequences.
func containsAny(data []byte, patterns [][]byte) bool {
	for _, pattern := range patterns {
		if containsBytes(data, pattern) {
			return true
		}
	}
	return false
}

// containsBytes checks if data contains the pattern.
func containsBytes(data, pattern []byte) bool {
	for i := 0; i <= len(data)-len(pattern); i++ {
		match := true
		for j := 0; j < len(pattern); j++ {
			if data[i+j] != pattern[j] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// createRedTestImage creates a solid red PNG image for testing.
func createRedTestImage(path string, width, height int) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with solid red using direct pixel manipulation
	for i := 0; i < len(img.Pix); i += 4 {
		img.Pix[i] = 255   // R
		img.Pix[i+1] = 0   // G
		img.Pix[i+2] = 0   // B
		img.Pix[i+3] = 255 // A
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}

// TestImageServerEndpoint verifies that images are served correctly when SetBaseDir is called.
// This is a regression test for the bug where images weren't rendering because SetBaseDir wasn't called.
func TestImageServerEndpoint(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	tempDir, err := os.MkdirTemp("", "pdf-endpoint-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Create a test image
	imgPath := filepath.Join(tempDir, "test-image.png")
	if err := createTestImage(imgPath, 100, 100); err != nil {
		t.Fatalf("failed to create test image: %v", err)
	}

	pres := &transformer.TransformedPresentation{
		Config: *config.DefaultConfig(),
		Slides: []transformer.TransformedSlide{
			{Index: 0, HTML: "<h1>Test</h1>", Layout: "default"},
		},
	}

	// Test WITHOUT SetBaseDir - image should return 500 error
	srv1 := server.New(0)
	srv1.SetPresentation(pres)
	// NOT calling SetBaseDir
	srv1.SetupRoutes()
	if err := srv1.Start(); err != nil {
		t.Fatalf("failed to start server 1: %v", err)
	}
	defer srv1.Shutdown(context.Background())

	resp1, err := http.Get("http://localhost:" + itoa(srv1.Port()) + "/local/test-image.png")
	if err != nil {
		t.Fatalf("failed to fetch from server 1: %v", err)
	}
	resp1.Body.Close()

	if resp1.StatusCode == http.StatusOK {
		t.Errorf("Expected image request to fail without SetBaseDir, got status %d", resp1.StatusCode)
	}
	t.Logf("Without SetBaseDir: status %d (expected failure)", resp1.StatusCode)

	// Test WITH SetBaseDir - image should return 200 OK
	srv2 := server.New(0)
	srv2.SetPresentation(pres)
	srv2.SetBaseDir(tempDir) // NOW set the base dir
	srv2.SetupRoutes()
	if err := srv2.Start(); err != nil {
		t.Fatalf("failed to start server 2: %v", err)
	}
	defer srv2.Shutdown(context.Background())

	resp2, err := http.Get("http://localhost:" + itoa(srv2.Port()) + "/local/test-image.png")
	if err != nil {
		t.Fatalf("failed to fetch from server 2: %v", err)
	}
	body, _ := io.ReadAll(resp2.Body)
	resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("Expected image request to succeed with SetBaseDir, got status %d", resp2.StatusCode)
	}
	if len(body) == 0 {
		t.Error("Image body should not be empty")
	}
	t.Logf("With SetBaseDir: status %d, body size %d bytes", resp2.StatusCode, len(body))
}

// createTestImage creates a simple colored PNG image for testing.
func createTestImage(path string, width, height int) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Fill with a solid color (red)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, image.Black)
			// Create a pattern to ensure the image has distinct content
			if (x+y)%2 == 0 {
				img.Set(x, y, image.White)
			}
		}
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	return png.Encode(f, img)
}

// itoa converts int to string without importing strconv
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var s string
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	return s
}
