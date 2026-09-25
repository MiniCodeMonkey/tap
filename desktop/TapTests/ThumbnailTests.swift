import WebKit
import XCTest
@testable import Tap

final class ThumbnailTests: HostedTestCase {
    func waitForThumbnails(_ document: DeckDocument, count: Int, timeout: TimeInterval = 40) async throws {
        let controller = try XCTUnwrap(document.sessionController)
        let panel = controller.slidePanel
        try await waitForRenderer(controller.thumbnails.renderer, of: controller, timeout: timeout, "\(count) thumbnails") {
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
        try await waitForRenderer(controller.thumbnails.renderer, of: controller, timeout: 20, "the first new render") { controller.thumbnails.renderer.renderCount == rendersBefore + 1 }
        XCTAssertEqual(controller.thumbnails.renderer.lastRenderedSlide, 3, "the current slide renders before the other changed one")
        try await waitForRenderer(controller.thumbnails.renderer, of: controller, timeout: 20, "thumbnail 3 to change") { controller.slidePanel.image(forSlide: 3)?.tiffRepresentation != before }
        try await waitForRenderer(controller.thumbnails.renderer, of: controller, timeout: 20, "the second render") { controller.thumbnails.renderer.renderCount == rendersBefore + 2 }
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

    func testCursorMoveReprioritizesTheQueue() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        let controller = try XCTUnwrap(document.sessionController)
        // Frozen before tap is even ready: nothing captures until the
        // cursor has already moved, so the render order that follows can
        // only be explained by whichever slide the queue currently favors.
        let productionCanPaint = controller.thumbnails.renderer.canPaint
        controller.thumbnails.renderer.canPaint = { false }
        _ = try await waitForRunningTap(document)
        try await waitForBoxes(document, count: 7)
        try await waitUntil(timeout: 20, "the queue to fill with every slide") {
            (1...7).allSatisfy { controller.thumbnails.renderer.isQueued($0) }
        }
        // The cursor starts on slide 1 (deck order). Move it to slide 3,
        // already on screen in the panel: selecting it fires no bounds
        // change notification, so only a direct reprioritize on the
        // cursor move itself can move slide 3 ahead of slide 1.
        controller.editor.moveCursor(toSlide: 2)
        try await waitUntil(timeout: 5, "the cursor to reach slide 3") { controller.currentSlideNumber == 3 }
        // The window paints before the gate reopens, and the gate that
        // reopens is the production one: a load the renderer starts is one
        // it would start for a person, on a window already painting.
        try await waitForPreview(document, slide: 3, timeout: 30)
        controller.thumbnails.renderer.canPaint = productionCanPaint
        try await waitForRenderer(controller.thumbnails.renderer, of: controller, timeout: 20, "the first render") { controller.thumbnails.renderer.renderCount == 1 }
        XCTAssertEqual(controller.thumbnails.renderer.lastRenderedSlide, 3, "the new cursor slide renders first")
    }

    func testScrollReprioritizesTheQueue() async throws {
        // A deck with enough slides that the panel cannot show them all at
        // once: seven slides fit in the panel without scrolling, which
        // would let the whole queue already read as "visible" before any
        // scroll and hide whether the wiring under test does anything.
        let folder = try Fixtures.temporaryFolder()
        let deckURL = folder.appendingPathComponent("many.md")
        let content = (1...30).map { "# Slide \($0)" }.joined(separator: "\n\n---\n\n")
        try content.write(to: deckURL, atomically: true, encoding: .utf8)

        let document = try await openDeck(deckURL)
        let controller = try XCTUnwrap(document.sessionController)
        let productionCanPaint = controller.thumbnails.renderer.canPaint
        controller.thumbnails.renderer.canPaint = { false }
        _ = try await waitForRunningTap(document)
        try await waitForBoxes(document, count: 30)
        try await waitUntil(timeout: 30, "the queue to fill with every slide") {
            (1...30).allSatisfy { controller.thumbnails.renderer.isQueued($0) }
        }
        XCTAssertFalse(controller.slidePanel.visibleNumbers.contains(30), "the last slide should not already be visible, or the scroll below proves nothing")
        // Paused again the instant the current slide finishes, before the
        // loop can start a second job, so the scroll below is the only
        // thing that can change which slide renders next.
        var pausedAfterFirst = false
        let originalOnImage = controller.thumbnails.renderer.onImage
        // Weakly captured, and restored below: a strong `controller` here,
        // plus a `canPaint` left open, would keep this renderer alive and
        // painting slides during later tests.
        controller.thumbnails.renderer.onImage = { [weak controller] job, image, png in
            if !pausedAfterFirst {
                pausedAfterFirst = true
                controller?.thumbnails.renderer.canPaint = { false }
            }
            originalOnImage?(job, image, png)
        }
        defer {
            controller.thumbnails.renderer.onImage = originalOnImage
            controller.thumbnails.renderer.canPaint = { false }
        }
        // The gate reopens to the production one once the window paints.
        try await waitForPreview(document, slide: 1, timeout: 30)
        controller.thumbnails.renderer.canPaint = productionCanPaint
        try await waitForRenderer(controller.thumbnails.renderer, of: controller, timeout: 20, "the current slide to render first") { controller.thumbnails.renderer.renderCount == 1 }
        XCTAssertEqual(controller.thumbnails.renderer.lastRenderedSlide, 1, "deck order with no edits starts on slide 1")

        // Scroll the panel to the last slide by moving the clip view, the
        // same underlying call a real scroll makes. A hosted, unfocused
        // window's clip view moves its bounds correctly (confirmed:
        // visibleNumbers reflects the new range right after this) but does
        // not reliably post AppKit's own bounds-change notification the
        // way the window server's real scroll machinery does, the same gap
        // HostedTestCase already documents for occlusion notifications
        // (see `occlusionStateOfPreviewWindow`). Posting it here exercises
        // exactly the panel's own observer and the wiring under test,
        // standing in only for the AppKit-owned notification delivery a
        // real scroll gesture would have produced.
        let clipView = controller.slidePanel.scrollView.contentView
        if let documentView = controller.slidePanel.scrollView.documentView {
            let target = NSPoint(x: 0, y: max(0, documentView.bounds.height - clipView.bounds.height))
            clipView.scroll(to: target)
            controller.slidePanel.scrollView.reflectScrolledClipView(clipView)
        }
        try await waitUntil(timeout: 5, "the panel to scroll to the last slide") { controller.slidePanel.visibleNumbers.contains(30) }
        NotificationCenter.default.post(name: NSView.boundsDidChangeNotification, object: clipView)

        controller.thumbnails.renderer.canPaint = productionCanPaint
        try await waitForRenderer(controller.thumbnails.renderer, of: controller, timeout: 20, "the second render") { controller.thumbnails.renderer.renderCount == 2 }
        XCTAssertNotEqual(controller.thumbnails.renderer.lastRenderedSlide, 2, "deck order alone would render slide 2 next")
        XCTAssertGreaterThan(controller.thumbnails.renderer.lastRenderedSlide ?? 0, 20,
                              "scrolling to the last slide should move it, or a near neighbour, to the front of the queue")
    }

    func testTypingPauseDelaysRendering() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        let controller = try XCTUnwrap(document.sessionController)
        // Frozen on the paint gate until an edit is in place, so the very
        // first job the renderer would work on is one held back by the
        // typing-pause gate alone, not by anything else.
        let productionCanPaint = controller.thumbnails.renderer.canPaint
        controller.thumbnails.renderer.canPaint = { false }
        _ = try await waitForRunningTap(document)
        try await waitForBoxes(document, count: 7)
        try await waitUntil(timeout: 20, "the queue to fill with every slide") {
            (1...7).allSatisfy { controller.thumbnails.renderer.isQueued($0) }
        }
        // The window paints, so the production gate is open by the time the
        // edit lands: the typing pause is then the only thing holding the
        // renderer back.
        try await waitForPreview(document, slide: 1, timeout: 30)
        try await waitUntil(timeout: 10, "the production paint gate to open") { productionCanPaint() }
        controller.editor.setSelectedRange(NSRange(location: 0, length: 0))
        controller.editor.insertText(" ", replacementRange: NSRange(location: NSNotFound, length: 0))
        XCTAssertTrue(controller.thumbnails.renderer.isPaused(), "an edit just now should hold the renderer back")
        controller.thumbnails.renderer.canPaint = productionCanPaint
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(controller.thumbnails.renderer.renderCount, 0, "still within the 0.5 s pause window, nothing should have rendered despite a full queue")
        try await waitForRenderer(controller.thumbnails.renderer, of: controller, timeout: 20, "a render once the pause window passes") { controller.thumbnails.renderer.renderCount > 0 }
    }

    func testDuplicatedSlidesShareOneThumbnail() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("seven-slides.md"))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForThumbnails(document, count: 7)
        let renders = controller.thumbnails.renderer.renderCount
        XCTAssertEqual(controller.perform(.duplicate(numbers: [3])), .applied)
        try await waitForBoxes(document, count: 8)
        try await waitForThumbnails(document, count: 8)
        XCTAssertEqual(controller.thumbnails.key(forSlide: 3), controller.thumbnails.key(forSlide: 4))
        XCTAssertEqual(controller.thumbnails.renderer.renderCount, renders, "a copy shares the original's image")
    }
}
