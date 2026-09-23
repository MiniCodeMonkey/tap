// Package cli provides the command-line interface for Tap.
package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/builder"
	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/layouts"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

// Flags for the build command
var (
	buildOutput   string
	buildJSON     bool
	buildProgress string
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build [deck]",
	Short: "Build presentation to static HTML",
	Long: `Build a presentation to static HTML files for deployment.

The build command generates a self-contained static website from your
markdown presentation. The output can be deployed to any static hosting
service (GitHub Pages, Netlify, Vercel, etc.).

The generated files include:
  - index.html with embedded presentation
  - All referenced images and assets
  - Necessary JavaScript and CSS

Note: Live code execution is not available in static builds.

Examples:
  tap build                             # The deck in this folder, to dist/
  tap build slides.md                   # Build to dist/ directory
  tap build slides.md --output public   # Build to custom directory
  tap build slides.md -o ./build        # Short form
  tap build talk.md --progress json      # Progress as JSON lines on stderr`,
	Args: cobra.MaximumNArgs(1),
	RunE: runBuild,
}

func init() {
	// Register the build command with root
	rootCmd.AddCommand(buildCmd)

	// Command-specific flags
	buildCmd.Flags().StringVarP(&buildOutput, "output", "o", "dist", "output directory for static files")
	buildCmd.Flags().BoolVar(&buildJSON, "json", false, "print the result as JSON")
	buildCmd.Flags().StringVar(&buildProgress, "progress", "", "print progress to stderr as JSON lines (json)")
}

// buildSteps is the number of --progress json steps tap build reports:
// load, parse, bundle and write.
const buildSteps = 4

// buildResultJSON is the --json and progress result of tap build.
type buildResultJSON struct {
	Output string `json:"output"`
	Files  int    `json:"files"`
	Bytes  int64  `json:"bytes"`
}

// runBuild executes the build command logic
func runBuild(cmd *cobra.Command, args []string) error {
	progress, err := newProgressReporter(buildProgress, cmd.ErrOrStderr())
	if err != nil {
		return err
	}

	file, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
	}

	// Get absolute path for base directory resolution
	absPath, err := filepath.Abs(file)
	if err != nil {
		return internalError(codeInternal, fmt.Errorf("failed to resolve file path: %w", err))
	}
	baseDir := filepath.Dir(absPath)

	// Start spinner
	spinner := newSpinner("Building presentation")
	if progress.enabled() {
		// Progress lines replace the spinner on stderr.
		spinner.isTerminal = func() bool { return false }
	}
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
	progress.Step(progressPhaseLoad, 1, buildSteps)

	// Step 2: Read and parse the presentation file
	spinner.update("Parsing presentation")
	content, err := os.ReadFile(file)
	if err != nil {
		spinner.stop()
		return userError(codeDeckNotFound, fmt.Errorf("failed to read file: %w", err))
	}

	p := parser.New()
	pres, err := p.Parse(content)
	if err != nil {
		spinner.stop()
		return userError(codeInvalidDeck, fmt.Errorf("failed to parse presentation: %s: %w", file, err))
	}
	progress.Step(progressPhaseParse, 2, buildSteps)

	// Resolve and bundle every component the presentation's slides use.
	// Static builds minify and skip source maps. Any bundle error fails
	// the build outright, printed in the same terminal format dev uses.
	// The asset URL prefix is relative ("components/", no leading slash),
	// matching trans.SetComponentURLPrefix below, so the built deck still
	// works when deployed under a sub-path.
	spinner.update("Bundling components")
	resolvedComponents, componentBuildErrs := buildComponents(pres, baseDir, true, false, "components/")
	if len(componentBuildErrs) > 0 {
		spinner.stop()
		printComponentErrors(os.Stderr, componentBuildErrs)
		return reportedError(codeComponentBuild, componentErrorsError(componentBuildErrs))
	}

	// Validate layouts and slots before building. This transforms pres to
	// check for warnings, and Builder.Build below transforms it again
	// internally; Builder's API takes the untransformed presentation, and
	// every other caller (dev server, export pdf, tests) relies on that, so the
	// second pass isn't threaded through here.
	trans := transformer.NewWithBaseDir(cfg, baseDir)
	trans.SetComponents(resolvedComponents)
	transformed := trans.Transform(pres)
	if warnings := layouts.Validate(transformed); len(warnings) > 0 {
		spinner.stop()
		for _, warning := range warnings {
			fmt.Fprintf(os.Stderr, "error: %s: slide %d: %s\n", file, warning.SlideNumber, warning.Message)
		}
		return reportedError(codeInvalidDeck, fmt.Errorf("%d layout error(s)", len(warnings)))
	}
	progress.Step(progressPhaseBundle, 3, buildSteps)

	// Step 3: Build static files
	spinner.update("Generating static files")
	b := builder.NewWithOutput(buildOutput)
	b.SetBaseDir(baseDir)
	b.SetComponents(resolvedComponents)

	result, err := b.Build(cfg, pres)
	if err != nil {
		spinner.stop()
		if errors.Is(err, builder.ErrAllSlidesSkipped) {
			return userError(codeInvalidDeck, err)
		}
		return internalError(codeInternal, fmt.Errorf("build failed: %w", err))
	}
	progress.Step(progressPhaseWrite, 4, buildSteps)

	// Stop spinner and show results
	spinner.stop()

	printComponentWarnings(os.Stderr, componentWarnings(resolvedComponents))
	for _, warning := range result.Warnings {
		Warning("warning: %s\n", warning)
	}

	jsonResult := buildResultJSON{Output: result.OutputDir, Files: result.FileCount, Bytes: result.TotalSize}
	if err := progress.Result(jsonResult); err != nil {
		return err
	}
	if buildJSON {
		return printJSONOK(cmd.OutOrStdout(), jsonResult)
	}

	Successln("\nBuild complete!")
	fmt.Println()
	fmt.Printf("  Output:     %s\n", result.OutputDir)
	fmt.Printf("  Files:      %d\n", result.FileCount)
	fmt.Printf("  Total size: %s\n", formatSize(result.TotalSize))
	fmt.Printf("  Build time: %s\n", formatDuration(result.BuildTime))
	fmt.Println()
	Muted("Run 'tap serve %s' to preview the build.\n", result.OutputDir)
	return nil
}

// spinner provides a simple terminal spinner for progress display
type spinner struct {
	done    chan bool
	message string
	running bool
	// isTerminal reports whether standard error is a terminal worth
	// drawing a spinner on. A field, not a direct isatty call, so a test
	// can force the no-terminal path without depending on how the test
	// binary itself happens to be run.
	isTerminal func() bool
}

// newSpinner creates a new spinner with the given message
func newSpinner(message string) *spinner {
	return &spinner{
		message:    message,
		done:       make(chan bool),
		isTerminal: stderrIsTerminal,
	}
}

// stderrIsTerminal reports whether standard error is a terminal.
func stderrIsTerminal() bool {
	return isatty.IsTerminal(os.Stderr.Fd())
}

// start begins the spinner animation, writing to standard error - progress
// output, not a command's result, which only ever belongs on standard
// output (see tap build and tap export pdf's own success lines). It draws
// nothing at all when standard error is not a terminal (redirected to a
// file, piped, or running in CI): a spinner frame with no terminal to
// erase it just leaves a stream of "\r..." noise behind.
func (s *spinner) start() {
	if !s.isTerminal() {
		return
	}

	s.running = true
	frames := []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}
	frameIndex := 0

	go func() {
		ticker := time.NewTicker(80 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-s.done:
				return
			case <-ticker.C:
				// Clear line and print spinner
				fmt.Fprintf(os.Stderr, "\r%s %s", InfoSprint(frames[frameIndex]), s.message)
				frameIndex = (frameIndex + 1) % len(frames)
			}
		}
	}()
}

// update changes the spinner message
func (s *spinner) update(message string) {
	s.message = message
}

// stop stops the spinner animation and clears its line from standard
// error. A no-op when the spinner was never started (standard error is
// not a terminal - see start).
func (s *spinner) stop() {
	if s.running {
		s.running = false
		s.done <- true
		// Clear the spinner line
		fmt.Fprint(os.Stderr, "\r\033[K")
	}
}

// formatSize formats a file size in bytes to a human-readable string
func formatSize(bytes int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
	)

	switch {
	case bytes >= GB:
		return fmt.Sprintf("%.2f GB", float64(bytes)/GB)
	case bytes >= MB:
		return fmt.Sprintf("%.2f MB", float64(bytes)/MB)
	case bytes >= KB:
		return fmt.Sprintf("%.2f KB", float64(bytes)/KB)
	default:
		return fmt.Sprintf("%d bytes", bytes)
	}
}

// formatDuration formats a duration to a human-readable string
func formatDuration(d time.Duration) string {
	if d < time.Millisecond {
		return fmt.Sprintf("%d\u00B5s", d.Microseconds())
	}
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	return fmt.Sprintf("%.2fs", d.Seconds())
}
