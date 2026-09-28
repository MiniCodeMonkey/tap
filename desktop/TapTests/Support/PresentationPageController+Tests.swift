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

    /// Where the page stands, for a failure message: its last ready, its
    /// loads and ended processes, and, in the page, its URL hash and ready
    /// state (window.__tapReadyState).
    func diagnostics() async -> String {
        var inThePage = "no answer in 5 s"
        if case .value(let value) = await webView.evaluate("location.hash + ' ' + JSON.stringify(window.__tapReadyState)", timeout: 5) {
            inThePage = value as? String ?? String(describing: value)
        }
        return "lastReady=\(String(describing: lastReady)) pageLoads=\(pageLoadCount) "
            + "processEnds=\(processTerminationCount) page=\(inThePage)"
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
