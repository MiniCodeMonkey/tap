import Foundation
@testable import Tap

/// Reads of the audience page for the live code tests, through D2's
/// test-only `pageValue`. The page renders the current slide alone, so a
/// test moves the cursor to the slide first and reads that slide's block.
extension PreviewViewController {
    /// The page's Run buttons in DOM order, as a JSON list: "Run" for a
    /// block that can run, "Not approved" for a declared driver this run
    /// does not allow, "disabled" while a block runs.
    func runButtonLabels() async -> String {
        await pageValue("JSON.stringify(Array.from(document.querySelectorAll('.run-button')).map(b => b.classList.contains('not-approved') ? 'Not approved' : (b.disabled ? 'disabled' : 'Run')))")
    }

    /// Clicks the page's Run button, as a person would; "none" when the slide has no enabled one.
    func clickRunButton() async -> String {
        await pageValue("(() => { const b = document.querySelector('.run-button:not(.not-approved)'); if (!b) return 'none'; b.click(); return 'clicked'; })()")
    }

    /// The text of the block's result, "" until one shows.
    func runResultText() async -> String {
        await pageValue("document.querySelector('.result-content')?.innerText ?? ''")
    }

    /// tap's message in the block for a driver the deck does not declare, "" when none.
    func blockProblemText() async -> String {
        await pageValue("document.querySelector('.live-code-problem')?.innerText ?? ''")
    }
}
