// Package cli provides the command-line interface for Tap.
package cli

import (
	"context"
	"fmt"
	"net"
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
	RunE: runServe,
}

func runServe(cmd *cobra.Command, args []string) error {
	// Determine directory to serve
	dir := "dist"
	if len(args) > 0 {
		dir = args[0]
	}

	// Check if directory exists
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		fmt.Println()
		Muted("  Hint: Run 'tap build [deck]' first to generate static files.\n")
		return userError(codeDeckNotFound, fmt.Errorf("directory does not exist: %s", dir))
	}
	if err != nil {
		return userError(codeUsage, fmt.Errorf("cannot access directory: %w", err))
	}
	if !info.IsDir() {
		return userError(codeUsage, fmt.Errorf("not a directory: %s", dir))
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

	// Bind the listener synchronously, before any success output: a busy
	// port must be known now, not discovered later from inside the
	// goroutine below after "Serving presentation from..." has already
	// printed. An explicitly requested port (--port) that is busy is a
	// hard error; the default port falls back to the next one instead, so
	// two tap serve processes can run side by side without flags.
	listener, err := listenOnAvailablePort(servePort, cmd.Flags().Changed("port"), "tap serve")
	if err != nil {
		return userError(codeUsage, err)
	}
	boundPort := servePort
	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
		boundPort = tcpAddr.Port
	}

	httpServer := &http.Server{
		Handler:           handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Display startup message
	fmt.Println()
	Success("  Serving presentation from %s\n", dir)
	fmt.Println()
	fmt.Printf("  Local:   http://localhost:%d\n", boundPort)
	fmt.Printf("  Network: http://0.0.0.0:%d\n", boundPort)
	fmt.Println()
	Muted("  Press Ctrl+C to stop\n")
	fmt.Println()

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Serve on the listener already bound above, in a goroutine.
	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	// Wait for signal or error
	select {
	case err := <-errCh:
		return internalError(codeInternal, fmt.Errorf("server error: %w", err))
	case <-sigCh:
		fmt.Println()
		Info("Shutting down server...\n")
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(ctx); err != nil {
		return internalError(codeInternal, fmt.Errorf("error during shutdown: %w", err))
	}

	Successln("Server stopped.")
	return nil
}

func init() {
	// Register the serve command with root
	rootCmd.AddCommand(serveCmd)

	// Command-specific flags
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 3000, "port for the server")
}
