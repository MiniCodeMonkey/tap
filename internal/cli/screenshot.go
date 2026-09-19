// Package cli provides the command-line interface for Tap.
package cli

import (
	"context"
	"errors"
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

// errSilent marks a failure that has already printed its own diagnostics
// to standard error (component build errors, one line per broken --all
// slide), so the caller (runScreenshot, runPDF) should exit 1 without
// printing anything more. Shared by both commands, so its text names
// neither.
var errSilent = errors.New("command failed")

// errInterrupted marks a failure caused by Ctrl-C (SIGINT) or SIGTERM
// during a capture or export: runScreenshot and runPDF print "interrupted"
// to standard error instead of the usual "Error: ..." line and exit with
// status 130, the conventional exit code for a process killed by SIGINT.
var errInterrupted = errors.New("interrupted")

// Flags for the screenshot command
var (
	screenshotSlide    int
	screenshotStep     int
	screenshotFragment int
	screenshotTheme    string
	screenshotOut      string
	screenshotWidth    int
	screenshotAll      bool
	screenshotWait     int
)

// screenshotCmd represents the screenshot command
var screenshotCmd = &cobra.Command{
	Use:   "screenshot <file>",
	Short: "Render a slide to a PNG",
	Long: `Render one slide, or every slide, of a presentation to a PNG image.

Made for an LLM (or a person) to check a slide it just wrote, without
opening a browser: it starts a temporary server, drives a headless
Chromium through the same machinery tap pdf uses, and screenshots the
requested slide state.

With neither --step nor --fragment, the slide renders its final state
through print mode, the same way tap pdf does. With --step and/or
--fragment, the slide renders that exact presenter state instead, without
print mode, but still settled: any component or theme animation is given
its finished state at that step by the readiness waits already applied
(network idle, fonts, running animations), not caught partway through.
--wait keeps the capture live and waits this many milliseconds after the
page is ready before capturing, instead of settling - for an animation
that starts on a timer or runs longer than the readiness waits.

Exits with status 1 and a one-line message on a missing deck, an
out-of-range slide, step or fragment, an unknown theme, an out-of-range
--wait, a browser that cannot start, or a rendered slide that shows a
slide or component error card. On success, prints the path of each file
written, one per line, and nothing else.

Examples:
  tap screenshot deck.md --slide 12                        # Final state of slide 12
  tap screenshot deck.md --slide 12 --step 3                # Slide 12 at step 3, settled
  tap screenshot deck.md --slide 12 --step 3 --wait 400      # ...400ms after ready, instead of settled
  tap screenshot deck.md --slide 12 --out slide.png          # Custom output file
  tap screenshot deck.md --all                                # Every slide's final state
  tap screenshot deck.md --slide 3 --theme bauhaus            # Render with a specific theme`,
	Args: cobra.ExactArgs(1),
	Run:  runScreenshot,
}

func init() {
	rootCmd.AddCommand(screenshotCmd)

	screenshotCmd.Flags().IntVar(&screenshotSlide, "slide", 0, "one-based slide number to capture (required unless --all)")
	screenshotCmd.Flags().IntVar(&screenshotStep, "step", 0, "presenter step to render, without print mode (default: final state, in print mode)")
	screenshotCmd.Flags().IntVar(&screenshotFragment, "fragment", -1, "fragment index to render, without print mode (default: final state, in print mode)")
	screenshotCmd.Flags().StringVar(&screenshotTheme, "theme", "", "theme slug to render with (default: the deck's own theme)")
	screenshotCmd.Flags().StringVar(&screenshotOut, "out", "", "output PNG file (single slide) or folder (--all); default derived from the deck's file name")
	screenshotCmd.Flags().IntVar(&screenshotWidth, "width", 1920, "viewport width in pixels; height follows the deck's aspect ratio")
	screenshotCmd.Flags().BoolVar(&screenshotAll, "all", false, "capture every slide's final state into a folder instead of one slide")
	screenshotCmd.Flags().IntVar(&screenshotWait, "wait", 0, "milliseconds to wait after the page is ready before capturing, instead of settling (0-60000); keeps the capture live, for an animation that starts on a timer or runs longer than the readiness waits")
}

// runScreenshot is the command's cobra.Run entry point. It delegates to
// runScreenshotE, which owns the temporary server and browser (and their
// cleanup) for the whole capture, and turns its returned error into the
// command's one exit(1). Keeping that work in a function that returns an
// error, rather than calling os.Exit from deep inside it, is what lets
// every defer along the way (server shutdown, browser close) actually run
// before the process exits - os.Exit skips deferred calls, which would
// otherwise orphan the headless browser and the temporary server on every
// failure path.
func runScreenshot(cmd *cobra.Command, args []string) {
	if err := runScreenshotE(cmd, args); err != nil {
		if errors.Is(err, errInterrupted) {
			fmt.Fprintln(os.Stderr, "interrupted")
			os.Exit(130)
		}
		if !errors.Is(err, errSilent) {
			Errorln("Error:", err)
		}
		os.Exit(1)
	}
}

// runScreenshotE implements the screenshot command. See runScreenshot for
// why this is a separate, error-returning function.
func runScreenshotE(cmd *cobra.Command, args []string) error {
	// Cancelled on Ctrl-C (SIGINT) or SIGTERM, so the capture loop below
	// can stop between slides instead of leaving a headless browser
	// running past the deferred cleanup below.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	file := args[0]
	hasSlideFlag := cmd.Flags().Changed("slide")
	hasStepFlag := cmd.Flags().Changed("step")
	hasFragmentFlag := cmd.Flags().Changed("fragment")
	hasWaitFlag := cmd.Flags().Changed("wait")

	if err := validateScreenshotFlags(screenshotAll, hasSlideFlag, hasStepFlag, hasFragmentFlag, screenshotWidth); err != nil {
		return err
	}

	if err := validateWaitFlag(screenshotWait); err != nil {
		return err
	}

	if screenshotTheme != "" && !themes.IsValid(screenshotTheme) {
		return unknownThemeError(screenshotTheme)
	}

	if _, err := os.Stat(file); os.IsNotExist(err) {
		return fmt.Errorf("deck not found: %s", file)
	}

	absPath, err := filepath.Abs(file)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}
	baseDir := filepath.Dir(absPath)

	cfg, err := config.Load(file)
	if err != nil {
		return fmt.Errorf("failed to load configuration: %w", err)
	}
	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	srv, pres, warnings, componentBuildErrs, componentBuildWarnings, err := prepareDeck(absPath, cfg, baseDir)
	if err != nil {
		return fmt.Errorf("failed to load presentation: %w", err)
	}
	printLayoutWarningsToStderr(absPath, warnings)
	printComponentErrorsToStderr(componentBuildErrs)
	if len(componentBuildErrs) > 0 {
		return errSilent
	}
	printComponentWarningsToStderr(componentBuildWarnings)
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	total := len(pres.Slides)
	if total == 0 {
		return fmt.Errorf("deck has no slides")
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
		return err
	}

	// The temporary server (already serving the deck's presentation and
	// component bundles) came back from prepareDeck above, exactly the
	// setup tap pdf uses.
	serverURL := fmt.Sprintf("http://localhost:%d", srv.Port())

	exporter, err := pdf.New()
	if err != nil {
		return fmt.Errorf("failed to create browser exporter: %w", err)
	}
	defer func() { _ = exporter.Close() }()

	if err := exporter.EnsureBrowser(); err != nil {
		return fmt.Errorf("failed to start browser: %w", err)
	}

	if screenshotAll {
		written, broken, err := captureAllSlides(ctx, exporter.CaptureSlide, serverURL, total, width, height, screenshotTheme, resolveAllOutputDir(screenshotOut, file))
		if err != nil {
			if errors.Is(err, context.Canceled) {
				return errInterrupted
			}
			return err
		}
		for _, path := range written {
			fmt.Println(path)
		}
		if len(broken) > 0 {
			for _, brokenSlideResult := range broken {
				fmt.Fprintf(os.Stderr, "slide %d: %s\n", brokenSlideResult.SlideNumber, brokenSlideResult.Reason)
			}
			return errSilent
		}
		return nil
	}

	outputPath := screenshotOut
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
	// --wait keeps the capture live even with no --step or --fragment (see
	// buildSlideURL): a live capture still wants live semantics, since
	// there is no such thing as "print mode, but wait first" - print mode
	// forces the settled final state, exactly what --wait exists to skip.
	if hasStepFlag || hasFragmentFlag || (hasWaitFlag && screenshotWait > 0) {
		if hasStepFlag {
			step := screenshotStep
			options.Step = &step
		}
		if hasFragmentFlag {
			fragment := screenshotFragment
			options.Fragment = &fragment
		}
	} else {
		options.Print = true
	}

	if err := exporter.CaptureSlide(ctx, serverURL, options, outputPath); err != nil {
		if errors.Is(err, context.Canceled) {
			return errInterrupted
		}
		return err
	}

	fmt.Println(outputPath)
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
			if errors.Is(err, context.Canceled) {
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
		return fmt.Errorf("--slide cannot be combined with --all")
	}
	if !all && !hasSlide {
		return fmt.Errorf("--slide is required unless --all is set")
	}
	if all && (hasStep || hasFragment) {
		return fmt.Errorf("--step and --fragment cannot be combined with --all")
	}
	if width <= 0 {
		return fmt.Errorf("--width must be a positive number of pixels, got %d", width)
	}
	return nil
}

// validateWaitFlag checks --wait is within the range CaptureSlide will
// actually sleep for: 0 (no wait, the default) up to 60 seconds.
func validateWaitFlag(waitMS int) error {
	const maxWaitMS = 60000
	if waitMS < 0 || waitMS > maxWaitMS {
		return fmt.Errorf("--wait must be between 0 and %d milliseconds, got %d", maxWaitMS, waitMS)
	}
	return nil
}

// slideOutOfRangeError reports a slide number outside the deck's range,
// saying how many slides the deck has.
func slideOutOfRangeError(slideNumber, total int) error {
	return fmt.Errorf("slide %d is out of range: deck has %d slide(s)", slideNumber, total)
}

// unknownThemeError reports an unknown theme slug, listing every valid one.
func unknownThemeError(slug string) error {
	return fmt.Errorf("unknown theme %q: valid themes are %s", slug, strings.Join(themes.Slugs(), ", "))
}

// validateStepAndFragment checks a requested --step and --fragment against
// the limits of the given slide, saying those limits when out of range.
func validateStepAndFragment(hasStep bool, step int, hasFragment bool, fragment int, slide transformer.TransformedSlide, slideNumber int) error {
	if hasStep && (step < 0 || step > slide.Steps) {
		return fmt.Errorf("step %d is out of range for slide %d: valid steps are 0-%d", step, slideNumber, slide.Steps)
	}
	maxFragment := slide.FragmentCount - 1
	if hasFragment && (fragment < -1 || fragment > maxFragment) {
		return fmt.Errorf("fragment %d is out of range for slide %d: valid fragments are -1-%d", fragment, slideNumber, maxFragment)
	}
	return nil
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

// resolveAllOutputDir returns the folder --all writes into: the --out
// value if given, otherwise ./<deck base name>-slides/.
func resolveAllOutputDir(out, file string) string {
	if out != "" {
		return out
	}
	return fmt.Sprintf("./%s-slides", deckBaseName(file))
}
