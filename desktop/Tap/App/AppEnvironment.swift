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
    /// The theme tour's slide thumbnails rendered at build time, nil when the app has none. A test replaces this.
    var tourThumbnails: BundledTourThumbnails? = Bundle.main.resourceURL.map(BundledTourThumbnails.init(resourcesFolder:))
    /// Every layout tap offers, loaded once from the bundled tap.
    lazy var layoutCatalog = LayoutCatalogLoader(executable: { [weak self] in self?.tapExecutableURL ?? URL(fileURLWithPath: "/usr/bin/false") })
    /// Every frontmatter key tap understands, loaded once from the bundled tap.
    lazy var deckSchema = DeckSchemaLoader(executable: { [weak self] in self?.tapExecutableURL ?? URL(fileURLWithPath: "/usr/bin/false") })
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
    /// The displays each deck was last played on, so Play asks first when a deck is new or the displays changed.
    var presentedDisplays = PresentedDisplaysStore()
    /// The port each deck's talks run on, so the talk pages keep one origin.
    var deckPorts = DeckPortStore()
    /// The Present popover's last settings, which Cmd+Option+P starts with.
    var presentationSettings = PresentationSettingsStore()
    /// A tap for talks alone, for tests that script tap present while the
    /// deck's real tap dev keeps running. nil runs the bundled tap.
    var presentExecutableURL: URL?
    /// The tap the one-shot commands run (tap new, tap theme show, tap
    /// export ...), for tests that script a subcommand while the deck's
    /// real tap dev keeps running. nil runs the bundled tap.
    var toolExecutableURL: URL?
    /// Where the Gemini key lives. Read in two places only: TapTool's image
    /// runs and the Image Generation pane. Under -TapDefaultsSuite (every
    /// UI test launch) it is a store in memory, so no test reads or writes
    /// the person's Keychain; every hosted test installs one too.
    var geminiKeyStore: GeminiKeyStore = KeychainGeminiKeyStore()
    /// Links the bundled tap into ~/.local/bin for Settings > Command
    /// Line. A test points this at a folder of its own.
    lazy var commandLineInstaller = CommandLineInstaller(bundledTap: tapExecutableURL)
    /// The General pane's settings. A test replaces this with one on a fresh suite.
    var generalSettings = GeneralSettings()
    /// The theme catalog and every theme's render, loaded once per app.
    lazy var themeImages = ThemeImageLoader()
    /// Whether the Focus hint has been shown on this Mac. A test replaces it.
    var focusHint = FocusHintState()
    /// The macOS permissions the recording setup screen asks for. A test replaces it with a fake, so no test shows a real prompt.
    var recordingPermissions: RecordingPermissions = LiveRecordingPermissions()
    /// Whether the person put the recording setup screen off, and whether Screen Recording awaits its confirming relaunch.
    var recordingSetupStore = RecordingSetupStore()
    #if DEBUG
    /// Test only, and compiled only into a Debug build: answers a live
    /// code approval before any sheet shows, for a deck a test approved
    /// ahead of time. It returns true (allow) or nil (show the sheet as
    /// usual), never false, so it cannot hide a question a test expects.
    /// Nil unless a test sets it; nothing reads it from defaults, launch
    /// arguments or the environment.
    var approvalAnswerForTests: (@MainActor (QuestionPayload) -> Bool?)?
    #endif
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
    private var generalSettingsObserver: NSObjectProtocol?

    init() {
        if let override = UserDefaults.standard.string(forKey: "TapExecutablePath") {
            tapExecutableURL = URL(fileURLWithPath: override)
        } else {
            tapExecutableURL = Bundle.main.url(forResource: "tap", withExtension: nil) ?? URL(fileURLWithPath: "/usr/bin/false")
        }
        loginShellLoader = LoginShellEnvironmentLoader(shellPath: ProcessInfo.processInfo.environment["SHELL"] ?? "/bin/zsh")
        // UI tests pass -TapConfigHome <folder>, so the tap they drive reads
        // and writes a settings file of their own, never the person's.
        if let configHome = UserDefaults.standard.string(forKey: "TapConfigHome") {
            extraEnvironment["XDG_CONFIG_HOME"] = configHome
        }
        // They pass -TapDefaultsSuite <name> too, so the app's own settings
        // (the Present popover's, the deck ports, the display assignments,
        // the panel and the last layout) go to a suite of their own.
        if let suiteName = UserDefaults.standard.string(forKey: "TapDefaultsSuite"), let defaults = UserDefaults(suiteName: suiteName) {
            panelState = SlidePanelState(defaults: defaults)
            lastLayout = LastLayout(defaults: defaults)
            displayAssignments = DisplayAssignmentStore(defaults: defaults)
            presentedDisplays = PresentedDisplaysStore(defaults: defaults)
            deckPorts = DeckPortStore(defaults: defaults)
            presentationSettings = PresentationSettingsStore(defaults: defaults)
            focusHint = FocusHintState(defaults: defaults)
            recordingSetupStore = RecordingSetupStore(defaults: defaults)
            generalSettings = GeneralSettings(defaults: defaults)
            geminiKeyStore = MemoryGeminiKeyStore()
        }
    }

    /// Which key tap's image runs get, for the Image Generation pane's
    /// label: the login shell's, else the Keychain's, else none. This
    /// reads the store; nothing else outside TapTool does.
    func geminiKeySource() async -> GeminiKeySource {
        var shell = await loginShellLoader.environment().variables
        shell.merge(extraEnvironment) { _, extra in extra }
        return GeminiKeySource.resolve(shellValue: shell["GEMINI_API_KEY"], storedKey: try? geminiKeyStore.read())
    }

    /// Starts reading the login shell environment and the tap version.
    func warmUp() {
        Task { @MainActor in
            let environment = await loginShellLoader.environment()
            environmentNotice = environment.notice
            tapSearchPath = extraEnvironment["PATH"] ?? environment.variables["PATH"]
            bundledTapVersion = await Self.readVersion(of: tapExecutableURL)
            NotificationCenter.default.post(name: Self.didLoadNotification, object: self)
        }
        Task { await layoutCatalog.load() }
        Task { await deckSchema.load() }
        EditorTypography.refresh(from: generalSettings)
        if let generalSettingsObserver { NotificationCenter.default.removeObserver(generalSettingsObserver) }
        generalSettingsObserver = NotificationCenter.default.addObserver(forName: GeneralSettings.didChangeNotification, object: nil, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated {
                guard let self else { return }
                EditorTypography.refresh(from: self.generalSettings)
                NSDocumentController.shared.autosavingDelay = self.generalSettings.autosaveDelay
            }
        }
    }

    func tapEnvironment() async -> [String: String] {
        var variables = await loginShellLoader.environment().variables
        variables.merge(extraEnvironment) { _, extra in extra }
        return variables
    }

    /// A test's answer to whether cloudflared is installed; nil looks on the PATH tap runs with.
    var cloudflaredProbe: (() -> Bool)?
    /// The PATH tap runs with, once the login shell's environment has loaded.
    private(set) var tapSearchPath: String?

    /// Whether tap can start its tunnel, the way tap decides: cloudflared
    /// on the PATH it runs with. Synchronous: it reads the PATH the last
    /// environment load left, or the app's own before that, and looks for
    /// one file per directory.
    func isCloudflaredInstalled() -> Bool {
        if let cloudflaredProbe { return cloudflaredProbe() }
        return CloudflaredLocator.isInstalled(searchPath: tapSearchPath ?? extraEnvironment["PATH"] ?? ProcessInfo.processInfo.environment["PATH"])
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
