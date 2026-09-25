import XCTest
import WebKit
@testable import Tap

/// A weak handle on a talk, for a test that lets go of the deck that
/// owned it and wants to see who keeps it alive.
final class WeakTalk {
    weak var presentation: PresentationController?
    init(_ presentation: PresentationController) { self.presentation = presentation }
}

/// A hosted test that runs a talk with the bundled tap present, on the one
/// screen the machine has. The consent question is answered ahead of time
/// in the test's own settings folder, so tap asks nothing and records
/// nothing; a test that wants the question removes the answer. On a host
/// the probe found unable to enter full screen, the talk windows are
/// plain windows over their frames and the tests that assert full screen
/// skip; everything else runs.
@MainActor
class PresentingTestCase: HostedTestCase {
    /// The probe's verdict for this process (Task 3), never an environment variable.
    private(set) var fullScreenAvailable = false

    override func setUp() async throws {
        try await super.setUp()
        try writeRecordingConsent(false)
        fullScreenAvailable = await FullScreenProbe.run().available
    }

    override func tearDown() async throws {
        for document in NSDocumentController.shared.documents {
            (document as? DeckDocument)?.sessionController?.presentationIfCreated?.stop()
        }
        try await super.tearDown()
        // A talk that is stopping outlives its deck; the next test starts
        // once its process is gone, its count is out and no window is left
        // in full screen.
        try await waitUntil(timeout: 40, "every talk to end (\(AppEnvironment.shared.presentingCount) counted, \(AppEnvironment.shared.endingTalks.count) ending)") {
            !AppEnvironment.shared.isPresenting && AppEnvironment.shared.endingTalks.isEmpty
        }
        try await waitUntil(timeout: 20, "every talk window to go away; left: \(Self.describeTalkWindows())") {
            fullScreenPresentationWindows().isEmpty && !NSApp.windows.contains { ($0 as? PresentationWindow).map { !$0.isClosed } ?? false }
        }
        await waitForFullScreenQuiet()
    }

    /// Every talk window this process still has, for a failure message:
    /// its title, role, full screen state, style mask, and whether it is
    /// closed, visible and on screen.
    static func describeTalkWindows() -> String {
        let onScreen = onScreenWindowNumbers()
        let windows = NSApp.windows.compactMap { $0 as? PresentationWindow }
        guard !windows.isEmpty else { return "none" }
        return windows.map { window in
            "[\"\(window.title)\" \(window.role) \(window.fullScreenState) styleMask \(window.styleMask.rawValue)"
                + (window.styleMask.contains(.fullScreen) ? " (fullScreen)" : "")
                + " closed \(window.isClosed) takingDown \(window.isTakingDown) visible \(window.isVisible) onScreen \(onScreen.contains(window.windowNumber))]"
        }.joined(separator: ", ")
    }

    var settingsFile: URL { configHome.appendingPathComponent("tap/settings.yaml") }

    /// The CLI's own answer to the consent question, at present.record.
    func writeRecordingConsent(_ record: Bool) throws {
        try FileManager.default.createDirectory(at: settingsFile.deletingLastPathComponent(), withIntermediateDirectories: true)
        try "present:\n  record: \(record)\n".write(to: settingsFile, atomically: true, encoding: .utf8)
    }

    func removeRecordingConsent() throws {
        if FileManager.default.fileExists(atPath: settingsFile.path) { try FileManager.default.removeItem(at: settingsFile) }
    }

    /// The real screen, as the only display.
    func oneScreen() -> [ScreenInfo] {
        [ScreenInfo(name: "Only Display", frame: NSScreen.screens[0].frame, isBuiltIn: true)]
    }

    /// The real screen split in two: the left half stands for the laptop,
    /// the right half for the projector. A full screen window fills the
    /// whole display whichever half it was given, so what a test checks
    /// is the frame the controller asked for (`targetFrame`) and settled
    /// on (`settledFrame`), never the window's own frame. Two Spaces on
    /// one screen need `requireSecondSpace()`.
    func halfScreens() -> [ScreenInfo] {
        let frame = NSScreen.screens[0].frame
        let left = CGRect(x: frame.minX, y: frame.minY, width: (frame.width / 2).rounded(.down), height: frame.height)
        let right = CGRect(x: left.maxX, y: frame.minY, width: frame.width - left.width, height: frame.height)
        return [ScreenInfo(name: "Built-in Display", frame: left, isBuiltIn: true),
                ScreenInfo(name: "Projector", frame: right, isBuiltIn: false)]
    }

    /// Opens a copy of the deck, waits for its preview and boxes, points
    /// its talk at the one real screen, and lets the talk use full screen
    /// only where the probe found it works. Two displays on one screen
    /// (`halfScreens()`) ask for two full screen entries on one screen,
    /// and the second is dropped and its window never closes; so a talk
    /// on two displays runs as plain windows over their frames unless the
    /// test has found a second Space (`requireSecondSpace()`). Frames,
    /// attachment and focus are checked the same either way.
    func openDeckForPresenting(_ name: String = "ops.md", slides: Int = 7) async throws -> (DeckDocument, DeckSessionController) {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck(name))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: slides)
        let screens = oneScreen()
        let presentation = controller.presentation
        presentation.screens = { screens }
        let available = fullScreenAvailable
        presentation.fullScreenAllowed = { [weak presentation] in
            available && ((presentation?.screens().count ?? 1) < 2 || FullScreenProbe.secondSpaceWorks)
        }
        return (document, controller)
    }

    /// Starts a talk and waits until it is presenting and every window is
    /// where it was asked to be (in full screen, or a plain window on a
    /// host without it, or after a refused entry, which the talk's log says).
    func startPresenting(_ controller: DeckSessionController, _ options: PresentationOptions, timeout: TimeInterval = 40) async throws {
        controller.presentation.start(options)
        try await waitUntil(timeout: timeout, "the talk to be presenting (state \(controller.presentation.state))") {
            controller.presentation.state == .presenting
        }
        try await waitUntil(timeout: 20, "the talk windows to settle") { controller.presentation.windowsAreSettled }
    }

    func stopPresenting(_ controller: DeckSessionController) async throws {
        controller.presentation.stop()
        try await waitUntil(timeout: 30, "the talk to end") { controller.presentation.state == .idle }
        try await waitUntil(timeout: 10, "the talk windows to close") { controller.presentation.windowsGoingDown.isEmpty }
    }

    /// The presenter cookie in the talk pages' data store, if any. The
    /// store is given 5 s to answer; past that the test fails here rather
    /// than hang the bundle.
    func presenterCookieInTheTalkStore() async -> String? {
        let cookies: [HTTPCookie]? = await withCheckedContinuation { continuation in
            var answered = false
            AppEnvironment.shared.presentationDataStore.httpCookieStore.getAllCookies { cookies in
                guard !answered else { return }
                answered = true
                continuation.resume(returning: cookies)
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + 5) {
                guard !answered else { return }
                answered = true
                continuation.resume(returning: nil)
            }
        }
        guard let cookies else {
            XCTFail("the talk's cookie store did not answer in 5 s")
            return nil
        }
        return cookies.first { $0.name == TapClient.presenterCookieName && $0.domain == "127.0.0.1" }?.value
    }

    func isRunning(_ pid: Int32) -> Bool {
        kill(pid, 0) == 0
    }
}
