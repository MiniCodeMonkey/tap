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
// WebSocket hub and an app token, and returns it with its base URL.
func startAppServer(t *testing.T) (*Server, *AppAuth, string) {
	t.Helper()
	s := NewWithHost(0, "127.0.0.1")
	s.SetPresentation(&transformer.TransformedPresentation{})
	s.SetupRoutes()
	hub := NewWebSocketHub()
	go hub.Run()
	t.Cleanup(hub.Stop)
	s.RegisterHandlerFunc("GET /ws", hub.HandleConnection)
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

func TestAppAuthRejectsARequestWithoutTheToken(t *testing.T) {
	_, _, base := startAppServer(t)
	for _, path := range []string{"/", "/api/presentation", "/presenter", "/assets/index.js"} {
		response := getStatus(t, http.DefaultClient, base+path, "", "")
		if response.StatusCode != http.StatusUnauthorized {
			t.Errorf("GET %s without the token: status %d, want 401", path, response.StatusCode)
		}
		if !strings.HasPrefix(response.Header.Get("WWW-Authenticate"), "Bearer") {
			t.Errorf("GET %s: WWW-Authenticate = %q, want Bearer", path, response.Header.Get("WWW-Authenticate"))
		}
	}
	wrong := getStatus(t, http.DefaultClient, base+"/api/presentation", "Authorization", "Bearer "+strings.Repeat("0", 64))
	if wrong.StatusCode != http.StatusUnauthorized {
		t.Errorf("a wrong token: status %d, want 401", wrong.StatusCode)
	}
}

func TestAppAuthAcceptsTheBearerToken(t *testing.T) {
	_, auth, base := startAppServer(t)
	response := getStatus(t, http.DefaultClient, base+"/api/presentation", "Authorization", "Bearer "+auth.Token())
	if response.StatusCode != http.StatusOK {
		t.Errorf("status %d, want 200", response.StatusCode)
	}
}

func TestAppAuthTradesTheLaunchCodeForACookieOnce(t *testing.T) {
	s, auth, base := startAppServer(t)
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	page := &http.Client{Jar: jar}

	response := getStatus(t, page, base+"/api/presentation?launch="+auth.LaunchCode(), "", "")
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
	if again := getStatus(t, page, base+"/api/presentation", "", ""); again.StatusCode != http.StatusOK {
		t.Errorf("a request with the cookie: status %d, want 200", again.StatusCode)
	}

	reused := getStatus(t, &http.Client{}, base+"/api/presentation?launch="+auth.LaunchCode(), "", "")
	if reused.StatusCode != http.StatusForbidden {
		t.Errorf("the launch code a second time: status %d, want 403", reused.StatusCode)
	}
}

func TestAppAuthGuardsTheWebSocketUpgrade(t *testing.T) {
	s, auth, base := startAppServer(t)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := "ws" + strings.TrimPrefix(base, "http") + "/ws"

	_, response, err := websocket.Dial(ctx, url, nil)
	if err == nil {
		t.Fatal("the WebSocket opened without the token")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Errorf("upgrade without the token: response %v, want 401", response)
	}

	for _, header := range []http.Header{
		{"Authorization": {"Bearer " + auth.Token()}},
		{"Cookie": {AppSessionCookieName(s.Port()) + "=" + auth.Token()}},
	} {
		conn, _, err := websocket.Dial(ctx, url, &websocket.DialOptions{HTTPHeader: header})
		if err != nil {
			t.Errorf("upgrade with %v: %v", header, err)
			continue
		}
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}
}

func TestAppAuthLetsTheTunnelIn(t *testing.T) {
	s, _, base := startAppServer(t)
	s.SetAllowedOrigins([]string{"quiet-river.trycloudflare.com", "https://quiet-river.trycloudflare.com"})
	throughTunnel := func() int {
		request, err := http.NewRequest(http.MethodGet, base+"/api/presentation", nil)
		if err != nil {
			t.Fatal(err)
		}
		request.Host = "quiet-river.trycloudflare.com"
		response, err := http.DefaultClient.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		return response.StatusCode
	}

	s.SetTunnelHost("quiet-river.trycloudflare.com")
	if status := throughTunnel(); status != http.StatusOK {
		t.Errorf("through the running tunnel: status %d, want 200", status)
	}
	s.SetTunnelHost("")
	if status := throughTunnel(); status != http.StatusUnauthorized {
		t.Errorf("after the tunnel stopped: status %d, want 401", status)
	}
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
