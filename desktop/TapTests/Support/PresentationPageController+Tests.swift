import Foundation
@testable import Tap

extension PresentationPageController {
    /// The page's visible text, or "" when the page does not answer in 5 s.
    func pageText() async -> String {
        if case .value(let value) = await webView.evaluate("document.body.innerText", timeout: 5) {
            return value as? String ?? ""
        }
        return ""
    }

    /// Where the page stands, for a failure message. From the app: its
    /// last ready, its loads, ended processes and navigation milestones
    /// (the page's own navigations included), and the web view's URL. From
    /// the page: its URL, how its document was loaded, its text, and its
    /// ready and socket records (window.__tapReadyState and
    /// window.__tapSocketState), so a page that never heard the talk move
    /// shows apart from one that heard it, and a replaced document shows.
    func diagnostics() async -> String {
        let script = """
        JSON.stringify({url: location.pathname + location.search + location.hash, document: document.readyState, \
        navigation: performance.getEntriesByType('navigation').map(entry => entry.type), pageMs: Math.round(performance.now()), \
        text: (document.body ? document.body.innerText : '').slice(0, 120), \
        ready: window.__tapReadyState ?? null, socket: window.__tapSocketState ?? null})
        """
        var inThePage = "no answer in 5 s"
        switch await webView.evaluate(script, timeout: 5) {
        case .value(let value): inThePage = value as? String ?? String(describing: value)
        case .failed(let error): inThePage = "script failed: \(error)"
        case .noAnswer: break
        }
        return "lastReady=\(String(describing: lastReady)) pageLoads=\(pageLoadCount) "
            + "processEnds=\(processTerminationCount) webViewURL=\(webView.url?.absoluteString ?? "none") "
            + "loading=\(webView.isLoading) navigation=[\(navigationMilestoneDescription)] page=\(inThePage)"
    }

    /// Presses `key` (a KeyboardEvent key name, "ArrowRight" or "o") in
    /// the page, the way the page's own handler on `window` sees it, so a
    /// key test does not depend on which window the host has as key.
    func pressKey(_ key: String) async {
        let encoded = String(decoding: (try? JSONEncoder().encode([key])) ?? Data("[\"\"]".utf8), as: UTF8.self)
        let script = "window.dispatchEvent(new KeyboardEvent('keydown', {key: \(encoded)[0], bubbles: true, cancelable: true})); true"
        _ = await webView.evaluate(script, timeout: 5)
    }
}
