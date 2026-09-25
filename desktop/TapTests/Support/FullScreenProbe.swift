import AppKit
import XCTest

/// Whether this host can put a window into system full screen, found out
/// once per process by trying, the way the spike did: a probe window
/// enters and leaves, then a second one enters on the same screen while
/// the first is in. The result is observed, never configured: nothing in
/// the environment can claim a capability the host does not have.
///
/// The spike found a host (both this machine and the CI runner) where a
/// second window's `styleMask` reports `.fullScreen` within a fraction of
/// a second of being asked, yet the window never receives a single full
/// screen notification (no will-enter, did-enter, will-exit or did-exit)
/// and is not on screen once the first window is made key again. The
/// style mask there is not evidence of a real transition, so `secondSpace`
/// is decided from the second window's own `didEnterFullScreenNotification`,
/// never from its style mask.
@MainActor
enum FullScreenProbe {
    struct Result {
        /// One window can enter and leave system full screen.
        let available: Bool
        /// A second window on the same screen gets its own Space while the first is in full screen.
        let secondSpace: Bool
        let reason: String
    }

    private static var cached: Result?

    static func run() async -> Result {
        if let cached { return cached }
        func probeWindow() -> NSWindow {
            let window = NSWindow(contentRect: NSScreen.screens[0].frame, styleMask: [.titled, .fullSizeContentView], backing: .buffered, defer: false)
            window.collectionBehavior = [.fullScreenPrimary, .fullScreenDisallowsTiling]
            window.isReleasedWhenClosed = false
            window.backgroundColor = .black
            return window
        }
        func wait(_ seconds: TimeInterval, until condition: @escaping @MainActor () -> Bool) async -> Bool {
            let deadline = Date().addingTimeInterval(seconds)
            while Date() < deadline {
                if condition() { return true }
                try? await Task.sleep(nanoseconds: 50_000_000)
            }
            return condition()
        }

        let a = probeWindow()
        var aEntered = false
        let aObserver = NotificationCenter.default.addObserver(forName: NSWindow.didEnterFullScreenNotification, object: a, queue: .main) { _ in aEntered = true }
        a.orderFrontRegardless()
        a.toggleFullScreen(nil)
        // The style mask is trustworthy for the first window: on both
        // hosts the spike saw it flip in step with a's own did-enter
        // notification, so either is a fine way to notice it, and waiting
        // on the notification alone would miss a host whose notification
        // never fires despite a real transition.
        let entered = await wait(8) { aEntered || a.styleMask.contains(.fullScreen) }
        var second = false
        if entered {
            let b = probeWindow()
            var bEntered = false
            let bObserver = NotificationCenter.default.addObserver(forName: NSWindow.didEnterFullScreenNotification, object: b, queue: .main) { _ in bEntered = true }
            b.orderFrontRegardless()
            b.toggleFullScreen(nil)
            second = await wait(8) { bEntered }
            NotificationCenter.default.removeObserver(bObserver)
            if b.styleMask.contains(.fullScreen) {
                b.toggleFullScreen(nil)
                _ = await wait(6) { !b.styleMask.contains(.fullScreen) }
            }
            b.close()
            a.toggleFullScreen(nil)
            // The spike saw a's own exit take anywhere from well under a
            // second to just over six, so this cleanup wait is generous:
            // leaving the probe's own window stuck in full screen would
            // outlive the probe and confuse whatever runs after it.
            _ = await wait(15) { !a.styleMask.contains(.fullScreen) }
        }
        NotificationCenter.default.removeObserver(aObserver)
        a.close()
        let result = Result(available: entered, secondSpace: second,
                            reason: entered
                                ? (second ? "full screen and a second Space work" : "one window enters full screen; a second window on the same screen never completes its own transition (its style mask flips but no full screen notification follows, and it is not on screen once the first window is key again)")
                                : "a window asked for system full screen never entered it within 8 s (active app: \(NSApp.isActive))")
        cached = result
        return result
    }
}

extension XCTestCase {
    /// Skips the test unless this host can enter system full screen. Call
    /// it before any assertion on `fullScreenState == .fullScreen`,
    /// `styleMask.contains(.fullScreen)` or which Space is active.
    @MainActor
    func requireFullScreen() async throws {
        let result = await FullScreenProbe.run()
        if !result.available {
            throw XCTSkip("this host cannot enter system full screen (\(result.reason)); the state machine is covered by the seam tests, real full screen by the person's UI tests")
        }
    }

    /// Skips the test unless a second window on the same screen can have a
    /// Space of its own: what the two-display tests on one screen need.
    @MainActor
    func requireSecondSpace() async throws {
        try await requireFullScreen()
        let result = await FullScreenProbe.run()
        if !result.secondSpace {
            throw XCTSkip("this host gives one full screen Space per screen (\(result.reason)); two displays on one screen cannot be stood in for here, and a real second display is the person's manual pass")
        }
    }
}
