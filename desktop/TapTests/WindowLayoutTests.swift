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
        split.setSidebarCollapsed(true)
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
        // Below 641 (320 + 1 + 320) AppKit itself pins both panes to their
        // minimum thickness, which must not read as a person dragging the
        // divider: growing the window back out should still rebalance it.
        controller.window?.setContentSize(NSSize(width: 600, height: 800))
        split.view.layoutSubtreeIfNeeded()
        XCTAssertFalse(split.dividerPolicy.userMovedDivider, "AppKit pinning both panes to their minimum thickness is not a person dragging the divider")
        controller.window?.setContentSize(NSSize(width: 1201, height: 800))
        split.view.layoutSubtreeIfNeeded()
        XCTAssertEqual(split.editorItem.viewController.view.frame.width, split.inspectorItem.viewController.view.frame.width, accuracy: 2, "the divider splits whatever width the window actually has")

        split.splitView.setPosition(split.sidebarItem.viewController.view.frame.width + 1 + 100, ofDividerAt: 1)
        split.userDidDragDivider()
        controller.window?.setContentSize(NSSize(width: 1000, height: 800))
        split.view.layoutSubtreeIfNeeded()
        XCTAssertNotEqual(split.editorItem.viewController.view.frame.width, split.inspectorItem.viewController.view.frame.width, accuracy: 2, "a dragged divider stays where the user put it")
    }

    func testAnAlreadyBalancedLayoutDoesNotMoveTheDividerAgain() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let controller = try windowController(for: document)
        let split = controller.splitViewController
        XCTAssertFalse(split.isSidebarCollapsed, "the sidebar shows by default: this is the case the guard must handle")
        split.view.layoutSubtreeIfNeeded()
        let countAfterFirstLayout = split.setPositionCallCount

        split.view.needsLayout = true
        split.view.layoutSubtreeIfNeeded()

        XCTAssertEqual(split.setPositionCallCount, countAfterFirstLayout, "a layout pass that is already balanced must not move the divider again")
    }

    func testTogglePreviewPinAndItsMenuItemTitle() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("plain.md"))
        let controller = try windowController(for: document)
        let sessionController = try XCTUnwrap(document.sessionController)
        let menuItem = NSMenuItem(title: "", action: #selector(DeckWindowController.togglePreviewPin(_:)), keyEquivalent: "")

        XCTAssertFalse(sessionController.navigator.isPinned)
        XCTAssertTrue(controller.validateMenuItem(menuItem))
        XCTAssertEqual(menuItem.title, "Pin Preview")

        controller.togglePreviewPin(nil)
        XCTAssertTrue(sessionController.navigator.isPinned)
        XCTAssertTrue(controller.validateMenuItem(menuItem))
        XCTAssertEqual(menuItem.title, "Unpin Preview")

        controller.togglePreviewPin(nil)
        XCTAssertFalse(sessionController.navigator.isPinned)
        XCTAssertTrue(controller.validateMenuItem(menuItem))
        XCTAssertEqual(menuItem.title, "Pin Preview")
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
