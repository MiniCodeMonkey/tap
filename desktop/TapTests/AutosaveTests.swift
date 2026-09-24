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

    /// The edited flag is derived from comparing the editor's text against
    /// the deck file's content, not from counting changes, so it agrees
    /// with reality regardless of how undo groups keystrokes. Every
    /// assertion runs synchronously right after the step it follows.
    /// Autosave is held off for the whole test: on a machine under heavy
    /// load, the ordinary 1 second delay is not a reliable guarantee that
    /// autosave cannot slip in between two consecutive lines of test code
    /// and confuse what is being measured.
    func testEditedFlagFollowsUndoAndRedo() async throws {
        let originalDelay = NSDocumentController.shared.autosavingDelay
        NSDocumentController.shared.autosavingDelay = 300
        defer { NSDocumentController.shared.autosavingDelay = originalDelay }

        // A. Opening a deck leaves it not edited.
        let a = try await openScenarioDeck()
        XCTAssertFalse(a.document.isDocumentEdited, "A: opening a deck does not mark it edited")

        // B. Type, undo: not edited.
        let b = try await openScenarioDeck()
        let originalTextB = b.controller.editor.string
        typeText("A new line.", into: b.controller.editor)
        XCTAssertTrue(b.document.isDocumentEdited, "B: typing marks the document edited")
        b.controller.editor.undoManager?.undo()
        XCTAssertEqual(b.controller.editor.string, originalTextB, "B: undo actually reverted the text")
        XCTAssertFalse(b.document.isDocumentEdited, "B: undoing back to the file's own text clears the edited flag")

        // C. Type, save, undo: edited (the file now holds the typed text,
        // so undoing away from it is what makes the document edited).
        let c = try await openScenarioDeck()
        let originalTextC = c.controller.editor.string
        typeText("A new line.", into: c.controller.editor)
        try await saveInPlace(c.document, to: c.deck)
        XCTAssertFalse(c.document.isDocumentEdited, "C: saving clears the edited flag")
        c.controller.editor.undoManager?.undo()
        XCTAssertEqual(c.controller.editor.string, originalTextC, "C: undo actually reverted the text")
        XCTAssertTrue(c.document.isDocumentEdited, "C: undoing past a save marks the document edited again")

        // D. ...redo: not edited (redo returns to what was saved).
        c.controller.editor.undoManager?.redo()
        XCTAssertTrue(c.controller.editor.string.contains("A new line."), "D: redo actually reapplied the text")
        XCTAssertFalse(c.document.isDocumentEdited, "D: redoing back to the saved text clears the edited flag")

        // E. Type three characters, undo until the text equals the file
        // again: not edited, whatever the grouping happens to be.
        let e = try await openScenarioDeck()
        let originalTextE = e.controller.editor.string
        typeText("xyz", into: e.controller.editor)
        XCTAssertTrue(e.document.isDocumentEdited, "E: typing marks the document edited")
        while e.controller.editor.string != originalTextE, e.controller.editor.undoManager?.canUndo == true {
            e.controller.editor.undoManager?.undo()
        }
        XCTAssertEqual(e.controller.editor.string, originalTextE, "E: undo eventually reverted every character")
        XCTAssertFalse(e.document.isDocumentEdited, "E: not edited once the text is back to the file's, regardless of grouping")

        // F. Two edits in separate undo groups, undo once: still edited;
        // undo again: not edited. Tried with real NSEvent key delivery
        // through NSApp.sendEvent, with breakUndoCoalescing and a run loop
        // turn between the two keystrokes, so the outer undo group would
        // close the way it does for a person typing: it still did not
        // produce two groups, an undo reverted both characters at once
        // every time. NSTextView's typing coalescing merges a run of plain
        // character insertions into one "Undo Typing" action regardless of
        // event boundaries; only a non-typing interruption breaks it, not
        // a bare breakUndoCoalescing call or a run loop turn on their own.
        // This matches fix round 2's own finding for insertText, now also
        // confirmed for genuine per-event keystrokes. So this scenario is
        // kept grouping-agnostic, per the brief's fallback: it checks the
        // actual invariant the ruling cares about, that isDocumentEdited
        // always agrees with whether the text differs from the file, at
        // every step, regardless of how many undo groups the edits landed
        // in.
        let f = try await openScenarioDeck()
        let originalTextF = f.controller.editor.string
        typeText("x", into: f.controller.editor)
        typeText("y", into: f.controller.editor)
        XCTAssertEqual(f.controller.editor.string, originalTextF + "xy", "F: both edits landed")
        XCTAssertTrue(f.document.isDocumentEdited, "F: edited while the text differs from the file")
        var sawEditedWhileDiffering = f.document.isDocumentEdited
        while f.controller.editor.string != originalTextF, f.controller.editor.undoManager?.canUndo == true {
            f.controller.editor.undoManager?.undo()
            let stillDiffers = f.controller.editor.string != originalTextF
            XCTAssertEqual(f.document.isDocumentEdited, stillDiffers,
                          "F: the edited flag agrees with whether the text still differs from the file, after undo")
            sawEditedWhileDiffering = sawEditedWhileDiffering || (stillDiffers && f.document.isDocumentEdited)
        }
        XCTAssertTrue(sawEditedWhileDiffering, "F: the document was edited at least once while the text differed from the file")
        XCTAssertFalse(f.document.isDocumentEdited, "F: not edited once every undo is exhausted and the text is back to the file's")
    }

    private struct Scenario {
        let document: DeckDocument
        let controller: DeckSessionController
        let deck: URL
    }

    private func openScenarioDeck() async throws -> Scenario {
        let deck = try Fixtures.copyDeck("plain.md")
        let document = try await openDeck(deck)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        window.makeFirstResponder(controller.editor)
        return Scenario(document: document, controller: controller, deck: deck)
    }

    private func typeText(_ text: String, into editor: EditorTextView) {
        editor.setSelectedRange(NSRange(location: (editor.string as NSString).length, length: 0))
        editor.insertText(text, replacementRange: NSRange(location: NSNotFound, length: 0))
        // Closes the coalesced typing group immediately, rather than
        // waiting for the run loop's own event boundary, so undo has
        // something committed to act on right away.
        editor.breakUndoCoalescing()
    }

    private func saveInPlace(_ document: DeckDocument, to url: URL) async throws {
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            document.save(to: url, ofType: "net.daringfireball.markdown", for: .saveOperation) { error in
                if let error { continuation.resume(throwing: error) } else { continuation.resume() }
            }
        }
    }
}
