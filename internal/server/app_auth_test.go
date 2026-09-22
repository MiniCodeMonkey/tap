package server

import (
	"context"
	"fmt"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// startAppServer starts a loopback server with the page routes, the
// WebSocket hub, a synthetic app control route standing in for a future
// route under AppSourcePath's /api/app/ prefix, and an app token. It
// returns the server, the auth and the base URL.
func startAppServer(t *testing.T) (*Server, *AppAuth, string) {
	t.Helper()
	s := NewWithHost(0, "127.0.0.1")
	s.SetPresentation(&transformer.TransformedPresentation{})
	s.SetupRoutes()
	hub := NewWebSocketHub()
	go hub.Run()
	t.Cleanup(hub.Stop)
	s.RegisterHandlerFunc("GET /ws", hub.HandleConnection)
	// A stand-in for a real /api/app/ route (the real PUT AppSourcePath
	// route is registered by a later task). It must behave like every
	// other route under that prefix: always needs the token.
	s.RegisterHandlerFunc("GET "+AppSourcePath, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	auth, err := NewAppAuth()
	if err != nil {
		t.Fatal(err)
	}
	s.SetAppAuth(auth)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	return s, auth, fmt.Sprintf("http://127.0.0.1:%d", s.Port())
}

// getStatus sends a GET with one optional header and returns the response,
// with its body closed.
func getStatus(t *testing.T, client *http.Client, url, headerName, headerValue string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if headerName != "" {
		request.Header.Set(headerName, headerValue)
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	return response
}

// getStatusWithHost is getStatus, but sets the request's Host header
// instead of a request header, to test that Host cannot decide an
// authorization question.
func getStatusWithHost(t *testing.T, client *http.Client, url, host string) *http.Response {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	if host != "" {
		request.Host = host
	}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	return response
}

func TestNewAppAuthMakesFreshSecrets(t *testing.T) {
	first, err := NewAppAuth()
	if err != nil {
		t.Fatal(err)
	}
	second, err := NewAppAuth()
	if err != nil {
		t.Fatal(err)
	}
	for _, secret := range []string{first.Token(), first.LaunchCode()} {
		if len(secret) != 64 {
			t.Errorf("secret %q has %d characters, want 64 (32 bytes, hex)", secret, len(secret))
		}
	}
	if first.Token() == first.LaunchCode() || first.Token() == second.Token() {
		t.Error("two secrets are equal")
	}
}

// TestAppAuthControlRoutesNeedTheToken checks the app's control surface: a
// route that is not one of the audience's, such as AppSourcePath. It must
// refuse a request without the token and accept one with it.
func TestAppAuthControlRoutesNeedTheToken(t *testing.T) {
	_, auth, base := startAppServer(t)
	for _, path := range []string{AppSourcePath, "/qr"} {
		response := getStatus(t, http.DefaultClient, base+path, "", "")
		if response.StatusCode != http.StatusUnauthorized {
			t.Errorf("GET %s without the token: status %d, want 401", path, response.StatusCode)
		}
		if !strings.HasPrefix(response.Header.Get("WWW-Authenticate"), "Bearer") {
			t.Errorf("GET %s: WWW-Authenticate = %q, want Bearer", path, response.Header.Get("WWW-Authenticate"))
		}
	}
	wrong := getStatus(t, http.DefaultClient, base+AppSourcePath, "Authorization", "Bearer "+strings.Repeat("0", 64))
	if wrong.StatusCode != http.StatusUnauthorized {
		t.Errorf("a wrong token on a control route: status %d, want 401", wrong.StatusCode)
	}
	right := getStatus(t, http.DefaultClient, base+AppSourcePath, "Authorization", "Bearer "+auth.Token())
	if right.StatusCode != http.StatusOK {
		t.Errorf("the right token on a control route: status %d, want 200", right.StatusCode)
	}
}

// TestAppAuthAudienceRoutesNeedNoToken checks that what an audience member
// or a phone remote needs - the deck page, its assets, the presentation
// data and the presenter view - works with no token at all, on the
// ordinary loopback address, because these routes carry nothing an app
// token protects.
func TestAppAuthAudienceRoutesNeedNoToken(t *testing.T) {
	_, _, base := startAppServer(t)
	for _, path := range []string{"/", "/api/presentation", "/presenter", "/assets/index.js"} {
		response := getStatus(t, http.DefaultClient, base+path, "", "")
		if response.StatusCode == http.StatusUnauthorized {
			t.Errorf("GET %s without the token: status %d, want anything but 401 (an audience route)", path, response.StatusCode)
		}
	}
}

// TestAppAuthForgedHostDoesNotBypassTheToken is the regression test for the
// auth bypass: a request cannot buy its way onto a control route by
// setting a Host header, forged or otherwise. Host is a value the client
// sends, so it can never be the thing an authorization decision turns on.
func TestAppAuthForgedHostDoesNotBypassTheToken(t *testing.T) {
	_, _, base := startAppServer(t)
	for _, host := range []string{"quiet-river.trycloudflare.com", "127.0.0.1", ""} {
		response := getStatusWithHost(t, http.DefaultClient, base+AppSourcePath, host)
		if response.StatusCode != http.StatusUnauthorized {
			t.Errorf("GET %s with Host %q and no token: status %d, want 401", AppSourcePath, host, response.StatusCode)
		}
	}
}

// TestAppAuthAudienceRoutesIgnoreHost checks the other side of the same
// fix: an audience route needs no token whatever the Host header claims,
// because route identity, not Host, is what decides. SetAllowedOrigins
// stands in for a real tunnel starting, so this isolates the app-token
// question from the unrelated DNS-rebinding host allow-list that
// requireAllowedHost enforces on some of these routes independently.
func TestAppAuthAudienceRoutesIgnoreHost(t *testing.T) {
	s, _, base := startAppServer(t)
	s.SetAllowedOrigins([]string{"quiet-river.trycloudflare.com"})
	response := getStatusWithHost(t, http.DefaultClient, base+"/api/presentation", "quiet-river.trycloudflare.com")
	if response.StatusCode != http.StatusOK {
		t.Errorf("GET /api/presentation with a forged Host: status %d, want 200", response.StatusCode)
	}
}

func TestAppAuthTradesTheLaunchCodeForACookieOnce(t *testing.T) {
	s, auth, base := startAppServer(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	page := &http.Client{Jar: jar}

	// Spend the launch code against a control route, so the cookie it
	// sets is the thing actually proven, not an accident of an audience
	// route that would have answered 200 regardless.
	response := getStatus(t, page, base+AppSourcePath+"?launch="+auth.LaunchCode(), "", "")
	if response.StatusCode != http.StatusOK {
		t.Fatalf("the launch URL ended with status %d, want 200", response.StatusCode)
	}
	if response.Request.URL.RawQuery != "" {
		t.Errorf("landed on %s, want the launch code gone from the URL", response.Request.URL)
	}
	var session *http.Cookie
	for _, cookie := range jar.Cookies(response.Request.URL) {
		if cookie.Name == AppSessionCookieName(s.Port()) {
			session = cookie
		}
	}
	if session == nil || session.Value != auth.Token() {
		t.Fatalf("cookies = %v, want %s set to the token", jar.Cookies(response.Request.URL), AppSessionCookieName(s.Port()))
	}
	if again := getStatus(t, page, base+AppSourcePath, "", ""); again.StatusCode != http.StatusOK {
		t.Errorf("a control route with the cookie: status %d, want 200", again.StatusCode)
	}

	reused := getStatus(t, &http.Client{}, base+AppSourcePath+"?launch="+auth.LaunchCode(), "", "")
	if reused.StatusCode != http.StatusForbidden {
		t.Errorf("the launch code a second time: status %d, want 403", reused.StatusCode)
	}
}

// TestAppAuthTheWebSocketNeedsNoToken checks that the WebSocket upgrade,
// one of the audience's routes, works with no token, the same as the deck
// page it serves.
func TestAppAuthTheWebSocketNeedsNoToken(t *testing.T) {
	_, _, base := startAppServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(base, "http") + "/ws"

	conn, _, err := websocket.Dial(ctx, url, nil)
	if err != nil {
		t.Fatalf("the WebSocket did not open with no token: %v", err)
	}
	_ = conn.Close(websocket.StatusNormalClosure, "")
}

func TestServerWithoutAppAuthNeedsNoToken(t *testing.T) {
	s := NewWithHost(0, "127.0.0.1")
	s.SetPresentation(&transformer.TransformedPresentation{})
	s.SetupRoutes()
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Shutdown(context.Background()) })
	response := getStatus(t, http.DefaultClient, fmt.Sprintf("http://127.0.0.1:%d/api/presentation", s.Port()), "", "")
	if response.StatusCode != http.StatusOK {
		t.Errorf("plain tap dev: status %d, want 200", response.StatusCode)
	}
}
