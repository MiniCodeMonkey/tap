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
