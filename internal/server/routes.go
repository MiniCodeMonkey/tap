// Package server provides HTTP route handlers for the tap dev server.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/fatih/color"
	"github.com/tapsh/tap/embedded"
)

// Color definitions for request logging
var (
	methodColor  = color.New(color.FgCyan, color.Bold)
	pathColor    = color.New(color.FgWhite)
	statusOKColor = color.New(color.FgGreen)
	statusErrColor = color.New(color.FgRed)
	timeColor    = color.New(color.FgHiBlack)
)

// loggingMiddleware wraps an http.Handler and logs requests with colorized output.
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap the response writer to capture status code
		lrw := &loggingResponseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		// Call the next handler
		next.ServeHTTP(lrw, r)

		// Log the request with colors
		duration := time.Since(start)
		logRequest(r.Method, r.URL.Path, lrw.statusCode, duration)
	})
}

// loggingResponseWriter wraps http.ResponseWriter to capture the status code.
type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

// WriteHeader captures the status code before writing it.
func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}

// logRequest logs a request with colorized output.
func logRequest(method, path string, status int, duration time.Duration) {
	// Format status with appropriate color
	var statusStr string
	if status >= 200 && status < 400 {
		statusStr = statusOKColor.Sprintf("%d", status)
	} else {
		statusStr = statusErrColor.Sprintf("%d", status)
	}

	// Format duration
	durationStr := timeColor.Sprintf("%v", duration.Round(time.Microsecond))

	// Print the log line
	fmt.Printf("%s %s %s %s\n",
		methodColor.Sprint(method),
		pathColor.Sprint(path),
		statusStr,
		durationStr,
	)
}

// SetupRoutes configures all HTTP routes on the server.
// This should be called before Start().
func (s *Server) SetupRoutes() {
	// Create a mux that applies logging to all requests
	loggedMux := http.NewServeMux()

	// Register all routes
	loggedMux.HandleFunc("GET /", s.handleIndex)
	loggedMux.HandleFunc("GET /presenter", s.handlePresenter)
	loggedMux.HandleFunc("GET /api/presentation", s.handleAPIPresentation)
	loggedMux.HandleFunc("POST /api/execute", s.handleAPIExecute)
	loggedMux.HandleFunc("GET /qr", s.handleQR)

	// Wrap with logging middleware and set as the server handler
	s.httpServer.Handler = loggingMiddleware(loggedMux)
}

// handleIndex serves the main presentation viewer (index.html).
func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	// Serve embedded index.html
	content, err := embedded.GetIndexHTML()
	if err != nil {
		http.Error(w, "Failed to load index.html", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// handlePresenter serves the presenter view.
// If a presenter password is configured, requires ?key=<password> query parameter.
func (s *Server) handlePresenter(w http.ResponseWriter, r *http.Request) {
	// Check password protection
	password := s.GetPresenterPassword()
	if password != "" {
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "Forbidden: presenter password required. Use ?key=<password>", http.StatusForbidden)
			return
		}
		if key != password {
			http.Error(w, "Forbidden: incorrect presenter password", http.StatusForbidden)
			return
		}
	}

	// Serve embedded presenter.html
	content, err := embedded.GetPresenterHTML()
	if err != nil {
		http.Error(w, "Failed to load presenter.html", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// handleAPIPresentation returns the presentation data as JSON.
func (s *Server) handleAPIPresentation(w http.ResponseWriter, r *http.Request) {
	pres := s.GetPresentation()
	if pres == nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_ = json.NewEncoder(w).Encode(map[string]string{
			"error": "No presentation loaded",
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(pres); err != nil {
		// If encoding fails, we've already started writing the response
		// so we can't change the status code. Just log internally.
		fmt.Printf("Error encoding presentation JSON: %v\n", err)
	}
}

// handleQR serves a page with QR codes for the audience and presenter URLs.
func (s *Server) handleQR(w http.ResponseWriter, r *http.Request) {
	cfg := QRConfig{
		Port:              s.Port(),
		PresenterPassword: s.presenterPassword,
	}

	audienceURL, err := GenerateAudienceURL(cfg)
	if err != nil {
		http.Error(w, "Failed to generate audience URL", http.StatusInternalServerError)
		return
	}

	presenterURL, err := GeneratePresenterURL(cfg)
	if err != nil {
		http.Error(w, "Failed to generate presenter URL", http.StatusInternalServerError)
		return
	}

	html, err := GenerateQRCodeHTML(audienceURL, presenterURL)
	if err != nil {
		http.Error(w, "Failed to generate QR code page", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprint(w, html)
}
