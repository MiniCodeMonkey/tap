package cli

import (
	"os"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/pdf"
)

// requireBrowser ensures the given exporter's browser is available for a
// test that needs to actually drive Chromium. Locally, where a machine may
// not have Playwright's browsers cached, it skips the test. In CI (the CI
// environment variable is set by GitHub Actions), it fails the test instead
// - these tests exist to prove the browser really runs, so a silent skip
// there would hide a broken pipeline.
func requireBrowser(t *testing.T, exporter *pdf.Exporter) {
	t.Helper()

	if err := exporter.EnsureBrowser(); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("browser unavailable in CI: %v", err)
		}
		t.Skipf("skipping: no browser available: %v", err)
	}
}
