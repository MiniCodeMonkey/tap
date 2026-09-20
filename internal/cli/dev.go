// Package cli provides the command-line interface for Tap.
package cli

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/components"
	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/layouts"
	"github.com/MiniCodeMonkey/tap/internal/parser"
	"github.com/MiniCodeMonkey/tap/internal/recorder"
	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
	"github.com/MiniCodeMonkey/tap/internal/tui"
	"github.com/spf13/cobra"
)

// Flags for the dev command
var (
	devPort              int
	devPresenterPassword string
	devHeadless          bool
	devAllowOrigins      []string
	devTunnel            bool
)

// devCmd represents the dev command
var devCmd = &cobra.Command{
	Use:   "dev [file]",
	Short: "Start the development server",
	Long: `Start the development server to preview and present your slides.

The dev server provides:
  - Live preview of your presentation at http://localhost:<port>
  - Hot reload on file changes
  - Presenter view with speaker notes
  - Live code execution for supported drivers

Examples:
  tap dev slides.md                      # Start server on port 3000
  tap dev slides.md --port 8080          # Use custom port
  tap dev slides.md -p 8080              # Short form
  tap dev slides.md --presenter-password secret  # Protect presenter view
  tap dev slides.md --tunnel             # Also serve it on a public https URL`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		var file string

		if len(args) == 0 {
			// No file provided - show file picker or error
			result, err := tui.RunFilePicker()
			if err != nil {
				return err
			}

			if result.Aborted {
				if result.File == "" {
					// No files found - show helpful error
					fmt.Print(tui.RenderNoFilesError())
					return nil
				}
				// User cancelled
				return nil
			}

			file = result.File
		} else {
			file = args[0]
		}

		return runDevServer(file, devPort, devPresenterPassword, devHeadless, cmd.Flags().Changed("port"), devAllowOrigins, devTunnel)
	},
}

func init() {
	// Register the dev command with root
	rootCmd.AddCommand(devCmd)

	// Command-specific flags
	devCmd.Flags().IntVarP(&devPort, "port", "p", 3000, "port for the dev server")
	devCmd.Flags().StringVar(&devPresenterPassword, "presenter-password", "", "password to protect the presenter view")
	devCmd.Flags().BoolVar(&devHeadless, "headless", false, "run without TUI (for testing/automation)")
	devCmd.Flags().BoolVar(&devTunnel, "tunnel", false, "also serve the deck on a public https URL through a Cloudflare Quick Tunnel (needs cloudflared; no account required)")
	devCmd.Flags().StringArrayVar(&devAllowOrigins, "allow-origin", nil, "additional origin (scheme://host:port) allowed to connect to the websocket hub, or host (host:port) allowed in a request's Host header, for a contributor's Vite dev server or a non-local presenter host (repeatable)")
}

// runDevServer starts the dev server with hot reload and TUI. portExplicit
// is whether the user passed --port themselves (cmd.Flags().Changed
// ("port")): it decides whether a busy port fails outright or falls back
// to the next one (see startOnAvailablePort).
func runDevServer(file string, port int, presenterPassword string, headless bool, portExplicit bool, allowOrigins []string, wantTunnel bool) error {
	// Resolve absolute path
	absFile, err := filepath.Abs(file)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Check file exists
	if _, err := os.Stat(absFile); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", file)
	}

	baseDir := filepath.Dir(absFile)

	// Load configuration from frontmatter
	cfg, err := config.Load(absFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	if err := cfg.Validate(); err != nil {
		return fmt.Errorf("invalid config: %w", err)
	}

	// Parse and transform the presentation
	pres, warnings, resolvedComponents, componentBuildErrs, rawSlides, err := loadPresentation(absFile, cfg, baseDir)
	if err != nil {
		return fmt.Errorf("failed to load presentation: %w", err)
	}

	// currentSlides gives the recording controller's TitleFor a live view
	// of the deck's raw markdown slides, so a chapter title is always
	// named from whatever is loaded right now. presMu guards rawSlides
	// because the watcher goroutine below writes it on every reload while
	// TitleFor can run concurrently from the hub's own goroutine.
	var presMu sync.RWMutex
	currentSlides := func() []parser.Slide {
		presMu.RLock()
		defer presMu.RUnlock()
		return rawSlides
	}
	setRawSlides := func(slides []parser.Slide) {
		presMu.Lock()
		rawSlides = slides
		presMu.Unlock()
	}
	// The TUI has not started yet at this point either way, so stderr is
	// always safe here.
	printLayoutWarningsToStderr(absFile, dropComponentBuildFailureWarnings(warnings))
	printComponentErrorsToStderr(componentBuildErrs)
	printComponentWarningsToStderr(componentWarnings(resolvedComponents))

	// Resolve custom theme path if configured
	customThemePath, err := cfg.ResolveCustomThemePath(baseDir)
	if err != nil {
		// Log warning but don't fail - fall back to default theme
		Warning("Custom theme not loaded: %v\n", err)
	}

	// Create WebSocket hub for hot reload
	hub := server.NewWebSocketHub()
	// TAP_HUB_STATE_RETENTION overrides how long the hub keeps its last
	// known slide state after the last client disconnects (default
	// server.DefaultStateRetention). Internal/CI knob, not a user-facing
	// flag: see CONTRIBUTING.md. The e2e suite's shared dev server sets it
	// to "0s" so specs that assert "no state" don't see state left behind
	// by a previous spec's viewer disconnecting.
	if raw := os.Getenv("TAP_HUB_STATE_RETENTION"); raw != "" {
		if retention, err := time.ParseDuration(raw); err != nil {
			Warning("Invalid TAP_HUB_STATE_RETENTION %q, using the default: %v\n", raw, err)
		} else {
			hub.SetStateRetention(retention)
		}
	}
	hub.SetAllowedOrigins(allowOrigins)
	hub.SetPresenterPassword(presenterPassword)
	// A presenter session token, generated once per process, is what the
	// presenter auth cookie actually carries (see
	// server.GeneratePresenterSessionToken): the raw password never rides
	// in a cookie, since Go's cookie jar sanitizes a value containing a
	// semicolon, quote, backslash, space, or non-ASCII character, which
	// would otherwise silently break the compare for a real password.
	var presenterSessionToken string
	if presenterPassword != "" {
		presenterSessionToken, err = server.GeneratePresenterSessionToken()
		if err != nil {
			return fmt.Errorf("failed to generate presenter session token: %w", err)
		}
	}
	hub.SetPresenterSessionToken(presenterSessionToken)
	go hub.Run()
	defer hub.Stop()

	hub.SetPresentationMeta(len(pres.Slides), server.ComputeRevision(pres, componentBundleFiles(resolvedComponents)))

	// Create, configure, and start the server. A candidate port that is
	// already bound (another tap dev, or anything else, listening on it)
	// is a hard error when the user asked for that exact port with
	// --port; otherwise (the default port) the next ports in turn are
	// tried instead, so two tap dev processes can run side by side
	// without flags. Each candidate gets its own Server, configured the
	// same way, since Server.New fixes its address at construction.
	buildServer := func(candidatePort int) *server.Server {
		candidate := server.New(candidatePort)
		candidate.SetPresentation(pres)
		candidate.SetPresenterPassword(presenterPassword)
		candidate.SetPresenterSessionToken(presenterSessionToken)
		candidate.SetAllowedOrigins(allowOrigins)
		candidate.SetBaseDir(baseDir) // Enable serving local files (images, etc.)
		candidate.SetComponentBundles(componentBundleFiles(resolvedComponents))
		if customThemePath != "" {
			candidate.SetCustomThemePath(customThemePath)
		}
		candidate.SetupRoutes()
		candidate.RegisterHandlerFunc("GET /ws", hub.HandleConnection)
		return candidate
	}

	srv, err := startOnAvailablePort(port, portExplicit, "tap dev", buildServer)
	if err != nil {
		return err
	}
	port = srv.Port()

	// Set up file watcher
	watcher, err := server.NewWatcher(absFile)
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	// A component can import a file from outside the deck directory tree
	// (e.g. "../shared/Thing.jsx"); the recursive watch below only covers
	// the deck directory itself, so such a file's directory needs adding
	// separately, from esbuild's metafile inputs.
	watcher.AddExtraDirs(externalInputDirs(resolvedComponents, baseDir))

	watcher.SetOnChange(func(path string) {
		// Reload config and presentation
		newCfg, err := config.Load(absFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reloading config: %v\n", err)
			return
		}

		newPres, warnings, newResolvedComponents, newComponentBuildErrs, newRawSlides, err := loadPresentation(absFile, newCfg, baseDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reloading presentation: %v\n", err)
			return
		}
		// This handler only runs before the headless/TUI branch below
		// installs its own (the TUI has not started yet either way), so
		// stderr is safe here.
		printLayoutWarningsToStderr(absFile, dropComponentBuildFailureWarnings(warnings))
		printComponentErrorsToStderr(newComponentBuildErrs)
		printComponentWarningsToStderr(componentWarnings(newResolvedComponents))

		setRawSlides(newRawSlides)
		watcher.AddExtraDirs(externalInputDirs(newResolvedComponents, baseDir))
		srv.SetComponentBundles(componentBundleFiles(newResolvedComponents))
		srv.SetPresentation(newPres)
		hub.SetPresentationMeta(len(newPres.Slides), server.ComputeRevision(newPres, componentBundleFiles(newResolvedComponents)))
		_ = hub.BroadcastReload()
	})

	if err := watcher.Start(); err != nil {
		return fmt.Errorf("failed to start file watcher: %w", err)
	}
	defer func() { _ = watcher.Stop() }()

	// Generate URLs
	audienceURL := fmt.Sprintf("http://localhost:%d", port)
	presenterURL := fmt.Sprintf("http://localhost:%d/presenter", port)
	if presenterPassword != "" {
		// URL-encoded, so a password with a space, "&", or other character
		// with meaning in a URL query still round-trips as the same ?key=
		// value a client sends back.
		presenterURL += "?key=" + url.QueryEscape(presenterPassword)
	}

	// A tunnel puts the deck on a public https URL, which is also the only
	// way a phone gets a secure context (and so a screen wake lock) from a
	// dev server. The controller also keeps the Host allow-list in step.
	tunnels := newTunnelController(port, allowOrigins, srv, hub)
	defer func() { _ = tunnels.Stop() }()

	// Recording is opt-in at the keyboard, but its preflight runs at
	// startup so a missing permission is found during setup rather than
	// on stage.
	recordOutputDir := filepath.Join(baseDir, "recordings")
	if cfg.Recording.Output != "" {
		recordOutputDir = cfg.Recording.Output
		if !filepath.IsAbs(recordOutputDir) {
			recordOutputDir = filepath.Join(baseDir, recordOutputDir)
		}
	}

	audioUID := cfg.Recording.Audio
	if audioUID == "default" {
		audioUID = ""
	}

	recordings := newRecordController(recordControllerOptions{
		DeckTitle:    cfg.Title,
		OutputDir:    recordOutputDir,
		AudioUID:     audioUID,
		NoAudio:      cfg.Recording.Audio == "none",
		ShowClicks:   cfg.Recording.ShowClicks,
		Chapters:     cfg.Recording.ChaptersEnabled(),
		CurrentSlide: hub.CurrentSlide,
		TitleFor: func(slideIndex int) string {
			slides := currentSlides()
			if slideIndex < 0 || slideIndex >= len(slides) {
				return fmt.Sprintf("Slide %d", slideIndex+1)
			}
			return recorder.SlideTitle(slides[slideIndex].Content, slideIndex)
		},
	})
	// Stop is idempotent, so this runs safely on every exit path,
	// including when the TUI already stopped the recording itself.
	defer func() { _, _ = recordings.Stop() }()

	hub.SetOnSlideChange(recordings.NoteSlideChange)

	if !recorder.Supported() && cfg.Recording != (config.Recording{}) {
		Warning("Recording is macOS only; the recording block in this deck is ignored\n")
	}

	tunnelURL := ""
	if wantTunnel {
		if !tunnels.Available() {
			return notInstalledError()
		}

		fmt.Println()
		Muted("  Starting tunnel...\n")

		startCtx, cancelStart := context.WithTimeout(context.Background(), 45*time.Second)
		tunnelURL, err = tunnels.Start(startCtx)
		cancelStart()
		if err != nil {
			return fmt.Errorf("starting the tunnel: %w", err)
		}
	}

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	if headless {
		// Headless mode - no TUI, just log and wait for signal
		fmt.Println()
		Success("  Dev server running (headless mode)\n")
		fmt.Println()
		fmt.Printf("  Version:   %s\n", displayVersion())
		fmt.Printf("  Audience:  %s\n", audienceURL)
		fmt.Printf("  Presenter: %s\n", presenterURL)
		if tunnelURL != "" {
			fmt.Printf("  Tunnel:    %s\n", tunnelURL)
		}
		fmt.Println()
		Muted("  Press Ctrl+C to stop\n")
		fmt.Println()

		// Update watcher for headless mode
		watcher.SetOnChange(func(path string) {
			// Reload config and presentation
			newCfg, err := config.Load(absFile)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reloading config: %v\n", err)
				return
			}

			newPres, warnings, newResolvedComponents, newComponentBuildErrs, newRawSlides, err := loadPresentation(absFile, newCfg, baseDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reloading presentation: %v\n", err)
				return
			}
			// No TUI in headless mode, so stderr is always safe.
			printLayoutWarningsToStderr(absFile, dropComponentBuildFailureWarnings(warnings))
			printComponentErrorsToStderr(newComponentBuildErrs)
			printComponentWarningsToStderr(componentWarnings(newResolvedComponents))

			// Update custom theme path if changed
			newCustomThemePath, err := newCfg.ResolveCustomThemePath(baseDir)
			if err != nil {
				Warning("Custom theme not loaded on reload: %v\n", err)
				srv.SetCustomThemePath("")
			} else {
				srv.SetCustomThemePath(newCustomThemePath)
			}

			setRawSlides(newRawSlides)
			watcher.AddExtraDirs(externalInputDirs(newResolvedComponents, baseDir))
			srv.SetComponentBundles(componentBundleFiles(newResolvedComponents))
			srv.SetPresentation(newPres)
			hub.SetPresentationMeta(len(newPres.Slides), server.ComputeRevision(newPres, componentBundleFiles(newResolvedComponents)))
			_ = hub.BroadcastReload()
			Info("Reloaded: %s\n", path)
		})

		// Wait for signal
		<-sigCh
		fmt.Println()
		Info("Shutting down...\n")
	} else {
		// Run the TUI
		tuiCfg := tui.DevConfig{
			MarkdownFile:      file,
			Port:              port,
			AudienceURL:       audienceURL,
			PresenterURL:      presenterURL,
			PresenterPassword: presenterPassword,
			CurrentTheme:      cfg.Theme,
			Version:           displayVersion(),
			TunnelURL:         tunnelURL,
			RecordWarnAfter:   cfg.Recording.WarnAfterDuration(),
			RecordStopAfter:   cfg.Recording.StopAfterDuration(),
			RecordDisplay:     cfg.Recording.Display,
		}

		// Create TUI model
		model := tui.NewDevModel(tuiCfg)
		model.UpdateWatcherStatus(true)
		model.SetThemeBroadcaster(hub)
		model.SetTunnelController(tunnels)
		model.SetRecorderController(recordings)

		// The probe is cheap and the result is not stored anywhere: Tap
		// keeps no state between runs, and macOS raises its consent
		// dialog once per application in any case.
		go func() {
			report := recordings.Preflight()
			for _, finding := range report.Findings {
				message := finding.Message
				if finding.Fix != "" {
					message += ". " + finding.Fix
				}
				model.SendEvent("error", message)
			}
		}()

		// Track WebSocket client count
		hub.SetOnClientCountChange(func(count int) {
			model.UpdateWebSocketCount(count)
		})

		// Update watcher to also update TUI
		watcher.SetOnChange(func(path string) {
			// Reload config and presentation
			newCfg, err := config.Load(absFile)
			if err != nil {
				model.SetError(err)
				return
			}

			newPres, warnings, newResolvedComponents, newComponentBuildErrs, newRawSlides, err := loadPresentation(absFile, newCfg, baseDir)
			if err != nil {
				model.SetError(err)
				return
			}

			// Update custom theme path if changed
			newCustomThemePath, err := newCfg.ResolveCustomThemePath(baseDir)
			if err != nil {
				// Log warning but continue - use empty path to disable custom theme
				Warning("Custom theme not loaded on reload: %v\n", err)
				srv.SetCustomThemePath("")
			} else {
				srv.SetCustomThemePath(newCustomThemePath)
			}

			// The TUI owns the terminal here, so layout and component
			// build errors go through the model's own message path
			// instead of stderr, which would corrupt its rendering.
			var messages []string
			if warningsErr := layoutWarningsError(dropComponentBuildFailureWarnings(warnings)); warningsErr != nil {
				messages = append(messages, warningsErr.Error())
			}
			if buildErr := componentErrorsError(newComponentBuildErrs); buildErr != nil {
				messages = append(messages, buildErr.Error())
			}
			if len(messages) > 0 {
				model.SetError(errors.New(strings.Join(messages, "\n")))
			} else {
				model.ClearError()
			}
			// Bundler warnings (an esbuild warning on a component bundle,
			// for example) are not fatal enough to be an error, but still
			// worth showing; the model clears them on the next reload that
			// has none, so a warning never outlives the build it came from.
			model.SetWarnings(componentWarningLines(componentWarnings(newResolvedComponents)))
			setRawSlides(newRawSlides)
			watcher.AddExtraDirs(externalInputDirs(newResolvedComponents, baseDir))
			srv.SetComponentBundles(componentBundleFiles(newResolvedComponents))
			srv.SetPresentation(newPres)
			hub.SetPresentationMeta(len(newPres.Slides), server.ComputeRevision(newPres, componentBundleFiles(newResolvedComponents)))
			_ = hub.BroadcastReload()
			model.SendReloadEvent(path)
		})

		// Run the TUI (blocks until user quits)
		if err := tui.RunDevTUIWithModel(model); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}

		// Quitting the TUI stops a running recording without going
		// through applyRecordMsg, so the speaker never sees where the
		// file went; the TUI is gone by now, so print it to the
		// terminal instead. Stop is idempotent, so this is safe whether
		// or not the TUI already stopped the recording itself.
		if result, err := recordings.Stop(); err == nil && result.Path != "" {
			Success("  Recording saved to %s\n", result.Path)
			if result.ChapterPath != "" {
				Muted("  Chapters: %s\n", result.ChapterPath)
			}
		}
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*1e9) // 5 seconds
	defer cancel()

	return srv.Shutdown(ctx)
}

// loadPresentation reads, parses, resolves components for, and transforms a
// presentation file. It also returns any layout or slot warnings found and
// the resolved components (bundles and build errors), letting the caller
// decide where to show them: printLayoutWarningsToStderr and
// printComponentErrorsToStderr for a plain terminal, or through the TUI
// model's own message path when the TUI owns the terminal (see the reload
// handler in the TUI branch of Run). The returned []parser.Slide is the
// deck's raw markdown slides, in the same order as the returned
// presentation's Slides: the recording controller's chapter titles need the
// original markdown (for its headings), which the transformed, HTML-only
// presentation no longer carries.
func loadPresentation(file string, cfg *config.Config, baseDir string) (*transformer.TransformedPresentation, []layouts.Warning, map[string]components.Result, []components.BuildError, []parser.Slide, error) {
	// Read file content
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse markdown
	p := parser.New()
	parsed, err := p.Parse(content)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to parse markdown: %s: %w", file, err)
	}

	// Resolve and bundle every component the presentation's slides use.
	// Dev builds keep source maps and skip minification. loadPresentation
	// is only ever used against a live server (tap dev, tap pdf, tap
	// screenshot all share it), so an emitted asset's URL always starts
	// from the server root.
	resolvedComponents, componentBuildErrs := buildComponents(parsed, baseDir, false, true, "/components/")

	// Transform to frontend format
	t := transformer.NewWithBaseDir(cfg, baseDir)
	t.SetComponents(resolvedComponents)
	transformed := t.Transform(parsed)

	return transformed, layouts.Validate(transformed), resolvedComponents, componentBuildErrs, parsed.Slides, nil
}

// printLayoutWarningsToStderr prints one line to stderr for each layout or
// slot warning. Only safe to call when nothing else owns the terminal (the
// TUI has not started, or is not in use); the TUI branch of Run routes
// warnings through the model instead.
func printLayoutWarningsToStderr(file string, warnings []layouts.Warning) {
	for _, warning := range warnings {
		fmt.Fprintf(os.Stderr, "warning: %s: slide %d: %s\n", file, warning.SlideNumber, warning.Message)
	}
}

// dropComponentBuildFailureWarnings removes the "component ... failed to
// build: ..." warning layouts.Validate adds for a broken whole-slide
// component. tap dev always prints that same failure as an "error:" line
// (see printComponentErrorsToStderr) right next to the warning list; a
// caller that also does so should filter here first, so the broken
// component is reported once, not twice.
func dropComponentBuildFailureWarnings(warnings []layouts.Warning) []layouts.Warning {
	filtered := make([]layouts.Warning, 0, len(warnings))
	for _, warning := range warnings {
		if strings.Contains(warning.Message, "failed to build:") {
			continue
		}
		filtered = append(filtered, warning)
	}
	return filtered
}

// layoutWarningsError joins layout/slot warnings into a single error for
// display through the TUI model's SetError, the model's only message path,
// or nil when there are none.
func layoutWarningsError(warnings []layouts.Warning) error {
	if len(warnings) == 0 {
		return nil
	}
	lines := make([]string, len(warnings))
	for i, warning := range warnings {
		lines[i] = fmt.Sprintf("slide %d: %s", warning.SlideNumber, warning.Message)
	}
	return fmt.Errorf("layout warning(s):\n%s", strings.Join(lines, "\n"))
}
