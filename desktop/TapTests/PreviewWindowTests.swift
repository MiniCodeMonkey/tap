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

        controller.editor.moveCursor(toSlide: 2)
        try await waitForPreview(document, slide: 3)

        previewWindow.performClose(nil)
        try await waitUntil(timeout: 5, "the preview to dock") { controller.previewViewController.view.window === deckWindow }
        XCTAssertFalse(windowController.splitViewController.isPreviewHidden)
        XCTAssertNil(windowController.previewWindowController)
    }
}
