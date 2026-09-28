import XCTest
@testable import Tap

/// The Generate Image sheet's layout at its real width (the GenerateImage
/// board's fixed sheet width): the prompt area, the Style row, the Aspect
/// row and the buttons stack one under the other with no overlap, and
/// each lies inside the sheet's content, in the default state and once
/// tap's error grows the sheet to hold it. The prompt is a real text
/// view that takes typing.
final class GenerateImageSheetLayoutTests: HostedTestCase {
    func testTheSheetsRowsDoNotOverlapAtItsRealWidth() throws {
        let sheet = GenerateImageSheet(slide: 5, themeName: "Terminal")
        let content = try XCTUnwrap(sheet.contentView)
        assertRowsDoNotOverlap([[sheet.promptView], [sheet.matchThemeSwitch], [sheet.aspectControl], [sheet.declineButton, sheet.acceptButton]], in: content)
    }

    func testThePromptTakesTyping() throws {
        let sheet = GenerateImageSheet(slide: 5, themeName: "Terminal")
        let content = try XCTUnwrap(sheet.contentView)
        content.layoutSubtreeIfNeeded()
        let scroll = try XCTUnwrap(sheet.promptView.enclosingScrollView)
        XCTAssertGreaterThan(sheet.promptView.frame.width, 0, "the text view has the box's width, so a click in the box lands on it")
        XCTAssertEqual(sheet.promptView.frame.width, scroll.contentView.bounds.width, accuracy: 0.5, "it tracks the scroll view's width")
        XCTAssertGreaterThan(sheet.promptView.textContainer?.size.width ?? 0, 0, "its text container lays out text")
        XCTAssertIdentical(sheet.initialFirstResponder, sheet.promptView, "the sheet opens with the caret in the prompt")
        XCTAssertTrue(sheet.makeFirstResponder(sheet.promptView), "the prompt takes the keyboard")
        XCTAssertFalse(sheet.generateButton.isEnabled, "no prompt yet")

        // Typing, as the keyboard does it: insertText posts the change the sheet listens for.
        sheet.promptView.insertText("a red fox at dusk", replacementRange: sheet.promptView.selectedRange())
        XCTAssertEqual(sheet.request.prompt, "a red fox at dusk")
        XCTAssertTrue(sheet.generateButton.isEnabled, "typing a prompt turns Generate on")
    }

    func testTheSheetGrowsToHoldTapsErrorWithoutOverlapping() throws {
        let sheet = GenerateImageSheet(slide: 5, themeName: "Terminal")
        let content = try XCTUnwrap(sheet.contentView)
        content.layoutSubtreeIfNeeded()
        let heightBefore = sheet.frame.height

        sheet.showError("GEMINI_API_KEY is not set. Set it in Settings > Image Generation, or export it in your shell.", code: "no_api_key")
        content.layoutSubtreeIfNeeded()

        XCTAssertGreaterThan(sheet.frame.height, heightBefore, "the sheet grows to hold the error row and the Settings button")
        assertRowsDoNotOverlap([[sheet.promptView], [sheet.matchThemeSwitch], [sheet.aspectControl], [sheet.errorLabel], [sheet.settingsButton],
                                [sheet.declineButton, sheet.acceptButton]], in: content)

        sheet.promptView.insertText("x", replacementRange: sheet.promptView.selectedRange())
        content.layoutSubtreeIfNeeded()
        XCTAssertEqual(sheet.frame.height, heightBefore, accuracy: 0.5, "typing hides the error and the sheet shrinks back")
    }
}
