import AppKit

/// A deck's window: the editor on the left and the Preview pane on the right,
/// under a unified toolbar. Deck windows open as tabs of each other.
final class DeckWindowController: NSWindowController, NSWindowDelegate, NSToolbarDelegate, NSMenuItemValidation {
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

        let toolbar = NSToolbar(identifier: "TapDeckToolbar")
        toolbar.delegate = self
        toolbar.displayMode = .iconOnly
        window.toolbar = toolbar
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func windowDidLoad() {
        super.windowDidLoad()
        window?.makeFirstResponder(sessionController.editor)
    }

    // NSWindowController's own default windowWillReturnUndoManager already
    // returns self.document?.undoManager, since this controller is the
    // window's delegate. That already makes the editor's undo manager the
    // document's own, which DeckSessionController relies on to observe undo
    // and redo (see DeckSessionController.init); an explicit override here
    // was tried and confirmed by mutation to change nothing.

    static let previewItemIdentifier = NSToolbarItem.Identifier("preview")

    private(set) var previewWindowController: PreviewWindowController?

    @objc func togglePreview(_ sender: Any?) {
        if let controller = previewWindowController {
            controller.close()
            return
        }
        splitViewController.setPreviewHidden(!splitViewController.isPreviewHidden)
    }

    @objc func togglePreviewPin(_ sender: Any?) {
        sessionController.togglePin()
    }

    /// Moves the preview into its own window. It keeps following the cursor.
    @objc func showPreviewInWindow(_ sender: Any?) {
        if let existing = previewWindowController {
            existing.showWindow(nil)
            return
        }
        let preview = sessionController.previewViewController
        preview.view.removeFromSuperview()
        preview.removeFromParent()
        let controller = PreviewWindowController(title: "\(window?.title ?? "Deck"): Preview")
        controller.deckWindowController = self
        controller.window?.contentViewController = preview
        controller.window?.setContentSize(NSSize(width: 960, height: 640))
        controller.onClose = { [weak self] in self?.dockPreview() }
        previewWindowController = controller
        splitViewController.setPreviewHidden(true)
        controller.window?.makeKeyAndOrderFront(nil)
    }

    /// Puts the preview back next to the editor.
    func dockPreview() {
        guard let controller = previewWindowController else { return }
        previewWindowController = nil
        controller.window?.contentViewController = nil
        sessionController.inspectorViewController.embed(sessionController.previewViewController)
        splitViewController.setPreviewHidden(false)
    }

    func windowWillClose(_ notification: Notification) {
        if let controller = previewWindowController {
            controller.onClose = nil
            previewWindowController = nil
            controller.close()
        }
    }

    func validateMenuItem(_ menuItem: NSMenuItem) -> Bool {
        if menuItem.action == #selector(togglePreview(_:)) {
            menuItem.title = splitViewController.isPreviewHidden ? "Show Preview" : "Hide Preview"
        }
        if menuItem.action == #selector(togglePreviewPin(_:)) {
            menuItem.title = sessionController.navigator.isPinned ? "Unpin Preview" : "Pin Preview"
        }
        return true
    }

    func toolbarDefaultItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
        [.flexibleSpace, Self.previewItemIdentifier]
    }

    func toolbarAllowedItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
        toolbarDefaultItemIdentifiers(toolbar)
    }

    func toolbar(_ toolbar: NSToolbar, itemForItemIdentifier identifier: NSToolbarItem.Identifier,
                 willBeInsertedIntoToolbar flag: Bool) -> NSToolbarItem? {
        guard identifier == Self.previewItemIdentifier else { return nil }
        let item = NSToolbarItem(itemIdentifier: identifier)
        item.label = "Preview"
        item.toolTip = "Show or hide the preview"
        item.image = NSImage(systemSymbolName: "sidebar.right", accessibilityDescription: "Preview")
        item.isBordered = true
        item.target = self
        item.action = #selector(togglePreview(_:))
        return item
    }
}
