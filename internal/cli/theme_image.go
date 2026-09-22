package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
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

	cached := false
	if Version != "dev" {
		if info, err := os.Stat(cachePath); err == nil && info.Mode().IsRegular() {
			cached = true
		}
	}
	if !cached {
		if err := renderIntoCache(theme, cachePath); err != nil {
			return err
		}
	}

	image := cachePath
	if themeShowOutput != "" {
		content, err := os.ReadFile(cachePath)
		if err != nil {
			return internalError(codeInternal, fmt.Errorf("reading the cached theme image: %w", err))
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

// renderIntoCache renders theme into a temporary file next to cachePath
// and moves it into place, so a cancelled render never leaves a partial
// image in the cache.
func renderIntoCache(theme themes.Theme, cachePath string) error {
	if err := os.MkdirAll(filepath.Dir(cachePath), 0o755); err != nil {
		return internalError(codeInternal, fmt.Errorf("creating the theme image cache: %w", err))
	}
	temporary, err := os.CreateTemp(filepath.Dir(cachePath), theme.Slug+".partial*.png")
	if err != nil {
		return internalError(codeInternal, fmt.Errorf("creating the theme image cache: %w", err))
	}
	temporaryPath := temporary.Name()
	_ = temporary.Close()
	defer os.Remove(temporaryPath)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := renderThemeImage(ctx, theme, temporaryPath); err != nil {
		if ctx.Err() != nil {
			return errInterrupted
		}
		return err
	}
	if err := os.Rename(temporaryPath, cachePath); err != nil {
		return internalError(codeInternal, fmt.Errorf("saving the theme image: %w", err))
	}
	return nil
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
