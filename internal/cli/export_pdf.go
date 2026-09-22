// Package cli provides the command-line interface for Tap.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/pdf"
	"github.com/spf13/cobra"
)

// Flags for the pdf command
var (
	pdfOutput  string
	pdfContent string
	pdfJSON    bool
)

// exportPDFCmd represents the export pdf command
var exportPDFCmd = &cobra.Command{
	Use:   "pdf [deck]",
	Short: "Export presentation to PDF",
	Long: `Export a presentation to a PDF file.

The pdf command renders your presentation and exports it to a PDF document.
This is useful for sharing your slides offline or printing handouts.

You can choose what content to include in the PDF:
  - slides: Only the slide content (default)
  - notes:  Only the speaker notes
  - both:   Slides with speaker notes below

Examples:
  tap export pdf                              # The deck in this folder, to <deck>.pdf
  tap export pdf slides.md                    # Export to slides.pdf
  tap export pdf slides.md --output handout.pdf
  tap export pdf slides.md -o talk.pdf        # Short form
  tap export pdf slides.md --content notes    # Only speaker notes
  tap export pdf slides.md --content both     # Slides with notes
  tap export pdf slides.md --json             # Print the result as JSON`,
	Args: cobra.MaximumNArgs(1),
	RunE: runExportPDF,
}

func init() {
	exportCmd.AddCommand(exportPDFCmd)

	exportPDFCmd.Flags().StringVarP(&pdfOutput, "output", "o", "", "output PDF file path (default: <deck>.pdf)")
	exportPDFCmd.Flags().StringVar(&pdfContent, "content", "slides", "content to include: slides, notes, or both")
	exportPDFCmd.Flags().BoolVar(&pdfJSON, "json", false, "print the result as JSON")
}

// runExportPDF implements the export pdf command. It returns an error
// rather than calling os.Exit from deep inside it, which is what lets
// every defer along the way (server shutdown, exporter close) actually
// run before the process exits - os.Exit skips deferred calls, which
// would otherwise orphan the headless browser and the temporary server on
// every failure path after they start.
func runExportPDF(cmd *cobra.Command, args []string) error {
	// Cancelled on Ctrl-C (SIGINT) or SIGTERM, so the export loop below can
	// stop between slides instead of leaving a headless browser running
	// past the deferred cleanup below.
	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	// stop() also runs the moment signalCtx is done, rather than waiting
	// for this function to return: signal.NotifyContext keeps intercepting
	// the signal until stop() runs, so without this a second Ctrl-C during
	// the cleanup below (server shutdown, exporter close) would just
	// cancel the already-cancelled context again instead of falling
	// through to the OS default handler, which is what actually kills the
	// process immediately.
	context.AfterFunc(signalCtx, stop)

	file, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
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
		return userError(codeUsage, err)
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
		return userError(codeInvalidDeck, fmt.Errorf("failed to load configuration: %w", err))
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		spinner.stop()
		return userError(codeInvalidDeck, fmt.Errorf("invalid configuration: %w", err))
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
	// The spinner and these warnings both write to standard error; stop it
	// first so a warning line never lands mid-frame, then start it again
	// (with the next step's message) once they are printed.
	spinner.stop()
	printLayoutWarningsToStderr(absPath, warnings)
	if len(componentBuildErrs) > 0 {
		printComponentErrorsToStderr(componentBuildErrs)
		return reportedError(codeComponentBuild, componentErrorsError(componentBuildErrs))
	}
	printComponentWarningsToStderr(componentBuildWarnings)
	spinner.start()

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
		return internalError(codeBrowser, fmt.Errorf("failed to create PDF exporter: %w", err))
	}

	// Ensure exporter is cleaned up on exit
	defer func() {
		_ = exporter.Close()
	}()

	// Step 6: Export to PDF
	spinner.update("Generating PDF (this may take a moment)")
	ctx, cancel := context.WithTimeout(signalCtx, 5*time.Minute)
	defer cancel()

	result, err := exporter.Export(ctx, serverURL, pdf.ExportOptions{
		Content: contentType,
		Output:  outputPath,
		Title:   cfg.Title,
		Author:  cfg.Author,
	})
	if err != nil {
		spinner.stop()
		// A real Ctrl-C signals the whole process group, so the headless
		// browser often dies first and Export fails with its own error
		// (a closed target, a lost connection) rather than one that wraps
		// context.Canceled. signalCtx itself is the source of truth for
		// whether this run was interrupted, regardless of how that
		// surfaced in err.
		if signalCtx.Err() != nil {
			return errInterrupted
		}
		return internalError(codeExportFailed, fmt.Errorf("PDF export failed: %w", err))
	}

	// Stop spinner and show results
	spinner.stop()

	// A slide that shows an error card at export time (a component that
	// throws at render, or a slide that fails to render) still ends up in
	// the PDF - the broken page just shows the card - so this only warns,
	// one line per affected slide, and still exits 0.
	for _, broken := range result.BrokenSlides {
		fmt.Fprintf(os.Stderr, "warning: slide %d shows an error card: %s\n", broken.SlideNumber, broken.Message)
	}

	if pdfJSON {
		brokenSlides := make([]brokenSlideJSON, 0, len(result.BrokenSlides))
		for _, broken := range result.BrokenSlides {
			brokenSlides = append(brokenSlides, brokenSlideJSON{Slide: broken.SlideNumber, Message: broken.Message})
		}
		return printJSONOK(cmd.OutOrStdout(), exportPDFResult{
			Output:       result.OutputPath,
			Pages:        result.PageCount,
			Bytes:        result.FileSize,
			BrokenSlides: brokenSlides,
		})
	}

	Successln("\nPDF export complete!")
	fmt.Println()
	fmt.Printf("  Output:    %s\n", result.OutputPath)
	fmt.Printf("  Pages:     %d\n", result.PageCount)
	fmt.Printf("  File size: %s\n", formatSize(result.FileSize))
	fmt.Printf("  Time:      %s\n", formatDuration(result.Duration))
	fmt.Println()
	return nil
}

// exportPDFResult is the --json result of tap export pdf.
type exportPDFResult struct {
	Output       string            `json:"output"`
	Pages        int               `json:"pages"`
	Bytes        int64             `json:"bytes"`
	BrokenSlides []brokenSlideJSON `json:"brokenSlides"`
}

// brokenSlideJSON is one slide that showed an error card, with its
// 1-based number.
type brokenSlideJSON struct {
	Slide   int    `json:"slide"`
	Message string `json:"message"`
}
