package cli

import (
	"context"
	"fmt"
	"image/png"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/pdf"
	"github.com/MiniCodeMonkey/tap/internal/themes"
)

// The size of a theme preview image, 16:9.
const (
	themeImageWidth  = 1280
	themeImageHeight = 720
)

// staleThemeImagePartialAge is how old a *.partial*.png file has to be
// before sweepStaleThemeImagePartials treats it as orphaned by a hard
// kill rather than owned by a render still in progress. It is far longer
// than any real render takes, so it never races a live one.
const staleThemeImagePartialAge = time.Hour

// themeImageCacheRoot is the folder the theme image cache lives under.
// Tests point it at a temporary folder.
var themeImageCacheRoot = os.UserCacheDir

// renderThemeImage writes a PNG of a title slide in theme to outputPath.
// Tests replace it, so only one test drives a browser.
var renderThemeImage = renderThemeImageWithBrowser

// themeImageResult is the --json result of tap theme show --image.
type themeImageResult struct {
	Slug   string `json:"slug"`
	Image  string `json:"image"`
	Cached bool   `json:"cached"`
}

// themeImageCachePath is where the preview of slug is cached for this tap
// version: <user cache folder>/tap/themes/<version>/<slug>.png. A new tap
// version gets a new folder, so a changed theme is never served stale.
func themeImageCachePath(slug string) (string, error) {
	root, err := themeImageCacheRoot()
	if err != nil {
		return "", internalError(codeInternal, fmt.Errorf("no cache folder for theme images: %w", err))
	}
	return filepath.Join(root, "tap", "themes", Version, slug+".png"), nil
}

// showThemeImage prints the path of a preview image of theme, rendering it
// into the cache first when the cache has none. A dev build always
// renders, because its version does not change when the frontend does.
// With --output, the image is copied there and that path is printed.
func showThemeImage(cmd *cobra.Command, theme themes.Theme) error {
	cachePath, err := themeImageCachePath(theme.Slug)
	if err != nil {
		return err
	}

	cached := Version != "dev" && validThemeImage(cachePath)

	image := cachePath
	if !cached {
		renderedPath, err := renderIntoCache(theme, cachePath)
		if err != nil {
			return err
		}
		image = renderedPath
	}

	if themeShowOutput != "" {
		content, err := os.ReadFile(image)
		if err != nil {
			return internalError(codeInternal, fmt.Errorf("reading the rendered theme image: %w", err))
		}
		if err := os.WriteFile(themeShowOutput, content, 0o644); err != nil {
			return userError(codeFailed, fmt.Errorf("cannot write %s: %w", themeShowOutput, err))
		}
		image = themeShowOutput
	}

	if themeShowJSON {
		return printJSONOK(cmd.OutOrStdout(), themeImageResult{Slug: theme.Slug, Image: image, Cached: cached})
	}
	fmt.Fprintln(cmd.OutOrStdout(), image)
	return nil
}

// renderIntoCache renders theme and returns the path of the rendered
// image. It renders into a temporary file next to cachePath and moves it
// into place, so a cancelled render never leaves a partial image visible
// at the cache path. The cache is an optimisation, not a requirement: when
// the cache directory cannot be created, written to, or moved into (a
// read-only or full disk), the render still happens, just to a plain
// temporary file outside the cache, so the command still succeeds and
// still produces its image.
func renderIntoCache(theme themes.Theme, cachePath string) (string, error) {
	cacheDir := filepath.Dir(cachePath)
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return renderToTemporaryFile(theme)
	}
	sweepStaleThemeImagePartials(cacheDir)

	temporary, err := os.CreateTemp(cacheDir, theme.Slug+".partial*.png")
	if err != nil {
		return renderToTemporaryFile(theme)
	}
	temporaryPath := temporary.Name()
	_ = temporary.Close()

	if err := renderTheme(theme, temporaryPath); err != nil {
		_ = os.Remove(temporaryPath)
		return "", err
	}
	if err := os.Rename(temporaryPath, cachePath); err != nil {
		// The render itself succeeded; only moving it into the cache
		// failed (for example the disk filled between MkdirAll and
		// here). Serve the rendered file directly instead of failing
		// the command over a cache write.
		return temporaryPath, nil
	}
	return cachePath, nil
}

// renderToTemporaryFile renders theme to a plain temporary file outside
// the theme image cache, for when the cache directory itself cannot be
// used. The command still succeeds and still produces an image; it just
// gets no persistent cache entry this time.
func renderToTemporaryFile(theme themes.Theme) (string, error) {
	temporary, err := os.CreateTemp("", theme.Slug+"-*.png")
	if err != nil {
		return "", internalError(codeInternal, fmt.Errorf("creating a temporary file for the theme image: %w", err))
	}
	temporaryPath := temporary.Name()
	_ = temporary.Close()

	if err := renderTheme(theme, temporaryPath); err != nil {
		_ = os.Remove(temporaryPath)
		return "", err
	}
	return temporaryPath, nil
}

// renderTheme renders theme to outputPath, cancelling the render and
// reporting errInterrupted on SIGINT or SIGTERM.
func renderTheme(theme themes.Theme, outputPath string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := renderThemeImage(ctx, theme, outputPath); err != nil {
		if ctx.Err() != nil {
			return errInterrupted
		}
		return err
	}
	return nil
}

// validThemeImage reports whether path is a complete, undamaged PNG. A
// cache hit is only trusted after this passes: a corrupt or truncated
// file sitting at the cache path (external tampering, disk corruption) is
// treated as a cache miss, so the entry gets re-rendered and replaced
// rather than served as a false success.
func validThemeImage(path string) bool {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	file, err := os.Open(path)
	if err != nil {
		return false
	}
	defer file.Close()
	_, err = png.Decode(file)
	return err == nil
}

// sweepStaleThemeImagePartials removes *.partial*.png files from dir that
// are older than staleThemeImagePartialAge. A hard kill (SIGKILL, a
// panic) during a render skips the deferred cleanup and the rename into
// place, leaving its temporary file orphaned in the cache directory
// forever; this sweeps those out on the next render into the same
// directory, without touching a partial recent enough that a live render
// might still own it, and without ever touching a finished cache entry
// (those never match the *.partial*.png pattern). Best effort: any error
// is ignored, since a failed cleanup must never fail the command.
func sweepStaleThemeImagePartials(dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-staleThemeImagePartialAge)
	for _, entry := range entries {
		if entry.IsDir() || !strings.Contains(entry.Name(), ".partial") {
			continue
		}
		info, err := entry.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		_ = os.Remove(filepath.Join(dir, entry.Name()))
	}
}

// themeImageDeck is a one-slide deck that shows theme on a title slide:
// the theme's name as the heading and its pitch as the subtitle.
func themeImageDeck(theme themes.Theme) string {
	return fmt.Sprintf("---\ntitle: %q\ntheme: %s\n---\n\n# %s\n\n%s\n", theme.Name, theme.Slug, theme.Name, theme.Pitch)
}

// renderThemeImageWithBrowser renders themeImageDeck in the headless
// browser, through the same temporary server and capture as tap export
// images, and writes the PNG to outputPath.
func renderThemeImageWithBrowser(ctx context.Context, theme themes.Theme, outputPath string) error {
	folder, err := os.MkdirTemp("", "tap-theme-image-*")
	if err != nil {
		return internalError(codeInternal, err)
	}
	defer os.RemoveAll(folder)

	deck := filepath.Join(folder, "theme.md")
	if err := os.WriteFile(deck, []byte(themeImageDeck(theme)), 0o644); err != nil {
		return internalError(codeInternal, err)
	}
	cfg, err := config.Load(deck)
	if err != nil {
		return internalError(codeInternal, fmt.Errorf("loading the theme deck: %w", err))
	}
	srv, _, _, buildErrors, _, err := prepareDeck(deck, cfg, folder)
	if err != nil {
		return internalError(codeInternal, fmt.Errorf("serving the theme deck: %w", err))
	}
	if len(buildErrors) > 0 || srv == nil {
		return internalError(codeInternal, fmt.Errorf("the theme deck did not build"))
	}
	defer func() {
		shutdownContext, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownContext)
	}()

	exporter, err := pdf.New()
	if err != nil {
		return internalError(codeBrowser, fmt.Errorf("failed to create browser exporter: %w", err))
	}
	defer func() { _ = exporter.Close() }()
	if err := exporter.EnsureBrowser(); err != nil {
		return internalError(codeBrowser, fmt.Errorf("failed to start browser: %w", err))
	}

	options := pdf.CaptureOptions{SlideNumber: 1, Width: themeImageWidth, Height: themeImageHeight, Print: true}
	if err := exporter.CaptureSlide(ctx, fmt.Sprintf("http://localhost:%d", srv.Port()), options, outputPath); err != nil {
		return internalError(codeExportFailed, fmt.Errorf("rendering the theme image: %w", err))
	}
	return nil
}
