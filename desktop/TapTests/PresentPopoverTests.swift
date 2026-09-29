import XCTest
@testable import Tap

final class PresentPopoverTests: PresentingTestCase {
    func windowController(_ controller: DeckSessionController) throws -> DeckWindowController {
        try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
    }

    /// Chooses `role` in a display's menu, as a person does.
    func choose(_ role: DisplayRole, for name: String, in popover: PresentPopoverController) throws {
        let tile = try XCTUnwrap(popover.arrangementView.tiles.first { $0.screen.name == name }, "no tile for \(name)")
        let item = try XCTUnwrap(tile.menuItem(for: role))
        NSApp.sendAction(try XCTUnwrap(item.action), to: item.target, from: item)
    }

    func roles(_ popover: PresentPopoverController) -> [String: String] {
        Dictionary(uniqueKeysWithValues: popover.arrangementView.tiles.map { ($0.screen.name, $0.box.roleLabel.stringValue) })
    }

    func testStartFromTheFirstSlide() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        try markPlayed(controller)
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

    func testPlayStartsAtOnce() async throws {
        AppEnvironment.shared.presentationSettings.settings = PresentationSettings(record: false, phoneRemote: false)
        let (document, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        try markPlayed(controller)
        controller.jumpToSlide(number: 4)
        deckWindow.presentPopover.phoneRemoteSwitch.state = .on
        deckWindow.presentPopover.passwordSwitch.state = .on
        deckWindow.presentPopover.passwordField.stringValue = "secret"
        deckWindow.playButtonClicked(modifiers: [])
        XCTAssertFalse(deckWindow.presentPopover.isShown, "a click on Play opens nothing")
        XCTAssertEqual(controller.presentation.options, PresentationOptions(mode: .play, startSlide: 4, record: false, phoneRemote: false),
                       "the remembered settings, from the cursor's slide; the popover's unsaved controls do not count")
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        let port = DeckPortStore.suggestedPort(for: try XCTUnwrap(document.fileURL))
        XCTAssertEqual(controller.presentation.session?.command, .present(record: false, presenterPassword: nil, port: port))
    }

    func testPresentSettingsOpenOnTheFirstRun() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let popover = deckWindow.presentPopover
        let screens = halfScreens()
        controller.presentation.screens = { screens }
        XCTAssertFalse(deckWindow.hasPlayedOnTheConnectedDisplays)
        deckWindow.playButtonClicked(modifiers: [])
        XCTAssertTrue(popover.isShown, "a deck never played on two displays opens Present Settings first")
        XCTAssertNil(controller.presentation.options, "nothing started")
        popover.startButton.performClick(nil)
        XCTAssertFalse(popover.isShown)
        XCTAssertEqual(controller.presentation.options?.mode, .play)
        XCTAssertTrue(deckWindow.hasPlayedOnTheConnectedDisplays, "Play from the popover is the deck's first play")
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        try await stopPresenting(controller)
        try await waitUntil(timeout: 20, "Play to be allowed again") { deckWindow.canStartATalk }
        deckWindow.playButtonClicked(modifiers: [])
        XCTAssertFalse(popover.isShown, "the next click starts at once")
        XCTAssertEqual(controller.presentation.state, .starting)
    }

    func testTheFirstClickOnOneDisplayJustPlays() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        XCTAssertFalse(deckWindow.hasPlayedOnTheConnectedDisplays)
        XCTAssertFalse(deckWindow.needsSettingsBeforePlaying, "one display leaves nothing to choose")
        deckWindow.playButtonClicked(modifiers: [])
        XCTAssertFalse(deckWindow.presentPopover.isShown)
        XCTAssertEqual(controller.presentation.options?.mode, .play)
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
    }

    func testRehearseFromPresentSettingsRecordsTheDisplays() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let screens = halfScreens()
        controller.presentation.screens = { screens }
        XCTAssertTrue(deckWindow.needsSettingsBeforePlaying)
        deckWindow.showPresentSettings(nil)
        deckWindow.presentPopover.rehearseButton.performClick(nil)
        XCTAssertEqual(controller.presentation.options?.mode, .rehearse)
        XCTAssertTrue(deckWindow.hasPlayedOnTheConnectedDisplays, "the person accepted these displays in Present Settings")
        XCTAssertFalse(deckWindow.needsSettingsBeforePlaying)
        try await waitUntil(timeout: 40, "the rehearsal") { controller.presentation.state == .presenting }
    }

    /// A Beginning chosen for one start does not carry into the next Play.
    func testStartFromBeginningDoesNotLeakIntoLaterPlays() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let popover = deckWindow.presentPopover
        controller.jumpToSlide(number: 2)
        deckWindow.showPresentSettings(nil)
        popover.startFromPopUp.selectItem(at: 1)
        popover.startButton.performClick(nil)
        XCTAssertEqual(controller.presentation.options?.startSlide, 1, "Beginning starts on slide 1")
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        try await stopPresenting(controller)
        try await waitUntil(timeout: 20, "Play to be allowed again") { deckWindow.canStartATalk }
        controller.jumpToSlide(number: 3)
        deckWindow.play(nil)
        XCTAssertEqual(controller.presentation.options?.startSlide, 3, "the cursor's slide, not the earlier Beginning")
        try await waitUntil(timeout: 40, "the second talk") { controller.presentation.state == .presenting }
    }

    /// With the remote on and cloudflared missing, Play in Present Settings starts without the remote and saves that.
    func testPlayingWithoutCloudflaredSavesTheRemoteOff() async throws {
        AppEnvironment.shared.cloudflaredProbe = { false }
        AppEnvironment.shared.presentationSettings.settings = PresentationSettings(record: false, phoneRemote: true)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        try markPlayed(controller)
        deckWindow.play(nil)
        XCTAssertTrue(deckWindow.presentPopover.isShown)
        deckWindow.presentPopover.startButton.performClick(nil)
        XCTAssertEqual(controller.presentation.options?.phoneRemote, false)
        XCTAssertEqual(AppEnvironment.shared.presentationSettings.settings.phoneRemote, false)
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        try await stopPresenting(controller)
        try await waitUntil(timeout: 20, "Play to be allowed again") { deckWindow.canStartATalk }
        deckWindow.play(nil)
        XCTAssertFalse(deckWindow.presentPopover.isShown, "later Plays start at once")
        XCTAssertEqual(controller.presentation.state, .starting)
    }

    func testPresentSettingsOpenWhenTheDisplaysChanged() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        try markPlayed(controller)
        let screens = halfScreens()
        controller.presentation.screens = { screens }
        XCTAssertFalse(deckWindow.hasPlayedOnTheConnectedDisplays, "a projector since the last play")
        deckWindow.playButtonClicked(modifiers: [])
        XCTAssertTrue(deckWindow.presentPopover.isShown)
        XCTAssertNil(controller.presentation.options)
        deckWindow.presentPopover.close()
        controller.presentation.screens = { screens.reversed() }
        try markPlayed(controller)
        XCTAssertTrue(deckWindow.hasPlayedOnTheConnectedDisplays, "the order the system lists the displays in does not matter")
    }

    func testThePopoverCollectsTheOptions() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let screens = halfScreens()
        controller.presentation.screens = { screens }
        // The popover reads whether the talk would use full screen; the talk itself runs as the test case decides for two displays on one screen.
        let fullScreenPolicy = controller.presentation.fullScreenAllowed
        controller.presentation.fullScreenAllowed = { true }
        controller.jumpToSlide(number: 3)
        deckWindow.playButtonClicked(modifiers: [])
        let popover = deckWindow.presentPopover
        XCTAssertTrue(popover.isShown, "the first click on Play opens the popover")
        XCTAssertTrue(popover.spacesNotice.isHidden)
        XCTAssertFalse(popover.displaysSection.isHidden)
        XCTAssertEqual(popover.startFromPopUp.itemTitles, ["Slide 3", "Beginning"])
        XCTAssertFalse(popover.startFromPopUp.isHidden)
        XCTAssertEqual(roles(popover), ["Projector": "Audience", "Built-in Display": "Presenter"])
        XCTAssertEqual(popover.recordSwitch.state, .on)
        XCTAssertEqual(popover.phoneRemoteSwitch.state, .off)
        XCTAssertTrue(popover.remoteGroup.isHidden, "the remote's options wait for the remote")
        XCTAssertEqual(popover.startButton.title, "Play")
        XCTAssertEqual(popover.startButton.keyEquivalent, "\r")

        // Clicking a display makes it the audience; the other one becomes the presenter.
        let builtIn = try XCTUnwrap(popover.arrangementView.tiles.first { $0.screen.name == "Built-in Display" })
        builtIn.box.onClick?()
        XCTAssertEqual(roles(popover), ["Projector": "Presenter", "Built-in Display": "Audience"])
        XCTAssertEqual(controller.presentation.currentArrangement?.audience.name, "Built-in Display")

        popover.recordSwitch.state = .off
        popover.phoneRemoteSwitch.performClick(nil)
        XCTAssertFalse(popover.remoteGroup.isHidden)
        XCTAssertTrue(popover.passwordField.isHidden, "the field waits for the password switch")
        popover.passwordSwitch.performClick(nil)
        XCTAssertFalse(popover.passwordField.isHidden)
        XCTAssertEqual(popover.passwordField.placeholderString, "Password")
        popover.passwordField.stringValue = "secret"
        XCTAssertEqual(popover.options(mode: .play), PresentationOptions(mode: .play, startSlide: 3, record: false, phoneRemote: true, presenterPassword: "secret"))
        popover.passwordSwitch.performClick(nil)
        XCTAssertNil(popover.presenterPassword, "no password with the switch off")
        popover.phoneRemoteSwitch.performClick(nil)
        XCTAssertTrue(popover.remoteGroup.isHidden)
        popover.startFromPopUp.selectItem(at: 1)
        XCTAssertEqual(popover.options(mode: .play), PresentationOptions(mode: .play, startSlide: 1, record: false))

        controller.presentation.fullScreenAllowed = fullScreenPolicy
        popover.startButton.performClick(nil)
        XCTAssertFalse(popover.isShown)
        XCTAssertEqual(controller.presentation.options, PresentationOptions(mode: .play, startSlide: 1, record: false))
        XCTAssertEqual(AppEnvironment.shared.presentationSettings.settings, PresentationSettings(record: false, phoneRemote: false),
                       "a start saves its settings for Play and the next launch")
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        let port = DeckPortStore.suggestedPort(for: try XCTUnwrap(document.fileURL))
        XCTAssertEqual(controller.presentation.session?.command, .present(record: false, presenterPassword: nil, port: port), "the deck's own port")
        try await waitUntil(timeout: 20, "the talk windows to settle") { controller.presentation.windowsAreSettled }
        XCTAssertEqual(controller.presentation.audienceWindow?.targetFrame, screens[0].frame, "the audience the person chose")
    }

    func testThreeDisplays() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let screens = thirdScreens()
        controller.presentation.screens = { screens }
        deckWindow.showPresentSettings(nil)
        let popover = deckWindow.presentPopover
        XCTAssertTrue(popover.isShown)
        XCTAssertEqual(popover.arrangementView.tiles.map(\.screen.name), ["Built-in Display", "Projector", "Studio Display"], "every connected display is drawn")
        XCTAssertEqual(roles(popover), ["Built-in Display": "Presenter", "Projector": "Audience", "Studio Display": "Not used"])
        XCTAssertTrue(popover.arrangementView.tiles.allSatisfy { $0.menuButton.title == $0.screen.name }, "each menu is named for its display")

        // An unused display takes the audience; the old audience is left unused.
        try choose(.audience, for: "Studio Display", in: popover)
        XCTAssertEqual(roles(popover), ["Built-in Display": "Presenter", "Projector": "Not used", "Studio Display": "Audience"])
        // A role that is taken swaps with its holder.
        try choose(.presenter, for: "Studio Display", in: popover)
        XCTAssertEqual(roles(popover), ["Built-in Display": "Audience", "Projector": "Not used", "Studio Display": "Presenter"])
        try choose(.presenter, for: "Projector", in: popover)
        XCTAssertEqual(roles(popover), ["Built-in Display": "Audience", "Projector": "Presenter", "Studio Display": "Not used"])
        try choose(.notUsed, for: "Projector", in: popover)
        XCTAssertEqual(roles(popover), ["Built-in Display": "Audience", "Projector": "Not used", "Studio Display": "Not used"], "a talk needs no presenter display")
        XCTAssertEqual(controller.presentation.currentArrangement?.audience.name, "Built-in Display")
        XCTAssertEqual(AppEnvironment.shared.displayAssignments.roles(for: screens), RememberedDisplayRoles(audience: "Built-in Display", presenter: nil),
                       "remembered for this set of displays")
        popover.close()

        // The displays changed while the popover is up: it follows.
        deckWindow.showPresentSettings(nil)
        let two = Array(screens.prefix(2))
        controller.presentation.screens = { two }
        NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
        XCTAssertEqual(popover.arrangementView.tiles.map(\.screen.name), ["Built-in Display", "Projector"], "an unplugged display leaves the diagram")
        controller.presentation.screens = { [screens[0]] }
        NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
        XCTAssertTrue(popover.displaysSection.isHidden, "one display left: no displays section")
        popover.close()
    }

    func testCmdOptionPStartsAtOnceWithTheLastSettings() async throws {
        AppEnvironment.shared.presentationSettings.settings = PresentationSettings(record: false, phoneRemote: true)
        let (document, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        controller.jumpToSlide(number: 4)
        deckWindow.presentPopover.passwordSwitch.state = .on
        deckWindow.presentPopover.passwordField.stringValue = "secret"
        // Present > Play from Slide 4, Cmd+Option+P: no popover, the last settings, the cursor's slide.
        deckWindow.play(nil)
        XCTAssertFalse(deckWindow.presentPopover.isShown)
        XCTAssertEqual(controller.presentation.options,
                       PresentationOptions(mode: .play, startSlide: 4, record: false, phoneRemote: true, presenterPassword: "secret"))
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        let port = DeckPortStore.suggestedPort(for: try XCTUnwrap(document.fileURL))
        XCTAssertEqual(controller.presentation.session?.command, .present(record: false, presenterPassword: "secret", port: port))
        try await stopPresenting(controller)
        // Present > Present Settings opens the popover, with the same settings showing.
        deckWindow.showPresentSettings(nil)
        XCTAssertTrue(deckWindow.presentPopover.isShown)
        XCTAssertEqual(deckWindow.presentPopover.recordSwitch.state, .off)
        XCTAssertEqual(deckWindow.presentPopover.phoneRemoteSwitch.state, .on)
        deckWindow.presentPopover.close()

        // The settings are app-wide: a second deck's Cmd+Option+P starts with what the first deck last chose, never with its own stale controls.
        let (_, other) = try await openDeckForPresenting()
        let otherWindow = try windowController(other)
        otherWindow.showPresentSettings(nil)
        otherWindow.presentPopover.recordSwitch.state = .on
        otherWindow.presentPopover.close()
        // The first deck starts from its popover, which saves its controls.
        deckWindow.showPresentSettings(nil)
        deckWindow.presentPopover.recordSwitch.state = .off
        deckWindow.presentPopover.phoneRemoteSwitch.state = .off
        deckWindow.presentPopover.startButton.performClick(nil)
        XCTAssertEqual(controller.presentation.options?.record, false)
        XCTAssertEqual(controller.presentation.options?.phoneRemote, false, "the first deck saved its own controls")
        XCTAssertEqual(AppEnvironment.shared.presentationSettings.settings.phoneRemote, false)
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
        try await stopPresenting(controller)
        other.jumpToSlide(number: 3)
        otherWindow.play(nil)
        XCTAssertEqual(other.presentation.options?.record, false, "the newest saved settings, not this popover's stale controls")
        XCTAssertEqual(other.presentation.options?.startSlide, 3, "this deck's cursor, read now")
        try await waitUntil(timeout: 40, "the other talk") { other.presentation.state == .presenting }
    }

    func testTheDisplaysAreDrawnLeftToRight() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let screens = halfScreens()
        controller.presentation.screens = { screens.reversed() }
        deckWindow.showPresentSettings(nil)
        XCTAssertEqual(deckWindow.presentPopover.arrangementView.tiles.map(\.screen.name), ["Built-in Display", "Projector"], "by position, not the order the system lists them")
        deckWindow.presentPopover.close()
    }

    func testThePopoverWithOneDisplayHidesTheDisplayControls() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        deckWindow.playButtonClicked(modifiers: [])
        let popover = deckWindow.presentPopover
        XCTAssertTrue(popover.isShown)
        XCTAssertTrue(popover.displaysSection.isHidden, "no displays section, no caption, no menus")
        XCTAssertTrue(popover.spacesNotice.isHidden, "one display never needs the setting")
        XCTAssertTrue(popover.startFromPopUp.isHidden, "slide 1: nothing to choose")
        XCTAssertTrue(popover.startFromLabel.isHidden)
        popover.rehearseButton.performClick(nil)
        XCTAssertFalse(popover.isShown)
        XCTAssertEqual(controller.presentation.options?.mode, .rehearse)
        XCTAssertEqual(controller.presentation.options?.startSlide, 1)
        try await waitUntil(timeout: 40, "the rehearsal") { controller.presentation.state == .presenting }
    }

    func testDisplaysShareOneSpace() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let screens = halfScreens()
        controller.presentation.screens = { screens }
        controller.presentation.screensHaveSeparateSpaces = { false }
        var openedSettings: [URL] = []
        deckWindow.openSystemSettings = { openedSettings.append($0) }
        deckWindow.showPresentSettings(nil)
        let popover = deckWindow.presentPopover
        XCTAssertFalse(popover.spacesNotice.isHidden)
        XCTAssertTrue(popover.spacesNoticeLabel.stringValue.contains("own Space"))
        XCTAssertTrue(popover.startButton.isEnabled, "the talk still runs, as plain windows")
        popover.spacesSettingsButton.performClick(nil)
        XCTAssertEqual(openedSettings, [DeckWindowController.desktopSettingsURL])
        XCTAssertEqual(DeckWindowController.desktopSettingsURL.absoluteString, "x-apple.systempreferences:com.apple.Desktop-Settings.extension")
        popover.close()
    }

    func testPhoneRemoteHelp() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        deckWindow.showPresentSettings(nil)
        let popover = deckWindow.presentPopover
        let help = try XCTUnwrap(popover.phoneRemoteRow.helpButton)
        XCTAssertEqual(help.bezelStyle, .helpButton)
        XCTAssertFalse(popover.isHelpShown)
        help.performClick(nil)
        XCTAssertTrue(popover.isHelpShown, "the (?) opens a popover")
        XCTAssertTrue(popover.helpText.contains("Phone remote"))
        XCTAssertTrue(popover.helpText.contains("cloudflared"), "the tunnel's tool is explained where the remote is")
        XCTAssertTrue(popover.helpText.contains("Wi\u{2011}Fi"))
        popover.close()
        XCTAssertFalse(popover.isHelpShown, "the help goes with the popover")
    }

    func testPhoneRemoteNeedsCloudflared() async throws {
        AppEnvironment.shared.cloudflaredProbe = { false }
        AppEnvironment.shared.presentationSettings.settings = PresentationSettings(record: true, phoneRemote: true)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.cloudflared.\(UUID().uuidString)"))
        deckWindow.presentPopover.pasteboard = pasteboard
        deckWindow.showPresentSettings(nil)
        let popover = deckWindow.presentPopover
        XCTAssertFalse(popover.cloudflaredInstalled)
        XCTAssertFalse(popover.cloudflaredBlock.isHidden)
        XCTAssertEqual(popover.cloudflaredMessage.stringValue, "Needs cloudflared, which is not installed.")
        XCTAssertEqual(popover.cloudflaredMessage.textColor, .systemRed)
        popover.copyInstallButton.performClick(nil)
        XCTAssertEqual(pasteboard.string(forType: .string), "brew install cloudflared")
        XCTAssertTrue(popover.startButton.isEnabled, "Play stays enabled")
        XCTAssertEqual(popover.options(mode: .play).phoneRemote, false, "and starts without the remote")
        popover.phoneRemoteSwitch.performClick(nil)
        XCTAssertTrue(popover.cloudflaredBlock.isHidden, "no complaint with the remote off")
        popover.phoneRemoteSwitch.performClick(nil)
        AppEnvironment.shared.cloudflaredProbe = { true }
        deckWindow.showPresentSettings(nil)
        XCTAssertTrue(popover.cloudflaredInstalled)
        XCTAssertTrue(popover.cloudflaredBlock.isHidden, "installing it clears the message")
        popover.close()
    }

    func testPlayWithTheRemoteOnAndNoCloudflaredOpensSettingsInstead() async throws {
        AppEnvironment.shared.cloudflaredProbe = { false }
        AppEnvironment.shared.presentationSettings.settings = PresentationSettings(record: false, phoneRemote: true)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        try markPlayed(controller)
        let popover = deckWindow.presentPopover
        for start in [{ deckWindow.playButtonClicked(modifiers: []) }, { deckWindow.playButtonClicked(modifiers: [.shift]) },
                      { deckWindow.play(nil) }, { deckWindow.playFromBeginning(nil) }] {
            start()
            XCTAssertTrue(popover.isShown, "Present Settings open instead of the talk")
            XCTAssertFalse(popover.cloudflaredBlock.isHidden, "showing the cloudflared fix")
            XCTAssertNil(controller.presentation.options, "nothing started")
            XCTAssertEqual(controller.presentation.state, .idle)
            popover.close()
        }
        AppEnvironment.shared.cloudflaredProbe = { true }
        deckWindow.play(nil)
        XCTAssertEqual(controller.presentation.options?.phoneRemote, true, "with cloudflared the remote starts")
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
    }

    func testThePlayButtonFollowsTheTalk() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let identifiers = try XCTUnwrap(deckWindow.window?.toolbar?.items.map(\.itemIdentifier))
        XCTAssertTrue(identifiers.contains(DeckWindowController.playItemIdentifier))
        XCTAssertEqual(deckWindow.playButton.accessibilityIdentifier(), "play-button")
        XCTAssertEqual(deckWindow.playButton.segmentCount, 2, "a play segment and a chevron")
        XCTAssertTrue(deckWindow.playButton.isEnabled)
        deckWindow.rehearse(nil)
        XCTAssertFalse(deckWindow.playButton.isEnabled, "no second talk while one runs")
        try await waitUntil(timeout: 40, "the rehearsal") { controller.presentation.state == .presenting }
        try await stopPresenting(controller)
        XCTAssertTrue(deckWindow.playButton.isEnabled)
    }

    func testTheToolbarOrder() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let identifiers = try XCTUnwrap(deckWindow.window?.toolbar?.items.map(\.itemIdentifier))
        let spaces: Set<NSToolbarItem.Identifier> = [.space, .flexibleSpace]
        XCTAssertEqual(identifiers.filter { !spaces.contains($0) },
                       [DeckWindowController.slidesItemIdentifier, DeckWindowController.newSlideItemIdentifier, DeckWindowController.themeItemIdentifier,
                        DeckWindowController.playItemIdentifier, DeckWindowController.previewItemIdentifier],
                       "Slides, then New Slide and Theme, then Play, then Preview")
        let play = try XCTUnwrap(identifiers.firstIndex(of: DeckWindowController.playItemIdentifier))
        let theme = try XCTUnwrap(identifiers.firstIndex(of: DeckWindowController.themeItemIdentifier))
        let preview = try XCTUnwrap(identifiers.firstIndex(of: DeckWindowController.previewItemIdentifier))
        XCTAssertTrue(identifiers[(theme + 1)..<play].contains(.space), "Play stands apart from the deck actions")
        XCTAssertTrue(identifiers[(play + 1)..<preview].contains(.space), "and from Preview")
        XCTAssertTrue(identifiers[..<theme].contains(.flexibleSpace), "the deck actions follow a flexible space")
    }

    func testThePlayMenuNamesTheSlide() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        var menu = deckWindow.makePlayMenu()
        XCTAssertEqual(menu.items.filter { !$0.isSeparatorItem }.map(\.title), ["Play from Slide 1", "Rehearse", "Present Settings\u{2026}"],
                       "from slide 1 the beginning would be the same play")
        controller.jumpToSlide(number: 3)
        menu = deckWindow.makePlayMenu()
        XCTAssertEqual(menu.items.filter { !$0.isSeparatorItem }.map(\.title),
                       ["Play from Slide 3", "Play from Beginning", "Rehearse", "Present Settings\u{2026}"])
        XCTAssertEqual(menu.items[0].keyEquivalent, "p")
        XCTAssertEqual(menu.items[0].keyEquivalentModifierMask, [.command, .option])
        XCTAssertEqual(menu.items[1].badge?.stringValue, "\u{21E7} click")
        let rehearse = try XCTUnwrap(menu.items.first { $0.title == "Rehearse" })
        XCTAssertEqual(rehearse.keyEquivalentModifierMask, [.command, .option, .shift])
        XCTAssertTrue(menu.items.last?.action == #selector(DeckWindowController.showPresentSettings(_:)))
        XCTAssertTrue(menu.items[menu.items.count - 2].isSeparatorItem)
    }

    func testThePlayButtonFollowsATalkInAnotherDeck() async throws {
        let (_, first) = try await openDeckForPresenting()
        let firstWindow = try windowController(first)
        try await startPresenting(first, PresentationOptions(mode: .rehearse, startSlide: 1))
        // A deck opened during another deck's talk.
        let (_, second) = try await openDeckForPresenting()
        let secondWindow = try windowController(second)
        XCTAssertFalse(secondWindow.playButton.isEnabled, "no second talk while one runs")
        try await stopPresenting(first)
        XCTAssertTrue(secondWindow.playButton.isEnabled, "the other deck's Play comes back when the talk ends")
        XCTAssertTrue(firstWindow.playButton.isEnabled)
        // And the other way: a deck open before the talk turns its Play off.
        try await startPresenting(second, PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertFalse(firstWindow.playButton.isEnabled, "a deck open before the talk follows it too")
        try await stopPresenting(second)
        XCTAssertTrue(firstWindow.playButton.isEnabled)
    }

    func testRehearseFromTheMenuLeavesTheSavedSettings() async throws {
        let saved = PresentationSettings(record: true, phoneRemote: false)
        AppEnvironment.shared.presentationSettings.settings = saved
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        // This deck's popover was last shown with recording off, and never started from.
        deckWindow.showPresentSettings(nil)
        deckWindow.presentPopover.recordSwitch.state = .off
        deckWindow.presentPopover.close()
        deckWindow.rehearse(nil)
        XCTAssertEqual(controller.presentation.options?.mode, .rehearse)
        XCTAssertEqual(AppEnvironment.shared.presentationSettings.settings, saved, "Rehearse from the menu saves no settings: the next Cmd+Option+P still records")
        try await waitUntil(timeout: 40, "the rehearsal") { controller.presentation.state == .presenting }
    }
}
