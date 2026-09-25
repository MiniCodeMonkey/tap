import AppKit

/// A deck's window: the editor on the left and the Preview pane on the right,
/// under a unified toolbar. Deck windows open as tabs of each other.
final class DeckWindowController: NSWindowController, NSWindowDelegate, NSToolbarDelegate, NSMenuItemValidation {
    static let slidesItemIdentifier = NSToolbarItem.Identifier("slides")
    static let newSlideItemIdentifier = NSToolbarItem.Identifier("newSlide")
    static let playItemIdentifier = NSToolbarItem.Identifier("play")
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
    /// The toolbar's Play button: a click opens the Present popover, a Shift-click starts from slide 1.
    let playButton = NSButton()
    /// The popover, whose controls are the last settings: loaded once from
    /// the environment, saved on every start.
    private(set) lazy var presentPopover: PresentPopoverController = {
        let popover = PresentPopoverController()
        popover.loadSettings(AppEnvironment.shared.presentationSettings.settings)
        popover.onSwap = { [weak self] in
            guard let self else { return }
            self.sessionController.presentation.swapDisplays()
            self.presentPopover.update(context: self.popoverContext())
        }
        popover.onStart = { [weak self] options in self?.startPresenting(options, savingSettings: true) }
        popover.onRehearse = { [weak self] options in self?.startPresenting(options, savingSettings: true) }
        return popover
    }()
    /// The sheet for tap's question, while it is up.
    private(set) var questionSheet: QuestionSheet?
    /// Reveals a kept recording. Production opens Finder on it; a test records the URL.
    var revealInFinder: (URL) -> Void = { url in NSWorkspace.shared.activateFileViewerSelecting([url]) }
    /// The phone remote panel, made with the window (a panel that is never
    /// shown costs nothing) and closed with it, so none outlives its deck.
    let remotePanel = RemotePanel()
    private(set) lazy var layoutGallery: LayoutGalleryController = {
        let gallery = LayoutGalleryController()
        gallery.onPick = { [weak self] name in self?.insertSlide(layout: name, after: .caret) }
        return gallery
    }()
    private var sidebarCollapseObservation: NSKeyValueObservation?
    /// Hears every deck's talks start and end, so the Play button follows them.
    private var presentingObserver: NSObjectProtocol?
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
        sessionController.presentation.onStateChange = { [weak self] state in
            self?.refreshPresentingControls()
            switch state {
            case .idle, .failed: self?.talkEnded()
            case .starting, .presenting, .stopping: break
            }
        }
        sessionController.presentation.onQuestion = { [weak self] question in self?.presentQuestion(question) }
        remotePanel.onTurnOff = { [weak self] in self?.sessionController.presentation.setTunnel(on: false) }
        sessionController.presentation.onTunnelChange = { [weak self] in self?.refreshRemotePanel() }
        presentingObserver = NotificationCenter.default.addObserver(forName: AppEnvironment.presentingDidChangeNotification, object: nil, queue: nil) { [weak self] _ in
            MainActor.assumeIsolated { self?.refreshPresentingControls() }
        }

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

    // MARK: Presenting

    /// Present > Play, Cmd+Option+P: the talk starts at once with the last
    /// settings (the popover's controls), from the cursor's slide unless
    /// those settings say slide 1.
    @objc func play(_ sender: Any?) {
        guard sessionController.presentation.canStart else { return }
        startPresenting(freshPopover().options(mode: .play))
    }

    /// Present > Play with Options: the popover, anchored on the Play button.
    @objc func playWithOptions(_ sender: Any?) {
        guard sessionController.presentation.canStart else { return }
        let anchor: NSView = playButton.window == nil ? (window?.contentView ?? playButton) : playButton
        freshPopover().show(context: popoverContext(), relativeTo: anchor.bounds, of: anchor)
    }

    /// The toolbar's Play button: the popover, or with Shift a start from slide 1 at once.
    @objc func playButtonPressed(_ sender: Any?) {
        playButtonClicked(modifiers: NSApp.currentEvent?.modifierFlags ?? [])
    }

    func playButtonClicked(modifiers: NSEvent.ModifierFlags) {
        guard sessionController.presentation.canStart else { return }
        if modifiers.contains(.shift) {
            var options = freshPopover().options(mode: .play)
            options.startSlide = 1
            startPresenting(options)
            return
        }
        playWithOptions(nil)
    }

    /// The popover with the settings as they are now and the cursor and
    /// displays as they are now: the settings are app-wide and another
    /// deck may have saved newer ones, and the cursor moved since the
    /// popover was last shown. The password field is left alone.
    private func freshPopover() -> PresentPopoverController {
        presentPopover.loadSettings(AppEnvironment.shared.presentationSettings.settings)
        presentPopover.update(context: popoverContext())
        return presentPopover
    }

    /// Present > Rehearse: the presenter view alone, from the cursor's slide.
    @objc func rehearse(_ sender: Any?) {
        guard sessionController.presentation.canStart else { return }
        startPresenting(PresentationOptions(mode: .rehearse, startSlide: sessionController.currentSlideNumber ?? 1))
    }

    /// Every start comes here: the popover's buttons, Play, the Shift-click
    /// and Rehearse. A start from the popover saves its settings, so the
    /// next Cmd+Option+P and the next launch start the same way; the other
    /// starts read the saved settings and leave them as they are.
    func startPresenting(_ options: PresentationOptions, savingSettings: Bool = false) {
        if savingSettings { AppEnvironment.shared.presentationSettings.settings = presentPopover.settings }
        // Play with the popover open starts at once; the popover goes, so a later click on its Start cannot save settings for a talk it did not start.
        if presentPopover.isShown { presentPopover.close() }
        sessionController.presentation.start(options)
        refreshPresentingControls()
    }

    func popoverContext() -> PresentPopoverController.Context {
        let presentation = sessionController.presentation
        return PresentPopoverController.Context(arrangement: presentation.currentArrangement,
                                                cursorSlide: sessionController.currentSlideNumber ?? 1,
                                                usesFullScreen: presentation.usesFullScreen)
    }

    /// The toolbar's Play button follows the talk: off while one runs.
    func refreshPresentingControls() {
        playButton.isEnabled = sessionController.presentation.canStart
    }

    /// Present > Stop, the toolbar's Stop, and Escape in the audience window.
    @objc func stopPresenting(_ sender: Any?) {
        sessionController.presentation.stop()
    }

    /// Present > Swap Displays and the toolbar's: during the talk the
    /// windows change places; before it the popover's arrangement does.
    @objc func swapDisplays(_ sender: Any?) {
        sessionController.presentation.swapDisplays()
        if presentPopover.isShown { presentPopover.update(context: popoverContext()) }
    }

    /// Present > Reload Slides and the toolbar's.
    @objc func reloadSlides(_ sender: Any?) {
        sessionController.presentation.reloadSlides()
    }

    /// The talk is idle or failed: no sheet of its outlives it. A
    /// keep-recording sheet still up means tap exited before an answer
    /// came: its 60 s wait ran out (or the app's stdin closed) and it kept
    /// the recording, so the sheet ends as a yes and the run is revealed
    /// like any kept run. Any other sheet ends answering nothing.
    func talkEnded() {
        guard let sheet = questionSheet else { return }
        if sheet.kind == "keep-recording" {
            // The talk's own log, which outlives its session; never tap dev's.
            sessionController.presentationIfCreated?.lastTalkLog?.append("tap kept the recording before an answer came", source: .app)
            endQuestionSheet(as: .OK)
        } else {
            endQuestionSheet(as: .abort)
        }
    }

    // MARK: The phone remote

    /// Present > Phone Remote: the tunnel on or off.
    @objc func togglePhoneRemote(_ sender: Any?) {
        let presentation = sessionController.presentation
        presentation.setTunnel(on: presentation.tunnel?.state != "running")
    }

    /// The panel follows tap's tunnel: shown with the QR code while the
    /// tunnel runs or starts, shown with the reason when tap could not
    /// start it, gone when it stops or the talk ends.
    func refreshRemotePanel() {
        let presentation = sessionController.presentation
        let running = presentation.tunnel?.state == "running" || presentation.tunnel?.state == "starting"
        guard presentation.isActive, running || presentation.tunnelError != nil else {
            remotePanel.hide()
            return
        }
        let screenFrame = presentation.presenterWindow?.targetFrame ?? window?.screen?.frame ?? NSScreen.screens[0].frame
        remotePanel.show(tunnel: presentation.tunnel, error: presentation.tunnelError,
                         ownPassword: presentation.options?.presenterPassword != nil, on: screenFrame)
    }

    // MARK: tap's questions

    /// tap asked something. Consent and keep-recording become sheets on
    /// this window; the live code approval is D5's and is declined until
    /// then, which runs no code.
    func presentQuestion(_ question: PresentationController.PendingQuestion) {
        let presentation = sessionController.presentation
        switch question.kind {
        case "record-consent":
            showQuestionSheet(QuestionSheet.consent(settingsPath: question.payload.settingsPath)) { record in
                presentation.answer(id: question.id, value: record)
            }
        case "keep-recording":
            let directory = question.payload.directory ?? ""
            let sheet = QuestionSheet.keepRecording(directory: directory, segments: question.payload.segments ?? 0,
                                                    size: Self.folderSize(at: URL(fileURLWithPath: directory)))
            showQuestionSheet(sheet) { [weak self] keep in
                // tap keeps the recording on a yes and on no answer at all; the app
                // never touches the folder itself.
                presentation.answer(id: question.id, value: keep)
                if keep, !directory.isEmpty { self?.revealInFinder(URL(fileURLWithPath: directory)) }
            }
        default:
            presentation.session?.log.append("the \(question.kind) question is not answered by this version of the app; declined", source: .app)
            presentation.answer(id: question.id, value: false)
        }
    }

    /// Puts `sheet` on this window and calls back with the answer. This
    /// window comes forward, which switches to its Space and leaves the
    /// talk's Spaces where they are: the sheet is the one thing the person
    /// must answer, so this is the one focus move outside the talk windows.
    /// After the answer the talk's front window is made key again, which
    /// switches back.
    func showQuestionSheet(_ sheet: QuestionSheet, completion: @escaping (Bool) -> Void) {
        guard let window else {
            completion(sheet.kind == "keep-recording")
            return
        }
        let presentation = sessionController.presentation
        questionSheet = sheet
        window.makeKeyAndOrderFront(nil)
        window.beginSheet(sheet) { [weak self] response in
            // Only the sheet that completed clears the slot: a stale sheet
            // ended late must not clear a newer one.
            if self?.questionSheet === sheet { self?.questionSheet = nil }
            completion(response == .OK)
            presentation.returnToTalk()
        }
    }

    /// The size of every file under `folder`, formatted for a person.
    static func folderSize(at folder: URL) -> String {
        var bytes: Int64 = 0
        if let files = FileManager.default.enumerator(at: folder, includingPropertiesForKeys: [.fileSizeKey]) {
            for case let file as URL in files {
                bytes += Int64((try? file.resourceValues(forKeys: [.fileSizeKey]))?.fileSize ?? 0)
            }
        }
        return ByteCountFormatter.string(fromByteCount: bytes, countStyle: .file)
    }

    /// Ends the sheet that is up, if any, as `response`. The talk ending
    /// calls this so no sheet outlives the talk that asked.
    func endQuestionSheet(as response: NSApplication.ModalResponse) {
        guard let sheet = questionSheet, let window else { return }
        window.endSheet(sheet, returnCode: response)
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
        if let presentingObserver { NotificationCenter.default.removeObserver(presentingObserver) }
        presentingObserver = nil
        if let controller = previewWindowController {
            controller.onClose = nil
            previewWindowController = nil
            controller.close()
        }
        remotePanel.orderOut(nil)
        remotePanel.close()
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
        let presentation = sessionController.presentation
        if [#selector(play(_:)), #selector(playWithOptions(_:)), #selector(rehearse(_:))].contains(menuItem.action) { return presentation.canStart }
        if menuItem.action == #selector(stopPresenting(_:)) { return presentation.isActive }
        if menuItem.action == #selector(togglePhoneRemote(_:)) {
            menuItem.state = presentation.tunnel?.state == "running" ? .on : .off
            return presentation.isActive
        }
        if menuItem.action == #selector(reloadSlides(_:)) { return presentation.state == .presenting }
        if menuItem.action == #selector(swapDisplays(_:)) {
            // While another deck presents, a swap here would change the remembered pair under its running talk.
            return presentation.currentArrangement?.isSingleDisplay == false && (presentation.isActive || !AppEnvironment.shared.isPresenting)
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
        [Self.slidesItemIdentifier, .flexibleSpace, Self.newSlideItemIdentifier, Self.playItemIdentifier, Self.previewItemIdentifier]
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
        if identifier == Self.playItemIdentifier {
            let item = NSToolbarItem(itemIdentifier: identifier)
            item.label = "Play"
            item.toolTip = "Present: choose displays and options. Shift-click to start from slide 1. Cmd+Option+P starts at once."
            playButton.image = NSImage(systemSymbolName: "play.fill", accessibilityDescription: "Play")
            playButton.bezelStyle = .toolbar
            playButton.setAccessibilityIdentifier("play-button")
            playButton.target = self
            playButton.action = #selector(playButtonPressed(_:))
            playButton.isEnabled = sessionController.presentation.canStart
            item.view = playButton
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
