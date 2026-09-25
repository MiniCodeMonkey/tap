import XCTest
@testable import Tap

/// Two displays on one screen: the left half is the laptop, the right half
/// the projector. A full screen window fills the whole display it is on,
/// so on one screen both windows end up as two Spaces of that display;
/// what these tests check is the frame each window was asked for and the
/// full screen state the window server reports, which is what a real
/// projector would also show.
final class PresentingDisplayTests: PresentingTestCase {
    func testStartPresentingWithTwoDisplays() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let screens = halfScreens()
        controller.presentation.screens = { screens }
        controller.jumpToSlide(number: 3)
        XCTAssertEqual(controller.currentSlideNumber, 3)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 3))

        let talk = try XCTUnwrap(presentation.session)
        XCTAssertEqual(presentation.client?.ready.port, AppEnvironment.shared.deckPorts.port(for: try XCTUnwrap(controller.document?.fileURL)))
        XCTAssertTrue(talk.log.text.contains("tap present --app"))
        XCTAssertNotEqual(talk.processIdentifier, controller.session.processIdentifier, "a second process")
        if case .running = controller.session.state {} else { XCTFail("the preview keeps running from tap dev") }

        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(audience.targetFrame, screens[1].frame, "the audience goes to the projector")
        XCTAssertEqual(presenter.targetFrame, screens[0].frame, "the presenter view goes to the built-in display")
        XCTAssertFalse(presenter.isAttached, "on two displays the presenter view is a window of its own")
        XCTAssertTrue(presenter.isVisible)
        XCTAssertTrue(presentation.frontWindow === presenter, "the speaker's keys go to the presenter window")
        try await waitUntil(timeout: 20, "the audience page on slide 3") { audience.page.lastReady?.slide == 3 }
        try await waitUntil(timeout: 20, "the presenter page on slide 3") { presenter.page.lastReady?.slide == 3 }
    }

    func testTwoDisplaysAreTwoSpacesThatLeaveOneAtATime() async throws {
        try await requireSecondSpace()
        let (_, controller) = try await openDeckForPresenting()
        let screens = halfScreens()
        let presentation = controller.presentation
        presentation.screens = { screens }
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(audience.fullScreenState, .fullScreen)
        XCTAssertEqual(presenter.fullScreenState, .fullScreen)
        XCTAssertEqual(fullScreenPresentationWindows().count, 2)
        try await waitUntil(timeout: 5, "the presenter's Space active, the audience's not") {
            let order = onScreenWindowNumbers()
            return order.contains(presenter.windowNumber) && !order.contains(audience.windowNumber)
        }

        // Stop: the front window leaves its Space and closes, then the other; never both at once.
        var events: [String] = []
        for window in [audience, presenter] {
            let name = window.role == .audience ? "audience" : "presenter"
            window.onFullScreenChange = { events.append("\(name) \($0)") }
            window.onClosed = { events.append("\(name) closed") }
        }
        presentation.stop()
        try await waitUntil(timeout: 20, "both windows closed") { audience.isClosed && presenter.isClosed }
        XCTAssertEqual(events, ["presenter exiting", "presenter windowed", "presenter closed", "audience exiting", "audience windowed", "audience closed"],
                       "each leaves full screen before it closes, and the second waits for the first")
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty)
        try await waitUntil(timeout: 30, "the talk to end") { presentation.state == .idle }
    }

    func testSwapDisplays() async throws {
        try await requireSecondSpace()
        let (_, controller) = try await openDeckForPresenting()
        let screens = halfScreens()
        let presentation = controller.presentation
        presentation.screens = { screens }
        XCTAssertEqual(presentation.currentArrangement?.audience.name, "Projector")
        // Before the talk: the popover's Swap Displays.
        presentation.swapDisplays()
        XCTAssertEqual(presentation.currentArrangement?.audience.name, "Built-in Display")
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(audience.targetFrame, screens[0].frame, "the presenter and audience displays swapped before the start")
        XCTAssertEqual(presenter.targetFrame, screens[1].frame)
        // During the talk: the toolbar's Swap Displays. Each window leaves its Space, moves and enters again.
        presentation.swapDisplays()
        XCTAssertEqual(audience.targetFrame, screens[1].frame)
        XCTAssertEqual(presenter.targetFrame, screens[0].frame)
        XCTAssertFalse(presentation.windowsAreSettled)
        try await waitUntil(timeout: 30, "the windows back in full screen on their new displays") {
            presentation.windowsAreSettled && audience.fullScreenState == .fullScreen && presenter.fullScreenState == .fullScreen
        }
        XCTAssertTrue(presentation.sleepAssertion.isHeld, "a swap is not an ending")
        XCTAssertEqual(fullScreenPresentationWindows().count, 2)
    }

    func testSwapDisplaysAfterATalkReadsTheDisplaysConnectedNow() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        let screens = halfScreens()
        presentation.screens = { screens }
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        try await stopPresenting(controller)
        XCTAssertNil(presentation.arrangement, "a finished talk's arrangement is not kept")
        // The projector is gone, another is plugged in: the popover shows what is connected now, and Swap swaps that.
        let other = [screens[0], ScreenInfo(name: "Epson", frame: screens[1].frame, isBuiltIn: false)]
        presentation.screens = { other }
        XCTAssertEqual(presentation.currentArrangement?.audience.name, "Epson")
        presentation.swapDisplays()
        XCTAssertEqual(presentation.currentArrangement?.audience.name, "Built-in Display")
        XCTAssertEqual(AppEnvironment.shared.displayAssignments.audienceName(for: other), "Built-in Display")
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertEqual(presentation.audienceWindow?.targetFrame, other[0].frame, "the next talk uses the swapped displays")
    }

    func testRememberTheDisplayAssignment() async throws {
        let (_, first) = try await openDeckForPresenting()
        let screens = halfScreens()
        first.presentation.screens = { screens }
        first.presentation.swapDisplays()
        try await startPresenting(first, PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertEqual(first.presentation.audienceWindow?.targetFrame, screens[0].frame)
        try await stopPresenting(first)
        XCTAssertEqual(AppEnvironment.shared.displayAssignments.audienceName(for: screens), "Built-in Display")

        // Another deck, the same pair of displays, in either order.
        let (_, second) = try await openDeckForPresenting()
        second.presentation.screens = { screens.reversed() }
        try await startPresenting(second, PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertEqual(second.presentation.audienceWindow?.targetFrame, screens[0].frame, "the same assignment, for every deck")
        XCTAssertEqual(second.presentation.presenterWindow?.targetFrame, screens[1].frame)
    }

    func testRehearse() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let screens = halfScreens()
        let presentation = controller.presentation
        presentation.screens = { screens }
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 4))
        let talk = try XCTUnwrap(presentation.session)
        // The deck's own port, as for a talk: the presenter page keeps its origin, and with it the presenter layout and notes size.
        let port = DeckPortStore.suggestedPort(for: try XCTUnwrap(document.fileURL))
        XCTAssertEqual(talk.command, .present(record: false, presenterPassword: nil, port: port))
        XCTAssertTrue(talk.log.text.contains("tap present --app --no-record --port \(port) ops.md"))
        XCTAssertNil(presentation.audienceWindow, "only the presenter view")
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(presenter.targetFrame, screens[0].frame, "on the laptop's display")
        XCTAssertTrue(presenter.isVisible)
        XCTAssertFalse(presenter.isAttached, "nothing to ride on: it has the display")
        XCTAssertTrue(presentation.frontWindow === presenter)
        try await waitUntil(timeout: 20, "the presenter page, with its timer, on slide 4") { presenter.page.lastReady?.slide == 4 }
        XCTAssertFalse(presentation.recording.isRecording)
    }

    func testTheAudienceWindowFallsBackWhenTheProjectorGoes() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let two = halfScreens()
        let one = oneScreen()
        let presentation = controller.presentation
        presentation.screens = { two }
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)

        // The projector is unplugged: macOS has moved its Space to the remaining
        // display already. The presenter view leaves its own Space and comes
        // over the audience view as its child, shown, since the speaker is at the laptop.
        presentation.screens = { one }
        presentation.screensChanged()
        XCTAssertEqual(presentation.arrangement?.isSingleDisplay, true)
        // The windows are placed one at a time, so each frame is read once both are settled.
        try await waitUntil(timeout: 30, "the windows settled and the presenter over the audience") {
            presentation.windowsAreSettled && presentation.presenterIsShownOverAudience
        }
        XCTAssertEqual(audience.targetFrame, one[0].frame, "the audience is asked for the remaining screen")
        XCTAssertEqual(presenter.frame, audience.frame, "the presenter view covers the audience view")
        XCTAssertTrue(presenter.isAttached)
        XCTAssertEqual(presenter.fullScreenState, .windowed, "a child has no Space of its own")
        XCTAssertTrue(presentation.frontWindow === presenter)
        try await waitUntil(timeout: 5, "both on screen, the presenter in front") {
            let order = onScreenWindowNumbers()
            guard let a = order.firstIndex(of: audience.windowNumber), let p = order.firstIndex(of: presenter.windowNumber) else { return false }
            return p < a
        }
        XCTAssertTrue(presentation.sleepAssertion.isHeld)
        XCTAssertLessThanOrEqual(fullScreenPresentationWindows().count, 1, "at most the audience window is in full screen")
        presentation.toggleFrontWindow()
        XCTAssertFalse(presentation.presenterIsShownOverAudience, "Option-Tab works as on one display now (Task 7 wires the key)")

        // The projector is back: the presenter view gets its own display and Space again, the audience goes to the projector.
        presentation.screens = { two }
        presentation.screensChanged()
        try await waitUntil(timeout: 30, "the windows back on their displays") {
            presentation.windowsAreSettled && !presenter.isAttached && presenter.isVisible
        }
        XCTAssertEqual(audience.targetFrame, two[1].frame)
        XCTAssertEqual(presenter.targetFrame, two[0].frame)
        XCTAssertTrue(presentation.frontWindow === presenter)
        presentation.toggleFrontWindow()
        XCTAssertFalse(presenter.isAttached, "nothing to toggle with a display each")
    }

    func testTheScreenObserverReachesTheTalkOnlyWhileItsWindowsExist() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        var changes = 0
        presentation.onScreensChanged = { changes += 1 }
        NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
        XCTAssertEqual(changes, 0, "no talk, no observer")
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        // AppKit posts the notification itself while the window enters full screen: counted from here on.
        try await Task.sleep(nanoseconds: 1_000_000_000)
        let settled = changes
        NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
        XCTAssertEqual(changes, settled + 1, "AppKit's notification reaches the talk")
        try await stopPresenting(controller)
        let stopped = changes
        NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
        XCTAssertEqual(changes, stopped, "the observer went with the windows")
    }

    func testOneDisplayScreenNotificationsLeaveTheAudienceInFront() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        if fullScreenAvailable {
            // AppKit posts didChangeScreenParametersNotification while the audience window enters full screen.
            try await waitUntil(timeout: 10, "the audience in full screen") { audience.styleMask.contains(.fullScreen) }
            try await Task.sleep(nanoseconds: 1_500_000_000)
        }
        XCTAssertTrue(presentation.frontWindow === audience, "the talk starts on the audience view")
        XCTAssertFalse(presenter.isAttached, "the notes stay off the audience's screen until Option-Tab or the S key")
        XCTAssertFalse(presenter.isVisible)

        // A notification with the same displays: nothing moves, nothing shows.
        NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
        XCTAssertTrue(presentation.windowsAreSettled, "no window was asked to move")
        XCTAssertTrue(presentation.frontWindow === audience)
        XCTAssertFalse(presenter.isAttached)
        XCTAssertFalse(presenter.isVisible)
        try await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertFalse(presentation.presenterIsShownOverAudience)
        XCTAssertFalse(onScreenWindowNumbers().contains(presenter.windowNumber), "the window server has no notes on screen")

        // The one display changes mode: the audience moves to the new frame, and the notes stay hidden.
        let full = oneScreen()[0]
        let resized = [ScreenInfo(name: full.name, frame: full.frame.insetBy(dx: 100, dy: 100), isBuiltIn: true)]
        presentation.screens = { resized }
        presentation.screensChanged()
        var notesSeen = false
        try await waitUntil(timeout: 30, "the audience on the new frame") {
            if onScreenWindowNumbers().contains(presenter.windowNumber) { notesSeen = true }
            return presentation.windowsAreSettled && audience.settledFrame == resized[0].frame
        }
        XCTAssertFalse(notesSeen, "the notes never came on screen while the audience window moved")
        XCTAssertTrue(presentation.frontWindow === audience)
        XCTAssertFalse(presenter.isAttached, "a change of mode on one display does not bring the notes over the audience")
        XCTAssertFalse(presenter.isVisible)

        // With the notes over the audience, a change of mode keeps them there.
        presentation.toggleFrontWindow()
        XCTAssertTrue(presentation.presenterIsShownOverAudience)
        presentation.screens = { [full] }
        presentation.screensChanged()
        try await waitUntil(timeout: 30, "the windows back on the full frame, the notes over the audience") {
            presentation.windowsAreSettled && audience.settledFrame == full.frame && presentation.presenterIsShownOverAudience
        }
        XCTAssertTrue(presentation.frontWindow === presenter)
    }

    /// Two displays on one screen, with AppKit's full screen toggle
    /// replaced by a recorder and each transition's end driven by hand, so
    /// the order holds on every host and no Space is made: the entries go
    /// one at a time, a swap moves each window in turn, and a take-down
    /// takes the front window down before it starts on the other.
    func testTwoDisplayOrderAndSwapThroughTheSeams() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        let screens = halfScreens()
        presentation.screens = { screens }
        presentation.fullScreenAllowed = { true }
        presentation.screensHaveSeparateSpaces = { true }
        var toggles: [String] = []
        presentation.windowCreated = { window in
            let name = window.role == .audience ? "audience" : "presenter"
            window.requestFullScreenToggle = { toggles.append("\(name) toggle") }
        }
        let entered = Notification(name: NSWindow.didEnterFullScreenNotification)
        let exited = Notification(name: NSWindow.didExitFullScreenNotification)

        // The start: the audience enters first, and the presenter only once it is in.
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 40, "the audience's entry (state \(presentation.state), toggles \(toggles))") { toggles == ["audience toggle"] }
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertFalse(presentation.windowsAreSettled)
        audience.windowDidEnterFullScreen(entered)
        XCTAssertEqual(toggles, ["audience toggle", "presenter toggle"])
        presenter.windowDidEnterFullScreen(entered)
        XCTAssertTrue(presentation.windowsAreSettled)
        XCTAssertTrue(presentation.frontWindow === presenter)
        XCTAssertEqual(audience.targetFrame, screens[1].frame)
        XCTAssertEqual(presenter.targetFrame, screens[0].frame)

        // The swap: the audience leaves its Space, moves and enters again; only then the presenter.
        toggles = []
        presentation.swapDisplays()
        XCTAssertEqual(toggles, ["audience toggle"], "the audience's exit, and nothing for the presenter yet")
        audience.windowDidExitFullScreen(exited)
        XCTAssertEqual(toggles, ["audience toggle", "audience toggle"], "the audience's entry on its new display")
        audience.windowDidEnterFullScreen(entered)
        XCTAssertEqual(audience.targetFrame, screens[0].frame)
        XCTAssertEqual(audience.settledFrame, screens[0].frame)
        XCTAssertEqual(toggles, ["audience toggle", "audience toggle", "presenter toggle"], "the presenter's exit, once the audience is in")
        presenter.windowDidExitFullScreen(exited)
        XCTAssertEqual(toggles.count, 4)
        presenter.windowDidEnterFullScreen(entered)
        XCTAssertEqual(presenter.targetFrame, screens[1].frame)
        XCTAssertEqual(presenter.settledFrame, screens[1].frame)
        XCTAssertTrue(presentation.windowsAreSettled)
        XCTAssertFalse(presenter.isAttached)
        XCTAssertTrue(presentation.frontWindow === presenter)
        XCTAssertTrue(presentation.sleepAssertion.isHeld, "a swap is not an ending")

        // The take-down: the front window leaves and closes before the other starts.
        toggles = []
        presentation.stop()
        XCTAssertEqual(toggles, ["presenter toggle"], "only the front window leaves first")
        presenter.windowDidExitFullScreen(exited)
        XCTAssertTrue(presenter.isClosed)
        XCTAssertFalse(audience.isClosed)
        XCTAssertEqual(toggles, ["presenter toggle", "audience toggle"], "the audience leaves once the presenter has closed")
        audience.windowDidExitFullScreen(exited)
        XCTAssertTrue(audience.isClosed)
        XCTAssertTrue(presentation.windowsGoingDown.isEmpty)
        try await waitUntil(timeout: 30, "the talk to end") { presentation.state == .idle }
    }

    /// One display with the presenter view over the audience view as its
    /// child, then the projector comes back: the presenter is taken out
    /// from over the audience before either window is placed, so it can
    /// have a Space of its own. AppKit's full screen toggle is a recorder
    /// and each transition's end is driven by hand, so this runs on every host.
    func testTheProjectorComingBackTakesThePresenterFromOverTheAudience() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        let one = oneScreen()
        let two = halfScreens()
        presentation.screens = { one }
        presentation.fullScreenAllowed = { true }
        presentation.screensHaveSeparateSpaces = { true }
        var toggles: [String] = []
        presentation.windowCreated = { window in
            let name = window.role == .audience ? "audience" : "presenter"
            window.requestFullScreenToggle = { toggles.append("\(name) toggle") }
        }
        let entered = Notification(name: NSWindow.didEnterFullScreenNotification)
        let exited = Notification(name: NSWindow.didExitFullScreenNotification)
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 40, "the audience's entry (state \(presentation.state), toggles \(toggles))") { toggles == ["audience toggle"] }
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        audience.windowDidEnterFullScreen(entered)
        XCTAssertTrue(presentation.windowsAreSettled)
        presentation.toggleFrontWindow()
        XCTAssertTrue(presenter.isAttached, "the presenter view over the audience view")
        XCTAssertTrue(presenter.parent === audience)

        // The projector is plugged in: the presenter leaves the audience before anything moves.
        toggles = []
        presentation.screens = { two }
        presentation.screensChanged()
        XCTAssertFalse(presenter.isAttached, "taken out from over the audience")
        XCTAssertNil(presenter.parent)
        XCTAssertFalse(audience.childWindows?.contains { $0 === presenter } ?? false)
        XCTAssertEqual(toggles, ["audience toggle"], "the audience leaves its Space first, to move")
        audience.windowDidExitFullScreen(exited)
        audience.windowDidEnterFullScreen(entered)
        XCTAssertEqual(toggles, ["audience toggle", "audience toggle", "presenter toggle"], "then the presenter enters a Space of its own")
        presenter.windowDidEnterFullScreen(entered)
        XCTAssertTrue(presentation.windowsAreSettled)
        XCTAssertEqual(presenter.fullScreenState, .fullScreen)
        XCTAssertEqual(presenter.targetFrame, two[0].frame)
        XCTAssertEqual(audience.targetFrame, two[1].frame)
        XCTAssertTrue(presentation.frontWindow === presenter)

        // The take-down, driven the same way.
        presentation.stop()
        presenter.windowDidExitFullScreen(exited)
        audience.windowDidExitFullScreen(exited)
        XCTAssertTrue(presenter.isClosed)
        XCTAssertTrue(audience.isClosed)
        try await waitUntil(timeout: 30, "the talk to end") { presentation.state == .idle }
    }

    func testWithoutSeparateSpacesTheTalkWindowsStayPlainWindows() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let screens = halfScreens()
        let presentation = controller.presentation
        presentation.screens = { screens }
        // "Displays have separate Spaces" is off: one full screen Space would black out the other display.
        // The host allows full screen here, so the setting is the only reason for plain windows.
        presentation.fullScreenAllowed = { true }
        presentation.screensHaveSeparateSpaces = { false }
        XCTAssertFalse(presentation.usesFullScreen)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(audience.fullScreenState, .windowed)
        XCTAssertEqual(presenter.fullScreenState, .windowed)
        XCTAssertTrue(audience.isVisible)
        XCTAssertTrue(presenter.isVisible)
        XCTAssertEqual(audience.targetFrame, screens[1].frame)
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty)
        XCTAssertTrue(presentation.session?.log.text.contains("Displays have separate Spaces") == true, "the talk's log says why")
        try await stopPresenting(controller)
        // One display never needs the setting. usesFullScreen reads the displays connected now, not a talk's.
        presentation.screens = { self.oneScreen() }
        XCTAssertTrue(presentation.usesFullScreen, "true wherever the host allows full screen at all")
    }
}
