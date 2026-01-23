// Package cli provides the command-line interface for Tap.
package cli

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

// Flags for the serve command
var (
	servePort int
)

// serveCmd represents the serve command
var serveCmd = &cobra.Command{
	Use:   "serve [dir]",
	Short: "Serve a built presentation",
	Long: `Serve a previously built presentation from static files.

This command starts a simple HTTP file server to preview your built
presentation locally before deploying. It's useful for testing your
static build output.

The serve command is intended for previewing static builds. For live
development with hot reload and code execution, use 'tap dev' instead.

Examples:
  tap serve                    # Serve from dist/ on port 3000
  tap serve public             # Serve from custom directory
  tap serve --port 8080        # Use custom port
  tap serve ./build -p 8080    # Both options together`,
	Args: cobra.MaximumNArgs(1),
	Run:  runServe,
}

func runServe(cmd *cobra.Command, args []string) {
	// Determine directory to serve
	dir := "dist"
	if len(args) > 0 {
		dir = args[0]
	}

	// Check if directory exists
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		Errorln("Error: Directory does not exist:", dir)
		fmt.Println()
		Muted("  Hint: Run 'tap build <file>' first to generate static files.\n")
		os.Exit(1)
	}
	if err != nil {
		Errorln("Error: Cannot access directory:", err)
		os.Exit(1)
	}
	if !info.IsDir() {
		Errorln("Error: Not a directory:", dir)
		os.Exit(1)
	}

	// Create file server
	fs := http.FileServer(http.Dir(dir))
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Log requests
		start := time.Now()
		fs.ServeHTTP(w, r)
		Info("GET ")
		fmt.Printf("%s ", r.URL.Path)
		Muted("(%s)\n", time.Since(start).Round(time.Microsecond))
	})

	// Create HTTP server
	addr := fmt.Sprintf("0.0.0.0:%d", servePort)
	server := &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Display startup message
	fmt.Println()
	Success("  Serving presentation from %s\n", dir)
	fmt.Println()
	fmt.Printf("  Local:   http://localhost:%d\n", servePort)
	fmt.Printf("  Network: http://0.0.0.0:%d\n", servePort)
	fmt.Println()
	Muted("  Press Ctrl+C to stop\n")
	fmt.Println()

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start server in goroutine
	errCh := make(chan error, 1)
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for signal or error
	select {
	case err := <-errCh:
		Errorln("Server error:", err)
		os.Exit(1)
	case <-sigCh:
		fmt.Println()
		Info("Shutting down server...\n")
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		Errorln("Error during shutdown:", err)
		os.Exit(1)
	}

	Successln("Server stopped.")
}

func init() {
	// Register the serve command with root
	rootCmd.AddCommand(serveCmd)

	// Command-specific flags
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 3000, "port for the server")
}
