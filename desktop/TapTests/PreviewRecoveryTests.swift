import XCTest
import WebKit
@testable import Tap

/// A preview whose web content process ends, or stops answering in the
/// middle of a load, gets a new web view and comes back on its own, with
/// no reopening of the deck. The process is ended or stopped for real,
/// through its process identifier (a test-only read of WebKit's private
/// key); the one seam is the load watchdog's interval, shortened so the
/// test does not wait the real ten seconds.
final class PreviewRecoveryTests: HostedTestCase {
    func testAPreviewWhoseWebProcessEndsComesBackInANewWebView() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController
        XCTAssertEqual(preview.pageRecoveryCount, 0)
        let stuck = preview.webView
        let process = try XCTUnwrap(stuck.contentProcessIdentifier, "the preview's content process identifier")

        kill(process, SIGKILL)

        try await waitUntil(timeout: 10, "a new web view in the preview (recoveries \(preview.pageRecoveryCount))") { preview.webView !== stuck }
        XCTAssertNil(stuck.superview, "the old web view is gone from the preview")
        XCTAssertTrue(preview.webView.superview === preview.pageContainer, "the new web view is where the old one was")
        try await waitUntil(timeout: 20, "the page to report ready again. \(self.appSideDiagnostics(preview))") { preview.lastReady != nil }
        XCTAssertEqual(preview.pageRecoveryCount, 1)
        XCTAssertNotEqual(preview.webView.contentProcessIdentifier, process)
        XCTAssertTrue(preview.overlay.isHidden, "the restarting notice goes once the page is back")
        XCTAssertTrue(controller.session.log.text.contains("web content process ended"), "the recovery is in the Tap Log")
        let url = try XCTUnwrap(preview.webView.url)
        XCTAssertFalse(url.absoluteString.contains("launch="), "the spent launch code is not used again")
    }

    func testAPreviewWhoseLoadNeverFinishesComesBackInANewWebView() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController
        preview.loadWatchdogInterval = 2
        let stuck = preview.webView
        let process = try XCTUnwrap(stuck.contentProcessIdentifier, "the preview's content process identifier")
        // A stopped process takes the load and never runs it, and never
        // answers script: the wedge seen on CI.
        kill(process, SIGSTOP)
        defer { kill(process, SIGKILL) }
        var noticesShown: [String] = []
        let observation = preview.overlay.observe(\.isHidden, options: [.new]) { overlay, _ in
            MainActor.assumeIsolated {
                if !overlay.isHidden { noticesShown.append(overlay.titleLabel.stringValue) }
            }
        }
        defer { observation.invalidate() }

        preview.reload()

        try await waitUntil(timeout: 15, "a new web view in the preview (recoveries \(preview.pageRecoveryCount))") { preview.webView !== stuck }
        XCTAssertNil(stuck.superview, "the old web view is gone from the preview")
        XCTAssertEqual(noticesShown, ["Restarting preview."], "the preview says it is restarting while the new page loads")
        try await waitUntil(timeout: 20, "the page to report ready again. \(self.appSideDiagnostics(preview))") { preview.lastReady != nil }
        XCTAssertEqual(preview.pageRecoveryCount, 1)
        XCTAssertTrue(preview.overlay.isHidden, "the restarting notice goes once the page is back")
        XCTAssertTrue(preview.navigationMilestoneDescription.contains("finish"), preview.navigationMilestoneDescription)
        XCTAssertTrue(controller.session.log.text.contains("did not finish loading"), "the recovery is in the Tap Log")
        print("recovered preview: \(appSideDiagnostics(preview))")
    }

    func testAPreviewThatLoadsNormallyIsNeverReplaced() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController
        preview.loadWatchdogInterval = 1
        let webView = preview.webView
        preview.reload()
        try await waitUntil(timeout: 20, "the reloaded page to report ready") { preview.lastReady != nil }
        // Past the watchdog and its answer wait: a page that finished, or
        // that answers, keeps its web view.
        try await Task.sleep(nanoseconds: 3_500_000_000)
        XCTAssertTrue(preview.webView === webView)
        XCTAssertEqual(preview.pageRecoveryCount, 0)
    }
}

/// A talk page whose web content process ends loads again in a new
/// process and reports ready again.
final class TalkPageRecoveryTests: PresentingTestCase {
    func testATalkPageWhoseWebProcessEndsComesBack() async throws {
        let (_, controller) = try await openDeckForPresenting()
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let page = try XCTUnwrap(controller.presentation.audienceWindow?.page)
        try await waitUntil(timeout: 20, "the audience page's first ready") { page.lastReady != nil }
        let process = try XCTUnwrap(page.webView.contentProcessIdentifier, "the audience page's content process identifier")

        kill(process, SIGKILL)

        try await waitUntil(timeout: 10, "WebKit to report the ended process") { page.processTerminationCount == 1 }
        try await waitUntil(timeout: 20, "the audience page to report ready again") { page.lastReady != nil }
        XCTAssertNotEqual(page.webView.contentProcessIdentifier, process)
        try await stopPresenting(controller)
    }
}
