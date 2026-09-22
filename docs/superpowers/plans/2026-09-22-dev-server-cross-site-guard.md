# Dev Server Cross-Site Guard Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stop a web page on another origin from triggering `POST /api/execute` on a running `tap dev` server.

**Architecture:** `requireAllowedHost` only checks the `Host` header. That blocks DNS rebinding, but a page on `evil.com` can still send a "simple" cross-site `POST` (a `text/plain` body, so no CORS preflight) to `http://127.0.0.1:3000/api/execute`. The `Host` is then `127.0.0.1:3000`, which passes. The server runs the request even though the browser hides the response. This plan adds a second middleware, `requireSameOriginJSON`, that rejects any mutating request whose `Origin` is foreign or whose body is not `application/json`. The origin rule is the one the WebSocket hub already uses, pulled out into one shared function so HTTP and WebSocket cannot drift apart.

**Tech Stack:** Go 1.21+, `net/http`, `httptest`.

**Spec:** `docs/superpowers/specs/2026-09-22-tap-desktop-design.md` (section "Security", item 1, and milestone 1). Only the Origin and Content-Type checks are in scope. The bearer token is for the future `tap dev --app` mode and is out of scope here.

## Global Constraints

- Keep the existing `isAllowedHost` Host check exactly as it is. The new check is added on top of it.
- Behavior of `WebSocketHub.checkOrigin` must not change. Its existing tests (`TestWebSocketHubOriginCheck`, `TestWebSocketHubCheckOrigin_RejectsDNSRebinding`) must pass unchanged.
- A request with no `Origin` header stays allowed. Only browsers send `Origin`, and curl, tests and the future desktop app do not.
- The frontend already sends `Content-Type: application/json` from `frontend/src/lib/components/LiveCodeBlock.tsx:105`, so no frontend change is needed.
- `--allow-origin` values (for example a Vite dev server at `http://localhost:5173`) and the tunnel origins set in `internal/cli/tunnel.go:96` must keep working for `/api/execute`.
- Identifiers are spelled out in full. Code comments describe the present and contain no ticket numbers or "before the fix" framing.

---

### Task 1: Shared origin rule

**Files:**
- Modify: `internal/server/allowed_host.go` (add `isAllowedOrigin`)
- Modify: `internal/server/websocket.go:272-288` (`checkOrigin` calls `isAllowedOrigin`)
- Test: `internal/server/allowed_host_test.go`

**Interfaces:**
- Produces: `func isAllowedOrigin(origin string, requestHost string, allowedHosts map[string]struct{}, allowedOrigins map[string]struct{}) bool`

- [ ] **Step 1: Write the failing test**

Append to `internal/server/allowed_host_test.go`:

```go
func TestIsAllowedOrigin(t *testing.T) {
	allowedOrigins := map[string]struct{}{"http://localhost:5173": {}}
	allowedHosts := allowedHostsFromOrigins([]string{"http://localhost:5173"})

	cases := []struct {
		name        string
		origin      string
		requestHost string
		want        bool
	}{
		{"no origin header", "", "127.0.0.1:3000", true},
		{"same origin on loopback", "http://127.0.0.1:3000", "127.0.0.1:3000", true},
		{"same origin on localhost", "http://localhost:3000", "localhost:3000", true},
		{"explicitly allowed origin", "http://localhost:5173", "localhost:3000", true},
		{"foreign site", "https://evil.com", "127.0.0.1:3000", false},
		{"opaque null origin", "null", "127.0.0.1:3000", false},
		{"same host but different port", "http://localhost:4000", "localhost:3000", false},
		{"rebinding name matching its own host", "http://evil.com:3000", "evil.com:3000", false},
		{"unparseable origin", "http://[::1", "127.0.0.1:3000", false},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			got := isAllowedOrigin(testCase.origin, testCase.requestHost, allowedHosts, allowedOrigins)
			if got != testCase.want {
				t.Errorf("isAllowedOrigin(%q, %q) = %v, want %v", testCase.origin, testCase.requestHost, got, testCase.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server -run TestIsAllowedOrigin -v`
Expected: FAIL to compile with `undefined: isAllowedOrigin`.

- [ ] **Step 3: Write minimal implementation**

Add to `internal/server/allowed_host.go` (add `"net/url"` to the imports):

```go
// isAllowedOrigin reports whether a request carrying the given Origin
// header may act on this server. No Origin at all is accepted, because
// only a browser sends one. Otherwise the origin must either name this
// same server (its host equals the request's Host header, and that host
// passes isAllowedHost, which defeats DNS rebinding) or be listed exactly
// in allowedOrigins (the --allow-origin flag and tunnel origins). An
// opaque "null" origin never matches either rule.
func isAllowedOrigin(origin string, requestHost string, allowedHosts map[string]struct{}, allowedOrigins map[string]struct{}) bool {
	if origin == "" {
		return true
	}
	if _, exactlyAllowed := allowedOrigins[origin]; exactlyAllowed {
		return true
	}
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}
	return originURL.Host == requestHost && isAllowedHost(requestHost, allowedHosts)
}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/server -run TestIsAllowedOrigin -v`
Expected: PASS.

- [ ] **Step 5: Make `checkOrigin` use the shared rule**

Replace the body of `checkOrigin` in `internal/server/websocket.go` (keep its doc comment, which still describes the behavior) with:

```go
func (h *WebSocketHub) checkOrigin(r *http.Request) bool {
	h.mu.RLock()
	allowedHosts := h.allowedHosts
	allowedOrigins := h.allowedOrigins
	h.mu.RUnlock()

	return isAllowedOrigin(r.Header.Get("Origin"), r.Host, allowedHosts, allowedOrigins)
}
```

Remove the `"net/url"` import from `websocket.go` if nothing else in that file uses it.

- [ ] **Step 6: Run the WebSocket origin tests**

Run: `go test ./internal/server -run 'TestWebSocketHubOriginCheck|TestWebSocketHubCheckOrigin' -v`
Expected: PASS with no test changes.

- [ ] **Step 7: Commit**

```bash
git add internal/server/allowed_host.go internal/server/allowed_host_test.go internal/server/websocket.go
git commit -m "refactor(server): share the origin rule between HTTP and WebSocket"
```

---

### Task 2: Guard mutating endpoints

**Files:**
- Modify: `internal/server/server.go` (store `allowedOrigins` on `Server`, set it in `SetAllowedOrigins`, add `requireSameOriginJSON`)
- Modify: `internal/server/routes.go:45` (wrap `POST /api/execute`)
- Test: `internal/server/server_test.go`

**Interfaces:**
- Consumes: `isAllowedOrigin` from Task 1.
- Produces: `func (s *Server) requireSameOriginJSON(next http.HandlerFunc) http.HandlerFunc`. Every future mutating route (including the desktop app's `/api/app/*` routes) wraps its handler with `s.requireAllowedHost(s.requireSameOriginJSON(handler))`.

- [ ] **Step 1: Write the failing test**

Append to `internal/server/server_test.go`. It drives the real mux the same way `internal/server/benchmark_test.go:138` does, so it proves the route is wired, not just the middleware. With no registry set, a request that gets through the guard reaches the handler and returns 500 "Driver registry not configured". A blocked request never reaches the handler.

```go
func TestExecuteRejectsCrossSiteRequests(t *testing.T) {
	srv := New(0)
	srv.SetupRoutes()
	srv.SetAllowedOrigins([]string{"http://localhost:5173"})

	body := `{"driver":"shell","code":"echo hi"}`
	cases := []struct {
		name        string
		origin      string
		contentType string
		wantStatus  int
	}{
		{"foreign site with text/plain", "https://evil.com", "text/plain", http.StatusForbidden},
		{"foreign site with json", "https://evil.com", "application/json", http.StatusForbidden},
		{"opaque null origin", "null", "application/json", http.StatusForbidden},
		{"same origin with text/plain", "http://127.0.0.1:3000", "text/plain", http.StatusUnsupportedMediaType},
		{"no origin with form encoding", "", "application/x-www-form-urlencoded", http.StatusUnsupportedMediaType},
		{"no content type", "", "", http.StatusUnsupportedMediaType},
		{"same origin with json", "http://127.0.0.1:3000", "application/json", http.StatusInternalServerError},
		{"same origin with json and charset", "http://127.0.0.1:3000", "application/json; charset=utf-8", http.StatusInternalServerError},
		{"allowed dev origin with json", "http://localhost:5173", "application/json", http.StatusInternalServerError},
		{"no origin with json", "", "application/json", http.StatusInternalServerError},
	}
	for _, testCase := range cases {
		t.Run(testCase.name, func(t *testing.T) {
			request := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:3000/api/execute", strings.NewReader(body))
			if testCase.origin != "" {
				request.Header.Set("Origin", testCase.origin)
			}
			if testCase.contentType != "" {
				request.Header.Set("Content-Type", testCase.contentType)
			}
			recorder := httptest.NewRecorder()
			srv.mux.ServeHTTP(recorder, request)
			if recorder.Code != testCase.wantStatus {
				t.Errorf("status = %d, want %d (body: %s)", recorder.Code, testCase.wantStatus, recorder.Body.String())
			}
		})
	}
}
```

Add `"net/http/httptest"` and `"strings"` to the test file's imports if they are not there yet.

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/server -run TestExecuteRejectsCrossSiteRequests -v`
Expected: FAIL. The forbidden and unsupported-media-type cases get 500, because nothing blocks them today.

- [ ] **Step 3: Store allowed origins on the server**

In the `Server` struct in `internal/server/server.go`, next to `allowedHosts`, add:

```go
	// allowedOrigins is the --allow-origin flag as full origins
	// ("http://localhost:5173"), matched exactly against a mutating
	// request's Origin header by requireSameOriginJSON. Mirrors
	// WebSocketHub.allowedOrigins; tap dev sets both from the same value.
	allowedOrigins map[string]struct{}
```

Change `SetAllowedOrigins` to fill both maps:

```go
func (s *Server) SetAllowedOrigins(origins []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allowedOrigins = make(map[string]struct{}, len(origins))
	for _, origin := range origins {
		s.allowedOrigins[origin] = struct{}{}
	}
	s.allowedHosts = allowedHostsFromOrigins(origins)
}
```

Update its doc comment so it says it also sets the origins that `requireSameOriginJSON` accepts.

- [ ] **Step 4: Add the middleware**

Add below `requireAllowedHost` in `internal/server/server.go` (add `"mime"` to the imports):

```go
// requireSameOriginJSON wraps a mutating handler so it only runs for a
// request that a page on another site could not have sent. The Origin
// header must pass isAllowedOrigin, and the body must be declared as
// application/json. A cross-site page can send a text/plain or form POST
// without a CORS preflight, and requireAllowedHost alone lets it through
// because its Host header is this machine's own; a JSON content type
// forces the preflight this server never approves.
func (s *Server) requireSameOriginJSON(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		allowedHosts := s.allowedHosts
		allowedOrigins := s.allowedOrigins
		s.mu.RUnlock()

		if !isAllowedOrigin(r.Header.Get("Origin"), r.Host, allowedHosts, allowedOrigins) {
			http.Error(w, "Forbidden: cross-origin request; use --allow-origin to allow it", http.StatusForbidden)
			return
		}
		mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
		if err != nil || mediaType != "application/json" {
			http.Error(w, "Unsupported Media Type: send application/json", http.StatusUnsupportedMediaType)
			return
		}
		next(w, r)
	}
}
```

- [ ] **Step 5: Wrap the route**

In `internal/server/routes.go`, change the execute route to:

```go
	s.mux.HandleFunc("POST /api/execute", s.requireAllowedHost(s.requireSameOriginJSON(s.handleAPIExecute)))
```

- [ ] **Step 6: Run the new test and the whole package**

Run: `go test ./internal/server -run TestExecuteRejectsCrossSiteRequests -v`
Expected: PASS.

Run: `go test ./internal/server/...`
Expected: PASS. The existing `TestHandleAPIExecute_*` tests call the handler directly, so the guard does not affect them. If any existing test sends a POST through the mux without `Content-Type: application/json`, add that header to the test's request; do not loosen the guard.

- [ ] **Step 7: Commit**

```bash
git add internal/server/server.go internal/server/routes.go internal/server/server_test.go
git commit -m "fix(server): reject cross-site requests to /api/execute"
```

---

### Task 3: End-to-end check, changelog, and the registry finding

**Files:**
- Modify: `CHANGELOG.md` (under `## [Unreleased]`)

- [ ] **Step 1: Check the guard against a real server**

Build and start a dev server on a sample deck, then send the attack request and a normal request:

```bash
go build -o /tmp/tap-guard ./cmd/tap
/tmp/tap-guard dev examples/basic.md --port 3917 --headless &
sleep 2
curl -s -o /dev/null -w '%{http_code}\n' -X POST http://127.0.0.1:3917/api/execute \
  -H 'Origin: https://evil.com' -H 'Content-Type: text/plain' --data '{"driver":"shell","code":"echo hi"}'
curl -s -o /dev/null -w '%{http_code}\n' -X POST http://127.0.0.1:3917/api/execute \
  -H 'Content-Type: application/json' --data '{"driver":"shell","code":"echo hi"}'
kill %1
```

Expected: the first prints `403`. The second is not `403` or `415`.

- [ ] **Step 2: Record the registry finding**

No production code calls `Server.SetRegistry` (only `internal/server/api_test.go` does), so in a real `tap dev` session `/api/execute` probably answers 500 "Driver registry not configured". The second curl in Step 1 shows which is true. Do not fix it in this branch. Write the result (the status code and response body of the second curl) in the pull request description under a "Note" heading, so it can be handled separately. The README advertises live code execution, so this is worth a separate issue if confirmed.

- [ ] **Step 3: Add the changelog entry**

Add under `## [Unreleased]` in `CHANGELOG.md`, in a `### Security` section (create it above `### Changed`):

```markdown
### Security

- **`tap dev` rejects cross-site requests to run code** - A web page open in the same browser could send a plain `POST` to `http://127.0.0.1:<port>/api/execute`. The browser hid the response, but the server still ran the request. `tap dev` now refuses any code execution request whose `Origin` is another site or whose body is not `application/json`, with 403 and 415 respectively. The slides' own Run buttons, `--allow-origin` origins and tunnel origins keep working. The same origin rule now guards both HTTP and WebSocket connections.
```

- [ ] **Step 4: Run the full test suite**

Run: `go test ./...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs(changelog): note the cross-site guard on tap dev"
```
