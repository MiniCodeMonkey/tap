package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// AppAuth is the per-launch secret of a tap --app server. The app sends
// the token as "Authorization: Bearer <token>", which forces a CORS
// preflight that the server never approves. The app's pages get the token
// as a cookie in exchange for the one-time launch code, so the token never
// appears in a URL.
type AppAuth struct {
	token      string
	launchCode string
	mu         sync.Mutex
	launchUsed bool
}

// NewAppAuth makes a token and a launch code, 32 random bytes each.
func NewAppAuth() (*AppAuth, error) {
	token, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	launchCode, err := randomHex(32)
	if err != nil {
		return nil, err
	}
	return &AppAuth{token: token, launchCode: launchCode}, nil
}

func randomHex(size int) (string, error) {
	buffer := make([]byte, size)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("generate an app secret: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}

// Token is the secret every request must carry.
func (a *AppAuth) Token() string { return a.token }

// LaunchCode is the one-time code the app puts in the first URL it loads.
func (a *AppAuth) LaunchCode() string { return a.launchCode }

// AppSessionCookieName is the cookie that carries the token for the server
// on port. Cookies do not keep ports apart, and the app runs tap dev and
// tap present side by side on 127.0.0.1, so the name carries the port.
func AppSessionCookieName(port int) string {
	return "tap_app_" + strconv.Itoa(port)
}

// authorize reports whether r may go on to the routes. When it may not, it
// has already answered: 401 without the token, a redirect that sets the
// cookie for a valid launch code, and 403 for a used or wrong one.
func (a *AppAuth) authorize(w http.ResponseWriter, r *http.Request, port int) bool {
	if code := r.URL.Query().Get("launch"); code != "" {
		a.exchangeLaunchCode(w, r, code, port)
		return false
	}
	if a.carriesToken(r, port) {
		return true
	}
	w.Header().Set("WWW-Authenticate", `Bearer realm="tap"`)
	http.Error(w, "Unauthorized: this tap server belongs to the Tap app", http.StatusUnauthorized)
	return false
}

// carriesToken reports whether r has the token, in the Authorization
// header or in the session cookie.
func (a *AppAuth) carriesToken(r *http.Request, port int) bool {
	if bearer, found := strings.CutPrefix(r.Header.Get("Authorization"), "Bearer "); found && secretsEqual(bearer, a.token) {
		return true
	}
	cookie, err := r.Cookie(AppSessionCookieName(port))
	return err == nil && secretsEqual(cookie.Value, a.token)
}

// exchangeLaunchCode sets the session cookie for a launch code that was
// never used, and redirects to the same URL without the code.
func (a *AppAuth) exchangeLaunchCode(w http.ResponseWriter, r *http.Request, code string, port int) {
	if r.Method != http.MethodGet {
		http.Error(w, "Forbidden: a launch code works only on a page load", http.StatusForbidden)
		return
	}
	a.mu.Lock()
	valid := !a.launchUsed && secretsEqual(code, a.launchCode)
	if valid {
		a.launchUsed = true
	}
	a.mu.Unlock()
	if !valid {
		http.Error(w, "Forbidden: this launch code is wrong or was already used", http.StatusForbidden)
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     AppSessionCookieName(port),
		Value:    a.token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
	})
	target := *r.URL
	query := target.Query()
	query.Del("launch")
	target.RawQuery = query.Encode()
	http.Redirect(w, r, target.RequestURI(), http.StatusSeeOther)
}

// secretsEqual compares two secrets in constant time.
func secretsEqual(given, want string) bool {
	return subtle.ConstantTimeCompare([]byte(given), []byte(want)) == 1
}

// SetAppAuth puts the server behind auth: from then on every request needs
// the token. tap dev --app and tap present --app call it before Start.
func (s *Server) SetAppAuth(auth *AppAuth) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appAuth = auth
}

// SetTunnelHost tells the server the host of the running tunnel, or ""
// when none runs.
func (s *Server) SetTunnelHost(host string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.tunnelHost = host
}

// arrivedThroughTunnel reports whether r came in through the running
// tunnel, whose host is tunnelHost.
func arrivedThroughTunnel(r *http.Request, tunnelHost string) bool {
	return tunnelHost != "" && hostnameWithoutPort(r.Host) == tunnelHost
}
