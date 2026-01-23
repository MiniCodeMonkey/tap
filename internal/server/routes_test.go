package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/tapsh/tap/internal/config"
	"github.com/tapsh/tap/internal/transformer"
)

func TestHandleIndex(t *testing.T) {
	s := New(0)

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	s.handleIndex(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/html") {
		t.Errorf("expected Content-Type text/html, got %s", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Check for expected content
	if !strings.Contains(bodyStr, "<!DOCTYPE html>") {
		t.Error("expected HTML doctype")
	}
	if !strings.Contains(bodyStr, "Tap") {
		t.Error("expected 'Tap' in body")
	}
}

func TestHandlePresenter(t *testing.T) {
	s := New(0)

	req := httptest.NewRequest(http.MethodGet, "/presenter", nil)
	w := httptest.NewRecorder()

	s.handlePresenter(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/html") {
		t.Errorf("expected Content-Type text/html, got %s", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Check for presenter view content
	if !strings.Contains(bodyStr, "Presenter View") {
		t.Error("expected 'Presenter View' in body")
	}
}

func TestHandleAPIPresentation_NoPresentation(t *testing.T) {
	s := New(0)

	req := httptest.NewRequest(http.MethodGet, "/api/presentation", nil)
	w := httptest.NewRecorder()

	s.handleAPIPresentation(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var result map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	if result["error"] != "No presentation loaded" {
		t.Errorf("expected error message 'No presentation loaded', got '%s'", result["error"])
	}
}

func TestHandleAPIPresentation_WithPresentation(t *testing.T) {
	s := New(0)

	// Set up a test presentation
	cfg := config.DefaultConfig()
	cfg.Title = "Test Presentation"
	cfg.Theme = "minimal"

	pres := &transformer.TransformedPresentation{
		Config: *cfg,
		Slides: []transformer.TransformedSlide{
			{
				Index:  0,
				Layout: "title",
				HTML:   "<h1>Hello World</h1>",
			},
			{
				Index:  1,
				Layout: "default",
				HTML:   "<p>Content here</p>",
			},
		},
	}
	s.SetPresentation(pres)

	req := httptest.NewRequest(http.MethodGet, "/api/presentation", nil)
	w := httptest.NewRecorder()

	s.handleAPIPresentation(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "application/json") {
		t.Errorf("expected Content-Type application/json, got %s", contentType)
	}

	var result transformer.TransformedPresentation
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode JSON response: %v", err)
	}

	if result.Config.Title != "Test Presentation" {
		t.Errorf("expected title 'Test Presentation', got '%s'", result.Config.Title)
	}

	if len(result.Slides) != 2 {
		t.Errorf("expected 2 slides, got %d", len(result.Slides))
	}

	if result.Slides[0].Layout != "title" {
		t.Errorf("expected first slide layout 'title', got '%s'", result.Slides[0].Layout)
	}
}

func TestHandleQR(t *testing.T) {
	s := New(0)

	req := httptest.NewRequest(http.MethodGet, "/qr", nil)
	w := httptest.NewRecorder()

	s.handleQR(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	contentType := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(contentType, "text/html") {
		t.Errorf("expected Content-Type text/html, got %s", contentType)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	// Check for QR page content
	if !strings.Contains(bodyStr, "QR Code") {
		t.Error("expected 'QR Code' in body")
	}
}

func TestSetupRoutes(t *testing.T) {
	s := New(0)
	s.SetupRoutes()

	// Set up a presentation for the API endpoint
	cfg := config.DefaultConfig()
	pres := &transformer.TransformedPresentation{
		Config: *cfg,
		Slides: []transformer.TransformedSlide{
			{Index: 0, Layout: "title", HTML: "<h1>Test</h1>"},
		},
	}
	s.SetPresentation(pres)

	// Start the server to test routes through actual HTTP
	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Shutdown(context.Background())

	baseURL := "http://" + s.Addr()

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		expectedType   string
		expectedBody   string
	}{
		{
			name:           "index route",
			path:           "/",
			expectedStatus: http.StatusOK,
			expectedType:   "text/html",
			expectedBody:   "Tap",
		},
		{
			name:           "presenter route",
			path:           "/presenter",
			expectedStatus: http.StatusOK,
			expectedType:   "text/html",
			expectedBody:   "Presenter View",
		},
		{
			name:           "api presentation route",
			path:           "/api/presentation",
			expectedStatus: http.StatusOK,
			expectedType:   "application/json",
			expectedBody:   `"title"`,
		},
		{
			name:           "qr route",
			path:           "/qr",
			expectedStatus: http.StatusOK,
			expectedType:   "text/html",
			expectedBody:   "QR Code",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resp, err := http.Get(baseURL + tt.path)
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			defer resp.Body.Close()

			if resp.StatusCode != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
			}

			contentType := resp.Header.Get("Content-Type")
			if !strings.Contains(contentType, tt.expectedType) {
				t.Errorf("expected Content-Type containing %s, got %s", tt.expectedType, contentType)
			}

			body, _ := io.ReadAll(resp.Body)
			if !strings.Contains(string(body), tt.expectedBody) {
				t.Errorf("expected body to contain '%s'", tt.expectedBody)
			}
		})
	}
}

func TestLoggingResponseWriter(t *testing.T) {
	w := httptest.NewRecorder()
	lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

	// Test default status code
	if lrw.statusCode != http.StatusOK {
		t.Errorf("expected default status code %d, got %d", http.StatusOK, lrw.statusCode)
	}

	// Test WriteHeader captures status code
	lrw.WriteHeader(http.StatusNotFound)
	if lrw.statusCode != http.StatusNotFound {
		t.Errorf("expected status code %d after WriteHeader, got %d", http.StatusNotFound, lrw.statusCode)
	}

	// Verify underlying writer received the status code
	if w.Code != http.StatusNotFound {
		t.Errorf("expected underlying writer status code %d, got %d", http.StatusNotFound, w.Code)
	}
}

func TestLoggingMiddleware(t *testing.T) {
	// Create a handler that sets a specific status code
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte("created"))
	})

	// Wrap with logging middleware
	logged := loggingMiddleware(handler)

	req := httptest.NewRequest(http.MethodPost, "/test", nil)
	w := httptest.NewRecorder()

	// This will also print a log line, which is expected behavior
	logged.ServeHTTP(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected status %d, got %d", http.StatusCreated, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "created" {
		t.Errorf("expected body 'created', got '%s'", string(body))
	}
}

func TestAPIPresentation_JSONEncodesAllFields(t *testing.T) {
	s := New(0)

	// Set up a comprehensive test presentation
	cfg := config.DefaultConfig()
	cfg.Title = "Full Test"
	cfg.Theme = "terminal"
	cfg.AspectRatio = "16:9"
	cfg.Transition = "slide"

	pres := &transformer.TransformedPresentation{
		Config: *cfg,
		Slides: []transformer.TransformedSlide{
			{
				Index:      0,
				Layout:     "title",
				HTML:       "<h1>Welcome</h1>",
				Transition: "fade",
				Notes:      "Opening slide notes",
			},
			{
				Index:  1,
				Layout: "code-focus",
				HTML:   "<pre><code>console.log('hi')</code></pre>",
				CodeBlocks: []transformer.TransformedCodeBlock{
					{
						Language:   "javascript",
						Code:       "console.log('hi')",
						Driver:     "shell",
						Connection: "",
					},
				},
			},
			{
				Index:  2,
				Layout: "default",
				HTML:   "<p>First</p><p>Second</p>",
				Fragments: []transformer.TransformedFragment{
					{Index: 0, Content: "<p>First</p>"},
					{Index: 1, Content: "<p>Second</p>"},
				},
			},
		},
	}
	s.SetPresentation(pres)

	req := httptest.NewRequest(http.MethodGet, "/api/presentation", nil)
	w := httptest.NewRecorder()

	s.handleAPIPresentation(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var result transformer.TransformedPresentation
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	// Verify config
	if result.Config.Theme != "terminal" {
		t.Errorf("expected theme 'terminal', got '%s'", result.Config.Theme)
	}
	if result.Config.Transition != "slide" {
		t.Errorf("expected transition 'slide', got '%s'", result.Config.Transition)
	}

	// Verify slides
	if len(result.Slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(result.Slides))
	}

	// Check first slide has notes
	if result.Slides[0].Notes != "Opening slide notes" {
		t.Errorf("expected notes 'Opening slide notes', got '%s'", result.Slides[0].Notes)
	}

	// Check second slide has code blocks
	if len(result.Slides[1].CodeBlocks) != 1 {
		t.Errorf("expected 1 code block, got %d", len(result.Slides[1].CodeBlocks))
	}
	if result.Slides[1].CodeBlocks[0].Driver != "shell" {
		t.Errorf("expected driver 'shell', got '%s'", result.Slides[1].CodeBlocks[0].Driver)
	}

	// Check third slide has fragments
	if len(result.Slides[2].Fragments) != 2 {
		t.Errorf("expected 2 fragments, got %d", len(result.Slides[2].Fragments))
	}
}
