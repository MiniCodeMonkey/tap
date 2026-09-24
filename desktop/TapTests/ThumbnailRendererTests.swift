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

    /// The flat-capture count belongs to the job it was counted for: a slide
    /// edited after two flat captures starts its new content's count at
    /// zero rather than being one flat capture away from being cached blank.
    func testAFlatCaptureCountResetsWhenTheJobChanges() async throws {
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

        let keyA = ThumbnailKey(slideHash: "hash-a", themeSignature: summary.themeSignature)
        renderer.setWork([ThumbnailRenderer.Job(slideNumber: 1, key: keyA)], revision: summary.revision, visible: [1], current: 1)
        try await waitUntil(timeout: 20, "two flat captures for key A") { snapshots >= 2 }
        XCTAssertTrue(images.isEmpty, "two flat captures alone are not yet accepted as blank")

        // The content changes (a different hash) before the third, deciding
        // capture for key A ever happens.
        let keyB = ThumbnailKey(slideHash: "hash-b", themeSignature: summary.themeSignature)
        renderer.setWork([ThumbnailRenderer.Job(slideNumber: 1, key: keyB)], revision: summary.revision, visible: [1], current: 1)
        let snapshotsBeforeB = snapshots
        try await waitUntil(timeout: 20, "one more flat capture, now for key B") { snapshots > snapshotsBeforeB }
        // With the bug, this single flat capture for the new key is treated
        // as the job's third in a row (two inherited from key A) and gets
        // delivered as blank right away.
        try await Task.sleep(nanoseconds: 800_000_000)
        XCTAssertTrue(images.isEmpty, "one flat capture of a freshly changed job must not be accepted as blank")
        XCTAssertEqual(renderer.renderCount, 0)
    }
}
