// Package cli provides the command-line interface for Tap.
package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/pdf"
	"github.com/spf13/cobra"
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

// runPDF is the command's cobra.Run entry point. It delegates to runPDFE,
// which owns the temporary server and PDF exporter (and their cleanup) for
// the whole export, and turns its returned error into the command's one
// exit(1). Keeping that work in a function that returns an error, rather
// than calling os.Exit from deep inside it, is what lets every defer along
// the way (server shutdown, exporter close) actually run before the
// process exits - os.Exit skips deferred calls, which would otherwise
// orphan the headless browser and the temporary server on every failure
// path after they start.
func runPDF(cmd *cobra.Command, args []string) {
	if err := runPDFE(args); err != nil {
		if !errors.Is(err, errSilent) {
			Errorln("Error:", err)
		}
		os.Exit(1)
	}
}

// runPDFE implements the pdf command. See runPDF for why this is a
// separate, error-returning function.
func runPDFE(args []string) error {
	file := args[0]

	// Validate that the file exists
	if _, err := os.Stat(file); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", file)
	}

	// Get absolute path for base directory resolution
	absPath, err := filepath.Abs(file)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}
	baseDir := filepath.Dir(absPath)

	// Validate content type
	contentType, err := pdf.ValidateContentType(pdfContent)
	if err != nil {
		return err
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
		return fmt.Errorf("failed to load configuration: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		spinner.stop()
		return fmt.Errorf("invalid configuration: %w", err)
	}

	// Step 2: Parse the deck, build its React components, and start a
	// temporary server (port 0 = random available port) with the
	// presentation and component bundles registered on it - exactly the
	// setup tap screenshot uses, via the shared prepareDeck (see
	// internal/cli/deck.go), so the two commands cannot drift apart.
	spinner.update("Parsing presentation and building components")
	srv, _, warnings, componentBuildErrs, componentBuildWarnings, err := prepareDeck(absPath, cfg, baseDir)
	if err != nil {
		spinner.stop()
		return fmt.Errorf("failed to load presentation: %w", err)
	}
	printLayoutWarningsToStderr(absPath, warnings)
	if len(componentBuildErrs) > 0 {
		spinner.stop()
		printComponentErrorsToStderr(componentBuildErrs)
		return errSilent
	}
	printComponentWarningsToStderr(componentBuildWarnings)

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
		return fmt.Errorf("failed to create PDF exporter: %w", err)
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
		Title:   cfg.Title,
		Author:  cfg.Author,
	})
	if err != nil {
		spinner.stop()
		return fmt.Errorf("PDF export failed: %w", err)
	}

	// Stop spinner and show results
	spinner.stop()

	// A slide that shows an error card at export time (a component that
	// throws at render, or a slide that fails to render) still ends up in
	// the PDF - the broken page just shows the card - so this only warns,
	// one line per affected slide, and still exits 0.
	for _, slideNumber := range result.BrokenSlides {
		fmt.Fprintf(os.Stderr, "warning: slide %d shows an error card\n", slideNumber)
	}

	// Print success message and export stats
	Successln("\nPDF export complete!")
	fmt.Println()
	fmt.Printf("  Output:    %s\n", result.OutputPath)
	fmt.Printf("  Pages:     %d\n", result.PageCount)
	fmt.Printf("  File size: %s\n", formatSize(result.FileSize))
	fmt.Printf("  Time:      %s\n", formatDuration(result.Duration))
	fmt.Println()
	return nil
}
