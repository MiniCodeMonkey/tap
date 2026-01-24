package server

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tapsh/tap/internal/config"
	"github.com/tapsh/tap/internal/parser"
	"github.com/tapsh/tap/internal/transformer"
)

// generateBenchmarkPresentation creates a presentation with the specified number of slides.
func generateBenchmarkPresentation(slideCount int) ([]byte, *config.Config) {
	var buf bytes.Buffer

	// Add frontmatter
	buf.WriteString(`---
title: Hot Reload Performance Test
theme: paper
author: Test Author
aspectRatio: "16:9"
transition: fade
---

`)

	// Generate slides
	for i := 0; i < slideCount; i++ {
		if i > 0 {
			buf.WriteString("\n---\n\n")
		}

		switch i % 3 {
		case 0:
			buf.WriteString(fmt.Sprintf("# Slide %d\n\nTitle slide content.\n", i+1))
		case 1:
			buf.WriteString(fmt.Sprintf("## Section %d\n\n- Point 1\n- Point 2\n- Point 3\n", i+1))
		case 2:
			buf.WriteString(fmt.Sprintf("## Content %d\n\nParagraph with some text.\n\n<!-- pause -->\n\nMore content here.\n", i+1))
		}
	}

	cfg := config.DefaultConfig()
	cfg.Title = "Hot Reload Performance Test"
	cfg.Theme = "minimal"
	cfg.AspectRatio = "16:9"
	cfg.Transition = "fade"

	return buf.Bytes(), cfg
}

// BenchmarkHotReloadLatency benchmarks the full hot reload cycle.
// This measures the time from file change detection to WebSocket broadcast.
// Target: <200ms for hot reload latency.
func BenchmarkHotReloadLatency(b *testing.B) {
	// Create a temporary markdown file
	tmpDir := b.TempDir()
	mdFile := tmpDir + "/presentation.md"
	content, _ := generateBenchmarkPresentation(50)

	if err := os.WriteFile(mdFile, content, 0644); err != nil {
		b.Fatalf("Failed to write markdown file: %v", err)
	}

	// Create watcher
	watcher, err := NewWatcher(mdFile)
	if err != nil {
		b.Fatalf("Failed to create watcher: %v", err)
	}

	// Count change callbacks
	var changeCount atomic.Int64
	changeChan := make(chan struct{}, 100)

	watcher.SetOnChange(func(_ string) {
		changeCount.Add(1)
		changeChan <- struct{}{}
	})

	// Set a very short debounce for benchmarking
	watcher.SetDebounceTime(10 * time.Millisecond)

	if err := watcher.Start(); err != nil {
		b.Fatalf("Failed to start watcher: %v", err)
	}
	defer watcher.Stop()

	// Wait for watcher to be ready
	time.Sleep(100 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate file change
		newContent := fmt.Sprintf("%s\n<!-- change %d -->", string(content), i)
		if err := os.WriteFile(mdFile, []byte(newContent), 0644); err != nil {
			b.Fatalf("Failed to write file: %v", err)
		}

		// Wait for change to be detected
		select {
		case <-changeChan:
			// Change detected
		case <-time.After(500 * time.Millisecond):
			b.Fatal("Timeout waiting for file change detection")
		}
	}
}

// BenchmarkServerRoutePerformance benchmarks HTTP route handling.
func BenchmarkServerRoutePerformance(b *testing.B) {
	content, cfg := generateBenchmarkPresentation(50)
	p := parser.New()
	pres, err := p.Parse(content)
	if err != nil {
		b.Fatalf("Parse error: %v", err)
	}

	trans := transformer.New(cfg)
	transformed := trans.Transform(pres)

	srv := New(0)
	srv.SetPresentation(transformed)
	srv.SetupRoutes()

	b.Run("GET_Index", func(b *testing.B) {
		req := httptest.NewRequest(http.MethodGet, "/", nil)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)
		}
	})

	b.Run("GET_API_Presentation", func(b *testing.B) {
		req := httptest.NewRequest(http.MethodGet, "/api/presentation", nil)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)
		}
	})

	b.Run("GET_Presenter", func(b *testing.B) {
		req := httptest.NewRequest(http.MethodGet, "/presenter", nil)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)
		}
	})

	b.Run("GET_QR", func(b *testing.B) {
		req := httptest.NewRequest(http.MethodGet, "/qr", nil)

		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			srv.mux.ServeHTTP(w, req)
		}
	})
}

// BenchmarkWebSocketHubBroadcast benchmarks WebSocket hub broadcasting.
func BenchmarkWebSocketHubBroadcast(b *testing.B) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	b.Run("Broadcast_NoClients", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = hub.Broadcast(Message{Type: "reload"})
		}
	})

	// Note: Full client simulation is complex and would require actual WebSocket connections
	// This benchmark focuses on the hub's message handling without clients
}

// BenchmarkWatcherDebounce benchmarks the debounce logic.
func BenchmarkWatcherDebounce(b *testing.B) {
	tmpDir := b.TempDir()
	mdFile := tmpDir + "/test.md"
	if err := os.WriteFile(mdFile, []byte("# Test"), 0644); err != nil {
		b.Fatalf("Failed to write file: %v", err)
	}

	watcher, err := NewWatcher(mdFile)
	if err != nil {
		b.Fatalf("Failed to create watcher: %v", err)
	}

	var callCount atomic.Int64
	watcher.SetOnChange(func(_ string) {
		callCount.Add(1)
	})

	watcher.SetDebounceTime(50 * time.Millisecond)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Simulate rapid file writes (should be debounced)
		for j := 0; j < 5; j++ {
			_ = os.WriteFile(mdFile, []byte(fmt.Sprintf("# Test %d-%d", i, j)), 0644)
		}
		time.Sleep(60 * time.Millisecond) // Wait for debounce
	}
}

// BenchmarkPresentationJSON benchmarks JSON serialization of presentations.
func BenchmarkPresentationJSON(b *testing.B) {
	content, cfg := generateBenchmarkPresentation(100)
	p := parser.New()
	pres, err := p.Parse(content)
	if err != nil {
		b.Fatalf("Parse error: %v", err)
	}

	trans := transformer.New(cfg)
	transformed := trans.Transform(pres)

	srv := New(0)
	srv.SetPresentation(transformed)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/presentation", nil)
		w := httptest.NewRecorder()
		srv.mux.ServeHTTP(w, req)
	}
}

// TestHotReloadLatency_PerformanceTarget tests that hot reload latency is under 200ms.
func TestHotReloadLatency_PerformanceTarget(t *testing.T) {
	// Create a temporary markdown file
	tmpDir := t.TempDir()
	mdFile := tmpDir + "/presentation.md"
	content, _ := generateBenchmarkPresentation(50)

	if err := os.WriteFile(mdFile, content, 0644); err != nil {
		t.Fatalf("Failed to write markdown file: %v", err)
	}

	// Create watcher
	watcher, err := NewWatcher(mdFile)
	if err != nil {
		t.Fatalf("Failed to create watcher: %v", err)
	}

	changeChan := make(chan time.Time, 1)
	watcher.SetOnChange(func(_ string) {
		changeChan <- time.Now()
	})

	watcher.SetDebounceTime(50 * time.Millisecond)

	if err := watcher.Start(); err != nil {
		t.Fatalf("Failed to start watcher: %v", err)
	}
	defer watcher.Stop()

	// Wait for watcher to be ready
	time.Sleep(100 * time.Millisecond)

	// Measure hot reload latency
	var totalLatency time.Duration
	const numTests = 5

	for i := 0; i < numTests; i++ {
		// Record start time just before writing
		start := time.Now()

		// Simulate file change
		newContent := fmt.Sprintf("%s\n<!-- change %d -->\n", string(content), i)
		if err := os.WriteFile(mdFile, []byte(newContent), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}

		// Wait for change to be detected
		select {
		case changeTime := <-changeChan:
			latency := changeTime.Sub(start)
			totalLatency += latency
			t.Logf("Hot reload iteration %d: %v", i+1, latency)
		case <-time.After(500 * time.Millisecond):
			t.Fatalf("Timeout waiting for file change detection on iteration %d", i+1)
		}

		// Small delay between tests
		time.Sleep(100 * time.Millisecond)
	}

	avgLatency := totalLatency / numTests
	t.Logf("Average hot reload latency: %v", avgLatency)

	// Target: <200ms
	if avgLatency > 200*time.Millisecond {
		t.Errorf("Performance target missed: average hot reload latency was %v (target: <200ms)", avgLatency)
	}
}

// BenchmarkServerStartStop benchmarks server startup and shutdown.
func BenchmarkServerStartStop(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		srv := New(0)
		srv.SetupRoutes()
		if err := srv.Start(); err != nil {
			b.Fatalf("Start error: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		if err := srv.Shutdown(ctx); err != nil {
			b.Fatalf("Shutdown error: %v", err)
		}
		cancel()
	}
}
