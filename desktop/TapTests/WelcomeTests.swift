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
    /// for slide 1 does get recorded. The genuine page's own ready still
    /// lands in `preview.lastReady` before `onReady` even runs (the message
    /// handler sets it unconditionally), so swapping out `onReady` alone
    /// does not stop it: an occlusion report later replays `lastReady`
    /// through `previewDidRender` regardless. The `tapReady` script handler
    /// is removed instead, before the page can post anything, so the
    /// genuine ready never reaches the preview at all and only the readies
    /// delivered directly below reach `previewDidRender`.
    func testUnsettledReadyDoesNotRecordAThumbnailUntilSettled() async throws {
        let deck = try Fixtures.copyAppFixture()
        let document = try await openDeck(deck)
        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController

        XCTAssertEqual(preview.pageLoadCount, 0, "the page has not loaded yet, so removing its ready handler now cannot race a message already in flight")
        preview.webView.configuration.userContentController.removeScriptMessageHandler(forName: "tapReady")

        _ = try await waitForRunningTap(document)

        let appOnReady = try XCTUnwrap(preview.onReady)
        preview.onReady = nil
        defer { preview.onReady = appOnReady }

        appOnReady(ReadyPayload(revision: "unsettled", slide: 1, step: 0, settled: false))
        // Longer than recentThumbnailSettleInterval, so a missing guard
        // would have captured by now.
        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertNil(preview.lastReady, "the genuine page's ready reached the preview, so this run cannot isolate the fabricated ready")
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
        // Counts snapshots started, completed and flat through the same
        // seam production wraps, so a timeout below names which of the
        // three the capture stalled at rather than leaving it a guess.
        var snapshotsStarted = 0
        var snapshotsCompleted = 0
        var snapshotsFlat = 0
        let realSnapshot = controller.takePreviewSnapshot
        controller.takePreviewSnapshot = { configuration, completion in
            snapshotsStarted += 1
            realSnapshot(configuration) { image, error in
                snapshotsCompleted += 1
                if let image, Self.isFlatSnapshot(image) { snapshotsFlat += 1 }
                completion(image, error)
            }
        }
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
        let thumbnailDeadline = Date().addingTimeInterval(10)
        while AppEnvironment.shared.recentThumbnailStore.imageData(for: deck) == nil {
            if Date() > thumbnailDeadline {
                // 3 started, 3 completed and 3 flat means the flat retries
                // gave up too soon. More started than completed means a
                // snapshot hung and the timeout did not clear it. 0 started
                // means no trigger ever arrived after uncovering.
                XCTFail("timed out waiting for the recent thumbnail. "
                        + "snapshotsStarted=\(snapshotsStarted) snapshotsCompleted=\(snapshotsCompleted) snapshotsFlat=\(snapshotsFlat) "
                        + "lastReady=\(String(describing: preview.lastReady)) "
                        + "occlusion=\(window.occlusionState.rawValue)")
                return
            }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
        XCTAssertEqual(readiesAfterUncovering, 0, "the thumbnail comes from the ready already received")
        XCTAssertEqual(preview.lastReady, readyWhileCovered)
        try assertRecordedThumbnailIsNotBlank(for: deck)
    }

    /// A snapshot whose completion never runs (a hung `takeSnapshot`) must
    /// not block the deck's thumbnail forever: `isCapturingRecentThumbnail`
    /// has to clear on its own so a later attempt can still record it.
    func testASnapshotThatNeverCompletesDoesNotBlockTheNextCapture() async throws {
        let deck = try Fixtures.copyAppFixture()
        let document = try await openDeck(deck)
        let controller = try XCTUnwrap(document.sessionController)
        let reported = ReportedOcclusion(controller: controller)
        var hangTheNextSnapshot = true
        let realSnapshot = controller.takePreviewSnapshot
        controller.takePreviewSnapshot = { configuration, completion in
            if hangTheNextSnapshot {
                hangTheNextSnapshot = false
                // Never calls completion: the production timeout is what
                // has to move this forward, not this seam.
                return
            }
            realSnapshot(configuration, completion)
        }
        try await waitForPreview(document, slide: 1, timeout: 30)
        let window = try XCTUnwrap(controller.previewViewController.webView.window)

        reported.report(visible: true, for: window)
        try await waitUntil(timeout: 10, "the recent thumbnail once the hung snapshot times out") { AppEnvironment.shared.recentThumbnailStore.imageData(for: deck) != nil }
        try assertRecordedThumbnailIsNotBlank(for: deck)
    }

    /// A snapshot that keeps coming back flat while the preview stays
    /// visible must keep being retried, not give up for good after a fixed
    /// number of attempts.
    func testFlatSnapshotsKeepBeingRetriedWhileVisible() async throws {
        let deck = try Fixtures.copyAppFixture()
        let document = try await openDeck(deck)
        let controller = try XCTUnwrap(document.sessionController)
        let reported = ReportedOcclusion(controller: controller)
        var snapshotsStarted = 0
        // More flat snapshots than the old fixed budget of 3 attempts, so a
        // regression back to giving up after 3 would leave this deck
        // unrecorded.
        let flatSnapshotsBeforeARealOne = 6
        let realSnapshot = controller.takePreviewSnapshot
        controller.takePreviewSnapshot = { configuration, completion in
            snapshotsStarted += 1
            if snapshotsStarted <= flatSnapshotsBeforeARealOne {
                let flat = NSImage(size: NSSize(width: 320, height: 180))
                flat.lockFocus()
                NSColor.white.setFill()
                NSRect(x: 0, y: 0, width: 320, height: 180).fill()
                flat.unlockFocus()
                completion(flat, nil)
                return
            }
            realSnapshot(configuration, completion)
        }
        try await waitForPreview(document, slide: 1, timeout: 30)
        let window = try XCTUnwrap(controller.previewViewController.webView.window)

        reported.report(visible: true, for: window)
        try await waitUntil(timeout: 10, "the recent thumbnail once the flat retries reach a real snapshot") { AppEnvironment.shared.recentThumbnailStore.imageData(for: deck) != nil }
        XCTAssertGreaterThan(snapshotsStarted, flatSnapshotsBeforeARealOne, "the capture must keep retrying past the old fixed attempt count")
        try assertRecordedThumbnailIsNotBlank(for: deck)
    }


    /// A crude flat check for the diagnostics above: samples a grid of
    /// points and reports whether they all read the same colour. Not the
    /// production check, just enough to say a snapshot was blank.
    private static func isFlatSnapshot(_ image: NSImage) -> Bool {
        guard let tiff = image.tiffRepresentation, let bitmap = NSBitmapImageRep(data: tiff) else { return false }
        let width = bitmap.pixelsWide
        let height = bitmap.pixelsHigh
        guard width > 0, height > 0 else { return true }
        var first: [Int]?
        for (x, y) in [(0, 0), (width - 1, 0), (0, height - 1), (width - 1, height - 1), (width / 2, height / 2)] {
            guard let color = bitmap.colorAt(x: x, y: y) else { continue }
            let sample = [Int(color.redComponent * 255), Int(color.greenComponent * 255), Int(color.blueComponent * 255)]
            if let first, first != sample { return false }
            first = first ?? sample
        }
        return true
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

    /// A borderless, opaque window over the deck window, and a count of the
    /// window server's reports of the deck window as visible since the
    /// cover took effect.
    @MainActor
    final class Cover {
        let window: NSWindow
        private(set) var visibleReportsWhileCovered = 0
        private var observer: NSObjectProtocol?

        init(window: NSWindow) { self.window = window }

        func startCounting(_ covered: NSWindow) {
            observer = NotificationCenter.default.addObserver(forName: NSWindow.didChangeOcclusionStateNotification, object: covered, queue: nil) { [weak self] _ in
                MainActor.assumeIsolated {
                    guard let self, self.window.isVisible, covered.occlusionState.contains(.visible) else { return }
                    self.visibleReportsWhileCovered += 1
                }
            }
        }

        func remove() {
            if let observer { NotificationCenter.default.removeObserver(observer) }
            observer = nil
            window.orderOut(nil)
            window.close()
        }
    }

    /// Places a borderless, opaque window exactly over `window` once the
    /// window server has made its first report on `window`, and returns
    /// only after `window` has read occluded continuously for half a
    /// second while it stays on screen. A freshly opened window reads no
    /// visible bit before that first report, and the window server reports
    /// a covered window visible for about 0.1 s within its first 0.3 s on
    /// screen, so neither a single occluded reading nor an early one proves
    /// the cover has taken effect.
    private func coverWindow(_ window: NSWindow) async throws -> Cover {
        try await waitUntil(timeout: 5, "the window server's first report of the deck window") { window.occlusionState.contains(.visible) }
        let coverWindow = NSWindow(contentRect: window.frame, styleMask: [.borderless], backing: .buffered, defer: false)
        coverWindow.isOpaque = true
        coverWindow.backgroundColor = .black
        coverWindow.level = .floating
        coverWindow.hasShadow = false
        coverWindow.animationBehavior = .none
        // Without this, closing the window also releases it (AppKit's
        // default for a window with no window controller), and the caller
        // still holding it then double-releases it when the test scope
        // ends, which crashes in objc_release during XCTest's post-test
        // deallocation check.
        coverWindow.isReleasedWhenClosed = false
        coverWindow.setFrame(window.frame, display: true)
        let cover = Cover(window: coverWindow)
        coverWindow.orderFrontRegardless()
        do {
            // Any report restarts the half second, so a visible report too
            // brief for a poll to read still counts against the cover.
            var reportCount = 0
            let recorder = NotificationCenter.default.addObserver(forName: NSWindow.didChangeOcclusionStateNotification, object: window, queue: nil) { _ in
                MainActor.assumeIsolated { reportCount += 1 }
            }
            defer { NotificationCenter.default.removeObserver(recorder) }
            var reportsSeen = 0
            var occludedSince: Date?
            let deadline = Date().addingTimeInterval(5)
            while true {
                let now = Date()
                if window.occlusionState.contains(.visible) || reportCount != reportsSeen {
                    reportsSeen = reportCount
                    occludedSince = nil
                }
                if occludedSince == nil, !window.occlusionState.contains(.visible) {
                    occludedSince = now
                }
                if let occludedSince, now.timeIntervalSince(occludedSince) >= 0.5 { break }
                guard now < deadline else {
                    XCTFail("the deck window never read occluded for half a second under the cover. occlusion=\(window.occlusionState.rawValue)")
                    throw CancellationError()
                }
                try await Task.sleep(nanoseconds: 20_000_000)
            }
        } catch {
            cover.remove()
            throw error
        }
        cover.startCounting(window)
        XCTAssertTrue(window.isVisible, "the deck window is still on screen, only covered")
        return cover
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
