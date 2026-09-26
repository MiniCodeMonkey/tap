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

    /// Holds the renderer's first snapshot until `release()`, then takes
    /// the real one. A test that hands the renderer new work "during a
    /// capture" waits for `isHolding` first: a real capture can finish
    /// between two polls, so starting the capture is no proof that it is
    /// still under way.
    @MainActor
    final class SnapshotGate {
        private(set) var isHolding = false
        private var waiter: CheckedContinuation<Void, Never>?
        private var released = false

        func hold() async {
            guard !released else { return }
            isHolding = true
            await withCheckedContinuation { waiter = $0 }
        }

        func release() {
            released = true
            isHolding = false
            waiter?.resume()
            waiter = nil
        }
    }

    func holdTheFirstSnapshot(of renderer: ThumbnailRenderer) -> SnapshotGate {
        let gate = SnapshotGate()
        let realSnapshot = renderer.snapshot
        renderer.snapshot = { webView, configuration in
            await gate.hold()
            return try await realSnapshot(webView, configuration)
        }
        return gate
    }

    func jobs(for summary: PresentationSummary) -> [ThumbnailRenderer.Job] {
        summary.slides.enumerated().map { index, slide in
            ThumbnailRenderer.Job(slideNumber: index + 1, key: ThumbnailKey(slideHash: slide.hash, themeSignature: summary.themeSignature))
        }
    }

    /// A renderer whose content process stops answering in the middle of a
    /// load gets a new web view and renders there. The process is stopped
    /// for real; the one seam is the ready wait, shortened.
    func testARendererWhoseProcessStopsAnsweringGetsANewWebView() async throws {
        let document = try await openDeck(try Fixtures.copyAppFixture())
        try await waitForBoxes(document, count: 4)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        var rendered: [Int] = []
        renderer.onImage = { job, _, _ in rendered.append(job.slideNumber) }
        let all = jobs(for: summary)
        renderer.setWork([all[0]], revision: summary.revision, visible: [1], current: 1)
        try await waitForRenderer(renderer, timeout: 30, "slide 1") { rendered.contains(1) }
        let stuck = renderer.webView
        let process = try XCTUnwrap(stuck.contentProcessIdentifier, "the renderer's content process identifier")
        kill(process, SIGSTOP)
        defer { kill(process, SIGKILL) }
        renderer.readyTimeoutForAttempt = { _ in 1 }

        renderer.setWork([all[1]], revision: summary.revision, visible: [2], current: 2)

        try await waitForRenderer(renderer, timeout: 30, "slide 2 in a new web view") { rendered.contains(2) }
        XCTAssertEqual(renderer.webViewReplacementCount, 1)
        XCTAssertTrue(renderer.webView !== stuck)
        XCTAssertNil(stuck.superview, "the old web view is gone from behind the editor")
        XCTAssertNotNil(renderer.webView.window, "the new web view is where the old one was")
    }

    /// A renderer whose content process ends renders the next slide in a
    /// new process, in the same web view.
    func testARendererWhoseProcessEndsRendersAgain() async throws {
        let document = try await openDeck(try Fixtures.copyAppFixture())
        try await waitForBoxes(document, count: 4)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        var rendered: [Int] = []
        renderer.onImage = { job, _, _ in rendered.append(job.slideNumber) }
        let all = jobs(for: summary)
        renderer.setWork([all[0]], revision: summary.revision, visible: [1], current: 1)
        try await waitForRenderer(renderer, timeout: 30, "slide 1") { rendered.contains(1) }
        let process = try XCTUnwrap(renderer.webView.contentProcessIdentifier, "the renderer's content process identifier")

        kill(process, SIGKILL)

        try await waitForRenderer(renderer, timeout: 10, "WebKit to report the ended process") { renderer.processTerminationCount == 1 }
        renderer.setWork([all[1]], revision: summary.revision, visible: [2], current: 2)
        try await waitForRenderer(renderer, timeout: 30, "slide 2 after the process ended") { rendered.contains(2) }
        XCTAssertEqual(renderer.webViewReplacementCount, 0)
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
        let cover = try await coverWindow(deckWindow)
        // A failed run must not leave the cover over every later test's window.
        defer { cover.remove() }

        var images: [Int] = []
        renderer.onImage = { job, _, _ in images.append(job.slideNumber) }
        renderer.setWork(jobs(for: summary), revision: summary.revision, visible: [1], current: 1)
        try await Task.sleep(nanoseconds: 1_500_000_000)
        XCTAssertEqual(renderer.navigationCount, 0, "the renderer does not even load the page while the window cannot paint")
        XCTAssertEqual(renderer.renderCount, 0)
        XCTAssertTrue(images.isEmpty)

        cover.window.orderOut(nil)
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

    /// The same job, unchanged, arriving while its slide is mid-capture must
    /// not requeue and render it a second time: `setWork` rebuilds its queue
    /// from every slide still in `jobs`, and until a finished capture clears
    /// its own entry, a hand-off during that capture would otherwise put the
    /// in-flight slide back in the queue for the pump loop to pick up again
    /// right after it delivers.
    func testASameJobArrivingDuringACaptureDoesNotRenderTwice() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        try await waitForBoxes(document, count: 1)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        let release = holdTheFirstSnapshot(of: renderer)
        var images: [Int] = []
        renderer.onImage = { job, _, _ in images.append(job.slideNumber) }
        let job = jobs(for: summary)
        renderer.setWork(job, revision: summary.revision, visible: [1], current: 1)
        try await waitUntil(timeout: 20, "the capture to reach its snapshot") { release.isHolding }
        // The same job, unchanged, arrives again while slide 1 is mid-capture.
        renderer.setWork(job, revision: summary.revision, visible: [1], current: 1)
        release.release()
        // A wrongly requeued slide renders again almost immediately, back to
        // back with the first delivery on the same loaded page, so the count
        // can skip straight from 0 to 2 between two polls: waiting for it to
        // equal 1 would miss that and time out instead of failing cleanly.
        try await waitUntil(timeout: 20, "the thumbnail") { !images.isEmpty }
        try await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertEqual(images, [1], "the in-flight job was not requeued, so it renders exactly once")
        XCTAssertEqual(renderer.renderCount, 1)
        XCTAssertFalse(renderer.isQueued(1))
    }

    /// A changed job for the same slide, arriving mid-capture, must still
    /// render again: the in-flight job's key no longer matches, so it is
    /// left in the renderer's work rather than cleared when the old capture
    /// finishes.
    func testAChangedJobArrivingDuringACaptureRendersAgain() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        try await waitForBoxes(document, count: 1)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        let release = holdTheFirstSnapshot(of: renderer)
        var deliveredKeys: [ThumbnailKey] = []
        renderer.onImage = { job, _, _ in deliveredKeys.append(job.key) }
        let originalJob = try XCTUnwrap(jobs(for: summary).first)
        renderer.setWork([originalJob], revision: summary.revision, visible: [1], current: 1)
        try await waitUntil(timeout: 20, "the capture to reach its snapshot") { release.isHolding }
        let changedKey = ThumbnailKey(slideHash: "changed-while-capturing", themeSignature: summary.themeSignature)
        let changedJob = ThumbnailRenderer.Job(slideNumber: 1, key: changedKey)
        renderer.setWork([changedJob], revision: summary.revision, visible: [1], current: 1)
        release.release()
        try await waitUntil(timeout: 20, "the changed job to render") { deliveredKeys.contains(changedKey) }
        XCTAssertGreaterThanOrEqual(renderer.renderCount, 1)
    }

    /// A slide whose real ready takes longer than the first attempt's
    /// window still renders, because the window grows on each retry rather
    /// than staying fixed at `readyTimeout` for as long as the deck is
    /// open. `readyTimeoutForAttempt` is shortened here so the test does
    /// not wait the real seconds the growing timeout implies: the first
    /// attempt's window is set far below what tap's real ready takes,
    /// forcing at least one retry, and the second attempt's window is
    /// plenty. If the renderer only ever used the fixed first-attempt
    /// timeout, the slide would never be delivered.
    func testASlideThatNeedsLongerThanTheFirstTimeoutIsStillDelivered() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        try await waitForBoxes(document, count: 1)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        renderer.readyTimeoutForAttempt = { attempt in attempt == 0 ? 0.05 : 5 }
        var images: [Int] = []
        renderer.onImage = { job, _, _ in images.append(job.slideNumber) }
        renderer.setWork(jobs(for: summary), revision: summary.revision, visible: [1], current: 1)
        try await waitUntil(timeout: 20, "the slide to be delivered after a retry with a longer window") { images == [1] }
        XCTAssertGreaterThan(renderer.navigationCount, 1, "the too-short first attempt required a reload and a second, longer attempt")
    }

    /// A ready for the right slide but a revision other than the one the
    /// work was set for is a stale page, not a finished capture: nothing is
    /// snapshotted or delivered for it.
    func testAReadyForAStaleRevisionIsNeverDelivered() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        try await waitForBoxes(document, count: 1)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        var images: [Int] = []
        renderer.onImage = { job, _, _ in images.append(job.slideNumber) }
        var reportedRevisions: [String] = []
        renderer.onPageRevision = { revision in reportedRevisions.append(revision) }
        renderer.setWork(jobs(for: summary), revision: "stale-revision-tap-never-serves", visible: [1], current: 1)
        try await waitUntil(timeout: 15, "the page's real revision to be reported as stale") { !reportedRevisions.isEmpty }
        try await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertTrue(images.isEmpty, "a ready for the wrong revision must not be captured or delivered")
        XCTAssertEqual(renderer.renderCount, 0)
        XCTAssertEqual(reportedRevisions.first, summary.revision)
    }

    /// A snapshot that never completes gives up after `snapshotTimeoutInterval`
    /// and counts as a failed attempt: the loop moves on to the other
    /// slides and tries the stuck one again, rather than waiting inside that
    /// render, with `running` set, for as long as the deck is open. The
    /// stuck snapshot completing late delivers nothing.
    func testASnapshotThatNeverCompletesIsGivenUpAndRetried() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        renderer.snapshotTimeoutInterval = 1
        let realSnapshot = renderer.snapshot
        var stuck: CheckedContinuation<NSImage, Error>?
        var snapshots = 0
        renderer.snapshot = { webView, configuration in
            snapshots += 1
            if snapshots == 1 {
                return try await withCheckedThrowingContinuation { stuck = $0 }
            }
            return try await realSnapshot(webView, configuration)
        }
        var images: [Int] = []
        renderer.onImage = { job, _, _ in images.append(job.slideNumber) }
        renderer.setWork(jobs(for: summary), revision: summary.revision, visible: [1], current: 1)
        try await waitForRenderer(renderer, timeout: 30, "seven thumbnails around the stuck snapshot") { images.count == 7 }
        XCTAssertNotEqual(images.first, 1, "the other slides render while slide 1's snapshot is given up on")
        XCTAssertEqual(images.sorted(), Array(1...7), "slide 1 renders on a later attempt")
        XCTAssertEqual(renderer.snapshotTimeoutCount, 1)

        stuck?.resume(returning: NSImage(size: NSSize(width: 320, height: 180)))
        stuck = nil
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(renderer.renderCount, 7, "a snapshot completing after its attempt was given up delivers nothing")
        XCTAssertEqual(renderer.phase, .idle, "the loop finished its work and stopped")
    }

    /// A failure reported for a navigation other than the waiting
    /// attempt's own, such as an earlier load a newer one replaced, must
    /// leave the wait alone. The report is delivered here the way WebKit
    /// delivers one, on the main queue, once the first attempt is waiting.
    func testAnotherNavigationsFailureDoesNotEndTheWait() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        try await waitForBoxes(document, count: 1)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        // A real navigation that is not the renderer's: WebKit hands out
        // navigations only from a load.
        let otherWebView = WKWebView()
        let otherNavigation = try XCTUnwrap(otherWebView.load(URLRequest(url: try XCTUnwrap(URL(string: "about:blank")))))
        var attemptsSeen: [Int] = []
        renderer.readyTimeoutForAttempt = { [weak renderer] attempt in
            attemptsSeen.append(attempt)
            if attempt == 0 {
                DispatchQueue.main.async {
                    guard let renderer else { return }
                    renderer.webView(renderer.webView, didFailProvisionalNavigation: otherNavigation, withError: URLError(.cancelled))
                }
            }
            return 15
        }
        var images: [Int] = []
        renderer.onImage = { job, _, _ in images.append(job.slideNumber) }
        renderer.setWork(jobs(for: summary), revision: summary.revision, visible: [1], current: 1)
        try await waitForRenderer(renderer, timeout: 30, "the slide") { images == [1] }
        XCTAssertEqual(attemptsSeen, [0], "the first attempt waited for its own page's ready")
        XCTAssertEqual(renderer.navigationCount, 1, "no reload: the other navigation's failure did not count against this one")
    }

    /// The waiting attempt's own navigation failing still ends the wait at
    /// once, rather than at the ready timeout.
    func testTheWaitingNavigationsFailureEndsTheWait() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        try await waitForBoxes(document, count: 1)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        var attemptsSeen: [Int] = []
        renderer.readyTimeoutForAttempt = { [weak renderer] attempt in
            attemptsSeen.append(attempt)
            if attempt == 0 {
                DispatchQueue.main.async {
                    guard let renderer else { return }
                    renderer.webView(renderer.webView, didFail: renderer.currentNavigation, withError: URLError(.cancelled))
                }
            }
            return 15
        }
        var images: [Int] = []
        renderer.onImage = { job, _, _ in images.append(job.slideNumber) }
        renderer.setWork(jobs(for: summary), revision: summary.revision, visible: [1], current: 1)
        try await waitForRenderer(renderer, timeout: 30, "the slide") { images == [1] }
        XCTAssertEqual(attemptsSeen, [0, 1], "the failure ended the first attempt, and a second one rendered")
        XCTAssertEqual(renderer.navigationCount, 2)
    }

    /// The failure count belongs to the job it was counted for, the same
    /// rule as the flat-capture count: a slide edited after two failed
    /// attempts starts its new content's first attempt at the first-attempt
    /// timeout, not the old content's already-grown one.
    func testAFailureCountResetsWhenTheJobChanges() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        try await waitForBoxes(document, count: 1)
        let (renderer, _, summary) = try await makeRenderer(for: document)
        var attemptsSeen: [Int] = []
        var switchedToB = false
        renderer.readyTimeoutForAttempt = { attempt in
            attemptsSeen.append(attempt)
            // Two short windows fail key A's job on purpose, growing its
            // attempt count; once the test swaps in key B, a generous
            // window lets the real ready land normally. 0.05 s is the
            // value proven too short for this fixture's real ready in the
            // growing-timeout test above.
            return switchedToB ? 5 : 0.05
        }
        let keyA = ThumbnailKey(slideHash: "hash-a", themeSignature: summary.themeSignature)
        renderer.setWork([ThumbnailRenderer.Job(slideNumber: 1, key: keyA)], revision: summary.revision, visible: [1], current: 1)
        try await waitUntil(timeout: 15, "key A to fail twice") { attemptsSeen.count >= 2 }
        XCTAssertEqual(Array(attemptsSeen.prefix(2)), [0, 1], "key A's own two failures climbed past the first-attempt window")

        // The content changes before key A's third, 20 s-windowed attempt
        // ever happens.
        let keyB = ThumbnailKey(slideHash: "hash-b", themeSignature: summary.themeSignature)
        switchedToB = true
        let attemptsBeforeB = attemptsSeen.count
        renderer.setWork([ThumbnailRenderer.Job(slideNumber: 1, key: keyB)], revision: summary.revision, visible: [1], current: 1)
        try await waitUntil(timeout: 15, "key B's first attempt") { attemptsSeen.count > attemptsBeforeB }
        XCTAssertEqual(attemptsSeen.last, 0, "an edited slide's first attempt must use the first-attempt timeout, not the old content's inherited failure count")
    }
}
