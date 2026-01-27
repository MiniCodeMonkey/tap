// Package pdf provides PDF export functionality for tap presentations.
package pdf

import (
	"context"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	"github.com/playwright-community/playwright-go"
)

// ContentType specifies what content to include in the exported PDF.
type ContentType string

const (
	// ContentSlides exports only the presentation slides.
	ContentSlides ContentType = "slides"
	// ContentNotes exports only the speaker notes.
	ContentNotes ContentType = "notes"
	// ContentBoth exports both slides and notes.
	ContentBoth ContentType = "both"
)

// ExportOptions configures the PDF export process.
type ExportOptions struct {
	// Content specifies what to include: "slides", "notes", or "both".
	// Default is "slides".
	Content ContentType
	// Output is the path for the generated PDF file.
	// If empty, defaults to "presentation.pdf" in the current directory.
	Output string
	// Title is the PDF document title metadata.
	Title string
	// Author is the PDF document author metadata.
	Author string
}

// DefaultExportOptions returns the default export options.
func DefaultExportOptions() ExportOptions {
	return ExportOptions{
		Content: ContentSlides,
		Output:  "presentation.pdf",
	}
}

// ExportResult contains information about the completed export.
type ExportResult struct {
	// OutputPath is the path to the generated PDF file.
	OutputPath string
	// PageCount is the number of pages in the PDF.
	PageCount int
	// Duration is how long the export took.
	Duration time.Duration
	// FileSize is the size of the generated PDF in bytes.
	FileSize int64
}

// Exporter handles PDF generation from tap presentations.
type Exporter struct {
	pw      *playwright.Playwright
	browser playwright.Browser
}

// New creates a new Exporter.
// Call Close() when done to clean up browser resources.
func New() (*Exporter, error) {
	return &Exporter{}, nil
}

// launchBrowser lazily launches the browser when needed.
func (e *Exporter) launchBrowser() error {
	if e.browser != nil {
		return nil
	}

	// Install browsers if not already installed
	err := playwright.Install(&playwright.RunOptions{
		Browsers: []string{"chromium"},
	})
	if err != nil {
		return fmt.Errorf("failed to install playwright browsers: %w", err)
	}

	pw, err := playwright.Run()
	if err != nil {
		return fmt.Errorf("failed to start playwright: %w", err)
	}
	e.pw = pw

	browser, err := pw.Chromium.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		_ = e.pw.Stop()
		return fmt.Errorf("failed to launch chromium: %w", err)
	}
	e.browser = browser

	return nil
}

// Close cleans up browser resources.
func (e *Exporter) Close() error {
	var errs []error

	if e.browser != nil {
		if err := e.browser.Close(); err != nil {
			errs = append(errs, fmt.Errorf("failed to close browser: %w", err))
		}
		e.browser = nil
	}

	if e.pw != nil {
		if err := e.pw.Stop(); err != nil {
			errs = append(errs, fmt.Errorf("failed to stop playwright: %w", err))
		}
		e.pw = nil
	}

	if len(errs) > 0 {
		return errs[0]
	}
	return nil
}

// Export generates a PDF from a running presentation server.
// serverURL should be the base URL of the tap dev server (e.g., "http://localhost:3000").
func (e *Exporter) Export(ctx context.Context, serverURL string, opts ExportOptions) (*ExportResult, error) {
	startTime := time.Now()

	// Apply defaults
	if opts.Content == "" {
		opts.Content = ContentSlides
	}
	if opts.Output == "" {
		opts.Output = "presentation.pdf"
	}

	// Ensure output directory exists
	outputDir := filepath.Dir(opts.Output)
	if outputDir != "" && outputDir != "." {
		if err := os.MkdirAll(outputDir, 0755); err != nil {
			return nil, fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	// Launch browser
	if err := e.launchBrowser(); err != nil {
		return nil, err
	}

	// Create a new page
	page, err := e.browser.NewPage(playwright.BrowserNewPageOptions{
		Viewport: &playwright.Size{
			Width:  1920,
			Height: 1080,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create new page: %w", err)
	}
	defer page.Close()

	// Navigate to the presentation
	targetURL := serverURL
	if opts.Content == ContentNotes {
		targetURL = serverURL + "/presenter"
	}

	if _, err := page.Goto(targetURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return nil, fmt.Errorf("failed to navigate to presentation: %w", err)
	}

	// Wait for the presentation to load
	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		return nil, fmt.Errorf("failed to wait for page load: %w", err)
	}

	// Get total slide count
	slideCount, err := e.getSlideCount(page)
	if err != nil {
		return nil, fmt.Errorf("failed to get slide count: %w", err)
	}

	if slideCount == 0 {
		return nil, fmt.Errorf("no slides found in presentation")
	}

	// Remove existing output file to ensure clean overwrite
	// (pdfcpu may not properly overwrite existing files)
	if err := os.Remove(opts.Output); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to remove existing output file: %w", err)
	}

	// Export based on content type
	var result *ExportResult
	switch opts.Content {
	case ContentSlides:
		result, err = e.exportSlides(ctx, page, serverURL, slideCount, opts.Output, opts)
	case ContentNotes:
		result, err = e.exportNotes(ctx, page, serverURL, slideCount, opts.Output)
	case ContentBoth:
		result, err = e.exportBoth(ctx, page, serverURL, slideCount, opts.Output)
	default:
		return nil, fmt.Errorf("invalid content type: %s", opts.Content)
	}

	if err != nil {
		return nil, err
	}

	result.Duration = time.Since(startTime)

	// Get file size
	if stat, err := os.Stat(opts.Output); err == nil {
		result.FileSize = stat.Size()
	}

	return result, nil
}

// getSlideCount determines the number of slides in the presentation.
// It retries for up to 10 seconds to allow the frontend to load the presentation data.
func (e *Exporter) getSlideCount(page playwright.Page) (int, error) {
	// Retry for up to 10 seconds (20 attempts * 500ms)
	const maxAttempts = 20
	const retryDelay = 500 * time.Millisecond

	for attempt := 0; attempt < maxAttempts; attempt++ {
		count, err := e.tryGetSlideCount(page)
		if err != nil {
			return 0, err
		}
		if count > 0 {
			return count, nil
		}

		// Wait before retrying
		if attempt < maxAttempts-1 {
			time.Sleep(retryDelay)
		}
	}

	return 0, nil
}

// tryGetSlideCount attempts to get the slide count once.
func (e *Exporter) tryGetSlideCount(page playwright.Page) (int, error) {
	// Try to get slide count from the presentation data
	count, err := page.Evaluate(`() => {
		// Try to get from embedded data
		const dataEl = document.getElementById('presentation-data');
		if (dataEl) {
			try {
				const data = JSON.parse(dataEl.textContent);
				if (data && data.slides) {
					return data.slides.length;
				}
			} catch (e) {}
		}
		// Try window.presentation
		if (window.presentation && window.presentation.slides) {
			return window.presentation.slides.length;
		}
		// Try data attribute
		const container = document.querySelector('[data-total-slides]');
		if (container) {
			return parseInt(container.dataset.totalSlides, 10);
		}
		// Fallback: look for slide counter text
		const counter = document.getElementById('slide-counter');
		if (counter) {
			const match = counter.textContent.match(/\d+\s*\/\s*(\d+)/);
			if (match) {
				return parseInt(match[1], 10);
			}
		}
		return 0;
	}`)
	if err != nil {
		return 0, fmt.Errorf("failed to evaluate slide count: %w", err)
	}

	if count == nil {
		return 0, nil
	}

	// Handle the returned value
	switch v := count.(type) {
	case float64:
		return int(v), nil
	case int:
		return v, nil
	case int64:
		return int(v), nil
	default:
		return 0, nil
	}
}

// exportSlides exports only the presentation slides to PDF.
// It captures each slide as a screenshot and combines them into a single PDF.
func (e *Exporter) exportSlides(ctx context.Context, page playwright.Page, serverURL string, slideCount int, output string, opts ExportOptions) (*ExportResult, error) {
	// Create a temporary directory for screenshots
	tempDir, err := os.MkdirTemp("", "tap-pdf-export-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Capture each slide as a screenshot
	var screenshotPaths []string
	for i := 0; i < slideCount; i++ {
		// Check for context cancellation
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Navigate to the slide (1-based hash for URL)
		// Use ?print=true to show all fragments
		slideURL := fmt.Sprintf("%s?print=true#%d", serverURL, i+1)
		if _, err := page.Goto(slideURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return nil, fmt.Errorf("failed to navigate to slide %d: %w", i+1, err)
		}

		// Wait for slide to render
		if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		}); err != nil {
			return nil, fmt.Errorf("failed to wait for slide %d to load: %w", i+1, err)
		}

		// Wait for all images to be fully loaded
		if err := e.waitForImages(page); err != nil {
			return nil, fmt.Errorf("failed to wait for images on slide %d: %w", i+1, err)
		}

		// Small delay to ensure animations complete
		time.Sleep(200 * time.Millisecond)

		// Take a screenshot
		screenshotPath := filepath.Join(tempDir, fmt.Sprintf("slide-%03d.png", i))
		if _, err := page.Screenshot(playwright.PageScreenshotOptions{
			Path:     playwright.String(screenshotPath),
			FullPage: playwright.Bool(false),
			Type:     playwright.ScreenshotTypePng,
		}); err != nil {
			return nil, fmt.Errorf("failed to capture slide %d: %w", i+1, err)
		}
		screenshotPaths = append(screenshotPaths, screenshotPath)
	}

	// Combine screenshots into a PDF
	if err := e.imagesToPDF(screenshotPaths, output); err != nil {
		return nil, fmt.Errorf("failed to create PDF from screenshots: %w", err)
	}

	// Add PDF metadata if provided
	if err := e.addMetadata(output, opts); err != nil {
		return nil, fmt.Errorf("failed to add PDF metadata: %w", err)
	}

	return &ExportResult{
		OutputPath: output,
		PageCount:  slideCount,
	}, nil
}

// imagesToPDF combines multiple PNG images into a single PDF file.
func (e *Exporter) imagesToPDF(imagePaths []string, outputPath string) error {
	if len(imagePaths) == 0 {
		return fmt.Errorf("no images to convert")
	}

	// Sort paths to ensure correct order
	sort.Strings(imagePaths)

	// Get dimensions from the first image to use as page size
	firstImg, err := loadPNG(imagePaths[0])
	if err != nil {
		return fmt.Errorf("failed to load first image: %w", err)
	}
	bounds := firstImg.Bounds()
	pageWidth := float64(bounds.Dx())
	pageHeight := float64(bounds.Dy())

	// Configure pdfcpu to use custom page dimensions matching the screenshots
	// Convert pixels to points (assuming 72 DPI for simplicity, but we'll use actual dimensions)
	conf := model.NewDefaultConfiguration()

	// Import images to PDF with custom page size
	// pdfcpu's ImportImages creates pages sized to fit each image
	imp, err := api.Import("form:A4L, pos:c, sc:1.0", types.POINTS) // Landscape A4, centered, scale to fit
	if err != nil {
		return fmt.Errorf("failed to create import config: %w", err)
	}

	// Override with custom dimensions based on screenshot size
	// Use the screenshot dimensions directly (in points, 1:1 ratio for screen capture)
	imp.PageDim = &types.Dim{Width: pageWidth, Height: pageHeight}
	imp.Pos = types.Center
	imp.ScaleAbs = true
	imp.Scale = 1.0

	// Import all images into a new PDF
	if err := api.ImportImagesFile(imagePaths, outputPath, imp, conf); err != nil {
		return fmt.Errorf("failed to import images to PDF: %w", err)
	}

	return nil
}

// loadPNG loads a PNG image and returns it.
func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	return png.Decode(f)
}

// waitForImages waits for all images on the page to be fully loaded.
func (e *Exporter) waitForImages(page playwright.Page) error {
	// Wait for all images to complete loading with a timeout
	_, err := page.Evaluate(`() => {
		return new Promise((resolve, reject) => {
			const timeout = setTimeout(() => {
				resolve(); // Don't fail on timeout, just continue
			}, 5000);

			const images = Array.from(document.querySelectorAll('img'));
			if (images.length === 0) {
				clearTimeout(timeout);
				resolve();
				return;
			}

			let loaded = 0;
			const total = images.length;

			const checkComplete = () => {
				loaded++;
				if (loaded >= total) {
					clearTimeout(timeout);
					resolve();
				}
			};

			images.forEach(img => {
				if (img.complete && img.naturalHeight !== 0) {
					checkComplete();
				} else {
					img.addEventListener('load', checkComplete);
					img.addEventListener('error', checkComplete); // Count errors as "done"
				}
			});
		});
	}`)
	return err
}

// exportNotes exports only the speaker notes to PDF.
// It creates an HTML page with all notes and converts it to PDF.
func (e *Exporter) exportNotes(ctx context.Context, page playwright.Page, serverURL string, slideCount int, output string) (*ExportResult, error) {
	// First, get all the notes by navigating to each slide
	var allNotes []string
	for i := 0; i < slideCount; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Navigate to presenter view for this slide
		slideURL := fmt.Sprintf("%s/presenter#%d", serverURL, i+1)
		if _, err := page.Goto(slideURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return nil, fmt.Errorf("failed to navigate to slide %d: %w", i+1, err)
		}

		if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		}); err != nil {
			return nil, fmt.Errorf("failed to wait for slide %d: %w", i+1, err)
		}

		// Small delay for content to render
		time.Sleep(100 * time.Millisecond)

		// Extract notes text
		notes, err := page.Evaluate(`() => {
			const notesEl = document.querySelector('.speaker-notes, .notes, [class*="notes"]');
			return notesEl ? notesEl.innerText : '';
		}`)
		if err != nil {
			return nil, fmt.Errorf("failed to extract notes for slide %d: %w", i+1, err)
		}
		noteText := ""
		if notes != nil {
			noteText = notes.(string)
		}
		allNotes = append(allNotes, noteText)
	}

	// Create an HTML page with all the notes
	html := `<!DOCTYPE html>
<html>
<head>
<style>
body { font-family: Georgia, serif; font-size: 12pt; line-height: 1.6; max-width: 800px; margin: 0 auto; padding: 2cm; }
.slide-notes { margin-bottom: 2em; padding-bottom: 1em; border-bottom: 1px solid #ccc; page-break-inside: avoid; }
.slide-number { font-weight: bold; color: #666; margin-bottom: 0.5em; }
.no-notes { color: #999; font-style: italic; }
</style>
</head>
<body>
<h1>Speaker Notes</h1>
`
	for i, note := range allNotes {
		html += fmt.Sprintf(`<div class="slide-notes">
<div class="slide-number">Slide %d</div>
`, i+1)
		if note == "" {
			html += `<p class="no-notes">No notes for this slide</p>`
		} else {
			html += fmt.Sprintf(`<p>%s</p>`, note)
		}
		html += "</div>\n"
	}
	html += "</body></html>"

	// Set the page content to our notes HTML
	if err := page.SetContent(html, playwright.PageSetContentOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return nil, fmt.Errorf("failed to set notes content: %w", err)
	}

	// Generate PDF
	_, err := page.PDF(playwright.PagePdfOptions{
		Path:            playwright.String(output),
		Format:          playwright.String("A4"),
		Landscape:       playwright.Bool(false),
		PrintBackground: playwright.Bool(true),
		Margin: &playwright.Margin{
			Top:    playwright.String("1cm"),
			Right:  playwright.String("1cm"),
			Bottom: playwright.String("1cm"),
			Left:   playwright.String("1cm"),
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to generate notes PDF: %w", err)
	}

	return &ExportResult{
		OutputPath: output,
		PageCount:  slideCount, // Approximate, actual pages depend on content
	}, nil
}

// exportBoth exports both slides and notes to PDF.
// It captures screenshots of the presenter view (showing slide + notes) for each slide.
func (e *Exporter) exportBoth(ctx context.Context, page playwright.Page, serverURL string, slideCount int, output string) (*ExportResult, error) {
	// Create a temporary directory for screenshots
	tempDir, err := os.MkdirTemp("", "tap-pdf-both-*")
	if err != nil {
		return nil, fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Capture each slide's presenter view as a screenshot
	var screenshotPaths []string
	for i := 0; i < slideCount; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		// Navigate to presenter view for this slide
		// Use ?print=true to show all fragments
		slideURL := fmt.Sprintf("%s/presenter?print=true#%d", serverURL, i+1)
		if _, err := page.Goto(slideURL, playwright.PageGotoOptions{
			WaitUntil: playwright.WaitUntilStateDomcontentloaded,
		}); err != nil {
			return nil, fmt.Errorf("failed to navigate to slide %d: %w", i+1, err)
		}

		if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
			State: playwright.LoadStateNetworkidle,
		}); err != nil {
			return nil, fmt.Errorf("failed to wait for slide %d: %w", i+1, err)
		}

		// Wait for all images to be fully loaded
		if err := e.waitForImages(page); err != nil {
			return nil, fmt.Errorf("failed to wait for images on slide %d: %w", i+1, err)
		}

		// Small delay for content to render
		time.Sleep(200 * time.Millisecond)

		// Take a screenshot of the presenter view
		screenshotPath := filepath.Join(tempDir, fmt.Sprintf("slide-%03d.png", i))
		if _, err := page.Screenshot(playwright.PageScreenshotOptions{
			Path:     playwright.String(screenshotPath),
			FullPage: playwright.Bool(false),
			Type:     playwright.ScreenshotTypePng,
		}); err != nil {
			return nil, fmt.Errorf("failed to capture slide %d: %w", i+1, err)
		}
		screenshotPaths = append(screenshotPaths, screenshotPath)
	}

	// Combine screenshots into a PDF
	if err := e.imagesToPDF(screenshotPaths, output); err != nil {
		return nil, fmt.Errorf("failed to create PDF from screenshots: %w", err)
	}

	return &ExportResult{
		OutputPath: output,
		PageCount:  slideCount,
	}, nil
}

// addMetadata adds PDF metadata (title, author, etc.) to an existing PDF file.
func (e *Exporter) addMetadata(pdfPath string, opts ExportOptions) error {
	properties := make(map[string]string)

	if opts.Title != "" {
		properties["Title"] = opts.Title
	}
	if opts.Author != "" {
		properties["Author"] = opts.Author
	}
	properties["Creator"] = "Tap - Markdown Presentations"
	properties["Producer"] = "Tap (https://tap.sh)"

	if len(properties) == 0 {
		return nil
	}

	conf := model.NewDefaultConfiguration()
	return api.AddPropertiesFile(pdfPath, pdfPath, properties, conf)
}

// ValidateContentType checks if a content type string is valid.
func ValidateContentType(content string) (ContentType, error) {
	switch content {
	case "slides", "":
		return ContentSlides, nil
	case "notes":
		return ContentNotes, nil
	case "both":
		return ContentBoth, nil
	default:
		return "", fmt.Errorf("invalid content type %q: must be 'slides', 'notes', or 'both'", content)
	}
}
