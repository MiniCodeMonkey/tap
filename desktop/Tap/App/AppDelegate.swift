import AppKit

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate, NSMenuItemValidation {
    // main.swift's top-level code runs on the main thread but is not itself
    // main-actor isolated, so it cannot call the implicit isolated init.
    // Constructing an NSObject subclass touches no main-actor state, so an
    // explicit nonisolated init is safe and lets main.swift call it directly.
    override nonisolated init() {
        super.init()
    }

    /// The menu bar is in place before launch finishes: NSDocumentController
    /// installs its Revert To menu (with Browse All Versions) and its Share
    /// menu into the File menu as launch finishes. A menu bar set any later
    /// keeps only the plain Revert to Saved item, which AppKit then hides for
    /// a document that autosaves in place.
    func applicationWillFinishLaunching(_ notification: Notification) {
        NSApp.mainMenu = MainMenu.build()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        AppEnvironment.shared.warmUp()
        NSDocumentController.shared.autosavingDelay = 1
        NotificationCenter.default.addObserver(self, selector: #selector(deckWindowWillClose(_:)), name: NSWindow.willCloseNotification, object: nil)
        // UI tests pass -TapOpenOnLaunch <path>. The completion-handler form
        // never presents an error panel, so a missing or unreadable path
        // fails quietly rather than blocking launch with a modal alert.
        if let path = UserDefaults.standard.string(forKey: "TapOpenOnLaunch") {
            NSDocumentController.shared.openDocument(withContentsOf: URL(fileURLWithPath: path), display: true) { _, _, _ in }
        }
        // Decks opened from Finder at launch arrive first.
        DispatchQueue.main.async { MainActor.assumeIsolated { self.showWelcomeIfNoDecks() } }
    }

    /// There is no untitled document to offer: "New deck" is deliberately
    /// inert until it has its own sheet. Without this, AppKit tries to open
    /// one anyway, fails, and presents the failure as a modal alert that
    /// blocks the app (and any test host) forever.
    func applicationShouldOpenUntitledFile(_ sender: NSApplication) -> Bool {
        false
    }

    /// The app has no untitled document and no dock-only mode: quitting the
    /// last deck window leaves the welcome window as the way back in, not a
    /// silent termination.
    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        false
    }

    func applicationShouldHandleReopen(_ sender: NSApplication, hasVisibleWindows: Bool) -> Bool {
        if !hasVisibleWindows { showWelcomeIfNoDecks() }
        return false
    }

    /// Shows the welcome window, unless a deck is already open. Safe to call
    /// whenever the set of open decks might have changed: at launch, after a
    /// deck window closes, and on a Dock reopen with no visible windows.
    func showWelcomeIfNoDecks() {
        guard !NSDocumentController.shared.documents.contains(where: { $0 is DeckDocument }) else { return }
        WelcomeWindowController.shared.showWindow(nil)
    }

    /// A deck window closing can be the app's last window. `DeckDocument`
    /// closes after its own window, so this looks one main-queue turn later,
    /// once the document itself is gone from `NSDocumentController`.
    @objc private func deckWindowWillClose(_ notification: Notification) {
        guard (notification.object as? NSWindow)?.windowController is DeckWindowController else { return }
        DispatchQueue.main.async { MainActor.assumeIsolated { self.showWelcomeIfNoDecks() } }
    }

    @objc func showAbout(_ sender: Any?) {
        NSApp.orderFrontStandardAboutPanel(options: aboutPanelOptions())
    }

    /// The About panel names the bundled tap's version.
    func aboutPanelOptions() -> [NSApplication.AboutPanelOptionKey: Any] {
        let version = AppEnvironment.shared.bundledTapVersion ?? "unknown"
        return [.credits: NSAttributedString(string: "Bundled tap \(version)")]
    }

    /// The deck that owns a window, whether that window is the deck's own
    /// window or its preview detached into a window of its own (see
    /// `DeckWindowController.showPreviewInWindow`). Kept as a standalone
    /// function so it can be tested against a real window without driving
    /// `NSApp.keyWindow`.
    static func deck(owning window: NSWindow?) -> DeckWindowController? {
        if let deck = window?.windowController as? DeckWindowController { return deck }
        return (window?.windowController as? PreviewWindowController)?.deckWindowController
    }

    @objc func showTapLog(_ sender: Any?) {
        let keyDeck = Self.deck(owning: NSApp.keyWindow)
        TapLogWindowController.shared.show(log: keyDeck?.sessionController.session.log)
    }

    @objc func showHelp(_ sender: Any?) {
        NSWorkspace.shared.open(URL(string: "https://github.com/MiniCodeMonkey/tap")!)
    }

    /// Go to Slide (Cmd+Shift+O) is a `DeckWindowController` action, found
    /// through the responder chain of the deck's own window. That chain does
    /// not reach it when the deck's preview is detached into a window of its
    /// own (`DeckWindowController.showPreviewInWindow`) and that window is
    /// key: the same gap `showTapLog` closes above. Having the same selector
    /// here too means AppKit's nil-targeted lookup falls back to this method
    /// once it walks past the key window to the app delegate, so Cmd+Shift+O
    /// still opens Go to Slide for the deck that owns the detached preview,
    /// over the detached preview window itself, the one the person actually
    /// invoked it from.
    @objc func goToSlide(_ sender: Any?) {
        let keyWindow = NSApp.keyWindow
        Self.deck(owning: keyWindow)?.showGoToSlide(over: keyWindow)
    }

    /// Whether Go to Slide should be enabled for the given key window.
    /// Kept as a standalone function, the same way `deck(owning:)` above is,
    /// so it can be tested against a real window without driving
    /// `NSApp.keyWindow`.
    static func goToSlideIsEnabled(forKeyWindow window: NSWindow?) -> Bool {
        deck(owning: window) != nil
    }

    /// Only asked when no responder earlier in the chain answered for
    /// `goToSlide(_:)` itself, which is `DeckWindowController`'s own case
    /// (its `validateMenuItem` decides then, always true: a deck window can
    /// always resolve its own deck). Here, with no deck window to reach, the
    /// item must be disabled rather than enabled and inert, so this returns
    /// false whenever the key window resolves to no deck at all, such as the
    /// welcome window or a detached preview whose deck window has since
    /// closed.
    func validateMenuItem(_ menuItem: NSMenuItem) -> Bool {
        if menuItem.action == #selector(goToSlide(_:)) {
            return Self.goToSlideIsEnabled(forKeyWindow: NSApp.keyWindow)
        }
        return true
    }
}
