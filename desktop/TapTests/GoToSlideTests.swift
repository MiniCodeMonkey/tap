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

    /// `AppDelegate.validateMenuItem` is what disables Go to Slide when no
    /// deck can be resolved, rather than leaving it enabled and inert (the
    /// task's own added requirement). The decision itself is factored into
    /// `goToSlideIsEnabled(forKeyWindow:)`, exactly the way `deck(owning:)`
    /// above is factored, so it can be tested against real windows without
    /// driving `NSApp.keyWindow`. `validateMenuItem` itself is also
    /// exercised directly with a real menu item: this host's `NSApp.keyWindow`
    /// is always nil (see `testGoToSlideFindsTheDeckOwningADetachedPreviewWindow`),
    /// which happens to be exactly the "no deck resolves" case, so that call
    /// covers the disabled path through the real method; the enabled path
    /// through `validateMenuItem` itself, which needs a real key window,
    /// stays uncovered along with the `NSApp.keyWindow` read.
    func testValidateMenuItemGoToSlide() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let deckWindowController = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        let appDelegate = try XCTUnwrap(NSApp.delegate as? AppDelegate)

        XCTAssertTrue(AppDelegate.goToSlideIsEnabled(forKeyWindow: deckWindowController.window))
        XCTAssertFalse(AppDelegate.goToSlideIsEnabled(forKeyWindow: nil))
        WelcomeWindowController.shared.showWindow(nil)
        XCTAssertFalse(AppDelegate.goToSlideIsEnabled(forKeyWindow: WelcomeWindowController.shared.window),
                      "the welcome window resolves to no deck")
        WelcomeWindowController.shared.window?.orderOut(nil)

        let goToSlideItem = NSMenuItem(title: "Go to Slide", action: #selector(AppDelegate.goToSlide(_:)), keyEquivalent: "")
        XCTAssertFalse(appDelegate.validateMenuItem(goToSlideItem),
                      "no key window here, so validateMenuItem itself must disable the item")

        // Other AppDelegate-targeted items are unaffected by the new check.
        let tapLogItem = NSMenuItem(title: "Tap Log", action: #selector(AppDelegate.showTapLog(_:)), keyEquivalent: "")
        XCTAssertTrue(appDelegate.validateMenuItem(tapLogItem))
    }

    /// The panel opens over whichever window the person invoked Go to Slide
    /// from, which is the detached preview window when that is the source,
    /// not always the deck's own window. Confirming a jump then brings the
    /// deck window to the front, since that is where the visible effect (the
    /// cursor move) actually happens; cancelling leaves window order alone.
    /// The bring-forward step is observed through `bringDeckWindowForward`,
    /// a replaceable recorder here, rather than `NSApp.orderedWindows`: this
    /// hosted test host does not reflect `makeKeyAndOrderFront` in that list
    /// reliably enough to tell a confirmed jump apart from one where the
    /// step never ran (confirmed directly: an earlier version of this test
    /// asserted on `orderedWindows` and passed whether or not the production
    /// call was present).
    func testGoToSlideOpensOverTheInvokingWindowAndBringsTheDeckWindowFrontOnConfirm() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        try await waitForBoxes(document, count: 4)
        let deckWindowController = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        deckWindowController.showPreviewInWindow(nil)
        let previewWindow = try XCTUnwrap(deckWindowController.previewWindowController?.window)
        let deckWindow = try XCTUnwrap(deckWindowController.window)

        var broughtForward: [NSWindow] = []
        deckWindowController.bringDeckWindowForward = { broughtForward.append($0) }

        deckWindowController.showGoToSlide(over: previewWindow)
        let outline = try XCTUnwrap(deckWindowController.goToSlideController)
        XCTAssertTrue(outline.panel.isVisible)
        XCTAssertTrue(outline.panel.parent === previewWindow,
                      "the panel opens over the window it was invoked from, not always the deck's own window")

        outline.setQuery("")
        outline.confirm()
        XCTAssertFalse(outline.panel.isVisible)
        XCTAssertEqual(broughtForward.count, 1, "confirm brings the deck window forward exactly once")
        XCTAssertTrue(broughtForward.first === deckWindow, "confirm brings the deck window forward, not some other window")

        broughtForward.removeAll()
        deckWindowController.showGoToSlide(over: previewWindow)
        deckWindowController.goToSlideController?.cancel()
        XCTAssertTrue(broughtForward.isEmpty, "cancel leaves window order alone")

        deckWindowController.dockPreview()
    }
}
