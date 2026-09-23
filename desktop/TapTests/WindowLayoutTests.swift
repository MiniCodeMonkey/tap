import XCTest
@testable import Tap

final class WindowLayoutTests: HostedTestCase {
    func windowController(for document: DeckDocument) throws -> DeckWindowController {
        try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
    }

    func testHideThePreview() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let controller = try windowController(for: document)
        let split = controller.splitViewController
        controller.togglePreview(nil)
        split.view.layoutSubtreeIfNeeded()
        XCTAssertTrue(split.isPreviewHidden)
        XCTAssertEqual(split.editorItem.viewController.view.frame.width, split.splitView.bounds.width, accuracy: 1)

        controller.togglePreview(nil)
        split.view.layoutSubtreeIfNeeded()
        XCTAssertFalse(split.isPreviewHidden)
        XCTAssertEqual(split.editorItem.viewController.view.frame.width, split.inspectorItem.viewController.view.frame.width, accuracy: 2)
    }

    func testTheDividerStaysInTheMiddleUntilTheUserDragsIt() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let controller = try windowController(for: document)
        let split = controller.splitViewController
        controller.window?.setContentSize(NSSize(width: 1201, height: 800))
        split.view.layoutSubtreeIfNeeded()
        XCTAssertEqual(split.editorItem.viewController.view.frame.width, 600, accuracy: 1)

        split.splitView.setPosition(400, ofDividerAt: 0)
        split.userDidDragDivider()
        controller.window?.setContentSize(NSSize(width: 1000, height: 800))
        split.view.layoutSubtreeIfNeeded()
        XCTAssertNotEqual(split.editorItem.viewController.view.frame.width, 499.5, accuracy: 1, "a dragged divider stays where the user put it")
    }

    func testTheEditorTextStartsBelowTheToolbar() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let controller = try windowController(for: document)
        XCTAssertNotNil(controller.window?.toolbar)
        let editorController = document.sessionController!.editorViewController
        editorController.view.layoutSubtreeIfNeeded()
        editorController.updateInsets()
        XCTAssertGreaterThan(editorController.view.safeAreaInsets.top, 0)
        XCTAssertEqual(editorController.scrollView.contentInsets.top, editorController.view.safeAreaInsets.top, accuracy: 0.5)
    }
}
