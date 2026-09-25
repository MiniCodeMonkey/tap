import Foundation
@testable import Tap

extension PresentationPageController {
    /// The page's visible text.
    func pageText() async -> String {
        (try? await webView.evaluateJavaScript("document.body.innerText") as? String) ?? ""
    }

    /// Presses `key` (a KeyboardEvent key name, "ArrowRight" or "o") in
    /// the page, the way the page's own handler on `window` sees it, so a
    /// key test does not depend on which window the host has as key.
    func pressKey(_ key: String) async {
        let encoded = String(decoding: (try? JSONEncoder().encode([key])) ?? Data("[\"\"]".utf8), as: UTF8.self)
        let script = "window.dispatchEvent(new KeyboardEvent('keydown', {key: \(encoded)[0], bubbles: true, cancelable: true})); true"
        _ = try? await webView.evaluateJavaScript(script)
    }
}
