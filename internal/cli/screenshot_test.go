package cli

import (
	"bytes"
	"context"
	"fmt"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/pdf"
	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

func TestValidateScreenshotFlags(t *testing.T) {
	tests := []struct {
		name                            string
		all, hasSlide, hasStep, hasFrag bool
		width                           int
		wantErr                         bool
	}{
		{"slide alone is fine", false, true, false, false, 1920, false},
		{"all alone is fine", true, false, false, false, 1920, false},
		{"neither slide nor all", false, false, false, false, 1920, true},
		{"both slide and all", true, true, false, false, 1920, true},
		{"step with all", true, false, true, false, 1920, true},
		{"fragment with all", true, false, false, true, 1920, true},
		{"step and fragment with slide", false, true, true, true, 1920, false},
		{"zero width", false, true, false, false, 0, true},
		{"negative width", false, true, false, false, -10, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateScreenshotFlags(tt.all, tt.hasSlide, tt.hasStep, tt.hasFrag, tt.width)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateScreenshotFlags() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateWaitFlag(t *testing.T) {
	tests := []struct {
		name    string
		waitMS  int
		wantErr bool
	}{
		{"zero (the default, no wait) is fine", 0, false},
		{"a typical wait is fine", 400, false},
		{"the maximum, 60000ms, is fine", 60000, false},
		{"negative is out of range", -1, true},
		{"over the maximum is out of range", 60001, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateWaitFlag(tt.waitMS)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateWaitFlag(%d) error = %v, wantErr %v", tt.waitMS, err, tt.wantErr)
			}
		})
	}
}

func TestSlideOutOfRangeError(t *testing.T) {
	err := slideOutOfRangeError(12, 5)
	want := "slide 12 is out of range: deck has 5 slide(s)"
	if err.Error() != want {
		t.Errorf("got %q, want %q", err.Error(), want)
	}
}

func TestUnknownThemeError(t *testing.T) {
	err := unknownThemeError("not-a-theme")
	if err == nil {
		t.Fatal("expected a non-nil error")
	}
	// Every valid theme slug should be listed, including the always-present
	// base theme.
	if !contains(err.Error(), "base") {
		t.Errorf("expected error to list valid themes, got %q", err.Error())
	}
}

func TestValidateStepAndFragment(t *testing.T) {
	slide := transformer.TransformedSlide{Steps: 3, FragmentCount: 4}

	tests := []struct {
		name        string
		hasStep     bool
		step        int
		hasFragment bool
		fragment    int
		wantErr     bool
	}{
		{"step within range", true, 2, false, 0, false},
		{"step at zero", true, 0, false, 0, false},
		{"step at max", true, 3, false, 0, false},
		{"step below zero", true, -1, false, 0, true},
		{"step above max", true, 4, false, 0, true},
		{"no step given, never checked", false, 99, false, 0, false},
		{"fragment within range", false, 0, true, 2, false},
		{"fragment at -1 (none revealed)", false, 0, true, -1, false},
		{"fragment at max", false, 0, true, 3, false},
		{"fragment below -1", false, 0, true, -2, true},
		{"fragment above max", false, 0, true, 4, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStepAndFragment(tt.hasStep, tt.step, tt.hasFragment, tt.fragment, slide, 1)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateStepAndFragment() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestResolveDimensions(t *testing.T) {
	tests := []struct {
		ratio      string
		width      int
		wantWidth  int
		wantHeight int
		wantErr    bool
	}{
		{"16:9", 1920, 1920, 1080, false},
		{"4:3", 1920, 1920, 1440, false},
		{"16:10", 1920, 1920, 1200, false},
		{"garbage", 1920, 0, 0, true},
		{"16:0", 1920, 0, 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.ratio, func(t *testing.T) {
			w, h, err := resolveDimensions(tt.ratio, tt.width)
			if (err != nil) != tt.wantErr {
				t.Fatalf("resolveDimensions() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err != nil {
				return
			}
			if w != tt.wantWidth || h != tt.wantHeight {
				t.Errorf("resolveDimensions() = (%d, %d), want (%d, %d)", w, h, tt.wantWidth, tt.wantHeight)
			}
		})
	}
}

func TestDeckBaseName(t *testing.T) {
	if got := deckBaseName("/a/b/deck.md"); got != "deck" {
		t.Errorf("deckBaseName() = %q, want %q", got, "deck")
	}
	if got := deckBaseName("deck.md"); got != "deck" {
		t.Errorf("deckBaseName() = %q, want %q", got, "deck")
	}
}

func TestResolveAllOutputDir(t *testing.T) {
	if got := resolveAllOutputDir("custom", "deck.md"); got != "custom" {
		t.Errorf("resolveAllOutputDir() = %q, want %q", got, "custom")
	}
	if got := resolveAllOutputDir("", "deck.md"); got != "./deck-slides" {
		t.Errorf("resolveAllOutputDir() = %q, want %q", got, "./deck-slides")
	}
}

// TestCaptureAllSlides_ContinuesPastBrokenSlide checks the --all loop
// against a fake captureFunc: a broken slide (a capture error, or one
// whose render shows an error card) is collected, not fatal, so the rest
// of the deck's slides still get captured and written. Cheap - no browser,
// no server - unlike the end-to-end coverage in
// TestScreenshotCommand_StdoutStderrSeparation.
func TestCaptureAllSlides_ContinuesPastBrokenSlide(t *testing.T) {
	tempDir := t.TempDir()

	var calls []int
	fakeCapture := func(ctx context.Context, serverURL string, opts pdf.CaptureOptions, outputPath string) error {
		calls = append(calls, opts.SlideNumber)
		if opts.SlideNumber == 2 {
			return fmt.Errorf("slide %d shows an error card: boom", opts.SlideNumber)
		}
		return os.WriteFile(outputPath, []byte("fake png"), 0644)
	}

	written, broken, err := captureAllSlides(context.Background(), fakeCapture, "http://localhost:0", 4, 1920, 1080, "", tempDir)
	if err != nil {
		t.Fatalf("captureAllSlides() error = %v", err)
	}
	if len(calls) != 4 {
		t.Errorf("expected capture to be tried for every slide, got %d calls: %v", len(calls), calls)
	}

	wantWritten := []string{
		filepath.Join(tempDir, "slide-001.png"),
		filepath.Join(tempDir, "slide-003.png"),
		filepath.Join(tempDir, "slide-004.png"),
	}
	if len(written) != len(wantWritten) {
		t.Fatalf("written = %v, want %v", written, wantWritten)
	}
	for i, want := range wantWritten {
		if written[i] != want {
			t.Errorf("written[%d] = %q, want %q", i, written[i], want)
		}
	}

	if len(broken) != 1 {
		t.Fatalf("expected 1 broken slide, got %d: %+v", len(broken), broken)
	}
	if broken[0].SlideNumber != 2 {
		t.Errorf("broken slide number = %d, want 2", broken[0].SlideNumber)
	}
	if !contains(broken[0].Reason, "error card") {
		t.Errorf("broken slide reason = %q, want it to mention the error card", broken[0].Reason)
	}
}

// TestCaptureAllSlides_MkdirFailureIsFatal checks that a failure before
// the loop can even start (the output directory can't be created) is
// returned as a fatal error, not folded into the broken-slide list.
func TestCaptureAllSlides_MkdirFailureIsFatal(t *testing.T) {
	// A regular file can't be MkdirAll'd into, so this always fails.
	blockingFile := filepath.Join(t.TempDir(), "not-a-directory")
	if err := os.WriteFile(blockingFile, []byte("x"), 0644); err != nil {
		t.Fatalf("failed to create blocking file: %v", err)
	}

	fakeCapture := func(ctx context.Context, serverURL string, opts pdf.CaptureOptions, outputPath string) error {
		t.Fatal("capture should never be called when the output directory can't be created")
		return nil
	}

	_, _, err := captureAllSlides(context.Background(), fakeCapture, "http://localhost:0", 2, 1920, 1080, "", filepath.Join(blockingFile, "slides"))
	if err == nil {
		t.Fatal("expected an error when the output directory can't be created")
	}
}

// TestCaptureAllSlides_CancelledContextStopsEvenWithoutWrappingCanceled
// reproduces the Ctrl-C bug: a real process-group signal often kills the
// headless browser before capture returns, so capture fails with its own
// error (here a stand-in for "target closed") rather than one that wraps
// context.Canceled. The loop must still stop instead of folding this into
// the broken-slide list, because ctx itself - not the error's shape - is
// what decides whether this was an interruption.
func TestCaptureAllSlides_CancelledContextStopsEvenWithoutWrappingCanceled(t *testing.T) {
	tempDir := t.TempDir()
	ctx, cancel := context.WithCancel(context.Background())

	var calls []int
	fakeCapture := func(ctx context.Context, serverURL string, opts pdf.CaptureOptions, outputPath string) error {
		calls = append(calls, opts.SlideNumber)
		if opts.SlideNumber == 2 {
			cancel()
			return fmt.Errorf("target closed")
		}
		return os.WriteFile(outputPath, []byte("fake png"), 0644)
	}

	written, broken, err := captureAllSlides(ctx, fakeCapture, "http://localhost:0", 4, 1920, 1080, "", tempDir)
	if err == nil {
		t.Fatal("expected an error when the context is cancelled mid-loop")
	}
	if len(calls) != 2 {
		t.Errorf("expected the loop to stop after the cancelled slide, got %d calls: %v", len(calls), calls)
	}
	if len(broken) != 0 {
		t.Errorf("expected no broken slides recorded, got %+v (a cancellation is not a broken slide)", broken)
	}
	if len(written) != 1 {
		t.Errorf("expected only slide 1 written, got %v", written)
	}
}

func contains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (func() bool {
		for i := 0; i+len(needle) <= len(haystack); i++ {
			if haystack[i:i+len(needle)] == needle {
				return true
			}
		}
		return false
	})()
}

// TestScreenshotIntegration renders slide 1 of testdata/sample.md to a temp
// folder and checks the PNG's dimensions and that two different slides
// produce different bytes. It requires the Playwright browser; skipped in
// short mode, following the existing PDF integration tests
// (internal/pdf/exporter_test.go).
//
// This only exercises the final-state (print mode) path: --step/--fragment
// depend on presentation.ts reading ?step=/?fragment=, a change to
// frontend source that the embedded, pre-built frontend
// (embedded/dist, embedded by another in-progress task) does not yet
// contain. See the frontend unit tests in
// frontend/src/lib/stores/presentation.test.ts for that behavior.
func TestScreenshotIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	sampleDeck, err := filepath.Abs(filepath.Join("..", "..", "testdata", "sample.md"))
	if err != nil {
		t.Fatalf("failed to resolve sample deck path: %v", err)
	}
	baseDir := filepath.Dir(sampleDeck)

	cfg, err := config.Load(sampleDeck)
	if err != nil {
		t.Fatalf("failed to load config: %v", err)
	}

	pres, _, resolvedComponents, componentErrs, _, err := loadPresentation(sampleDeck, cfg, baseDir)
	if err != nil {
		t.Fatalf("failed to load presentation: %v", err)
	}
	if len(componentErrs) > 0 {
		t.Fatalf("unexpected component build errors: %v", componentErrs)
	}
	if len(pres.Slides) < 2 {
		t.Fatalf("sample deck needs at least 2 slides for this test, has %d", len(pres.Slides))
	}

	srv := server.New(0)
	srv.SetPresentation(pres)
	srv.SetBaseDir(baseDir)
	srv.SetComponentBundles(componentBundleFiles(resolvedComponents))
	srv.SetupRoutes()
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
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

	tempDir, err := os.MkdirTemp("", "tap-screenshot-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	width, height, err := resolveDimensions(cfg.AspectRatio, 1920)
	if err != nil {
		t.Fatalf("resolveDimensions() error = %v", err)
	}

	slide1Path := filepath.Join(tempDir, "slide-1.png")
	if err := exporter.CaptureSlide(context.Background(), serverURL, pdf.CaptureOptions{
		SlideNumber: 1,
		Width:       width,
		Height:      height,
		Print:       true,
	}, slide1Path); err != nil {
		t.Fatalf("CaptureSlide() error = %v", err)
	}

	f, err := os.Open(slide1Path)
	if err != nil {
		t.Fatalf("failed to open captured PNG: %v", err)
	}
	img, err := png.Decode(f)
	f.Close()
	if err != nil {
		t.Fatalf("failed to decode PNG: %v", err)
	}
	bounds := img.Bounds()
	if bounds.Dx() != 1920 || bounds.Dy() != 1080 {
		t.Errorf("slide 1 dimensions = %dx%d, want 1920x1080", bounds.Dx(), bounds.Dy())
	}

	slide2Path := filepath.Join(tempDir, "slide-2.png")
	if err := exporter.CaptureSlide(context.Background(), serverURL, pdf.CaptureOptions{
		SlideNumber: 2,
		Width:       width,
		Height:      height,
		Print:       true,
	}, slide2Path); err != nil {
		t.Fatalf("CaptureSlide() error = %v", err)
	}

	bytes1, err := os.ReadFile(slide1Path)
	if err != nil {
		t.Fatalf("failed to read slide 1: %v", err)
	}
	bytes2, err := os.ReadFile(slide2Path)
	if err != nil {
		t.Fatalf("failed to read slide 2: %v", err)
	}
	if string(bytes1) == string(bytes2) {
		t.Error("expected slide 1 and slide 2 screenshots to differ, got identical bytes")
	}
}

// TestScreenshotCommand_StdoutStderrSeparation runs the real tap binary as
// a subprocess, so fatih/color's Error writer (bound once, at process
// start, to the real file descriptor 2) is genuinely redirected the way it
// would be for `tap screenshot ... 2>/dev/null` - reassigning the
// in-process os.Stderr *variable* from inside this test package would not
// do that, since color.Error already holds its own reference to the
// original stderr file. Checks that a failure prints nothing to standard
// output and its message to standard error, and that a success prints
// only the written path to standard output, nothing to standard error.
// Requires the Playwright browser for the success case; skipped in short
// mode like the other integration tests in this file.
func TestScreenshotCommand_StdoutStderrSeparation(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	binary := buildTapBinaryForTest(t)

	t.Run("failure prints only to standard error", func(t *testing.T) {
		var stdout, stderr bytes.Buffer
		cmd := exec.Command(binary, "screenshot", "does-not-exist.md", "--slide", "1")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err == nil {
			t.Fatal("expected the command to exit non-zero")
		}
		if stdout.String() != "" {
			t.Errorf("expected empty standard output, got %q", stdout.String())
		}
		if !strings.Contains(stderr.String(), "deck not found") {
			t.Errorf("expected standard error to mention the failure, got %q", stderr.String())
		}
	})

	t.Run("success prints only the written path to standard output", func(t *testing.T) {
		exporter, err := pdf.New()
		if err != nil {
			t.Fatalf("pdf.New() error = %v", err)
		}
		requireBrowser(t, exporter)
		_ = exporter.Close()

		sampleDeck, err := filepath.Abs(filepath.Join("..", "..", "testdata", "sample.md"))
		if err != nil {
			t.Fatalf("failed to resolve sample deck path: %v", err)
		}
		outputPath := filepath.Join(t.TempDir(), "slide-1.png")

		var stdout, stderr bytes.Buffer
		cmd := exec.Command(binary, "screenshot", sampleDeck, "--slide", "1", "--out", outputPath)
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		if err := cmd.Run(); err != nil {
			t.Fatalf("tap screenshot failed: %v\nstderr: %s", err, stderr.String())
		}
		if got := strings.TrimRight(stdout.String(), "\n"); got != outputPath {
			t.Errorf("standard output = %q, want %q", got, outputPath)
		}
		if stderr.String() != "" {
			t.Errorf("expected empty standard error, got %q", stderr.String())
		}
	})
}

// tapBinaryForTestOnce, tapBinaryForTestPath, and tapBinaryForTestErr back
// buildTapBinaryForTest: every caller across the package's subprocess
// tests shares the one binary this builds, instead of each running its
// own "go build". It cannot live in a t.TempDir(), since that directory is
// removed when the test that created it finishes, before a later test's
// call would still need the binary.
var (
	tapBinaryForTestOnce sync.Once
	tapBinaryForTestPath string
	tapBinaryForTestErr  error
)

// buildTapBinaryForTest builds the tap binary once, the first time any
// test calls it, and returns that same binary's path to every caller.
func buildTapBinaryForTest(t *testing.T) string {
	t.Helper()

	tapBinaryForTestOnce.Do(func() {
		repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
		if err != nil {
			tapBinaryForTestErr = fmt.Errorf("failed to resolve repo root: %w", err)
			return
		}

		dir, err := os.MkdirTemp("", "tap-test-binary-*")
		if err != nil {
			tapBinaryForTestErr = fmt.Errorf("failed to create temp dir: %w", err)
			return
		}

		binary := filepath.Join(dir, "tap-test-binary")
		cmd := exec.Command("go", "build", "-o", binary, "./cmd/tap")
		cmd.Dir = repoRoot
		if out, err := cmd.CombinedOutput(); err != nil {
			tapBinaryForTestErr = fmt.Errorf("failed to build tap binary: %w\n%s", err, out)
			return
		}
		tapBinaryForTestPath = binary
	})

	if tapBinaryForTestErr != nil {
		t.Fatalf("%v", tapBinaryForTestErr)
	}
	return tapBinaryForTestPath
}

// TestScreenshotIntegration_RollingDeployStepsDiffer captures the example
// deck's whole-slide RollingDeploy component at step 0 and at its final
// step and checks the two PNGs differ: proof that --step actually renders
// a different presenter state rather than always falling back to the
// final one. Requires the Playwright browser; skipped in short mode and
// when the browser isn't installed, following TestScreenshotIntegration
// above.
func TestScreenshotIntegration_RollingDeployStepsDiffer(t *testing.T) {
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

	pres, _, resolvedComponents, componentErrs, _, err := loadPresentation(deckPath, cfg, baseDir)
	if err != nil {
		t.Fatalf("failed to load presentation: %v", err)
	}
	if len(componentErrs) > 0 {
		t.Fatalf("unexpected component build errors: %v", componentErrs)
	}

	const rollingDeploySlide = 3

	srv := server.New(0)
	srv.SetPresentation(pres)
	srv.SetBaseDir(baseDir)
	srv.SetComponentBundles(componentBundleFiles(resolvedComponents))
	srv.SetupRoutes()
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
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

	tempDir := t.TempDir()

	width, height, err := resolveDimensions(cfg.AspectRatio, 1920)
	if err != nil {
		t.Fatalf("resolveDimensions() error = %v", err)
	}

	stepZero := 0
	step0Path := filepath.Join(tempDir, "rolling-step-0.png")
	if err := exporter.CaptureSlide(context.Background(), serverURL, pdf.CaptureOptions{
		SlideNumber: rollingDeploySlide,
		Width:       width,
		Height:      height,
		Step:        &stepZero,
	}, step0Path); err != nil {
		t.Fatalf("CaptureSlide() at step 0 error = %v", err)
	}

	finalPath := filepath.Join(tempDir, "rolling-final.png")
	if err := exporter.CaptureSlide(context.Background(), serverURL, pdf.CaptureOptions{
		SlideNumber: rollingDeploySlide,
		Width:       width,
		Height:      height,
		Print:       true,
	}, finalPath); err != nil {
		t.Fatalf("CaptureSlide() at the final state error = %v", err)
	}

	step0Bytes, err := os.ReadFile(step0Path)
	if err != nil {
		t.Fatalf("failed to read step 0 PNG: %v", err)
	}
	finalBytes, err := os.ReadFile(finalPath)
	if err != nil {
		t.Fatalf("failed to read final-state PNG: %v", err)
	}
	if bytes.Equal(step0Bytes, finalBytes) {
		t.Error("expected step 0 and the final state to produce different PNG bytes, got identical images")
	}
}
