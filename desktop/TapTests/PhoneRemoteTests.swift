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
        XCTAssertTrue(panel.collectionBehavior.contains(.fullScreenAuxiliary), "shows over the presenter window's full screen Space")
        try await waitUntil(timeout: 5, "the panel on screen") { onScreenWindowNumbers().contains(panel.windowNumber) }

        panel.turnOffButton.performClick(nil)
        try await waitUntil(timeout: 5, "the tunnel stop") { self.recorded(record).contains(#"stdin: {"type":"tunnel","start":false}"#) }
        try await waitUntil(timeout: 5, "tap's stopped tunnel") { presentation.tunnel?.state == "stopped" }
        XCTAssertFalse(panel.isVisible)

        // Present > Phone Remote turns it back on.
        deckWindow.togglePhoneRemote(nil)
        try await waitUntil(timeout: 5, "the tunnel again") { presentation.tunnel?.state == "running" && panel.isVisible }
        try await stopPresenting(controller)
        XCTAssertFalse(panel.isVisible, "the panel goes with the talk")
    }

    func testAdvancedRemoteOptions() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1, tunnel: true, presenterPassword: "secret"))
        XCTAssertEqual(presentation.session?.command, .present(record: true, presenterPassword: "secret", port: nil))
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
        document.close()
        XCTAssertFalse(panel.isVisible, "no panel outlives its deck window")
        try await waitUntil(timeout: 5, "the panel off screen") { !onScreenWindowNumbers().contains(panel.windowNumber) }
    }
}
