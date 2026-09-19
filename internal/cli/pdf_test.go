package cli

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/pdf"
)

// TestPrepareDeck_ServesComponentBundles checks that the server prepareDeck
// starts (see internal/cli/deck.go) actually serves every component bundle
// file the deck's components resolve to, at status 200 - the bug this test
// guards against is tap pdf never registering those bundles on its
// temporary server, so a PDF's component slides showed a "component not
// resolved" error card even though the components built fine. Needs no
// browser, so it runs unconditionally.
func TestPrepareDeck_ServesComponentBundles(t *testing.T) {
	deckPath, err := filepath.Abs(filepath.Join("..", "..", "examples", "components", "deck.md"))
	if err != nil {
		t.Fatalf("failed to resolve deck path: %v", err)
	}
	baseDir := filepath.Dir(deckPath)

	cfg, err := config.Load(deckPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	// loadPresentation is called a second time here, alongside prepareDeck,
	// only to get at the resolved component map so the test knows which
	// bundle file names to expect; prepareDeck itself is what the fix
	// exercises.
	_, _, resolvedComponents, componentErrs, err := loadPresentation(deckPath, cfg, baseDir)
	if err != nil {
		t.Fatalf("failed to load presentation: %v", err)
	}
	if len(componentErrs) > 0 {
		t.Fatalf("unexpected component build errors: %v", componentErrs)
	}
	bundleFiles := componentBundleFiles(resolvedComponents)
	if len(bundleFiles) == 0 {
		t.Fatal("expected the components example deck to resolve at least one bundle file")
	}

	srv, _, _, prepareErrs, _, err := prepareDeck(deckPath, cfg, baseDir)
	if err != nil {
		t.Fatalf("prepareDeck() error = %v", err)
	}
	if len(prepareErrs) > 0 {
		t.Fatalf("unexpected component build errors from prepareDeck: %v", prepareErrs)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	serverURL := "http://localhost:" + strconv.Itoa(srv.Port())

	for name := range bundleFiles {
		resp, err := http.Get(serverURL + "/components/" + name)
		if err != nil {
			t.Fatalf("GET /components/%s error = %v", name, err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("GET /components/%s status = %d, want 200", name, resp.StatusCode)
		}
	}
}

// TestPDFExportIntegration exports the components example deck to a temp
// folder and asserts it succeeds and writes a non-empty file. Requires the
// Playwright browser; skipped in short mode and when the browser isn't
// installed, following the screenshot integration tests in
// internal/cli/screenshot_test.go.
func TestPDFExportIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	deckPath, err := filepath.Abs(filepath.Join("..", "..", "examples", "components", "deck.md"))
	if err != nil {
		t.Fatalf("failed to resolve deck path: %v", err)
	}
	baseDir := filepath.Dir(deckPath)

	cfg, err := config.Load(deckPath)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	srv, _, _, componentErrs, _, err := prepareDeck(deckPath, cfg, baseDir)
	if err != nil {
		t.Fatalf("prepareDeck() error = %v", err)
	}
	if len(componentErrs) > 0 {
		t.Fatalf("unexpected component build errors: %v", componentErrs)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	serverURL := "http://localhost:" + strconv.Itoa(srv.Port())

	exporter, err := pdf.New()
	if err != nil {
		t.Fatalf("pdf.New() error = %v", err)
	}
	defer func() { _ = exporter.Close() }()

	if err := exporter.EnsureBrowser(); err != nil {
		t.Skipf("skipping: no browser available: %v", err)
	}

	outputPath := filepath.Join(t.TempDir(), "deck.pdf")

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	if _, err := exporter.Export(ctx, serverURL, pdf.ExportOptions{
		Content: pdf.ContentSlides,
		Output:  outputPath,
	}); err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	info, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("failed to stat exported PDF: %v", err)
	}
	if info.Size() == 0 {
		t.Error("exported PDF is empty")
	}
}
