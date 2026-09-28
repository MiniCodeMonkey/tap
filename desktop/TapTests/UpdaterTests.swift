import XCTest
import Sparkle
@testable import Tap

/// Sparkle's controller in the app: never started under tests, wired to
/// Tap > Check for Updates…, and held back by every talk.
final class UpdaterTests: PresentingTestCase {
    var updates: UpdateController { (NSApp.delegate as! AppDelegate).updateController }

    override func tearDown() async throws {
        updates.isUpdateSessionInProgress = { false }
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

    func testPlayWaitsForAnUpdateInProgress() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        updates.isUpdateSessionInProgress = { true }
        deckWindow.startPresenting(PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertEqual(controller.presentation.state, .idle, "nothing started")
        let bar = try XCTUnwrap(controller.editorViewController.bar(.talkNotStarted), "the bar says why")
        XCTAssertEqual(bar.detail, UpdateGate.updateInProgressMessage)
        XCTAssertFalse(AppEnvironment.shared.isPresenting)

        updates.isUpdateSessionInProgress = { false }
        deckWindow.startPresenting(PresentationOptions(mode: .rehearse, startSlide: 1))
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        try await stopPresenting(controller)
    }
}
