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
	"github.com/MiniCodeMonkey/tap/internal/slidelist"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
	"github.com/MiniCodeMonkey/tap/internal/tui"
	"github.com/MiniCodeMonkey/tap/internal/usersettings"
	"github.com/spf13/cobra"
)

// Flags for the dev command
var (
	devPort              int
	devPresenterPassword string
	devHeadless          bool
	devAllowOrigins      []string
	devTunnel            bool
	devLAN               bool
	devAllowCode         bool
	devApp               bool
)

// devCmd represents the dev command
var devCmd = &cobra.Command{
	Use:   "dev [deck]",
	Short: "Start the development server",
	Long: `Start the development server to preview and present your slides.

The dev server provides:
  - Live preview of your presentation at http://localhost:<port>
  - Hot reload on file changes
  - Presenter view with speaker notes
  - Live code execution for supported drivers

The server listens on this machine only. --lan opens it to the local
network, and --tunnel puts it on a public https URL.

A deck with live code asks for approval once, in the terminal, before it
runs anything. tap approval list shows the approved decks.

Examples:
  tap dev                                 # The deck in this folder
  tap dev slides.md                      # Start server on port 3000
  tap dev slides.md --port 8080          # Use custom port
  tap dev slides.md -p 8080              # Short form
  tap dev slides.md --presenter-password secret  # Protect presenter view
  tap dev slides.md --tunnel             # Also serve it on a public https URL
  tap dev slides.md --lan                # Let a phone on the same network connect
  tap dev slides.md --headless --allow-code   # Run live code without asking, for this run only`,
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		if devApp && len(args) == 0 {
			return userError(codeUsage, errors.New("--app needs the deck: tap dev --app <deck>"))
		}
		file, err := resolveDeck(firstArg(args))
		if err != nil {
			return err
		}
		return runDevServer(serverOptions{
			file:              file,
			port:              devPort,
			portExplicit:      cmd.Flags().Changed("port"),
			presenterPassword: devPresenterPassword,
			headless:          devHeadless,
			allowOrigins:      devAllowOrigins,
			tunnel:            devTunnel,
			lan:               devLAN,
			allowCode:         devAllowCode,
			app:               devApp,
		})
	},
}

// serverOptions is everything runDevServer needs from tap dev or
// tap present.
type serverOptions struct {
	file              string
	presenterPassword string
	allowOrigins      []string
	port              int
	portExplicit      bool
	headless          bool
	tunnel            bool
	// lan listens on every interface, so a phone on the same network can connect.
	lan bool
	// present runs tap present: no file watcher, the audience view opens
	// at launch, and recording follows the run instead of the c key.
	present bool
	// record starts recording at launch. Only tap present sets it.
	record bool
	// allowCode lets live code run for this run without an approval, and
	// stores nothing. It is --allow-code.
	allowCode bool
	// app runs tap as the engine of the Tap desktop app (--app): no TUI
	// and no browser, a loopback server on a free port behind a token,
	// JSON events on standard output and commands on standard input.
	app bool
	// noRecord is tap present --no-record. Only --app reads it: without
	// --app, tap present settles recording before runDevServer.
	noRecord bool
}

func init() {
	// Register the dev command with root
	rootCmd.AddCommand(devCmd)

	// Command-specific flags
	devCmd.Flags().IntVarP(&devPort, "port", "p", 3000, "port for the dev server")
	devCmd.Flags().StringVar(&devPresenterPassword, "presenter-password", "", "password to protect the presenter view")
	devCmd.Flags().BoolVar(&devHeadless, "headless", false, "run without TUI (for testing/automation)")
	devCmd.Flags().BoolVar(&devTunnel, "tunnel", false, "also serve the deck on a public https URL through a Cloudflare Quick Tunnel (needs cloudflared; no account required)")
	devCmd.Flags().BoolVar(&devLAN, "lan", false, "listen on the local network too, so a phone on the same network can open the presenter view (default: this machine only)")
	devCmd.Flags().StringArrayVar(&devAllowOrigins, "allow-origin", nil, "additional origin (scheme://host:port) allowed to connect to the websocket hub, or host (host:port) allowed in a request's Host header, for a contributor's Vite dev server or a non-local presenter host (repeatable)")
	devCmd.Flags().BoolVar(&devAllowCode, "allow-code", false, "let the deck's live code run for this run without an approval, and save none (for --headless and scripts)")
	devCmd.Flags().BoolVar(&devApp, "app", false, "run as the engine of the Tap desktop app: JSON events on standard output, commands on standard input (an interface for the app, not for people)")
}

// recordingAudioOptions maps the deck's recording.audio config value to the
// two things the recorder needs: a CoreAudio UID and whether to record
// silently. "none" is a documented value that must record without a
// microphone, not one that gets validated as an unknown device UID; that
// mapping used to be split across two independent checks below, and the
// second one missed "none" entirely, which blocked every silent recording.
func recordingAudioOptions(configured string) (audioUID string, noAudio bool) {
	switch configured {
	case "none":
		return "", true
	case "default", "":
		return "", false
	default:
		return configured, false
	}
}

// runDevServer starts the dev server with hot reload and TUI. portExplicit
// is whether the user passed --port themselves (cmd.Flags().Changed
// ("port")): it decides whether a busy port fails outright or falls back
// to the next one (see startOnAvailablePort).
func runDevServer(options serverOptions) (err error) {
	file, port, presenterPassword, headless := options.file, options.port, options.presenterPassword, options.headless
	portExplicit, allowOrigins, wantTunnel := options.portExplicit, options.allowOrigins, options.tunnel

	// In --app mode standard output carries only the event lines. Every
	// failure from here on is also an error event, the only line when tap
	// cannot start.
	var appEvents *appEventWriter
	var appAuth *server.AppAuth
	appDisk := &appDiskStatus{}
	if options.app {
		if headless {
			return userError(codeUsage, errors.New("--app and --headless cannot be combined"))
		}
		if options.lan {
			return userError(codeUsage, errors.New("--app listens on 127.0.0.1 only, so it cannot be combined with --lan"))
		}
		if !portExplicit {
			// A free port, which the ready line reports.
			port, portExplicit = 0, true
		}
		stdout, restoreStdout := claimStdoutForApp()
		appEvents = newAppEventWriter(stdout, os.Stderr)
		defer func() {
			if err != nil {
				_, code, _ := classify(err)
				appEvents.emit(appErrorEvent{Type: appEventError, Code: code, Message: err.Error()})
			}
			closeAppEventWriter(appEvents, os.Stderr)
			restoreStdout()
		}()
		appAuth, err = server.NewAppAuth()
		if err != nil {
			return internalError(codeInternal, err)
		}
		if presenterPassword == "" {
			// Viewing is open in --app mode and steering is not. The
			// WebSocket is an audience route, so without a presenter
			// password any local process, and anyone holding the tunnel
			// link, could drive the audience's deck. The app holds this
			// generated secret instead of a person typing one, and the
			// ready line hands it over. A --presenter-password of the
			// user's own is left alone, and nothing changes for tap dev
			// without --app.
			presenterPassword = appAuth.PresenterPassword()
		}
	}

	// Resolve absolute path
	absFile, err := filepath.Abs(file)
	if err != nil {
		return fmt.Errorf("failed to resolve file path: %w", err)
	}

	// Check file exists
	if _, err := os.Stat(absFile); os.IsNotExist(err) {
		return userError(codeDeckNotFound, fmt.Errorf("file not found: %s", file))
	}

	baseDir := filepath.Dir(absFile)

	// Load configuration from frontmatter
	cfg, err := config.Load(absFile)
	if err != nil {
		return userError(codeInvalidDeck, fmt.Errorf("failed to load config: %w", err))
	}

	if err := cfg.Validate(); err != nil {
		return userError(codeInvalidDeck, fmt.Errorf("invalid config: %w", err))
	}

	// Parse and transform the presentation
	pres, warnings, resolvedComponents, componentBuildErrs, rawSlides, err := loadPresentation(absFile, cfg, baseDir)
	if err != nil {
		return userError(codeInvalidDeck, fmt.Errorf("failed to load presentation: %w", err))
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

	startupDriverWarnings := undeclaredDriverWarnings(absFile, pres)
	for _, warning := range startupDriverWarnings {
		fmt.Fprintln(os.Stderr, warning)
	}

	// Live code approval happens here, before the TUI owns the terminal.
	settingsPath, err := usersettings.Path()
	if err != nil {
		return internalError(codeInternal, err)
	}
	// In --app mode the question goes to the app after the ready line (see
	// the --app branch below), and live code stays off until it is
	// answered.
	var liveCodePolicy server.LiveCodePolicy
	if !options.app {
		liveCodePolicy, err = liveCodeApproval(approvalInput{
			Now:          time.Now,
			Config:       cfg,
			Presentation: pres,
			Asker:        terminalAsker{in: os.Stdin, out: os.Stdout},
			Out:          os.Stdout,
			SettingsPath: settingsPath,
			Deck:         absFile,
			AllowCode:    options.allowCode,
			Interactive:  stdinIsTerminal() && !headless,
		})
		if err != nil {
			return internalError(codeInternal, err)
		}
	}

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
			return internalError(codeInternal, fmt.Errorf("failed to generate presenter session token: %w", err))
		}
	}
	hub.SetPresenterSessionToken(presenterSessionToken)
	if options.present {
		hub.SetPresentMode(true)
	}
	go hub.Run()
	defer hub.Stop()

	initialRevision := server.ComputeRevision(pres, componentBundleFiles(resolvedComponents))
	hub.SetPresentationMeta(len(pres.Slides), initialRevision)
	hub.SetVersion(displayVersion())

	// Create, configure, and start the server. A candidate port that is
	// already bound (another tap dev, or anything else, listening on it)
	// is a hard error when the user asked for that exact port with
	// --port; otherwise (the default port) the next ports in turn are
	// tried instead, so two tap dev processes can run side by side
	// without flags. Each candidate gets its own Server, configured the
	// same way, since Server.New fixes its address at construction.
	buildServer := func(candidatePort int) *server.Server {
		candidate := server.NewWithHost(candidatePort, listenHost(options.lan))
		if appAuth != nil {
			candidate.SetAppAuth(appAuth)
		}
		candidate.SetPresentation(pres)
		candidate.SetRevision(initialRevision)
		candidate.SetPresenterPassword(presenterPassword)
		candidate.SetPresenterSessionToken(presenterSessionToken)
		candidate.SetAllowedOrigins(allowOrigins)
		candidate.SetBaseDir(baseDir) // Enable serving local files (images, etc.)
		candidate.SetRegistry(buildDriverRegistry(cfg, baseDir))
		// The policy stays the same for the whole run. A driver added by a
		// reload is not in it, so its blocks show "Not approved" until the
		// next start asks. This also means an approved custom driver whose
		// command changes mid-run (a git pull, an edited frontmatter) has
		// its new command run without asking again: approval is keyed by
		// driver name, not by command, and the registry below is rebuilt on
		// every reload while this policy is not.
		candidate.SetLiveCodePolicy(liveCodePolicy)
		candidate.SetComponentBundles(componentBundleFiles(resolvedComponents))
		if customThemePath != "" {
			candidate.SetCustomThemePath(customThemePath)
		}
		candidate.SetupRoutes()
		candidate.RegisterHandlerFunc("GET /ws", hub.HandleConnection)
		return candidate
	}

	srv, err := startOnAvailablePort(port, portExplicit, "tap dev", !options.lan, buildServer)
	if err != nil {
		return err
	}
	port = srv.Port()

	// Every reload below goes through publisher, which decides whether
	// open pages update in place or reload.
	publisher := newDeckPublisher(srv, hub, pres, initialRevision, customThemePath)

	var networkURL, networkQRCode string
	if options.lan {
		var found bool
		networkURL, networkQRCode, found = lanPresenterAddress(port, presenterPassword)
		if !found {
			Warning("--lan: no local network address found; only this machine can connect\n")
		}
	}

	recordOutputDir := filepath.Join(baseDir, config.DefaultRecordingOutput)
	if cfg.Recording.Output != "" {
		recordOutputDir = cfg.Recording.Output
		if !filepath.IsAbs(recordOutputDir) {
			recordOutputDir = filepath.Join(baseDir, recordOutputDir)
		}
	}

	// Set up file watcher
	watcher, err := server.NewWatcher(absFile)
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}
	// A recording rewrites its chapter list on every slide change and
	// streams the movie file as it goes; both land in the recordings
	// directory, usually inside the deck folder. Watching it would rebuild
	// the deck and reload every window, resetting the presenter timer, on
	// each slide change during a recorded talk.
	watcher.IgnoreDir(recordOutputDir)
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
		for _, warning := range undeclaredDriverWarnings(absFile, newPres) {
			fmt.Fprintln(os.Stderr, warning)
		}

		setRawSlides(newRawSlides)
		watcher.AddExtraDirs(externalInputDirs(newResolvedComponents, baseDir))
		srv.SetRegistry(buildDriverRegistry(newCfg, baseDir))
		publisher.publish(newPres, componentBundleFiles(newResolvedComponents), customThemePath, false)
	})

	if !options.present {
		if err := watcher.Start(); err != nil {
			return fmt.Errorf("failed to start file watcher: %w", err)
		}
		defer func() { _ = watcher.Stop() }()
	}

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
	audioUID, noAudio := recordingAudioOptions(cfg.Recording.Audio)

	// The controller is built before the TUI model exists, so its
	// unexpected-exit callback closes over this variable and the TUI
	// branch below fills it in once the model is created. Headless mode
	// leaves it nil, and the callback tolerates that.
	var devModel *tui.DevModel

	titleFor := func(slideIndex int) string {
		slides := currentSlides()
		if slideIndex < 0 || slideIndex >= len(slides) {
			return fmt.Sprintf("Slide %d", slideIndex+1)
		}
		return recorder.SlideTitle(slides[slideIndex].Content, slideIndex)
	}

	recordings := newRecordController(recordControllerOptions{
		DeckTitle:    cfg.Title,
		OutputDir:    recordOutputDir,
		AudioUID:     audioUID,
		NoAudio:      noAudio,
		ShowClicks:   cfg.Recording.ShowClicks,
		Chapters:     cfg.Recording.ChaptersEnabled(),
		CurrentSlide: hub.CurrentSlide,
		TitleFor:     titleFor,
		OnUnexpectedExit: func(err error) {
			if devModel != nil {
				devModel.NoteRecordingEnded(err)
			}
		},
		OnDiskLevel: func(level recorder.DiskLevel) {
			appDisk.set(level)
			_ = hub.BroadcastDiskStatus(diskStatusName(level))
			if devModel != nil {
				devModel.NoteDiskLevel(level)
			}
		},
	})
	// Stop is idempotent, so this runs safely on every exit path,
	// including when the TUI already stopped the recording itself.
	defer func() { _, _ = recordings.Stop() }()
	// Test captures are scratch files under the OS temp directory, never
	// the talk itself, so they are removed unconditionally on every exit
	// path rather than left for the OS to clean up eventually.
	defer recordings.Close()

	// present is the tap present recorder, driven by the hub's slide
	// changes instead of the c key. It is nil for tap dev.
	var present *presentRecorder
	if options.present {
		present = newPresentRecorder(presentRecorderOptions{
			Run: recorder.RunOptions{
				Parent:       recordOutputDir,
				DeckTitle:    cfg.Title,
				Base:         recorder.Options{AudioUID: audioUID, NoAudio: noAudio, ShowClicks: cfg.Recording.ShowClicks},
				Chapters:     cfg.Recording.ChaptersEnabled(),
				CurrentSlide: hub.CurrentSlide,
				TitleFor:     titleFor,
			},
			OutputDir: recordOutputDir,
			OnEvent: func(eventType, message string) {
				if devModel != nil {
					devModel.SendEvent(eventType, message)
				}
			},
			OnDiskLevel: func(level recorder.DiskLevel) {
				appDisk.set(level)
				_ = hub.BroadcastDiskStatus(diskStatusName(level))
				if devModel != nil {
					devModel.NoteDiskLevel(level)
				}
			},
		})
		// A kept recording must survive an early return above this point
		// and RunDevTUIWithModel itself returning an error below, not just
		// a clean quit through the TUI branch's own present.Finish call.
		defer func() { _, _ = present.Finish(true) }()
		hub.SetOnSlideChange(present.NoteSlideChange)
	} else {
		hub.SetOnSlideChange(recordings.NoteSlideChange)
	}

	if !recorder.Supported() && cfg.Recording != (config.Recording{}) {
		Warning("Recording is macOS only; the recording block in this deck is ignored\n")
	}

	tunnelURL := ""
	if wantTunnel && !options.app {
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

	if options.app {
		deckSource := newAppDeckSource(absFile)
		if initialSource, readErr := os.ReadFile(absFile); readErr == nil {
			deckSource.remember(initialSource)
		}

		// renderApp renders source, the app's buffer or the deck file, and
		// returns the step that serves the result and tells every open
		// page about it. It is the only path that changes what --app mode
		// shows. Nothing it does before that step is visible anywhere, and
		// the deck source runs the step only while this render is still
		// the newest one, so a render another has overtaken is thrown away
		// whole rather than published over the newer deck.
		//
		// forceReload decides between the two ways a page is told the deck
		// changed, and the choice is the caller's alone. An ordinary edit,
		// a save, or a component rebuilt by the watcher passes false: open
		// pages fetch the deck and re-render the slides that changed,
		// keeping the audience's position and everything a slide is
		// holding, such as a running animation or the output of a live
		// code block. Only the reload command passes true, which loads
		// every page again and throws all of that away. See
		// deckPublisher.publish for what each one sends.
		renderApp := func(ctx context.Context, source []byte, forceReload bool) (func(), error) {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}
			newCfg, loadErr := config.FromSource(source)
			if loadErr != nil {
				return nil, fmt.Errorf("failed to load config: %w", loadErr)
			}
			if loadErr := newCfg.Validate(); loadErr != nil {
				return nil, fmt.Errorf("invalid config: %w", loadErr)
			}
			newPres, warnings, newResolvedComponents, newComponentBuildErrs, newRawSlides, loadErr := loadPresentationSource(source, absFile, newCfg, baseDir)
			if loadErr != nil {
				return nil, fmt.Errorf("failed to load presentation: %w", loadErr)
			}
			newCustomThemePath, themeErr := newCfg.ResolveCustomThemePath(baseDir)
			if themeErr != nil {
				Warning("Custom theme not loaded on reload: %v\n", themeErr)
				newCustomThemePath = ""
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return nil, ctxErr
			}

			return func() {
				// Standard error is the Tap Log in --app mode.
				printLayoutWarningsToStderr(absFile, dropComponentBuildFailureWarnings(warnings))
				printComponentErrorsToStderr(newComponentBuildErrs)
				printComponentWarningsToStderr(componentWarnings(newResolvedComponents))
				if !deckSource.buffering() {
					// These warnings carry line numbers from the deck file,
					// which an unsaved buffer does not match.
					for _, warning := range undeclaredDriverWarnings(absFile, newPres) {
						fmt.Fprintln(os.Stderr, warning)
					}
				}
				srv.SetCustomThemePath(newCustomThemePath)
				srv.SetRegistry(buildDriverRegistry(newCfg, baseDir))
				setRawSlides(newRawSlides)
				watcher.AddExtraDirs(externalInputDirs(newResolvedComponents, baseDir))
				publisher.publish(newPres, componentBundleFiles(newResolvedComponents), newCustomThemePath, forceReload)
			}, nil
		}
		// renderCurrentForApp renders what tap shows now: the app's buffer
		// while there is one, and the deck file otherwise.
		renderCurrentForApp := func(ctx context.Context, forceReload bool) error {
			return deckSource.renderCurrent(func(source []byte) (func(), error) {
				return renderApp(ctx, source, forceReload)
			})
		}

		questions := newAppQuestions(appEvents)
		commands := make(chan appCommand, appCommandQueueSize)

		// tap present --app has no buffer and no watcher: the audience sees
		// only what the deck file held at the last reload.
		var saved func(ctx context.Context) error
		if !options.present {
			srv.RegisterHandlerFunc("PUT "+server.AppSourcePath, handleAppSource(deckSource, func(buffer []byte) (func(), error) {
				return renderApp(context.Background(), buffer, false)
			}, baseDir, os.Stderr))
			saved = func(ctx context.Context) error {
				deckSource.dropBuffer()
				return renderCurrentForApp(ctx, false)
			}

			emitFileChanged := func(path string, list *slidelist.Result) {
				appEvents.emit(appFileChangedEvent{Type: appEventFileChanged, Path: path, Result: list})
				_ = hub.BroadcastFileChanged(path)
			}
			watcher.SetOnChange(func(path string) {
				if filepath.Clean(path) == absFile {
					changed, readErr := deckSource.diskChanged()
					if readErr != nil && !os.IsNotExist(readErr) {
						fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", absFile, readErr)
						return
					}
					if readErr == nil && !changed {
						// The app's own save, or a write of what tap
						// already shows.
						return
					}
					emitFileChanged(absFile, nil)
					if readErr != nil || deckSource.buffering() {
						// The buffer wins until the app says it saved, and
						// a deleted deck leaves the last render on screen.
						return
					}
					if renderErr := renderCurrentForApp(context.Background(), false); renderErr != nil {
						fmt.Fprintf(os.Stderr, "Error reloading presentation: %v\n", renderErr)
					}
					return
				}

				// Another file in the deck folder, such as a component:
				// render again, and send the slide list, whose step counts
				// can change with a component's steps export.
				if renderErr := renderCurrentForApp(context.Background(), false); renderErr != nil {
					fmt.Fprintf(os.Stderr, "Error reloading presentation: %v\n", renderErr)
				}
				var list *slidelist.Result
				source, sourceErr := deckSource.current()
				if sourceErr != nil {
					fmt.Fprintf(os.Stderr, "Error reading %s: %v\n", absFile, sourceErr)
				} else if built, listErr := slidelist.Build(source, baseDir); listErr == nil {
					list = &built
				}
				emitFileChanged(path, list)
			})
		}

		var presentControl appPresentControl
		recordContext, stopRecording := context.WithCancel(context.Background())
		defer stopRecording()
		if present != nil {
			presentControl = present
			slides := newSlideReporter(appEvents)
			hub.SetOnSlideChange(func(slideIndex int) {
				present.NoteSlideChange(slideIndex)
				if index, step, known := hub.CurrentPosition(); known {
					slides.report(index, step)
				}
			})
		}

		startup := func(ctx context.Context) {
			if present != nil {
				record, consentErr := presentRecordingWanted(consentInput{
					SettingsPath: settingsPath,
					Out:          os.Stderr,
					NoRecord:     options.noRecord,
					Supported:    recorder.Supported(),
					Interactive:  true,
					Asker:        appConsentAsker{ctx: ctx, questions: questions, settingsPath: settingsPath},
				})
				if consentErr != nil {
					appEvents.emit(appErrorEvent{Type: appEventError, Code: codeInternal, Message: "saving the recording answer: " + consentErr.Error()})
				}
				if ctx.Err() != nil {
					return
				}
				// The same launch as the TUI branch of tap present.
				if recorder.Supported() {
					report := presentLaunchPreflight(recordings, record)
					for _, finding := range report.Findings {
						if finding.Blocking {
							present.Block(finding.Describe())
							appEvents.emit(appErrorEvent{Type: appEventError, Code: appErrorRecordingBlocked, Message: finding.Describe()})
							break
						}
					}
					present.Begin(recordContext, record)
				}
			}
			if ctx.Err() != nil {
				return
			}

			// The live code approval is its own question, never answered by
			// the recording consent above: one yes must not turn on both.
			policy, approvalErr := liveCodeApproval(approvalInput{
				Now:          time.Now,
				Config:       cfg,
				Presentation: pres,
				Asker:        appApprovalAsker{ctx: ctx, questions: questions},
				Out:          os.Stderr,
				SettingsPath: settingsPath,
				Deck:         absFile,
				AllowCode:    options.allowCode,
				Interactive:  true,
			})
			if approvalErr != nil {
				appEvents.emit(appErrorEvent{Type: appEventError, Code: codeInternal, Message: approvalErr.Error()})
				return
			}
			srv.SetLiveCodePolicy(policy)
			// Pages read which drivers may run from /api/presentation.
			_ = hub.BroadcastReload()
		}

		appEvents.emit(appReadyEvent{Type: appEventReady, Port: port, Token: appAuth.Token(), Launch: appAuth.LaunchCode(), Presenter: presenterPassword})
		go readAppCommands(os.Stdin, questions, appEvents, commands)
		runAppSession(appSessionOptions{
			Events:    appEvents,
			Questions: questions,
			Commands:  commands,
			Signals:   sigCh,
			Startup:   startup,
			Reload: func(ctx context.Context) error {
				return renderCurrentForApp(ctx, true)
			},
			Saved:             saved,
			Tunnels:           tunnels,
			Present:           presentControl,
			DiskStatus:        appDisk.get,
			Log:               os.Stderr,
			PresenterPassword: presenterPassword,
			StartTunnel:       wantTunnel,
		})
	} else if headless {
		// Headless mode - no TUI, just log and wait for signal
		fmt.Println()
		Success("  Dev server running (headless mode)\n")
		fmt.Println()
		fmt.Printf("  Version:   %s\n", displayVersion())
		fmt.Printf("  Audience:  %s\n", audienceURL)
		fmt.Printf("  Presenter: %s\n", presenterURL)
		if networkURL != "" {
			fmt.Printf("  Network:   %s\n", networkURL)
		}
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
			for _, warning := range undeclaredDriverWarnings(absFile, newPres) {
				fmt.Fprintln(os.Stderr, warning)
			}

			// Update custom theme path if changed
			newCustomThemePath := resolveCustomThemePathForReload(newCfg, baseDir, srv)

			setRawSlides(newRawSlides)
			watcher.AddExtraDirs(externalInputDirs(newResolvedComponents, baseDir))
			srv.SetRegistry(buildDriverRegistry(newCfg, baseDir))
			publisher.publish(newPres, componentBundleFiles(newResolvedComponents), newCustomThemePath, false)
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
			NetworkURL:        networkURL,
			QRCodeASCII:       networkQRCode,
			PresenterPassword: presenterPassword,
			CurrentTheme:      cfg.Theme,
			Version:           displayVersion(),
			TunnelURL:         tunnelURL,
			RecordWarnAfter:   cfg.Recording.WarnAfterDuration(),
			RecordStopAfter:   cfg.Recording.StopAfterDuration(),
			RecordDisplay:     cfg.Recording.Display,
			Present:           options.present,
		}

		// Create TUI model
		model := tui.NewDevModel(tuiCfg)
		model.UpdateWatcherStatus(true)
		model.SetThemeBroadcaster(hub)
		model.SetTunnelController(tunnels)
		model.SetRecorderController(recordings)
		devModel = model

		// The terminal lines above scroll away when the TUI starts, so the
		// TUI shows them too.
		if len(startupDriverWarnings) > 0 {
			model.SetWarnings(startupDriverWarnings)
		}

		if present != nil {
			model.SetPresentRecorder(present)
			// Recording is never offered off macOS, so there is nothing to
			// block there: the run stays unblocked and shows a plain NOT
			// RECORDING rather than an error.
			if recorder.Supported() {
				report := presentLaunchPreflight(recordings, options.record)
				for _, finding := range report.Findings {
					if finding.Blocking {
						// The speaker sees the first actionable reason, not
						// whichever blocking finding happened to come last.
						present.Block(finding.Describe())
						break
					}
				}
				recordContext, stopRecording := context.WithCancel(context.Background())
				defer stopRecording()
				present.Begin(recordContext, options.record)
			}
		} else {
			// The probe is cheap and the result is not stored anywhere: Tap
			// keeps no state between runs, and macOS raises its consent
			// dialog once per application in any case. This only checks the
			// Screen Recording permission, not the full four-check preflight:
			// that one also creates the output directory, which stays created
			// on demand, when the speaker actually presses C.
			go func() {
				report := recordings.StartupPreflight()
				for _, finding := range report.Findings {
					model.SendEvent(finding.EventType(), finding.Describe())
				}
			}()
		}

		// Track WebSocket client count
		hub.SetOnClientCountChange(func(count int) {
			model.UpdateWebSocketCount(count)
		})

		// Update watcher to also update TUI. reloadInTUI also backs r, so
		// tap present (which never starts the watcher) can still reload
		// the deck from disk by hand. The watcher serializes its own calls
		// (see server.Watcher's callbackMu), but r runs on Bubble Tea's
		// command goroutine outside that lock, so a file-save reload and a
		// manual r could otherwise interleave their writes to rawSlides,
		// srv, hub and the model; reloadMu makes the two callers mutually
		// exclusive.
		var reloadMu sync.Mutex
		// reloadInTUI reloads the deck and reports the outcome through the
		// model's own message path. manual is true for r, which drives
		// applyReloadMsg's own "Reloaded the deck" event; the file-watch
		// wrapper below passes false and reports the reload itself, so the
		// two callers never both announce the same reload.
		reloadInTUI := func(path string, manual bool) error {
			reloadMu.Lock()
			defer reloadMu.Unlock()

			// Reload config and presentation
			newCfg, err := config.Load(absFile)
			if err != nil {
				model.SetError(err)
				return err
			}

			newPres, warnings, newResolvedComponents, newComponentBuildErrs, newRawSlides, err := loadPresentation(absFile, newCfg, baseDir)
			if err != nil {
				model.SetError(err)
				return err
			}

			// Update custom theme path if changed
			newCustomThemePath := resolveCustomThemePathForReload(newCfg, baseDir, srv)

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
			model.SetWarnings(append(componentWarningLines(componentWarnings(newResolvedComponents)), undeclaredDriverWarnings(absFile, newPres)...))
			setRawSlides(newRawSlides)
			watcher.AddExtraDirs(externalInputDirs(newResolvedComponents, baseDir))
			srv.SetRegistry(buildDriverRegistry(newCfg, baseDir))
			// r forces a full reload, so a person can always get a fresh
			// page. A file change updates open pages in place.
			publisher.publish(newPres, componentBundleFiles(newResolvedComponents), newCustomThemePath, manual)
			if !manual {
				model.SendReloadEvent(path)
			}
			return nil
		}
		watcher.SetOnChange(func(path string) { _ = reloadInTUI(path, false) })
		model.SetReloader(func() error {
			return reloadInTUI(absFile, true)
		})

		// Run the TUI (blocks until user quits)
		if err := tui.RunDevTUIWithModel(model); err != nil {
			return internalError(codeInternal, fmt.Errorf("TUI error: %w", err))
		}

		if present != nil {
			summary, err := present.Finish(model.KeepRecording())
			switch {
			case err != nil:
				Warning("  Recording: %v\n", err)
			case summary.Dir == "":
			case summary.NeverLeftFirstSlide:
				Muted("  Deleted the recording: the deck never left the first slide.\n")
			case summary.Deleted:
				Muted("  Deleted the recording.\n")
			default:
				Success("  Recording kept: %s\n", summary.Dir)
			}
		} else if result, err := recordings.Stop(); err == nil && result.Path != "" {
			// Quitting the TUI stops a running recording without going
			// through applyRecordMsg, so the speaker never sees where the
			// file went; the TUI is gone by now, so print it to the
			// terminal instead. Stop is idempotent and remembers the last
			// finished recording, so this also fires (correctly, as a
			// summary rather than a fresh event) when the recording was
			// already stopped with c well before quitting.
			Success("  Last recording: %s\n", result.Path)
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

// presentLaunchPreflight picks the preflight a tap present launch runs.
// Recording from launch (startNow) needs the full Preflight, the same one
// the dev controller runs before c: it also creates the output directory,
// which Begin(startNow: true) needs right away instead of on demand.
// Waiting to record keeps the lighter StartupPreflight, which does not
// create that directory.
func presentLaunchPreflight(recordings *recordController, startNow bool) recorder.Report {
	if startNow {
		return recordings.Preflight()
	}
	return recordings.StartupPreflight()
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
	return loadPresentationSource(content, file, cfg, baseDir)
}

// loadPresentationSource is loadPresentation for a deck's text that is
// already in memory, such as the app's unsaved buffer. file names the deck
// in error messages.
func loadPresentationSource(source []byte, file string, cfg *config.Config, baseDir string) (*transformer.TransformedPresentation, []layouts.Warning, map[string]components.Result, []components.BuildError, []parser.Slide, error) {
	// Parse markdown
	p := parser.New()
	parsed, err := p.Parse(source)
	if err != nil {
		return nil, nil, nil, nil, nil, fmt.Errorf("failed to parse markdown: %s: %w", file, err)
	}

	// Resolve and bundle every component the presentation's slides use.
	// Dev builds keep source maps and skip minification. loadPresentation
	// is only ever used against a live server (tap dev, tap export pdf, tap
	// export images all share it), so an emitted asset's URL always starts
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
