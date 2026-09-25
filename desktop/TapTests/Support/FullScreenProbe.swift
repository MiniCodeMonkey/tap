import AppKit
import XCTest

/// Whether this host can put a window into system full screen, found out
/// once per process by trying, the way the spike did: a probe window
/// enters and leaves. Whether a second window on the same screen gets a
/// Space of its own while the first is in is a separate probe, run only
/// when a test asks for it (`requireSecondSpace()`), since the second
/// window's stuck transition would otherwise run before every single
/// Space test. The results are observed, never configured: nothing in
/// the environment can claim a capability the host does not have.
@MainActor
enum FullScreenProbe {
    struct Result {
        /// One window can enter and leave system full screen.
        let available: Bool
        let reason: String
    }

    struct SecondSpaceResult {
        /// A second window on the same screen gets its own Space while the first is in full screen.
        let works: Bool
        let reason: String
    }

    private static var cached: Result?
    private static var cachedSecondSpace: SecondSpaceResult?

    private static func probeWindow() -> NSWindow {
        let window = NSWindow(contentRect: NSScreen.screens[0].frame, styleMask: [.titled, .fullSizeContentView], backing: .buffered, defer: false)
        window.collectionBehavior = [.fullScreenPrimary, .fullScreenDisallowsTiling]
        window.isReleasedWhenClosed = false
        window.backgroundColor = .black
        return window
    }

    private static func wait(_ seconds: TimeInterval, until condition: @escaping @MainActor () -> Bool) async -> Bool {
        let deadline = Date().addingTimeInterval(seconds)
        while Date() < deadline {
            if condition() { return true }
            try? await Task.sleep(nanoseconds: 50_000_000)
        }
        return condition()
    }

    /// A probe window, with flags for its own did-enter and did-exit
    /// notifications. `leave` stops the observing.
    @MainActor
    private final class Probed {
        let window = FullScreenProbe.probeWindow()
        var entered = false
        var exited = false
        private var observers: [NSObjectProtocol] = []

        init() {
            observers.append(NotificationCenter.default.addObserver(forName: NSWindow.didEnterFullScreenNotification, object: window, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated { self?.entered = true }
            })
            observers.append(NotificationCenter.default.addObserver(forName: NSWindow.didExitFullScreenNotification, object: window, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated { self?.exited = true }
            })
        }

        func stopObserving() {
            observers.forEach { NotificationCenter.default.removeObserver($0) }
            observers = []
        }
    }

    /// Asks `probed` to enter full screen and waits up to 8 s. The style
    /// mask is trustworthy for a first window: on both hosts the spike saw
    /// it flip in step with the window's own did-enter notification, so
    /// either is a fine way to notice it.
    private static func enter(_ probed: Probed) async -> Bool {
        probed.window.orderFrontRegardless()
        probed.window.toggleFullScreen(nil)
        return await wait(8) { probed.entered || probed.window.styleMask.contains(.fullScreen) }
    }

    /// Takes `probed` out of full screen and closes it. The wait is for
    /// its did-exit notification, not the style mask: the style mask
    /// clears at will-exit, while the exit animation still runs, and an
    /// entry asked for during that animation is dropped by AppKit with no
    /// notification at all. A window that never had a did-enter (a second
    /// window's stuck transition) will not get a did-exit either, so there
    /// the style mask is all there is. The spike saw an exit take up to
    /// just over six seconds, so the wait is generous.
    private static func leave(_ probed: Probed) async {
        if probed.window.styleMask.contains(.fullScreen) {
            probed.window.toggleFullScreen(nil)
            _ = await wait(15) { probed.exited || (!probed.entered && !probed.window.styleMask.contains(.fullScreen)) }
        }
        probed.window.close()
        probed.stopObserving()
    }

    static func run() async -> Result {
        if let cached { return cached }
        let a = Probed()
        let entered = await enter(a)
        await leave(a)
        await waitForFullScreenQuiet()
        let result = Result(available: entered,
                            reason: entered
                                ? "a window enters and leaves system full screen"
                                : "a window asked for system full screen never entered it within 8 s (active app: \(NSApp.isActive))")
        cached = result
        return result
    }

    /// Whether a second window on the same screen gets its own Space
    /// while a first is in full screen. The spike found a host (both this
    /// machine and the CI runner) where the second window's `styleMask`
    /// reports `.fullScreen` within a fraction of a second, yet it never
    /// receives a single full screen notification and is not on screen
    /// once the first window is made key again. The style mask there is
    /// not evidence of a real transition, so this is decided from the
    /// second window's own `didEnterFullScreenNotification`.
    static func runSecondSpace() async -> SecondSpaceResult {
        if let cachedSecondSpace { return cachedSecondSpace }
        let a = Probed()
        var works = false
        if await enter(a) {
            let b = Probed()
            b.window.orderFrontRegardless()
            b.window.toggleFullScreen(nil)
            works = await wait(8) { b.entered }
            await leave(b)
        }
        await leave(a)
        await waitForFullScreenQuiet()
        let result = SecondSpaceResult(works: works,
                                       reason: works
                                           ? "a second window on the same screen gets its own full screen Space"
                                           : "a second window on the same screen never completes its own transition (its style mask flips but no full screen notification follows, and it is not on screen once the first window is key again)")
        cachedSecondSpace = result
        return result
    }
}

/// Waits until no window of this process, of any class, is in full screen
/// and the system presentation options no longer say full screen, then a
/// further second for the Space animation to finish. An entry asked for
/// while a previous Space is still being torn down is dropped by AppKit
/// with no notification, so every full screen test starts, and every
/// test that used full screen ends, here. It never fails a test: a host
/// that stays busy past `timeout` gets its quiet period anyway, and the
/// line it prints says so.
@MainActor
func waitForFullScreenQuiet(timeout: TimeInterval = 20) async {
    let deadline = Date().addingTimeInterval(timeout)
    func busy() -> Bool {
        NSApp.windows.contains { $0.styleMask.contains(.fullScreen) } || NSApp.currentSystemPresentationOptions.contains(.fullScreen)
    }
    while busy(), Date() < deadline {
        try? await Task.sleep(nanoseconds: 50_000_000)
    }
    if busy() {
        print("waitForFullScreenQuiet: still in full screen after \(Int(timeout)) s; going on after the quiet period")
    }
    try? await Task.sleep(nanoseconds: 1_000_000_000)
}

extension XCTestCase {
    /// Skips the test unless this host can enter system full screen, and
    /// otherwise waits for the host to be quiet, so the test's own entry
    /// is not dropped for a transition still running from the test before.
    /// Call it before any assertion on `fullScreenState == .fullScreen`,
    /// `styleMask.contains(.fullScreen)` or which Space is active.
    @MainActor
    func requireFullScreen() async throws {
        let result = await FullScreenProbe.run()
        if !result.available {
            throw XCTSkip("this host cannot enter system full screen (\(result.reason)); the state machine is covered by the seam tests, real full screen by the person's UI tests")
        }
        await waitForFullScreenQuiet()
    }

    /// Skips the test unless a second window on the same screen can have a
    /// Space of its own: what the two-display tests on one screen need.
    @MainActor
    func requireSecondSpace() async throws {
        try await requireFullScreen()
        let result = await FullScreenProbe.runSecondSpace()
        if !result.works {
            throw XCTSkip("this host gives one full screen Space per screen (\(result.reason)); two displays on one screen cannot be stood in for here, and a real second display is the person's manual pass")
        }
        await waitForFullScreenQuiet()
    }
}
