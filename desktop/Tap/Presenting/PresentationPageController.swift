import AppKit
import WebKit

/// One of tap's own pages for a talk, the audience page or the presenter
/// page, in a `WKWebView` with what tap dev's browser gives it: element
/// full screen for the F key, a persistent data store so the presenter
/// layout and notes size (the page's localStorage, keyed by origin, which
/// is why the deck keeps one port) survive between launches, and the
/// window the S key opens answered by bringing the presenter window
/// forward. The app drives the page only through the URL it loads (its
/// start slide is in the fragment, the presenter key in the query) and
/// through tap's hub, and reads it only through the tapReady handler.
final class PresentationPageController: NSViewController, WKNavigationDelegate, WKUIDelegate, WKScriptMessageHandler {
    let webView: WKWebView
    var onReady: ((ReadyPayload) -> Void)?
    var onLoadFailed: ((Error) -> Void)?
    /// The page asked for a window at /presenter: the S key.
    var onPresenterPopup: (() -> Void)?
    /// Opens a URL outside the app. A test replaces it to see what the app tried to open.
    var openExternally: (URL) -> Void = { url in NSWorkspace.shared.open(url) }
    private(set) var lastReady: ReadyPayload?
    private(set) var pageLoadCount = 0
    private(set) var lastLoadedURL: URL?
    private var allowedPort: Int?

    init(accessibilityIdentifier: String, dataStore: WKWebsiteDataStore) {
        let configuration = WKWebViewConfiguration()
        configuration.userContentController = WKUserContentController()
        configuration.websiteDataStore = dataStore
        configuration.preferences.isElementFullscreenEnabled = true
        webView = WKWebView(frame: .zero, configuration: configuration)
        super.init(nibName: nil, bundle: nil)
        configuration.userContentController.add(WeakScriptMessageHandler(self), name: "tapReady")
        webView.navigationDelegate = self
        webView.uiDelegate = self
        webView.setAccessibilityIdentifier(accessibilityIdentifier)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func loadView() {
        view = webView
    }

    /// Loads one of tap's pages. `allowedPort` is the talk's tap, the only
    /// origin the page may navigate within.
    func load(_ url: URL, allowedPort: Int) {
        self.allowedPort = allowedPort
        pageLoadCount += 1
        lastLoadedURL = url
        lastReady = nil
        webView.load(URLRequest(url: url))
    }

    // MARK: WebKit

    func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction) async -> WKNavigationActionPolicy {
        guard let url = navigationAction.request.url else { return .cancel }
        if url.host == "127.0.0.1", url.port == allowedPort { return .allow }
        if url.scheme == "about" || url.scheme == "blob" || url.scheme == "data" { return .allow }
        if PreviewViewController.isExternalWebLink(url: url, navigationType: navigationAction.navigationType) { openExternally(url) }
        return .cancel
    }

    func webView(_ webView: WKWebView, createWebViewWith configuration: WKWebViewConfiguration,
                 for navigationAction: WKNavigationAction, windowFeatures: WKWindowFeatures) -> WKWebView? {
        popupRequested(for: navigationAction.request.url, navigationType: navigationAction.navigationType)
        return nil
    }

    /// Answers a `window.open` from the page: this talk's presenter view
    /// goes to the presenter window, a clicked web link to the browser,
    /// anything else nowhere. Internal so a test can drive it without a page.
    func popupRequested(for url: URL?, navigationType: WKNavigationType) {
        guard let url else { return }
        if url.host == "127.0.0.1", url.port == allowedPort, url.path.hasPrefix("/presenter") {
            onPresenterPopup?()
        } else if PreviewViewController.isExternalWebLink(url: url, navigationType: navigationType) {
            openExternally(url)
        }
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        onLoadFailed?(error)
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        onLoadFailed?(error)
    }

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        guard message.name == "tapReady", let body = message.body as? [String: Any],
              let slide = (body["slide"] as? NSNumber)?.intValue else { return }
        pageReportedReady(ReadyPayload(revision: body["revision"] as? String ?? "",
                                       slide: slide,
                                       step: (body["step"] as? NSNumber)?.intValue ?? 0,
                                       settled: (body["settled"] as? NSNumber)?.boolValue ?? true))
    }

    /// Records a ready the page reported and passes it on. Internal so a
    /// test can deliver one through the same path the handler uses.
    func pageReportedReady(_ payload: ReadyPayload) {
        lastReady = payload
        onReady?(payload)
    }
}
