import AppKit

/// A resizable window that holds the preview while it is out of the deck window.
final class PreviewWindowController: NSWindowController, NSWindowDelegate {
    var onClose: (() -> Void)?
    /// The deck window this preview was detached from, so a person acting on
    /// the detached preview (such as Show Tap Log) still reaches its deck.
    weak var deckWindowController: DeckWindowController?

    init(title: String) {
        let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 960, height: 640),
                              styleMask: [.titled, .closable, .miniaturizable, .resizable],
                              backing: .buffered, defer: false)
        window.title = title
        window.isReleasedWhenClosed = false
        window.center()
        super.init(window: window)
        window.delegate = self
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    func windowWillClose(_ notification: Notification) {
        let close = onClose
        onClose = nil
        close?()
    }
}
