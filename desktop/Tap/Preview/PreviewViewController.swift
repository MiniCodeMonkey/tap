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

/// Forwards script messages without the user content controller keeping
/// the preview alive.
private final class WeakScriptMessageHandler: NSObject, WKScriptMessageHandler {
    weak var target: WKScriptMessageHandler?

    init(_ target: WKScriptMessageHandler) {
        self.target = target
    }

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        target?.userContentController(userContentController, didReceive: message)
    }
}

/// tap's audience page for the slide under the cursor, a status line, the
/// step controls and the pin.
final class PreviewViewController: NSViewController, WKNavigationDelegate, WKUIDelegate, WKScriptMessageHandler {
    let webView: WKWebView
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
    private var allowedPort: Int?
    /// Opens a URL outside the app. Production hands this to `NSWorkspace`;
    /// a test replaces it to see what the app tried to open without
    /// touching the person's browser.
    var openExternally: (URL) -> Void = { url in NSWorkspace.shared.open(url) }

    override init(nibName: NSNib.Name?, bundle: Bundle?) {
        let configuration = WKWebViewConfiguration()
        configuration.userContentController = WKUserContentController()
        configuration.websiteDataStore = .default()
        webView = WKWebView(frame: .zero, configuration: configuration)
        stepControl = NSSegmentedControl(images: [
            NSImage(systemSymbolName: "chevron.left", accessibilityDescription: "Previous step")!,
            NSImage(systemSymbolName: "chevron.right", accessibilityDescription: "Next step")!,
        ], trackingMode: .momentary, target: nil, action: nil)
        pinButton = NSButton(image: NSImage(systemSymbolName: "pin", accessibilityDescription: "Pin this slide")!, target: nil, action: nil)
        super.init(nibName: nil, bundle: nil)
        configuration.userContentController.add(WeakScriptMessageHandler(self), name: "tapReady")
        webView.navigationDelegate = self
        webView.uiDelegate = self
        webView.setAccessibilityIdentifier("preview")
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

    override func loadView() {
        let root = NSView()
        statusLabel.font = .systemFont(ofSize: 12)
        statusLabel.textColor = .secondaryLabelColor
        stepLabel.font = .systemFont(ofSize: 12)
        stepLabel.textColor = .secondaryLabelColor
        webView.wantsLayer = true
        webView.layer?.cornerRadius = 8
        webView.layer?.masksToBounds = true
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
        pageLoadCount += 1
        lastLoadDate = Date()
        allowedPort = client.ready.port
        lastReady = nil
        webView.load(URLRequest(url: client.previewLaunchURL))
    }

    /// Loads the page the preview already shows again, from scratch. The
    /// launch code is spent by now, so this reloads the address tap
    /// redirected to, which the cookie from that launch still opens. The
    /// page runs its ready cycle again and reports a fresh ready.
    func reload() {
        pageLoadCount += 1
        lastLoadDate = Date()
        lastReady = nil
        webView.reload()
    }

    func show(_ navigator: PreviewNavigator) {
        statusLabel.stringValue = navigator.statusLabel
        stepLabel.stringValue = navigator.stepLabel
        pinButton.state = navigator.isPinned ? .on : .off
        stepControl.setEnabled(navigator.positionIndex > 0, forSegment: 0)
        stepControl.setEnabled(navigator.positionIndex < navigator.revealCount, forSegment: 1)
    }

    /// The page's visible text. Tests read it; the app never runs script in the page.
    func pageText() async -> String {
        (try? await webView.evaluateJavaScript("document.body.innerText") as? String) ?? ""
    }

    /// Evaluates `script` in the page and describes what came back. Tests
    /// read it when they need to say which side of the hub a stall is on.
    func pageValue(_ script: String) async -> String {
        do {
            return String(describing: try await webView.evaluateJavaScript(script))
        } catch {
            return "script failed: \(error)"
        }
    }

    /// `restartPolicy` names the session's own exit count and window, so the
    /// stopped notice reports the numbers that actually made it give up
    /// rather than a fixed guess. `pausedMessage` explains a session the app
    /// stopped on purpose, such as while the deck file is deleted.
    func showSessionState(_ state: TapSession.State, restartPolicy: RestartPolicy, pausedMessage: String? = nil) {
        switch state {
        case .running:
            overlay.hide()
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
        lastReady = payload
        onReady?(payload)
    }
}
