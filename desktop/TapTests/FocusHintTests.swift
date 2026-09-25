import XCTest
@testable import Tap

final class FocusHintTests: PresentingTestCase {
    func testNothingInterruptsTheTalk() async throws {
        AppEnvironment.shared.focusHint = FocusHintState(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.focus.first.\(UUID().uuidString)")))
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        var settingsOpened: [URL] = []
        deckWindow.openSystemSettings = { settingsOpened.append($0) }

        // The first time I present, the app suggests a Focus mode.
        deckWindow.startPresenting(PresentationOptions(mode: .play, startSlide: 1))
        let sheet = try XCTUnwrap(deckWindow.focusHintSheet)
        XCTAssertTrue(deckWindow.window?.attachedSheet === sheet)
        XCTAssertEqual(sheet.titleLabel.stringValue, "Turn on a Focus before your talk?")
        XCTAssertTrue(sheet.bodyLabel.stringValue.contains("macOS does not let apps turn on a Focus for you"))
        XCTAssertEqual(presentation.state, .idle, "the talk waits for the hint")
        try XCTUnwrap(sheet.button(titled: "Open Focus Settings")).performClick(nil)
        XCTAssertEqual(settingsOpened, [URL(string: "x-apple.systempreferences:com.apple.Focus-Settings.extension")], "a button that opens its setting")
        // System Settings opens a pane by its extension's bundle identifier; where this host has the Focus pane, the URL names it.
        let focusPane = URL(fileURLWithPath: "/System/Library/ExtensionKit/Extensions/FocusSettingsExtension.appex")
        if let identifier = Bundle(url: focusPane)?.bundleIdentifier {
            XCTAssertEqual(settingsOpened.first?.absoluteString, "x-apple.systempreferences:\(identifier)")
        }
        XCTAssertEqual(presentation.state, .idle, "the person sets the Focus, then presses Play again")
        XCTAssertTrue(AppEnvironment.shared.focusHint.hasBeenShown)

        // Never again: Play starts at once.
        deckWindow.startPresenting(PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertNil(deckWindow.focusHintSheet)
        XCTAssertEqual(presentation.state, .starting)
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }

        // While I am presenting, nothing else may interrupt: no update prompt, no second talk.
        XCTAssertTrue(AppEnvironment.shared.isPresenting)
        XCTAssertFalse(AppEnvironment.shared.updatesMayInterrupt, "D7's Sparkle checks this before any prompt or restart")
        // The second deck opens on the desktop Space, under nothing; its
        // window is ordered front by openDeck, which switches Spaces away
        // from the talk. Its preview's ready still relies on pull request
        // 27 (a hidden page reports ready) if the window server has not
        // switched yet when the page paints.
        let (_, other) = try await openDeckForPresenting()
        let otherWindow = try XCTUnwrap(other.editor.window?.windowController as? DeckWindowController)
        let play = try XCTUnwrap(NSApp.mainMenu?.items.first { $0.submenu?.title == "Present" }?.submenu?.items.first { $0.action == #selector(DeckWindowController.play(_:)) })
        XCTAssertFalse(otherWindow.validateMenuItem(play), "one talk at a time")
        XCTAssertFalse(otherWindow.playButton.isEnabled)
        try await stopPresenting(controller)
        XCTAssertFalse(AppEnvironment.shared.isPresenting)
        XCTAssertTrue(AppEnvironment.shared.updatesMayInterrupt)
        XCTAssertTrue(otherWindow.validateMenuItem(play))
        XCTAssertTrue(otherWindow.playButton.isEnabled)
    }

    func testNotNowStartsTheTalk() async throws {
        AppEnvironment.shared.focusHint = FocusHintState(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.focus.notnow.\(UUID().uuidString)")))
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        deckWindow.startPresenting(PresentationOptions(mode: .rehearse, startSlide: 2))
        let sheet = try XCTUnwrap(deckWindow.focusHintSheet)
        try XCTUnwrap(sheet.button(titled: "Not Now")).performClick(nil)
        XCTAssertNil(deckWindow.focusHintSheet)
        XCTAssertEqual(controller.presentation.state, .starting)
        XCTAssertEqual(controller.presentation.options?.startSlide, 2)
        try await waitUntil(timeout: 40, "the rehearsal") { controller.presentation.state == .presenting }
    }

    func testTheHintEndedAnyOtherWayStartsNothing() async throws {
        AppEnvironment.shared.focusHint = FocusHintState(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.focus.abort.\(UUID().uuidString)")))
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        var settingsOpened: [URL] = []
        deckWindow.openSystemSettings = { settingsOpened.append($0) }
        deckWindow.startPresenting(PresentationOptions(mode: .play, startSlide: 1))
        let sheet = try XCTUnwrap(deckWindow.focusHintSheet)
        // Neither button: the sheet ends as its window goes, say.
        try XCTUnwrap(deckWindow.window).endSheet(sheet, returnCode: .abort)
        XCTAssertNil(deckWindow.focusHintSheet)
        XCTAssertEqual(controller.presentation.state, .idle, "only Not Now starts the talk")
        XCTAssertTrue(settingsOpened.isEmpty)
    }

    func testNotNowWhileAnotherDeckPresentsSaysWhy() async throws {
        AppEnvironment.shared.focusHint = FocusHintState(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.focus.other.\(UUID().uuidString)")))
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        deckWindow.startPresenting(PresentationOptions(mode: .play, startSlide: 1))
        let sheet = try XCTUnwrap(deckWindow.focusHintSheet)
        // While the hint is up, another deck's talk starts: the hint has been shown, so it goes straight in.
        let (_, other) = try await openDeckForPresenting()
        try await startPresenting(other, PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertNil(controller.editorViewController.bar(.talkNotStarted))
        try XCTUnwrap(sheet.button(titled: "Not Now")).performClick(nil)
        XCTAssertNil(deckWindow.focusHintSheet)
        XCTAssertEqual(controller.presentation.state, .idle, "one talk at a time")
        let bar = try XCTUnwrap(controller.editorViewController.bar(.talkNotStarted), "Not Now says why nothing started")
        XCTAssertEqual(bar.message, "The talk did not start.")
        XCTAssertNil(deckWindow.window?.attachedSheet, "a bar, not a modal alert")
        try await stopPresenting(other)
        try await waitUntil(timeout: 20, "Play back once the other talk is gone") { controller.presentation.canStart }
        // The next start clears it.
        deckWindow.startPresenting(PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertEqual(controller.presentation.state, .starting)
        XCTAssertNil(controller.editorViewController.bar(.talkNotStarted))
    }
}
