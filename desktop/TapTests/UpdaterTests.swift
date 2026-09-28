import XCTest
import Sparkle
@testable import Tap

/// Sparkle's controller in the app: never started under tests, wired to
/// Tap > Check for Updates…, and held back by every talk.
final class UpdaterTests: PresentingTestCase {
    var updates: UpdateController { (NSApp.delegate as! AppDelegate).updateController }

    override func tearDown() async throws {
        updates.interface.sessionFinished()
        try await super.tearDown()
    }

    func testTheUpdaterNeverStartsUnderTests() {
        XCTAssertTrue(UpdateController.runsUnderTests)
        XCTAssertFalse(updates.isStarted, "a hosted test process never reaches the feed")
        updates.startIfAllowed()
        XCTAssertFalse(updates.isStarted, "not even when asked")
        XCTAssertFalse(updates.canCheckForUpdates)
    }

    func testCheckForUpdatesIsInTheTapMenu() throws {
        let tapMenu = try XCTUnwrap(NSApp.mainMenu?.items.first { $0.title == "Tap" }?.submenu)
        let item = try XCTUnwrap(tapMenu.items.first { $0.title == "Check for Updates…" })
        XCTAssertEqual(item.action, #selector(AppDelegate.checkForUpdates(_:)))
        XCTAssertNil(item.target, "nil-targeted, so the responder chain reaches the app delegate")
        let delegate = try XCTUnwrap(NSApp.delegate as? AppDelegate)
        XCTAssertFalse(delegate.validateMenuItem(item), "disabled while the updater is not started, as under tests")
    }

    func testTheFeedAndTheKeysAreInThePlist() {
        let info = Bundle.main.infoDictionary ?? [:]
        XCTAssertEqual(info["SUFeedURL"] as? String, "https://github.com/MiniCodeMonkey/tap/releases/latest/download/appcast.xml")
        XCTAssertEqual(info["SUPublicEDKey"] as? String, "Pc0PbtYL9fkyK3XnqULlpKLlH94TE4OWx4Q3n74EftU=")
        XCTAssertEqual(info["SURequireSignedFeed"] as? Bool, true, "an unsigned feed is refused before anything is shown")
        XCTAssertEqual(info["SUVerifyUpdateBeforeExtraction"] as? Bool, true, "which a signed feed requires")
        XCTAssertNil(info["SUEnableAutomaticChecks"], "Sparkle asks the person on the second launch; the app does not decide for them")
        XCTAssertNotNil(info["NSMicrophoneUsageDescription"], "a recorded talk takes the microphone")
    }

    func testNoCheckDuringATalk() async throws {
        let (_, controller) = try await openDeckForPresenting()
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertFalse(AppEnvironment.shared.updatesMayInterrupt)
        XCTAssertThrowsError(try updates.updater(updates.updater, mayPerform: .updatesInBackground)) { error in
            XCTAssertEqual(error.localizedDescription, UpdateGate.presentingMessage)
        }
        XCTAssertThrowsError(try updates.updater(updates.updater, mayPerform: .updates), "the person's own check waits too")
        XCTAssertFalse(updates.canCheckForUpdates)
        try await stopPresenting(controller)
        XCTAssertNoThrow(try updates.updater(updates.updater, mayPerform: .updatesInBackground))
    }

    func testAnUpdateFoundDuringATalkIsDropped() async throws {
        let (_, controller) = try await openDeckForPresenting()
        XCTAssertNoThrow(try updates.updater(updates.updater, shouldProceedWithUpdate: SUAppcastItem.empty(), updateCheck: .updatesInBackground))
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertThrowsError(try updates.updater(updates.updater, shouldProceedWithUpdate: SUAppcastItem.empty(), updateCheck: .updatesInBackground)) { error in
            XCTAssertEqual(error.localizedDescription, UpdateGate.presentingMessage)
        }
        try await stopPresenting(controller)
    }

    func testAPostponedRelaunchRunsWhenTheTalkWindowsAreDown() async throws {
        let (_, controller) = try await openDeckForPresenting()
        var relaunched = 0
        XCTAssertFalse(updates.updater(updates.updater, shouldPostponeRelaunchForUpdate: SUAppcastItem.empty()) { relaunched += 1 }, "no talk: Sparkle relaunches now")
        XCTAssertEqual(relaunched, 0)

        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertTrue(updates.updater(updates.updater, shouldPostponeRelaunchForUpdate: SUAppcastItem.empty()) { relaunched += 1 })
        XCTAssertEqual(relaunched, 0, "the talk runs; the app stays")

        // The talk ends (updatesMayInterrupt turns true) before its windows
        // are down; the relaunch waits for the windows, never mid-transition.
        controller.presentation.stop()
        try await waitUntil(timeout: 30, "the talk to end") { controller.presentation.state == .idle }
        if !controller.presentation.windowsGoingDown.isEmpty {
            XCTAssertEqual(relaunched, 0, "windows still going down: no relaunch yet")
        }
        try await waitUntil(timeout: 20, "the postponed relaunch, once the windows are down") { relaunched == 1 }
        XCTAssertTrue(updates.talkWindowsAreDown)
        XCTAssertNil(updates.gate.postponedRelaunch)
    }

    func testPlayWaitsForAWindowSparkleHasUp() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        updates.interface.updateWindowShown()
        deckWindow.startPresenting(PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertEqual(controller.presentation.state, .idle, "nothing started")
        let bar = try XCTUnwrap(controller.editorViewController.bar(.talkNotStarted), "the bar says why")
        XCTAssertEqual(bar.detail, SparkleInterface.updateWindowMessage)
        XCTAssertFalse(AppEnvironment.shared.isPresenting)

        updates.interface.sessionFinished()
        updates.interface.permissionPromptShown()
        deckWindow.startPresenting(PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertEqual(controller.presentation.state, .idle, "the permission prompt holds Play too")
        let promptBar = try XCTUnwrap(controller.editorViewController.bar(.talkNotStarted))
        XCTAssertEqual(promptBar.detail, SparkleInterface.permissionPromptMessage, "in the words for the prompt")

        updates.interface.permissionPromptAnswered()
        deckWindow.startPresenting(PresentationOptions(mode: .rehearse, startSlide: 1))
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        try await stopPresenting(controller)
    }

    /// The app's driver notes what Sparkle's standard interface puts up and
    /// takes down. The inner driver here shows nothing.
    func testTheDriverNotesWhatSparkleShows() {
        XCTAssertTrue(updates.userDriver.interface === updates.interface, "the updater's driver keeps the state Play reads")
        let inner = SilentUserDriver()
        let interface = SparkleInterface()
        let driver = WatchedUserDriver(inner: inner, interface: interface)

        var answered = 0
        driver.show(SPUUpdatePermissionRequest(systemProfile: [])) { _ in answered += 1 }
        XCTAssertTrue(interface.permissionPromptIsUp)
        inner.permissionReply?(SUUpdatePermissionResponse(automaticUpdateChecks: false, sendSystemProfile: false))
        XCTAssertFalse(interface.permissionPromptIsUp, "the answer takes the prompt down")
        XCTAssertEqual(answered, 1, "and reaches Sparkle")

        driver.showDownloadDidReceiveData(ofLength: 10)
        XCTAssertFalse(interface.updateWindowIsUp, "progress alone shows no window")
        let windows: [(String, () -> Void)] = [
            ("the checking window", { driver.showUserInitiatedUpdateCheck {} }),
            ("the up-to-date alert", { driver.showUpdateNotFoundWithError(UpdateGate.PresentingError()) {} }),
            ("an error alert", { driver.showUpdaterError(UpdateGate.PresentingError()) {} }),
            ("the download window", { driver.showDownloadInitiated {} }),
            ("the extraction window", { driver.showDownloadDidStartExtractingUpdate() }),
            ("ready to install", { driver.showReady { _ in } }),
            ("installing", { driver.showInstallingUpdate(withApplicationTerminated: false) {} }),
            ("installed", { driver.showUpdateInstalledAndRelaunched(true) {} }),
        ]
        for (name, show) in windows {
            show()
            XCTAssertTrue(interface.updateWindowIsUp, name)
            driver.dismissUpdateInstallation()
            XCTAssertFalse(interface.updateWindowIsUp, "\(name): the end of the session takes it down")
        }
        XCTAssertEqual(inner.dismissals, windows.count, "every call reaches Sparkle's own driver")
        XCTAssertNil(interface.playRefusal)
    }
}

/// A user driver that shows nothing and keeps the permission reply.
private final class SilentUserDriver: NSObject, SPUUserDriver {
    var permissionReply: ((SUUpdatePermissionResponse) -> Void)?
    var dismissals = 0

    func show(_ request: SPUUpdatePermissionRequest, reply: @escaping (SUUpdatePermissionResponse) -> Void) { permissionReply = reply }
    func showUserInitiatedUpdateCheck(cancellation: @escaping () -> Void) {}
    func showUpdateFound(with appcastItem: SUAppcastItem, state: SPUUserUpdateState, reply: @escaping (SPUUserUpdateChoice) -> Void) {}
    func showUpdateReleaseNotes(with downloadData: SPUDownloadData) {}
    func showUpdateReleaseNotesFailedToDownloadWithError(_ error: any Error) {}
    func showUpdateNotFoundWithError(_ error: any Error, acknowledgement: @escaping () -> Void) {}
    func showUpdaterError(_ error: any Error, acknowledgement: @escaping () -> Void) {}
    func showDownloadInitiated(cancellation: @escaping () -> Void) {}
    func showDownloadDidReceiveExpectedContentLength(_ expectedContentLength: UInt64) {}
    func showDownloadDidReceiveData(ofLength length: UInt64) {}
    func showDownloadDidStartExtractingUpdate() {}
    func showExtractionReceivedProgress(_ progress: Double) {}
    func showReady(toInstallAndRelaunch reply: @escaping (SPUUserUpdateChoice) -> Void) {}
    func showInstallingUpdate(withApplicationTerminated applicationTerminated: Bool, retryTerminatingApplication: @escaping () -> Void) {}
    func showUpdateInstalledAndRelaunched(_ relaunched: Bool, acknowledgement: @escaping () -> Void) {}
    func dismissUpdateInstallation() { dismissals += 1 }
}
