import XCTest
@testable import Tap

final class PreviewTests: HostedTestCase {
    func testThePreviewFollowsTheCursor() async throws {
        throw XCTSkip("Waits on Task 13: needs the internal/cli/testdata/app fixture directory, which does not exist on this branch.")
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
        throw XCTSkip("Waits on Task 13: needs the internal/cli/testdata/app fixture directory, which does not exist on this branch.")
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 4)
        controller.editor.moveCursor(toSlide: 2)
        let before = try await waitForPreview(document, slide: 3)
        let headingEnd = (controller.editor.string as NSString).range(of: "# Fragments").upperBound
        controller.editor.setSelectedRange(NSRange(location: headingEnd, length: 0))
        controller.editor.insertText(" edited", replacementRange: NSRange(location: NSNotFound, length: 0))
        let typed = Date()
        try await waitUntil(timeout: 5, "a new revision in the preview") {
            controller.previewViewController.lastReady.map { $0.revision != before.revision && $0.slide == 3 } ?? false
        }
        XCTAssertLessThan(Date().timeIntervalSince(typed), 0.2, "the preview shows an edit within 200 ms after the typing pause starts")
        let text = await controller.previewViewController.pageText()
        XCTAssertTrue(text.contains("Fragments edited"))
    }

    func testThePreviewIsTheAudienceView() async throws {
        throw XCTSkip("Waits on Task 13: needs the internal/cli/testdata/app fixture directory, which does not exist on this branch.")
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let url = try XCTUnwrap(document.sessionController?.previewViewController.webView.url)
        XCTAssertEqual(url.path, "/")
        XCTAssertFalse(url.absoluteString.contains("presenter"))
        XCTAssertFalse(url.absoluteString.contains("launch="), "the launch code is spent and gone from the URL")
    }

    func testThePreviewShowsTheAudienceSafeErrorForm() async throws {
        throw XCTSkip("Waits on Task 13: needs a real audience page and websocket, which the fake tap does not provide.")
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("throwing"))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 2)
        controller.editor.moveCursor(toSlide: 1)
        try await waitForPreview(document, slide: 2)
        let text = await controller.previewViewController.pageText()
        XCTAssertTrue(text.contains("Boom from the fixture"), "the preview shows the full error card, as tap dev does: \(text)")
    }

    func testThePreviewUpdatesInPlace() async throws {
        throw XCTSkip("Waits on Task 13: needs the internal/cli/testdata/app fixture directory, which does not exist on this branch.")
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 4)
        controller.editor.moveCursor(toSlide: 2)
        let before = try await waitForPreview(document, slide: 3)
        let navigations = controller.previewViewController.finishedNavigationCount
        var messages: [HubMessage] = []
        controller.onHubMessage = { messages.append($0) }

        let headingEnd = (controller.editor.string as NSString).range(of: "# Fragments").upperBound
        controller.editor.setSelectedRange(NSRange(location: headingEnd, length: 0))
        controller.editor.insertText(" again", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitUntil(timeout: 5, "an update message") {
            messages.contains { if case .update(_, let slides) = $0 { return slides.contains(3) } else { return false } }
        }
        try await waitUntil(timeout: 5, "the re-rendered slide") { controller.previewViewController.lastReady?.revision != before.revision }

        XCTAssertFalse(messages.contains(.reload), "tap sends update, not reload")
        XCTAssertEqual(controller.previewViewController.finishedNavigationCount, navigations, "the page did not reload")
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
        process.waitUntilExit()
        return String(decoding: output.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
            .split(separator: "\n")
            .filter { $0.trimmingCharacters(in: .whitespaces).hasPrefix("\(parent) ") }
            .filter { $0.localizedCaseInsensitiveContains("chrom") }
            .map(String.init)
    }
}
