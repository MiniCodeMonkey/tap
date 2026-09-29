import XCTest
@testable import Tap

/// A save can write a keystroke tap has not been sent yet: a key that lands
/// after the periodic autosave checked for unsent text, or a Save right
/// after typing. The app names the text to tap before it writes, so tap
/// takes the write as the app's own and the preview is not reloaded for it.
final class AnnouncedSaveTests: HostedTestCase {
    func testAWriteOfTextTapHasNotBeenSentDoesNotReloadThePreview() async throws {
        let savedDelay = NSDocumentController.shared.autosavingDelay
        NSDocumentController.shared.autosavingDelay = 300
        addTeardownBlock { @MainActor in NSDocumentController.shared.autosavingDelay = savedDelay }
        let deck = try Fixtures.copyAppFixture()
        let document = try await openDeckAndWaitForPreview(deck)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 4)
        controller.editor.moveCursor(toSlide: 2)
        try await waitForPreview(document, slide: 3)

        // Every PUT is held back well past tap's watcher, which reads a
        // write about a tenth of a second after it lands, so the text on
        // disk is text tap has not been sent.
        let sourceSync = try XCTUnwrap(controller.sourceSync)
        let send = try XCTUnwrap(sourceSync.sender)
        sourceSync.sender = { source in
            try await Task.sleep(nanoseconds: 3_000_000_000)
            return try await send(source)
        }
        let loads = controller.previewViewController.pageLoadCount
        var messages: [HubMessage] = []
        controller.onHubMessage = { messages.append($0) }

        let headingEnd = (controller.editor.string as NSString).range(of: "# Fragments").upperBound
        controller.editor.setSelectedRange(NSRange(location: headingEnd, length: 0))
        controller.editor.insertText(" written at once", replacementRange: NSRange(location: NSNotFound, length: 0))
        XCTAssertTrue(sourceSync.hasUnsentText)

        // The write NSDocument makes, without the "saved" that follows it,
        // so tap's watcher alone decides what the pages are sent.
        // data(ofType:) is where every save of the deck file takes its
        // text, and where the app names it to tap.
        let data = try document.data(ofType: document.fileType ?? "net.daringfireball.markdown")
        try data.write(to: deck)
        try await waitUntil(timeout: 5, "tap to tell the app about the write") {
            controller.session.log.text.contains("file changed on disk")
        }
        let sentWhenTapReadTheWrite = sourceSync.hasUnsentText

        var text = ""
        let deadline = Date().addingTimeInterval(15)
        while !text.contains("Fragments written at once") && Date() < deadline {
            text = await controller.previewViewController.pageText()
            try await Task.sleep(nanoseconds: 20_000_000)
        }
        // Anything the write set off has reached the page by now.
        try await Task.sleep(nanoseconds: 500_000_000)

        XCTAssertTrue(sentWhenTapReadTheWrite, "the PUT carrying the text had not reached tap when it read the write")
        XCTAssertTrue(text.contains("Fragments written at once"), "the preview shows the written text")
        XCTAssertFalse(messages.contains { if case .fileChanged = $0 { return true } else { return false } },
                       "the pages were told the deck changed elsewhere: \(messages)")
        XCTAssertFalse(messages.contains(.reload), "tap sent reload: \(messages)")
        XCTAssertEqual(controller.previewViewController.pageLoadCount, loads, "the page did not reload")
    }

    /// Opens the app fixture with tap running, types into it, and records
    /// every command the app sends tap from then on.
    private func openAndRecordCommands() async throws -> (deck: URL, document: DeckDocument, controller: DeckSessionController, sent: () -> [TapCommand]) {
        let savedDelay = NSDocumentController.shared.autosavingDelay
        NSDocumentController.shared.autosavingDelay = 300
        addTeardownBlock { @MainActor in NSDocumentController.shared.autosavingDelay = savedDelay }
        let deck = try Fixtures.copyAppFixture()
        let document = try await openDeck(deck)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 4)
        let headingEnd = (controller.editor.string as NSString).range(of: "# Fragments").upperBound
        controller.editor.setSelectedRange(NSRange(location: headingEnd, length: 0))
        controller.editor.insertText(" not in the deck file", replacementRange: NSRange(location: NSNotFound, length: 0))
        var sent: [TapCommand] = []
        controller.session.onCommandSent = { sent.append($0) }
        return (deck, document, controller, { sent })
    }

    /// Whether `commands` hold a saving command.
    private func containsSaving(_ commands: [TapCommand]) -> Bool {
        commands.contains { if case .saving = $0 { return true } else { return false } }
    }

    /// A save of the deck file's own text names it, which is what shows
    /// the recording sees a saving command when one is sent.
    private func assertAnOwnFileSaveNamesItsText(_ document: DeckDocument, _ controller: DeckSessionController,
                                                 sent: () -> [TapCommand], file: StaticString = #filePath, line: UInt = #line) throws {
        _ = try document.data(ofType: document.fileType ?? "net.daringfireball.markdown")
        XCTAssertEqual(sent().last, .saving(text: controller.editor.string), "a save of the deck file names its text", file: file, line: line)
    }

    /// Duplicate takes the text through data(ofType:) but writes no part of
    /// the deck file, so it names nothing to tap.
    func testDuplicateNamesNothingToTap() async throws {
        let (_, document, controller, sent) = try await openAndRecordCommands()
        // tearDown closes the duplicate with every other document.
        _ = try document.duplicate()
        XCTAssertFalse(containsSaving(sent()), "Duplicate named its text to tap: \(sent())")
        try assertAnOwnFileSaveNamesItsText(document, controller, sent: sent)
    }

    /// Save To and an autosave elsewhere write a file other than the deck,
    /// so they take no save snapshot and name nothing to tap. NSDocument
    /// refuses an autosave elsewhere to a URL of the caller's choosing for
    /// a deck that autosaves in place, before it takes any text, and runs
    /// one itself only for a deck with no file, into its own autosave
    /// folder; so this checks the rule save(to:ofType:for:completionHandler:)
    /// applies rather than driving that write.
    func testOnlySavesOfTheDeckFileNameTheirText() {
        XCTAssertFalse(DeckDocument.writesOwnFile(.autosaveElsewhereOperation))
        XCTAssertFalse(DeckDocument.writesOwnFile(.saveToOperation))
        XCTAssertTrue(DeckDocument.writesOwnFile(.saveOperation))
        XCTAssertTrue(DeckDocument.writesOwnFile(.saveAsOperation))
        XCTAssertTrue(DeckDocument.writesOwnFile(.autosaveInPlaceOperation))
    }
}
