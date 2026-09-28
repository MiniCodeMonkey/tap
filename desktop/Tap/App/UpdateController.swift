import AppKit
import Sparkle

/// Sparkle's standard controller and its delegate, with the app's one rule:
/// a talk is never interrupted. Every decision reads `updatesMayInterrupt`
/// through `UpdateGate`, a relaunch a talk postponed runs once the last
/// talk's windows are down, and Play is refused while an update session is
/// in progress.
@MainActor
final class UpdateController: NSObject, SPUUpdaterDelegate {
    let gate = UpdateGate()
    private(set) var controller: SPUStandardUpdaterController!
    /// Whether `startUpdater` has run. Tests and 0.0.0 builds never start it.
    private(set) var isStarted = false
    private var presentingObserver: NSObjectProtocol?
    /// Whether Sparkle is between a check and its end (an alert up, a
    /// download, an install waiting). Read at Play. A test replaces it.
    lazy var isUpdateSessionInProgress: () -> Bool = { [weak self] in self?.isStarted == true && self?.updater.sessionInProgress == true }

    override init() {
        super.init()
        controller = SPUStandardUpdaterController(startingUpdater: false, updaterDelegate: self, userDriverDelegate: nil)
        presentingObserver = NotificationCenter.default.addObserver(forName: AppEnvironment.presentingDidChangeNotification, object: nil, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated { self?.presentingChanged() }
        }
    }

    deinit {
        if let presentingObserver { NotificationCenter.default.removeObserver(presentingObserver) }
    }

    var updater: SPUUpdater { controller.updater }

    /// A process that hosts tests: xcodebuild sets the configuration path.
    static var isHostedByTests: Bool { ProcessInfo.processInfo.environment["XCTestConfigurationFilePath"] != nil }
    /// A process a UI test launched: it passes its own defaults suite.
    static var hasTestDefaultsSuite: Bool { UserDefaults.standard.string(forKey: "TapDefaultsSuite") != nil }
    static var runsUnderTests: Bool { isHostedByTests || hasTestDefaultsSuite }

    static var bundleVersion: String? {
        Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String
    }

    /// Starts Sparkle where a release runs for a person; nowhere else.
    func startIfAllowed() {
        guard !isStarted, UpdateGate.updaterMayStart(bundleVersion: Self.bundleVersion, isHostedByTests: Self.isHostedByTests, hasTestDefaultsSuite: Self.hasTestDefaultsSuite) else { return }
        controller.startUpdater()
        isStarted = true
    }

    /// Tap > Check for Updates…: Sparkle's own check, with its own windows.
    func checkForUpdates(_ sender: Any?) {
        controller.checkForUpdates(sender)
    }

    /// The menu item's state: a started updater that Sparkle allows to check
    /// and no talk running.
    var canCheckForUpdates: Bool {
        isStarted && updater.canCheckForUpdates && gate.mayCheck(mayInterrupt: AppEnvironment.shared.updatesMayInterrupt)
    }

    /// Whether every talk is over and its windows are gone: no deck's talk
    /// is ending, no talk that outlived its deck is still ending, and no
    /// talk window is mid full screen. A relaunch runs only then.
    var talkWindowsAreDown: Bool {
        guard AppEnvironment.shared.updatesMayInterrupt, AppEnvironment.shared.endingTalks.isEmpty, !PresentationWindow.anyIsBusyWithFullScreen else { return false }
        return !NSDocumentController.shared.documents.contains { document in
            (document as? DeckDocument)?.sessionController?.presentationIfCreated?.isEnding == true
        }
    }

    /// Talks change in three steps (started, ended, windows down), each
    /// posting the same notification; the postponed relaunch waits for the
    /// last one. A talk whose deck closed during its ending leaves
    /// `endingTalks` one run-loop turn after its windows-down notification,
    /// so the check runs again on the next turn.
    private func presentingChanged() {
        if talkWindowsAreDown { gate.resumePostponedRelaunch(); return }
        DispatchQueue.main.async { [weak self] in
            MainActor.assumeIsolated {
                guard let self, self.talkWindowsAreDown else { return }
                self.gate.resumePostponedRelaunch()
            }
        }
    }

    // MARK: SPUUpdaterDelegate

    func updater(_ updater: SPUUpdater, mayPerform updateCheck: SPUUpdateCheck) throws {
        guard gate.mayCheck(mayInterrupt: AppEnvironment.shared.updatesMayInterrupt) else { throw UpdateGate.PresentingError() }
    }

    func updater(_ updater: SPUUpdater, shouldProceedWithUpdate updateItem: SUAppcastItem, updateCheck: SPUUpdateCheck) throws {
        guard gate.mayProceed(mayInterrupt: AppEnvironment.shared.updatesMayInterrupt) else { throw UpdateGate.PresentingError() }
    }

    func updater(_ updater: SPUUpdater, shouldPostponeRelaunchForUpdate item: SUAppcastItem, untilInvokingBlock installHandler: @escaping () -> Void) -> Bool {
        gate.shouldPostponeRelaunch(mayInterrupt: AppEnvironment.shared.updatesMayInterrupt, resume: installHandler)
    }
}
