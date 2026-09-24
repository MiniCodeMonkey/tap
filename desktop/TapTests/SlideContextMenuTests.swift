import XCTest
@testable import Tap

final class SlideContextMenuTests: HostedTestCase {
    func openOps() async throws -> (DeckDocument, DeckSessionController, DeckWindowController) {
        let document = try await openDeck(try Fixtures.copyDeck("ops.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "the panel") { controller.slidePanel.slides.count == 7 }
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        controller.editor.layoutSubtreeIfNeeded()
        controller.editor.textLayoutManager?.ensureLayout(for: controller.editor.textLayoutManager!.documentRange)
        return (document, controller, try XCTUnwrap(document.windowControllers.first as? DeckWindowController))
    }

    func titles(_ controller: DeckSessionController) async throws -> [String] {
        try await TapSlideList.list(text: controller.editor.string).slides.map(\.title)
    }

    func rightClick(at point: NSPoint, in view: NSView) throws -> NSEvent {
        let window = try XCTUnwrap(view.window)
        return try XCTUnwrap(NSEvent.mouseEvent(with: .rightMouseDown, location: view.convert(point, to: nil), modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                                windowNumber: window.windowNumber, context: nil, eventNumber: 0, clickCount: 1, pressure: 1))
    }

    func keyDown(_ keyCode: UInt16, characters: String, in view: NSView) throws -> NSEvent {
        let window = try XCTUnwrap(view.window)
        return try XCTUnwrap(NSEvent.keyEvent(with: .keyDown, location: .zero, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                              windowNumber: window.windowNumber, context: nil, characters: characters, charactersIgnoringModifiers: characters,
                                              isARepeat: false, keyCode: keyCode))
    }

    func testContextMenuOnASlide() async throws {
        let (_, controller, windowController) = try await openOps()
        let panel = controller.slidePanel
        // A right-click on thumbnail 3, through the collection view's own menu(for:).
        let item3 = try XCTUnwrap(panel.item(forSlide: 3))
        let frame = item3.view.convert(item3.view.bounds, to: panel.collectionView)
        let menu = try XCTUnwrap(panel.collectionView.menu(for: try rightClick(at: NSPoint(x: frame.midX, y: frame.midY), in: panel.collectionView)))
        XCTAssertEqual(panel.selectedNumbers, [3], "a right-click on an unselected thumbnail selects it")
        let menuTitles = menu.items.map(\.title)
        for required in ["New Slide After", "Duplicate", "Delete Slide", "Move to Top", "Move to Bottom", "Copy"] {
            XCTAssertTrue(menuTitles.contains(required), "\(required) is missing from \(menuTitles)")
        }
        // A right-click on box 3's header, through the editor's own menu(for:).
        let header3 = try XCTUnwrap(controller.editor.headerRect(forBoxAt: 2))
        let editorMenu = try XCTUnwrap(controller.editor.menu(for: try rightClick(at: NSPoint(x: header3.midX, y: header3.midY), in: controller.editor)))
        XCTAssertEqual(editorMenu.items.map(\.title), menuTitles, "a box header offers the same menu")
        XCTAssertNil(controller.editor.menu(for: try rightClick(at: NSPoint(x: header3.midX, y: header3.maxY + EditorTextView.lineHeight * 2), in: controller.editor))?.items.first { $0.title == "Move to Top" },
                     "a right-click in the text is the text view's own menu")

        windowController.moveSlidesToBottom(nil)
        let afterMove = try await titles(controller)
        XCTAssertEqual(afterMove, ["One", "Two", "Four", "Five", "Six", "Seven", "Three"])
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }

        let pasteboard = AppEnvironment.shared.slidePasteboard
        windowController.copySlides(nil)
        XCTAssertNotNil(pasteboard.data(forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType)))
        XCTAssertEqual(pasteboard.string(forType: .string), "<!-- layout: section -->\n# Three")
        controller.editor.moveCursor(toSlide: 0)
        let paste = NSMenuItem(title: "", action: #selector(DeckWindowController.pasteSlides(_:)), keyEquivalent: "")
        XCTAssertTrue(windowController.validateMenuItem(paste))
        windowController.pasteSlides(nil)
        let afterPaste = try await titles(controller)
        XCTAssertEqual(afterPaste, ["One", "Three", "Two", "Four", "Five", "Six", "Seven", "Three"], "pasted after the current slide")
    }

    /// Types a new slide, "Split", at the end of slide `number`, and says
    /// tap has not answered for it: until the answer, the boxes and the
    /// thumbnails still count seven slides, and the answer renumbers every
    /// slide after `number`.
    func typeASplitSlide(atTheEndOfSlide number: Int, _ controller: DeckSessionController) {
        let end = NSMaxRange(controller.editor.boxes[number - 1].range)
        controller.editor.setSelectedRange(NSRange(location: end, length: 0))
        controller.editor.insertText("\n\n---\n\n# Split", replacementRange: NSRange(location: NSNotFound, length: 0))
        XCTAssertNotEqual(controller.editor.string, controller.lastAppliedText, "tap has not answered for the typing")
        XCTAssertEqual(controller.slidePanel.slides.map(\.title), ["One", "Two", "Three", "Four", "Five", "Six", "Seven"], "the thumbnails are the old ones")
    }

    /// Waits until tap has answered for the text and the deck has `count` slides.
    func waitForConfirmedSlides(_ controller: DeckSessionController, count: Int, _ message: String) async throws {
        try await waitUntil(timeout: 10, message) { controller.lastAppliedText == controller.editor.string && controller.editor.boxes.count == count }
    }

    func testTheDeleteKeyDeletesTheThumbnailClickedBeforeTapRenumbers() async throws {
        let (_, controller, _) = try await openOps()
        let panel = controller.slidePanel
        panel.collectionView.window?.makeFirstResponder(panel.collectionView)
        typeASplitSlide(atTheEndOfSlide: 1, controller)
        panel.click(slide: 3, extendingSelection: false)
        panel.collectionView.keyDown(with: try keyDown(51, characters: "\u{7f}", in: panel.collectionView))
        try await waitForConfirmedSlides(controller, count: 7, "the queued delete to land and tap to answer")
        let titles = try await titles(controller)
        XCTAssertEqual(titles, ["One", "Split", "Two", "Four", "Five", "Six", "Seven"], "the thumbnail showed Three, so Three goes")
    }

    /// Move Up/Down and Move to Top/Bottom go through the same
    /// `perform(on: captureSelection())` rule as every other command: a
    /// hand-made selection is checked against tap's ranges when the move
    /// runs, and a queued move refuses rather than moving whatever now
    /// holds the selected numbers.
    func testAQueuedMoveOnASelectionThatTapRenumbersIsRefused() async throws {
        let (_, controller, windowController) = try await openOps()
        let panel = controller.slidePanel
        panel.select(numbers: [3, 4], scroll: false)
        typeASplitSlide(atTheEndOfSlide: 1, controller)
        windowController.moveSlidesDown(nil)
        try await waitForConfirmedSlides(controller, count: 8, "the queued move to land and tap to answer")
        try await Task.sleep(nanoseconds: 500_000_000)
        let titles = try await titles(controller)
        XCTAssertEqual(titles, ["One", "Split", "Two", "Three", "Four", "Five", "Six", "Seven"],
                       "slides 3 and 4 no longer hold Three and Four, so the move is refused")
    }

    func testADeleteOfASelectionThatTapRenumbersIsRefused() async throws {
        let (_, controller, _) = try await openOps()
        let panel = controller.slidePanel
        panel.collectionView.window?.makeFirstResponder(panel.collectionView)
        typeASplitSlide(atTheEndOfSlide: 1, controller)
        panel.select(numbers: [3, 4], scroll: false)
        panel.collectionView.keyDown(with: try keyDown(51, characters: "\u{7f}", in: panel.collectionView))
        // tap's answer adds Split; the queued delete, refused or not, runs
        // within one 20 ms poll of it.
        try await waitUntil(timeout: 10, "tap's answer for the typing") {
            controller.lastAppliedText == controller.editor.string && controller.editor.boxes.count != 7
        }
        try await Task.sleep(nanoseconds: 500_000_000)
        let titles = try await titles(controller)
        XCTAssertEqual(titles, ["One", "Split", "Two", "Three", "Four", "Five", "Six", "Seven"],
                       "slides 3 and 4 no longer hold Three and Four, so nothing is deleted")
    }

    func testPasteAfterAThumbnailBeforeTapRenumbers() async throws {
        let (_, controller, windowController) = try await openOps()
        let panel = controller.slidePanel
        panel.click(slide: 7, extendingSelection: false)
        windowController.copySlides(nil)
        typeASplitSlide(atTheEndOfSlide: 1, controller)
        let item3 = try XCTUnwrap(panel.item(forSlide: 3))
        let frame = item3.view.convert(item3.view.bounds, to: panel.collectionView)
        _ = try XCTUnwrap(panel.collectionView.menu(for: try rightClick(at: NSPoint(x: frame.midX, y: frame.midY), in: panel.collectionView)))
        windowController.pasteSlides(nil)
        try await waitForConfirmedSlides(controller, count: 9, "the queued paste to land and tap to answer")
        let titles = try await titles(controller)
        XCTAssertEqual(titles, ["One", "Split", "Two", "Three", "Seven", "Four", "Five", "Six", "Seven"], "pasted after the thumbnail that showed Three")
    }

    func testTheDeleteKeyInTheSidebarDeletesTheSelection() async throws {
        let (_, controller, _) = try await openOps()
        let panel = controller.slidePanel
        let window = try XCTUnwrap(panel.collectionView.window)
        window.makeFirstResponder(panel.collectionView)
        panel.select(numbers: [5, 6], scroll: false)
        // The real key path: keyDown with the Delete key's code, as the key event reaches the first responder.
        panel.collectionView.keyDown(with: try keyDown(51, characters: "\u{7f}", in: panel.collectionView))
        let afterDelete = try await titles(controller)
        XCTAssertEqual(afterDelete, ["One", "Two", "Three", "Four", "Seven"])
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }
        panel.select(numbers: [1], scroll: false)
        panel.collectionView.keyDown(with: try keyDown(117, characters: "\u{f728}", in: panel.collectionView))
        let afterForwardDelete = try await titles(controller)
        XCTAssertEqual(afterForwardDelete, ["Two", "Three", "Four", "Seven"], "Forward Delete too")
    }

    func testBackspaceInTheEditorDeletesTextNotSlides() async throws {
        let (_, controller, _) = try await openOps()
        let editor = controller.editor
        let window = try XCTUnwrap(editor.window)
        window.makeFirstResponder(editor)
        let end = NSMaxRange(editor.boxes[0].range)
        editor.setSelectedRange(NSRange(location: end, length: 0))
        let before = editor.string
        editor.keyDown(with: try keyDown(51, characters: "\u{7f}", in: editor))
        XCTAssertEqual((editor.string as NSString).length, (before as NSString).length - 1, "Backspace deleted one character")
        XCTAssertEqual(editor.boxes.count, 7, "and no slide")
        XCTAssertFalse(editor.string.contains("# One\n\nFirst."), "the character came off slide 1's last line")
    }
}
