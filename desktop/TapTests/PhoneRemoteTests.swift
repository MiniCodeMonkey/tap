import XCTest
@testable import Tap

final class PhoneRemoteTests: PresentingTestCase {
    func recorded(_ record: URL) -> String {
        (try? String(contentsOf: record, encoding: .utf8)) ?? ""
    }

    func testPhoneRemote() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1, phoneRemote: true))
        try await waitUntil(timeout: 5, "the tunnel command") { self.recorded(record).contains(#"stdin: {"type":"tunnel","start":true}"#) }
        try await waitUntil(timeout: 5, "tap's running tunnel") { presentation.tunnel?.state == "running" }
        XCTAssertEqual(presentation.tunnel?.url, "https://stark-lake-1234.trycloudflare.com")
        let panel = deckWindow.remotePanel
        XCTAssertTrue(panel.isVisible, "the QR code that tap generates is on screen")
        XCTAssertEqual(panel.urlLabel.stringValue, "https://stark-lake-1234.trycloudflare.com")
        XCTAssertEqual(panel.qrImageView.image?.size, NSSize(width: 1, height: 1), "tap's PNG, decoded")
        XCTAssertTrue(panel.noteLabel.stringValue.contains("tap made a presenter password for this talk"))
        XCTAssertTrue(panel.messageLabel.isHidden)
        XCTAssertEqual(panel.level, .floating)
        XCTAssertTrue(panel.styleMask.contains(.nonactivatingPanel), "the panel never takes focus from the talk")
        XCTAssertTrue(panel.collectionBehavior.contains(.fullScreenAuxiliary), "shows over the presenter window's full screen Space")
        try await waitUntil(timeout: 5, "the panel on screen") { onScreenWindowNumbers().contains(panel.windowNumber) }

        panel.turnOffButton.performClick(nil)
        try await waitUntil(timeout: 5, "the tunnel stop") { self.recorded(record).contains(#"stdin: {"type":"tunnel","start":false}"#) }
        try await waitUntil(timeout: 5, "tap's stopped tunnel") { presentation.tunnel?.state == "stopped" }
        XCTAssertFalse(panel.isVisible)

        // Present > Phone Remote turns it back on, and off again while it runs.
        deckWindow.togglePhoneRemote(nil)
        try await waitUntil(timeout: 5, "the tunnel again") { presentation.tunnel?.state == "running" && panel.isVisible }
        deckWindow.togglePhoneRemote(nil)
        XCTAssertFalse(presentation.wantsRemote, "the toggle turns a running remote off")
        try await waitUntil(timeout: 5, "a second tunnel stop") {
            self.recorded(record).components(separatedBy: #"stdin: {"type":"tunnel","start":false}"#).count == 3
        }
        try await waitUntil(timeout: 5, "the panel gone") { !panel.isVisible }
        deckWindow.togglePhoneRemote(nil)
        try await waitUntil(timeout: 5, "the tunnel once more") { presentation.tunnel?.state == "running" && panel.isVisible }
        // Stop forgets the tunnel and says so, so nothing shows a remote of a talk that has ended.
        var tunnelGoneAtEachChange: [Bool] = []
        let refreshRemotePanel = presentation.onTunnelChange
        presentation.onTunnelChange = { [weak presentation] in
            tunnelGoneAtEachChange.append(presentation?.tunnel == nil && presentation?.tunnelError == nil)
            refreshRemotePanel?()
        }
        try await stopPresenting(controller)
        XCTAssertNil(presentation.tunnel)
        XCTAssertNil(presentation.tunnelError)
        XCTAssertEqual(tunnelGoneAtEachChange.last, true, "the stop reported the tunnel gone: \(tunnelGoneAtEachChange)")
        XCTAssertFalse(panel.isVisible, "the panel goes with the talk")
    }

    /// The presenter toolbar's Phone Remote, between Swap Displays and
    /// Stop, toggles the same remote as Present > Phone Remote.
    func testThePresenterToolbarTogglesTheRemote() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        let menuItem = try XCTUnwrap(NSApp.mainMenu?.items.first { $0.submenu?.title == "Present" }?.submenu?.items.first {
            $0.action == #selector(DeckWindowController.togglePhoneRemote(_:))
        })
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let toolbar = try XCTUnwrap(presentation.presenterWindow?.presenterToolbar)
        let button = toolbar.phoneRemoteButton
        XCTAssertEqual(button.title, "Phone Remote")
        let row = try XCTUnwrap(button.superview as? NSStackView)
        let swapIndex = try XCTUnwrap(row.arrangedSubviews.firstIndex { $0 === toolbar.swapButton })
        XCTAssertEqual(row.arrangedSubviews.firstIndex { $0 === button }, swapIndex + 1, "right after Swap Displays")
        XCTAssertEqual(row.arrangedSubviews.firstIndex { $0 === toolbar.stopButton }, swapIndex + 2, "right before Stop")
        XCTAssertTrue(button.isEnabled)
        XCTAssertTrue(deckWindow.validateMenuItem(menuItem), "the same validation as the menu item")
        XCTAssertEqual(button.state, .off)
        let panel = deckWindow.remotePanel
        XCTAssertFalse(panel.isVisible)

        toolbar.pointerReachedBottomEdge()
        button.performClick(nil)
        try await waitUntil(timeout: 5, "the tunnel command") { self.recorded(record).contains(#"stdin: {"type":"tunnel","start":true}"#) }
        try await waitUntil(timeout: 5, "the running tunnel and its panel") { presentation.tunnel?.state == "running" && panel.isVisible }
        XCTAssertEqual(button.state, .on)
        _ = deckWindow.validateMenuItem(menuItem)
        XCTAssertEqual(menuItem.state, .on, "the menu item shows the same remote")

        button.performClick(nil)
        try await waitUntil(timeout: 5, "the tunnel stop") { self.recorded(record).contains(#"stdin: {"type":"tunnel","start":false}"#) }
        try await waitUntil(timeout: 5, "the panel gone") { !panel.isVisible }
        XCTAssertEqual(button.state, .off)

        // Turned on from the menu, the button shows it too, with no click of its own.
        deckWindow.togglePhoneRemote(nil)
        try await waitUntil(timeout: 5, "the tunnel from the menu") { presentation.tunnel?.state == "running" && panel.isVisible }
        XCTAssertEqual(button.state, .on)

        try await stopPresenting(controller)
        XCTAssertFalse(presentation.canTogglePhoneRemote)
        XCTAssertFalse(deckWindow.validateMenuItem(menuItem))
    }

    /// tap's quit stops the tunnel and may report tunnel_failed while the
    /// talk is stopping; the remote panel stays away.
    func testTheRemoteStaysAwayOnceTheTalkStops() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        let folder = try Fixtures.temporaryFolder().appendingPathComponent("run")
        // tap asks keep-recording at quit, which holds the talk in stopping while the test sends its events.
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], quit: .askToKeep(directory: folder, segments: 1), recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        deckWindow.revealInFinder = { _ in }
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1, phoneRemote: true))
        let panel = deckWindow.remotePanel
        try await waitUntil(timeout: 5, "the panel") { panel.isVisible }
        deckWindow.stopPresenting(nil)
        XCTAssertEqual(presentation.state, .stopping)
        XCTAssertFalse(panel.isVisible)
        presentation.handle(.error(TapErrorPayload(code: "tunnel_failed", message: "stopping the tunnel at quit: signal: killed")))
        presentation.handle(.tunnel(TunnelEvent(state: "stopped", url: nil, qr: nil)))
        XCTAssertNil(presentation.tunnelError, "a stopping talk hears no tunnel news")
        XCTAssertFalse(panel.isVisible, "the panel does not come back after Stop")
        try await waitUntil(timeout: 10, "the keep-recording question") { deckWindow.questionSheet?.kind == "keep-recording" }
        try XCTUnwrap(deckWindow.questionSheet?.button(titled: "Delete")).performClick(nil)
        try await waitUntil(timeout: 10, "the talk to end") { presentation.state == .idle }
        XCTAssertFalse(panel.isVisible)
    }

    /// A tap that restarts mid-talk has no tunnel: the dead process's URL
    /// and QR code go, and a remote the person turned on is asked for again.
    func testARestartedTapIsAskedForTheRemoteAgain() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        deckWindow.togglePhoneRemote(nil)
        try await waitUntil(timeout: 5, "the running tunnel") { presentation.tunnel?.state == "running" }
        var tunnelStates: [String?] = []
        let deckWindowHears = presentation.onTunnelChange
        presentation.onTunnelChange = {
            tunnelStates.append(presentation.tunnel?.state)
            deckWindowHears?()
        }
        let pid = try XCTUnwrap(presentation.session?.processIdentifier)
        kill(pid, SIGKILL)
        try await waitUntil(timeout: 20, "tap present back") {
            if let next = presentation.session?.processIdentifier, next != pid, case .running = presentation.session?.state { return true }
            return false
        }
        try await waitUntil(timeout: 5, "the remote asked for again") {
            self.recorded(record).components(separatedBy: #"stdin: {"type":"tunnel","start":true}"#).count == 3
        }
        try await waitUntil(timeout: 5, "the new process's tunnel") { presentation.tunnel?.state == "running" }
        XCTAssertEqual(tunnelStates.first, .some(nil), "the dead process's tunnel went first: \(tunnelStates)")
        XCTAssertTrue(deckWindow.remotePanel.isVisible)
        XCTAssertEqual(presentation.state, .presenting)

        // Turned off from the menu, the remote stays off across the next restart.
        deckWindow.togglePhoneRemote(nil)
        try await waitUntil(timeout: 5, "the tunnel stopped") { presentation.tunnel?.state == "stopped" }
        let second = try XCTUnwrap(presentation.session?.processIdentifier)
        kill(second, SIGKILL)
        try await waitUntil(timeout: 20, "tap present back again") {
            if let next = presentation.session?.processIdentifier, next != second, case .running = presentation.session?.state { return true }
            return false
        }
        try await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertEqual(self.recorded(record).components(separatedBy: #"stdin: {"type":"tunnel","start":true}"#).count, 3, "no remote nobody wants")
        XCTAssertFalse(deckWindow.remotePanel.isVisible)
        presentation.onTunnelChange = deckWindowHears
    }

    func testAdvancedRemoteOptions() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        let deck = try XCTUnwrap(controller.document?.fileURL)
        // A first talk asks for the deck's own port.
        let port = presentation.deckPorts.port(for: deck) ?? DeckPortStore.suggestedPort(for: deck)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1, tunnel: true, presenterPassword: "secret"))
        XCTAssertEqual(presentation.session?.command, .present(record: true, presenterPassword: "secret", port: port))
        try await waitUntil(timeout: 5, "the arguments") { self.recorded(record).contains("--presenter-password secret") }
        XCTAssertFalse(recorded(record).contains("--tunnel"), "tap present has no --tunnel flag; the tunnel is a command")
        try await waitUntil(timeout: 5, "the tunnel command") { self.recorded(record).contains(#"stdin: {"type":"tunnel","start":true}"#) }
        try await waitUntil(timeout: 5, "the running tunnel") { presentation.tunnel?.state == "running" }
        XCTAssertTrue(deckWindow.remotePanel.noteLabel.stringValue.contains("your presenter password"), "the person's own password, not a generated one")
        try await stopPresenting(controller)
    }

    func testWithoutCloudflaredThePanelSaysSo() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], tunnelUnavailable: true, recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1, phoneRemote: true))
        try await waitUntil(timeout: 5, "tap's error") { presentation.tunnelError != nil }
        let panel = deckWindow.remotePanel
        XCTAssertTrue(panel.isVisible)
        XCTAssertFalse(panel.messageLabel.isHidden)
        XCTAssertTrue(panel.messageLabel.stringValue.contains("brew install cloudflared"))
        XCTAssertNil(panel.qrImageView.image)
        try await stopPresenting(controller)
    }

    func testAFailedTunnelStartKeepsItsReason() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], tunnelFailed: true, recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1, phoneRemote: true))
        // tap sends tunnel_failed and then a stopped tunnel event; the reason must survive the second.
        try await waitUntil(timeout: 5, "tap's stopped tunnel") { presentation.tunnel?.state == "stopped" }
        XCTAssertEqual(presentation.tunnelError, "cloudflared exited: connection refused")
        let panel = deckWindow.remotePanel
        XCTAssertTrue(panel.isVisible, "the panel stays with the reason")
        XCTAssertFalse(panel.messageLabel.isHidden)
        XCTAssertTrue(panel.messageLabel.stringValue.contains("connection refused"))
        // Trying again sends a second start; the fake answers starting, failed and stopped in one write, so the
        // observable proof is the second command in the record file and the reason still standing afterwards.
        deckWindow.togglePhoneRemote(nil)
        try await waitUntil(timeout: 5, "a second tunnel start") {
            (try? String(contentsOf: record, encoding: .utf8))?.components(separatedBy: #"stdin: {"type":"tunnel","start":true}"#).count == 3
        }
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(presentation.tunnelError, "cloudflared exited: connection refused", "the second failure's reason stands")
        XCTAssertTrue(panel.isVisible)
        try await stopPresenting(controller)
        XCTAssertFalse(panel.isVisible)
    }

    func testThePanelClosesWithTheDeckWindow() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], recordingTo: record)
        let (document, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1, phoneRemote: true))
        let panel = deckWindow.remotePanel
        try await waitUntil(timeout: 5, "the panel") { panel.isVisible }
        let closing = expectation(forNotification: NSWindow.willCloseNotification, object: panel)
        document.close()
        await fulfillment(of: [closing], timeout: 1)
        XCTAssertFalse(panel.isVisible, "no panel outlives its deck window")
        try await waitUntil(timeout: 5, "the panel off screen") { !onScreenWindowNumbers().contains(panel.windowNumber) }
    }
}
