import AppKit
import WebKit

/// One deck's talk: the `tap present --app` process beside the deck's own
/// `tap dev --app`, the audience and presenter windows, the display sleep
/// assertion, and the slide the audience is on. tap dev keeps running the
/// preview the whole time; nothing here touches it.
///
/// On two displays each window has its own full screen Space. On one
/// display the audience window has the Space and the presenter window is
/// its child, shown over it by Option-Tab and the S key and hidden again
/// by Option-Tab, with no Space switch.
///
/// The state moves idle, starting (the save, the process, the ready line,
/// the windows loading), presenting (the windows are up), stopping (the
/// windows are down and tap is quitting, which may take a keep-recording
/// answer), and back to idle; or to failed, with a message for the
/// person, when tap present cannot start or stops restarting. A talk is
/// counted in `AppEnvironment.presentingCount` from start to idle or
/// failed, and a talk whose deck has closed is kept alive by
/// `AppEnvironment.endingTalks` until its process has exited and its
/// windows are down.
@MainActor
final class PresentationController {
    enum State: Equatable {
        case idle
        case starting
        case presenting
        case stopping
        case failed(String)
    }

    struct PendingQuestion: Equatable {
        let id: String
        let kind: String
        let payload: QuestionPayload
    }

    private(set) var state: State = .idle {
        didSet { if state != oldValue { onStateChange?(state) } }
    }
    private(set) var options: PresentationOptions?
    private(set) var session: TapSession?
    private(set) var client: TapClient?
    private(set) var audienceWindow: PresentationWindow?
    private(set) var presenterWindow: PresentationWindow?
    /// The window the speaker's keys go to: the one whose Space is active
    /// on two displays, the one on top on one display.
    private(set) weak var frontWindow: PresentationWindow?
    /// The displays this talk runs on; nil between talks, so the popover
    /// always reads the displays that are connected now.
    private(set) var arrangement: DisplayArrangement?
    let sleepAssertion = SleepAssertion()
    /// The slide the audience is on, 1-based: the start slide until tap's
    /// first slide event, then the last event's slide.
    private(set) var lastSlide = 1
    /// The log of the last talk, kept after its session is gone, so Window
    /// > Tap Log still shows it and the deck window can append to it.
    private(set) var lastTalkLog: TapLog?
    private(set) var recording = RecordingStatus()
    /// True once the windows have been asked to show for this talk.
    private(set) var windowsShown = false
    private var recordingTimer: Timer?
    /// tap's last tunnel event, and the last tunnel error, for the remote panel.
    private(set) var tunnel: TunnelEvent?
    private(set) var tunnelError: String?
    /// Whether the person wants the phone remote now: the Present option
    /// at the start, then Present > Phone Remote and Turn Off Remote. A
    /// restarted tap has no tunnel, and is asked again from this.
    private(set) var wantsRemote = false
    var onTunnelChange: (() -> Void)?
    /// True once this talk's windows were shown at all, so an ending moves
    /// the editor's cursor only for a talk the person actually saw.
    private var windowsWereShown = false
    /// tap's questions in the order they came; the first is the one on
    /// screen. tap asks one at a time at startup, but a later question can
    /// arrive on top of a sheet that is still up (D5's approval).
    private(set) var pendingQuestions: [PendingQuestion] = []
    var pendingQuestion: PendingQuestion? { pendingQuestions.first }
    /// Set once a page reported ready or failed to load, or the fallback
    /// fired: the windows may be shown as soon as no question is pending.
    private var pagesReported = false
    /// How many typing pauses tap dev has answered for since tap present
    /// last read the file, so the toolbar can say what the audience has not
    /// seen. Zero again when the text equals what was presented.
    private(set) var editsNotShown = 0
    /// The deck text tap present last read: at the start and after Reload Slides.
    var presentedText: String?
    /// The text tap dev last answered for, so an answer for the same text is not counted twice.
    private var lastCountedText: String?
    /// How long the pointer rests on a talk window before the cursor hides.
    var cursorHideDelay: TimeInterval = 3
    /// Hides the cursor until the mouse moves. A test replaces it.
    var hideCursor: () -> Void = { NSCursor.setHiddenUntilMouseMoves(true) }
    private var cursorHideWork: DispatchWorkItem?
    var isCursorHideArmed: Bool { cursorHideWork != nil }
    private var showWindowsFallback: DispatchWorkItem?
    /// Windows taking themselves down, one at a time, the front one first:
    /// two exits at once fail the same way two entries do. Each is kept
    /// until it has closed.
    private(set) var windowsGoingDown: [PresentationWindow] = []
    private var takingDown = false
    /// The windows still to be put on their displays, one at a time.
    private var placements: [(window: PresentationWindow, frame: CGRect, fullScreen: Bool)] = []
    private var placing = false
    private var placementCompletion: (() -> Void)?
    /// True while the first placement waits for other talk windows to be quiet.
    private var waitingForQuiet = false
    /// How long the first placement waits for quiet, and how long after it.
    static let quietTimeout: TimeInterval = 10
    static let quietPeriod: TimeInterval = 0.5
    /// Installed while the talk's windows exist, removed with them, so a
    /// finished talk never hears about displays and nothing is read in deinit.
    private var screenObserver: NSObjectProtocol?
    private var keyMonitor: Any?
    /// True while the talk's windows are up: Escape and Option-Tab reach
    /// `handleKey` only through the monitor.
    var isKeyMonitorInstalled: Bool { keyMonitor != nil }
    /// Each talk window as it is made, before it is placed. A test sets
    /// its full screen toggle here to drive the transitions through the seams.
    var windowCreated: ((PresentationWindow) -> Void)?
    /// The screens changed while the windows exist. A test counts the calls.
    var onScreensChanged: (() -> Void)?
    /// The remembered port tap said is taken; the next attempt asks for none.
    private var takenPort: Int?
    private var portFallbackPending = false
    /// Counts starts, so a save that completes for a start that was
    /// stopped in the meantime launches nothing.
    private var startGeneration = 0
    /// True while this talk is counted in `AppEnvironment.presentingCount`.
    /// Read in deinit, which is not on the main actor, as a last guard.
    nonisolated(unsafe) private var countedAsPresenting = false

    var onStateChange: ((State) -> Void)?
    var onEvent: ((TapEvent) -> Void)?
    /// The talk ended, by Stop or a failure, after its windows had shown;
    /// this is the last slide the audience saw.
    var onStopped: ((_ lastSlide: Int) -> Void)?
    var onRecordingChange: ((RecordingStatus) -> Void)?
    var onQuestion: ((PendingQuestion) -> Void)?
    /// tap present could not start or stopped restarting.
    var onFailed: ((String) -> Void)?

    /// The deck file tap present reads: nil while the deck has no file.
    let deckURL: () -> URL?
    /// Writes the buffer to the deck file, then calls back. tap present
    /// reads the file, so this runs before the process starts. A test can
    /// wrap it.
    var saveDeck: (@escaping (Error?) -> Void) -> Void
    let sessionConfiguration: () -> TapSession.Configuration
    var displayAssignments: DisplayAssignmentStore
    var deckPorts: DeckPortStore
    /// The displays. Production reads NSScreen; tests hand in frames of their own.
    var screens: () -> [ScreenInfo] = { NSScreen.screens.map(ScreenInfo.init(screen:)) }
    /// Whether each display has its own Spaces (System Settings > Desktop &
    /// Dock). Without it, one full screen Space blacks out every other
    /// display, so a two-display talk cannot use full screen at all.
    var screensHaveSeparateSpaces: () -> Bool = { NSScreen.screensHaveSeparateSpaces }
    /// Whether this host may use system full screen at all. Production
    /// always may; a hosted test on a host the probe found unable says no,
    /// and the talk runs as plain windows over the displays.
    var fullScreenAllowed: () -> Bool = { true }
    /// Trades the presenter secret for the hub's cookie. A test replaces it
    /// to hold the exchange open while it presses Stop.
    var authorizePresenter: (TapClient) async throws -> String = { try await $0.authorizePresenter() }
    /// The deck's window controller, which the talk's windows forward menu actions to.
    weak var deckWindowController: DeckWindowController?
    /// How long the windows wait for the first page to report before they
    /// are shown anyway: a page that never reports (no server, a load
    /// failure) must not keep the talk from starting.
    static let showWindowsFallbackInterval: TimeInterval = 3
    /// How long quit waits for a tap that got ready (its own quit deadline
    /// is 8 s), and for one that never did.
    static let quitTimeout: TimeInterval = 15
    static let quitTimeoutBeforeReady: TimeInterval = 2
    /// Once tap has asked keep-recording it waits for the answer while
    /// stdin is open, up to 60 s. The deadline must outlast that wait, or
    /// closing stdin would answer the person's question for them.
    static let quitTimeoutWithRecording: TimeInterval = 75

    init(deckURL: @escaping () -> URL?,
         saveDeck: @escaping (@escaping (Error?) -> Void) -> Void,
         sessionConfiguration: @escaping () -> TapSession.Configuration,
         displayAssignments: DisplayAssignmentStore,
         deckPorts: DeckPortStore) {
        self.deckURL = deckURL
        self.saveDeck = saveDeck
        self.sessionConfiguration = sessionConfiguration
        self.displayAssignments = displayAssignments
        self.deckPorts = deckPorts
    }

    deinit {
        // A talk that is freed while counted (which retainEndingTalk is
        // there to prevent) must not keep Play off in every deck forever.
        if countedAsPresenting {
            countedAsPresenting = false
            MainActor.assumeIsolated { AppEnvironment.shared.noteTalkEnded() }
        }
    }

    /// True from Play until the talk is idle or failed again.
    var isActive: Bool {
        switch state {
        case .starting, .presenting, .stopping: return true
        case .idle, .failed: return false
        }
    }

    /// True while the talk still has work of its own: a process to quit,
    /// or windows going down.
    var isEnding: Bool { isActive || !windowsGoingDown.isEmpty }

    /// Play and Rehearse need a deck file, no talk in progress, in this
    /// deck or any other, and the last talk's windows gone and out of
    /// full screen: an entry asked for during another window's exit is
    /// dropped by AppKit, and that window then never closes. The Play
    /// button hears `presentingDidChangeNotification` when this turns true.
    var canStart: Bool {
        deckURL() != nil && !isActive && !AppEnvironment.shared.isPresenting
            && windowsGoingDown.isEmpty && !PresentationWindow.anyIsBusyWithFullScreen
    }

    /// The arrangement the next talk would use, for the popover; the
    /// running talk's while one runs.
    var currentArrangement: DisplayArrangement? {
        arrangement ?? DisplayArrangement.resolve(screens: screens(), store: displayAssignments)
    }

    /// Whether the audience window (and on two displays the presenter
    /// window) goes to full screen: when the host allows it, and on two
    /// displays only when each display has its own Spaces. Reads the
    /// displays connected now, not a running talk's.
    var usesFullScreen: Bool {
        guard fullScreenAllowed() else { return false }
        let connected = DisplayArrangement.resolve(screens: screens(), store: displayAssignments)
        return (connected?.isSingleDisplay ?? true) || screensHaveSeparateSpaces()
    }

    /// True once every window is where it was asked to be.
    var windowsAreSettled: Bool { placements.isEmpty && !placing && !waitingForQuiet }

    /// On one display: the presenter view is over the audience view.
    var presenterIsShownOverAudience: Bool {
        guard let presenterWindow else { return false }
        return presenterWindow.isAttached && presenterWindow.isVisible
    }

    // MARK: Start

    func start(_ options: PresentationOptions) {
        guard canStart, let deck = deckURL() else { return }
        self.options = options
        lastSlide = options.startSlide
        recording = RecordingStatus()
        editsNotShown = 0
        tunnel = nil
        tunnelError = nil
        wantsRemote = options.wantsTunnel
        lastCountedText = nil
        pendingQuestions = []
        pagesReported = false
        windowsShown = false
        windowsWereShown = false
        takenPort = nil
        portFallbackPending = false
        startGeneration += 1
        let generation = startGeneration
        state = .starting
        countIn()
        saveDeck { [weak self] error in
            guard let self, self.state == .starting, self.startGeneration == generation else { return }
            if let error {
                self.fail(Self.saveFailureMessage(for: error))
                return
            }
            self.launch(deck: deck, options: options, port: self.deckPorts.port(for: deck) ?? DeckPortStore.suggestedPort(for: deck))
        }
    }

    /// What a refused save means to the person. The document answers
    /// userCancelled while a disk conflict is showing, which says nothing
    /// on its own.
    static func saveFailureMessage(for error: Error) -> String {
        if (error as? CocoaError)?.code == .userCancelled {
            return "The deck could not be saved: resolve the change on disk first."
        }
        return "The deck could not be saved: \(error.localizedDescription)"
    }

    private func launch(deck: URL, options: PresentationOptions, port: Int?) {
        let session = TapSession(deckURL: deck, configuration: sessionConfiguration(), command: options.command(port: port))
        // A replaced session (the port fallback) may still report; only the current one is heard.
        session.onStateChange = { [weak self, weak session] sessionState in
            guard let self, let session, session === self.session else { return }
            self.sessionStateChanged(sessionState)
        }
        session.onEvent = { [weak self, weak session] event in
            guard let self, let session, session === self.session else { return }
            self.handle(event)
        }
        self.session = session
        lastTalkLog = session.log
        session.start()
    }

    private func sessionStateChanged(_ sessionState: TapSession.State) {
        switch sessionState {
        case .running(let ready):
            tapIsReady(ready)
        case .failed(let lastOutput):
            endBecauseTapFailed(lastOutput: lastOutput)
        case .stopped:
            if state == .stopping {
                finishStopping()
            } else if portFallbackPending {
                relaunchOnAFreePort()
            }
        case .restarting:
            // The port attempt exited before its error line was read: stop
            // the restart, which lands in .stopped and relaunches.
            if portFallbackPending { session?.stop() }
        case .starting:
            break
        }
    }

    /// tap said the remembered port is taken: its error event, code
    /// "failed", "port n is already in use (another tap dev may be
    /// running); pass --port <other>" (present runs the dev server, which
    /// names itself). At the start, or on a restart mid-talk when
    /// something grabbed the port in between, the session is stopped
    /// before D2's policy can restart it on the same port, and a new one
    /// starts with no port. That talk's presenter layout starts fresh,
    /// since the page's origin changed; the next talk remembers the new port.
    private func portIsTaken() {
        guard state == .starting || state == .presenting, let session, let port = session.command.port, !portFallbackPending else { return }
        portFallbackPending = true
        takenPort = port
        session.stop()
    }

    private func relaunchOnAFreePort() {
        portFallbackPending = false
        guard state == .starting || state == .presenting, let options else { return }
        guard let deck = deckURL() else {
            // The session is stopped and nothing would start another: the
            // talk would stay up with no process until Stop.
            fail("The deck file is gone, so the talk cannot restart.")
            return
        }
        launch(deck: deck, options: options, port: nil)
        if let takenPort {
            session?.log.append("port \(takenPort) was taken; this talk runs on a new port, and the presenter layout starts fresh", source: .app)
        }
    }

    /// tap present printed its ready line, at the start or after a restart.
    /// The port is remembered for the deck. The presenter secret is traded
    /// for the hub's cookie first, so the audience page's own WebSocket
    /// connection is relayed: the speaker's keys in the audience window
    /// move the presenter view, and tap hears every position for its
    /// slide events and chapters. The presenter page brings its own key.
    /// A late exchange for a talk that has ended installs nothing.
    private func tapIsReady(_ ready: TapReady) {
        guard state == .starting || state == .presenting, let session else { return }
        deckPorts.setPort(ready.port, for: session.deckURL)
        let client = TapClient(ready: ready)
        self.client = client
        Task { @MainActor [weak self] in
            var cookie: String?
            do {
                cookie = try await self?.authorizePresenter(client)
            } catch {
                self?.session?.log.append("tap refused the presenter secret: \(error)", source: .app)
            }
            guard let self, self.client === client, self.state == .starting || self.state == .presenting else { return }
            if let cookie { await Self.installPresenterCookie(cookie, into: AppEnvironment.shared.presentationDataStore) }
            guard self.client === client, self.state == .starting || self.state == .presenting else { return }
            self.openWindows(client: client)
        }
    }

    /// Puts the hub's presenter cookie into the talk pages' data store.
    /// Cookies ignore ports, so the one value for 127.0.0.1 is the present
    /// process's while a talk runs; the preview is driven by the app's own
    /// socket, which carries its cookie in a header, and never needs this one.
    static func installPresenterCookie(_ value: String, into store: WKWebsiteDataStore) async {
        guard let cookie = HTTPCookie(properties: [.name: TapClient.presenterCookieName, .value: value,
                                                    .domain: "127.0.0.1", .path: "/"]) else { return }
        await withCheckedContinuation { (continuation: CheckedContinuation<Void, Never>) in
            store.httpCookieStore.setCookie(cookie) { continuation.resume() }
        }
    }

    /// Creates the windows off screen, or reuses them after a restart, and
    /// loads the pages at `lastSlide`. The assertion is held from here:
    /// tap is up and the windows exist.
    private func openWindows(client: TapClient) {
        guard let options else { return }
        guard let arrangement = DisplayArrangement.resolve(screens: screens(), store: displayAssignments) else {
            fail("No display is connected.")
            return
        }
        self.arrangement = arrangement
        sleepAssertion.acquire()
        startRecordingTimer()
        if screenObserver == nil {
            screenObserver = NotificationCenter.default.addObserver(forName: NSApplication.didChangeScreenParametersNotification, object: nil, queue: nil) { [weak self] _ in
                MainActor.assumeIsolated { self?.screensChanged() }
            }
        }
        if options.mode == .play {
            let audience = audienceWindow ?? makeWindow(role: .audience, frame: arrangement.audience.frame)
            audienceWindow = audience
            audience.page.load(client.audienceLaunchURL(slide: lastSlide), allowedPort: client.ready.port)
        }
        let presenter = presenterWindow ?? makeWindow(role: .presenter, frame: arrangement.presenter.frame)
        presenterWindow = presenter
        presenter.page.load(client.presenterURL(slide: lastSlide), allowedPort: client.ready.port)
        if !windowsShown { armShowWindowsFallback() }
        refreshPresenterToolbar()
        // A new ready is a new process: the last one's tunnel, URL and QR
        // code went with it, and a remote the person wants is asked for again.
        tunnel = nil
        tunnelError = nil
        if wantsRemote { session?.send(.tunnel(start: true)) }
        onTunnelChange?()
    }

    private func makeWindow(role: PresentationWindow.Role, frame: CGRect) -> PresentationWindow {
        let window = PresentationWindow(role: role, screenFrame: frame)
        window.deckWindowController = deckWindowController
        window.page.onReady = { [weak self] _ in self?.pageReported() }
        window.page.onLoadFailed = { [weak self] _ in self?.pageReported() }
        window.page.onPresenterPopup = { [weak self] in self?.bringPresenterWindowForward() }
        window.onMouseMoved = { [weak self] in self?.noteMouseMoved() }
        if let toolbar = window.presenterToolbar {
            toolbar.onRecord = { [weak self] in self?.toggleRecording() }
            toolbar.onReload = { [weak self] in self?.reloadSlides() }
            toolbar.onSwap = { [weak self] in self?.swapDisplays() }
            toolbar.onStop = { [weak self] in self?.stop() }
        }
        windowCreated?(window)
        return window
    }

    private func armShowWindowsFallback() {
        showWindowsFallback?.cancel()
        let work = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated { self?.pageReported() }
        }
        showWindowsFallback = work
        DispatchQueue.main.asyncAfter(deadline: .now() + Self.showWindowsFallbackInterval, execute: work)
    }

    /// A page reported ready or failed to load, or the fallback fired.
    private func pageReported() {
        pagesReported = true
        showWindowsIfReady()
    }

    private func showWindowsIfReady() {
        guard state == .starting, !windowsShown, pagesReported, pendingQuestions.isEmpty else { return }
        showWindows()
    }

    /// Puts the talk's windows up. This is the one place presenting takes
    /// the screen: the person clicked Play, and taking the projector is
    /// what the click means. On two displays each window enters its own
    /// Space, one at a time (AppKit runs one transition at a time), the
    /// presenter last so its Space is active and it is key, since that is
    /// where the speaker's keys go. On one display only the audience
    /// window goes up (to full screen); the presenter window waits as its
    /// child-to-be, shown by Option-Tab or the S key. The first entry
    /// waits until the last talk's windows are quiet.
    private func showWindows() {
        showWindowsFallback?.cancel()
        showWindowsFallback = nil
        guard arrangement != nil, presenterWindow != nil else { return }
        windowsShown = true
        installKeyMonitor()
        windowsWereShown = true
        state = .presenting
        let fullScreen = usesFullScreen
        if !fullScreen, !fullScreenAllowed() {
            session?.log.append("this host does not allow system full screen; the talk windows are plain windows over their displays", source: .app)
        } else if !fullScreen {
            session?.log.append("\"Displays have separate Spaces\" is off in System Settings > Desktop & Dock, so the talk windows are plain windows over their displays rather than full screen Spaces", source: .app)
        }
        whenFullScreenIsQuiet { [weak self] in self?.placeShownWindows() }
    }

    /// The first placement of the talk's windows, on the arrangement as it
    /// is now.
    private func placeShownWindows() {
        guard let arrangement, let presenterWindow, windowsShown else { return }
        let fullScreen = usesFullScreen
        var order: [(window: PresentationWindow, frame: CGRect, fullScreen: Bool)] = []
        let front: PresentationWindow
        if let audienceWindow, arrangement.isSingleDisplay {
            order = [(audienceWindow, arrangement.audience.frame, fullScreen)]
            front = audienceWindow
        } else {
            if let audienceWindow { order.append((audienceWindow, arrangement.audience.frame, fullScreen)) }
            order.append((presenterWindow, arrangement.presenter.frame, fullScreen))
            front = presenterWindow
        }
        frontWindow = front
        place(order) { [weak self, weak front] in
            guard let self, let front, front === self.frontWindow, self.windowsShown else { return }
            front.makeKeyAndOrderFront(nil)
        }
    }

    /// Runs `work` once no talk window, of this deck or another, is busy
    /// with full screen (`PresentationWindow.isBusyWithFullScreen`), and
    /// a moment later if one was, for the Space animation to finish: an
    /// entry asked for before then is dropped. At once when nothing is
    /// busy. A host that stays busy past `quietTimeout` gets `work` anyway.
    /// A take-down cancels the wait.
    private func whenFullScreenIsQuiet(_ work: @escaping () -> Void) {
        guard PresentationWindow.anyIsBusyWithFullScreen else { return work() }
        waitingForQuiet = true
        let deadline = Date().addingTimeInterval(Self.quietTimeout)
        let generation = startGeneration
        func poll() {
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.1) { [weak self] in
                MainActor.assumeIsolated {
                    guard let self, self.waitingForQuiet, self.startGeneration == generation, self.windowsShown else { return }
                    if PresentationWindow.anyIsBusyWithFullScreen, Date() < deadline { return poll() }
                    DispatchQueue.main.asyncAfter(deadline: .now() + Self.quietPeriod) { [weak self] in
                        MainActor.assumeIsolated {
                            guard let self, self.waitingForQuiet, self.startGeneration == generation, self.windowsShown else { return }
                            self.waitingForQuiet = false
                            work()
                        }
                    }
                }
            }
        }
        poll()
    }

    /// Puts each window on its display in turn, then calls `completion`.
    /// A second window asked to enter full screen while another is still
    /// animating fails to, so the queue waits for each to settle.
    private func place(_ windows: [(window: PresentationWindow, frame: CGRect, fullScreen: Bool)], completion: @escaping () -> Void = {}) {
        placements += windows
        placementCompletion = completion
        placeNext()
    }

    private func placeNext() {
        guard !placing else { return }
        guard let next = placements.first else {
            let completion = placementCompletion
            placementCompletion = nil
            completion?()
            return
        }
        placements.removeFirst()
        placing = true
        next.window.present(on: next.frame, fullScreen: next.fullScreen) { [weak self] in
            guard let self else { return }
            self.placing = false
            if next.fullScreen, next.window.fullScreenState != .fullScreen, !next.window.isClosed {
                let name = next.window.role == .audience ? "audience" : "presenter"
                self.session?.log.append("the \(name) window could not enter full screen and stays a plain window over its display", source: .app)
            }
            self.placeNext()
        }
    }

    /// Option-Tab on one display: the presenter view comes over the
    /// audience view as its child, in the same Space, or goes away again.
    /// No Space switch, no animation.
    func toggleFrontWindow() {
        guard let audienceWindow, let presenterWindow, windowsShown, arrangement?.isSingleDisplay == true else { return }
        if presenterIsShownOverAudience {
            presenterWindow.detach()
            frontWindow = audienceWindow
            audienceWindow.makeKey()
        } else {
            showPresenterOverAudience()
        }
    }

    /// The S key in the audience page: the presenter view comes forward.
    /// On one display it comes over the audience view; on two its Space
    /// becomes the active one, which is what making it key does.
    func bringPresenterWindowForward() {
        guard let presenterWindow, windowsShown else { return }
        if arrangement?.isSingleDisplay == true, audienceWindow != nil {
            showPresenterOverAudience()
        } else {
            presenterWindow.makeKeyAndOrderFront(nil)
            frontWindow = presenterWindow
        }
    }

    // MARK: Keys

    /// Escape in the audience window ends the talk (in the presenter window
    /// too, when there is no audience window: a rehearsal), and on one
    /// display Option-Tab switches to the other window's Space. Every
    /// other key goes to tap's page unchanged, and so does Option-Tab when
    /// each window has a display of its own. Internal so a test can drive
    /// it with an event of its own; the monitor calls it for every key
    /// down while the windows show.
    func handleKey(_ event: NSEvent) -> NSEvent? {
        guard isActive, let window = event.window as? PresentationWindow,
              window === audienceWindow || window === presenterWindow else { return event }
        if event.keyCode == 53, window.role == .audience || audienceWindow == nil {
            stop()
            return nil
        }
        if event.keyCode == 48, event.modifierFlags.contains(.option), arrangement?.isSingleDisplay == true, audienceWindow != nil {
            toggleFrontWindow()
            return nil
        }
        return event
    }

    private func installKeyMonitor() {
        guard keyMonitor == nil else { return }
        keyMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
            guard let self else { return event }
            return self.handleKey(event)
        }
    }

    private func removeKeyMonitor() {
        if let keyMonitor { NSEvent.removeMonitor(keyMonitor) }
        keyMonitor = nil
    }

    // MARK: The presenter toolbar

    func refreshPresenterToolbar() {
        guard let presenterWindow, let options else { return }
        presenterWindow.presenterToolbar?.update(recording: recording, editsNotShown: editsNotShown, mode: options.mode)
        presenterWindow.recordingDot?.isHidden = !recording.isRecording
    }

    /// tap dev answered for `text`: an edit the audience has not seen, unless
    /// the text is back to what tap present read. tap dev also answers
    /// after a restart and after a component change with the text
    /// unchanged; those are not edits, so a text counts once.
    func deckTextChanged(_ text: String) {
        guard isActive else { return }
        if text == presentedText {
            editsNotShown = 0
        } else if text != lastCountedText {
            editsNotShown += 1
        }
        lastCountedText = text
        refreshPresenterToolbar()
    }

    /// Reload Slides: the buffer goes to the file, then tap present reads
    /// it again, as r does.
    func reloadSlides() {
        guard state == .presenting else { return }
        saveDeck { [weak self] error in
            guard let self, self.state == .presenting else { return }
            if let error {
                self.session?.log.append("Reload Slides could not save the deck: \(error.localizedDescription)", source: .app)
                return
            }
            self.session?.send(.reload)
            self.editsNotShown = 0
            self.refreshPresenterToolbar()
        }
    }

    /// REC: stop a recording, or start a new segment, as c does.
    func toggleRecording() {
        guard isActive else { return }
        session?.send(.recording(action: recording.isRecording ? .stop : .newSegment))
    }

    /// Starts or stops tap's tunnel, as u does, while the talk is up.
    func setTunnel(on: Bool) {
        guard state == .starting || state == .presenting else { return }
        wantsRemote = on
        session?.send(.tunnel(start: on))
    }

    /// True while tap's tunnel runs or is starting.
    var remoteIsOn: Bool { tunnel?.state == "running" || tunnel?.state == "starting" }

    /// Present > Phone Remote works only
    /// while the talk is up: a stopping tap is quitting, and a starting
    /// one asks for the remote itself once it is ready.
    var canTogglePhoneRemote: Bool { state == .presenting }

    /// Present > Phone Remote: the remote off while it
    /// runs or starts, on otherwise.
    func togglePhoneRemote() {
        guard canTogglePhoneRemote else { return }
        setTunnel(on: !remoteIsOn)
    }

    /// The pointer moved over a talk window: the cursor hides again after it rests.
    func noteMouseMoved() {
        guard isActive else { return }
        cursorHideWork?.cancel()
        let work = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated {
                guard let self else { return }
                self.cursorHideWork = nil
                self.hideCursor()
            }
        }
        cursorHideWork = work
        DispatchQueue.main.asyncAfter(deadline: .now() + cursorHideDelay, execute: work)
    }

    // MARK: Displays

    /// Exchanges the audience and presenter displays, before the talk (the
    /// popover's Swap Displays) or during it (the toolbar's), and remembers
    /// the choice for this pair of displays, across decks. During a talk
    /// each window leaves its Space, moves and enters the other display's.
    /// Nothing to swap on one display.
    func swapDisplays() {
        let screens = self.screens()
        guard let current = arrangement ?? DisplayArrangement.resolve(screens: screens, store: displayAssignments),
              !current.isSingleDisplay else { return }
        let swapped = current.swapped()
        displayAssignments.setAudienceName(swapped.audience.name, for: screens)
        guard isActive, arrangement != nil else { return }
        arrangement = swapped
        guard windowsShown else { return }
        moveWindows(to: swapped, from: current)
    }

    /// The displays changed while a talk runs: a projector unplugged or
    /// plugged back in. macOS has already moved a vanished display's Space
    /// to a remaining one; the windows are asked for the new arrangement
    /// (a window already on its frame does nothing). With one display
    /// left, the presenter window leaves its Space and becomes the
    /// audience window's child, shown, since the speaker is at the laptop;
    /// with the projector back it detaches and gets its Space again.
    /// AppKit also posts the notification when a talk window enters or
    /// leaves full screen, and for Dock and menu bar changes; with the
    /// displays unchanged nothing moves. Before the windows show, the
    /// arrangement is updated and `showWindows` reads it.
    func screensChanged() {
        onScreensChanged?()
        guard isActive, let current = arrangement,
              let resolved = DisplayArrangement.resolve(screens: screens(), store: displayAssignments),
              resolved != current else { return }
        arrangement = resolved
        guard windowsShown else { return }
        moveWindows(to: resolved, from: current)
    }

    /// Puts the windows on `arrangement`'s displays, coming from
    /// `previous`. A rehearsal has only the presenter window, which goes
    /// where the arrangement puts it. A talk on one display places the
    /// audience window; the presenter window comes over it as its child
    /// when the talk has just lost its second display (the speaker is at
    /// the laptop) or was over it already, and otherwise stays hidden
    /// until Option-Tab or the S key. On two, each window gets its
    /// display, the presenter last so its Space is the active one.
    private func moveWindows(to arrangement: DisplayArrangement, from previous: DisplayArrangement) {
        // The first placement has not run yet; it reads the new arrangement.
        guard !waitingForQuiet else { return }
        let fullScreen = usesFullScreen
        guard let presenterWindow else { return }
        guard let audienceWindow else {
            place([(presenterWindow, arrangement.presenter.frame, fullScreen)]) { [weak self, weak presenterWindow] in
                guard let self, let presenterWindow, self.windowsShown else { return }
                presenterWindow.makeKeyAndOrderFront(nil)
            }
            return
        }
        if arrangement.isSingleDisplay {
            let showPresenter = !previous.isSingleDisplay || presenterIsShownOverAudience
            presenterWindow.detach()
            // A presenter window with a Space of its own leaves it first (a
            // child may not have one), then rides over the audience. One
            // without is not placed: placing orders it front, which would
            // show the notes while the audience window moves.
            var order: [(window: PresentationWindow, frame: CGRect, fullScreen: Bool)] = []
            if presenterWindow.fullScreenState != .windowed { order.append((presenterWindow, arrangement.presenter.frame, false)) }
            order.append((audienceWindow, arrangement.audience.frame, fullScreen))
            place(order) { [weak self] in
                guard let self, self.windowsShown, let presenterWindow = self.presenterWindow else { return }
                presenterWindow.orderOut(nil)
                self.frontWindow = self.audienceWindow
                if showPresenter { self.showPresenterOverAudience() }
            }
        } else {
            presenterWindow.detach()
            place([(audienceWindow, arrangement.audience.frame, fullScreen), (presenterWindow, arrangement.presenter.frame, fullScreen)]) { [weak self] in
                guard let self, self.windowsShown, let presenterWindow = self.presenterWindow else { return }
                self.frontWindow = presenterWindow
                presenterWindow.makeKeyAndOrderFront(nil)
            }
        }
    }

    private func showPresenterOverAudience() {
        guard let audienceWindow, let presenterWindow, !presenterIsShownOverAudience else { return }
        presenterWindow.attach(to: audienceWindow)
        presenterWindow.makeKey()
        frontWindow = presenterWindow
    }

    // MARK: Events

    /// tap's stdout events. Internal so a test can deliver one through the
    /// same path the session uses.
    func handle(_ event: TapEvent) {
        switch event {
        case .slide(let slide, _):
            lastSlide = slide
        case .recording(let recordingEvent):
            recording.apply(recordingEvent)
            onRecordingChange?(recording)
            refreshPresenterToolbar()
        case .error(let payload) where payload.code == "recording_blocked":
            recording.blockedReason = payload.message
            onRecordingChange?(recording)
            refreshPresenterToolbar()
        case .error(let payload) where payload.code == "failed" && payload.message.hasPrefix("port ") && payload.message.contains("already in use"):
            portIsTaken()
        case .question(let id, let kind, let payload):
            let question = PendingQuestion(id: id, kind: kind, payload: payload)
            pendingQuestions.append(question)
            if kind == "keep-recording", state == .stopping {
                session?.extendQuit(timeout: Self.quitTimeoutWithRecording)
            }
            if pendingQuestions.count == 1 { onQuestion?(question) }
        case .tunnel:
            handleTunnel(event)
        case .error(let payload) where payload.code == "tunnel_unavailable" || payload.code == "tunnel_failed":
            handleTunnel(event)
        default:
            break
        }
        onEvent?(event)
    }

    /// tap's tunnel events and errors, heard only while the talk is up:
    /// tap's quit stops the tunnel and may report tunnel_failed while the
    /// talk is stopping, which is no reason to show the remote again.
    private func handleTunnel(_ event: TapEvent) {
        guard state == .starting || state == .presenting else { return }
        switch event {
        case .tunnel(let tunnelEvent):
            tunnel = tunnelEvent
            // A failed start is tunnel_failed followed by a stopped event
            // (app_session.go, tunnel); only a new start clears the reason.
            if tunnelEvent.state == "starting" || tunnelEvent.state == "running" { tunnelError = nil }
            onTunnelChange?()
        case .error(let payload):
            tunnelError = payload.message
            onTunnelChange?()
        default:
            break
        }
    }

    /// Answers the question with `id`, puts up the next queued one, and
    /// lets the windows show if they were waiting on it.
    func answer(id: String, value: Bool) {
        guard let index = pendingQuestions.firstIndex(where: { $0.id == id }) else { return }
        pendingQuestions.remove(at: index)
        session?.send(.answer(id: id, value: value))
        if index == 0, let next = pendingQuestions.first { onQuestion?(next) }
        showWindowsIfReady()
    }

    /// A sheet on the deck window is gone: the talk's front window is made
    /// key again, which brings its Space back. Not while another question
    /// waits, whose sheet the talk would cover, and not while the windows
    /// are still being placed, whose placement makes the front window key
    /// when it is done.
    func returnToTalk() {
        guard isActive, windowsShown, pendingQuestions.isEmpty, windowsAreSettled, let frontWindow else { return }
        frontWindow.makeKeyAndOrderFront(nil)
    }

    /// Counts the recording's seconds up between tap's events.
    private func startRecordingTimer() {
        recordingTimer?.invalidate()
        let timer = Timer(timeInterval: 1, repeats: true) { [weak self] _ in
            MainActor.assumeIsolated {
                guard let self, self.recording.isRecording else { return }
                self.recording.tick()
                self.refreshPresenterToolbar()
            }
        }
        // The common modes: a menu open over the talk (the menu bar drops
        // into full screen) runs the loop in tracking mode, and a timer in
        // the default mode alone would lose those seconds.
        RunLoop.main.add(timer, forMode: .common)
        recordingTimer = timer
    }

    // MARK: Stop

    /// Ends the talk: the windows leave full screen and close and the
    /// assertion is released at once, then tap is asked to quit, which may
    /// bring a keep-recording question before it exits.
    func stop() {
        guard state == .starting || state == .presenting else { return }
        state = .stopping
        takeDownWindows()
        // A session that is already stopped reports no further change, so
        // waiting for one would leave the talk stopping for good.
        guard let session, session.state != .stopped else {
            finishStopping()
            return
        }
        let ready: Bool = { if case .running = session.state { return true } else { return false } }()
        session.quit(timeout: ready ? Self.quitTimeout : Self.quitTimeoutBeforeReady)
    }

    /// Every ending goes through here: Stop, a failed start, tap giving up,
    /// the deck window closing and the app quitting. The windows go down
    /// one at a time, the front one first (an attached presenter window
    /// closes at once as its parent's child); each leaves full screen and
    /// closes on its own clock. The assertion goes now.
    private func takeDownWindows() {
        removeKeyMonitor()
        if let screenObserver { NotificationCenter.default.removeObserver(screenObserver) }
        screenObserver = nil
        recordingTimer?.invalidate()
        recordingTimer = nil
        tunnel = nil
        tunnelError = nil
        showWindowsFallback?.cancel()
        showWindowsFallback = nil
        placements = []
        placing = false
        placementCompletion = nil
        waitingForQuiet = false
        var order: [PresentationWindow] = []
        if let frontWindow, !frontWindow.isAttached { order.append(frontWindow) }
        for window in [audienceWindow, presenterWindow].compactMap({ $0 }) where !order.contains(where: { $0 === window }) && !window.isAttached {
            order.append(window)
        }
        windowsGoingDown += order
        audienceWindow = nil
        presenterWindow = nil
        frontWindow = nil
        windowsShown = false
        cursorHideWork?.cancel()
        cursorHideWork = nil
        sleepAssertion.release()
        takeDownNext()
        onTunnelChange?()
    }

    private func takeDownNext() {
        guard !takingDown, let window = windowsGoingDown.first(where: { !$0.isClosed }) else {
            windowsGoingDown.removeAll { $0.isClosed }
            // The last window is down: Play may be able to start again.
            if windowsGoingDown.isEmpty {
                AppEnvironment.shared.noteTalkWindowsWentDown()
                releaseIfDone()
            }
            return
        }
        takingDown = true
        window.takeDown { [weak self] in
            guard let self else { return }
            self.takingDown = false
            self.windowsGoingDown.removeAll { $0.isClosed }
            self.takeDownNext()
        }
    }

    private func finishStopping() {
        session = nil
        client = nil
        pendingQuestions = []
        arrangement = nil
        state = .idle
        countOut()
        releaseIfDone()
        if windowsWereShown { onStopped?(lastSlide) }
    }

    /// tap present exited three times in thirty seconds, or never got ready.
    private func endBecauseTapFailed(lastOutput: [String]) {
        let summary = session?.restartPolicy.exitSummary ?? "tap present exited"
        let detail = lastOutput.last.map { "\(summary). Last output: \($0)" } ?? summary
        let showed = windowsWereShown
        fail(detail)
        if showed { onStopped?(lastSlide) }
    }

    private func fail(_ message: String) {
        takeDownWindows()
        if let session { AppEnvironment.shared.stopAndRetain(session) }
        session = nil
        client = nil
        pendingQuestions = []
        arrangement = nil
        state = .failed(message)
        countOut()
        releaseIfDone()
        onFailed?(message)
    }

    /// A talk whose deck has closed lets go of itself once nothing of it is
    /// left: its process has exited and its last window has closed. A take
    /// down's completion holds the talk weakly, so a talk freed earlier
    /// would leave its remaining windows open.
    private func releaseIfDone() {
        guard !isEnding else { return }
        AppEnvironment.shared.releaseEndingTalk(self)
    }

    private func countIn() {
        guard !countedAsPresenting else { return }
        countedAsPresenting = true
        AppEnvironment.shared.noteTalkStarted()
    }

    private func countOut() {
        guard countedAsPresenting else { return }
        countedAsPresenting = false
        AppEnvironment.shared.noteTalkEnded()
    }
}

extension ScreenInfo {
    /// A real display. The built-in flag comes from CoreGraphics, the name
    /// from the system ("Built-in Retina Display", "LG UltraFine").
    init(screen: NSScreen) {
        let number = screen.deviceDescription[NSDeviceDescriptionKey(rawValue: "NSScreenNumber")] as? NSNumber
        let displayID = CGDirectDisplayID(number?.uint32Value ?? 0)
        self.init(name: screen.localizedName, frame: screen.frame, isBuiltIn: CGDisplayIsBuiltin(displayID) != 0)
    }
}
