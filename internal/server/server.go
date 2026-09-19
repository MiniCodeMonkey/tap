// Package server provides the HTTP dev server for tap presentations.
package server

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/driver"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// Server is the HTTP server for serving presentations in development mode.
type Server struct {
	// Fields ordered by size for better memory alignment
	presentation          *transformer.TransformedPresentation
	registry              *driver.Registry
	httpServer            *http.Server
	mux                   *http.ServeMux
	shutdownCh            chan struct{}
	addr                  string
	presenterPassword     string
	presenterSessionToken string
	customThemePath       string
	baseDir               string // Base directory for serving local files (images, etc.)
	componentBundles      *ComponentBundleStore
	// allowedHosts is the --allow-origin flag reduced to bare hosts (see
	// allowedHostsFromOrigins), checked by requireAllowedHost against a
	// request's Host header on the routes that matter against DNS
	// rebinding. Mirrors WebSocketHub.allowedHosts; tap dev sets both from
	// the same flag value.
	allowedHosts map[string]struct{}
	mu           sync.RWMutex
	started      bool
}

// New creates a new Server bound to the specified port on 0.0.0.0, so a
// presenter can open it from another device on the network. Use
// NewWithHost to bind a narrower host, such as a build-time helper server
// nothing outside the machine needs to reach.
func New(port int) *Server {
	return NewWithHost(port, "0.0.0.0")
}

// NewWithHost creates a new Server bound to the specified host and port.
func NewWithHost(port int, host string) *Server {
	s := &Server{
		addr:             fmt.Sprintf("%s:%d", host, port),
		mux:              http.NewServeMux(),
		shutdownCh:       make(chan struct{}),
		componentBundles: NewComponentBundleStore(),
	}

	s.httpServer = &http.Server{
		Addr:              s.addr,
		Handler:           s.mux,
		ReadHeaderTimeout: 10 * time.Second,
	}

	return s
}

// SetPresentation sets the current presentation data.
// This method is thread-safe and can be called while the server is running.
func (s *Server) SetPresentation(pres *transformer.TransformedPresentation) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.presentation = pres
}

// GetPresentation returns the current presentation data.
// This method is thread-safe.
func (s *Server) GetPresentation() *transformer.TransformedPresentation {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.presentation
}

// Addr returns the server address.
// After Start() is called, this returns the actual bound address
// (useful when port 0 is used to get an ephemeral port).
func (s *Server) Addr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.addr
}

// Port returns the server port.
// After Start() is called, this returns the actual bound port
// (useful when port 0 is used to get an ephemeral port).
func (s *Server) Port() int {
	s.mu.RLock()
	addr := s.addr
	s.mu.RUnlock()

	_, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	p, err := strconv.Atoi(port)
	if err != nil {
		return 0
	}
	return p
}

// RegisterHandler registers an HTTP handler for the given pattern.
// This should be called before Start().
func (s *Server) RegisterHandler(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

// RegisterHandlerFunc registers an HTTP handler function for the given pattern.
// This should be called before Start().
func (s *Server) RegisterHandlerFunc(pattern string, handler http.HandlerFunc) {
	s.mux.HandleFunc(pattern, handler)
}

// Start starts the HTTP server in a goroutine.
// It returns immediately after the server starts listening.
// Use Shutdown() to stop the server.
func (s *Server) Start() error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return fmt.Errorf("server already started")
	}
	s.started = true
	s.mu.Unlock()

	// Create listener to verify we can bind to the port
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		s.mu.Lock()
		s.started = false
		s.mu.Unlock()
		return fmt.Errorf("failed to listen on %s: %w", s.addr, err)
	}

	// Update addr with the actual address (important when using port 0)
	s.mu.Lock()
	s.addr = listener.Addr().String()
	s.mu.Unlock()

	// Start serving in a goroutine
	go func() {
		if err := s.httpServer.Serve(listener); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
		}
	}()

	return nil
}

// StartWithGracefulShutdown starts the HTTP server and sets up signal handling
// for graceful shutdown on SIGINT and SIGTERM.
// This method blocks until shutdown is complete.
func (s *Server) StartWithGracefulShutdown() error {
	if err := s.Start(); err != nil {
		return err
	}

	// Set up signal handling
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Wait for signal or shutdown request
	select {
	case <-sigCh:
	case <-s.shutdownCh:
	}

	// Clean up signal handler
	signal.Stop(sigCh)

	// Graceful shutdown with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return s.httpServer.Shutdown(ctx)
}

// Shutdown gracefully shuts down the server.
// It waits for active connections to complete with a 10-second timeout.
func (s *Server) Shutdown(ctx context.Context) error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	// Signal the shutdown channel if StartWithGracefulShutdown is waiting
	select {
	case s.shutdownCh <- struct{}{}:
	default:
	}

	return s.httpServer.Shutdown(ctx)
}

// IsStarted returns whether the server has been started.
func (s *Server) IsStarted() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.started
}

// SetPresenterPassword sets the presenter password for protected presenter view.
func (s *Server) SetPresenterPassword(password string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.presenterPassword = password
}

// GetPresenterPassword returns the presenter password.
func (s *Server) GetPresenterPassword() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.presenterPassword
}

// SetPresenterSessionToken sets the random per-process token handlePresenter
// stores in the presenter auth cookie once a request proves it knows the
// presenter password (see GeneratePresenterSessionToken). The caller
// generates this once per process and sets it on every candidate Server and
// on the WebSocketHub, so a cookie one of them issues validates against the
// others too.
func (s *Server) SetPresenterSessionToken(token string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.presenterSessionToken = token
}

// GetPresenterSessionToken returns the current presenter session token.
func (s *Server) GetPresenterSessionToken() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.presenterSessionToken
}

// GeneratePresenterSessionToken returns a fresh random 32-byte token,
// hex-encoded, for use as the presenter auth cookie's value. It never
// carries the actual presenter password (see handlePresenter in routes.go):
// Go's cookie jar sanitizes cookie values, silently changing a raw password
// that contains a semicolon, quote, backslash, space, or non-ASCII
// character, which would otherwise break the cookie compare a real
// presenter password could easily trigger.
func GeneratePresenterSessionToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("generate presenter session token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// SetCustomThemePath sets the path to a custom CSS theme file.
func (s *Server) SetCustomThemePath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.customThemePath = path
}

// SetAllowedOrigins sets the --allow-origin values requireAllowedHost
// checks a request's Host header against, on top of localhost, a loopback,
// private, or link-local IP, a ".local" name, and this machine's own
// hostname (see isAllowedHost). tap dev calls this with the same value it
// passes to WebSocketHub.SetAllowedOrigins.
func (s *Server) SetAllowedOrigins(origins []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allowedHosts = allowedHostsFromOrigins(origins)
}

// requireAllowedHost wraps next so it only runs for a request whose Host
// header is on the allow-list (see isAllowedHost); anything else gets 403.
// This is the DNS rebinding defense for tap dev's HTTP routes: a same-host
// compare alone (as the WebSocket origin check used to rely on) is not
// enough, since a hostile domain an attacker controls can resolve to
// 127.0.0.1 and still send that exact Host header.
func (s *Server) requireAllowedHost(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		allowedHosts := s.allowedHosts
		s.mu.RUnlock()
		if !isAllowedHost(r.Host, allowedHosts) {
			http.Error(w, "Forbidden: host not allowed; use --allow-origin to allow it", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

// GetCustomThemePath returns the path to the custom CSS theme file.
func (s *Server) GetCustomThemePath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.customThemePath
}

// SetBaseDir sets the base directory for serving local files.
func (s *Server) SetBaseDir(dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.baseDir = dir
}

// GetBaseDir returns the base directory for serving local files.
func (s *Server) GetBaseDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.baseDir
}

// SetComponentBundles atomically replaces the dev server's set of component
// bundle files served under /components/.
func (s *Server) SetComponentBundles(files map[string]ComponentBundleFile) {
	s.mu.RLock()
	store := s.componentBundles
	s.mu.RUnlock()
	store.Set(files)
}

// ComponentBundles returns the server's component bundle store.
func (s *Server) ComponentBundles() *ComponentBundleStore {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.componentBundles
}
