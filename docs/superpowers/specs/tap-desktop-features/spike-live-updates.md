# tap present live-preview latency spike

Throwaway measurement spike. Nothing here is meant to be merged; it answers
one question for the planned macOS app's WKWebView live preview: how fast is
`tap dev`'s reload today, and is an in-place update feasible and faster.

Machine: Apple M4 Max. Node v23.7.0. Playwright 1.58 (Chromium and WebKit
both installed locally for this spike). 20 iterations per scenario unless
noted; the component-bundle-swap sub-scenario (5b) used 5 iterations because
each one is a real esbuild rebuild and the marginal iterations added little.

Raw numbers: `spike/results.json`. Script: `spike/measure.mjs`. Test decks:
`spike/decks/conference-talk.md` (the example deck), `spike/decks/stress-200.md`
(a generated 200-slide deck), `spike/decks/components-deck/` (a copy of
`examples/components`, which has a whole-slide component with 5 steps).

## Results

All times in milliseconds, write (or PUT) to visible-in-DOM unless noted.

| Scenario | WebKit median | WebKit p95 | Chromium median | Chromium p95 |
|---|---|---|---|---|
| Full reload (today) | 154 | 157 | 144 | 146 |
| In-place update (file watcher, prototype) | 104 | 105 | 103 | 106 |
| In-place update via PUT (bypass watcher, prototype) | 2 | 3 | 2.4 | 3 |
| Component .jsx edit, in-place path (n=5) | 411 | 414 | 411 | 411 |

Server-only stages (no browser involved), from `internal/server` timing
markers, via the normal file-watcher path:

| Stage | conference-talk.md median | conference-talk.md p95 | 200-slide deck median | 200-slide deck p95 |
|---|---|---|---|---|
| Watcher debounce (write -> debounce fires) | 101.0 | 101.1 | 101.0 | 101.1 |
| Parse + transform | 0.29 | 0.59 | 5.03 | 5.48 |
| Broadcast (hub fan-out) | 0.007 | 0.017 | 0.010 | 0.015 |
| Total, write to broadcast | 101.5 | 101.8 | 106.9 | 107.5 |

Parse + transform alone, isolated from the debounce (via a direct PUT, no
watcher involved): conference-talk.md median 0.23ms / p95 0.58ms; the
200-slide deck median 4.99ms / p95 5.21ms. So parse+transform is not the
bottleneck at any scale tested; the fixed 100ms debounce dominates the
server side entirely.

## Yes/no

- Does the full reload lose the current slide? No, in practice. A page
  reload's own state (URL hash, JS memory) does not survive, but the
  WebSocket hub replays its last-known slide/fragment/step to a
  reconnecting client whenever the URL hash still names the same slide (see
  applyHubLateJoinerState in frontend/src/lib/stores/websocket.ts and the
  hub's lastSlideState replay in internal/server/websocket.go's register
  case). For a single audience window this means state survives today's
  reload already, as long as that window had itself broadcast the state
  before reloading (true whenever a person, not just the file watcher,
  drove the last navigation).
- Does it lose the presenter step? No, for the same reason - step is part
  of the same replayed message.
- Does the full reload flash? No, not detectably: 0/20 iterations on
  either browser showed document.body empty or the slide element missing,
  per a requestAnimationFrame poll running continuously across the
  navigation. It is still a full document reload (new HTTP requests for
  the HTML/JS/CSS, a fresh React mount), which is the real cost - see the
  gap between the full-reload number and the in-place number above, even
  though neither visibly blanks the page on this machine and this deck.
- Does the in-place update keep slide, fragment and step? Yes, 0/20 lost
  on either path (file-watcher-triggered or PUT-triggered), by
  construction (updatePresentationInPlace clamps instead of resetting).
- Does the in-place update re-render only the changed slide? Only one
  slide is ever mounted in the audience view (frontend/src/App.tsx renders
  currentSlide alone, not the whole deck), so "only the changed slide"
  reduces to "the app re-renders once, not more, and the mounted slide's
  DOM node is reused rather than remounted." Measured: exactly 1 App
  render per update on every iteration (appRendersPerUpdate in the
  results), and no full-page navigation - the same page, same DOM node,
  just a reconciliation.
- Can component bundles swap in place? Yes, and no extra work was needed:
  bundle filenames are already content-hashed
  (internal/components/bundler.go's contentHash), and
  frontend/src/lib/components/DeckComponent.tsx already keys its
  React.lazy wrapper on the bundle's URL (makeLazyComponent, memoized by
  url). So updatePresentationInPlace handing the component a new
  slide.component.url was, by itself, enough to make it dynamically
  import() the rebuilt bundle with no reload and no manual cache-busting
  logic to add. Confirmed via network trace: editing the whole-slide
  component's .jsx produces a fresh /components/<newhash>.js request and
  no framenavigated event.
- Does editing an unrelated slide's markdown preserve a mounted
  component's step and internal state? Yes (stepSurvivedUnrelatedEdit:
  true on both browsers): editing slide 1 while parked on step 2 of the
  slide-3 component left step and slideIndex unchanged.

## What the real implementation would need

The prototype's pieces, and where they live:

- Server sends {"type":"update","revision":...} instead of reload on a file
  change: internal/server/websocket.go:33-36 (new MessageUpdate type) and
  :571-575 (BroadcastUpdate); wired into the existing reload call sites via
  internal/cli/dev.go:124-130 (broadcastChange, gated here by an env var
  for A/B measurement - a real implementation would just always send
  update and drop reload entirely, or keep reload only as a fallback for a
  client that fails to apply an update).
- Frontend fetches /api/presentation and merges into the store instead of
  reloading: frontend/src/lib/types.ts:286 (add 'update' to the message
  union), frontend/src/lib/stores/websocket.ts:373-376 (dispatch) and
  :435-446 (handleUpdate, calls fetchPresentation() then
  updatePresentationInPlace), frontend/src/lib/stores/presentation.ts:608-632
  (updatePresentationInPlace - keeps slide/fragment/step, clamped to the
  new deck's counts, unlike loadPresentation's reset-from-hash).
- Desktop app bypass endpoint: internal/cli/dev.go:325-343 (PUT
  /api/app/source, runs the same parse/build/transform pipeline via the
  refactored loadPresentationContent at :634, then always broadcasts
  update). This is the one a WKWebView-embedding macOS app would actually
  call, sending the unsaved editor buffer directly - no disk write, no
  100ms watcher debounce, no OS filesystem event round trip.
- A real implementation should also: drop the watcher path's 100ms
  debounce entirely for the app-driven case (the PUT endpoint already
  does, since it never goes through the watcher), keep reload as a
  fallback for a client whose update fetch fails (the prototype's
  handleUpdate just swallows that error -
  frontend/src/lib/stores/websocket.ts:443-445 - a real version should
  fall back to window.location.reload()), and decide what "revision"
  means for the app path (the prototype reuses ComputeRevision, which
  hashes the transformed presentation plus component bundles - already
  convenient for detecting no-op edits and skipping a broadcast, though
  this prototype does not do that optimization).

## What surprised me

- The 100ms watcher debounce is by far the largest cost in today's
  pipeline for a file-driven edit - larger than parsing a 200-slide deck
  (5ms) by 20x, and larger than the entire broadcast fan-out (0.01ms) by
  10,000x. Bypassing the watcher (scenario 4) cuts write-to-visible from
  ~104ms to ~2-3ms - not because in-place DOM patching is much faster than
  a websocket round trip, but because the debounce and the filesystem
  event round trip disappear entirely. For the desktop app's use case (an
  editor buffer that never touches disk on every keystroke), the debounce
  is pure waste; a direct PUT is worth roughly 100ms all on its own,
  independent of the reload-vs-in-place question.
- Today's full reload does not actually lose the slide or step in the
  single-viewer case people mostly use tap dev for - the hub's
  late-joiner state replay already covers it. I expected this to be a
  clear-cut "reload loses state" finding going in, based on reading
  initializeFromURL in isolation (it only takes the slide from the URL
  hash and resets step and fragment to their initial state); it took
  reading the reconnect handling in
  frontend/src/lib/stores/websocket.ts to find the mechanism that
  actually saves it. So the state-loss motivation for in-place updates is
  weaker than assumed; the latency motivation (154ms vs 2-104ms) still
  stands fully.
- Component bundle hot-swapping needed zero new frontend code. The
  existing React.lazy cache is already keyed by URL and bundle filenames
  are already content-hashed for unrelated reasons (cache-friendliness
  for a static export, presumably) - the two together happen to be
  exactly what in-place bundle swapping needs. I went in expecting to
  have to add a dynamic import() with a manual cache-busting query
  string; that machinery already exists.
- The .jsx edit latency (~411ms) is much larger than a markdown edit
  (~104ms) on the same in-place path, and is dominated by the esbuild
  rebuild cost, not by anything frontend-side - parse_start to parse_done
  in the server log for a component-touching change was consistently
  several hundred ms in manual testing (not captured in the aggregate
  parse+transform numbers above, since those runs used markdown-only
  decks). A production implementation should probably debounce or
  coalesce rapid .jsx edits more aggressively than the 100ms used for
  markdown, or show a distinct "rebuilding component..." state, since it
  is a meaningfully longer wait.

## What I could not measure

- WebKit's real-world behavior as an actual WKWebView (versus
  Playwright's bundled WebKit build) - close, per the task's own framing,
  but not identical; I could not verify this gap directly.
- A true worst-case flash: this spike's deck and machine never produced a
  visible blank frame on a full reload (0/20 both browsers), but a slower
  machine, a much heavier deck, or a throttled network could still show
  one; the requestAnimationFrame-based detector in spike/measure.mjs
  would catch it if it happened, it just didn't happen here.
- Iteration count for the .jsx bundle-swap sub-scenario is 5, not 20
  (each iteration is a real esbuild rebuild plus a settle wait for the
  deploy animation, and 5 was already enough to see a consistent number
  after the first, colder iteration - see the 111ms outlier in
  results.json, the only run before esbuild's own warm caches kicked in).
- Reload vs in-place latency for the .jsx case: only the in-place path
  was measured end-to-end for component edits; a reload-path number for
  the same edit was not collected (time budget), though it should be
  close to the full-reload markdown number (~150ms) plus the same ~400ms
  rebuild, since the rebuild cost is identical on both paths.
