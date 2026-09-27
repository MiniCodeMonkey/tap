import XCTest
@testable import Tap

/// The Settings window's General and Live Code panes at the window's real
/// width (760pt), as the SettingsGeneral and SettingsLiveCode boards draw
/// them: each card as tall as its rows, the rows one under the other
/// inside it, the cards one under the other, nothing overlapping. This is
/// the exact defect D5's Deck tab shipped with (an NSBox's contentView
/// set to a stack collapses the box to its title), so every state here is
/// checked on alignment rects, not raw frames.
final class SettingsLayoutTests: HostedTestCase {
    func testGeneralPaneLayout() {
        let general = GeneralSettingsViewController()
        let content = general.view
        content.setFrameSize(NSSize(width: 760, height: 400))
        content.layoutSubtreeIfNeeded()

        let cards = general.cards
        XCTAssertEqual(cards.count, 3, "Editor, New decks, Saving")
        assertRowsDoNotOverlap([
            [general.fontSizePopup],
            [general.lineSpacingControl],
            [general.defaultThemePopup],
            [general.autosavePopup],
        ], in: content)
        for card in cards {
            assertCard(card, holds: card.rows, in: content)
        }
    }

    func testLiveCodePaneLayoutWithNoApprovals() {
        let liveCode = LiveCodeSettingsViewController()
        let content = liveCode.view
        content.setFrameSize(NSSize(width: 760, height: 460))
        content.layoutSubtreeIfNeeded()

        assertRowsDoNotOverlap([
            [liveCode.introLabel],
            [liveCode.table],
            [liveCode.revokeButton, liveCode.revealButton],
            [liveCode.errorLabel],
        ], in: content)
    }

    /// The error state: `errorLabel` shows tap's message under the buttons,
    /// without pushing anything out from under it or off the pane.
    func testLiveCodePaneLayoutWithAnError() {
        let liveCode = LiveCodeSettingsViewController()
        let content = liveCode.view
        content.setFrameSize(NSSize(width: 760, height: 460))
        liveCode.errorLabel.stringValue = "/t/gone.md is not approved to run live code"
        liveCode.errorLabel.isHidden = false
        content.layoutSubtreeIfNeeded()

        assertRowsDoNotOverlap([
            [liveCode.introLabel],
            [liveCode.table],
            [liveCode.revokeButton, liveCode.revealButton],
            [liveCode.errorLabel],
        ], in: content)
    }
}
