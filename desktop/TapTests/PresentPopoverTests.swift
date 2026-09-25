import XCTest
@testable import Tap

final class PresentPopoverTests: PresentingTestCase {
    func windowController(_ controller: DeckSessionController) throws -> DeckWindowController {
        try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
    }

    func testStartFromTheFirstSlide() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        controller.jumpToSlide(number: 3)
        deckWindow.playButtonClicked(modifiers: [.shift])
        XCTAssertFalse(deckWindow.presentPopover.isShown, "Shift-click starts at once")
        XCTAssertEqual(controller.presentation.options?.mode, .play)
        XCTAssertEqual(controller.presentation.options?.startSlide, 1)
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        let audience = try XCTUnwrap(controller.presentation.audienceWindow)
        try await waitUntil(timeout: 20, "the audience page on slide 1") { audience.page.lastReady?.slide == 1 }
        XCTAssertEqual(controller.currentSlideNumber, 3, "the cursor stays where it was")
    }

    func testThePopoverCollectsTheOptions() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let screens = halfScreens()
        controller.presentation.screens = { screens }
        // The popover reads whether the talk would use full screen; the talk itself runs as the test case decides for two displays on one screen.
        let fullScreenPolicy = controller.presentation.fullScreenAllowed
        controller.presentation.fullScreenAllowed = { true }
        controller.jumpToSlide(number: 3)
        deckWindow.playButtonClicked(modifiers: [])
        let popover = deckWindow.presentPopover
        XCTAssertTrue(popover.isShown, "a click on Play opens the popover")
        XCTAssertTrue(popover.separateSpacesLabel.isHidden)
        XCTAssertEqual(popover.startFromControl.label(forSegment: 0), "Slide 3")
        XCTAssertEqual(popover.startFromControl.label(forSegment: 1), "Slide 1")
        XCTAssertEqual(popover.startFromControl.selectedSegment, 0)
        XCTAssertEqual(popover.arrangementView.audienceBox.nameLabel.stringValue, "Projector")
        XCTAssertEqual(popover.arrangementView.presenterBox.nameLabel.stringValue, "Built-in Display")
        XCTAssertFalse(popover.arrangementView.isHidden)
        XCTAssertTrue(popover.singleDisplayLabel.isHidden)
        XCTAssertTrue(popover.swapButton.isEnabled)
        XCTAssertEqual(popover.recordCheckbox.state, .on)
        XCTAssertEqual(popover.phoneRemoteCheckbox.state, .off)
        XCTAssertTrue(popover.advancedStack.isHidden)

        popover.swapButton.performClick(nil)
        XCTAssertEqual(popover.arrangementView.audienceBox.nameLabel.stringValue, "Built-in Display", "Swap Displays swaps before the start")
        XCTAssertEqual(controller.presentation.currentArrangement?.audience.name, "Built-in Display")

        popover.advancedButton.performClick(nil)
        XCTAssertFalse(popover.advancedStack.isHidden)
        popover.recordCheckbox.state = .off
        popover.passwordField.stringValue = "secret"
        popover.tunnelCheckbox.state = .on
        let advanced = popover.options(mode: .play)
        XCTAssertEqual(advanced, PresentationOptions(mode: .play, startSlide: 3, record: false, phoneRemote: false, tunnel: true, presenterPassword: "secret"))
        popover.tunnelCheckbox.state = .off
        popover.phoneRemoteCheckbox.performClick(nil)
        XCTAssertEqual(popover.phoneRemoteCheckbox.state, .on)
        XCTAssertEqual(popover.tunnelCheckbox.state, .on, "the phone remote is the tunnel")
        XCTAssertFalse(popover.tunnelCheckbox.isEnabled)
        popover.phoneRemoteCheckbox.performClick(nil)
        XCTAssertTrue(popover.tunnelCheckbox.isEnabled)
        popover.tunnelCheckbox.state = .off
        popover.passwordField.stringValue = ""
        popover.startFromControl.selectedSegment = 1
        XCTAssertEqual(popover.options(mode: .play), PresentationOptions(mode: .play, startSlide: 1, record: false))

        controller.presentation.fullScreenAllowed = fullScreenPolicy
        popover.startButton.performClick(nil)
        XCTAssertFalse(popover.isShown)
        XCTAssertEqual(controller.presentation.options, PresentationOptions(mode: .play, startSlide: 1, record: false))
        XCTAssertEqual(AppEnvironment.shared.presentationSettings.settings,
                       PresentationSettings(startFromSlideOne: true, record: false, phoneRemote: false, tunnel: false),
                       "a start saves its settings for Cmd+Option+P and the next launch")
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        XCTAssertEqual(controller.presentation.session?.command, .present(record: false, presenterPassword: nil, port: nil))
        XCTAssertEqual(controller.presentation.audienceWindow?.targetFrame, screens[0].frame, "the swap held")
    }

    func testCmdOptionPStartsAtOnceWithTheLastSettings() async throws {
        AppEnvironment.shared.presentationSettings.settings = PresentationSettings(startFromSlideOne: false, record: false, phoneRemote: false, tunnel: true)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        controller.jumpToSlide(number: 4)
        deckWindow.presentPopover.passwordField.stringValue = "secret"
        // Present > Play, Cmd+Option+P: no popover, the last settings, the cursor's slide.
        deckWindow.play(nil)
        XCTAssertFalse(deckWindow.presentPopover.isShown)
        XCTAssertEqual(controller.presentation.options,
                       PresentationOptions(mode: .play, startSlide: 4, record: false, phoneRemote: false, tunnel: true, presenterPassword: "secret"))
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        XCTAssertEqual(controller.presentation.session?.command, .present(record: false, presenterPassword: "secret", port: nil))
        try await stopPresenting(controller)
        // Present > Play with Options opens the popover, with the same settings showing.
        deckWindow.playWithOptions(nil)
        XCTAssertTrue(deckWindow.presentPopover.isShown)
        XCTAssertEqual(deckWindow.presentPopover.recordCheckbox.state, .off)
        XCTAssertEqual(deckWindow.presentPopover.tunnelCheckbox.state, .on)
        deckWindow.presentPopover.close()

        // The settings are app-wide: a second deck's Cmd+Option+P starts with what the first deck last chose, never with its own stale controls.
        let (_, other) = try await openDeckForPresenting()
        let otherWindow = try windowController(other)
        otherWindow.playWithOptions(nil)
        otherWindow.presentPopover.recordCheckbox.state = .on
        otherWindow.presentPopover.close()
        deckWindow.presentPopover.recordCheckbox.state = .off
        deckWindow.presentPopover.tunnelCheckbox.state = .off
        deckWindow.play(nil)
        XCTAssertEqual(controller.presentation.options?.record, false)
        XCTAssertEqual(controller.presentation.options?.tunnel, false, "the first deck saved its own controls")
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        try await stopPresenting(controller)
        other.jumpToSlide(number: 3)
        otherWindow.play(nil)
        XCTAssertEqual(other.presentation.options?.record, false, "the newest saved settings, not this popover's stale controls")
        XCTAssertEqual(other.presentation.options?.startSlide, 3, "this deck's cursor, read now")
        try await waitUntil(timeout: 40, "the other talk") { other.presentation.state == .presenting }
    }

    func testThePopoverWithOneDisplaySaysSo() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        deckWindow.playButtonClicked(modifiers: [])
        let popover = deckWindow.presentPopover
        XCTAssertTrue(popover.isShown)
        XCTAssertTrue(popover.arrangementView.isHidden)
        XCTAssertFalse(popover.singleDisplayLabel.isHidden)
        XCTAssertTrue(popover.separateSpacesLabel.isHidden, "one display never needs the setting")
        XCTAssertFalse(popover.swapButton.isEnabled)
        popover.rehearseButton.performClick(nil)
        XCTAssertFalse(popover.isShown)
        XCTAssertEqual(controller.presentation.options?.mode, .rehearse)
        XCTAssertEqual(controller.presentation.options?.startSlide, 1)
        try await waitUntil(timeout: 40, "the rehearsal") { controller.presentation.state == .presenting }
    }

    func testThePopoverSaysWhenDisplaysShareOneSpace() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let screens = halfScreens()
        controller.presentation.screens = { screens }
        controller.presentation.screensHaveSeparateSpaces = { false }
        deckWindow.playButtonClicked(modifiers: [])
        let popover = deckWindow.presentPopover
        XCTAssertFalse(popover.separateSpacesLabel.isHidden)
        XCTAssertTrue(popover.separateSpacesLabel.stringValue.contains("Displays have separate Spaces"))
        XCTAssertTrue(popover.startButton.isEnabled, "the talk still runs, as plain windows")
        popover.close()
    }

    func testThePlayButtonFollowsTheTalk() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let identifiers = try XCTUnwrap(deckWindow.window?.toolbar?.items.map(\.itemIdentifier))
        XCTAssertTrue(identifiers.contains(DeckWindowController.playItemIdentifier))
        XCTAssertEqual(deckWindow.playButton.accessibilityIdentifier(), "play-button")
        XCTAssertTrue(deckWindow.playButton.isEnabled)
        deckWindow.rehearse(nil)
        XCTAssertFalse(deckWindow.playButton.isEnabled, "no second talk while one runs")
        try await waitUntil(timeout: 40, "the rehearsal") { controller.presentation.state == .presenting }
        try await stopPresenting(controller)
        XCTAssertTrue(deckWindow.playButton.isEnabled)
    }
}
