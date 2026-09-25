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
        let (_, controller) = try await openDeckForPresenting()
        let screens = halfScreens()
        let presentation = controller.presentation
        presentation.screens = { screens }
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 4))
        let talk = try XCTUnwrap(presentation.session)
        XCTAssertEqual(talk.command, .present(record: false, presenterPassword: nil, port: nil))
        XCTAssertTrue(talk.log.text.contains("tap present --app --no-record ops.md"))
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
        XCTAssertEqual(audience.targetFrame, one[0].frame, "the audience is asked for the remaining screen")
        XCTAssertEqual(presenter.targetFrame, one[0].frame)
        try await waitUntil(timeout: 30, "the windows settled and the presenter over the audience") {
            presentation.windowsAreSettled && presentation.presenterIsShownOverAudience
        }
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
        XCTAssertEqual(audience.targetFrame, two[1].frame)
        XCTAssertEqual(presenter.targetFrame, two[0].frame)
        try await waitUntil(timeout: 30, "the windows back on their displays") {
            presentation.windowsAreSettled && !presenter.isAttached && presenter.isVisible
        }
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
        NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
        XCTAssertEqual(changes, 1, "AppKit's notification reaches the talk")
        try await stopPresenting(controller)
        NotificationCenter.default.post(name: NSApplication.didChangeScreenParametersNotification, object: NSApp)
        XCTAssertEqual(changes, 1, "the observer went with the windows")
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
