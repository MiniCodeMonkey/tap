package pdf

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/mxschmitt/playwright-go"
)

// ErrorCardSelector matches the elements that mark a slide as failed to
// render: the slide error boundary's card (see
// frontend/src/lib/components/SlideErrorBoundary.tsx, class "slide-error")
// and a deck component's build/render error card (class "deck-error-card",
// the name agreed with the deck components work for DeckComponent.tsx).
// Detecting both covers a slide-level React error and a component-level
// one, whichever the frontend renders.
const ErrorCardSelector = ".slide-error, .deck-error-card"

// CaptureOptions configures a single-slide screenshot capture.
type CaptureOptions struct {
	// SlideNumber is the one-based slide to capture.
	SlideNumber int
	// Width and Height are the viewport size in pixels.
	Width, Height int
	// Print renders the slide's final state through print mode
	// (?print=true), the same URL form tap pdf uses. Set this or Step/
	// Fragment, not both: print mode always shows the final step and
	// fragment state, so it overrides them.
	Print bool
	// Step, when set, renders that exact presenter step (?step=N) without
	// print mode.
	Step *int
	// Fragment, when set, renders that exact fragment index
	// (?fragment=N) without print mode.
	Fragment *int
	// Theme, if non-empty, selects a theme through ?theme=<slug>.
	Theme string
	// WaitMS, when greater than 0, sleeps this many milliseconds after the
	// existing readiness waits (network idle, images, maps, fonts, CSS
	// animations) before capturing - for the rare case where a moment
	// mid-animation is wanted rather than the settled state. A capture with
	// WaitMS > 0 stays live (?live=true): print/settled semantics never
	// apply, since waiting only makes sense for something still running.
	WaitMS int
}

// buildSlideURL builds the URL for a single slide capture from the state
// rules in CaptureOptions: print mode's query parameter, or the step/
// fragment query parameters read by presentation.ts's initializeFromURL,
// plus an optional theme override, and the slide's one-based URL hash.
func buildSlideURL(serverURL string, options CaptureOptions) string {
	query := url.Values{}
	if options.Print {
		query.Set("print", "true")
	}
	if options.Step != nil {
		query.Set("step", strconv.Itoa(*options.Step))
	}
	if options.Fragment != nil {
		query.Set("fragment", strconv.Itoa(*options.Fragment))
	}
	if options.Step != nil || options.Fragment != nil || options.WaitMS > 0 {
		// A stepped or fragment capture, or a live (WaitMS > 0) one, opens
		// the live, non-print viewer to render an exact presenter state,
		// but it is still a capture: the temporary server this runs
		// against never serves the websocket route (see
		// cli.runScreenshotE), so the frontend must never try to connect it
		// or show the connection badge. See capture=true's handling next to
		// App.tsx's PRINT_MODE/CAPTURE_MODE read.
		query.Set("capture", "true")
	}
	if options.WaitMS > 0 {
		// Without --wait, a capture settles (App.tsx's SETTLE): no
		// component or theme animation is still running by the time the
		// screenshot is taken. --wait exists specifically to catch one
		// still running, so it must skip that settling - see CAPTURE_LIVE
		// next to App.tsx's SETTLE.
		query.Set("live", "true")
	}
	if options.Theme != "" {
		query.Set("theme", options.Theme)
	}

	target := serverURL
	if encoded := query.Encode(); encoded != "" {
		target += "?" + encoded
	}
	return fmt.Sprintf("%s#%d", target, options.SlideNumber)
}

// CaptureSlide renders one slide state, per options, and writes it as a PNG to
// outputPath. It waits for the page to settle - network idle, images, map
// tiles, web fonts, and any running CSS animations or transitions - before
// screenshotting, then fails with a descriptive error if the rendered slide
// shows a slide or component error card (see ErrorCardSelector). With
// options.WaitMS > 0, it sleeps that many extra milliseconds after all of
// the above, for a capture that deliberately wants a moment mid-animation
// rather than the settled state.
func (e *Exporter) CaptureSlide(serverURL string, options CaptureOptions, outputPath string) error {
	if err := e.launchBrowser(); err != nil {
		return err
	}

	page, err := e.browser.NewPage(playwright.BrowserNewPageOptions{
		Viewport: &playwright.Size{
			Width:  options.Width,
			Height: options.Height,
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create new page: %w", err)
	}
	defer page.Close()

	targetURL := buildSlideURL(serverURL, options)
	if _, err := page.Goto(targetURL, playwright.PageGotoOptions{
		WaitUntil: playwright.WaitUntilStateDomcontentloaded,
	}); err != nil {
		return fmt.Errorf("failed to navigate to slide %d: %w", options.SlideNumber, err)
	}

	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		return fmt.Errorf("failed to wait for slide %d to load: %w", options.SlideNumber, err)
	}

	if err := e.waitForImages(page); err != nil {
		return fmt.Errorf("failed to wait for images on slide %d: %w", options.SlideNumber, err)
	}
	if err := e.waitForMaps(page); err != nil {
		return fmt.Errorf("failed to wait for maps on slide %d: %w", options.SlideNumber, err)
	}
	if err := waitForFonts(page); err != nil {
		return fmt.Errorf("failed to wait for fonts on slide %d: %w", options.SlideNumber, err)
	}
	if err := waitForAnimations(page); err != nil {
		return fmt.Errorf("failed to wait for animations on slide %d: %w", options.SlideNumber, err)
	}

	if options.WaitMS > 0 {
		// Applied after every readiness wait above, not instead of them: a
		// live capture (see buildSlideURL) still wants network, images,
		// maps and fonts settled - only the "is anything still animating"
		// part is skipped, by design, since this is the one case where an
		// answer of "yes, still animating" is the point.
		if _, err := page.Evaluate(fmt.Sprintf(`() => new Promise((resolve) => setTimeout(resolve, %d))`, options.WaitMS)); err != nil {
			return fmt.Errorf("failed to wait %dms on slide %d: %w", options.WaitMS, options.SlideNumber, err)
		}
	}

	if message, hasError := detectErrorCard(page); hasError {
		return fmt.Errorf("slide %d shows an error card: %s", options.SlideNumber, message)
	}

	if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create output directory: %w", err)
		}
	}

	if _, err := page.Screenshot(playwright.PageScreenshotOptions{
		Path: playwright.String(outputPath),
		Type: playwright.ScreenshotTypePng,
	}); err != nil {
		return fmt.Errorf("failed to capture slide %d: %w", options.SlideNumber, err)
	}

	return nil
}

// waitForFonts waits for every web font used on the page to finish
// loading, via the CSS Font Loading API's document.fonts.ready promise.
func waitForFonts(page playwright.Page) error {
	_, err := page.Evaluate(`() => {
		if (!document.fonts || !document.fonts.ready) {
			return Promise.resolve();
		}
		return document.fonts.ready;
	}`)
	return err
}

// waitForAnimations waits for every running CSS animation and transition on
// the page to finish, via the Web Animations API's Animation.finished
// promises, with a timeout so a deliberately infinite animation (a looping
// spinner, for example) cannot hang a capture forever. Two animation frames
// are awaited afterward so the final frame has actually painted before the
// screenshot is taken.
func waitForAnimations(page playwright.Page) error {
	_, err := page.Evaluate(`() => {
		return new Promise((resolve) => {
			const settle = () => {
				requestAnimationFrame(() => requestAnimationFrame(resolve));
			};
			const timeout = setTimeout(settle, 3000);
			const animations = document.getAnimations ? document.getAnimations() : [];
			if (animations.length === 0) {
				clearTimeout(timeout);
				settle();
				return;
			}
			Promise.allSettled(animations.map((animation) => animation.finished)).then(() => {
				clearTimeout(timeout);
				settle();
			});
		});
	}`)
	return err
}

// detectErrorCard reports whether the rendered slide shows a slide or
// component error card (see ErrorCardSelector), and that card's text when
// it does.
func detectErrorCard(page playwright.Page) (message string, hasError bool) {
	result, err := page.Evaluate(fmt.Sprintf(`() => {
		const el = document.querySelector(%q);
		return el ? (el.textContent || '').trim() : null;
	}`, ErrorCardSelector))
	if err != nil || result == nil {
		return "", false
	}
	text, ok := result.(string)
	if !ok || text == "" {
		return "error card detected", true
	}
	return text, true
}

// EnsureBrowser makes sure the exporter's browser is running, launching it
// if necessary. Exported so callers outside this package (tap screenshot)
// can surface the exact "browser cannot start" error tap pdf gives, and
// fail fast before doing any other work.
func (e *Exporter) EnsureBrowser() error {
	return e.launchBrowser()
}
