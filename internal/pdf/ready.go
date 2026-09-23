package pdf

import (
	"fmt"
	"time"

	"github.com/mxschmitt/playwright-go"
)

// readyTimeout is how long a slide may take to settle before an export
// gives up on it. A variable so tests can shorten it.
var readyTimeout = 30 * time.Second

// waitForReady waits until the page reports that slideNumber (1-based) has
// settled. The frontend owns this decision: it sets window.__tapReady to
// {revision, slide, step} once fonts, images, maps, deck components, error
// cards, transitions and theme animations are done (see
// frontend/src/lib/ready/readySignal.ts). Tap Desktop's thumbnail renderer
// waits for the same signal.
func waitForReady(page playwright.Page, slideNumber int) error {
	_, err := page.WaitForFunction(
		`(slide) => window.__tapReady != null && window.__tapReady.slide === slide`,
		slideNumber,
		playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(float64(readyTimeout.Milliseconds()))},
	)
	if err != nil {
		return fmt.Errorf("slide %d did not finish rendering within %s: %w", slideNumber, readyTimeout, err)
	}
	return nil
}
