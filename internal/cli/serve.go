// Package cli provides the command-line interface for Tap.
package cli

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

// Flags for the serve command
var (
	servePort int
	serveJSON bool
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

With --json, tap listens on 127.0.0.1 only, prints one line,
{"ok":true,"dir":...,"port":...,"url":...} (or, on a failure, one line
{"ok":false,"error":{...}}), logs no requests, and exits
when its standard input closes, for a program that opens the site and
holds the other end.

Examples:
  tap serve                    # Serve from dist/ on port 3000
  tap serve public             # Serve from custom directory
  tap serve --port 8080        # Use custom port
  tap serve ./build -p 8080    # Both options together
  tap serve dist --port 0 --json`,
	Args:        cobra.MaximumNArgs(1),
	RunE:        runServe,
	Annotations: map[string]string{oneLineJSONAnnotation: "true"},
}

func runServe(cmd *cobra.Command, args []string) error {
	dir := "dist"
	if len(args) > 0 {
		dir = args[0]
	}
	httpServer, listener, err := startServe(dir, servePort, cmd.Flags().Changed("port"), serveJSON, cmd.OutOrStdout())
	if err != nil {
		return err
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// With --json the caller is a program holding the other end of stdin:
	// when it goes, stdin reaches EOF and the server stops, so a preview
	// server never outlives the app that opened it. A person's terminal
	// never closes stdin, so the human mode does not watch it.
	stdinClosed := make(chan struct{})
	if serveJSON {
		go func() {
			_, _ = io.Copy(io.Discard, serveStandardInput)
			close(stdinClosed)
		}()
	}

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		return internalError(codeInternal, fmt.Errorf("server error: %w", err))
	case <-sigCh:
		if !serveJSON {
			fmt.Fprintln(cmd.OutOrStdout())
			infoColor.Fprintf(cmd.OutOrStdout(), "Shutting down server...\n")
		}
	case <-stdinClosed:
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		return internalError(codeInternal, fmt.Errorf("error during shutdown: %w", err))
	}
	if !serveJSON {
		successColor.Fprintln(cmd.OutOrStdout(), "Server stopped.")
	}
	return nil
}

// serveStandardInput is what tap serve --json watches for EOF. A test
// replaces it with a pipe it closes.
var serveStandardInput io.Reader = os.Stdin

// serveReady is the --json ready line of tap serve.
type serveReady struct {
	Dir  string `json:"dir"`
	Port int    `json:"port"`
	URL  string `json:"url"`
}

// startServe checks dir, binds the listener and prints what a person or a
// program needs to open the site: the banner, or with jsonMode one ready
// line on out and nothing else there (no request log). Every human line
// goes through out too (the color helpers' Fprint forms), so a test reads
// the banner. jsonMode binds loopback: the site is for the program's own
// browser, as --app binds. The caller serves on the listener and shuts
// the server down. Separate from runServe so a test can bind, read the
// line and stop, without a signal.
func startServe(dir string, port int, explicitPort, jsonMode bool, out io.Writer) (*http.Server, net.Listener, error) {
	muted := color.New(color.FgHiBlack)
	info, err := os.Stat(dir)
	if os.IsNotExist(err) {
		if !jsonMode {
			fmt.Fprintln(out)
			muted.Fprintf(out, "  Hint: Run 'tap build [deck]' first to generate static files.\n")
		}
		return nil, nil, userError(codeDeckNotFound, fmt.Errorf("directory does not exist: %s", dir))
	}
	if err != nil {
		return nil, nil, userError(codeUsage, fmt.Errorf("cannot access directory: %w", err))
	}
	if !info.IsDir() {
		return nil, nil, userError(codeUsage, fmt.Errorf("not a directory: %s", dir))
	}

	fs := http.FileServer(http.Dir(dir))
	handler := fs
	if !jsonMode {
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			fs.ServeHTTP(w, r)
			infoColor.Fprint(out, "GET ")
			fmt.Fprintf(out, "%s ", r.URL.Path)
			muted.Fprintf(out, "(%s)\n", time.Since(start).Round(time.Microsecond))
		})
	}

	host := "0.0.0.0"
	if jsonMode {
		host = "127.0.0.1"
	}
	listener, err := listenOnAvailablePort(host, port, explicitPort, "tap serve")
	if err != nil {
		return nil, nil, userError(codeUsage, err)
	}
	boundPort := port
	if tcpAddr, ok := listener.Addr().(*net.TCPAddr); ok {
		boundPort = tcpAddr.Port
	}
	httpServer := &http.Server{Handler: handler, ReadHeaderTimeout: 10 * time.Second}

	if jsonMode {
		// The URL names the address bound: a browser trying ::1 first would find nothing there.
		if err := printJSONLine(out, serveReady{Dir: dir, Port: boundPort, URL: fmt.Sprintf("http://127.0.0.1:%d", boundPort)}); err != nil {
			_ = listener.Close()
			return nil, nil, err
		}
		return httpServer, listener, nil
	}
	fmt.Fprintln(out)
	successColor.Fprintf(out, "  Serving presentation from %s\n", dir)
	fmt.Fprintln(out)
	fmt.Fprintf(out, "  Local:   http://localhost:%d\n", boundPort)
	fmt.Fprintf(out, "  Network: http://0.0.0.0:%d\n", boundPort)
	fmt.Fprintln(out)
	muted.Fprintf(out, "  Press Ctrl+C to stop\n")
	fmt.Fprintln(out)
	return httpServer, listener, nil
}

func init() {
	// Register the serve command with root
	rootCmd.AddCommand(serveCmd)

	// Command-specific flags
	serveCmd.Flags().IntVarP(&servePort, "port", "p", 3000, "port for the server")
	serveCmd.Flags().BoolVar(&serveJSON, "json", false, "print one ready line as JSON and log no requests")
}
