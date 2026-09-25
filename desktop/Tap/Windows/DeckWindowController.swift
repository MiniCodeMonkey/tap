import AppKit

/// A deck's window: the editor on the left and the Preview pane on the right,
/// under a unified toolbar. Deck windows open as tabs of each other.
final class DeckWindowController: NSWindowController, NSWindowDelegate, NSToolbarDelegate, NSMenuItemValidation {
    static let slidesItemIdentifier = NSToolbarItem.Identifier("slides")
    static let newSlideItemIdentifier = NSToolbarItem.Identifier("newSlide")
    let sessionController: DeckSessionController
    let splitViewController: MainSplitViewController
    let sidebarHost = SidebarHostViewController()
    let deckContentViewController: DeckContentViewController
    let panelOverlay = SlidePanelOverlay(frame: .zero)
    let panelPeek = SlidePanelPeek()
    private(set) var isPanelPinned = true
    /// The toolbar's Slides button: on while the panel is pinned.
    let slidesButton = HoverButton()
    /// The toolbar's New Slide button: a click inserts the last layout, a hold opens the gallery.
    let newSlideButton = NewSlideButton()
    private(set) lazy var layoutGallery: LayoutGalleryController = {
        let gallery = LayoutGalleryController()
        gallery.onPick = { [weak self] name in self?.insertSlide(layout: name, after: .caret) }
        return gallery
    }()
    private var sidebarCollapseObservation: NSKeyValueObservation?
    private var isReconcilingSidebarCollapse = false

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
        sessionController.presentation.deckWindowController = self

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
            // panel leaves it in the collection view, and a Delete there
            // would delete slides out of sight. The editor is inside this same
            // window, and the person's own pointer closed the peek.
            if let responder = self.window?.firstResponder as? NSView, responder.isDescendant(of: self.panelOverlay) {
                self.window?.makeFirstResponder(self.sessionController.editor)
            }
            self.panelOverlay.isHidden = true
        }
        let deckURL = sessionController.document?.fileURL
        setPanelPinned(deckURL.map { AppEnvironment.shared.panelState.isPinned(deck: $0) } ?? true)
        // Two things change whether the sidebar is collapsed besides
        // setPanelPinned. A person can drag the sidebar's divider: that is
        // the same as the pin button, so dragging it closed unpins this
        // deck and dragging it open pins it, and the state is stored per
        // deck. And on every tab swap, AppKit's own window tab stack copies
        // the prior tab's split-view divider positions into the new tab
        // (NSWindowStackController _syncWindowFrameStateForSwapWithNewWindow,
        // called from NSDocument.showWindows through makeKeyAndOrderFront),
        // which can collapse or uncollapse this deck's sidebar to match
        // whichever deck was the prior tab. That runs on every swap, so it
        // is watched for the life of the window. A change that leaves
        // isCollapsed agreeing with isPanelPinned (the correct state is
        // always the opposite) and that no person's drag made is AppKit's
        // copy misfiring, and it is undone. Both are applied a turn later,
        // after AppKit's own change has finished.
        sidebarCollapseObservation = splitViewController.sidebarItem.observe(\.isCollapsed, options: [.new]) { [weak self] _, change in
            MainActor.assumeIsolated {
                guard let self, let isCollapsed = change.newValue else { return }
                if self.splitViewController.isPersonDraggingADivider {
                    DispatchQueue.main.async { [weak self] in
                        MainActor.assumeIsolated {
                            guard let self else { return }
                            let pinned = !self.splitViewController.sidebarItem.isCollapsed
                            if pinned != self.isPanelPinned { self.setPanelPinned(pinned) }
                        }
                    }
                    return
                }
                guard isCollapsed == self.isPanelPinned, !self.isReconcilingSidebarCollapse else { return }
                self.isReconcilingSidebarCollapse = true
                DispatchQueue.main.async { [weak self] in
                    MainActor.assumeIsolated {
                        guard let self else { return }
                        self.isReconcilingSidebarCollapse = false
                        self.setPanelPinned(self.isPanelPinned)
                    }
                }
            }
        }
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

    @objc func newSlide(_ sender: Any?) {
        insertSlide(layout: AppEnvironment.shared.lastLayout.name, after: .caret)
    }

    @objc func newSlideFromLayout(_ sender: Any?) {
        guard let name = (sender as? NSMenuItem)?.representedObject as? String else { return }
        insertSlide(layout: name, after: .caret)
    }

    @objc func showLayoutGallery(_ sender: Any?) {
        let anchor: NSView = newSlideButton.window == nil ? (window?.contentView ?? newSlideButton) : newSlideButton
        layoutGallery.show(templates: AppEnvironment.shared.layoutCatalog.templates, relativeTo: anchor.bounds, of: anchor,
                           afterSlide: sessionController.currentSlideNumber)
    }

    /// Every New Slide comes here: the toolbar button, the Slide menu, the
    /// context menu and the gallery. It inserts a slide of the layout after
    /// `selection` (or at the end), and remembers the layout for the next
    /// New Slide. The name is resolved against tap's catalog first, so a
    /// stored last layout that tap no longer offers becomes one it does.
    func insertSlide(layout requested: String, after selection: SlideSelection) {
        let catalog = AppEnvironment.shared.layoutCatalog
        let layout = LayoutCatalog.resolvedName(requested, in: catalog.templates)
        guard let template = catalog.template(named: layout) else {
            sessionController.session.log.append("no template for layout \(layout): the layout catalog has not loaded", source: .app)
            NSSound.beep()
            Task { await catalog.load() }
            return
        }
        // The layout becomes the last one only once its slide exists: an
        // insert queued behind tap's confirmation and then abandoned leaves
        // the last layout as it was.
        sessionController.insertNewSlide(markdown: template.markdown, after: selection) { inserted in
            if inserted { AppEnvironment.shared.lastLayout.name = layout }
        }
    }

    // The slide commands name the selection now and resolve it when they
    // run, which may be after tap's answer renumbers the slides. See
    // `SlideSelection`.

    @objc func duplicateSlides(_ sender: Any?) {
        sessionController.perform(on: sessionController.captureSelection()) { .duplicate(numbers: $0) }
    }

    /// The core refuses to delete every slide, so a deck keeps at least one.
    @objc func deleteSlides(_ sender: Any?) {
        sessionController.perform(on: sessionController.captureSelection()) { .delete(numbers: $0) }
    }

    /// Skips the slides, or unskips them when every one is skipped, judged
    /// on the slides the command resolves to.
    @objc func toggleSkipSlides(_ sender: Any?) {
        sessionController.perform(on: sessionController.captureSelection()) { [weak sessionController] numbers in
            .setSkip(numbers: numbers, skipped: !(sessionController?.slidesAreSkipped(numbers) ?? false))
        }
    }

    /// True when every selected slide is skipped, so the menu offers Unskip.
    private var selectionIsSkipped: Bool {
        sessionController.slidesAreSkipped(sessionController.selectedSlideNumbers)
    }

    @objc func moveSlidesUp(_ sender: Any?) { sessionController.moveSelectedSlides(by: -1) }
    @objc func moveSlidesDown(_ sender: Any?) { sessionController.moveSelectedSlides(by: 1) }
    @objc func moveSlidesToTop(_ sender: Any?) { sessionController.moveSelectedSlides(toTop: true) }
    @objc func moveSlidesToBottom(_ sender: Any?) { sessionController.moveSelectedSlides(toTop: false) }

    @objc func newSlideAfter(_ sender: Any?) {
        insertSlide(layout: AppEnvironment.shared.lastLayout.name, after: sessionController.captureSelection())
    }

    @objc func copySlides(_ sender: Any?) {
        sessionController.copySlides(sessionController.selectedSlideNumbers, to: AppEnvironment.shared.slidePasteboard)
    }

    @objc func pasteSlides(_ sender: Any?) {
        sessionController.pasteSlides(from: AppEnvironment.shared.slidePasteboard, after: sessionController.captureSelection())
    }

    func windowWillClose(_ notification: Notification) {
        sidebarCollapseObservation?.invalidate()
        sidebarCollapseObservation = nil
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
        let count = sessionController.selectedSlideNumbers.count
        if menuItem.action == #selector(deleteSlides(_:)) {
            menuItem.title = count > 1 ? "Delete \(count) Slides" : "Delete Slide"
            return count > 0 && count < sessionController.editor.boxes.count
        }
        if menuItem.action == #selector(toggleSkipSlides(_:)) {
            menuItem.title = (selectionIsSkipped ? "Unskip" : "Skip") + (count > 1 ? " Slides" : " Slide")
            return count > 0
        }
        if [#selector(duplicateSlides(_:)), #selector(moveSlidesUp(_:)), #selector(moveSlidesDown(_:)), #selector(moveSlidesToTop(_:)),
            #selector(moveSlidesToBottom(_:)), #selector(newSlideAfter(_:))].contains(menuItem.action) {
            return count > 0
        }
        if menuItem.action == #selector(copySlides(_:)) { return count > 0 }
        if menuItem.action == #selector(pasteSlides(_:)) {
            // Type detection only: reading the pasteboard's contents here, on
            // every menu validation, is what raises the system's clipboard
            // privacy alert. A whitespace-only string then enables Paste, but
            // `pasteSlides(from:after:)` still refuses it, so nothing harmful
            // happens.
            let pasteboard = AppEnvironment.shared.slidePasteboard
            return pasteboard.availableType(from: [NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType), .string]) != nil
        }
        return true
    }

    func toolbarDefaultItemIdentifiers(_ toolbar: NSToolbar) -> [NSToolbarItem.Identifier] {
        [Self.slidesItemIdentifier, .flexibleSpace, Self.newSlideItemIdentifier, Self.previewItemIdentifier]
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
        if identifier == Self.newSlideItemIdentifier {
            let item = NSToolbarItem(itemIdentifier: identifier)
            item.label = "New Slide"
            item.toolTip = "New slide with the last layout. Hold for the layout gallery."
            newSlideButton.image = NSImage(systemSymbolName: "plus.rectangle.on.rectangle", accessibilityDescription: "New Slide")
            newSlideButton.bezelStyle = .toolbar
            newSlideButton.setAccessibilityIdentifier("new-slide-button")
            // VoiceOver's press and Full Keyboard Access go through target and action; the mouse through onClick and onHold.
            newSlideButton.target = self
            newSlideButton.action = #selector(newSlide(_:))
            newSlideButton.onClick = { [weak self] in self?.newSlide(nil) }
            newSlideButton.onHold = { [weak self] in self?.showLayoutGallery(nil) }
            item.view = newSlideButton
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
