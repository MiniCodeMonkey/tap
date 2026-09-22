package cli

import (
	"bufio"
	"bytes"
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/pdf"
	"github.com/creack/pty"
)

func TestExportPDFCommandShape(t *testing.T) {
	command, _, err := rootCmd.Find([]string{"export", "pdf"})
	if err != nil || command.Name() != "pdf" || command.Parent().Name() != "export" {
		t.Fatalf("tap export pdf not found: %v", err)
	}
	if command.Use != "pdf [deck]" {
		t.Errorf("Use = %q, want %q", command.Use, "pdf [deck]")
	}
	for name, shorthand := range map[string]string{"output": "o", "content": "", "json": ""} {
		flag := command.Flags().Lookup(name)
		if flag == nil {
			t.Errorf("missing --%s", name)
			continue
		}
		if flag.Shorthand != shorthand {
			t.Errorf("--%s shorthand = %q, want %q", name, flag.Shorthand, shorthand)
		}
	}
}

func TestExportPDFMissingDeckIsAUserError(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.md")
	exitCode, stdout, _ := runTap(t, "export", "pdf", missing, "--json")
	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
	}
	if !strings.Contains(stdout, `"code": "deck_not_found"`) {
		t.Errorf("stdout = %q, want a deck_not_found JSON error", stdout)
	}
}

// TestPrepareDeck_ServesComponentBundles checks that the server prepareDeck
// starts (see internal/cli/deck.go) actually serves every component bundle
// file the deck's components resolve to, at status 200 - the bug this test
// guards against is tap export pdf never registering those bundles on its
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
	_, _, resolvedComponents, componentErrs, _, err := loadPresentation(deckPath, cfg, baseDir)
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

	requireBrowser(t, exporter)

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

// TestExportPDFProgress_TerminalOutputIsAllJSON runs tap export pdf
// --progress json with a real pseudo-terminal attached as the process's
// standard error, the way a real terminal session looks. The spinner
// (see newSpinner in internal/cli/build.go) writes straight to os.Stderr,
// bypassing cmd.ErrOrStderr(), so a test that only captures the command's
// stderr into a buffer never sees them collide. Without the guard in
// runExportPDF that forces the spinner to think standard error is not a
// terminal while progress is enabled, this test fails with a line like
// "⠦ Generating PDF (this may take a moment){"phase":"render",...}"
// instead of a clean JSON line.
func TestExportPDFProgress_TerminalOutputIsAllJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	deckPath, err := filepath.Abs(filepath.Join("..", "..", "examples", "basic.md"))
	if err != nil {
		t.Fatalf("failed to resolve deck path: %v", err)
	}

	probe, err := pdf.New()
	if err != nil {
		t.Fatalf("pdf.New() error = %v", err)
	}
	requireBrowser(t, probe)
	_ = probe.Close()

	outputPath := filepath.Join(t.TempDir(), "basic.pdf")

	ptmx, pts, err := pty.Open()
	if err != nil {
		t.Fatalf("pty.Open() error = %v", err)
	}
	defer ptmx.Close()

	originalStderr := os.Stderr
	os.Stderr = pts
	t.Cleanup(func() { os.Stderr = originalStderr })

	captured := &bytes.Buffer{}
	readDone := make(chan struct{})
	go func() {
		defer close(readDone)
		_, _ = captured.ReadFrom(ptmx)
	}()

	var stdout bytes.Buffer
	exitCode := execute(rootCmd, []string{
		"export", "pdf", deckPath, "-o", outputPath, "--progress", "json",
	}, &stdout, pts)

	os.Stderr = originalStderr
	_ = pts.Close()
	<-readDone

	if exitCode != exitOK {
		t.Fatalf("exit code = %d, want %d; captured stderr:\n%s", exitCode, exitOK, captured.String())
	}

	scanner := bufio.NewScanner(strings.NewReader(captured.String()))
	sawProgressLine := false
	for scanner.Scan() {
		line := strings.TrimRight(scanner.Text(), "\r")
		if line == "" {
			continue
		}
		if !strings.HasPrefix(line, "{") {
			t.Errorf("stderr line is not pure JSON: %q", line)
		} else {
			sawProgressLine = true
		}
	}
	if !sawProgressLine {
		t.Error("expected at least one JSON progress line on stderr")
	}
}
