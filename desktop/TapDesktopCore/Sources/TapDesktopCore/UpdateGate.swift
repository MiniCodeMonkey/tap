import Foundation

/// The rule between Sparkle and a talk: while a talk runs, no check starts,
/// no found update is shown, and a relaunch waits for the talk's windows to
/// come down. The app's updater asks this object at each of Sparkle's
/// decision points with `AppEnvironment.updatesMayInterrupt`, and tells it
/// when the last talk is down.
public final class UpdateGate {
    public static let presentingMessage = "Tap does not check for updates during a talk."
    public static let updateInProgressMessage = "Sparkle is checking for or installing an update. Let it finish, or close its window, then press Play again."

    /// What Sparkle's delegate throws to refuse a check or a found update
    /// while a talk runs. Sparkle shows the message in its own alert for a
    /// check the person asked for and logs it for a scheduled one.
    public struct PresentingError: LocalizedError {
        public init() {}
        public var errorDescription: String? { UpdateGate.presentingMessage }
    }

    /// The relaunch Sparkle was told to wait with, until the talk is down.
    public private(set) var postponedRelaunch: (() -> Void)?

    public init() {}

    /// Whether a check may start now.
    public func mayCheck(mayInterrupt: Bool) -> Bool {
        mayInterrupt
    }

    /// Whether an update a check found may be shown now.
    public func mayProceed(mayInterrupt: Bool) -> Bool {
        mayInterrupt
    }

    /// Returns true when the relaunch is postponed, keeping `resume` for
    /// `resumePostponedRelaunch`; false lets Sparkle relaunch now. Sparkle
    /// asks again for the same update, so a later call replaces the block.
    public func shouldPostponeRelaunch(mayInterrupt: Bool, resume: @escaping () -> Void) -> Bool {
        guard !mayInterrupt else { return false }
        postponedRelaunch = resume
        return true
    }

    /// Runs a postponed relaunch, once.
    public func resumePostponedRelaunch() {
        let resume = postponedRelaunch
        postponedRelaunch = nil
        resume?()
    }

    /// Whether the updater starts at all: only in a build with a release
    /// version and never in a process that hosts tests or was launched by a
    /// UI test. A 0.0.0 build is a Debug build or a local dry run, which
    /// must never offer to replace itself from the feed.
    public static func updaterMayStart(bundleVersion: String?, isHostedByTests: Bool, hasTestDefaultsSuite: Bool) -> Bool {
        guard !isHostedByTests, !hasTestDefaultsSuite, let bundleVersion else { return false }
        return !bundleVersion.hasPrefix("0.0.0")
    }
}
