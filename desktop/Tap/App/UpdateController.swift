import AppKit
import Sparkle

/// Sparkle's updater with its standard interface, and its delegate, with
/// the app's one rule: a talk is never interrupted. Every decision waits,
/// through `UpdateGate`, until every talk and its windows are down (a
/// relaunch a talk postponed runs then), and Play is refused while Sparkle
/// has a window or its permission prompt up.
@MainActor
final class UpdateController: NSObject, SPUUpdaterDelegate {
    let gate = UpdateGate()
    /// What Sparkle's interface has up, kept by `userDriver`. Read at Play.
    let interface = SparkleInterface()
    private(set) var userDriver: WatchedUserDriver!
    private(set) var updater: SPUUpdater!
    /// Whether the updater started. Tests and 0.0.0 builds never start it.
    private(set) var isStarted = false
    private var presentingObserver: NSObjectProtocol?

    override init() {
        super.init()
        userDriver = WatchedUserDriver(inner: SPUStandardUserDriver(hostBundle: .main, delegate: nil), interface: interface)
        updater = SPUUpdater(hostBundle: .main, applicationBundle: .main, userDriver: userDriver, delegate: self)
        presentingObserver = NotificationCenter.default.addObserver(forName: AppEnvironment.presentingDidChangeNotification, object: nil, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated { self?.presentingChanged() }
        }
    }

    deinit {
        if let presentingObserver { NotificationCenter.default.removeObserver(presentingObserver) }
    }

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
        do {
            try updater.start()
            isStarted = true
        } catch {
            NSLog("Tap: the updater did not start: %@", error.localizedDescription)
        }
    }

    /// Tap > Check for Updates…: Sparkle's own check, with its own windows.
    func checkForUpdates(_ sender: Any?) {
        updater.checkForUpdates()
    }

    /// Why Play waits for Sparkle, or nil. When a window is the reason, it
    /// comes to the front, so the words point at something on screen
    /// (Sparkle may hold an alert a scheduled check found until the app is
    /// next activated). Only while the app is active: nothing here
    /// activates it.
    func playRefusal() -> String? {
        guard let refusal = interface.playRefusal else { return nil }
        if NSApp.isActive { userDriver.showUpdateInFocus() }
        return refusal
    }

    /// The menu item's state: a started updater that Sparkle allows to check
    /// and every talk down, as `mayPerform` asks.
    var canCheckForUpdates: Bool {
        isStarted && updater.canCheckForUpdates && gate.mayCheck(mayInterrupt: talkWindowsAreDown)
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
    // Each decision waits for `talkWindowsAreDown`, not only for the talk
    // to be counted out, so nothing of Sparkle's comes up over a talk
    // window still leaving full screen.

    func updater(_ updater: SPUUpdater, mayPerform updateCheck: SPUUpdateCheck) throws {
        guard gate.mayCheck(mayInterrupt: talkWindowsAreDown) else { throw UpdateGate.PresentingError() }
    }

    func updater(_ updater: SPUUpdater, shouldProceedWithUpdate updateItem: SUAppcastItem, updateCheck: SPUUpdateCheck) throws {
        guard gate.mayProceed(mayInterrupt: talkWindowsAreDown) else { throw UpdateGate.PresentingError() }
    }

    func updater(_ updater: SPUUpdater, shouldPostponeRelaunchForUpdate item: SUAppcastItem, untilInvokingBlock installHandler: @escaping () -> Void) -> Bool {
        gate.shouldPostponeRelaunch(mayInterrupt: talkWindowsAreDown, resume: installHandler)
    }
}
