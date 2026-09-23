import AppKit

@MainActor
final class AppDelegate: NSObject, NSApplicationDelegate {
    // main.swift's top-level code runs on the main thread but is not itself
    // main-actor isolated, so it cannot call the implicit isolated init.
    // Constructing an NSObject subclass touches no main-actor state, so an
    // explicit nonisolated init is safe and lets main.swift call it directly.
    override nonisolated init() {
        super.init()
    }

    func applicationDidFinishLaunching(_ notification: Notification) {
        NSApp.mainMenu = MainMenu.build()
        AppEnvironment.shared.warmUp()
        NSDocumentController.shared.autosavingDelay = 1
    }

    /// There is no untitled document to offer: "New deck" is deliberately
    /// inert until it has its own sheet. Without this, AppKit tries to open
    /// one anyway, fails, and presents the failure as a modal alert that
    /// blocks the app (and any test host) forever.
    func applicationShouldOpenUntitledFile(_ sender: NSApplication) -> Bool {
        false
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
}
