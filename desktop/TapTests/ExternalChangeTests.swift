import XCTest
@testable import Tap

/// The delegate object `canClose(withDelegate:shouldClose:contextInfo:)`
/// expects: it calls back on this selector, not with a return value or a
/// completion handler.
private final class CanCloseSpy: NSObject {
    private(set) var results: [Bool] = []

    @objc func document(_ document: NSDocument, shouldClose: Bool, contextInfo: UnsafeMutableRawPointer?) {
        results.append(shouldClose)
    }
}

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

        // Forces the app's PUT to land at tap before the outside write below,
        // rather than leaving the two racing: tap must still tell the app
        // about a disk write that lands on the buffer it already has, not
        // only one that lands while tap still has the buffer from before.
        await controller.sourceSync.sendNow()

        // Something outside writes exactly the buffer's own text: the file
        // now already matches what is open, so the edited flag must clear
        // even though nothing was loaded or saved through this document.
        try writeOutside(mine, to: deck)

        try await waitUntil(timeout: 10, "the edited flag to clear") { !document.isDocumentEdited }
        XCTAssertEqual(controller.editor.string, mine, "the buffer itself is untouched")
        XCTAssertNil(controller.editorViewController.bar(.changedOnDisk))
    }

    /// Drives `diskChanged()` directly rather than through tap's own
    /// `file-changed` event: tap keeps rendering its own live buffer and is
    /// not obliged to report every successive disk write to the same file
    /// promptly, which made a version of this test that waited on a second
    /// real tap event flaky under load. `diskChanged()` is the exact method
    /// the event handler calls, so this tests the same logic directly.
    func testDiskConvergingWhileAConflictIsShowingClearsIt() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        controller.editor.moveCursor(toSlide: 5)
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))
        let mine = controller.editor.string
        try writeOutside(mine.replacingOccurrences(of: " mine", with: " theirs"), to: deck)
        controller.diskChanged()
        XCTAssertNotNil(controller.editorViewController.bar(.changedOnDisk))
        XCTAssertTrue(controller.hasDiskConflict)

        // The file converges on exactly what is already open, without a
        // person ever resolving the bar: the stale conflict must not
        // survive to block a later autosave.
        try writeOutside(mine, to: deck)
        controller.diskChanged()

        XCTAssertNil(controller.editorViewController.bar(.changedOnDisk))
        XCTAssertFalse(controller.hasDiskConflict, "the conflict is cleared once the disk matches the buffer")
        XCTAssertFalse(document.isDocumentEdited)
    }

    func testPendingCursorSlideNumberDoesNotOutliveARefusedAnswer() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        controller.editor.moveCursor(toSlide: 4)

        let before = controller.editor.string
        let changed = before.replacingOccurrences(of: "# The Page", with: "# The Pages, renamed")
        try writeOutside(changed, to: deck)
        try await waitUntil(timeout: 10, "the disk version") { controller.editor.string == changed }
        // loadDiskVersion just set pendingCursorSlideNumber for slide 5.

        // A refused answer, computed from text this generation can no longer
        // describe, must not leave the pending slide number for some later,
        // unrelated slide list to pick up.
        controller.applySlideList(SlideList(slides: [], errors: []), sentText: "stale", generation: -1)

        // An edit to a different slide, and tap's real answer for it, must
        // not have the cursor hijacked back to the load's slide.
        controller.editor.moveCursor(toSlide: 0)
        controller.editor.insertText("x", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitUntil(timeout: 10, "tap's boxes for the new edit") { controller.editor.boxes.count == 7 }
        XCTAssertEqual(controller.editor.currentBoxIndex, 0, "a stale pending slide number from the load did not move the cursor")
    }

    func testExplicitSaveDuringAConflictDoesNotOverwrite() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        controller.editor.moveCursor(toSlide: 5)
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))
        let mine = controller.editor.string
        let theirs = mine.replacingOccurrences(of: " mine", with: " theirs")
        try writeOutside(theirs, to: deck)
        try await waitUntil(timeout: 10, "the changed on disk bar") { controller.editorViewController.bar(.changedOnDisk) != nil }

        // Cmd-S, through the exact action the menu item invokes.
        document.save(nil)

        // A real, unguarded save would complete well within this: give it
        // every chance to happen before asserting it did not.
        try await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertEqual(try String(contentsOf: deck, encoding: .utf8), theirs, "the other program's text is untouched")
        XCTAssertNotNil(controller.editorViewController.bar(.changedOnDisk), "the bar is still showing")
        XCTAssertTrue(document.isDocumentEdited, "still edited: nothing was saved")
        XCTAssertNil(document.windowControllers.first?.window?.attachedSheet, "no alert")
    }

    /// Closing a window or quitting with unsaved changes, for a document
    /// whose class declares `autosavesInPlace`, does not show the classic
    /// Save/Don't Save/Cancel alert or call `saveDocumentWithDelegate:` at
    /// all: NSDocument's own documented behavior for
    /// `canCloseDocumentWithDelegate:shouldCloseSelector:contextInfo:` is to
    /// invoke `autosaveWithImplicitCancellability(false, completionHandler:)`
    /// directly and treat a reported error as "do not close". This drives
    /// that exact call, which is the real close/quit path for this class,
    /// directly: no window is asked to close and no alert is driven, so this
    /// proves the write is blocked and the completion reports failure (which
    /// is what makes canClose refuse to close), not that a real window
    /// close or app quit is cancelled end to end. See the report for what
    /// that further step leaves unproven.
    func testCloseOrQuitAutosaveDuringAConflictDoesNotOverwrite() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        controller.editor.moveCursor(toSlide: 5)
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))
        let mine = controller.editor.string
        let theirs = mine.replacingOccurrences(of: " mine", with: " theirs")
        try writeOutside(theirs, to: deck)
        try await waitUntil(timeout: 10, "the changed on disk bar") { controller.editorViewController.bar(.changedOnDisk) != nil }

        let reportedError = await withCheckedContinuation { (continuation: CheckedContinuation<Error?, Never>) in
            document.autosave(withImplicitCancellability: false) { error in continuation.resume(returning: error) }
        }
        XCTAssertNotNil(reportedError, "canClose treats this as a failed save and does not close")
        XCTAssertEqual((reportedError as NSError?)?.code, CocoaError.userCancelled.rawValue, "the one error NSDocument treats as a silent refusal")
        XCTAssertEqual(try String(contentsOf: deck, encoding: .utf8), theirs, "the other program's text is untouched")
        XCTAssertNotNil(controller.editorViewController.bar(.changedOnDisk), "the bar is still showing")
        XCTAssertTrue(document.isDocumentEdited, "still edited: nothing was saved")
        XCTAssertNil(document.windowControllers.first?.window?.attachedSheet, "no alert")
    }

    /// Drives the real close/quit entry point twice in a row while a
    /// conflict is showing. The first refusal used to leave
    /// `isDocumentEdited` false as an undocumented AppKit side effect of
    /// refusing the close (confirmed in the re-review by instrumenting this
    /// exact call), even though nothing was written; the second attempt
    /// would then see a clean document and close right over the person's
    /// edits with no guard and no bar. Both attempts here must refuse to
    /// close, leave the file untouched, and read as edited afterward.
    func testCanCloseDuringAConflictDoesNotDiscardEditsOnASecondAttempt() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        controller.editor.moveCursor(toSlide: 5)
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))
        let mine = controller.editor.string
        let theirs = mine.replacingOccurrences(of: " mine", with: " theirs")
        try writeOutside(theirs, to: deck)
        try await waitUntil(timeout: 10, "the changed on disk bar") { controller.editorViewController.bar(.changedOnDisk) != nil }

        let spy = CanCloseSpy()
        let selector = #selector(CanCloseSpy.document(_:shouldClose:contextInfo:))

        document.canClose(withDelegate: spy, shouldClose: selector, contextInfo: nil)
        try await waitUntil(timeout: 5, "the first refusal") { spy.results.count == 1 }
        XCTAssertEqual(spy.results[0], false, "canClose refuses while the conflict is showing")
        XCTAssertEqual(try String(contentsOf: deck, encoding: .utf8), theirs, "the other program's text is untouched")
        XCTAssertNotNil(controller.editorViewController.bar(.changedOnDisk), "the bar is still showing")
        XCTAssertTrue(document.isDocumentEdited, "still edited after the first refused close")
        XCTAssertNil(document.windowControllers.first?.window?.attachedSheet, "no alert")

        document.canClose(withDelegate: spy, shouldClose: selector, contextInfo: nil)
        try await waitUntil(timeout: 5, "the second refusal") { spy.results.count == 2 }
        XCTAssertEqual(spy.results[1], false, "a second close attempt is refused the same way")
        XCTAssertEqual(try String(contentsOf: deck, encoding: .utf8), theirs, "still untouched")
        XCTAssertNotNil(controller.editorViewController.bar(.changedOnDisk), "the bar is still showing")
        XCTAssertTrue(document.isDocumentEdited, "still edited after the second refused close: nothing was silently discarded")
        XCTAssertNil(document.windowControllers.first?.window?.attachedSheet, "no alert")
    }

    /// Opens seven-slides.md, types " mine" on slide 6, writes " theirs" in
    /// its place from outside, and waits for the changed on disk bar.
    private func openDeckWithAShownConflict() async throws -> (deck: URL, document: DeckDocument, mine: String, theirs: String) {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        controller.editor.moveCursor(toSlide: 5)
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))
        let mine = controller.editor.string
        let theirs = mine.replacingOccurrences(of: " mine", with: " theirs")
        try writeOutside(theirs, to: deck)
        try await waitUntil(timeout: 10, "the changed on disk bar") { controller.editorViewController.bar(.changedOnDisk) != nil }
        XCTAssertTrue(controller.hasDiskConflict)
        return (deck, document, mine, theirs)
    }

    /// Asks the real close entry point whether the document may close, and
    /// returns its answer.
    private func canClose(_ document: DeckDocument) async throws -> Bool {
        let spy = CanCloseSpy()
        document.canClose(withDelegate: spy, shouldClose: #selector(CanCloseSpy.document(_:shouldClose:contextInfo:)), contextInfo: nil)
        try await waitUntil(timeout: 5, "the close answer") { spy.results.count == 1 }
        return spy.results[0]
    }

    /// A Save As while a conflict is showing writes the person's text to
    /// the new file and leaves the other program's text in the old one.
    /// The conflict described the old file, so it is gone, and the window
    /// can close.
    func testSaveAsDuringAConflictClearsIt() async throws {
        let (deck, document, mine, theirs) = try await openDeckWithAShownConflict()
        let controller = try XCTUnwrap(document.sessionController)

        let moved = deck.deletingLastPathComponent().appendingPathComponent("moved.md")
        try await document.save(to: moved, ofType: "net.daringfireball.markdown", for: .saveAsOperation)
        XCTAssertEqual(document.fileURL.map(FilePaths.canonical), FilePaths.canonical(moved))
        XCTAssertEqual(try String(contentsOf: deck, encoding: .utf8), theirs, "the other program's text stays in the old file")
        XCTAssertEqual(try String(contentsOf: moved, encoding: .utf8), mine, "the new file holds the person's text")
        XCTAssertFalse(controller.hasDiskConflict, "the conflict described the old file")
        XCTAssertNil(controller.editorViewController.bar(.changedOnDisk), "the bar naming the old file is gone")
        XCTAssertFalse(document.isDocumentEdited, "the new file holds the editor's text")
        let mayClose = try await canClose(document)
        XCTAssertTrue(mayClose, "the window can close")
    }

    /// Another program renaming the deck while a conflict is showing is not
    /// the person choosing their version: the conflict stays, the bar names
    /// the new file, and the periodic autosave still does not write the
    /// person's text over the other program's change at the new path.
    func testRenameByAnotherProgramDuringAConflictKeepsIt() async throws {
        let originalDelay = NSDocumentController.shared.autosavingDelay
        NSDocumentController.shared.autosavingDelay = 0.3
        defer { NSDocumentController.shared.autosavingDelay = originalDelay }
        let (deck, document, mine, theirs) = try await openDeckWithAShownConflict()
        let controller = try XCTUnwrap(document.sessionController)
        let renamed = deck.deletingLastPathComponent().appendingPathComponent("renamed.md")

        // One coordinator instance for the whole move, as in
        // DeletedDeckTests.testDeckDeletedOrMovedWhileOpen.
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

        // Long enough for several periodic autosaves at the delay above.
        try await Task.sleep(nanoseconds: 2_000_000_000)
        XCTAssertEqual(try String(contentsOf: renamed, encoding: .utf8), theirs, "the other program's text is still on disk at the new path")
        XCTAssertTrue(controller.hasDiskConflict, "only Keep Mine, Load Disk Version or the person's own Save As resolves the conflict")
        XCTAssertEqual(controller.editorViewController.bar(.changedOnDisk)?.message, "renamed.md changed on disk.", "the bar names the new file")
        XCTAssertEqual(controller.editor.string, mine, "the person's text is still in the editor")
        XCTAssertTrue(document.isDocumentEdited)
    }

    /// Revert To Last Saved while a conflict is showing loads the file into
    /// the editor, which resolves the conflict the same way Load Disk
    /// Version does. A Versions restore reads the file through the same
    /// path.
    func testRevertDuringAConflictClearsIt() async throws {
        let (deck, document, _, theirs) = try await openDeckWithAShownConflict()
        let controller = try XCTUnwrap(document.sessionController)

        try document.revert(toContentsOf: deck, ofType: "net.daringfireball.markdown")
        XCTAssertEqual(controller.editor.string, theirs, "the editor holds the file's text")
        XCTAssertFalse(document.isDocumentEdited)
        XCTAssertFalse(controller.hasDiskConflict, "the editor now matches the file")
        XCTAssertNil(controller.editorViewController.bar(.changedOnDisk), "the bar is gone")
        let mayClose = try await canClose(document)
        XCTAssertTrue(mayClose, "the window can close")
    }

    /// The re-review's repro: tap crashes, the disk version loads while it
    /// is down (so nothing can answer it yet), an unrelated edit lands, and
    /// only then does tap restart and answer. The one answer that finally
    /// arrives describes the combined text, not the load alone, so it must
    /// not move the cursor back to the load's slide.
    func testPendingCursorLoadDoesNotOutliveACrashAndRestart() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let first = try XCTUnwrap(controller.session.processIdentifier)
        controller.editor.moveCursor(toSlide: 5)

        let before = controller.editor.string
        let changed = before.replacingOccurrences(of: "# The Page", with: "# The Pages, renamed")

        kill(first, SIGKILL)
        try await waitUntil(timeout: 5, "tap to go down") {
            if case .running = controller.session.state { return false } else { return true }
        }

        try writeOutside(changed, to: deck)
        controller.diskChanged()
        XCTAssertEqual(controller.editor.string, changed, "the load landed while tap was still down")

        controller.editor.moveCursor(toSlide: 0)
        controller.editor.insertText("x", replacementRange: NSRange(location: NSNotFound, length: 0))

        _ = try await waitForRunningTap(document)
        try await waitUntil(timeout: 20, "tap's answer for the combined text") { controller.editor.boxes.count == 7 }
        XCTAssertEqual(controller.editor.currentBoxIndex, 0, "the crash and restart did not hijack the cursor back to the load's slide")
    }

    /// Reproduces the app mistaking its own in-flight autosave for an
    /// outside change: the app writes the file, then keeps typing before
    /// "saved" is processed. A `file-changed` report can arrive in that
    /// window, and comparing the disk text against the live editor text
    /// alone (which has since moved past the save) cannot tell this apart
    /// from a real outside change, wrongly raising the conflict bar for the
    /// app's own save. Drives `diskChanged()` directly, past the save's own
    /// snapshot-taking step (`data(ofType:)`) and the write it stands in
    /// for, rather than depending on tap's watcher and the app's autosave
    /// timer landing in this order by luck.
    func testDiskChangedDoesNotMistakeAnInFlightAutosaveForAConflict() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        controller.editor.moveCursor(toSlide: 5)
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))

        // Stands in for a save taking its snapshot of the buffer, the moment
        // before it writes to disk. `document.savedSnapshot` now holds this
        // text, as it would while the write and the "saved" message are
        // still in flight.
        _ = try document.data(ofType: "net.daringfireball.markdown")
        let snapshot = controller.editor.string

        // The person keeps typing before "saved" is processed.
        controller.editor.insertText(" more", replacementRange: NSRange(location: NSNotFound, length: 0))
        let withMore = controller.editor.string
        XCTAssertNotEqual(snapshot, withMore, "the editor has moved on past the snapshot")

        // Stands in for tap's watcher reporting the app's own write: the
        // disk holds exactly the snapshot the save is still writing.
        try writeOutside(snapshot, to: deck)
        controller.diskChanged()

        XCTAssertNil(controller.editorViewController.bar(.changedOnDisk), "the app's own save must not raise a conflict bar")
        XCTAssertFalse(controller.hasDiskConflict)
        XCTAssertEqual(controller.editor.string, withMore, "nothing was loaded over what the person kept typing")
        XCTAssertTrue(document.isDocumentEdited, "the editor is still ahead of the snapshot the disk now holds")
    }

    /// A Save To writes the exported text to a different file, never this
    /// document's own, so `savedSnapshot` must not be left holding that text
    /// as an own-write match target for the deck's real file. If it were, an
    /// outside program writing the deck's own file with exactly that
    /// exported text would be silently absorbed as if it were the app's own
    /// write, which it never was. The exported text is captured from an
    /// edited buffer and the buffer moves on again afterward, so neither
    /// `document.text` (the original) nor `editor.string` (the later edit)
    /// coincidentally equals it: only a leftover `savedSnapshot` could mask
    /// the outside write below, and the fix clears it once Save To completes.
    func testSaveToADifferentFileDoesNotMaskAnOutsideChangeMatchingTheExport() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let original = controller.editor.string
        let elsewhere = deck.deletingLastPathComponent().appendingPathComponent("exported-copy.md")
        defer { try? FileManager.default.removeItem(at: elsewhere) }

        controller.editor.moveCursor(toSlide: 2)
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))
        let exported = controller.editor.string

        let saveToError = await withCheckedContinuation { (continuation: CheckedContinuation<Error?, Never>) in
            document.save(to: elsewhere, ofType: "net.daringfireball.markdown", for: .saveToOperation) { error in
                continuation.resume(returning: error)
            }
        }
        XCTAssertNil(saveToError)
        XCTAssertNil(document.savedSnapshot, "a save that lands elsewhere leaves nothing behind to match against")

        // The buffer moves on again after the export, back to the original
        // text, so it no longer holds what was exported: with no unsaved
        // edits (editor matches document.text again), an outside write of
        // exactly the exported text must be loaded as a real change, not
        // swallowed as this Save To's own write.
        let fullRange = NSRange(location: 0, length: (controller.editor.string as NSString).length)
        controller.editor.replaceText(in: fullRange, with: original, actionName: "reset for the test")
        XCTAssertEqual(controller.editor.string, original)
        XCTAssertFalse(document.isDocumentEdited)

        try writeOutside(exported, to: deck)
        controller.diskChanged()
        try await waitUntil(timeout: 10, "the exported text to be loaded as a real outside change") {
            controller.editor.string == exported
        }
        XCTAssertNil(controller.editorViewController.bar(.changedOnDisk))
        XCTAssertFalse(document.isDocumentEdited)

        // Repeat with unsaved edits in the buffer: the same coincidental
        // match must raise the conflict bar rather than being swallowed.
        let secondDeck = try Fixtures.copyDeck("seven-slides.md")
        let secondDocument = try await openDeck(secondDeck)
        try await waitForBoxes(secondDocument, count: 7)
        _ = try await waitForRunningTap(secondDocument)
        let secondController = try XCTUnwrap(secondDocument.sessionController)
        secondController.editor.moveCursor(toSlide: 2)
        secondController.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))
        let secondExported = secondController.editor.string
        let secondElsewhere = secondDeck.deletingLastPathComponent().appendingPathComponent("exported-copy.md")
        defer { try? FileManager.default.removeItem(at: secondElsewhere) }
        let secondSaveToError = await withCheckedContinuation { (continuation: CheckedContinuation<Error?, Never>) in
            secondDocument.save(to: secondElsewhere, ofType: "net.daringfireball.markdown", for: .saveToOperation) { error in
                continuation.resume(returning: error)
            }
        }
        XCTAssertNil(secondSaveToError)
        secondController.editor.insertText(" more", replacementRange: NSRange(location: NSNotFound, length: 0))
        let withMore = secondController.editor.string
        XCTAssertNotEqual(withMore, secondExported)

        try writeOutside(secondExported, to: secondDeck)
        secondController.diskChanged()
        try await waitUntil(timeout: 10, "the conflict bar") { secondController.editorViewController.bar(.changedOnDisk) != nil }
        XCTAssertTrue(secondController.hasDiskConflict)
        XCTAssertEqual(secondController.editor.string, withMore, "nothing was loaded over the unsaved edit")
    }

    /// Once an own-file save completes, whatever `data(ofType:)` captured for
    /// it must not still be sitting there: the save is no longer in flight,
    /// so nothing should keep matching disk text against it.
    func testSavedSnapshotIsNilAfterAnOwnSaveCompletes() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        controller.editor.moveCursor(toSlide: 2)
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))

        let saveError = await withCheckedContinuation { (continuation: CheckedContinuation<Error?, Never>) in
            document.save(to: deck, ofType: "net.daringfireball.markdown", for: .saveOperation) { error in
                continuation.resume(returning: error)
            }
        }
        XCTAssertNil(saveError)
        XCTAssertNil(document.savedSnapshot, "the snapshot does not outlive the save it stood in for")
    }

    /// A save records the text it hands to the file and the revision that
    /// was current at that moment. If a newer `adopt(diskText:)` runs before
    /// that save's completion fires, the completion's own snapshot is now
    /// stale and must not overwrite the newer text. Driven directly against
    /// `data(ofType:)` and the completion's own logic, rather than by racing
    /// real file I/O against a real disk change, which is not deterministic.
    func testSaveCompletionDoesNotOverwriteANewerAdoptedText() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)

        // Stands in for a save taking its snapshot: it captures the text and
        // the revision current at that moment, the same way `save(to:...)`
        // does before its completion handler runs later.
        _ = try document.data(ofType: "net.daringfireball.markdown")

        // A newer change lands before that save's completion fires.
        document.adopt(diskText: "changed from outside while the save was in flight")

        // The save "completes" now, well after the newer text landed.
        document.adoptSavedSnapshotIfCurrent()

        XCTAssertEqual(document.text, "changed from outside while the save was in flight",
                       "the in-flight save's older snapshot did not overwrite the newer text")
    }
}
