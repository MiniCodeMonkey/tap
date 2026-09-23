import XCTest
@testable import Tap

final class AutosaveTests: HostedTestCase {
    func testAutosaveInPlace() async throws {
        let deck = try Fixtures.copyDeck("plain.md")
        let document = try await openDeck(deck)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        window.makeFirstResponder(controller.editor)

        controller.editor.setSelectedRange(NSRange(location: (controller.editor.string as NSString).length, length: 0))
        controller.editor.insertText("A new line.", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitUntil(timeout: 2, "the edit to count") { document.isDocumentEdited }

        try await waitUntil(timeout: 6, "the autosave") {
            !document.isDocumentEdited && ((try? String(contentsOf: deck, encoding: .utf8))?.contains("A new line.") ?? false)
        }
        XCTAssertFalse(window.isDocumentEdited, "the title bar shows the document as saved")
        try await waitUntil(timeout: 2, "tap to hear about the save") { controller.session.log.text.contains("saved the deck") }

        // "saved the deck" only proves the app called documentDidSave, not
        // that tap received "saved". tap keeps rendering a buffer until it
        // does, and ignores changes to the deck file on disk while a buffer
        // is live (see appDeckSource.dropBuffer in internal/cli/app_source.go).
        // A write straight to the file, bypassing the editor, is something
        // tap only reacts to once the buffer is gone: it re-renders from
        // disk, and a live code block with an undeclared driver produces a
        // warning on tap's own stderr naming that driver. If
        // session.send(.saved) were removed, the buffer would stay live
        // forever and tap would never render this write, so this warning
        // would never appear.
        let saved = try XCTUnwrap(try? String(contentsOf: deck, encoding: .utf8))
        let withUndeclaredDriver = saved + "\n\n---\n\n# Query\n\n```sql {driver: sqlite}\nSELECT 1;\n```\n"
        try withUndeclaredDriver.write(to: deck, atomically: true, encoding: .utf8)
        try await waitUntil(timeout: 6, "tap to render the disk write and warn about the undeclared sqlite driver") {
            controller.session.log.text.contains("does not declare the sqlite driver")
        }
    }

    /// Save To writes a copy elsewhere, not the deck file tap watches, so it
    /// must not tell tap anything was saved.
    func testSaveToDoesNotTellTap() async throws {
        let deck = try Fixtures.copyDeck("plain.md")
        let document = try await openDeck(deck)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let copy = deck.deletingLastPathComponent().appendingPathComponent("a-copy.md")

        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            document.save(to: copy, ofType: "net.daringfireball.markdown", for: .saveToOperation) { error in
                if let error { continuation.resume(throwing: error) } else { continuation.resume() }
            }
        }
        XCTAssertTrue(FileManager.default.fileExists(atPath: copy.path), "Save To wrote the copy")
        XCTAssertFalse(controller.session.log.text.contains("saved the deck"),
                       "Save To must not tell tap the deck file was saved")
    }

    /// The edited flag follows undo and redo, checked synchronously right
    /// after each call. Autosave is held off for the whole test: on a
    /// machine under heavy load, the ordinary 1 second delay is not a
    /// reliable guarantee that autosave cannot slip in between two
    /// consecutive lines of test code and confuse what is being measured.
    func testUndoAndRedoTrackTheEditedFlag() async throws {
        let originalDelay = NSDocumentController.shared.autosavingDelay
        NSDocumentController.shared.autosavingDelay = 300
        defer { NSDocumentController.shared.autosavingDelay = originalDelay }

        let deck = try Fixtures.copyDeck("plain.md")
        let document = try await openDeck(deck)
        _ = try await waitForRunningTap(document)
        XCTAssertFalse(document.isDocumentEdited, "opening a deck does not mark it edited")

        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        window.makeFirstResponder(controller.editor)
        XCTAssertTrue(controller.editor.undoManager === document.undoManager,
                     "the editor's undo manager is the document's own")

        let originalText = controller.editor.string
        controller.editor.setSelectedRange(NSRange(location: (controller.editor.string as NSString).length, length: 0))
        controller.editor.insertText("A new line.", replacementRange: NSRange(location: NSNotFound, length: 0))
        // Closes the coalesced typing group immediately, rather than waiting
        // for the run loop's own event boundary, so undo has something
        // committed to act on right away.
        controller.editor.breakUndoCoalescing()
        XCTAssertTrue(document.isDocumentEdited, "typing marks the document edited")

        controller.editor.undoManager?.undo()
        XCTAssertEqual(controller.editor.string, originalText, "undo actually reverted the text")
        XCTAssertFalse(document.isDocumentEdited, "the document is not edited immediately after undo, before autosave could run")

        controller.editor.undoManager?.redo()
        XCTAssertTrue(controller.editor.string.contains("A new line."), "redo actually reapplied the text")
        XCTAssertTrue(document.isDocumentEdited, "redo marks the document edited again")
    }
}
