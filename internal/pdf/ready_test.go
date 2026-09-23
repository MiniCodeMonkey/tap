package pdf

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestWaitForReady(t *testing.T) {
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

	page, err := exporter.browser.NewPage()
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	defer page.Close()

	// The page reports slide 1 first and slide 3 later, as a page does
	// after a hash navigation. Only slide 3 may end the wait.
	if err := page.SetContent(`<script>
		setTimeout(() => {
			window.__tapReady = { revision: "", slide: 1, step: 0 };
			setTimeout(() => { window.__tapReady = { revision: "", slide: 3, step: 0 }; }, 100);
		}, 100);
	</script>`); err != nil {
		t.Fatalf("SetContent() error = %v", err)
	}
	if err := waitForReady(page, 3); err != nil {
		t.Fatalf("waitForReady(3) error = %v", err)
	}

	previousTimeout := readyTimeout
	readyTimeout = 300 * time.Millisecond
	defer func() { readyTimeout = previousTimeout }()

	err = waitForReady(page, 4)
	if err == nil || !strings.Contains(err.Error(), "slide 4 did not finish rendering") {
		t.Errorf("waitForReady(4) error = %v, want a timeout that names slide 4", err)
	}
}
