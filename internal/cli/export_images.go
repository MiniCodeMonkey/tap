// Package cli provides the command-line interface for Tap.
package cli

import (
	"context"
	"fmt"
	"math"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/pdf"
	"github.com/MiniCodeMonkey/tap/internal/themes"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
	"github.com/spf13/cobra"
)

// Flags for the export images command
var (
	screenshotSlide    int
	screenshotStep     int
	screenshotFragment int
	screenshotTheme    string
	screenshotOutput   string
	screenshotWidth    int
	screenshotAll      bool
	screenshotWait     int
	screenshotJSON     bool
)

// exportImagesCmd represents the export images command
var exportImagesCmd = &cobra.Command{
	Use:   "images [deck]",
	Short: "Render slides to PNG images",
	Long: `Render one slide, or every slide, of a presentation to a PNG image.

Made for an LLM (or a person) to check a slide it just wrote, without
opening a browser: it starts a temporary server, drives a headless
Chromium through the same machinery tap export pdf uses, and screenshots
the requested slide state.

With neither --step nor --fragment, the slide renders its final state
through print mode, the same way tap export pdf does. --step N renders the
slide after N steps and --fragment N with N fragments shown; 0 is the
state before the first one. A flag you leave out takes its final value.
--wait keeps the capture live and waits this many milliseconds after the
page is ready before capturing, instead of settling - for an animation
that starts on a timer or runs longer than the readiness waits.

Exits with status 1 and a one-line message on a missing deck, an
out-of-range slide, step or fragment, an unknown theme, an out-of-range
--wait, a browser that cannot start, or a rendered slide that shows a
slide or component error card. On success, prints the path of each file
written, one per line, and nothing else.

Examples:
  tap export images --slide 12                       # The deck in this folder, slide 12
  tap export images deck.md --slide 12               # Final state of slide 12
  tap export images deck.md --slide 12 --step 3      # Slide 12 after its third step
  tap export images deck.md --slide 12 --step 3 --wait 400
  tap export images deck.md --slide 12 -o slide.png  # Custom output file
  tap export images deck.md --all                    # Every slide's final state
  tap export images deck.md --slide 3 -t bauhaus     # Render with a specific theme
  tap export images deck.md --all --json             # Print the written files as JSON`,
	Args: cobra.MaximumNArgs(1),
	RunE: runExportImages,
}

func init() {
	exportCmd.AddCommand(exportImagesCmd)

	exportImagesCmd.Flags().IntVar(&screenshotSlide, "slide", 0, "slide number to capture, from 1 (required unless --all)")
	exportImagesCmd.Flags().IntVar(&screenshotStep, "step", 0, "render the slide after this many steps, from 0 (default: the final step)")
	exportImagesCmd.Flags().IntVar(&screenshotFragment, "fragment", 0, "render the slide with this many fragments shown, from 0 (default: all)")
	exportImagesCmd.Flags().StringVarP(&screenshotTheme, "theme", "t", "", "theme slug to render with (default: the deck's own theme)")
	exportImagesCmd.Flags().StringVarP(&screenshotOutput, "output", "o", "", "output PNG file (one slide) or folder (--all); default derived from the deck's file name")
	exportImagesCmd.Flags().IntVar(&screenshotWidth, "width", 1920, "viewport width in pixels; height follows the deck's aspect ratio")
	exportImagesCmd.Flags().BoolVar(&screenshotAll, "all", false, "capture every slide's final state into a folder instead of one slide")
	exportImagesCmd.Flags().IntVar(&screenshotWait, "wait", 0, "milliseconds to wait after the page is ready before capturing, instead of settling (0-60000)")
	exportImagesCmd.Flags().BoolVar(&screenshotJSON, "json", false, "print the written files as JSON")
}

// runExportImages implements the export images command. It returns an
// error rather than calling os.Exit from deep inside it, which is what
// lets every defer along the way (server shutdown, browser close) actually
// run before the process exits - os.Exit skips deferred calls, which would
// otherwise orphan the headless browser and the temporary server on every
// failure path.
func runExportImages(cmd *cobra.Command, args []string) error {
	// Cancelled on Ctrl-C (SIGINT) or SIGTERM, so the capture loop below
	// can stop between slides instead of leaving a headless browser
	// running past the deferred cleanup below.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()
	// stop() also runs the moment ctx is done, rather than waiting for
	// this function to return: signal.NotifyContext keeps intercepting the
	// signal until stop() runs, so without this a second Ctrl-C during the
	// cleanup below (browser close) would just cancel the already
	// cancelled context again instead of falling through to the OS
	// default handler, which is what actually kills the process
	// immediately.
	context.AfterFunc(ctx, stop)

	hasSlideFlag := cmd.Flags().Changed("slide")
	hasStepFlag := cmd.Flags().Changed("step")
	hasFragmentFlag := cmd.Flags().Changed("fragment")

	if err := validateScreenshotFlags(screenshotAll, hasSlideFlag, hasStepFlag, hasFragmentFlag, screenshotWidth); err != nil {
		return err
	}

	if err := validateWaitFlag(screenshotWait); err != nil {
		return err
	}

	if screenshotTheme != "" && !themes.IsValid(screenshotTheme) {
		return userError(codeUnknownTheme, unknownThemeError(screenshotTheme))
	}

	file, err := resolveDeck(firstArg(args))
	if err != nil {
		return err
	}

	absPath, err := filepath.Abs(file)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}
	baseDir := filepath.Dir(absPath)

	cfg, err := config.Load(file)
	if err != nil {
		return userError(codeInvalidDeck, fmt.Errorf("failed to load configuration: %w", err))
	}
	if err := cfg.Validate(); err != nil {
		return userError(codeInvalidDeck, fmt.Errorf("invalid configuration: %w", err))
	}

	srv, pres, warnings, componentBuildErrs, componentBuildWarnings, err := prepareDeck(absPath, cfg, baseDir)
	if err != nil {
		return fmt.Errorf("failed to load presentation: %w", err)
	}
	printLayoutWarningsToStderr(absPath, warnings)
	if len(componentBuildErrs) > 0 {
		printComponentErrorsToStderr(componentBuildErrs)
		return reportedError(codeComponentBuild, componentErrorsError(componentBuildErrs))
	}
	printComponentWarningsToStderr(componentBuildWarnings)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	total := len(pres.Slides)
	if total == 0 {
		return userError(codeInvalidDeck, fmt.Errorf("deck has no slides"))
	}

	if !screenshotAll {
		if screenshotSlide < 1 || screenshotSlide > total {
			return slideOutOfRangeError(screenshotSlide, total)
		}
	}

	var slide *transformer.TransformedSlide
	if !screenshotAll {
		slide = &pres.Slides[screenshotSlide-1]
		if err := validateStepAndFragment(hasStepFlag, screenshotStep, hasFragmentFlag, screenshotFragment, *slide, screenshotSlide); err != nil {
			return err
		}
	}

	width, height, err := resolveDimensions(cfg.AspectRatio, screenshotWidth)
	if err != nil {
		return userError(codeInvalidDeck, err)
	}

	// The temporary server (already serving the deck's presentation and
	// component bundles) came back from prepareDeck above, exactly the
	// setup tap export pdf uses.
	serverURL := fmt.Sprintf("http://localhost:%d", srv.Port())

	exporter, err := pdf.New()
	if err != nil {
		return internalError(codeBrowser, fmt.Errorf("failed to create browser exporter: %w", err))
	}
	defer func() { _ = exporter.Close() }()

	if err := exporter.EnsureBrowser(); err != nil {
		return internalError(codeBrowser, fmt.Errorf("failed to start browser: %w", err))
	}

	if screenshotAll {
		written, broken, err := captureAllSlides(ctx, exporter.CaptureSlide, serverURL, total, width, height, screenshotTheme, resolveAllOutputDir(screenshotOutput, file))
		if err != nil {
			// A real Ctrl-C signals the whole process group, so the
			// headless browser often dies first and capture fails with its
			// own error rather than one that wraps context.Canceled. ctx
			// itself is the source of truth for whether this run was
			// interrupted, regardless of how that surfaced in err.
			if ctx.Err() != nil {
				return errInterrupted
			}
			return err
		}
		if len(broken) > 0 {
			for _, brokenSlideResult := range broken {
				fmt.Fprintf(os.Stderr, "slide %d: %s\n", brokenSlideResult.SlideNumber, brokenSlideResult.Reason)
			}
			if !screenshotJSON {
				if err := printWrittenImages(cmd, written); err != nil {
					return err
				}
			}
			return reportedError(codeBrokenSlides, fmt.Errorf("%d slide(s) failed to capture", len(broken)))
		}
		return printWrittenImages(cmd, written)
	}

	outputPath := screenshotOutput
	if outputPath == "" {
		outputPath = fmt.Sprintf("./%s-slide-%d.png", deckBaseName(file), screenshotSlide)
	}

	options := pdf.CaptureOptions{
		SlideNumber: screenshotSlide,
		Width:       width,
		Height:      height,
		Theme:       screenshotTheme,
		WaitMS:      screenshotWait,
	}
	// A step, a fragment or --wait needs the live viewer at an exact
	// state. Otherwise print mode renders the final state.
	if hasStepFlag || hasFragmentFlag || screenshotWait > 0 {
		urlStep, urlFragment := captureState(hasStepFlag, screenshotStep, hasFragmentFlag, screenshotFragment, *slide)
		options.Step = &urlStep
		options.Fragment = &urlFragment
	} else {
		options.Print = true
	}

	if err := exporter.CaptureSlide(ctx, serverURL, options, outputPath); err != nil {
		if ctx.Err() != nil {
			return errInterrupted
		}
		return err
	}

	return printWrittenImages(cmd, []string{outputPath})
}

// printWrittenImages prints the written PNG paths: one per line, or as
// the --json result.
func printWrittenImages(cmd *cobra.Command, paths []string) error {
	if screenshotJSON {
		return printJSONOK(cmd.OutOrStdout(), struct {
			Files []string `json:"files"`
		}{Files: paths})
	}
	for _, path := range paths {
		fmt.Fprintln(cmd.OutOrStdout(), path)
	}
	return nil
}

// brokenSlide describes one slide that failed to capture during --all: a
// capture error (a browser or navigation failure) or a rendered error card
// (see pdf.ErrorCardSelector). Reason is the underlying error's message.
type brokenSlide struct {
	SlideNumber int
	Reason      string
}

// captureFunc matches (*pdf.Exporter).CaptureSlide's signature, so
// captureAllSlides's loop can be tested against a fake instead of a real
// browser.
type captureFunc func(ctx context.Context, serverURL string, options pdf.CaptureOptions, outputPath string) error

// captureAllSlides writes one PNG per slide, at its final state, into
// outputDir, named slide-001.png and so on. It tries every slide even
// after one fails: a broken slide (a capture error, or a rendered error
// card) is collected and does not stop the rest from being written. The
// only fatal errors are one that stops the loop before it can try any
// slide at all (failing to create outputDir), and ctx being cancelled
// (Ctrl-C or SIGTERM), which is checked between slides so the loop stops
// there instead of starting one more capture.
//
// Returns the paths successfully written, in slide order, and the slides
// that failed, also in slide order. Printing - the written paths to
// standard output, one "slide N: reason" line per broken slide to standard
// error - is the caller's job, which is what keeps this loop cheap to
// test against a fake captureFunc.
func captureAllSlides(ctx context.Context, capture captureFunc, serverURL string, slideCount int, width, height int, theme, outputDir string) ([]string, []brokenSlide, error) {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create output directory: %w", err)
	}

	var written []string
	var broken []brokenSlide
	for i := 0; i < slideCount; i++ {
		if err := ctx.Err(); err != nil {
			return written, broken, err
		}

		slideNumber := i + 1
		outputPath := filepath.Join(outputDir, fmt.Sprintf("slide-%03d.png", slideNumber))
		options := pdf.CaptureOptions{
			SlideNumber: slideNumber,
			Width:       width,
			Height:      height,
			Theme:       theme,
			Print:       true,
		}
		if err := capture(ctx, serverURL, options, outputPath); err != nil {
			// ctx itself, not the shape of err, decides whether this was
			// an interruption: a real Ctrl-C signals the whole process
			// group, so the headless browser can die first and capture
			// can fail with its own error rather than one that wraps
			// context.Canceled.
			if ctx.Err() != nil {
				return written, broken, err
			}
			broken = append(broken, brokenSlide{SlideNumber: slideNumber, Reason: err.Error()})
			continue
		}
		written = append(written, outputPath)
	}
	return written, broken, nil
}

// validateScreenshotFlags checks the flag combination rules that don't
// need the deck's content: exactly one of --slide or --all, --step/
// --fragment only make sense with --slide, and --width must be positive.
func validateScreenshotFlags(all, hasSlide, hasStep, hasFragment bool, width int) error {
	if all && hasSlide {
		return userError(codeUsage, fmt.Errorf("--slide cannot be combined with --all"))
	}
	if !all && !hasSlide {
		return userError(codeUsage, fmt.Errorf("--slide is required unless --all is set"))
	}
	if all && (hasStep || hasFragment) {
		return userError(codeUsage, fmt.Errorf("--step and --fragment cannot be combined with --all"))
	}
	if width <= 0 {
		return userError(codeUsage, fmt.Errorf("--width must be a positive number of pixels, got %d", width))
	}
	return nil
}

// validateWaitFlag checks --wait is within the range CaptureSlide will
// actually sleep for: 0 (no wait, the default) up to 60 seconds.
func validateWaitFlag(waitMS int) error {
	const maxWaitMS = 60000
	if waitMS < 0 || waitMS > maxWaitMS {
		return userError(codeUsage, fmt.Errorf("--wait must be between 0 and %d milliseconds, got %d", maxWaitMS, waitMS))
	}
	return nil
}

// slideOutOfRangeError reports a slide number outside the deck's range,
// saying how many slides the deck has.
func slideOutOfRangeError(slideNumber, total int) error {
	return userError(codeOutOfRange, fmt.Errorf("slide %d is out of range: deck has %d slide(s)", slideNumber, total))
}

// unknownThemeError reports an unknown theme slug, listing every valid one.
func unknownThemeError(slug string) error {
	return fmt.Errorf("unknown theme %q: valid themes are %s", slug, strings.Join(themes.Slugs(), ", "))
}

// validateStepAndFragment checks --step and --fragment against the slide.
// Both count reveals: 0 is the state before the first one, and the last
// valid value is the slide's own count.
func validateStepAndFragment(hasStep bool, step int, hasFragment bool, fragment int, slide transformer.TransformedSlide, slideNumber int) error {
	if hasStep && (step < 0 || step > slide.Steps) {
		return userError(codeOutOfRange, fmt.Errorf("step %d is out of range for slide %d: valid steps are 0-%d", step, slideNumber, slide.Steps))
	}
	if hasFragment && (fragment < 0 || fragment > slide.FragmentCount) {
		return userError(codeOutOfRange, fmt.Errorf("fragment %d is out of range for slide %d: valid fragments are 0-%d", fragment, slideNumber, slide.FragmentCount))
	}
	return nil
}

// captureState turns --step and --fragment into the frontend's ?step= and
// ?fragment= values. ?step=N is the state after N steps, and ?fragment=N
// shows fragments 0 to N, so a count of N fragments is ?fragment=N-1. A
// flag left out takes its final value.
func captureState(hasStep bool, step int, hasFragment bool, fragment int, slide transformer.TransformedSlide) (urlStep, urlFragment int) {
	urlStep = slide.Steps
	if hasStep {
		urlStep = step
	}
	urlFragment = slide.FragmentCount - 1
	if hasFragment {
		urlFragment = fragment - 1
	}
	return urlStep, urlFragment
}

// resolveDimensions turns an aspect ratio string ("16:9", "4:3", "16:10")
// and a target width into a (width, height) pixel pair, rounding height to
// the nearest pixel.
func resolveDimensions(aspectRatio string, width int) (int, int, error) {
	parts := strings.SplitN(aspectRatio, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("invalid aspect ratio %q", aspectRatio)
	}
	widthRatio, widthRatioErr := strconv.ParseFloat(parts[0], 64)
	heightRatio, heightRatioErr := strconv.ParseFloat(parts[1], 64)
	if widthRatioErr != nil || heightRatioErr != nil || widthRatio <= 0 || heightRatio <= 0 {
		return 0, 0, fmt.Errorf("invalid aspect ratio %q", aspectRatio)
	}
	height := int(math.Round(float64(width) * heightRatio / widthRatio))
	return width, height, nil
}

// deckBaseName returns a deck file's base name without its extension, used
// to build default output file and folder names.
func deckBaseName(file string) string {
	base := filepath.Base(file)
	return strings.TrimSuffix(base, filepath.Ext(base))
}

// resolveAllOutputDir returns the folder --all writes into: the --output
// value if given, otherwise ./<deck base name>-slides/.
func resolveAllOutputDir(out, file string) string {
	if out != "" {
		return out
	}
	return fmt.Sprintf("./%s-slides", deckBaseName(file))
}
