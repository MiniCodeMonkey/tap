import AppKit

/// A deck's window: the editor on the left and the Preview pane on the right,
/// under a unified toolbar. Deck windows open as tabs of each other.
final class DeckWindowController: NSWindowController, NSWindowDelegate, NSToolbarDelegate, NSMenuItemValidation {
    static let slidesItemIdentifier = NSToolbarItem.Identifier("slides")
    let sessionController: DeckSessionController
    let splitViewController: MainSplitViewController
    let sidebarHost = SidebarHostViewController()
    let deckContentViewController: DeckContentViewController
    let panelOverlay = SlidePanelOverlay(frame: .zero)
    let panelPeek = SlidePanelPeek()
    private(set) var isPanelPinned = true
    private let slidesButton = HoverButton()
    private var visibilityObserver: NSObjectProtocol?

    init(sessionController: DeckSessionController) {
        self.sessionController = sessionController
        sidebarHost.host(sessionController.slidePanel.view)
        splitViewController = MainSplitViewController(sidebar: sidebarHost, editor: sessionController.editorViewController,
                                                      inspector: sessionController.inspectorViewController)
        deckContentViewController = DeckContentViewController(splitViewController: splitViewController, overlay: panelOverlay)
        let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 1440, height: 900),
                              styleMask: [.titled, .closable, .miniaturizable, .resizable, .fullSizeContentView],
                              backing: .buffered, defer: false)
        window.toolbarStyle = .unified
        window.tabbingMode = .preferred
        window.tabbingIdentifier = "TapDeck"
        window.isReleasedWhenClosed = false
        window.contentViewController = deckContentViewController
        window.setContentSize(NSSize(width: 1440, height: 900))
        window.center()
        super.init(window: window)
        window.delegate = self
        shouldCascadeWindows = true

        let toolbar = NSToolbar(identifier: "TapDeckToolbar")
        toolbar.delegate = self
        toolbar.displayMode = .iconOnly
        window.toolbar = toolbar

        panelOverlay.onPointerEntered = { [weak self] in self?.panelPeek.pointerEnteredPanel() }
        panelOverlay.onPointerLeft = { [weak self] in self?.panelPeek.pointerLeftPanel() }
        panelPeek.onShow = { [weak self] in
            guard let self else { return }
            self.panelOverlay.host(self.sessionController.slidePanel.view)
            self.panelOverlay.isHidden = false
        }
        panelPeek.onHide = { [weak self] in
            guard let self else { return }
            // Focus must not stay in a panel nobody can see: a click in the peeked
            // panel left it in the collection view (decision 1), and a Delete there
            // would delete slides out of sight. The editor is inside this same
            // window, and the person's own pointer closed the peek.
            if let responder = self.window?.firstResponder as? NSView, responder.isDescendant(of: self.panelOverlay) {
                self.window?.makeFirstResponder(self.sessionController.editor)
            }
            self.panelOverlay.isHidden = true
        }
        let deckURL = sessionController.document?.fileURL
        setPanelPinned(deckURL.map { AppEnvironment.shared.panelState.isPinned(deck: $0) } ?? true)
        // AppKit's own space negotiation for a sidebar-style split item runs
        // its first real pass only once the window actually appears on
        // screen, joining any existing tab group, which can silently
        // collapse a panel that was set pinned above before the window had
        // a real frame to lay out against. This reasserts it the first
        // time the window becomes visible, then stops watching: a later
        // pin or unpin the person makes must not be undone by a stray
        // occlusion change.
        visibilityObserver = NotificationCenter.default.addObserver(forName: NSWindow.didChangeOcclusionStateNotification, object: window, queue: nil) { [weak self] _ in
            MainActor.assumeIsolated {
                guard let self, window.occlusionState.contains(.visible) else { return }
                self.setPanelPinned(self.isPanelPinned)
                if let observer = self.visibilityObserver {
                    NotificationCenter.default.removeObserver(observer)
                    self.visibilityObserver = nil
                }
            }
        }
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    deinit {
        if let visibilityObserver {
            NotificationCenter.default.removeObserver(visibilityObserver)
        }
    }

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

    private(set) var goToSlideController: GoToSlideController?

    /// Brings the deck's own window forward after a confirmed jump. A
    /// separate, replaceable step rather than an inline `makeKeyAndOrderFront`
    /// call so a test can observe that it ran, and ran on exactly the deck
    /// window, without relying on `NSApp.orderedWindows`: this hosted test
    /// host does not reflect `makeKeyAndOrderFront` there reliably enough to
    /// tell a confirmed jump apart from one with this step removed
    /// (confirmed directly while fixing that gap).
    var bringDeckWindowForward: (NSWindow) -> Void = { $0.makeKeyAndOrderFront(nil) }

    /// Shows the Go to Slide panel over this deck's own window.
    @objc func goToSlide(_ sender: Any?) {
        showGoToSlide(over: window)
    }

    /// Shows the Go to Slide panel over the given window, or brings the
    /// existing one back if it is already open. The window is whichever one
    /// the person actually invoked it from: this deck's own window when
    /// Cmd+Shift+O reaches `goToSlide(_:)` directly, or the detached preview
    /// window when `AppDelegate.goToSlide` forwards here for that case
    /// (`DeckWindowController.showPreviewInWindow`). A confirmed jump always
    /// brings this deck's own window to the front afterward, since that is
    /// where the visible effect of the jump, the cursor move, actually
    /// happens, and the panel might have been shown over a different window
    /// than that; cancelling leaves window order alone.
    func showGoToSlide(over window: NSWindow?) {
        guard let window else { return }
        let controller = goToSlideController ?? GoToSlideController(
            slides: { [weak self] in self?.sessionController.editor.boxes.map(\.slide) ?? [] },
            jump: { [weak self] number in
                if let self, let deckWindow = self.window {
                    self.bringDeckWindowForward(deckWindow)
                }
                self?.sessionController.jumpToSlide(number: number)
            })
        goToSlideController = controller
        controller.show(over: window)
    }

    /// Pins the panel as a sidebar that pushes the editor and the right
    /// pane over, or unpins it so hovering the toolbar button peeks at it.
    func setPanelPinned(_ pinned: Bool) {
        isPanelPinned = pinned
        panelPeek.isEnabled = !pinned
        if pinned {
            panelOverlay.isHidden = true
            sidebarHost.host(sessionController.slidePanel.view)
            splitViewController.setSidebarCollapsed(false)
        } else {
            splitViewController.setSidebarCollapsed(true)
            panelOverlay.host(sessionController.slidePanel.view)
            panelOverlay.isHidden = true
        }
        slidesButton.state = pinned ? .on : .off
        if let deck = sessionController.document?.fileURL {
            AppEnvironment.shared.panelState.setPinned(pinned, deck: deck)
        }
    }

    @objc func toggleSlidePanel(_ sender: Any?) {
        setPanelPinned(!isPanelPinned)
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
        if menuItem.action == #selector(toggleSlidePanel(_:)) {
            menuItem.title = isPanelPinned ? "Unpin Slide Panel" : "Pin Slide Panel"
        }
        return true
    }

    func toolbarDefaultItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
        [Self.slidesItemIdentifier, .flexibleSpace, Self.previewItemIdentifier]
    }

    func toolbarAllowedItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
        toolbarDefaultItemIdentifiers(toolbar)
    }

    func toolbar(_ toolbar: NSToolbar, itemForItemIdentifier identifier: NSToolbarItem.Identifier,
                 willBeInsertedIntoToolbar flag: Bool) -> NSToolbarItem? {
        if identifier == Self.slidesItemIdentifier {
            let item = NSToolbarItem(itemIdentifier: identifier)
            item.label = "Slides"
            item.toolTip = "Hover to peek at the slides, click to pin them"
            slidesButton.image = NSImage(systemSymbolName: "sidebar.left", accessibilityDescription: "Slides")
            slidesButton.bezelStyle = .toolbar
            slidesButton.setButtonType(.pushOnPushOff)
            slidesButton.target = self
            slidesButton.action = #selector(toggleSlidePanel(_:))
            slidesButton.setAccessibilityIdentifier("slides-button")
            slidesButton.onPointerEntered = { [weak self] in self?.panelPeek.pointerEnteredButton() }
            slidesButton.onPointerLeft = { [weak self] in self?.panelPeek.pointerLeftButton() }
            item.view = slidesButton
            return item
        }
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
