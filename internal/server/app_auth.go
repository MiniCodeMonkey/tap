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

// audienceRoutes are the routes reachable with no app token: what an
// audience member's browser or a phone remote needs to render the deck and
// follow along, whatever address they reached the server on. /presenter
// keeps its own presenter-password gate on top (see handlePresenter);
// nothing here needs a second one. Everything else, including every route
// under AppSourcePath's /api/app/ prefix and POST /api/execute (which runs
// code), always needs the token: see needsAppToken for why route identity,
// not the request's Host header, is what decides.
var audienceRoutes = map[string]struct{}{
	"GET /{$}":                  {},
	"GET /index.html":           {},
	"GET /presenter":            {},
	"GET /presenter.html":       {},
	"GET /presenter/":           {},
	"GET /api/presentation":     {},
	"GET /api/custom-theme.css": {},
	"GET /assets/":              {},
	"GET /local/":               {},
	"GET /components/":          {},
	"GET /ws":                   {},
}

// needsAppToken reports whether r must carry the app token to reach its
// handler. It asks the server's own mux which registered pattern r
// matches - the exact lookup that will dispatch the request right after -
// and checks that pattern against audienceRoutes. A request that matches
// no registered pattern needs the token too, so an unknown path is never
// accidentally exempt.
//
// This is deliberately not a Host check. A pattern like
// "the request came through the tunnel" sounds like an authorization
// decision, but Host is a header the client sends, so any local process,
// and once a tunnel is up, anyone who has seen the tunnel URL the
// presenter shares with the room, can set it to whatever they like. Route
// identity comes from the server's own routing table instead, which a
// request cannot forge by sending a header.
func (s *Server) needsAppToken(r *http.Request) bool {
	_, pattern := s.mux.Handler(r)
	_, exempt := audienceRoutes[pattern]
	return !exempt
}

// authorize reports whether r may go on to the routes. When it may not, it
// has already answered: 401 without the token, a redirect that sets the
// cookie for a valid launch code, and 403 for a used or wrong one.
func (a *AppAuth) authorize(w http.ResponseWriter, r *http.Request, s *Server) bool {
	if code := r.URL.Query().Get("launch"); code != "" {
		a.exchangeLaunchCode(w, r, code, s.Port())
		return false
	}
	if !s.needsAppToken(r) {
		return true
	}
	if a.carriesToken(r, s.Port()) {
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

// SetAppAuth puts the server behind auth: from then on every request to a
// route outside audienceRoutes needs the token. tap dev --app and tap
// present --app call it before Start.
func (s *Server) SetAppAuth(auth *AppAuth) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appAuth = auth
}
