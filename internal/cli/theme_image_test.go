package cli

import (
	"context"
	"encoding/json"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/pdf"
	"github.com/MiniCodeMonkey/tap/internal/themes"
)

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
		return os.WriteFile(outputPath, []byte("png of "+theme.Slug), 0o644)
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
	if err != nil || string(content) != "png of terminal" {
		t.Errorf("cached image = (%q, %v)", content, err)
	}

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
	if err != nil || string(content) != "png of terminal" {
		t.Errorf("output file = (%q, %v)", content, err)
	}
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
