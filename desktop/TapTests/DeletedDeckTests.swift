import XCTest
@testable import Tap

final class DeletedDeckTests: HostedTestCase {
    func testDeckDeletedWhileOpen() async throws {
        let deck = try Fixtures.copyDeck("plain.md")
        let document = try await openDeck(deck)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let text = controller.editor.string

        try FileManager.default.removeItem(at: deck)
        try await waitUntil(timeout: 10, "the deleted bar") { controller.editorViewController.bar(.deleted) != nil }
        let bar = try XCTUnwrap(controller.editorViewController.bar(.deleted))
        XCTAssertEqual(bar.message, "plain.md was deleted.")
        XCTAssertNotNil(bar.button(titled: "Save As…"))
        XCTAssertEqual(controller.editor.string, text, "the buffer stays")
        XCTAssertTrue(document.isDocumentEdited, "the buffer is an unsaved document")
        XCTAssertNil(document.fileURL)
        XCTAssertEqual(document.displayName, "plain")
        XCTAssertFalse(FileManager.default.fileExists(atPath: deck.path), "autosave does not bring the file back")

        // Save As puts the deck somewhere new, and tap follows it.
        let saved = deck.deletingLastPathComponent().appendingPathComponent("saved.md")
        try await document.save(to: saved, ofType: "net.daringfireball.markdown", for: .saveAsOperation)
        XCTAssertEqual(document.fileURL.map(FilePaths.canonical), FilePaths.canonical(saved))
        XCTAssertNil(controller.editorViewController.bar(.deleted))
        try await waitUntil(timeout: 30, "tap on the new path") {
            if case .running = controller.session.state { return FilePaths.same(controller.session.deckURL, saved) } else { return false }
        }
    }

    func testDeckDeletedOrMovedWhileOpen() async throws {
        let deck = try Fixtures.copyDeck("plain.md")
        let document = try await openDeck(deck)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let renamed = deck.deletingLastPathComponent().appendingPathComponent("renamed.md")

        // Finder renames with file coordination, as other programs should.
        // item(at:willMoveTo:)/item(at:didMoveTo:) must be called on the same
        // NSFileCoordinator instance that opened this coordination: calling
        // them on a second, unrelated instance throws "may only be invoked
        // from within a block passed to a -coordinate... method", and that
        // exception unwinding through this async test's Swift concurrency
        // task frame corrupts the task allocator and aborts the process.
        let coordinator = NSFileCoordinator(filePresenter: nil)
        var coordinationError: NSError?
        coordinator.coordinate(writingItemAt: deck, options: .forMoving,
                               writingItemAt: renamed, options: .forReplacing,
                               error: &coordinationError) { source, destination in
            coordinator.item(at: source, willMoveTo: destination)
            try? FileManager.default.moveItem(at: source, to: destination)
            coordinator.item(at: source, didMoveTo: destination)
        }
        XCTAssertNil(coordinationError)

        try await waitUntil(timeout: 10, "the document to follow") { document.fileURL.map { FilePaths.same($0, renamed) } ?? false }
        XCTAssertNil(controller.editorViewController.bar(.deleted), "a move is not a deletion")
        try await waitUntil(timeout: 30, "tap on the new path") {
            if case .running = controller.session.state { return FilePaths.same(controller.session.deckURL, renamed) } else { return false }
        }
    }

    /// A Save As of a deck that was never deleted also moves tap to the new
    /// path, and the old file is left an inert copy: a write straight to it
    /// afterward must not reach the render, only a write to the new path can.
    func testSaveAsWhileOpenMovesTapToTheNewPathAndStopsFollowingTheOldOne() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)

        let moved = deck.deletingLastPathComponent().appendingPathComponent("moved.md")
        try await document.save(to: moved, ofType: "net.daringfireball.markdown", for: .saveAsOperation)
        XCTAssertEqual(document.fileURL.map(FilePaths.canonical), FilePaths.canonical(moved))
        try await waitUntil(timeout: 30, "tap on the new path") {
            if case .running = controller.session.state { return FilePaths.same(controller.session.deckURL, moved) } else { return false }
        }

        // Write straight to the old file, bypassing the editor entirely.
        // tap's process now runs against "moved.md", so this must never
        // reach the render.
        let stillOld = try String(contentsOf: deck, encoding: .utf8)
            .replacingOccurrences(of: "# The Page", with: "# Should Not Appear")
        try stillOld.write(to: deck, atomically: false, encoding: .utf8)
        try await Task.sleep(nanoseconds: 2_000_000_000)
        XCTAssertFalse(controller.editor.boxes.contains { $0.slide.title == "Should Not Appear" },
                       "a write to the old file must not reach a render that now watches the new one")

        // The new file is the one that actually drives tap now.
        let stillNew = try String(contentsOf: moved, encoding: .utf8)
            .replacingOccurrences(of: "# The Page", with: "# The Renamed File Works")
        try stillNew.write(to: moved, atomically: false, encoding: .utf8)
        try await waitUntil(timeout: 10, "tap's boxes for the new file's own change") {
            controller.editor.boxes.contains { $0.slide.title == "The Renamed File Works" }
        }
    }

    /// A shown conflict bar and a real deletion cannot both describe the
    /// deck's file at once, and the deletion is the more urgent fact.
    func testDeletionWinsOverAShownConflict() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        controller.editor.moveCursor(toSlide: 5)
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))
        let mine = controller.editor.string
        try mine.replacingOccurrences(of: " mine", with: " theirs").write(to: deck, atomically: false, encoding: .utf8)
        controller.diskChanged()
        XCTAssertNotNil(controller.editorViewController.bar(.changedOnDisk))
        XCTAssertTrue(controller.hasDiskConflict)

        try FileManager.default.removeItem(at: deck)
        try await waitUntil(timeout: 10, "the deleted bar") { controller.editorViewController.bar(.deleted) != nil }
        XCTAssertNil(controller.editorViewController.bar(.changedOnDisk), "the deletion wins over a shown conflict")
        XCTAssertFalse(controller.hasDiskConflict)
    }

    /// NSDocument's safe save writes a temporary file and renames it over
    /// the deck's own file, which the raw DispatchSource watcher sees as a
    /// delete or a rename on every autosave. That must never be mistaken for
    /// a real deletion, and the watcher must still be following the file
    /// afterward, so a later, genuine deletion is still caught. No further
    /// edit happens after the two autosaves, so the document is not edited
    /// and nothing schedules another autosave. The final deletion is a raw
    /// `unlink`, bypassing NSFileManager's own file coordination (a real
    /// `rm` in a terminal bypasses it the same way, which is the whole
    /// reason the raw watcher exists); confirmed directly that this does not
    /// reach `presentedItemDidChange` either. With tap still running, its
    /// own `file-changed` event also independently reports a deletion
    /// through `diskChanged()`, so `session.onEvent` is cut here to isolate
    /// what this test is actually about: whether the raw watcher's own
    /// rearm, on its own, still catches the deletion.
    func testAutosaveIsNotMistakenForADeletion() async throws {
        let originalDelay = NSDocumentController.shared.autosavingDelay
        NSDocumentController.shared.autosavingDelay = 0.3
        defer { NSDocumentController.shared.autosavingDelay = originalDelay }

        let deck = try Fixtures.copyDeck("plain.md")
        let document = try await openDeck(deck)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        window.makeFirstResponder(controller.editor)

        controller.editor.setSelectedRange(NSRange(location: (controller.editor.string as NSString).length, length: 0))
        controller.editor.insertText(" one", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitUntil(timeout: 6, "the first autosave") { !document.isDocumentEdited }
        XCTAssertNil(controller.editorViewController.bar(.deleted), "the safe save's own rename must not look like a deletion")

        controller.editor.insertText(" two", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitUntil(timeout: 6, "the second autosave") { !document.isDocumentEdited }
        XCTAssertNil(controller.editorViewController.bar(.deleted), "a second safe save must not look like a deletion either")
        XCTAssertTrue(FileManager.default.fileExists(atPath: deck.path))
        XCTAssertFalse(document.isDocumentEdited, "nothing left unsaved: no further autosave can run")

        controller.session.onEvent = nil
        XCTAssertEqual(unlink(deck.path), 0, "the raw unlink itself succeeded")
        try await waitUntil(timeout: 10, "the deleted bar") { controller.editorViewController.bar(.deleted) != nil }
    }

    /// An empty, freshly opened deck is not edited, but a deletion must
    /// still mark it edited: the "no file" state is a distinct fact from
    /// whatever the buffer's text happens to be, not something inferred by
    /// comparing text that could coincidentally already match.
    func testEmptyDeckDeletedWhileUneditedStillReadsEdited() async throws {
        let folder = try Fixtures.temporaryFolder()
        let deck = folder.appendingPathComponent("empty.md")
        try "".write(to: deck, atomically: true, encoding: .utf8)
        let document = try await openDeck(deck)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        XCTAssertEqual(controller.editor.string, "")
        XCTAssertFalse(document.isDocumentEdited, "an empty, freshly opened deck is not edited")

        try FileManager.default.removeItem(at: deck)
        try await waitUntil(timeout: 10, "the deleted bar") { controller.editorViewController.bar(.deleted) != nil }
        XCTAssertTrue(document.isDocumentEdited, "a deleted document reads as edited even when its content was empty and unedited")
    }

    /// Once the deck's file is deleted, tap is not restarted if it exits,
    /// and the preview shows a paused message instead of trying to
    /// reconnect.
    func testDeletedDeckPausesTapAndShowsAPausedPreview() async throws {
        let deck = try Fixtures.copyDeck("plain.md")
        let document = try await openDeck(deck)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let pid = try XCTUnwrap(controller.session.processIdentifier)

        try FileManager.default.removeItem(at: deck)
        try await waitUntil(timeout: 10, "the deleted bar") { controller.editorViewController.bar(.deleted) != nil }
        XCTAssertFalse(controller.session.restartsWhenExited, "tap is not restarted once the deck's file is gone")

        kill(pid, SIGKILL)
        try await waitUntil(timeout: 10, "tap to stop without restarting") {
            if case .stopped = controller.session.state { return true } else { return false }
        }
        // A real restart would happen well within this if it were going to;
        // give it every chance before proving it did not.
        try await Task.sleep(nanoseconds: 500_000_000)
        if case .running = controller.session.state { XCTFail("tap must not restart while the deck's file is deleted") }

        try await waitUntil(timeout: 5, "the paused preview overlay") { !controller.previewViewController.overlay.isHidden }
        XCTAssertEqual(controller.previewViewController.overlay.titleLabel.stringValue, "The preview is paused")
        XCTAssertEqual(controller.previewViewController.overlay.detailLabel.stringValue, "Save the deck to see the preview.")
    }
}
