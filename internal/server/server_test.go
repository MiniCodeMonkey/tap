package server

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

func TestNew(t *testing.T) {
	tests := []struct {
		name string
		port int
		want string
	}{
		{"default port", 3000, "0.0.0.0:3000"},
		{"custom port", 8080, "0.0.0.0:8080"},
		{"low port", 80, "0.0.0.0:80"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := New(tt.port)
			if s.Addr() != tt.want {
				t.Errorf("New(%d).Addr() = %q, want %q", tt.port, s.Addr(), tt.want)
			}
		})
	}
}

func TestPort(t *testing.T) {
	s := New(3000)
	if got := s.Port(); got != 3000 {
		t.Errorf("Port() = %d, want 3000", got)
	}
}

func TestSetGetPresentation(t *testing.T) {
	s := New(3000)

	// Initially nil
	if got := s.GetPresentation(); got != nil {
		t.Errorf("GetPresentation() = %v, want nil", got)
	}

	// Set presentation
	pres := &transformer.TransformedPresentation{
		Config: *config.DefaultConfig(),
		Slides: []transformer.TransformedSlide{
			{Index: 0, HTML: "<h1>Test</h1>", Layout: "title"},
		},
	}
	s.SetPresentation(pres)

	// Get presentation
	got := s.GetPresentation()
	if got == nil {
		t.Fatal("GetPresentation() = nil, want non-nil")
	}
	if len(got.Slides) != 1 {
		t.Errorf("GetPresentation().Slides length = %d, want 1", len(got.Slides))
	}
	if got.Slides[0].HTML != "<h1>Test</h1>" {
		t.Errorf("GetPresentation().Slides[0].HTML = %q, want %q", got.Slides[0].HTML, "<h1>Test</h1>")
	}
}

func TestStartAndShutdown(t *testing.T) {
	// Use a high port to avoid conflicts
	s := New(0) // Port 0 lets the OS assign an available port

	// Register a simple handler
	s.RegisterHandlerFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "ok")
	})

	// Create a listener to get an available port
	if err := s.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// Give the server a moment to start
	time.Sleep(10 * time.Millisecond)

	if !s.IsStarted() {
		t.Error("IsStarted() = false, want true")
	}

	// Shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := s.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() error = %v", err)
	}
}

func TestStartAlreadyStarted(t *testing.T) {
	s := New(0)

	if err := s.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	// Try to start again
	if err := s.Start(); err == nil {
		t.Error("Start() on already started server should return error")
	}
}

func TestShutdownNotStarted(t *testing.T) {
	s := New(3000)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Shutdown on non-started server should not error
	if err := s.Shutdown(ctx); err != nil {
		t.Errorf("Shutdown() on non-started server error = %v", err)
	}
}

func TestRegisterHandler(t *testing.T) {
	s := New(0)

	// Register handlers
	s.RegisterHandlerFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "test handler")
	})

	s.RegisterHandler("/handler", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, "handler")
	}))

	if err := s.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	// Give the server a moment to start
	time.Sleep(10 * time.Millisecond)

	// Test /test handler
	resp, err := http.Get(fmt.Sprintf("http://%s/test", s.Addr()))
	if err != nil {
		t.Fatalf("GET /test error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /test status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "test handler" {
		t.Errorf("GET /test body = %q, want %q", string(body), "test handler")
	}

	// Test /handler handler
	resp2, err := http.Get(fmt.Sprintf("http://%s/handler", s.Addr()))
	if err != nil {
		t.Fatalf("GET /handler error = %v", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode != http.StatusOK {
		t.Errorf("GET /handler status = %d, want %d", resp2.StatusCode, http.StatusOK)
	}

	body2, _ := io.ReadAll(resp2.Body)
	if string(body2) != "handler" {
		t.Errorf("GET /handler body = %q, want %q", string(body2), "handler")
	}
}

func TestIsStarted(t *testing.T) {
	s := New(0)

	if s.IsStarted() {
		t.Error("IsStarted() = true before Start(), want false")
	}

	if err := s.Start(); err != nil {
		t.Fatalf("Start() error = %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.Shutdown(ctx)
	}()

	if !s.IsStarted() {
		t.Error("IsStarted() = false after Start(), want true")
	}
}

func TestConcurrentSetGetPresentation(t *testing.T) {
	s := New(3000)

	// Run concurrent reads and writes
	done := make(chan bool)
	go func() {
		for i := 0; i < 100; i++ {
			pres := &transformer.TransformedPresentation{
				Config: *config.DefaultConfig(),
				Slides: []transformer.TransformedSlide{
					{Index: i, HTML: fmt.Sprintf("<h1>Slide %d</h1>", i), Layout: "title"},
				},
			}
			s.SetPresentation(pres)
		}
		done <- true
	}()

	go func() {
		for i := 0; i < 100; i++ {
			_ = s.GetPresentation()
		}
		done <- true
	}()

	// Wait for both goroutines
	<-done
	<-done

	// If we get here without a race condition, the test passes
}
