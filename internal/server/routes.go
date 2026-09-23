// Package server provides HTTP route handlers for the tap dev server.
package server

import (
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"path"
	"strings"

	"github.com/MiniCodeMonkey/tap/embedded"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// SetupRoutes configures all HTTP routes on the server.
// This should be called before Start().
// Any routes registered with RegisterHandlerFunc before or after this call
// will also be included since we use the server's shared mux.
func (s *Server) SetupRoutes() {
	// Register all routes on the server's shared mux.
	// / and /assets/ are left off the host allow-list: they carry nothing
	// presenter-specific and nothing a DNS-rebinding attacker gains from
	// (see requireAllowedHost) - the audience view and its static assets
	// are meant to be reachable the same way the presentation itself is.
	// The app is only ever served at its exact paths, not the whole "/"
	// subtree Go's ServeMux would otherwise catch every unmatched request
	// with: the frontend now builds with a relative base ("base: './'"),
	// so its script and stylesheet URLs resolve relative to whatever path
	// served the page, and serving index.html for, say, /foo/bar would
	// point those URLs at /foo/assets/... - which returns this same HTML,
	// not a script - leaving the page blank. "/{$}" matches only the
	// literal root; "/index.html" and "/presenter.html" are the frontend's
	// own file names, aliased to the same handlers. A trailing-slash
	// variant of /presenter redirects to the canonical path; everything
	// else unmatched falls through to ServeMux's own 404.
	s.handleRoute("GET /{$}", s.handleIndex)
	s.handleRoute("GET /index.html", s.handleIndex)
	s.handleRoute("GET /presenter", s.requireAllowedHost(s.handlePresenter))
	s.handleRoute("GET /presenter.html", s.requireAllowedHost(s.handlePresenter))
	s.handleRoute("GET /presenter/", s.requireAllowedHost(redirectToCanonicalPath("/presenter")))
	s.handleRoute("GET /api/presentation", s.requireAllowedHost(s.handleAPIPresentation))
	s.handleRoute("GET /api/custom-theme.css", s.requireAllowedHost(s.handleCustomTheme))
	s.handleRoute("POST /api/execute", s.requireAllowedHost(s.requireSameOriginJSON(s.handleAPIExecute)))
	s.handleRoute("GET /qr", s.requireAllowedHost(s.handleQR))

	// Serve static assets (JS, CSS) from embedded dist/assets/
	s.handleRoute("GET /assets/", s.handleAssets)

	// Serve local files (images, etc.) from the presentation's base directory
	s.handleRoute("GET /local/", s.requireAllowedHost(s.handleLocalFiles))

	// Serve component bundles from the in-memory store the dev command
	// swaps atomically after each rebuild.
	s.handleRoute("GET /components/", s.requireAllowedHost(s.handleComponentBundle))

	// Note: We don't wrap with logging middleware here because the TUI
	// manages the terminal in alternate screen mode, and raw fmt.Printf
	// output would interfere with the display. HTTP activity is visible
	// through the TUI's connection status instead.
}

// redirectToCanonicalPath returns a handler that redirects a request for a
// trailing-slash variant of canonical (for example "/presenter/") to
// canonical itself, preserving the query string, so a link or a reverse
// proxy that appends a trailing slash still lands on the one path the
// frontend's relative asset URLs and any auth cookie are built for.
func redirectToCanonicalPath(canonical string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		target := canonical
		if r.URL.RawQuery != "" {
			target += "?" + r.URL.RawQuery
		}
		http.Redirect(w, r, target, http.StatusMovedPermanently)
	}
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
	// Disable caching in dev mode so browsers always fetch fresh assets
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// handlePresenter serves the presenter view.
// If a presenter password is configured, requires either a presenter auth
// cookie from an earlier visit or a ?key=<password> query parameter. A
// correct ?key= sets PresenterAuthCookieName to the server's random
// per-process session token, never the password itself (see
// GeneratePresenterSessionToken), so the same browser's WebSocket
// connection can prove it too (see WebSocketHub.checkPresenterAuth) -
// without it, a client that never saw this password-gated page could still
// open /ws directly and send navigation messages that drive every other
// client. After a correct ?key=, the request is redirected to the same
// path without the key query parameter, so the password does not stay in
// the address bar or browser history.
func (s *Server) handlePresenter(w http.ResponseWriter, r *http.Request) {
	password := s.GetPresenterPassword()
	if password != "" {
		sessionToken := s.GetPresenterSessionToken()
		authorized := s.presenterCookieAuthorized(r)

		if !authorized {
			key := r.URL.Query().Get("key")
			if key == "" {
				http.Error(w, "Forbidden: presenter password required. Use ?key=<password>", http.StatusForbidden)
				return
			}
			if subtle.ConstantTimeCompare([]byte(key), []byte(password)) != 1 {
				http.Error(w, "Forbidden: incorrect presenter password", http.StatusForbidden)
				return
			}

			http.SetCookie(w, &http.Cookie{
				Name:     PresenterAuthCookieName,
				Value:    sessionToken,
				Path:     "/",
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
			})

			redirectURL := *r.URL
			query := redirectURL.Query()
			query.Del("key")
			redirectURL.RawQuery = query.Encode()
			http.Redirect(w, r, redirectURL.RequestURI(), http.StatusFound)
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
	// Disable caching in dev mode so browsers always fetch fresh assets
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// presenterCookieAuthorized reports whether r carries a presenter auth
// cookie matching the server's current session token, compared in
// constant time. False whenever no session token has been issued yet (no
// one has ever passed ?key=).
func (s *Server) presenterCookieAuthorized(r *http.Request) bool {
	sessionToken := s.GetPresenterSessionToken()
	if sessionToken == "" {
		return false
	}
	cookie, err := r.Cookie(PresenterAuthCookieName)
	if err != nil {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(cookie.Value), []byte(sessionToken)) == 1
}

// presenterAuthorized reports whether r may see presenter-only information
// (the presenter page itself, and anything that reveals the presenter
// password, such as /qr): always true when no presenter password is
// configured, and otherwise true when r carries a valid presenter auth
// cookie or the correct ?key=<password>, compared in constant time.
func (s *Server) presenterAuthorized(r *http.Request) bool {
	password := s.GetPresenterPassword()
	if password == "" {
		return true
	}
	if s.presenterCookieAuthorized(r) {
		return true
	}
	key := r.URL.Query().Get("key")
	if key == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(key), []byte(password)) == 1
}

// presentationResponse is the GET /api/presentation body: the client-facing
// view of the deck (transformer.PublicPresentation, never the full
// TransformedPresentation with its driver settings), and which of its
// drivers this run lets run when the server can run code.
type presentationResponse struct {
	transformer.PublicPresentation
	LiveCode *liveCodeStatus `json:"liveCode,omitempty"`
	// Revision is the deck's revision, which the page reports in its
	// ready signal.
	Revision string `json:"revision"`
}

// liveCodeStatus tells the page which live code blocks can run. A block
// whose driver is not in Drivers shows "Not approved".
type liveCodeStatus struct {
	Drivers []string `json:"drivers"`
}

// liveCodeStatusFor returns the live code status for pres, or nil when the
// server has no driver registry and so runs no code at all. A driver is
// listed only when it is declared, approved, and actually registered: a
// custom driver declared with no command is skipped when the registry is
// built, and listing it anyway would show a Run button that /api/execute
// then refuses with "driver not found".
func (s *Server) liveCodeStatusFor(pres *transformer.TransformedPresentation) *liveCodeStatus {
	registry := s.GetRegistry()
	if registry == nil {
		return nil
	}
	policy := s.LiveCodePolicy()
	allowed := []string{}
	for _, name := range pres.Config.DeclaredDrivers() {
		if policy.Allows(name) && registry.Has(name) {
			allowed = append(allowed, name)
		}
	}
	return &liveCodeStatus{Drivers: allowed}
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
	// Disable caching so changes are always picked up
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	if err := json.NewEncoder(w).Encode(presentationResponse{
		PublicPresentation: pres.Public(),
		LiveCode:           s.liveCodeStatusFor(pres),
		Revision:           s.Revision(),
	}); err != nil {
		// If encoding fails, we've already started writing the response
		// so we can't change the status code. Just log internally.
		fmt.Printf("Error encoding presentation JSON: %v\n", err)
	}
}

// handleCustomTheme serves the custom CSS theme file if configured.
func (s *Server) handleCustomTheme(w http.ResponseWriter, r *http.Request) {
	themePath := s.GetCustomThemePath()
	if themePath == "" {
		http.NotFound(w, r)
		return
	}

	// Read the custom theme file
	content, err := os.ReadFile(themePath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to read custom theme file", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/css; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// handleQR serves a page with QR codes for the audience and presenter URLs.
// The presenter QR code and URL carry the presenter password, so this route
// requires the same proof handlePresenter does (a valid auth cookie or
// ?key=<password>) whenever a password is configured; otherwise an
// unauthenticated request on the network could read the password straight
// off this page without ever passing the presenter gate.
func (s *Server) handleQR(w http.ResponseWriter, r *http.Request) {
	// The page shows LAN addresses, which a server that listens on
	// loopback only does not answer.
	if s.ListensOnLoopbackOnly() {
		http.Error(w, "The QR page needs the server on the network: start tap dev with --lan", http.StatusNotFound)
		return
	}

	if !s.presenterAuthorized(r) {
		http.Error(w, "Forbidden: presenter password required. Use ?key=<password>", http.StatusForbidden)
		return
	}

	cfg := QRConfig{
		Port:              s.Port(),
		PresenterPassword: s.GetPresenterPassword(),
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

// handleAssets serves static assets (JS, CSS) from the embedded dist/assets/ directory.
func (s *Server) handleAssets(w http.ResponseWriter, r *http.Request) {
	// Get the embedded filesystem rooted at dist/
	distFS, err := embedded.DistFS()
	if err != nil {
		http.Error(w, "Failed to access embedded assets", http.StatusInternalServerError)
		return
	}

	// The request path is "/assets/filename", we need to serve "assets/filename" from distFS
	// Strip leading slash to get the relative path
	filePath := strings.TrimPrefix(r.URL.Path, "/")

	// Read the file from the embedded filesystem
	content, err := fs.ReadFile(distFS, filePath)
	if err != nil {
		http.NotFound(w, r)
		return
	}

	// Set content type based on file extension
	contentType := getContentType(filePath)
	w.Header().Set("Content-Type", contentType)

	// Disable caching in dev mode - users need to see changes immediately
	// For production exports, assets are served from static files with proper caching
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// handleLocalFiles serves local files (images, etc.) from the presentation's base directory.
func (s *Server) handleLocalFiles(w http.ResponseWriter, r *http.Request) {
	baseDir := s.GetBaseDir()
	if baseDir == "" {
		http.Error(w, "Base directory not configured", http.StatusInternalServerError)
		return
	}

	// Get the requested file path, stripping "/local/" prefix
	requestedPath := strings.TrimPrefix(r.URL.Path, "/local/")
	if requestedPath == "" {
		http.NotFound(w, r)
		return
	}

	// Security: prevent directory traversal
	if strings.Contains(requestedPath, "..") {
		http.Error(w, "Invalid path", http.StatusBadRequest)
		return
	}

	// Construct the full file path
	fullPath := path.Join(baseDir, requestedPath)

	// Read the file
	content, err := os.ReadFile(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			http.NotFound(w, r)
			return
		}
		http.Error(w, "Failed to read file", http.StatusInternalServerError)
		return
	}

	// Set content type based on extension
	contentType := getContentType(fullPath)
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(content)
}

// handleComponentBundle serves a component bundle file (JavaScript, CSS, or
// a dev source map) from the in-memory store. Unknown names give 404;
// responses always carry Cache-Control: no-store so a rebuilt bundle under
// the same name (a fixed file, minus its error) is never served stale.
func (s *Server) handleComponentBundle(w http.ResponseWriter, r *http.Request) {
	name := strings.TrimPrefix(r.URL.Path, "/components/")
	if name == "" || strings.Contains(name, "..") || strings.Contains(name, "/") {
		http.NotFound(w, r)
		return
	}

	file, ok := s.ComponentBundles().Get(name)
	if !ok {
		http.NotFound(w, r)
		return
	}

	w.Header().Set("Content-Type", file.ContentType)
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(file.Content)
}

// getContentType returns the appropriate Content-Type header for a file path.
func getContentType(filePath string) string {
	ext := strings.ToLower(path.Ext(filePath))
	switch ext {
	case ".js":
		return "application/javascript; charset=utf-8"
	case ".css":
		return "text/css; charset=utf-8"
	case ".html":
		return "text/html; charset=utf-8"
	case ".json":
		return "application/json; charset=utf-8"
	case ".svg":
		return "image/svg+xml"
	case ".png":
		return "image/png"
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".gif":
		return "image/gif"
	case ".woff":
		return "font/woff"
	case ".woff2":
		return "font/woff2"
	case ".ttf":
		return "font/ttf"
	case ".eot":
		return "application/vnd.ms-fontobject"
	case ".cast":
		return "application/json; charset=utf-8"
	default:
		return "application/octet-stream"
	}
}
