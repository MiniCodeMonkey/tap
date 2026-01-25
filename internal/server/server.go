// Package server provides the HTTP dev server for tap presentations.
package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/tapsh/tap/internal/driver"
	"github.com/tapsh/tap/internal/transformer"
)

// Server is the HTTP server for serving presentations in development mode.
type Server struct {
	// Fields ordered by size for better memory alignment
	presentation      *transformer.TransformedPresentation
	registry          *driver.Registry
	httpServer        *http.Server
	mux               *http.ServeMux
	shutdownCh        chan struct{}
	addr              string
	presenterPassword string
	customThemePath   string
	baseDir           string // Base directory for serving local files (images, etc.)
	mu                sync.RWMutex
	started           bool
}

// New creates a new Server bound to the specified port.
// The server listens on 0.0.0.0 to allow network access.
func New(port int) *Server {
	s := &Server{
		addr:       fmt.Sprintf("0.0.0.0:%d", port),
		mux:        http.NewServeMux(),
		shutdownCh: make(chan struct{}),
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

// SetCustomThemePath sets the path to a custom CSS theme file.
func (s *Server) SetCustomThemePath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.customThemePath = path
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
