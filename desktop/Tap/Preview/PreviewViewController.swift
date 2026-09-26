import AppKit
import WebKit

/// P5's ready signal: the page finished rendering this slide and step.
struct ReadyPayload: Equatable {
    let revision: String
    let slide: Int
    let step: Int
    /// False only when a live page ran out of settle rounds and reported
    /// ready anyway. Absent means the round settled normally.
    let settled: Bool

    init(revision: String, slide: Int, step: Int, settled: Bool = true) {
        self.revision = revision
        self.slide = slide
        self.step = step
        self.settled = settled
    }
}

/// tap's audience page for the slide under the cursor, a status line, the
/// step controls and the pin.
final class PreviewViewController: NSViewController, WKNavigationDelegate, WKUIDelegate, WKScriptMessageHandler {
    /// The page's web view. A web view whose content process stops
    /// answering is replaced by a new one (see `recoverPage`), so this is
    /// read afresh rather than kept.
    private(set) var webView: WKWebView
    let statusLabel = NSTextField(labelWithString: "")
    let stepLabel = NSTextField(labelWithString: "")
    let stepControl: NSSegmentedControl
    let pinButton: NSButton
    /// Holds the web view and anything laid over it.
    let pageContainer = NSView()

    let overlay = PreviewOverlayView()
    var onReady: ((ReadyPayload) -> Void)?
    var onStepBackward: (() -> Void)?
    var onStepForward: (() -> Void)?
    var onPinToggled: (() -> Void)?
    var onTryAgain: (() -> Void)?
    private(set) var lastReady: ReadyPayload?
    /// How many times the preview has been pointed at a page. Every reload
    /// of the preview the app asks for goes through `load` or `reload`, and
    /// an in-place update goes through neither: the page keeps its document
    /// and rerenders from the hub. The count rises inside `load` and
    /// `reload`, before the navigation each starts, so what reads it sees a
    /// reload the moment the app asks for one rather than whenever WebKit
    /// gets round to reporting it.
    private(set) var pageLoadCount = 0
    /// When `load` or `reload` last pointed the preview at a page.
    private(set) var lastLoadDate: Date?
    /// How many script messages the tapReady handler has received, counting
    /// ones whose body it could not read. A test that times out waiting for
    /// ready reads it to tell a page that never posted from a post the app
    /// dropped.
    private(set) var readyMessagesReceived = 0
    /// How many times the preview has put a new web view in place of one
    /// whose content process stopped answering or went away.
    private(set) var pageRecoveryCount = 0
    /// What WebKit reported about the current load, in order, each with its
    /// time. A test that times out waiting for ready prints them to tell a
    /// load that never started from one that stalled after the server
    /// answered or after the page began.
    private(set) var navigationMilestones: [(name: String, date: Date)] = []
    /// A load WebKit has not finished after this long has its page asked to
    /// answer a no-op script. A seam: a test shortens it.
    var loadWatchdogInterval: TimeInterval = 10
    /// How long the page has to answer that script before its content
    /// process counts as stuck.
    static let pageAnswerTimeout: TimeInterval = 2
    /// New web views in a row, with no load finishing in between, before
    /// the preview stops trying and shows that it stopped.
    static let maximumRecoveriesInARow = 2
    /// New web views within `recoveryWindow`, finished loads or not, before
    /// the preview stops trying: a page whose process ends after every
    /// load would otherwise be started again for as long as the deck is
    /// open. Seams: a test lowers the count.
    var maximumRecoveriesInWindow = 3
    var recoveryWindow: TimeInterval = 60
    /// True once the preview stopped trying and shows that it stopped. Try
    /// Again, or a new tap, starts over in a new web view.
    private(set) var hasGivenUp = false
    /// Navigation callbacks WebKit made for a load other than the current
    /// one, such as a load that a newer one replaced. They are ignored.
    private(set) var ignoredNavigationCallbackCount = 0
    /// How many times the watchdog asked an unfinished load's page to
    /// answer. A test reads it to know the check ran on a live page.
    private(set) var pageAnswerCheckCount = 0
    /// Whether `webView`'s page answers a no-op script within the time
    /// given. Evaluating "1" here, in the preview and in the thumbnail
    /// renderer, is the single exception to the rule that the app runs no
    /// script in a page, and the person allowed it: the script reads and
    /// changes nothing, and it is the only way to tell a content process
    /// that stopped running from a page that is waiting on tap. A seam: a
    /// test replaces it to stand for a process that does not answer.
    var pageAnswers: (WKWebView, TimeInterval) async -> Bool = { webView, timeout in
        if case .noAnswer = await webView.evaluate("1", timeout: timeout) { return false }
        return true
    }
    /// Asked to restart tap, for a recovery that needs a new launch code.
    var onRestartSession: (() -> Void)?
    /// Writes a line to the deck's Tap Log.
    var onLog: ((String) -> Void)?
    private var allowedPort: Int?
    private var client: TapClient?
    /// True once tap has answered this client's launch URL, which spends
    /// the launch code and sets the cookie the base URL needs.
    private var launchCodeSpent = false
    private var loadFinished = false
    private var recoveriesInARow = 0
    private var recentRecoveryDates: [Date] = []
    private var isShowingRecovery = false
    /// The load `load`, `reload` or a recovery started. WebKit's callbacks
    /// for any other navigation say nothing about the page shown now.
    private var currentNavigation: WKNavigation?
    private var watchdog: DispatchWorkItem?
    /// Opens a URL outside the app. Production hands this to `NSWorkspace`;
    /// a test replaces it to see what the app tried to open without
    /// touching the person's browser.
    var openExternally: (URL) -> Void = { url in NSWorkspace.shared.open(url) }

    override init(nibName: NSNib.Name?, bundle: Bundle?) {
        webView = Self.makeWebView()
        stepControl = NSSegmentedControl(images: [
            NSImage(systemSymbolName: "chevron.left", accessibilityDescription: "Previous step")!,
            NSImage(systemSymbolName: "chevron.right", accessibilityDescription: "Next step")!,
        ], trackingMode: .momentary, target: nil, action: nil)
        pinButton = NSButton(image: NSImage(systemSymbolName: "pin", accessibilityDescription: "Pin this slide")!, target: nil, action: nil)
        super.init(nibName: nil, bundle: nil)
        adopt(webView)
        stepControl.target = self
        stepControl.action = #selector(stepControlPressed(_:))
        stepControl.setAccessibilityIdentifier("step-control")
        pinButton.setButtonType(.pushOnPushOff)
        pinButton.bezelStyle = .toolbar
        pinButton.target = self
        pinButton.action = #selector(pinPressed(_:))
        pinButton.setAccessibilityIdentifier("pin")
        overlay.tryAgainButton.target = self
        overlay.tryAgainButton.action = #selector(tryAgainPressed(_:))
        overlay.showLogButton.action = #selector(AppDelegate.showTapLog(_:))
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    /// A web view for the page: the default data store, where the launch
    /// cookie lives, so every web view the preview makes shares it.
    private static func makeWebView() -> WKWebView {
        let configuration = WKWebViewConfiguration()
        configuration.userContentController = WKUserContentController()
        configuration.websiteDataStore = .default()
        return WKWebView(frame: .zero, configuration: configuration)
    }

    /// Wires a web view to this controller: the tapReady handler, both
    /// delegates, its identifier and its rounded corners.
    private func adopt(_ webView: WKWebView) {
        webView.configuration.userContentController.add(WeakScriptMessageHandler(self), name: "tapReady")
        webView.navigationDelegate = self
        webView.uiDelegate = self
        webView.setAccessibilityIdentifier("preview")
        webView.wantsLayer = true
        webView.layer?.cornerRadius = 8
        webView.layer?.masksToBounds = true
    }

    override func loadView() {
        let root = NSView()
        statusLabel.font = .systemFont(ofSize: 12)
        statusLabel.textColor = .secondaryLabelColor
        stepLabel.font = .systemFont(ofSize: 12)
        stepLabel.textColor = .secondaryLabelColor
        let stepRow = NSStackView(views: [stepControl, stepLabel, NSView(), pinButton])
        stepRow.spacing = 10
        webView.translatesAutoresizingMaskIntoConstraints = false
        pageContainer.addSubview(webView)
        overlay.translatesAutoresizingMaskIntoConstraints = false
        pageContainer.addSubview(overlay)
        for view in [statusLabel, pageContainer, stepRow] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(view)
        }
        NSLayoutConstraint.activate([
            statusLabel.topAnchor.constraint(equalTo: root.topAnchor),
            statusLabel.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: 20),
            pageContainer.topAnchor.constraint(equalTo: statusLabel.bottomAnchor, constant: 10),
            pageContainer.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: 20),
            pageContainer.trailingAnchor.constraint(equalTo: root.trailingAnchor, constant: -20),
            pageContainer.heightAnchor.constraint(equalTo: pageContainer.widthAnchor, multiplier: 9.0 / 16.0),
            webView.topAnchor.constraint(equalTo: pageContainer.topAnchor),
            webView.leadingAnchor.constraint(equalTo: pageContainer.leadingAnchor),
            webView.trailingAnchor.constraint(equalTo: pageContainer.trailingAnchor),
            webView.bottomAnchor.constraint(equalTo: pageContainer.bottomAnchor),
            overlay.topAnchor.constraint(equalTo: pageContainer.topAnchor),
            overlay.leadingAnchor.constraint(equalTo: pageContainer.leadingAnchor),
            overlay.trailingAnchor.constraint(equalTo: pageContainer.trailingAnchor),
            overlay.bottomAnchor.constraint(equalTo: pageContainer.bottomAnchor),
            stepRow.topAnchor.constraint(equalTo: pageContainer.bottomAnchor, constant: 12),
            stepRow.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: 20),
            stepRow.trailingAnchor.constraint(equalTo: root.trailingAnchor, constant: -20),
        ])
        view = root
    }

    /// Loads the audience page of a newly started tap. The launch code works
    /// once: tap sets its cookie and redirects to the same URL without it.
    func load(client: TapClient) {
        self.client = client
        allowedPort = client.ready.port
        launchCodeSpent = false
        if hasGivenUp {
            hasGivenUp = false
            replaceWebView()
        }
        beginLoad()
        currentNavigation = webView.load(URLRequest(url: client.previewLaunchURL))
    }

    /// Loads the page the preview already shows again, from scratch. The
    /// launch code is spent by now, so this reloads the address tap
    /// redirected to, which the cookie from that launch still opens. The
    /// page runs its ready cycle again and reports a fresh ready.
    func reload() {
        beginLoad()
        currentNavigation = webView.reload()
    }

    /// Resets what the app knows about the page for a load about to start,
    /// and arms the load watchdog.
    private func beginLoad() {
        pageLoadCount += 1
        lastLoadDate = Date()
        lastReady = nil
        loadFinished = false
        navigationMilestones = []
        currentNavigation = nil
        armWatchdog()
    }

    // MARK: Recovery

    /// Checks the load `loadWatchdogInterval` from now. Only the web view
    /// in place now is checked, and a newer load re-arms it.
    private func armWatchdog() {
        watchdog?.cancel()
        let watched = webView
        let work = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated {
                guard let self else { return }
                Task { @MainActor [weak self] in await self?.checkLoad(of: watched) }
            }
        }
        watchdog = work
        DispatchQueue.main.asyncAfter(deadline: .now() + loadWatchdogInterval, execute: work)
    }

    /// A load that has not finished is either waiting on tap or stuck in a
    /// content process that no longer runs anything. A page still loading
    /// answers a no-op script from its current document at once, so no
    /// answer within `pageAnswerTimeout` means the process is stuck. A
    /// reload would go to that same process, so the page gets a new web
    /// view instead. A page that answers is waiting on tap and is checked
    /// again later. WebKit may throttle or suspend a web view nobody can
    /// see (out of any window, hidden, or in a covered window) on purpose,
    /// so such a view is only checked again later too.
    private func checkLoad(of watched: WKWebView) async {
        guard watched === webView, !loadFinished else { return }
        guard let window = watched.window, !watched.isHiddenOrHasHiddenAncestor, window.occlusionState.contains(.visible) else {
            armWatchdog()
            return
        }
        pageAnswerCheckCount += 1
        let answers = await pageAnswers(watched, Self.pageAnswerTimeout)
        guard watched === webView, !loadFinished else { return }
        if !answers {
            recoverPage("the preview's page did not finish loading in \(Int(loadWatchdogInterval)) s "
                        + "and did not answer in \(Int(Self.pageAnswerTimeout)) s")
        } else {
            armWatchdog()
        }
    }

    /// Puts a new web view, with a new content process, in place of one
    /// that stopped answering or lost its process, and loads the page in
    /// it. The launch code works once: when tap has already answered it,
    /// the cookie it set is in the shared data store and the base URL
    /// opens the page; otherwise tap is restarted for a new code, and the
    /// running state that follows loads the page. After
    /// `maximumRecoveriesInARow` new web views with no load finishing, or
    /// `maximumRecoveriesInWindow` within `recoveryWindow`, the preview
    /// shows that it stopped, with Try Again, as it does for a tap that
    /// stopped.
    private func recoverPage(_ reason: String) {
        watchdog?.cancel()
        watchdog = nil
        guard !hasGivenUp else { return }
        // With tap down there is no page to bring back: the next running
        // state loads one, in a new web view.
        guard client != nil else {
            replaceWebView()
            return
        }
        let now = Date()
        recentRecoveryDates = recentRecoveryDates.filter { now.timeIntervalSince($0) < recoveryWindow }
        guard recoveriesInARow < Self.maximumRecoveriesInARow else {
            giveUp("\(reason); the preview gave up after \(recoveriesInARow) new web views in a row")
            return
        }
        guard recentRecoveryDates.count < maximumRecoveriesInWindow else {
            giveUp("\(reason); the preview gave up after \(recentRecoveryDates.count) new web views in \(Int(recoveryWindow)) s")
            return
        }
        recentRecoveryDates.append(now)
        recoveriesInARow += 1
        pageRecoveryCount += 1
        onLog?("\(reason) (milestones: \(navigationMilestoneDescription)); "
               + "starting it again in a new web view (recovery \(pageRecoveryCount))")
        isShowingRecovery = true
        overlay.show(title: "Restarting preview.", detail: "The page stopped responding.", output: [], opaque: false, buttons: false)
        replaceWebView()
        if launchCodeSpent, let client {
            beginLoad()
            currentNavigation = webView.load(URLRequest(url: client.baseURL))
        } else {
            onRestartSession?()
        }
    }

    /// Stops recovering and shows that the preview stopped, with Try Again.
    /// The web view is left as it is; Try Again, or the next tap, starts
    /// over in a new one.
    private func giveUp(_ line: String) {
        watchdog?.cancel()
        watchdog = nil
        onLog?(line)
        hasGivenUp = true
        isShowingRecovery = false
        overlay.show(title: "The preview stopped", detail: "Its page stopped responding.", output: [], opaque: true, buttons: true)
    }

    /// A load failed. One that was bringing the page back after a recovery
    /// leaves nothing to show, so the preview shows that it stopped.
    private func currentLoadFailed(_ error: Error) {
        loadFinished = true
        watchdog?.cancel()
        if isShowingRecovery {
            giveUp("the preview's page did not load after a recovery: \(error.localizedDescription)")
        }
    }

    /// True for the current load's navigation; any other is counted and ignored.
    private func isCurrent(_ navigation: WKNavigation?) -> Bool {
        guard navigation === currentNavigation else {
            ignoredNavigationCallbackCount += 1
            return false
        }
        return true
    }

    private func replaceWebView() {
        let old = webView
        old.navigationDelegate = nil
        old.uiDelegate = nil
        old.configuration.userContentController.removeScriptMessageHandler(forName: "tapReady")
        old.stopLoading()
        let replacement = Self.makeWebView()
        adopt(replacement)
        webView = replacement
        old.replaceInSuperview(with: replacement)
    }

    /// A load that finished, or a ready, proves the page works again.
    private func pageWorks() {
        recoveriesInARow = 0
        if isShowingRecovery {
            isShowingRecovery = false
            overlay.hide()
        }
    }

    private func recordMilestone(_ name: String) {
        navigationMilestones.append((name: name, date: Date()))
    }

    /// The milestones of the current load, each as seconds after the load
    /// began: "start+0.02s redirect+0.05s commit+0.09s finish+0.31s".
    var navigationMilestoneDescription: String {
        guard let start = lastLoadDate else { return "none" }
        let described = navigationMilestones.map { "\($0.name)+\(String(format: "%.2f", $0.date.timeIntervalSince(start)))s" }
        return described.isEmpty ? "none" : described.joined(separator: " ")
    }

    func show(_ navigator: PreviewNavigator) {
        statusLabel.stringValue = navigator.statusLabel
        stepLabel.stringValue = navigator.stepLabel
        pinButton.state = navigator.isPinned ? .on : .off
        stepControl.setEnabled(navigator.positionIndex > 0, forSegment: 0)
        stepControl.setEnabled(navigator.positionIndex < navigator.revealCount, forSegment: 1)
    }

    /// The page's visible text, or "" when the page does not answer
    /// within `timeout`. Tests read it. The app itself runs no script in
    /// the page beyond the load watchdog's no-op check that it answers.
    func pageText(timeout: TimeInterval = 5) async -> String {
        if case .value(let value) = await webView.evaluate("document.body.innerText", timeout: timeout) {
            return value as? String ?? ""
        }
        return ""
    }

    /// Evaluates `script` in the page and describes what came back. Tests
    /// read it when they need to say which side of the hub a stall is on,
    /// often after a wait has already timed out on a page that may no
    /// longer answer, so it gives the page `timeout` seconds.
    func pageValue(_ script: String, timeout: TimeInterval = 3) async -> String {
        switch await webView.evaluate(script, timeout: timeout) {
        case .value(let value): return String(describing: value)
        case .failed(let error): return "script failed: \(error)"
        case .noAnswer(let seconds): return "no answer in \(Int(seconds)) s"
        }
    }

    /// `restartPolicy` names the session's own exit count and window, so the
    /// stopped notice reports the numbers that actually made it give up
    /// rather than a fixed guess. `pausedMessage` explains a session the app
    /// stopped on purpose, such as while the deck file is deleted.
    func showSessionState(_ state: TapSession.State, restartPolicy: RestartPolicy, pausedMessage: String? = nil) {
        if case .running = state {} else {
            // The client is gone with tap, so nothing checks or reloads a
            // page until the next running state loads one.
            client = nil
            watchdog?.cancel()
            watchdog = nil
        }
        switch state {
        case .running:
            // A recovery that restarted tap keeps its notice until the new
            // page finishes loading.
            if !isShowingRecovery { overlay.hide() }
        case .starting where overlay.isHidden:
            break
        case .starting, .restarting:
            overlay.show(title: "Restarting preview.", detail: "Showing the last good render.", output: [], opaque: false, buttons: false)
        case .failed(let lastOutput):
            overlay.show(title: "The preview stopped", detail: "\(restartPolicy.exitSummary). Last output:",
                         output: lastOutput, opaque: true, buttons: true)
        case .stopped:
            if let pausedMessage {
                overlay.show(title: "The preview is paused", detail: pausedMessage, output: [], opaque: false, buttons: false)
            }
        }
    }

    @objc private func tryAgainPressed(_ sender: NSButton) {
        recoveriesInARow = 0
        recentRecoveryDates = []
        if hasGivenUp {
            hasGivenUp = false
            replaceWebView()
        }
        onTryAgain?()
    }

    @objc private func stepControlPressed(_ sender: NSSegmentedControl) {
        if sender.selectedSegment == 0 { onStepBackward?() } else { onStepForward?() }
    }

    @objc private func pinPressed(_ sender: NSButton) {
        onPinToggled?()
    }

    // MARK: WebKit

    /// A deck's own page is hostile input: it can redirect itself, submit a
    /// form or open a window with no click at all, and none of that may
    /// reach anything outside the sandboxed preview. The one navigation
    /// worth handing to the system browser is a person actually clicking a
    /// plain web link, so both conditions have to hold: `navigationType` is
    /// `.linkActivated`, WebKit's own record of a click on an anchor, and
    /// the scheme is `http` or `https`. The scheme check stands even for a
    /// genuine click, so a link a page points at `file:`, `javascript:` or
    /// another app's registered scheme still goes nowhere.
    static func isExternalWebLink(url: URL, navigationType: WKNavigationType) -> Bool {
        navigationType == .linkActivated && (url.scheme == "http" || url.scheme == "https")
    }

    func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction) async -> WKNavigationActionPolicy {
        guard let url = navigationAction.request.url else { return .cancel }
        if url.host == "127.0.0.1", url.port == allowedPort { return .allow }
        if url.scheme == "about" || url.scheme == "blob" || url.scheme == "data" { return .allow }
        if Self.isExternalWebLink(url: url, navigationType: navigationAction.navigationType) { openExternally(url) }
        return .cancel
    }

    func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
                 for navigationAction: WKNavigationAction, windowFeatures: WKWindowFeatures) -> WKWebView? {
        if let url = navigationAction.request.url, Self.isExternalWebLink(url: url, navigationType: navigationAction.navigationType) {
            openExternally(url)
        }
        return nil
    }

    func webView(_ webView: WKWebView, didStartProvisionalNavigation navigation: WKNavigation!) {
        guard isCurrent(navigation) else { return }
        recordMilestone("start")
    }

    /// tap answers the launch URL with its cookie and a redirect, so a
    /// redirect means the launch code is spent.
    func webView(_ webView: WKWebView, didReceiveServerRedirectForProvisionalNavigation navigation: WKNavigation!) {
        guard isCurrent(navigation) else { return }
        recordMilestone("redirect")
        launchCodeSpent = true
    }

    func webView(_ webView: WKWebView, didCommit navigation: WKNavigation!) {
        guard isCurrent(navigation) else { return }
        recordMilestone("commit")
        launchCodeSpent = true
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        guard isCurrent(navigation) else { return }
        recordMilestone("finish")
        loadFinished = true
        watchdog?.cancel()
        pageWorks()
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        guard isCurrent(navigation) else { return }
        recordMilestone("failProvisional")
        currentLoadFailed(error)
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        guard isCurrent(navigation) else { return }
        recordMilestone("fail")
        currentLoadFailed(error)
    }

    /// The page's content process exited or crashed. The web view stays
    /// blank from then on, so the page gets a new web view.
    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
        guard webView === self.webView else { return }
        recordMilestone("processTerminated")
        recoverPage("the preview's web content process ended")
    }

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        readyMessagesReceived += 1
        guard message.name == "tapReady", let body = message.body as? [String: Any],
              let slide = (body["slide"] as? NSNumber)?.intValue else { return }
        let payload = ReadyPayload(revision: body["revision"] as? String ?? "",
                                   slide: slide,
                                   step: (body["step"] as? NSNumber)?.intValue ?? 0,
                                   settled: (body["settled"] as? NSNumber)?.boolValue ?? true)
        pageReportedReady(payload)
    }

    /// Records a ready the page reported and passes it on. Internal, not
    /// private, so a test can deliver a ready of its own making through the
    /// same path the tapReady handler uses.
    func pageReportedReady(_ payload: ReadyPayload) {
        pageWorks()
        lastReady = payload
        onReady?(payload)
    }
}
