// Package cli provides the command-line interface for Tap.
package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"github.com/tapsh/tap/internal/config"
	"github.com/tapsh/tap/internal/parser"
	"github.com/tapsh/tap/internal/pdf"
	"github.com/tapsh/tap/internal/server"
	"github.com/tapsh/tap/internal/transformer"
)

// Flags for the pdf command
var (
	pdfOutput  string
	pdfContent string
)

// pdfCmd represents the pdf command
var pdfCmd = &cobra.Command{
	Use:   "pdf <file>",
	Short: "Export presentation to PDF",
	Long: `Export a presentation to a PDF file.

The pdf command renders your presentation and exports it to a PDF document.
This is useful for sharing your slides offline or printing handouts.

You can choose what content to include in the PDF:
  - slides: Only the slide content (default)
  - notes:  Only the speaker notes
  - both:   Slides with speaker notes below

Examples:
  tap pdf slides.md                        # Export to slides.pdf
  tap pdf slides.md --output handout.pdf   # Custom output filename
  tap pdf slides.md -o talk.pdf            # Short form
  tap pdf slides.md --content notes        # Export only speaker notes
  tap pdf slides.md --content both         # Slides with notes`,
	Args: cobra.ExactArgs(1),
	Run:  runPDF,
}

func init() {
	// Register the pdf command with root
	rootCmd.AddCommand(pdfCmd)

	// Command-specific flags
	pdfCmd.Flags().StringVarP(&pdfOutput, "output", "o", "", "output PDF file path (default: <input>.pdf)")
	pdfCmd.Flags().StringVar(&pdfContent, "content", "slides", "content to include: slides, notes, or both")
}

// runPDF executes the pdf command logic
func runPDF(cmd *cobra.Command, args []string) {
	file := args[0]

	// Validate that the file exists
	if _, err := os.Stat(file); os.IsNotExist(err) {
		Errorln("Error: file not found:", file)
		os.Exit(1)
	}

	// Get absolute path for base directory resolution
	absPath, err := filepath.Abs(file)
	if err != nil {
		Errorln("Error: failed to resolve file path:", err)
		os.Exit(1)
	}
	baseDir := filepath.Dir(absPath)

	// Validate content type
	contentType, err := pdf.ValidateContentType(pdfContent)
	if err != nil {
		Errorln("Error:", err)
		os.Exit(1)
	}

	// Determine output path
	outputPath := pdfOutput
	if outputPath == "" {
		// Default: replace extension with .pdf
		ext := filepath.Ext(file)
		outputPath = strings.TrimSuffix(file, ext) + ".pdf"
	}

	// Start spinner
	spinner := newSpinner("Preparing PDF export")
	spinner.start()

	// Step 1: Load configuration from frontmatter
	spinner.update("Loading configuration")
	cfg, err := config.Load(file)
	if err != nil {
		spinner.stop()
		Errorln("Error: failed to load configuration:", err)
		os.Exit(1)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		spinner.stop()
		Errorln("Error: invalid configuration:", err)
		os.Exit(1)
	}

	// Step 2: Read and parse the presentation file
	spinner.update("Parsing presentation")
	content, err := os.ReadFile(file)
	if err != nil {
		spinner.stop()
		Errorln("Error: failed to read file:", err)
		os.Exit(1)
	}

	p := parser.New()
	pres, err := p.Parse(content)
	if err != nil {
		spinner.stop()
		Errorln("Error: failed to parse presentation:", err)
		os.Exit(1)
	}

	// Step 3: Transform the presentation
	spinner.update("Transforming presentation")
	trans := transformer.NewWithBaseDir(cfg, baseDir)
	transformed := trans.Transform(pres)

	// Step 4: Start temporary dev server (port 0 = random available port)
	spinner.update("Starting temporary server")
	srv := server.New(0)
	srv.SetPresentation(transformed)
	srv.SetBaseDir(baseDir) // Required for serving local images
	srv.SetupRoutes()

	if err := srv.Start(); err != nil {
		spinner.stop()
		Errorln("Error: failed to start temporary server:", err)
		os.Exit(1)
	}

	// Ensure server is cleaned up on exit
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	// Get the server URL
	serverURL := fmt.Sprintf("http://localhost:%d", srv.Port())

	// Step 5: Create PDF exporter
	spinner.update("Initializing PDF exporter")
	exporter, err := pdf.New()
	if err != nil {
		spinner.stop()
		Errorln("Error: failed to create PDF exporter:", err)
		os.Exit(1)
	}

	// Ensure exporter is cleaned up on exit
	defer func() {
		_ = exporter.Close()
	}()

	// Step 6: Export to PDF
	spinner.update("Generating PDF (this may take a moment)")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	result, err := exporter.Export(ctx, serverURL, pdf.ExportOptions{
		Content: contentType,
		Output:  outputPath,
	})
	if err != nil {
		spinner.stop()
		Errorln("Error: PDF export failed:", err)
		os.Exit(1)
	}

	// Stop spinner and show results
	spinner.stop()

	// Print success message and export stats
	Successln("\nPDF export complete!")
	fmt.Println()
	fmt.Printf("  Output:    %s\n", result.OutputPath)
	fmt.Printf("  Pages:     %d\n", result.PageCount)
	fmt.Printf("  File size: %s\n", formatSize(result.FileSize))
	fmt.Printf("  Time:      %s\n", formatDuration(result.Duration))
	fmt.Println()
}
