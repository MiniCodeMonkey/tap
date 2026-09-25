import XCTest
@testable import Tap

/// What this host does when a plain window asks for system full screen.
/// The test asserts nothing about full screen: it records what happened,
/// one line per event, so a local run and a CI run can be compared in
/// the ledger. (a) is one window entering and leaving; (b) is a second
/// window on the same screen entering while the first is in full screen,
/// which the two-display tests on one screen rely on. Every wait here is
/// this file's own polling loop, never `HostedTestCase.waitUntil`: that
/// helper calls `XCTFail` on a timeout, which would turn a slow or stuck
/// host into a red run. A timeout here is itself a recorded event, never
/// a failure, because CI runs this test on every pull request.
/// The window numbers the window server has on screen for this process,
/// front to back. Local to this file because `Support/WindowServer.swift`
/// (the shared helper of the same name) does not exist until Step 1.
private func onScreenWindowNumbers() -> [Int] {
    let pid = Int(ProcessInfo.processInfo.processIdentifier)
    let windows = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
    return windows.compactMap { window in
        guard window[kCGWindowOwnerPID as String] as? Int == pid,
              window[kCGWindowIsOnscreen as String] as? Bool == true else { return nil }
        return window[kCGWindowNumber as String] as? Int
    }
}

final class FullScreenSpikeTests: HostedTestCase {
    func spikeWindow(_ title: String) -> NSWindow {
        let window = NSWindow(contentRect: NSScreen.screens[0].frame, styleMask: [.titled, .fullSizeContentView], backing: .buffered, defer: false)
        window.title = title
        window.collectionBehavior = [.fullScreenPrimary, .fullScreenDisallowsTiling]
        window.isReleasedWhenClosed = false
        window.backgroundColor = .black
        return window
    }

    func testFullScreenSpike() async throws {
        var events: [String] = []
        let names: [Notification.Name] = [NSWindow.willEnterFullScreenNotification, NSWindow.didEnterFullScreenNotification,
                                          NSWindow.willExitFullScreenNotification, NSWindow.didExitFullScreenNotification]
        let a = spikeWindow("spike A")
        let b = spikeWindow("spike B")
        var observers: [NSObjectProtocol] = []
        for window in [a, b] {
            for name in names {
                observers.append(NotificationCenter.default.addObserver(forName: name, object: window, queue: .main) { _ in
                    events.append("\(window.title): \(name.rawValue)")
                })
            }
        }
        defer { observers.forEach { NotificationCenter.default.removeObserver($0) } }
        let started = Date()
        func note(_ line: String) { events.append(String(format: "%5.1fs ", Date().timeIntervalSince(started)) + "active: \(NSApp.isActive), " + line) }

        /// Polls `condition` until it is true, or records a timeout and
        /// returns false. Never fails the test: a slow or stuck host is
        /// exactly what this spike exists to find out.
        func wait(_ seconds: TimeInterval, _ what: String, _ condition: () -> Bool) async -> Bool {
            let deadline = Date().addingTimeInterval(seconds)
            while Date() < deadline {
                if condition() { return true }
                try? await Task.sleep(nanoseconds: 50_000_000)
            }
            if condition() { return true }
            note("timed out waiting for \(what) after \(Int(seconds)) s")
            return false
        }

        note("screens: \(NSScreen.screens.count), separate Spaces: \(NSScreen.screensHaveSeparateSpaces)")
        a.orderFrontRegardless()
        a.toggleFullScreen(nil)
        _ = await wait(8, "A in full screen") { a.styleMask.contains(.fullScreen) }
        note("A styleMask.fullScreen: \(a.styleMask.contains(.fullScreen)), on screen: \(onScreenWindowNumbers().contains(a.windowNumber))")

        b.orderFrontRegardless()
        b.toggleFullScreen(nil)
        _ = await wait(8, "B in full screen") { b.styleMask.contains(.fullScreen) }
        note("B styleMask.fullScreen: \(b.styleMask.contains(.fullScreen)), on screen: \(onScreenWindowNumbers().contains(b.windowNumber)), A on screen: \(onScreenWindowNumbers().contains(a.windowNumber))")

        // However long the DidEnter for B takes, this is where it can still show up in the record.
        _ = await wait(8, "B's did-enter-full-screen notification") { events.contains("spike B: \(NSWindow.didEnterFullScreenNotification.rawValue)") }
        note("B did-enter notification seen: \(events.contains("spike B: \(NSWindow.didEnterFullScreenNotification.rawValue)"))")

        a.makeKeyAndOrderFront(nil)
        _ = await wait(4, "A's Space active") { onScreenWindowNumbers().contains(a.windowNumber) }
        note("after makeKey A: A on screen: \(onScreenWindowNumbers().contains(a.windowNumber)), B on screen: \(onScreenWindowNumbers().contains(b.windowNumber))")

        for window in [b, a] where window.styleMask.contains(.fullScreen) {
            window.toggleFullScreen(nil)
            // A gets the longer allowance: an earlier run left it stuck
            // exiting well past 6 s, and the point of this spike is to find
            // out whether that is slow or never happens.
            let timeout: TimeInterval = window === a ? 15 : 6
            let left = await wait(timeout, "\(window.title) out of full screen") { !window.styleMask.contains(.fullScreen) }
            note("\(window.title) left full screen: \(left)")
            if window === a {
                let sawDidExit = events.contains("spike A: \(NSWindow.didExitFullScreenNotification.rawValue)")
                note("A did-exit notification seen: \(sawDidExit)")
            }
        }
        note("B did-enter: \(events.contains("spike B: \(NSWindow.didEnterFullScreenNotification.rawValue)")), B will-exit: \(events.contains("spike B: \(NSWindow.willExitFullScreenNotification.rawValue)")), B did-exit: \(events.contains("spike B: \(NSWindow.didExitFullScreenNotification.rawValue)"))")
        a.close()
        b.close()
        _ = await wait(4, "the spike windows gone") { !onScreenWindowNumbers().contains(a.windowNumber) && !onScreenWindowNumbers().contains(b.windowNumber) }
        note("closed; any spike window still on screen: \(onScreenWindowNumbers().contains(a.windowNumber) || onScreenWindowNumbers().contains(b.windowNumber))")

        // Never a failure: the record is the result.
        let report = events.joined(separator: "\n")
        print("FullScreenSpike:\n\(report)")
        XCTContext.runActivity(named: "FullScreenSpike") { activity in
            let attachment = XCTAttachment(string: report)
            attachment.lifetime = .keepAlways
            activity.add(attachment)
        }
    }
}
