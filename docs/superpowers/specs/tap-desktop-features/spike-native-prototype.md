# Tap Desktop native prototype: findings

Date: 2026-09-22. Throwaway spike. None of this code ships.

## Bottom line

- The native approach works. A real WKWebView shows an edit 1.7 ms (median) after the PUT on the example deck. That is the same as the browser spike's 2 to 3 ms in Playwright WebKit.
- On a 200-slide deck with a component on every slide, PUT to DOM is 14 ms, not 2 to 3 ms. tap's parse, build, and transform alone take 10 ms of that.
- From a keystroke to new text in the preview takes 104 ms on the example deck and 116 ms on the 200-slide deck (medians). The 100 ms typing pause is most of it.
- Typing in the TextKit 2 editor, with a box per slide, stays far under the 16 ms target on the 200-slide deck: 1.6 ms median, 6.0 ms p95, and 10 ms worst, measured to the frame commit. Drawing the boxes adds nothing measurable.
- Every piece was buildable with public API only. Two pieces fought back: the hidden thumbnail web view, and the thumbnails' need for a ready signal. The design spec should change in both places (see "Changes the design spec should get").

## What was built

The Swift project is `/Users/codemonkey/projects/tap-desktop-prototype` (about 1,860 lines in `Sources/TapDesktopPrototype`). It is a SwiftPM executable that `build.sh` wraps in an ad-hoc signed `.app`. It uses AppKit only, with no SwiftUI. It was built with Xcode 26.6 and tested on macOS 26.5.2 on an Apple M4 Max.

- `TapServer.swift` runs `tap dev --headless --port <free port> <deck>`, sends `PUT /api/app/source`, and drives the preview with tap's existing WebSocket `slide` message.
- `Editor.swift` is an `NSTextView` on TextKit 2. It draws the slide boxes, keeps paragraph roles and highlighting, shifts ranges locally, and hides the frontmatter.
- `MainWindow.swift` holds the window: a unified toolbar, an `NSSplitViewController` (sidebar, editor, preview), the floating glass overlay, and the pin switch.
- `SlidePanel.swift` is the thumbnail `NSCollectionView`, linked to the cursor in both directions.
- `Thumbnails.swift` is the hidden-`WKWebView` snapshot queue.
- `Preview.swift` is the preview pane. For measurement only, it injects a probe script (see Caveats).
- `Bench.swift` runs the measurements. It drives real `NSEvent` key events into the window.

The tap side is on the local branch `spike/desktop-prototype` (commit `ff60b46`, based on `spike/live-updates`). The worktree is `/Users/codemonkey/projects/tap-desktop-spike`. The branch is not pushed or merged. The commit changes two things:

- `PUT /api/app/source` now returns JSON: `{revision, slides: [{index, startLine, endLine, layout, title, steps, fragments, drivers}]}`, with the parse time in an `X-Tap-Parse-Ms` header. Line ranges come from tap's own splitter (`splitSlidesPreservingCodeBlocksWithLines`), with leading and trailing blank lines trimmed. Swift never parses markdown to find slides.
- `tap dev` binds to 127.0.0.1. The PUT endpoint rejects any Host header other than `127.0.0.1` or `localhost`, and caps the body at 8 MB.

Run it:

```sh
./build.sh
./build/TapDesktopPrototype.app/Contents/MacOS/TapDesktopPrototype --deck decks/conference-talk.md --panel floating
# measurements: --bench put,key,typing,typing-plain,thumbs,put-busy --slide 100 --iterations 30
```

The two test decks are `decks/conference-talk.md` (9 slides, the browser spike's deck) and `decks/stress-200/deck.md` (200 slides, 4,524 lines, a `LatencyDrop.jsx` component on every slide, a code block and a pause on every 4th slide). The raw numbers are in `results/*.json`.

## Screenshots

- Floating: `screenshots/panel-floating.png`. The glass panel (`NSGlassEffectView`) sits over the left edge of the editor, and the text scrolls under it.
- Pinned: `screenshots/panel-pinned.png`. This is the stock `NSSplitViewController` sidebar, which macOS 26 draws as an inset glass sidebar.
- Floating, then pinned with the pin button: `screenshots/floating-then-pinned.png`. This one shows the split that is no longer 50/50 (see Window).
- 200-slide deck: `screenshots/stress-pinned.png`. The thumbnails are missing their charts (see Thumbnails).

## Numbers

All times are in milliseconds. Each row has 30 runs unless it says otherwise, with 0 misses in every run.

### Preview update in a real WKWebView

"PUT to DOM" is the metric the browser spike used: from just before the PUT is sent until a MutationObserver in the page sees the new text. The two frame rows bound when it can reach the screen. The first `requestAnimationFrame` after the mutation runs before that frame paints. The second one runs a frame later, after the paint.

| Scenario | Median | p95 | Browser spike, Playwright WebKit (median / p95) |
|---|---|---|---|
| Example deck: PUT to DOM | 1.69 | 2.66 | 2 / 3 |
| Example deck: PUT to first frame after the change | 2.07 | 4.26 | |
| Example deck: PUT to second frame (painted) | 9.8 | 17.6 | |
| Example deck: PUT round trip (the app gets its slide list back) | 0.94 | 1.69 | |
| Example deck: tap parse, build, and transform | 0.26 | 0.60 | 0.23 / 0.58 |
| Example deck: PUT to DOM while all thumbnails render | 1.68 | 3.58 | |
| 200-slide deck: PUT to DOM | 13.8 | 15.5 | |
| 200-slide deck: PUT to second frame (painted) | 42.7 | 46.5 | |
| 200-slide deck: PUT round trip | 11.2 | 13.4 | |
| 200-slide deck: tap parse, build, and transform | 9.8 | 11.7 | 5.0 / 5.2 (no components) |
| 200-slide deck: PUT to DOM while all thumbnails render | 13.5 | 15.2 | |

The following rows start at a real key event in the editor, so they include the app's 100 ms typing pause:

| Scenario | Median | p95 |
|---|---|---|
| Example deck: key to PUT sent | 102.2 | 105.1 |
| Example deck: key to DOM | 104.0 | 106.8 |
| Example deck: key to second frame | 130.5 | 135.5 |
| 200-slide deck: key to PUT sent | 102.9 | 105.1 |
| 200-slide deck: key to DOM | 115.8 | 120.0 |
| 200-slide deck: key to second frame | 142.0 | 146.6 |

What the numbers show:

- A real WKWebView behaves like Playwright's WebKit build. The in-place path stays at about 2 ms to the DOM on a small deck. The browser spike could not check this gap, and there is none.
- Deck size matters more than the spec expects. On the 200-slide component deck, tap takes about 10 ms per PUT (the spike measured 5 ms without components, and the spec quotes 3.3 ms for the parser alone). The page then fetches the full `/api/presentation` again and re-renders, and a frame is painted about 43 ms after the PUT. That is still well inside the 200 ms target.
- The key-to-DOM number on the 200-slide deck is 116 ms: 103 ms until the PUT goes out, then 14 ms until the DOM changes. tap's parse, build, and transform is about 10 ms of those 14 ms. The page's fetch of the whole deck's JSON and its render take the rest, a few ms. The DOM-to-paint time is also longer on this deck (about 29 ms, against about 8 ms on the example deck). I did not break that down.
- Rendering thumbnails does not slow the preview: 13.5 ms against 13.8 ms. I did not check whether WebKit gives the two views one web content process or two.

### Typing in the editor, 200-slide deck

These runs use 280 real key events: 10 bursts of letters at 35 ms intervals, plus Return, a letter, and Backspace. Each burst is followed by a 250 ms pause, so tap answers 10 times and the boxes are replaced while typing goes on. The cursor is in slide 100. "Key event to frame committed" runs from the event's timestamp to a run loop observer that fires after Core Animation commits. It covers the key handler, TextKit 2 layout, box drawing, and the commit. It does not cover the window server or the display.

| Metric | With boxes: median | p95 | max | Without boxes: median | p95 | max |
|---|---|---|---|---|---|---|
| Key event to frame committed | 1.61 | 6.00 | 10.02 | 2.73 | 6.20 | 11.13 |
| Key handler (`keyDown`) | 1.20 | 4.14 | 5.93 | 1.23 | 4.24 | 8.20 |
| Drawing the boxes, per `drawBackground` call | 0.37 | 0.55 | 0.96 | | | |
| Applying tap's answer (ranges and restyle) | 0.02 | 0.05 | 0.67 | | | |

The two columns are within noise of each other. Box drawing costs about 0.4 ms per draw. Applying tap's answer costs almost nothing, because the locally shifted ranges already match tap's ranges after ordinary typing, so no paragraph needs a new style.

### Thumbnails

| Deck | All thumbnails | Per thumbnail, median | p95 | `takeSnapshot` median |
|---|---|---|---|---|
| Example deck (9) | 300 ms | 24.0 | 58.0 | 1.7 |
| 200-slide deck (200) | 4.9 s | 23.8 | 25.2 | 1.4 |

Each thumbnail means a hash change, a wait for the slide element, a 16 ms settle, and a 320-pixel-wide snapshot. The snapshot is cheap, and the wait is most of the time. A cold run of the 200-slide deck takes 5 s at idle. The queue does visible thumbnails first and pauses while the user types.

## How hard each piece was

### Box drawing in TextKit 2: easy, about 250 lines

I did not use custom `NSTextLayoutFragment` subclasses. Two simpler parts were enough:

1. **Space for the header.** Each paragraph gets a paragraph style for its role: the first line of a box, a middle line, the last line, a `---` separator, or a blank line between boxes. The first line has `paragraphSpacingBefore` for the header. Separators and blank lines between boxes get a short line height, so boxes sit close together as in the mockup. Because the spacing is a text attribute, it moves with the text on its own.
2. **Drawing.** `drawBackground(in:)` asks `NSTextLayoutManager` for the layout fragments of the visible boxes (taken from `textViewportLayoutController.viewportRange`, with a binary search over the boxes). It draws the rounded rectangle, the header (number, layout, title, and a step badge), and the blue ring for the cursor's slide. The text layers draw on top of it.

Things to know:

- `drawBackground(in:)` is still called on a TextKit 2 `NSTextView`, and it draws under the text fragment layers. It is enough, and it is fast.
- Boxes that start above the viewport cannot trust off-screen fragment frames. The box is extended past the dirty rect instead.
- Ranges shift locally on every edit, in `textStorage(_:didProcessEditing:)`, and edits that happen while a PUT is in flight are replayed onto tap's answer (an edit log keyed by send generation). With that, boxes never jumped in testing. When Return is pressed at the end of a slide, the box grows by one line at once. tap then trims the trailing blank line, and the box border moves up one line about 100 ms later. The text does not move.
- **Hiding the frontmatter works, with one catch.** `NSTextContentManagerDelegate.textContentManager(_:shouldEnumerate:options:)` returns false for every paragraph before tap's first slide, so slide 1 becomes the first box. But hidden text still takes the caret. Up Arrow from slide 1 moved the caret to offset 0, and a typed character then broke the frontmatter: tap reported 10 slides instead of 9. Clamping selections in `setSelectedRanges` fixes this (tested with `--bench hidden-caret`). Select All, Find, and Undo across hidden text are untested.

### Floating glass panel and switching to pinned: easy, about 80 lines

- Floating is an `NSGlassEffectView` (public API on macOS 26) added over the editor pane. It falls back to `NSVisualEffectView` on macOS 14 and 15, which was compiled but not run.
- Pinned is a stock `NSSplitViewItem(sidebarWithViewController:)`. On macOS 26 the stock sidebar already draws as an inset glass panel, so the two states look like relatives.
- Switching moves one `SlidePanelView` between the glass view and the sidebar, and collapses or expands the sidebar item with animation. No private API was needed.
- **What fought:** the split does not return to 50/50 after the sidebar opens. The sidebar takes its width from the editor. Resetting the divider in the animation's completion handler did not fix it (see `screenshots/floating-then-pinned.png`). A real app needs holding priorities or its own divider logic. This is small, but the spec's "50/50" is not free.
- **What fought:** with `fullSizeContentView`, the editor's text scrolls under the window title and subtitle, and the title does not always blur or cover it. The screenshots show text behind "conference-talk". I did not find a setting that fixes this. A real app needs a top content inset or the macOS 26 scroll edge effect set up on purpose.

### Snapshot thumbnails: medium, and the spec needs changes

Each problem below cost time, and each one is a spec issue:

1. **A hidden WKWebView in an off-screen window never finishes anything.** WebKit treats the page as hidden and suspends it. `requestAnimationFrame` never fires, and even a `setTimeout` loop inside `callAsyncJavaScript` never returned. The fix uses only public API: the thumbnail web view is a real subview of the main window, placed behind the preview pane under an opaque cover view. WebKit then treats it as visible, and everything works. I also tried the private `_windowOcclusionDetectionEnabled` key, only to confirm the cause. That run hung at launch, so I dropped it. The final code uses no private API.
2. **The thumbnail page must not join the WebSocket hub.** Every tap client broadcasts its slide position and follows everyone else's. A hidden page that walks through slides would drag the preview along with it. `?print=true` never connects the socket, so the thumbnail view uses it.
3. **Print mode lays the slide out at 1920x1080 CSS pixels.** The 960x540 view showed a quarter of the slide until I set `pageZoom = 0.5`.
4. **A reload whose URL differs only in the `#` fragment is a same-document navigation.** It never calls `didFinish`, and the queue hung. Each reload now adds a counter to the query.
5. **`data-index` on `.slide` is 1-based.** A 0-based check passed on the example deck by matching the previous slide during its transition, then waited for its whole timeout on every slide of the 200-slide deck (950 ms per thumbnail, 167 s for the deck).
6. **The thumbnails have no ready signal, and they show it.** Charts from `.jsx` components are missing from every hidden-view thumbnail, because the snapshot fires before the component bundle loads. The spec's "tap's single ready signal" is not optional, and it must cover component bundles.
7. **"The current slide's thumbnail is a snapshot of the live preview" gives bad images.** Taken on DOM change plus 500 ms, the snapshot still catches the deck's fade transition and the terminal theme's type-in animation ("Debuggin", "# What W" in the screenshots). The live page needs a "settled" signal, or the current slide should come from the hidden renderer like the others.

## Does it look and feel native?

Mostly yes, from the screenshots and short hands-on use.

- The toolbar items, the prominent blue Play button (`NSToolbarItem.style = .prominent`), the segmented Preview/Deck control, the stock sidebar, and the glass overlay all come from AppKit, so they match the system in light mode. Dark mode was not tested.
- The box editor is close to the mockup: 10 pt corner radius, a header with number, layout, and title, a step badge, the accent ring for the cursor's slide, and faint `---` separators.
- Typing, selection, undo, and scrolling are the stock `NSTextView`, so they feel native. There was no visible lag on the 200-slide deck.
- Not native yet: the text showing under the title, the split not returning to 50/50, and the placeholder toolbar buttons that do nothing.

## Changes the design spec should get

1. **Performance.** Replace "2 to 3 ms from the PUT until the new text is visible" with measured numbers by deck size. For example: "about 2 ms to the DOM on a 10-slide deck, and about 14 ms to the DOM (about 43 ms to a painted frame) on a 200-slide deck with a component on every slide. tap's parse, build, and transform takes about 10 ms of that." Also note that the 3.3 ms benchmark covers the parser only.
2. **Parse cost on large decks.** tap's parse, build, and transform is the largest part of the 14 ms on the 200-slide deck. It is fine for the 200 ms target. If the target tightens, look at it first, then at the page's full `/api/presentation` re-fetch on every `update`.
3. **PUT response.** Return `fragments` as well as `steps` (the header badge needs both), and define the line range as the slide's content without leading or trailing blank lines. Also say how a range treats blank lines between slides. The `drivers` list came back empty for the example deck's `{driver: sqlite, …} {2-3}` fence. I did not look into why.
4. **Hidden thumbnail renderer.** State that it must live inside a visible window's view hierarchy (behind other content), not in an off-screen window. State that it loads the page with `?print=true` so it never joins the WebSocket hub, and that it scales the 1920x1080 print layout.
5. **Ready signal.** Make the ready signal a hard prerequisite, and have it cover component bundles, fonts, and images. The app also needs a way to receive it without injecting script. For example, the page could send a `rendered {revision, slide}` message over the WebSocket, and the app reads it on its own socket connection.
6. **Current-slide thumbnail.** Either require a "settled" signal from the live preview (after transitions and theme animations), or drop the idea that the current thumbnail is a snapshot of the preview and render it in the hidden view after each update.
7. **Frontmatter.** Say that hiding uses the TextKit 2 enumeration delegate, and that the app must keep the caret and selections out of the hidden range. Name Select All, Find, and Undo as cases to test.
8. **Window layout.** Say that the app owns the 50/50 divider and restores it after the sidebar is pinned, collapsed, or resized. Also decide how the editor's top edge looks under the unified toolbar.
9. **Child processes.** When the prototype was killed (SIGKILL, as in a crash), its `tap dev` children kept running: 10 were left over after this spike. The spec's "Processes" section should say how tap exits when its parent dies, for example by quitting when stdin closes, which suits the `--app` stdin control channel.
10. **Security** (already in the prerequisites spec on PR #16): the throwaway endpoint showed why `--app` mode needs the token, a loopback bind, and a body cap.

## Caveats

- **Throwaway code.** All of it: the Swift project, the tap branch, and the benchmark harness.
- **The spike endpoint.** The original `PUT /api/app/source` on `spike/live-updates` has no token, no host or origin guard, and an unbounded `io.ReadAll`, and upstream `tap dev` binds to `0.0.0.0`. My branch binds to 127.0.0.1, checks the Host header, and caps the body at 8 MB. It still has no token. Run it only on a machine you trust, never on a shared network.
- **Injected script.** The preview injects a MutationObserver probe to timestamp the DOM change. That script is for measurement only. The real app injects nothing, which is why item 5 above asks tap for a signal.
- **Clocks.** The PUT-to-page numbers compare the app's wall clock with the page's `performance.timeOrigin + performance.now()`. The spike did the same. Both clocks read the same system clock. WebKit may coarsen page timers, but the sub-millisecond values in the results suggest it did not here.
- **Synthesized keys.** The benchmarks call `window.sendEvent` with synthesized `NSEvent`s, so they skip the window server and input methods. The typing numbers measure the app's main thread up to the Core Animation commit, not the time until light leaves the screen.
- **One machine.** Every number comes from one M4 Max with the window in front and visible (recorded as `windowVisibleAtStart` and `windowVisibleAtEnd` in the results). A slower Mac will be slower.
- **Not tested:** dark mode, macOS 14 and 15, multi-slide selection and dragging, the Deck tab, tap's error display, and the thumbnail disk cache.
