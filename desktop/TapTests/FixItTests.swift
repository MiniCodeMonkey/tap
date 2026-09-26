import XCTest
@testable import Tap

/// A block whose driver the deck does not declare: tap's message on the
/// box, and the fix-it that declares the driver as one undo step.
final class FixItTests: HostedTestCase {
    func mouseDown(at point: NSPoint, in editor: EditorTextView) throws -> NSEvent {
        let window = try XCTUnwrap(editor.window)
        let inWindow = editor.convert(point, to: nil)
        return try XCTUnwrap(NSEvent.mouseEvent(with: .leftMouseDown, location: inWindow, modifierFlags: [], timestamp: 0,
                                                windowNumber: window.windowNumber, context: nil, eventNumber: 0, clickCount: 1, pressure: 1))
    }

    func testABlockUsesAnUndeclaredDriver() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("undeclared-driver.md"))
        let controller = try XCTUnwrap(document.sessionController)
        let editor = controller.editor
        try await waitForBoxes(document, count: 6)
        try await waitUntil(timeout: 10, "tap's problem on the shell block") { editor.boxes[5].slide.codeBlocks.first?.problem != nil }
        let header = editor.header(forBoxAt: 5)
        XCTAssertEqual(header.errors, [#"Line 33: This deck does not declare the shell driver. Add "shell: {}" under drivers in the frontmatter."#],
                       "the block shows tap's message, with the line; the message says exactly what to add")
        XCTAssertEqual(header.fixIt?.title, "Allow shell in This Deck")
        XCTAssertNil(editor.header(forBoxAt: 3).fixIt, "the sqlite block is declared")
        // tap dev's own output says the same, with the file and line.
        try await waitUntil(timeout: 10, "tap's warning in the log") {
            controller.session.log.text.contains("undeclared-driver.md:33: This deck does not declare the shell driver")
        }
        controller.jumpToSlide(number: 6)
        try await waitForPreview(document, slide: 6)
        let problem = await controller.previewViewController.blockProblemText()
        XCTAssertTrue(problem.hasPrefix("This deck does not declare the shell driver"), "the page shows it in the block: \(problem)")

        // The fix-it, clicked on the box header the way a person clicks it.
        editor.layoutSubtreeIfNeeded()
        let pill = try XCTUnwrap(editor.fixItRect(forBoxAt: 5), "the pill is on the box")
        let original = editor.string
        editor.mouseDown(with: try mouseDown(at: NSPoint(x: pill.midX, y: pill.midY), in: editor))
        XCTAssertTrue(editor.string.hasPrefix("---\ntitle: Undeclared Driver\ndrivers:\n  sqlite: {}\n  shell: {}\n---\n"), "shell: {} under drivers, one edit: \(editor.string.prefix(80))")
        XCTAssertEqual(editor.undoManager?.undoActionName, "Allow shell in This Deck")
        XCTAssertNil(editor.header(forBoxAt: 5).fixIt, "declared in the buffer: the pill goes before tap answers")
        try await waitUntil(timeout: 10, "tap to accept the declaration") { editor.boxes[5].slide.codeBlocks.first?.problem == nil }
        XCTAssertNil(editor.header(forBoxAt: 5).fixIt)
        XCTAssertEqual(editor.deckErrors, [], "tap parses what the fix-it wrote")
        let deck = try XCTUnwrap(document.fileURL)
        try await waitUntil(timeout: 10, "the fix-it's save") { (try? String(contentsOf: deck, encoding: .utf8))?.contains("  shell: {}\n") == true }
        // tap's render of the edited text asks about shell (Task 10); this test leaves that sheet alone.
        editor.undoManager?.undo()
        XCTAssertEqual(editor.string, original, "one undo step")
        try await waitUntil(timeout: 15, "the problem back after the undo") { editor.boxes[5].slide.codeBlocks.first?.problem != nil }
        XCTAssertEqual(editor.header(forBoxAt: 5).fixIt?.title, "Allow shell in This Deck", "the undo took the declaration away, so the pill is back")
    }

    func testADeckWithLiveCodeMustListItsDrivers() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("no-drivers.md"))
        let controller = try XCTUnwrap(document.sessionController)
        let editor = controller.editor
        try await waitForBoxes(document, count: 4)
        try await waitUntil(timeout: 10, "tap's problem") { editor.boxes[3].slide.codeBlocks.first?.problem != nil }
        XCTAssertEqual(editor.header(forBoxAt: 3).errors,
                       ["Line 19: This deck does not declare the sqlite driver. Add this to the frontmatter: drivers: sqlite: {}"],
                       "tap's words: the whole block to paste, on one line here")
        XCTAssertEqual(editor.header(forBoxAt: 3).fixIt?.title, "Allow sqlite in This Deck")
        try await waitUntil(timeout: 10, "tap's warning") { controller.session.log.text.contains("no-drivers.md:19: This deck does not declare the sqlite driver") }
        XCTAssertNil(controller.pendingQuestion, "no block runs, and nothing asks: there is no declared driver to approve")
        controller.jumpToSlide(number: 4)
        try await waitForPreview(document, slide: 4)
        let labels = await controller.previewViewController.runButtonLabels()
        XCTAssertEqual(labels, "[]", "no block runs")

        // The Slide menu's item, for the cursor's slide.
        let deckWindow = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        let item = NSMenuItem(title: "Allow Driver in This Deck", action: #selector(DeckWindowController.allowDriverInThisDeck(_:)), keyEquivalent: "")
        XCTAssertTrue(deckWindow.validateMenuItem(item))
        XCTAssertEqual(item.title, "Allow sqlite in This Deck")
        controller.jumpToSlide(number: 1)
        XCTAssertFalse(deckWindow.validateMenuItem(item), "slide 1 has nothing to fix")
        controller.jumpToSlide(number: 4)
        deckWindow.allowDriverInThisDeck(item)
        XCTAssertTrue(editor.string.hasPrefix("---\ntitle: No Drivers\ndrivers:\n  sqlite: {}\n---\n"), "the list, as one undo step: \(editor.string.prefix(60))")
        XCTAssertEqual(editor.undoManager?.undoActionName, "Allow sqlite in This Deck")
        try await waitUntil(timeout: 10, "tap to accept it") { editor.boxes[3].slide.codeBlocks.first?.problem == nil }
        XCTAssertEqual(editor.deckErrors, [])

        // The box's context menu carries the same item while the problem is there.
        editor.undoManager?.undo()
        try await waitUntil(timeout: 15, "the problem back") { editor.boxes[3].slide.codeBlocks.first?.problem != nil }
        let menu = try XCTUnwrap(controller.editor(editor, contextMenuForBoxAt: 3))
        XCTAssertEqual(menu.items.last?.title, "Allow sqlite in This Deck")
        XCTAssertEqual(menu.items.last?.representedObject as? String, "sqlite")

    }
}
