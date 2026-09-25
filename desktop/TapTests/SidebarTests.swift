import XCTest
@testable import Tap

final class SidebarTests: HostedTestCase {
    func testTheCurrentSlide() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        let panel = controller.slidePanel
        try await waitUntil(timeout: 5, "seven thumbnails") { panel.slides.count == 7 }

        controller.editor.moveCursor(toSlide: 2)
        XCTAssertEqual(controller.editor.currentBoxIndex, 2, "the box of slide 3 is the highlighted one")
        XCTAssertEqual(controller.currentSlideNumber, 3)
        XCTAssertEqual(panel.selectedNumbers, [3], "thumbnail 3 is highlighted")
        XCTAssertTrue(try XCTUnwrap(panel.item(forSlide: 3)).isSelected)
        XCTAssertFalse(try XCTUnwrap(panel.item(forSlide: 1)).isSelected)
        XCTAssertEqual(try XCTUnwrap(panel.item(forSlide: 3)).numberLabel.stringValue, "3")
        XCTAssertEqual(panel.item(forSlide: 3)?.view.accessibilityLabel(), "Slide 3, default layout, What We Knew, 1 step")
    }

    func testSidebarAndCursorAreLinked() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        let panel = controller.slidePanel
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        try await waitUntil(timeout: 5, "seven thumbnails") { panel.slides.count == 7 }

        // A real click makes the collection view first responder before the selection changes; focus then stays there (decision 1).
        window.makeFirstResponder(panel.collectionView)
        panel.click(slide: 5, extendingSelection: false)
        XCTAssertTrue(window.firstResponder === panel.collectionView, "clicking a thumbnail leaves focus in the sidebar")
        XCTAssertEqual(controller.editor.selectedRange().location, controller.editor.boxes[4].range.location + ("# Root Cause" as NSString).length,
                       "the cursor moves to slide 5 (the end of its heading line, as Go to Slide does)")
        XCTAssertEqual(controller.editor.currentBoxIndex, 4)
        try await waitForPreview(document, slide: 5)

        panel.click(slide: 7, extendingSelection: true)
        XCTAssertEqual(panel.selectedNumbers, [5, 6, 7], "Shift-click selects the range")
        XCTAssertEqual(controller.editor.currentBoxIndex, 6, "and the cursor is in the slide clicked last")
        XCTAssertEqual(controller.selectedSlideNumbers, [5, 6, 7])
        panel.click(slide: 6, extendingSelection: false)
        panel.click(slide: 2, extendingSelection: true)
        XCTAssertEqual(panel.selectedNumbers, [2, 3, 4, 5, 6], "the anchor is the slide last clicked without Shift, whichever way the range runs")
        panel.click(slide: 4, extendingSelection: true)
        XCTAssertEqual(panel.selectedNumbers, [4, 5, 6], "a second Shift-click extends from the same anchor, not from the range's end")

        controller.editor.moveCursor(toSlide: 1)
        XCTAssertEqual(panel.selectedNumbers, [2], "moving the cursor selects only that thumbnail")
        XCTAssertEqual(controller.selectedSlideNumbers, [2])
    }

    func testThePanelIsAPinnedSidebarNextToTheEditor() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let windowController = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        let split = windowController.splitViewController
        let window = try XCTUnwrap(windowController.window)
        split.view.layoutSubtreeIfNeeded()
        XCTAssertFalse(split.sidebarItem.isCollapsed)
        XCTAssertEqual(split.splitView.bounds.width, window.contentView?.bounds.width ?? 0, accuracy: 1, "the split fills the window; pinning never resizes the window")
        XCTAssertEqual(split.sidebarItem.viewController.view.frame.width, SlidePanelViewController.width + 24, accuracy: 1)
        // A split item's own view sits inside AppKit's own wrapper for the
        // item, so its frame is relative to that wrapper, not to the split
        // view; converting into the split view's coordinate space is what
        // makes the two panes' positions comparable.
        let editorFrameInSplit = split.editorItem.viewController.view.convert(split.editorItem.viewController.view.bounds, to: split.splitView)
        let sidebarFrameInSplit = split.sidebarItem.viewController.view.convert(split.sidebarItem.viewController.view.bounds, to: split.splitView)
        XCTAssertGreaterThanOrEqual(editorFrameInSplit.minX, sidebarFrameInSplit.maxX, "no overlap")
        XCTAssertEqual(split.editorItem.viewController.view.frame.width, split.inspectorItem.viewController.view.frame.width, accuracy: 2,
                       "the editor and the preview still split what is left evenly")
    }
}
