# Ready signal, in-place updates and progress output: implementation plan

## Controller rulings on the open questions (2026-09-22)

- The plan's defaults stand for all eight questions, and the three additions beyond the spec (`revision` on `/api/presentation`, `version` on `connected`, and the extra blockers) are accepted.
- Question 2: in `--progress json` mode, stderr lines that start with `{` are progress; any other stderr line is a plain-text diagnostic. Document this rule in `docs/reference/cli-commands.md`.

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Give tap one render-ready signal that the exporters and Tap Desktop wait for, update open pages in place instead of reloading them, and print machine-readable progress from `tap export pdf`, `tap export images` and `tap build`.

**Architecture:** The frontend owns readiness. Components that load or animate hold a "blocker" in a small registry (`frontend/src/lib/ready/blockers.ts`). A ready cycle starts on every change of revision, slide, step, fragment or theme; it waits for stylesheets and fonts, images, zero blockers and animations, then publishes `window.__tapReady = {revision, slide, step}`, a `tap:ready` event, and a `tapReady` WebKit message. The Go exporter drops its own checklist and waits for `window.__tapReady.slide`. On the server, each transformed slide carries a content hash; a deck change sends `{"type": "update", "revision", "slides"}`, and the page fetches `/api/presentation`, keeps unchanged slide objects by hash, and clamps its position. `--progress json` is one reporter in `internal/cli` that writes JSON lines to stderr.

**Tech Stack:** Go 1.24, cobra, `github.com/coder/websocket`, playwright-go (`github.com/mxschmitt/playwright-go`), React 19, Zustand, Motion, MapLibre, Vitest with jsdom.

**Spec:** `docs/superpowers/specs/2026-09-22-tap-desktop-prerequisites-design.md`, "Part 5: Render-ready signal and progress" (5.1, 5.2, 5.3 and its Tests). Part 6 and `tap-desktop-features/13-performance.feature` show how the app uses the signal and `update`. The roadmap's "Contracts between plans" section (`docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md`) fixes the names this plan produces. The findings behind 5.3 are in `tap-desktop-features/spike-live-updates.md`, and the thumbnail findings in `tap-desktop-features/spike-native-prototype.md`. All of these are on the `docs/tap-desktop` branch.

## Global Constraints

- Branch `feat/ready-signal`, from an up-to-date `main` after plan P1 (`2026-09-22-cli-command-tree-and-driver-registry.md`) has merged, in its own worktree under `/Users/codemonkey/projects/`. One pull request.
- This plan is written against the code after P1: `tap pdf` is `tap export pdf` in `internal/cli/export_pdf.go`, `tap screenshot` is `tap export images` in `internal/cli/export_images.go`, commands return errors through `execute`, `printJSONOK`, `jsonRequested`, `userError`, `internalError`, `reportedError`, the `code*` constants, `runTap`, `flagConventions`, `buildTapBinaryForTest`, `freePort` and `requireBrowser` exist, and `tap dev` binds `127.0.0.1`.
- Contract names, exactly (roadmap, "From P5"):
  - `window.__tapReady = {revision, slide, step}`; the `tap:ready` event; `window.webkit.messageHandlers.tapReady.postMessage({revision, slide, step})` when that handler exists.
  - WebSocket message `{"type": "update", "revision": "...", "slides": [<1-based numbers>]}`.
  - `--progress json` lines on stderr: `{"phase": "...", "done": n, "total": n}`, `{"phase": "download", "bytes": n, "totalBytes": n}`, then one final result line.
- The signal settles on: fonts loaded, images complete, maps reported ready, components settled, no error card pending, transitions and theme animations finished. It resets when the slide or step changes.
- `tap export pdf`, `tap export images` and the thumbnail renderer wait for this one signal. The old checklist in `internal/pdf/exporter.go` and `internal/pdf/capture.go` is removed. Export output must not change.
- Slides re-render only when their content changed, keyed by a content hash. An update clamps slide, fragment and step to the new counts.
- `reload` stays for a changed custom theme CSS file and a new tap version.
- Numbers a person or a program sees are 1-based: `slide` in the signal, `slides` in `update`.
- Spell identifiers out in full (`options`, `presentation`, `revision`), in Go and TypeScript.
- Code comments describe the present. No ticket numbers, no "before the fix" wording.
- No em dashes in docs, changelog, comments or test names. Use `--`, a comma, or a new sentence.
- A `--json` or progress struct declares its fields in output order.
- Frontend tests: `cd frontend && npm test -- --run`. Typecheck: `cd frontend && npm run check`. Lint: `cd frontend && npm run lint`.
- Go browser tests run the embedded frontend. Run `make frontend` before any Go test that starts Chromium, or it tests a stale build.

## Open questions

These are gaps in the spec. The plan takes the marked decision so work can go ahead; each one is easy to change later.

1. **"No error card is still pending."** The spec does not define it. The plan reads it as: a component whose bundle failed to load, whose error card (or build fallback) has not been committed yet. A deck component holds an `error-card` blocker in that window. A slide error boundary renders its card in the same commit that catches the error, so it needs no blocker.
2. **Log lines in `--progress json` mode.** Warnings (a broken slide, a component warning) stay human text on stderr. The spec says "one JSON line per step". The plan's rule for readers: parse only lines that start with `{` and have a `phase` field. The final line repeats every warning in its result (`brokenSlides`), so nothing is lost. Say if stderr must be JSON only.
3. **Download progress comes from Playwright's own text.** Playwright's installer prints `|■■■■■■■■      |  10% of 162.3 MiB` lines when stdout is not a terminal. The plan parses those. A first install downloads several archives (Chromium, the headless shell, ffmpeg), so `bytes` restarts at 0 for each one. A Playwright upgrade could change the format; the parser test pins today's.
4. **Fragments.** The payload has no fragment field (contract), but a fragment change restarts the cycle, so `__tapReady` is null while a fragment fades in. Two states that differ only in fragment report the same payload.
5. **The `r` key and P6's stdin `reload`** force a full page reload, as today, instead of an update. A person presses `r` to recover a page, so the plan keeps the stronger action.
6. **A reconnect with a different revision still reloads.** The socket was down, so the page may have missed a custom theme change as well. Only the live `update` path is in place.
7. **Infinite animations.** A print or capture page waits for every animation for up to 3 s, as the exporter does today, so export output does not change. A live page (`tap dev` in a browser, the app's preview) ignores animations that repeat forever, so a spinner does not delay every cycle by 3 s.
8. **`tap build` phases.** The spec gives no names. The plan uses `load`, `parse`, `bundle` and `write`, with `total` 4.

## Decisions this plan makes

- The slide hash leaves out the slide's position (`index`), so a slide that only moved keeps its hash. The frontend reuses an old slide object only when the slide at the same position has the same hash, so a moved slide is still a new object with the right `index`.
- `/api/presentation` gains a `revision` field, because a print page has no WebSocket and still reports the revision.
- The hub's `connected` message gains a `version` field. A reconnect to a different tap version reloads the page.
- A save that changes nothing on screen (same revision) sends nothing.
- `tap export pdf --content both` and `--content notes` load `/presenter`, so the presenter page publishes the signal too.
- Stylesheets count as part of "fonts are loaded": a font only starts loading after the stylesheet that declares it has loaded.
- Mermaid diagrams, asciinema players and Shiki highlighting count as components: the slide holds a `component` blocker while they render.
- The theme counts as fonts: the page holds a `fonts` blocker until the requested theme's CSS and fonts are applied.
- Time limits stay as the exporter has them today: 5 s for images and stylesheets, 3 s for a map to go idle after it loads, 3 s for animations. The Go side gives a slide 30 s to become ready.

## File structure

| File | Status | Responsibility |
|---|---|---|
| `internal/transformer/transformer.go` | modify | `TransformedSlide.Hash`, `SlideHash` |
| `internal/server/revision.go` | modify | `ChangedSlides` |
| `internal/server/server.go` | modify | `SetRevision`, `Revision` |
| `internal/server/routes.go` | modify | `/api/presentation` carries `revision` |
| `internal/server/websocket.go` | modify | `MessageUpdate`, `UpdateMessage`, `BroadcastUpdate`, `SetVersion`, `version` on `connected` |
| `internal/cli/publish.go` | new | `deckPublisher`: update or reload after a deck change |
| `internal/cli/dev.go` | modify | Every reload path goes through `deckPublisher` |
| `internal/pdf/exporter.go`, `internal/pdf/capture.go` | modify | Wait for the ready signal; the checklist is removed |
| `internal/pdf/ready.go` | new | `waitForReady` |
| `internal/pdf/progress.go` | new | `Progress`, `SetProgress`, the install progress parser |
| `internal/cli/progress.go` | new | `progressReporter`, `--progress json` lines |
| `internal/cli/root.go` | modify | The final failure line in `execute` |
| `internal/cli/export_pdf.go`, `export_images.go`, `build.go` | modify | `--progress` |
| `frontend/src/lib/types.ts` | modify | `Slide.hash`, `Presentation.revision`, the `update` message |
| `frontend/src/lib/stores/presentation.ts` | modify | `updatePresentationInPlace`, `slideKey` |
| `frontend/src/lib/stores/websocket.ts` | modify | Handles `update`; reloads on a new tap version |
| `frontend/src/lib/ready/blockers.ts` | new | The blocker registry and `useReadyHold` |
| `frontend/src/lib/ready/probes.ts` | new | DOM probes: stylesheets and fonts, images, animations, paint |
| `frontend/src/lib/ready/readySignal.ts` | new | The cycle and `publishReady` |
| `frontend/src/lib/ready/useReadySignal.ts` | new | The hook the two pages call |
| `frontend/src/lib/components/DeckComponent.tsx` | modify | `component` and `error-card` blockers |
| `frontend/src/lib/hooks/useRichBlocks.ts` | modify | `component` blocker while rich blocks render |
| `frontend/src/lib/components/MapSlide.tsx` | modify | `map` blocker |
| `frontend/src/lib/components/SlideTransition.tsx` | modify | `animations` blocker |
| `frontend/src/lib/hooks/useResolvedTheme.ts` | modify | `fonts` blocker while a theme loads |
| `frontend/src/lib/components/Slide.tsx` | modify | `memo` |
| `frontend/src/lib/components/SlideOverview.tsx`, `frontend/src/PresenterApp.tsx` | modify | Keys from `slideKey` |
| `frontend/src/App.tsx`, `frontend/src/PresenterApp.tsx` | modify | Call `useReadySignal` |
| Docs | modify | `docs/reference/cli-commands.md`, `docs/guide/building-export.md`, `skills/tap/rules/cli.md`, `CHANGELOG.md`, `docs/changelog.md` |

---

### Task 1: A content hash on every slide

**Files:**
- Modify: `internal/transformer/transformer.go` (`TransformedSlide`, `Transform`)
- Test: `internal/transformer/transformer_test.go`

**Interfaces:**
- Produces:
  - `TransformedSlide.Hash string` with JSON name `hash`
  - `func SlideHash(slide TransformedSlide) string`: 12 hex characters, the same for two slides whose JSON is equal apart from `index` and `hash`
  - `Transform` sets `Hash` on every slide it returns

- [ ] **Step 1: Write the failing tests**

Add to `internal/transformer/transformer_test.go`:

```go
func transformMarkdown(t *testing.T, markdown string) *TransformedPresentation {
	t.Helper()
	parsed, err := parser.New().Parse([]byte(markdown))
	if err != nil {
		t.Fatalf("Parse() error = %v", err)
	}
	return New(config.DefaultConfig()).Transform(parsed)
}

func TestTransformSetsASlideHash(t *testing.T) {
	presentation := transformMarkdown(t, "# One\n\n---\n\n# Two\n")
	for index, slide := range presentation.Slides {
		if len(slide.Hash) != 12 {
			t.Errorf("slide %d hash = %q, want 12 hex characters", index+1, slide.Hash)
		}
		if slide.Hash != SlideHash(slide) {
			t.Errorf("slide %d hash = %q, want SlideHash() = %q", index+1, slide.Hash, SlideHash(slide))
		}
	}
	if presentation.Slides[0].Hash == presentation.Slides[1].Hash {
		t.Error("two slides with different content have the same hash")
	}
}

func TestSlideHashChangesOnlyWithContent(t *testing.T) {
	before := transformMarkdown(t, "# One\n\n---\n\n# Two\n\n---\n\n# Three\n")
	after := transformMarkdown(t, "# One\n\n---\n\n# Two, edited\n\n---\n\n# Three\n")

	if before.Slides[0].Hash != after.Slides[0].Hash {
		t.Error("slide 1 did not change, but its hash did")
	}
	if before.Slides[1].Hash == after.Slides[1].Hash {
		t.Error("slide 2 changed, but its hash did not")
	}
	if before.Slides[2].Hash != after.Slides[2].Hash {
		t.Error("slide 3 did not change, but its hash did")
	}
}

func TestSlideHashLeavesOutThePosition(t *testing.T) {
	slide := TransformedSlide{Index: 0, Layout: "default", HTML: "<h1>Same</h1>"}
	moved := slide
	moved.Index = 7
	if SlideHash(slide) != SlideHash(moved) {
		t.Error("a slide that only moved got a different hash")
	}
	withOldHash := slide
	withOldHash.Hash = "ffffffffffff"
	if SlideHash(slide) != SlideHash(withOldHash) {
		t.Error("SlideHash() depends on the slide's own Hash field")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/transformer -run 'TestTransformSetsASlideHash|TestSlideHash' -v`
Expected: compile failure, `slide.Hash undefined` and `undefined: SlideHash`.

- [ ] **Step 3: Add the field and the function**

In `internal/transformer/transformer.go`, add `"crypto/sha256"` and `"encoding/hex"` to the imports (`encoding/json` is already there).

In `TransformedSlide`, after the `Components` field and before `StepsInvalid`, add:

```go
	// Hash identifies the slide's content (see SlideHash). The frontend
	// keeps a slide it already rendered when the slide at the same
	// position has the same hash, and tap dev lists the slides whose hash
	// changed in its "update" message.
	Hash string `json:"hash"`
```

After the `Transform` function, add:

```go
// SlideHash returns a short hash of everything the frontend renders for
// slide: its JSON with Index and Hash left out, so a slide that only moved
// keeps its hash. json.Marshal sorts map keys, so equal slides always give
// equal hashes.
func SlideHash(slide TransformedSlide) string {
	slide.Index = 0
	slide.Hash = ""
	data, err := json.Marshal(slide)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:6])
}
```

In `Transform`, change the loop body to set the hash:

```go
	for _, slide := range pres.Slides {
		transformed := t.transformSlide(slide)
		transformed.Hash = SlideHash(transformed)
		result.Slides = append(result.Slides, transformed)
	}
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `go test ./internal/transformer -v -run 'TestTransformSetsASlideHash|TestSlideHash'`
Expected: PASS.

Run: `go test ./internal/... -short`
Expected: PASS. No existing test compares whole `TransformedSlide` values, so the new field breaks nothing. If one does, set `Hash` in its expected value with `SlideHash`.

- [ ] **Step 5: Commit**

```bash
git add internal/transformer/transformer.go internal/transformer/transformer_test.go
git commit -m "feat(transformer): give every slide a content hash"
```

---

### Task 2: The revision on `/api/presentation`, and which slides changed

**Files:**
- Modify: `internal/server/revision.go`
- Modify: `internal/server/server.go`
- Modify: `internal/server/routes.go` (`handleAPIPresentation`)
- Test: `internal/server/revision_test.go`, `internal/server/routes_test.go`

**Interfaces:**
- Consumes: `TransformedSlide.Hash` (Task 1)
- Produces:
  - `func ChangedSlides(previous, next *transformer.TransformedPresentation) []int`: 1-based, never nil
  - `func (s *Server) SetRevision(revision string)` and `func (s *Server) Revision() string`
  - `GET /api/presentation` answers `{"config": ..., "slides": [...], "revision": "..."}`

- [ ] **Step 1: Write the failing tests**

Add to `internal/server/revision_test.go`:

```go
func presentationWithHashes(hashes ...string) *transformer.TransformedPresentation {
	presentation := &transformer.TransformedPresentation{}
	for index, hash := range hashes {
		presentation.Slides = append(presentation.Slides, transformer.TransformedSlide{Index: index, Hash: hash})
	}
	return presentation
}

func TestChangedSlides(t *testing.T) {
	tests := []struct {
		name     string
		previous *transformer.TransformedPresentation
		next     *transformer.TransformedPresentation
		want     []int
	}{
		{"nothing changed", presentationWithHashes("a", "b", "c"), presentationWithHashes("a", "b", "c"), []int{}},
		{"the second slide changed", presentationWithHashes("a", "b", "c"), presentationWithHashes("a", "x", "c"), []int{2}},
		{"a slide was added at the end", presentationWithHashes("a", "b"), presentationWithHashes("a", "b", "c"), []int{3}},
		{"the last slide was removed", presentationWithHashes("a", "b", "c"), presentationWithHashes("a", "b"), []int{}},
		{"a slide was inserted first", presentationWithHashes("a", "b"), presentationWithHashes("z", "a", "b"), []int{1, 2, 3}},
		{"no previous deck", nil, presentationWithHashes("a", "b"), []int{1, 2}},
		{"no next deck", presentationWithHashes("a"), nil, []int{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ChangedSlides(tt.previous, tt.next)
			if got == nil {
				t.Fatal("ChangedSlides() = nil, want an empty slice")
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("ChangedSlides() = %v, want %v", got, tt.want)
			}
		})
	}
}
```

Add `"fmt"` to the imports of `revision_test.go` if it is missing, and `"github.com/MiniCodeMonkey/tap/internal/transformer"` if it is missing.

Add to `internal/server/routes_test.go`:

```go
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
```

Add `"encoding/json"`, `"github.com/MiniCodeMonkey/tap/internal/config"` and `"github.com/MiniCodeMonkey/tap/internal/transformer"` to the imports of `routes_test.go` if they are missing.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/server -run 'TestChangedSlides|TestHandleAPIPresentationCarriesTheRevision' -v`
Expected: compile failure, `undefined: ChangedSlides` and `s.SetRevision undefined`.

- [ ] **Step 3: Write `ChangedSlides`**

Add to `internal/server/revision.go`:

```go
// ChangedSlides returns the 1-based numbers of the slides in next whose
// content differs from the slide at the same position in previous, by
// their content hashes (see transformer.SlideHash). A slide past the end of
// previous counts as changed, and so does every slide when previous is
// nil. A removed slide is not listed, because it no longer has a number.
// The result is never nil, so it encodes as [] in the "update" message.
func ChangedSlides(previous, next *transformer.TransformedPresentation) []int {
	changed := []int{}
	if next == nil {
		return changed
	}
	for index, slide := range next.Slides {
		if previous == nil || index >= len(previous.Slides) || previous.Slides[index].Hash != slide.Hash {
			changed = append(changed, index+1)
		}
	}
	return changed
}
```

- [ ] **Step 4: Store the revision on the server**

In `internal/server/server.go`, add a field to `Server`, after `customThemePath`:

```go
	// revision is the served deck's content hash (see ComputeRevision).
	// /api/presentation returns it, so a page with no WebSocket (a print
	// page) can still report it in its ready signal.
	revision string
```

After `GetPresentation`, add:

```go
// SetRevision sets the revision /api/presentation reports. tap dev calls
// it with the result of ComputeRevision on every load and reload.
// This method is thread-safe.
func (s *Server) SetRevision(revision string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.revision = revision
}

// Revision returns the revision set with SetRevision, or "" when none was.
// This method is thread-safe.
func (s *Server) Revision() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.revision
}
```

- [ ] **Step 5: Return it from `/api/presentation`**

In `internal/server/routes.go`, add `"github.com/MiniCodeMonkey/tap/internal/transformer"` to the imports. Above `handleAPIPresentation`, add:

```go
// presentationResponse is the /api/presentation body: the deck's own
// fields, and the revision the page reports in its ready signal.
type presentationResponse struct {
	*transformer.TransformedPresentation
	Revision string `json:"revision"`
}
```

In `handleAPIPresentation`, replace `if err := json.NewEncoder(w).Encode(pres); err != nil {` with:

```go
	response := presentationResponse{TransformedPresentation: pres, Revision: s.Revision()}
	if err := json.NewEncoder(w).Encode(response); err != nil {
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/server -short`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/server/revision.go internal/server/revision_test.go internal/server/server.go internal/server/routes.go internal/server/routes_test.go
git commit -m "feat(server): report the revision on /api/presentation and list changed slides"
```

---

### Task 3: The `update` message and the tap version on `connected`

**Files:**
- Modify: `internal/server/websocket.go`
- Test: `internal/server/websocket_test.go`

**Interfaces:**
- Produces:
  - `const MessageUpdate MessageType = "update"`
  - `type UpdateMessage struct { Type MessageType; Revision string; Slides []int }` with JSON names `type`, `revision`, `slides`
  - `func (h *WebSocketHub) BroadcastUpdate(revision string, slides []int) error`: sends `{"type":"update","revision":"...","slides":[...]}`, with `slides` always an array
  - `Message.Version string` (JSON `version`, omitted when empty) and `func (h *WebSocketHub) SetVersion(version string)`. Every `connected` message carries the version once it is set.

- [ ] **Step 1: Write the failing tests**

Add to `internal/server/websocket_test.go`:

```go
// dialHub connects to hub through a test server and reads the "connected"
// message, which it returns.
func dialHub(t *testing.T, hub *WebSocketHub) (*websocket.Conn, Message, context.Context) {
	t.Helper()
	testServer := httptest.NewServer(http.HandlerFunc(hub.HandleConnection))
	t.Cleanup(testServer.Close)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)

	conn, _, err := websocket.Dial(ctx, "ws"+strings.TrimPrefix(testServer.URL, "http")+"/", nil)
	if err != nil {
		t.Fatalf("websocket.Dial() error = %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(websocket.StatusNormalClosure, "") })

	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("reading the connected message: %v", err)
	}
	var connected Message
	if err := json.Unmarshal(data, &connected); err != nil {
		t.Fatalf("json.Unmarshal() error = %v", err)
	}
	return conn, connected, ctx
}

func TestWebSocketHubBroadcastUpdate(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	conn, _, ctx := dialHub(t, hub)
	time.Sleep(50 * time.Millisecond)

	if err := hub.BroadcastUpdate("r2", []int{2, 5}); err != nil {
		t.Fatalf("BroadcastUpdate() error = %v", err)
	}
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read() error = %v", err)
	}
	if string(data) != `{"type":"update","revision":"r2","slides":[2,5]}` {
		t.Errorf("update message = %s", data)
	}
}

func TestWebSocketHubBroadcastUpdateWithNoChangedSlides(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	conn, _, ctx := dialHub(t, hub)
	time.Sleep(50 * time.Millisecond)

	if err := hub.BroadcastUpdate("r3", nil); err != nil {
		t.Fatalf("BroadcastUpdate() error = %v", err)
	}
	_, data, err := conn.Read(ctx)
	if err != nil {
		t.Fatalf("conn.Read() error = %v", err)
	}
	if string(data) != `{"type":"update","revision":"r3","slides":[]}` {
		t.Errorf("update message = %s, want slides as an empty array", data)
	}
}

func TestWebSocketHubConnectedMessageCarriesTheVersion(t *testing.T) {
	hub := NewWebSocketHub()
	go hub.Run()
	defer hub.Stop()

	_, before, _ := dialHub(t, hub)
	if before.Version != "" {
		t.Errorf("Version = %q before SetVersion, want empty", before.Version)
	}

	hub.SetVersion("v2.1.0")
	_, after, _ := dialHub(t, hub)
	if after.Version != "v2.1.0" {
		t.Errorf("Version = %q, want %q", after.Version, "v2.1.0")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/server -run 'TestWebSocketHubBroadcastUpdate|TestWebSocketHubConnectedMessageCarriesTheVersion' -v`
Expected: compile failure, `hub.BroadcastUpdate undefined`.

- [ ] **Step 3: Add the message type, the struct and the version field**

In `internal/server/websocket.go`, in the `const` block of message types, after `MessageReload`, add:

```go
	// MessageUpdate tells clients the deck changed: a page fetches
	// /api/presentation again and replaces its deck data in place,
	// instead of reloading (see UpdateMessage).
	MessageUpdate MessageType = "update"
```

Change the comment on `MessageReload` to:

```go
	// MessageReload tells clients to reload the page. tap dev sends it only
	// when an update in place is not enough: a changed custom theme file,
	// or the r key.
```

In `Message`, after the `Mode` field, add:

```go
	// Version is the tap version, set only on a "connected" message once
	// SetVersion has been called. A page compares it across reconnects and
	// reloads when tap itself changed, since the page's own code is tap's.
	Version string `json:"version,omitempty"`
```

After the `Message` type, add:

```go
// UpdateMessage is the "update" message: the deck's new revision, and the
// 1-based numbers of the slides whose content changed (see ChangedSlides).
// Slides is always an array, never null.
type UpdateMessage struct {
	Type     MessageType `json:"type"`
	Revision string      `json:"revision"`
	Slides   []int       `json:"slides"`
}
```

In `WebSocketHub`, next to the `revision` field, add:

```go
	// version is the tap version sent on every "connected" message (see
	// SetVersion). Empty until set, and then omitted from the message.
	version string
```

- [ ] **Step 4: Send the version on `connected`**

After `SetPresentationMeta`, add:

```go
// SetVersion sets the tap version that every "connected" message carries
// from then on. Safe to call at any time.
func (h *WebSocketHub) SetVersion(version string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.version = version
}
```

In `Run`'s register case, next to `revision := h.revision`, add `version := h.version`, and change the `connectedMsg` line to:

```go
			connectedMsg, _ := json.Marshal(Message{Type: MessageConnected, Revision: revision, Mode: mode, Version: version})
```

- [ ] **Step 5: Queue raw messages and add `BroadcastUpdate`**

In `Broadcast`, replace the final `select` block and `return nil` with:

```go
	h.queueBroadcast(data)
	return nil
}

// queueBroadcast hands an encoded message to Run, which sends it to every
// client. A full broadcast channel drops the message rather than block
// the caller.
func (h *WebSocketHub) queueBroadcast(data []byte) {
	select {
	case h.broadcast <- data:
	default:
		// Broadcast channel is full, skip
	}
```

(The closing brace of `Broadcast` now closes `queueBroadcast`. Check that the file still compiles with `go build ./internal/server`.)

After `BroadcastReload`, add:

```go
// BroadcastUpdate sends an "update" message with the deck's new revision
// and the 1-based numbers of the slides that changed. A nil slides list is
// sent as [].
func (h *WebSocketHub) BroadcastUpdate(revision string, slides []int) error {
	if slides == nil {
		slides = []int{}
	}
	data, err := json.Marshal(UpdateMessage{Type: MessageUpdate, Revision: revision, Slides: slides})
	if err != nil {
		return err
	}
	h.queueBroadcast(data)
	return nil
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/server -short -race`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/server/websocket.go internal/server/websocket_test.go
git commit -m "feat(server): add the update message and send the tap version on connect"
```

---

### Task 4: `tap dev` sends `update` instead of `reload`

**Files:**
- Create: `internal/cli/publish.go`
- Create: `internal/cli/publish_test.go`
- Create: `internal/cli/dev_update_test.go`
- Modify: `internal/cli/dev.go` (`runDevServer`: the initial revision, `buildServer`, the three reload paths)

**Interfaces:**
- Consumes: `server.ChangedSlides`, `(*server.Server).SetRevision` (Task 2); `(*server.WebSocketHub).BroadcastUpdate`, `SetVersion` (Task 3); `server.ComputeRevision` (existing)
- Produces:
  - `type deckServer interface { SetPresentation(*transformer.TransformedPresentation); SetComponentBundles(map[string]server.ComponentBundleFile); SetRevision(string) }`
  - `type deckHub interface { SetPresentationMeta(int, string); BroadcastReload() error; BroadcastUpdate(string, []int) error }`
  - `func newDeckPublisher(target deckServer, hub deckHub, presentation *transformer.TransformedPresentation, revision, customThemePath string) *deckPublisher`
  - `func (p *deckPublisher) publish(presentation *transformer.TransformedPresentation, bundles map[string]server.ComponentBundleFile, customThemePath string, forceReload bool)`. P6's `PUT /api/app/source` calls this too.
  - `func customThemeFingerprint(path string) string`

- [ ] **Step 1: Write the failing unit tests**

`internal/cli/publish_test.go`:

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

type fakeDeckServer struct {
	presentation *transformer.TransformedPresentation
	revision     string
}

func (f *fakeDeckServer) SetPresentation(presentation *transformer.TransformedPresentation) {
	f.presentation = presentation
}
func (f *fakeDeckServer) SetComponentBundles(map[string]server.ComponentBundleFile) {}
func (f *fakeDeckServer) SetRevision(revision string)                           { f.revision = revision }

type fakeDeckHub struct {
	sent       []string
	slideCount int
	revision   string
}

func (f *fakeDeckHub) SetPresentationMeta(slideCount int, revision string) {
	f.slideCount, f.revision = slideCount, revision
}
func (f *fakeDeckHub) BroadcastReload() error {
	f.sent = append(f.sent, "reload")
	return nil
}
func (f *fakeDeckHub) BroadcastUpdate(revision string, slides []int) error {
	f.sent = append(f.sent, fmt.Sprintf("update %v", slides))
	return nil
}

// deckOf builds a transformed deck with one slide per HTML string, each
// with its content hash.
func deckOf(htmls ...string) *transformer.TransformedPresentation {
	presentation := &transformer.TransformedPresentation{}
	for index, html := range htmls {
		slide := transformer.TransformedSlide{Index: index, Layout: "default", HTML: html}
		slide.Hash = transformer.SlideHash(slide)
		presentation.Slides = append(presentation.Slides, slide)
	}
	return presentation
}

func newTestPublisher(t *testing.T, first *transformer.TransformedPresentation, customThemePath string) (*deckPublisher, *fakeDeckServer, *fakeDeckHub) {
	t.Helper()
	target := &fakeDeckServer{}
	hub := &fakeDeckHub{}
	revision := server.ComputeRevision(first, nil)
	return newDeckPublisher(target, hub, first, revision, customThemePath), target, hub
}

func TestPublishSendsAnUpdateWithTheChangedSlides(t *testing.T) {
	publisher, target, hub := newTestPublisher(t, deckOf("<h1>One</h1>", "<h1>Two</h1>"), "")
	next := deckOf("<h1>One</h1>", "<h1>Two, edited</h1>")

	publisher.publish(next, nil, "", false)

	if fmt.Sprint(hub.sent) != "[update [2]]" {
		t.Errorf("sent %v, want one update for slide 2", hub.sent)
	}
	if target.presentation != next {
		t.Error("the server does not serve the new deck")
	}
	wantRevision := server.ComputeRevision(next, nil)
	if target.revision != wantRevision || hub.revision != wantRevision || hub.slideCount != 2 {
		t.Errorf("server revision %q, hub revision %q and count %d, want %q and 2", target.revision, hub.revision, hub.slideCount, wantRevision)
	}
}

func TestPublishSendsNothingWhenTheRevisionIsTheSame(t *testing.T) {
	publisher, _, hub := newTestPublisher(t, deckOf("<h1>One</h1>"), "")

	publisher.publish(deckOf("<h1>One</h1>"), nil, "", false)

	if len(hub.sent) != 0 {
		t.Errorf("sent %v, want nothing for an unchanged deck", hub.sent)
	}
}

func TestPublishListsAnAddedSlide(t *testing.T) {
	publisher, _, hub := newTestPublisher(t, deckOf("<h1>One</h1>", "<h1>Two</h1>"), "")

	publisher.publish(deckOf("<h1>One</h1>", "<h1>Two</h1>", "<h1>Three</h1>"), nil, "", false)

	if fmt.Sprint(hub.sent) != "[update [3]]" {
		t.Errorf("sent %v, want an update for slide 3", hub.sent)
	}
}

func TestPublishReloadsWhenTheCustomThemeFileChanges(t *testing.T) {
	themePath := filepath.Join(t.TempDir(), "theme.css")
	if err := os.WriteFile(themePath, []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	publisher, _, hub := newTestPublisher(t, deckOf("<h1>One</h1>"), themePath)

	publisher.publish(deckOf("<h1>One</h1>"), nil, themePath, false)
	if len(hub.sent) != 0 {
		t.Fatalf("sent %v for an unchanged theme file, want nothing", hub.sent)
	}

	if err := os.WriteFile(themePath, []byte("body { color: blue; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	publisher.publish(deckOf("<h1>One</h1>"), nil, themePath, false)
	if fmt.Sprint(hub.sent) != "[reload]" {
		t.Errorf("sent %v, want a reload for a changed theme file", hub.sent)
	}
}

func TestPublishReloadsWhenTheCustomThemeIsRemoved(t *testing.T) {
	themePath := filepath.Join(t.TempDir(), "theme.css")
	if err := os.WriteFile(themePath, []byte("body { color: red; }"), 0o644); err != nil {
		t.Fatal(err)
	}
	publisher, _, hub := newTestPublisher(t, deckOf("<h1>One</h1>"), themePath)

	publisher.publish(deckOf("<h1>One</h1>"), nil, "", false)

	if fmt.Sprint(hub.sent) != "[reload]" {
		t.Errorf("sent %v, want a reload when the custom theme goes away", hub.sent)
	}
}

func TestPublishWithForceReloadAlwaysReloads(t *testing.T) {
	publisher, _, hub := newTestPublisher(t, deckOf("<h1>One</h1>"), "")

	publisher.publish(deckOf("<h1>One</h1>"), nil, "", true)

	if fmt.Sprint(hub.sent) != "[reload]" {
		t.Errorf("sent %v, want a reload", hub.sent)
	}
}

func TestCustomThemeFingerprint(t *testing.T) {
	if customThemeFingerprint("") != "" {
		t.Error("no custom theme should give an empty fingerprint")
	}
	themePath := filepath.Join(t.TempDir(), "theme.css")
	if err := os.WriteFile(themePath, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	first := customThemeFingerprint(themePath)
	if err := os.WriteFile(themePath, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	if customThemeFingerprint(themePath) == first {
		t.Error("the fingerprint did not change with the file's contents")
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestPublish|TestCustomThemeFingerprint' -short`
Expected: compile failure, `undefined: newDeckPublisher`.

- [ ] **Step 3: Write `internal/cli/publish.go`**

```go
package cli

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"sync"

	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

// deckServer is the part of *server.Server that a deck change updates.
type deckServer interface {
	SetPresentation(presentation *transformer.TransformedPresentation)
	SetComponentBundles(files map[string]server.ComponentBundleFile)
	SetRevision(revision string)
}

// deckHub is the part of *server.WebSocketHub that a deck change notifies.
type deckHub interface {
	SetPresentationMeta(slideCount int, revision string)
	BroadcastReload() error
	BroadcastUpdate(revision string, slides []int) error
}

// deckPublisher hands a reloaded deck to the server and tells every open
// page about it. An ordinary edit sends "update", and each page fetches
// the deck again and re-renders only the slides that changed. A page
// reloads only when an update is not enough: the custom theme file
// changed (a page loads it once), or the caller forces it (the r key).
// A deck whose revision did not change sends nothing.
type deckPublisher struct {
	target           deckServer
	hub              deckHub
	presentation     *transformer.TransformedPresentation
	revision         string
	themeFingerprint string
	mu               sync.Mutex
}

// newDeckPublisher starts from the deck the server already serves, with
// its revision and custom theme file.
func newDeckPublisher(target deckServer, hub deckHub, presentation *transformer.TransformedPresentation, revision, customThemePath string) *deckPublisher {
	return &deckPublisher{
		target:           target,
		hub:              hub,
		presentation:     presentation,
		revision:         revision,
		themeFingerprint: customThemeFingerprint(customThemePath),
	}
}

// publish serves presentation and its component bundles, and sends every
// open page an "update", a "reload", or nothing (see deckPublisher).
func (p *deckPublisher) publish(presentation *transformer.TransformedPresentation, bundles map[string]server.ComponentBundleFile, customThemePath string, forceReload bool) {
	revision := server.ComputeRevision(presentation, bundles)
	fingerprint := customThemeFingerprint(customThemePath)

	p.mu.Lock()
	defer p.mu.Unlock()

	p.target.SetComponentBundles(bundles)
	p.target.SetPresentation(presentation)
	p.target.SetRevision(revision)
	p.hub.SetPresentationMeta(len(presentation.Slides), revision)

	switch {
	case forceReload || fingerprint != p.themeFingerprint:
		_ = p.hub.BroadcastReload()
	case revision != p.revision:
		_ = p.hub.BroadcastUpdate(revision, server.ChangedSlides(p.presentation, presentation))
	}

	p.presentation = presentation
	p.revision = revision
	p.themeFingerprint = fingerprint
}

// customThemeFingerprint identifies the custom theme CSS a page loaded: its
// path and a hash of its contents. It is "" when there is no custom theme.
// A file that cannot be read gives only its path, so reading it again
// later counts as a change.
func customThemeFingerprint(path string) string {
	if path == "" {
		return ""
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return path
	}
	sum := sha256.Sum256(content)
	return path + "\x00" + hex.EncodeToString(sum[:])
}
```

- [ ] **Step 4: Run the unit tests to verify they pass**

Run: `go test ./internal/cli -run 'TestPublish|TestCustomThemeFingerprint' -short -v`
Expected: PASS.

- [ ] **Step 5: Write the failing process test**

`internal/cli/dev_update_test.go`:

```go
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"testing"
	"time"

	"github.com/coder/websocket"
)

// TestDevSendsAnUpdateWhenASlideChanges starts a real tap dev, edits the
// second slide on disk, and expects an "update" message that names slide
// 2, not a "reload".
func TestDevSendsAnUpdateWhenASlideChanges(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}
	binary := buildTapBinaryForTest(t)

	deckPath := filepath.Join(t.TempDir(), "talk.md")
	if err := os.WriteFile(deckPath, []byte("# One\n\n---\n\n# Two\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	port := freePort(t)
	command := exec.Command(binary, "dev", deckPath, "--headless", "--port", fmt.Sprint(port))
	var output bytes.Buffer
	command.Stdout = &output
	command.Stderr = &output
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_ = command.Process.Signal(syscall.SIGINT)
		_ = command.Wait()
	})

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var conn *websocket.Conn
	for {
		var err error
		conn, _, err = websocket.Dial(ctx, fmt.Sprintf("ws://127.0.0.1:%d/ws", port), nil)
		if err == nil {
			break
		}
		if ctx.Err() != nil {
			t.Fatalf("tap dev never accepted a WebSocket: %v\n%s", err, output.String())
		}
		time.Sleep(100 * time.Millisecond)
	}
	defer conn.Close(websocket.StatusNormalClosure, "")

	readMessage := func() map[string]any {
		t.Helper()
		_, data, err := conn.Read(ctx)
		if err != nil {
			t.Fatalf("conn.Read() error = %v\n%s", err, output.String())
		}
		var message map[string]any
		if err := json.Unmarshal(data, &message); err != nil {
			t.Fatalf("json.Unmarshal(%s) error = %v", data, err)
		}
		return message
	}

	if connected := readMessage(); connected["type"] != "connected" {
		t.Fatalf("first message = %v, want connected", connected)
	}

	if err := os.WriteFile(deckPath, []byte("# One\n\n---\n\n# Two, edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	for {
		message := readMessage()
		switch message["type"] {
		case "reload":
			t.Fatal("tap dev sent reload for a markdown edit, want update")
		case "update":
			if fmt.Sprint(message["slides"]) != "[2]" {
				t.Errorf("slides = %v, want [2]", message["slides"])
			}
			if revision, _ := message["revision"].(string); revision == "" {
				t.Error("the update carries no revision")
			}
			return
		}
	}
}
```

- [ ] **Step 6: Run the process test to verify it fails**

Run: `go test ./internal/cli -run TestDevSendsAnUpdateWhenASlideChanges -v`
Expected: FAIL with "tap dev sent reload for a markdown edit, want update".

- [ ] **Step 7: Route every reload in `runDevServer` through the publisher**

In `internal/cli/dev.go`:

1. Replace the line `hub.SetPresentationMeta(len(pres.Slides), server.ComputeRevision(pres, componentBundleFiles(resolvedComponents)))` (right after `defer hub.Stop()`) with:

```go
	initialRevision := server.ComputeRevision(pres, componentBundleFiles(resolvedComponents))
	hub.SetPresentationMeta(len(pres.Slides), initialRevision)
	hub.SetVersion(displayVersion())
```

2. In `buildServer`, after `candidate.SetPresentation(pres)`, add:

```go
		candidate.SetRevision(initialRevision)
```

3. After `port = srv.Port()`, add:

```go
	// Every reload below goes through publisher, which decides whether
	// open pages update in place or reload.
	publisher := newDeckPublisher(srv, hub, pres, initialRevision, customThemePath)
```

4. In the first `watcher.SetOnChange` handler (before the headless and TUI branches), replace these four lines:

```go
		srv.SetComponentBundles(componentBundleFiles(newResolvedComponents))
		srv.SetPresentation(newPres)
		hub.SetPresentationMeta(len(newPres.Slides), server.ComputeRevision(newPres, componentBundleFiles(newResolvedComponents)))
		_ = hub.BroadcastReload()
```

with:

```go
		publisher.publish(newPres, componentBundleFiles(newResolvedComponents), customThemePath, false)
```

Leave the `srv.SetRegistry(buildDriverRegistry(newCfg, baseDir))` line that P1 put above `srv.SetPresentation` where it is.

5. In the headless `watcher.SetOnChange` handler, replace the custom theme block:

```go
			newCustomThemePath, err := newCfg.ResolveCustomThemePath(baseDir)
			if err != nil {
				Warning("Custom theme not loaded on reload: %v\n", err)
				srv.SetCustomThemePath("")
			} else {
				srv.SetCustomThemePath(newCustomThemePath)
			}
```

with:

```go
			newCustomThemePath, err := newCfg.ResolveCustomThemePath(baseDir)
			if err != nil {
				Warning("Custom theme not loaded on reload: %v\n", err)
				newCustomThemePath = ""
			}
			srv.SetCustomThemePath(newCustomThemePath)
```

and replace the same four lines as in item 4 with:

```go
			publisher.publish(newPres, componentBundleFiles(newResolvedComponents), newCustomThemePath, false)
```

6. In `reloadInTUI`, make the same custom theme change as in item 5 (keep its comment "Log warning but continue - use empty path to disable custom theme" above the `Warning` call), and replace the four lines with:

```go
			// r forces a full reload, so a person can always get a fresh
			// page. A file change updates open pages in place.
			publisher.publish(newPres, componentBundleFiles(newResolvedComponents), newCustomThemePath, manual)
```

7. Check that nothing else sends reload or sets the meta:

Run: `grep -n "BroadcastReload\|SetPresentationMeta\|srv.SetPresentation(newPres)" internal/cli/dev.go`
Expected: one line, the `hub.SetPresentationMeta(len(pres.Slides), initialRevision)` from item 1.

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestDevSendsAnUpdateWhenASlideChanges|TestPublish|TestDevRunsALiveShellBlock' -v`
Expected: PASS.

Run: `go test ./... -short`
Expected: PASS.

- [ ] **Step 9: Commit**

```bash
git add internal/cli/publish.go internal/cli/publish_test.go internal/cli/dev_update_test.go internal/cli/dev.go
git commit -m "feat(dev): update open pages in place when the deck changes"
```

---

### Task 5: Replace the deck in place in the store

**Files:**
- Modify: `frontend/src/lib/types.ts` (`Slide`, `Presentation`, `WebSocketMessageType`, `WebSocketMessage`)
- Modify: `frontend/src/lib/stores/presentation.ts`
- Create: `frontend/src/lib/stores/presentation.update.test.ts`

**Interfaces:**
- Consumes: `hash` on each slide and `revision` on `/api/presentation` (Tasks 1 and 2)
- Produces:
  - `Slide.hash?: string`, `Presentation.revision?: string`
  - `WebSocketMessageType` includes `'update'`; `WebSocketMessage.slides?: number[]`; `WebSocketMessage.version?: string`
  - `export function updatePresentationInPlace(data: Presentation): void`
  - `export function slideKey(slide: Slide): string`

- [ ] **Step 1: Write the failing tests**

`frontend/src/lib/stores/presentation.update.test.ts`:

```ts
/**
 * Replacing the deck in place, after an "update" message: the position is
 * kept and clamped, and unchanged slides keep their objects.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import {
	loadPresentation,
	resetPresentation,
	slideKey,
	updatePresentationInPlace,
	usePresentationStore
} from './presentation';
import type { Presentation, Slide } from '$lib/types';

function makeSlide(index: number, hash: string | undefined, overrides: Partial<Slide> = {}): Slide {
	return {
		index,
		layout: 'default',
		html: `<p>${hash ?? 'no hash'}</p>`,
		slots: {},
		slotOrder: [],
		fragmentCount: 0,
		steps: 0,
		hash,
		...overrides
	};
}

function makeDeck(revision: string, slides: Slide[]): Presentation {
	return { config: { title: 'Deck' }, slides, revision };
}

const replaceState = vi.fn();

beforeEach(() => {
	vi.stubGlobal('window', {
		location: { hash: '', search: '', pathname: '/' },
		history: { replaceState }
	});
	replaceState.mockClear();
	resetPresentation();
});

afterEach(() => {
	vi.unstubAllGlobals();
});

describe('updatePresentationInPlace', () => {
	it('keeps the slide, fragment and step', () => {
		loadPresentation(
			makeDeck('r1', [makeSlide(0, 'a'), makeSlide(1, 'b', { steps: 3, fragmentCount: 2 })])
		);
		usePresentationStore.setState({ currentSlideIndex: 1, currentStep: 2, currentFragmentIndex: 1 });

		updatePresentationInPlace(
			makeDeck('r2', [makeSlide(0, 'a2'), makeSlide(1, 'b', { steps: 3, fragmentCount: 2 })])
		);

		const state = usePresentationStore.getState();
		expect(state.presentation?.revision).toBe('r2');
		expect(state.currentSlideIndex).toBe(1);
		expect(state.currentStep).toBe(2);
		expect(state.currentFragmentIndex).toBe(1);
	});

	it('clamps the step when the slide loses steps', () => {
		loadPresentation(makeDeck('r1', [makeSlide(0, 'a', { steps: 4 })]));
		usePresentationStore.setState({ currentStep: 4 });

		updatePresentationInPlace(makeDeck('r2', [makeSlide(0, 'a2', { steps: 1 })]));

		expect(usePresentationStore.getState().currentStep).toBe(1);
	});

	it('clamps the fragment when the slide loses fragments', () => {
		loadPresentation(makeDeck('r1', [makeSlide(0, 'a', { fragmentCount: 3 })]));
		usePresentationStore.setState({ currentFragmentIndex: 2 });

		updatePresentationInPlace(makeDeck('r2', [makeSlide(0, 'a2', { fragmentCount: 0 })]));

		expect(usePresentationStore.getState().currentFragmentIndex).toBe(-1);
	});

	it('moves to the last slide when the current one was removed, and updates the URL hash', () => {
		loadPresentation(makeDeck('r1', [makeSlide(0, 'a'), makeSlide(1, 'b'), makeSlide(2, 'c')]));
		usePresentationStore.setState({ currentSlideIndex: 2 });
		replaceState.mockClear();

		updatePresentationInPlace(makeDeck('r2', [makeSlide(0, 'a'), makeSlide(1, 'b')]));

		expect(usePresentationStore.getState().currentSlideIndex).toBe(1);
		expect(replaceState).toHaveBeenCalledWith(null, '', '#2');
	});

	it('does not touch the URL hash when the slide stays the same', () => {
		loadPresentation(makeDeck('r1', [makeSlide(0, 'a'), makeSlide(1, 'b')]));
		usePresentationStore.setState({ currentSlideIndex: 1 });
		replaceState.mockClear();

		updatePresentationInPlace(makeDeck('r2', [makeSlide(0, 'a2'), makeSlide(1, 'b')]));

		expect(replaceState).not.toHaveBeenCalled();
	});

	it('keeps the object of every slide whose hash did not change', () => {
		const first = makeDeck('r1', [makeSlide(0, 'a'), makeSlide(1, 'b'), makeSlide(2, 'c')]);
		loadPresentation(first);

		const next = makeDeck('r2', [makeSlide(0, 'a'), makeSlide(1, 'b2'), makeSlide(2, 'c')]);
		updatePresentationInPlace(next);

		const slides = usePresentationStore.getState().presentation?.slides ?? [];
		expect(slides[0]).toBe(first.slides[0]);
		expect(slides[1]).toBe(next.slides[1]);
		expect(slides[2]).toBe(first.slides[2]);
	});

	it('replaces a slide that has no hash', () => {
		const first = makeDeck('r1', [makeSlide(0, undefined)]);
		loadPresentation(first);

		const next = makeDeck('r2', [makeSlide(0, undefined)]);
		updatePresentationInPlace(next);

		expect(usePresentationStore.getState().presentation?.slides[0]).toBe(next.slides[0]);
	});

	it('uses the new slide when a slide moved to another position', () => {
		const first = makeDeck('r1', [makeSlide(0, 'a'), makeSlide(1, 'b')]);
		loadPresentation(first);

		const next = makeDeck('r2', [makeSlide(0, 'b'), makeSlide(1, 'a')]);
		updatePresentationInPlace(next);

		const slides = usePresentationStore.getState().presentation?.slides ?? [];
		expect(slides[0]).toBe(next.slides[0]);
		expect(slides[1]).toBe(next.slides[1]);
	});

	it('keeps the config object when the config did not change', () => {
		const first = makeDeck('r1', [makeSlide(0, 'a')]);
		loadPresentation(first);

		updatePresentationInPlace(makeDeck('r2', [makeSlide(0, 'a2')]));

		expect(usePresentationStore.getState().presentation?.config).toBe(first.config);
	});
});

describe('slideKey', () => {
	it('combines the position and the hash', () => {
		expect(slideKey(makeSlide(4, 'abc'))).toBe('4:abc');
	});

	it('falls back to the position for a slide with no hash', () => {
		expect(slideKey(makeSlide(4, undefined))).toBe('4');
	});
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd frontend && npx vitest run src/lib/stores/presentation.update.test.ts`
Expected: FAIL, `updatePresentationInPlace` is not exported (and `hash`/`revision` are type errors in the editor).

- [ ] **Step 3: Extend the types**

In `frontend/src/lib/types.ts`, add to `Slide`, after `components`:

```ts
	/**
	 * Short hash of the slide's content, without its position (see
	 * SlideHash in internal/transformer). The same hash at the same
	 * position means the same slide.
	 */
	hash?: string;
```

Add to `Presentation`:

```ts
	/**
	 * The deck's revision (see ComputeRevision in internal/server), from
	 * /api/presentation. The ready signal reports it. Absent in a static
	 * build.
	 */
	revision?: string;
```

Change `WebSocketMessageType` to:

```ts
export type WebSocketMessageType = 'connected' | 'reload' | 'update' | 'slide' | 'theme' | 'recording';
```

Add to `WebSocketMessage`, after `revision`:

```ts
	/**
	 * The 1-based numbers of the slides whose content changed, on an
	 * "update" message. The page fetches the whole deck either way; this
	 * says which slides will re-render.
	 */
	slides?: number[];
	/**
	 * The tap version, on a "connected" message. A reconnect to another
	 * version reloads the page, since the page's code comes from tap.
	 */
	version?: string;
```

Change the `revision` field's comment to say it is also on an "update" message:

```ts
	/**
	 * Short content hash of the deck currently served, on a "connected"
	 * message (see internal/server/websocket.go's register case) and on an
	 * "update" message. Absent when the hub has never had a presentation set.
	 */
```

- [ ] **Step 4: Add `slideKey` and `updatePresentationInPlace`**

In `frontend/src/lib/stores/presentation.ts`, after `loadPresentation`, add:

```ts
/**
 * A React key for a slide in a list of slides: its position and content
 * hash, so a list re-renders only the slides whose content changed. A
 * slide with no hash falls back to its position.
 */
export function slideKey(slide: Slide): string {
	return slide.hash ? `${slide.index}:${slide.hash}` : String(slide.index);
}

/**
 * Replace the deck with a newer copy without reloading the page, after an
 * "update" message (see stores/websocket.ts). Unlike loadPresentation, this
 * keeps the current slide, fragment and step, clamped to the new deck's
 * counts, and keeps the scroll reveal when the slide stays the same.
 *
 * A slide keeps its old object when the slide at the same position has the
 * same content hash, and the config keeps its old object when it did not
 * change, so a memoized Slide (see components/Slide.tsx) skips re-rendering
 * everything the edit did not touch.
 */
export function updatePresentationInPlace(data: Presentation): void {
	const current = usePresentationStore.getState();
	const previous = current.presentation;
	const previousSlides = previous?.slides ?? [];
	const slides = data.slides.map((slide, index) => {
		const previousSlide = previousSlides[index];
		const unchanged = previousSlide !== undefined && slide.hash !== undefined && previousSlide.hash === slide.hash;
		return unchanged ? previousSlide : slide;
	});
	const config =
		previous && JSON.stringify(previous.config) === JSON.stringify(data.config) ? previous.config : data.config;
	const presentation: Presentation = { ...data, config, slides };

	const total = slides.length;
	const slideIndex = total > 0 ? clamp(current.currentSlideIndex, 0, total - 1) : 0;
	const slide = slides[slideIndex] ?? null;
	const sameSlide = slideIndex === current.currentSlideIndex;

	usePresentationStore.setState({
		presentation,
		currentSlideIndex: slideIndex,
		currentStep: clamp(current.currentStep, 0, Math.max(slide?.steps ?? 0, 0)),
		currentFragmentIndex: clamp(current.currentFragmentIndex, -1, Math.max((slide?.fragmentCount ?? 0) - 1, -1)),
		scrollRevealed: sameSlide && slide?.scroll === true ? current.scrollRevealed : false
	});

	if (typeof window !== 'undefined') {
		(window as unknown as { presentation: Presentation }).presentation = presentation;
	}
	if (!sameSlide) {
		updateURLHash(slideIndex);
	}
}
```

`clamp` and `updateURLHash` already exist in this file, above `loadPresentation`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd frontend && npx vitest run src/lib/stores/presentation.update.test.ts`
Expected: PASS.

Run: `cd frontend && npm run check && npm test -- --run`
Expected: no type errors, and every test passes.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/types.ts frontend/src/lib/stores/presentation.ts frontend/src/lib/stores/presentation.update.test.ts
git commit -m "feat(frontend): replace the deck in place, keeping the position and unchanged slides"
```

---

### Task 6: Handle `update` in the WebSocket client

**Files:**
- Modify: `frontend/src/lib/stores/websocket.ts`
- Test: `frontend/src/lib/stores/websocket.test.ts`

**Interfaces:**
- Consumes: `updatePresentationInPlace` (Task 5), `fetchPresentation` (existing), the `update` message and `version` on `connected` (Task 3)
- Produces: an `update` message fetches `/api/presentation` and applies it in place. A failed fetch reloads the page. Only the newest of overlapping updates applies. A reconnect reloads when the revision or the tap version differs from the one the page last applied.

- [ ] **Step 1: Write the failing tests**

In `frontend/src/lib/stores/websocket.test.ts`, add these tests inside `describe('WebSocketClient', ...)`, after the `'should handle "reload" message by reloading the page'` test:

```ts
		describe('"update" messages', () => {
			function deck(revision: string, secondSlideSteps: number): Presentation {
				return {
					config: { title: 'Deck' },
					revision,
					slides: [
						{ index: 0, layout: 'default', html: '<p>1</p>', slots: {}, slotOrder: [], fragmentCount: 0, steps: 0, hash: `a-${revision}` },
						{ index: 1, layout: 'default', html: '<p>2</p>', slots: {}, slotOrder: [], fragmentCount: 0, steps: secondSlideSteps, hash: 'b' }
					]
				};
			}

			function respondWith(...presentations: Presentation[]): ReturnType<typeof vi.fn> {
				const fetchMock = vi.fn();
				for (const presentation of presentations) {
					fetchMock.mockResolvedValueOnce({
						ok: true,
						statusText: 'OK',
						json: () => Promise.resolve(presentation)
					} as Response);
				}
				vi.stubGlobal('fetch', fetchMock);
				return fetchMock;
			}

			function stubWindow(reloadSpy: ReturnType<typeof vi.fn>): void {
				vi.stubGlobal('window', {
					location: { protocol: 'http:', host: 'localhost:3000', reload: reloadSpy, hash: '#2', search: '', pathname: '/' },
					history: { replaceState: vi.fn() }
				});
			}

			it('applies the new deck in place, keeping the slide and step, with no reload', async () => {
				const reloadSpy = vi.fn();
				stubWindow(reloadSpy);
				loadPresentation(deck('r1', 3));
				usePresentationStore.setState({ currentSlideIndex: 1, currentStep: 2 });
				const fetchMock = respondWith(deck('r2', 3));

				client.connect();
				mockWs?.simulateOpen();
				mockWs?.simulateMessage({ type: 'update', revision: 'r2', slides: [1] });

				await vi.waitFor(() => {
					expect(usePresentationStore.getState().presentation?.revision).toBe('r2');
				});
				expect(fetchMock).toHaveBeenCalledWith('/api/presentation');
				expect(usePresentationStore.getState().currentSlideIndex).toBe(1);
				expect(usePresentationStore.getState().currentStep).toBe(2);
				expect(reloadSpy).not.toHaveBeenCalled();
			});

			it('clamps the step when the current slide lost steps', async () => {
				stubWindow(vi.fn());
				loadPresentation(deck('r1', 3));
				usePresentationStore.setState({ currentSlideIndex: 1, currentStep: 3 });
				respondWith(deck('r2', 1));

				client.connect();
				mockWs?.simulateOpen();
				mockWs?.simulateMessage({ type: 'update', revision: 'r2', slides: [2] });

				await vi.waitFor(() => {
					expect(usePresentationStore.getState().currentStep).toBe(1);
				});
			});

			it('reloads the page when the new deck cannot be fetched', async () => {
				const reloadSpy = vi.fn();
				stubWindow(reloadSpy);
				loadPresentation(deck('r1', 0));
				vi.stubGlobal('fetch', vi.fn().mockResolvedValue({ ok: false, statusText: 'Not Found' } as Response));

				client.connect();
				mockWs?.simulateOpen();
				mockWs?.simulateMessage({ type: 'update', revision: 'r2', slides: [] });

				await vi.waitFor(() => {
					expect(reloadSpy).toHaveBeenCalledTimes(1);
				});
			});

			it('applies only the newest of two overlapping updates', async () => {
				stubWindow(vi.fn());
				loadPresentation(deck('r1', 0));
				let resolveFirst!: (response: Response) => void;
				const fetchMock = vi
					.fn()
					.mockReturnValueOnce(new Promise<Response>((resolve) => { resolveFirst = resolve; }))
					.mockResolvedValueOnce({ ok: true, statusText: 'OK', json: () => Promise.resolve(deck('r3', 0)) } as Response);
				vi.stubGlobal('fetch', fetchMock);

				client.connect();
				mockWs?.simulateOpen();
				mockWs?.simulateMessage({ type: 'update', revision: 'r2', slides: [1] });
				mockWs?.simulateMessage({ type: 'update', revision: 'r3', slides: [1] });

				await vi.waitFor(() => {
					expect(usePresentationStore.getState().presentation?.revision).toBe('r3');
				});
				resolveFirst({ ok: true, statusText: 'OK', json: () => Promise.resolve(deck('r2', 0)) } as Response);
				await new Promise((resolve) => setTimeout(resolve, 0));

				expect(usePresentationStore.getState().presentation?.revision).toBe('r3');
			});

			it('does not reload on a reconnect to the revision an update already applied', async () => {
				const reloadSpy = vi.fn();
				stubWindow(reloadSpy);
				loadPresentation(deck('r1', 0));
				respondWith(deck('r2', 0));

				client.connect();
				mockWs?.simulateOpen();
				mockWs?.simulateMessage({ type: 'connected', revision: 'r1' });
				mockWs?.simulateMessage({ type: 'update', revision: 'r2', slides: [1] });
				await vi.waitFor(() => {
					expect(usePresentationStore.getState().presentation?.revision).toBe('r2');
				});

				if (mockWs) mockWs.readyState = MockWebSocket.CLOSED;
				client.connect();
				mockWs?.simulateOpen();
				mockWs?.simulateMessage({ type: 'connected', revision: 'r2' });

				expect(reloadSpy).not.toHaveBeenCalled();
			});
		});

		it('reloads on a reconnect to a different tap version', () => {
			const reloadSpy = vi.fn();
			vi.stubGlobal('window', {
				location: { protocol: 'http:', host: 'localhost:3000', reload: reloadSpy }
			});

			client.connect();
			mockWs?.simulateOpen();
			mockWs?.simulateMessage({ type: 'connected', revision: 'abc123', version: 'v2.0.0' });

			if (mockWs) mockWs.readyState = MockWebSocket.CLOSED;
			client.connect();
			mockWs?.simulateOpen();
			mockWs?.simulateMessage({ type: 'connected', revision: 'abc123', version: 'v2.1.0' });

			expect(reloadSpy).toHaveBeenCalledTimes(1);
		});
```

Run `cd frontend && npx prettier --write src/lib/stores/websocket.test.ts` after pasting, so the long object literals follow the repo's formatting.

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd frontend && npx vitest run src/lib/stores/websocket.test.ts`
Expected: FAIL. The update tests time out waiting for revision `r2`, and the version test sees no reload.

- [ ] **Step 3: Handle the message**

In `frontend/src/lib/stores/websocket.ts`:

1. Import the new pieces:

```ts
import {
	applyRemoteState,
	usePresentationStore,
	setThemeOverride,
	getHashSlideIndexAtLoad,
	updatePresentationInPlace
} from '$lib/stores/presentation';
import { fetchPresentation } from '$lib/utils/fetchPresentation';
```

2. Rename the private field `firstRevision` to `knownRevision` (both the declaration and every use), and replace its doc comment with:

```ts
	/**
	 * The deck revision this page shows: the one from this page load's
	 * first "connected" message, then the one from each "update" it
	 * applied. undefined until the first "connected" message, or when that
	 * message carried no revision (a hub that has never had a presentation
	 * set).
	 */
	private knownRevision: string | undefined = undefined;

	/**
	 * The tap version from this page load's first "connected" message. A
	 * reconnect to a hub with another version reloads the page, because the
	 * page's own code comes from tap.
	 */
	private firstVersion: string | undefined = undefined;

	/**
	 * Counts "update" messages, so that only the newest one's fetch is
	 * applied when two overlap.
	 */
	private updateSequence: number = 0;
```

Update the doc comment of `hasSeenFirstConnected` to say it is set "together with knownRevision and firstVersion".

3. In `dispatchMessage`, add a case after `'reload'`:

```ts
			case 'update':
				// The deck changed: fetch it and re-render in place
				this.handleUpdate();
				break;
```

and change the `'connected'` case to pass the version:

```ts
			case 'connected':
				this.handleConnected(message.revision, message.mode, message.version);
				break;
```

4. Replace `handleConnected` with:

```ts
	/**
	 * Handle a "connected" message: remembers the revision and tap version
	 * carried by the first one this page load receives, and reloads the page
	 * on any later one (a reconnect - the socket dropped and came back, or
	 * the hub itself restarted) whose revision differs from the one this
	 * page shows, or whose tap version differs from the first one. This is
	 * how a window left open through a `tap dev` restart, or a deck reload
	 * while its socket was down, notices the deck changed instead of going
	 * on showing the old one until someone reloads manually.
	 *
	 * Also sets presentMode on every "connected" message (not just the
	 * first), since it reflects the hub's current mode rather than
	 * something to compare across reconnects.
	 *
	 * Never reloads on the very first "connected" message - there is
	 * nothing to compare it against yet. Never loops: a reload starts a new
	 * page load, and the new WebSocketClient's first "connected" message is
	 * recorded, not compared.
	 */
	private handleConnected(
		revision: string | undefined,
		mode: 'present' | undefined,
		version: string | undefined
	): void {
		// The hub resends a non-fine disk status right after "connected", so a stale one from before a reconnect is cleared here.
		useConnectionStore.setState({ diskStatus: 'ok', presentMode: mode === 'present' });
		if (!this.hasSeenFirstConnected) {
			this.hasSeenFirstConnected = true;
			this.knownRevision = revision;
			this.firstVersion = version;
			return;
		}
		const revisionChanged = revision !== undefined && revision !== this.knownRevision;
		const versionChanged = version !== undefined && this.firstVersion !== undefined && version !== this.firstVersion;
		if (revisionChanged || versionChanged) {
			this.handleReload();
		}
	}

	/**
	 * Handle an "update" message: fetch the deck again and replace it in the
	 * store without reloading (see updatePresentationInPlace), so the slide,
	 * fragment, step and every component's state survive an edit. Only the
	 * newest of overlapping updates applies. A fetch that fails reloads the
	 * page instead, which is never worse than an update.
	 */
	private handleUpdate(): void {
		this.updateSequence += 1;
		const sequence = this.updateSequence;
		fetchPresentation()
			.then((data) => {
				if (sequence !== this.updateSequence) return;
				updatePresentationInPlace(data);
				this.knownRevision = data.revision ?? this.knownRevision;
			})
			.catch(() => {
				if (sequence === this.updateSequence) this.handleReload();
			});
	}
```

5. In `disconnect`, next to `this.knownRevision = undefined;` (renamed in item 2), add:

```ts
		this.firstVersion = undefined;
		this.updateSequence = 0;
```

- [ ] **Step 4: Run the tests to verify they pass**

Run: `cd frontend && npx vitest run src/lib/stores/websocket.test.ts`
Expected: PASS, including the existing revision tests.

Run: `cd frontend && npm run check && npm run lint && npm test -- --run`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add frontend/src/lib/stores/websocket.ts frontend/src/lib/stores/websocket.test.ts
git commit -m "feat(frontend): apply an update message in place instead of reloading"
```

---

### Task 7: Only the slides that changed re-render

**Files:**
- Modify: `frontend/src/lib/components/Slide.tsx`
- Modify: `frontend/src/lib/components/SlideOverview.tsx`
- Modify: `frontend/src/PresenterApp.tsx` (the next-slide panel's key)
- Create: `frontend/src/lib/components/Slide.memo.test.tsx`

**Interfaces:**
- Consumes: `slideKey`, `updatePresentationInPlace` (Task 5)
- Produces: `Slide` is `React.memo`, so a slide whose object and props did not change skips its render. Lists of slides use `key={slideKey(slide)}`.

The audience view renders one slide, and a changed current slide re-renders in place: `SlideTransition` stays keyed by the slide's position, so an edit never plays a transition, and a whole-slide component keeps its state unless its bundle URL changed. The presenter's current-slide panel stays keyed by position for the same reason.

- [ ] **Step 1: Write the failing test**

`frontend/src/lib/components/Slide.memo.test.tsx`:

```tsx
/**
 * After an in-place update, a list of slides re-renders only the slides
 * whose content hash changed.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, render } from '@testing-library/react';
import { Slide } from './Slide';
import { resolveLayout } from '../layouts/registry';
import {
	loadPresentation,
	resetPresentation,
	slideKey,
	updatePresentationInPlace,
	usePresentationStore
} from '$lib/stores/presentation';
import type { LayoutProps, Presentation, Slide as SlideData } from '$lib/types';

vi.mock('../layouts/registry', async (importOriginal) => {
	const actual = await importOriginal<typeof import('../layouts/registry')>();
	return { ...actual, resolveLayout: vi.fn(actual.resolveLayout) };
});

const renderedSlideNumbers: number[] = [];

function CountingLayout({ slide }: LayoutProps) {
	renderedSlideNumbers.push(slide.index + 1);
	return <p>{slide.html}</p>;
}

function makeSlide(index: number, hash: string): SlideData {
	return { index, layout: 'default', html: hash, slots: {}, slotOrder: [], fragmentCount: 0, steps: 0, hash };
}

function makeDeck(revision: string, hashes: string[]): Presentation {
	return { config: { title: 'Deck' }, revision, slides: hashes.map((hash, index) => makeSlide(index, hash)) };
}

function SlideList() {
	const slides = usePresentationStore((state) => state.presentation?.slides ?? []);
	return (
		<>
			{slides.map((slide) => (
				<Slide
					key={slideKey(slide)}
					slide={slide}
					active={false}
					printMode={false}
					fragmentIndex={-1}
					step={0}
					total={slides.length}
				/>
			))}
		</>
	);
}

beforeEach(() => {
	resetPresentation();
	renderedSlideNumbers.length = 0;
	vi.mocked(resolveLayout).mockReturnValue({ component: CountingLayout, slots: ['default'] });
});

afterEach(() => {
	cleanup();
	vi.mocked(resolveLayout).mockReset();
});

describe('Slide after an in-place update', () => {
	it('re-renders only the slide whose content changed', () => {
		loadPresentation(makeDeck('r1', ['a', 'b', 'c']));
		render(<SlideList />);
		expect(new Set(renderedSlideNumbers)).toEqual(new Set([1, 2, 3]));

		renderedSlideNumbers.length = 0;
		act(() => {
			updatePresentationInPlace(makeDeck('r2', ['a', 'b2', 'c']));
		});

		expect(new Set(renderedSlideNumbers)).toEqual(new Set([2]));
	});

	it('re-renders nothing when no slide changed', () => {
		loadPresentation(makeDeck('r1', ['a', 'b']));
		render(<SlideList />);

		renderedSlideNumbers.length = 0;
		act(() => {
			updatePresentationInPlace(makeDeck('r2', ['a', 'b']));
		});

		expect(renderedSlideNumbers).toEqual([]);
	});
});
```

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd frontend && npx vitest run src/lib/components/Slide.memo.test.tsx`
Expected: FAIL. The first test sees slides 1, 2 and 3 render again, because `SlideList` re-renders and `Slide` is not memoized.

- [ ] **Step 3: Memoize `Slide`**

In `frontend/src/lib/components/Slide.tsx`:

1. Add `memo` to the React import: `import { memo, useLayoutEffect, useMemo, useRef, useState, type CSSProperties } from 'react';`
2. Rename the function `export function Slide({` to `function SlideView({` (the body does not change).
3. After the function, add:

```tsx
/**
 * A slide, memoized: an in-place update keeps the object of every slide
 * whose content hash did not change (see updatePresentationInPlace), so
 * those slides skip rendering. A slide still re-renders on its own when a
 * store value it reads changes.
 */
export const Slide = memo(SlideView);
```

4. Add one line to the file's top comment: `Memoized, so a slide whose object and props did not change skips its render.`

- [ ] **Step 4: Key lists of slides by content**

In `frontend/src/lib/components/SlideOverview.tsx`, change the import to `import { usePresentationStore, goToSlide, slideKey } from '$lib/stores/presentation';` and the thumbnail button's `key={slide.index}` to `key={slideKey(slide)}`.

In `frontend/src/PresenterApp.tsx`, add `slideKey` to the import from `'$lib/stores/presentation'`, and change the next-slide panel's `key={nextSlideData.index}` to `key={slideKey(nextSlideData)}`. Leave the current-slide panel's `key={currentSlide.index}`.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd frontend && npx vitest run src/lib/components/Slide.memo.test.tsx`
Expected: PASS.

Run: `cd frontend && npm run check && npm run lint && npm test -- --run`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/components/Slide.tsx frontend/src/lib/components/Slide.memo.test.tsx frontend/src/lib/components/SlideOverview.tsx frontend/src/PresenterApp.tsx
git commit -m "perf(frontend): re-render only the slides whose content changed"
```

---

### Task 8: The ready signal: blockers, probes and the cycle

**Files:**
- Create: `frontend/src/lib/ready/blockers.ts`
- Create: `frontend/src/lib/ready/probes.ts`
- Create: `frontend/src/lib/ready/readySignal.ts`
- Create: `frontend/src/lib/ready/blockers.test.tsx`
- Create: `frontend/src/lib/ready/probes.test.ts`
- Create: `frontend/src/lib/ready/readySignal.test.ts`

**Interfaces:**
- Produces, in `blockers.ts`:
  - `type ReadyBlockerKind = 'fonts' | 'images' | 'map' | 'component' | 'error-card' | 'animations'`
  - `function holdReady(kind: ReadyBlockerKind): () => void` (the release function is safe to call twice)
  - `function heldBlockers(): ReadyBlockerKind[]`
  - `function subscribeToBlockers(listener: () => void): () => void`
  - `function whenNoBlockers(): Promise<void>`
  - `function useReadyHold(kind: ReadyBlockerKind, holding: boolean): void`
  - `function resetBlockersForTests(): void`
- Produces, in `probes.ts`:
  - `interface ReadyProbes { fonts(): Promise<void>; images(): Promise<void>; animations(): Promise<void>; paint(): Promise<void>; settledNow(): boolean }`
  - `interface DomProbeOptions { includeInfiniteAnimations: boolean; document?: Document }`
  - `function createDomProbes(options: DomProbeOptions): ReadyProbes`
  - `STYLESHEET_TIMEOUT_MS = 5000`, `IMAGE_TIMEOUT_MS = 5000`, `ANIMATION_TIMEOUT_MS = 3000`
- Produces, in `readySignal.ts`:
  - `interface ReadyPayload { revision: string; slide: number; step: number }`
  - `READY_EVENT = 'tap:ready'`, `MAX_SETTLE_ROUNDS = 20`
  - `function publishReady(payload: ReadyPayload): void`, `function clearReady(): void`
  - `function waitUntilSettled(probes: ReadyProbes, isCancelled: () => boolean): Promise<boolean>`
  - `function startReadyCycle(payload: ReadyPayload, probes: ReadyProbes): () => void` (returns the cancel function)

A cycle runs rounds of: stylesheets and fonts, images, no blockers, animations, two animation frames. It publishes after the first round that ends with no blocker held and nothing loading (`settledNow`). Starting a cycle sets `window.__tapReady` to `null` at once, and cancels the earlier cycle.

- [ ] **Step 1: Write the failing tests**

`frontend/src/lib/ready/blockers.test.tsx`:

```ts
import { afterEach, describe, expect, it } from 'vitest';
import { cleanup, render } from '@testing-library/react';
import {
	heldBlockers,
	holdReady,
	resetBlockersForTests,
	subscribeToBlockers,
	useReadyHold,
	whenNoBlockers,
	type ReadyBlockerKind
} from './blockers';

afterEach(() => {
	cleanup();
	resetBlockersForTests();
});

describe('blockers', () => {
	it('lists a held blocker until it is released', () => {
		const release = holdReady('map');
		expect(heldBlockers()).toEqual(['map']);
		release();
		expect(heldBlockers()).toEqual([]);
	});

	it('releases once, however often the release function runs', () => {
		const releaseMap = holdReady('map');
		holdReady('component');
		releaseMap();
		releaseMap();
		expect(heldBlockers()).toEqual(['component']);
	});

	it('tells subscribers about every hold and release', () => {
		const seen: ReadyBlockerKind[][] = [];
		const unsubscribe = subscribeToBlockers(() => seen.push(heldBlockers()));
		const release = holdReady('fonts');
		release();
		unsubscribe();
		holdReady('images');
		expect(seen).toEqual([['fonts'], []]);
	});

	it('resolves whenNoBlockers once the last blocker is released', async () => {
		const releaseFirst = holdReady('component');
		const releaseSecond = holdReady('map');
		let resolved = false;
		const waiting = whenNoBlockers().then(() => {
			resolved = true;
		});

		releaseFirst();
		await Promise.resolve();
		expect(resolved).toBe(false);

		releaseSecond();
		await waiting;
		expect(resolved).toBe(true);
	});

	it('holds from a component while holding is true', () => {
		function Holder({ holding }: { holding: boolean }) {
			useReadyHold('animations', holding);
			return null;
		}
		const { rerender, unmount } = render(<Holder holding />);
		expect(heldBlockers()).toEqual(['animations']);

		rerender(<Holder holding={false} />);
		expect(heldBlockers()).toEqual([]);

		rerender(<Holder holding />);
		unmount();
		expect(heldBlockers()).toEqual([]);
	});
});
```

`frontend/src/lib/ready/probes.test.ts`:

```ts
import { afterEach, describe, expect, it, vi } from 'vitest';
import { ANIMATION_TIMEOUT_MS, IMAGE_TIMEOUT_MS, STYLESHEET_TIMEOUT_MS, createDomProbes } from './probes';

function track(promise: Promise<void>): { done: () => boolean } {
	let finished = false;
	void promise.then(() => {
		finished = true;
	});
	return { done: () => finished };
}

afterEach(() => {
	vi.useRealTimers();
	document.body.innerHTML = '';
	document.head.querySelectorAll('link').forEach((link) => link.remove());
});

describe('images', () => {
	it('waits for an image that has not loaded, until it loads', async () => {
		const image = document.createElement('img');
		Object.defineProperty(image, 'complete', { value: false, configurable: true });
		document.body.appendChild(image);

		const waiting = track(createDomProbes({ includeInfiniteAnimations: false }).images());
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		image.dispatchEvent(new Event('load'));
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
	});

	it('counts a broken image as done', async () => {
		const image = document.createElement('img');
		Object.defineProperty(image, 'complete', { value: false, configurable: true });
		document.body.appendChild(image);

		const waiting = track(createDomProbes({ includeInfiniteAnimations: false }).images());
		image.dispatchEvent(new Event('error'));
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
	});

	it('gives up on an image after IMAGE_TIMEOUT_MS', async () => {
		vi.useFakeTimers();
		const image = document.createElement('img');
		Object.defineProperty(image, 'complete', { value: false, configurable: true });
		document.body.appendChild(image);

		const waiting = track(createDomProbes({ includeInfiniteAnimations: false }).images());
		await vi.advanceTimersByTimeAsync(IMAGE_TIMEOUT_MS - 1);
		expect(waiting.done()).toBe(false);
		await vi.advanceTimersByTimeAsync(1);
		expect(waiting.done()).toBe(true);
	});

	it('does not wait for an image that is complete', async () => {
		const image = document.createElement('img');
		Object.defineProperty(image, 'complete', { value: true, configurable: true });
		document.body.appendChild(image);

		await expect(createDomProbes({ includeInfiniteAnimations: false }).images()).resolves.toBeUndefined();
	});
});

describe('stylesheets and fonts', () => {
	it('waits for a stylesheet to load, and then counts it as settled', async () => {
		const link = document.createElement('link');
		link.rel = 'stylesheet';
		document.head.appendChild(link);
		const probes = createDomProbes({ includeInfiniteAnimations: false });
		expect(probes.settledNow()).toBe(false);

		const waiting = track(probes.fonts());
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		link.dispatchEvent(new Event('load'));
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
		expect(probes.settledNow()).toBe(true);
	});

	it('gives up on a stylesheet after STYLESHEET_TIMEOUT_MS', async () => {
		vi.useFakeTimers();
		const link = document.createElement('link');
		link.rel = 'stylesheet';
		document.head.appendChild(link);

		const waiting = track(createDomProbes({ includeInfiniteAnimations: false }).fonts());
		await vi.advanceTimersByTimeAsync(STYLESHEET_TIMEOUT_MS);
		expect(waiting.done()).toBe(true);
	});

	it('waits for document.fonts.ready, and is not settled while fonts load', async () => {
		let resolveFonts!: () => void;
		const fonts = {
			status: 'loading',
			ready: new Promise<void>((resolve) => {
				resolveFonts = resolve;
			})
		};
		const fakeDocument = { querySelectorAll: () => [], fonts } as unknown as Document;
		const probes = createDomProbes({ includeInfiniteAnimations: false, document: fakeDocument });
		expect(probes.settledNow()).toBe(false);

		const waiting = track(probes.fonts());
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		fonts.status = 'loaded';
		resolveFonts();
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
		expect(probes.settledNow()).toBe(true);
	});
});

describe('animations', () => {
	function fakeAnimation(endTime: number): { animation: unknown; finish: () => void } {
		let finish!: () => void;
		const finished = new Promise<void>((resolve) => {
			finish = resolve;
		});
		return { animation: { effect: { getComputedTiming: () => ({ endTime }) }, finished }, finish };
	}

	function documentWith(animations: unknown[]): Document {
		return { querySelectorAll: () => [], getAnimations: () => animations } as unknown as Document;
	}

	it('waits for a running animation to finish', async () => {
		const running = fakeAnimation(400);
		const waiting = track(
			createDomProbes({ includeInfiniteAnimations: false, document: documentWith([running.animation]) }).animations()
		);
		await Promise.resolve();
		expect(waiting.done()).toBe(false);

		running.finish();
		await vi.waitFor(() => expect(waiting.done()).toBe(true));
	});

	it('ignores an animation that repeats forever on a live page', async () => {
		const looping = fakeAnimation(Infinity);
		await expect(
			createDomProbes({ includeInfiniteAnimations: false, document: documentWith([looping.animation]) }).animations()
		).resolves.toBeUndefined();
	});

	it('waits up to ANIMATION_TIMEOUT_MS for an animation that repeats forever on a print page', async () => {
		vi.useFakeTimers();
		const looping = fakeAnimation(Infinity);
		const waiting = track(
			createDomProbes({ includeInfiniteAnimations: true, document: documentWith([looping.animation]) }).animations()
		);
		await vi.advanceTimersByTimeAsync(ANIMATION_TIMEOUT_MS - 1);
		expect(waiting.done()).toBe(false);
		await vi.advanceTimersByTimeAsync(1);
		expect(waiting.done()).toBe(true);
	});
});
```

`frontend/src/lib/ready/readySignal.test.ts`:

```ts
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { holdReady, resetBlockersForTests, type ReadyBlockerKind } from './blockers';
import type { ReadyProbes } from './probes';
import { READY_EVENT, clearReady, startReadyCycle, type ReadyPayload } from './readySignal';

interface ReadyWindow {
	__tapReady?: ReadyPayload | null;
	webkit?: unknown;
}

function readyValue(): ReadyPayload | null | undefined {
	return (window as unknown as ReadyWindow).__tapReady;
}

function instantProbes(overrides: Partial<ReadyProbes> = {}): ReadyProbes {
	return {
		fonts: () => Promise.resolve(),
		images: () => Promise.resolve(),
		animations: () => Promise.resolve(),
		paint: () => Promise.resolve(),
		settledNow: () => true,
		...overrides
	};
}

function deferred(): { promise: Promise<void>; resolve: () => void } {
	let resolve!: () => void;
	const promise = new Promise<void>((settle) => {
		resolve = settle;
	});
	return { promise, resolve };
}

/** Lets every pending promise callback run. */
async function settleMicrotasks(): Promise<void> {
	await new Promise((resolve) => setTimeout(resolve, 0));
}

const payload: ReadyPayload = { revision: 'r1', slide: 2, step: 1 };

beforeEach(() => {
	resetBlockersForTests();
	clearReady();
});

afterEach(() => {
	delete (window as unknown as ReadyWindow).webkit;
});

describe('startReadyCycle', () => {
	it('publishes the payload once nothing blocks', async () => {
		startReadyCycle(payload, instantProbes());
		await vi.waitFor(() => expect(readyValue()).toEqual(payload));
	});

	it('sets __tapReady to null as soon as a cycle starts', async () => {
		startReadyCycle(payload, instantProbes());
		await vi.waitFor(() => expect(readyValue()).toEqual(payload));

		holdReady('component');
		startReadyCycle({ ...payload, slide: 3 }, instantProbes());
		expect(readyValue()).toBeNull();
	});

	it.each<ReadyBlockerKind>(['fonts', 'images', 'map', 'component', 'error-card', 'animations'])(
		'waits for a held %s blocker',
		async (kind) => {
			const release = holdReady(kind);
			startReadyCycle(payload, instantProbes());
			await settleMicrotasks();
			expect(readyValue()).toBeNull();

			release();
			await vi.waitFor(() => expect(readyValue()).toEqual(payload));
		}
	);

	it.each<keyof Omit<ReadyProbes, 'settledNow'>>(['fonts', 'images', 'animations', 'paint'])(
		'waits for the %s probe',
		async (probe) => {
			const pending = deferred();
			startReadyCycle(payload, instantProbes({ [probe]: () => pending.promise } as Partial<ReadyProbes>));
			await settleMicrotasks();
			expect(readyValue()).toBeNull();

			pending.resolve();
			await vi.waitFor(() => expect(readyValue()).toEqual(payload));
		}
	);

	it('runs another round when something started loading during the first one', async () => {
		const fonts = vi.fn(() => Promise.resolve());
		const settledNow = vi.fn().mockReturnValueOnce(false).mockReturnValue(true);
		startReadyCycle(payload, instantProbes({ fonts, settledNow }));

		await vi.waitFor(() => expect(readyValue()).toEqual(payload));
		expect(fonts).toHaveBeenCalledTimes(2);
	});

	it('runs another round when a blocker was held during the first one', async () => {
		let release: (() => void) | null = null;
		const paint = vi.fn(() => {
			if (paint.mock.calls.length === 1) {
				release = holdReady('component');
				setTimeout(() => release?.(), 0);
			}
			return Promise.resolve();
		});
		startReadyCycle(payload, instantProbes({ paint }));

		await vi.waitFor(() => expect(readyValue()).toEqual(payload));
		expect(paint).toHaveBeenCalledTimes(2);
	});

	it('never lets an earlier cycle publish over a later one', async () => {
		const slowFonts = deferred();
		startReadyCycle({ ...payload, slide: 1 }, instantProbes({ fonts: () => slowFonts.promise }));
		startReadyCycle({ ...payload, slide: 2 }, instantProbes());
		await vi.waitFor(() => expect(readyValue()?.slide).toBe(2));

		slowFonts.resolve();
		await settleMicrotasks();
		expect(readyValue()?.slide).toBe(2);
	});

	it('does not publish after it is cancelled', async () => {
		const cancel = startReadyCycle(payload, instantProbes());
		cancel();
		await settleMicrotasks();
		expect(readyValue()).toBeNull();
	});

	it('dispatches tap:ready with the payload', async () => {
		const listener = vi.fn();
		window.addEventListener(READY_EVENT, listener);
		startReadyCycle(payload, instantProbes());

		await vi.waitFor(() => expect(listener).toHaveBeenCalledTimes(1));
		expect((listener.mock.calls[0]?.[0] as CustomEvent<ReadyPayload>).detail).toEqual(payload);
		window.removeEventListener(READY_EVENT, listener);
	});

	it('posts the payload to the tapReady message handler when one exists', async () => {
		const postMessage = vi.fn();
		(window as unknown as ReadyWindow).webkit = { messageHandlers: { tapReady: { postMessage } } };
		startReadyCycle(payload, instantProbes());

		await vi.waitFor(() => expect(postMessage).toHaveBeenCalledWith(payload));
	});

	it('does not fail when webkit has no tapReady handler', async () => {
		(window as unknown as ReadyWindow).webkit = { messageHandlers: {} };
		startReadyCycle(payload, instantProbes());
		await vi.waitFor(() => expect(readyValue()).toEqual(payload));
	});
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd frontend && npx vitest run src/lib/ready`
Expected: FAIL, the three modules do not exist.

- [ ] **Step 3: Write `frontend/src/lib/ready/blockers.ts`**

```ts
/**
 * The things a slide waits on before it counts as rendered. A deck
 * component whose bundle is still loading, a map that has not drawn its
 * tiles, a slide transition that is still running: each one holds a
 * blocker and releases it when it is done. The ready signal (see
 * readySignal.ts) fires only when no blocker is held.
 */

import { useLayoutEffect } from 'react';

/** What a blocker waits on. */
export type ReadyBlockerKind = 'fonts' | 'images' | 'map' | 'component' | 'error-card' | 'animations';

const held = new Map<number, ReadyBlockerKind>();
const listeners = new Set<() => void>();
let nextBlockerId = 1;

function notify(): void {
	for (const listener of [...listeners]) {
		listener();
	}
}

/**
 * Holds a blocker of `kind` until the returned function runs. Calling the
 * returned function again does nothing.
 */
export function holdReady(kind: ReadyBlockerKind): () => void {
	const id = nextBlockerId;
	nextBlockerId += 1;
	held.set(id, kind);
	notify();
	return () => {
		if (held.delete(id)) {
			notify();
		}
	};
}

/** The kind of every blocker held right now, one entry per blocker. */
export function heldBlockers(): ReadyBlockerKind[] {
	return [...held.values()];
}

/** Calls `listener` after every hold and release. Returns the unsubscribe function. */
export function subscribeToBlockers(listener: () => void): () => void {
	listeners.add(listener);
	return () => {
		listeners.delete(listener);
	};
}

/** Resolves once no blocker is held. */
export function whenNoBlockers(): Promise<void> {
	if (held.size === 0) {
		return Promise.resolve();
	}
	return new Promise((resolve) => {
		const unsubscribe = subscribeToBlockers(() => {
			if (held.size === 0) {
				unsubscribe();
				resolve();
			}
		});
	});
}

/**
 * Holds a blocker of `kind` while `holding` is true. A layout effect, so
 * the blocker is held by the time the page's own effects start a ready
 * cycle for the same commit.
 */
export function useReadyHold(kind: ReadyBlockerKind, holding: boolean): void {
	useLayoutEffect(() => {
		if (!holding) {
			return undefined;
		}
		return holdReady(kind);
	}, [kind, holding]);
}

/** Drops every blocker and listener (for testing). */
export function resetBlockersForTests(): void {
	held.clear();
	listeners.clear();
}
```

- [ ] **Step 4: Write `frontend/src/lib/ready/probes.ts`**

```ts
/**
 * The page checks the ready signal runs in each round: stylesheets and web
 * fonts, images, and running animations, plus two animation frames so the
 * last change has painted. Each check resolves when there is nothing left
 * to wait for, or when its time limit runs out, so a broken image or a
 * looping animation never holds the signal forever. The limits are the ones
 * tap's exporter has always used.
 */

/** How long to wait for stylesheets to load. */
export const STYLESHEET_TIMEOUT_MS = 5000;

/** How long to wait for images to load. */
export const IMAGE_TIMEOUT_MS = 5000;

/** How long to wait for running animations to finish. */
export const ANIMATION_TIMEOUT_MS = 3000;

/** The checks one ready round runs. createDomProbes gives the real ones; tests pass their own. */
export interface ReadyProbes {
	/** Resolves when every stylesheet has loaded or failed, and web fonts are ready. */
	fonts(): Promise<void>;
	/** Resolves when every image has loaded or failed. */
	images(): Promise<void>;
	/** Resolves when running animations have finished. */
	animations(): Promise<void>;
	/** Resolves after two animation frames, so the last change has painted. */
	paint(): Promise<void>;
	/** Whether nothing is loading right now: no stylesheet and no web font. */
	settledNow(): boolean;
}

export interface DomProbeOptions {
	/**
	 * Wait for animations that repeat forever too, up to
	 * ANIMATION_TIMEOUT_MS. Print and capture pages do, so an export
	 * captures what it always did. A live page skips them, so a spinner
	 * does not hold every ready cycle for the full limit.
	 */
	includeInfiniteAnimations: boolean;
	/** The document to check. Defaults to the page's own. */
	document?: Document;
}

/** Stylesheets that failed, or took too long: never waited for again. */
const settledStylesheets = new WeakSet<HTMLLinkElement>();

/** Resolves when `promise` settles or `timeoutMs` passes, whichever is first. */
function settleWithin(promise: Promise<unknown>, timeoutMs: number): Promise<void> {
	return new Promise((resolve) => {
		const timer = setTimeout(resolve, timeoutMs);
		const finish = (): void => {
			clearTimeout(timer);
			resolve();
		};
		promise.then(finish, finish);
	});
}

/** Resolves on the element's next load or error event. */
function loadedOrFailed(element: HTMLElement): Promise<void> {
	return new Promise((resolve) => {
		element.addEventListener('load', () => resolve(), { once: true });
		element.addEventListener('error', () => resolve(), { once: true });
	});
}

function pendingStylesheets(target: Document): HTMLLinkElement[] {
	return Array.from(target.querySelectorAll<HTMLLinkElement>('link[rel="stylesheet"]')).filter(
		(link) => link.sheet === null && !settledStylesheets.has(link)
	);
}

async function waitForStylesheetsAndFonts(target: Document): Promise<void> {
	const stylesheets = pendingStylesheets(target);
	if (stylesheets.length > 0) {
		await settleWithin(Promise.all(stylesheets.map(loadedOrFailed)), STYLESHEET_TIMEOUT_MS);
		for (const link of stylesheets) {
			settledStylesheets.add(link);
		}
	}
	const fonts: FontFaceSet | undefined = target.fonts;
	if (fonts?.ready) {
		await fonts.ready;
	}
}

async function waitForImages(target: Document): Promise<void> {
	const images = Array.from(target.querySelectorAll<HTMLImageElement>('img')).filter((image) => !image.complete);
	if (images.length === 0) {
		return;
	}
	await settleWithin(Promise.all(images.map(loadedOrFailed)), IMAGE_TIMEOUT_MS);
}

async function waitForAnimations(target: Document, includeInfinite: boolean): Promise<void> {
	if (typeof target.getAnimations !== 'function') {
		return;
	}
	const running = target
		.getAnimations()
		.filter((animation) => includeInfinite || animation.effect?.getComputedTiming().endTime !== Infinity);
	if (running.length === 0) {
		return;
	}
	await settleWithin(Promise.allSettled(running.map((animation) => animation.finished)), ANIMATION_TIMEOUT_MS);
}

function nextPaint(): Promise<void> {
	return new Promise((resolve) => {
		if (typeof requestAnimationFrame !== 'function') {
			setTimeout(resolve, 16);
			return;
		}
		requestAnimationFrame(() => requestAnimationFrame(() => resolve()));
	});
}

/** The real checks, against a document. */
export function createDomProbes(options: DomProbeOptions): ReadyProbes {
	const target = options.document ?? document;
	return {
		fonts: () => waitForStylesheetsAndFonts(target),
		images: () => waitForImages(target),
		animations: () => waitForAnimations(target, options.includeInfiniteAnimations),
		paint: nextPaint,
		settledNow: () => {
			const fonts: FontFaceSet | undefined = target.fonts;
			return pendingStylesheets(target).length === 0 && fonts?.status !== 'loading';
		}
	};
}
```

- [ ] **Step 5: Write `frontend/src/lib/ready/readySignal.ts`**

```ts
/**
 * The one ready signal. tap export pdf, tap export images and Tap
 * Desktop's thumbnail renderer all wait for it before they capture a
 * slide. Once the slide on screen has settled (see waitUntilSettled), the
 * page sets window.__tapReady to {revision, slide, step}, dispatches a
 * "tap:ready" event on window with the same object as its detail, and
 * posts it to window.webkit.messageHandlers.tapReady when the page runs in
 * a WKWebView that registered that handler. window.__tapReady is null
 * while a slide is settling. `slide` is 1-based, and `step` is the number
 * of steps taken on that slide.
 */

import { heldBlockers, whenNoBlockers } from './blockers';
import type { ReadyProbes } from './probes';

export interface ReadyPayload {
	revision: string;
	slide: number;
	step: number;
}

/** The event dispatched on window when a slide has settled. */
export const READY_EVENT = 'tap:ready';

/** A cycle publishes after this many rounds even if fonts keep starting to load. */
export const MAX_SETTLE_ROUNDS = 20;

interface ReadyWindow {
	__tapReady?: ReadyPayload | null;
	webkit?: {
		messageHandlers?: {
			tapReady?: { postMessage(message: ReadyPayload): void };
		};
	};
}

function readyWindow(): ReadyWindow | null {
	return typeof window === 'undefined' ? null : (window as unknown as ReadyWindow);
}

/** Marks the page as not settled. */
export function clearReady(): void {
	const target = readyWindow();
	if (target) {
		target.__tapReady = null;
	}
}

/** Reports a settled slide through all three channels. */
export function publishReady(payload: ReadyPayload): void {
	const target = readyWindow();
	if (!target) {
		return;
	}
	const message: ReadyPayload = { revision: payload.revision, slide: payload.slide, step: payload.step };
	target.__tapReady = message;
	window.dispatchEvent(new CustomEvent<ReadyPayload>(READY_EVENT, { detail: message }));
	target.webkit?.messageHandlers?.tapReady?.postMessage(message);
}

/**
 * Waits until the page has settled: each round waits for stylesheets and
 * fonts, images, every blocker's release, animations, and a paint. The
 * page has settled after a round that ends with no blocker held and
 * nothing loading. Resolves false when the cycle was cancelled.
 */
export async function waitUntilSettled(probes: ReadyProbes, isCancelled: () => boolean): Promise<boolean> {
	for (let round = 0; round < MAX_SETTLE_ROUNDS; round += 1) {
		await probes.fonts();
		await probes.images();
		await whenNoBlockers();
		await probes.animations();
		await probes.paint();
		if (isCancelled()) {
			return false;
		}
		if (heldBlockers().length === 0 && probes.settledNow()) {
			return true;
		}
	}
	return !isCancelled();
}

let currentCycle = 0;

/**
 * Starts waiting for the slide in `payload` to settle, and publishes
 * `payload` when it has. Clears the signal at once. Starting another
 * cycle, or calling the returned function, cancels this one, so an older
 * slide never reports ready over a newer one.
 */
export function startReadyCycle(payload: ReadyPayload, probes: ReadyProbes): () => void {
	currentCycle += 1;
	const cycle = currentCycle;
	let cancelled = false;
	const isCancelled = (): boolean => cancelled || cycle !== currentCycle;

	clearReady();
	void waitUntilSettled(probes, isCancelled).then((settled) => {
		if (settled && !isCancelled()) {
			publishReady(payload);
		}
	});

	return () => {
		cancelled = true;
	};
}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd frontend && npx vitest run src/lib/ready`
Expected: PASS.

Run: `cd frontend && npm run check && npm run lint`
Expected: no errors. If the linter flags the `FontFaceSet | undefined` annotations as unnecessary, keep them: jsdom has no `document.fonts`, so the value really can be undefined at run time.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/lib/ready
git commit -m "feat(frontend): add the ready signal: blockers, page checks and the settle cycle"
```

---

### Task 9: Deck components and rich blocks hold the signal

**Files:**
- Modify: `frontend/src/lib/components/DeckComponent.tsx`
- Modify: `frontend/src/lib/hooks/useRichBlocks.ts`
- Test: `frontend/src/lib/components/DeckComponent.test.tsx`
- Create: `frontend/src/lib/hooks/useRichBlocks.ready.test.tsx`

**Interfaces:**
- Consumes: `useReadyHold`, `holdReady`, `heldBlockers`, `subscribeToBlockers`, `resetBlockersForTests` (Task 8)
- Produces:
  - A `DeckComponent` holds `component` from mount until its component has rendered or its boundary caught an error, and holds `error-card` from a failed load until the boundary has committed the card or fallback. A `buildError` holds nothing.
  - `useRichBlocks` holds `component` while it renders mermaid, asciinema and Shiki output.

- [ ] **Step 1: Write the failing tests**

Add to the imports of `frontend/src/lib/components/DeckComponent.test.tsx`:

```ts
import { heldBlockers, resetBlockersForTests, subscribeToBlockers, type ReadyBlockerKind } from '$lib/ready/blockers';
```

Add at the end of the file:

```tsx
describe('DeckComponent and the ready signal', () => {
	beforeEach(() => resetBlockersForTests());

	function renderComponent(importer: (url: string) => Promise<unknown>, url = '/components/Chart-1.js') {
		return render(
			<DeckComponent
				source="slides/Chart.jsx"
				url={url}
				props={{}}
				slots={{}}
				slide={makeSlide()}
				step={0}
				steps={0}
				active
				printMode={false}
				importer={importer}
			/>
		);
	}

	it('holds a component blocker until the bundle has loaded and rendered', async () => {
		let resolveImport!: (module: unknown) => void;
		const importer = vi.fn(
			() =>
				new Promise((resolve) => {
					resolveImport = resolve;
				})
		);
		const { container } = renderComponent(importer);
		expect(heldBlockers()).toEqual(['component']);

		await act(async () => {
			resolveImport({ default: () => <p className="chart">chart</p> });
		});

		await waitFor(() => expect(container.querySelector('.chart')).not.toBeNull());
		await waitFor(() => expect(heldBlockers()).toEqual([]));
	});

	it('holds an error-card blocker from a failed load until the error card is on screen', async () => {
		const seen: ReadyBlockerKind[][] = [];
		const unsubscribe = subscribeToBlockers(() => seen.push(heldBlockers()));
		let rejectImport!: (error: Error) => void;
		const importer = vi.fn(
			() =>
				new Promise((_, reject) => {
					rejectImport = reject;
				})
		);
		const consoleError = vi.spyOn(console, 'error').mockImplementation(() => {});
		const { container } = renderComponent(importer, '/components/Broken-1.js');

		await act(async () => {
			rejectImport(new Error('boom'));
		});

		await waitFor(() => expect(container.querySelector('.deck-error-card')).not.toBeNull());
		await waitFor(() => expect(heldBlockers()).toEqual([]));
		unsubscribe();
		consoleError.mockRestore();
		expect(seen.some((kinds) => kinds.includes('error-card'))).toBe(true);
	});

	it('holds nothing for a component with a build error', () => {
		render(
			<DeckComponent
				source="slides/Broken.jsx"
				url=""
				buildError="slides/Broken.jsx:3:7: bad"
				props={{}}
				slots={{}}
				slide={makeSlide()}
				step={0}
				steps={0}
				active
				printMode={false}
			/>
		);
		expect(heldBlockers()).toEqual([]);
	});

	it('releases its blocker when it unmounts before the bundle loads', () => {
		const importer = vi.fn(() => new Promise(() => {}));
		const { unmount } = renderComponent(importer, '/components/Never-1.js');
		expect(heldBlockers()).toEqual(['component']);

		unmount();
		expect(heldBlockers()).toEqual([]);
	});
});
```

`frontend/src/lib/hooks/useRichBlocks.ready.test.tsx`:

```tsx
/**
 * A slide holds a "component" blocker while its rich blocks (mermaid,
 * asciinema, Shiki) render, so the ready signal waits for them.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@testing-library/react';
import { useRef } from 'react';
import type { Slide } from '$lib/types';
import { heldBlockers, resetBlockersForTests } from '$lib/ready/blockers';
import { useRichBlocks } from './useRichBlocks';
import { renderMermaidBlocksInElement } from '../utils/mermaid';

vi.mock('../utils/mermaid', () => ({ renderMermaidBlocksInElement: vi.fn(() => Promise.resolve()) }));
vi.mock('../utils/highlighting', () => ({ highlightCodeBlocksInElement: vi.fn(() => Promise.resolve()) }));
vi.mock('../utils/asciinema', () => ({ renderAsciinemaBlocksInElement: vi.fn(() => Promise.resolve([])) }));

function makeSlide(): Slide {
	return {
		index: 0,
		layout: 'default',
		html: '',
		slots: { default: '<pre><code class="language-mermaid">graph TD; A-->B</code></pre>' },
		slotOrder: ['default'],
		fragmentCount: 0,
		steps: 0
	};
}

function Harness({ slide, active }: { slide: Slide; active: boolean }) {
	const elementRef = useRef<HTMLDivElement>(null);
	useRichBlocks(elementRef, { slide, active, printMode: false });
	return <div ref={elementRef} />;
}

beforeEach(() => resetBlockersForTests());
afterEach(() => cleanup());

describe('useRichBlocks and the ready signal', () => {
	it('holds a component blocker until the rich blocks have rendered', async () => {
		let finishMermaid!: () => void;
		vi.mocked(renderMermaidBlocksInElement).mockReturnValueOnce(
			new Promise<void>((resolve) => {
				finishMermaid = resolve;
			})
		);
		render(<Harness slide={makeSlide()} active />);
		expect(heldBlockers()).toEqual(['component']);

		finishMermaid();
		await waitFor(() => expect(heldBlockers()).toEqual([]));
	});

	it('holds nothing for a slide that is neither active nor printed', () => {
		render(<Harness slide={makeSlide()} active={false} />);
		expect(heldBlockers()).toEqual([]);
	});
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd frontend && npx vitest run src/lib/components/DeckComponent.test.tsx src/lib/hooks/useRichBlocks.ready.test.tsx`
Expected: FAIL. `heldBlockers()` is `[]` where a blocker is expected.

- [ ] **Step 3: Hold the blockers in `DeckComponent`**

In `frontend/src/lib/components/DeckComponent.tsx`:

1. Add `useCallback` and `useState` to the React import, and add:

```ts
import { useReadyHold } from '$lib/ready/blockers';
```

2. In `DeckComponentBoundaryProps`, add:

```ts
	/** Called after the boundary has committed its error card or fallback. */
	onCaught?: () => void;
```

and change `componentDidCatch` to:

```ts
	componentDidCatch(error: unknown): void {
		console.error(`[tap] Component ${this.props.source} failed to render`, error);
		this.props.onCaught?.();
	}
```

3. After `DeckComponentBoundary`, add:

```tsx
/**
 * Calls onSettled when it commits. Rendered inside Suspense next to the
 * lazy component, it commits only once the bundle has loaded and the
 * component has rendered.
 */
function ComponentSettled({ onSettled }: { onSettled: () => void }) {
	useEffect(() => {
		onSettled();
	}, [onSettled]);
	return null;
}

/** Where one bundle is on its way to the screen, for the ready signal. */
type LoadPhase = 'loading' | 'failed' | 'settled';
```

4. In `DeckComponent`, after the `LazyComponent` `useMemo` and before `if (buildError) {`, add:

```tsx
	// The ready signal waits for this component: a bundle that is still
	// loading holds a "component" blocker, and one that failed holds an
	// "error-card" blocker until the boundary below has committed its card
	// or fallback. A build error renders its card at once and holds
	// nothing. Keyed by source and URL, like the boundary, so a new bundle
	// starts over.
	const loadKey = `${source}\u0000${url}`;
	const [load, setLoad] = useState<{ key: string; phase: LoadPhase }>({ key: loadKey, phase: 'loading' });
	const phase: LoadPhase = buildError ? 'settled' : load.key === loadKey ? load.phase : 'loading';
	useReadyHold('component', phase === 'loading');
	useReadyHold('error-card', phase === 'failed');
	const markSettled = useCallback(() => setLoad({ key: loadKey, phase: 'settled' }), [loadKey]);

	// Watches the same cached import the lazy component uses, to learn
	// that it failed before the boundary has shown the error.
	useEffect(() => {
		if (buildError) {
			return undefined;
		}
		let cancelled = false;
		importModule(url, importer, printMode ? undefined : { source }).catch(() => {
			if (cancelled) return;
			setLoad((previous) =>
				previous.key === loadKey && previous.phase === 'settled' ? previous : { key: loadKey, phase: 'failed' }
			);
		});
		return () => {
			cancelled = true;
		};
	}, [buildError, url, importer, printMode, source, loadKey]);
```

5. In the returned tree, pass `onCaught` to the boundary, and add `ComponentSettled` after `LazyComponent`:

```tsx
				<DeckComponentBoundary
					key={`${source}\u0000${url}`}
					source={source}
					buildFallback={buildFallback}
					onCaught={markSettled}
				>
```

```tsx
								<MotionConfig reducedMotion={printMode ? 'always' : 'never'}>
									<LazyComponent slots={slots} props={props} slide={slide} step={step} steps={steps} active={active} printMode={printMode} />
									<ComponentSettled onSettled={markSettled} />
								</MotionConfig>
```

6. The comment above `MotionConfig` names "tap pdf's waitForAnimations". Change that phrase to "the ready signal's animation check". In `DeckComponent.css`, change "`tap pdf`'s waitForAnimations" in the top comment to "the ready signal's animation check" the same way.

- [ ] **Step 4: Hold a blocker in `useRichBlocks`**

In `frontend/src/lib/hooks/useRichBlocks.ts`, add `import { holdReady } from '$lib/ready/blockers';`. In the main effect, right before `void (async () => {`, add:

```ts
		// The slide holds a "component" blocker while this chain runs, so
		// the ready signal waits for mermaid, asciinema and Shiki output.
		const releaseReady = holdReady('component');
```

and add a `finally` to the chain's `try`:

```ts
			} catch (err) {
				console.error('Error processing slide rich content:', err);
			} finally {
				releaseReady();
			}
```

The early `return` statements inside the `try` run the `finally` too.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `cd frontend && npx vitest run src/lib/components/DeckComponent.test.tsx src/lib/hooks`
Expected: PASS, including every existing DeckComponent and useRichBlocks test.

Run: `cd frontend && npm run check && npm run lint && npm test -- --run`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add frontend/src/lib/components/DeckComponent.tsx frontend/src/lib/components/DeckComponent.css frontend/src/lib/components/DeckComponent.test.tsx frontend/src/lib/hooks/useRichBlocks.ts frontend/src/lib/hooks/useRichBlocks.ready.test.tsx
git commit -m "feat(frontend): deck components and rich blocks hold the ready signal"
```

---

### Task 10: Maps, slide transitions and themes hold the signal

**Files:**
- Modify: `frontend/src/lib/components/MapSlide.tsx`
- Modify: `frontend/src/lib/components/SlideTransition.tsx`
- Modify: `frontend/src/lib/hooks/useResolvedTheme.ts`
- Test: `frontend/src/lib/components/MapSlide.test.tsx`, `frontend/src/lib/components/SlideTransition.test.tsx`
- Create: `frontend/src/lib/hooks/useResolvedTheme.test.tsx`

**Interfaces:**
- Consumes: `holdReady`, `useReadyHold`, `heldBlockers`, `resetBlockersForTests` (Task 8)
- Produces:
  - `MAP_LOAD_TIMEOUT_MS = 10000` and `MAP_READY_TIMEOUT_MS = 3000` in `MapSlide.tsx`. A map holds `map` from mount until its first `idle` after `load` (at most `MAP_LOAD_TIMEOUT_MS` before `load`, then at most `MAP_READY_TIMEOUT_MS`), and again during each move to another step, until the next `idle`.
  - `TRANSITION_READY_MARGIN_MS = 1000` in `SlideTransition.tsx`. An animated transition holds `animations` from the key change until the entering slide's animation completes, or `2 * duration + TRANSITION_READY_MARGIN_MS` passes.
  - `useResolvedTheme` holds `fonts` until the requested theme's CSS and fonts are applied.

- [ ] **Step 1: Write the failing tests**

In `frontend/src/lib/components/MapSlide.test.tsx`, change the first import to `import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';`, change `import { MapSlide } from './MapSlide';` to `import { MAP_LOAD_TIMEOUT_MS, MapSlide } from './MapSlide';`, add `import { heldBlockers, resetBlockersForTests } from '$lib/ready/blockers';`, and add at the end of the file:

```tsx
describe('MapSlide and the ready signal', () => {
	beforeEach(() => resetBlockersForTests());
	afterEach(() => {
		cleanup();
		vi.useRealTimers();
		MockMap.instances = [];
	});

	it('holds a map blocker until the map is idle after it loaded', () => {
		render(<MapSlide config={baseConfig()} step={0} />);
		expect(heldBlockers()).toEqual(['map']);

		act(() => latestMap().trigger('idle'));
		expect(heldBlockers()).toEqual(['map']);

		act(() => latestMap().trigger('load'));
		expect(heldBlockers()).toEqual(['map']);

		act(() => latestMap().trigger('idle'));
		expect(heldBlockers()).toEqual([]);
	});

	it('stops waiting for a map that never loads', () => {
		vi.useFakeTimers();
		render(<MapSlide config={baseConfig()} step={0} />);
		expect(heldBlockers()).toEqual(['map']);

		act(() => {
			vi.advanceTimersByTime(MAP_LOAD_TIMEOUT_MS);
		});
		expect(heldBlockers()).toEqual([]);
	});

	it('holds again while the map flies to the next step', () => {
		const config = baseConfig();
		const { rerender } = render(<MapSlide config={config} step={0} />);
		act(() => latestMap().trigger('load'));
		act(() => latestMap().trigger('idle'));
		expect(heldBlockers()).toEqual([]);

		rerender(<MapSlide config={config} step={1} />);
		expect(latestMap().flyTo).toHaveBeenCalled();
		expect(heldBlockers()).toEqual(['map']);

		act(() => latestMap().trigger('idle'));
		expect(heldBlockers()).toEqual([]);
	});

	it('releases its blocker when it unmounts', () => {
		const { unmount } = render(<MapSlide config={baseConfig()} step={0} />);
		unmount();
		expect(heldBlockers()).toEqual([]);
	});
});
```

In `frontend/src/lib/components/SlideTransition.test.tsx`, change the imports to:

```tsx
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, screen, waitFor } from '@testing-library/react';
import { SlideTransition } from './SlideTransition';
import { heldBlockers, resetBlockersForTests } from '$lib/ready/blockers';
```

and add at the end of the file:

```tsx
describe('SlideTransition and the ready signal', () => {
	beforeEach(() => resetBlockersForTests());

	it('holds an animations blocker while it moves to the next slide', async () => {
		const { rerender } = render(
			<SlideTransition slideKey={0} transition="fade" direction="forward">
				<div>first</div>
			</SlideTransition>
		);
		expect(heldBlockers()).toEqual([]);

		rerender(
			<SlideTransition slideKey={1} transition="fade" direction="forward">
				<div>second</div>
			</SlideTransition>
		);
		expect(heldBlockers()).toEqual(['animations']);

		await waitFor(() => expect(heldBlockers()).toEqual([]), { timeout: 3000 });
	});

	it('holds nothing in print mode', () => {
		const { rerender } = render(
			<SlideTransition slideKey={0} transition="fade" direction="forward" printMode>
				<div>first</div>
			</SlideTransition>
		);
		rerender(
			<SlideTransition slideKey={1} transition="fade" direction="forward" printMode>
				<div>second</div>
			</SlideTransition>
		);
		expect(heldBlockers()).toEqual([]);
	});

	it('holds nothing when the transition is none', () => {
		const { rerender } = render(
			<SlideTransition slideKey={0} transition="none" direction="forward">
				<div>first</div>
			</SlideTransition>
		);
		rerender(
			<SlideTransition slideKey={1} transition="none" direction="forward">
				<div>second</div>
			</SlideTransition>
		);
		expect(heldBlockers()).toEqual([]);
	});
});
```

`frontend/src/lib/hooks/useResolvedTheme.test.tsx`:

```tsx
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, renderHook } from '@testing-library/react';
import { heldBlockers, resetBlockersForTests } from '$lib/ready/blockers';
import { loadTheme, type ThemeDefinition } from '$lib/themes/loader';
import { resetPresentation } from '$lib/stores/presentation';
import { useResolvedTheme } from './useResolvedTheme';

vi.mock('$lib/themes/loader', () => ({ loadTheme: vi.fn(), listThemes: vi.fn(() => []) }));

beforeEach(() => {
	resetBlockersForTests();
	resetPresentation();
});
afterEach(() => cleanup());

describe('useResolvedTheme and the ready signal', () => {
	it('holds a fonts blocker until the requested theme has loaded', async () => {
		let finish!: (definition: ThemeDefinition) => void;
		vi.mocked(loadTheme).mockReturnValue(
			new Promise<ThemeDefinition>((resolve) => {
				finish = resolve;
			})
		);
		const { result } = renderHook(() => useResolvedTheme());
		expect(heldBlockers()).toEqual(['fonts']);

		await act(async () => {
			finish({ slug: 'base' } as unknown as ThemeDefinition);
		});

		expect(heldBlockers()).toEqual([]);
		expect(result.current?.slug).toBe('base');
	});
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd frontend && npx vitest run src/lib/components/MapSlide.test.tsx src/lib/components/SlideTransition.test.tsx src/lib/hooks/useResolvedTheme.test.tsx`
Expected: FAIL. `MAP_LOAD_TIMEOUT_MS` is not exported, and no blocker is held.

- [ ] **Step 3: Hold the map**

In `frontend/src/lib/components/MapSlide.tsx`:

1. Change the React import to `import { useEffect, useRef, type MutableRefObject } from 'react';` and add `import { holdReady } from '$lib/ready/blockers';`.
2. After `RESET_DURATION`, add:

```tsx
/** How long the ready signal waits for a new map to load its style. */
export const MAP_LOAD_TIMEOUT_MS = 10000;

/** How long the ready signal waits for a map to go idle after it loaded or moved. */
export const MAP_READY_TIMEOUT_MS = 3000;

/**
 * Holds a "map" blocker until the next "idle" event releases it through
 * `releaseRef`, or until `timeoutMs` passes. Releases the hold that
 * `releaseRef` already had, so a map holds at most one blocker.
 */
function holdMapUntilIdle(releaseRef: MutableRefObject<(() => void) | null>, timeoutMs: number): void {
	releaseRef.current?.();
	const release = holdReady('map');
	let timer: ReturnType<typeof setTimeout> | undefined;
	const releaseThisHold = (): void => {
		clearTimeout(timer);
		release();
		if (releaseRef.current === releaseThisHold) {
			releaseRef.current = null;
		}
	};
	timer = setTimeout(releaseThisHold, timeoutMs);
	releaseRef.current = releaseThisHold;
}
```

3. In `MapSlide`, after `const previousStepRef = useRef(step);`, add:

```tsx
	// The ready signal waits while this map draws: from mount until its
	// first "idle" after "load", and during each move until the next "idle".
	const releaseMapHoldRef = useRef<(() => void) | null>(null);
```

4. In the create effect, after `isReadyRef.current = false;`, add `holdMapUntilIdle(releaseMapHoldRef, MAP_LOAD_TIMEOUT_MS);`. At the end of the `map.on('load', ...)` handler, after the `__tapMapReady` block, add `holdMapUntilIdle(releaseMapHoldRef, MAP_READY_TIMEOUT_MS);`. After the `map.on('error', ...)` call, add:

```tsx
		map.on('idle', () => {
			if (isReadyRef.current) {
				releaseMapHoldRef.current?.();
			}
		});
```

In the effect's cleanup, first line, add `releaseMapHoldRef.current?.();`.

5. In the step effect, right before each `map.flyTo(...)` and `map.jumpTo(...)` call, add a hold. For the flight to the end view:

```tsx
				holdMapUntilIdle(releaseMapHoldRef, config.duration + MAP_READY_TIMEOUT_MS);
```

For the flight back to the start: `holdMapUntilIdle(releaseMapHoldRef, RESET_DURATION + MAP_READY_TIMEOUT_MS);`. For both `jumpTo` calls: `holdMapUntilIdle(releaseMapHoldRef, MAP_READY_TIMEOUT_MS);`.

Keep `window.__tapMap` and `window.__tapMapReady`: the e2e map specs read them.

- [ ] **Step 4: Hold the transition**

In `frontend/src/lib/components/SlideTransition.tsx`:

1. Add `import { useEffect, useRef, useState, type ReactNode } from 'react';` (replacing the `ReactNode`-only import) and `import { useReadyHold } from '$lib/ready/blockers';`.
2. Add after the imports:

```tsx
/**
 * Extra time, past the exit and enter animations, after which the ready
 * signal stops waiting for a transition that never reported completion.
 */
export const TRANSITION_READY_MARGIN_MS = 1000;
```

3. At the top of `SlideTransition`, after `skipAnimation`, add:

```tsx
	// The ready signal waits from a slide change until the entering slide's
	// animation completes. The exiting slide reports completion with its
	// own, older key, which is ignored.
	const [settledKey, setSettledKey] = useState(slideKey);
	const latestKeyRef = useRef(slideKey);
	latestKeyRef.current = slideKey;
	const transitioning = !skipAnimation && settledKey !== slideKey;
	useReadyHold('animations', transitioning);
	const markSettled = (completedKey: number | string): void => {
		if (completedKey === latestKeyRef.current) {
			setSettledKey(completedKey);
		}
	};

	useEffect(() => {
		if (!transitioning) {
			return undefined;
		}
		const timer = setTimeout(() => setSettledKey(slideKey), duration * 2 + TRANSITION_READY_MARGIN_MS);
		return () => clearTimeout(timer);
	}, [transitioning, slideKey, duration]);
```

4. On the `motion.div`, add `onAnimationComplete={() => markSettled(slideKey)}`.
5. Add a sentence to the file's top comment: "An animated change holds the ready signal until the entering slide has finished animating."

- [ ] **Step 5: Hold the theme**

Replace the body of `useResolvedTheme` in `frontend/src/lib/hooks/useResolvedTheme.ts` with:

```ts
export function useResolvedTheme(): ThemeDefinition | null {
	const requestedTheme = usePresentationStore(selectCurrentThemeSlug);

	// The theme actually applied lags one step behind requestedTheme: it only
	// updates once loadTheme resolves (its CSS and fonts loaded), so
	// data-theme never switches to a theme whose styles aren't ready yet.
	const [resolved, setResolved] = useState<{ requested: string; definition: ThemeDefinition } | null>(null);
	useEffect(() => {
		let cancelled = false;
		void loadTheme(requestedTheme).then((definition) => {
			if (!cancelled) setResolved({ requested: requestedTheme, definition });
		});
		return () => {
			cancelled = true;
		};
	}, [requestedTheme]);

	// The ready signal waits until the requested theme is the one applied.
	useReadyHold('fonts', resolved?.requested !== requestedTheme);

	return resolved?.definition ?? null;
}
```

Add `import { useReadyHold } from '$lib/ready/blockers';`. Comparing the requested slug, not the definition's slug, matters: an unknown theme resolves to `base`, and must still release the blocker.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd frontend && npx vitest run src/lib/components/MapSlide.test.tsx src/lib/components/SlideTransition.test.tsx src/lib/hooks/useResolvedTheme.test.tsx`
Expected: PASS.

Run: `cd frontend && npm run check && npm run lint && npm test -- --run`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/lib/components/MapSlide.tsx frontend/src/lib/components/MapSlide.test.tsx frontend/src/lib/components/SlideTransition.tsx frontend/src/lib/components/SlideTransition.test.tsx frontend/src/lib/hooks/useResolvedTheme.ts frontend/src/lib/hooks/useResolvedTheme.test.tsx
git commit -m "feat(frontend): maps, slide transitions and theme loading hold the ready signal"
```

---

### Task 11: The audience and presenter pages publish the signal

**Files:**
- Create: `frontend/src/lib/ready/useReadySignal.ts`
- Modify: `frontend/src/App.tsx`
- Modify: `frontend/src/PresenterApp.tsx`
- Create: `frontend/src/App.ready.test.tsx`
- Create: `frontend/src/PresenterApp.ready.test.tsx`

**Interfaces:**
- Consumes: `startReadyCycle`, `clearReady` (Task 8), `createDomProbes` (Task 8), `presentation.revision` (Task 5), `updatePresentationInPlace` (Task 5)
- Produces:
  - `interface ReadySignalState { enabled: boolean; revision: string; slide: number; step: number; fragment: number; theme: string; includeInfiniteAnimations: boolean }`
  - `function useReadySignal(state: ReadySignalState): void`: a new cycle on every change of any field
  - Both pages report `{revision, slide, step}`: `slide` is `currentSlideIndex + 1`, and `step` is the step rendered (the slide's step count in print mode)

- [ ] **Step 1: Write the failing tests**

`frontend/src/App.ready.test.tsx`:

```tsx
/**
 * The audience page reports {revision, slide, step} once the slide on
 * screen has settled, and resets when the slide or the deck changes. Print
 * mode is read once at module load, so the URL is set and App imported
 * fresh for each test.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { act, cleanup, render, waitFor } from '@testing-library/react';
import type { Presentation } from '$lib/types';

const presentation: Presentation = {
	config: { title: 'Ready Deck' },
	revision: 'r1',
	slides: [
		{
			index: 0,
			layout: 'default',
			html: '<p>One</p>',
			slots: { default: '<p>One</p>' },
			slotOrder: ['default'],
			fragmentCount: 0,
			steps: 2,
			hash: 'a'
		},
		{
			index: 1,
			layout: 'default',
			html: '<p>Two</p>',
			slots: { default: '<p>Two</p>' },
			slotOrder: ['default'],
			fragmentCount: 0,
			steps: 0,
			hash: 'b'
		}
	]
};

interface ReadyWindow {
	__tapReady?: unknown;
	webkit?: unknown;
}

function readyValue(): unknown {
	return (window as unknown as ReadyWindow).__tapReady;
}

async function renderApp(): Promise<typeof import('$lib/stores/presentation')> {
	const store = await import('$lib/stores/presentation');
	store.resetPresentation();
	const { default: App } = await import('./App');
	render(<App />);
	return store;
}

describe('App ready signal in print mode', () => {
	beforeEach(() => {
		vi.resetModules();
		window.history.pushState({}, '', '/?print=true#1');
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					statusText: 'OK',
					json: () => Promise.resolve(presentation)
				} as Response)
			)
		);
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
		window.history.pushState({}, '', '/');
		delete (window as unknown as ReadyWindow).__tapReady;
		delete (window as unknown as ReadyWindow).webkit;
	});

	it('reports the revision, the 1-based slide and the final step', async () => {
		await renderApp();
		await waitFor(() => expect(readyValue()).toEqual({ revision: 'r1', slide: 1, step: 2 }));
	});

	it('posts the same payload to the tapReady message handler', async () => {
		const postMessage = vi.fn();
		(window as unknown as ReadyWindow).webkit = { messageHandlers: { tapReady: { postMessage } } };
		await renderApp();
		await waitFor(() => expect(postMessage).toHaveBeenCalledWith({ revision: 'r1', slide: 1, step: 2 }));
	});

	it('resets on navigation and reports the new slide', async () => {
		await renderApp();
		await waitFor(() => expect(readyValue()).toEqual({ revision: 'r1', slide: 1, step: 2 }));

		act(() => {
			window.history.pushState({}, '', '/?print=true#2');
			window.dispatchEvent(new HashChangeEvent('hashchange'));
		});
		expect(readyValue()).toBeNull();

		await waitFor(() => expect(readyValue()).toEqual({ revision: 'r1', slide: 2, step: 0 }));
	});

	it('resets after an in-place update and reports the new revision', async () => {
		const store = await renderApp();
		await waitFor(() => expect(readyValue()).toEqual({ revision: 'r1', slide: 1, step: 2 }));

		act(() => {
			store.updatePresentationInPlace({ ...presentation, revision: 'r2' });
		});
		expect(readyValue()).toBeNull();

		await waitFor(() => expect(readyValue()).toEqual({ revision: 'r2', slide: 1, step: 2 }));
	});
});
```

`frontend/src/PresenterApp.ready.test.tsx`:

```tsx
/**
 * tap export pdf --content both captures /presenter?print=true, so the
 * presenter page reports the ready signal too.
 */
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { cleanup, render, waitFor } from '@testing-library/react';
import type { Presentation } from '$lib/types';

const presentation: Presentation = {
	config: { title: 'Ready Deck' },
	revision: 'r7',
	slides: [
		{
			index: 0,
			layout: 'default',
			html: '<p>One</p>',
			slots: { default: '<p>One</p>' },
			slotOrder: ['default'],
			fragmentCount: 0,
			steps: 1,
			hash: 'a'
		}
	]
};

describe('PresenterApp ready signal in print mode', () => {
	beforeEach(() => {
		vi.resetModules();
		window.history.pushState({}, '', '/presenter?print=true#1');
		vi.stubGlobal(
			'fetch',
			vi.fn(() =>
				Promise.resolve({
					ok: true,
					statusText: 'OK',
					json: () => Promise.resolve(presentation)
				} as Response)
			)
		);
		vi.stubGlobal('navigator', { wakeLock: undefined });
	});

	afterEach(() => {
		cleanup();
		vi.unstubAllGlobals();
		window.history.pushState({}, '', '/');
		delete (window as unknown as { __tapReady?: unknown }).__tapReady;
	});

	it('reports the revision, the slide and the final step', async () => {
		const { resetPresentation } = await import('$lib/stores/presentation');
		resetPresentation();
		const { default: PresenterApp } = await import('./PresenterApp');
		render(<PresenterApp />);

		await waitFor(() =>
			expect((window as unknown as { __tapReady?: unknown }).__tapReady).toEqual({
				revision: 'r7',
				slide: 1,
				step: 1
			})
		);
	});
});
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `cd frontend && npx vitest run src/App.ready.test.tsx src/PresenterApp.ready.test.tsx`
Expected: FAIL, `__tapReady` stays undefined.

- [ ] **Step 3: Write `frontend/src/lib/ready/useReadySignal.ts`**

```ts
/**
 * Runs the ready signal for the slide a page shows (see readySignal.ts):
 * a new cycle starts whenever the revision, slide, step, fragment or theme
 * changes, and the signal is cleared while the deck has not loaded.
 */

import { useEffect } from 'react';
import { createDomProbes } from './probes';
import { clearReady, startReadyCycle } from './readySignal';

export interface ReadySignalState {
	/** False while the deck is loading or failed to load; the page then reports nothing. */
	enabled: boolean;
	/** The deck's revision, or "" when it has none (a static build). */
	revision: string;
	/** The 1-based slide on screen. */
	slide: number;
	/** The step rendered, from 0 to the slide's step count. */
	step: number;
	/** The fragment rendered. A change restarts the signal; it is not part of the payload. */
	fragment: number;
	/** The theme applied. A change restarts the signal; it is not part of the payload. */
	theme: string;
	/** See DomProbeOptions.includeInfiniteAnimations. */
	includeInfiniteAnimations: boolean;
}

export function useReadySignal({
	enabled,
	revision,
	slide,
	step,
	fragment,
	theme,
	includeInfiniteAnimations
}: ReadySignalState): void {
	useEffect(() => {
		if (!enabled) {
			clearReady();
			return undefined;
		}
		return startReadyCycle({ revision, slide, step }, createDomProbes({ includeInfiniteAnimations }));
		// fragment and theme are not read here, but a change to either is a
		// new rendering that has to settle again.
	}, [enabled, revision, slide, step, fragment, theme, includeInfiniteAnimations]);
}
```

- [ ] **Step 4: Call it from `App.tsx`**

In `frontend/src/App.tsx`, add `import { useReadySignal } from '$lib/ready/useReadySignal';`. After `const transition = resolveTransition(...)`, add:

```tsx
	// Tells tap export and Tap Desktop when the slide on screen has settled
	// (see lib/ready/readySignal.ts). Print mode renders the final step and
	// fragment, so it reports those. A print or capture page waits for
	// looping animations too, as exports always have.
	useReadySignal({
		enabled: !isLoading && loadError === null && currentSlide !== null,
		revision: presentation?.revision ?? '',
		slide: currentSlideIndex + 1,
		step: PRINT_MODE ? (currentSlide?.steps ?? 0) : currentStep,
		fragment: PRINT_MODE ? (currentSlide?.fragmentCount ?? 0) : currentFragmentIndex,
		theme,
		includeInfiniteAnimations: PRINT_MODE || CAPTURE_MODE
	});
```

Add one line to the file's top comment: "Reports the ready signal for the slide on screen."

- [ ] **Step 5: Call it from `PresenterApp.tsx`**

In `frontend/src/PresenterApp.tsx`, add `import { useReadySignal } from '$lib/ready/useReadySignal';`. After the `useFitText(...)` call (before any `if (isLoading)` return), add:

```tsx
	// tap export pdf --content both and --content notes capture this page,
	// so it reports the ready signal for its current slide too.
	useReadySignal({
		enabled: !isLoading && loadError === null && currentSlide !== null,
		revision: presentation?.revision ?? '',
		slide: currentSlideIndex + 1,
		step: PRINT_MODE ? (currentSlide?.steps ?? 0) : currentStep,
		fragment: PRINT_MODE ? (currentSlide?.fragmentCount ?? 0) : currentFragmentIndex,
		theme,
		includeInfiniteAnimations: PRINT_MODE
	});
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `cd frontend && npx vitest run src/App.ready.test.tsx src/PresenterApp.ready.test.tsx`
Expected: PASS.

Run: `cd frontend && npm run check && npm run lint && npm test -- --run`
Expected: PASS, including `App.print.test.tsx`, `App.capture.test.tsx` and `PresenterApp.print.test.tsx`.

- [ ] **Step 7: Commit**

```bash
git add frontend/src/lib/ready/useReadySignal.ts frontend/src/App.tsx frontend/src/PresenterApp.tsx frontend/src/App.ready.test.tsx frontend/src/PresenterApp.ready.test.tsx
git commit -m "feat(frontend): the audience and presenter pages report when a slide has settled"
```

---

### Task 12: The exporter waits for the signal, and its checklist goes

**Files:**
- Create: `internal/pdf/ready.go`
- Create: `internal/pdf/ready_test.go`
- Modify: `internal/pdf/exporter.go` (`Export`, `exportSlides`, `exportNotes`, `exportBoth`; delete `waitForImages`, `waitForMaps`)
- Modify: `internal/pdf/capture.go` (`CaptureOptions.WaitMS` comment, `CaptureSlide`; delete `waitForFonts`, `waitForAnimations`)
- Test: `internal/cli/export_images_test.go`

**Interfaces:**
- Consumes: the frontend's `window.__tapReady` (Tasks 8 and 11)
- Produces: `var readyTimeout = 30 * time.Second` and `func waitForReady(page playwright.Page, slideNumber int) error` in `internal/pdf`. Every capture path waits for `window.__tapReady.slide === slideNumber` and nothing else.

- [ ] **Step 1: Record the current export output**

Before changing any Go code, build `main` in a second worktree and export the example decks with it. These images are the reference for "export output must not change".

```bash
git worktree add /Users/codemonkey/projects/tap-ready-baseline main
(cd /Users/codemonkey/projects/tap-ready-baseline && make build)
export BASELINE="$TMPDIR/tap-ready-before"
for deck in examples/components/deck.md examples/conference-talk.md examples/theme-tour.md examples/code-demo.md examples/basic.md; do
  name="$(basename "$(dirname "$deck")")-$(basename "$deck" .md)"
  /Users/codemonkey/projects/tap-ready-baseline/bin/tap export images "$deck" --all --output "$BASELINE/$name"
  /Users/codemonkey/projects/tap-ready-baseline/bin/tap export pdf "$deck" --output "$BASELINE/$name.pdf" --json > "$BASELINE/$name.json"
done
```

Run the loop a second time into `$TMPDIR/tap-ready-before-again`, and compare the two runs with the `cmp` loop from Step 8. Any file that differs between two runs of the same binary is nondeterministic already (a map tile, a clock); note those names, because Step 8 cannot hold them to byte equality.

- [ ] **Step 2: Write the failing tests**

`internal/pdf/ready_test.go`:

```go
package pdf

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestWaitForReady(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}
	exporter, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer exporter.Close()
	if err := exporter.EnsureBrowser(); err != nil {
		if os.Getenv("CI") != "" {
			t.Fatalf("browser unavailable in CI: %v", err)
		}
		t.Skipf("skipping: no browser available: %v", err)
	}

	page, err := exporter.browser.NewPage()
	if err != nil {
		t.Fatalf("NewPage() error = %v", err)
	}
	defer page.Close()

	// The page reports slide 1 first and slide 3 later, as a page does
	// after a hash navigation. Only slide 3 may end the wait.
	if err := page.SetContent(`<script>
		setTimeout(() => {
			window.__tapReady = { revision: "", slide: 1, step: 0 };
			setTimeout(() => { window.__tapReady = { revision: "", slide: 3, step: 0 }; }, 100);
		}, 100);
	</script>`); err != nil {
		t.Fatalf("SetContent() error = %v", err)
	}
	if err := waitForReady(page, 3); err != nil {
		t.Fatalf("waitForReady(3) error = %v", err)
	}

	previousTimeout := readyTimeout
	readyTimeout = 300 * time.Millisecond
	defer func() { readyTimeout = previousTimeout }()

	err = waitForReady(page, 4)
	if err == nil || !strings.Contains(err.Error(), "slide 4 did not finish rendering") {
		t.Errorf("waitForReady(4) error = %v, want a timeout that names slide 4", err)
	}
}
```

Add to `internal/cli/export_images_test.go`:

```go
// TestExportImagesWaitsForAComponentBundle exports a whole-slide component
// that paints the slide red. The PNG is red only if the capture waited for
// the bundle to load and render, which is what the ready signal is for.
func TestExportImagesWaitsForAComponentBundle(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}
	deckFolder := t.TempDir()
	if err := os.MkdirAll(filepath.Join(deckFolder, "slides"), 0o755); err != nil {
		t.Fatal(err)
	}
	component := "export default function Solid() {\n  return <div style={{ position: 'absolute', inset: 0, background: 'rgb(255, 0, 0)' }} />;\n}\n"
	if err := os.WriteFile(filepath.Join(deckFolder, "slides", "Solid.jsx"), []byte(component), 0o644); err != nil {
		t.Fatal(err)
	}
	deckPath := filepath.Join(deckFolder, "deck.md")
	deck := "---\ntitle: Solid\n---\n\n<!--\nlayout: ./slides/Solid.jsx\n-->\n\n# Solid\n"
	if err := os.WriteFile(deckPath, []byte(deck), 0o644); err != nil {
		t.Fatal(err)
	}

	outputPath := filepath.Join(deckFolder, "solid.png")
	exitCode, _, stderr := runTap(t, "export", "images", deckPath, "--slide", "1", "--output", outputPath)
	if exitCode != exitOK {
		if exitCode == exitInternal && os.Getenv("CI") == "" {
			t.Skipf("skipping: the export could not start a browser: %s", stderr)
		}
		t.Fatalf("tap export images exited %d: %s", exitCode, stderr)
	}

	file, err := os.Open(outputPath)
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	picture, err := png.Decode(file)
	if err != nil {
		t.Fatalf("png.Decode() error = %v", err)
	}
	bounds := picture.Bounds()
	red, green, blue, _ := picture.At(bounds.Dx()/2, bounds.Dy()/2).RGBA()
	if red>>8 < 200 || green>>8 > 60 || blue>>8 > 60 {
		t.Errorf("center pixel = (%d, %d, %d), want red: the component had not rendered", red>>8, green>>8, blue>>8)
	}
}
```

Add `"image/png"` to the imports of `export_images_test.go` if it is missing.

- [ ] **Step 3: Run the tests to verify they fail**

Run: `go test ./internal/pdf -run TestWaitForReady -v`
Expected: compile failure, `undefined: waitForReady`.

`TestExportImagesWaitsForAComponentBundle` can pass before this change too, because network idle happens to cover the bundle's import today. It stays as the guard for the new path.

- [ ] **Step 4: Write `internal/pdf/ready.go`**

```go
package pdf

import (
	"fmt"
	"time"

	"github.com/mxschmitt/playwright-go"
)

// readyTimeout is how long a slide may take to settle before an export
// gives up on it. A variable so tests can shorten it.
var readyTimeout = 30 * time.Second

// waitForReady waits until the page reports that slideNumber (1-based) has
// settled. The frontend owns this decision: it sets window.__tapReady to
// {revision, slide, step} once fonts, images, maps, deck components, error
// cards, transitions and theme animations are done (see
// frontend/src/lib/ready/readySignal.ts). Tap Desktop's thumbnail renderer
// waits for the same signal.
func waitForReady(page playwright.Page, slideNumber int) error {
	_, err := page.WaitForFunction(
		`(slide) => window.__tapReady != null && window.__tapReady.slide === slide`,
		slideNumber,
		playwright.PageWaitForFunctionOptions{Timeout: playwright.Float(float64(readyTimeout.Milliseconds()))},
	)
	if err != nil {
		return fmt.Errorf("slide %d did not finish rendering within %s: %w", slideNumber, readyTimeout, err)
	}
	return nil
}
```

- [ ] **Step 5: Use it in `exporter.go`**

In `internal/pdf/exporter.go`:

1. In `Export`, delete the block after the first `page.Goto`:

```go
	// Wait for the presentation to load
	if err := page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{
		State: playwright.LoadStateNetworkidle,
	}); err != nil {
		return nil, fmt.Errorf("failed to wait for page load: %w", err)
	}
```

`getSlideCount` already retries for 10 seconds until the deck has loaded.

2. In `exportSlides`, replace everything from `// Wait for slide to render` down to the end of the `waitForAnimations` check (the network idle wait, `waitForImages`, `waitForMaps`, `waitForFonts`, `waitForAnimations` and their comments) with:

```go
		// The page reports when this slide has settled: fonts, images,
		// maps, components and animations are all done.
		if err := waitForReady(page, i+1); err != nil {
			return nil, err
		}
```

3. In `exportNotes`, replace the network idle wait and the `time.Sleep(100 * time.Millisecond)` with the same `waitForReady(page, i+1)` block.

4. In `exportBoth`, replace the network idle wait, `waitForImages`, `waitForMaps` and `time.Sleep(200 * time.Millisecond)` with the same block.

5. Delete the functions `waitForImages` and `waitForMaps`.

6. Change the comment on `ExportResult.BrokenSlides` if it names `tap pdf`; it should say `tap export pdf`.

- [ ] **Step 6: Use it in `capture.go`**

In `internal/pdf/capture.go`:

1. Replace the `WaitMS` comment in `CaptureOptions` with:

```go
	// WaitMS, when greater than 0, sleeps this many milliseconds after the
	// page reports the slide ready, before capturing: for the rare case
	// where a moment mid-animation is wanted rather than the settled
	// state. A capture with WaitMS > 0 stays live (?live=true), so
	// components and themes animate instead of settling.
	WaitMS int
```

2. In `CaptureSlide`, replace everything from the network idle wait through the `waitForAnimations` check with:

```go
	if err := waitForReady(page, options.SlideNumber); err != nil {
		return err
	}
```

and change the comment inside the `WaitMS` block to: `// Applied after the ready signal: a live capture still waits for fonts, images, maps and components, and then this long on top.`

3. Replace `CaptureSlide`'s doc comment with:

```go
// CaptureSlide renders one slide state, per options, and writes it as a PNG to
// outputPath. It waits for the page's ready signal (see waitForReady), then
// fails with a descriptive error if the rendered slide shows a slide or
// component error card (see ErrorCardSelector). With options.WaitMS > 0, it
// sleeps that many extra milliseconds after the signal, for a capture that
// deliberately wants a moment mid-animation rather than the settled state.
// ctx is checked before the capture starts and again before the screenshot
// is taken, so a caller looping over several slides (tap export images
// --all) can stop between slides on cancellation instead of starting one it
// will only throw away.
```

4. Delete `waitForFonts` and `waitForAnimations`.

5. Run: `grep -n "waitForImages\|waitForMaps\|waitForFonts\|waitForAnimations\|LoadStateNetworkidle\|time.Sleep" internal/pdf/*.go`
Expected: no output, apart from test files.

- [ ] **Step 7: Run the tests to verify they pass**

Run: `make frontend && go build ./... && go vet ./internal/pdf`
Expected: no errors.

Run: `go test ./internal/pdf ./internal/cli -run 'TestWaitForReady|TestExport|TestScreenshotIntegration|TestGetSlideCount|TestImageServerEndpoint' -v`
Expected: PASS (or SKIP with no browser outside CI). This includes the existing `TestExportSlides`, `TestExportSlides_BrokenSlideReportsDataMessage`, `TestExportSlidesWithImages`, `TestExportNoSlides` and `TestScreenshotIntegration_RollingDeployStepsDiffer`, now running against the signal.

- [ ] **Step 8: Check that the output did not change**

In a new shell, set `BASELINE="$TMPDIR/tap-ready-before"` again first.

```bash
make build
export AFTER="$TMPDIR/tap-ready-after"
for deck in examples/components/deck.md examples/conference-talk.md examples/theme-tour.md examples/code-demo.md examples/basic.md; do
  name="$(basename "$(dirname "$deck")")-$(basename "$deck" .md)"
  ./bin/tap export images "$deck" --all --output "$AFTER/$name"
  ./bin/tap export pdf "$deck" --output "$AFTER/$name.pdf" --json > "$AFTER/$name.json"
done
for before in "$BASELINE"/*/*.png; do
  after="$AFTER/${before#"$BASELINE"/}"
  cmp -s "$before" "$after" || echo "DIFFERS: ${before#"$BASELINE"/}"
done
for before in "$BASELINE"/*.json; do
  diff <(grep -o '"pages": [0-9]*' "$before") <(grep -o '"pages": [0-9]*' "$AFTER/$(basename "$before")") || echo "PAGE COUNT DIFFERS: $(basename "$before")"
done
```

Expected: no `DIFFERS` line, except the files Step 1 found nondeterministic. For any other difference, open both PNGs side by side. A difference is a regression to fix before this task is done, unless the new image is the correct one and the old one caught a slide mid-render (then say so in the PR, with both images).

Remove the baseline worktree: `git worktree remove /Users/codemonkey/projects/tap-ready-baseline`.

- [ ] **Step 9: Commit**

```bash
git add internal/pdf internal/cli/export_images_test.go
git commit -m "refactor(pdf): wait for the frontend's ready signal instead of a checklist"
```

---

### Task 13: The `--progress json` reporter and its schema

**Files:**
- Create: `internal/cli/progress.go`
- Create: `internal/cli/progress_test.go`
- Create: `internal/cli/progress_schema_test.go`
- Modify: `internal/cli/root.go` (`execute`)
- Modify: `internal/cli/conventions_test.go` (`flagConventions`)

**Interfaces:**
- Consumes: `jsonError`, `userError`, `codeUsage`, `classify`, `execute` (P1)
- Produces:
  - `const progressFormatJSON = "json"` and the phase constants `progressPhaseDownload` (`download`), `progressPhaseRender` (`render`), `progressPhaseLoad` (`load`), `progressPhaseParse` (`parse`), `progressPhaseBundle` (`bundle`), `progressPhaseWrite` (`write`), `progressPhaseDone` (`done`)
  - `func newProgressReporter(format string, writer io.Writer) (*progressReporter, error)`: `""` reports nothing, `"json"` writes lines, anything else is a `codeUsage` user error
  - methods on `*progressReporter`: `enabled() bool`, `Step(phase string, done, total int)`, `Render(done, total int)`, `Download(downloaded, totalBytes int64)`, `Result(payload any) error`
  - `func progressRequested(command *cobra.Command) bool`
  - `func writeProgressFailure(writer io.Writer, code, message string)`
  - Test helpers: `func checkProgressLine(line string) error`, `func checkProgressOutput(t *testing.T, stderr string) []map[string]any`

The lines, one JSON object each, in this field order:

```
{"phase":"render","done":7,"total":14}
{"phase":"download","bytes":52428800,"totalBytes":170175488}
{"phase":"done","ok":true,<the command's --json result fields>}
{"phase":"done","ok":false,"error":{"code":"deck_not_found","message":"..."}}
```

- [ ] **Step 1: Write the failing tests**

`internal/cli/progress_schema_test.go`:

```go
package cli

import (
	"encoding/json"
	"fmt"
	"math"
	"strings"
	"testing"
)

// progressStepPhases are the phases of a step line.
var progressStepPhases = map[string]bool{"render": true, "load": true, "parse": true, "bundle": true, "write": true}

// checkProgressLine checks one --progress json line against the shapes the
// docs promise: a step line, a download line, or the final "done" line.
func checkProgressLine(line string) error {
	var fields map[string]any
	if err := json.Unmarshal([]byte(line), &fields); err != nil {
		return fmt.Errorf("not a JSON object: %w: %s", err, line)
	}
	phase, ok := fields["phase"].(string)
	if !ok {
		return fmt.Errorf(`no string "phase": %s`, line)
	}
	switch {
	case progressStepPhases[phase]:
		done, doneOK := wholeNumber(fields["done"])
		total, totalOK := wholeNumber(fields["total"])
		if !doneOK || !totalOK || done < 0 || total < 1 || done > total {
			return fmt.Errorf("a step line needs whole numbers with 0 <= done <= total and total >= 1: %s", line)
		}
		return onlyFields(fields, line, "phase", "done", "total")
	case phase == "download":
		bytes, bytesOK := wholeNumber(fields["bytes"])
		totalBytes, totalOK := wholeNumber(fields["totalBytes"])
		if !bytesOK || !totalOK || bytes < 0 || bytes > totalBytes {
			return fmt.Errorf("a download line needs whole numbers with 0 <= bytes <= totalBytes: %s", line)
		}
		return onlyFields(fields, line, "phase", "bytes", "totalBytes")
	case phase == "done":
		succeeded, isBool := fields["ok"].(bool)
		if !isBool {
			return fmt.Errorf(`a done line needs a boolean "ok": %s`, line)
		}
		if succeeded {
			if _, hasError := fields["error"]; hasError {
				return fmt.Errorf(`a successful done line has no "error": %s`, line)
			}
			return nil
		}
		errorObject, isObject := fields["error"].(map[string]any)
		code, _ := errorObject["code"].(string)
		message, _ := errorObject["message"].(string)
		if !isObject || code == "" || message == "" {
			return fmt.Errorf(`a failed done line needs "error" with a code and a message: %s`, line)
		}
		return onlyFields(fields, line, "phase", "ok", "error")
	default:
		return fmt.Errorf("unknown phase %q: %s", phase, line)
	}
}

func wholeNumber(value any) (float64, bool) {
	number, ok := value.(float64)
	return number, ok && number == math.Trunc(number)
}

func onlyFields(fields map[string]any, line string, allowed ...string) error {
	for name := range fields {
		found := false
		for _, allowedName := range allowed {
			if name == allowedName {
				found = true
			}
		}
		if !found {
			return fmt.Errorf("unexpected field %q: %s", name, line)
		}
	}
	return nil
}

// checkProgressOutput checks every JSON line in stderr against the schema,
// and that the last one is the "done" line. Lines that do not start with
// "{" are human log text, which --progress json leaves on stderr.
func checkProgressOutput(t *testing.T, stderr string) []map[string]any {
	t.Helper()
	var lines []map[string]any
	for _, line := range strings.Split(stderr, "\n") {
		if !strings.HasPrefix(line, "{") {
			continue
		}
		if err := checkProgressLine(line); err != nil {
			t.Errorf("progress line fails the schema: %v", err)
			continue
		}
		var fields map[string]any
		_ = json.Unmarshal([]byte(line), &fields)
		lines = append(lines, fields)
	}
	if len(lines) == 0 || lines[len(lines)-1]["phase"] != "done" {
		t.Fatalf("the last progress line is not the done line:\n%s", stderr)
	}
	return lines
}

func TestCheckProgressLineAcceptsTheDocumentedShapes(t *testing.T) {
	for _, line := range []string{
		`{"phase":"render","done":7,"total":14}`,
		`{"phase":"parse","done":2,"total":4}`,
		`{"phase":"download","bytes":10,"totalBytes":100}`,
		`{"phase":"done","ok":true,"output":"talk.pdf","pages":3}`,
		`{"phase":"done","ok":false,"error":{"code":"deck_not_found","message":"no deck"}}`,
	} {
		if err := checkProgressLine(line); err != nil {
			t.Errorf("checkProgressLine(%s) error = %v", line, err)
		}
	}
}

func TestCheckProgressLineRejectsOtherShapes(t *testing.T) {
	for _, line := range []string{
		`not json`,
		`{"done":1,"total":2}`,
		`{"phase":"render","done":3,"total":2}`,
		`{"phase":"render","done":1.5,"total":2}`,
		`{"phase":"render","done":1,"total":2,"extra":true}`,
		`{"phase":"download","bytes":200,"totalBytes":100}`,
		`{"phase":"done"}`,
		`{"phase":"done","ok":false}`,
		`{"phase":"done","ok":true,"error":{"code":"x","message":"y"}}`,
		`{"phase":"thinking","done":1,"total":1}`,
	} {
		if err := checkProgressLine(line); err == nil {
			t.Errorf("checkProgressLine(%s) = nil, want an error", line)
		}
	}
}
```

`internal/cli/progress_test.go`:

```go
package cli

import (
	"bytes"
	"errors"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestProgressReporterWritesTheDocumentedLines(t *testing.T) {
	var output bytes.Buffer
	reporter, err := newProgressReporter("json", &output)
	if err != nil {
		t.Fatalf("newProgressReporter() error = %v", err)
	}

	reporter.Step(progressPhaseParse, 2, 4)
	reporter.Download(10, 100)
	reporter.Render(1, 3)
	if err := reporter.Result(struct {
		Output string `json:"output"`
		Pages  int    `json:"pages"`
	}{Output: "talk.pdf", Pages: 3}); err != nil {
		t.Fatalf("Result() error = %v", err)
	}

	want := strings.Join([]string{
		`{"phase":"parse","done":2,"total":4}`,
		`{"phase":"download","bytes":10,"totalBytes":100}`,
		`{"phase":"render","done":1,"total":3}`,
		`{"phase":"done","ok":true,"output":"talk.pdf","pages":3}`,
	}, "\n") + "\n"
	if output.String() != want {
		t.Errorf("output:\n%s\nwant:\n%s", output.String(), want)
	}
	checkProgressOutput(t, output.String())
}

func TestProgressReporterResultWithNoPayload(t *testing.T) {
	var output bytes.Buffer
	reporter, _ := newProgressReporter("json", &output)
	if err := reporter.Result(nil); err != nil {
		t.Fatalf("Result(nil) error = %v", err)
	}
	if output.String() != `{"phase":"done","ok":true}`+"\n" {
		t.Errorf("output = %q", output.String())
	}
}

func TestProgressReporterWithoutProgressWritesNothing(t *testing.T) {
	var output bytes.Buffer
	reporter, err := newProgressReporter("", &output)
	if err != nil {
		t.Fatalf("newProgressReporter() error = %v", err)
	}
	reporter.Step(progressPhaseRender, 1, 1)
	reporter.Download(1, 2)
	_ = reporter.Result(struct{}{})
	if reporter.enabled() || output.Len() != 0 {
		t.Errorf("a reporter without --progress wrote %q", output.String())
	}
}

func TestNewProgressReporterRejectsAnUnknownFormat(t *testing.T) {
	_, err := newProgressReporter("xml", &bytes.Buffer{})
	if err == nil {
		t.Fatal("newProgressReporter(xml) = nil error")
	}
	if exitCode, code, _ := classify(err); exitCode != exitUserError || code != codeUsage {
		t.Errorf("classify() = (%d, %q), want (%d, %q)", exitCode, code, exitUserError, codeUsage)
	}
}

func TestExecuteEndsAFailedProgressRunWithADoneLine(t *testing.T) {
	root := &cobra.Command{Use: "tap", SilenceErrors: true, SilenceUsage: true}
	var format string
	work := &cobra.Command{
		Use: "work",
		RunE: func(*cobra.Command, []string) error {
			return userError(codeDeckNotFound, errors.New("no deck here"))
		},
	}
	work.Flags().StringVar(&format, "progress", "", "")
	root.AddCommand(work)

	var stdout, stderr bytes.Buffer
	exitCode := execute(root, []string{"work", "--progress", "json"}, &stdout, &stderr)

	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
	}
	want := `{"phase":"done","ok":false,"error":{"code":"deck_not_found","message":"no deck here"}}` + "\n"
	if stderr.String() != want {
		t.Errorf("stderr = %q, want only the done line %q", stderr.String(), want)
	}
	checkProgressOutput(t, stderr.String())
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestCheckProgressLine|TestProgressReporter|TestNewProgressReporter|TestExecuteEndsAFailedProgressRun' -short`
Expected: compile failure, `undefined: newProgressReporter`.

- [ ] **Step 3: Write `internal/cli/progress.go`**

```go
package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"sync"

	"github.com/spf13/cobra"
)

// progressFormatJSON is the one --progress format: one JSON object per
// line on stderr.
const progressFormatJSON = "json"

// The phase of each --progress json line.
const (
	progressPhaseDownload = "download"
	progressPhaseRender   = "render"
	progressPhaseLoad     = "load"
	progressPhaseParse    = "parse"
	progressPhaseBundle   = "bundle"
	progressPhaseWrite    = "write"
	progressPhaseDone     = "done"
)

// progressReporter writes --progress json lines: a step line
// ({"phase","done","total"}) for each step, a download line
// ({"phase":"download","bytes","totalBytes"}) while the export browser
// downloads, and one final "done" line with the command's result. A
// reporter for a command run without --progress writes nothing, so callers
// never check first. A failed command's "done" line comes from execute
// (see writeProgressFailure).
type progressReporter struct {
	writer io.Writer
	mu     sync.Mutex
}

// newProgressReporter returns the reporter for a --progress value.
func newProgressReporter(format string, writer io.Writer) (*progressReporter, error) {
	switch format {
	case "":
		return &progressReporter{}, nil
	case progressFormatJSON:
		return &progressReporter{writer: writer}, nil
	default:
		return nil, userError(codeUsage, fmt.Errorf("unknown --progress format %q: the only format is json", format))
	}
}

// enabled reports whether this reporter writes anything.
func (p *progressReporter) enabled() bool {
	return p != nil && p.writer != nil
}

// progressStep is a step line.
type progressStep struct {
	Phase string `json:"phase"`
	Done  int    `json:"done"`
	Total int    `json:"total"`
}

// progressDownload is a download line.
type progressDownload struct {
	Phase      string `json:"phase"`
	Bytes      int64  `json:"bytes"`
	TotalBytes int64  `json:"totalBytes"`
}

// progressFailure is the "done" line of a command that failed.
type progressFailure struct {
	Phase string    `json:"phase"`
	OK    bool      `json:"ok"`
	Error jsonError `json:"error"`
}

// Step reports that done of total units of phase are finished.
func (p *progressReporter) Step(phase string, done, total int) {
	p.writeValue(progressStep{Phase: phase, Done: done, Total: total})
}

// Render reports that done of total slides or pages are rendered. It lets
// a reporter serve as the export engine's pdf.Progress.
func (p *progressReporter) Render(done, total int) {
	p.Step(progressPhaseRender, done, total)
}

// Download reports the export browser's first-time download.
func (p *progressReporter) Download(downloaded, totalBytes int64) {
	p.writeValue(progressDownload{Phase: progressPhaseDownload, Bytes: downloaded, TotalBytes: totalBytes})
}

// Result writes the final line of a successful command:
// {"phase":"done","ok":true} followed by the fields of payload, the same
// fields as the command's --json result. payload must encode to a JSON
// object, or be nil.
func (p *progressReporter) Result(payload any) error {
	if !p.enabled() {
		return nil
	}
	body := []byte("{}")
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return internalError(codeInternal, fmt.Errorf("encoding the progress result: %w", err))
		}
		body = encoded
	}
	if len(body) < 2 || body[0] != '{' {
		return internalError(codeInternal, fmt.Errorf("the progress result must be an object, got %s", body))
	}

	var line bytes.Buffer
	line.WriteString(`{"phase":"done","ok":true`)
	if rest := body[1:]; string(rest) == "}" {
		line.WriteByte('}')
	} else {
		line.WriteByte(',')
		line.Write(rest)
	}
	p.writeLine(line.Bytes())
	return nil
}

func (p *progressReporter) writeValue(value any) {
	if !p.enabled() {
		return
	}
	encoded, err := json.Marshal(value)
	if err != nil {
		return
	}
	p.writeLine(encoded)
}

func (p *progressReporter) writeLine(line []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()
	_, _ = p.writer.Write(append(line, '\n'))
}

// progressRequested reports whether command was run with --progress json.
func progressRequested(command *cobra.Command) bool {
	if command == nil {
		return false
	}
	flag := command.Flags().Lookup("progress")
	return flag != nil && flag.Value.String() == progressFormatJSON
}

// writeProgressFailure writes the final line of a failed command.
func writeProgressFailure(writer io.Writer, code, message string) {
	encoded, err := json.Marshal(progressFailure{Phase: progressPhaseDone, Error: jsonError{Code: code, Message: message}})
	if err != nil {
		return
	}
	_, _ = writer.Write(append(encoded, '\n'))
}
```

- [ ] **Step 4: End a failed progress run with the done line**

In `internal/cli/root.go`, in `execute`, right after `exitCode, code, reported := classify(err)`, add:

```go
	// A --progress json run ends with one "done" line on stderr, so a
	// program reading the lines learns the outcome without parsing text.
	progress := progressRequested(command)
	if progress {
		writeProgressFailure(stderr, code, err.Error())
	}
```

and right after the `if jsonRequested(command) { ... }` block, add:

```go
	if progress {
		return exitCode
	}
```

so the human `Error:` line is not printed next to the done line.

- [ ] **Step 5: Add the flag to the conventions**

In `internal/cli/conventions_test.go`, add `"progress": "",` to `flagConventions`, so `--progress` has no short form on any command.

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./internal/cli -run 'TestCheckProgressLine|TestProgressReporter|TestNewProgressReporter|TestExecute|TestEveryCommandFollows' -short -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/progress.go internal/cli/progress_test.go internal/cli/progress_schema_test.go internal/cli/root.go internal/cli/conventions_test.go
git commit -m "feat(cli): add the --progress json reporter"
```

---

### Task 14: Progress from the export engine

**Files:**
- Create: `internal/pdf/progress.go`
- Create: `internal/pdf/progress_test.go`
- Modify: `internal/pdf/exporter.go` (`Exporter`, `launchBrowser`, the three export loops)

**Interfaces:**
- Produces:
  - `type Progress interface { Download(downloaded, totalBytes int64); Render(done, total int) }`
  - `func (e *Exporter) SetProgress(progress Progress)`
  - The PDF export calls `Render(n, slideCount)` after each slide or notes page. `launchBrowser` reports Playwright's download lines through `Download` and keeps Playwright's own text off stdout and stderr, when a `Progress` is set.
- Consumed by: Task 15, where `*progressReporter` is the `Progress`

- [ ] **Step 1: Write the failing tests**

`internal/pdf/progress_test.go`:

```go
package pdf

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/MiniCodeMonkey/tap/internal/config"
	"github.com/MiniCodeMonkey/tap/internal/server"
	"github.com/MiniCodeMonkey/tap/internal/transformer"
)

type recordedProgress struct {
	mu        sync.Mutex
	downloads []string
	renders   []string
}

func (r *recordedProgress) Download(downloaded, totalBytes int64) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.downloads = append(r.downloads, fmt.Sprintf("%d/%d", downloaded, totalBytes))
}

func (r *recordedProgress) Render(done, total int) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.renders = append(r.renders, fmt.Sprintf("%d/%d", done, total))
}

func TestInstallProgressWriterReportsPlaywrightDownloadLines(t *testing.T) {
	progress := &recordedProgress{}
	writer := &installProgressWriter{progress: progress}

	// Playwright's installer output when stdout is not a terminal, split
	// in the middle of a line the way a pipe can deliver it.
	chunks := []string{
		"Downloading Chromium 131.0.6778.33 (playwright build v1148) from https://playwright.azureedge.net/builds/chromium/1148/chromium-mac-arm64.zip\n",
		"|■■■■■■■■                                                                        |  10% of 162.3 MiB\n|■■■■■■■■■■■■",
		"■■■■                                                                |  20% of 162.3 MiB\r\n",
		"Chromium 131.0.6778.33 (playwright build v1148) downloaded to /Users/someone/Library/Caches/ms-playwright/chromium-1148\n",
	}
	for _, chunk := range chunks {
		if _, err := writer.Write([]byte(chunk)); err != nil {
			t.Fatalf("Write() error = %v", err)
		}
	}

	totalBytes := int64(162.3 * 1024 * 1024)
	want := []string{
		fmt.Sprintf("%d/%d", totalBytes*10/100, totalBytes),
		fmt.Sprintf("%d/%d", totalBytes*20/100, totalBytes),
	}
	if fmt.Sprint(progress.downloads) != fmt.Sprint(want) {
		t.Errorf("downloads = %v, want %v", progress.downloads, want)
	}
}

func TestInstallOptions(t *testing.T) {
	exporter, _ := New()
	if options := exporter.installOptions(); options.Stdout != nil || options.Stderr != nil {
		t.Error("without a Progress, the installer should keep its default output")
	}

	exporter.SetProgress(&recordedProgress{})
	options := exporter.installOptions()
	if _, ok := options.Stdout.(*installProgressWriter); !ok {
		t.Errorf("Stdout = %T, want *installProgressWriter", options.Stdout)
	}
	if options.Stderr != io.Discard || options.Logger == nil {
		t.Error("with a Progress, the installer's own text must stay off stderr")
	}
	if len(options.Browsers) != 1 || options.Browsers[0] != "chromium" {
		t.Errorf("Browsers = %v, want chromium", options.Browsers)
	}
}

func TestExportReportsRenderProgress(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}
	presentation := &transformer.TransformedPresentation{
		Config: *config.DefaultConfig(),
		Slides: []transformer.TransformedSlide{
			{Index: 0, HTML: "<h1>Slide 1</h1>", Layout: "title"},
			{Index: 1, HTML: "<h1>Slide 2</h1>", Layout: "default"},
		},
	}
	srv := server.New(0)
	srv.SetPresentation(presentation)
	srv.SetupRoutes()
	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Shutdown(context.Background())

	exporter, err := New()
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer exporter.Close()
	progress := &recordedProgress{}
	exporter.SetProgress(progress)

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	output := filepath.Join(t.TempDir(), "progress.pdf")
	if _, err := exporter.Export(ctx, "http://localhost:"+itoa(srv.Port()), ExportOptions{Content: ContentSlides, Output: output}); err != nil {
		if os.Getenv("CI") == "" {
			t.Skipf("skipping: export failed, likely no browser: %v", err)
		}
		t.Fatalf("Export() error = %v", err)
	}

	if fmt.Sprint(progress.renders) != "[1/2 2/2]" {
		t.Errorf("renders = %v, want [1/2 2/2]", progress.renders)
	}
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/pdf -run 'TestInstallProgressWriter|TestInstallOptions|TestExportReportsRenderProgress' -v`
Expected: compile failure, `undefined: installProgressWriter`.

- [ ] **Step 3: Write `internal/pdf/progress.go`**

```go
package pdf

import (
	"bytes"
	"io"
	"log/slog"
	"regexp"
	"strconv"

	"github.com/mxschmitt/playwright-go"
)

// Progress receives an export's progress. tap's --progress json reporter
// implements it.
type Progress interface {
	// Download reports the export browser's first-time download:
	// downloaded of totalBytes bytes of the archive being fetched.
	Download(downloaded, totalBytes int64)
	// Render reports that done of total slides or pages are rendered.
	Render(done, total int)
}

// SetProgress makes the exporter report to progress. A nil progress
// reports nothing.
func (e *Exporter) SetProgress(progress Progress) {
	e.progress = progress
}

// reportRender tells the Progress, if any, that done of total are rendered.
func (e *Exporter) reportRender(done, total int) {
	if e.progress != nil {
		e.progress.Render(done, total)
	}
}

// installOptions are the options for installing the export browser. With a
// Progress, the installer's download lines go to installProgressWriter and
// the rest of its text is dropped, so stdout keeps only the command's
// result and stderr only progress lines.
func (e *Exporter) installOptions() *playwright.RunOptions {
	options := &playwright.RunOptions{Browsers: []string{"chromium"}}
	if e.progress != nil {
		options.Stdout = &installProgressWriter{progress: e.progress}
		options.Stderr = io.Discard
		options.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return options
}

// installProgressLine matches the download lines Playwright's installer
// prints when its output is not a terminal, such as
// "|■■■■■■■■      |  10% of 162.3 MiB". One archive downloads at a time, so
// the percentage restarts for each one.
var installProgressLine = regexp.MustCompile(`(\d+)% of ([\d.]+) MiB`)

// installProgressWriter turns the installer's output into Download calls.
// Lines can arrive split across writes, so it keeps the unfinished line.
type installProgressWriter struct {
	progress Progress
	pending  []byte
}

func (w *installProgressWriter) Write(data []byte) (int, error) {
	w.pending = append(w.pending, data...)
	for {
		end := bytes.IndexAny(w.pending, "\r\n")
		if end < 0 {
			break
		}
		w.report(string(w.pending[:end]))
		w.pending = w.pending[end+1:]
	}
	return len(data), nil
}

func (w *installProgressWriter) report(line string) {
	match := installProgressLine.FindStringSubmatch(line)
	if match == nil {
		return
	}
	percent, err := strconv.Atoi(match[1])
	if err != nil {
		return
	}
	mebibytes, err := strconv.ParseFloat(match[2], 64)
	if err != nil {
		return
	}
	totalBytes := int64(mebibytes * 1024 * 1024)
	w.progress.Download(totalBytes*int64(percent)/100, totalBytes)
}
```

- [ ] **Step 4: Wire it into `exporter.go`**

1. Add `progress Progress` to the `Exporter` struct, with the comment `// progress receives download and render progress; nil reports nothing.`
2. In `launchBrowser`, replace

```go
	err := playwright.Install(&playwright.RunOptions{
		Browsers: []string{"chromium"},
	})
```

with `err := playwright.Install(e.installOptions())`.

3. In `exportSlides`, after `screenshotPaths = append(screenshotPaths, screenshotPath)`, add `e.reportRender(i+1, slideCount)`. In `exportNotes`, after `allNotes = append(allNotes, noteText)`, add the same line. In `exportBoth`, after its `screenshotPaths = append(...)`, add the same line.

- [ ] **Step 5: Run the tests to verify they pass**

Run: `make frontend && go test ./internal/pdf -run 'TestInstallProgressWriter|TestInstallOptions|TestExportReportsRenderProgress' -v`
Expected: PASS (the export test may SKIP with no browser outside CI).

Run: `go test ./internal/pdf -short`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/pdf/progress.go internal/pdf/progress_test.go internal/pdf/exporter.go
git commit -m "feat(pdf): report download and render progress"
```

---

### Task 15: `--progress json` on `export pdf`, `export images` and `build`

**Files:**
- Modify: `internal/cli/export_pdf.go`
- Modify: `internal/cli/export_images.go`
- Modify: `internal/cli/build.go`
- Create: `internal/cli/progress_commands_test.go`

**Interfaces:**
- Consumes: `newProgressReporter`, `progressReporter` and its methods, the phase constants, `checkProgressOutput` (Task 13); `pdf.Progress`, `(*pdf.Exporter).SetProgress` (Task 14); `exportPDFResult`, `printWrittenImages`, `captureFunc`, `captureAllSlides` (P1 and existing)
- Produces:
  - `--progress` on the three commands (no short form), value `json`
  - `type exportImagesResult struct { Files []string }` (JSON `files`) and `type buildResultJSON struct { Output string; Files int; Bytes int64 }` (JSON `output`, `files`, `bytes`)
  - `func countCaptures(capture captureFunc, progress *progressReporter, total int) captureFunc`
  - `const buildSteps = 4`
  - Lines: `export pdf` and `export images` write one `render` line per slide (or notes page), `build` writes `load`, `parse`, `bundle`, `write` with `total` 4, and each ends with the `done` line, whose fields are the command's `--json` result.

- [ ] **Step 1: Write the failing tests**

`internal/cli/progress_commands_test.go`:

```go
package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const progressDeck = "---\ntitle: Progress\n---\n\n# One\n\n---\n\n# Two\n"

func writeProgressDeck(t *testing.T) string {
	t.Helper()
	deckPath := filepath.Join(t.TempDir(), "progress.md")
	if err := os.WriteFile(deckPath, []byte(progressDeck), 0o644); err != nil {
		t.Fatal(err)
	}
	return deckPath
}

func phasesOf(lines []map[string]any) string {
	var phases []string
	for _, line := range lines {
		phases = append(phases, line["phase"].(string))
	}
	return strings.Join(phases, ",")
}

func TestProgressFlagOnTheLongRunningCommands(t *testing.T) {
	for _, path := range [][]string{{"export", "pdf"}, {"export", "images"}, {"build"}} {
		command, _, err := rootCmd.Find(path)
		if err != nil {
			t.Fatalf("%v not found: %v", path, err)
		}
		if command.Flags().Lookup("progress") == nil {
			t.Errorf("tap %s has no --progress", strings.Join(path, " "))
		}
	}
}

func TestBuildProgressJSON(t *testing.T) {
	deckPath := writeProgressDeck(t)
	outputDir := filepath.Join(t.TempDir(), "dist")

	exitCode, _, stderr := runTap(t, "build", deckPath, "--output", outputDir, "--progress", "json")
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}

	lines := checkProgressOutput(t, stderr)
	if phasesOf(lines) != "load,parse,bundle,write,done" {
		t.Errorf("phases = %s, want load,parse,bundle,write,done", phasesOf(lines))
	}
	done := lines[len(lines)-1]
	if done["ok"] != true || done["output"] != outputDir {
		t.Errorf("done line = %v, want ok and output %q", done, outputDir)
	}
	if files, _ := done["files"].(float64); files < 1 {
		t.Errorf("done line files = %v, want at least 1", done["files"])
	}
}

func TestBuildProgressJSONFailure(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing.md")

	exitCode, stdout, stderr := runTap(t, "build", missing, "--progress", "json")
	if exitCode != exitUserError {
		t.Errorf("exit code = %d, want %d", exitCode, exitUserError)
	}
	lines := checkProgressOutput(t, stderr)
	done := lines[len(lines)-1]
	errorObject, _ := done["error"].(map[string]any)
	if done["ok"] != false || errorObject["code"] != "deck_not_found" {
		t.Errorf("done line = %v, want a deck_not_found failure", done)
	}
	if strings.Contains(stderr, "Error:") {
		t.Errorf("stderr has the human error line next to the done line: %q", stderr)
	}
	if stdout != "" {
		t.Errorf("stdout = %q, want nothing", stdout)
	}
}

func TestProgressRejectsAnUnknownFormat(t *testing.T) {
	deckPath := writeProgressDeck(t)
	exitCode, _, stderr := runTap(t, "build", deckPath, "--progress", "xml")
	if exitCode != exitUserError || !strings.Contains(stderr, "the only format is json") {
		t.Errorf("exit %d, stderr %q", exitCode, stderr)
	}
}

func TestExportPDFProgressJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}
	deckPath := writeProgressDeck(t)
	outputPath := filepath.Join(t.TempDir(), "progress.pdf")

	exitCode, _, stderr := runTap(t, "export", "pdf", deckPath, "--output", outputPath, "--progress", "json")
	if exitCode == exitInternal && os.Getenv("CI") == "" {
		t.Skipf("skipping: no browser: %s", stderr)
	}
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}

	lines := checkProgressOutput(t, stderr)
	var renders []string
	for _, line := range lines {
		if line["phase"] == "render" {
			// fmt prints a whole float64 such as 1 as "1".
			renders = append(renders, fmt.Sprintf("%v/%v", line["done"], line["total"]))
		}
	}
	if strings.Join(renders, " ") != "1/2 2/2" {
		t.Errorf("render lines = %v, want 1/2 2/2", renders)
	}
	done := lines[len(lines)-1]
	if done["ok"] != true || done["pages"] != float64(2) || done["output"] != outputPath {
		t.Errorf("done line = %v, want ok, 2 pages and output %q", done, outputPath)
	}
}

func TestExportImagesProgressJSON(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping browser test in short mode")
	}
	deckPath := writeProgressDeck(t)
	outputDir := filepath.Join(t.TempDir(), "images")

	exitCode, _, stderr := runTap(t, "export", "images", deckPath, "--all", "--output", outputDir, "--progress", "json")
	if exitCode == exitInternal && os.Getenv("CI") == "" {
		t.Skipf("skipping: no browser: %s", stderr)
	}
	if exitCode != exitOK {
		t.Fatalf("exit code = %d, stderr %q", exitCode, stderr)
	}

	lines := checkProgressOutput(t, stderr)
	if phasesOf(lines) != "render,render,done" {
		t.Errorf("phases = %s, want render,render,done", phasesOf(lines))
	}
	files, _ := lines[len(lines)-1]["files"].([]any)
	if len(files) != 2 {
		t.Errorf("done line files = %v, want 2 files", lines[len(lines)-1]["files"])
	}
}

```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `go test ./internal/cli -run 'TestProgressFlagOnTheLongRunningCommands|TestBuildProgressJSON|TestProgressRejectsAnUnknownFormat' -short -v`
Expected: FAIL, `tap export pdf has no --progress` and `unknown flag: --progress`.

- [ ] **Step 3: `tap export pdf`**

In `internal/cli/export_pdf.go`:

1. Add `pdfProgress string` to the flag `var` block, and in `init`:

```go
	exportPDFCmd.Flags().StringVar(&pdfProgress, "progress", "", "print progress to stderr as JSON lines (json)")
```

2. Add `tap export pdf slides.md --progress json   # Progress as JSON lines on stderr` to the examples in `Long`.
3. At the start of `runExportPDF`, before `resolveDeck`:

```go
	progress, err := newProgressReporter(pdfProgress, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
```

The next line, `file, err := resolveDeck(firstArg(args))`, still compiles, because `file` is new.

4. After the `exporter, err := pdf.New()` error check, add:

```go
	if progress.enabled() {
		exporter.SetProgress(progress)
	}
```

5. Replace the `if pdfJSON { ... }` block that P1 added with one result value used by both outputs:

```go
	brokenSlides := make([]brokenSlideJSON, 0, len(result.BrokenSlides))
	for _, broken := range result.BrokenSlides {
		brokenSlides = append(brokenSlides, brokenSlideJSON{Slide: broken.SlideNumber, Message: broken.Message})
	}
	jsonResult := exportPDFResult{
		Output:       result.OutputPath,
		Pages:        result.PageCount,
		Bytes:        result.FileSize,
		BrokenSlides: brokenSlides,
	}
	if err := progress.Result(jsonResult); err != nil {
		return err
	}
	if pdfJSON {
		return printJSONOK(cmd.OutOrStdout(), jsonResult)
	}
```

- [ ] **Step 4: `tap export images`**

In `internal/cli/export_images.go`:

1. Add `screenshotProgress string` to the flag `var` block, and in `init`:

```go
	exportImagesCmd.Flags().StringVar(&screenshotProgress, "progress", "", "print progress to stderr as JSON lines (json)")
```

2. Add `tap export images deck.md --all --progress json   # Progress as JSON lines on stderr` to the examples.
3. At the start of `runExportImages`, before the flag checks:

```go
	progress, err := newProgressReporter(screenshotProgress, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
```

(If a later line in the function declares `err` with `:=` in the same scope and nothing else new, change it to `=`; `go build` names the line.)

4. After the `exporter, err := pdf.New()` error check, add `if progress.enabled() { exporter.SetProgress(progress) }` (on three lines, gofmt style).
5. Add, next to `captureFunc`:

```go
// exportImagesResult is the --json and progress result of tap export images.
type exportImagesResult struct {
	Files []string `json:"files"`
}

// countCaptures wraps capture so that each finished capture, whether it
// worked or not, reports render progress: done of total.
func countCaptures(capture captureFunc, progress *progressReporter, total int) captureFunc {
	done := 0
	return func(ctx context.Context, serverURL string, options pdf.CaptureOptions, outputPath string) error {
		err := capture(ctx, serverURL, options, outputPath)
		done++
		progress.Render(done, total)
		return err
	}
}
```

6. In the `--all` path, change `captureAllSlides(ctx, exporter.CaptureSlide, ...)` to `captureAllSlides(ctx, countCaptures(exporter.CaptureSlide, progress, total), ...)`.
7. In the single-slide path, after the `exporter.CaptureSlide(ctx, serverURL, options, outputPath)` error check, add `progress.Render(1, 1)`.
8. In `printWrittenImages`, replace the anonymous struct with `exportImagesResult{Files: paths}`.
9. At each of the two successful ends (the single slide, and `--all` with no broken slides), right before `return printWrittenImages(...)`, add:

```go
	if err := progress.Result(exportImagesResult{Files: paths}); err != nil {
		return err
	}
```

using the same slice the `printWrittenImages` call gets (`[]string{outputPath}` for one slide, `written` for `--all`). A run with broken slides returns `reportedError`, and `execute` writes the failure line.

- [ ] **Step 5: `tap build`**

In `internal/cli/build.go`:

1. Add `buildProgress string` to the flag `var` block, and in `init`:

```go
	buildCmd.Flags().StringVar(&buildProgress, "progress", "", "print progress to stderr as JSON lines (json)")
```

2. Add `tap build talk.md --progress json      # Progress as JSON lines on stderr` to the examples.
3. Add above `runBuild`:

```go
// buildSteps is the number of --progress json steps tap build reports:
// load, parse, bundle and write.
const buildSteps = 4

// buildResultJSON is the --json and progress result of tap build.
type buildResultJSON struct {
	Output string `json:"output"`
	Files  int    `json:"files"`
	Bytes  int64  `json:"bytes"`
}
```

(Use the integer types of `builder.BuildResult`'s `FileCount` and `TotalSize` for `Files` and `Bytes`, as P1's anonymous struct did.)

4. At the start of `runBuild`, before `resolveDeck`:

```go
	progress, err := newProgressReporter(buildProgress, cmd.ErrOrStderr())
	if err != nil {
		return err
	}
```

5. Right after `spinner := newSpinner("Building presentation")`, add:

```go
	if progress.enabled() {
		// Progress lines replace the spinner on stderr.
		spinner.isTerminal = func() bool { return false }
	}
```

6. Report each step when it is done:
   - after the `cfg.Validate()` check: `progress.Step(progressPhaseLoad, 1, buildSteps)`
   - after the `p.Parse(content)` check: `progress.Step(progressPhaseParse, 2, buildSteps)`
   - after the `layouts.Validate` check passes: `progress.Step(progressPhaseBundle, 3, buildSteps)`
   - after the `b.Build(cfg, pres)` check: `progress.Step(progressPhaseWrite, 4, buildSteps)`
7. Replace P1's `if buildJSON { return printJSONOK(..., struct{...}{...}) }` block with:

```go
	jsonResult := buildResultJSON{Output: result.OutputDir, Files: result.FileCount, Bytes: result.TotalSize}
	if err := progress.Result(jsonResult); err != nil {
		return err
	}
	if buildJSON {
		return printJSONOK(cmd.OutOrStdout(), jsonResult)
	}
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go build ./... && go vet ./internal/cli`
Expected: no output.

Run: `make frontend && go test ./internal/cli -run 'TestProgress|TestBuildProgressJSON|TestExportPDFProgressJSON|TestExportImagesProgressJSON|TestEveryCommandFollows|TestExportPDF|TestExportImages|TestBuild' -v`
Expected: PASS (browser tests may SKIP with no browser outside CI).

Try the download line by hand once, if you can spare the download: `PLAYWRIGHT_BROWSERS_PATH="$TMPDIR/empty-browsers" ./bin/tap export pdf examples/basic.md --output "$TMPDIR/basic.pdf" --progress json 2>&1 >/dev/null | head -20` (after `make build`). Expected: `{"phase":"download",...}` lines, then `render` lines and the `done` line. Note what you saw in the PR.

- [ ] **Step 7: Commit**

```bash
git add internal/cli/export_pdf.go internal/cli/export_images.go internal/cli/build.go internal/cli/progress_commands_test.go
git commit -m "feat(cli): add --progress json to export pdf, export images and build"
```

---

### Task 16: Docs, skill and changelog

**Files:**
- Modify: `docs/reference/cli-commands.md`
- Modify: `docs/guide/building-export.md`
- Modify: `docs/guide/custom-components.md` (the "Any file change under the deck folder reloads the page" paragraph)
- Modify: `skills/tap/rules/cli.md`
- Modify: `CHANGELOG.md`, `docs/changelog.md`

P1 rewrote `cli-commands.md` and `skills/tap/rules/cli.md` around the new command tree and added a "Conventions" section to both. Add to those sections; do not restructure them. Do not edit `docs/superpowers/**` or released changelog sections.

- [ ] **Step 1: `docs/reference/cli-commands.md`**

1. In the flag tables of `tap export pdf`, `tap export images` and `tap build`, add a row:

| Flag | Default | Description |
|---|---|---|
| `--progress json` | none | Print progress to stderr as JSON lines, for a program driving tap (see Progress output). |

2. In the "Conventions" section, add a subsection:

```markdown
### Progress output

`tap export pdf`, `tap export images` and `tap build` accept `--progress json`. Each step prints one JSON object on its own line to stderr:

    {"phase":"render","done":7,"total":14}

- `export pdf` and `export images` print one `render` line per slide (per notes page for `--content notes`).
- `build` prints `load`, `parse`, `bundle` and `write`, with `total` 4.
- A first export downloads the export browser and prints `{"phase":"download","bytes":52428800,"totalBytes":170175488}` lines while it does. Each downloaded archive starts again from 0.
- The last line is `{"phase":"done","ok":true, ...}` with the same fields as the command's `--json` result, or `{"phase":"done","ok":false,"error":{"code":"...","message":"..."}}`.

stdout keeps the command's normal output (or its `--json` result). Warnings can still appear on stderr as plain text; read only the lines that start with `{`.
```

3. In `tap dev`'s "Features" list, replace the "Live reload" item with:

```markdown
- **Live updates**: Saving the deck updates every open page in place. Only the slides you changed re-render, and each page keeps its slide, fragment and step (moved back if the slide lost steps). A changed custom theme file, or `r` in the terminal, reloads the page instead.
```

4. Add a short section at the end of the "Conventions" section:

```markdown
### Ready signal

Every tap page reports when the slide on screen has finished rendering: fonts and images loaded, maps drawn, components loaded, error cards shown, and transitions and theme animations done. It sets `window.__tapReady` to `{"revision": "...", "slide": 3, "step": 1}` (`slide` counts from 1), dispatches a `tap:ready` event on `window` with the same object, and, inside a macOS web view that registered a `tapReady` message handler, posts it to that handler. `window.__tapReady` is `null` while a slide is still rendering, and resets when the slide, step, fragment, theme or deck changes. `tap export pdf` and `tap export images` wait for this signal before each capture.
```

- [ ] **Step 2: `docs/guide/building-export.md`**

1. In the PDF Export section, after the options table, add:

```markdown
### How tap knows a slide is ready

Before it captures a slide, the export waits until the page reports that the slide has finished rendering: fonts and images are loaded, maps have drawn their tiles, components have loaded (or shown their error card), and slide transitions and theme animations are done. A slide gets 30 seconds. See "Ready signal" in the CLI reference for how other tools can wait for the same signal.
```

2. In the section on screenshotting one slide (P1 renamed it for `tap export images`), add one sentence: "The capture waits for the same ready signal as a PDF export."
3. Add a section before "Best Practices":

```markdown
## Progress for Scripts and Apps

Add `--progress json` to `tap export pdf`, `tap export images` or `tap build` to get one JSON line per step on stderr, and a final line with the result:

    tap export pdf talk.md --output talk.pdf --progress json

See "Progress output" in the CLI reference for every line's fields.
```

- [ ] **Step 3: `docs/guide/custom-components.md`**

Replace the paragraph that starts "**Any file change under the deck folder reloads the page**" with:

```markdown
**A file change under the deck folder updates the page in place.** A slide whose content did not change keeps its component mounted, with its state and its step. A component whose file changed gets a new bundle and mounts again, which restarts its animations. A change to the custom theme file reloads the whole page. Write screenshots and scratch files **outside** the deck folder, or into a dot folder or a folder named `dist`: the watcher skips both. It also skips the recordings folder and Finder's `.DS_Store` files.
```

Keep the code block under it (P1 already renamed its commands to `tap export images`).

- [ ] **Step 4: `skills/tap/rules/cli.md`**

1. Add `--progress json` to the flag lists of `tap export pdf`, `tap export images` and `tap build`, with the description "progress as JSON lines on stderr".
2. In the "Conventions" section, add: "`--progress json` prints one JSON line per step on stderr (`{"phase":"render","done":2,"total":9}`) and a final `{"phase":"done","ok":true,...}` line with the `--json` result fields. Use it when a program needs progress; use `--json` when it only needs the result."
3. In the `tap dev` section, add: "Saving the deck updates open pages in place and keeps the current slide and step; there is no need to reload the browser."

- [ ] **Step 5: The changelogs**

In `CHANGELOG.md` and `docs/changelog.md`, under `## [Unreleased]`, add. Match each file's existing style.

Under `### Added`:

```markdown
- **`--progress json`** on `tap export pdf`, `tap export images` and `tap build` - One JSON line per step on stderr, such as `{"phase":"render","done":7,"total":14}`, download progress for the export browser on a first run, and a final line with the result. Made for scripts and for Tap Desktop.
- **A ready signal for every page** - Each tap page reports when the slide on screen has finished rendering, through `window.__tapReady`, a `tap:ready` event, and a `tapReady` message for a macOS web view. Exports and Tap Desktop's thumbnails wait for it.
```

Under `### Changed`:

```markdown
- **Saving updates `tap dev` pages in place** - A saved edit used to reload every open page. The page now fetches the deck and re-renders only the slides that changed, keeping its slide, fragment and step, so a component keeps its state and fonts do not reload. A changed custom theme file, or `r`, still reloads the page.
- **Exports wait for the page, not a checklist** - `tap export pdf` and `tap export images` wait for the page's own ready signal before each capture, instead of network idle and a list of checks. The output is the same. A slide that does not finish rendering within 30 seconds fails with a message that names it.
```

- [ ] **Step 6: Check the docs**

Run: `cd docs && npm run build`
Expected: the build succeeds with no broken links. (Skip this if `docs/node_modules` is missing, and say so in the PR.)

Run: `git diff --unified=0 -- docs skills CHANGELOG.md | grep '^+' | grep $'\xe2\x80\x94'`
Expected: no output. The lines this task adds use no em dash.

- [ ] **Step 7: Commit**

```bash
git add docs/reference/cli-commands.md docs/guide/building-export.md docs/guide/custom-components.md skills/tap/rules/cli.md CHANGELOG.md docs/changelog.md
git commit -m "docs: describe the ready signal, live updates and --progress json"
```

---

## Final check

- [ ] `cd frontend && npm run check && npm run lint && npm test -- --run`
- [ ] `make frontend && go test ./...` (not `-short`, so the browser and subprocess tests run) and `go vet ./...`, and `make lint` if the Makefile has it.
- [ ] `make build`, then by hand:
  - `./bin/tap dev examples/components/deck.md`, open it in a browser, go to the rolling deploy slide and step twice, then edit another slide's text and save. The page does not reload (the step and the component's state stay), and the edited slide shows the new text when you go to it.
  - Edit `examples/components/slides/RollingDeploy.jsx` and save: the component re-mounts with the new bundle, and the page does not reload.
  - In the browser console on any slide: `window.__tapReady` shows `{revision, slide, step}`; press the right arrow and it is `null`, then the new state.
  - `./bin/tap export pdf examples/components/deck.md --output "$TMPDIR/c.pdf" --progress json`: render lines on stderr, the done line last, and the PDF shows the rolling deploy chart.
  - `./bin/tap build examples/basic.md --output "$TMPDIR/dist" --progress json 2>&1 >/dev/null`: four step lines and the done line.
- [ ] Check the spec's Part 5 Tests list against the tests this plan added:
  - an update keeps the slide and step: `presentation.update.test.ts`, `websocket.test.ts` ("applies the new deck in place")
  - only changed slides re-render: `Slide.memo.test.tsx`
  - the step clamps when a slide loses steps: `presentation.update.test.ts`, `websocket.test.ts`
  - the signal fires for each kind of blocker, and resets on navigation: `readySignal.test.ts`, `probes.test.ts`, the component tests of Tasks 9 and 10, `App.ready.test.tsx`
  - the export tests run against the signal with no change in output: Task 12 Steps 7 and 8
  - progress lines against a small schema: `progress_schema_test.go`, used by every progress test
