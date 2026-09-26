import Foundation
@testable import Tap

/// Reads of the audience page for the live code tests. Each returns the
/// string the script evaluates to, itself, never a description of the
/// optional WebKit hands back (`pageValue` is for diagnostics and reads
/// "Optional(...)"). The page renders the current slide alone, so a test
/// moves the cursor to the slide first and reads that slide's block.
extension PreviewViewController {
    /// The string `script` evaluates to; a script that fails or does not
    /// answer reads as what happened, which no expected value equals.
    func liveCodeValue(_ script: String, timeout: TimeInterval = 3) async -> String {
        switch await webView.evaluate(script, timeout: timeout) {
        case .value(let value): return value as? String ?? "not a string: \(String(describing: value))"
        case .failed(let error): return "script failed: \(error)"
        case .noAnswer(let seconds): return "no answer in \(Int(seconds)) s"
        }
    }

    /// The page's Run buttons in DOM order, as a JSON list: "Run" for a
    /// block that can run, "Not approved" for a declared driver this run
    /// does not allow, "disabled" while a block runs.
    func runButtonLabels() async -> String {
        await liveCodeValue("JSON.stringify(Array.from(document.querySelectorAll('.run-button')).map(b => b.classList.contains('not-approved') ? 'Not approved' : (b.disabled ? 'disabled' : 'Run')))")
    }

    /// Clicks the page's Run button, as a person would; "none" when the slide has no enabled one.
    func clickRunButton() async -> String {
        await liveCodeValue("(() => { const b = document.querySelector('.run-button:not(.not-approved)'); if (!b) return 'none'; b.click(); return 'clicked'; })()")
    }

    /// The text of the block's result, "" until one shows.
    func runResultText() async -> String {
        await liveCodeValue("document.querySelector('.result-content')?.innerText ?? ''")
    }

    /// tap's message in the block for a driver the deck does not declare, "" when none.
    func blockProblemText() async -> String {
        await liveCodeValue("document.querySelector('.live-code-problem')?.innerText ?? ''")
    }
}
