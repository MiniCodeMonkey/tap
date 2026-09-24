import XCTest
@testable import Tap

final class SlidePanelLayoutTests: HostedTestCase {
    func windowController(for document: DeckDocument) throws -> DeckWindowController {
        try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
    }

    func testSplitLayout() async throws {
        let deck = try Fixtures.copyDeck("plain.md")
        let document = try await openDeck(deck)
        let controller = try windowController(for: document)
        let split = controller.splitViewController
        split.view.layoutSubtreeIfNeeded()
        XCTAssertTrue(controller.isPanelPinned, "pinned on first launch")
        XCTAssertFalse(split.sidebarItem.isCollapsed)
        XCTAssertEqual(split.editorItem.viewController.view.frame.width, split.inspectorItem.viewController.view.frame.width, accuracy: 2,
                       "the editor and the preview each take half of what is left")

        controller.setPanelPinned(false)
        // Decision 4: the state is per deck. A second deck opened while the
        // first is unpinned is pinned, and stays pinned when it reopens; one
        // key shared by every deck would open it unpinned.
        let other = try Fixtures.copyDeck("seven-slides.md")
        let otherDocument = try await openDeck(other)
        let otherController = try windowController(for: otherDocument)
        otherController.splitViewController.view.layoutSubtreeIfNeeded()
        XCTAssertTrue(otherController.isPanelPinned, "unpinning one deck leaves another pinned")
        XCTAssertFalse(otherController.splitViewController.sidebarItem.isCollapsed, "the other deck's own panel stays pinned")
        XCTAssertTrue(controller.splitViewController.sidebarItem.isCollapsed, "and the first deck's window is still unpinned")
        otherDocument.close()
        try await waitUntil(timeout: 10, "the other window to close") { otherDocument.windowControllers.first?.window?.isVisible != true }

        let renamed = deck.deletingLastPathComponent().appendingPathComponent("renamed.md")
        try await document.move(to: renamed)
        document.close()
        try await waitUntil(timeout: 10, "the window to close") { document.windowControllers.first?.window?.isVisible != true }
        let reopened = try await openDeck(renamed)
        let again = try windowController(for: reopened)
        XCTAssertFalse(again.isPanelPinned, "each deck remembers its state")
        XCTAssertTrue(again.splitViewController.sidebarItem.isCollapsed)
        let otherAgain = try windowController(for: try await openDeck(other))
        otherAgain.splitViewController.view.layoutSubtreeIfNeeded()
        XCTAssertTrue(otherAgain.isPanelPinned, "the other deck reopens pinned, beside a deck that reopened unpinned")
        XCTAssertFalse(otherAgain.splitViewController.sidebarItem.isCollapsed)
    }

    func testPeekAtTheSlidePanel() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try windowController(for: document)
        let session = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(controller.window)
        controller.setPanelPinned(false)
        controller.panelPeek.showDelay = 0.1
        controller.panelPeek.hideDelay = 0.05
        XCTAssertTrue(controller.panelOverlay.isHidden)
        XCTAssertFalse(controller.panelOverlay.isDescendant(of: controller.splitViewController.view), "the overlay is a sibling of the split view, never one of its panes")

        // A pointer that crosses the button and leaves before the show delay shows nothing.
        controller.panelPeek.pointerEnteredButton()
        controller.panelPeek.pointerLeftButton()
        try await Task.sleep(nanoseconds: 250_000_000)
        XCTAssertTrue(controller.panelOverlay.isHidden, "a pointer that only crossed the button does not open the panel")

        controller.panelPeek.pointerEnteredButton()
        try await waitUntil(timeout: 2, "the overlay to show") { !controller.panelOverlay.isHidden }
        XCTAssertTrue(session.slidePanel.view.isDescendant(of: controller.panelOverlay), "the panel is a glass overlay")
        if #available(macOS 26.0, *) {
            XCTAssertTrue(controller.panelOverlay.subviews.contains { $0 is NSGlassEffectView }, "the system's glass on macOS 26")
        }
        XCTAssertTrue(controller.splitViewController.sidebarItem.isCollapsed, "nothing is pushed aside")

        controller.panelPeek.pointerLeftButton()
        controller.panelPeek.pointerEnteredPanel()
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertFalse(controller.panelOverlay.isHidden, "it stays while the pointer is over the panel, well past the hide delay")
        window.makeFirstResponder(session.slidePanel.collectionView)
        session.slidePanel.click(slide: 5, extendingSelection: false)
        XCTAssertEqual(session.editor.currentBoxIndex, 4, "a click in it jumps there")

        // AppKit's own hidden-first-responder fallback would also land on
        // the editor today, since it is next in the window's key view loop,
        // which would let the app's own explicit move in onHide be removed
        // without this test noticing. Pointing the collection view's own
        // next key view somewhere else first means only the app's line can
        // put focus on the editor.
        session.slidePanel.collectionView.nextKeyView = session.previewViewController.webView
        controller.panelPeek.pointerLeftPanel()
        try await waitUntil(timeout: 2, "the overlay to hide") { controller.panelOverlay.isHidden }
        XCTAssertTrue(window.firstResponder === session.editor, "focus never stays in a hidden panel: the editor has it once the peek closes")
    }

    func testPinTheSlidePanel() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let controller = try windowController(for: document)
        let session = try XCTUnwrap(document.sessionController)
        let split = controller.splitViewController
        controller.setPanelPinned(false)
        split.view.layoutSubtreeIfNeeded()

        let frameBefore = try XCTUnwrap(controller.window?.frame)
        controller.toggleSlidePanel(nil)
        split.view.layoutSubtreeIfNeeded()
        XCTAssertEqual(controller.window?.frame, frameBefore, "pinning pushes the panes, it never resizes the window")
        XCTAssertTrue(controller.isPanelPinned)
        XCTAssertFalse(split.sidebarItem.isCollapsed, "the panel docks as a sidebar")
        XCTAssertTrue(session.slidePanel.view.isDescendant(of: split.sidebarItem.viewController.view))
        XCTAssertTrue(controller.panelOverlay.isHidden)
        // A split item's own view sits inside AppKit's own wrapper for the
        // item, so its frame is relative to that wrapper, not to the split
        // view; converting into the split view's coordinate space is what
        // makes the two panes' positions comparable (see MainSplitViewController.balance()).
        let editorFrameInSplit = split.editorItem.viewController.view.convert(split.editorItem.viewController.view.bounds, to: split.splitView)
        let sidebarFrameInSplit = split.sidebarItem.viewController.view.convert(split.sidebarItem.viewController.view.bounds, to: split.splitView)
        XCTAssertGreaterThanOrEqual(editorFrameInSplit.minX, sidebarFrameInSplit.maxX,
                                    "the editor and preview are pushed right, with no overlap")
        XCTAssertEqual(split.editorItem.viewController.view.frame.width, split.inspectorItem.viewController.view.frame.width, accuracy: 2)
        XCTAssertFalse(controller.panelPeek.isEnabled, "hovering does nothing while pinned")

        controller.toggleSlidePanel(nil)
        split.view.layoutSubtreeIfNeeded()
        XCTAssertEqual(controller.window?.frame, frameBefore)
        XCTAssertFalse(controller.isPanelPinned)
        XCTAssertTrue(split.sidebarItem.isCollapsed)
        XCTAssertTrue(controller.panelPeek.isEnabled, "hovering the button peeks again")
        XCTAssertEqual(split.editorItem.viewController.view.frame.minX, 0, accuracy: 1)
        XCTAssertEqual(split.editorItem.viewController.view.frame.width, split.inspectorItem.viewController.view.frame.width, accuracy: 2)

        let menuItem = NSMenuItem(title: "", action: #selector(DeckWindowController.toggleSlidePanel(_:)), keyEquivalent: "")
        XCTAssertTrue(controller.validateMenuItem(menuItem))
        XCTAssertEqual(menuItem.title, "Pin Slide Panel")
        controller.toggleSlidePanel(nil)
        XCTAssertTrue(controller.validateMenuItem(menuItem))
        XCTAssertEqual(menuItem.title, "Unpin Slide Panel")
    }

    /// AppKit's window tab stack copies the prior tab's split-view divider
    /// positions into the new tab on every swap
    /// (NSWindowStackController._syncWindowFrameStateForSwapWithNewWindow),
    /// which can carry one deck's collapsed sidebar onto another deck that
    /// should not be collapsed, and the reverse. Each deck's own pinned
    /// state must survive switching to it and away from it, in both
    /// directions, and an unpinned deck's sidebar column must never reopen
    /// empty (the panel view belongs to the overlay while unpinned, not to
    /// the sidebar host).
    func testTabSwitchKeepsEachDecksPinnedState() async throws {
        let pinnedDocument = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let pinned = try windowController(for: pinnedDocument)
        let unpinnedDocument = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        let unpinned = try windowController(for: unpinnedDocument)
        unpinned.setPanelPinned(false)
        let pinnedWindow = try XCTUnwrap(pinned.window)
        let unpinnedWindow = try XCTUnwrap(unpinned.window)
        XCTAssertEqual(pinnedWindow.tabbedWindows?.count, 2, "both decks share one tab group")

        for round in 1...2 {
            pinnedWindow.tabGroup?.selectedWindow = pinnedWindow
            try await waitUntil(timeout: 2, "the pinned deck to stay pinned after round \(round)") { !pinned.splitViewController.sidebarItem.isCollapsed }
            XCTAssertTrue(pinned.isPanelPinned, "round \(round): switching to the pinned deck leaves it pinned")
            XCTAssertFalse(pinned.splitViewController.sidebarItem.isCollapsed)

            unpinnedWindow.tabGroup?.selectedWindow = unpinnedWindow
            try await waitUntil(timeout: 2, "the unpinned deck to stay unpinned after round \(round)") { unpinned.splitViewController.sidebarItem.isCollapsed }
            XCTAssertFalse(unpinned.isPanelPinned, "round \(round): switching to the unpinned deck leaves it unpinned")
            XCTAssertTrue(unpinned.splitViewController.sidebarItem.isCollapsed)
            XCTAssertTrue(unpinned.sidebarHost.view.subviews.isEmpty, "round \(round): the unpinned deck's sidebar column stays empty, not a column with nothing pushed into it")
        }
    }
}
