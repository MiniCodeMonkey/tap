import XCTest
@testable import Tap

final class GoToSlideTests: HostedTestCase {
    func testGoToSlide() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let windowController = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        let editor = try XCTUnwrap(document.sessionController?.editor)

        windowController.goToSlide(nil)
        let outline = try XCTUnwrap(windowController.goToSlideController)
        XCTAssertTrue(outline.panel.isVisible)
        XCTAssertEqual(outline.entries.count, 7)

        outline.searchField.stringValue = "ro"
        outline.setQuery("ro")
        XCTAssertEqual(outline.entries.map(\.title), ["Debugging Production at 3am", "Root Cause"])
        XCTAssertEqual(outline.tableView.selectedRow, 0)

        let fieldEditor = NSTextView()
        XCTAssertTrue(outline.control(outline.searchField, textView: fieldEditor, doCommandBy: #selector(NSResponder.moveDown(_:))))
        XCTAssertTrue(outline.control(outline.searchField, textView: fieldEditor, doCommandBy: #selector(NSResponder.insertNewline(_:))))
        XCTAssertEqual(editor.currentBoxIndex, 4, "Return jumps to Root Cause, slide 5")
        XCTAssertFalse(outline.panel.isVisible)
    }

    /// `DeckWindowController.goToSlide` is a menu action, and a menu action
    /// aimed at the deck window controller does not reach it through the
    /// responder chain when the key window is the preview detached into its
    /// own window (`DeckWindowController.showPreviewInWindow`, Task 21). Show
    /// Tap Log solved the same gap by resolving the owning deck through
    /// `AppDelegate.deck(owning:)` before acting; Go to Slide reuses that
    /// same resolution so Cmd+Shift+O finds the right deck. This hosted test
    /// host never becomes key (see `TapLogWindowTests`), so this drives
    /// `AppDelegate.deck(owning:)` directly with a real detached preview
    /// window rather than the menu item itself; the one line this leaves
    /// uncovered is the menu validator's own `NSApp.keyWindow` read.
    func testGoToSlideFindsTheDeckOwningADetachedPreviewWindow() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let deckWindowController = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        try await waitForBoxes(document, count: 4)

        deckWindowController.showPreviewInWindow(nil)
        let previewWindow = try XCTUnwrap(deckWindowController.previewWindowController?.window)
        XCTAssertTrue(AppDelegate.deck(owning: previewWindow) === deckWindowController,
                      "the detached preview window must resolve back to the deck it was detached from")

        AppDelegate.deck(owning: previewWindow)?.goToSlide(nil)
        XCTAssertTrue(deckWindowController.goToSlideController?.panel.isVisible ?? false,
                      "Go to Slide opens for the deck owning the detached preview window")

        deckWindowController.goToSlideController?.cancel()
        deckWindowController.dockPreview()
    }
}
