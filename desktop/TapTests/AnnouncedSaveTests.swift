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

    /// Opens the app fixture on slide 3 with every PUT held back 5 s, and
    /// types into its heading, so the editor holds text tap has not been
    /// sent. Returns the typed text.
    private func openWithUnsentText() async throws -> (deck: URL, document: DeckDocument, controller: DeckSessionController, typed: String) {
        let savedDelay = NSDocumentController.shared.autosavingDelay
        NSDocumentController.shared.autosavingDelay = 300
        addTeardownBlock { @MainActor in NSDocumentController.shared.autosavingDelay = savedDelay }
        let deck = try Fixtures.copyAppFixture()
        let document = try await openDeckAndWaitForPreview(deck)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 4)
        controller.editor.moveCursor(toSlide: 2)
        try await waitForPreview(document, slide: 3)
        let sourceSync = try XCTUnwrap(controller.sourceSync)
        let send = try XCTUnwrap(sourceSync.sender)
        sourceSync.sender = { source in
            try await Task.sleep(nanoseconds: 5_000_000_000)
            return try await send(source)
        }
        let headingEnd = (controller.editor.string as NSString).range(of: "# Fragments").upperBound
        controller.editor.setSelectedRange(NSRange(location: headingEnd, length: 0))
        controller.editor.insertText(" not in the deck file", replacementRange: NSRange(location: NSNotFound, length: 0))
        XCTAssertTrue(sourceSync.hasUnsentText)
        return (deck, document, controller, controller.editor.string)
    }

    /// Another program writing `text` to the deck reaches the pages as a
    /// change made elsewhere: the bytes were never named to tap.
    private func assertAnOutsideWriteReloadsThePages(_ text: String, deck: URL, controller: DeckSessionController,
                                                     file: StaticString = #filePath, line: UInt = #line) async throws {
        var messages: [HubMessage] = []
        controller.onHubMessage = { messages.append($0) }
        func isTheDeck(_ message: HubMessage) -> Bool {
            if case .fileChanged(let path) = message { return (path as NSString).lastPathComponent == deck.lastPathComponent }
            return false
        }
        try text.write(to: deck, atomically: true, encoding: .utf8)
        let deadline = Date().addingTimeInterval(5)
        while !messages.contains(where: { isTheDeck($0) }), Date() < deadline {
            try await Task.sleep(nanoseconds: 20_000_000)
        }
        XCTAssertTrue(messages.contains { isTheDeck($0) },
                      "the pages were not told about a write of bytes no save of the deck file wrote: \(messages)",
                      file: file, line: line)
    }

    /// Duplicate takes the text through data(ofType:) but writes no part of
    /// the deck file, so it names nothing to tap.
    func testDuplicateNamesNothingToTap() async throws {
        let (deck, document, controller, typed) = try await openWithUnsentText()
        // tearDown closes the duplicate with every other document.
        _ = try document.duplicate()
        try await assertAnOutsideWriteReloadsThePages(typed, deck: deck, controller: controller)
    }

    /// An autosave elsewhere writes a file other than the deck, so it names
    /// nothing to tap.
    func testAnAutosaveElsewhereNamesNothingToTap() async throws {
        let (deck, document, controller, typed) = try await openWithUnsentText()
        // Outside the deck's folder, which tap watches.
        let elsewhere = FileManager.default.temporaryDirectory.appendingPathComponent("tap-autosaved-elsewhere-\(UUID().uuidString).md")
        addTeardownBlock { try? FileManager.default.removeItem(at: elsewhere) }
        try await withCheckedThrowingContinuation { (continuation: CheckedContinuation<Void, Error>) in
            document.save(to: elsewhere, ofType: document.fileType ?? "net.daringfireball.markdown", for: .autosaveElsewhereOperation) { error in
                if let error { continuation.resume(throwing: error) } else { continuation.resume() }
            }
        }
        XCTAssertEqual(try String(contentsOf: elsewhere, encoding: .utf8), typed, "the autosave wrote the text elsewhere")
        try await assertAnOutsideWriteReloadsThePages(typed, deck: deck, controller: controller)
    }
}
