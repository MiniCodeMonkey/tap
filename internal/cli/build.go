// Package cli provides the command-line interface for Tap.
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/builder"
	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/layouts"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
	"github.com/spf13/cobra"
)

// Flags for the build command
var (
	buildOutput string
)

// buildCmd represents the build command
var buildCmd = &cobra.Command{
	Use:   "build <file>",
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
  tap build slides.md                   # Build to dist/ directory
  tap build slides.md --output public   # Build to custom directory
  tap build slides.md -o ./build        # Short form`,
	Args: cobra.ExactArgs(1),
	Run:  runBuild,
}

func init() {
	// Register the build command with root
	rootCmd.AddCommand(buildCmd)

	// Command-specific flags
	buildCmd.Flags().StringVarP(&buildOutput, "output", "o", "dist", "output directory for static files")
}

// runBuild executes the build command logic
func runBuild(cmd *cobra.Command, args []string) {
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

	// Start spinner
	spinner := newSpinner("Building presentation")
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
		Errorln("Error: failed to parse presentation:", file, err)
		os.Exit(1)
	}

	// Resolve and bundle every component the presentation's slides use.
	// Static builds minify and skip source maps. Any bundle error fails
	// the build outright, printed in the same terminal format dev uses.
	spinner.update("Bundling components")
	resolvedComponents, componentBuildErrs := buildComponents(pres, baseDir, true, false)
	if len(componentBuildErrs) > 0 {
		spinner.stop()
		printComponentErrorsToStderr(componentBuildErrs)
		os.Exit(1)
	}

	// Validate layouts and slots before building. This transforms pres to
	// check for warnings, and Builder.Build below transforms it again
	// internally; Builder's API takes the untransformed presentation, and
	// every other caller (dev server, pdf, tests) relies on that, so the
	// second pass isn't threaded through here.
	trans := transformer.NewWithBaseDir(cfg, baseDir)
	trans.SetComponents(resolvedComponents)
	transformed := trans.Transform(pres)
	if warnings := layouts.Validate(transformed); len(warnings) > 0 {
		spinner.stop()
		for _, warning := range warnings {
			fmt.Fprintf(os.Stderr, "error: %s: slide %d: %s\n", file, warning.SlideNumber, warning.Message)
		}
		os.Exit(1)
	}

	// Step 3: Build static files
	spinner.update("Generating static files")
	b := builder.NewWithOutput(buildOutput)
	b.SetBaseDir(baseDir)
	b.SetComponents(resolvedComponents)

	result, err := b.Build(cfg, pres)
	if err != nil {
		spinner.stop()
		Errorln("Error: build failed:", err)
		os.Exit(1)
	}

	// Stop spinner and show results
	spinner.stop()

	printComponentWarningsToStderr(componentWarnings(resolvedComponents))

	// Print success message and build stats
	Successln("\nBuild complete!")
	fmt.Println()
	fmt.Printf("  Output:     %s\n", result.OutputDir)
	fmt.Printf("  Files:      %d\n", result.FileCount)
	fmt.Printf("  Total size: %s\n", formatSize(result.TotalSize))
	fmt.Printf("  Build time: %s\n", formatDuration(result.BuildTime))
	fmt.Println()

	// Show next steps
	Muted("Run 'tap serve %s' to preview the build.\n", result.OutputDir)
}

// spinner provides a simple terminal spinner for progress display
type spinner struct {
	done    chan bool
	message string
	running bool
}

// newSpinner creates a new spinner with the given message
func newSpinner(message string) *spinner {
	return &spinner{
		message: message,
		done:    make(chan bool),
	}
}

// start begins the spinner animation
func (s *spinner) start() {
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
				fmt.Printf("\r%s %s", InfoSprint(frames[frameIndex]), s.message)
				frameIndex = (frameIndex + 1) % len(frames)
			}
		}
	}()
}

// update changes the spinner message
func (s *spinner) update(message string) {
	s.message = message
}

// stop stops the spinner animation
func (s *spinner) stop() {
	if s.running {
		s.running = false
		s.done <- true
		// Clear the spinner line
		fmt.Print("\r\033[K")
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
