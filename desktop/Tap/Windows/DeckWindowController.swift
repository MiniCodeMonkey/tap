import AppKit

/// A deck's window: the editor on the left and the Preview pane on the right,
/// under a unified toolbar. Deck windows open as tabs of each other.
final class DeckWindowController: NSWindowController, NSWindowDelegate {
    let sessionController: DeckSessionController
    let splitViewController: MainSplitViewController

    init(sessionController: DeckSessionController) {
        self.sessionController = sessionController
        splitViewController = MainSplitViewController(editor: sessionController.editorViewController,
                                                      inspector: sessionController.inspectorViewController)
        let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 1440, height: 900),
                              styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView],
                              backing: .buffered, defer: false)
        window.toolbarStyle = .unified
        window.tabbingMode = .preferred
        window.tabbingIdentifier = "TapDeck"
        window.isReleasedWhenClosed = false
        window.contentViewController = splitViewController
        window.setContentSize(NSSize(width: 1440, height: 900))
        window.center()
        super.init(window: window)
        window.delegate = self
        shouldCascadeWindows = true
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func windowDidLoad() {
        super.windowDidLoad()
        window?.makeFirstResponder(sessionController.editor)
    }
}
