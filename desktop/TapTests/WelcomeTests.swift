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
        defer {
            cover.orderOut(nil)
            cover.close()
        }

        _ = try await waitForRunningTap(document)
        // Long enough for a ready signal to have arrived while covered, on a
        // deck this small.
        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertNil(AppEnvironment.shared.recentThumbnailStore.imageData(for: deck), "an occluded window must not be captured")

        cover.orderOut(nil)
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
        defer {
            cover.orderOut(nil)
            cover.close()
        }

        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController
        try await waitUntil(timeout: 30, "slide 1 ready behind the cover") { preview.lastReady?.slide == 1 }
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
        cover.orderOut(nil)
        try await waitUntil(timeout: 10, "the recent thumbnail") { AppEnvironment.shared.recentThumbnailStore.imageData(for: deck) != nil }
        XCTAssertEqual(readiesAfterUncovering, 0, "the thumbnail comes from the ready already received")
        XCTAssertEqual(preview.lastReady, readyWhileCovered)
        try assertRecordedThumbnailIsNotBlank(for: deck)
    }

    /// Places a borderless, opaque window exactly over `window` and waits
    /// until the window server reports `window` occluded while it stays on
    /// screen.
    private func coverWindow(_ window: NSWindow) async throws -> NSWindow {
        let cover = NSWindow(contentRect: window.frame, styleMask: [.borderless], backing: .buffered, defer: false)
        cover.isOpaque = true
        cover.backgroundColor = .black
        cover.level = .floating
        cover.hasShadow = false
        // Without this, closing the window also releases it (AppKit's
        // default for a window with no window controller), and the caller
        // still holding it then double-releases it when the test scope
        // ends, which crashes in objc_release during XCTest's post-test
        // deallocation check.
        cover.isReleasedWhenClosed = false
        cover.setFrame(window.frame, display: true)
        cover.orderFrontRegardless()
        do {
            try await waitUntil(timeout: 5, "the deck window to report occluded") { !window.occlusionState.contains(.visible) }
        } catch {
            cover.orderOut(nil)
            cover.close()
            throw error
        }
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
