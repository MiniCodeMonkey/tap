import XCTest
@testable import Tap

final class PreviewWindowTests: HostedTestCase {
    func testPreviewInItsOwnWindow() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let windowController = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        let controller = try XCTUnwrap(document.sessionController)
        let deckWindow = try XCTUnwrap(windowController.window)
        try await waitForBoxes(document, count: 4)

        windowController.showPreviewInWindow(nil)
        let previewWindow = try XCTUnwrap(windowController.previewWindowController?.window)
        XCTAssertTrue(controller.previewViewController.view.window === previewWindow)
        XCTAssertTrue(previewWindow.title.hasSuffix(": Preview"))
        XCTAssertTrue(previewWindow.styleMask.contains(.resizable))
        XCTAssertTrue(windowController.splitViewController.isPreviewHidden)

        // A person choosing Preview in Window is already in the app, so the
        // new window comes forward with it. A test runner is not, and a
        // window of an app the window server has never activated counts as
        // covered: the page in it reports document.hidden, stops running,
        // and never reports a slide ready. openDeck brings the deck window
        // forward for the same reason.
        NSApp.activate(ignoringOtherApps: true)
        previewWindow.orderFrontRegardless()

        controller.editor.moveCursor(toSlide: 2)
        try await waitForPreview(document, slide: 3)

        previewWindow.performClose(nil)
        try await waitUntil(timeout: 5, "the preview to dock") { controller.previewViewController.view.window === deckWindow }
        XCTAssertFalse(windowController.splitViewController.isPreviewHidden)
        XCTAssertNil(windowController.previewWindowController)
    }

    func testTheDeckCardLeavesADetachedPreviewAlone() async throws {
        Task { await AppEnvironment.shared.deckSchema.load() }
        try await waitUntil(timeout: 30, "the schema") { AppEnvironment.shared.deckSchema.isLoaded }
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("seven-slides.md"))
        let controller = try XCTUnwrap(document.sessionController)
        let deckWindow = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        deckWindow.showDeckSettings(nil)
        deckWindow.showPreviewInWindow(nil)
        XCTAssertFalse(controller.previewViewController.view.isHidden, "detached while the card was open: shown in its window")
        deckWindow.showPreview(nil)
        deckWindow.showDeckSettings(nil)
        XCTAssertFalse(controller.previewViewController.view.isHidden, "the Deck card does not reach a preview in its own window")
        XCTAssertEqual(controller.deckCard.display, .form)
        deckWindow.dockPreview()
        XCTAssertFalse(controller.previewViewController.view.isHidden, "docked back beside the open card")
        XCTAssertEqual(controller.deckCard.display, .form, "and the card stays as it was")
    }
}
