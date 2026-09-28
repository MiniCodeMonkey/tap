import XCTest
@testable import Tap

/// The New Component sheet's layout at its real width (the NewComponent
/// board's fixed sheet width): the Name, Kind and TypeScript rows and the
/// buttons stack one under the other with no overlap, and each lies inside
/// the sheet's content, both in the default state and once tap's error
/// grows the sheet to hold it.
final class NewComponentSheetLayoutTests: HostedTestCase {
    func testTheSheetsRowsDoNotOverlapAtItsRealWidth() throws {
        let sheet = NewComponentSheet(slide: 5)
        let content = try XCTUnwrap(sheet.contentView)
        assertRowsDoNotOverlap([[sheet.nameField], [sheet.kindControl], [sheet.typeScriptSwitch], [sheet.declineButton, sheet.acceptButton]], in: content)
    }

    func testTheSheetGrowsToHoldTapsErrorWithoutOverlapping() throws {
        let sheet = NewComponentSheet(slide: 5)
        let content = try XCTUnwrap(sheet.contentView)
        content.layoutSubtreeIfNeeded()
        let heightBefore = sheet.frame.height

        sheet.showError("slides/Counter.jsx already exists")
        content.layoutSubtreeIfNeeded()

        XCTAssertGreaterThan(sheet.frame.height, heightBefore, "the sheet grows to hold the error row")
        assertRowsDoNotOverlap([[sheet.nameField], [sheet.kindControl], [sheet.typeScriptSwitch], [sheet.errorLabel], [sheet.declineButton, sheet.acceptButton]], in: content)
    }
}
