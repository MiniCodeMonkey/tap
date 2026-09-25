import AppKit
@testable import Tap

/// The window numbers of this process's windows that the window server
/// has on screen, front to back. This is the window server's own truth,
/// not AppKit's bookkeeping, so it holds in a host with no key window and
/// on the CI runner alike. A window in a full screen Space that is not
/// the active one is not on screen, so a Space switch shows up here.
func onScreenWindowNumbers() -> [Int] {
    let pid = Int(ProcessInfo.processInfo.processIdentifier)
    let windows = CGWindowListCopyWindowInfo([.optionOnScreenOnly, .excludeDesktopElements], kCGNullWindowID) as? [[String: Any]] ?? []
    return windows.compactMap { window in
        guard window[kCGWindowOwnerPID as String] as? Int == pid,
              window[kCGWindowIsOnscreen as String] as? Bool == true else { return nil }
        return window[kCGWindowNumber as String] as? Int
    }
}

/// Runs `pmset -g assertions` and reports whether this process holds an
/// assertion with `name`: the kernel's own view. The line is matched on
/// this process's pid, so the person's own Tap presenting, or their
/// `make uitest`, cannot make the check pass or fail.
func powerAssertionIsListed(named name: String) -> Bool {
    let process = Process()
    process.executableURL = URL(fileURLWithPath: "/usr/bin/pmset")
    process.arguments = ["-g", "assertions"]
    let output = Pipe()
    process.standardOutput = output
    process.standardError = FileHandle.nullDevice
    guard (try? process.run()) != nil else { return false }
    let data = output.fileHandleForReading.readDataToEndOfFile()
    process.waitUntilExit()
    let ownPid = "pid \(ProcessInfo.processInfo.processIdentifier)("
    return String(decoding: data, as: UTF8.self).split(separator: "\n").contains { $0.contains(ownPid) && $0.contains(name) }
}

/// Every talk window that still exists and is in full screen. Empty is
/// what every ending must leave behind, on a host with full screen or without.
@MainActor
func fullScreenPresentationWindows() -> [PresentationWindow] {
    NSApp.windows.compactMap { $0 as? PresentationWindow }.filter { !$0.isClosed && $0.styleMask.contains(.fullScreen) }
}
