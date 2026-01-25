// Package cli provides the command-line interface for Tap.
package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"github.com/spf13/cobra"
	"github.com/tapsh/tap/internal/config"
	"github.com/tapsh/tap/internal/parser"
	"github.com/tapsh/tap/internal/server"
	"github.com/tapsh/tap/internal/transformer"
	"github.com/tapsh/tap/internal/tui"
)

// Flags for the dev command
var (
	devPort              int
	devPresenterPassword string
	devHeadless          bool
)

// devCmd represents the dev command
var devCmd = &cobra.Command{
	Use:   "dev <file>",
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
  tap dev slides.md --presenter-password secret  # Protect presenter view`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runDevServer(args[0], devPort, devPresenterPassword, devHeadless)
	},
}

func init() {
	// Register the dev command with root
	rootCmd.AddCommand(devCmd)

	// Command-specific flags
	devCmd.Flags().IntVarP(&devPort, "port", "p", 3000, "port for the dev server")
	devCmd.Flags().StringVar(&devPresenterPassword, "presenter-password", "", "password to protect the presenter view")
	devCmd.Flags().BoolVar(&devHeadless, "headless", false, "run without TUI (for testing/automation)")
}

// runDevServer starts the dev server with hot reload and TUI.
func runDevServer(file string, port int, presenterPassword string, headless bool) error {
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
	pres, err := loadPresentation(absFile, cfg, baseDir)
	if err != nil {
		return fmt.Errorf("failed to load presentation: %w", err)
	}

	// Resolve custom theme path if configured
	customThemePath, err := cfg.ResolveCustomThemePath(baseDir)
	if err != nil {
		// Log warning but don't fail - fall back to default theme
		Warning("Custom theme not loaded: %v\n", err)
	}

	// Create WebSocket hub for hot reload
	hub := server.NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	// Create and configure the server
	srv := server.New(port)
	srv.SetPresentation(pres)
	srv.SetPresenterPassword(presenterPassword)
	if customThemePath != "" {
		srv.SetCustomThemePath(customThemePath)
	}
	srv.SetupRoutes()

	// Register WebSocket handler
	srv.RegisterHandlerFunc("GET /ws", hub.HandleConnection)

	// Start the server
	if err := srv.Start(); err != nil {
		return fmt.Errorf("failed to start server: %w", err)
	}

	// Set up file watcher
	watcher, err := server.NewWatcher(absFile)
	if err != nil {
		return fmt.Errorf("failed to create file watcher: %w", err)
	}

	watcher.SetOnChange(func(path string) {
		// Reload config and presentation
		newCfg, err := config.Load(absFile)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reloading config: %v\n", err)
			return
		}

		newPres, err := loadPresentation(absFile, newCfg, baseDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error reloading presentation: %v\n", err)
			return
		}

		srv.SetPresentation(newPres)
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
		presenterURL += "?key=" + presenterPassword
	}

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	if headless {
		// Headless mode - no TUI, just log and wait for signal
		fmt.Println()
		Success("  Dev server running (headless mode)\n")
		fmt.Println()
		fmt.Printf("  Audience:  %s\n", audienceURL)
		fmt.Printf("  Presenter: %s\n", presenterURL)
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

			newPres, err := loadPresentation(absFile, newCfg, baseDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Error reloading presentation: %v\n", err)
				return
			}

			// Update custom theme path if changed
			newCustomThemePath, err := newCfg.ResolveCustomThemePath(baseDir)
			if err != nil {
				Warning("Custom theme not loaded on reload: %v\n", err)
				srv.SetCustomThemePath("")
			} else {
				srv.SetCustomThemePath(newCustomThemePath)
			}

			srv.SetPresentation(newPres)
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
		}

		// Create TUI model
		model := tui.NewDevModel(tuiCfg)
		model.UpdateWatcherStatus(true)
		model.SetThemeBroadcaster(hub)

		// Update watcher to also update TUI
		watcher.SetOnChange(func(path string) {
			// Reload config and presentation
			newCfg, err := config.Load(absFile)
			if err != nil {
				model.SetError(err)
				return
			}

			newPres, err := loadPresentation(absFile, newCfg, baseDir)
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

			model.ClearError()
			srv.SetPresentation(newPres)
			_ = hub.BroadcastReload()
			model.SendReloadEvent(path)
		})

		// Run the TUI (blocks until user quits)
		if err := tui.RunDevTUIWithModel(model); err != nil {
			return fmt.Errorf("TUI error: %w", err)
		}
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*1e9) // 5 seconds
	defer cancel()

	return srv.Shutdown(ctx)
}

// loadPresentation reads, parses, and transforms a presentation file.
func loadPresentation(file string, cfg *config.Config, baseDir string) (*transformer.TransformedPresentation, error) {
	// Read file content
	content, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	// Parse markdown
	p := parser.New()
	parsed, err := p.Parse(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse markdown: %w", err)
	}

	// Transform to frontend format
	t := transformer.NewWithBaseDir(cfg, baseDir)
	return t.Transform(parsed), nil
}
