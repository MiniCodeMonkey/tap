package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/pdf"
	"github.com/MiniCodeMonkey/tap/internal/themes"
)

// fakeThemeImagePNG returns a valid, minimal one-pixel PNG. useFakeThemeRenderer
// writes this for its fake renders, so a cache hit's PNG validity check
// trusts them the same way it would trust a real render.
func fakeThemeImagePNG() []byte {
	pixel := image.NewRGBA(image.Rect(0, 0, 1, 1))
	pixel.Set(0, 0, color.RGBA{R: 1, G: 2, B: 3, A: 255})
	var buffer bytes.Buffer
	_ = png.Encode(&buffer, pixel)
	return buffer.Bytes()
}

// requireValidPNG fails the test unless content decodes as a PNG.
func requireValidPNG(t *testing.T, content []byte) {
	t.Helper()
	if _, err := png.Decode(bytes.NewReader(content)); err != nil {
		t.Errorf("content is not a valid PNG: %v", err)
	}
}

// useFakeThemeRenderer replaces the browser renderer and the cache folder
// for the rest of the test, sets a release version, and returns the
// number of renders so far and the cache root.
func useFakeThemeRenderer(t *testing.T) (renders *int, cacheRoot string) {
	t.Helper()
	count := 0
	cacheRoot = t.TempDir()
	originalRender, originalRoot, originalVersion := renderThemeImage, themeImageCacheRoot, Version
	renderThemeImage = func(ctx context.Context, theme themes.Theme, outputPath string) error {
		count++
		return os.WriteFile(outputPath, fakeThemeImagePNG(), 0o644)
	}
	themeImageCacheRoot = func() (string, error) { return cacheRoot, nil }
	Version = "9.9.9-test"
	t.Cleanup(func() {
		renderThemeImage, themeImageCacheRoot, Version = originalRender, originalRoot, originalVersion
	})
	return &count, cacheRoot
}

func TestThemeShowImageRendersOnceThenUsesTheCache(t *testing.T) {
	renders, cacheRoot := useFakeThemeRenderer(t)
	wantPath := filepath.Join(cacheRoot, "tap", "themes", "9.9.9-test", "terminal.png")

	exitCode, stdout, stderr := runTap(t, "theme", "show", "terminal", "--image")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if stdout != wantPath+"\n" {
		t.Errorf("stdout = %q, want %q", stdout, wantPath)
	}
	content, err := os.ReadFile(wantPath)
	if err != nil {
		t.Fatalf("reading cached image: %v", err)
	}
	requireValidPNG(t, content)

	exitCode, stdout, _ = runTap(t, "theme", "show", "terminal", "--image", "--json")
	if exitCode != exitOK {
		t.Fatalf("second run exit code = %d", exitCode)
	}
	var output struct {
		OK     bool   `json:"ok"`
		Slug   string `json:"slug"`
		Image  string `json:"image"`
		Cached bool   `json:"cached"`
	}
	if err := json.Unmarshal([]byte(stdout), &output); err != nil {
		t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
	}
	if !output.OK || output.Slug != "terminal" || output.Image != wantPath || !output.Cached {
		t.Errorf("output = %+v", output)
	}
	if *renders != 1 {
		t.Errorf("rendered %d times, want 1", *renders)
	}
	leftovers, _ := filepath.Glob(filepath.Join(filepath.Dir(wantPath), "*.partial*"))
	if len(leftovers) > 0 {
		t.Errorf("partial files left in the cache: %v", leftovers)
	}
}

func TestThemeShowImageOutputCopiesTheImage(t *testing.T) {
	useFakeThemeRenderer(t)
	output := filepath.Join(t.TempDir(), "preview.png")
	exitCode, stdout, _ := runTap(t, "theme", "show", "terminal", "--image", "-o", output)
	if exitCode != exitOK || stdout != output+"\n" {
		t.Errorf("(%d, %q), want (0, %q)", exitCode, stdout, output)
	}
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	requireValidPNG(t, content)
}

func TestThemeShowImageInADevBuildAlwaysRenders(t *testing.T) {
	renders, _ := useFakeThemeRenderer(t)
	Version = "dev"
	runTap(t, "theme", "show", "terminal", "--image")
	runTap(t, "theme", "show", "terminal", "--image")
	if *renders != 2 {
		t.Errorf("rendered %d times, want 2: a dev build must not trust the cache", *renders)
	}
}

func TestThemeShowImageNewVersionRendersAgain(t *testing.T) {
	renders, _ := useFakeThemeRenderer(t)
	runTap(t, "theme", "show", "terminal", "--image")
	Version = "9.9.10-test"
	runTap(t, "theme", "show", "terminal", "--image")
	if *renders != 2 {
		t.Errorf("rendered %d times, want 2", *renders)
	}
}

// TestThemeShowImageStillRendersWhenCacheDirIsUnusable covers a read-only
// or full cache directory: the cache is an optimisation, so the command
// must still render and still produce its image when it cannot be
// written, not fail outright.
func TestThemeShowImageStillRendersWhenCacheDirIsUnusable(t *testing.T) {
	_, cacheRoot := useFakeThemeRenderer(t)
	// Block the cache directory: put a plain file where "tap" would need
	// to be a directory, so nothing under it can ever be created.
	if err := os.WriteFile(filepath.Join(cacheRoot, "tap"), []byte("blocked"), 0o644); err != nil {
		t.Fatal(err)
	}

	output := filepath.Join(t.TempDir(), "preview.png")
	exitCode, stdout, stderr := runTap(t, "theme", "show", "terminal", "--image", "-o", output)
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if stdout != output+"\n" {
		t.Errorf("stdout = %q, want %q", stdout, output)
	}
	content, err := os.ReadFile(output)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	requireValidPNG(t, content)
}

// TestThemeShowImageIgnoresACorruptCacheEntry covers a corrupt or
// truncated file sitting at the cache path: it must not be trusted as a
// cache hit, and must be re-rendered and replaced.
func TestThemeShowImageIgnoresACorruptCacheEntry(t *testing.T) {
	_, cacheRoot := useFakeThemeRenderer(t)
	cachePath := filepath.Join(cacheRoot, "tap", "themes", "9.9.9-test", "terminal.png")

	tests := map[string][]byte{
		"truncated": []byte("\x89PNG\r\n\x1a\n"),
		"zeroed":    make([]byte, 4096),
	}
	for name, content := range tests {
		content := content
		t.Run(name, func(t *testing.T) {
			if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(cachePath, content, 0o644); err != nil {
				t.Fatal(err)
			}

			exitCode, stdout, stderr := runTap(t, "theme", "show", "terminal", "--image", "--json")
			if exitCode != exitOK {
				t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
			}
			var output struct {
				Cached bool `json:"cached"`
			}
			if err := json.Unmarshal([]byte(stdout), &output); err != nil {
				t.Fatalf("stdout is not JSON: %v\n%s", err, stdout)
			}
			if output.Cached {
				t.Errorf("a corrupt cache entry was trusted as a hit")
			}
			replaced, err := os.ReadFile(cachePath)
			if err != nil {
				t.Fatalf("reading replaced cache entry: %v", err)
			}
			if bytes.Equal(replaced, content) {
				t.Errorf("cache entry was not replaced")
			}
			requireValidPNG(t, replaced)
		})
	}
}

// TestThemeShowImageSweepsStalePartials covers a hard kill during a
// render: it leaves a *.partial*.png file behind with nothing to remove
// it, so the cache directory must sweep old ones on the next render,
// without touching a partial file recent enough that a live render might
// still own it.
func TestThemeShowImageSweepsStalePartials(t *testing.T) {
	_, cacheRoot := useFakeThemeRenderer(t)
	cacheDir := filepath.Join(cacheRoot, "tap", "themes", "9.9.9-test")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}

	stale := filepath.Join(cacheDir, "other.partial111.png")
	if err := os.WriteFile(stale, []byte("orphaned"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-2 * time.Hour)
	if err := os.Chtimes(stale, old, old); err != nil {
		t.Fatal(err)
	}

	fresh := filepath.Join(cacheDir, "other.partial222.png")
	if err := os.WriteFile(fresh, []byte("in progress"), 0o644); err != nil {
		t.Fatal(err)
	}

	exitCode, _, stderr := runTap(t, "theme", "show", "terminal", "--image")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}
	if _, err := os.Stat(stale); !os.IsNotExist(err) {
		t.Errorf("stale partial file was not swept: %v", err)
	}
	if _, err := os.Stat(fresh); err != nil {
		t.Errorf("a fresh partial file (possibly a live render) was swept: %v", err)
	}
}

func TestThemeShowImageFlagErrors(t *testing.T) {
	useFakeThemeRenderer(t)
	tests := [][]string{
		{"theme", "show", "terminal", "--image", "--prompt"},
		{"theme", "show", "terminal", "-o", "x.png"},
	}
	for _, args := range tests {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			exitCode, _, stderr := runTap(t, args...)
			if exitCode != exitUserError || stderr == "" {
				t.Errorf("%v: (%d, %q), want exit 1 and a message", args, exitCode, stderr)
			}
		})
	}
}

func TestThemeImageDeckIsATitleSlide(t *testing.T) {
	deck := themeImageDeck(themes.Theme{Slug: "terminal", Name: "Terminal", Pitch: "Green on black"})
	for _, want := range []string{"theme: terminal\n", "\n# Terminal\n", "\nGreen on black\n"} {
		if !strings.Contains(deck, want) {
			t.Errorf("deck %q is missing %q", deck, want)
		}
	}
}

func TestRenderThemeImageWithBrowser(t *testing.T) {
	if testing.Short() {
		t.Skip("drives a real browser")
	}
	exporter, err := pdf.New()
	if err != nil {
		t.Fatal(err)
	}
	requireBrowser(t, exporter)
	_ = exporter.Close()

	theme, _ := findTheme("terminal")
	output := filepath.Join(t.TempDir(), "terminal.png")
	if err := renderThemeImageWithBrowser(context.Background(), theme, output); err != nil {
		t.Fatalf("renderThemeImageWithBrowser() error = %v", err)
	}
	file, err := os.Open(output)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	config, err := png.DecodeConfig(file)
	if err != nil {
		t.Fatalf("not a PNG: %v", err)
	}
	if config.Width != themeImageWidth || config.Height != themeImageHeight {
		t.Errorf("size = %dx%d, want %dx%d", config.Width, config.Height, themeImageWidth, themeImageHeight)
	}
}
