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
}
