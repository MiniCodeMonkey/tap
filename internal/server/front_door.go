package server

import (
	"errors"
	"net/http"
	"slices"
)

// AppSourcePath is the route tap dev --app registers for the app's unsaved
// buffer.
const AppSourcePath = "/api/app/source"

// Request body limits. The app's unsaved buffer can be a large deck. Every
// other body is a small JSON request.
const (
	AppSourceBodyLimit = 8 << 20
	RequestBodyLimit   = 64 << 10
)

// requestBodyLimit is the largest body a request to path may send.
func requestBodyLimit(path string) int64 {
	if path == AppSourcePath {
		return AppSourceBodyLimit
	}
	return RequestBodyLimit
}

// isMutatingMethod reports whether a request with method can change state
// on the server.
func isMutatingMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return false
	}
	return true
}

// IsBodyTooLarge reports whether err came from reading a request body past
// its limit.
func IsBodyTooLarge(err error) bool {
	var tooLarge *http.MaxBytesError
	return errors.As(err, &tooLarge)
}

// WriteBodyTooLarge answers a request whose body is over its limit.
func WriteBodyTooLarge(w http.ResponseWriter) {
	http.Error(w, "Request Entity Too Large: the body is over this route's limit", http.StatusRequestEntityTooLarge)
}

// serveHTTP is the server's front door. Every request passes through it
// before the mux. With an app token set, it answers every request to a
// route outside audienceRoutes that lacks the token. It caps the body
// before any handler reads it, and it applies the same-origin JSON guard
// to every method that can change state, so no mutating route can be
// registered without the guard.
func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	auth := s.appAuth
	s.mu.RUnlock()
	if auth != nil && !auth.authorize(w, r, s) {
		return
	}

	limit := requestBodyLimit(r.URL.Path)
	if r.ContentLength > limit {
		WriteBodyTooLarge(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, limit)

	if isMutatingMethod(r.Method) {
		s.requireAllowedHost(s.requireSameOriginJSON(s.mux.ServeHTTP))(w, r)
		return
	}
	s.mux.ServeHTTP(w, r)
}

// handleRoute registers handler for pattern on the mux, and remembers the
// pattern for Routes.
func (s *Server) handleRoute(pattern string, handler http.HandlerFunc) {
	s.mu.Lock()
	s.routes = append(s.routes, pattern)
	s.mu.Unlock()
	s.mux.HandleFunc(pattern, handler)
}

// Routes lists every pattern registered on the server, sorted.
func (s *Server) Routes() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	routes := slices.Clone(s.routes)
	slices.Sort(routes)
	return routes
}
