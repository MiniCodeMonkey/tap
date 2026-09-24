import XCTest
@testable import Tap

final class WelcomeTests: HostedTestCase {
    var appDelegate: AppDelegate { NSApp.delegate as! AppDelegate }
    var welcome: WelcomeWindowController { WelcomeWindowController.shared }

    func testWelcomeWindow() async throws {
        appDelegate.showWelcomeIfNoDecks()
        XCTAssertTrue(welcome.window?.isVisible ?? false, "no deck is open")
        XCTAssertEqual(welcome.newDeckButton.title, "New Deck…")
        XCTAssertEqual(welcome.openButton.title, "Open…")

        // Opening a deck closes the welcome window and records a thumbnail of slide 1.
        let deck = try Fixtures.copyAppFixture()
        let document = try await openDeckAndWaitForPreview(deck)
        XCTAssertFalse(welcome.window?.isVisible ?? false)
        try await waitUntil(timeout: 10, "the recent thumbnail") { AppEnvironment.shared.recentThumbnailStore.imageData(for: deck) != nil }

        document.close()
        try await waitUntil(timeout: 5, "the welcome window") { self.welcome.window?.isVisible ?? false }
        let row = try XCTUnwrap(welcome.recentURLs.firstIndex { FilePaths.same($0, deck) }, "the deck is a recent deck")
        XCTAssertNotNil(welcome.thumbnail(forRow: row))
    }

    func testLastWindowClosed() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        XCTAssertFalse(welcome.window?.isVisible ?? false)
        document.windowControllers.first?.window?.performClose(nil)
        try await waitUntil(timeout: 5, "the welcome window") { self.welcome.window?.isVisible ?? false }
        XCTAssertFalse(appDelegate.applicationShouldTerminateAfterLastWindowClosed(NSApp), "the app keeps running")
    }

    /// A snapshot taken while the preview was hidden or unpainted comes back
    /// blank: a page of one flat colour. This proves the recorded thumbnail
    /// is a real render of slide 1, not that blank capture, by sampling
    /// pixels at the corners and the center of the saved PNG and checking
    /// they are not all the same colour.
    func testRecordedThumbnailIsNotABlankCapture() async throws {
        let deck = try Fixtures.copyAppFixture()
        _ = try await openDeckAndWaitForPreview(deck)
        try await waitUntil(timeout: 10, "the recent thumbnail") { AppEnvironment.shared.recentThumbnailStore.imageData(for: deck) != nil }
        try assertRecordedThumbnailIsNotBlank(for: deck)
    }

    /// A preview collapsed out of the split view is
    /// `webView.isHiddenOrHasHiddenAncestor`, so a ready signal that arrives
    /// while it is hidden must not be captured. A later ready for slide 1,
    /// once the pane is shown again, does get recorded; that second ready is
    /// delivered directly (rather than waited for organically) because tap
    /// only ever reports ready again when the slide, step or revision
    /// actually changes, which merely showing the pane does not do on its
    /// own.
    func testHiddenPreviewDoesNotRecordAThumbnailUntilShown() async throws {
        let deck = try Fixtures.copyAppFixture()
        let document = try await openDeck(deck)
        let windowController = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        windowController.splitViewController.setPreviewHidden(true)
        _ = try await waitForRunningTap(document)
        // Long enough for a ready signal to have arrived while hidden, on a
        // deck this small.
        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertNil(AppEnvironment.shared.recentThumbnailStore.imageData(for: deck), "a hidden preview must not be captured")

        windowController.splitViewController.setPreviewHidden(false)
        // Lets AppKit's own collapse/uncollapse propagate to the web view
        // before the next ready arrives.
        try await Task.sleep(nanoseconds: 300_000_000)
        let controller = try XCTUnwrap(document.sessionController)
        controller.previewViewController.onReady?(ReadyPayload(revision: "shown-again", slide: 1, step: 0))
        try await waitUntil(timeout: 10, "the recent thumbnail") { AppEnvironment.shared.recentThumbnailStore.imageData(for: deck) != nil }
        try assertRecordedThumbnailIsNotBlank(for: deck)
    }

    /// A live page that ran out of settle rounds still posts ready with
    /// `settled: false` rather than leaving the app waiting forever, but its
    /// slide 1 may not actually have painted. That ready must not be
    /// captured even while the preview is visible; a later, settled ready
    /// for slide 1 does get recorded. The preview's own `onReady` wiring is
    /// swapped out for the duration, so the genuine page (which settles
    /// normally and would otherwise race this test's own fabricated
    /// readies) cannot itself record a thumbnail; only the readies
    /// delivered directly below reach `previewDidRender`.
    func testUnsettledReadyDoesNotRecordAThumbnailUntilSettled() async throws {
        let deck = try Fixtures.copyAppFixture()
        let document = try await openDeck(deck)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController

        let appOnReady = try XCTUnwrap(preview.onReady)
        preview.onReady = nil
        defer { preview.onReady = appOnReady }

        appOnReady(ReadyPayload(revision: "unsettled", slide: 1, step: 0, settled: false))
        // Longer than recentThumbnailSettleInterval, so a missing guard
        // would have captured by now.
        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertNil(AppEnvironment.shared.recentThumbnailStore.imageData(for: deck), "an unsettled ready must not be captured")

        appOnReady(ReadyPayload(revision: "settled-after", slide: 1, step: 0))
        try await waitUntil(timeout: 10, "the recent thumbnail") { AppEnvironment.shared.recentThumbnailStore.imageData(for: deck) != nil }
        try assertRecordedThumbnailIsNotBlank(for: deck)
    }

    /// `isVisible` alone does not tell a window on screen apart from one
    /// completely covered by another opaque window: both read true. A
    /// second, borderless, opaque window placed exactly over the deck
    /// window is what actually clears the occlusion state's visible bit
    /// while `isVisible` stays true, so this is the one scenario that
    /// exercises `occlusionState.contains(.visible)` on its own, independent
    /// of `isVisible`. A ready signal that arrives while covered must not be
    /// captured; removing the cover and delivering a later ready for slide 1
    /// directly (tap does not refire ready on its own for an unchanged
    /// slide) does get recorded, and is a real render, not a blank capture.
    func testOccludedPreviewDoesNotRecordAThumbnailUntilVisible() async throws {
        let deck = try Fixtures.copyAppFixture()
        let document = try await openDeck(deck)
        let window = try XCTUnwrap(document.windowControllers.first?.window)

        let cover = try await coverWindow(window)
        defer { cover.remove() }

        _ = try await waitForRunningTap(document)
        // Long enough for a ready signal to have arrived while covered, on a
        // deck this small.
        try await Task.sleep(nanoseconds: 1_000_000_000)
        // A visible report while covered lets the page paint and makes the
        // assertion below say nothing about the product, whichever way it
        // comes out, so the run fails naming the report instead.
        guard cover.visibleReportsWhileCovered == 0 else {
            XCTFail("the window server reported the covered deck window visible \(cover.visibleReportsWhileCovered) time(s) while the cover was up, so this run cannot tell whether an occluded window is captured")
            return
        }
        XCTAssertNil(AppEnvironment.shared.recentThumbnailStore.imageData(for: deck), "an occluded window must not be captured")

        cover.window.orderOut(nil)
        try await waitUntil(timeout: 5, "the deck window to report visible again") { window.occlusionState.contains(.visible) }
        let controller = try XCTUnwrap(document.sessionController)
        controller.previewViewController.onReady?(ReadyPayload(revision: "visible-again", slide: 1, step: 0))
        try await waitUntil(timeout: 10, "the recent thumbnail") { AppEnvironment.shared.recentThumbnailStore.imageData(for: deck) != nil }
        try assertRecordedThumbnailIsNotBlank(for: deck)
    }

    /// A deck opened behind another window renders slide 1 with no
    /// thumbnail. Uncovering the window records one from the ready the
    /// page already sent: tap renders nothing new and no ready arrives.
    func testCoveredDeckRecordsItsThumbnailOnceUncovered() async throws {
        let deck = try Fixtures.copyAppFixture()
        let document = try await openDeck(deck)
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        let cover = try await coverWindow(window)
        defer { cover.remove() }

        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController
        let deadline = Date().addingTimeInterval(30)
        while preview.lastReady?.slide != 1 {
            if Date() > deadline {
                // Records whether the page ever saw itself hidden behind the
                // cover, which separates a page whose paint wait never ended
                // from one that never received the deck.
                let inThePage = await preview.pageValue(Self.pageStateScript)
                XCTFail("timed out waiting for slide 1 ready behind the cover. "
                        + "lastReady=\(String(describing: preview.lastReady)) page=\(inThePage) "
                        + "occlusion=\(window.occlusionState.rawValue) "
                        + "visibleReportsWhileCovered=\(cover.visibleReportsWhileCovered) "
                        + "socket=\(controller.socket == nil ? "none" : "open")")
                return
            }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
        // Long enough for a capture to have finished, had one started.
        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertNil(AppEnvironment.shared.recentThumbnailStore.imageData(for: deck), "a covered window must not be captured")
        let readyWhileCovered = preview.lastReady

        var readiesAfterUncovering = 0
        let onReady = preview.onReady
        preview.onReady = { payload in
            readiesAfterUncovering += 1
            onReady?(payload)
        }
        cover.window.orderOut(nil)
        try await waitUntil(timeout: 10, "the recent thumbnail") { AppEnvironment.shared.recentThumbnailStore.imageData(for: deck) != nil }
        XCTAssertEqual(readiesAfterUncovering, 0, "the thumbnail comes from the ready already received")
        XCTAssertEqual(preview.lastReady, readyWhileCovered)
        try assertRecordedThumbnailIsNotBlank(for: deck)
    }

    /// The window server reports its own occlusion changes, and cannot be
    /// made to report a covered window visible for a moment on demand. These
    /// two tests leave the deck window really on screen, so the page paints
    /// and a snapshot is a real image, and drive what the controller reads
    /// through `occlusionStateOfPreviewWindow`, posting the occlusion
    /// notification the window server would.
    func testAVisibleReportShorterThanTheSettleIntervalRecordsNoThumbnail() async throws {
        let deck = try Fixtures.copyAppFixture()
        let document = try await openDeck(deck)
        let controller = try XCTUnwrap(document.sessionController)
        let reported = ReportedOcclusion(controller: controller)
        var snapshotsStarted = 0
        let realSnapshot = controller.takePreviewSnapshot
        controller.takePreviewSnapshot = { configuration, completion in
            snapshotsStarted += 1
            realSnapshot(configuration, completion)
        }
        try await waitForPreview(document, slide: 1, timeout: 30)
        let window = try XCTUnwrap(controller.previewViewController.webView.window)
        XCTAssertTrue(window.occlusionState.contains(.visible), "the page paints on a window really on screen")

        reported.report(visible: true, for: window)
        try await Task.sleep(nanoseconds: 300_000_000)
        reported.report(visible: false, for: window)
        // Well past the settle interval and any snapshot it could have
        // started.
        try await Task.sleep(nanoseconds: 800_000_000)
        XCTAssertEqual(snapshotsStarted, 0, "a visible report shorter than the settle interval must not start a capture")
        XCTAssertNil(AppEnvironment.shared.recentThumbnailStore.imageData(for: deck), "a visible report shorter than the settle interval must not be captured")

        reported.report(visible: true, for: window)
        try await waitUntil(timeout: 10, "the recent thumbnail once visible for the settle interval") { AppEnvironment.shared.recentThumbnailStore.imageData(for: deck) != nil }
        try assertRecordedThumbnailIsNotBlank(for: deck)
    }

    /// See the test above for why the occlusion state is driven directly.
    /// The window is reported covered the moment the snapshot starts, so the
    /// snapshot completes on a real, painted image of a window the
    /// controller must treat as covered.
    func testASnapshotOfAWindowCoveredDuringTheCaptureIsDiscarded() async throws {
        let deck = try Fixtures.copyAppFixture()
        let document = try await openDeck(deck)
        let controller = try XCTUnwrap(document.sessionController)
        let reported = ReportedOcclusion(controller: controller)
        var snapshotsStarted = 0
        var coverDuringSnapshot = true
        let realSnapshot = controller.takePreviewSnapshot
        controller.takePreviewSnapshot = { configuration, completion in
            snapshotsStarted += 1
            if coverDuringSnapshot, let window = controller.previewViewController.webView.window {
                reported.report(visible: false, for: window)
            }
            realSnapshot(configuration, completion)
        }
        try await waitForPreview(document, slide: 1, timeout: 30)
        let window = try XCTUnwrap(controller.previewViewController.webView.window)

        reported.report(visible: true, for: window)
        try await waitUntil(timeout: 5, "a capture to start") { snapshotsStarted == 1 }
        // Long enough for the snapshot to have completed and been saved,
        // had it been kept.
        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertNil(AppEnvironment.shared.recentThumbnailStore.imageData(for: deck), "a snapshot of a window covered while it was taken must be discarded")

        coverDuringSnapshot = false
        reported.report(visible: true, for: window)
        try await waitUntil(timeout: 10, "the recent thumbnail once visible again") { AppEnvironment.shared.recentThumbnailStore.imageData(for: deck) != nil }
        try assertRecordedThumbnailIsNotBlank(for: deck)
    }

    /// What the controller reads as the preview window's occlusion state,
    /// starting covered, and the notification that tells it to read again.
    @MainActor
    private final class ReportedOcclusion {
        private var state: NSWindow.OcclusionState = []

        init(controller: DeckSessionController) {
            controller.occlusionStateOfPreviewWindow = { _ in self.state }
        }

        func report(visible: Bool, for window: NSWindow) {
            state = visible ? .visible : []
            NotificationCenter.default.post(name: NSWindow.didChangeOcclusionStateNotification, object: window)
        }
    }

    /// Samples the corners and the center of a recorded thumbnail and fails
    /// if they are all the same colour, the signature of a blank capture.
    private func assertRecordedThumbnailIsNotBlank(for deck: URL, file: StaticString = #filePath, line: UInt = #line) throws {
        let data = try XCTUnwrap(AppEnvironment.shared.recentThumbnailStore.imageData(for: deck), file: file, line: line)
        let image = try XCTUnwrap(NSBitmapImageRep(data: data), file: file, line: line)
        let width = image.pixelsWide
        let height = image.pixelsHigh
        var colors = Set<[Int]>()
        let points = [(0, 0), (width - 1, 0), (0, height - 1), (width - 1, height - 1), (width / 2, height / 2)]
        for (x, y) in points {
            guard let color = image.colorAt(x: x, y: y) else { continue }
            colors.insert([Int(color.redComponent * 255), Int(color.greenComponent * 255), Int(color.blueComponent * 255)])
        }
        XCTAssertGreaterThan(colors.count, 1, "a blank capture reads back as a single flat colour", file: file, line: line)
    }
}
