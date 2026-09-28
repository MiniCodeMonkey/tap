import XCTest
@testable import Tap

/// The delegate object `canClose(withDelegate:shouldClose:contextInfo:)`
/// calls back on this selector.
private final class CloseAnswer: NSObject {
    private(set) var answers: [Bool] = []

    @objc func document(_ document: NSDocument, shouldClose: Bool, contextInfo: UnsafeMutableRawPointer?) {
        answers.append(shouldClose)
    }
}

/// A periodic autosave that finds text tap has not taken yet starts the
/// send and reports itself cancelled at once, then asks for a new autosave
/// once the send is done. It never holds NSDocument's activity open while
/// the send runs, so a save or close that starts meanwhile goes ahead.
final class AutosaveHandOffTests: HostedTestCase {
    /// Opens the seven-slide deck with tap running and makes every PUT take
    /// `seconds` longer, so a send stays pending.
    private func openDeckWithASlowSend(seconds: Double) async throws -> (deck: URL, document: DeckDocument, controller: DeckSessionController) {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let sourceSync = try XCTUnwrap(controller.sourceSync)
        let send = try XCTUnwrap(sourceSync.sender)
        sourceSync.sender = { source in
            try await Task.sleep(nanoseconds: UInt64(seconds * 1_000_000_000))
            return try await send(source)
        }
        return (deck, document, controller)
    }

    private func type(_ text: String, into controller: DeckSessionController) {
        controller.editor.moveCursor(toSlide: 5)
        controller.editor.insertText(text, replacementRange: NSRange(location: NSNotFound, length: 0))
    }

    private func periodicAutosave(_ document: DeckDocument) async -> Error? {
        await withCheckedContinuation { (continuation: CheckedContinuation<Error?, Never>) in
            document.autosave(withImplicitCancellability: true) { error in continuation.resume(returning: error) }
        }
    }

    func testASaveAndACloseDuringAPendingSendFinishAndWriteTheText() async throws {
        let (deck, document, controller) = try await openDeckWithASlowSend(seconds: 3)
        type(" typed before the save", into: controller)
        let typed = controller.editor.string
        XCTAssertTrue(controller.sourceSync.hasUnsentText)

        let started = Date()
        let autosaveError = await periodicAutosave(document)
        XCTAssertEqual((autosaveError as NSError?)?.code, CocoaError.userCancelled.rawValue,
                       "the periodic autosave hands off and reports itself cancelled")
        XCTAssertLessThan(Date().timeIntervalSince(started), 1, "the periodic autosave does not wait for the send")
        XCTAssertTrue(controller.sourceSync.hasUnsentText, "the send is still pending")

        // The close's own autosave writes the edited text while the send
        // is still pending.
        let answer = CloseAnswer()
        document.canClose(withDelegate: answer, shouldClose: #selector(CloseAnswer.document(_:shouldClose:contextInfo:)), contextInfo: nil)
        try await waitUntil(timeout: 3, "the close answer") { answer.answers.count == 1 }
        XCTAssertEqual(answer.answers, [true])
        XCTAssertEqual(try String(contentsOf: deck, encoding: .utf8), typed, "the close's autosave wrote the text")

        type(" and before Save", into: controller)
        let typedAgain = controller.editor.string
        XCTAssertTrue(controller.sourceSync.hasUnsentText)
        document.save(withDelegate: nil, didSave: nil, contextInfo: nil)
        try await waitUntil(timeout: 3, "the save to write the text typed since") {
            (try? String(contentsOf: deck, encoding: .utf8)) == typedAgain
        }
        XCTAssertLessThan(Date().timeIntervalSince(started), 5, "neither the close nor the save waited for the send")
    }

    func testTheHandedOffAutosaveWritesOnceTheSendIsDone() async throws {
        let savedDelay = NSDocumentController.shared.autosavingDelay
        NSDocumentController.shared.autosavingDelay = 1
        addTeardownBlock { @MainActor in NSDocumentController.shared.autosavingDelay = savedDelay }
        let (deck, document, controller) = try await openDeckWithASlowSend(seconds: 1)
        type(" typed before the autosave", into: controller)
        let typed = controller.editor.string

        let autosaveError = await periodicAutosave(document)
        XCTAssertEqual((autosaveError as NSError?)?.code, CocoaError.userCancelled.rawValue)
        XCTAssertNotEqual(try String(contentsOf: deck, encoding: .utf8), typed, "nothing written before tap has the text")

        try await waitUntil(timeout: 10, "the rescheduled autosave to write the typed text") {
            (try? String(contentsOf: deck, encoding: .utf8)) == typed
        }
        XCTAssertFalse(controller.sourceSync.hasUnsentText, "tap had the text before it was written")
    }
}
