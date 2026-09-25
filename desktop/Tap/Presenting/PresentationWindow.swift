import AppKit

/// A window for one of a talk's pages. It enters its own macOS full screen
/// Space on the display it is given (system full screen, as a browser's
/// is), so Cmd-Tab to another app, the menu bar at the top edge and the
/// page's own F key work as they do in a browser. It is windowed,
/// entering, in full screen or exiting. A move to another display goes
/// exit, move, enter. A take-down goes exit, close, and closes anyway
/// after `exitTimeout`, so no ending can leave a window in full screen.
/// On one display the presenter window is instead a child of the audience
/// window, riding in its Space (`attach(to:)`). Escape and Option-Tab are
/// the controller's key monitor's.
final class PresentationWindow: NSWindow, NSWindowDelegate {
    enum Role: Equatable {
        case audience
        case presenter
    }

    enum FullScreenState: Equatable {
        case windowed
        case entering
        case fullScreen
        case exiting
    }

    let role: Role
    let page: PresentationPageController
    /// Holds the page and, in the presenter window, the toolbar and the REC dot over it.
    let container = PresentationContentView()
    /// The presenter window's toolbar; nil in the audience window. Named
    /// `presenterToolbar`, not `toolbar`: `NSWindow` already declares a
    /// `toolbar: NSToolbar?` property, and a same-named property of a
    /// different type cannot coexist with it.
    private(set) var presenterToolbar: PresenterToolbar?
    /// The presenter window's REC dot; nil in the audience window.
    private(set) var recordingDot: RecordingDot?
    /// The pointer moved over this window.
    var onMouseMoved: (() -> Void)?
    /// The deck this window presents, for menu actions that reach this window first.
    weak var deckWindowController: DeckWindowController?
    private(set) var fullScreenState: FullScreenState = .windowed
    /// The screen frame this window is meant to fill: what the controller
    /// asked for, set before any transition, so a test can read it while
    /// the window server is still animating.
    private(set) var targetFrame: CGRect
    /// The frame the window last settled on, in full screen or as a plain
    /// window. A request for the same frame is already satisfied: in full
    /// screen the window's own frame is the whole display, so it cannot
    /// tell a half-screen "display" from the display itself.
    private(set) var settledFrame: CGRect?
    private(set) var isClosed = false
    /// True from `takeDown` until the window has closed.
    var isTakingDown: Bool { closeWhenSettled && !isClosed }
    /// True while this window is not closed and is going down, in a full
    /// screen transition or in full screen by its style mask. A full
    /// screen entry asked for while another window is like this is
    /// dropped by AppKit with no notification, and that window never
    /// closes, so a new talk waits for every talk window to be quiet.
    var isBusyWithFullScreen: Bool {
        !isClosed && (isTakingDown || fullScreenState != .windowed || styleMask.contains(.fullScreen))
    }

    /// Whether any talk window, of any deck, is busy with full screen.
    @MainActor
    static var anyIsBusyWithFullScreen: Bool {
        NSApp.windows.contains { ($0 as? PresentationWindow)?.isBusyWithFullScreen == true }
    }
    /// True while this window is a child of another (the presenter view over the audience on one display).
    private(set) var isAttached = false
    var onFullScreenChange: ((FullScreenState) -> Void)?
    var onClosed: (() -> Void)?
    /// Asks AppKit to toggle full screen. A test replaces it with a no-op
    /// to stand for a transition that never completes, or with a recorder.
    var requestFullScreenToggle: (() -> Void)?
    /// How long an entry may take before the window is treated as
    /// windowed, and how long an exit may take before a take-down closes
    /// the window regardless.
    static let enterTimeout: TimeInterval = 5
    static let exitTimeout: TimeInterval = 3

    private var settled: (() -> Void)?
    private var closedCompletion: (() -> Void)?
    private var pendingFrame: CGRect?
    private var wantsFullScreen = true
    private var closeWhenSettled = false
    private var transitionDeadline: DispatchWorkItem?

    init(role: Role, screenFrame: CGRect) {
        self.role = role
        targetFrame = screenFrame
        page = PresentationPageController(accessibilityIdentifier: role == .audience ? "audience-page" : "presenter-page",
                                          dataStore: AppEnvironment.shared.presentationDataStore)
        // A titled window with its chrome hidden: AppKit puts a titled
        // window in a full screen Space and hides the title bar there.
        super.init(contentRect: screenFrame, styleMask: [.titled, .fullSizeContentView], backing: .buffered, defer: false)
        delegate = self
        titleVisibility = .hidden
        titlebarAppearsTransparent = true
        for kind in [.closeButton, .miniaturizeButton, .zoomButton] as [NSWindow.ButtonType] {
            standardWindowButton(kind)?.isHidden = true
        }
        isMovable = false
        level = .normal
        collectionBehavior = [.fullScreenPrimary, .fullScreenDisallowsTiling, .ignoresCycle]
        isReleasedWhenClosed = false
        hasShadow = false
        animationBehavior = .none
        backgroundColor = .black
        isOpaque = true
        acceptsMouseMovedEvents = true
        setAccessibilityIdentifier(role == .audience ? "audience-window" : "presenter-window")
        page.view.translatesAutoresizingMaskIntoConstraints = false
        container.addSubview(page.view)
        NSLayoutConstraint.activate([
            page.view.topAnchor.constraint(equalTo: container.topAnchor),
            page.view.leadingAnchor.constraint(equalTo: container.leadingAnchor),
            page.view.trailingAnchor.constraint(equalTo: container.trailingAnchor),
            page.view.bottomAnchor.constraint(equalTo: container.bottomAnchor),
        ])
        contentView = container
        if role == .presenter {
            let toolbar = PresenterToolbar(frame: .zero)
            let dot = RecordingDot(frame: .zero)
            for view in [toolbar, dot] as [NSView] {
                view.translatesAutoresizingMaskIntoConstraints = false
                container.addSubview(view)
            }
            NSLayoutConstraint.activate([
                toolbar.bottomAnchor.constraint(equalTo: container.bottomAnchor),
                toolbar.leadingAnchor.constraint(equalTo: container.leadingAnchor),
                toolbar.trailingAnchor.constraint(equalTo: container.trailingAnchor),
                toolbar.heightAnchor.constraint(equalToConstant: PresenterToolbar.height),
                dot.topAnchor.constraint(equalTo: container.topAnchor, constant: 16),
                dot.trailingAnchor.constraint(equalTo: container.trailingAnchor, constant: -18),
            ])
            self.presenterToolbar = toolbar
            recordingDot = dot
        }
        // The bottom edge: the top is where full screen drops the menu bar.
        container.onMouseMoved = { [weak self] point in
            guard let self else { return }
            self.onMouseMoved?()
            guard let toolbar = self.presenterToolbar else { return }
            if point.y <= 2 {
                toolbar.pointerReachedBottomEdge()
            } else if point.y > PresenterToolbar.height {
                toolbar.pointerLeft()
            }
        }
        // The pointer left the window (onto the other display, or the mouse was put down for a clicker): the toolbar goes too.
        container.onMouseExited = { [weak self] in self?.presenterToolbar?.pointerLeft() }
        container.layoutSubtreeIfNeeded()
    }

    override var canBecomeKey: Bool { true }
    override var canBecomeMain: Bool { true }

    /// View > Enter Full Screen (Ctrl+Cmd+F) never reaches a talk window:
    /// the controller owns its full screen, and a window taken out from
    /// under it would stay a plain window.
    override func validateUserInterfaceItem(_ item: NSValidatedUserInterfaceItem) -> Bool {
        if item.action == #selector(NSWindow.toggleFullScreen(_:)) { return false }
        return super.validateUserInterfaceItem(item)
    }

    // MARK: Full screen

    /// Fills `frame`, a screen's frame, in that screen's own full screen
    /// Space, and calls `completion` once the window is settled there (or
    /// has failed to enter and stays a plain window over the frame). From
    /// full screen on another frame the window exits, moves and enters
    /// again; the frame it settled on last is already satisfied. During a
    /// transition the frame waits for it to end. With `fullScreen` false
    /// the window is a plain window over the frame: what a two-display
    /// talk gets while "Displays have separate Spaces" is off, since one
    /// full screen Space would black out the other display, and what a
    /// test host without full screen gets.
    func present(on frame: CGRect, fullScreen: Bool = true, completion: @escaping () -> Void = {}) {
        guard !isClosed else { return completion() }
        targetFrame = frame
        wantsFullScreen = fullScreen
        settled = completion
        switch fullScreenState {
        case .windowed:
            setFrame(frame, display: true)
            orderFrontRegardless()
            if wantsFullScreen {
                enterFullScreen()
            } else {
                settledFrame = frame
                settle()
            }
        case .fullScreen:
            if wantsFullScreen, settledFrame == frame {
                settle()
            } else {
                pendingFrame = frame
                exitFullScreen()
            }
        case .entering, .exiting:
            pendingFrame = frame
        }
    }

    /// Leaves full screen and closes, then calls `completion`. A
    /// transition in flight finishes first; one that never finishes is cut
    /// short by the deadline, and the window is closed in whatever state
    /// it is in, which drops its Space. A window with children lets them
    /// go first.
    func takeDown(completion: @escaping () -> Void = {}) {
        guard !isClosed else { return completion() }
        for child in childWindows?.compactMap({ $0 as? PresentationWindow }) ?? [] {
            child.detach()
            child.takeDown()
        }
        settled = nil
        pendingFrame = nil
        closedCompletion = completion
        closeWhenSettled = true
        switch fullScreenState {
        case .windowed:
            finishClose()
        case .fullScreen:
            exitFullScreen()
        case .entering, .exiting:
            armDeadline(Self.exitTimeout)
        }
    }

    /// Makes this window a child of `parent`, covering it, in its Space:
    /// the presenter view over the audience on one display. A child may
    /// not have a Space of its own, so the collection behaviour changes
    /// with it and changes back on `detach`.
    func attach(to parent: NSWindow) {
        guard !isClosed, !isAttached, fullScreenState == .windowed else { return }
        isAttached = true
        collectionBehavior = [.fullScreenAuxiliary, .ignoresCycle]
        setFrame(parent.frame, display: true)
        parent.addChildWindow(self, ordered: .above)
    }

    /// Takes the window out of its parent and off the screen.
    func detach() {
        guard isAttached else { return }
        isAttached = false
        parent?.removeChildWindow(self)
        orderOut(nil)
        collectionBehavior = [.fullScreenPrimary, .fullScreenDisallowsTiling, .ignoresCycle]
    }

    private func enterFullScreen() {
        fullScreenState = .entering
        onFullScreenChange?(.entering)
        armDeadline(Self.enterTimeout)
        toggle()
    }

    private func exitFullScreen() {
        fullScreenState = .exiting
        onFullScreenChange?(.exiting)
        armDeadline(Self.exitTimeout)
        toggle()
    }

    private func toggle() {
        if let requestFullScreenToggle {
            requestFullScreenToggle()
        } else {
            toggleFullScreen(nil)
        }
    }

    private func armDeadline(_ seconds: TimeInterval) {
        transitionDeadline?.cancel()
        let work = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated { self?.transitionTimedOut() }
        }
        transitionDeadline = work
        DispatchQueue.main.asyncAfter(deadline: .now() + seconds, execute: work)
    }

    private func clearDeadline() {
        transitionDeadline?.cancel()
        transitionDeadline = nil
    }

    /// AppKit never finished the transition. The state follows the style
    /// mask, a take-down closes the window as it is, and a presentation
    /// goes on with the window as it is.
    private func transitionTimedOut() {
        transitionDeadline = nil
        fullScreenState = styleMask.contains(.fullScreen) ? .fullScreen : .windowed
        onFullScreenChange?(fullScreenState)
        if closeWhenSettled {
            finishClose()
            return
        }
        pendingFrame = nil
        settledFrame = targetFrame
        let completion = settled
        settled = nil
        completion?()
    }

    /// A transition finished. A queued close runs, then a queued move,
    /// then the caller's completion.
    private func settle() {
        if closeWhenSettled {
            if fullScreenState == .fullScreen {
                exitFullScreen()
            } else {
                finishClose()
            }
            return
        }
        if let frame = pendingFrame {
            pendingFrame = nil
            if fullScreenState == .fullScreen, !wantsFullScreen || settledFrame != frame {
                targetFrame = frame
                pendingFrame = frame
                exitFullScreen()
                return
            }
            if fullScreenState == .windowed {
                targetFrame = frame
                setFrame(frame, display: true)
                orderFrontRegardless()
                if wantsFullScreen {
                    enterFullScreen()
                    return
                }
            }
        }
        settledFrame = targetFrame
        let completion = settled
        settled = nil
        completion?()
    }

    private func finishClose() {
        clearDeadline()
        close()
    }

    // MARK: NSWindowDelegate

    func windowDidEnterFullScreen(_ notification: Notification) {
        clearDeadline()
        fullScreenState = .fullScreen
        onFullScreenChange?(.fullScreen)
        settle()
    }

    func windowDidFailToEnterFullScreen(_ window: NSWindow) {
        clearDeadline()
        fullScreenState = .windowed
        onFullScreenChange?(.windowed)
        pendingFrame = nil
        settle()
    }

    func windowDidExitFullScreen(_ notification: Notification) {
        clearDeadline()
        fullScreenState = .windowed
        onFullScreenChange?(.windowed)
        settle()
    }

    func windowDidFailToExitFullScreen(_ window: NSWindow) {
        clearDeadline()
        fullScreenState = .fullScreen
        onFullScreenChange?(.fullScreen)
        pendingFrame = nil
        if closeWhenSettled {
            finishClose()
            return
        }
        settle()
    }

    /// In full screen the menu bar and the Dock stay out of the way until
    /// the pointer asks for them, as in a browser's full screen.
    func window(_ window: NSWindow, willUseFullScreenPresentationOptions proposedOptions: NSApplication.PresentationOptions) -> NSApplication.PresentationOptions {
        [.fullScreen, .autoHideMenuBar, .autoHideDock]
    }

    func windowWillClose(_ notification: Notification) {
        isClosed = true
        clearDeadline()
        if isAttached {
            isAttached = false
            parent?.removeChildWindow(self)
        }
        let completion = closedCompletion
        closedCompletion = nil
        onClosed?()
        completion?()
    }

    /// Menu actions this window cannot answer go to the deck's window
    /// controller: Stop, Reload Slides, Swap Displays and Tap Log work
    /// while a presentation window is key.
    override func supplementalTarget(forAction action: Selector, sender: Any?) -> Any? {
        if let deckWindowController, deckWindowController.responds(to: action) { return deckWindowController }
        return super.supplementalTarget(forAction: action, sender: sender)
    }
}

/// The talk window's content view: it tracks the pointer everywhere in the
/// window, so the toolbar can slide up at the bottom edge and the cursor can
/// hide when idle.
final class PresentationContentView: NSView {
    var onMouseMoved: ((NSPoint) -> Void)?
    var onMouseExited: (() -> Void)?
    private var trackingArea: NSTrackingArea?

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let trackingArea { removeTrackingArea(trackingArea) }
        let area = NSTrackingArea(rect: bounds, options: [.mouseMoved, .mouseEnteredAndExited, .activeAlways, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        trackingArea = area
    }

    override func mouseMoved(with event: NSEvent) {
        onMouseMoved?(convert(event.locationInWindow, from: nil))
    }

    override func mouseExited(with event: NSEvent) {
        onMouseExited?()
    }
}
