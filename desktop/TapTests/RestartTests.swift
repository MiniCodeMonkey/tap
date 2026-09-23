import XCTest
@testable import Tap

final class RestartTests: HostedTestCase {
    func testCrashAndRestart() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 4)
        let overlay = controller.previewViewController.overlay
        let first = try XCTUnwrap(controller.session.processIdentifier)

        kill(first, SIGKILL)
        try await waitUntil(timeout: 5, "the restarting notice") { !overlay.isHidden && overlay.titleLabel.stringValue == "Restarting preview." }
        XCTAssertEqual(overlay.detailLabel.stringValue, "Showing the last good render.")

        // The editor keeps working while tap is down.
        let end = (controller.editor.string as NSString).range(of: "# Fragments").upperBound
        controller.editor.setSelectedRange(NSRange(location: end, length: 0))
        controller.editor.insertText(" while tap was down", replacementRange: NSRange(location: NSNotFound, length: 0))
        XCTAssertTrue(controller.editor.string.contains("# Fragments while tap was down"))

        _ = try await waitForRunningTap(document)
        XCTAssertNotEqual(controller.session.processIdentifier, first)
        try await waitUntil(timeout: 15, "tap's answer for the edit") { controller.editor.boxes.count == 4 && controller.editor.boxes[2].slide.title == "Fragments while tap was down" }
        try await waitUntil(timeout: 15, "the preview to come back") { overlay.isHidden && controller.previewViewController.lastReady != nil }
    }

    func testTapKeepsFailing() async throws {
        let bundled = AppEnvironment.shared.tapExecutableURL
        AppEnvironment.shared.tapExecutableURL = try FakeTapScripts.crashing()
        defer { AppEnvironment.shared.tapExecutableURL = bundled }

        let document = try await openDeck(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        let overlay = controller.previewViewController.overlay
        try await waitUntil(timeout: 15, "the stopped preview") { overlay.titleLabel.stringValue == "The preview stopped" }
        XCTAssertFalse(overlay.isHidden)
        XCTAssertEqual(overlay.detailLabel.stringValue, "tap exited 3 times in 30 seconds. Last output:")
        XCTAssertTrue(overlay.outputLabel.stringValue.contains("panic: runtime error: index out of range"))
        XCTAssertFalse(overlay.tryAgainButton.isHidden)
        XCTAssertFalse(overlay.showLogButton.isHidden)

        overlay.tryAgainButton.performClick(nil)
        try await waitUntil(timeout: 5, "Try Again to start tap") { controller.session.log.text.contains("Try Again") }
        try await waitUntil(timeout: 15, "the second stop") {
            if case .failed = controller.session.state { return true } else { return false }
        }
    }
}
