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
        let data = try XCTUnwrap(AppEnvironment.shared.recentThumbnailStore.imageData(for: deck))
        let image = try XCTUnwrap(NSBitmapImageRep(data: data))
        let width = image.pixelsWide
        let height = image.pixelsHigh
        var colors = Set<[Int]>()
        let points = [(0, 0), (width - 1, 0), (0, height - 1), (width - 1, height - 1), (width / 2, height / 2)]
        for (x, y) in points {
            guard let color = image.colorAt(x: x, y: y) else { continue }
            colors.insert([Int(color.redComponent * 255), Int(color.greenComponent * 255), Int(color.blueComponent * 255)])
        }
        XCTAssertGreaterThan(colors.count, 1, "a blank capture reads back as a single flat colour")
    }

    /// A preview collapsed out of the split view never paints, so a ready
    /// signal that arrives while it is hidden must not be captured. A later
    /// ready for slide 1, once the pane is shown again, does get recorded;
    /// that second ready is delivered directly (rather than waited for
    /// organically) because tap only ever reports ready again when the
    /// slide, step or revision actually changes, which merely showing the
    /// pane does not do on its own.
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
    }
}
