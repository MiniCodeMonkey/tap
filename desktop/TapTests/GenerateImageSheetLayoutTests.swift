import XCTest
@testable import Tap

/// The Generate Image sheet's layout at its real width (the GenerateImage
/// board's fixed sheet width): the prompt area, the Style row, the Aspect
/// row and the buttons stack one under the other with no overlap, and
/// each lies inside the sheet's content.
final class GenerateImageSheetLayoutTests: HostedTestCase {
    func testTheSheetsRowsDoNotOverlapAtItsRealWidth() throws {
        let sheet = GenerateImageSheet(slide: 5, themeName: "Terminal")
        let content = try XCTUnwrap(sheet.contentView)
        assertRowsDoNotOverlap([[sheet.promptView], [sheet.matchThemeSwitch], [sheet.aspectControl], [sheet.declineButton, sheet.acceptButton]], in: content)
    }
}
