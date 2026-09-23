import XCTest
@testable import Tap

final class PreviewTests: HostedTestCase {
    func testThePreviewFollowsTheCursor() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 4)
        controller.editor.moveCursor(toSlide: 2)
        try await waitForPreview(document, slide: 3)
        // Slide 3 has one fragment; the preview shows it revealed.
        XCTAssertEqual(controller.navigator.message, SlideMessage(slideIndex: 2, fragment: 0, step: 0))
        XCTAssertEqual(controller.previewViewController.stepLabel.stringValue, "All steps shown, 1 of 1")
        XCTAssertEqual(controller.previewViewController.statusLabel.stringValue, "Slide 3, follows the cursor")
    }

    func testThePreviewUpdatesWhileIType() async throws {
        let deck = try Fixtures.copyAppFixture()
        let document = try await openDeckAndWaitForPreview(deck)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 4)
        controller.editor.moveCursor(toSlide: 2)
        let before = try await waitForPreview(document, slide: 3)
        let headingEnd = (controller.editor.string as NSString).range(of: "# Fragments").upperBound
        controller.editor.setSelectedRange(NSRange(location: headingEnd, length: 0))
        controller.editor.insertText(" edited", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitUntil(timeout: 5, "a new revision in the preview") {
            controller.previewViewController.lastReady.map { $0.revision != before.revision && $0.slide == 3 } ?? false
        }
        let text = await controller.previewViewController.pageText()
        XCTAssertTrue(text.contains("Fragments edited"))
        // While I type, not when I save: the edit reached the preview out of
        // the unsaved buffer, and the deck on disk still holds the old text.
        // How long that took is a property of the machine, so it is not
        // asserted here; the debounce that decides when the send goes out is
        // covered by TapDesktopCore's SourceSyncTests.
        XCTAssertTrue(controller.editor.string.contains("# Fragments edited"), "the edit is only in the buffer")
        let onDisk = try String(contentsOf: deck, encoding: .utf8)
        XCTAssertFalse(onDisk.contains("Fragments edited"), "nothing was written to the deck file")
    }

    func testThePreviewIsTheAudienceView() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let url = try XCTUnwrap(document.sessionController?.previewViewController.webView.url)
        XCTAssertEqual(url.path, "/")
        XCTAssertFalse(url.absoluteString.contains("presenter"))
        XCTAssertFalse(url.absoluteString.contains("launch="), "the launch code is spent and gone from the URL")
    }

    func testThePreviewShowsTheAudienceSafeErrorForm() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("throwing"))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 2)
        controller.editor.moveCursor(toSlide: 1)
        try await waitForPreview(document, slide: 2)
        let text = await controller.previewViewController.pageText()
        XCTAssertTrue(text.contains("Boom from the fixture"), "the preview shows the full error card, as tap dev does: \(text)")
    }

    func testThePreviewUpdatesInPlace() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 4)
        controller.editor.moveCursor(toSlide: 2)
        let before = try await waitForPreview(document, slide: 3)
        let loads = controller.previewViewController.pageLoadCount
        var messages: [HubMessage] = []
        controller.onHubMessage = { messages.append($0) }

        let headingEnd = (controller.editor.string as NSString).range(of: "# Fragments").upperBound
        controller.editor.setSelectedRange(NSRange(location: headingEnd, length: 0))
        controller.editor.insertText(" again", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitUntil(timeout: 5, "an update message") {
            messages.contains { if case .update(_, let slides) = $0 { return slides.contains(3) } else { return false } }
        }
        // Reading the edit out of the page is what puts the assertions
        // after everything the edit sets off. A reload is not visible in
        // lastReady, which a reload only clears, nor reliably in WebKit's
        // navigation callbacks, which arrive whenever the run loop gets to
        // them; it is visible in pageLoadCount, which rises inside load()
        // the moment the app asks for a new page.
        var text = ""
        let deadline = Date().addingTimeInterval(15)
        while !text.contains("Fragments again") && Date() < deadline {
            text = await controller.previewViewController.pageText()
            try await Task.sleep(nanoseconds: 20_000_000)
        }

        XCTAssertFalse(messages.contains(.reload), "tap sends update, not reload")
        XCTAssertTrue(text.contains("Fragments again"), "the preview shows the edit")
        XCTAssertEqual(controller.previewViewController.pageLoadCount, loads, "the page did not reload")
        XCTAssertEqual(controller.previewViewController.lastReady?.step, before.step, "the current step is kept")

        let tapProcess = try XCTUnwrap(controller.session.processIdentifier)
        XCTAssertTrue(browserChildren(of: tapProcess).isEmpty, "no browser process is started")
    }

    func browserChildren(of parent: Int32) -> [String] {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/bin/ps")
        process.arguments = ["-A", "-o", "ppid=,comm="]
        let output = Pipe()
        process.standardOutput = output
        try? process.run()
        // The listing is read before the wait, because ps writes more than a
        // pipe holds and would block forever on a full one.
        let listing = output.fileHandleForReading.readDataToEndOfFile()
        process.waitUntilExit()
        return String(decoding: listing, as: UTF8.self)
            .split(separator: "\n")
            .filter { $0.trimmingCharacters(in: .whitespaces).hasPrefix("\(parent) ") }
            .filter { $0.localizedCaseInsensitiveContains("chrom") }
            .map(String.init)
    }
}
