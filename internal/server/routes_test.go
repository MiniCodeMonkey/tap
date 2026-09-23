package server

import (
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/driver"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
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

	// Check for expected content (Vite outputs lowercase doctype)
	bodyLower := strings.ToLower(bodyStr)
	if !strings.Contains(bodyLower, "<!doctype html>") {
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

func TestHandlePresenter_PasswordProtection_NoPassword(t *testing.T) {
	s := New(0)
	s.SetPresenterPassword("mysecret")

	// Request without password
	req := httptest.NewRequest(http.MethodGet, "/presenter", nil)
	w := httptest.NewRecorder()

	s.handlePresenter(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "presenter password required") {
		t.Error("expected error message about password required")
	}
}

func TestHandlePresenter_PasswordProtection_WrongPassword(t *testing.T) {
	s := New(0)
	s.SetPresenterPassword("mysecret")

	// Request with wrong password
	req := httptest.NewRequest(http.MethodGet, "/presenter?key=wrongpassword", nil)
	w := httptest.NewRecorder()

	s.handlePresenter(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "incorrect presenter password") {
		t.Error("expected error message about incorrect password")
	}
}

// TestHandlePresenter_PasswordProtection_CorrectPassword verifies that a
// correct ?key= redirects to the presenter page without the key parameter,
// so the password does not stay in the address bar or browser history; the
// redirect target is what actually serves the presenter page once the
// browser follows it with the cookie this response just set.
func TestHandlePresenter_PasswordProtection_CorrectPassword(t *testing.T) {
	s := New(0)
	s.SetPresenterPassword("mysecret")
	s.SetPresenterSessionToken("session-token")

	req := httptest.NewRequest(http.MethodGet, "/presenter?key=mysecret", nil)
	w := httptest.NewRecorder()

	s.handlePresenter(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusFound {
		t.Errorf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}
	if location := resp.Header.Get("Location"); location != "/presenter" {
		t.Errorf("Location = %q, want %q (key parameter stripped)", location, "/presenter")
	}

	// Following the redirect, with the cookie the first response set,
	// serves the presenter page.
	followReq := httptest.NewRequest(http.MethodGet, "/presenter", nil)
	for _, c := range resp.Cookies() {
		followReq.AddCookie(c)
	}
	followW := httptest.NewRecorder()
	s.handlePresenter(followW, followReq)
	followResp := followW.Result()
	defer followResp.Body.Close()

	if followResp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d after following the redirect, got %d", http.StatusOK, followResp.StatusCode)
	}
	body, _ := io.ReadAll(followResp.Body)
	if !strings.Contains(string(body), "Presenter View") {
		t.Error("expected 'Presenter View' in body")
	}
}

// TestHandlePresenter_SetsAuthCookieOnCorrectPassword verifies that a
// correct ?key= sets PresenterAuthCookieName to the server's session
// token, never the raw password, Path=/, so the same browser's later
// WebSocket upgrade to /ws can prove it too (see
// WebSocketHub.checkPresenterAuth).
func TestHandlePresenter_SetsAuthCookieOnCorrectPassword(t *testing.T) {
	s := New(0)
	s.SetPresenterPassword("mysecret")
	s.SetPresenterSessionToken("session-token")

	req := httptest.NewRequest(http.MethodGet, "/presenter?key=mysecret", nil)
	w := httptest.NewRecorder()

	s.handlePresenter(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == PresenterAuthCookieName {
			cookie = c
			break
		}
	}
	if cookie == nil {
		t.Fatal("no auth cookie set on a correct password")
	}
	if cookie.Value != "session-token" {
		t.Errorf("cookie value = %q, want the session token, not the raw password", cookie.Value)
	}
	if cookie.Path != "/" {
		t.Errorf("cookie path = %q, want \"/\" so it also rides along on /ws", cookie.Path)
	}
}

// TestHandlePresenter_NoAuthCookieOnWrongPassword verifies a wrong ?key=
// sets no auth cookie.
func TestHandlePresenter_NoAuthCookieOnWrongPassword(t *testing.T) {
	s := New(0)
	s.SetPresenterPassword("mysecret")
	s.SetPresenterSessionToken("session-token")

	req := httptest.NewRequest(http.MethodGet, "/presenter?key=wrong", nil)
	w := httptest.NewRecorder()

	s.handlePresenter(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	for _, c := range resp.Cookies() {
		if c.Name == PresenterAuthCookieName {
			t.Fatal("auth cookie set on a wrong password")
		}
	}
}

// TestHandlePresenter_PasswordWithSpecialCharactersAuthenticates checks a
// password containing a semicolon and a space - characters Go's cookie jar
// sanitizes out of a raw cookie value - authenticates correctly, since the
// cookie carries the random session token rather than the password.
func TestHandlePresenter_PasswordWithSpecialCharactersAuthenticates(t *testing.T) {
	s := New(0)
	password := `weird; pass"word`
	s.SetPresenterPassword(password)
	s.SetPresenterSessionToken("session-token")

	req := httptest.NewRequest(http.MethodGet, "/presenter?key="+url.QueryEscape(password), nil)
	w := httptest.NewRecorder()
	s.handlePresenter(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Fatalf("expected status %d, got %d", http.StatusFound, resp.StatusCode)
	}

	var cookie *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == PresenterAuthCookieName {
			cookie = c
		}
	}
	if cookie == nil {
		t.Fatal("no auth cookie set on a correct password")
	}

	followReq := httptest.NewRequest(http.MethodGet, "/presenter", nil)
	followReq.AddCookie(cookie)
	followW := httptest.NewRecorder()
	s.handlePresenter(followW, followReq)
	if followW.Result().StatusCode != http.StatusOK {
		t.Errorf("expected status %d for cookie-only access, got %d", http.StatusOK, followW.Result().StatusCode)
	}
}

// TestHandlePresenter_CookieOnlyAccessWorks checks that a request carrying
// only the presenter auth cookie (no ?key=) is authorized, so the
// presenter page keeps working after the first authentication without the
// password reappearing in the URL.
func TestHandlePresenter_CookieOnlyAccessWorks(t *testing.T) {
	s := New(0)
	s.SetPresenterPassword("mysecret")
	s.SetPresenterSessionToken("session-token")

	req := httptest.NewRequest(http.MethodGet, "/presenter", nil)
	req.AddCookie(&http.Cookie{Name: PresenterAuthCookieName, Value: "session-token"})
	w := httptest.NewRecorder()
	s.handlePresenter(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d for cookie-only access, got %d", http.StatusOK, resp.StatusCode)
	}
}

// TestHandlePresenter_WrongCookieMatchesNoKeyResponse checks that a wrong
// cookie value, with no ?key=, gets the exact same response as no
// credentials at all, rather than leaking that a cookie was present but
// invalid.
func TestHandlePresenter_WrongCookieMatchesNoKeyResponse(t *testing.T) {
	s := New(0)
	s.SetPresenterPassword("mysecret")
	s.SetPresenterSessionToken("session-token")

	noCredsReq := httptest.NewRequest(http.MethodGet, "/presenter", nil)
	noCredsW := httptest.NewRecorder()
	s.handlePresenter(noCredsW, noCredsReq)
	noCredsResp := noCredsW.Result()
	defer noCredsResp.Body.Close()
	noCredsBody, _ := io.ReadAll(noCredsResp.Body)

	wrongCookieReq := httptest.NewRequest(http.MethodGet, "/presenter", nil)
	wrongCookieReq.AddCookie(&http.Cookie{Name: PresenterAuthCookieName, Value: "not-the-token"})
	wrongCookieW := httptest.NewRecorder()
	s.handlePresenter(wrongCookieW, wrongCookieReq)
	wrongCookieResp := wrongCookieW.Result()
	defer wrongCookieResp.Body.Close()
	wrongCookieBody, _ := io.ReadAll(wrongCookieResp.Body)

	if wrongCookieResp.StatusCode != noCredsResp.StatusCode {
		t.Errorf("status with wrong cookie = %d, want %d (same as no key)", wrongCookieResp.StatusCode, noCredsResp.StatusCode)
	}
	if string(wrongCookieBody) != string(noCredsBody) {
		t.Errorf("body with wrong cookie = %q, want %q (same as no key)", wrongCookieBody, noCredsBody)
	}
}

func TestHandlePresenter_PasswordProtection_EmptyKey(t *testing.T) {
	s := New(0)
	s.SetPresenterPassword("mysecret")

	// Request with empty key parameter
	req := httptest.NewRequest(http.MethodGet, "/presenter?key=", nil)
	w := httptest.NewRecorder()

	s.handlePresenter(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d, got %d", http.StatusForbidden, resp.StatusCode)
	}
}

func TestHandlePresenter_NoPasswordConfigured(t *testing.T) {
	s := New(0)
	// No password set - should allow access without key

	req := httptest.NewRequest(http.MethodGet, "/presenter", nil)
	w := httptest.NewRecorder()

	s.handlePresenter(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
}

func TestHandleAPIPresentationCarriesTheRevision(t *testing.T) {
	s := New(0)
	s.SetPresentation(&transformer.TransformedPresentation{
		Config: config.Config{Title: "Deck"},
		Slides: []transformer.TransformedSlide{{Index: 0, Layout: "default", HTML: "<h1>One</h1>", Hash: "abc"}},
	})
	s.SetRevision("r1")

	request := httptest.NewRequest(http.MethodGet, "/api/presentation", nil)
	recorder := httptest.NewRecorder()
	s.handleAPIPresentation(recorder, request)

	var body struct {
		Revision string `json:"revision"`
		Config   struct {
			Title string `json:"title"`
		} `json:"config"`
		Slides []struct {
			Hash string `json:"hash"`
		} `json:"slides"`
	}
	if err := json.NewDecoder(recorder.Body).Decode(&body); err != nil {
		t.Fatalf("decoding the body: %v", err)
	}
	if body.Revision != "r1" {
		t.Errorf("revision = %q, want %q", body.Revision, "r1")
	}
	if body.Config.Title != "Deck" || len(body.Slides) != 1 || body.Slides[0].Hash != "abc" {
		t.Errorf("body = %+v, want the deck's config and slides next to the revision", body)
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
	s := New(3000)

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
	requiredContent := []string{
		"Audience View",
		"Presenter View",
		"data:image/png;base64,", // QR code images
		":3000",                  // Port in URLs
		"/presenter",             // Presenter path in URL
	}

	for _, required := range requiredContent {
		if !strings.Contains(bodyStr, required) {
			t.Errorf("expected body to contain '%s'", required)
		}
	}
}

// TestHandleQR_WithPasswordRequiresAuth checks that /qr - which prints the
// presenter password straight into the presenter URL and QR code - refuses
// an unauthenticated request once a presenter password is configured,
// exactly like /presenter itself. Without this, anyone on the network
// could read the password off this page without ever passing the
// presenter gate.
func TestHandleQR_WithPasswordRequiresAuth(t *testing.T) {
	s := New(3000)
	s.SetPresenterPassword("secretpass")

	req := httptest.NewRequest(http.MethodGet, "/qr", nil)
	w := httptest.NewRecorder()

	s.handleQR(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected status %d for an unauthenticated request, got %d", http.StatusForbidden, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if strings.Contains(string(body), "secretpass") {
		t.Error("response must not leak the presenter password")
	}
}

// TestHandleQR_WithCorrectKeyServesThePage checks that /qr serves the page,
// with the password embedded in the presenter URL, once the request
// carries the correct ?key=.
func TestHandleQR_WithCorrectKeyServesThePage(t *testing.T) {
	s := New(3000)
	s.SetPresenterPassword("secretpass")

	req := httptest.NewRequest(http.MethodGet, "/qr?key=secretpass", nil)
	w := httptest.NewRecorder()

	s.handleQR(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	bodyStr := string(body)

	if !strings.Contains(bodyStr, "?key=secretpass") {
		t.Error("expected presenter URL to contain password query param")
	}
}

// TestHandleQR_WithPresenterCookieServesThePage checks that /qr also
// accepts the presenter auth cookie in place of ?key=, matching
// handlePresenter.
func TestHandleQR_WithPresenterCookieServesThePage(t *testing.T) {
	s := New(3000)
	s.SetPresenterPassword("secretpass")
	s.SetPresenterSessionToken("session-token")

	req := httptest.NewRequest(http.MethodGet, "/qr", nil)
	req.AddCookie(&http.Cookie{Name: PresenterAuthCookieName, Value: "session-token"})
	w := httptest.NewRecorder()

	s.handleQR(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
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

	// 127.0.0.1, not s.Addr()'s bound "0.0.0.0", which is the unspecified
	// address, not itself on the DNS-rebinding host allow-list (see
	// isAllowedHost) - a real client always dials a concrete address.
	_, addrPort, err := net.SplitHostPort(s.Addr())
	if err != nil {
		t.Fatalf("failed to parse server address: %v", err)
	}
	baseURL := "http://127.0.0.1:" + addrPort

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

// TestSetupRoutes_ExactPathsOnly reproduces the nested-path bug the
// frontend's relative base ("base: './'") introduces: index.html (or
// presenter.html) for an unmatched nested path leaves the page's relative
// asset URLs resolving against the wrong directory, and the page comes up
// blank. The app must be served only at its exact paths; a trailing-slash
// variant of /presenter redirects to the canonical path, and any other
// unmatched path gets a real 404.
func TestSetupRoutes_ExactPathsOnly(t *testing.T) {
	s := New(0)
	s.SetupRoutes()

	cfg := config.DefaultConfig()
	pres := &transformer.TransformedPresentation{
		Config: *cfg,
		Slides: []transformer.TransformedSlide{
			{Index: 0, Layout: "title", HTML: "<h1>Test</h1>"},
		},
	}
	s.SetPresentation(pres)

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Shutdown(context.Background())

	_, addrPort, err := net.SplitHostPort(s.Addr())
	if err != nil {
		t.Fatalf("failed to parse server address: %v", err)
	}
	baseURL := "http://127.0.0.1:" + addrPort

	get := func(t *testing.T, path string) *http.Response {
		t.Helper()
		client := &http.Client{
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return http.ErrUseLastResponse
			},
		}
		resp, err := client.Get(baseURL + path)
		if err != nil {
			t.Fatalf("GET %s failed: %v", path, err)
		}
		return resp
	}

	t.Run("index.html serves the index page", func(t *testing.T) {
		resp := get(t, "/index.html")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
	})

	t.Run("presenter.html serves the presenter page", func(t *testing.T) {
		resp := get(t, "/presenter.html")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusOK)
		}
		body, _ := io.ReadAll(resp.Body)
		if !strings.Contains(string(body), "Presenter View") {
			t.Error("expected 'Presenter View' in body")
		}
	})

	t.Run("a trailing slash on /presenter redirects to the canonical path", func(t *testing.T) {
		resp := get(t, "/presenter/")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusMovedPermanently {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusMovedPermanently)
		}
		if location := resp.Header.Get("Location"); location != "/presenter" {
			t.Errorf("Location = %q, want %q", location, "/presenter")
		}
	})

	t.Run("an unknown nested path gets a real 404, not index.html", func(t *testing.T) {
		resp := get(t, "/foo/bar")
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusNotFound {
			t.Errorf("status = %d, want %d", resp.StatusCode, http.StatusNotFound)
		}
		body, _ := io.ReadAll(resp.Body)
		if strings.Contains(string(body), "Tap") {
			t.Errorf("expected a plain 404, not the index page: %q", body)
		}
	})
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
				Index:         2,
				Layout:        "default",
				HTML:          "<p>First</p><p>Second</p>",
				FragmentCount: 2,
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

	// Check third slide has the fragment count
	if result.Slides[2].FragmentCount != 2 {
		t.Errorf("expected fragmentCount 2, got %d", result.Slides[2].FragmentCount)
	}
}

func TestHandleComponentBundle_ServesKnownFile(t *testing.T) {
	s := New(0)
	s.SetComponentBundles(map[string]ComponentBundleFile{
		"RollingDeploy-abc123.js": {ContentType: "application/javascript; charset=utf-8", Content: []byte("export default 1;")},
	})

	req := httptest.NewRequest(http.MethodGet, "/components/RollingDeploy-abc123.js", nil)
	w := httptest.NewRecorder()
	s.handleComponentBundle(w, req)

	resp := w.Result()
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "application/javascript; charset=utf-8" {
		t.Errorf("expected Content-Type %q, got %q", "application/javascript; charset=utf-8", ct)
	}
	if cc := resp.Header.Get("Cache-Control"); cc != "no-store" {
		t.Errorf("expected Cache-Control %q, got %q", "no-store", cc)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != "export default 1;" {
		t.Errorf("expected body %q, got %q", "export default 1;", string(body))
	}
}

func TestHandleComponentBundle_UnknownNameGives404(t *testing.T) {
	s := New(0)

	req := httptest.NewRequest(http.MethodGet, "/components/Nope-000000.js", nil)
	w := httptest.NewRecorder()
	s.handleComponentBundle(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestHandleComponentBundle_RejectsPathTraversal(t *testing.T) {
	s := New(0)

	req := httptest.NewRequest(http.MethodGet, "/components/../../etc/passwd", nil)
	req.URL.Path = "/components/../../etc/passwd"
	w := httptest.NewRecorder()
	s.handleComponentBundle(w, req)

	resp := w.Result()
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected status %d, got %d", http.StatusNotFound, resp.StatusCode)
	}
}

func TestQRPageExplainsLANOnALoopbackServer(t *testing.T) {
	s := NewWithHost(0, "127.0.0.1")
	s.SetupRoutes()
	request := httptest.NewRequest(http.MethodGet, "/qr", nil)
	request.Host = "localhost"
	recorder := httptest.NewRecorder()
	s.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusNotFound {
		t.Errorf("status = %d, want %d", recorder.Code, http.StatusNotFound)
	}
	if !strings.Contains(recorder.Body.String(), "--lan") {
		t.Errorf("body = %q, want it to name --lan", recorder.Body.String())
	}
}

func TestListensOnLoopbackOnly(t *testing.T) {
	if !NewWithHost(0, "127.0.0.1").ListensOnLoopbackOnly() {
		t.Error("127.0.0.1 should be loopback only")
	}
	if NewWithHost(0, "0.0.0.0").ListensOnLoopbackOnly() {
		t.Error("0.0.0.0 is not loopback only")
	}
}

func getPresentationJSON(t *testing.T, s *Server) map[string]json.RawMessage {
	t.Helper()
	s.SetupRoutes()
	request := httptest.NewRequest(http.MethodGet, "/api/presentation", nil)
	request.Host = "localhost"
	recorder := httptest.NewRecorder()
	s.mux.ServeHTTP(recorder, request)
	if recorder.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", recorder.Code, recorder.Body.String())
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(recorder.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func TestPresentationListsTheDriversThisRunAllows(t *testing.T) {
	s := NewWithHost(0, "127.0.0.1")
	registry := driver.NewRegistry()
	registry.Register(&mockDriver{name: "sqlite"})
	s.SetRegistry(registry)
	s.SetPresentation(&transformer.TransformedPresentation{
		Config: config.Config{Drivers: map[string]config.DriverConfig{"shell": {}, "sqlite": {}}},
		Slides: []transformer.TransformedSlide{{Index: 0}},
	})
	s.SetLiveCodePolicy(LiveCodePolicy{Drivers: []string{"sqlite", "mysql"}})

	body := getPresentationJSON(t, s)
	if string(body["liveCode"]) != `{"drivers":["sqlite"]}` {
		t.Errorf("liveCode = %s, want only the declared, approved sqlite", body["liveCode"])
	}
	if _, found := body["slides"]; !found {
		t.Error("the slides are missing")
	}
}

// TestPresentationOmitsADeclaredDriverTheRegistryNeverBuilt covers a custom
// driver declared with no command: buildDriverRegistry skips it, so it is
// declared and can be approved, but never actually runs. The advisory list
// must not promise it, or the page shows a Run button that /api/execute
// then refuses with "driver not found".
func TestPresentationOmitsADeclaredDriverTheRegistryNeverBuilt(t *testing.T) {
	s := NewWithHost(0, "127.0.0.1")
	registry := driver.NewRegistry()
	registry.Register(&mockDriver{name: "sqlite"})
	s.SetRegistry(registry)
	s.SetPresentation(&transformer.TransformedPresentation{
		Config: config.Config{Drivers: map[string]config.DriverConfig{"sqlite": {}, "python": {}}},
		Slides: []transformer.TransformedSlide{{Index: 0}},
	})
	s.SetLiveCodePolicy(LiveCodePolicy{Drivers: []string{"sqlite", "python"}})

	body := getPresentationJSON(t, s)
	if string(body["liveCode"]) != `{"drivers":["sqlite"]}` {
		t.Errorf("liveCode = %s, want python left out: it is declared and approved but the registry never built it", body["liveCode"])
	}
}

func TestPresentationListsNoDriverForAnUnapprovedDeck(t *testing.T) {
	s := NewWithHost(0, "127.0.0.1")
	s.SetRegistry(driver.NewRegistry())
	s.SetPresentation(&transformer.TransformedPresentation{
		Config: config.Config{Drivers: map[string]config.DriverConfig{"shell": {}}},
	})
	body := getPresentationJSON(t, s)
	if string(body["liveCode"]) != `{"drivers":[]}` {
		t.Errorf("liveCode = %s, want an empty list", body["liveCode"])
	}
}

func TestPresentationHasNoLiveCodeWithoutARegistry(t *testing.T) {
	s := NewWithHost(0, "127.0.0.1")
	s.SetPresentation(&transformer.TransformedPresentation{})
	body := getPresentationJSON(t, s)
	if _, found := body["liveCode"]; found {
		t.Errorf("liveCode = %s, want no key on a server that cannot run code", body["liveCode"])
	}
}

// deckWithConnectionSetting builds a deck config whose one driver has one
// connection, so a test can compare a literal secret against a
// placeholder one under an otherwise identical response.
func deckWithConnectionSetting(password string) *transformer.TransformedPresentation {
	return &transformer.TransformedPresentation{
		Config: config.Config{
			// Deliberately contains "port" and "user" as ordinary English
			// inside other words, so a leak check that sweeps the body for
			// those substrings would fail on the title alone.
			Title: "Import and Export",
			Drivers: map[string]config.DriverConfig{
				"postgres": {
					Command: "psql",
					Args:    []string{"--quiet"},
					Timeout: 5,
					Connections: map[string]config.ConnectionConfig{
						"prod": {
							Host:     "db.internal.example.com",
							User:     "admin",
							Password: password,
							Database: "billing",
							Port:     5432,
						},
					},
				},
			},
		},
		Slides: []transformer.TransformedSlide{
			{
				Index:  0,
				Layout: "default",
				CodeBlocks: []transformer.TransformedCodeBlock{
					{Language: "sql", Code: "select 1", Driver: "postgres", Connection: "prod", Block: 1},
				},
			},
		},
	}
}

// TestPresentationNeverServesALiteralPassword covers the measured leak: a
// password typed directly into a deck's frontmatter must never come back
// in /api/presentation's body, and neither must any other connection or
// driver setting, only the driver and connection names the page already
// carries per code block.
func TestPresentationNeverServesALiteralPassword(t *testing.T) {
	s := NewWithHost(0, "127.0.0.1")
	s.SetPresentation(deckWithConnectionSetting("hunter2literal"))

	rawBody := func() string {
		s.SetupRoutes()
		request := httptest.NewRequest(http.MethodGet, "/api/presentation", nil)
		request.Host = "localhost"
		recorder := httptest.NewRecorder()
		s.mux.ServeHTTP(recorder, request)
		if recorder.Code != http.StatusOK {
			t.Fatalf("status = %d: %s", recorder.Code, recorder.Body.String())
		}
		return recorder.Body.String()
	}
	body := rawBody()

	// Field names (drivers/connections/command/args/timeout/host/user/
	// password/database/path/port) are proven absent structurally by
	// TestPresentationConfigNeverEmbedsTheWholeConfigStruct, which decodes
	// the config object's keys; a substring sweep for those words here
	// would risk failing on a deck title that happens to contain one of
	// them (see the Title comment on deckWithConnectionSetting). This
	// checks only the specific secret values, which no real deck's own
	// wording could produce.
	for _, secret := range []string{
		"hunter2literal", "db.internal.example.com", "billing", "5432", "psql", "--quiet",
	} {
		if strings.Contains(body, secret) {
			t.Errorf("/api/presentation body contains %q, want it absent entirely: %s", secret, body)
		}
	}

	if !strings.Contains(body, `"title":"Import and Export"`) {
		t.Errorf("/api/presentation dropped a setting the page genuinely reads: %s", body)
	}
	if !strings.Contains(body, `"driver":"postgres"`) || !strings.Contains(body, `"connection":"prod"`) {
		t.Errorf("/api/presentation dropped the driver/connection names the Run button needs: %s", body)
	}
}

// TestPresentationNeverServesAPlaceholderConnectionValue covers the other
// half of the same leak: a deck that references a secret with ${NAME}
// rather than typing it in gets the identical treatment. The page never
// receives the connection at all, placeholder or not.
func TestPresentationNeverServesAPlaceholderConnectionValue(t *testing.T) {
	s := NewWithHost(0, "127.0.0.1")
	s.SetPresentation(deckWithConnectionSetting("${DB_PASSWORD}"))

	body := getPresentationJSON(t, s)
	slidesJSON := string(body["slides"])
	configJSON := string(body["config"])

	for _, secret := range []string{"${DB_PASSWORD}", "db.internal.example.com", "billing"} {
		if strings.Contains(slidesJSON, secret) || strings.Contains(configJSON, secret) {
			t.Errorf("/api/presentation body contains %q for a placeholder-valued connection, want it absent entirely: config=%s slides=%s", secret, configJSON, slidesJSON)
		}
	}

	var configFields map[string]json.RawMessage
	if err := json.Unmarshal(body["config"], &configFields); err != nil {
		t.Fatalf("decoding config: %v", err)
	}
	if _, found := configFields["connections"]; found {
		t.Errorf("config carries a connections key for a placeholder-valued connection: %s", configJSON)
	}
}

// TestPresentationConfigNeverEmbedsTheWholeConfigStruct guards the shape
// of the wire format itself: decoding the config object into a Go map must
// find only settings the page is deliberately given, never a driver's
// name as a top-level key. This is what would fail if handleAPIPresentation
// ever went back to encoding *transformer.TransformedPresentation (whose
// Config field is the full config.Config) instead of pres.Public().
func TestPresentationConfigNeverEmbedsTheWholeConfigStruct(t *testing.T) {
	s := NewWithHost(0, "127.0.0.1")
	s.SetPresentation(deckWithConnectionSetting("hunter2literal"))

	body := getPresentationJSON(t, s)
	var configFields map[string]json.RawMessage
	if err := json.Unmarshal(body["config"], &configFields); err != nil {
		t.Fatalf("decoding config: %v", err)
	}

	allowed := map[string]bool{
		"title": true, "theme": true, "customTheme": true, "aspectRatio": true,
		"transition": true, "themeColors": true, "slideNumbers": true, "presenterLayout": true,
	}
	for key := range configFields {
		if !allowed[key] {
			t.Errorf("config carries unexpected key %q; the config struct may have been embedded wholesale again: %v", key, configFields)
		}
	}
}
