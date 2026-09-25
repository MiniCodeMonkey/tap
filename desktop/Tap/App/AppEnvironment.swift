import AppKit

/// What every deck shares: the bundled tap, the login shell environment
/// read once at launch, and the bundled tap's version.
@MainActor
final class AppEnvironment {
    static let shared = AppEnvironment()
    static let didLoadNotification = Notification.Name("TapAppEnvironmentDidLoad")

    /// The tap every session runs: the one inside the bundle, unless the
    /// `TapExecutablePath` default names another. Every test runs the
    /// bundled tap.
    var tapExecutableURL: URL
    /// Variables added on top of the login shell environment.
    var extraEnvironment: [String: String] = [:]
    /// Where a deck's slide-1 thumbnail is saved for the welcome window. A
    /// test replaces this with a store rooted in its own temporary folder.
    var recentThumbnailStore = RecentThumbnailStore()
    /// Whether each deck's slide panel is pinned as a sidebar or peeks on
    /// hover. A test replaces this with one on a fresh UserDefaults suite.
    var panelState = SlidePanelState()
    /// Where slide thumbnails are cached on disk. A test replaces this with
    /// a cache rooted in its own temporary folder.
    var thumbnailCache = ThumbnailCache()
    /// Every layout tap offers, loaded once from the bundled tap.
    lazy var layoutCatalog = LayoutCatalogLoader(executable: { [weak self] in self?.tapExecutableURL ?? URL(fileURLWithPath: "/usr/bin/false") })
    /// The layout New Slide inserts: the one used last.
    var lastLayout = LastLayout()
    /// Where copied slides go and paste reads from: the general pasteboard,
    /// unless a test replaces it with a named one so a run never touches
    /// the person's real clipboard.
    var slidePasteboard: NSPasteboard = .general
    private(set) var environmentNotice: String?
    private(set) var bundledTapVersion: String?
    private let loginShellLoader: LoginShellEnvironmentLoader

    init() {
        if let override = UserDefaults.standard.string(forKey: "TapExecutablePath") {
            tapExecutableURL = URL(fileURLWithPath: override)
        } else {
            tapExecutableURL = Bundle.main.url(forResource: "tap", withExtension: nil) ?? URL(fileURLWithPath: "/usr/bin/false")
        }
        loginShellLoader = LoginShellEnvironmentLoader(shellPath: ProcessInfo.processInfo.environment["SHELL"] ?? "/bin/zsh")
    }

    /// Starts reading the login shell environment and the tap version.
    func warmUp() {
        Task { @MainActor in
            let environment = await loginShellLoader.environment()
            environmentNotice = environment.notice
            bundledTapVersion = await Self.readVersion(of: tapExecutableURL)
            NotificationCenter.default.post(name: Self.didLoadNotification, object: self)
        }
        Task { await layoutCatalog.load() }
    }

    func tapEnvironment() async -> [String: String] {
        var variables = await loginShellLoader.environment().variables
        variables.merge(extraEnvironment) { _, extra in extra }
        return variables
    }

    /// The environment closure falls back to the app's own process
    /// environment if this object is gone, which is what tap would inherit
    /// from the app anyway.
    func sessionConfiguration() -> TapSession.Configuration {
        TapSession.Configuration(executableURL: tapExecutableURL, environment: { [weak self] in
            await self?.tapEnvironment() ?? ProcessInfo.processInfo.environment
        })
    }

    /// Runs `tap --version` and returns the version from "tap version <version>".
    nonisolated static func readVersion(of executable: URL) async -> String? {
        await withCheckedContinuation { continuation in
            DispatchQueue.global().async {
                let process = Process()
                process.executableURL = executable
                process.arguments = ["--version"]
                let output = Pipe()
                process.standardOutput = output
                process.standardError = FileHandle.nullDevice
                guard (try? process.run()) != nil else { return continuation.resume(returning: nil) }
                let data = output.fileHandleForReading.readDataToEndOfFile()
                process.waitUntilExit()
                let text = String(decoding: data, as: UTF8.self).trimmingCharacters(in: .whitespacesAndNewlines)
                let prefix = "tap version "
                continuation.resume(returning: text.hasPrefix(prefix) ? String(text.dropFirst(prefix.count)) : nil)
            }
        }
    }
}
