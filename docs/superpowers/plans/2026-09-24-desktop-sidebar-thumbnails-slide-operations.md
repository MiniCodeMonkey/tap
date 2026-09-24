# Tap Desktop sidebar, thumbnails and slide operations (D3): implementation plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The slide panel (peek on hover, click to pin) with cached thumbnails from one hidden print-mode `WKWebView`, and every slide operation (move, duplicate, delete, skip, insert, multi-select, drag between decks) as one undo step on the buffer, cut along the ranges tap reports and proven by a round trip through `tap slide list`.

**Architecture:** `TapDesktopCore` gains the pure parts: a `SlideDocument` that cuts the buffer along tap's boxes into prefix, slides, gaps and suffix and reassembles it with exactly one `---` between slides; `SlideEditing`, which turns a `SlideOperation` into new text, new boxes and a caret; `DirectiveComment`, which rewrites a slide's leading comment for `skip: true`; the thumbnail key, disk cache, priority queue and flat-image check; the `/api/presentation` summary; the layout catalog; and the panel's pinned state. The app target adds the panel (`NSCollectionView` of thumbnails, peek overlay and pinned sidebar), the thumbnail renderer (a `WKWebView` behind the editor's opaque scroll view, loading `?print=true` and waiting for the `tapReady` handler), the operations wired into `DeckSessionController` as single undo steps, drag and drop in the sidebar and from box headers, the Slide menu and context menus, and New Slide with the layout gallery. Swift never finds a slide boundary: every range comes from tap's last answer, and after an operation the app adopts the permuted ranges it built from them until tap's next answer replaces them.

**Tech Stack:** Swift 6.3 compiler in Swift 5 language mode, AppKit (`NSCollectionView`, `NSSplitViewController` sidebar, `NSGlassEffectView` on macOS 26 with `NSVisualEffectView` below it, `NSDraggingSession`, `NSPopover`), TextKit 2, WebKit (`WKWebView`, `WKSnapshotConfiguration`, `WKScriptMessageHandler`), CryptoKit, XCTest and XCUITest, XcodeGen, the bundled `tap` (`tap slide list --json`, `tap slide add --layout <x> --print`, `tap slide add --print --json`).

**Spec:** `docs/superpowers/specs/2026-09-22-tap-desktop-design.md` (milestone 3; the sections "Window layout", "Slide panel and slide operations", "Performance" and "Testing"), the D3 outline and "Decisions for the desktop app" in `docs/superpowers/plans/2026-09-22-tap-desktop-roadmap.md`, and the feature files in `docs/superpowers/specs/tap-desktop-features/` (`03-slide-operations.feature` whole, plus the D3 scenarios of `02`, `04`, `12` and `13` listed below). The mockups are the "Tap Desktop Mockups" canvas (Main, MainPinned, Reorder, Find, Dark, LayoutGallery, ContextMenu, MenusSlide). Where the mockups and the spec differ, the spec wins (roadmap decision, 2026-09-22): the mockups still draw the old floating panel; the built behavior is peek on hover and click to pin. The starting code is D2's `desktop/` as it landed on `feat/desktop-app-shell`, and the prototype's `Thumbnails.swift`, `SlidePanel.swift` and `MainWindow.swift` in `/Users/codemonkey/projects/tap-desktop-prototype` (read only, never modified).

**Branch:** `feat/desktop-sidebar-slide-operations`, branched from `feat/desktop-app-shell` (the D2 branch, at 6f8d3ca or later) in a worktree at `/Users/codemonkey/projects/tap-d3`, so Tasks 1 to 5 (the core package) run before D2 merges; the branch is rebased onto `main` once pull request 29 has merged, and Task 6 onward runs on the rebased branch. One pull request.

**Prerequisites on `main`, checked 2026-09-24 (`internal/cli`):** `tap slide list [deck] --json` (`slide_list.go`, P3), `tap slide add --layout <x> --print` and `tap slide add --print --json` with no `--layout`, which prints every layout's template in the wizard's order (`slide.go`, P4), the `skip` directive (`internal/parser/parser.go`, `directiveFields`), and P5's ready signal in print mode (`frontend/src/App.tsx`, `useReadySignal` with `requirePaint: PRINT_MODE`). No tap prerequisite task is needed. Each task checks only what it uses: Task 1 that D2's core package is present, Task 6 that the app target builds, Task 16 that D2's `desktop/scenarios.txt` and `desktop/scripts/check-scenarios.sh` exist (they landed in commit 8f0c0f1).

## Global Constraints

- Everything in D2's Global Constraints still holds: macOS 14 or later, AppKit core, ad-hoc signing, the bundled `tap`, P6's protocol exactly, the app drives the preview only through the WebSocket `slide` message, spelled-out identifiers, present-tense comments, no em dashes anywhere (`--`, a comma or a new sentence instead), never modify the prototype repository, `make frontend` before the first Xcode build.
- **The app never injects script into a page.** No `evaluateJavaScript` or `callAsyncJavaScript` in production code; `PreviewViewController.pageText` and `pageValue` stay test-only. The thumbnail renderer navigates by loading URLs and learns that a slide is ready only through the `tapReady` message handler. (D2 ledger, Task 27.)
- **A capture needs real paint.** A hidden, covered or minimized `WKWebView` reports ready (a live page settles on the DOM) but has not painted, and its snapshot comes back blank. The thumbnail web view lives inside the deck window's view hierarchy, behind the editor's opaque scroll view; the renderer runs only while the window's `occlusionState` has `.visible` and the web view is not `isHiddenOrHasHiddenAncestor`; and a snapshot whose pixels are one flat colour is never cached and never shown. Print mode's ready signal already waits for a paint (`requirePaint: PRINT_MODE`), which is why a print page in a hidden window never reports ready at all: the renderer treats that as a timeout and retries later, not as a blank image. (D2 ledger, Tasks 21 and 27, and pull request 27.)
- **No production code steals focus.** No `NSApp.activate` and no `makeKeyAndOrderFront` on another app's behalf outside the test targets. The panel and the gallery are shown inside the active app in answer to the person's own click or shortcut. (D2 ledger, Task 21.)
- **No modal alerts.** A refused operation (deleting every slide, dropping onto itself) does nothing or beeps; it never shows `NSAlert`. (D2 ledger, Tasks 15 and 26.)
- **Never write to the editor's text storage directly.** Every structural edit goes through `EditorTextView.replaceText(in:with:actionName:)`, which runs the selection clamp and `shouldChangeText`. A review must grep for `textStorage?.replaceCharacters` outside `EditorTextView` and find nothing. (D2 ledger, Task 15 gate.)
- **The edited flag is derived from content.** `DeckSessionController.refreshEditedState` is the only caller of `updateChangeCount`; no task adds another. An operation marks the document edited by changing the text, nothing else. (D2 ledger, Task 24.)
- `weak self` in every closure that outlives a call, no `unowned`. (D2 ledger, Task 16.)
- **Coalesce to current state, never replay a queue.** When the session restarts, `sessionStateChanged` rebuilds the client and socket and resends the navigator's current intent; the thumbnail controller re-requests its current work the same way. (D2 ledger, Tasks 10 and 11.)
- **Tests, the branch rules.** Agents run `make -C desktop core-test` (`swift test` in the package) locally for every task. A hosted test runs locally only one at a time, scoped: `make -C desktop test ONLY=TapTests/<Class>/<test>`, never the whole `TapTests` bundle, which CI runs on `macos-26`. `make -C desktop uitest` and `make -C desktop bench` are the person's to run; agents write and compile them (`xcodebuild build-for-testing`) and never launch them. One Xcode process at a time in the worktree; check for a running `xcodebuild` before starting one. Before diagnosing any render, ready or thumbnail problem, confirm `embedded/dist` exists and is newer than `frontend/` (the build script refuses otherwise). (D2 ledger.)
- **A hosted test host has no key window on this machine and often has one on the CI runner.** No test may depend on either: tests drive actions, tracking handlers and synthesized events directly (`windowController.newSlide(nil)`, `peek.pointerEnteredButton()`, `button.mouseDown(with:)`), set the first responder themselves with `window.makeFirstResponder` when focus matters, never read `NSApp.keyWindow`, and read the window's occlusion state rather than assuming it. (D2 ledger, Task 22, and commit 2629958.)
- **Mutation testing is how this branch finds tests that cannot fail.** Every task's implementer removes or inverts the one line of production code its test exists for and quotes the failure; every review runs at least one independent mutation. A test that survives its mutation is not done. (D2 ledger, Tasks 13, 20, 22, 24 and 28.)
- Every scenario this plan claims has a test named `test` plus the scenario name in UpperCamelCase (every word capitalized, punctuation dropped): "Move one slide by dragging in the sidebar" is `testMoveOneSlideByDraggingInTheSidebar`. The claims go into `desktop/scenarios.txt` as `D3 | <file> | <scenario>` rows, and `make -C desktop check-scenarios` must pass.
- Every slide operation edits the buffer from tap's slide ranges, is one undo step, and its hosted test round-trips the result through the bundled `tap slide list --json` (`TapSlideList.list(text:)`, Task 10) and checks the slide order and that exactly one `---` line sits between each pair of slides.
- The thumbnail cache lives at `~/Library/Application Support/Tap/Thumbnails/<sha256>.png`, keyed by tap's per-slide content hash, the deck's theme signature (theme, custom theme flag, theme colours, aspect ratio) and the snapshot width. Tests point `AppEnvironment.shared.thumbnailCache` at a temporary folder so they never touch the person's cache.

## Review Focus

Five conditions the spec implies that no scenario names, most likely to bite first. Each has its test pinned to the task that owns the code.

1. **Undo after a slide operation while tap has not answered yet.** Cmd+Z reverts the text; the boxes must follow the reverted text at once and tap must be sent the reverted text without a keystroke. Otherwise the sidebar keeps the new order while the editor shows the old one. Task 10, `testUndoRestoresBoxesAndResyncsTap`.
2. **A `---` inside a fenced block in a moved slide.** The slide moves as one range and the fence's `---` is never treated as a separator. Task 1, `testAFenceWithASeparatorLineMovesWhole` (core) and the `ops.md` fixture's slide 4 in every hosted round trip.
3. **Deleting the selection that is every slide.** The deck would have no slide 1 and the frontmatter would come out of hiding; the operation is refused and the menu item is disabled. Task 1, `testDeletingEverySlideIsRefused`; Task 14, `testDeleteIsDisabledWhenEverySlideIsSelected`.
4. **The window is covered while thumbnails render.** No blank image may reach the panel or the disk; the queue waits and resumes. Task 6, `testACoveredWindowRendersNothingAndResumes` and `testAFlatCaptureIsRetriedAndAcceptedOnlyAfterThreeTries`.
5. **A duplicated slide shares its thumbnail key with the original.** Both cells must show the image, and the renderer must not render it twice. Task 10, `testDuplicatedSlidesShareOneThumbnail`.

## Scenarios this plan claims

| Feature file | Scenario | Test | Target |
|---|---|---|---|
| 02-slide-structure | The current slide | `testTheCurrentSlide` | TapTests |
| 02-slide-structure | Sidebar and cursor are linked | `testSidebarAndCursorAreLinked` | TapTests |
| 03-slide-operations | Move one slide by dragging in the sidebar | `testMoveOneSlideByDraggingInTheSidebar` | TapTests (drop path); real drag in TapUITests |
| 03-slide-operations | Move with the keyboard | `testMoveWithTheKeyboard` | TapTests |
| 03-slide-operations | Move several slides | `testMoveSeveralSlides` | TapTests |
| 03-slide-operations | Move by dragging a box header in the editor | `testMoveByDraggingABoxHeaderInTheEditor` | TapTests (drop path); real drag in TapUITests |
| 03-slide-operations | Insert a slide with the last layout | `testInsertASlideWithTheLastLayout` | TapTests |
| 03-slide-operations | Pick a layout from the gallery | `testPickALayoutFromTheGallery` | TapTests |
| 03-slide-operations | Duplicate and delete | `testDuplicateAndDelete` | TapTests |
| 03-slide-operations | Skip a slide | `testSkipASlide` | TapTests |
| 03-slide-operations | Frontmatter never moves | `testFrontmatterNeverMoves` | TapTests |
| 03-slide-operations | Drag slides to another deck | `testDragSlidesToAnotherDeck` | TapTests (the scenario text now says the drop moves, per decision 2) |
| 04-preview | Split layout | `testSplitLayout` | TapTests |
| 04-preview | Peek at the slide panel | `testPeekAtTheSlidePanel` | TapTests (handlers); real hover in TapUITests |
| 04-preview | Pin the slide panel | `testPinTheSlidePanel` | TapTests |
| 04-preview | Step through a custom component | `testStepThroughACustomComponent` | TapTests |
| 12-menus-and-shortcuts | Slide menu | `testSlideMenu` | TapTests |
| 12-menus-and-shortcuts | Context menu on a slide | `testContextMenuOnASlide` | TapTests |
| 12-menus-and-shortcuts | VoiceOver | `testVoiceOver` | TapTests |
| 13-performance | The current thumbnail follows the preview | `testTheCurrentThumbnailFollowsThePreview` | TapTests |
| 13-performance | Thumbnails | `testThumbnails` | TapTests |
| 13-performance | Reopen | `testReopen` | TapTests |

22 scenarios. Left for later milestones, unchanged from D2's open question 3: "Open a folder" (D6), "Presenting shortcuts" (D4), "Deck settings live in the inspector" (D5). The Slide menu's Generate Image, Insert Image and New Component items are present and disabled until D6, the same way D2 shipped New Deck.

## File structure

| Path | Responsibility |
|---|---|
| `desktop/TapDesktopCore/Sources/TapDesktopCore/SlideDocument.swift` | The buffer cut along tap's boxes: prefix, entries (slide text and the gap after it), suffix; reassembly with one separator per boundary |
| `.../TapDesktopCore/SlideEditing.swift` | `SlideOperation`, `SlideEditResult`, `SlideEditing.apply`, `actionName`, `firstSlotRange` |
| `.../TapDesktopCore/DirectiveComment.swift` | Reading and rewriting a slide's leading directive comment (`skip: true`) |
| `.../TapDesktopCore/SlideRangeTracker.swift` | Modify: `adopt(_ boxes:)` |
| `.../TapDesktopCore/SlideDragPayload.swift` | The pasteboard type and payload for dragging slides, within and between decks |
| `.../TapDesktopCore/SlideAccessibility.swift` | VoiceOver labels for boxes, thumbnails and drop indicators |
| `.../TapDesktopCore/PresentationSummary.swift` | The parts of `GET /api/presentation` the thumbnails need: revision, theme signature, per-slide hash, skip and steps |
| `.../TapDesktopCore/TapClient.swift` | Modify: `presentation()` |
| `.../TapDesktopCore/ThumbnailKey.swift`, `ThumbnailCache.swift` | The cache key and the PNG files on disk |
| `.../TapDesktopCore/ThumbnailQueue.swift` | Priority order: current, visible, nearest, the rest |
| `.../TapDesktopCore/FlatImageCheck.swift` | Whether a snapshot is one flat colour |
| `.../TapDesktopCore/LayoutCatalog.swift` | Decoding `tap slide add --print --json`, display names, the schematic each gallery cell draws, the last layout used (Task 5) |
| `.../TapDesktopCore/SlidePanelState.swift` | The panel's pinned state, remembered per deck, pinned on first launch |
| `.../TapDesktopCore/DividerPolicy.swift` | Modify: the balanced position with a sidebar in front |
| `desktop/Tap/Preview/WeakScriptMessageHandler.swift` | D2's weak script message forwarder, shared by the preview and the renderer |
| `desktop/Tap/Sidebar/ThumbnailItem.swift` | One thumbnail cell: number, image, skipped dimming, updating mark, selection ring, VoiceOver label |
| `desktop/Tap/Sidebar/SidebarHostViewController.swift` | The sidebar split item's controller, which holds the panel while it is pinned |
| `desktop/Tap/Sidebar/HoverButton.swift` | The toolbar button that reports the pointer entering and leaving |
| `desktop/Tap/Windows/DeckContentViewController.swift` | The window's content: the split view and the peek overlay as siblings |
| `desktop/Tap/Sidebar/SlidePanelViewController.swift` | The `NSCollectionView`, selection linked to the cursor, drag and drop, context menu, Delete key |
| `desktop/Tap/Sidebar/SlidePanelOverlay.swift` | The peek overlay: `NSGlassEffectView` on 26, `NSVisualEffectView` before |
| `desktop/Tap/Sidebar/SlidePanelPeek.swift` | The tracking areas and timers that show and hide the peek |
| `desktop/Tap/Thumbnails/ThumbnailRenderer.swift` | The hidden print-mode web view, the `tapReady` handler, the render loop |
| `desktop/Tap/Thumbnails/ThumbnailController.swift` | Keys from the presentation summary, memory and disk cache, the queue, the panel's images |
| `desktop/Tap/Slides/SlideOperations.swift` | `DeckSessionController` extension: selection, `perform(_:)`, undo adoption, copy, paste, drop |
| `desktop/Tap/Slides/SlideContextMenu.swift` | The context menu for thumbnails and box headers |
| `desktop/Tap/Slides/LayoutMenuDelegate.swift` | Fills New Slide from Layout from tap's catalog |
| `desktop/Tap/Slides/LayoutSchematicView.swift` | Draws a layout's schematic in a gallery cell |
| `desktop/Tap/Slides/NewSlideButton.swift` | The toolbar button: click inserts, hold opens the gallery |
| `desktop/Tap/Slides/LayoutGalleryController.swift` | The popover with the 12 layouts |
| `desktop/Tap/Slides/LayoutCatalogLoader.swift` | Runs the bundled tap once for the catalog |
| `desktop/Tap/Editor/EditorTextView.swift` | Modify: `adoptBoxes`, header hit testing, header drag source and slide drop target, the drop indicator, accessibility children |
| `desktop/Tap/Editor/EditorViewController.swift` | Modify: hosts the hidden thumbnail web view behind the scroll view |
| `desktop/Tap/Windows/MainSplitViewController.swift` | Modify: the sidebar split item, balance with a sidebar |
| `desktop/Tap/Windows/DeckWindowController.swift` | Modify: the Slides and New Slide toolbar items, peek and pin, the slide actions |
| `desktop/Tap/Documents/DeckSessionController.swift` | Modify: panel and thumbnails wired in, hub messages routed, undo and redo resync tap |
| `desktop/Tap/App/MainMenu.swift` | Modify: the Slide menu |
| `desktop/Tap/App/AppEnvironment.swift` | Modify: `thumbnailCache`, `panelDefaults`, `layoutCatalogLoader` |
| `desktop/TapTests/Support/TapSlideList.swift` | Runs the bundled `tap slide list --json` on a text, and checks the separator rule against its ranges |
| `desktop/TapTests/Support/TapTemplate.swift` | Runs the bundled `tap slide add --layout <x> --print` |
| `desktop/TapTests/Support/FakeDraggingInfo.swift` | A hand-built `NSDraggingInfo`, so the real drop methods run in hosted tests |
| `desktop/TapTests/Fixtures/ops.md`, `Fixtures/stepped/` | Seven titled slides for operations; a whole-slide component with `export const steps = 5` |
| `desktop/TapTests/*.swift` | The hosted tests: `ThumbnailRendererTests`, `SidebarTests`, `SlidePanelLayoutTests`, `ThumbnailTests`, `SlideOperationTests`, `DragAndDropTests`, `EditorHeaderDragTests`, `NewSlideTests`, `SlideMenuTests`, `SlideContextMenuTests`; D2's `WindowLayoutTests` and `EditorTextViewTests` are touched |
| `desktop/TapBenchmarks/ThumbnailBenchmark.swift` | The cold pass and the reopen on the 200-slide deck, compile only |
| `desktop/TapUITests/SlideDragUITests.swift`, `SlidePanelUITests.swift` | Real drags and the real hover, local only |
| `desktop/scenarios.txt` | Modify: the 22 D3 rows |
| `docs/superpowers/specs/2026-09-22-tap-desktop-design.md`, `tap-desktop-features/03-slide-operations.feature`, `04-preview.feature` | Modify: decisions 2 and 4 (a drag between decks moves; the panel state is per deck) |

---

### Task 1: The slide document and the operations on it

**Files:**
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/SlideDocument.swift`
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/SlideEditing.swift`
- Test: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/SlideDocumentTests.swift`, `SlideEditingTests.swift`

**Interfaces:**
- Consumes: D2's `Slide` and `SlideBox` (`SlideRangeTracker.swift`, `TapProtocol.swift`).
- Produces:
  - `struct SlideDocument { prefix: String; entries: [Entry]; suffix: String }` with `struct Entry { slide: Slide; text: String; gapAfter: String }`, `init(text: String, boxes: [SlideBox])`, `var text: String`, `var boxes: [SlideBox]` (renumbered 1 to n, line numbers recomputed), `static let separator = "\n\n---\n\n"`, `mutating func normalizeGaps()`
  - `enum SlideOperation { move(numbers: [Int], beforeNumber: Int?); duplicate(numbers: [Int]); delete(numbers: [Int]); insert(markdowns: [String], beforeNumber: Int?); setSkip(numbers: [Int], skipped: Bool) }`
  - `struct SlideEditResult { text: String; boxes: [SlideBox]; selectedNumbers: [Int]; caret: Int; sourceNumbers: [Int?] }`
  - `enum SlideEditing { static func apply(_:to:boxes:caretOffsetInSlide:) -> SlideEditResult?; static func actionName(for:) -> String; static func firstSlotRange(inSlideText:) -> NSRange }`
  - Slide numbers in an operation are the numbers of the boxes it is given (1-based); `beforeNumber` nil means the end.

- [ ] **Step 1: Check the prerequisites**

Run:

```bash
cd /Users/codemonkey/projects/tap-d3
git log --oneline -1
git merge-base --is-ancestor 6f8d3ca HEAD && echo "on D2 at 6f8d3ca or later"
test -f desktop/TapDesktopCore/Sources/TapDesktopCore/SlideRangeTracker.swift && echo "D2 core present"
grep -n '"add \[deck\]"' internal/cli/slide.go && grep -n 'func printAllLayouts' internal/cli/slide.go && grep -n '"list \[deck\]"' internal/cli/slide_list.go
```

Expected: the two "present" lines and the three grep hits. Nothing in Tasks 1 to 5 needs the app target or the scenario check; Task 6 checks the app target and Task 16 the scenario tooling.

- [ ] **Step 2: Write the failing document tests**

`desktop/TapDesktopCore/Tests/TapDesktopCoreTests/SlideDocumentTests.swift`:

~~~~swift
import XCTest
@testable import TapDesktopCore

final class SlideDocumentTests: XCTestCase {
    /// Frontmatter, three slides, a fence holding a "---" line inside slide 2.
    static let text = """
    ---
    title: Ops
    ---

    # One

    ---

    # Two

    ```text
    ---
    ```

    ---

    # Three

    """

    static func boxes(for text: String) -> [SlideBox] {
        // The ranges tap reports for `text`: each slide from its first line to its last, blank lines outside.
        let nsText = text as NSString
        func range(_ marker: String, to end: String) -> NSRange {
            let start = nsText.range(of: marker).location
            let stop = NSMaxRange(nsText.range(of: end, range: NSRange(location: start, length: nsText.length - start)))
            return NSRange(location: start, length: stop - start)
        }
        return [
            SlideBox(range: range("# One", to: "# One"), slide: Slide(number: 1, startLine: 5, endLine: 5, title: "One")),
            SlideBox(range: range("# Two", to: "```\n"), slide: Slide(number: 2, startLine: 9, endLine: 13, title: "Two")),
            SlideBox(range: range("# Three", to: "# Three"), slide: Slide(number: 3, startLine: 17, endLine: 17, title: "Three")),
        ].map { box in
            // Ranges end at the end of the last line, before its newline.
            var trimmed = box
            while trimmed.range.length > 0, nsText.character(at: NSMaxRange(trimmed.range) - 1) == 10 {
                trimmed.range.length -= 1
            }
            return trimmed
        }
    }

    func testCutsTheTextIntoPrefixSlidesGapsAndSuffix() {
        let document = SlideDocument(text: Self.text, boxes: Self.boxes(for: Self.text))
        XCTAssertEqual(document.prefix, "---\ntitle: Ops\n---\n\n")
        XCTAssertEqual(document.entries.map(\.text), ["# One", "# Two\n\n```text\n---\n```", "# Three"])
        XCTAssertEqual(document.entries.map(\.gapAfter), ["\n\n---\n\n", "\n\n---\n\n", ""])
        XCTAssertEqual(document.suffix, "\n")
        XCTAssertEqual(document.text, Self.text, "cutting and joining is the identity")
    }

    func testBoxesAreRenumberedWithRecomputedLines() {
        var document = SlideDocument(text: Self.text, boxes: Self.boxes(for: Self.text))
        document.entries.swapAt(0, 2)
        document.normalizeGaps()
        let boxes = document.boxes
        XCTAssertEqual(boxes.map(\.slide.number), [1, 2, 3])
        XCTAssertEqual(boxes.map(\.slide.title), ["Three", "Two", "One"])
        XCTAssertEqual(boxes[0].range, NSRange(location: 20, length: 7))
        XCTAssertEqual(boxes.map(\.slide.startLine), [5, 9, 17])
        XCTAssertEqual(boxes.map(\.slide.endLine), [5, 13, 17])
        let text = document.text as NSString
        XCTAssertEqual(text.substring(with: boxes[2].range), "# One")
    }

    func testNormalizeGapsRewritesOnlyNewBoundaries() {
        var document = SlideDocument(text: Self.text, boxes: Self.boxes(for: Self.text))
        document.entries[0].gapAfter = "\n---\n"
        let moved = document.entries.remove(at: 2)
        document.entries.insert(moved, at: 0)
        document.normalizeGaps()
        XCTAssertEqual(document.entries.map(\.slide.title), ["Three", "One", "Two"])
        XCTAssertEqual(document.entries[0].gapAfter, SlideDocument.separator, "a new boundary gets the canonical separator")
        XCTAssertEqual(document.entries[1].gapAfter, "\n---\n", "One and Two were neighbours before, so their gap is kept")
        XCTAssertEqual(document.entries[2].gapAfter, "", "nothing follows the last slide")
    }

    func testAnEmptyDeckIsAllPrefix() {
        let document = SlideDocument(text: "---\ntitle: x\n---\n", boxes: [])
        XCTAssertEqual(document.prefix, "---\ntitle: x\n---\n")
        XCTAssertTrue(document.entries.isEmpty)
        XCTAssertEqual(document.boxes, [])
    }
}
~~~~

- [ ] **Step 3: Write the failing editing tests**

`desktop/TapDesktopCore/Tests/TapDesktopCoreTests/SlideEditingTests.swift`:

~~~~swift
import XCTest
@testable import TapDesktopCore

final class SlideEditingTests: XCTestCase {
    /// Seven one-line slides after a frontmatter. Slide 4 holds a fence with a "---" line.
    static let text = """
    ---
    title: Ops
    ---

    # One

    ---

    # Two

    ---

    <!-- layout: section -->
    # Three

    ---

    # Four

    ```text
    ---
    ```

    ---

    # Five

    ---

    # Six

    ---

    # Seven

    """

    static let boxes: [SlideBox] = {
        let nsText = text as NSString
        let starts = ["# One", "# Two", "<!-- layout: section -->", "# Four", "# Five", "# Six", "# Seven"]
        let ends = ["# One", "# Two", "# Three", "```\n\n---\n\n# Five", "# Five", "# Six", "# Seven"]
        return zip(starts, ends).enumerated().map { index, pair in
            let start = nsText.range(of: pair.0).location
            var end: Int
            if index == 3 {
                end = nsText.range(of: "```\n\n---\n\n# Five").location + 3
            } else {
                end = NSMaxRange(nsText.range(of: pair.1, range: NSRange(location: start, length: nsText.length - start)))
            }
            return SlideBox(range: NSRange(location: start, length: end - start),
                            slide: Slide(number: index + 1, startLine: 0, endLine: 0, title: ["One", "Two", "Three", "Four", "Five", "Six", "Seven"][index]))
        }
    }()

    func titles(_ result: SlideEditResult?) -> [String] {
        (result?.boxes ?? []).map(\.slide.title)
    }

    /// "---" lines outside every slide's range: the frontmatter's two and
    /// one per boundary. A "---" inside a fenced block sits inside a
    /// slide's range and is never counted.
    func separatorLinesOutsideSlides(_ result: SlideEditResult) -> Int {
        let document = SlideDocument(text: result.text, boxes: result.boxes)
        func count(_ string: String) -> Int {
            string.components(separatedBy: "\n").filter { $0.trimmingCharacters(in: .whitespaces) == "---" }.count
        }
        return count(document.prefix) + document.entries.map { count($0.gapAfter) }.reduce(0, +) + count(document.suffix)
    }

    func testMoveOneSlideAboveAnother() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.move(numbers: [5], beforeNumber: 3), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(result), ["One", "Two", "Five", "Three", "Four", "Six", "Seven"])
        XCTAssertEqual(result.selectedNumbers, [3])
        XCTAssertEqual(result.sourceNumbers, [1, 2, 5, 3, 4, 6, 7])
        XCTAssertEqual(separatorLinesOutsideSlides(result), 2 + 7 - 1, "the frontmatter's two and one between each pair; the fence's --- is inside slide 4")
        XCTAssertTrue(result.boxes.map(\.slide.number).allSatisfy { number in
            let document = SlideDocument(text: result.text, boxes: result.boxes)
            return number == 7 || document.entries[number - 1].gapAfter.components(separatedBy: "\n").filter { $0 == "---" }.count == 1
        }, "exactly one --- in every gap")
        XCTAssertEqual(result.caret, result.boxes[2].range.location)
        XCTAssertTrue(result.text.hasPrefix("---\ntitle: Ops\n---\n\n# One"), "the frontmatter never moves")
    }

    func testMoveSeveralSlidesKeepsTheirOrderAsOneBlock() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.move(numbers: [6, 5], beforeNumber: 3), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(result), ["One", "Two", "Five", "Six", "Three", "Four", "Seven"])
        XCTAssertEqual(result.selectedNumbers, [3, 4])
    }

    func testMoveToTheEndAndToTheTop() throws {
        let toEnd = try XCTUnwrap(SlideEditing.apply(.move(numbers: [2], beforeNumber: nil), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(toEnd), ["One", "Three", "Four", "Five", "Six", "Seven", "Two"])
        XCTAssertFalse(toEnd.text.hasSuffix("---\n"), "no separator dangles after the last slide")
        let toTop = try XCTUnwrap(SlideEditing.apply(.move(numbers: [7], beforeNumber: 1), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(toTop), ["Seven", "One", "Two", "Three", "Four", "Five", "Six"])
        XCTAssertTrue(toTop.text.hasPrefix("---\ntitle: Ops\n---\n\n# Seven\n\n---\n\n# One"))
    }

    func testAMoveThatChangesNothingIsRefused() {
        XCTAssertNil(SlideEditing.apply(.move(numbers: [3], beforeNumber: 4), to: Self.text, boxes: Self.boxes))
        XCTAssertNil(SlideEditing.apply(.move(numbers: [3], beforeNumber: 3), to: Self.text, boxes: Self.boxes))
        XCTAssertNil(SlideEditing.apply(.move(numbers: [7], beforeNumber: nil), to: Self.text, boxes: Self.boxes))
        XCTAssertNil(SlideEditing.apply(.move(numbers: [9], beforeNumber: 1), to: Self.text, boxes: Self.boxes))
        XCTAssertNil(SlideEditing.apply(.move(numbers: [], beforeNumber: 1), to: Self.text, boxes: Self.boxes))
    }

    func testAFenceWithASeparatorLineMovesWhole() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.move(numbers: [4], beforeNumber: 1), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(result), ["Four", "One", "Two", "Three", "Five", "Six", "Seven"])
        let four = (result.text as NSString).substring(with: result.boxes[0].range)
        XCTAssertEqual(four, "# Four\n\n```text\n---\n```", "the fence and its --- line stay inside the slide")
    }

    func testMoveCarriesTheCaretOffset() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.move(numbers: [3], beforeNumber: 2), to: Self.text, boxes: Self.boxes, caretOffsetInSlide: 30))
        XCTAssertEqual(result.caret, result.boxes[1].range.location + 30)
        let clamped = try XCTUnwrap(SlideEditing.apply(.move(numbers: [3], beforeNumber: 2), to: Self.text, boxes: Self.boxes, caretOffsetInSlide: 500))
        XCTAssertEqual(clamped.caret, NSMaxRange(clamped.boxes[1].range), "an offset past the slide's end lands at its end")
    }

    func testDuplicatePutsCopiesAfterTheLastSelectedSlide() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.duplicate(numbers: [3]), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(result), ["One", "Two", "Three", "Three", "Four", "Five", "Six", "Seven"])
        XCTAssertEqual(result.selectedNumbers, [4])
        XCTAssertEqual(result.sourceNumbers, [1, 2, 3, nil, 4, 5, 6, 7], "the copy has no image yet")
        XCTAssertEqual((result.text as NSString).substring(with: result.boxes[3].range), "<!-- layout: section -->\n# Three")
        let several = try XCTUnwrap(SlideEditing.apply(.duplicate(numbers: [2, 3]), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(several), ["One", "Two", "Three", "Two", "Three", "Four", "Five", "Six", "Seven"])
        XCTAssertEqual(several.selectedNumbers, [4, 5])
    }

    func testDeleteRemovesSlidesAndTheirSeparators() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.delete(numbers: [5, 6]), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(result), ["One", "Two", "Three", "Four", "Seven"])
        XCTAssertEqual(separatorLinesOutsideSlides(result), 2 + 4)
        XCTAssertEqual(result.selectedNumbers, [5], "the slide now at the first deleted position")
        let last = try XCTUnwrap(SlideEditing.apply(.delete(numbers: [7]), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(last.selectedNumbers, [6])
        XCTAssertFalse(last.text.contains("# Six\n\n---\n\n\n"), "the separator before the deleted last slide goes with it")
    }

    func testDeletingEverySlideIsRefused() {
        XCTAssertNil(SlideEditing.apply(.delete(numbers: Array(1...7)), to: Self.text, boxes: Self.boxes))
    }

    func testInsertAfterTheFrontmatterAndAfterASlide() throws {
        let template = "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n"
        let top = try XCTUnwrap(SlideEditing.apply(.insert(markdowns: [template], beforeNumber: 1), to: Self.text, boxes: Self.boxes))
        XCTAssertTrue(top.text.hasPrefix("---\ntitle: Ops\n---\n\n<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n\n---\n\n# One"))
        XCTAssertEqual(top.selectedNumbers, [1])
        XCTAssertEqual(top.boxes.count, 8)
        let after3 = try XCTUnwrap(SlideEditing.apply(.insert(markdowns: [template], beforeNumber: 4), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(after3.boxes[3].slide.number, 4)
        XCTAssertEqual((after3.text as NSString).substring(with: after3.boxes[3].range), "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription")
        XCTAssertEqual(SlideEditing.actionName(for: .insert(markdowns: [template], beforeNumber: nil)), "New Slide")
    }

    func testInsertIntoAnEmptyDeck() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.insert(markdowns: ["# First\n"], beforeNumber: nil), to: "---\ntitle: x\n---\n", boxes: []))
        XCTAssertEqual(result.text, "---\ntitle: x\n---\n\n# First")
        XCTAssertEqual(result.boxes.count, 1)
    }

    func testSetSkipRewritesTheDirectiveComment() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.setSkip(numbers: [3, 4], skipped: true), to: Self.text, boxes: Self.boxes))
        let text = result.text as NSString
        XCTAssertEqual(text.substring(with: result.boxes[2].range), "<!--\nlayout: section\nskip: true\n-->\n# Three")
        XCTAssertEqual(text.substring(with: result.boxes[3].range), "<!--\nskip: true\n-->\n\n# Four\n\n```text\n---\n```")
        let back = try XCTUnwrap(SlideEditing.apply(.setSkip(numbers: [3, 4], skipped: false), to: result.text, boxes: result.boxes))
        XCTAssertEqual(back.text, Self.text.replacingOccurrences(of: "<!-- layout: section -->", with: "<!--\nlayout: section\n-->"))
        XCTAssertNil(SlideEditing.apply(.setSkip(numbers: [1], skipped: false), to: Self.text, boxes: Self.boxes), "unskipping a slide that is not skipped changes nothing")
    }

    func testActionNames() {
        XCTAssertEqual(SlideEditing.actionName(for: .move(numbers: [1], beforeNumber: nil)), "Move Slide")
        XCTAssertEqual(SlideEditing.actionName(for: .move(numbers: [1, 2], beforeNumber: nil)), "Move 2 Slides")
        XCTAssertEqual(SlideEditing.actionName(for: .duplicate(numbers: [1])), "Duplicate Slide")
        XCTAssertEqual(SlideEditing.actionName(for: .delete(numbers: [1, 2, 3])), "Delete 3 Slides")
        XCTAssertEqual(SlideEditing.actionName(for: .insert(markdowns: ["a", "b"], beforeNumber: nil)), "Insert 2 Slides")
        XCTAssertEqual(SlideEditing.actionName(for: .setSkip(numbers: [1], skipped: true)), "Skip Slide")
        XCTAssertEqual(SlideEditing.actionName(for: .setSkip(numbers: [1, 2], skipped: false)), "Unskip 2 Slides")
    }

    func testFirstSlotRangeSelectsThePlaceholderAfterTheMarker() {
        XCTAssertEqual(SlideEditing.firstSlotRange(inSlideText: "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription"),
                       NSRange(location: 29, length: 4), "the text after \"# \"")
        XCTAssertEqual(SlideEditing.firstSlotRange(inSlideText: "::left\n\nLeft content\n\n::right\n\nRight content"),
                       NSRange(location: 8, length: 12), "a slot marker is skipped")
        XCTAssertEqual(SlideEditing.firstSlotRange(inSlideText: "<!--\nlayout: code-focus\n-->\n\n```\n// Your code here\n```"),
                       NSRange(location: 33, length: 17), "inside a fence, the first code line")
        XCTAssertEqual(SlideEditing.firstSlotRange(inSlideText: ""), NSRange(location: 0, length: 0))
    }
}
~~~~

- [ ] **Step 4: Run the tests to verify they fail**

Run: `make -C desktop core-test`
Expected: compile failure, `cannot find 'SlideDocument' in scope`.

- [ ] **Step 5: Write `SlideDocument.swift`**

```swift
import Foundation

/// The buffer cut along tap's slide ranges: the text before slide 1
/// (frontmatter and blank lines), each slide's own text with the gap that
/// follows it (blank lines and the one "---" separator line), and the text
/// after the last slide. Joining the pieces back gives the buffer exactly.
/// Slide boundaries come from tap's boxes; this type never looks for one.
public struct SlideDocument: Equatable, Sendable {
    public struct Entry: Equatable, Sendable {
        /// tap's description of the slide. `number` is the slide's number
        /// before the edit, and 0 for a slide the edit created.
        public var slide: Slide
        public var text: String
        /// The text between this slide and the next; "" after the last.
        public var gapAfter: String

        public init(slide: Slide, text: String, gapAfter: String) {
            self.slide = slide
            self.text = text
            self.gapAfter = gapAfter
        }
    }

    /// The gap written between two slides that were not neighbours before.
    public static let separator = "\n\n---\n\n"

    public var prefix: String
    public var entries: [Entry]
    public var suffix: String

    public init(prefix: String, entries: [Entry], suffix: String) {
        self.prefix = prefix
        self.entries = entries
        self.suffix = suffix
    }

    /// Cuts `text` along `boxes`, tap's boxes for that text, in order.
    public init(text: String, boxes: [SlideBox]) {
        let nsText = text as NSString
        guard let first = boxes.first, let last = boxes.last else {
            prefix = text
            entries = []
            suffix = ""
            return
        }
        prefix = nsText.substring(to: min(first.range.location, nsText.length))
        var cut: [Entry] = []
        for (index, box) in boxes.enumerated() {
            let range = NSIntersectionRange(box.range, NSRange(location: 0, length: nsText.length))
            let nextStart = index + 1 < boxes.count ? boxes[index + 1].range.location : NSMaxRange(range)
            let gap = NSRange(location: NSMaxRange(range), length: max(0, min(nextStart, nsText.length) - NSMaxRange(range)))
            cut.append(Entry(slide: box.slide, text: nsText.substring(with: range), gapAfter: nsText.substring(with: gap)))
        }
        entries = cut
        suffix = nsText.substring(from: min(last.end, nsText.length))
    }

    /// The buffer these pieces make.
    public var text: String {
        prefix + entries.map { $0.text + $0.gapAfter }.joined() + suffix
    }

    /// The boxes of `text`: one per entry, numbered 1 to n, with the line
    /// numbers tap would report for the same text.
    public var boxes: [SlideBox] {
        var location = (prefix as NSString).length
        var line = 1 + Self.newlineCount(prefix)
        var boxes: [SlideBox] = []
        for (index, entry) in entries.enumerated() {
            let length = (entry.text as NSString).length
            let newlines = Self.newlineCount(entry.text)
            let old = entry.slide
            let slide = Slide(number: index + 1, startLine: line, endLine: line + newlines, layout: old.layout, title: old.title,
                              fragments: old.fragments, steps: old.steps, skip: old.skip, errors: old.errors, codeBlocks: old.codeBlocks)
            boxes.append(SlideBox(range: NSRange(location: location, length: length), slide: slide))
            location += length + (entry.gapAfter as NSString).length
            line += newlines + Self.newlineCount(entry.gapAfter)
        }
        return boxes
    }

    /// Gives every boundary between two slides that were not neighbours
    /// before the edit the canonical separator, keeps the gap between two
    /// that were, and leaves nothing after the last slide. Entries carry
    /// their old numbers, so old neighbours are the ones whose numbers are
    /// consecutive; a created slide, numbered 0, was nobody's neighbour.
    public mutating func normalizeGaps() {
        for index in entries.indices {
            guard index + 1 < entries.count else {
                entries[index].gapAfter = ""
                continue
            }
            let current = entries[index].slide.number
            let next = entries[index + 1].slide.number
            let wereNeighbours = current > 0 && next == current + 1
            if !wereNeighbours || !entries[index].gapAfter.contains("---") {
                entries[index].gapAfter = Self.separator
            }
        }
    }

    static func newlineCount(_ string: String) -> Int {
        string.utf16.reduce(0) { $0 + ($1 == 10 ? 1 : 0) }
    }
}
```

- [ ] **Step 6: Write `SlideEditing.swift`**

```swift
import Foundation

/// A structural edit on the deck's slides. Numbers are the 1-based
/// numbers of the boxes the edit is applied to; `beforeNumber` names the
/// slide the moved or inserted slides go above, or the end when nil.
public enum SlideOperation: Equatable, Sendable {
    case move(numbers: [Int], beforeNumber: Int?)
    case duplicate(numbers: [Int])
    case delete(numbers: [Int])
    case insert(markdowns: [String], beforeNumber: Int?)
    case setSkip(numbers: [Int], skipped: Bool)
}

/// What an operation produces: the new text, the boxes of that text, the
/// new numbers of the slides the operation acted on, and where the caret
/// goes (inside the first of them).
public struct SlideEditResult: Equatable, Sendable {
    public let text: String
    public let boxes: [SlideBox]
    public let selectedNumbers: [Int]
    public let caret: Int
    /// For each slide of the new text, its number before the edit, or nil
    /// for a slide the edit created. The panel moves its images by this.
    public let sourceNumbers: [Int?]

    public init(text: String, boxes: [SlideBox], selectedNumbers: [Int], caret: Int, sourceNumbers: [Int?]) {
        self.text = text
        self.boxes = boxes
        self.selectedNumbers = selectedNumbers
        self.caret = caret
        self.sourceNumbers = sourceNumbers
    }
}

public enum SlideEditing {
    /// Applies `operation` to `text` cut along `boxes`. Returns nil when the
    /// operation is invalid (a number outside the deck, an empty selection,
    /// deleting every slide) or changes nothing, so the caller registers no
    /// undo step for it. `caretOffsetInSlide` is where the caret sat inside
    /// the first operated slide; it lands at the same offset afterwards.
    public static func apply(_ operation: SlideOperation, to text: String, boxes: [SlideBox], caretOffsetInSlide: Int = 0) -> SlideEditResult? {
        var document = SlideDocument(text: text, boxes: boxes)
        let count = document.entries.count
        func validated(_ numbers: [Int]) -> [Int]? {
            let sorted = Array(Set(numbers)).sorted()
            guard count > 0, !sorted.isEmpty, sorted.allSatisfy({ $0 >= 1 && $0 <= count }) else { return nil }
            return sorted
        }
        var selected: [Int] = []

        switch operation {
        case .move(let numbers, let beforeNumber):
            guard let numbers = validated(numbers) else { return nil }
            if let beforeNumber, !(1...count).contains(beforeNumber) || numbers.contains(beforeNumber) { return nil }
            let moving = numbers.map { document.entries[$0 - 1] }
            var remaining = document.entries.enumerated().filter { !numbers.contains($0.offset + 1) }.map(\.element)
            let insertionIndex = beforeNumber.flatMap { before in remaining.firstIndex { $0.slide.number == before } } ?? remaining.count
            remaining.insert(contentsOf: moving, at: insertionIndex)
            guard remaining.map(\.slide.number) != document.entries.map(\.slide.number) else { return nil }
            document.entries = remaining
            selected = Array(insertionIndex + 1 ... insertionIndex + moving.count)

        case .duplicate(let numbers):
            guard let numbers = validated(numbers) else { return nil }
            let copies = numbers.map { number -> SlideDocument.Entry in
                var copy = document.entries[number - 1]
                copy.slide = created(from: copy.slide)
                copy.gapAfter = SlideDocument.separator
                return copy
            }
            let insertionIndex = numbers[numbers.count - 1]
            document.entries.insert(contentsOf: copies, at: insertionIndex)
            selected = Array(insertionIndex + 1 ... insertionIndex + copies.count)

        case .delete(let numbers):
            guard let numbers = validated(numbers), numbers.count < count else { return nil }
            document.entries = document.entries.enumerated().filter { !numbers.contains($0.offset + 1) }.map(\.element)
            selected = [min(numbers[0], document.entries.count)]

        case .insert(let markdowns, let beforeNumber):
            guard !markdowns.isEmpty else { return nil }
            if let beforeNumber, beforeNumber < 1 || beforeNumber > count { return nil }
            let created = markdowns.map { markdown in
                SlideDocument.Entry(slide: Slide(number: 0, startLine: 0, endLine: 0, layout: "", title: ""),
                                    text: markdown.trimmingCharacters(in: .whitespacesAndNewlines),
                                    gapAfter: SlideDocument.separator)
            }
            let insertionIndex = beforeNumber.map { $0 - 1 } ?? count
            document.entries.insert(contentsOf: created, at: insertionIndex)
            if count == 0, !document.prefix.isEmpty, !document.prefix.hasSuffix("\n\n") {
                document.prefix += document.prefix.hasSuffix("\n") ? "\n" : "\n\n"
            }
            selected = Array(insertionIndex + 1 ... insertionIndex + created.count)

        case .setSkip(let numbers, let skipped):
            guard let numbers = validated(numbers) else { return nil }
            for number in numbers {
                let entry = document.entries[number - 1]
                document.entries[number - 1].text = DirectiveComment.rewrite(slideText: entry.text, setting: "skip", to: skipped ? "true" : nil)
            }
            selected = numbers
        }

        let sourceNumbers = document.entries.map { $0.slide.number > 0 ? Optional($0.slide.number) : nil }
        document.normalizeGaps()
        let newText = document.text
        guard newText != text else { return nil }
        let newBoxes = document.boxes
        let target = newBoxes[selected[0] - 1]
        let caret = target.range.location + min(max(0, caretOffsetInSlide), target.range.length)
        return SlideEditResult(text: newText, boxes: newBoxes, selectedNumbers: selected, caret: caret, sourceNumbers: sourceNumbers)
    }

    /// The undo menu's name for an operation.
    public static func actionName(for operation: SlideOperation) -> String {
        func plural(_ verb: String, _ count: Int) -> String { count == 1 ? "\(verb) Slide" : "\(verb) \(count) Slides" }
        switch operation {
        case .move(let numbers, _): return plural("Move", Set(numbers).count)
        case .duplicate(let numbers): return plural("Duplicate", Set(numbers).count)
        case .delete(let numbers): return plural("Delete", Set(numbers).count)
        case .insert(let markdowns, _): return markdowns.count == 1 ? "New Slide" : "Insert \(markdowns.count) Slides"
        case .setSkip(let numbers, let skipped): return plural(skipped ? "Skip" : "Unskip", Set(numbers).count)
        }
    }

    /// The range to select in a freshly inserted slide so typing replaces
    /// its first placeholder: the first content line after the directive
    /// comment and any slot marker, without its markdown prefix; inside a
    /// fence, the first code line. An empty slide gives an empty range at 0.
    public static func firstSlotRange(inSlideText slideText: String) -> NSRange {
        let text = slideText as NSString
        var location = 0
        if let comment = DirectiveComment.leading(in: slideText) {
            // The line that holds "-->" is the comment's, not content.
            location = NSMaxRange(text.lineRange(for: NSRange(location: NSMaxRange(comment.range), length: 0)))
        }
        var insideFence = false
        while location < text.length {
            let lineRange = text.lineRange(for: NSRange(location: location, length: 0))
            let line = text.substring(with: lineRange).trimmingCharacters(in: .newlines)
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            location = NSMaxRange(lineRange)
            if trimmed.hasPrefix("```") {
                insideFence.toggle()
                continue
            }
            if trimmed.isEmpty || (!insideFence && trimmed.hasPrefix("::")) { continue }
            let prefixLength = insideFence ? 0 : markdownPrefixLength(of: line)
            let contentLength = (line as NSString).length - prefixLength
            return NSRange(location: lineRange.location + prefixLength, length: contentLength)
        }
        return NSRange(location: 0, length: 0)
    }

    private static let markdownPrefix = try! NSRegularExpression(pattern: #"^(#{1,6} |> |- |\* |\d+\. |> "|")"#)

    static func markdownPrefixLength(of line: String) -> Int {
        let nsLine = line as NSString
        guard let match = markdownPrefix.firstMatch(in: line, range: NSRange(location: 0, length: nsLine.length)) else { return 0 }
        return match.range.length
    }

    /// A copy's metadata: the same slide, numbered 0 as a slide the edit created.
    private static func created(from slide: Slide) -> Slide {
        Slide(number: 0, startLine: 0, endLine: 0, layout: slide.layout, title: slide.title, fragments: slide.fragments,
              steps: slide.steps, skip: slide.skip, errors: slide.errors, codeBlocks: slide.codeBlocks)
    }
}
```

`DirectiveComment` comes in Task 2; until then, add this stub to `DirectiveComment.swift` so the package compiles, and replace it in Task 2:

```swift
import Foundation

public struct DirectiveComment: Equatable, Sendable {
    public let range: NSRange
    public let body: String
    public static func leading(in slideText: String) -> DirectiveComment? { nil }
    public static func rewrite(slideText: String, setting key: String, to value: String?) -> String { slideText }
}
```

- [ ] **Step 7: Run the tests**

Run: `make -C desktop core-test`
Expected: every `SlideDocumentTests` test passes. In `SlideEditingTests` everything passes except two tests that need the real `DirectiveComment`: `testSetSkipRewritesTheDirectiveComment`, and `testFirstSlotRangeSelectsThePlaceholderAfterTheMarker` on its big-stat and code-focus cases (the stub's `leading` returns nil, so the comment is not skipped). Task 2 turns them green. Do not weaken the tests to pass now.

- [ ] **Step 8: Mutate and commit**

Mutation: in `normalizeGaps`, change `next == current + 1` to `next == current`. Run the tests. Expected: `testNormalizeGapsRewritesOnlyNewBoundaries` fails on the kept gap, and `testMoveOneSlideAboveAnother` on the gap count (every gap is rewritten, so the kept "\n---\n" form disappears and a moved block loses its inner gap). Revert the mutation and run again to confirm green.

```bash
git add desktop/TapDesktopCore
git commit -m "feat(desktop): cut the buffer along tap's ranges and apply slide operations to it"
```

---

### Task 2: The directive comment

**Files:**
- Modify: `desktop/TapDesktopCore/Sources/TapDesktopCore/DirectiveComment.swift` (replace the Task 1 stub)
- Test: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/DirectiveCommentTests.swift`

**Interfaces:**
- Consumes: tap's rule for the directive comment, `directivePattern` in `internal/parser/parser.go`: an HTML comment at the very start of the slide after optional whitespace; its body is YAML, or a mix of `key: value` lines and a `notes:` block whose free text runs to the next directive line (`splitMixedDirectiveComment`).
- Produces: `struct DirectiveComment { range: NSRange; body: String }`, `static func leading(in:) -> DirectiveComment?`, `static func rewrite(slideText:setting:to:) -> String`.

- [ ] **Step 1: Write the failing tests**

`desktop/TapDesktopCore/Tests/TapDesktopCoreTests/DirectiveCommentTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class DirectiveCommentTests: XCTestCase {
    func testFindsTheLeadingCommentInBothForms() throws {
        let single = try XCTUnwrap(DirectiveComment.leading(in: "<!-- layout: title -->\n\n# Hello"))
        XCTAssertEqual(single.range, NSRange(location: 0, length: 22))
        XCTAssertEqual(single.body, " layout: title ")
        let multi = try XCTUnwrap(DirectiveComment.leading(in: "\n<!--\nlayout: cover\nbackground: images/hero.jpg\n-->\n# Big"))
        XCTAssertEqual(multi.range, NSRange(location: 1, length: 50))
        XCTAssertEqual(multi.body, "\nlayout: cover\nbackground: images/hero.jpg\n")
    }

    func testOnlyACommentAtTheStartIsTheDirectiveComment() {
        XCTAssertNil(DirectiveComment.leading(in: "# Hello\n\n<!-- layout: title -->"))
        XCTAssertNil(DirectiveComment.leading(in: "# Hello\n\n<!-- pause -->"))
        XCTAssertNil(DirectiveComment.leading(in: "<!-- never closed"))
        XCTAssertNil(DirectiveComment.leading(in: ""))
    }

    func testSettingAKeyAddsOneLineAndKeepsTheRest() {
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "<!-- layout: title -->\n\n# Hello", setting: "skip", to: "true"),
                       "<!--\nlayout: title\nskip: true\n-->\n\n# Hello", "a single-line comment becomes the multi-line form")
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "# Hello\n\nBody", setting: "skip", to: "true"),
                       "<!--\nskip: true\n-->\n\n# Hello\n\nBody")
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "<!--\nskip: false\n-->\n# Hello", setting: "skip", to: "true"),
                       "<!--\nskip: true\n-->\n# Hello", "an existing key line is replaced, not doubled")
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "<!--\n  layout: cover\n  background:\n    image: hero.jpg\n-->\n# Big", setting: "skip", to: "true"),
                       "<!--\n  layout: cover\n  background:\n    image: hero.jpg\nskip: true\n-->\n# Big", "every other line stays byte for byte, indentation included")
    }

    func testAKeyGoesBeforeTheNotesBlockAndNeverTouchesIt() {
        let slide = "<!--\nlayout: default\nnotes:\nLet the room guess.\n\nskip: this is a sentence in the notes\n-->\n\n## What We Knew"
        XCTAssertEqual(DirectiveComment.rewrite(slideText: slide, setting: "skip", to: "true"),
                       "<!--\nlayout: default\nskip: true\nnotes:\nLet the room guess.\n\nskip: this is a sentence in the notes\n-->\n\n## What We Knew",
                       "the key goes before notes:, and the notes keep their blank line and their own skip: line")
        let block = "<!--\nnotes: |\n  Line one.\n\n  Line two.\n-->\n# H"
        XCTAssertEqual(DirectiveComment.rewrite(slideText: block, setting: "skip", to: "true"),
                       "<!--\nskip: true\nnotes: |\n  Line one.\n\n  Line two.\n-->\n# H", "a block scalar keeps its indentation")
        XCTAssertEqual(DirectiveComment.rewrite(slideText: DirectiveComment.rewrite(slideText: block, setting: "skip", to: "true"), setting: "skip", to: nil),
                       block, "removing the key gives the original back")
    }

    func testRemovingAKeyDropsAnEmptyComment() {
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "<!--\nskip: true\n-->\n\n# Hello", setting: "skip", to: nil), "# Hello")
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "<!--\nlayout: title\nskip: true\n-->\n\n# Hello", setting: "skip", to: nil),
                       "<!--\nlayout: title\n-->\n\n# Hello")
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "<!-- skip: true -->\n# Hello", setting: "skip", to: nil), "# Hello")
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "# Hello", setting: "skip", to: nil), "# Hello")
    }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `make -C desktop core-test`
Expected: the five tests fail on the stub (`leading` returns nil, `rewrite` returns its input).

- [ ] **Step 3: Write `DirectiveComment.swift`**

```swift
import Foundation

/// A slide's directive comment: the HTML comment at the very start of the
/// slide's text, which tap reads its per-slide directives from (`layout`,
/// `skip`, `steps`, `transition`, `notes`, ...). This matches what
/// `directivePattern` in internal/parser/parser.go matches: optional
/// whitespace, `<!--`, the body, `-->`. A comment anywhere else, such as
/// `<!-- pause -->`, is not a directive comment.
public struct DirectiveComment: Equatable, Sendable {
    /// The whole comment's range in the slide text, from `<!--` to `-->`.
    public let range: NSRange
    /// The text between `<!--` and `-->`, byte for byte.
    public let body: String

    public init(range: NSRange, body: String) {
        self.range = range
        self.body = body
    }

    public static func leading(in slideText: String) -> DirectiveComment? {
        let text = slideText as NSString
        var start = 0
        while start < text.length, let scalar = Unicode.Scalar(text.character(at: start)), CharacterSet.whitespacesAndNewlines.contains(scalar) {
            start += 1
        }
        guard start + 4 <= text.length, text.substring(with: NSRange(location: start, length: 4)) == "<!--" else { return nil }
        let searchRange = NSRange(location: start + 4, length: text.length - start - 4)
        let close = text.range(of: "-->", range: searchRange)
        guard close.location != NSNotFound else { return nil }
        let body = text.substring(with: NSRange(location: start + 4, length: close.location - start - 4))
        return DirectiveComment(range: NSRange(location: start, length: NSMaxRange(close) - start), body: body)
    }

    /// The slide text with `key` set to `value` in its directive comment,
    /// or removed when `value` is nil. Only the one top-level `key:` line
    /// is added, replaced or removed; every other line of the comment stays
    /// byte for byte, so block scalars, nested maps and the notes' own
    /// lines are untouched. The key goes before a `notes:` line, because
    /// tap's mixed comment rule ends the notes text at the next directive
    /// line, and a line after `notes:` is never treated as the key. A
    /// single-line comment is written back in the multi-line form. A slide
    /// without a comment gets one and a blank line; a comment left empty
    /// is removed with the blank lines after it.
    public static func rewrite(slideText: String, setting key: String, to value: String?) -> String {
        let text = slideText as NSString
        let existing = leading(in: slideText)
        // Lines of the body between the "<!--" line and the "-->" line.
        var lines: [String]
        if let existing {
            if existing.body.contains("\n") {
                var parts = existing.body.components(separatedBy: "\n")
                // The first part is what follows "<!--" on its line and the last what precedes "-->"; both are blank in the multi-line form.
                if parts.first?.trimmingCharacters(in: .whitespaces).isEmpty == true { parts.removeFirst() } 
                if parts.last?.trimmingCharacters(in: .whitespaces).isEmpty == true { parts.removeLast() }
                lines = parts
            } else {
                let single = existing.body.trimmingCharacters(in: .whitespaces)
                lines = single.isEmpty ? [] : [single]
            }
        } else {
            lines = []
        }
        let notesIndex = lines.firstIndex { $0.hasPrefix("notes:") }
        let topLevel = notesIndex.map { Array(lines[..<$0]) } ?? lines
        let keyIndex = topLevel.firstIndex { $0.hasPrefix(key + ":") }
        if let keyIndex { lines.remove(at: keyIndex) }
        if let value {
            // The old key's place, else before notes:, else the end.
            let position = keyIndex ?? lines.firstIndex { $0.hasPrefix("notes:") } ?? lines.count
            lines.insert("\(key): \(value)", at: position)
        }
        if lines.allSatisfy({ $0.trimmingCharacters(in: .whitespaces).isEmpty }) {
            guard let existing else { return slideText }
            var end = NSMaxRange(existing.range)
            while end < text.length, text.character(at: end) == 10 { end += 1 }
            return text.replacingCharacters(in: NSRange(location: existing.range.location, length: end - existing.range.location), with: "")
        }
        let comment = "<!--\n" + lines.joined(separator: "\n") + "\n-->"
        if let existing {
            return text.replacingCharacters(in: existing.range, with: comment)
        }
        return comment + "\n\n" + slideText
    }
}
```

- [ ] **Step 4: Run the tests**

Run: `make -C desktop core-test`
Expected: all of `DirectiveCommentTests`, `SlideEditingTests.testSetSkipRewritesTheDirectiveComment` and `testFirstSlotRangeSelectsThePlaceholderAfterTheMarker` pass. The whole package is green.

- [ ] **Step 5: Mutate and commit**

Mutations, each reverted: make `leading` accept any text without the `"<!--"` guard (expected: `testOnlyACommentAtTheStartIsTheDirectiveComment` fails); trim every line in `rewrite` before joining (expected: `testSettingAKeyAddsOneLineAndKeepsTheRest` fails on the indented map and `testAKeyGoesBeforeTheNotesBlockAndNeverTouchesIt` on the block scalar); search the key line in all lines rather than the top-level ones (expected: the notes' own `skip:` sentence is deleted and that test fails).

```bash
git add desktop/TapDesktopCore
git commit -m "feat(desktop): read and rewrite a slide's directive comment for skip"
```

---

### Task 3: Adopting boxes, the drag payload, the panel state, accessibility labels and the divider with a sidebar

**Files:**
- Modify: `desktop/TapDesktopCore/Sources/TapDesktopCore/SlideRangeTracker.swift`
- Modify: `desktop/TapDesktopCore/Sources/TapDesktopCore/DividerPolicy.swift`
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/SlideDragPayload.swift`, `SlideAccessibility.swift`, `SlidePanelState.swift`
- Test: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/SlideRangeTrackerTests.swift` (add), `DividerPolicyTests.swift` (add), `SlideDragPayloadTests.swift`, `SlideAccessibilityTests.swift`, `SlidePanelStateTests.swift`

**Interfaces:**
- Consumes: `SlideRangeTracker`, `DividerPolicy`, `FilePaths.canonical`, `BoxHeader.revealCount` (D2).
- Produces:
  - `SlideRangeTracker.adopt(_ boxes: [SlideBox])`: replaces the boxes with ranges the app built from tap's own ranges, keeps the deck errors, and refuses every answer to a send begun before it (the edit log is cleared and `earliestLiveGeneration` moves past the current generation, as `reset` does), so the operation's own send is the next answer that lands.
  - `DividerPolicy.balancedPosition(totalWidth:dividerThickness:leadingWidth:) -> CGFloat?`: the editor and the right pane split what is left after a sidebar of `leadingWidth` and its divider.
  - `struct SlideDragPayload: Codable, Equatable { deckPath: String; slideNumbers: [Int]; markdowns: [String] }`, `static let pasteboardType = "io.geocod.tap.slides"`, `func data() throws -> Data`, `init?(data:)`.
  - `enum SlideAccessibility { static func label(for slide: Slide) -> String; static func dropLabel(beforeNumber: Int?, count: Int) -> String }`.
  - `struct SlidePanelState { init(defaults: UserDefaults); func isPinned(deck: URL) -> Bool; func setPinned(_:deck:); func moveState(from:to:) }`: pinned on first launch, remembered per deck, following a rename.

- [ ] **Step 1: Write the failing tests**

Add to `SlideRangeTrackerTests.swift`:

```swift
    func testAdoptRefusesAnswersToEarlierSends() {
        var tracker = SlideRangeTracker()
        let text = "# A\n\n---\n\n# B"
        _ = tracker.beginSend()
        _ = tracker.apply(SlideList(slides: [Slide(number: 1, startLine: 1, endLine: 1, title: "A"), Slide(number: 2, startLine: 5, endLine: 5, title: "B")], errors: ["x"]),
                          sentText: text, sentGeneration: tracker.generation, currentLength: (text as NSString).length)
        // A PUT of the old text is in flight when the operation permutes the boxes.
        let inFlight = tracker.beginSend()
        tracker.recordEdit(location: 0, oldLength: 13, newLength: 13)
        tracker.adopt([SlideBox(range: NSRange(location: 0, length: 3), slide: Slide(number: 1, startLine: 1, endLine: 1, title: "B")),
                       SlideBox(range: NSRange(location: 10, length: 3), slide: Slide(number: 2, startLine: 5, endLine: 5, title: "A"))])
        XCTAssertEqual(tracker.boxes.map(\.slide.title), ["B", "A"])
        XCTAssertEqual(tracker.deckErrors, ["x"], "adopting boxes says nothing about the deck's errors")

        let stale = tracker.apply(SlideList(slides: [Slide(number: 1, startLine: 1, endLine: 1, title: "A"), Slide(number: 2, startLine: 5, endLine: 5, title: "B")], errors: []),
                                  sentText: text, sentGeneration: inFlight, currentLength: 13)
        XCTAssertNil(stale, "the answer to a send begun before the adoption describes text the operation replaced")
        XCTAssertEqual(tracker.boxes.map(\.slide.title), ["B", "A"], "the adopted order survives it")

        // The operation's own send, begun after, is applied.
        let own = tracker.beginSend()
        let applied = tracker.apply(SlideList(slides: [Slide(number: 1, startLine: 1, endLine: 1, title: "B"), Slide(number: 2, startLine: 5, endLine: 5, title: "A")], errors: []),
                                    sentText: "# B\n\n---\n\n# A", sentGeneration: own, currentLength: 13)
        XCTAssertNotNil(applied)
        XCTAssertEqual(tracker.boxes.map(\.slide.title), ["B", "A"])
        XCTAssertEqual(tracker.boxes[1].range, NSRange(location: 10, length: 3))
    }

    func testAdoptSeparatesOverlappingBoxes() {
        var tracker = SlideRangeTracker()
        tracker.adopt([SlideBox(range: NSRange(location: 0, length: 10), slide: Slide(number: 1, startLine: 1, endLine: 1)),
                       SlideBox(range: NSRange(location: 8, length: 10), slide: Slide(number: 2, startLine: 2, endLine: 2))])
        XCTAssertEqual(tracker.boxes[1].range.location, 11)
    }
```

Add to `DividerPolicyTests.swift`:

```swift
    func testBalancesWhatIsLeftAfterASidebar() {
        let policy = DividerPolicy()
        XCTAssertEqual(policy.balancedPosition(totalWidth: 1000, dividerThickness: 1, leadingWidth: 0), 499)
        XCTAssertEqual(policy.balancedPosition(totalWidth: 1000, dividerThickness: 1, leadingWidth: 224),
                       224 + 1 + ((1000 - 224 - 1 - 1) / 2).rounded(.down), "the position is measured from the split view's left edge")
        var dragged = policy
        dragged.userDragged()
        XCTAssertNil(dragged.balancedPosition(totalWidth: 1000, dividerThickness: 1, leadingWidth: 224))
    }
```

`SlideDragPayloadTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class SlideDragPayloadTests: XCTestCase {
    func testRoundTripsThroughData() throws {
        let payload = SlideDragPayload(deckPath: "/talks/talk.md", slideNumbers: [5, 6], markdowns: ["# Five", "# Six\n\nBody"])
        let decoded = try XCTUnwrap(SlideDragPayload(data: try payload.data()))
        XCTAssertEqual(decoded, payload)
        XCTAssertNil(SlideDragPayload(data: Data("not json".utf8)))
        XCTAssertEqual(SlideDragPayload.pasteboardType, "io.geocod.tap.slides")
    }

    func testTellsItsOwnDeckFromAnotherThroughSymlinks() throws {
        let payload = SlideDragPayload(deckPath: "/tmp/talks/talk.md", slideNumbers: [1], markdowns: ["# One"])
        XCTAssertTrue(payload.comesFrom(deck: URL(fileURLWithPath: "/private/tmp/talks/talk.md")))
        XCTAssertFalse(payload.comesFrom(deck: URL(fileURLWithPath: "/tmp/talks/other.md")))
    }
}
```

`SlideAccessibilityTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class SlideAccessibilityTests: XCTestCase {
    func testTheLabelNamesNumberLayoutTitleAndSteps() {
        let slide = Slide(number: 3, startLine: 1, endLine: 2, layout: "default", title: "What We Knew", fragments: 1, steps: 1)
        XCTAssertEqual(SlideAccessibility.label(for: slide), "Slide 3, default layout, What We Knew, 2 steps")
        XCTAssertEqual(SlideAccessibility.label(for: Slide(number: 1, startLine: 1, endLine: 1, layout: "title", title: "Hello")), "Slide 1, title layout, Hello")
        XCTAssertEqual(SlideAccessibility.label(for: Slide(number: 4, startLine: 1, endLine: 1, layout: "code-focus", title: "", fragments: 0, steps: 1)),
                       "Slide 4, code-focus layout, 1 step")
        XCTAssertEqual(SlideAccessibility.label(for: Slide(number: 2, startLine: 1, endLine: 1, layout: "section", title: "The Page", skip: true)),
                       "Slide 2, section layout, The Page, skipped")
    }

    func testTheDropLabelNamesTheBoundary() {
        XCTAssertEqual(SlideAccessibility.dropLabel(beforeNumber: 3, count: 1), "Drop 1 slide above slide 3")
        XCTAssertEqual(SlideAccessibility.dropLabel(beforeNumber: nil, count: 2), "Drop 2 slides at the end")
    }
}
```

`SlidePanelStateTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class SlidePanelStateTests: XCTestCase {
    func testPinnedOnFirstLaunchAndRememberedPerDeck() throws {
        let suiteName = "SlidePanelStateTests.\(UUID().uuidString)"
        let defaults = try XCTUnwrap(UserDefaults(suiteName: suiteName))
        let state = SlidePanelState(defaults: defaults)
        let deckA = URL(fileURLWithPath: "/tmp/a.md")
        let deckB = URL(fileURLWithPath: "/tmp/b.md")
        XCTAssertTrue(state.isPinned(deck: deckA), "pinned on first launch, so people find the panel")
        state.setPinned(false, deck: deckA)
        XCTAssertFalse(state.isPinned(deck: deckA))
        XCTAssertFalse(state.isPinned(deck: URL(fileURLWithPath: "/private/tmp/a.md")), "the same deck through a symlink")
        XCTAssertTrue(state.isPinned(deck: deckB), "another deck starts from the default")
        state.moveState(from: deckA, to: deckB)
        XCTAssertFalse(state.isPinned(deck: deckB), "a renamed or saved-as deck keeps its state")
        XCTAssertTrue(state.isPinned(deck: deckA), "and the old key is gone")
        defaults.removePersistentDomain(forName: suiteName)
    }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `make -C desktop core-test`
Expected: compile failures on `adopt`, `leadingWidth:`, `SlideDragPayload`, `SlideAccessibility`, `SlidePanelState`.

- [ ] **Step 3: Add `adopt` to the tracker**

In `SlideRangeTracker.swift`, after `recordEdit`:

```swift
    /// Replaces the boxes with ranges the app built from tap's own ranges,
    /// as a slide operation does when it permutes the slides it was given.
    /// The deck errors stay. Every answer to a send begun before this is
    /// refused, as after `reset`: such an answer describes the text the
    /// operation replaced, and replaying the operation's whole-region edit
    /// onto it would stretch and collapse the boxes. The operation sends
    /// the new text right after, and that answer is the next one applied.
    public mutating func adopt(_ newBoxes: [SlideBox]) {
        boxes = Self.separated(newBoxes)
        editLog = []
        generation += 1
        earliestLiveGeneration = generation
    }
```

- [ ] **Step 4: Extend the divider policy**

Replace `balancedPosition` in `DividerPolicy.swift`:

```swift
    /// The position of the divider between the editor and the right pane
    /// that splits them evenly, measured from the split view's left edge,
    /// or nil once the user moved it. `leadingWidth` is the width of a
    /// pinned sidebar in front of the editor, 0 when there is none; its own
    /// divider takes `dividerThickness` too.
    public func balancedPosition(totalWidth: CGFloat, dividerThickness: CGFloat, leadingWidth: CGFloat = 0) -> CGFloat? {
        guard !userMovedDivider else { return nil }
        let leading = leadingWidth > 0 ? leadingWidth + dividerThickness : 0
        let remaining = totalWidth - leading - dividerThickness
        return leading + (remaining / 2).rounded(.down)
    }
```

The existing call in `MainSplitViewController.balance()` keeps compiling through the default.

- [ ] **Step 5: Write the payload, the labels and the panel state**

`SlideDragPayload.swift`:

```swift
import Foundation

/// What a drag of slides carries on the pasteboard: which deck they come
/// from, their numbers there, and each slide's text. Within one deck the
/// numbers drive a move; into another deck the texts are inserted.
public struct SlideDragPayload: Codable, Equatable, Sendable {
    public static let pasteboardType = "io.geocod.tap.slides"

    public let deckPath: String
    public let slideNumbers: [Int]
    public let markdowns: [String]

    public init(deckPath: String, slideNumbers: [Int], markdowns: [String]) {
        self.deckPath = deckPath
        self.slideNumbers = slideNumbers
        self.markdowns = markdowns
    }

    public init?(data: Data) {
        guard let decoded = try? JSONDecoder().decode(SlideDragPayload.self, from: data) else { return nil }
        self = decoded
    }

    public func data() throws -> Data {
        try JSONEncoder().encode(self)
    }

    /// Whether the slides come from `deck`, compared through symlinks.
    public func comesFrom(deck: URL) -> Bool {
        FilePaths.same(URL(fileURLWithPath: deckPath), deck)
    }
}
```

`SlideAccessibility.swift`:

```swift
import Foundation

/// VoiceOver labels for boxes, thumbnails and drop indicators.
public enum SlideAccessibility {
    /// "Slide 3, default layout, What We Knew, 2 steps", with the title and
    /// the steps left out when the slide has none, and "skipped" at the end
    /// of a skipped slide.
    public static func label(for slide: Slide) -> String {
        var parts = ["Slide \(slide.number)"]
        if !slide.layout.isEmpty { parts.append("\(slide.layout) layout") }
        if !slide.title.isEmpty { parts.append(slide.title) }
        let reveals = BoxHeader.revealCount(for: slide)
        if reveals == 1 { parts.append("1 step") } else if reveals > 1 { parts.append("\(reveals) steps") }
        if slide.skip { parts.append("skipped") }
        return parts.joined(separator: ", ")
    }

    public static func dropLabel(beforeNumber: Int?, count: Int) -> String {
        let slides = count == 1 ? "1 slide" : "\(count) slides"
        if let beforeNumber { return "Drop \(slides) above slide \(beforeNumber)" }
        return "Drop \(slides) at the end"
    }
}
```

`SlidePanelState.swift`:

```swift
import Foundation

/// Whether the slide panel is pinned as a sidebar or peeks on hover. The
/// panel is pinned the first time a deck opens, so people find it, and
/// each deck then remembers its own state.
public struct SlidePanelState: Sendable {
    private let defaults: UserDefaults

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    static func key(for deck: URL) -> String {
        "SlidePanelPinned:" + FilePaths.canonical(deck)
    }

    public func isPinned(deck: URL) -> Bool {
        defaults.object(forKey: Self.key(for: deck)) as? Bool ?? true
    }

    public func setPinned(_ pinned: Bool, deck: URL) {
        defaults.set(pinned, forKey: Self.key(for: deck))
    }

    /// The deck was renamed, moved or saved as another file: its state follows it.
    public func moveState(from old: URL, to new: URL) {
        guard let value = defaults.object(forKey: Self.key(for: old)) as? Bool else { return }
        defaults.removeObject(forKey: Self.key(for: old))
        defaults.set(value, forKey: Self.key(for: new))
    }
}
```

`UserDefaults` is not `Sendable`; mark the struct `@unchecked Sendable` if the compiler objects, with a comment that `UserDefaults` is thread-safe.

- [ ] **Step 6: Run the tests, mutate, commit**

Run: `make -C desktop core-test`
Expected: green.

Mutations, each reverted: in `SlidePanelState.isPinned`, change the default `?? true` to `?? false` (expected: `testPinnedOnFirstLaunchAndRememberedPerDeck` fails on its first assertion); in `adopt`, drop the `earliestLiveGeneration` line (expected: `testAdoptRefusesAnswersToEarlierSends` fails, the stale answer is applied and the order flips back).

```bash
git add desktop/TapDesktopCore
git commit -m "feat(desktop): adopted boxes, the slide drag payload, panel state and accessibility labels"
```

---

### Task 4: The thumbnail core: presentation summary, key, cache, queue and the flat-image check

**Files:**
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/PresentationSummary.swift`, `ThumbnailKey.swift`, `ThumbnailCache.swift`, `ThumbnailQueue.swift`, `FlatImageCheck.swift`
- Modify: `desktop/TapDesktopCore/Sources/TapDesktopCore/TapClient.swift`
- Test: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/PresentationSummaryTests.swift`, `ThumbnailCacheTests.swift`, `ThumbnailQueueTests.swift`, `FlatImageCheckTests.swift`, `TapClientTests.swift` (add)

**Interfaces:**
- Consumes: `GET /api/presentation` as `internal/server/routes.go` serves it (`presentationResponse`: `transformer.PublicPresentation` plus `revision`; `config.theme`, `config.customTheme`, `config.themeColors`, `config.aspectRatio`; each slide's `hash`, `skip`, `steps`); `TapClient.authorizedRequest`; D2's `StubURLProtocol` in `TapClientTests` (a `URLProtocol` stub with `responseStatus`, `responseBody`, `lastRequest`; D2's core has no HTTP fake server, and none is to be built).
- Produces:
  - `struct PresentationSummary { revision: String; themeSignature: String; slides: [SlideSummary] }`, `struct SlideSummary { hash: String; skip: Bool; steps: Int }`, `static func decode(_ data: Data) throws -> PresentationSummary`
  - `TapClient.presentation() async throws -> PresentationSummary`
  - `struct ThumbnailKey: Hashable { slideHash: String; themeSignature: String; width: Int }`, `var fileName: String` (SHA-256 hex + `.png`); `static let width = 320`
  - `struct ThumbnailCache { directory: URL; static var defaultDirectory; url(for:); save(_ png: Data, for:) throws; data(for:) -> Data?; contains(_:) }`
  - `struct ThumbnailQueue { mutating func replace(with numbers: [Int], visible: [Int], current: Int?); mutating func next() -> Int?; mutating func requeue(_:); mutating func remove(_:); var isEmpty; var pending: [Int] }`
  - `enum FlatImageCheck { static func isFlat(_ image: NSImage) -> Bool }`

- [ ] **Step 1: Write the failing tests**

`PresentationSummaryTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class PresentationSummaryTests: XCTestCase {
    func testDecodesWhatTheThumbnailsNeed() throws {
        let json = """
        {"config": {"title": "Ops", "theme": "terminal", "customTheme": true, "aspectRatio": "16:9",
                    "themeColors": {"accent": "#ffb84d", "background": "#0f1a16"}},
         "slides": [{"index": 0, "layout": "title", "html": "<h1>x</h1>", "slots": {}, "slotOrder": [], "fragmentCount": 0, "steps": 0, "hash": "aaaaaaaaaaaa"},
                    {"index": 1, "layout": "default", "html": "", "slots": {}, "slotOrder": [], "fragmentCount": 1, "steps": 2, "hash": "bbbbbbbbbbbb", "skip": true}],
         "liveCode": {"drivers": []}, "revision": "7f3c1a2b9d0e"}
        """
        let summary = try PresentationSummary.decode(Data(json.utf8))
        XCTAssertEqual(summary.revision, "7f3c1a2b9d0e")
        XCTAssertEqual(summary.themeSignature, "terminal|custom|16:9|accent=#ffb84d;background=#0f1a16")
        XCTAssertEqual(summary.slides, [SlideSummary(hash: "aaaaaaaaaaaa", skip: false, steps: 0), SlideSummary(hash: "bbbbbbbbbbbb", skip: true, steps: 2)])
    }

    func testDefaultsWhenTheConfigIsBare() throws {
        let summary = try PresentationSummary.decode(Data(#"{"config": {}, "slides": [], "revision": ""}"#.utf8))
        XCTAssertEqual(summary.themeSignature, "|||", "no theme, no custom theme, no aspect ratio, no colours")
        XCTAssertEqual(summary.slides, [])
    }
}
```

`ThumbnailCacheTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class ThumbnailCacheTests: XCTestCase {
    func testTheKeyNamesOneFilePerSlideThemeAndSize() {
        let key = ThumbnailKey(slideHash: "abc", themeSignature: "terminal|||", width: 320)
        XCTAssertEqual(key.fileName.count, 64 + 4)
        XCTAssertTrue(key.fileName.hasSuffix(".png"))
        XCTAssertNotEqual(key.fileName, ThumbnailKey(slideHash: "abc", themeSignature: "base|||", width: 320).fileName, "the theme is part of the key")
        XCTAssertNotEqual(key.fileName, ThumbnailKey(slideHash: "abc", themeSignature: "terminal|||", width: 640).fileName, "so is the size")
        XCTAssertEqual(key.fileName, ThumbnailKey(slideHash: "abc", themeSignature: "terminal|||", width: 320).fileName)
    }

    func testSavesAndReadsPNGsInItsDirectory() throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("thumbnails-\(UUID().uuidString)")
        let cache = ThumbnailCache(directory: directory)
        let key = ThumbnailKey(slideHash: "abc", themeSignature: "terminal|||", width: 320)
        XCTAssertFalse(cache.contains(key))
        XCTAssertNil(cache.data(for: key))
        try cache.save(Data([0x89, 0x50, 0x4E, 0x47]), for: key)
        XCTAssertTrue(cache.contains(key))
        XCTAssertEqual(cache.data(for: key), Data([0x89, 0x50, 0x4E, 0x47]))
        XCTAssertEqual(cache.url(for: key).lastPathComponent, key.fileName)
        XCTAssertTrue(ThumbnailCache.defaultDirectory.path.hasSuffix("Application Support/Tap/Thumbnails"))
        try FileManager.default.removeItem(at: directory)
    }
}
```

`ThumbnailQueueTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class ThumbnailQueueTests: XCTestCase {
    func testCurrentFirstThenVisibleThenNearestThenTheRest() {
        var queue = ThumbnailQueue()
        queue.replace(with: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10], visible: [4, 5, 6], current: 5)
        XCTAssertEqual(queue.pending, [5, 4, 6, 3, 7, 2, 8, 1, 9, 10])
    }

    func testWithoutAVisibleRangeTheCurrentSlideAnchorsTheOrder() {
        var queue = ThumbnailQueue()
        queue.replace(with: [1, 2, 3, 4, 5], visible: [], current: 4)
        XCTAssertEqual(queue.pending, [4, 3, 5, 2, 1])
        queue.replace(with: [3, 1, 2], visible: [], current: nil)
        XCTAssertEqual(queue.pending, [1, 2, 3], "nothing to anchor on: deck order")
    }

    func testNextAndRequeue() {
        var queue = ThumbnailQueue()
        queue.replace(with: [1, 2, 3], visible: [1], current: nil)
        XCTAssertEqual(queue.next(), 1)
        queue.requeue(1)
        XCTAssertEqual(queue.pending, [2, 3, 1])
        XCTAssertEqual(queue.next(), 2)
        XCTAssertEqual(queue.next(), 3)
        XCTAssertEqual(queue.next(), 1)
        XCTAssertNil(queue.next())
        XCTAssertTrue(queue.isEmpty)
        queue.replace(with: [4, 5], visible: [], current: nil)
        queue.remove(4)
        XCTAssertEqual(queue.pending, [5])
    }

    func testReplaceDropsNumbersNoLongerWanted() {
        var queue = ThumbnailQueue()
        queue.replace(with: [1, 2, 3], visible: [], current: nil)
        queue.replace(with: [2], visible: [], current: nil)
        XCTAssertEqual(queue.pending, [2])
    }
}
```

`FlatImageCheckTests.swift`:

```swift
import AppKit
import XCTest
@testable import TapDesktopCore

final class FlatImageCheckTests: XCTestCase {
    func image(width: Int, height: Int, draw: (NSRect) -> Void) -> NSImage {
        let image = NSImage(size: NSSize(width: width, height: height))
        image.lockFocus()
        draw(NSRect(x: 0, y: 0, width: width, height: height))
        image.unlockFocus()
        return image
    }

    func testAOneColourImageIsFlat() {
        let white = image(width: 320, height: 180) { NSColor.white.setFill(); $0.fill() }
        XCTAssertTrue(FlatImageCheck.isFlat(white))
        let black = image(width: 320, height: 180) { NSColor.black.setFill(); $0.fill() }
        XCTAssertTrue(FlatImageCheck.isFlat(black))
    }

    func testAnImageWithContentIsNotFlat() {
        let slide = image(width: 320, height: 180) { rect in
            NSColor(white: 0.06, alpha: 1).setFill(); rect.fill()
            NSColor.white.setFill(); NSRect(x: 40, y: 80, width: 200, height: 30).fill()
        }
        XCTAssertFalse(FlatImageCheck.isFlat(slide))
    }

    func testAThinStrokeBetweenSamplePointsIsStillContent() {
        // One white row on black, at a height a 24 by 14 sample grid never reads (row 7 of 180).
        let sparse = image(width: 320, height: 180) { rect in
            NSColor.black.setFill(); rect.fill()
            NSColor.white.setFill(); NSRect(x: 0, y: 7, width: 320, height: 1).fill()
        }
        XCTAssertFalse(FlatImageCheck.isFlat(sparse), "every pixel counts, through the downsample")
        // A single blue channel difference is content too.
        let blue = image(width: 320, height: 180) { rect in
            NSColor(red: 0.5, green: 0.5, blue: 0.5, alpha: 1).setFill(); rect.fill()
            NSColor(red: 0.5, green: 0.5, blue: 0.9, alpha: 1).setFill(); NSRect(x: 0, y: 0, width: 160, height: 180).fill()
        }
        XCTAssertFalse(FlatImageCheck.isFlat(blue))
    }

    func testAnEmptyImageIsFlat() {
        XCTAssertTrue(FlatImageCheck.isFlat(NSImage(size: .zero)))
    }
}
```

Add to `TapClientTests.swift`, on its `StubURLProtocol` and `stubbedClient()` like the `putSource` tests:

```swift
    func testPresentationFetchesTheSummaryWithTheToken() async throws {
        StubURLProtocol.responseStatus = 200
        StubURLProtocol.responseBody = Data(#"{"config": {"theme": "base"}, "slides": [{"hash": "h1", "steps": 1}], "revision": "r1"}"#.utf8)
        let summary = try await stubbedClient().presentation()
        XCTAssertEqual(summary.revision, "r1")
        XCTAssertEqual(summary.slides, [SlideSummary(hash: "h1", skip: false, steps: 1)])
        let request = try XCTUnwrap(StubURLProtocol.lastRequest)
        XCTAssertEqual(request.httpMethod, "GET")
        XCTAssertEqual(request.url?.absoluteString, "http://127.0.0.1:49152/api/presentation")
        XCTAssertEqual(request.value(forHTTPHeaderField: "Authorization"), "Bearer " + ready.token)
    }

    func testPresentationThrowsTheStatusOnAnError() async {
        StubURLProtocol.responseStatus = 404
        StubURLProtocol.responseBody = Data(#"{"error": "No presentation loaded"}"#.utf8)
        do {
            _ = try await stubbedClient().presentation()
            XCTFail("expected an error")
        } catch {
            XCTAssertEqual((error as? TapErrorPayload)?.code, "http_404")
        }
    }
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `make -C desktop core-test`
Expected: compile failures on the new types.

- [ ] **Step 3: Write the core files**

`PresentationSummary.swift`:

```swift
import Foundation

/// One slide of `GET /api/presentation`, reduced to what the thumbnails
/// need: tap's content hash, which changes whenever the slide's text or
/// its component bundle changes, and the fields the panel shows.
public struct SlideSummary: Equatable, Sendable {
    public let hash: String
    public let skip: Bool
    public let steps: Int

    public init(hash: String, skip: Bool, steps: Int) {
        self.hash = hash
        self.skip = skip
        self.steps = steps
    }
}

/// `GET /api/presentation`, reduced to the revision, a signature of the
/// deck-wide settings that change how every slide looks, and each slide's
/// summary. Everything else in the response is the page's business.
public struct PresentationSummary: Equatable, Sendable {
    public let revision: String
    public let themeSignature: String
    public let slides: [SlideSummary]

    public init(revision: String, themeSignature: String, slides: [SlideSummary]) {
        self.revision = revision
        self.themeSignature = themeSignature
        self.slides = slides
    }

    private struct Envelope: Decodable {
        struct Config: Decodable {
            let theme: String?
            let customTheme: Bool?
            let aspectRatio: String?
            let themeColors: [String: String]?
        }
        struct Slide: Decodable {
            let hash: String?
            let skip: Bool?
            let steps: Int?
        }
        let config: Config
        let slides: [Slide]
        let revision: String?
    }

    public static func decode(_ data: Data) throws -> PresentationSummary {
        let envelope = try JSONDecoder().decode(Envelope.self, from: data)
        let colours = (envelope.config.themeColors ?? [:]).sorted { $0.key < $1.key }.map { "\($0.key)=\($0.value)" }.joined(separator: ";")
        let signature = [envelope.config.theme ?? "", envelope.config.customTheme == true ? "custom" : "",
                         envelope.config.aspectRatio ?? "", colours].joined(separator: "|")
        let slides = envelope.slides.map { SlideSummary(hash: $0.hash ?? "", skip: $0.skip ?? false, steps: $0.steps ?? 0) }
        return PresentationSummary(revision: envelope.revision ?? "", themeSignature: signature, slides: slides)
    }
}
```

In `TapClient.swift`, after `putSource`:

```swift
    /// The rendered deck's summary, from the render tap shows now: the
    /// buffer while there is one, and the file otherwise.
    public func presentation() async throws -> PresentationSummary {
        let request = authorizedRequest(path: "/api/presentation")
        let (data, response) = try await session.data(for: request)
        let status = (response as? HTTPURLResponse)?.statusCode ?? 0
        guard status == 200 else {
            throw TapErrorPayload(code: "http_\(status)", message: String(decoding: data, as: UTF8.self))
        }
        return try PresentationSummary.decode(data)
    }
```

`ThumbnailKey.swift`:

```swift
import CryptoKit
import Foundation

/// What decides a thumbnail's pixels: tap's hash of the slide's content
/// (its text and its component bundle), the deck's theme signature, and
/// the snapshot width. Two slides with equal keys share one image.
public struct ThumbnailKey: Hashable, Sendable {
    public static let width = 320

    public let slideHash: String
    public let themeSignature: String
    public let width: Int

    public init(slideHash: String, themeSignature: String, width: Int = ThumbnailKey.width) {
        self.slideHash = slideHash
        self.themeSignature = themeSignature
        self.width = width
    }

    public var fileName: String {
        let digest = SHA256.hash(data: Data("\(slideHash)\n\(themeSignature)\n\(width)".utf8))
        return digest.map { String(format: "%02x", $0) }.joined() + ".png"
    }
}
```

`ThumbnailCache.swift`:

```swift
import Foundation

/// The thumbnails on disk: one PNG per key, so a reopened deck shows every
/// thumbnail without rendering.
public struct ThumbnailCache: Sendable {
    public let directory: URL

    public static var defaultDirectory: URL {
        FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("Tap/Thumbnails", isDirectory: true)
    }

    public init(directory: URL = ThumbnailCache.defaultDirectory) {
        self.directory = directory
    }

    public func url(for key: ThumbnailKey) -> URL {
        directory.appendingPathComponent(key.fileName)
    }

    public func contains(_ key: ThumbnailKey) -> Bool {
        FileManager.default.fileExists(atPath: url(for: key).path)
    }

    public func data(for key: ThumbnailKey) -> Data? {
        try? Data(contentsOf: url(for: key))
    }

    public func save(_ png: Data, for key: ThumbnailKey) throws {
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try png.write(to: url(for: key), options: .atomic)
    }
}
```

`ThumbnailQueue.swift`:

```swift
import Foundation

/// Which slide renders next: the current slide, then the visible ones in
/// order, then the rest by distance from the visible range (or from the
/// current slide when nothing is visible), then deck order.
public struct ThumbnailQueue: Equatable, Sendable {
    public private(set) var pending: [Int] = []

    public init() {}

    public var isEmpty: Bool { pending.isEmpty }

    public mutating func replace(with numbers: [Int], visible: [Int], current: Int?) {
        let wanted = Array(Set(numbers)).sorted()
        let visibleSet = Set(visible)
        var ordered: [Int] = []
        if let current, wanted.contains(current) { ordered.append(current) }
        ordered.append(contentsOf: visible.filter { wanted.contains($0) && !ordered.contains($0) })
        let anchorLow = visible.min() ?? current
        let anchorHigh = visible.max() ?? current
        let rest = wanted.filter { !ordered.contains($0) && !visibleSet.contains($0) }
        let sortedRest: [Int]
        if let anchorLow, let anchorHigh {
            func distance(_ number: Int) -> Int {
                if number < anchorLow { return anchorLow - number }
                if number > anchorHigh { return number - anchorHigh }
                return 0
            }
            sortedRest = rest.sorted { (distance($0), $0) < (distance($1), $1) }
        } else {
            sortedRest = rest
        }
        pending = ordered + sortedRest
    }

    public mutating func next() -> Int? {
        pending.isEmpty ? nil : pending.removeFirst()
    }

    public mutating func requeue(_ number: Int) {
        pending.removeAll { $0 == number }
        pending.append(number)
    }

    public mutating func remove(_ number: Int) {
        pending.removeAll { $0 == number }
    }
}
```

`FlatImageCheck.swift`:

```swift
import AppKit

/// Whether a snapshot is one flat colour, which is what an unpainted page
/// gives. The image is drawn down to a small bitmap and every pixel of
/// that bitmap is read, so a thin stroke anywhere still counts as content
/// (it shades the block it lands in), unlike a grid of sample points that
/// can fall between strokes.
public enum FlatImageCheck {
    static let sampleSize = NSSize(width: 64, height: 36)

    public static func isFlat(_ image: NSImage) -> Bool {
        guard image.size.width > 0, image.size.height > 0,
              let bitmap = NSBitmapImageRep(bitmapDataPlanes: nil, pixelsWide: Int(sampleSize.width), pixelsHigh: Int(sampleSize.height),
                                            bitsPerSample: 8, samplesPerPixel: 4, hasAlpha: true, isPlanar: false,
                                            colorSpaceName: .deviceRGB, bytesPerRow: 0, bitsPerPixel: 0) else { return true }
        NSGraphicsContext.saveGraphicsState()
        defer { NSGraphicsContext.restoreGraphicsState() }
        guard let context = NSGraphicsContext(bitmapImageRep: bitmap) else { return true }
        NSGraphicsContext.current = context
        context.imageInterpolation = .high
        image.draw(in: NSRect(origin: .zero, size: sampleSize), from: .zero, operation: .copy, fraction: 1)
        context.flushGraphics()
        var minimum = [255, 255, 255]
        var maximum = [0, 0, 0]
        for y in 0..<bitmap.pixelsHigh {
            for x in 0..<bitmap.pixelsWide {
                guard let colour = bitmap.colorAt(x: x, y: y) else { continue }
                let channels = [Int(colour.redComponent * 255), Int(colour.greenComponent * 255), Int(colour.blueComponent * 255)]
                for index in 0..<3 {
                    minimum[index] = min(minimum[index], channels[index])
                    maximum[index] = max(maximum[index], channels[index])
                }
            }
        }
        return (0..<3).allSatisfy { maximum[$0] - minimum[$0] <= 6 }
    }
}
```

D2's `DeckSessionController` carries a private `NSBitmapImageRep.isSingleColor` for the welcome window's thumbnail, a 16 by 16 sample grid. Task 7 replaces that call with `FlatImageCheck.isFlat` and deletes the extension, so one check decides what a blank capture is.

- [ ] **Step 4: Run the tests, mutate, commit**

Run: `make -C desktop core-test`
Expected: green.

Mutations, one at a time, each reverted: in `ThumbnailQueue.replace`, drop the `current` line (expected: `testCurrentFirstThenVisibleThenNearestThenTheRest` fails, 5 is no longer first); in `FlatImageCheck.isFlat`, return `true` unconditionally (expected: `testAnImageWithContentIsNotFlat` fails); read only every fourth row of the bitmap (expected: `testAThinStrokeBetweenSamplePointsIsStillContent` fails on the one-row image, since the downsample puts that row in one block row).

```bash
git add desktop/TapDesktopCore
git commit -m "feat(desktop): the thumbnail key, disk cache, priority queue and presentation summary"
```

---

### Task 5: The layout catalog, the schematics and the last layout

**Files:**
- Create: `desktop/TapDesktopCore/Sources/TapDesktopCore/LayoutCatalog.swift`
- Test: `desktop/TapDesktopCore/Tests/TapDesktopCoreTests/LayoutCatalogTests.swift`

**Interfaces:**
- Consumes: `tap slide add --print --json` with no `--layout` (`printAllLayouts` in `internal/cli/slide.go`: `{"ok": true, "layouts": [{"name": "title", "template": "# Title\n"}, ...]}` in the wizard's order: title, section, default, two-column, code-focus, quote, big-stat, three-column, sidebar, split-media, cover, blank), `DirectiveComment.leading` (Task 2), `TapErrorPayload` (D2).
- Produces:
  - `struct LayoutTemplate: Equatable { name: String; markdown: String }`, `enum LayoutCatalog { static func decode(_:) throws -> [LayoutTemplate]; static func displayName(_:) -> String }`
  - `enum LayoutSchematic { enum Element: Equatable { heading, line, columns(Int), sidebar, code, quote, media, bigNumber }; static func elements(for markdown: String) -> [Element] }`
  - `struct LastLayout { init(defaults:); var name: String }` (default `"default"`)

- [ ] **Step 1: Write the failing tests**

`LayoutCatalogTests.swift`:

```swift
import XCTest
@testable import TapDesktopCore

final class LayoutCatalogTests: XCTestCase {
    func testDecodesTheLayoutListInTapsOrder() throws {
        let json = #"{"ok": true, "layouts": [{"name": "title", "template": "# Title\n"}, {"name": "big-stat", "template": "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n"}]}"#
        let templates = try LayoutCatalog.decode(Data(json.utf8))
        XCTAssertEqual(templates, [LayoutTemplate(name: "title", markdown: "# Title\n"),
                                   LayoutTemplate(name: "big-stat", markdown: "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n")])
        XCTAssertThrowsError(try LayoutCatalog.decode(Data(#"{"ok": false, "error": {"code": "usage", "message": "no"}}"#.utf8)))
    }

    func testDisplayNames() {
        XCTAssertEqual(LayoutCatalog.displayName("big-stat"), "Big Stat")
        XCTAssertEqual(LayoutCatalog.displayName("two-column"), "Two Column")
        XCTAssertEqual(LayoutCatalog.displayName("title"), "Title")
    }

    func testSchematicsFollowTheTemplate() {
        XCTAssertEqual(LayoutSchematic.elements(for: "## Header\n\n- Point one\n- Point two\n"), [.heading, .line, .line])
        XCTAssertEqual(LayoutSchematic.elements(for: "::left\n\nLeft content\n\n::right\n\nRight content\n"), [.columns(2)])
        XCTAssertEqual(LayoutSchematic.elements(for: "::left\n\nA\n\n::center\n\nB\n\n::right\n\nC\n"), [.columns(3)])
        XCTAssertEqual(LayoutSchematic.elements(for: "<!--\nlayout: code-focus\n-->\n\n```\n// Your code here\n```\n"), [.code])
        XCTAssertEqual(LayoutSchematic.elements(for: "<!--\nlayout: quote\n-->\n\n> \"Your quote here\"\n"), [.quote])
        XCTAssertEqual(LayoutSchematic.elements(for: "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n"), [.bigNumber, .line])
        XCTAssertEqual(LayoutSchematic.elements(for: "<!--\nlayout: sidebar\n-->\n\n## Header\n\nMain content\n\n::sidebar\n\n- Note one\n"), [.heading, .line, .sidebar])
        XCTAssertEqual(LayoutSchematic.elements(for: "<!--\nlayout: split-media\n-->\n\n## Header\n\nDescribe the image\n\n::media\n\nImage or video\n"), [.heading, .line, .media])
        XCTAssertEqual(LayoutSchematic.elements(for: "<!--\nlayout: blank\n-->\n\nContent\n"), [.line])
        XCTAssertEqual(LayoutSchematic.elements(for: ""), [])
    }

    func testLastLayoutDefaultsToDefault() throws {
        let defaults = try XCTUnwrap(UserDefaults(suiteName: "LayoutCatalogTests.\(UUID().uuidString)"))
        var last = LastLayout(defaults: defaults)
        XCTAssertEqual(last.name, "default")
        last.name = "two-column"
        XCTAssertEqual(LastLayout(defaults: defaults).name, "two-column")
    }
}
```

- [ ] **Step 2: Run the tests to verify they fail**

Run: `make -C desktop core-test`
Expected: compile failure, `cannot find 'LayoutCatalog' in scope`.

- [ ] **Step 3: Write `LayoutCatalog.swift`**

```swift
import Foundation

/// One of tap's layouts and the template `tap slide add --layout <name> --print` writes for it.
public struct LayoutTemplate: Equatable, Sendable {
    public let name: String
    public let markdown: String

    public init(name: String, markdown: String) {
        self.name = name
        self.markdown = markdown
    }
}

public enum LayoutCatalog {
    /// Decodes `tap slide add --print --json`: every layout in the wizard's order.
    public static func decode(_ data: Data) throws -> [LayoutTemplate] {
        struct Envelope: Decodable {
            struct Entry: Decodable { let name: String; let template: String }
            let ok: Bool
            let layouts: [Entry]?
            let error: TapErrorPayload?
        }
        let envelope = try JSONDecoder().decode(Envelope.self, from: data)
        guard envelope.ok, let layouts = envelope.layouts else {
            throw envelope.error ?? TapErrorPayload(code: "invalid_response", message: "tap printed no layouts")
        }
        return layouts.map { LayoutTemplate(name: $0.name, markdown: $0.template) }
    }

    /// "big-stat" reads "Big Stat" in a menu.
    public static func displayName(_ name: String) -> String {
        name.split(separator: "-").map { $0.prefix(1).uppercased() + $0.dropFirst() }.joined(separator: " ")
    }
}

/// The little picture a gallery cell draws for a layout, read off the
/// template's own markdown: headings, lines, slot columns, a code block,
/// a quote, media, a big number. A sketch of the template, not a render.
public enum LayoutSchematic {
    public enum Element: Equatable, Sendable {
        case heading, line, columns(Int), sidebar, code, quote, media, bigNumber
    }

    public static func elements(for markdown: String) -> [Element] {
        var elements: [Element] = []
        var columnMarkers = 0
        var insideFence = false
        let body = DirectiveComment.leading(in: markdown).map { (markdown as NSString).substring(from: NSMaxRange($0.range)) } ?? markdown
        for rawLine in body.components(separatedBy: "\n") {
            let line = rawLine.trimmingCharacters(in: .whitespaces)
            if line.hasPrefix("```") {
                if !insideFence { elements.append(.code) }
                insideFence.toggle()
                continue
            }
            if insideFence || line.isEmpty { continue }
            if line.hasPrefix("::") {
                switch line {
                case "::left", "::center", "::right": columnMarkers += 1
                case "::sidebar": elements.append(.sidebar)
                case "::media": elements.append(.media)
                default: break
                }
                continue
            }
            if columnMarkers > 0 { continue }
            if line.hasPrefix("# "), let first = line.dropFirst(2).first, first.isNumber {
                elements.append(.bigNumber)
            } else if line.hasPrefix("#") {
                elements.append(.heading)
            } else if line.hasPrefix(">") {
                if elements.last != .quote { elements.append(.quote) }
            } else if line.hasPrefix("![") {
                elements.append(.media)
            } else {
                elements.append(.line)
            }
        }
        if columnMarkers > 0 { elements.append(.columns(columnMarkers)) }
        return Array(elements.prefix(5))
    }
}

/// The layout New Slide inserts: the one used last, "default" at first.
public struct LastLayout {
    private let defaults: UserDefaults
    static let key = "LastSlideLayout"

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    public var name: String {
        get { defaults.string(forKey: Self.key) ?? "default" }
        nonmutating set { defaults.set(newValue, forKey: Self.key) }
    }
}
```

Run: `make -C desktop core-test`. Expected: green.

- [ ] **Step 4: Mutate and commit**

Mutations, each reverted: in `LayoutSchematic.elements`, treat every `#` line as `.heading` (expected: `testSchematicsFollowTheTemplate` fails on `.bigNumber`); in `LastLayout.name`, default to `"title"` (expected: `testLastLayoutDefaultsToDefault` fails).

```bash
git add desktop/TapDesktopCore
git commit -m "feat(desktop): decode tap's layout catalog, sketch each template, remember the last layout"
```

---


### Task 6: The thumbnail renderer

**Files:**
- Create: `desktop/Tap/Thumbnails/ThumbnailRenderer.swift`
- Create: `desktop/Tap/Preview/WeakScriptMessageHandler.swift` (moved out of `PreviewViewController.swift`, made internal)
- Modify: `desktop/Tap/Preview/PreviewViewController.swift` (remove the private copy), `desktop/Tap/Editor/EditorViewController.swift` (`hostHiddenView`), `desktop/Tap/Documents/DeckSessionController.swift` (the welcome thumbnail's `isSingleColor` becomes `FlatImageCheck.isFlat`)
- Test: `desktop/TapTests/ThumbnailRendererTests.swift`

**Interfaces:**
- Consumes: `ReadyPayload` (D2), `TapClient.baseURL`, `TapClient.presentation()` (Task 4), `ThumbnailKey`, `ThumbnailQueue`, `FlatImageCheck` (Task 4), `HostedTestCase` and `Fixtures` (D2). The print page: `GET /?print=true` is an audience route (`audienceRoutes` in `internal/server/app_auth.go`) and so are `/api/presentation`, `/assets/` and `/components/`, so the print page needs no session cookie and no launch code. It never opens the WebSocket (`App.tsx`, `PRINT_MODE`), it shows the slide the URL fragment names (`parseURLHashSlideIndex`, 1-based, and `setupHashChangeListener`), it renders the final step and fragment, and it publishes `tapReady` only after a paint (`requirePaint: PRINT_MODE`).
- Produces:
  - `final class ThumbnailRenderer: NSObject` (`@MainActor`) with `struct Job: Equatable { slideNumber: Int; key: ThumbnailKey }`, `static let viewSize = NSSize(width: 960, height: 540)`, `static let readyTimeout: TimeInterval = 5`, `static let blankAcceptAttempts = 3`, `let webView: WKWebView`, `var isPaused: () -> Bool`, `var canPaint: () -> Bool`, `var onImage: ((Job, NSImage, Data) -> Void)?`, `var onPageRevision: ((String) -> Void)?`, `var snapshot: (WKWebView, WKSnapshotConfiguration) async throws -> NSImage` (a seam, defaulting to `takeSnapshot`), `private(set) var readyBySlide: [Int: ReadyPayload]`, `private(set) var renderCount: Int`, `private(set) var navigationCount: Int` (loads the renderer started), `private(set) var lastRenderedSlide: Int?`, `var pendingCount: Int`, `func isQueued(_ number: Int) -> Bool`, `func configure(client: TapClient?)` (nil stops the loop: tap is down), `func setWork(_ jobs: [Job], revision: String, visible: [Int], current: Int?)`
  - A capture that comes back one flat colour is retried with a growing back-off (0.5 s, 1 s, 2 s, then 5 s per job); after `blankAcceptAttempts` flat captures in a row, each following a paint-proven ready for that slide and revision, the slide is taken to be blank and its image is delivered and cached like any other. A ready that never arrives, or a failed load, is retried with the same back-off and never delivers an image.
  - `EditorViewController.hostHiddenView(_ view: NSView)`: hosts a view behind the editor's opaque scroll view.

- [ ] **Step 1: Check the app target and write the failing tests**

Run: `make frontend` from the repository root if `embedded/dist/index.html` is missing or older than `frontend/`, then `make -C desktop build`. Expected: `** BUILD SUCCEEDED **` with `tap` bundled. This is the first task that needs D2's app target; if the branch has not been rebased onto `main` after pull request 29 merged, do that first.

`desktop/TapTests/ThumbnailRendererTests.swift`:

```swift
import AppKit
import WebKit
import XCTest
@testable import Tap

final class ThumbnailRendererTests: HostedTestCase {
    /// A renderer hosted behind the deck's editor, pointed at its running tap.
    func makeRenderer(for document: DeckDocument) async throws -> (ThumbnailRenderer, TapClient, PresentationSummary) {
        let ready = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let renderer = ThumbnailRenderer()
        controller.editorViewController.hostHiddenView(renderer.webView)
        renderer.canPaint = { [weak renderer] in
            guard let webView = renderer?.webView, let window = webView.window else { return false }
            return !webView.isHiddenOrHasHiddenAncestor && window.occlusionState.contains(.visible)
        }
        let client = TapClient(ready: ready)
        renderer.configure(client: client)
        // The first PUT answer is what makes /api/presentation serve the buffer.
        try await waitForBoxes(document, count: controller.editor.boxes.count == 0 ? 1 : controller.editor.boxes.count)
        let summary = try await client.presentation()
        return (renderer, client, summary)
    }

    func jobs(for summary: PresentationSummary) -> [ThumbnailRenderer.Job] {
        summary.slides.enumerated().map { index, slide in
            ThumbnailRenderer.Job(slideNumber: index + 1, key: ThumbnailKey(slideHash: slide.hash, themeSignature: summary.themeSignature))
        }
    }

    func testRendersPaintedThumbnailsThroughTheReadySignal() async throws {
        let document = try await openDeck(try Fixtures.copyAppFixture())
        try await waitForBoxes(document, count: 4)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        var images: [Int: (NSImage, Data)] = [:]
        renderer.onImage = { job, image, png in images[job.slideNumber] = (image, png) }
        renderer.setWork(jobs(for: summary), revision: summary.revision, visible: [1, 2], current: 2)
        try await waitUntil(timeout: 30, "four thumbnails") { images.count == 4 }

        XCTAssertEqual(renderer.renderCount, 4)
        for number in 1...4 {
            let (image, png) = try XCTUnwrap(images[number])
            XCTAssertFalse(FlatImageCheck.isFlat(image), "slide \(number) is painted, not blank")
            XCTAssertNotNil(NSBitmapImageRep(data: png), "slide \(number) encodes as PNG")
            XCTAssertEqual(Int(image.size.width.rounded()), ThumbnailKey.width)
        }
        XCTAssertNotEqual(images[1]?.1, images[2]?.1, "two different slides give two different images")
        XCTAssertEqual(renderer.readyBySlide[4]?.step, 2, "the component slide reported ready at its final step, so its bundle had loaded")
        XCTAssertEqual(renderer.readyBySlide[3]?.revision, summary.revision)
        let url = try XCTUnwrap(renderer.webView.url)
        XCTAssertTrue(url.query?.contains("print=true") ?? false, "the renderer is a print page, which never joins the hub")
        XCTAssertEqual(renderer.readyBySlide.keys.sorted(), [1, 2, 3, 4])
    }

    func testTheCurrentSlideRendersFirst() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        var order: [Int] = []
        renderer.onImage = { job, _, _ in order.append(job.slideNumber) }
        renderer.setWork(jobs(for: summary), revision: summary.revision, visible: [1, 2, 3], current: 6)
        try await waitUntil(timeout: 30, "seven thumbnails") { order.count == 7 }
        XCTAssertEqual(order.first, 6, "the current slide goes to the front of the queue")
        XCTAssertEqual(Array(order[1...3]), [1, 2, 3], "then the visible ones")
    }

    func testACoveredWindowRendersNothingAndResumes() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        let deckWindow = try XCTUnwrap(document.windowControllers.first?.window)
        // An opaque window over the whole deck window: the window server reports the deck window as not visible.
        let cover = NSWindow(contentRect: deckWindow.frame.insetBy(dx: -50, dy: -50), styleMask: [.titled], backing: .buffered, defer: false)
        cover.isOpaque = true
        cover.backgroundColor = .black
        cover.level = .floating
        cover.orderFrontRegardless()
        // A failed run must not leave the cover over every later test's window.
        defer { cover.orderOut(nil) }
        try await waitUntil(timeout: 5, "the deck window to be covered") { !deckWindow.occlusionState.contains(.visible) }

        var images: [Int] = []
        renderer.onImage = { job, _, _ in images.append(job.slideNumber) }
        renderer.setWork(jobs(for: summary), revision: summary.revision, visible: [1], current: 1)
        try await Task.sleep(nanoseconds: 1_500_000_000)
        XCTAssertEqual(renderer.navigationCount, 0, "the renderer does not even load the page while the window cannot paint")
        XCTAssertEqual(renderer.renderCount, 0)
        XCTAssertTrue(images.isEmpty)

        cover.orderOut(nil)
        try await waitUntil(timeout: 10, "the deck window to be visible again") { deckWindow.occlusionState.contains(.visible) }
        try await waitUntil(timeout: 30, "thumbnails after the cover is gone") { images.count == 7 }
        XCTAssertGreaterThan(renderer.navigationCount, 0)
    }

    func testAFlatCaptureIsRetriedAndAcceptedOnlyAfterThreeTries() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        try await waitForBoxes(document, count: 1)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        let flat = NSImage(size: NSSize(width: 320, height: 180))
        flat.lockFocus()
        NSColor.white.setFill()
        NSRect(x: 0, y: 0, width: 320, height: 180).fill()
        flat.unlockFocus()
        var snapshots = 0
        renderer.snapshot = { _, _ in
            snapshots += 1
            return flat
        }
        var images: [Int] = []
        renderer.onImage = { job, _, _ in images.append(job.slideNumber) }
        renderer.setWork(jobs(for: summary), revision: summary.revision, visible: [1], current: 1)
        try await waitUntil(timeout: 20, "the flat snapshot to be taken twice") { snapshots >= 2 }
        XCTAssertTrue(images.isEmpty, "a flat capture is not taken at its word: it is retried")
        XCTAssertEqual(renderer.renderCount, 0)
        XCTAssertTrue(renderer.isQueued(1), "the slide is requeued rather than dropped")
        try await waitUntil(timeout: 20, "the slide to be accepted as blank") { images == [1] }
        XCTAssertEqual(snapshots, ThumbnailRenderer.blankAcceptAttempts, "three flat captures in a row, each after a paint-proven ready, mean a blank slide")
        XCTAssertEqual(renderer.renderCount, 1)
        XCTAssertFalse(renderer.isQueued(1))
    }

    func testTheRendererStopsWhileTapIsDown() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        try await waitForBoxes(document, count: 1)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        renderer.configure(client: nil)
        renderer.setWork(jobs(for: summary), revision: summary.revision, visible: [1], current: 1)
        try await Task.sleep(nanoseconds: 700_000_000)
        XCTAssertEqual(renderer.navigationCount, 0, "no client, no loads: a failing load would otherwise be retried without end")
        XCTAssertTrue(renderer.isQueued(1), "the work waits for the next client")
    }
}        XCTAssertEqual(renderer.pendingCount, 1, "the slide is requeued rather than dropped")
    }
}
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/ThumbnailRendererTests/testRendersPaintedThumbnailsThroughTheReadySignal`
Expected: compile failure, `cannot find 'ThumbnailRenderer' in scope`.

- [ ] **Step 3: Move `WeakScriptMessageHandler` to its own file**

`desktop/Tap/Preview/WeakScriptMessageHandler.swift`:

```swift
import WebKit

/// Forwards script messages without the user content controller keeping
/// its target alive. Shared by the preview and the thumbnail renderer.
final class WeakScriptMessageHandler: NSObject, WKScriptMessageHandler {
    weak var target: WKScriptMessageHandler?

    init(_ target: WKScriptMessageHandler) {
        self.target = target
    }

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        target?.userContentController(userContentController, didReceive: message)
    }
}
```

Delete the `private final class WeakScriptMessageHandler` from `PreviewViewController.swift`.

In `DeckSessionController.captureThumbnail`, replace `guard !bitmap.isSingleColor else {` with `guard !FlatImageCheck.isFlat(image) else {` and delete the `private extension NSBitmapImageRep` at the end of the file, so the welcome window's thumbnail and the slide thumbnails share one definition of a blank capture. `WelcomeTests` still pass: run `make -C desktop test ONLY=TapTests/WelcomeTests` once after the change.

- [ ] **Step 4: Host a hidden view behind the editor**

Add to `EditorViewController`:

```swift
    /// Hosts a view behind the editor, inside the visible window, where
    /// WebKit treats it as visible and paints it. The scroll view draws an
    /// opaque background over it, so nobody sees it. An off-screen window
    /// would not do: WebKit suspends a page there and it never paints.
    func hostHiddenView(_ hidden: NSView) {
        hidden.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(hidden, positioned: .below, relativeTo: scrollView)
        NSLayoutConstraint.activate([
            hidden.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            hidden.bottomAnchor.constraint(equalTo: view.bottomAnchor),
            hidden.widthAnchor.constraint(equalToConstant: ThumbnailRenderer.viewSize.width),
            hidden.heightAnchor.constraint(equalToConstant: ThumbnailRenderer.viewSize.height),
        ])
    }
```

- [ ] **Step 5: Write `ThumbnailRenderer.swift`**

```swift
import AppKit
import WebKit

/// Renders slide thumbnails in one print-mode web view that nobody sees.
///
/// The web view sits inside the deck window behind the editor's opaque
/// scroll view (`EditorViewController.hostHiddenView`): WebKit suspends a
/// page in an off-screen window and never paints it, and print mode's
/// ready signal waits for a paint. The page is `/?print=true`, which never
/// joins the WebSocket hub, so walking through slides here never moves the
/// preview; it needs no cookie, because the page and the presentation are
/// audience routes. A slide is chosen with the URL fragment, and it counts
/// as rendered when the page posts `tapReady` for it with the revision the
/// work was set for. The app injects no script into the page.
@MainActor
final class ThumbnailRenderer: NSObject, WKScriptMessageHandler, WKNavigationDelegate {
    struct Job: Equatable {
        let slideNumber: Int
        let key: ThumbnailKey
    }

    static let viewSize = NSSize(width: 960, height: 540)
    /// How long a slide may take to report ready before it is requeued.
    static let readyTimeout: TimeInterval = 5

    let webView: WKWebView
    /// True while the person is typing: the loop waits.
    var isPaused: () -> Bool = { false }
    /// True while the web view can paint: its window is on screen and it is not hidden.
    var canPaint: () -> Bool = { true }
    var onImage: ((Job, NSImage, Data) -> Void)?
    /// The page reported a revision other than the one the work was set for.
    var onPageRevision: ((String) -> Void)?
    /// Takes the snapshot. A seam: a test replaces it to hand the renderer a
    /// flat image, which no real page in a visible window produces.
    var snapshot: (WKWebView, WKSnapshotConfiguration) async throws -> NSImage = { webView, configuration in
        try await webView.takeSnapshot(configuration: configuration)
    }
    /// The ready payload each slide reported when it was captured.
    private(set) var readyBySlide: [Int: ReadyPayload] = [:]
    /// Images delivered.
    private(set) var renderCount = 0
    /// Loads this renderer asked the web view for.
    private(set) var navigationCount = 0
    private(set) var lastRenderedSlide: Int?
    /// After this many flat captures in a row, each following a paint-proven ready, a slide counts as blank.
    static let blankAcceptAttempts = 3

    private var baseURL: URL?
    private var allowedPort: Int?
    private var wantedRevision: String?
    private var loadedRevision: String?
    private var loadCount = 0
    private var jobs: [Int: Job] = [:]
    private var queue = ThumbnailQueue()
    private var running = false
    private var lastReady: ReadyPayload?
    private var waitingForSlide: Int?
    private var readyWaiter: CheckedContinuation<ReadyPayload?, Never>?
    private var timeoutWork: DispatchWorkItem?
    /// Per slide: failed attempts in a row, when it may be tried again, and flat captures in a row.
    private var failures: [Int: Int] = [:]
    private var notBefore: [Int: Date] = [:]
    private var flatCaptures: [Int: Int] = [:]

    private enum Outcome { case rendered, retry }

    override init() {
        let configuration = WKWebViewConfiguration()
        configuration.userContentController = WKUserContentController()
        configuration.websiteDataStore = .nonPersistent()
        webView = WKWebView(frame: NSRect(origin: .zero, size: Self.viewSize), configuration: configuration)
        super.init()
        configuration.userContentController.add(WeakScriptMessageHandler(self), name: "tapReady")
        // Print mode lays a slide out at 1920 by 1080 CSS pixels; half zoom fits it in the view.
        webView.pageZoom = 0.5
        webView.navigationDelegate = self
        webView.setAccessibilityElement(false)
        webView.setAccessibilityIdentifier("thumbnail-renderer")
    }

    var pendingCount: Int { queue.pending.count + (running ? 1 : 0) }

    func isQueued(_ number: Int) -> Bool { queue.pending.contains(number) }

    /// Points the renderer at a running tap, or at nothing while tap is
    /// down: with no base URL the loop stops, and the work waits.
    func configure(client: TapClient?) {
        baseURL = client?.baseURL
        allowedPort = client?.ready.port
        loadedRevision = nil
        lastReady = nil
        failures = [:]
        notBefore = [:]
        pump()
    }

    /// Replaces the work: the slides without a cached image, and the
    /// revision their keys were computed from. A page loaded for another
    /// revision is reloaded before the next capture.
    func setWork(_ newJobs: [Job], revision: String, visible: [Int], current: Int?) {
        wantedRevision = revision
        jobs = Dictionary(newJobs.map { ($0.slideNumber, $0) }, uniquingKeysWith: { first, _ in first })
        queue.replace(with: Array(jobs.keys), visible: visible, current: current)
        pump()
    }

    private func pump() {
        guard !running, baseURL != nil else { return }
        running = true
        Task { @MainActor [weak self] in
            while true {
                guard let self, self.baseURL != nil else {
                    self?.running = false
                    return
                }
                if self.isPaused() || !self.canPaint() {
                    try? await Task.sleep(nanoseconds: 150_000_000)
                    continue
                }
                guard let number = self.nextDueSlide() else {
                    if self.queue.isEmpty {
                        self.running = false
                        return
                    }
                    // Every queued job is backing off.
                    try? await Task.sleep(nanoseconds: 100_000_000)
                    continue
                }
                guard let job = self.jobs[number] else { continue }
                switch await self.render(job) {
                case .rendered:
                    self.failures[number] = nil
                    self.notBefore[number] = nil
                case .retry:
                    guard self.jobs[number] == job else { continue }
                    let count = (self.failures[number] ?? 0) + 1
                    self.failures[number] = count
                    // 0.5 s, 1 s, 2 s, then 5 s: a job that keeps failing never spins the page.
                    self.notBefore[number] = Date().addingTimeInterval(min(5, 0.5 * pow(2, Double(count - 1))))
                    self.queue.requeue(number)
                }
            }
        }
    }

    /// The first queued slide whose back-off has passed, removed from the queue.
    private func nextDueSlide() -> Int? {
        let now = Date()
        guard let number = queue.pending.first(where: { (notBefore[$0] ?? .distantPast) <= now }) else { return nil }
        queue.remove(number)
        return number
    }

    private func printURL(slide: Int) -> URL? {
        guard let baseURL else { return nil }
        // A URL that differs only in its fragment is a same-document
        // navigation, so the query carries a count that makes each fresh
        // load a real one.
        return URL(string: "\(baseURL.absoluteString)?print=true&load=\(loadCount)#\(slide)")
    }

    /// Renders one slide. `.retry` when the slide did not report ready in
    /// time, reported another revision, or captured one flat colour fewer
    /// than `blankAcceptAttempts` times in a row; the loop requeues it with
    /// a back-off.
    private func render(_ job: Job) async -> Outcome {
        let number = job.slideNumber
        if loadedRevision == nil || loadedRevision != wantedRevision {
            loadCount += 1
            lastReady = nil
            guard let url = printURL(slide: number) else { return .retry }
            navigationCount += 1
            webView.load(URLRequest(url: url))
        } else if lastReady?.slide != number {
            lastReady = nil
            guard let url = printURL(slide: number) else { return .retry }
            navigationCount += 1
            webView.load(URLRequest(url: url))
        }
        guard let ready = await waitForReady(slide: number) else {
            loadedRevision = nil
            return .retry
        }
        loadedRevision = ready.revision
        guard ready.revision == wantedRevision else {
            onPageRevision?(ready.revision)
            return .retry
        }
        let configuration = WKSnapshotConfiguration()
        configuration.snapshotWidth = NSNumber(value: job.key.width)
        guard let image = try? await snapshot(webView, configuration) else { return .retry }
        if FlatImageCheck.isFlat(image) {
            // The page reported ready after a paint, for this slide and revision, and the
            // pixels are still one colour: either the paint came late, or the slide is blank.
            // Only several such captures in a row settle it as blank.
            let flat = (flatCaptures[number] ?? 0) + 1
            flatCaptures[number] = flat
            guard flat >= Self.blankAcceptAttempts else { return .retry }
        }
        flatCaptures[number] = nil
        guard let tiff = image.tiffRepresentation,
              let png = NSBitmapImageRep(data: tiff)?.representation(using: .png, properties: [:]),
              jobs[number] == job else { return .retry }
        renderCount += 1
        lastRenderedSlide = number
        readyBySlide[number] = ready
        onImage?(job, image, png)
        return .rendered
    }

    private func waitForReady(slide: Int) async -> ReadyPayload? {
        if let lastReady, lastReady.slide == slide { return lastReady }
        waitingForSlide = slide
        return await withCheckedContinuation { continuation in
            readyWaiter = continuation
            let work = DispatchWorkItem { [weak self] in
                MainActor.assumeIsolated { self?.resumeWaiter(with: nil) }
            }
            timeoutWork = work
            DispatchQueue.main.asyncAfter(deadline: .now() + Self.readyTimeout, execute: work)
        }
    }

    private func resumeWaiter(with payload: ReadyPayload?) {
        timeoutWork?.cancel()
        timeoutWork = nil
        waitingForSlide = nil
        readyWaiter?.resume(returning: payload)
        readyWaiter = nil
    }

    // MARK: WebKit

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        guard message.name == "tapReady", let body = message.body as? [String: Any],
              let slide = (body["slide"] as? NSNumber)?.intValue else { return }
        let payload = ReadyPayload(revision: body["revision"] as? String ?? "", slide: slide,
                                   step: (body["step"] as? NSNumber)?.intValue ?? 0)
        lastReady = payload
        if waitingForSlide == slide { resumeWaiter(with: payload) }
    }

    func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction) async -> WKNavigationActionPolicy {
        guard let url = navigationAction.request.url else { return .cancel }
        if url.host == "127.0.0.1", url.port == allowedPort { return .allow }
        if url.scheme == "about" || url.scheme == "blob" || url.scheme == "data" { return .allow }
        return .cancel
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        resumeWaiter(with: nil)
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        resumeWaiter(with: nil)
    }
}
```

- [ ] **Step 6: Run the tests one at a time**

Run each, scoped:

```bash
make -C desktop test ONLY=TapTests/ThumbnailRendererTests/testRendersPaintedThumbnailsThroughTheReadySignal
make -C desktop test ONLY=TapTests/ThumbnailRendererTests/testTheCurrentSlideRendersFirst
make -C desktop test ONLY=TapTests/ThumbnailRendererTests/testACoveredWindowRendersNothingAndResumes
make -C desktop test ONLY=TapTests/ThumbnailRendererTests/testAFlatCaptureIsRetriedAndAcceptedOnlyAfterThreeTries
make -C desktop test ONLY=TapTests/ThumbnailRendererTests/testTheRendererStopsWhileTapIsDown
```

Expected: each passes. `testTheRendererStopsWhileTapIsDown` and the flat-capture test share nothing with the page, so they pass on any machine. If the first one times out, check the embedded assets first (`ls -la embedded/dist/index.html` newer than `frontend/src`), then read the renderer's `webView.url` and `lastReady` in the failure message before touching timeouts. If fragment navigation turns out not to fire `hashchange` in this WebKit (the ready signal for the second slide never arrives), switch `render` to always load a fresh URL with a new `load=` count per slide and record that in the ledger; the tests do not change.

- [ ] **Step 7: Mutate and commit**

Mutations, each reverted: skip the `FlatImageCheck.isFlat` branch in `render` (expected: `testAFlatCaptureIsRetriedAndAcceptedOnlyAfterThreeTries` fails, an image reaches `onImage` on the first snapshot); make `canPaint` unused in `pump` (expected: `testACoveredWindowRendersNothingAndResumes` fails on `navigationCount == 0`, because the page is loaded although the window is covered); make `configure(client: nil)` keep the old `baseURL` (expected: `testTheRendererStopsWhileTapIsDown` fails on `navigationCount`).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): render thumbnails in a hidden print-mode web view behind the editor"
```

---

### Task 7: The slide panel and its link to the cursor

**Files:**
- Create: `desktop/Tap/Sidebar/ThumbnailItem.swift`, `desktop/Tap/Sidebar/SlidePanelViewController.swift`, `desktop/Tap/Sidebar/SidebarHostViewController.swift`
- Modify: `desktop/Tap/Windows/MainSplitViewController.swift`, `desktop/Tap/Windows/DeckWindowController.swift`, `desktop/Tap/Documents/DeckSessionController.swift`, `desktop/TapTests/WindowLayoutTests.swift` (D2's divider and hide-preview tests, which now run with a sidebar in the split)
- Test: `desktop/TapTests/SidebarTests.swift`

**Interfaces:**
- Consumes: `SlideAccessibility.label(for:)` (Task 3), `EditorTextView.moveCursor(toSlide:)`, `currentBoxIndex`, `boxes` (D2), `MainSplitViewController` (D2).
- Produces:
  - `final class ThumbnailItem: NSCollectionViewItem` with `static let identifier`, `static let imageSize = NSSize(width: 150, height: 84)`, `func configure(slide: Slide, image: NSImage?, isUpdating: Bool)`, `var isUpdating: Bool`, `let thumbnailImageView: NSImageView` (not `imageView`, which `NSCollectionViewItem` already declares), `let numberLabel: NSTextField`, `let updatingLabel: NSTextField`
  - `protocol SlidePanelDelegate: AnyObject { func slidePanel(_:didClickSlide:selection:); func slidePanelSelectionDidChange(_:) }` (Tasks 11 and 14 add methods)
  - `final class SlidePanelViewController: NSViewController` with `weak var delegate`, `let collectionView: SlidePanelCollectionView`, `private(set) var slides: [Slide]`, `func setSlides(_:)`, `func setImage(_:forSlide:)`, `func image(forSlide:) -> NSImage?`, `func setUpdating(_ numbers: Set<Int>)`, `func remapImages(sourceNumbers: [Int?])` (moves the images with the slides after an operation), `func select(numbers:scroll:)`, `var selectedNumbers: [Int]`, `var visibleNumbers: [Int]`, `func item(forSlide:) -> ThumbnailItem?`, `func click(slide:extendingSelection:)` (the one path a click takes, used by the real mouse and by tests; a Shift-click extends from the slide last clicked without Shift), `var onVisibleRangeChanged: (() -> Void)?`, `static let width: CGFloat = 200`
  - `final class SidebarHostViewController: NSViewController` with `func host(_ panel: NSView)`
  - `MainSplitViewController.init(sidebar:editor:inspector:)`, `let sidebarItem: NSSplitViewItem`, `var isSidebarCollapsed: Bool`, `func setSidebarCollapsed(_:)`
  - `DeckSessionController.slidePanel: SlidePanelViewController`, `var currentSlideNumber: Int?`, `var selectedSlideNumbers: [Int]`

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/SidebarTests.swift`:

```swift
import XCTest
@testable import Tap

final class SidebarTests: HostedTestCase {
    func testTheCurrentSlide() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        let panel = controller.slidePanel
        try await waitUntil(timeout: 5, "seven thumbnails") { panel.slides.count == 7 }

        controller.editor.moveCursor(toSlide: 2)
        XCTAssertEqual(controller.editor.currentBoxIndex, 2, "the box of slide 3 is the highlighted one")
        XCTAssertEqual(controller.currentSlideNumber, 3)
        XCTAssertEqual(panel.selectedNumbers, [3], "thumbnail 3 is highlighted")
        XCTAssertTrue(try XCTUnwrap(panel.item(forSlide: 3)).isSelected)
        XCTAssertFalse(try XCTUnwrap(panel.item(forSlide: 1)).isSelected)
        XCTAssertEqual(try XCTUnwrap(panel.item(forSlide: 3)).numberLabel.stringValue, "3")
        XCTAssertEqual(panel.item(forSlide: 3)?.view.accessibilityLabel(), "Slide 3, default layout, What We Knew, 1 step")
    }

    func testSidebarAndCursorAreLinked() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        let panel = controller.slidePanel
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        try await waitUntil(timeout: 5, "seven thumbnails") { panel.slides.count == 7 }

        // A real click makes the collection view first responder before the selection changes; focus then stays there (decision 1).
        window.makeFirstResponder(panel.collectionView)
        panel.click(slide: 5, extendingSelection: false)
        XCTAssertTrue(window.firstResponder === panel.collectionView, "clicking a thumbnail leaves focus in the sidebar")
        XCTAssertEqual(controller.editor.selectedRange().location, controller.editor.boxes[4].range.location + ("# Root Cause" as NSString).length,
                       "the cursor moves to slide 5 (the end of its heading line, as Go to Slide does)")
        XCTAssertEqual(controller.editor.currentBoxIndex, 4)
        try await waitForPreview(document, slide: 5)

        panel.click(slide: 7, extendingSelection: true)
        XCTAssertEqual(panel.selectedNumbers, [5, 6, 7], "Shift-click selects the range")
        XCTAssertEqual(controller.editor.currentBoxIndex, 6, "and the cursor is in the slide clicked last")
        XCTAssertEqual(controller.selectedSlideNumbers, [5, 6, 7])
        panel.click(slide: 6, extendingSelection: false)
        panel.click(slide: 2, extendingSelection: true)
        XCTAssertEqual(panel.selectedNumbers, [2, 3, 4, 5, 6], "the anchor is the slide last clicked without Shift, whichever way the range runs")
        panel.click(slide: 4, extendingSelection: true)
        XCTAssertEqual(panel.selectedNumbers, [4, 5, 6], "a second Shift-click extends from the same anchor, not from the range's end")

        controller.editor.moveCursor(toSlide: 1)
        XCTAssertEqual(panel.selectedNumbers, [2], "moving the cursor selects only that thumbnail")
        XCTAssertEqual(controller.selectedSlideNumbers, [2])
    }

    func testThePanelIsAPinnedSidebarNextToTheEditor() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let windowController = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        let split = windowController.splitViewController
        let window = try XCTUnwrap(windowController.window)
        split.view.layoutSubtreeIfNeeded()
        XCTAssertFalse(split.sidebarItem.isCollapsed)
        XCTAssertEqual(split.splitView.bounds.width, window.contentView?.bounds.width ?? 0, accuracy: 1, "the split fills the window; pinning never resizes the window")
        XCTAssertEqual(split.sidebarItem.viewController.view.frame.width, SlidePanelViewController.width + 24, accuracy: 1)
        XCTAssertGreaterThanOrEqual(split.editorItem.viewController.view.frame.minX, split.sidebarItem.viewController.view.frame.maxX, "no overlap")
        XCTAssertEqual(split.editorItem.viewController.view.frame.width, split.inspectorItem.viewController.view.frame.width, accuracy: 2,
                       "the editor and the preview still split what is left evenly")
    }
}
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/SidebarTests/testTheCurrentSlide`
Expected: compile failure, `value of type 'DeckSessionController' has no member 'slidePanel'`.

- [ ] **Step 3: Write `ThumbnailItem.swift`**

```swift
import AppKit

/// One thumbnail in the slide panel: the number, the image, an "updating"
/// mark while a new render is on its way, and the selection ring.
final class ThumbnailItem: NSCollectionViewItem {
    static let identifier = NSUserInterfaceItemIdentifier("ThumbnailItem")
    static let imageSize = NSSize(width: 150, height: 84)
    static let numberWidth: CGFloat = 20
    static let gap: CGFloat = 8

    let numberLabel = NSTextField(labelWithString: "")
    let thumbnailImageView = NSImageView()
    let updatingLabel = NSTextField(labelWithString: "updating")
    private(set) var slide: Slide?

    var isUpdating = false {
        didSet { updatingLabel.isHidden = !isUpdating }
    }

    override func loadView() {
        let root = NSView()
        numberLabel.font = .monospacedDigitSystemFont(ofSize: 11, weight: .semibold)
        numberLabel.alignment = .right
        numberLabel.textColor = .secondaryLabelColor
        thumbnailImageView.wantsLayer = true
        thumbnailImageView.imageScaling = .scaleProportionallyUpOrDown
        thumbnailImageView.layer?.cornerRadius = 6
        thumbnailImageView.layer?.masksToBounds = true
        thumbnailImageView.layer?.backgroundColor = NSColor.quaternaryLabelColor.cgColor
        updatingLabel.font = .systemFont(ofSize: 9, weight: .medium)
        updatingLabel.textColor = .white
        updatingLabel.wantsLayer = true
        updatingLabel.layer?.backgroundColor = NSColor.black.withAlphaComponent(0.55).cgColor
        updatingLabel.layer?.cornerRadius = 4
        updatingLabel.isHidden = true
        for view in [numberLabel, thumbnailImageView, updatingLabel] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(view)
        }
        NSLayoutConstraint.activate([
            numberLabel.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            numberLabel.widthAnchor.constraint(equalToConstant: Self.numberWidth),
            numberLabel.topAnchor.constraint(equalTo: root.topAnchor, constant: 2),
            thumbnailImageView.leadingAnchor.constraint(equalTo: numberLabel.trailingAnchor, constant: Self.gap),
            thumbnailImageView.topAnchor.constraint(equalTo: root.topAnchor),
            thumbnailImageView.widthAnchor.constraint(equalToConstant: Self.imageSize.width),
            thumbnailImageView.heightAnchor.constraint(equalToConstant: Self.imageSize.height),
            updatingLabel.trailingAnchor.constraint(equalTo: thumbnailImageView.trailingAnchor, constant: -4),
            updatingLabel.bottomAnchor.constraint(equalTo: thumbnailImageView.bottomAnchor, constant: -4),
        ])
        root.setAccessibilityElement(true)
        root.setAccessibilityRole(.button)
        view = root
        applySelection()
    }

    func configure(slide: Slide, image: NSImage?, isUpdating: Bool) {
        self.slide = slide
        numberLabel.stringValue = "\(slide.number)"
        thumbnailImageView.image = image
        self.isUpdating = isUpdating
        // A skipped slide is dimmed, as its box is in the editor.
        thumbnailImageView.alphaValue = slide.skip ? 0.45 : 1
        numberLabel.alphaValue = slide.skip ? 0.6 : 1
        view.setAccessibilityLabel(SlideAccessibility.label(for: slide))
        view.setAccessibilityIdentifier("thumbnail-\(slide.number)")
    }

    override var isSelected: Bool {
        didSet { applySelection() }
    }

    private func applySelection() {
        thumbnailImageView.layer?.borderWidth = isSelected ? 3 : 0.5
        thumbnailImageView.layer?.borderColor = isSelected ? NSColor.controlAccentColor.cgColor : NSColor.black.withAlphaComponent(0.07).cgColor
        numberLabel.textColor = isSelected ? .controlAccentColor : .secondaryLabelColor
        view.setAccessibilitySelected(isSelected)
    }
}
```

- [ ] **Step 4: Write `SlidePanelViewController.swift`**

```swift
import AppKit

protocol SlidePanelDelegate: AnyObject {
    /// A click on a thumbnail. `selection` is the panel's selection after the click.
    func slidePanel(_ panel: SlidePanelViewController, didClickSlide number: Int, selection: [Int])
    func slidePanelSelectionDidChange(_ panel: SlidePanelViewController)
}

/// The collection view, which remembers which item a click landed on, so
/// a Shift-click reports the slide clicked last rather than the range's end.
final class SlidePanelCollectionView: NSCollectionView {
    private(set) var clickedIndexPath: IndexPath?

    override func mouseDown(with event: NSEvent) {
        clickedIndexPath = indexPathForItem(at: convert(event.locationInWindow, from: nil))
        super.mouseDown(with: event)
    }

    override var acceptsFirstResponder: Bool { true }
}

/// The thumbnails, in a virtualized collection view. The panel is linked
/// to the cursor both ways by its delegate: a click moves the cursor, and
/// a cursor move selects the thumbnail.
final class SlidePanelViewController: NSViewController, NSCollectionViewDataSource, NSCollectionViewDelegateFlowLayout {
    static let width: CGFloat = 200

    weak var delegate: SlidePanelDelegate?
    let collectionView = SlidePanelCollectionView()
    let scrollView = NSScrollView()
    let titleLabel = NSTextField(labelWithString: "Slides")
    /// Runs when the visible thumbnails change, so the renderer can reorder its queue.
    var onVisibleRangeChanged: (() -> Void)?
    private(set) var slides: [Slide] = []
    private var images: [Int: NSImage] = [:]
    private var updating: Set<Int> = []
    private var isSyncingSelection = false
    /// The slide last clicked without Shift: a Shift-click extends from it.
    private var selectionAnchor: Int?

    override func loadView() {
        let root = NSView()
        titleLabel.font = .systemFont(ofSize: 12, weight: .semibold)
        titleLabel.textColor = .secondaryLabelColor

        let layout = NSCollectionViewFlowLayout()
        layout.itemSize = NSSize(width: ThumbnailItem.numberWidth + ThumbnailItem.gap + ThumbnailItem.imageSize.width, height: ThumbnailItem.imageSize.height)
        layout.minimumLineSpacing = 10
        layout.sectionInset = NSEdgeInsets(top: 4, left: 6, bottom: 12, right: 12)
        collectionView.collectionViewLayout = layout
        collectionView.isSelectable = true
        collectionView.allowsMultipleSelection = true
        collectionView.allowsEmptySelection = true
        collectionView.backgroundColors = [.clear]
        collectionView.register(ThumbnailItem.self, forItemWithIdentifier: ThumbnailItem.identifier)
        collectionView.dataSource = self
        collectionView.delegate = self
        collectionView.setAccessibilityIdentifier("slide-panel")
        collectionView.setAccessibilityLabel("Slides")
        scrollView.documentView = collectionView
        scrollView.drawsBackground = false
        scrollView.hasVerticalScroller = true
        scrollView.automaticallyAdjustsContentInsets = false
        scrollView.contentView.postsBoundsChangedNotifications = true
        NotificationCenter.default.addObserver(self, selector: #selector(boundsChanged(_:)), name: NSView.boundsDidChangeNotification, object: scrollView.contentView)

        for view in [titleLabel, scrollView] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(view)
        }
        NSLayoutConstraint.activate([
            titleLabel.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: 14),
            titleLabel.centerYAnchor.constraint(equalTo: root.topAnchor, constant: 20),
            scrollView.topAnchor.constraint(equalTo: root.topAnchor, constant: 40),
            scrollView.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            scrollView.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            scrollView.bottomAnchor.constraint(equalTo: root.bottomAnchor),
        ])
        view = root
    }

    deinit {
        NotificationCenter.default.removeObserver(self)
    }

    @objc private func boundsChanged(_ notification: Notification) {
        onVisibleRangeChanged?()
    }

    // MARK: Content

    /// Replaces the slides. The selection is kept by number where the
    /// numbers still exist.
    func setSlides(_ newSlides: [Slide]) {
        let previousSelection = selectedNumbers
        let countChanged = newSlides.count != slides.count
        slides = newSlides
        images = images.filter { $0.key <= newSlides.count }
        updating = updating.filter { $0 <= newSlides.count }
        if countChanged {
            collectionView.reloadData()
        } else {
            for (index, slide) in newSlides.enumerated() {
                (collectionView.item(at: IndexPath(item: index, section: 0)) as? ThumbnailItem)?
                    .configure(slide: slide, image: images[slide.number], isUpdating: updating.contains(slide.number))
            }
        }
        let kept = previousSelection.filter { $0 <= newSlides.count }
        if kept != selectedNumbers { select(numbers: kept, scroll: false) }
    }

    func setImage(_ image: NSImage?, forSlide number: Int) {
        images[number] = image
        updating.remove(number)
        refreshItem(forSlide: number)
    }

    func image(forSlide number: Int) -> NSImage? {
        images[number]
    }

    /// Moves the images with their slides after an operation: the new slide
    /// `index + 1` shows the image the old slide `sourceNumbers[index]` had,
    /// and a created slide has none until the renderer delivers one.
    func remapImages(sourceNumbers: [Int?]) {
        let old = images
        images = [:]
        for (index, source) in sourceNumbers.enumerated() {
            if let source, let image = old[source] { images[index + 1] = image }
        }
        for number in 1...max(1, slides.count) { refreshItem(forSlide: number) }
    }

    /// Marks the slides whose thumbnails are being rendered again. Their
    /// old images stay until the new ones replace them.
    func setUpdating(_ numbers: Set<Int>) {
        let changed = updating.symmetricDifference(numbers)
        updating = numbers
        changed.forEach(refreshItem(forSlide:))
    }

    private func refreshItem(forSlide number: Int) {
        guard number >= 1, number <= slides.count,
              let item = collectionView.item(at: IndexPath(item: number - 1, section: 0)) as? ThumbnailItem else { return }
        item.configure(slide: slides[number - 1], image: images[number], isUpdating: updating.contains(number))
    }

    func item(forSlide number: Int) -> ThumbnailItem? {
        guard number >= 1, number <= slides.count else { return nil }
        return collectionView.item(at: IndexPath(item: number - 1, section: 0)) as? ThumbnailItem
    }

    // MARK: Selection

    var selectedNumbers: [Int] {
        collectionView.selectionIndexPaths.map { $0.item + 1 }.sorted()
    }

    var visibleNumbers: [Int] {
        collectionView.indexPathsForVisibleItems().map { $0.item + 1 }.sorted()
    }

    /// Selects thumbnails without telling the delegate: the cursor moved,
    /// or an operation chose the slides it made.
    func select(numbers: [Int], scroll: Bool) {
        let paths = Set(numbers.filter { $0 >= 1 && $0 <= slides.count }.map { IndexPath(item: $0 - 1, section: 0) })
        guard collectionView.selectionIndexPaths != paths else { return }
        isSyncingSelection = true
        collectionView.selectionIndexPaths = paths
        isSyncingSelection = false
        if scroll, let first = paths.min() {
            collectionView.scrollToItems(at: [first], scrollPosition: .nearestHorizontalEdge)
        }
    }

    /// A click on a thumbnail, with or without Shift: the same path the
    /// mouse takes through `didSelectItemsAt`. With Shift, the selection
    /// extends from its anchor to the clicked slide; the cursor goes to the
    /// clicked slide either way.
    func click(slide number: Int, extendingSelection: Bool) {
        guard number >= 1, number <= slides.count else { return }
        var numbers = [number]
        if extendingSelection, let anchor = selectionAnchor ?? selectedNumbers.first {
            numbers = Array(min(anchor, number)...max(anchor, number))
        } else {
            selectionAnchor = number
        }
        select(numbers: numbers, scroll: false)
        delegate?.slidePanel(self, didClickSlide: number, selection: selectedNumbers)
    }

    // MARK: NSCollectionViewDataSource and delegate

    func collectionView(_ collectionView: NSCollectionView, numberOfItemsInSection section: Int) -> Int {
        slides.count
    }

    func collectionView(_ collectionView: NSCollectionView, itemForRepresentedObjectAt indexPath: IndexPath) -> NSCollectionViewItem {
        let item = collectionView.makeItem(withIdentifier: ThumbnailItem.identifier, for: indexPath) as! ThumbnailItem
        let slide = slides[indexPath.item]
        item.configure(slide: slide, image: images[slide.number], isUpdating: updating.contains(slide.number))
        return item
    }

    func collectionView(_ collectionView: NSCollectionView, didSelectItemsAt indexPaths: Set<IndexPath>) {
        guard !isSyncingSelection else { return }
        let clicked = self.collectionView.clickedIndexPath.map { $0.item + 1 } ?? (indexPaths.map { $0.item + 1 }.max() ?? 0)
        guard clicked > 0 else { return }
        if NSApp.currentEvent?.modifierFlags.contains(.shift) != true { selectionAnchor = clicked }
        delegate?.slidePanel(self, didClickSlide: clicked, selection: selectedNumbers)
    }

    func collectionView(_ collectionView: NSCollectionView, didDeselectItemsAt indexPaths: Set<IndexPath>) {
        guard !isSyncingSelection else { return }
        delegate?.slidePanelSelectionDidChange(self)
    }
}
```

`SidebarHostViewController.swift`:

```swift
import AppKit

/// The sidebar split item's controller: an empty view the slide panel's
/// view is placed in while the panel is pinned.
final class SidebarHostViewController: NSViewController {
    override func loadView() {
        view = NSView()
    }

    func host(_ panel: NSView) {
        panel.removeFromSuperview()
        panel.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(panel)
        NSLayoutConstraint.activate([
            panel.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor),
            panel.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            panel.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            panel.bottomAnchor.constraint(equalTo: view.bottomAnchor),
        ])
    }
}
```

- [ ] **Step 5: Give the split view a sidebar**

In `MainSplitViewController.swift`, replace the initializer and `balance()`:

```swift
    let sidebarItem: NSSplitViewItem
    let editorItem: NSSplitViewItem
    let inspectorItem: NSSplitViewItem
    /// The sidebar's width: the panel plus the stock sidebar's inset.
    static let sidebarWidth = SlidePanelViewController.width + 24

    init(sidebar: NSViewController, editor: NSViewController, inspector: NSViewController) {
        sidebarItem = NSSplitViewItem(sidebarWithViewController: sidebar)
        sidebarItem.minimumThickness = Self.sidebarWidth
        sidebarItem.maximumThickness = Self.sidebarWidth
        sidebarItem.canCollapse = true
        // Collapsing or expanding the sidebar moves the editor and the right
        // pane, never the window: the pinned panel pushes them to the right.
        sidebarItem.collapseBehavior = .preferResizingSiblingsWithFixedSplitView
        editorItem = NSSplitViewItem(viewController: editor)
        editorItem.minimumThickness = 320
        inspectorItem = NSSplitViewItem(viewController: inspector)
        inspectorItem.minimumThickness = 320
        inspectorItem.canCollapse = true
        super.init(nibName: nil, bundle: nil)
        addSplitViewItem(sidebarItem)
        addSplitViewItem(editorItem)
        addSplitViewItem(inspectorItem)
    }

    var isSidebarCollapsed: Bool { sidebarItem.isCollapsed }

    func setSidebarCollapsed(_ collapsed: Bool) {
        guard sidebarItem.isCollapsed != collapsed else { return }
        sidebarItem.isCollapsed = collapsed
        view.needsLayout = true
        view.layoutSubtreeIfNeeded()
        balance()
    }

    /// The divider between the editor and the right pane, which sits after
    /// the sidebar's own divider.
    private var editorDividerIndex: Int { 1 }

    func balance() {
        guard !inspectorItem.isCollapsed else { return }
        let leadingWidth = sidebarItem.isCollapsed ? 0 : sidebarItem.viewController.view.frame.width
        guard let position = dividerPolicy.balancedPosition(totalWidth: splitView.bounds.width, dividerThickness: splitView.dividerThickness,
                                                            leadingWidth: leadingWidth),
              abs(editorItem.viewController.view.frame.maxX - position) > 0.5 else { return }
        isBalancing = true
        splitView.setPosition(position, ofDividerAt: editorDividerIndex)
        isBalancing = false
    }
```

In `splitViewDidResizeSubviews`, the user-drag check stays as it is: it counts a mouseDown that hit the split view itself, whichever divider.

Two D2 tests in `WindowLayoutTests.swift` assume the editor is the split's first pane; both must stay honest with a sidebar in front. In `testHideThePreview`, the panel is unpinned first (`controller.setPanelPinned(false)` arrives in the next task; until then use `split.setSidebarCollapsed(true)`), so "the editor takes the full width" is still what is asserted:

```swift
        split.setSidebarCollapsed(true)
        controller.togglePreview(nil)
        split.view.layoutSubtreeIfNeeded()
        XCTAssertTrue(split.isPreviewHidden)
        XCTAssertEqual(split.editorItem.viewController.view.frame.width, split.splitView.bounds.width, accuracy: 1)
```

In `testTheDividerStaysInTheMiddleUntilTheUserDragsIt`, the divider the person drags is index 1 now (index 0 is the sidebar's fixed-width divider): `split.splitView.setPosition(split.sidebarItem.viewController.view.frame.width + 1 + 100, ofDividerAt: 1)`. The equal-width assertions compare the editor and the inspector to each other, as before, so they hold with or without the sidebar.

- [ ] **Step 6: Wire the panel into the session controller and the window**

In `DeckSessionController`:

```swift
    let slidePanel = SlidePanelViewController()
    /// True while a panel click moves the cursor, so the cursor's own
    /// selection sync does not collapse a Shift-click's range.
    private var isSelectingFromPanel = false
```

In `init`, after `editor.editorDelegate = self`: `slidePanel.delegate = self`.

In `applySlideList`, after the `guard editor.apply(...)`: `slidePanel.setSlides(editor.boxes.map(\.slide))`.

Add:

```swift
    /// The number of the slide under the cursor.
    var currentSlideNumber: Int? {
        editor.currentBoxIndex.map { editor.boxes[$0].slide.number }
    }

    /// The slides an operation acts on: the panel's selection, which
    /// follows the cursor when nothing was selected by hand.
    var selectedSlideNumbers: [Int] {
        let selected = slidePanel.selectedNumbers
        if !selected.isEmpty { return selected }
        return currentSlideNumber.map { [$0] } ?? []
    }

    /// Selects the cursor's slide alone, unless the panel is driving the cursor.
    func syncPanelSelectionToCursor() {
        guard !isSelectingFromPanel, let number = currentSlideNumber else { return }
        slidePanel.select(numbers: [number], scroll: true)
    }
```

In `editor(_:currentSlideDidChange:)`, add `syncPanelSelectionToCursor()` after the preview message.

Conformance:

```swift
extension DeckSessionController: SlidePanelDelegate {
    func slidePanel(_ panel: SlidePanelViewController, didClickSlide number: Int, selection: [Int]) {
        guard let index = editor.boxes.firstIndex(where: { $0.slide.number == number }) else { return }
        isSelectingFromPanel = true
        editor.moveCursor(toSlide: index)
        isSelectingFromPanel = false
    }

    func slidePanelSelectionDidChange(_ panel: SlidePanelViewController) {}
}
```

In `DeckWindowController.init`, build the split with the sidebar and host the panel:

```swift
        let sidebarHost = SidebarHostViewController()
        sidebarHost.host(sessionController.slidePanel.view)
        splitViewController = MainSplitViewController(sidebar: sidebarHost, editor: sessionController.editorViewController,
                                                      inspector: sessionController.inspectorViewController)
```

(`sidebarHost` becomes a stored property in Task 8; a `let` local is enough here.)

- [ ] **Step 7: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/SidebarTests/testTheCurrentSlide
make -C desktop test ONLY=TapTests/SidebarTests/testSidebarAndCursorAreLinked
make -C desktop test ONLY=TapTests/SidebarTests/testThePanelIsAPinnedSidebarNextToTheEditor
make -C desktop test ONLY=TapTests/WindowLayoutTests/testTheDividerStaysInTheMiddleUntilTheUserDragsIt
make -C desktop test ONLY=TapTests/WindowLayoutTests/testHideThePreview
```

Expected: all pass. The last two are D2's, edited above to run with a sidebar in the split.

- [ ] **Step 8: Mutate and commit**

Mutations: drop `syncPanelSelectionToCursor()` from `currentSlideDidChange` (expected: `testTheCurrentSlide` fails on `selectedNumbers`); in `click(slide:extendingSelection:)`, ignore `extendingSelection` (expected: `testSidebarAndCursorAreLinked` fails on `[5, 6, 7]`); add `editor.window?.makeFirstResponder(editor)` to `slidePanel(_:didClickSlide:selection:)` (expected: the same test fails on its first-responder assertion, which is decision 1's protection); anchor on `selectedNumbers.first` instead of `selectionAnchor` (expected: it fails on `[4, 5, 6]`).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): the slide panel, linked to the cursor both ways"
```

---

### Task 8: Peek on hover and click to pin

**Files:**
- Create: `desktop/Tap/Sidebar/SlidePanelOverlay.swift`, `desktop/Tap/Sidebar/SlidePanelPeek.swift`, `desktop/Tap/Sidebar/HoverButton.swift`, `desktop/Tap/Windows/DeckContentViewController.swift`
- Modify: `desktop/Tap/Windows/DeckWindowController.swift`, `desktop/Tap/App/MainMenu.swift`, `desktop/Tap/App/AppEnvironment.swift`, `desktop/Tap/Documents/DeckSessionController.swift` (the panel state follows `deckMoved`), `desktop/TapTests/Support/HostedTestCase.swift`, `desktop/TapTests/WindowLayoutTests.swift` (`testHideThePreview` unpins through `setPanelPinned(false)`)
- Test: `desktop/TapTests/SlidePanelLayoutTests.swift`

**Interfaces:**
- Consumes: `SlidePanelState` (Task 3), `SidebarHostViewController`, `MainSplitViewController.setSidebarCollapsed` (Task 7), `DividerPolicy` (Task 3).
- Produces:
  - `final class SlidePanelOverlay: NSView` with `let contentView: NSView`, `func host(_ panel: NSView)`, `var onPointerEntered`, `var onPointerLeft`; `NSGlassEffectView` on macOS 26, `NSVisualEffectView` (`.popover`, `.withinWindow`) before.
  - `final class SlidePanelPeek` (`@MainActor`) with `var showDelay`, `var hideDelay`, `var isEnabled`, `private(set) var isShowing`, `var onShow`, `var onHide`, `func pointerEnteredButton()`, `pointerLeftButton()`, `pointerEnteredPanel()`, `pointerLeftPanel()`, `hideNow()`.
  - `final class HoverButton: NSButton` with `var onPointerEntered`, `var onPointerLeft`.
  - `final class DeckContentViewController: NSViewController`: the window's content view controller, holding the split view controller as a child and the overlay as its sibling, so the overlay is never a pane of the split view.
  - `DeckWindowController`: `static let slidesItemIdentifier`, `let contentViewController: DeckContentViewController`, `let panelPeek: SlidePanelPeek`, `let panelOverlay: SlidePanelOverlay`, `private(set) var isPanelPinned: Bool`, `func setPanelPinned(_:)`, `@objc func toggleSlidePanel(_:)`. When the peek hides while the first responder is inside the overlay, the editor becomes first responder, so typing and Delete never go to a hidden panel.
  - `AppEnvironment.shared.panelState: SlidePanelState` (tests replace it with one on a fresh `UserDefaults` suite).
  - View menu: "Unpin Slide Panel" / "Pin Slide Panel", Control+Command+S.

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/SlidePanelLayoutTests.swift`:

```swift
import XCTest
@testable import Tap

final class SlidePanelLayoutTests: HostedTestCase {
    func windowController(for document: DeckDocument) throws -> DeckWindowController {
        try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
    }

    func testSplitLayout() async throws {
        let deck = try Fixtures.copyDeck("plain.md")
        let document = try await openDeck(deck)
        let controller = try windowController(for: document)
        let split = controller.splitViewController
        split.view.layoutSubtreeIfNeeded()
        XCTAssertTrue(controller.isPanelPinned, "pinned on first launch")
        XCTAssertFalse(split.sidebarItem.isCollapsed)
        XCTAssertEqual(split.editorItem.viewController.view.frame.width, split.inspectorItem.viewController.view.frame.width, accuracy: 2,
                       "the editor and the preview each take half of what is left")

        controller.setPanelPinned(false)
        let renamed = deck.deletingLastPathComponent().appendingPathComponent("renamed.md")
        try await document.move(to: renamed)
        document.close()
        try await waitUntil(timeout: 10, "the window to close") { document.windowControllers.first?.window?.isVisible != true }
        let reopened = try await openDeck(renamed)
        let again = try windowController(for: reopened)
        XCTAssertFalse(again.isPanelPinned, "each deck remembers its state")
        XCTAssertTrue(again.splitViewController.sidebarItem.isCollapsed)
    }

    func testPeekAtTheSlidePanel() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try windowController(for: document)
        let session = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(controller.window)
        controller.setPanelPinned(false)
        controller.panelPeek.showDelay = 0.1
        controller.panelPeek.hideDelay = 0.05
        XCTAssertTrue(controller.panelOverlay.isHidden)
        XCTAssertFalse(controller.panelOverlay.isDescendant(of: controller.splitViewController.view), "the overlay is a sibling of the split view, never one of its panes")

        // A pointer that crosses the button and leaves before the show delay shows nothing.
        controller.panelPeek.pointerEnteredButton()
        controller.panelPeek.pointerLeftButton()
        try await Task.sleep(nanoseconds: 250_000_000)
        XCTAssertTrue(controller.panelOverlay.isHidden, "a pointer that only crossed the button does not open the panel")

        controller.panelPeek.pointerEnteredButton()
        try await waitUntil(timeout: 2, "the overlay to show") { !controller.panelOverlay.isHidden }
        XCTAssertTrue(session.slidePanel.view.isDescendant(of: controller.panelOverlay), "the panel is a glass overlay")
        if #available(macOS 26.0, *) {
            XCTAssertTrue(controller.panelOverlay.subviews.contains { $0 is NSGlassEffectView }, "the system's glass on macOS 26")
        }
        XCTAssertTrue(controller.splitViewController.sidebarItem.isCollapsed, "nothing is pushed aside")

        controller.panelPeek.pointerLeftButton()
        controller.panelPeek.pointerEnteredPanel()
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertFalse(controller.panelOverlay.isHidden, "it stays while the pointer is over the panel, well past the hide delay")
        window.makeFirstResponder(session.slidePanel.collectionView)
        session.slidePanel.click(slide: 5, extendingSelection: false)
        XCTAssertEqual(session.editor.currentBoxIndex, 4, "a click in it jumps there")

        controller.panelPeek.pointerLeftPanel()
        try await waitUntil(timeout: 2, "the overlay to hide") { controller.panelOverlay.isHidden }
        XCTAssertTrue(window.firstResponder === session.editor, "focus never stays in a hidden panel: the editor has it once the peek closes")
    }

    func testPinTheSlidePanel() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let controller = try windowController(for: document)
        let session = try XCTUnwrap(document.sessionController)
        let split = controller.splitViewController
        controller.setPanelPinned(false)
        split.view.layoutSubtreeIfNeeded()

        let frameBefore = try XCTUnwrap(controller.window?.frame)
        controller.toggleSlidePanel(nil)
        split.view.layoutSubtreeIfNeeded()
        XCTAssertEqual(controller.window?.frame, frameBefore, "pinning pushes the panes, it never resizes the window")
        XCTAssertTrue(controller.isPanelPinned)
        XCTAssertFalse(split.sidebarItem.isCollapsed, "the panel docks as a sidebar")
        XCTAssertTrue(session.slidePanel.view.isDescendant(of: split.sidebarItem.viewController.view))
        XCTAssertTrue(controller.panelOverlay.isHidden)
        XCTAssertGreaterThanOrEqual(split.editorItem.viewController.view.frame.minX, split.sidebarItem.viewController.view.frame.maxX,
                                    "the editor and preview are pushed right, with no overlap")
        XCTAssertEqual(split.editorItem.viewController.view.frame.width, split.inspectorItem.viewController.view.frame.width, accuracy: 2)
        XCTAssertFalse(controller.panelPeek.isEnabled, "hovering does nothing while pinned")

        controller.toggleSlidePanel(nil)
        split.view.layoutSubtreeIfNeeded()
        XCTAssertEqual(controller.window?.frame, frameBefore)
        XCTAssertFalse(controller.isPanelPinned)
        XCTAssertTrue(split.sidebarItem.isCollapsed)
        XCTAssertTrue(controller.panelPeek.isEnabled, "hovering the button peeks again")
        XCTAssertEqual(split.editorItem.viewController.view.frame.minX, 0, accuracy: 1)
        XCTAssertEqual(split.editorItem.viewController.view.frame.width, split.inspectorItem.viewController.view.frame.width, accuracy: 2)

        let menuItem = NSMenuItem(title: "", action: #selector(DeckWindowController.toggleSlidePanel(_:)), keyEquivalent: "")
        XCTAssertTrue(controller.validateMenuItem(menuItem))
        XCTAssertEqual(menuItem.title, "Pin Slide Panel")
        controller.toggleSlidePanel(nil)
        XCTAssertTrue(controller.validateMenuItem(menuItem))
        XCTAssertEqual(menuItem.title, "Unpin Slide Panel")
    }
}
```

In `HostedTestCase.setUp`, add a fresh panel state per test so "first launch" is true for every test:

```swift
        AppEnvironment.shared.panelState = SlidePanelState(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.\(UUID().uuidString)")))
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/SlidePanelLayoutTests/testPinTheSlidePanel`
Expected: compile failure, `has no member 'setPanelPinned'`.

- [ ] **Step 3: Write the overlay, the peek and the hover button**

`SlidePanelOverlay.swift`:

```swift
import AppKit

/// The peeked slide panel: a glass overlay at the window's left edge.
/// macOS 26 draws it with the system's glass; 14 and 15 with a popover
/// material inside the window.
final class SlidePanelOverlay: NSView {
    let contentView: NSView
    var onPointerEntered: (() -> Void)?
    var onPointerLeft: (() -> Void)?
    private var trackingArea: NSTrackingArea?

    override init(frame: NSRect) {
        let backing: NSView
        let content: NSView
        if #available(macOS 26.0, *) {
            let glass = NSGlassEffectView()
            glass.cornerRadius = 16
            content = NSView()
            glass.contentView = content
            backing = glass
        } else {
            let material = NSVisualEffectView()
            material.material = .popover
            material.blendingMode = .withinWindow
            material.state = .active
            material.wantsLayer = true
            material.layer?.cornerRadius = 16
            material.layer?.masksToBounds = true
            backing = material
            content = material
        }
        contentView = content
        super.init(frame: frame)
        wantsLayer = true
        shadow = NSShadow()
        layer?.shadowOpacity = 0.22
        layer?.shadowRadius = 25
        layer?.shadowOffset = CGSize(width: 0, height: -18)
        backing.translatesAutoresizingMaskIntoConstraints = false
        addSubview(backing)
        NSLayoutConstraint.activate([
            backing.topAnchor.constraint(equalTo: topAnchor),
            backing.leadingAnchor.constraint(equalTo: leadingAnchor),
            backing.trailingAnchor.constraint(equalTo: trailingAnchor),
            backing.bottomAnchor.constraint(equalTo: bottomAnchor),
        ])
        setAccessibilityIdentifier("slide-panel-overlay")
        isHidden = true
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    func host(_ panel: NSView) {
        panel.removeFromSuperview()
        panel.translatesAutoresizingMaskIntoConstraints = false
        contentView.addSubview(panel)
        NSLayoutConstraint.activate([
            panel.topAnchor.constraint(equalTo: contentView.topAnchor),
            panel.leadingAnchor.constraint(equalTo: contentView.leadingAnchor),
            panel.trailingAnchor.constraint(equalTo: contentView.trailingAnchor),
            panel.bottomAnchor.constraint(equalTo: contentView.bottomAnchor),
        ])
    }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let trackingArea { removeTrackingArea(trackingArea) }
        let area = NSTrackingArea(rect: bounds, options: [.mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        trackingArea = area
    }

    override func mouseEntered(with event: NSEvent) { onPointerEntered?() }
    override func mouseExited(with event: NSEvent) { onPointerLeft?() }
}
```

`SlidePanelPeek.swift`:

```swift
import Foundation

/// When the peeked panel shows and hides. It shows a moment after the
/// pointer reaches the toolbar button, stays while the pointer is over the
/// button or the panel, and hides a moment after it has left both.
@MainActor
final class SlidePanelPeek {
    var showDelay: TimeInterval = 0.15
    var hideDelay: TimeInterval = 0.25
    /// False while the panel is pinned: hovering then does nothing.
    var isEnabled = true {
        didSet { if !isEnabled { hideNow() } }
    }
    var onShow: (() -> Void)?
    var onHide: (() -> Void)?
    private(set) var isShowing = false
    private var overButton = false
    private var overPanel = false
    private var pending: DispatchWorkItem?

    func pointerEnteredButton() {
        overButton = true
        guard isEnabled, !isShowing else { return }
        schedule(after: showDelay) { [weak self] in
            guard let self, self.overButton || self.overPanel else { return }
            self.isShowing = true
            self.onShow?()
        }
    }

    func pointerLeftButton() {
        overButton = false
        scheduleHide()
    }

    func pointerEnteredPanel() {
        overPanel = true
        pending?.cancel()
    }

    func pointerLeftPanel() {
        overPanel = false
        scheduleHide()
    }

    func hideNow() {
        pending?.cancel()
        pending = nil
        guard isShowing else { return }
        isShowing = false
        onHide?()
    }

    private func scheduleHide() {
        guard !overButton, !overPanel else { return }
        schedule(after: hideDelay) { [weak self] in
            guard let self, !self.overButton, !self.overPanel else { return }
            self.hideNow()
        }
    }

    private func schedule(after delay: TimeInterval, _ work: @escaping () -> Void) {
        pending?.cancel()
        let item = DispatchWorkItem { MainActor.assumeIsolated { work() } }
        pending = item
        DispatchQueue.main.asyncAfter(deadline: .now() + delay, execute: item)
    }
}
```

`DeckContentViewController.swift`:

```swift
import AppKit

/// The deck window's content: the split view controller's view filling
/// it, and the peek overlay laid over it as a sibling. The overlay must
/// not be inside the split view, which lays out every subview it has as a
/// pane.
final class DeckContentViewController: NSViewController {
    let splitViewController: MainSplitViewController
    let overlay: SlidePanelOverlay

    init(splitViewController: MainSplitViewController, overlay: SlidePanelOverlay) {
        self.splitViewController = splitViewController
        self.overlay = overlay
        super.init(nibName: nil, bundle: nil)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func loadView() {
        let root = NSView()
        addChild(splitViewController)
        for view in [splitViewController.view, overlay] {
            view.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(view)
        }
        NSLayoutConstraint.activate([
            splitViewController.view.topAnchor.constraint(equalTo: root.topAnchor),
            splitViewController.view.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            splitViewController.view.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            splitViewController.view.bottomAnchor.constraint(equalTo: root.bottomAnchor),
            overlay.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: 12),
            overlay.topAnchor.constraint(equalTo: root.safeAreaLayoutGuide.topAnchor, constant: 8),
            overlay.bottomAnchor.constraint(equalTo: root.bottomAnchor, constant: -12),
            overlay.widthAnchor.constraint(equalToConstant: SlidePanelViewController.width),
        ])
        view = root
    }
}
```

`HoverButton.swift`:

```swift
import AppKit

/// A toolbar button that reports the pointer entering and leaving it.
final class HoverButton: NSButton {
    var onPointerEntered: (() -> Void)?
    var onPointerLeft: (() -> Void)?
    private var trackingArea: NSTrackingArea?

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let trackingArea { removeTrackingArea(trackingArea) }
        let area = NSTrackingArea(rect: bounds, options: [.mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        trackingArea = area
    }

    override func mouseEntered(with event: NSEvent) { onPointerEntered?() }
    override func mouseExited(with event: NSEvent) { onPointerLeft?() }
}
```

- [ ] **Step 4: Pin and peek in the window controller**

In `DeckWindowController`:

```swift
    static let slidesItemIdentifier = NSToolbarItem.Identifier("slides")
    let sidebarHost = SidebarHostViewController()
    let contentViewController: DeckContentViewController
    let panelOverlay = SlidePanelOverlay(frame: .zero)
    let panelPeek = SlidePanelPeek()
    private(set) var isPanelPinned = true
    private let slidesButton = HoverButton()
```

In `init`, use `sidebarHost` for the split (replacing the previous task's local), build the content controller and make it the window's content: `contentViewController = DeckContentViewController(splitViewController: splitViewController, overlay: panelOverlay)` before `super.init`, and `window.contentViewController = contentViewController` in place of D2's `window.contentViewController = splitViewController`. After the toolbar is set:

```swift
        panelOverlay.onPointerEntered = { [weak self] in self?.panelPeek.pointerEnteredPanel() }
        panelOverlay.onPointerLeft = { [weak self] in self?.panelPeek.pointerLeftPanel() }
        panelPeek.onShow = { [weak self] in
            guard let self else { return }
            self.panelOverlay.host(self.sessionController.slidePanel.view)
            self.panelOverlay.isHidden = false
        }
        panelPeek.onHide = { [weak self] in
            guard let self else { return }
            // Focus must not stay in a panel nobody can see: a click in the peeked
            // panel left it in the collection view (decision 1), and a Delete there
            // would delete slides out of sight. The editor is inside this same
            // window, and the person's own pointer closed the peek.
            if let responder = self.window?.firstResponder as? NSView, responder.isDescendant(of: self.panelOverlay) {
                self.window?.makeFirstResponder(self.sessionController.editor)
            }
            self.panelOverlay.isHidden = true
        }
        let deckURL = sessionController.document?.fileURL
        setPanelPinned(deckURL.map { AppEnvironment.shared.panelState.isPinned(deck: $0) } ?? true)
```

Add:

```swift
    /// Pins the panel as a sidebar that pushes the editor and the right
    /// pane over, or unpins it so hovering the toolbar button peeks at it.
    func setPanelPinned(_ pinned: Bool) {
        isPanelPinned = pinned
        panelPeek.isEnabled = !pinned
        if pinned {
            panelOverlay.isHidden = true
            sidebarHost.host(sessionController.slidePanel.view)
            splitViewController.setSidebarCollapsed(false)
        } else {
            splitViewController.setSidebarCollapsed(true)
            panelOverlay.host(sessionController.slidePanel.view)
            panelOverlay.isHidden = true
        }
        slidesButton.state = pinned ? .on : .off
        if let deck = sessionController.document?.fileURL {
            AppEnvironment.shared.panelState.setPinned(pinned, deck: deck)
        }
    }

    @objc func toggleSlidePanel(_ sender: Any?) {
        setPanelPinned(!isPanelPinned)
    }
```

In `validateMenuItem`, add:

```swift
        if menuItem.action == #selector(toggleSlidePanel(_:)) {
            menuItem.title = isPanelPinned ? "Unpin Slide Panel" : "Pin Slide Panel"
        }
```

Toolbar: `toolbarDefaultItemIdentifiers` returns `[Self.slidesItemIdentifier, .flexibleSpace, Self.previewItemIdentifier]` (Task 13 adds New Slide before the flexible space). In `toolbar(_:itemForItemIdentifier:willBeInsertedIntoToolbar:)`, add a case:

```swift
        if identifier == Self.slidesItemIdentifier {
            let item = NSToolbarItem(itemIdentifier: identifier)
            item.label = "Slides"
            item.toolTip = "Hover to peek at the slides, click to pin them"
            slidesButton.image = NSImage(systemSymbolName: "sidebar.left", accessibilityDescription: "Slides")
            slidesButton.bezelStyle = .toolbar
            slidesButton.setButtonType(.pushOnPushOff)
            slidesButton.target = self
            slidesButton.action = #selector(toggleSlidePanel(_:))
            slidesButton.setAccessibilityIdentifier("slides-button")
            slidesButton.onPointerEntered = { [weak self] in self?.panelPeek.pointerEnteredButton() }
            slidesButton.onPointerLeft = { [weak self] in self?.panelPeek.pointerLeftButton() }
            item.view = slidesButton
            return item
        }
```

In `MainMenu.viewMenu()`, before Hide Preview: `menu.addItem(item("Unpin Slide Panel", action: #selector(DeckWindowController.toggleSlidePanel(_:)), key: "s", modifiers: [.command, .control]))`.

In `AppEnvironment`: `var panelState = SlidePanelState()`.

In `DeckSessionController.deckMoved(to:)`, before `session.changeDeck(to: url)`, the state follows the deck: `if let old = previousDeckURL { AppEnvironment.shared.panelState.moveState(from: old, to: url) }`, where `previousDeckURL` is a private property the controller sets from `document.fileURL` in `init` and at the end of `deckMoved`. `testSplitLayout` covers the rename: after unpinning, `document.move(to:)` the deck to a new name in the same folder (a coordinated move, as D2's `DeletedDeckTests` does it), close, reopen the new name, and assert it is still unpinned.

- [ ] **Step 5: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/SlidePanelLayoutTests/testSplitLayout
make -C desktop test ONLY=TapTests/SlidePanelLayoutTests/testPeekAtTheSlidePanel
make -C desktop test ONLY=TapTests/SlidePanelLayoutTests/testPinTheSlidePanel
make -C desktop test ONLY=TapTests/SidebarTests/testThePanelIsAPinnedSidebarNextToTheEditor
```

Expected: all pass. If `sidebarItem.isCollapsed = true` animates and the frame assertions read mid-animation, wrap the change in `NSAnimationContext.runAnimationGroup` with `duration = 0` inside `setSidebarCollapsed`; do not add sleeps to the tests.

- [ ] **Step 6: Mutate and commit**

Mutations, each reverted: in `setPanelPinned`, drop the `panelState.setPinned` call (expected: `testSplitLayout` fails on reopen); in `SlidePanelPeek.scheduleHide`'s closure, drop the `!self.overPanel` re-check (expected: `testPeekAtTheSlidePanel` fails, the overlay hides while the pointer is over the panel); in the show closure, drop the `overButton || overPanel` re-check (expected: it fails on "a pointer that only crossed the button"); drop the first-responder move in `onHide` (expected: it fails on its last assertion); use `.preferResizingSplitViewWithFixedSiblings` (expected: `testPinTheSlidePanel` fails on the window frame); add the overlay to the split view's view instead of the container (expected: the descendant assertion fails).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): peek at the slide panel on hover and pin it with a click"
```

---

### Task 9: Thumbnails in the panel: keys, cache, queue and the current slide first

**Files:**
- Create: `desktop/Tap/Thumbnails/ThumbnailController.swift`
- Create: `desktop/TapTests/Fixtures/stepped/talk.md`, `desktop/TapTests/Fixtures/stepped/slides/RollingDeploy.jsx`
- Modify: `desktop/Tap/Documents/DeckSessionController.swift`, `desktop/Tap/App/AppEnvironment.swift`, `desktop/TapTests/Support/HostedTestCase.swift`
- Test: `desktop/TapTests/ThumbnailTests.swift`

**Interfaces:**
- Consumes: `ThumbnailRenderer` (Task 6), `ThumbnailKey`, `ThumbnailCache`, `PresentationSummary`, `TapClient.presentation()` (Task 4), `SlidePanelViewController` (Task 7), `SourceSync.lastEditDate` (D2).
- Produces:
  - `final class ThumbnailController` (`@MainActor`) with `let renderer: ThumbnailRenderer`, `let cache: ThumbnailCache`, `weak var panel: SlidePanelViewController?`, `var client: TapClient?`, `var currentSlideNumber: () -> Int?`, `private(set) var lastSummary: PresentationSummary?`, `func key(forSlide number: Int) -> ThumbnailKey?`, `func deckChanged()`, `func reprioritize()`, `var onImagesChanged: (() -> Void)?`
  - `DeckSessionController.thumbnails: ThumbnailController`
  - `AppEnvironment.shared.thumbnailCache: ThumbnailCache` (tests replace it with a temporary folder)

- [ ] **Step 1: Write the fixture**

`desktop/TapTests/Fixtures/stepped/talk.md`:

```markdown
# Before

---

<!--
layout: ./slides/RollingDeploy.jsx
-->

# Rolling Deploy
```

`desktop/TapTests/Fixtures/stepped/slides/RollingDeploy.jsx`:

```jsx
export const steps = 5;

export default function RollingDeploy({ step }) {
	return <h1>Deploy step {step} of {steps}</h1>;
}
```

- [ ] **Step 2: Write the failing tests**

`desktop/TapTests/ThumbnailTests.swift`:

```swift
import WebKit
import XCTest
@testable import Tap

final class ThumbnailTests: HostedTestCase {
    func waitForThumbnails(_ document: DeckDocument, count: Int, timeout: TimeInterval = 40) async throws {
        let panel = try XCTUnwrap(document.sessionController?.slidePanel)
        try await waitUntil(timeout: timeout, "\(count) thumbnails") {
            panel.slides.count == count && (1...count).allSatisfy { panel.image(forSlide: $0) != nil }
        }
    }

    func webViews(in view: NSView) -> [WKWebView] {
        view.subviews.flatMap { subview -> [WKWebView] in
            (subview as? WKWebView).map { [$0] } ?? webViews(in: subview)
        }
    }

    func testThumbnails() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("seven-slides.md"))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForThumbnails(document, count: 7)

        let window = try XCTUnwrap(document.windowControllers.first?.window?.contentView)
        let views = webViews(in: window)
        XCTAssertEqual(views.count, 2, "the preview and the one hidden renderer")
        XCTAssertTrue(views.contains { $0 === controller.previewViewController.webView })
        XCTAssertTrue(views.contains { $0 === controller.thumbnails.renderer.webView })
        XCTAssertFalse(controller.previewViewController.webView.url?.query?.contains("print=true") ?? false, "only the preview is a live render")
        XCTAssertTrue(controller.thumbnails.renderer.webView.url?.query?.contains("print=true") ?? false)
        XCTAssertEqual(controller.thumbnails.renderer.renderCount, 7, "static images from one hidden renderer")
        let revision = try XCTUnwrap(controller.thumbnails.lastSummary?.revision)
        for number in 1...7 {
            let key = try XCTUnwrap(controller.thumbnails.key(forSlide: number))
            XCTAssertTrue(AppEnvironment.shared.thumbnailCache.contains(key), "slide \(number) is cached on disk")
            let ready = try XCTUnwrap(controller.thumbnails.renderer.readyBySlide[number], "slide \(number) was captured after tap's ready signal")
            XCTAssertEqual(ready.slide, number, "the ready signal the capture waited for was this slide's")
            XCTAssertEqual(ready.revision, revision, "and this render's")
        }
    }

    func testReopen() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let first = try await openDeckAndWaitForPreview(deck)
        try await waitForThumbnails(first, count: 7)
        XCTAssertEqual(first.sessionController?.thumbnails.renderer.renderCount, 7)
        first.close()
        try await waitUntil(timeout: 10, "the deck to close") { NSDocumentController.shared.documents.isEmpty }

        let second = try await openDeckAndWaitForPreview(deck)
        try await waitForThumbnails(second, count: 7)
        XCTAssertEqual(second.sessionController?.thumbnails.renderer.renderCount, 0, "every thumbnail came from the cache")
        XCTAssertEqual(second.sessionController?.thumbnails.renderer.pendingCount, 0)
    }

    func testTheCurrentThumbnailFollowsThePreview() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 4)
        try await waitForThumbnails(document, count: 4)
        let before = try XCTUnwrap(controller.slidePanel.image(forSlide: 3)?.tiffRepresentation)
        let rendersBefore = controller.thumbnails.renderer.renderCount
        let revisionBefore = try XCTUnwrap(controller.previewViewController.lastReady?.revision)

        // Two slides change in one pause, slide 1 and slide 3, and the cursor ends in slide 3.
        let text = controller.editor.string as NSString
        controller.editor.setSelectedRange(NSRange(location: text.range(of: "# App Mode Fixture").upperBound, length: 0))
        controller.editor.insertText(" too", replacementRange: NSRange(location: NSNotFound, length: 0))
        let headingEnd = (controller.editor.string as NSString).range(of: "# Fragments").upperBound
        controller.editor.setSelectedRange(NSRange(location: headingEnd, length: 0))
        controller.editor.insertText(" changed", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitForPreview(document, slide: 3)
        try await waitUntil(timeout: 10, "the preview to re-render slide 3") {
            controller.previewViewController.lastReady.map { $0.slide == 3 && $0.revision != revisionBefore } ?? false
        }
        try await waitUntil(timeout: 20, "the first new render") { controller.thumbnails.renderer.renderCount == rendersBefore + 1 }
        XCTAssertEqual(controller.thumbnails.renderer.lastRenderedSlide, 3, "the current slide renders before the other changed one")
        try await waitUntil(timeout: 20, "thumbnail 3 to change") { controller.slidePanel.image(forSlide: 3)?.tiffRepresentation != before }
        try await waitUntil(timeout: 20, "the second render") { controller.thumbnails.renderer.renderCount == rendersBefore + 2 }
        XCTAssertEqual(controller.thumbnails.renderer.lastRenderedSlide, 1)
        XCTAssertEqual(controller.thumbnails.renderer.renderCount, rendersBefore + 2, "only the changed slides rendered again")
        XCTAssertTrue(controller.slidePanel.item(forSlide: 3)?.isUpdating == false)
    }

    func testStepThroughACustomComponent() async throws {
        let deck = try Fixtures.copyDeck("stepped")
        let document = try await openDeckAndWaitForPreview(deck)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 2)
        try await waitUntil(timeout: 30, "tap to report 5 steps") { controller.editor.header(forBoxAt: 1).badges.contains("5 steps") }

        controller.editor.moveCursor(toSlide: 1)
        try await waitForPreview(document, slide: 2)
        XCTAssertEqual(controller.previewViewController.stepLabel.stringValue, "All steps shown, 5 of 5")
        let observer = HubObserver(ready: try await waitForRunningTap(document))
        defer { observer.close() }
        try await Task.sleep(nanoseconds: 300_000_000)
        controller.previewViewController.onStepBackward?()
        controller.previewViewController.onStepForward?()
        try await waitUntil(timeout: 5, "two slide messages") { observer.slideMessages.count >= 2 }
        XCTAssertEqual(observer.slideMessages.suffix(2).map(\.step), [4, 5], "the component receives the step through the hub, as in tap dev")

        try await waitForThumbnails(document, count: 2)
        XCTAssertEqual(controller.thumbnails.renderer.readyBySlide[2]?.step, 5, "the thumbnail shows the final step: tap renders previews at the last step")

        let component = deck.deletingLastPathComponent().appendingPathComponent("slides/RollingDeploy.jsx")
        let source = try String(contentsOf: component, encoding: .utf8)
        try source.replacingOccurrences(of: "steps = 5", with: "steps = 6").write(to: component, atomically: true, encoding: .utf8)
        try await waitUntil(timeout: 30, "tap to rebuild and report 6 steps") { controller.editor.header(forBoxAt: 1).badges.contains("6 steps") }
    }
}
```

The Review Focus test `testDuplicatedSlidesShareOneThumbnail` also belongs in this file, but it drives `perform(_:)`, which Task 10 adds; Task 10 adds the test.

In `HostedTestCase.setUp`, add: `AppEnvironment.shared.thumbnailCache = ThumbnailCache(directory: try Fixtures.temporaryFolder())`. Note that `testReopen` needs the cache to survive between its two opens, which it does: `setUp` runs once per test.

- [ ] **Step 3: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/ThumbnailTests/testThumbnails`
Expected: compile failure, `has no member 'thumbnails'`.

- [ ] **Step 4: Write `ThumbnailController.swift`**

```swift
import AppKit

/// Keeps the panel's thumbnails current. After every render tap answers,
/// it reads the presentation summary, works out each slide's key, shows
/// the images it has in memory or on disk, and hands the rest to the
/// renderer with the current slide first.
@MainActor
final class ThumbnailController {
    let renderer = ThumbnailRenderer()
    let cache: ThumbnailCache
    weak var panel: SlidePanelViewController?
    var currentSlideNumber: () -> Int? = { nil }
    var onImagesChanged: (() -> Void)?
    var client: TapClient? {
        didSet {
            renderer.configure(client: client)
            if client != nil { deckChanged() }
        }
    }
    private(set) var lastSummary: PresentationSummary?
    private var keys: [ThumbnailKey] = []
    private var images: [ThumbnailKey: NSImage] = [:]
    private var jobs: [ThumbnailRenderer.Job] = []
    private var fetching = false
    private var fetchAgain = false

    init(cache: ThumbnailCache, panel: SlidePanelViewController) {
        self.cache = cache
        self.panel = panel
        renderer.onImage = { [weak self] job, image, png in self?.rendered(job, image: image, png: png) }
        renderer.onPageRevision = { [weak self] _ in self?.deckChanged() }
        panel.onVisibleRangeChanged = { [weak self] in self?.reprioritize() }
    }

    func key(forSlide number: Int) -> ThumbnailKey? {
        guard number >= 1, number <= keys.count else { return nil }
        return keys[number - 1]
    }

    /// tap rendered again: fetch the summary and refresh. One fetch runs
    /// at a time; a call during a fetch runs another when it returns.
    func deckChanged() {
        guard let client else { return }
        if fetching {
            fetchAgain = true
            return
        }
        fetching = true
        Task { @MainActor [weak self] in
            defer { self?.fetching = false }
            repeat {
                self?.fetchAgain = false
                if let summary = try? await client.presentation() {
                    self?.refresh(with: summary)
                }
            } while self?.fetchAgain == true
        }
    }

    /// Reorders the renderer's work around what is visible now.
    func reprioritize() {
        guard let summary = lastSummary else { return }
        renderer.setWork(jobs, revision: summary.revision, visible: panel?.visibleNumbers ?? [], current: currentSlideNumber())
    }

    private func refresh(with summary: PresentationSummary) {
        lastSummary = summary
        // The bundled tap's version is part of the signature: an app update
        // renders every slide again rather than showing yesterday's pixels.
        let signature = summary.themeSignature + "|tap=" + (AppEnvironment.shared.bundledTapVersion ?? "")
        keys = summary.slides.map { ThumbnailKey(slideHash: $0.hash, themeSignature: signature) }
        var pending: [ThumbnailRenderer.Job] = []
        var updating: Set<Int> = []
        for (index, key) in keys.enumerated() {
            let number = index + 1
            if let image = images[key] {
                panel?.setImage(image, forSlide: number)
                continue
            }
            if let data = cache.data(for: key), let image = NSImage(data: data) {
                images[key] = image
                panel?.setImage(image, forSlide: number)
                continue
            }
            guard !key.slideHash.isEmpty else { continue }
            updating.insert(number)
            if !pending.contains(where: { $0.key == key }) {
                pending.append(ThumbnailRenderer.Job(slideNumber: number, key: key))
            }
        }
        jobs = pending
        panel?.setUpdating(updating)
        renderer.setWork(jobs, revision: summary.revision, visible: panel?.visibleNumbers ?? [], current: currentSlideNumber())
        onImagesChanged?()
    }

    private func rendered(_ job: ThumbnailRenderer.Job, image: NSImage, png: Data) {
        images[job.key] = image
        try? cache.save(png, for: job.key)
        jobs.removeAll { $0.key == job.key }
        for (index, key) in keys.enumerated() where key == job.key {
            panel?.setImage(image, forSlide: index + 1)
        }
        onImagesChanged?()
    }
}
```

- [ ] **Step 5: Wire it into the session controller**

In `DeckSessionController`:

```swift
    private(set) lazy var thumbnails = ThumbnailController(cache: AppEnvironment.shared.thumbnailCache, panel: slidePanel)
```

In `init`, after the panel delegate line:

```swift
        editorViewController.hostHiddenView(thumbnails.renderer.webView)
        thumbnails.currentSlideNumber = { [weak self] in self?.currentSlideNumber }
        thumbnails.renderer.isPaused = { [weak self] in
            guard let last = self?.sourceSync.lastEditDate else { return false }
            return Date().timeIntervalSince(last) < 0.5
        }
        thumbnails.renderer.canPaint = { [weak self] in
            guard let webView = self?.thumbnails.renderer.webView, let window = webView.window else { return false }
            return !webView.isHiddenOrHasHiddenAncestor && window.occlusionState.contains(.visible)
        }
```

In `sessionStateChanged`, after `previewViewController.load(client: newClient)`: `thumbnails.client = newClient`. In the non-running branch, `thumbnails.client = nil`, which the controller passes on as `renderer.configure(client: nil)` so the loop stops while tap is down.

In `applySlideList`, after `slidePanel.setSlides(...)`: `thumbnails.deckChanged()`.

In `AppEnvironment`: `var thumbnailCache = ThumbnailCache()`.

- [ ] **Step 6: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/ThumbnailTests/testThumbnails
make -C desktop test ONLY=TapTests/ThumbnailTests/testReopen
make -C desktop test ONLY=TapTests/ThumbnailTests/testTheCurrentThumbnailFollowsThePreview
make -C desktop test ONLY=TapTests/ThumbnailTests/testStepThroughACustomComponent
```

Expected: all pass. `testStepThroughACustomComponent` depends on tap watching a saved `.jsx` and sending `file-changed` with a slide list (P6); if the badge never reaches "6 steps", read the Tap Log for the rebuild before suspecting the app.

- [ ] **Step 7: Mutate and commit**

Mutations: in `refresh`, skip the disk cache branch (expected: `testReopen` fails, `renderCount` is 7); in `setWork`'s call inside `refresh`, pass `current: nil` (expected: `testTheCurrentThumbnailFollowsThePreview` fails on `lastRenderedSlide == 3`, since slide 1 is first in deck order and both changed); in `ThumbnailRenderer.waitForReady`, return any `lastReady` without checking its slide (expected: `testThumbnails` fails on `ready.slide == number`).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): thumbnails in the panel from the cache and the hidden renderer"
```

---

### Task 10: Slide operations as one undo step, round-tripped through `tap slide list`

**Files:**
- Create: `desktop/Tap/Slides/SlideOperations.swift`
- Create: `desktop/TapTests/Support/TapSlideList.swift`, `desktop/TapTests/Fixtures/ops.md`
- Modify: `desktop/Tap/Editor/EditorTextView.swift` (`adoptBoxes`), `desktop/Tap/Documents/DeckSessionController.swift` (`lastAppliedText`, and D2's `undoOrRedoDidChangeText` adopts pending boxes before it resends)
- Test: `desktop/TapTests/SlideOperationTests.swift`, `desktop/TapTests/ThumbnailTests.swift` (add `testDuplicatedSlidesShareOneThumbnail`)

**Interfaces:**
- Consumes: `SlideEditing`, `SlideOperation`, `SlideEditResult` (Task 1), `SlideRangeTracker.adopt` (Task 3), `EditorTextView.replaceText(in:with:actionName:)`, `TextDiff.replacement(from:to:)`, `SourceSync.sendNow()`, D2's `undoOrRedoDidChangeText` (commit 6f8d3ca: undo and redo already refresh the edited flag and `sendNow` the text; this task builds on it and adds no second resend path), `SlideList.decodeResponse` (D2), the bundled tap's `slide list <deck> --json`.
- Produces:
  - `EditorTextView.adoptBoxes(_ boxes: [SlideBox])`
  - `DeckSessionController.lastAppliedText: String?` (the text tap last answered for and the editor applied), `func whenTextIsConfirmed(_ action: @escaping () -> Void)` (runs now when the editor's text is `lastAppliedText`, otherwise sends the text and runs after tap's answer), `func perform(_ operation: SlideOperation) -> Bool` (`@discardableResult`; true when applied or queued behind a confirmation), `func moveSelectedSlides(by offset: Int) -> Bool` (-1 up, +1 down), `func moveSelectedSlides(toTop: Bool) -> Bool`, `func markdown(forSlides numbers: [Int]) -> [String]`, `func insertSlides(markdowns: [String], beforeNumber: Int?) -> Bool`, `var pendingBoxAdoption: [SlideBox]?`
  - `TapSlideList.list(text:) async throws -> SlideList`, `TapSlideList.assertOneSeparatorBetweenSlides(_:list:file:line:)`, which counts `---` lines outside tap's slide ranges
  - The rule every operation follows: it runs only on boxes tap has confirmed (the editor's text equals `lastAppliedText`); `SlideEditing.apply` gives the text; `TextDiff` gives one replacement; `replaceText` applies it as one undo step; the boxes the operation built are adopted at once, which makes every answer to an earlier send stale (Task 3), and tap is sent the text right away; after an undo or redo the matching boxes are adopted before D2's resend goes out.

- [ ] **Step 1: Write the fixture and the round-trip helper**

`desktop/TapTests/Fixtures/ops.md` (seven titled slides; slide 3 has a single-line directive; slide 4 holds a fence with a `---` line; slide 6 has notes):

~~~~markdown
---
title: Ops
---

# One

First.

---

<!-- layout: title -->

# Two

---

<!-- layout: section -->
# Three

---

# Four

```text
not a separator:
---
still slide four
```

---

# Five

---

<!--
notes:
Remember to breathe.
-->

# Six

---

# Seven
~~~~

`desktop/TapTests/Support/TapSlideList.swift`:

```swift
import Foundation
import XCTest
@testable import Tap

/// The round trip every slide operation is checked with: the buffer is
/// written to a temporary deck and the bundled `tap slide list --json`
/// parses it, so the order and the ranges come from tap, never from Swift.
enum TapSlideList {
    static func list(text: String) async throws -> SlideList {
        let folder = try Fixtures.temporaryFolder()
        let file = folder.appendingPathComponent("roundtrip.md")
        try text.write(to: file, atomically: true, encoding: .utf8)
        let executable = await MainActor.run { AppEnvironment.shared.tapExecutableURL }
        return try await withCheckedThrowingContinuation { continuation in
            DispatchQueue.global().async {
                let process = Process()
                process.executableURL = executable
                process.arguments = ["slide", "list", file.path, "--json"]
                let output = Pipe()
                process.standardOutput = output
                process.standardError = FileHandle.nullDevice
                do {
                    try process.run()
                } catch {
                    return continuation.resume(throwing: error)
                }
                // Read before waiting: a slide list longer than a pipe would block the exit.
                let data = output.fileHandleForReading.readDataToEndOfFile()
                process.waitUntilExit()
                do {
                    continuation.resume(returning: try SlideList.decodeResponse(data))
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    /// Exactly one "---" line in every gap between two slides, and the
    /// frontmatter's two before slide 1. Lines inside a slide's range are
    /// the slide's own (a fence can hold a "---"), so they are never counted.
    static func assertOneSeparatorBetweenSlides(_ text: String, list: SlideList, frontmatterSeparators: Int = 2, file: StaticString = #filePath, line: UInt = #line) {
        let lines = text.components(separatedBy: "\n")
        func separators(fromLine start: Int, toLine end: Int) -> Int {
            guard start <= end, start >= 1 else { return 0 }
            return lines[(start - 1)..<min(end, lines.count)].filter { $0.trimmingCharacters(in: .whitespaces) == "---" }.count
        }
        let slides = list.slides
        guard let first = slides.first else { return }
        XCTAssertEqual(separators(fromLine: 1, toLine: first.startLine - 1), frontmatterSeparators, "only the frontmatter's separators before slide 1", file: file, line: line)
        for (previous, next) in zip(slides, slides.dropFirst()) {
            XCTAssertEqual(separators(fromLine: previous.endLine + 1, toLine: next.startLine - 1), 1,
                           "one --- between slide \(previous.number) and slide \(next.number)", file: file, line: line)
        }
        XCTAssertEqual(separators(fromLine: slides[slides.count - 1].endLine + 1, toLine: lines.count), 0, "nothing after the last slide", file: file, line: line)
    }
}
```

- [ ] **Step 2: Write the failing tests**

`desktop/TapTests/SlideOperationTests.swift`:

```swift
import XCTest
@testable import Tap

final class SlideOperationTests: HostedTestCase {
    func openOps() async throws -> (DeckDocument, DeckSessionController) {
        let document = try await openDeck(try Fixtures.copyDeck("ops.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "the panel") { controller.slidePanel.slides.count == 7 }
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        return (document, controller)
    }

    func titles(_ controller: DeckSessionController) async throws -> [String] {
        try await TapSlideList.list(text: controller.editor.string).slides.map(\.title)
    }

    /// The buffer as tap parses it, with the separator rule checked.
    func roundTrip(_ controller: DeckSessionController) async throws -> SlideList {
        let list = try await TapSlideList.list(text: controller.editor.string)
        TapSlideList.assertOneSeparatorBetweenSlides(controller.editor.string, list: list)
        return list
    }

    func waitForConfirmation(_ controller: DeckSessionController) async throws {
        try await waitUntil(timeout: 10, "tap's answer for the current text") { controller.lastAppliedText == controller.editor.string }
    }

    func testMoveWithTheKeyboard() async throws {
        let (document, controller) = try await openOps()
        controller.editor.moveCursor(toSlide: 2)
        let offset = controller.editor.selectedRange().location - controller.editor.boxes[2].range.location
        XCTAssertGreaterThan(offset, 0, "the caret sits after the heading, not at the slide's start")

        XCTAssertTrue(controller.moveSelectedSlides(by: -1), "Cmd+Option+Up")
        XCTAssertEqual(controller.editor.boxes.map(\.slide.title), ["One", "Three", "Two", "Four", "Five", "Six", "Seven"], "the boxes follow at once")
        XCTAssertEqual(controller.editor.currentBoxIndex, 1, "slide 3 is now slide 2, and the cursor moved with it")
        XCTAssertEqual(controller.editor.selectedRange().location - controller.editor.boxes[1].range.location, offset)
        XCTAssertEqual(controller.slidePanel.selectedNumbers, [2])

        let list = try await roundTrip(controller)
        XCTAssertEqual(list.slides.map(\.title), ["One", "Three", "Two", "Four", "Five", "Six", "Seven"])
        XCTAssertEqual(list.slides[1].layout, "section")
        try await waitForConfirmation(controller)
        XCTAssertEqual(controller.editor.boxes.map(\.slide.title), list.slides.map(\.title), "tap's answer agrees with the adopted boxes")

        XCTAssertTrue(controller.moveSelectedSlides(by: -1), "slide 2 moves to the top")
        try await waitForConfirmation(controller)
        XCTAssertFalse(controller.moveSelectedSlides(by: -1), "slide 1 cannot move up")
        XCTAssertEqual(document.undoManager?.undoActionName, "Move Slide")
    }

    func testDuplicateAndDelete() async throws {
        let (document, controller) = try await openOps()
        controller.editor.moveCursor(toSlide: 2)
        XCTAssertTrue(controller.perform(.duplicate(numbers: [3])))
        XCTAssertEqual(try await roundTrip(controller).slides.map(\.title), ["One", "Two", "Three", "Three", "Four", "Five", "Six", "Seven"])
        XCTAssertEqual(controller.slidePanel.selectedNumbers, [4], "the copy is selected")
        XCTAssertEqual(controller.editor.currentBoxIndex, 3)
        try await waitForConfirmation(controller)

        controller.slidePanel.select(numbers: [6, 7], scroll: false)
        XCTAssertEqual(controller.selectedSlideNumbers, [6, 7])
        XCTAssertTrue(controller.perform(.delete(numbers: controller.selectedSlideNumbers)))
        XCTAssertEqual(try await roundTrip(controller).slides.map(\.title), ["One", "Two", "Three", "Three", "Four", "Seven"])
        XCTAssertEqual(document.undoManager?.undoActionName, "Delete 2 Slides")
        try await waitForConfirmation(controller)

        document.undoManager?.undo()
        XCTAssertEqual(controller.editor.boxes.count, 8, "one undo step brings both back, and the boxes follow at once")
        XCTAssertEqual(try await roundTrip(controller).slides.map(\.title), ["One", "Two", "Three", "Three", "Four", "Five", "Six", "Seven"])
    }

    func testSkipASlide() async throws {
        let (document, controller) = try await openOps()
        XCTAssertTrue(controller.perform(.setSkip(numbers: [4], skipped: true)))
        let text = controller.editor.string as NSString
        XCTAssertTrue(text.substring(with: controller.editor.boxes[3].range).hasPrefix("<!--\nskip: true\n-->\n\n# Four"), "skip: true in slide 4's directive comment")
        let list = try await roundTrip(controller)
        XCTAssertTrue(list.slides[3].skip, "tap reads the directive")
        XCTAssertEqual(list.slides.count, 7, "a skipped slide keeps its number")
        try await waitForConfirmation(controller)
        XCTAssertTrue(controller.editor.boxes[3].slide.skip)
        XCTAssertTrue(controller.editor.header(forBoxAt: 3).badges.contains("skipped"), "the box is marked in the editor")
        XCTAssertEqual(controller.slidePanel.item(forSlide: 4)?.thumbnailImageView.alphaValue, 0.45, "and dimmed in the sidebar")

        XCTAssertTrue(controller.perform(.setSkip(numbers: [4], skipped: false)))
        XCTAssertFalse(try await roundTrip(controller).slides[3].skip)
        XCTAssertFalse(controller.editor.string.contains("skip:"))
        _ = document
    }

    func testUndoRestoresBoxesAndResyncsTap() async throws {
        let (document, controller) = try await openOps()
        let original = controller.editor.string
        XCTAssertTrue(controller.perform(.move(numbers: [5], beforeNumber: 3)))
        let moved = controller.editor.string
        try await waitForConfirmation(controller)
        XCTAssertEqual(controller.lastAppliedText, moved)

        document.undoManager?.undo()
        XCTAssertEqual(controller.editor.string, original, "Cmd+Z restores the text")
        XCTAssertEqual(controller.editor.boxes.map(\.slide.title), ["One", "Two", "Three", "Four", "Five", "Six", "Seven"],
                       "the boxes follow the reverted text the moment the undo group ends, before tap answers")
        try await waitUntil(timeout: 10, "tap to answer for the reverted text") { controller.lastAppliedText == original }
        XCTAssertEqual(controller.sourceSync.lastSentText, original, "the reverted text is what was sent")

        document.undoManager?.redo()
        XCTAssertEqual(controller.editor.string, moved)
        XCTAssertEqual(controller.editor.boxes.map(\.slide.title), ["One", "Two", "Five", "Three", "Four", "Six", "Seven"], "redo adopts the moved boxes again")
        try await waitUntil(timeout: 10, "tap to answer for the redone text") { controller.lastAppliedText == moved }
        XCTAssertEqual(controller.sourceSync.lastSentText, moved)
    }

    /// The defect this pins: a PUT of the old text is in flight when an
    /// operation permutes the boxes. Its answer must be refused, never
    /// applied with the operation's whole-region edit replayed onto it,
    /// which would stretch and collapse the boxes.
    func testAStaleAnswerNeverDisturbsAnOperation() async throws {
        let (_, controller) = try await openOps()
        var ordersSeen: [[String]] = []
        controller.onSlideListApplied = { [weak controller] _ in
            ordersSeen.append(controller?.editor.boxes.map(\.slide.title) ?? [])
        }
        // A send of the unchanged text goes out, and the move lands while it is in flight.
        Task { await controller.sourceSync.sendNow() }
        try await Task.sleep(nanoseconds: 5_000_000)
        XCTAssertTrue(controller.perform(.move(numbers: [5], beforeNumber: 3)))
        try await waitForConfirmation(controller)
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertFalse(ordersSeen.isEmpty)
        for order in ordersSeen {
            XCTAssertEqual(order, ["One", "Two", "Five", "Three", "Four", "Six", "Seven"], "every answer applied after the move describes the moved text; the stale one was refused")
        }
        XCTAssertEqual(try await roundTrip(controller).slides.map(\.title), ["One", "Two", "Five", "Three", "Four", "Six", "Seven"])
    }

    /// Typing that tap has not parsed yet shifts the boxes, and a typed
    /// "---" sits inside the box above it. An operation waits for tap's
    /// answer and then acts on the slides as tap counts them.
    func testAnOperationWaitsForTapToConfirmTypedText() async throws {
        let (_, controller) = try await openOps()
        controller.editor.moveCursor(toSlide: 6)
        let end = NSMaxRange(controller.editor.boxes[6].range)
        controller.editor.setSelectedRange(NSRange(location: end, length: 0))
        controller.editor.insertText("\n\n---\n\n# Eight", replacementRange: NSRange(location: NSNotFound, length: 0))
        XCTAssertNotEqual(controller.editor.string, controller.lastAppliedText, "tap has not answered for the new separator yet")
        XCTAssertTrue(controller.moveSelectedSlides(by: -1), "queued until tap confirms the text")
        try await waitUntil(timeout: 10, "the deferred move to land") { controller.editor.boxes.count == 8 && controller.editor.boxes[6].slide.title == "Eight" }
        try await waitForConfirmation(controller)
        XCTAssertEqual(try await roundTrip(controller).slides.map(\.title), ["One", "Two", "Three", "Four", "Five", "Six", "Eight", "Seven"],
                       "the cursor was in the new slide 8, so slide 8 moved above slide 7, and slide 7's separator stayed where it was typed")
    }
}
```

Add to `ThumbnailTests.swift` the test held back from Task 9:

```swift
    func testDuplicatedSlidesShareOneThumbnail() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("seven-slides.md"))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForThumbnails(document, count: 7)
        let renders = controller.thumbnails.renderer.renderCount
        XCTAssertTrue(controller.perform(.duplicate(numbers: [3])))
        try await waitForBoxes(document, count: 8)
        try await waitForThumbnails(document, count: 8)
        XCTAssertEqual(controller.thumbnails.key(forSlide: 3), controller.thumbnails.key(forSlide: 4))
        XCTAssertEqual(controller.thumbnails.renderer.renderCount, renders, "a copy shares the original's image")
    }
```

- [ ] **Step 3: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/SlideOperationTests/testMoveWithTheKeyboard`
Expected: compile failure, `has no member 'lastAppliedText'`.

- [ ] **Step 4: Add `adoptBoxes` to the editor**

In `EditorTextView`, after `apply(_:sentText:sentGeneration:)`:

```swift
    /// Adopts boxes the app built by permuting tap's own ranges, right
    /// after a slide operation changed the text, so the boxes and the
    /// sidebar show the new order without waiting for tap's next answer.
    func adoptBoxes(_ newBoxes: [SlideBox]) {
        tracker.adopt(newBoxes)
        textStorage?.beginEditing()
        restyle(NSRange(location: 0, length: (string as NSString).length))
        textStorage?.endEditing()
        updateHiddenLayout()
        updateCurrentBox()
        needsDisplay = true
    }
```

- [ ] **Step 5: Write `SlideOperations.swift`**

First, in `DeckSessionController` (D2's file): add `private(set) var lastAppliedText: String?` and `var pendingBoxAdoption: [SlideBox]?`, and in `applySlideList`, right after `guard editor.apply(list, sentText: sentText, sentGeneration: generation) else { return }`, add `lastAppliedText = sentText`.

```swift
import AppKit

/// Every structural edit on the deck's slides. Each one is one undo step
/// on the buffer, cut along the ranges tap reported: the operation runs
/// only on boxes tap has confirmed; it gives the new text and the boxes
/// of that text; the text goes in as a single replacement through the
/// editor's clamp; the boxes are adopted at once, which makes every
/// answer to an earlier send stale, and tap is sent the text right away.
extension DeckSessionController {
    /// Runs `action` once the editor's text is the text tap last answered
    /// for. Boxes shifted by typing tap has not parsed yet are not tap's
    /// ranges: a "---" typed a moment ago still sits inside the box above
    /// it. If the text is not confirmed, it is sent now and `action` runs
    /// after the answer; if no answer comes in time, nothing happens but a beep.
    func whenTextIsConfirmed(_ action: @escaping () -> Void) {
        if editor.string == lastAppliedText {
            action()
            return
        }
        Task { @MainActor [weak self] in
            guard let self else { return }
            await self.sourceSync.sendNow()
            let deadline = Date().addingTimeInterval(3)
            while self.editor.string != self.lastAppliedText, Date() < deadline {
                try? await Task.sleep(nanoseconds: 20_000_000)
            }
            guard self.editor.string == self.lastAppliedText else {
                NSSound.beep()
                return
            }
            action()
        }
    }

    @discardableResult
    func perform(_ operation: SlideOperation) -> Bool {
        guard editor.string == lastAppliedText else {
            whenTextIsConfirmed { [weak self] in self?.performNow(operation) }
            return true
        }
        return performNow(operation)
    }

    @discardableResult
    private func performNow(_ operation: SlideOperation) -> Bool {
        let text = editor.string
        let boxes = editor.boxes
        guard let result = SlideEditing.apply(operation, to: text, boxes: boxes, caretOffsetInSlide: caretOffset(for: operation)) else {
            NSSound.beep()
            return false
        }
        guard let replacement = TextDiff.replacement(from: text, to: result.text), let undoManager = document?.undoManager else { return false }
        // One group holds the box adoption and the text change. AppKit's
        // event group would hold both on its own; the explicit group makes
        // that a property of this code rather than of the run loop.
        undoManager.beginUndoGrouping()
        registerBoxAdoption(undo: boxes, redo: result.boxes)
        editor.replaceText(in: replacement.range, with: replacement.replacement, actionName: SlideEditing.actionName(for: operation))
        undoManager.endUndoGrouping()
        // The cursor's own selection sync would collapse a multi-slide
        // selection to the caret's slide; the panel selects the operated
        // slides itself, last.
        isSelectingFromPanel = true
        editor.adoptBoxes(result.boxes)
        editor.setSelectedRange(NSRange(location: result.caret, length: 0))
        isSelectingFromPanel = false
        slidePanel.remapImages(sourceNumbers: result.sourceNumbers)
        slidePanel.setSlides(editor.boxes.map(\.slide))
        slidePanel.select(numbers: result.selectedNumbers, scroll: true)
        if let first = result.selectedNumbers.first, editor.boxes.indices.contains(first - 1) {
            editor.scrollRangeToVisible(editor.boxes[first - 1].range)
        }
        Task { await sourceSync.sendNow() }
        return true
    }

    /// The caret's offset inside the first slide the operation acts on,
    /// when the caret is in it, so the caret moves with the slide.
    private func caretOffset(for operation: SlideOperation) -> Int {
        let numbers: [Int]
        switch operation {
        case .move(let moved, _): numbers = moved
        case .duplicate(let selected), .delete(let selected): numbers = selected
        case .setSkip(let selected, _): numbers = selected
        case .insert: return 0
        }
        guard let first = numbers.min(), let index = editor.currentBoxIndex, editor.boxes[index].slide.number == first else { return 0 }
        return max(0, editor.selectedRange().location - editor.boxes[index].range.location)
    }

    /// Registers, in the current undo group, that after the text is undone
    /// the boxes `undo` belong to it, and after it is redone `redo` do.
    /// AppKit reverts the text through the text storage, which shifts the
    /// boxes by the inverse edit and leaves them wrong. The closure only
    /// notes which boxes to adopt; D2's `undoOrRedoDidChangeText`, which
    /// runs on the undo manager's notification once the whole group has
    /// run, adopts them and then sends the text to tap. The undo manager is
    /// looked up from the target each time, never captured.
    private func registerBoxAdoption(undo: [SlideBox], redo: [SlideBox]) {
        document?.undoManager?.registerUndo(withTarget: self) { target in
            target.registerBoxAdoption(undo: redo, redo: undo)
            target.pendingBoxAdoption = undo
        }
    }

    /// Called by `undoOrRedoDidChangeText` with the boxes the reverted or
    /// redone text has.
    func adoptPendingBoxes(_ boxes: [SlideBox]) {
        guard boxes.map(\.range).allSatisfy({ NSMaxRange($0) <= (editor.string as NSString).length }) else { return }
        editor.adoptBoxes(boxes)
        slidePanel.setSlides(editor.boxes.map(\.slide))
        syncPanelSelectionToCursor()
    }

    /// Moves the selection up (-1) or down (+1) by one slide. The selection
    /// is read once the text is confirmed, so the slides that move are the
    /// ones tap counts, not the ones a shifted box suggests.
    @discardableResult
    func moveSelectedSlides(by offset: Int) -> Bool {
        guard editor.string == lastAppliedText else {
            whenTextIsConfirmed { [weak self] in _ = self?.moveSelectedSlides(by: offset) }
            return true
        }
        let numbers = selectedSlideNumbers
        guard let first = numbers.min(), let last = numbers.max() else { return false }
        let count = editor.boxes.count
        if offset < 0 {
            guard first > 1 else { return false }
            return performNow(.move(numbers: numbers, beforeNumber: first - 1))
        }
        guard last < count else { return false }
        return performNow(.move(numbers: numbers, beforeNumber: last + 2 <= count ? last + 2 : nil))
    }

    @discardableResult
    func moveSelectedSlides(toTop: Bool) -> Bool {
        guard editor.string == lastAppliedText else {
            whenTextIsConfirmed { [weak self] in _ = self?.moveSelectedSlides(toTop: toTop) }
            return true
        }
        let numbers = selectedSlideNumbers
        guard !numbers.isEmpty else { return false }
        return performNow(.move(numbers: numbers, beforeNumber: toTop ? 1 : nil))
    }

    /// The text of each slide, from tap's ranges.
    func markdown(forSlides numbers: [Int]) -> [String] {
        let text = editor.string as NSString
        return numbers.compactMap { number in
            editor.boxes.first { $0.slide.number == number }.map { text.substring(with: $0.range) }
        }
    }

    @discardableResult
    func insertSlides(markdowns: [String], beforeNumber: Int?) -> Bool {
        perform(.insert(markdowns: markdowns, beforeNumber: beforeNumber))
    }
}
```

`moveSelectedSlides(by: +1)`: moving a block that ends at slide `last` down by one means placing it above the slide two past `last` (the slide right after the block moves in front of it), or at the end when there is none.

- [ ] **Step 6: Adopt the pending boxes where D2 already resends**

D2's `DeckSessionController.undoOrRedoDidChangeText` (commit 6f8d3ca) already refreshes the edited flag and sends the text to tap after every undo and redo. Do not add a second resend. Insert the adoption at its top, so it runs after the text is reverted and before the send begins:

```swift
    private func undoOrRedoDidChangeText() {
        // A slide operation's undo or redo leaves the boxes it belongs to
        // here; adopting them first makes the send below the next answer applied.
        if let boxes = pendingBoxAdoption {
            pendingBoxAdoption = nil
            adoptPendingBoxes(boxes)
        }
        refreshEditedState()
        Task { await sourceSync.sendNow() }
    }
```

The order matters twice over: the notification fires once the whole undo group has run, so the text is already reverted; and `adopt` makes every earlier send stale (Task 3), so the send that starts after it is the one whose answer lands.

- [ ] **Step 7: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/SlideOperationTests/testMoveWithTheKeyboard
make -C desktop test ONLY=TapTests/SlideOperationTests/testDuplicateAndDelete
make -C desktop test ONLY=TapTests/SlideOperationTests/testSkipASlide
make -C desktop test ONLY=TapTests/SlideOperationTests/testUndoRestoresBoxesAndResyncsTap
make -C desktop test ONLY=TapTests/SlideOperationTests/testAStaleAnswerNeverDisturbsAnOperation
make -C desktop test ONLY=TapTests/SlideOperationTests/testAnOperationWaitsForTapToConfirmTypedText
make -C desktop test ONLY=TapTests/ThumbnailTests/testDuplicatedSlidesShareOneThumbnail
```

Expected: all pass. If `testUndoRestoresBoxesAndResyncsTap` shows the text reverted but the boxes not adopted, the registration and the text change landed in different undo groups; check that `replaceText`'s `breakUndoCoalescing` calls did not close the explicit group early (they must not; they only end typing coalescing). Removing the explicit `beginUndoGrouping` and `endUndoGrouping` is expected to leave every test green, because AppKit's event group already holds both registrations; the explicit group is written down as a property of the code, not proven by a test, and the review should not count that mutation.

- [ ] **Step 8: Mutate and commit**

Mutations, each reverted: in `undoOrRedoDidChangeText`, make D2's `sendNow` run only on undo (add a flag set by the undo observer) (expected: `testUndoRestoresBoxesAndResyncsTap` times out after redo waiting for `lastAppliedText == moved`); run it only on redo (expected: it times out after undo); remove the `adoptPendingBoxes` call (expected: the same test fails on the boxes right after undo); in `SlideRangeTracker.adopt`, keep the edit log and the old `earliestLiveGeneration` (expected: `testAStaleAnswerNeverDisturbsAnOperation` records an order with a title twice); make `perform` skip the confirmation and call `performNow` at once (expected: `testAnOperationWaitsForTapToConfirmTypedText` fails, slide 7 and the typed slide move together); drop the `isSelectingFromPanel` guard around the caret move (expected: the next task's `testMoveSeveralSlides` fails on `[3, 4]`); in `SlideEditing.apply`'s `.setSkip` branch, pass `"false"` instead of nil for unskip (expected: `testSkipASlide` fails on `contains("skip:")`).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): move, duplicate, delete and skip slides as single undo steps on tap's ranges"
```

---

### Task 11: Drag and drop in the sidebar, within a deck and between decks

**Files:**
- Modify: `desktop/Tap/Sidebar/SlidePanelViewController.swift`, `desktop/Tap/Slides/SlideOperations.swift`
- Create: `desktop/TapTests/Support/FakeDraggingInfo.swift`
- Test: `desktop/TapTests/DragAndDropTests.swift`

**Interfaces:**
- Consumes: `SlideDragPayload` (Task 3), `SlideAccessibility.dropLabel` (Task 3), `DeckSessionController.perform`, `markdown(forSlides:)` (Task 10), `FilePaths.same` (D2). Decision 2 (the person, 2026-09-24): a drag between decks moves the slides, one undo step per document; Option copies, the macOS convention.
- Produces:
  - `SlidePanelDelegate` gains `func slidePanel(_:payloadForSlides:) -> SlideDragPayload?` and `func slidePanel(_:acceptDrop:beforeNumber:isMove:) -> Bool`
  - `SlidePanelViewController.dropDecision(proposedIndex: Int, payload: SlideDragPayload, deck: URL?, optionHeld: Bool) -> (beforeNumber: Int?, operation: NSDragOperation)` (pure, tested), `func performDrop(payload:beforeNumber:isMove:) -> Bool`, `var optionHeld: () -> Bool` (reads `NSEvent.modifierFlags`; a test replaces it)
  - `DeckSessionController.dragPayload(forSlides:) -> SlideDragPayload?`, `func dropSlides(payload:beforeNumber:isMove:) -> Bool`, `static func document(forDeckPath:) -> DeckDocument?`
  - `FakeDraggingInfo: NSObject, NSDraggingInfo` (test support): a real pasteboard, a location, a source and a window, so the real `validateDrop`, `acceptDrop`, `draggingUpdated` and `performDragOperation` run in hosted tests.
  - The pasteboard type `io.geocod.tap.slides`; the drag image is the first slide's thumbnail with a count badge when more than one slide moves; a drop into another deck moves the slides (the source deletes them as its own undo step), or copies them when Option is held.

- [ ] **Step 1: Write the fake dragging info**

`desktop/TapTests/Support/FakeDraggingInfo.swift`:

```swift
import AppKit

/// What AppKit hands a drop target, built by hand: `NSDraggingInfo` is a
/// protocol, so the real `validateDrop`, `acceptDrop`, `draggingUpdated`
/// and `performDragOperation` can run against it in a hosted test.
final class FakeDraggingInfo: NSObject, NSDraggingInfo {
    let pasteboard: NSPasteboard
    var location: NSPoint
    let source: Any?
    let window: NSWindow?

    init(pasteboard: NSPasteboard, location: NSPoint, source: Any?, window: NSWindow?) {
        self.pasteboard = pasteboard
        self.location = location
        self.source = source
        self.window = window
    }

    var draggingDestinationWindow: NSWindow? { window }
    var draggingSourceOperationMask: NSDragOperation { [.move, .copy] }
    var draggingLocation: NSPoint { location }
    var draggedImageLocation: NSPoint { location }
    var draggedImage: NSImage? { nil }
    var draggingPasteboard: NSPasteboard { pasteboard }
    var draggingSource: Any? { source }
    var draggingSequenceNumber: Int { 1 }
    var draggingFormation: NSDraggingFormation = .default
    var animatesToDestination = false
    var numberOfValidItemsForDrop = 1
    var springLoadingHighlight: NSSpringLoadingHighlight { .none }
    func slideDraggedImage(to screenPoint: NSPoint) {}
    func enumerateDraggingItems(options: NSDraggingItemEnumerationOptions, for view: NSView?, classes: [AnyClass],
                                searchOptions: [NSPasteboard.ReadingOptionKey: Any], using block: @escaping (NSDraggingItem, Int, UnsafeMutablePointer<ObjCBool>) -> Void) {}
    func resetSpringLoading() {}
}
```

If the compiler names a member of `NSDraggingInfo` this list lacks, add it with the simplest value; do not weaken the tests around it.

- [ ] **Step 2: Write the failing tests**

`desktop/TapTests/DragAndDropTests.swift`:

```swift
import AppKit
import XCTest
@testable import Tap

final class DragAndDropTests: HostedTestCase {
    func openOps() async throws -> (DeckDocument, DeckSessionController) {
        let document = try await openDeck(try Fixtures.copyDeck("ops.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "the panel") { controller.slidePanel.slides.count == 7 }
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        return (document, controller)
    }

    func roundTrip(_ controller: DeckSessionController) async throws -> [String] {
        let list = try await TapSlideList.list(text: controller.editor.string)
        TapSlideList.assertOneSeparatorBetweenSlides(controller.editor.string, list: list)
        return list.slides.map(\.title)
    }

    /// A pasteboard written by the panel's real writer for `numbers`, as a drag of them would write it.
    func dragPasteboard(from panel: SlidePanelViewController, numbers: [Int]) throws -> NSPasteboard {
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.drag.\(UUID().uuidString)"))
        pasteboard.clearContents()
        panel.select(numbers: numbers, scroll: false)
        let writer = try XCTUnwrap(panel.collectionView(panel.collectionView, pasteboardWriterForItemAt: IndexPath(item: numbers[0] - 1, section: 0)),
                                   "the real pasteboard writer")
        pasteboard.writeObjects([writer])
        return pasteboard
    }

    /// The real validate and accept path of the panel, fed a fake dragging info.
    func drop(_ pasteboard: NSPasteboard, into panel: SlidePanelViewController, beforeIndex index: Int, source: Any?, window: NSWindow?) -> (NSDragOperation, Bool) {
        let info = FakeDraggingInfo(pasteboard: pasteboard, location: .zero, source: source, window: window)
        var proposed = NSIndexPath(forItem: index, inSection: 0)
        var operation = NSCollectionView.DropOperation.on
        let allowed = panel.collectionView(panel.collectionView, validateDrop: info, proposedIndexPath: &proposed, dropOperation: &operation)
        XCTAssertEqual(operation, .before, "a drop always lands between thumbnails")
        guard allowed != [] else { return (allowed, false) }
        let accepted = panel.collectionView(panel.collectionView, acceptDrop: info, indexPath: IndexPath(item: index, section: 0), dropOperation: .before)
        return (allowed, accepted)
    }

    func testMoveOneSlideByDraggingInTheSidebar() async throws {
        let (document, controller) = try await openOps()
        let panel = controller.slidePanel
        let window = document.windowControllers.first?.window
        let payload = try XCTUnwrap(controller.dragPayload(forSlides: [5]))
        XCTAssertEqual(payload.markdowns, ["# Five"])
        XCTAssertTrue(payload.comesFrom(deck: try XCTUnwrap(document.fileURL)))
        XCTAssertEqual(panel.dropDecision(proposedIndex: 2, payload: payload, deck: document.fileURL, optionHeld: false).beforeNumber, 3)
        XCTAssertEqual(panel.dropDecision(proposedIndex: 4, payload: payload, deck: document.fileURL, optionHeld: false).operation, [],
                       "dropping a slide onto its own place is refused")
        XCTAssertEqual(panel.dropDecision(proposedIndex: 7, payload: payload, deck: document.fileURL, optionHeld: false).beforeNumber, nil, "after the last slide")

        let pasteboard = try dragPasteboard(from: panel, numbers: [5])
        let (operation, accepted) = drop(pasteboard, into: panel, beforeIndex: 2, source: panel.collectionView, window: window)
        XCTAssertEqual(operation, .move)
        XCTAssertTrue(accepted)
        XCTAssertEqual(try await roundTrip(controller), ["One", "Two", "Five", "Three", "Four", "Six", "Seven"], "tap parses the moved text in the new order")
        XCTAssertEqual(panel.selectedNumbers, [3])

        document.undoManager?.undo()
        XCTAssertEqual(try await roundTrip(controller), ["One", "Two", "Three", "Four", "Five", "Six", "Seven"], "Cmd+Z restores the old order")
    }

    func testMoveSeveralSlides() async throws {
        let (document, controller) = try await openOps()
        let panel = controller.slidePanel
        let pasteboard = try dragPasteboard(from: panel, numbers: [5, 6])
        XCTAssertEqual(try XCTUnwrap(SlideDragPayload(data: try XCTUnwrap(pasteboard.data(forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType))))).slideNumbers, [5, 6],
                       "dragging one selected thumbnail drags the whole selection")
        let (_, accepted) = drop(pasteboard, into: panel, beforeIndex: 2, source: panel.collectionView, window: document.windowControllers.first?.window)
        XCTAssertTrue(accepted)
        XCTAssertEqual(try await roundTrip(controller), ["One", "Two", "Five", "Six", "Three", "Four", "Seven"])
        XCTAssertEqual(document.undoManager?.undoActionName, "Move 2 Slides")
        XCTAssertEqual(panel.selectedNumbers, [3, 4], "the moved slides stay selected, in their order")
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }

        XCTAssertTrue(controller.moveSelectedSlides(by: -1), "a keyboard move acts on the whole selection")
        XCTAssertEqual(try await roundTrip(controller), ["One", "Five", "Six", "Two", "Three", "Four", "Seven"])
        XCTAssertEqual(panel.selectedNumbers, [2, 3])
        document.undoManager?.undo()
        document.undoManager?.undo()
        XCTAssertEqual(try await roundTrip(controller), ["One", "Two", "Three", "Four", "Five", "Six", "Seven"], "one undo step each")
    }

    func testFrontmatterNeverMoves() async throws {
        let (document, controller) = try await openOps()
        let panel = controller.slidePanel
        let hidden = controller.editor.hiddenLength
        let pasteboard = try dragPasteboard(from: panel, numbers: [4])
        let (_, accepted) = drop(pasteboard, into: panel, beforeIndex: 0, source: panel.collectionView, window: document.windowControllers.first?.window)
        XCTAssertTrue(accepted)
        XCTAssertTrue(controller.editor.string.hasPrefix("---\ntitle: Ops\n---\n\n# Four"), "inserted after the frontmatter, never above it")
        XCTAssertEqual(controller.editor.hiddenLength, hidden)
        XCTAssertEqual(try await roundTrip(controller), ["Four", "One", "Two", "Three", "Five", "Six", "Seven"])
    }

    func testDragSlidesToAnotherDeck() async throws {
        let (source, sourceController) = try await openOps()
        let target = try await openDeck(try Fixtures.copyAppFixture())
        try await waitForBoxes(target, count: 4)
        let targetController = try XCTUnwrap(target.sessionController)
        try await waitUntil(timeout: 5, "the target's answer") { targetController.lastAppliedText == targetController.editor.string }
        let sourcePanel = sourceController.slidePanel
        let targetPanel = targetController.slidePanel

        let payload = try XCTUnwrap(sourceController.dragPayload(forSlides: [5, 6]))
        XCTAssertEqual(targetPanel.dropDecision(proposedIndex: 4, payload: payload, deck: target.fileURL, optionHeld: false).operation, .move,
                       "into another deck, a drop moves")
        XCTAssertEqual(targetPanel.dropDecision(proposedIndex: 4, payload: payload, deck: target.fileURL, optionHeld: true).operation, .copy,
                       "with Option held, it copies")

        let pasteboard = try dragPasteboard(from: sourcePanel, numbers: [5, 6])
        let (operation, accepted) = drop(pasteboard, into: targetPanel, beforeIndex: 4, source: sourcePanel.collectionView, window: target.windowControllers.first?.window)
        XCTAssertEqual(operation, .move)
        XCTAssertTrue(accepted)
        XCTAssertEqual(try await roundTrip(targetController), ["App Mode Fixture", "Live Code", "Fragments", "Counter", "Five", "Six"])
        XCTAssertEqual(try await roundTrip(sourceController), ["One", "Two", "Three", "Four", "Seven"], "the source loses them")
        XCTAssertEqual(target.undoManager?.undoActionName, "Insert 2 Slides", "one undo step in the target")
        XCTAssertEqual(source.undoManager?.undoActionName, "Delete 2 Slides", "and one in the source")
        source.undoManager?.undo()
        XCTAssertEqual(try await roundTrip(sourceController).count, 7)

        targetPanel.optionHeld = { true }
        let copied = try dragPasteboard(from: sourcePanel, numbers: [1])
        let (copyOperation, copyAccepted) = drop(copied, into: targetPanel, beforeIndex: 0, source: sourcePanel.collectionView, window: target.windowControllers.first?.window)
        XCTAssertEqual(copyOperation, .copy)
        XCTAssertTrue(copyAccepted)
        XCTAssertEqual(try await roundTrip(targetController), ["One", "App Mode Fixture", "Live Code", "Fragments", "Counter", "Five", "Six"])
        XCTAssertEqual(try await roundTrip(sourceController).count, 7, "a copy leaves the source alone")
    }
}
```

- [ ] **Step 3: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/DragAndDropTests/testMoveOneSlideByDraggingInTheSidebar`
Expected: compile failure, `has no member 'dragPayload'`.

- [ ] **Step 4: Drag and drop in the panel**

In `SlidePanelDelegate`, add:

```swift
    func slidePanel(_ panel: SlidePanelViewController, payloadForSlides numbers: [Int]) -> SlideDragPayload?
    func slidePanel(_ panel: SlidePanelViewController, acceptDrop payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool
```

In `SlidePanelViewController`, add a stored `var deckURL: URL?` (the session controller sets it from the document's `fileURL`, and again on `deckMoved`) and `var optionHeld: () -> Bool = { NSEvent.modifierFlags.contains(.option) }`, register the type in `loadView`:

```swift
        collectionView.registerForDraggedTypes([NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType)])
        collectionView.setDraggingSourceOperationMask([.move, .copy], forLocal: true)
        collectionView.setDraggingSourceOperationMask([.move, .copy], forLocal: false)
```

and the drag methods:

```swift
    // MARK: Drag and drop

    /// Where a drop lands and what it does. `proposedIndex` is the gap
    /// before that item (`count` means after the last). A slide dropped
    /// onto its own place, inside the dragged block, is refused. A drop
    /// moves, within the deck and from another deck alike; with Option
    /// held it copies, as the Finder does.
    func dropDecision(proposedIndex: Int, payload: SlideDragPayload, deck: URL?, optionHeld: Bool) -> (beforeNumber: Int?, operation: NSDragOperation) {
        let beforeNumber: Int? = proposedIndex < slides.count ? proposedIndex + 1 : nil
        let sameDeck = deck.map { payload.comesFrom(deck: $0) } ?? false
        if sameDeck {
            let block = payload.slideNumbers.sorted()
            let before = beforeNumber ?? (slides.count + 1)
            if let first = block.first, let last = block.last, before >= first, before <= last + 1 {
                return (beforeNumber, [])
            }
            return (beforeNumber, .move)
        }
        return (beforeNumber, optionHeld ? .copy : .move)
    }

    func performDrop(payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool {
        delegate?.slidePanel(self, acceptDrop: payload, beforeNumber: beforeNumber, isMove: isMove) ?? false
    }

    private var lastAnnouncedDrop: Int??

    func collectionView(_ collectionView: NSCollectionView, canDragItemsAt indexPaths: Set<IndexPath>, with event: NSEvent) -> Bool {
        true
    }

    func collectionView(_ collectionView: NSCollectionView, pasteboardWriterForItemAt indexPath: IndexPath) -> NSPasteboardWriting? {
        // Every dragged item writes the whole selection's payload; the drop reads the first.
        let numbers = selectedNumbers.contains(indexPath.item + 1) ? selectedNumbers : [indexPath.item + 1]
        guard let payload = delegate?.slidePanel(self, payloadForSlides: numbers), let data = try? payload.data() else { return nil }
        let item = NSPasteboardItem()
        item.setData(data, forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType))
        return item
    }

    func collectionView(_ collectionView: NSCollectionView, draggingSession session: NSDraggingSession, willBeginAt screenPoint: NSPoint,
                        forItemsAt indexPaths: Set<IndexPath>) {
        let numbers = selectedNumbers.isEmpty ? indexPaths.map { $0.item + 1 } : selectedNumbers
        guard let first = numbers.min(), let image = images[first] else { return }
        let count = numbers.count
        session.enumerateDraggingItems(options: [], for: collectionView, classes: [NSPasteboardItem.self], searchOptions: [:]) { item, index, stop in
            item.draggingFrame = NSRect(origin: item.draggingFrame.origin, size: NSSize(width: 164, height: 92))
            item.imageComponentsProvider = {
                let picture = NSDraggingImageComponent(key: .icon)
                picture.contents = image
                picture.frame = NSRect(x: 0, y: 0, width: 164, height: 92)
                guard count > 1 else { return [picture] }
                let badge = NSDraggingImageComponent(key: .label)
                badge.contents = Self.countBadge("\(count)")
                badge.frame = NSRect(x: 164 - 13, y: 92 - 13, width: 22, height: 22)
                return [picture, badge]
            }
            // Only the first item carries an image; the others ride along invisibly.
            if index > 0 { item.imageComponentsProvider = { [] } }
        }
    }

    static func countBadge(_ text: String) -> NSImage {
        NSImage(size: NSSize(width: 22, height: 22), flipped: false) { rect in
            NSColor.systemRed.setFill()
            NSBezierPath(ovalIn: rect).fill()
            let attributes: [NSAttributedString.Key: Any] = [.font: NSFont.systemFont(ofSize: 12, weight: .bold), .foregroundColor: NSColor.white]
            let size = (text as NSString).size(withAttributes: attributes)
            (text as NSString).draw(at: NSPoint(x: (rect.width - size.width) / 2, y: (rect.height - size.height) / 2), withAttributes: attributes)
            return true
        }
    }

    private func payload(on draggingInfo: NSDraggingInfo) -> SlideDragPayload? {
        guard let data = draggingInfo.draggingPasteboard.data(forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType)) else { return nil }
        return SlideDragPayload(data: data)
    }

    func collectionView(_ collectionView: NSCollectionView, validateDrop draggingInfo: NSDraggingInfo,
                        proposedIndexPath: AutoreleasingUnsafeMutablePointer<NSIndexPath>,
                        dropOperation: UnsafeMutablePointer<NSCollectionView.DropOperation>) -> NSDragOperation {
        guard let payload = payload(on: draggingInfo) else { return [] }
        dropOperation.pointee = .before
        let decision = dropDecision(proposedIndex: proposedIndexPath.pointee.item, payload: payload, deck: deckURL, optionHeld: optionHeld())
        if decision.operation != [], lastAnnouncedDrop != .some(decision.beforeNumber) {
            lastAnnouncedDrop = .some(decision.beforeNumber)
            NSAccessibility.post(element: collectionView, notification: .announcementRequested,
                                 userInfo: [.announcement: SlideAccessibility.dropLabel(beforeNumber: decision.beforeNumber, count: payload.slideNumbers.count),
                                            .priority: NSAccessibilityPriorityLevel.medium.rawValue])
        }
        return decision.operation
    }

    func collectionView(_ collectionView: NSCollectionView, acceptDrop draggingInfo: NSDraggingInfo, indexPath: IndexPath,
                        dropOperation: NSCollectionView.DropOperation) -> Bool {
        guard let payload = payload(on: draggingInfo) else { return false }
        lastAnnouncedDrop = nil
        let decision = dropDecision(proposedIndex: indexPath.item, payload: payload, deck: deckURL, optionHeld: optionHeld())
        guard decision.operation != [] else { return false }
        return performDrop(payload: payload, beforeNumber: decision.beforeNumber, isMove: decision.operation == .move)
    }
```

- [ ] **Step 5: The session controller's side**

In `SlideOperations.swift`:

```swift
extension DeckSessionController {
    func dragPayload(forSlides numbers: [Int]) -> SlideDragPayload? {
        guard let deck = document?.fileURL, !numbers.isEmpty else { return nil }
        let sorted = numbers.sorted()
        return SlideDragPayload(deckPath: deck.path, slideNumbers: sorted, markdowns: markdown(forSlides: sorted))
    }

    /// A drop of slides: a move within this deck; from another deck, an
    /// insert here and, unless the drop is a copy, a delete there. Each
    /// deck registers its own undo step (decision 2).
    @discardableResult
    func dropSlides(payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool {
        if let deck = document?.fileURL, payload.comesFrom(deck: deck) {
            return perform(.move(numbers: payload.slideNumbers, beforeNumber: beforeNumber))
        }
        guard perform(.insert(markdowns: payload.markdowns, beforeNumber: beforeNumber)) else { return false }
        if isMove, let source = Self.document(forDeckPath: payload.deckPath)?.sessionController, source !== self {
            source.perform(.delete(numbers: payload.slideNumbers))
        }
        return true
    }

    static func document(forDeckPath path: String) -> DeckDocument? {
        let url = URL(fileURLWithPath: path)
        return NSDocumentController.shared.documents.compactMap { $0 as? DeckDocument }
            .first { $0.fileURL.map { FilePaths.same($0, url) } ?? false }
    }
}
```

In the `SlidePanelDelegate` conformance:

```swift
    func slidePanel(_ panel: SlidePanelViewController, payloadForSlides numbers: [Int]) -> SlideDragPayload? {
        dragPayload(forSlides: numbers)
    }

    func slidePanel(_ panel: SlidePanelViewController, acceptDrop payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool {
        dropSlides(payload: payload, beforeNumber: beforeNumber, isMove: isMove)
    }
```

Set `slidePanel.deckURL = document.fileURL` in `init` and in `deckMoved(to:)`.

- [ ] **Step 6: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/DragAndDropTests/testMoveOneSlideByDraggingInTheSidebar
make -C desktop test ONLY=TapTests/DragAndDropTests/testMoveSeveralSlides
make -C desktop test ONLY=TapTests/DragAndDropTests/testFrontmatterNeverMoves
make -C desktop test ONLY=TapTests/DragAndDropTests/testDragSlidesToAnotherDeck
```

Expected: all pass. `testDragSlidesToAnotherDeck` opens two decks in one window as tabs; `openDeck` orders each front, which is fine.

- [ ] **Step 7: Mutate and commit**

Mutations, each reverted: in `dropDecision`, return `.move` for a drop inside the block (expected: the first test fails on `[]`); return `.copy` by default for another deck (expected: `testDragSlidesToAnotherDeck` fails on `.move` and on the source's titles); in `dropSlides`, skip the source delete (expected: the same test fails on the source's titles); make `pasteboardWriterForItemAt` return nil (expected: every test fails in `dragPasteboard`); make `acceptDrop` return false without performing (expected: every test fails on `accepted`).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): drag slides in the sidebar, within a deck and into another"
```

---

### Task 12: Dragging a box header in the editor

**Files:**
- Modify: `desktop/Tap/Editor/EditorTextView.swift`, `desktop/Tap/Documents/DeckSessionController.swift`
- Test: `desktop/TapTests/EditorHeaderDragTests.swift`; `desktop/TapTests/EditorTextViewTests.swift` (D2's `Recorder` conforms to `EditorTextViewDelegate` and must keep compiling: the new delegate methods get default implementations)

**Interfaces:**
- Consumes: `SlideDragPayload`, `SlideAccessibility.dropLabel` (Task 3), `dragPayload(forSlides:)`, `dropSlides(payload:beforeNumber:isMove:)` (Task 11), `EditorTextView.drawBoxes` (D2).
- Produces:
  - `EditorTextView.boxRect(forBoxAt index: Int) -> NSRect?` (nil when the box is outside the viewport), `headerRect(forBoxAt:) -> NSRect?`, `boxIndex(forHeaderAt point: NSPoint) -> Int?`, `dropBoundary(at point: NSPoint) -> Int?` (the `beforeNumber` for a point; nil means the end), `private(set) var dropIndicatorBeforeNumber: Int?`, `func performSlideDrop(payload:beforeNumber:isMove:) -> Bool`, `var optionHeld: () -> Bool`, `var headerDragStarter: (SlideDragPayload, NSRect, NSEvent) -> Void` (a seam defaulting to `beginDraggingSession`; a test records the call), `static let dragThreshold: CGFloat = 4`
  - A mouse down on a header waits for the pointer to move `dragThreshold` points before it starts a drag; a mouse up before that is a click, which places the caret as any click does. Dropping slides from the same deck or another deck moves them; Option copies (decision 2). `draggingEnded` calls `super`, and `sourceOperationMaskFor` changes nothing for `NSTextView`'s own text drags.
  - `EditorTextViewDelegate` gains `func editor(_:payloadForHeaderDragOfBoxAt:) -> SlideDragPayload?` and `func editor(_:dropSlides:beforeNumber:isMove:) -> Bool`, both with default implementations in a protocol extension (nil and false), so D2's `Recorder` in `EditorTextViewTests` compiles unchanged
  - The drop indicator: a 2 pt accent line across the box width at the boundary with a 6 pt ring at its left end, and an accessibility element labelled with `SlideAccessibility.dropLabel`.

- [ ] **Step 1: Write the failing test**

`desktop/TapTests/EditorHeaderDragTests.swift`:

```swift
import XCTest
@testable import Tap

final class EditorHeaderDragTests: HostedTestCase {
    func openOpsLaidOut() async throws -> (DeckDocument, DeckSessionController, EditorTextView) {
        let document = try await openDeck(try Fixtures.copyDeck("ops.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        let editor = controller.editor
        editor.layoutSubtreeIfNeeded()
        editor.textLayoutManager?.ensureLayout(for: editor.textLayoutManager!.documentRange)
        editor.needsDisplay = true
        editor.displayIfNeeded()
        return (document, controller, editor)
    }

    func mouseEvent(_ type: NSEvent.EventType, at point: NSPoint, in editor: EditorTextView) throws -> NSEvent {
        let window = try XCTUnwrap(editor.window)
        return try XCTUnwrap(NSEvent.mouseEvent(with: type, location: editor.convert(point, to: nil), modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                                windowNumber: window.windowNumber, context: nil, eventNumber: 0, clickCount: 1, pressure: 1))
    }

    func testMoveByDraggingABoxHeaderInTheEditor() async throws {
        let (document, controller, editor) = try await openOpsLaidOut()
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        let header5 = try XCTUnwrap(editor.headerRect(forBoxAt: 4), "box 5 is on screen with a header")
        XCTAssertEqual(editor.boxIndex(forHeaderAt: NSPoint(x: header5.midX, y: header5.midY)), 4)
        XCTAssertNil(editor.boxIndex(forHeaderAt: NSPoint(x: header5.midX, y: header5.maxY + EditorTextView.lineHeight)), "the body of a box is not its header")

        // A mouse down on the header followed by a drag past the threshold starts a slide drag.
        var started: [SlideDragPayload] = []
        editor.headerDragStarter = { payload, _, _ in started.append(payload) }
        let down = NSPoint(x: header5.midX, y: header5.midY)
        NSApp.postEvent(try mouseEvent(.leftMouseDragged, at: NSPoint(x: down.x + 12, y: down.y), in: editor), atStart: false)
        editor.mouseDown(with: try mouseEvent(.leftMouseDown, at: down, in: editor))
        XCTAssertEqual(started.map(\.slideNumbers), [[5]], "the drag carries slide 5")

        // A mouse down and up on a header with no drag is a click, which places the caret in that slide.
        editor.setSelectedRange(NSRange(location: 0, length: 0))
        NSApp.postEvent(try mouseEvent(.leftMouseUp, at: down, in: editor), atStart: false)
        editor.mouseDown(with: try mouseEvent(.leftMouseDown, at: down, in: editor))
        XCTAssertEqual(started.count, 1, "no second drag")
        XCTAssertEqual(editor.currentBoxIndex, 4, "the click put the caret in slide 5")

        // The drop, through the real drag destination methods.
        let box3 = try XCTUnwrap(editor.boxRect(forBoxAt: 2))
        let above3 = NSPoint(x: box3.midX, y: box3.minY + 4)
        XCTAssertEqual(editor.dropBoundary(at: above3), 3, "the upper half of box 3 means above slide 3")
        XCTAssertEqual(editor.dropBoundary(at: NSPoint(x: box3.midX, y: box3.maxY - 4)), 4, "the lower half means below it")
        XCTAssertEqual(editor.dropBoundary(at: NSPoint(x: box3.midX, y: -100)), 1, "above everything is above slide 1")

        let payload = try XCTUnwrap(controller.dragPayload(forSlides: [5]))
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.headerdrag.\(UUID().uuidString)"))
        pasteboard.clearContents()
        let item = NSPasteboardItem()
        item.setData(try payload.data(), forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType))
        pasteboard.writeObjects([item])
        let info = FakeDraggingInfo(pasteboard: pasteboard, location: editor.convert(above3, to: nil), source: editor, window: window)
        XCTAssertEqual(editor.draggingUpdated(info), .move)
        XCTAssertEqual(editor.dropIndicatorBeforeNumber, 3, "the indicator follows the pointer")
        XCTAssertTrue(editor.performDragOperation(info))
        editor.draggingEnded(info)
        XCTAssertNil(editor.dropIndicatorBeforeNumber, "the indicator is gone after the drop")
        let list = try await TapSlideList.list(text: editor.string)
        XCTAssertEqual(list.slides.map(\.title), ["One", "Two", "Five", "Three", "Four", "Six", "Seven"], "the same result as dragging in the sidebar")
        TapSlideList.assertOneSeparatorBetweenSlides(editor.string, list: list)

        // From another deck, a drop moves the slides there too, and Option copies.
        editor.optionHeld = { true }
        let other = SlideDragPayload(deckPath: "/tmp/elsewhere/talk.md", slideNumbers: [1], markdowns: ["# Elsewhere"])
        let otherPasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.headerdrag.\(UUID().uuidString)"))
        otherPasteboard.clearContents()
        let otherItem = NSPasteboardItem()
        otherItem.setData(try other.data(), forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType))
        otherPasteboard.writeObjects([otherItem])
        let otherInfo = FakeDraggingInfo(pasteboard: otherPasteboard, location: editor.convert(NSPoint(x: box3.midX, y: -100), to: nil), source: nil, window: window)
        XCTAssertEqual(editor.draggingUpdated(otherInfo), .copy, "Option held: a copy")
        editor.optionHeld = { false }
        XCTAssertEqual(editor.draggingUpdated(otherInfo), .move, "otherwise a move, as in the sidebar")
        editor.draggingExited(otherInfo)
        XCTAssertNil(editor.dropIndicatorBeforeNumber)
    }

    func testTheDropIndicatorHasALabel() async throws {
        let (_, _, editor) = try await openOpsLaidOut()
        let box2 = try XCTUnwrap(editor.boxRect(forBoxAt: 1))
        editor.updateDropIndicator(at: NSPoint(x: box2.midX, y: box2.minY + 2), count: 2)
        XCTAssertEqual(editor.dropIndicatorBeforeNumber, 2)
        let element = try XCTUnwrap(editor.accessibilityChildren()?.compactMap { $0 as? NSAccessibilityElement }.first { $0.accessibilityIdentifier() == "drop-indicator" })
        XCTAssertEqual(element.accessibilityLabel(), "Drop 2 slides above slide 2")
        editor.clearDropIndicator()
        XCTAssertNil(editor.dropIndicatorBeforeNumber)
    }
}
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/EditorHeaderDragTests/testMoveByDraggingABoxHeaderInTheEditor`
Expected: compile failure, `has no member 'headerRect'`.

- [ ] **Step 3: Refactor box geometry out of `drawBoxes`**

In `EditorTextView`, extract the frame computation that `drawBoxes` does per box into:

```swift
    /// The rectangle of a box whose start or end is inside the viewport,
    /// in view coordinates; nil for a box that is entirely off screen. A
    /// box that starts above the viewport has no trustworthy top, so the
    /// rectangle is extended far above it, as drawing does.
    func boxRect(forBoxAt index: Int) -> NSRect? {
        guard boxes.indices.contains(index), let layoutManager = textLayoutManager,
              let contentManager = layoutManager.textContentManager,
              let viewport = layoutManager.textViewportLayoutController.viewportRange else { return nil }
        let documentStart = contentManager.documentRange.location
        let viewportStart = contentManager.offset(from: documentStart, to: viewport.location)
        let viewportEnd = contentManager.offset(from: documentStart, to: viewport.endLocation)
        let box = boxes[index]
        guard box.end >= viewportStart, box.range.location <= viewportEnd else { return nil }
        let origin = textContainerOrigin
        let left = origin.x - Self.boxOutset
        let right = bounds.width - origin.x + Self.boxOutset
        let errorSpace = CGFloat(box.slide.errors.count) * Self.errorLineHeight
        var top = visibleRect.minY - 40
        var bottom = visibleRect.maxY + 40
        if box.range.location >= viewportStart,
           let location = contentManager.location(documentStart, offsetBy: box.range.location),
           let fragment = layoutManager.textLayoutFragment(for: location),
           let line = fragment.textLineFragments.first {
            top = fragment.layoutFragmentFrame.minY + line.typographicBounds.minY + origin.y - Self.headerHeight - Self.boxPaddingTop - errorSpace
        }
        if box.end <= viewportEnd,
           let location = contentManager.location(documentStart, offsetBy: max(box.range.location, box.end - 1)),
           let fragment = layoutManager.textLayoutFragment(for: location),
           let line = fragment.textLineFragments.last {
            bottom = fragment.layoutFragmentFrame.minY + line.typographicBounds.maxY + origin.y + Self.boxPaddingBottom
        }
        return NSRect(x: left, y: top, width: right - left, height: bottom - top)
    }

    func headerRect(forBoxAt index: Int) -> NSRect? {
        boxRect(forBoxAt: index).map { NSRect(x: $0.minX, y: $0.minY, width: $0.width, height: Self.headerHeight) }
    }

    /// The box whose header is under `point`, in view coordinates.
    func boxIndex(forHeaderAt point: NSPoint) -> Int? {
        for index in visibleBoxIndices() where headerRect(forBoxAt: index)?.contains(point) == true {
            return index
        }
        return nil
    }

    /// The slide number a drop at `point` lands above: the box under the
    /// point when the point is in its upper half, the next one otherwise;
    /// 1 above every box; nil below the last.
    func dropBoundary(at point: NSPoint) -> Int? {
        guard !boxes.isEmpty else { return nil }
        for index in visibleBoxIndices() {
            guard let rect = boxRect(forBoxAt: index) else { continue }
            if point.y < rect.minY { return boxes[index].slide.number }
            if point.y <= rect.maxY {
                return point.y < rect.midY ? boxes[index].slide.number : (index + 1 < boxes.count ? boxes[index + 1].slide.number : nil)
            }
        }
        if let first = visibleBoxIndices().first, let rect = boxRect(forBoxAt: first), point.y < rect.minY { return boxes[first].slide.number }
        return nil
    }

    /// The indices of the boxes touching the viewport, in order. This is
    /// the same walk `drawBoxes` makes.
    private func visibleBoxIndices() -> [Int] {
        guard let layoutManager = textLayoutManager, let contentManager = layoutManager.textContentManager,
              let viewport = layoutManager.textViewportLayoutController.viewportRange, !boxes.isEmpty else { return [] }
        let documentStart = contentManager.documentRange.location
        let viewportStart = contentManager.offset(from: documentStart, to: viewport.location)
        let viewportEnd = contentManager.offset(from: documentStart, to: viewport.endLocation)
        var low = 0
        var high = boxes.count
        while low < high {
            let middle = (low + high) / 2
            if boxes[middle].end < viewportStart { low = middle + 1 } else { high = middle }
        }
        var indices: [Int] = []
        var index = low
        while index < boxes.count, boxes[index].range.location <= viewportEnd {
            indices.append(index)
            index += 1
        }
        return indices
    }
```

Then make `drawBoxes(in:)` iterate `visibleBoxIndices()` and call `boxRect(forBoxAt:)` for each, and after the boxes, draw the indicator:

```swift
        if let before = dropIndicatorBeforeNumber, let y = dropIndicatorY(beforeNumber: before) {
            NSColor.controlAccentColor.setFill()
            NSRect(x: left + 8, y: y - 1, width: right - left - 8, height: 2).fill()
            let ring = NSBezierPath(ovalIn: NSRect(x: left + 1, y: y - 4, width: 8, height: 8))
            NSColor.controlAccentColor.setStroke()
            ring.lineWidth = 2
            ring.stroke()
        }
```

with

```swift
    /// The y of the boundary above slide `beforeNumber`: the top of that
    /// box's rectangle less half the gap, or below the last box for nil.
    private func dropIndicatorY(beforeNumber: Int?) -> CGFloat? {
        if let beforeNumber, let index = boxes.firstIndex(where: { $0.slide.number == beforeNumber }), let rect = boxRect(forBoxAt: index) {
            return rect.minY - 6
        }
        if beforeNumber == nil, let last = boxes.indices.last, let rect = boxRect(forBoxAt: last) {
            return rect.maxY + 6
        }
        return nil
    }
```

- [ ] **Step 4: The drag source and the drop target**

Add to the delegate protocol:

```swift
    func editor(_ editor: EditorTextView, payloadForHeaderDragOfBoxAt index: Int) -> SlideDragPayload?
    func editor(_ editor: EditorTextView, dropSlides payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool
```

Give the new delegate methods defaults, so every conformer that only records slide changes (D2's `Recorder` in `EditorTextViewTests`) keeps compiling:

```swift
extension EditorTextViewDelegate {
    func editor(_ editor: EditorTextView, payloadForHeaderDragOfBoxAt index: Int) -> SlideDragPayload? { nil }
    func editor(_ editor: EditorTextView, dropSlides payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool { false }
}
```

In `make()`: `view.registerForDraggedTypes(view.registeredDraggedTypes + [NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType)])`.

In `EditorTextView`:

```swift
    private(set) var dropIndicatorBeforeNumber: Int?
    private var dropIndicatorCount = 0
    private static let slideType = NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType)
    static let dragThreshold: CGFloat = 4
    var optionHeld: () -> Bool = { NSEvent.modifierFlags.contains(.option) }
    /// Starts the drag session. A seam: a test replaces it to see what a
    /// header drag carries without a real drag session.
    lazy var headerDragStarter: (SlideDragPayload, NSRect, NSEvent) -> Void = { [weak self] payload, rect, event in
        guard let self, let data = try? payload.data() else { return }
        let item = NSPasteboardItem()
        item.setData(data, forType: Self.slideType)
        let draggingItem = NSDraggingItem(pasteboardWriter: item)
        let index = self.boxes.firstIndex { $0.slide.number == payload.slideNumbers[0] } ?? 0
        draggingItem.setDraggingFrame(rect, contents: self.headerImage(forBoxAt: index, count: payload.slideNumbers.count))
        self.beginDraggingSession(with: [draggingItem], event: event, source: self)
    }

    // MARK: Dragging a header

    /// A mouse down on a box header: a drag past the threshold drags the
    /// slide (or the selection it belongs to); a mouse up before that is a
    /// click, handled as any click in the text.
    override func mouseDown(with event: NSEvent) {
        let point = convert(event.locationInWindow, from: nil)
        guard let index = boxIndex(forHeaderAt: point), let window,
              let payload = editorDelegate?.editor(self, payloadForHeaderDragOfBoxAt: index) else {
            super.mouseDown(with: event)
            return
        }
        var outcome: NSEvent?
        window.trackEvents(matching: [.leftMouseDragged, .leftMouseUp], timeout: 1.5, mode: .eventTracking) { tracked, stop in
            guard let tracked else {
                stop.pointee = true
                return
            }
            if tracked.type == .leftMouseUp || hypot(tracked.locationInWindow.x - event.locationInWindow.x, tracked.locationInWindow.y - event.locationInWindow.y) >= Self.dragThreshold {
                outcome = tracked
                stop.pointee = true
            }
        }
        if let outcome, outcome.type == .leftMouseDragged {
            let rect = headerRect(forBoxAt: index) ?? NSRect(origin: point, size: NSSize(width: 200, height: Self.headerHeight))
            headerDragStarter(payload, rect, event)
            return
        }
        // A click: the caret goes where a click in the text would put it.
        let caret = characterIndexForInsertion(at: point)
        setSelectedRange(NSRange(location: min(caret, (string as NSString).length), length: 0))
    }

    /// The dragged picture: the header's number and title, with a count.
    private func headerImage(forBoxAt index: Int, count: Int) -> NSImage {
        let header = BoxHeader(slide: boxes[index].slide)
        let text = count > 1 ? "\(header.number) \(header.meta)  +\(count - 1)" : "\(header.number) \(header.meta)"
        let attributes: [NSAttributedString.Key: Any] = [.font: NSFont.systemFont(ofSize: 12, weight: .semibold), .foregroundColor: NSColor.labelColor]
        let size = (text as NSString).size(withAttributes: attributes)
        return NSImage(size: NSSize(width: size.width + 24, height: Self.headerHeight), flipped: false) { rect in
            EditorPalette.boxFill.setFill()
            NSBezierPath(roundedRect: rect, xRadius: 8, yRadius: 8).fill()
            (text as NSString).draw(at: NSPoint(x: 12, y: (rect.height - size.height) / 2), withAttributes: attributes)
            return true
        }
    }

    override func draggingSession(_ session: NSDraggingSession, sourceOperationMaskFor context: NSDraggingContext) -> NSDragOperation {
        // Only a slide drag is this view's own; a text drag keeps NSTextView's answer.
        guard session.draggingPasteboard.data(forType: Self.slideType) != nil else {
            return super.draggingSession(session, sourceOperationMaskFor: context)
        }
        return [.move, .copy]
    }

    // MARK: Dropping slides

    private func slidePayload(_ sender: NSDraggingInfo) -> SlideDragPayload? {
        sender.draggingPasteboard.data(forType: Self.slideType).flatMap { SlideDragPayload(data: $0) }
    }

    func updateDropIndicator(at point: NSPoint, count: Int) {
        let boundary = dropBoundary(at: point)
        if boundary != dropIndicatorBeforeNumber || count != dropIndicatorCount {
            dropIndicatorBeforeNumber = boundary
            dropIndicatorCount = count
            needsDisplay = true
            NSAccessibility.post(element: self, notification: .announcementRequested,
                                 userInfo: [.announcement: SlideAccessibility.dropLabel(beforeNumber: boundary, count: count),
                                            .priority: NSAccessibilityPriorityLevel.medium.rawValue])
        }
    }

    func clearDropIndicator() {
        guard dropIndicatorBeforeNumber != nil || dropIndicatorCount != 0 else { return }
        dropIndicatorBeforeNumber = nil
        dropIndicatorCount = 0
        needsDisplay = true
    }

    /// A slide drop moves, from this deck or another; Option copies (decision 2).
    private func slideDropOperation() -> NSDragOperation {
        optionHeld() ? .copy : .move
    }

    override func draggingEntered(_ sender: NSDraggingInfo) -> NSDragOperation {
        guard let payload = slidePayload(sender) else { return super.draggingEntered(sender) }
        updateDropIndicator(at: convert(sender.draggingLocation, from: nil), count: payload.slideNumbers.count)
        return slideDropOperation()
    }

    override func draggingUpdated(_ sender: NSDraggingInfo) -> NSDragOperation {
        guard let payload = slidePayload(sender) else { return super.draggingUpdated(sender) }
        updateDropIndicator(at: convert(sender.draggingLocation, from: nil), count: payload.slideNumbers.count)
        return slideDropOperation()
    }

    override func draggingExited(_ sender: NSDraggingInfo?) {
        if sender.flatMap(slidePayload) != nil { clearDropIndicator() } else { super.draggingExited(sender) }
    }

    override func performDragOperation(_ sender: NSDraggingInfo) -> Bool {
        guard let payload = slidePayload(sender) else { return super.performDragOperation(sender) }
        let beforeNumber = dropBoundary(at: convert(sender.draggingLocation, from: nil))
        return performSlideDrop(payload: payload, beforeNumber: beforeNumber, isMove: slideDropOperation() == .move)
    }

    override func draggingEnded(_ sender: NSDraggingInfo) {
        clearDropIndicator()
        super.draggingEnded(sender)
    }

    /// The drop itself, shared with `performDragOperation` so a test can
    /// drive it without a real drag.
    @discardableResult
    func performSlideDrop(payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool {
        clearDropIndicator()
        return editorDelegate?.editor(self, dropSlides: payload, beforeNumber: beforeNumber, isMove: isMove) ?? false
    }

    // MARK: Accessibility

    /// The text view's own children, plus one element per visible box and
    /// one for the drop indicator while a drag is over the editor.
    override func accessibilityChildren() -> [Any]? {
        var children = super.accessibilityChildren() ?? []
        for index in visibleBoxIndices() {
            guard let rect = boxRect(forBoxAt: index) else { continue }
            let element = NSAccessibilityElement.element(withRole: .group, frame: convertToScreen(rect), label: SlideAccessibility.label(for: boxes[index].slide), parent: self) as! NSAccessibilityElement
            element.setAccessibilityIdentifier("box-\(boxes[index].slide.number)")
            children.append(element)
        }
        // A drag is over the editor exactly while the count is set: both
        // update paths set it and clearDropIndicator zeroes it.
        if dropIndicatorCount > 0 {
            let y = dropIndicatorY(beforeNumber: dropIndicatorBeforeNumber) ?? 0
            let rect = NSRect(x: textContainerOrigin.x - Self.boxOutset, y: y - 4, width: bounds.width, height: 8)
            let element = NSAccessibilityElement.element(withRole: .splitter, frame: convertToScreen(rect),
                                                         label: SlideAccessibility.dropLabel(beforeNumber: dropIndicatorBeforeNumber, count: dropIndicatorCount),
                                                         parent: self) as! NSAccessibilityElement
            element.setAccessibilityIdentifier("drop-indicator")
            children.append(element)
        }
        return children
    }

    private func convertToScreen(_ rect: NSRect) -> NSRect {
        guard let window else { return rect }
        return window.convertToScreen(convert(rect, to: nil))
    }
```

In `DeckSessionController`'s `EditorTextViewDelegate` conformance:

```swift
    func editor(_ editor: EditorTextView, payloadForHeaderDragOfBoxAt index: Int) -> SlideDragPayload? {
        let number = editor.boxes[index].slide.number
        let numbers = slidePanel.selectedNumbers.contains(number) ? slidePanel.selectedNumbers : [number]
        return dragPayload(forSlides: numbers)
    }

    func editor(_ editor: EditorTextView, dropSlides payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool {
        dropSlides(payload: payload, beforeNumber: beforeNumber, isMove: isMove)
    }
```

- [ ] **Step 5: Run the tests and D2's editor tests**

```bash
make -C desktop test ONLY=TapTests/EditorHeaderDragTests/testMoveByDraggingABoxHeaderInTheEditor
make -C desktop test ONLY=TapTests/EditorHeaderDragTests/testTheDropIndicatorHasALabel
make -C desktop test ONLY=TapTests/EditorTextViewTests
```

Expected: all pass; D2's editor tests compile through the delegate defaults and are unchanged by the geometry refactor. If `boxRect` returns nil for boxes that `drawBoxes` used to draw, the viewport had not been laid out yet: the test's `ensureLayout` call covers it, and production calls happen from mouse events after layout. If `trackEvents` does not see the posted drag event in the hosted host, post it with `atStart: true` and record which worked; do not replace the tracking with a synchronous call.

- [ ] **Step 6: Mutate and commit**

Mutations, each reverted: in `dropBoundary`, use `rect.maxY` instead of `rect.midY` (expected: the lower-half assertion fails); in `mouseDown`, always call `super` (expected: `testMoveByDraggingABoxHeaderInTheEditor` fails on `started`); in `performDragOperation`, pass `beforeNumber: nil` (expected: it fails on the round trip, Five lands last); in `slideDropOperation`, return `.copy` (expected: it fails on `.move`).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): drag a box header in the editor to move its slide"
```

---

### Task 13: New Slide and the layout gallery

**Files:**
- Create: `desktop/Tap/Slides/LayoutCatalogLoader.swift`, `desktop/Tap/Slides/NewSlideButton.swift`, `desktop/Tap/Slides/LayoutGalleryController.swift`, `desktop/Tap/Slides/LayoutSchematicView.swift`
- Modify: `desktop/Tap/Windows/DeckWindowController.swift`, `desktop/Tap/App/AppEnvironment.swift`, `desktop/Tap/Slides/SlideOperations.swift`, `desktop/TapTests/Support/HostedTestCase.swift`
- Test: `desktop/TapTests/NewSlideTests.swift`, `desktop/TapTests/Support/TapTemplate.swift`

**Interfaces:**
- Consumes: `LayoutCatalog`, `LayoutSchematic`, `LastLayout` (Task 5), `tap slide add --layout <x> --print` (the same template, plain), `SlideEditing.firstSlotRange` (Task 1), `perform(.insert)` (Task 10).
- Produces:
  - `final class LayoutCatalogLoader` (`@MainActor`) with `private(set) var templates: [LayoutTemplate]`, `var isLoaded: Bool`, `func load() async`, `func template(named:) -> LayoutTemplate?`; `AppEnvironment.shared.layoutCatalog`, `AppEnvironment.shared.lastLayout`
  - `final class NewSlideButton: NSButton` with `var onClick`, `var onHold`, `var holdDelay`; its `target` and `action` are the window controller's `newSlide(_:)`, so VoiceOver and Full Keyboard Access reach it
  - `final class LayoutGalleryController` with `private(set) var templates`, `let footerLabel: NSTextField`, `var onPick: ((String) -> Void)?`, `func show(templates:relativeTo:of:afterSlide:)`, `func pick(_ name: String)`, `private(set) var isShown: Bool` (the controller's own state, not the popover's), `func close()`
  - `DeckWindowController`: `static let newSlideItemIdentifier`, `let newSlideButton: NewSlideButton`, `private(set) var layoutGallery: LayoutGalleryController`, `@objc func newSlide(_:)`, `@objc func newSlideFromLayout(_:)` (the menu item's `representedObject` is the layout name), `@objc func showLayoutGallery(_:)`, `func insertSlide(layout: String, after number: Int?)`
  - `DeckSessionController.insertNewSlide(markdown: String, after number: Int?) -> Bool`: inserts, then selects the first slot.
  - `TapTemplate.print(layout:) async throws -> String` (test support: runs the bundled `tap slide add --layout <x> --print`)
  - Entry points: a click on the toolbar button inserts the last layout; holding it opens the gallery; Slide > New Slide inserts the last layout (the mockup's Option+Command+N) and Slide > New Slide from Layout > Show Layout Gallery opens the gallery. Feature 03 says "choose Slide > New Slide" opens the gallery, which the mockup contradicts: open question 4.

- [ ] **Step 1: Write the failing hosted tests**

`desktop/TapTests/Support/TapTemplate.swift`:

```swift
import Foundation
@testable import Tap

/// What tap writes for a layout, straight from the bundled binary.
enum TapTemplate {
    static func print(layout: String) async throws -> String {
        let executable = await MainActor.run { AppEnvironment.shared.tapExecutableURL }
        return try await withCheckedThrowingContinuation { continuation in
            DispatchQueue.global().async {
                let process = Process()
                process.executableURL = executable
                process.arguments = ["slide", "add", "--layout", layout, "--print"]
                let output = Pipe()
                process.standardOutput = output
                process.standardError = FileHandle.nullDevice
                do { try process.run() } catch { return continuation.resume(throwing: error) }
                let data = output.fileHandleForReading.readDataToEndOfFile()
                process.waitUntilExit()
                continuation.resume(returning: String(decoding: data, as: UTF8.self))
            }
        }
    }
}
```

`desktop/TapTests/NewSlideTests.swift`:

```swift
import XCTest
@testable import Tap

final class NewSlideTests: HostedTestCase {
    func openOps() async throws -> (DeckDocument, DeckSessionController, DeckWindowController) {
        let document = try await openDeck(try Fixtures.copyDeck("ops.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        await AppEnvironment.shared.layoutCatalog.load()
        return (document, controller, try XCTUnwrap(document.windowControllers.first as? DeckWindowController))
    }

    /// A press on the toolbar's New Slide button, through its real mouse
    /// handling: a short press is a click, a long one a hold.
    func pressNewSlideButton(_ windowController: DeckWindowController, hold: Bool) async throws {
        let button = windowController.newSlideButton
        button.holdDelay = 0.05
        let window = try XCTUnwrap(button.window, "the toolbar has laid the button out")
        let point = button.convert(NSPoint(x: button.bounds.midX, y: button.bounds.midY), to: nil)
        func event(_ type: NSEvent.EventType) throws -> NSEvent {
            try XCTUnwrap(NSEvent.mouseEvent(with: type, location: point, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                             windowNumber: window.windowNumber, context: nil, eventNumber: 0, clickCount: 1, pressure: 1))
        }
        button.mouseDown(with: try event(.leftMouseDown))
        if hold { try await Task.sleep(nanoseconds: 150_000_000) }
        button.mouseUp(with: try event(.leftMouseUp))
    }

    func testInsertASlideWithTheLastLayout() async throws {
        let (document, controller, windowController) = try await openOps()
        AppEnvironment.shared.lastLayout.name = "two-column"
        controller.editor.moveCursor(toSlide: 2)

        try await pressNewSlideButton(windowController, hold: false)
        let template = try await TapTemplate.print(layout: "two-column")
        let text = controller.editor.string as NSString
        XCTAssertEqual(controller.editor.boxes.count, 8)
        XCTAssertEqual(text.substring(with: controller.editor.boxes[3].range), template.trimmingCharacters(in: .whitespacesAndNewlines),
                       "the two-column template is slide 4, after slide 3")
        let list = try await TapSlideList.list(text: controller.editor.string)
        XCTAssertEqual(list.slides.map(\.title), ["One", "Two", "Three", list.slides[3].title, "Four", "Five", "Six", "Seven"])
        TapSlideList.assertOneSeparatorBetweenSlides(controller.editor.string, list: list)
        let selected = controller.editor.selectedRange()
        XCTAssertEqual(text.substring(with: selected), "Left content", "the cursor is in the new slide's first slot, with its placeholder selected")
        XCTAssertEqual(document.undoManager?.undoActionName, "New Slide")
        XCTAssertTrue(windowController.window?.firstResponder === controller.editor, "typing replaces the placeholder")
        XCTAssertFalse(windowController.layoutGallery.isShown, "a click never opens the gallery")

        // The menu item with a layout inserts that layout and makes it the last one.
        let item = NSMenuItem(title: "Quote", action: #selector(DeckWindowController.newSlideFromLayout(_:)), keyEquivalent: "")
        item.representedObject = "quote"
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }
        windowController.newSlideFromLayout(item)
        let quote = try await TapTemplate.print(layout: "quote")
        XCTAssertEqual((controller.editor.string as NSString).substring(with: controller.editor.boxes[4].range), quote.trimmingCharacters(in: .whitespacesAndNewlines))
        XCTAssertEqual(AppEnvironment.shared.lastLayout.name, "quote")
    }

    func testPickALayoutFromTheGallery() async throws {
        let (_, controller, windowController) = try await openOps()
        controller.editor.moveCursor(toSlide: 2)
        try await pressNewSlideButton(windowController, hold: true)
        let gallery = windowController.layoutGallery
        XCTAssertTrue(gallery.isShown, "holding the button opens the gallery")
        XCTAssertEqual(gallery.templates.map(\.name), AppEnvironment.shared.layoutCatalog.templates.map(\.name), "the layouts and their templates come from tap")
        XCTAssertEqual(gallery.templates.count, 12)
        XCTAssertEqual(gallery.templates.first?.name, "title", "in tap's order")
        XCTAssertTrue(gallery.templates.allSatisfy { !LayoutSchematic.elements(for: $0.markdown).isEmpty }, "every cell has a preview")
        XCTAssertEqual(gallery.footerLabel.stringValue, "Inserts after slide 3")

        gallery.pick("big-stat")
        XCTAssertFalse(gallery.isShown)
        let template = try await TapTemplate.print(layout: "big-stat")
        XCTAssertEqual((controller.editor.string as NSString).substring(with: controller.editor.boxes[3].range), template.trimmingCharacters(in: .whitespacesAndNewlines))
        XCTAssertEqual(AppEnvironment.shared.lastLayout.name, "big-stat", "the gallery's pick becomes the last layout")
        XCTAssertEqual((controller.editor.string as NSString).substring(with: controller.editor.selectedRange()), "100%")

        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }
        windowController.showLayoutGallery(nil)
        XCTAssertTrue(gallery.isShown, "Slide > New Slide from Layout > Show Layout Gallery opens it too")
        gallery.close()
        XCTAssertFalse(gallery.isShown)
        try await pressNewSlideButton(windowController, hold: false)
        XCTAssertEqual(controller.editor.boxes.count, 9)
        XCTAssertEqual((controller.editor.string as NSString).substring(with: controller.editor.boxes[4].range), template.trimmingCharacters(in: .whitespacesAndNewlines),
                       "New Slide now inserts big-stat")
    }
}
```

In `HostedTestCase.setUp`: `AppEnvironment.shared.lastLayout = LastLayout(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.layout.\(UUID().uuidString)")))`.

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/NewSlideTests/testInsertASlideWithTheLastLayout`
Expected: compile failure, `has no member 'layoutCatalog'`.

- [ ] **Step 3: The loader, the button, the gallery**

`LayoutCatalogLoader.swift`:

```swift
import Foundation

/// Runs the bundled tap once for every layout's template. The gallery and
/// the Slide menu read `templates`; New Slide reads `template(named:)`.
@MainActor
final class LayoutCatalogLoader {
    static let didLoadNotification = Notification.Name("TapLayoutCatalogDidLoad")
    private(set) var templates: [LayoutTemplate] = []
    var isLoaded: Bool { !templates.isEmpty }
    private var loading: Task<Void, Never>?
    private let executable: () -> URL

    init(executable: @escaping () -> URL) {
        self.executable = executable
    }

    func template(named name: String) -> LayoutTemplate? {
        templates.first { $0.name == name }
    }

    func load() async {
        if isLoaded { return }
        if let loading { return await loading.value }
        let task = Task { @MainActor [weak self] in
            guard let self else { return }
            let executable = self.executable()
            let data = await Self.run(executable, arguments: ["slide", "add", "--print", "--json"])
            if let data, let templates = try? LayoutCatalog.decode(data) {
                self.templates = templates
                NotificationCenter.default.post(name: Self.didLoadNotification, object: self)
            }
        }
        loading = task
        await task.value
        loading = nil
    }

    nonisolated static func run(_ executable: URL, arguments: [String]) async -> Data? {
        await withCheckedContinuation { continuation in
            DispatchQueue.global().async {
                let process = Process()
                process.executableURL = executable
                process.arguments = arguments
                let output = Pipe()
                process.standardOutput = output
                process.standardError = FileHandle.nullDevice
                guard (try? process.run()) != nil else { return continuation.resume(returning: nil) }
                let data = output.fileHandleForReading.readDataToEndOfFile()
                process.waitUntilExit()
                continuation.resume(returning: process.terminationStatus == 0 ? data : nil)
            }
        }
    }
}
```

In `AppEnvironment`: `lazy var layoutCatalog = LayoutCatalogLoader(executable: { [weak self] in self?.tapExecutableURL ?? URL(fileURLWithPath: "/usr/bin/false") })`, `var lastLayout = LastLayout()`, and in `warmUp`: `Task { await layoutCatalog.load() }`.

`NewSlideButton.swift`:

```swift
import AppKit

/// The toolbar's New Slide button: a click inserts a slide with the last
/// layout, holding it opens the layout gallery.
final class NewSlideButton: NSButton {
    var onClick: (() -> Void)?
    var onHold: (() -> Void)?
    var holdDelay: TimeInterval = 0.35
    private var holdWork: DispatchWorkItem?
    private var held = false

    override func mouseDown(with event: NSEvent) {
        held = false
        highlight(true)
        let work = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated {
                guard let self else { return }
                self.held = true
                self.highlight(false)
                self.onHold?()
            }
        }
        holdWork = work
        DispatchQueue.main.asyncAfter(deadline: .now() + holdDelay, execute: work)
    }

    override func mouseUp(with event: NSEvent) {
        holdWork?.cancel()
        holdWork = nil
        highlight(false)
        if !held, bounds.contains(convert(event.locationInWindow, from: nil)) { onClick?() }
    }
}
```

`LayoutSchematicView.swift`:

```swift
import AppKit

/// Draws a layout's schematic: bars for headings and lines, blocks for
/// columns, code and media, a big bar for a statistic.
final class LayoutSchematicView: NSView {
    var elements: [LayoutSchematic.Element] = [] { didSet { needsDisplay = true } }

    override func draw(_ dirtyRect: NSRect) {
        NSColor.white.setFill()
        NSBezierPath(roundedRect: bounds, xRadius: 7, yRadius: 7).fill()
        let dark = NSColor.black.withAlphaComponent(0.38)
        let light = NSColor.black.withAlphaComponent(0.12)
        let heights: [CGFloat] = elements.map { element in
            switch element {
            case .heading: return 5
            case .line: return 3
            case .columns, .sidebar, .media: return 26
            case .code: return 30
            case .quote: return 14
            case .bigNumber: return 12
            }
        }
        let total = heights.reduce(0, +) + CGFloat(max(0, elements.count - 1)) * 4
        var y = (bounds.height + total) / 2
        for (element, height) in zip(elements, heights) {
            y -= height
            let width = bounds.width * 0.86
            let x = (bounds.width - width) / 2
            switch element {
            case .heading:
                dark.setFill(); NSRect(x: bounds.midX - width * 0.29, y: y, width: width * 0.58, height: height).fill()
            case .line:
                light.setFill(); NSRect(x: bounds.midX - width * 0.4, y: y, width: width * 0.8, height: height).fill()
            case .bigNumber:
                dark.setFill(); NSRect(x: bounds.midX - width * 0.22, y: y, width: width * 0.44, height: height).fill()
            case .columns(let count):
                let gap: CGFloat = 5
                let column = (width - gap * CGFloat(count - 1)) / CGFloat(count)
                for index in 0..<count {
                    light.setFill(); NSRect(x: x + CGFloat(index) * (column + gap), y: y, width: column, height: height).fill()
                }
            case .sidebar:
                light.setFill(); NSRect(x: x, y: y, width: width * 0.62, height: height).fill()
                dark.setFill(); NSRect(x: x + width * 0.68, y: y, width: width * 0.32, height: height).fill()
            case .media:
                light.setFill(); NSRect(x: x + width * 0.5, y: y, width: width * 0.5, height: height).fill()
                dark.setFill(); NSRect(x: x, y: y + height - 4, width: width * 0.42, height: 4).fill()
            case .code:
                dark.withAlphaComponent(0.2).setFill(); NSRect(x: x, y: y, width: width, height: height).fill()
            case .quote:
                dark.setFill(); NSRect(x: x, y: y, width: 3, height: height).fill()
                light.setFill(); NSRect(x: x + 8, y: y + height - 3, width: width * 0.7, height: 3).fill()
                NSRect(x: x + 8, y: y + 2, width: width * 0.5, height: 3).fill()
            }
            y -= 4
        }
    }
}
```

`LayoutGalleryController.swift`:

```swift
import AppKit

/// The gallery of tap's layouts in a popover: a schematic and a name per
/// layout, in tap's order, and a footer saying where the slide goes.
@MainActor
final class LayoutGalleryController: NSObject, NSCollectionViewDataSource, NSCollectionViewDelegate, NSPopoverDelegate {
    private static let cellIdentifier = NSUserInterfaceItemIdentifier("LayoutCell")
    private(set) var templates: [LayoutTemplate] = []
    let footerLabel = NSTextField(labelWithString: "")
    let sourceLabel = NSTextField(labelWithString: "Templates from tap slide add --print")
    var onPick: ((String) -> Void)?
    /// Kept by this controller rather than read from the popover, whose
    /// `isShown` can depend on whether the app is active, which differs
    /// between this machine and the CI runner.
    private(set) var isShown = false
    private let popover = NSPopover()
    private let collectionView: GalleryCollectionView

    /// Return picks the selected layout; NSCollectionView moves the selection with the arrow keys on its own.
    final class GalleryCollectionView: NSCollectionView {
        var onReturn: (() -> Void)?
        override func insertNewline(_ sender: Any?) { onReturn?() }
        override func keyDown(with event: NSEvent) {
            if event.keyCode == 36 || event.keyCode == 76 { onReturn?() } else { super.keyDown(with: event) }
        }
    }

    final class LayoutCell: NSCollectionViewItem {
        let schematic = LayoutSchematicView()
        let nameLabel = NSTextField(labelWithString: "")

        override func loadView() {
            let root = NSView()
            schematic.wantsLayer = true
            schematic.layer?.cornerRadius = 7
            nameLabel.font = .systemFont(ofSize: 11.5)
            nameLabel.alignment = .center
            for view in [schematic, nameLabel] as [NSView] {
                view.translatesAutoresizingMaskIntoConstraints = false
                root.addSubview(view)
            }
            NSLayoutConstraint.activate([
                schematic.topAnchor.constraint(equalTo: root.topAnchor, constant: 4),
                schematic.centerXAnchor.constraint(equalTo: root.centerXAnchor),
                schematic.widthAnchor.constraint(equalToConstant: 116),
                schematic.heightAnchor.constraint(equalToConstant: 65),
                nameLabel.topAnchor.constraint(equalTo: schematic.bottomAnchor, constant: 6),
                nameLabel.centerXAnchor.constraint(equalTo: root.centerXAnchor),
            ])
            view = root
        }

        override var isSelected: Bool {
            didSet {
                schematic.layer?.borderWidth = isSelected ? 3 : 0
                schematic.layer?.borderColor = NSColor.controlAccentColor.cgColor
            }
        }
    }

    override init() {
        collectionView = GalleryCollectionView()
        super.init()
        let layout = NSCollectionViewFlowLayout()
        layout.itemSize = NSSize(width: 124, height: 96)
        layout.minimumInteritemSpacing = 6
        layout.minimumLineSpacing = 8
        layout.sectionInset = NSEdgeInsets(top: 12, left: 12, bottom: 4, right: 12)
        collectionView.collectionViewLayout = layout
        collectionView.isSelectable = true
        collectionView.register(LayoutCell.self, forItemWithIdentifier: Self.cellIdentifier)
        collectionView.dataSource = self
        collectionView.delegate = self
        collectionView.backgroundColors = [.clear]
        collectionView.setAccessibilityIdentifier("layout-gallery")
        collectionView.onReturn = { [weak self] in
            guard let self, let path = self.collectionView.selectionIndexPaths.first else { return }
            self.pick(self.templates[path.item].name)
        }
        footerLabel.font = .systemFont(ofSize: 11)
        footerLabel.textColor = .secondaryLabelColor
        sourceLabel.font = .systemFont(ofSize: 11)
        sourceLabel.textColor = .tertiaryLabelColor

        let footer = NSStackView(views: [footerLabel, NSView(), sourceLabel])
        footer.orientation = .horizontal
        footer.edgeInsets = NSEdgeInsets(top: 4, left: 16, bottom: 12, right: 16)
        let scroll = NSScrollView()
        scroll.documentView = collectionView
        scroll.drawsBackground = false
        let stack = NSStackView(views: [scroll, footer])
        stack.orientation = .vertical
        stack.spacing = 0
        let content = NSViewController()
        content.view = stack
        stack.widthAnchor.constraint(equalToConstant: 4 * 124 + 3 * 6 + 24).isActive = true
        scroll.heightAnchor.constraint(equalToConstant: 3 * 96 + 2 * 8 + 16).isActive = true
        popover.contentViewController = content
        popover.behavior = .transient
        popover.delegate = self
    }

    func show(templates: [LayoutTemplate], relativeTo rect: NSRect, of view: NSView, afterSlide number: Int?) {
        self.templates = templates
        footerLabel.stringValue = number.map { "Inserts after slide \($0)" } ?? "Inserts at the end"
        collectionView.reloadData()
        popover.show(relativeTo: rect, of: view, preferredEdge: .maxY)
        isShown = true
        if !templates.isEmpty { collectionView.selectionIndexPaths = [IndexPath(item: 0, section: 0)] }
        // The popover has its own window; Return and the arrow keys go to the grid only there.
        collectionView.window?.makeFirstResponder(collectionView)
    }

    func close() {
        isShown = false
        popover.performClose(nil)
    }

    func popoverDidClose(_ notification: Notification) {
        isShown = false
    }

    /// Picks a layout by name: the click on a cell, Return, and the tests all come here.
    func pick(_ name: String) {
        guard templates.contains(where: { $0.name == name }) else { return }
        close()
        onPick?(name)
    }

    func collectionView(_ collectionView: NSCollectionView, numberOfItemsInSection section: Int) -> Int { templates.count }

    func collectionView(_ collectionView: NSCollectionView, itemForRepresentedObjectAt indexPath: IndexPath) -> NSCollectionViewItem {
        let cell = collectionView.makeItem(withIdentifier: Self.cellIdentifier, for: indexPath) as! LayoutCell
        let template = templates[indexPath.item]
        cell.schematic.elements = LayoutSchematic.elements(for: template.markdown)
        cell.nameLabel.stringValue = LayoutCatalog.displayName(template.name)
        cell.view.setAccessibilityLabel(LayoutCatalog.displayName(template.name))
        cell.view.setAccessibilityIdentifier("layout-\(template.name)")
        return cell
    }

    func collectionView(_ collectionView: NSCollectionView, didSelectItemsAt indexPaths: Set<IndexPath>) {
        // A mouse click selects and picks; the keyboard's selection changes only select.
        guard let path = indexPaths.first, NSApp.currentEvent?.type == .leftMouseUp || NSApp.currentEvent?.type == .leftMouseDown else { return }
        pick(templates[path.item].name)
    }
}
```

- [ ] **Step 4: The toolbar item and the actions**

In `DeckWindowController`:

```swift
    static let newSlideItemIdentifier = NSToolbarItem.Identifier("newSlide")
    let newSlideButton = NewSlideButton()
    private(set) lazy var layoutGallery: LayoutGalleryController = {
        let gallery = LayoutGalleryController()
        gallery.onPick = { [weak self] name in self?.insertSlide(layout: name, after: self?.sessionController.currentSlideNumber) }
        return gallery
    }()
```

`toolbarDefaultItemIdentifiers` returns `[Self.slidesItemIdentifier, .flexibleSpace, Self.newSlideItemIdentifier, Self.previewItemIdentifier]`, and the item:

```swift
        if identifier == Self.newSlideItemIdentifier {
            let item = NSToolbarItem(itemIdentifier: identifier)
            item.label = "New Slide"
            item.toolTip = "New slide with the last layout. Hold for the layout gallery."
            newSlideButton.image = NSImage(systemSymbolName: "plus.rectangle.on.rectangle", accessibilityDescription: "New Slide")
            newSlideButton.bezelStyle = .toolbar
            newSlideButton.setAccessibilityIdentifier("new-slide-button")
            // VoiceOver's press and Full Keyboard Access go through target and action; the mouse through onClick and onHold.
            newSlideButton.target = self
            newSlideButton.action = #selector(newSlide(_:))
            newSlideButton.onClick = { [weak self] in self?.newSlide(nil) }
            newSlideButton.onHold = { [weak self] in self?.showLayoutGallery(nil) }
            item.view = newSlideButton
            return item
        }
```

Actions:

```swift
    @objc func newSlide(_ sender: Any?) {
        insertSlide(layout: AppEnvironment.shared.lastLayout.name, after: sessionController.currentSlideNumber)
    }

    @objc func newSlideFromLayout(_ sender: Any?) {
        guard let name = (sender as? NSMenuItem)?.representedObject as? String else { return }
        insertSlide(layout: name, after: sessionController.currentSlideNumber)
    }

    @objc func showLayoutGallery(_ sender: Any?) {
        let anchor: NSView = newSlideButton.window == nil ? (window?.contentView ?? newSlideButton) : newSlideButton
        layoutGallery.show(templates: AppEnvironment.shared.layoutCatalog.templates, relativeTo: anchor.bounds, of: anchor,
                           afterSlide: sessionController.currentSlideNumber)
    }

    /// Inserts a slide of the layout after `number` (or at the end), and
    /// remembers the layout for the next New Slide.
    func insertSlide(layout: String, after number: Int?) {
        guard let template = AppEnvironment.shared.layoutCatalog.template(named: layout) else {
            sessionController.session.log.append("no template for layout \(layout): the layout catalog has not loaded", source: .app)
            NSSound.beep()
            return
        }
        if sessionController.insertNewSlide(markdown: template.markdown, after: number) {
            AppEnvironment.shared.lastLayout.name = layout
        }
    }
```

In `SlideOperations.swift`:

```swift
    /// Inserts one slide after `number` (nil or the last slide: at the
    /// end) and selects its first slot, so typing replaces the placeholder.
    @discardableResult
    func insertNewSlide(markdown: String, after number: Int?) -> Bool {
        let count = editor.boxes.count
        let beforeNumber = number.flatMap { $0 + 1 <= count ? $0 + 1 : nil }
        guard perform(.insert(markdowns: [markdown], beforeNumber: beforeNumber)) else { return false }
        let newNumber = beforeNumber ?? editor.boxes.count
        guard editor.boxes.indices.contains(newNumber - 1) else { return true }
        let box = editor.boxes[newNumber - 1]
        let slot = SlideEditing.firstSlotRange(inSlideText: (editor.string as NSString).substring(with: box.range))
        editor.setSelectedRange(NSRange(location: box.range.location + slot.location, length: slot.length))
        editor.window?.makeFirstResponder(editor)
        return true
    }
```

- [ ] **Step 5: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/NewSlideTests/testInsertASlideWithTheLastLayout
make -C desktop test ONLY=TapTests/NewSlideTests/testPickALayoutFromTheGallery
```

Expected: green. The gallery's popover shows over the deck window without activating the app; `popover.show` inside a non-active app is allowed and does not steal focus from another app. If a synthesized `mouseDown` on the toolbar button cannot find the button in a window on the CI runner, the toolbar has not laid out yet: call `window.toolbar?.validateVisibleItems()` and `window.layoutIfNeeded()` in `pressNewSlideButton` before reading `button.window`.

- [ ] **Step 6: Mutate and commit**

Mutations, each reverted: in `insertSlide`, drop the `lastLayout.name = layout` line (expected: `testPickALayoutFromTheGallery` fails on the last layout); in `insertNewSlide`, skip the `setSelectedRange` (expected: `testInsertASlideWithTheLastLayout` fails on "Left content"); remove `onClick?()` from `NewSlideButton.mouseUp` (expected: `testInsertASlideWithTheLastLayout` fails on the box count); remove the hold timer (expected: `testPickALayoutFromTheGallery` fails on `isShown`); in `newSlideFromLayout`, ignore the `representedObject` and insert the last layout (expected: the quote assertion fails).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): New Slide with the last layout, and the layout gallery from tap's templates"
```

---

### Task 14: The Slide menu, its actions and its validation

**Files:**
- Create: `desktop/Tap/Slides/LayoutMenuDelegate.swift`
- Modify: `desktop/Tap/App/MainMenu.swift`, `desktop/Tap/Windows/DeckWindowController.swift`
- Test: `desktop/TapTests/SlideMenuTests.swift`

**Interfaces:**
- Consumes: `perform`, `moveSelectedSlides(by:)`, `moveSelectedSlides(toTop:)` (Task 10), `LayoutCatalogLoader`, `newSlideFromLayout`, `showLayoutGallery` (Task 13), `SlideAccessibility` (Task 3), `EditorTextView.accessibilityChildren` (Task 12). Decision 3 (the person, 2026-09-24): Command+Delete in the Slide menu; plain Delete deletes slides only while the sidebar has focus; Backspace in the editor stays text deletion.
- Produces:
  - `DeckWindowController` actions: `duplicateSlides(_:)`, `deleteSlides(_:)`, `toggleSkipSlides(_:)`, `moveSlidesUp(_:)`, `moveSlidesDown(_:)`, `moveSlidesToTop(_:)`, `moveSlidesToBottom(_:)`, `newSlideAfter(_:)`; `validateMenuItem` sets "Skip Slide"/"Unskip Slide" and "Delete Slide"/"Delete 2 Slides", and disables Delete when every slide is selected.
  - `final class LayoutMenuDelegate: NSObject, NSMenuDelegate` filling "New Slide from Layout" from the catalog, ending with "Show Layout Gallery…".
  - The Slide menu: New Slide (Option+Command+N), New Slide from Layout (submenu), Duplicate (Command+D), Skip Slide, Delete (Command+Delete), Move Up and Move Down (Option+Command+Up and Down), Move to Top, Move to Bottom, Insert Image…, Generate Image…, New Component… (the last three disabled until D6), Go to Slide….

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/SlideMenuTests.swift`:

```swift
import XCTest
@testable import Tap

final class SlideMenuTests: HostedTestCase {
    func slideMenu() throws -> NSMenu {
        try XCTUnwrap(NSApp.mainMenu?.items.first { $0.title == "Slide" }?.submenu)
    }

    /// Items are found by action, not title: validation renames Delete and Skip.
    func item(_ menu: NSMenu, _ action: Selector) throws -> NSMenuItem {
        try XCTUnwrap(menu.items.first { $0.action == action }, "no item for \(action)")
    }

    func openOps() async throws -> (DeckDocument, DeckSessionController, DeckWindowController) {
        let document = try await openDeck(try Fixtures.copyDeck("ops.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "the panel") { controller.slidePanel.slides.count == 7 }
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        return (document, controller, try XCTUnwrap(document.windowControllers.first as? DeckWindowController))
    }

    func testSlideMenu() async throws {
        _ = try await openOps()
        await AppEnvironment.shared.layoutCatalog.load()
        let menu = try slideMenu()
        let titles = menu.items.map(\.title)
        for required in ["New Slide", "New Slide from Layout", "Duplicate", "Delete", "Move Up", "Move Down", "Generate Image…", "New Component…", "Go to Slide…"] {
            XCTAssertTrue(titles.contains(required), "\(required) is missing from \(titles)")
        }
        let newSlide = try item(menu, #selector(DeckWindowController.newSlide(_:)))
        XCTAssertEqual(newSlide.keyEquivalent, "n")
        XCTAssertEqual(newSlide.keyEquivalentModifierMask, [.command, .option])
        let layouts = try XCTUnwrap(menu.items.first { $0.title == "New Slide from Layout" }?.submenu)
        layouts.delegate?.menuNeedsUpdate?(layouts)
        XCTAssertEqual(layouts.items.compactMap { $0.representedObject as? String }, AppEnvironment.shared.layoutCatalog.templates.map(\.name), "with layouts, from tap")
        XCTAssertEqual(layouts.items.first?.title, "Title")
        XCTAssertEqual(layouts.items.last?.action, #selector(DeckWindowController.showLayoutGallery(_:)))
        XCTAssertEqual(try item(menu, #selector(DeckWindowController.moveSlidesUp(_:))).keyEquivalentModifierMask, [.command, .option])
        XCTAssertEqual(try item(menu, #selector(DeckWindowController.duplicateSlides(_:))).keyEquivalent, "d")
        let delete = try item(menu, #selector(DeckWindowController.deleteSlides(_:)))
        XCTAssertEqual(delete.keyEquivalent, "\u{8}")
        XCTAssertEqual(delete.keyEquivalentModifierMask, [.command], "Command+Delete, so Backspace stays the editor's")
        XCTAssertNil(menu.items.first { $0.title == "Generate Image…" }?.action, "disabled until the image commands arrive")
        XCTAssertNil(menu.items.first { $0.title == "New Component…" }?.action)
    }

    func testDeleteIsDisabledWhenEverySlideIsSelected() async throws {
        let (_, controller, windowController) = try await openOps()
        let item = NSMenuItem(title: "", action: #selector(DeckWindowController.deleteSlides(_:)), keyEquivalent: "")
        controller.slidePanel.select(numbers: [2, 3], scroll: false)
        XCTAssertTrue(windowController.validateMenuItem(item))
        XCTAssertEqual(item.title, "Delete 2 Slides")
        controller.slidePanel.select(numbers: Array(1...7), scroll: false)
        XCTAssertFalse(windowController.validateMenuItem(item), "a deck keeps at least one slide")
        windowController.deleteSlides(nil)
        XCTAssertEqual(controller.editor.boxes.count, 7)
    }

    func testMoveToTopAndBottomAndSkipFromTheMenu() async throws {
        let (_, controller, windowController) = try await openOps()
        controller.editor.moveCursor(toSlide: 2)
        windowController.moveSlidesToBottom(nil)
        XCTAssertEqual(try await TapSlideList.list(text: controller.editor.string).slides.map(\.title), ["One", "Two", "Four", "Five", "Six", "Seven", "Three"])
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }
        windowController.moveSlidesToTop(nil)
        XCTAssertEqual(try await TapSlideList.list(text: controller.editor.string).slides.map(\.title), ["Three", "One", "Two", "Four", "Five", "Six", "Seven"])
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }

        let skip = NSMenuItem(title: "", action: #selector(DeckWindowController.toggleSkipSlides(_:)), keyEquivalent: "")
        XCTAssertTrue(windowController.validateMenuItem(skip))
        XCTAssertEqual(skip.title, "Skip Slide")
        windowController.toggleSkipSlides(nil)
        XCTAssertTrue(try await TapSlideList.list(text: controller.editor.string).slides[0].skip)
        try await waitUntil(timeout: 10, "tap's answer") { controller.editor.boxes[0].slide.skip }
        XCTAssertTrue(windowController.validateMenuItem(skip))
        XCTAssertEqual(skip.title, "Unskip Slide")
    }

    func testVoiceOver() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        let windowController = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        try await waitUntil(timeout: 5, "the panel") { controller.slidePanel.slides.count == 7 }
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        controller.editor.layoutSubtreeIfNeeded()

        XCTAssertEqual(controller.slidePanel.item(forSlide: 3)?.view.accessibilityLabel(), "Slide 3, default layout, What We Knew, 1 step")
        let boxElements = try XCTUnwrap(controller.editor.accessibilityChildren()).compactMap { $0 as? NSAccessibilityElement }
        XCTAssertEqual(boxElements.first { $0.accessibilityIdentifier() == "box-3" }?.accessibilityLabel(), "Slide 3, default layout, What We Knew, 1 step")
        XCTAssertEqual(SlideAccessibility.dropLabel(beforeNumber: 3, count: 1), "Drop 1 slide above slide 3")

        // Every slide operation from the keyboard alone: a menu item with a key, or a menu item at all.
        let menu = try slideMenu()
        for action in [#selector(DeckWindowController.newSlide(_:)), #selector(DeckWindowController.duplicateSlides(_:)),
                       #selector(DeckWindowController.deleteSlides(_:)), #selector(DeckWindowController.moveSlidesUp(_:)),
                       #selector(DeckWindowController.moveSlidesDown(_:))] {
            XCTAssertFalse(try item(menu, action).keyEquivalent.isEmpty, "\(action) has a shortcut")
        }
        for action in [#selector(DeckWindowController.toggleSkipSlides(_:)), #selector(DeckWindowController.moveSlidesToTop(_:)),
                       #selector(DeckWindowController.moveSlidesToBottom(_:))] {
            XCTAssertNotNil(try item(menu, action), "\(action) is reachable through the menu bar")
        }
        controller.editor.moveCursor(toSlide: 2)
        windowController.moveSlidesUp(nil)
        XCTAssertEqual(controller.editor.boxes[1].slide.title, "What We Knew", "Cmd+Option+Up moved the slide")
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }
        windowController.moveSlidesDown(nil)
        XCTAssertEqual(controller.editor.boxes[2].slide.title, "What We Knew")
    }
}
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/SlideMenuTests/testSlideMenu`
Expected: failure on the missing menu items ("New Slide is missing").

- [ ] **Step 3: The menus**

`LayoutMenuDelegate.swift`:

```swift
import AppKit

/// Fills "New Slide from Layout" from tap's catalog each time it opens,
/// so the menu never hard-codes a layout name.
@MainActor
final class LayoutMenuDelegate: NSObject, NSMenuDelegate {
    static let shared = LayoutMenuDelegate()

    func menuNeedsUpdate(_ menu: NSMenu) {
        menu.removeAllItems()
        let templates = AppEnvironment.shared.layoutCatalog.templates
        if templates.isEmpty {
            menu.addItem(MainMenu.item("Loading layouts from tap…", action: nil))
        }
        for template in templates {
            let item = MainMenu.item(LayoutCatalog.displayName(template.name), action: #selector(DeckWindowController.newSlideFromLayout(_:)))
            item.representedObject = template.name
            menu.addItem(item)
        }
        menu.addItem(.separator())
        menu.addItem(MainMenu.item("Show Layout Gallery…", action: #selector(DeckWindowController.showLayoutGallery(_:))))
    }
}
```

`MainMenu.slideMenu()`:

```swift
    static func slideMenu() -> NSMenu {
        let menu = NSMenu(title: "Slide")
        menu.addItem(item("New Slide", action: #selector(DeckWindowController.newSlide(_:)), key: "n", modifiers: [.command, .option]))
        let fromLayout = item("New Slide from Layout", action: nil)
        let layouts = NSMenu(title: "New Slide from Layout")
        layouts.delegate = LayoutMenuDelegate.shared
        fromLayout.submenu = layouts
        menu.addItem(fromLayout)
        menu.addItem(.separator())
        menu.addItem(item("Duplicate", action: #selector(DeckWindowController.duplicateSlides(_:)), key: "d"))
        menu.addItem(item("Skip Slide", action: #selector(DeckWindowController.toggleSkipSlides(_:))))
        // Command-Delete, not Delete alone: a bare Delete key equivalent would take Backspace away from the editor (decision 3).
        menu.addItem(item("Delete", action: #selector(DeckWindowController.deleteSlides(_:)), key: "\u{8}"))
        menu.addItem(.separator())
        let upArrow = String(Character(Unicode.Scalar(UInt16(NSUpArrowFunctionKey))!))
        let downArrow = String(Character(Unicode.Scalar(UInt16(NSDownArrowFunctionKey))!))
        menu.addItem(item("Move Up", action: #selector(DeckWindowController.moveSlidesUp(_:)), key: upArrow, modifiers: [.command, .option]))
        menu.addItem(item("Move Down", action: #selector(DeckWindowController.moveSlidesDown(_:)), key: downArrow, modifiers: [.command, .option]))
        menu.addItem(item("Move to Top", action: #selector(DeckWindowController.moveSlidesToTop(_:))))
        menu.addItem(item("Move to Bottom", action: #selector(DeckWindowController.moveSlidesToBottom(_:))))
        menu.addItem(.separator())
        // The image and component commands arrive with tap image and tap component new.
        menu.addItem(item("Insert Image…", action: nil, key: "i", modifiers: [.command, .shift]))
        menu.addItem(item("Generate Image…", action: nil))
        menu.addItem(item("New Component…", action: nil))
        menu.addItem(.separator())
        menu.addItem(item("Go to Slide…", action: #selector(DeckWindowController.goToSlide(_:)), key: "o", modifiers: [.command, .shift]))
        return menu
    }
```

- [ ] **Step 4: The actions**

In `DeckWindowController`:

```swift
    @objc func duplicateSlides(_ sender: Any?) {
        sessionController.perform(.duplicate(numbers: sessionController.selectedSlideNumbers))
    }

    @objc func deleteSlides(_ sender: Any?) {
        let numbers = sessionController.selectedSlideNumbers
        guard numbers.count < sessionController.editor.boxes.count else { return }
        sessionController.perform(.delete(numbers: numbers))
    }

    @objc func toggleSkipSlides(_ sender: Any?) {
        let numbers = sessionController.selectedSlideNumbers
        sessionController.perform(.setSkip(numbers: numbers, skipped: !selectionIsSkipped))
    }

    /// True when every selected slide is skipped, so the menu offers Unskip.
    private var selectionIsSkipped: Bool {
        let numbers = Set(sessionController.selectedSlideNumbers)
        let selected = sessionController.editor.boxes.filter { numbers.contains($0.slide.number) }
        return !selected.isEmpty && selected.allSatisfy(\.slide.skip)
    }

    @objc func moveSlidesUp(_ sender: Any?) { sessionController.moveSelectedSlides(by: -1) }
    @objc func moveSlidesDown(_ sender: Any?) { sessionController.moveSelectedSlides(by: 1) }
    @objc func moveSlidesToTop(_ sender: Any?) { sessionController.moveSelectedSlides(toTop: true) }
    @objc func moveSlidesToBottom(_ sender: Any?) { sessionController.moveSelectedSlides(toTop: false) }

    @objc func newSlideAfter(_ sender: Any?) {
        insertSlide(layout: AppEnvironment.shared.lastLayout.name, after: sessionController.selectedSlideNumbers.max())
    }
```

In `validateMenuItem`, before `return true`:

```swift
        let count = sessionController.selectedSlideNumbers.count
        if menuItem.action == #selector(deleteSlides(_:)) {
            menuItem.title = count > 1 ? "Delete \(count) Slides" : "Delete Slide"
            return count > 0 && count < sessionController.editor.boxes.count
        }
        if menuItem.action == #selector(toggleSkipSlides(_:)) {
            menuItem.title = (selectionIsSkipped ? "Unskip" : "Skip") + (count > 1 ? " Slides" : " Slide")
            return count > 0
        }
        if [#selector(duplicateSlides(_:)), #selector(moveSlidesUp(_:)), #selector(moveSlidesDown(_:)), #selector(moveSlidesToTop(_:)),
            #selector(moveSlidesToBottom(_:)), #selector(newSlideAfter(_:))].contains(menuItem.action) {
            return count > 0
        }
```

- [ ] **Step 5: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/SlideMenuTests/testSlideMenu
make -C desktop test ONLY=TapTests/SlideMenuTests/testDeleteIsDisabledWhenEverySlideIsSelected
make -C desktop test ONLY=TapTests/SlideMenuTests/testMoveToTopAndBottomAndSkipFromTheMenu
make -C desktop test ONLY=TapTests/SlideMenuTests/testVoiceOver
make -C desktop test ONLY=TapTests/MenuTests/testMenuBar
```

Expected: all pass; D2's menu bar test still sees eight menus in the same order.

- [ ] **Step 6: Mutate and commit**

Mutations, each reverted: in `validateMenuItem`, return `true` for `deleteSlides` regardless of the count (expected: `testDeleteIsDisabledWhenEverySlideIsSelected` fails on the second validation); give Delete no modifier (expected: `testSlideMenu` fails on the modifier mask); make `moveSlidesToTop` call `moveSelectedSlides(toTop: false)` (expected: `testMoveToTopAndBottomAndSkipFromTheMenu` fails).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): the Slide menu with its layouts, actions and validation"
```

---

### Task 15: Context menus, the sidebar's keys, copy and paste

**Files:**
- Create: `desktop/Tap/Slides/SlideContextMenu.swift`
- Modify: `desktop/Tap/Windows/DeckWindowController.swift`, `desktop/Tap/Sidebar/SlidePanelViewController.swift`, `desktop/Tap/Editor/EditorTextView.swift`, `desktop/Tap/Slides/SlideOperations.swift`, `desktop/Tap/App/AppEnvironment.swift`
- Test: `desktop/TapTests/SlideContextMenuTests.swift`

**Interfaces:**
- Consumes: the Slide menu actions (Task 14), `perform`, `markdown(forSlides:)`, `insertSlides(markdowns:beforeNumber:)` (Task 10), `SlideDragPayload` (Task 3), `EditorTextView.boxIndex(forHeaderAt:)` (Task 12).
- Produces:
  - `enum SlideContextMenu { static func build(for numbers: [Int], target: AnyObject) -> NSMenu }`: New Slide After, Duplicate, Skip Slide, Move to Top, Move to Bottom, Copy, Paste, Generate Image… and Insert Image… (disabled until D6), Delete Slide or Delete Slides.
  - `DeckWindowController.copySlides(_:)`, `pasteSlides(_:)`, using `AppEnvironment.shared.slidePasteboard` (the general pasteboard; tests replace it with a named one so a local run never overwrites the person's clipboard); `validateMenuItem` enables Paste when the pasteboard holds slides or text.
  - `DeckSessionController.copySlides(_:to:)`, `pasteSlides(from:after:) -> Bool`
  - `SlidePanelDelegate` gains `func slidePanelContextMenu(_:) -> NSMenu?`, `func slidePanelDeleteSelection(_:)`, `func slidePanelCopySelection(_:)`, `func slidePanelPaste(_:)`.
  - `SlidePanelCollectionView` handles the Delete and Forward Delete keys in `keyDown`, `copy:`, `paste:`, and shows the context menu from `menu(for:)`; `SlidePanelViewController.contextMenu(forSlide:) -> NSMenu?` selects an unselected slide first.
  - `EditorTextView.menu(for:)` shows the same menu on a right-click on a box header, through `EditorTextViewDelegate.editor(_:contextMenuForBoxAt:)` (default nil).
  - Copy puts `io.geocod.tap.slides` and plain text on the pasteboard.

- [ ] **Step 1: Write the failing tests**

`desktop/TapTests/SlideContextMenuTests.swift`:

```swift
import XCTest
@testable import Tap

final class SlideContextMenuTests: HostedTestCase {
    func openOps() async throws -> (DeckDocument, DeckSessionController, DeckWindowController) {
        let document = try await openDeck(try Fixtures.copyDeck("ops.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "the panel") { controller.slidePanel.slides.count == 7 }
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        controller.editor.layoutSubtreeIfNeeded()
        controller.editor.textLayoutManager?.ensureLayout(for: controller.editor.textLayoutManager!.documentRange)
        return (document, controller, try XCTUnwrap(document.windowControllers.first as? DeckWindowController))
    }

    func titles(_ controller: DeckSessionController) async throws -> [String] {
        try await TapSlideList.list(text: controller.editor.string).slides.map(\.title)
    }

    func rightClick(at point: NSPoint, in view: NSView) throws -> NSEvent {
        let window = try XCTUnwrap(view.window)
        return try XCTUnwrap(NSEvent.mouseEvent(with: .rightMouseDown, location: view.convert(point, to: nil), modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                                windowNumber: window.windowNumber, context: nil, eventNumber: 0, clickCount: 1, pressure: 1))
    }

    func keyDown(_ keyCode: UInt16, characters: String, in view: NSView) throws -> NSEvent {
        let window = try XCTUnwrap(view.window)
        return try XCTUnwrap(NSEvent.keyEvent(with: .keyDown, location: .zero, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                              windowNumber: window.windowNumber, context: nil, characters: characters, charactersIgnoringModifiers: characters,
                                              isARepeat: false, keyCode: keyCode))
    }

    func testContextMenuOnASlide() async throws {
        let (_, controller, windowController) = try await openOps()
        let panel = controller.slidePanel
        // A right-click on thumbnail 3, through the collection view's own menu(for:).
        let item3 = try XCTUnwrap(panel.item(forSlide: 3))
        let frame = item3.view.convert(item3.view.bounds, to: panel.collectionView)
        let menu = try XCTUnwrap(panel.collectionView.menu(for: try rightClick(at: NSPoint(x: frame.midX, y: frame.midY), in: panel.collectionView)))
        XCTAssertEqual(panel.selectedNumbers, [3], "a right-click on an unselected thumbnail selects it")
        let titles = menu.items.map(\.title)
        for required in ["New Slide After", "Duplicate", "Delete Slide", "Move to Top", "Move to Bottom", "Copy"] {
            XCTAssertTrue(titles.contains(required), "\(required) is missing from \(titles)")
        }
        // A right-click on box 3's header, through the editor's own menu(for:).
        let header3 = try XCTUnwrap(controller.editor.headerRect(forBoxAt: 2))
        let editorMenu = try XCTUnwrap(controller.editor.menu(for: try rightClick(at: NSPoint(x: header3.midX, y: header3.midY), in: controller.editor)))
        XCTAssertEqual(editorMenu.items.map(\.title), titles, "a box header offers the same menu")
        XCTAssertNil(controller.editor.menu(for: try rightClick(at: NSPoint(x: header3.midX, y: header3.maxY + EditorTextView.lineHeight * 2), in: controller.editor))?.items.first { $0.title == "Move to Top" },
                     "a right-click in the text is the text view's own menu")

        windowController.moveSlidesToBottom(nil)
        XCTAssertEqual(try await titles(controller), ["One", "Two", "Four", "Five", "Six", "Seven", "Three"])
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }

        let pasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.copy.\(UUID().uuidString)"))
        AppEnvironment.shared.slidePasteboard = pasteboard
        windowController.copySlides(nil)
        XCTAssertNotNil(pasteboard.data(forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType)))
        XCTAssertEqual(pasteboard.string(forType: .string), "<!-- layout: section -->\n# Three")
        controller.editor.moveCursor(toSlide: 0)
        let paste = NSMenuItem(title: "", action: #selector(DeckWindowController.pasteSlides(_:)), keyEquivalent: "")
        XCTAssertTrue(windowController.validateMenuItem(paste))
        windowController.pasteSlides(nil)
        XCTAssertEqual(try await titles(controller), ["One", "Three", "Two", "Four", "Five", "Six", "Seven", "Three"], "pasted after the current slide")
    }

    func testTheDeleteKeyInTheSidebarDeletesTheSelection() async throws {
        let (_, controller, _) = try await openOps()
        let panel = controller.slidePanel
        let window = try XCTUnwrap(panel.collectionView.window)
        window.makeFirstResponder(panel.collectionView)
        panel.select(numbers: [5, 6], scroll: false)
        // The real key path: keyDown with the Delete key's code, as the key event reaches the first responder.
        panel.collectionView.keyDown(with: try keyDown(51, characters: "\u{7f}", in: panel.collectionView))
        XCTAssertEqual(try await titles(controller), ["One", "Two", "Three", "Four", "Seven"])
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }
        panel.select(numbers: [1], scroll: false)
        panel.collectionView.keyDown(with: try keyDown(117, characters: "\u{f728}", in: panel.collectionView))
        XCTAssertEqual(try await titles(controller), ["Two", "Three", "Four", "Seven"], "Forward Delete too")
    }

    func testBackspaceInTheEditorDeletesTextNotSlides() async throws {
        let (_, controller, _) = try await openOps()
        let editor = controller.editor
        let window = try XCTUnwrap(editor.window)
        window.makeFirstResponder(editor)
        let end = NSMaxRange(editor.boxes[0].range)
        editor.setSelectedRange(NSRange(location: end, length: 0))
        let before = editor.string
        editor.keyDown(with: try keyDown(51, characters: "\u{7f}", in: editor))
        XCTAssertEqual((editor.string as NSString).length, (before as NSString).length - 1, "Backspace deleted one character")
        XCTAssertEqual(editor.boxes.count, 7, "and no slide")
        XCTAssertFalse(editor.string.contains("# One\n\nFirst."), "the character came off slide 1's last line")
    }
}
```

- [ ] **Step 2: Run one test to verify it fails**

Run: `make -C desktop test ONLY=TapTests/SlideContextMenuTests/testContextMenuOnASlide`
Expected: compile failure, `has no member 'slidePasteboard'`.

- [ ] **Step 3: The context menu and the pasteboard**

`SlideContextMenu.swift`:

```swift
import AppKit

/// The menu a right-click on a thumbnail or a box header shows. Every item
/// is also in the Slide menu, so it is reachable from the keyboard.
enum SlideContextMenu {
    static func build(for numbers: [Int], target: AnyObject) -> NSMenu {
        let menu = NSMenu(title: "Slide")
        func add(_ title: String, _ action: Selector?, key: String = "", modifiers: NSEvent.ModifierFlags = [.command]) {
            let item = NSMenuItem(title: title, action: action, keyEquivalent: key)
            item.keyEquivalentModifierMask = modifiers
            item.target = action == nil ? nil : target
            menu.addItem(item)
        }
        let several = numbers.count > 1
        add("New Slide After", #selector(DeckWindowController.newSlideAfter(_:)))
        add("Duplicate", #selector(DeckWindowController.duplicateSlides(_:)), key: "d")
        add("Skip Slide", #selector(DeckWindowController.toggleSkipSlides(_:)))
        menu.addItem(.separator())
        add("Move to Top", #selector(DeckWindowController.moveSlidesToTop(_:)))
        add("Move to Bottom", #selector(DeckWindowController.moveSlidesToBottom(_:)))
        menu.addItem(.separator())
        add("Copy", #selector(DeckWindowController.copySlides(_:)), key: "c")
        add("Paste", #selector(DeckWindowController.pasteSlides(_:)), key: "v")
        menu.addItem(.separator())
        add("Generate Image…", nil)
        add("Insert Image…", nil, key: "i", modifiers: [.command, .shift])
        menu.addItem(.separator())
        add(several ? "Delete Slides" : "Delete Slide", #selector(DeckWindowController.deleteSlides(_:)), key: "\u{8}")
        return menu
    }
}
```

In `AppEnvironment`: `var slidePasteboard: NSPasteboard = .general`.

In `DeckWindowController`:

```swift
    @objc func copySlides(_ sender: Any?) {
        sessionController.copySlides(sessionController.selectedSlideNumbers, to: AppEnvironment.shared.slidePasteboard)
    }

    @objc func pasteSlides(_ sender: Any?) {
        sessionController.pasteSlides(from: AppEnvironment.shared.slidePasteboard, after: sessionController.selectedSlideNumbers.max())
    }
```

and in `validateMenuItem`, with the other slide actions:

```swift
        if menuItem.action == #selector(copySlides(_:)) { return count > 0 }
        if menuItem.action == #selector(pasteSlides(_:)) {
            let pasteboard = AppEnvironment.shared.slidePasteboard
            return pasteboard.data(forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType)) != nil || pasteboard.string(forType: .string) != nil
        }
```

In `SlideOperations.swift`:

```swift
    /// Copies slides as the app's own type and as plain markdown.
    func copySlides(_ numbers: [Int], to pasteboard: NSPasteboard) {
        guard let payload = dragPayload(forSlides: numbers), let data = try? payload.data() else { return }
        pasteboard.clearContents()
        let item = NSPasteboardItem()
        item.setData(data, forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType))
        item.setString(payload.markdowns.joined(separator: SlideDocument.separator), forType: .string)
        pasteboard.writeObjects([item])
    }

    /// Pastes slides after `number`: the app's own type when present,
    /// otherwise plain text as one slide.
    @discardableResult
    func pasteSlides(from pasteboard: NSPasteboard, after number: Int?) -> Bool {
        let beforeNumber = number.flatMap { $0 + 1 <= editor.boxes.count ? $0 + 1 : nil }
        if let data = pasteboard.data(forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType)), let payload = SlideDragPayload(data: data) {
            return perform(.insert(markdowns: payload.markdowns, beforeNumber: beforeNumber))
        }
        if let text = pasteboard.string(forType: .string), !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            return perform(.insert(markdowns: [text], beforeNumber: beforeNumber))
        }
        return false
    }
```

- [ ] **Step 4: The panel's keys and menu, the editor's header menu**

In `SlidePanelDelegate`, add:

```swift
    func slidePanelContextMenu(_ panel: SlidePanelViewController) -> NSMenu?
    func slidePanelDeleteSelection(_ panel: SlidePanelViewController)
    func slidePanelCopySelection(_ panel: SlidePanelViewController)
    func slidePanelPaste(_ panel: SlidePanelViewController)
```

In `SlidePanelCollectionView`:

```swift
    var onDelete: (() -> Void)?
    var onCopy: (() -> Void)?
    var onPaste: (() -> Void)?
    var contextMenuProvider: ((Int?) -> NSMenu?)?

    @objc func copy(_ sender: Any?) { onCopy?() }
    @objc func paste(_ sender: Any?) { onPaste?() }

    /// Delete (51) and Forward Delete (117) delete the selected slides. Only
    /// here, with the sidebar focused: in the editor these keys edit text.
    override func keyDown(with event: NSEvent) {
        if event.keyCode == 51 || event.keyCode == 117 {
            onDelete?()
        } else {
            super.keyDown(with: event)
        }
    }

    override func menu(for event: NSEvent) -> NSMenu? {
        let number = indexPathForItem(at: convert(event.locationInWindow, from: nil)).map { $0.item + 1 }
        return contextMenuProvider?(number)
    }
```

In `SlidePanelViewController.loadView`, wire them:

```swift
        collectionView.onDelete = { [weak self] in self.map { $0.delegate?.slidePanelDeleteSelection($0) } }
        collectionView.onCopy = { [weak self] in self.map { $0.delegate?.slidePanelCopySelection($0) } }
        collectionView.onPaste = { [weak self] in self.map { $0.delegate?.slidePanelPaste($0) } }
        collectionView.contextMenuProvider = { [weak self] number in number.flatMap { self?.contextMenu(forSlide: $0) } }
```

and add:

```swift
    /// The context menu for a thumbnail. A right-click on a thumbnail
    /// outside the selection selects it alone first, as Finder does.
    func contextMenu(forSlide number: Int) -> NSMenu? {
        if !selectedNumbers.contains(number) {
            select(numbers: [number], scroll: false)
            delegate?.slidePanel(self, didClickSlide: number, selection: [number])
        }
        return delegate?.slidePanelContextMenu(self)
    }
```

In `DeckSessionController`'s conformance:

```swift
    func slidePanelContextMenu(_ panel: SlidePanelViewController) -> NSMenu? {
        guard let windowController = editor.window?.windowController as? DeckWindowController else { return nil }
        return SlideContextMenu.build(for: selectedSlideNumbers, target: windowController)
    }

    func slidePanelDeleteSelection(_ panel: SlidePanelViewController) {
        let numbers = selectedSlideNumbers
        guard numbers.count < editor.boxes.count else { return NSSound.beep() }
        perform(.delete(numbers: numbers))
    }

    func slidePanelCopySelection(_ panel: SlidePanelViewController) {
        copySlides(selectedSlideNumbers, to: AppEnvironment.shared.slidePasteboard)
    }

    func slidePanelPaste(_ panel: SlidePanelViewController) {
        pasteSlides(from: AppEnvironment.shared.slidePasteboard, after: selectedSlideNumbers.max())
    }
```

In `EditorTextView`, add to the delegate protocol `func editor(_ editor: EditorTextView, contextMenuForBoxAt index: Int) -> NSMenu?` with a default of nil in the protocol extension, and:

```swift
    override func menu(for event: NSEvent) -> NSMenu? {
        if let index = boxIndex(forHeaderAt: convert(event.locationInWindow, from: nil)) {
            return editorDelegate?.editor(self, contextMenuForBoxAt: index)
        }
        return super.menu(for: event)
    }
```

with the session controller's implementation selecting the box's slide in the panel (`slidePanel.click(slide: number, extendingSelection: false)` when it is not selected) and returning `SlideContextMenu.build(for: selectedSlideNumbers, target: windowController)`.

- [ ] **Step 5: Run the tests one at a time**

```bash
make -C desktop test ONLY=TapTests/SlideContextMenuTests/testContextMenuOnASlide
make -C desktop test ONLY=TapTests/SlideContextMenuTests/testTheDeleteKeyInTheSidebarDeletesTheSelection
make -C desktop test ONLY=TapTests/SlideContextMenuTests/testBackspaceInTheEditorDeletesTextNotSlides
```

Expected: all pass.

- [ ] **Step 6: Mutate and commit**

Mutations, each reverted: remove the `keyCode == 51 || keyCode == 117` branch from `keyDown` (expected: `testTheDeleteKeyInTheSidebarDeletesTheSelection` fails); make `SlidePanelCollectionView.menu(for:)` return nil (expected: `testContextMenuOnASlide` fails on its first unwrap); make `EditorTextView.menu(for:)` always call `super` (expected: it fails on the editor menu); make `pasteSlides` insert at the end regardless of `number` (expected: it fails on the pasted order).

```bash
git add desktop/Tap desktop/TapTests
git commit -m "feat(desktop): slide context menus, the sidebar's Delete key, copy and paste"
```

---

### Task 16: The scenario manifest, the UI tests, the thumbnail benchmark and the README

**Files:**
- Modify: `desktop/scenarios.txt`, `desktop/README.md`
- Create: `desktop/TapUITests/SlideDragUITests.swift`, `desktop/TapUITests/SlidePanelUITests.swift`, `desktop/TapBenchmarks/ThumbnailBenchmark.swift`

**Interfaces:**
- Consumes: `desktop/scripts/check-scenarios.sh` (D2 Task 31), the accessibility identifiers this plan set: `thumbnail-<n>`, `slide-panel`, `slide-panel-overlay`, `slides-button`, `new-slide-button`, `layout-gallery`, `layout-<name>`, `box-<n>`, `editor` (D2).
- Produces: the 22 `D3` rows; UI tests that the person runs with `make -C desktop uitest`; a thumbnail benchmark on D2's 200-slide deck that the person runs with `make -C desktop bench`.

- [ ] **Step 1: Check the scenario tooling, then claim the scenarios**

Run: `test -f desktop/scenarios.txt && test -x desktop/scripts/check-scenarios.sh && echo present`. Expected: `present` (D2's Task 31). Append to `desktop/scenarios.txt`:

```text
D3 | 02-slide-structure.feature | The current slide
D3 | 02-slide-structure.feature | Sidebar and cursor are linked
D3 | 03-slide-operations.feature | Move one slide by dragging in the sidebar
D3 | 03-slide-operations.feature | Move with the keyboard
D3 | 03-slide-operations.feature | Move several slides
D3 | 03-slide-operations.feature | Move by dragging a box header in the editor
D3 | 03-slide-operations.feature | Insert a slide with the last layout
D3 | 03-slide-operations.feature | Pick a layout from the gallery
D3 | 03-slide-operations.feature | Duplicate and delete
D3 | 03-slide-operations.feature | Skip a slide
D3 | 03-slide-operations.feature | Frontmatter never moves
D3 | 03-slide-operations.feature | Drag slides to another deck
D3 | 04-preview.feature | Split layout
D3 | 04-preview.feature | Peek at the slide panel
D3 | 04-preview.feature | Pin the slide panel
D3 | 04-preview.feature | Step through a custom component
D3 | 12-menus-and-shortcuts.feature | Slide menu
D3 | 12-menus-and-shortcuts.feature | Context menu on a slide
D3 | 12-menus-and-shortcuts.feature | VoiceOver
D3 | 13-performance.feature | The current thumbnail follows the preview
D3 | 13-performance.feature | Thumbnails
D3 | 13-performance.feature | Reopen
```

Run: `make -C desktop check-scenarios`
Expected: `every claimed scenario has a test`.

- [ ] **Step 2: Write the UI tests (compile only)**

`desktop/TapUITests/SlideDragUITests.swift`:

```swift
import XCTest

/// Real drags, with the real pointer. Local only: they take over the
/// screen and need Xcode's permission to control the computer.
final class SlideDragUITests: UITestCase {
    func testDraggingThumbnailFiveAboveThreeReordersTheDeck() throws {
        let application = launch(withDeck: try copyFixture("ops.md"))
        let panel = application.collectionViews["slide-panel"]
        XCTAssertTrue(panel.waitForExistence(timeout: 30))
        let five = panel.otherElements["thumbnail-5"].firstMatch
        let three = panel.otherElements["thumbnail-3"].firstMatch
        XCTAssertTrue(five.waitForExistence(timeout: 30))
        five.click(forDuration: 0.4, thenDragTo: three.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.05)))
        let editor = application.textViews["editor"]
        XCTAssertTrue(editor.waitForExistence(timeout: 5))
        let text = try XCTUnwrap(editor.value as? String)
        XCTAssertLessThan(try XCTUnwrap(text.range(of: "# Five")?.lowerBound), try XCTUnwrap(text.range(of: "# Three")?.lowerBound), "Five now comes before Three")
    }

    func testDraggingABoxHeaderReordersTheDeck() throws {
        let application = launch(withDeck: try copyFixture("ops.md"))
        let editor = application.textViews["editor"]
        XCTAssertTrue(editor.waitForExistence(timeout: 30))
        let box5 = application.groups["box-5"].firstMatch
        let box3 = application.groups["box-3"].firstMatch
        XCTAssertTrue(box5.waitForExistence(timeout: 30))
        box5.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.08))
            .click(forDuration: 0.4, thenDragTo: box3.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.1)))
        let text = try XCTUnwrap(editor.value as? String)
        XCTAssertLessThan(try XCTUnwrap(text.range(of: "# Five")?.lowerBound), try XCTUnwrap(text.range(of: "# Three")?.lowerBound))
    }
}
```

`desktop/TapUITests/SlidePanelUITests.swift`:

```swift
import XCTest

final class SlidePanelUITests: UITestCase {
    func testHoveringTheSlidesButtonPeeksAtThePanel() throws {
        let application = launch(withDeck: try copyFixture("seven-slides.md"))
        let button = application.buttons["slides-button"]
        XCTAssertTrue(button.waitForExistence(timeout: 30))
        // Unpin first: a fresh install starts pinned.
        button.click()
        let overlay = application.otherElements["slide-panel-overlay"]
        XCTAssertFalse(overlay.exists)
        button.hover()
        XCTAssertTrue(overlay.waitForExistence(timeout: 3), "hovering the button shows the glass overlay")
        application.textViews["editor"].coordinate(withNormalizedOffset: CGVector(dx: 0.8, dy: 0.5)).hover()
        let gone = NSPredicate(format: "exists == false")
        expectation(for: gone, evaluatedWith: overlay)
        waitForExpectations(timeout: 3)
    }

    func testHoldingNewSlideOpensTheGallery() throws {
        let application = launch(withDeck: try copyFixture("ops.md"))
        let button = application.buttons["new-slide-button"]
        XCTAssertTrue(button.waitForExistence(timeout: 30))
        button.press(forDuration: 0.6)
        XCTAssertTrue(application.collectionViews["layout-gallery"].waitForExistence(timeout: 3))
        application.otherElements["layout-big-stat"].firstMatch.click()
        let editor = application.textViews["editor"]
        XCTAssertTrue(try XCTUnwrap(editor.value as? String).contains("layout: big-stat"))
    }
}
```

The elements' XCUI types (`otherElements`, `groups`, `buttons`) depend on the accessibility roles the views expose; the person's first run settles them. Record any that need changing in the ledger rather than guessing further here.

Run: `cd desktop && xcodebuild -project Tap.xcodeproj -derivedDataPath build/DerivedData -scheme Tap -destination 'platform=macOS' CODE_SIGN_IDENTITY=- CODE_SIGNING_REQUIRED=NO build-for-testing`
Expected: `** TEST BUILD SUCCEEDED **`, nothing launched.

- [ ] **Step 3: The thumbnail benchmark (compile only)**

13-performance's "Thumbnails" and "Reopen" scenarios stand under a 200-slide Background that the hosted tests, on seven slides, never use. `desktop/TapBenchmarks/ThumbnailBenchmark.swift`, on D2's `BenchmarkCase` and its `stress-200` deck:

```swift
import XCTest
@testable import Tap

/// A cold pass over every thumbnail of the 200-slide deck, and a reopen
/// that must render nothing. The person runs this with `make -C desktop bench`.
final class ThumbnailBenchmark: BenchmarkCase {
    func testColdThumbnailPassAndReopenFromTheCache() async throws {
        AppEnvironment.shared.thumbnailCache = ThumbnailCache(directory: FileManager.default.temporaryDirectory.appendingPathComponent("tap-bench-thumbnails-\(UUID().uuidString)"))
        let deck = try await openStressDeck()
        let controller = try XCTUnwrap(deck.sessionController)
        let started = Date()
        try await waitUntil(timeout: 180, "200 thumbnails") { (1...200).allSatisfy { controller.slidePanel.image(forSlide: $0) != nil } }
        let seconds = Date().timeIntervalSince(started)
        XCTAssertEqual(controller.thumbnails.renderer.renderCount, 200)
        XCTAssertLessThan(seconds / 200, 0.1, "under 100 ms per slide; the prototype measured about 24 ms")
        let deckURL = try XCTUnwrap(deck.fileURL)
        deck.close()
        try await Task.sleep(nanoseconds: 500_000_000)

        let (document, _) = try await NSDocumentController.shared.openDocument(withContentsOf: deckURL, display: true)
        let reopened = try XCTUnwrap(document as? DeckDocument)
        let again = try XCTUnwrap(reopened.sessionController)
        try await waitUntil(timeout: 60, "200 thumbnails from the cache") { (1...200).allSatisfy { again.slidePanel.image(forSlide: $0) != nil } }
        XCTAssertEqual(again.thumbnails.renderer.renderCount, 0, "reopening loads every thumbnail from the cache without rendering")
    }
}
```

Run: `cd desktop && xcodebuild -project Tap.xcodeproj -derivedDataPath build/DerivedData -scheme TapBenchmarks -destination 'platform=macOS' CODE_SIGN_IDENTITY=- CODE_SIGNING_REQUIRED=NO build-for-testing`
Expected: `** TEST BUILD SUCCEEDED **`, nothing launched.

- [ ] **Step 4: The README**

Add to `desktop/README.md` under Test:

```markdown
The slide operation tests round-trip every result through the bundled
`tap slide list --json` (`TapTests/Support/TapSlideList.swift`), so the
slide order and the separators are checked by tap, never by Swift. The
thumbnail tests need the deck window on screen: the hidden renderer only
paints while the window is visible. Thumbnails are cached under
`~/Library/Application Support/Tap/Thumbnails`; the tests use a temporary
folder instead.
```

- [ ] **Step 5: Commit**

```bash
git add desktop/scenarios.txt desktop/README.md desktop/TapUITests desktop/TapBenchmarks
git commit -m "test(desktop): claim the D3 scenarios, add the drag and peek UI tests and the thumbnail benchmark"
```

---

## Final check

- [ ] Run: `make -C desktop core-test`
  Expected: every `TapDesktopCore` test passes, including the new `SlideDocumentTests`, `SlideEditingTests`, `DirectiveCommentTests`, `SlideDragPayloadTests`, `SlideAccessibilityTests`, `SlidePanelStateTests`, `PresentationSummaryTests`, `ThumbnailCacheTests`, `ThumbnailQueueTests`, `FlatImageCheckTests`, `LayoutCatalogTests`.
- [ ] Run: `make -C desktop check-scenarios`
  Expected: `every claimed scenario has a test`.
- [ ] Push the branch and read CI's `Desktop Tests` job: every hosted test green on the runner, including the D2 tests that now run with a sidebar in the split. The hosted bundle is not run locally in full (branch rule).
- [ ] Run: `cd desktop && xcodebuild -project Tap.xcodeproj -derivedDataPath build/DerivedData -scheme Tap -destination 'platform=macOS' CODE_SIGN_IDENTITY=- CODE_SIGNING_REQUIRED=NO build-for-testing`, and the same with `-scheme TapBenchmarks`
  Expected: the UI tests and the thumbnail benchmark compile. `make -C desktop uitest` and `make -C desktop bench` are the person's runs.
- [ ] Run: `grep -rn "$(printf '\342\200\224')" desktop/ docs/superpowers/plans/2026-09-24-desktop-sidebar-thumbnails-slide-operations.md --include=*.swift --include=*.md --include=*.yml --include=*.sh --include=*.txt`
  Expected: no output.
- [ ] Run: `grep -rn "evaluateJavaScript\|callAsyncJavaScript" desktop/Tap`
  Expected: only `PreviewViewController.pageText` and `pageValue`, the test-only surface D2 documented.
- [ ] Run: `grep -rn "textStorage?.replaceCharacters\|textStorage!.replaceCharacters" desktop/Tap`
  Expected: only inside `EditorTextView.swift`.
- [ ] Run: `grep -rn "updateChangeCount" desktop/Tap`
  Expected: only `DeckSessionController.refreshEditedState`.
- [ ] Run: `grep -rn "NSApp.activate\|activate(ignoringOtherApps" desktop/Tap`
  Expected: no output.
- [ ] Run: `grep -rn "unowned" desktop/Tap desktop/TapDesktopCore/Sources`
  Expected: no output (D2 removed its last `unowned` in 3d93dbf).
- [ ] Run: `grep -rn "isSingleColor" desktop/Tap`
  Expected: no output; `FlatImageCheck` is the one definition of a blank capture.
- [ ] Run: `git diff main -- docs/superpowers/specs`
  Expected: exactly the decision 2 and decision 4 edits (a drag between decks moves, Option copies; the panel's state is per deck).
- [ ] Open the app by hand: `open "desktop/$(make -s -C desktop app-path)"`, then open `examples/basic.md`. Check: the panel pinned on first open with thumbnails filling in, the current one first; unpin and hover the Slides button; drag a thumbnail; Cmd+Option+Up; right-click a box header; hold New Slide and pick a layout; Cmd+Z after each; quit and reopen to see thumbnails come from the cache; `pgrep -fl "tap dev"` prints nothing afterwards.
- [ ] Every part of the D3 outline maps to a task:

  | D3 outline item | Task |
  |---|---|
  | The slide panel, peek and pinned, glass on 26 and visual effect before | 7, 8 |
  | The thumbnail renderer: hidden web view in the main window, `?print=true`, `pageZoom`, `tapReady`, the priority queue that pauses while typing, the disk cache keyed by slide, theme, bundle and size | 4, 6, 9 |
  | Move, duplicate, delete, skip, multi-select, drag and drop between decks, one undo step each, round trips through `tap slide list` | 1, 2, 3, 10, 11, 12, 14, 15 |
  | New Slide and the layout gallery from `tap slide add --layout <x> --print` | 5, 13 |
  | The scenario manifest, the UI tests and the thumbnail benchmark | 16 |

## Pre-flight: conflicts found, rulings and what each costs if wrong

The person's four decisions (2026-09-24) are settled and implemented: 1, focus stays in the sidebar after a thumbnail click (Task 7's test pins it, and Task 8 moves focus to the editor when a peek hides); 2, a drag between decks moves, one undo step per document, with Option copying (Tasks 11 and 12, and the spec and feature edits below); 3, Command+Delete in the Slide menu and plain Delete only while the sidebar has focus, with Backspace in the editor left alone (Tasks 14 and 15, driven through the real key path); 4, the panel's pinned state is per deck and follows a rename (Tasks 3 and 8).

1. **The current thumbnail: a snapshot of the preview (13-performance) or the hidden renderer (spec, "Performance")?** The feature file says "thumbnail 3 becomes a snapshot of the preview"; the spec says it is not a snapshot of the live preview, which can be caught mid-transition (prototype finding 7), and that the current slide goes to the front of the hidden renderer's queue. Ruling: the spec's mechanism; `testTheCurrentThumbnailFollowsThePreview` asserts the observable outcome (thumbnail 3 changes right after the preview re-renders slide 3, before another changed slide). Cost if wrong: the scenario is claimed by a test that does not implement its literal step, and the thumbnail lags the preview by one render (about 25 ms) instead of being the same pixels.
2. **The mockups' floating panel with its own pin button, versus peek on hover and click to pin.** The decided behavior wins (roadmap decision, memory note). Ruling: one toolbar button, `slides-button`, peeks on hover and pins on click; no pin button inside the panel; the View menu item is "Pin Slide Panel" / "Unpin Slide Panel" (Control+Command+S), where the mockup says "Hide Sidebar". Cost if wrong: two small UI details to move.
3. **"Each window remembers its state" (spec and 04-preview) versus decision 4, per deck.** This commit edits the spec ("Each deck then remembers its own state.") and the feature file ("each deck remembers its state after that"). Cost if wrong: one key scheme in `SlidePanelState`.
4. **"Copies the slides, and holding Cmd moves them" (spec and 03-slide-operations) versus decision 2, move by default.** This commit edits the spec ("moves the slides, and holding Option copies them") and the feature file's last scenario (moves, one undo step in each deck; Option copies). Option, not Command, is the macOS convention for a copy. Cost if wrong: one branch in `dropDecision` and one in the editor's `slideDropOperation`.
5. **Command+Delete in the Slide menu takes Command+Backspace from the editor.** A menu key equivalent wins over `NSTextView`'s own Command+Backspace (delete to the beginning of the line) while the editor has focus. Decision 3 accepts this; it is recorded so nobody reads it as a bug. Cost if wrong: one key equivalent string.
6. **The gallery's layout order.** The mockup lists Three Column before Code Focus and Cover before Sidebar; tap's `printAllLayouts` uses the wizard's order. The spec says the layouts come from tap. Ruling: tap's order, and the menu and gallery never hard-code names. Cost if wrong: cosmetic.
7. **"A preview of each layout" in the gallery.** The mockup draws wireframe schematics; a real render needs a running tap on a deck that contains the template, which no open deck does. Ruling: `LayoutSchematic` sketches each template from its own markdown. Cost if wrong: if real renders are wanted, D6 can render a generated twelve-slide deck through the thumbnail renderer alongside the theme renders and swap the cells; the gallery's API does not change.
8. **Slide > New Slide: insert at once (mockup) or open the gallery (03-slide-operations and the spec's "or using Slide > New Slide")?** Ruling: the mockup's model, since it also has "New Slide from Layout" with "Show Layout Gallery…" one level down; the gallery test drives the toolbar hold and the submenu item. Open question 4. Cost if wrong: one menu action swap.
9. **The cache key.** The spec keys thumbnails by "slide text, theme, component bundle, and canvas size". The plan keys by tap's per-slide content hash (`TransformedSlide.hash`, which covers the slide's rendered content and its content-hashed component URL), a theme signature (theme, custom theme flag, theme colours, aspect ratio, and the bundled tap's version), and the snapshot width. Ruling: the same intent with tap's own change detection as the source of truth; two slides with equal text share one image. Cost if wrong: an edit to a custom theme's CSS file changes no key (the public config carries only a boolean), so those thumbnails stay stale until their slides change; open question 2.
10. **Where the hidden web view lives.** The spec and the prototype put it behind the preview pane. D2 made the preview hideable and detachable, both of which hide or move that pane. Ruling: behind the editor's opaque scroll view, which is always in the deck window. Cost if wrong: if WebKit treats a view under an opaque sibling differently from the prototype's arrangement, `testRendersPaintedThumbnailsThroughTheReadySignal` fails on the first run and the host moves to the inspector's root view with an opaque cover, as the prototype did.
11. **The print page and the session cookie.** `audienceRoutes` in `internal/server/app_auth.go` lists `/`, `/api/presentation`, `/assets/`, `/components/` and `/ws` as reachable with no token. Ruling: the renderer loads `?print=true` with no cookie and no launch code. Cost if wrong: if a later tap change gates `/api/presentation` in app mode, thumbnails stop loading and the renderer test fails at once; the fix is a non-persistent data store with the cookie set from the token (its value is the token, `AppSessionCookieName(port)`).
12. **A blank slide and the flat-image check.** A capture of one flat colour is what an unpainted page gives, but a `blank` or `cover` slide with only a background really is one colour. Ruling: a flat capture is retried with a back-off; after three flat captures in a row, each following a paint-proven ready for that slide and revision, the slide is accepted as blank and cached. The check reads every pixel of a 64 by 36 downsample, so a thin stroke anywhere counts. Cost if wrong: a slide that painted late three times in a row is cached blank until its text changes; the "updating" mark never shows for it.
13. **Operations on unconfirmed text.** The spec says every range comes from tap's last answer, and typing shifts the boxes locally until it does. Ruling: an operation on text tap has not answered for waits for that answer (the text is sent at once), then runs on tap's ranges; after three seconds with no answer it beeps and does nothing. Cost if wrong: a keyboard move pressed within about 100 ms of a keystroke lands a moment later rather than at once.
14. **A stale answer after an operation.** An answer to a send begun before the operation would be replayed with the operation's whole-region edit and collapse the boxes. Ruling: adopting boxes refuses every earlier send, as `reset` does; the operation's own send is the next answer applied. Cost if wrong: none beyond one extra PUT when an operation lands during a typing pause's send.
15. **Undo and redo of an operation.** D2 (6f8d3ca) resends the buffer after every undo and redo. Ruling: the operation's undo closure only notes which boxes belong to the reverted text; D2's `undoOrRedoDidChangeText` adopts them before its send, so there is one resend path and its answer is the one applied. Cost if wrong: the boxes are shifted for one round trip after Cmd+Z.
16. **The explicit undo group around an operation.** Removing `beginUndoGrouping` and `endUndoGrouping` leaves every test green, because AppKit's event group already holds the box registration and the text change. The group stays as a written property of the code, not a tested one; a review should not count that mutation.
17. **First responder after clicking a thumbnail (decision 1).** Keynote keeps focus in the navigator so Delete and the arrows keep working there; D2's Go to Slide makes the editor first responder after a jump because the person typed in a panel. Ruling: the panel keeps focus, and a hidden peek hands focus to the editor. Cost if wrong: one `makeFirstResponder` line.
18. **"The cursor moves to the start of slide 5" (02-slide-structure).** D2's `moveCursor(toSlide:)` places the caret at the end of the slide's first heading line, by a D2 ruling for Go to Slide, and it is the one function that guarantees the caret reads back as the same slide. Ruling: the same function; "start" means "into". Cost if wrong: one assertion.
19. **Deleting every slide.** The spec is silent. A deck with no slides has no slide 1, so the frontmatter would come out of hiding and the preview would show nothing. Ruling: refused, and Delete is disabled when the selection is the whole deck. Cost if wrong: a person who wants an empty deck deletes all but one slide and clears its text.
20. **A move between decks is two undo steps** (decision 2 accepts this): each `NSDocument` has its own undo manager, so the insert in the target and the delete in the source cannot share one. Cost if wrong: Cmd+Z in the target restores the target only.
21. **Fragment navigation in the renderer.** `webView.load` with a URL that differs only in its fragment is a same-document navigation (prototype finding 4); the page's `hashchange` listener changes the slide and the ready cycle publishes for it. If a WebKit release performs a full load instead, the ready signal still arrives, only slower. Cost if wrong: speed, never correctness; Task 6 names the fallback.
22. **The Slide menu has Generate Image, Insert Image and New Component items with no action.** The scenario says the menu "has" them; D2 shipped New Deck the same way pending D6. Cost if wrong: three lines when D6 wires them.
23. **The summary fetch on every typing pause** (review minor 19). `deckChanged` fetches `/api/presentation` after every PUT answer, which is the whole deck's JSON; the page does the same on every `update`. Declined for D3: driving it from the hub's `update` message and its changed-slide list would save one request per pause but leave the first fetch and every `reload` on the same path. The 200-slide benchmark shows the cost if it matters.
24. **D2's hidden-page rule and the thumbnail renderer.** Pull request 27 made a live page report ready while hidden; a print page does not (it waits for a paint). The renderer relies on that difference: a covered window stalls the queue and loads nothing. Cost if wrong: if the frontend ever drops `requirePaint` for print mode, the flat-image check and its three-attempt rule are the last defence.
25. **Two review minors declined.** Minor 13 (Command+Backspace) is recorded in item 5 rather than changed. Minor 19 is item 23. Every other review finding is addressed in the tasks.

Decided by the person, 2026-09-24: Slide > New Slide (the menu item) inserts a new slide at once using the last layout picked. The layout gallery opens from Slide > New Slide from Layout (or the menu's layout submenu) and from holding the toolbar's New Slide button. A plain click of the toolbar button inserts at once, like the menu item.

## Open questions

Each has the default this plan implements. Change the plan before running it if an answer differs.

1. **Recent-deck thumbnails on the welcome window.** D2 snapshots the preview on the first ready for slide 1 (`RecentThumbnailStore`), and D2's open question 11 said D3 may replace that with the thumbnail cache. Default: leave D2's store alone in D3; a later task can read slide 1's cached PNG instead.
2. **Custom theme CSS and the cache key.** `PublicConfig.customTheme` is a boolean, so editing a custom theme's CSS file does not change any thumbnail key. Default: accepted for D3. The clean fix is a small tap change (a content hash of the custom CSS in the public config), which would go on its own branch.
3. **Gallery previews.** Schematics drawn from the template text (default), or real renders of a generated twelve-slide deck through the thumbnail renderer, cached like the theme renders D6 adds.
4. **The person's runs.** `make -C desktop uitest` (four new UI tests, real drags and hover) and `make -C desktop bench` (the typing and preview benchmarks, and the new thumbnail cold pass and reopen) are compiled by the agents and run by the person, as in D2.

## Execution handoff

Plan complete and saved to `docs/superpowers/plans/2026-09-24-desktop-sidebar-thumbnails-slide-operations.md`. Execute with superpowers:subagent-driven-development, one fresh subagent per task, as the roadmap requires. Tasks 1 to 5 are the core package and run on a branch off `feat/desktop-app-shell` before pull request 29 merges; Task 6 onward runs after the branch is rebased onto `main`.
