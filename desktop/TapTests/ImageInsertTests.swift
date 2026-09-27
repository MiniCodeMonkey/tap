import XCTest
@testable import Tap

final class ImageInsertTests: HostedTestCase {
    let diagram = Fixtures.repositoryRoot.appendingPathComponent("desktop/TapTests/Fixtures/diagram.png")

    func openOps() async throws -> (DeckDocument, DeckSessionController, URL) {
        let deck = try Fixtures.copyDeck("ops.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "tap's answer") { controller.lastAppliedText == controller.editor.string }
        return (document, controller, deck)
    }

    /// A pasteboard of the test's own, for the image and the text branch alike.
    func privatePasteboard(_ controller: DeckSessionController) -> NSPasteboard {
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.paste.\(UUID().uuidString)"))
        pasteboard.clearContents()
        controller.editor.pasteboardForPaste = pasteboard
        return pasteboard
    }

    func testPasteAnImage() async throws {
        let (document, controller, deck) = try await openOps()
        controller.jumpToSlide(number: 3)
        let caret = controller.editor.selectedRange().location
        // Three selected characters survive: the insert goes at the caret, replacing nothing.
        controller.editor.setSelectedRange(NSRange(location: caret, length: 3))
        let selected = (controller.editor.string as NSString).substring(with: NSRange(location: caret, length: 3))
        let pasteboard = privatePasteboard(controller)
        pasteboard.writeObjects([diagram as NSURL])

        controller.editor.paste(nil)
        try await waitUntil(timeout: 20, "the markdown") { controller.editor.string.contains("images/diagram.png") }
        let imagesFolder = deck.deletingLastPathComponent().appendingPathComponent("images")
        XCTAssertTrue(FileManager.default.fileExists(atPath: imagesFolder.appendingPathComponent("diagram.png").path), "tap copied it next to the deck")
        let text = controller.editor.string as NSString
        let inserted = "![diagram](images/diagram.png)\n"
        XCTAssertEqual(text.range(of: inserted).location, caret, "inserted at the caret, tap's markdown as printed")
        XCTAssertEqual(text.substring(with: NSRange(location: caret + (inserted as NSString).length, length: 3)), selected, "the selection was not replaced")
        XCTAssertEqual(controller.currentSlideNumber, 3)
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Insert Image")
        XCTAssertTrue(document.isDocumentEdited, "the buffer changed; tap did not touch the deck file")

        // A second paste of the same file: tap names it diagram-2.png.
        controller.editor.paste(nil)
        try await waitUntil(timeout: 20, "the second markdown") { controller.editor.string.contains("images/diagram-2.png") }
        XCTAssertTrue(FileManager.default.fileExists(atPath: imagesFolder.appendingPathComponent("diagram-2.png").path))

        // Not an image: tap refuses, the app shows tap's words and inserts nothing.
        let notes = try Fixtures.temporaryFolder().appendingPathComponent("notes.txt")
        try "not an image".write(to: notes, atomically: true, encoding: .utf8)
        let before = controller.editor.string
        controller.insertImages([notes])
        try await waitUntil(timeout: 20, "tap's refusal") { controller.editorViewController.bar(.toolFailed) != nil }
        XCTAssertEqual(controller.editorViewController.bar(.toolFailed)?.message, "Insert Image failed.")
        XCTAssertEqual(controller.editor.string, before)

        controller.editor.undoManager?.undo()
        controller.editor.undoManager?.undo()
        XCTAssertFalse(controller.editor.string.contains("images/diagram"), "two undo steps take both inserts back")
    }

    /// Edit > Paste, the item Command-V presses: the one in the app's main menu.
    func pasteMenuItem() throws -> NSMenuItem {
        let items = NSApp.mainMenu?.items.compactMap(\.submenu).flatMap(\.items) ?? []
        return try XCTUnwrap(items.first { $0.action == #selector(NSText.paste(_:)) }, "Edit > Paste")
    }

    /// A screenshot (Command-Control-Shift-4) puts PNG data alone on the
    /// pasteboard, with no string: Edit > Paste is on for it, and AppKit's
    /// own path from the menu item reaches the editor's paste.
    func testPastedImageDataBecomesAFile() async throws {
        let (_, controller, deck) = try await openOps()
        controller.jumpToSlide(number: 2)
        let pasteboard = privatePasteboard(controller)
        let pasteItem = try pasteMenuItem()
        XCTAssertFalse(controller.editor.validateMenuItem(pasteItem), "nothing on the pasteboard: Paste is off")
        pasteboard.setData(try Data(contentsOf: diagram), forType: .png)
        XCTAssertNil(pasteboard.availableType(from: [.string]), "image data alone, as a screenshot leaves it")

        XCTAssertTrue(controller.editor.validateMenuItem(pasteItem), "Paste is on for image data, so Command-V is not a beep")
        XCTAssertTrue(NSApp.sendAction(try XCTUnwrap(pasteItem.action), to: controller.editor, from: pasteItem))
        try await waitUntil(timeout: 20, "the markdown") { controller.editor.string.contains("images/pasted-image.png") }
        XCTAssertTrue(FileManager.default.fileExists(atPath: deck.deletingLastPathComponent().appendingPathComponent("images/pasted-image.png").path))
    }

    func testPlainTextPasteIsStillText() async throws {
        let (_, controller, _) = try await openOps()
        controller.jumpToSlide(number: 2)
        let pasteboard = privatePasteboard(controller)
        pasteboard.setString("plain words", forType: .string)
        controller.editor.paste(nil)
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertTrue(controller.editor.string.contains("plain words"), "the text branch reads the same pasteboard, never the person's clipboard")
    }

    func testDropAnImageFile() async throws {
        let (_, controller, deck) = try await openOps()
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.drop.\(UUID().uuidString)"))
        pasteboard.clearContents()
        pasteboard.writeObjects([diagram as NSURL, URL(fileURLWithPath: "/tmp/notes.txt") as NSURL])
        XCTAssertEqual(EditorTextView.imageFileURLs(on: pasteboard), [diagram], "only files with an extension tap accepts")
        let editor = controller.editor
        // The drop lands on slide 4's first line: the caret moves there, then tap image add.
        let box = try XCTUnwrap(editor.boxes.first { $0.slide.number == 4 })
        let point = editor.pointForCharacter(at: box.range.location)
        let info = FakeDraggingInfo(pasteboard: pasteboard, location: editor.convert(point, to: nil), source: nil, window: editor.window)
        XCTAssertEqual(editor.draggingEntered(info), .copy)
        XCTAssertTrue(editor.prepareForDragOperation(info), "an image drop is accepted before NSTextView could refuse a pasteboard with no text")
        XCTAssertTrue(editor.performDragOperation(info))
        try await waitUntil(timeout: 20, "the markdown") { controller.editor.string.contains("images/diagram.png") }
        XCTAssertEqual((controller.editor.string as NSString).range(of: "![diagram](images/diagram.png)").location, box.range.location, "at the drop point")
        XCTAssertTrue(FileManager.default.fileExists(atPath: deck.deletingLastPathComponent().appendingPathComponent("images/diagram.png").path))
    }

    func testPasteIntoAnUnsavedDeckIsRefused() async throws {
        let (_, controller, deck) = try await openOps()
        try FileManager.default.removeItem(at: deck)
        try await waitUntil(timeout: 10, "the deck to read as gone") { controller.document?.fileURL == nil || !FileManager.default.fileExists(atPath: deck.path) }
        controller.insertImages([diagram])
        try await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertFalse(controller.editor.string.contains("images/diagram.png"))
        XCTAssertTrue(controller.session.log.text.contains("Insert Image needs a saved deck"), controller.session.log.text)
    }
}
