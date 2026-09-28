import Foundation

/// What Sparkle's standard interface has on screen, as its user driver
/// reports it: the permission prompt, or an update window (the checking
/// window, an update alert, the progress window, an error or an "up to
/// date" alert). Play waits only while one of these is up; a session that
/// shows nothing (a scheduled fetch, a silent download) never holds a talk
/// back, since the updater's delegate already keeps such a session from
/// showing anything once a talk runs.
public final class SparkleInterface {
    public static let permissionPromptMessage = "Tap is asking whether to check for updates automatically. Answer it, then press Play again."
    public static let updateWindowMessage = "An update window is open. Finish with it or close it, then press Play again."

    public private(set) var permissionPromptIsUp = false
    public private(set) var updateWindowIsUp = false

    public init() {}

    public func permissionPromptShown() { permissionPromptIsUp = true }
    public func permissionPromptAnswered() { permissionPromptIsUp = false }
    public func updateWindowShown() { updateWindowIsUp = true }

    /// Sparkle ends a session that showed anything by dismissing all of it,
    /// the permission prompt included.
    public func sessionFinished() {
        permissionPromptIsUp = false
        updateWindowIsUp = false
    }

    /// Why Play waits, in the words for what is up, or nil when nothing is.
    public var playRefusal: String? {
        if permissionPromptIsUp { return Self.permissionPromptMessage }
        if updateWindowIsUp { return Self.updateWindowMessage }
        return nil
    }
}
