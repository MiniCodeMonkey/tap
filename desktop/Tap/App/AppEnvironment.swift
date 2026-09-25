import AppKit
import WebKit

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
    /// The data store every talk page uses: persistent, so the presenter
    /// layout and notes size (the page's localStorage) survive the process.
    /// A test replaces it with a store of its own, so a run never touches
    /// the person's.
    var presentationDataStore: WKWebsiteDataStore = .default()
    /// Which display is the audience for each pair of displays, across
    /// decks. A test replaces this with a store on a fresh UserDefaults suite.
    var displayAssignments = DisplayAssignmentStore()
    /// The port each deck's talks run on, so the talk pages keep one origin.
    var deckPorts = DeckPortStore()
    /// The Present popover's last settings, which Cmd+Option+P starts with.
    var presentationSettings = PresentationSettingsStore()
    /// A tap for talks alone, for tests that script tap present while the
    /// deck's real tap dev keeps running. nil runs the bundled tap.
    var presentExecutableURL: URL?
    /// How many talks are running across every deck, from Play to idle or
    /// failed. Play is off while one runs, and D7's updater reads
    /// `updatesMayInterrupt` before any prompt or restart.
    private(set) var presentingCount = 0
    static let presentingDidChangeNotification = Notification.Name("TapPresentingDidChange")
    /// Talks whose deck closed while they were still ending, kept alive
    /// until their process has exited (or the talk has failed) and their
    /// windows are down.
    private(set) var endingTalks: [PresentationController] = []

    var isPresenting: Bool { presentingCount > 0 }
    var updatesMayInterrupt: Bool { !isPresenting }

    func noteTalkStarted() {
        presentingCount += 1
        NotificationCenter.default.post(name: Self.presentingDidChangeNotification, object: self)
    }

    func noteTalkEnded() {
        presentingCount = max(0, presentingCount - 1)
        NotificationCenter.default.post(name: Self.presentingDidChangeNotification, object: self)
    }

    /// A talk's windows have all closed, which a new talk waits for.
    func noteTalkWindowsWentDown() {
        NotificationCenter.default.post(name: Self.presentingDidChangeNotification, object: self)
    }

    func retainEndingTalk(_ talk: PresentationController) {
        guard !endingTalks.contains(where: { $0 === talk }) else { return }
        endingTalks.append(talk)
    }

    func releaseEndingTalk(_ talk: PresentationController) {
        endingTalks.removeAll { $0 === talk }
    }

    /// The tap sessions of talks that failed while their process still
    /// ran, kept until the process has exited: the SIGTERM and SIGKILL
    /// escalation of a stop holds its process weakly, so a session freed
    /// at once would leave a tap that ignores its closed stdin running.
    private(set) var stoppingSessions: [TapSession] = []

    /// Stops `session` and keeps it alive until it reports stopped.
    func stopAndRetain(_ session: TapSession) {
        guard session.processIdentifier != nil else {
            session.stop()
            return
        }
        stoppingSessions.append(session)
        session.onEvent = nil
        session.onStateChange = { [weak self, weak session] state in
            guard let self, let session, state == .stopped else { return }
            self.stoppingSessions.removeAll { $0 === session }
        }
        session.stop()
    }

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

    /// The session configuration for a talk: the same tap and environment as
    /// tap dev, unless a test named another executable for talks.
    func presentSessionConfiguration() -> TapSession.Configuration {
        TapSession.Configuration(executableURL: presentExecutableURL ?? tapExecutableURL, environment: { [weak self] in
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
