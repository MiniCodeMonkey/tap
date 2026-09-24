import XCTest
@testable import Tap

final class ExternalChangeTests: HostedTestCase {
    func writeOutside(_ text: String, to url: URL) throws {
        try text.write(to: url, atomically: false, encoding: .utf8)
    }

    func testExternalChangeWithNoUnsavedEdits() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let before = controller.editor.string
        controller.editor.moveCursor(toSlide: 4)
        XCTAssertFalse(document.isDocumentEdited)

        let changed = before.replacingOccurrences(of: "# The Page", with: "# The Pages, renamed")
            .replacingOccurrences(of: "# Eleven Minutes", with: "# Twelve Minutes")
        try writeOutside(changed, to: deck)

        try await waitUntil(timeout: 10, "the disk version") { controller.editor.string == changed }
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Load Disk Version")
        XCTAssertFalse(document.isDocumentEdited, "the document reads as not edited right after loading")
        try await waitUntil(timeout: 10, "tap's boxes for it") { controller.editor.boxes.count == 7 && controller.editor.boxes[1].slide.title == "The Pages, renamed" }
        XCTAssertEqual(controller.editor.currentBoxIndex, 4, "the cursor stays on slide 5")
        XCTAssertNil(controller.editorViewController.bar(.changedOnDisk))

        controller.editor.undoManager?.undo()
        XCTAssertEqual(controller.editor.string, before, "one undo returns to the text before the change")
        XCTAssertTrue(document.isDocumentEdited, "an undo of the load leaves it edited")
    }

    func testExternalChangeWithUnsavedEdits() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        controller.editor.moveCursor(toSlide: 5)
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))
        let mine = controller.editor.string
        try writeOutside(mine.replacingOccurrences(of: " mine", with: " theirs"), to: deck)

        try await waitUntil(timeout: 10, "the changed on disk bar") { controller.editorViewController.bar(.changedOnDisk) != nil }
        let bar = try XCTUnwrap(controller.editorViewController.bar(.changedOnDisk))
        XCTAssertEqual(bar.message, "seven-slides.md changed on disk.")
        XCTAssertNotNil(bar.button(titled: "Load Disk Version"))
        XCTAssertNotNil(bar.button(titled: "Keep Mine"))
        XCTAssertEqual(controller.editor.string, mine, "nothing is loaded until the user chooses")
        XCTAssertNil(document.windowControllers.first?.window?.attachedSheet, "no alert")

        bar.button(titled: "Load Disk Version")?.performClick(nil)
        XCTAssertTrue(controller.editor.string.contains(" theirs"))
        XCTAssertNil(controller.editorViewController.bar(.changedOnDisk))
        XCTAssertFalse(document.isDocumentEdited, "Load Disk Version from the bar leaves it not edited")
    }

    func testKeepMine() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        controller.editor.moveCursor(toSlide: 5)
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))
        let mine = controller.editor.string
        try writeOutside(mine.replacingOccurrences(of: " mine", with: " theirs"), to: deck)
        try await waitUntil(timeout: 10, "the changed on disk bar") { controller.editorViewController.bar(.changedOnDisk) != nil }

        controller.editorViewController.bar(.changedOnDisk)?.button(titled: "Keep Mine")?.performClick(nil)
        try await waitUntil(timeout: 5, "my buffer on disk") { (try? String(contentsOf: deck, encoding: .utf8)) == mine }
        XCTAssertNil(controller.editorViewController.bar(.changedOnDisk))
        try await waitUntil(timeout: 5, "the saved state") { !document.isDocumentEdited }
        XCTAssertNil(document.windowControllers.first?.window?.attachedSheet, "no alert about the file changing")
    }

    func testExternalChangeToFrontmatter() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let before = controller.editor.string
        XCTAssertFalse(document.isDocumentEdited)

        // The frontmatter is the hidden prefix before slide 1; the outside
        // change edits it, and the editor's clamp on user edits must not
        // stop this app-driven load from reaching it.
        let changed = before.replacingOccurrences(of: "title: Seven Slides", with: "title: Renamed From Outside")
        try writeOutside(changed, to: deck)

        try await waitUntil(timeout: 10, "the disk version") { controller.editor.string == changed }
        XCTAssertEqual(controller.editor.string, changed, "the frontmatter change reached the editor")
        XCTAssertFalse(document.isDocumentEdited)
    }

    func testExternalChangeMatchingUnsavedEditsClearsTheEditedFlag() async throws {
        // Autosave, one second after an edit, would write this same text and
        // clear the flag on its own, which would hide whatever diskChanged
        // does. It is held off well past this test's own waits so the only
        // thing that can clear the flag here is diskChanged's own branch for
        // a disk text that already equals the editor's.
        let originalDelay = NSDocumentController.shared.autosavingDelay
        NSDocumentController.shared.autosavingDelay = 60
        defer { NSDocumentController.shared.autosavingDelay = originalDelay }

        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        controller.editor.moveCursor(toSlide: 5)
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))
        let mine = controller.editor.string
        XCTAssertTrue(document.isDocumentEdited)

        // Something outside writes exactly the buffer's own text: the file
        // now already matches what is open, so the edited flag must clear
        // even though nothing was loaded or saved through this document.
        try writeOutside(mine, to: deck)

        try await waitUntil(timeout: 10, "the edited flag to clear") { !document.isDocumentEdited }
        XCTAssertEqual(controller.editor.string, mine, "the buffer itself is untouched")
        XCTAssertNil(controller.editorViewController.bar(.changedOnDisk))
    }
}
