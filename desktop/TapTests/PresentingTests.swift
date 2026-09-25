import XCTest
@testable import Tap

final class PresentingTests: PresentingTestCase {
    func testSaveBeforePresenting() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deck = try XCTUnwrap(document.fileURL)
        let end = (controller.editor.string as NSString).range(of: "# One").upperBound
        controller.editor.setSelectedRange(NSRange(location: end, length: 0))
        controller.editor.insertText(" edited", replacementRange: NSRange(location: NSNotFound, length: 0))
        XCTAssertTrue(document.isDocumentEdited)

        controller.presentation.start(PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertEqual(controller.presentation.state, .starting)
        XCTAssertTrue(AppEnvironment.shared.isPresenting, "counted from the start, so no other deck can start during the save")
        // The process exists only once the save has completed.
        try await waitUntil(timeout: 10, "tap present to be started") { controller.presentation.session != nil }
        let onDisk = try String(contentsOf: deck, encoding: .utf8)
        XCTAssertTrue(onDisk.contains("# One edited"), "the buffer reached the file before tap present started")
        XCTAssertFalse(document.isDocumentEdited)
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
    }

    func testStopThenPlayDuringTheSaveStartsOneTalk() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        // A save that takes its time, so Stop and Play can land inside it.
        let realSave = presentation.saveDeck
        var completions: [(Error?) -> Void] = []
        presentation.saveDeck = { completion in completions.append(completion) }
        presentation.start(PresentationOptions(mode: .rehearse, startSlide: 1))
        XCTAssertEqual(completions.count, 1)
        presentation.stop()
        XCTAssertEqual(presentation.state, .idle, "nothing to quit yet")
        presentation.start(PresentationOptions(mode: .rehearse, startSlide: 2))
        XCTAssertEqual(completions.count, 2)
        completions[0](nil)
        XCTAssertNil(presentation.session, "the old save's completion belongs to a start that was stopped")
        completions[1](nil)
        let session = try XCTUnwrap(presentation.session, "the new save's completion launches")
        XCTAssertEqual(presentation.options?.startSlide, 2)
        presentation.saveDeck = realSave
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }
        XCTAssertTrue(presentation.session === session, "one process, not two")
    }

    func testOneDisplayIsOneSpaceWithThePresenterViewOverIt() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 2))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        let screen = NSScreen.screens[0].frame
        XCTAssertEqual(audience.targetFrame, screen)
        XCTAssertEqual(audience.settledFrame, screen)
        XCTAssertEqual(presenter.fullScreenState, .windowed, "the presenter view never has a Space of its own on one display")
        XCTAssertFalse(presenter.isVisible, "hidden until Option-Tab or the S key")
        XCTAssertFalse(presenter.isAttached)
        XCTAssertFalse(presentation.presenterIsShownOverAudience)
        XCTAssertTrue(audience.isVisible)
        XCTAssertTrue(audience.deckWindowController === controller.editor.window?.windowController as? DeckWindowController)
        XCTAssertTrue(presentation.frontWindow === audience, "on one display the audience is in front")
        try await waitUntil(timeout: 5, "the audience on screen, the presenter not") {
            let order = onScreenWindowNumbers()
            return order.contains(audience.windowNumber) && !order.contains(presenter.windowNumber)
        }

        XCTAssertEqual(audience.page.lastLoadedURL?.fragment, "2")
        XCTAssertEqual(presenter.page.lastLoadedURL?.fragment, "2")
        let client = try XCTUnwrap(presentation.client)
        XCTAssertEqual(audience.page.lastLoadedURL?.query, "launch=\(client.ready.launch)")
        XCTAssertEqual(presenter.page.lastLoadedURL?.query, "key=\(client.ready.presenter)", "the presenter page brings its own key")
        try await waitUntil(timeout: 20, "the audience page on slide 2") { audience.page.lastReady?.slide == 2 }
        try await waitUntil(timeout: 20, "the presenter page ready, though hidden") { presenter.page.lastReady != nil }
        let cookie = await presenterCookieInTheTalkStore()
        XCTAssertNotNil(cookie)
        XCTAssertEqual(cookie, client.presenterCookie, "the audience page holds the hub's presenter cookie too")

        // Option-Tab: the presenter view comes over the audience, in the same Space, no animation.
        presentation.toggleFrontWindow()
        XCTAssertTrue(presentation.presenterIsShownOverAudience)
        XCTAssertTrue(presenter.isAttached)
        XCTAssertTrue(audience.childWindows?.contains(presenter) == true)
        XCTAssertTrue(presenter.isVisible)
        XCTAssertEqual(presenter.frame, audience.frame, "it covers the audience view")
        XCTAssertTrue(presentation.frontWindow === presenter)
        try await waitUntil(timeout: 5, "both on screen, the presenter in front") {
            let order = onScreenWindowNumbers()
            guard let a = order.firstIndex(of: audience.windowNumber), let p = order.firstIndex(of: presenter.windowNumber) else { return false }
            return p < a
        }
        XCTAssertTrue(audience.isVisible, "the audience view is still there underneath")
        presentation.toggleFrontWindow()
        XCTAssertFalse(presentation.presenterIsShownOverAudience)
        XCTAssertFalse(presenter.isVisible)
        XCTAssertTrue(presentation.frontWindow === audience)
        try await waitUntil(timeout: 5, "the presenter off the screen again") { !onScreenWindowNumbers().contains(presenter.windowNumber) }
    }

    func testTheAudienceWindowIsInItsOwnFullScreenSpace() async throws {
        try await requireFullScreen()
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        XCTAssertTrue(presentation.usesFullScreen)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(audience.fullScreenState, .fullScreen, "the audience view has the display's full screen Space")
        XCTAssertTrue(audience.styleMask.contains(.fullScreen))
        XCTAssertEqual(fullScreenPresentationWindows().count, 1, "one Space on one display")
        presentation.toggleFrontWindow()
        XCTAssertEqual(presenter.fullScreenState, .windowed)
        XCTAssertTrue(presenter.collectionBehavior.contains(.fullScreenAuxiliary), "the child rides in the audience's Space")
        try await waitUntil(timeout: 5, "the presenter over the audience in the full screen Space") {
            let order = onScreenWindowNumbers()
            return order.contains(presenter.windowNumber) && order.contains(audience.windowNumber)
        }
        XCTAssertEqual(fullScreenPresentationWindows().count, 1)
    }

    func testTheMacStaysAwake() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertTrue(presentation.sleepAssertion.isHeld)
        XCTAssertTrue(powerAssertionIsListed(named: SleepAssertion.reason), "the kernel holds the display awake for this process")

        // The cursor hides once the pointer has rested on a talk window.
        presentation.cursorHideDelay = 0.1
        var hides = 0
        presentation.hideCursor = { hides += 1 }
        presentation.noteMouseMoved()
        XCTAssertTrue(presentation.isCursorHideArmed)
        try await waitUntil(timeout: 2, "the cursor to hide") { hides == 1 }
        XCTAssertFalse(presentation.isCursorHideArmed)
        presentation.noteMouseMoved()
        presentation.noteMouseMoved()
        try await waitUntil(timeout: 2, "the cursor to hide once more") { hides == 2 }
        // Two moves, one hide: the first move's work item was cancelled, not merely outrun.
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(hides, 2)
        try await stopPresenting(controller)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertFalse(powerAssertionIsListed(named: SleepAssertion.reason))
    }

    /// Starts a talk and closes its deck, letting go of every reference of
    /// its own, so what keeps the talk alive afterwards is the app's doing.
    func startTalkAndCloseTheDeck() async throws -> (talk: WeakTalk, pid: Int32) {
        let (document, controller) = try await openDeckForPresenting()
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let pid = try XCTUnwrap(controller.presentation.session?.processIdentifier)
        let talk = WeakTalk(controller.presentation)
        document.close()
        return (talk, pid)
    }

    func testClosingTheDeckMidTalkStillEndsTheProcessAndFreesPlay() async throws {
        let (talk, pid) = try await startTalkAndCloseTheDeck()
        let presentation = try XCTUnwrap(talk.presentation, "the ending talk is kept alive until its process exits")
        XCTAssertEqual(presentation.state, .stopping)
        XCTAssertEqual(AppEnvironment.shared.endingTalks.count, 1)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertFalse(powerAssertionIsListed(named: SleepAssertion.reason))
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertTrue(AppEnvironment.shared.isPresenting, "still counted while tap present runs")
        try await waitUntil(timeout: 20, "tap present to exit") { !self.isRunning(pid) }
        try await waitUntil(timeout: 10, "the talk to be counted out") { !AppEnvironment.shared.isPresenting }
        try await waitUntil(timeout: 15, "the talk let go of itself once its process was gone and its windows were down") {
            AppEnvironment.shared.endingTalks.isEmpty
        }
        try await waitUntil(timeout: 10, "no talk window left; left: \(Self.describeTalkWindows())") {
            fullScreenPresentationWindows().isEmpty && !NSApp.windows.contains { ($0 as? PresentationWindow).map { !$0.isClosed } ?? false }
        }

        // Another deck can present at once.
        let (_, other) = try await openDeckForPresenting()
        XCTAssertTrue(other.presentation.canStart)
        XCTAssertTrue(AppEnvironment.shared.updatesMayInterrupt)
    }

    /// Every talk window made, held by the test so it can check each one closed.
    @MainActor final class MadeWindows {
        var windows: [PresentationWindow] = []
        var allClosed: Bool { windows.allSatisfy(\.isClosed) }
    }

    /// Starts a talk on one display with AppKit's full screen toggle
    /// replaced by a recorder that never completes, drives the audience's
    /// entry by hand, then closes the deck, letting go of every reference
    /// of its own to the talk.
    func startTalkWithAnExitThatNeverCompletesAndCloseTheDeck(_ made: MadeWindows, stopFirst: Bool = false) async throws -> WeakTalk {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], recordingTo: record)
        let (document, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        presentation.fullScreenAllowed = { true }
        var toggles = 0
        presentation.windowCreated = { window in
            made.windows.append(window)
            window.requestFullScreenToggle = { toggles += 1 }
        }
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 40, "the audience's entry (state \(presentation.state), toggles \(toggles))") { toggles == 1 }
        let audience = try XCTUnwrap(presentation.audienceWindow)
        audience.windowDidEnterFullScreen(Notification(name: NSWindow.didEnterFullScreenNotification))
        XCTAssertEqual(audience.fullScreenState, .fullScreen)
        XCTAssertEqual(made.windows.count, 2, "the audience and the hidden presenter")
        XCTAssertTrue(presentation.windowsAreSettled)
        if stopFirst {
            // tap exits at once on quit while the audience is still leaving full screen.
            presentation.stop()
            try await waitUntil(timeout: 10, "the talk idle (state \(presentation.state))") { presentation.state == .idle }
            XCTAssertFalse(presentation.windowsGoingDown.isEmpty, "idle, with its windows still going down")
        }
        let talk = WeakTalk(presentation)
        document.close()
        return talk
    }

    /// The scripted tap exits at once on quit while the audience's exit
    /// waits out its deadline, on every host. The talk has no process left
    /// and windows still going down: it keeps itself until every window
    /// has closed, and Play in other decks hears when they have.
    func testAClosedDeckTakesDownEveryWindow() async throws {
        let made = MadeWindows()
        var postsWithEveryWindowClosed = 0
        let observer = NotificationCenter.default.addObserver(forName: AppEnvironment.presentingDidChangeNotification, object: nil, queue: nil) { _ in
            MainActor.assumeIsolated {
                if !made.windows.isEmpty, made.allClosed { postsWithEveryWindowClosed += 1 }
            }
        }
        defer { NotificationCenter.default.removeObserver(observer) }
        let talk = try await startTalkWithAnExitThatNeverCompletesAndCloseTheDeck(made)
        XCTAssertNotNil(talk.presentation, "the ending talk is kept alive")
        XCTAssertEqual(AppEnvironment.shared.endingTalks.count, 1)
        XCTAssertFalse(made.allClosed, "the audience is still leaving full screen")
        try await waitUntil(timeout: 15, "every talk window closed; left: \(Self.describeTalkWindows())") {
            if AppEnvironment.shared.endingTalks.isEmpty {
                XCTAssertTrue(made.allClosed, "the talk let go of itself before its windows were down: \(Self.describeTalkWindows())")
                return true
            }
            return made.allClosed
        }
        XCTAssertTrue(made.allClosed)
        try await waitUntil(timeout: 20, "the talk to let go of itself") { AppEnvironment.shared.endingTalks.isEmpty }
        XCTAssertFalse(AppEnvironment.shared.isPresenting)
        XCTAssertNil(talk.presentation, "nothing keeps the talk once it is over")
        XCTAssertGreaterThan(postsWithEveryWindowClosed, 0, "Play in other decks heard that the last window went down")
    }

    /// The same, with the talk already stopped and idle when the deck
    /// closes: its process is gone, but its windows are not.
    func testADeckClosedRightAfterItsTalkStillTakesDownEveryWindow() async throws {
        let made = MadeWindows()
        let talk = try await startTalkWithAnExitThatNeverCompletesAndCloseTheDeck(made, stopFirst: true)
        XCTAssertNotNil(talk.presentation, "the idle talk is kept while its windows go down")
        XCTAssertEqual(AppEnvironment.shared.endingTalks.count, 1)
        try await waitUntil(timeout: 15, "every talk window closed; left: \(Self.describeTalkWindows())") { made.allClosed }
        try await waitUntil(timeout: 5, "the talk to let go of itself") { AppEnvironment.shared.endingTalks.isEmpty }
        XCTAssertNil(talk.presentation)
    }

    func testTheSleepAssertionIsReleasedWhenTapPresentDies() async throws {
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.readyAndWaiting()
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 10, "the assertion at ready") { presentation.sleepAssertion.isHeld }
        // Three deaths in thirty seconds: the app stops restarting (D2's policy).
        for _ in 0..<3 {
            try await waitUntil(timeout: 10, "a running tap present or the end") {
                if case .failed = presentation.state { return true }
                return presentation.session?.processIdentifier != nil
            }
            guard let pid = presentation.session?.processIdentifier else { break }
            kill(pid, SIGKILL)
            // The restart's process has a new identifier; the failure has none.
            try await waitUntil(timeout: 10, "the killed process to be gone") { presentation.session?.processIdentifier != pid }
        }
        try await waitUntil(timeout: 10, "the talk to fail") { if case .failed = presentation.state { return true } else { return false } }
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertFalse(powerAssertionIsListed(named: SleepAssertion.reason))
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertNil(presentation.presenterWindow)
        XCTAssertFalse(AppEnvironment.shared.isPresenting)
        try await waitUntil(timeout: 10, "no window left in full screen") { fullScreenPresentationWindows().isEmpty && presentation.windowsGoingDown.isEmpty }
    }

    func testTheSleepAssertionIsReleasedWhenTheAppQuits() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let pid = try XCTUnwrap(presentation.session?.processIdentifier)
        let delegate = try XCTUnwrap(NSApp.delegate as? AppDelegate)
        delegate.applicationWillTerminate(Notification(name: NSApplication.willTerminateNotification))
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertEqual(presentation.state, .stopping)
        try await waitUntil(timeout: 20, "tap present to exit") { !self.isRunning(pid) }
        try await waitUntil(timeout: 5, "the talk to be idle") { presentation.state == .idle }
        try await waitUntil(timeout: 10, "no window left in full screen") { fullScreenPresentationWindows().isEmpty && presentation.windowsGoingDown.isEmpty }
    }

    func testStopPresenting() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        controller.jumpToSlide(number: 2)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 2))
        XCTAssertEqual(presentation.lastSlide, 2, "the start slide until tap says otherwise")
        // Drive the deck through tap's hub, as the pages do, and hear tap's slide event.
        let client = try XCTUnwrap(presentation.client)
        XCTAssertNotNil(client.presenterCookie)
        let socket = client.openSocket()
        socket.resume()
        try await Task.sleep(nanoseconds: 300_000_000)
        socket.send(SlideMessage(slideIndex: 3, fragment: -1, step: 0))
        try await waitUntil(timeout: 10, "tap's slide event for slide 4") { presentation.lastSlide == 4 }
        socket.close()
        let pid = try XCTUnwrap(presentation.session?.processIdentifier)
        let devPid = controller.session.processIdentifier
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        presentation.toggleFrontWindow()
        XCTAssertTrue(presenter.isAttached)

        presentation.stop()
        XCTAssertEqual(presentation.state, .stopping)
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertNil(presentation.presenterWindow)
        XCTAssertFalse(presenter.isAttached, "the child goes first")
        XCTAssertTrue(presenter.isClosed, "a plain window closes at once")
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        try await waitUntil(timeout: 30, "the talk to end") { presentation.state == .idle }
        XCTAssertEqual(controller.currentSlideNumber, 4, "the cursor is on the last slide presented")
        XCTAssertTrue(presentation.lastTalkLog?.text.contains("tap quit") == true, "the talk's log is kept after the talk")
        try await waitUntil(timeout: 10, "tap present to exit") { !self.isRunning(pid) }
        try await waitUntil(timeout: 10, "the windows closed") { presentation.windowsGoingDown.isEmpty && audience.isClosed }
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty, "nothing is left in full screen")
        XCTAssertEqual(controller.session.processIdentifier, devPid, "tap dev is untouched")
        if case .running = controller.session.state {} else { XCTFail("tap dev keeps running the preview") }
    }

    func testStopLeavesFullScreenBeforeClosing() async throws {
        try await requireFullScreen()
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        var history: [PresentationWindow.FullScreenState] = []
        var closedAfter: [PresentationWindow.FullScreenState] = []
        audience.onFullScreenChange = { history.append($0) }
        audience.onClosed = { closedAfter = history }
        presentation.stop()
        try await waitUntil(timeout: 10, "the audience window closed") { audience.isClosed }
        XCTAssertEqual(closedAfter.suffix(2), [.exiting, .windowed], "it left full screen, then closed")
        try await waitUntil(timeout: 30, "the talk to end") { presentation.state == .idle }
    }

    func testStopWhileTheExchangeIsInFlightOpensNoWindow() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        var gate: CheckedContinuation<Void, Never>?
        // The exchange waits for the test to let it through, then returns a
        // cookie whatever tap does while it quits, so the only thing that
        // can keep the cookie out of the store is the talk having ended.
        // The gate is set inside the continuation's closure, so a resume can never miss it.
        presentation.authorizePresenter = { _ in
            await withCheckedContinuation { continuation in gate = continuation }
            return "late-cookie"
        }
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "the exchange to begin") { gate != nil }
        let pid = try XCTUnwrap(presentation.session?.processIdentifier)
        presentation.stop()
        XCTAssertEqual(presentation.state, .stopping)
        gate?.resume()
        try await waitUntil(timeout: 20, "the talk to be idle") { presentation.state == .idle }
        try await Task.sleep(nanoseconds: 3_500_000_000)
        XCTAssertNil(presentation.audienceWindow, "no window opens after the exchange returns, nor after the fallback")
        XCTAssertNil(presentation.presenterWindow)
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertEqual(presentation.state, .idle)
        XCTAssertFalse(isRunning(pid))
        let cookie = await presenterCookieInTheTalkStore()
        XCTAssertNil(cookie, "a late exchange installs nothing for a talk that has ended")
    }

    func testTheTalkKeepsItsPortAcrossTalks() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deck = try XCTUnwrap(document.fileURL)
        let presentation = controller.presentation
        XCTAssertNil(AppEnvironment.shared.deckPorts.port(for: deck), "nothing remembered before the first talk")
        let suggested = DeckPortStore.suggestedPort(for: deck)
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let first = try XCTUnwrap(presentation.session)
        let port = try XCTUnwrap(presentation.client).ready.port
        if first.command.port == suggested {
            XCTAssertEqual(port, suggested, "the first talk asks for the deck's own port, outside the ephemeral range")
        } else {
            XCTAssertNil(first.command.port, "the suggested port was taken on this machine, so the fallback ran")
            XCTAssertTrue(first.log.text.contains("port \(suggested) was taken"))
        }
        XCTAssertEqual(AppEnvironment.shared.deckPorts.port(for: deck), port, "the port tap runs on is remembered for the deck")
        try await stopPresenting(controller)

        // The next talk asks for the same port, so the presenter page keeps its origin, and with it its layout and notes size.
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let second = try XCTUnwrap(presentation.session)
        XCTAssertEqual(second.command.port, port)
        XCTAssertTrue(second.log.text.contains("--port \(port)"))
        XCTAssertEqual(presentation.client?.ready.port, port)
        XCTAssertEqual(presentation.presenterWindow?.page.lastLoadedURL?.port, port)
    }

    func testATakenPortFallsBackToANewOne() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deck = try XCTUnwrap(document.fileURL)
        let presentation = controller.presentation
        // The deck's own tap dev holds a port; remember that one for the talk.
        let taken = try await waitForRunningTap(document).port
        AppEnvironment.shared.deckPorts.setPort(taken, for: deck)
        // The first attempt's session, taken the moment the save's
        // completion has launched it, before the fallback replaces it.
        let realSave = presentation.saveDeck
        var firstAttempt: TapSession?
        presentation.saveDeck = { completion in
            realSave { error in
                completion(error)
                firstAttempt = presentation.session
            }
        }
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1), timeout: 60)
        let first = try XCTUnwrap(firstAttempt)
        let talk = try XCTUnwrap(presentation.session)
        XCTAssertFalse(first === talk)
        XCTAssertEqual(first.command.port, taken)
        XCTAssertEqual(first.state, .stopped)
        XCTAssertNil(talk.command.port, "the second attempt asks for no port")
        let port = try XCTUnwrap(presentation.client).ready.port
        XCTAssertNotEqual(port, taken)
        XCTAssertEqual(AppEnvironment.shared.deckPorts.port(for: deck), port, "the new port replaces the taken one")
        XCTAssertTrue(talk.log.text.contains("port \(taken) was taken"), "the talk's log says why the layout starts fresh")
        XCTAssertTrue(first.log.text.contains("already in use"), "the first attempt heard tap's port error")
        let launchLine = first.command.logLine(deck: deck)
        XCTAssertEqual(first.log.text.components(separatedBy: launchLine).count - 1, 1,
                       "D2's policy never restarted the failed attempt on the same port: one launch in its log")
        XCTAssertTrue(presentation.sleepAssertion.isHeld)
    }

    func testATakenPortOnARestartFallsBackToo() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deck = try XCTUnwrap(document.fileURL)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let port = try XCTUnwrap(presentation.client).ready.port
        let firstPid = try XCTUnwrap(presentation.session?.processIdentifier)
        // Something else grabs the deck's port the moment tap present dies: a listener of the test's own.
        kill(firstPid, SIGKILL)
        try await waitUntil(timeout: 10, "the process gone") { !self.isRunning(firstPid) }
        // A race the test must win: the squatter binds within D2's first
        // restart delay (0.5 s), and the wait above polls every 20 ms.
        let squatter = try TestListener(port: port)
        defer { squatter.close() }
        try await waitUntil(timeout: 40, "tap present back on another port") {
            if let ready = presentation.client?.ready, ready.port != port, case .running = presentation.session?.state { return true }
            return false
        }
        XCTAssertEqual(presentation.state, .presenting, "the talk never ended")
        XCTAssertNil(presentation.session?.command.port, "the fallback ran mid-talk")
        XCTAssertEqual(AppEnvironment.shared.deckPorts.port(for: deck), presentation.client?.ready.port)
        XCTAssertTrue(presentation.session?.log.text.contains("port \(port) was taken") == true)
    }

    /// A session that has already stopped reports no further change, so
    /// Stop must not wait for one.
    func testStopFinishesAtOnceWhenTapHasAlreadyStopped() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let session = try XCTUnwrap(presentation.session)
        let pid = try XCTUnwrap(session.processIdentifier)
        // The session stops under the talk, the way a relaunch that never
        // came would leave it: the talk is up with no process.
        session.stop()
        try await waitUntil(timeout: 10, "the session stopped") { session.state == .stopped }
        try await waitUntil(timeout: 10, "tap present to exit") { !self.isRunning(pid) }
        XCTAssertEqual(presentation.state, .presenting)
        presentation.stop()
        XCTAssertEqual(presentation.state, .idle, "nothing left to wait for")
        XCTAssertFalse(AppEnvironment.shared.isPresenting, "Play is on again in every deck")
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
    }

    /// The deck file is deleted while the talk starts on a taken port: the
    /// fallback has no file to launch, so the talk fails rather than stay
    /// up with no process.
    func testAPortFallbackWithTheDeckFileGoneFailsTheTalk() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deck = try XCTUnwrap(document.fileURL)
        let presentation = controller.presentation
        let taken = try await waitForRunningTap(document).port
        AppEnvironment.shared.deckPorts.setPort(taken, for: deck)
        let realSave = presentation.saveDeck
        presentation.saveDeck = { completion in
            realSave { error in
                completion(error)
                // tap present is launched on the taken port; the file goes before it answers.
                document.fileWasDeleted()
            }
        }
        presentation.start(PresentationOptions(mode: .rehearse, startSlide: 1))
        try await waitUntil(timeout: 30, "the talk to fail (state \(presentation.state))") {
            if case .failed = presentation.state { return true } else { return false }
        }
        XCTAssertEqual(presentation.state, .failed("The deck file is gone, so the talk cannot restart."))
        XCTAssertNil(presentation.session)
        XCTAssertFalse(AppEnvironment.shared.isPresenting)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
    }

    /// A talk that fails while tap present still runs keeps the session
    /// until its process has exited, so the stop's SIGTERM and SIGKILL
    /// escalation still reaches a tap that ignores its closed stdin.
    func testAFailedTalkStillEndsATapThatIgnoresItsClosedInput() async throws {
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.readyAndDeafToQuit()
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        presentation.authorizePresenter = { _ in "cookie" }
        presentation.screens = { [] }
        var pid: Int32?
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 20, "tap present to be running") {
            pid = pid ?? presentation.session?.processIdentifier
            return pid != nil
        }
        let running = try XCTUnwrap(pid)
        defer { kill(running, SIGKILL) }
        try await waitUntil(timeout: 10, "the talk to fail") { if case .failed = presentation.state { return true } else { return false } }
        XCTAssertEqual(presentation.state, .failed("No display is connected."))
        XCTAssertNil(presentation.session)
        XCTAssertTrue(isRunning(running), "tap ignores its closed stdin and SIGTERM")
        XCTAssertEqual(AppEnvironment.shared.stoppingSessions.count, 1, "the session outlives the talk")
        // Two seconds of grace, SIGTERM, two more, SIGKILL.
        try await waitUntil(timeout: 10, "the escalation to end tap present") { !self.isRunning(running) }
        try await waitUntil(timeout: 5, "the session to be let go") { AppEnvironment.shared.stoppingSessions.isEmpty }
    }

    /// Pages that never report (a server that takes the connection and
    /// never answers) still let the talk start, after the fallback. The
    /// presenter cookie is in the store before any page has loaded, so it
    /// can only be the talk's own install.
    func testTheWindowsShowAfterTheFallbackWhenNoPageReports() async throws {
        let silent = try TestListener()
        defer { silent.close() }
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.readyAndWaiting(port: silent.port)
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        presentation.authorizePresenter = { _ in "installed-cookie" }
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 20, "the talk windows to exist") { presentation.audienceWindow != nil }
        let opened = Date()
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let cookie = await presenterCookieInTheTalkStore()
        XCTAssertEqual(cookie, "installed-cookie", "installed before the pages load")
        XCTAssertEqual(presentation.state, .starting, "no page has reported yet")
        try await waitUntil(timeout: PresentationController.showWindowsFallbackInterval + 5, "the fallback to show the windows") {
            presentation.state == .presenting
        }
        XCTAssertGreaterThanOrEqual(Date().timeIntervalSince(opened), PresentationController.showWindowsFallbackInterval - 1)
        XCTAssertNil(audience.page.lastReady, "no page reported: the fallback showed them")
        XCTAssertTrue(presentation.windowsShown)
    }

    func testTheTalksLogIsListedInTheTapLogWindow() async throws {
        let (_, controller) = try await openDeckForPresenting()
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let logWindow = TapLogWindowController.shared
        logWindow.reload()
        XCTAssertEqual(logWindow.picker.segmentCount, 2)
        XCTAssertEqual(logWindow.picker.label(forSegment: 1), "ops, talk")
        try await stopPresenting(controller)
        logWindow.reload()
        XCTAssertEqual(logWindow.picker.segmentCount, 1)
    }
}

/// A TCP listener on one port of 127.0.0.1, so a test can make a port
/// taken. It never accepts, so a page that connects waits for an answer
/// that never comes.
final class TestListener {
    private let socket: Int32
    /// The port it listens on: the one asked for, or the one the system
    /// picked for port 0.
    private(set) var port = 0

    init(port: Int = 0) throws {
        socket = Darwin.socket(AF_INET, SOCK_STREAM, 0)
        var yes: Int32 = 1
        setsockopt(socket, SOL_SOCKET, SO_REUSEADDR, &yes, socklen_t(MemoryLayout<Int32>.size))
        var address = sockaddr_in()
        address.sin_len = UInt8(MemoryLayout<sockaddr_in>.size)
        address.sin_family = sa_family_t(AF_INET)
        address.sin_port = in_port_t(port).bigEndian
        // 127.0.0.1, the address tap's server binds. A wildcard listener
        // does not take the port from tap on macOS: tap's bind to
        // 127.0.0.1 succeeds beside it. SO_REUSEADDR gets past TIME_WAIT
        // from a killed tap's connections.
        address.sin_addr.s_addr = in_addr_t(INADDR_LOOPBACK).bigEndian
        let bound = withUnsafePointer(to: &address) { pointer in
            pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) { bind(socket, $0, socklen_t(MemoryLayout<sockaddr_in>.size)) }
        }
        guard bound == 0, listen(socket, 16) == 0 else {
            Darwin.close(socket)
            throw CocoaError(.fileWriteUnknown)
        }
        var boundAddress = sockaddr_in()
        var length = socklen_t(MemoryLayout<sockaddr_in>.size)
        _ = withUnsafeMutablePointer(to: &boundAddress) { pointer in
            pointer.withMemoryRebound(to: sockaddr.self, capacity: 1) { getsockname(socket, $0, &length) }
        }
        self.port = Int(in_port_t(bigEndian: boundAddress.sin_port))
    }

    func close() {
        Darwin.close(socket)
    }
}
