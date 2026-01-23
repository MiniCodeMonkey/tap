// Package server provides HTTP route handlers for the tap dev server.
package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/fatih/color"
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
	// For now, return a placeholder HTML page
	// This will be replaced with embedded frontend assets in US-045
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Tap Presentation</title>
    <style>
        body { font-family: system-ui, sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; background: #1a1a1a; color: white; }
        .message { text-align: center; }
        h1 { font-size: 3rem; margin-bottom: 1rem; }
        p { color: #888; }
    </style>
</head>
<body>
    <div class="message">
        <h1>🎯 Tap</h1>
        <p>Presentation viewer will be loaded here</p>
        <p><a href="/api/presentation" style="color: #7C3AED;">View presentation data</a></p>
    </div>
</body>
</html>`)
}

// handlePresenter serves the presenter view.
func (s *Server) handlePresenter(w http.ResponseWriter, r *http.Request) {
	// For now, return a placeholder HTML page
	// Password protection will be added in US-035
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Tap Presenter View</title>
    <style>
        body { font-family: system-ui, sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; background: #0a0a0a; color: white; }
        .message { text-align: center; }
        h1 { font-size: 2rem; margin-bottom: 1rem; }
        p { color: #888; }
    </style>
</head>
<body>
    <div class="message">
        <h1>📋 Presenter View</h1>
        <p>Speaker notes and controls will appear here</p>
    </div>
</body>
</html>`)
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

// handleQR serves a page with a QR code for the presenter URL.
func (s *Server) handleQR(w http.ResponseWriter, r *http.Request) {
	// For now, return a placeholder HTML page
	// Actual QR code generation will be added in US-034
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(http.StatusOK)
	fmt.Fprintf(w, `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Tap QR Code</title>
    <style>
        body { font-family: system-ui, sans-serif; display: flex; justify-content: center; align-items: center; height: 100vh; margin: 0; background: white; color: #1a1a1a; }
        .message { text-align: center; }
        h1 { font-size: 2rem; margin-bottom: 1rem; }
        p { color: #666; }
        .qr-placeholder { width: 200px; height: 200px; border: 2px dashed #ccc; margin: 2rem auto; display: flex; justify-content: center; align-items: center; color: #999; }
    </style>
</head>
<body>
    <div class="message">
        <h1>📱 QR Code</h1>
        <div class="qr-placeholder">QR Code</div>
        <p>Scan to join the presentation</p>
    </div>
</body>
</html>`)
}
