import AppKit
import WebKit

/// P5's ready signal: the page finished rendering this slide and step.
struct ReadyPayload: Equatable {
    let revision: String
    let slide: Int
    let step: Int
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

    var onReady: ((ReadyPayload) -> Void)?
    var onStepBackward: (() -> Void)?
    var onStepForward: (() -> Void)?
    var onPinToggled: (() -> Void)?
    private(set) var lastReady: ReadyPayload?
    private(set) var finishedNavigationCount = 0
    private var allowedPort: Int?

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
            stepRow.topAnchor.constraint(equalTo: pageContainer.bottomAnchor, constant: 12),
            stepRow.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: 20),
            stepRow.trailingAnchor.constraint(equalTo: root.trailingAnchor, constant: -20),
        ])
        view = root
    }

    /// Loads the audience page of a newly started tap. The launch code works
    /// once: tap sets its cookie and redirects to the same URL without it.
    func load(client: TapClient) {
        allowedPort = client.ready.port
        lastReady = nil
        webView.load(URLRequest(url: client.previewLaunchURL))
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

    @objc private func stepControlPressed(_ sender: NSSegmentedControl) {
        if sender.selectedSegment == 0 { onStepBackward?() } else { onStepForward?() }
    }

    @objc private func pinPressed(_ sender: NSButton) {
        onPinToggled?()
    }

    // MARK: WebKit

    func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction) async -> WKNavigationActionPolicy {
        guard let url = navigationAction.request.url else { return .cancel }
        if url.host == "127.0.0.1", url.port == allowedPort { return .allow }
        if url.scheme == "about" || url.scheme == "blob" || url.scheme == "data" { return .allow }
        if navigationAction.targetFrame?.isMainFrame ?? true { NSWorkspace.shared.open(url) }
        return .cancel
    }

    func webView(_ webView: WKWebView, didFinish navigation: WKNavigation!) {
        finishedNavigationCount += 1
    }

    func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
                 for navigationAction: WKNavigationAction, windowFeatures: WKWindowFeatures) -> WKWebView? {
        if let url = navigationAction.request.url { NSWorkspace.shared.open(url) }
        return nil
    }

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        guard message.name == "tapReady", let body = message.body as? [String: Any],
              let slide = (body["slide"] as? NSNumber)?.intValue else { return }
        let payload = ReadyPayload(revision: body["revision"] as? String ?? "",
                                   slide: slide,
                                   step: (body["step"] as? NSNumber)?.intValue ?? 0)
        lastReady = payload
        onReady?(payload)
    }
}
