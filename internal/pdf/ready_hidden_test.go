package pdf

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/mxschmitt/playwright-go"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// hiddenReadyDeadline is how long this test gives a page to report ready.
// It is short because the two capture cases prove their point by running
// out, and a live page that works reports in well under a second.
const hiddenReadyDeadline = 5 * time.Second

// hiddenPageScript makes a page behave the way WebKit treats a window that
// is covered, minimized or on another space: document.visibilityState is
// "hidden" and animation frames never run. It is installed before the
// bundle loads, so the frontend sees a hidden page from its first render.
const hiddenPageScript = `
	Object.defineProperty(document, 'visibilityState', { configurable: true, get: () => 'hidden' });
	Object.defineProperty(document, 'hidden', { configurable: true, get: () => true });
	window.requestAnimationFrame = () => 0;
	window.cancelAnimationFrame = () => {};
`

// hiddenPageDeck is the smallest deck the viewer will render: one slide,
// no images, no deck components, so the only thing a ready cycle can be
// waiting on is a paint.
func hiddenPageDeck() *transformer.TransformedPresentation {
	return &transformer.TransformedPresentation{
		Config: *config.DefaultConfig(),
		Slides: []transformer.TransformedSlide{
			{Index: 0, Layout: "title", Slots: map[string]string{"default": "<h1>Hidden</h1>"}, SlotOrder: []string{"default"}},
		},
	}
}

// openHiddenPage serves a one-slide deck, opens query (for example
// "?print=true") in a page that reports itself hidden and runs no
// animation frames, and reports whether the page published the ready
// signal within hiddenReadyDeadline.
//
// It polls window.__tapReady itself rather than calling waitForReady:
// Playwright's own WaitForFunction polls on animation frames, which this
// page deliberately does not run, so only the page's own condition, the
// one internal/pdf/ready.go waits on, is being tested here.
func openHiddenPage(t *testing.T, exporter *Exporter, query string) bool {
	t.Helper()

	srv := server.New(0)
	srv.SetPresentation(hiddenPageDeck())
	srv.SetupRoutes()
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer func() { _ = srv.Shutdown(context.Background()) }()

	page, err := exporter.browser.NewPage()
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	defer func() { _ = page.Close() }()

	if err := page.AddInitScript(playwright.Script{Content: playwright.String(hiddenPageScript)}); err != nil {
		t.Fatalf("AddInitScript() error = %v", err)
	}
	if _, err := page.Goto("http://localhost:" + itoa(srv.Port()) + query + "#1"); err != nil {
		t.Fatalf("Goto() error = %v", err)
	}

	deadline := time.Now().Add(hiddenReadyDeadline)
	for time.Now().Before(deadline) {
		reported, err := page.Evaluate(`() => window.__tapReady != null && window.__tapReady.slide === 1`)
		if err != nil {
			t.Fatalf("Evaluate() error = %v", err)
		}
		if ready, ok := reported.(bool); ok && ready {
			return true
		}
		time.Sleep(100 * time.Millisecond)
	}
	return false
}

// TestReadyOnAHiddenPage runs the built frontend (embedded/dist, so `make
// frontend` is part of this test's setup) on a page that reports itself
// hidden and never runs an animation frame. A live page must still report
// ready, since the desktop app asks only whether the DOM caught up with an
// edit. A capture page must not, since a screenshot of a page that has not
// painted comes out blank.
func TestReadyOnAHiddenPage(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}
	exporter, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer exporter.Close()
	if err := exporter.EnsureBrowser(); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("browser unavailable in CI: %v", err)
		}
		t.Skipf("skipping: no browser available: %v", err)
	}
	if err := exporter.launchBrowser(); err != nil {
		t.Fatalf("launchBrowser() error = %v", err)
	}

	t.Run("a live page reports ready while it is hidden", func(t *testing.T) {
		if !openHiddenPage(t, exporter, "") {
			t.Error("a hidden live page never reported ready; it is waiting for a paint that a hidden window never makes")
		}
	})

	t.Run("a print page still waits for a paint", func(t *testing.T) {
		if openHiddenPage(t, exporter, "?print=true") {
			t.Error("a print page reported ready without painting; a PDF captured from it would be blank")
		}
	})

	t.Run("a capture page still waits for a paint", func(t *testing.T) {
		if openHiddenPage(t, exporter, "?capture=true&step=0") {
			t.Error("a capture page reported ready without painting; a PNG captured from it would be blank")
		}
	})
}
