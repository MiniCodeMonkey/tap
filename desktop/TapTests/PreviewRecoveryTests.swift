import XCTest
import WebKit
@testable import Tap

/// A preview whose web content process ends, or stops answering in the
/// middle of a load, gets a new web view and comes back on its own, with
/// no reopening of the deck. The process is ended or stopped for real,
/// through its process identifier (a test-only read of WebKit's private
/// key). The seams are the load watchdog's interval, shortened so a test
/// does not wait the real ten seconds, the recovery limit per time
/// window, and the page answer check, which one test replaces to stand
/// for a process that does not answer.
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
        // The watchdog fires again and again while the load runs, so the
        // page answering is what keeps its web view, not the load finishing
        // first.
        preview.loadWatchdogInterval = 0.05
        let webView = preview.webView
        preview.reload()
        try await waitUntil(timeout: 20, "the reloaded page to report ready") { preview.lastReady != nil }
        try await Task.sleep(nanoseconds: 3_000_000_000)
        XCTAssertTrue(preview.webView === webView)
        XCTAssertEqual(preview.pageRecoveryCount, 0)
    }
}

extension PreviewRecoveryTests {
    /// The stall seen on CI: the first load of a new tap goes to a process
    /// that never runs it, so tap never answers the launch URL and the code
    /// is not spent. The preview gets a new web view and restarts tap for a
    /// new code, and the restarting notice stays up through the restart.
    func testAPreviewStuckBeforeItsLaunchCodeIsSpentComesBackWithANewTap() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController
        let client = try XCTUnwrap(controller.client)
        let firstTap = try XCTUnwrap(controller.session.processIdentifier)
        preview.loadWatchdogInterval = 2
        let stuck = preview.webView
        let process = try XCTUnwrap(stuck.contentProcessIdentifier, "the preview's content process identifier")
        kill(process, SIGSTOP)
        defer { kill(process, SIGKILL) }
        var noticeAtEachState: [String] = []
        let appStateChange = controller.session.onStateChange
        controller.session.onStateChange = { state in
            appStateChange?(state)
            noticeAtEachState.append(preview.overlay.isHidden ? "none" : preview.overlay.titleLabel.stringValue)
        }
        defer { controller.session.onStateChange = appStateChange }

        preview.load(client: client)

        try await waitUntil(timeout: 30, "a new tap (recoveries \(preview.pageRecoveryCount))") {
            controller.session.processIdentifier.map { $0 != firstTap } ?? false
        }
        try await waitUntil(timeout: 30, "the page to report ready again. \(self.appSideDiagnostics(preview))") { preview.lastReady != nil }
        XCTAssertEqual(preview.pageRecoveryCount, 1)
        XCTAssertTrue(preview.webView !== stuck, "the page came back in a new web view")
        XCTAssertTrue(controller.session.log.text.contains("restarting tap"), "tap restarted for a new launch code")
        XCTAssertGreaterThanOrEqual(noticeAtEachState.count, 3, "stopped, starting and running")
        XCTAssertEqual(Set(noticeAtEachState), ["Restarting preview."], "the notice stays up through the restart: \(noticeAtEachState)")
        XCTAssertTrue(preview.overlay.isHidden, "the notice goes once the page is back")
    }

    /// Two new web views in a row with no load finishing, and the preview
    /// stops: it shows that it stopped, with Try Again, and makes no third.
    func testAPreviewGivesUpAfterTwoRecoveriesInARow() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController
        let tap = try XCTUnwrap(controller.session.processIdentifier)
        preview.loadWatchdogInterval = 1
        preview.pageAnswers = { _, _ in false }
        // A stopped tap answers no load, so none finishes.
        kill(tap, SIGSTOP)
        defer { kill(tap, SIGCONT) }

        preview.reload()

        try await waitUntil(timeout: 20, "the preview to give up (recoveries \(preview.pageRecoveryCount))") { preview.hasGivenUp }
        XCTAssertEqual(preview.pageRecoveryCount, 2)
        XCTAssertEqual(preview.overlay.titleLabel.stringValue, "The preview stopped")
        XCTAssertFalse(preview.overlay.isHidden)
        XCTAssertFalse(preview.overlay.tryAgainButton.isHidden, "Try Again is offered")
        let last = preview.webView
        try await Task.sleep(nanoseconds: 3_000_000_000)
        XCTAssertTrue(preview.webView === last, "no third web view")
        XCTAssertEqual(preview.pageRecoveryCount, 2)
    }

    /// A page whose process ends after every load is started again only
    /// so many times within the window, even with finished loads and
    /// readies in between, then the preview shows that it stopped.
    func testAPreviewWhoseProcessEndsAfterEveryLoadStopsWithinTheWindow() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController
        preview.maximumRecoveriesInWindow = 2
        for round in 1...2 {
            let ended = preview.webView
            let process = try XCTUnwrap(ended.contentProcessIdentifier, "round \(round): the content process identifier")
            kill(process, SIGKILL)
            try await waitUntil(timeout: 10, "round \(round): a new web view") { preview.webView !== ended }
            try await waitUntil(timeout: 20, "round \(round): the page ready again") { preview.lastReady != nil }
        }
        XCTAssertEqual(preview.pageRecoveryCount, 2)
        let last = preview.webView
        let process = try XCTUnwrap(last.contentProcessIdentifier)

        kill(process, SIGKILL)

        try await waitUntil(timeout: 10, "the preview to give up") { preview.hasGivenUp }
        XCTAssertTrue(preview.webView === last, "no third recovery")
        XCTAssertEqual(preview.pageRecoveryCount, 2)
        XCTAssertEqual(preview.overlay.titleLabel.stringValue, "The preview stopped")
        XCTAssertFalse(preview.overlay.tryAgainButton.isHidden, "Try Again is offered")
    }

    /// A page whose load stays in flight, waiting on a tap that does not
    /// answer, while its process runs and answers every check: the
    /// watchdog asks again and again and never replaces it, and the load
    /// finishes once tap answers.
    func testALivePageWhoseLoadWaitsOnTapIsNeverReplaced() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController
        let tap = try XCTUnwrap(controller.session.processIdentifier)
        let webView = preview.webView
        preview.loadWatchdogInterval = 0.3
        kill(tap, SIGSTOP)
        var resumed = false
        defer { if !resumed { kill(tap, SIGCONT) } }
        let checksBefore = preview.pageAnswerCheckCount

        preview.reload()

        try await waitUntil(timeout: 10, "three checks of the waiting page (checks \(preview.pageAnswerCheckCount - checksBefore))") {
            preview.pageAnswerCheckCount - checksBefore >= 3
        }
        XCTAssertTrue(preview.webView.isLoading, "the load is still in flight: \(preview.navigationMilestoneDescription)")
        XCTAssertTrue(preview.webView === webView, "a page that answers keeps its web view")
        XCTAssertEqual(preview.pageRecoveryCount, 0)
        kill(tap, SIGCONT)
        resumed = true

        try await waitUntil(timeout: 20, "the load to finish and the page to report ready") { preview.lastReady != nil }
        XCTAssertTrue(preview.navigationMilestoneDescription.contains("finish"), preview.navigationMilestoneDescription)
        XCTAssertTrue(preview.webView === webView)
        XCTAssertEqual(preview.pageRecoveryCount, 0)
        XCTAssertTrue(preview.overlay.isHidden)
    }

    /// A load that replaces one still in flight: WebKit reports the old
    /// navigation as cancelled, and that says nothing about the new load.
    /// tap is stopped so the first load stays in flight, waiting on it,
    /// until the second replaces it.
    func testCallbacksForAReplacedLoadAreIgnored() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController
        let tap = try XCTUnwrap(controller.session.processIdentifier)
        kill(tap, SIGSTOP)
        var resumed = false
        defer { if !resumed { kill(tap, SIGCONT) } }
        let ignoredBefore = preview.ignoredNavigationCallbackCount

        preview.reload()
        try await waitUntil(timeout: 5, "the first load to start") { preview.navigationMilestoneDescription.contains("start") }
        preview.reload()
        try await waitUntil(timeout: 5, "WebKit's callback for the replaced load") { preview.ignoredNavigationCallbackCount > ignoredBefore }
        kill(tap, SIGCONT)
        resumed = true

        try await waitUntil(timeout: 20, "the second load's ready") { preview.lastReady != nil }
        XCTAssertFalse(preview.navigationMilestoneDescription.contains("fail"), preview.navigationMilestoneDescription)
        XCTAssertTrue(preview.navigationMilestoneDescription.contains("finish"), preview.navigationMilestoneDescription)
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

    /// A page whose process ends after every load is reloaded only so
    /// many times within the window, then reported as failed and left.
    func testATalkPageWhoseProcessKeepsEndingIsLeftAlone() async throws {
        let (_, controller) = try await openDeckForPresenting()
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let page = try XCTUnwrap(controller.presentation.audienceWindow?.page)
        page.maximumReloadsInWindow = 1
        var failures = 0
        let appLoadFailed = page.onLoadFailed
        page.onLoadFailed = { error in
            failures += 1
            appLoadFailed?(error)
        }
        try await waitUntil(timeout: 20, "the audience page's first ready") { page.lastReady != nil }
        kill(try XCTUnwrap(page.webView.contentProcessIdentifier), SIGKILL)
        try await waitUntil(timeout: 10, "the first reload") { page.reloadAfterTerminationCount == 1 }
        try await waitUntil(timeout: 20, "the audience page ready again") { page.lastReady != nil }

        kill(try XCTUnwrap(page.webView.contentProcessIdentifier), SIGKILL)

        try await waitUntil(timeout: 10, "the failure report") { failures == 1 }
        try await Task.sleep(nanoseconds: 2_000_000_000)
        XCTAssertEqual(page.reloadAfterTerminationCount, 1, "no second reload")
        XCTAssertEqual(page.processTerminationCount, 2)
        XCTAssertNil(page.lastReady)
        try await stopPresenting(controller)
    }
}
