import XCTest
@testable import Tap

/// The New Component sheet's layout at its real width (the NewComponent
/// board's fixed sheet width): the Name, Kind and TypeScript rows and the
/// buttons stack one under the other with no overlap, and each lies inside
/// the sheet's content, both in the default state and once tap's error
/// grows the sheet to hold it.
final class NewComponentSheetLayoutTests: HostedTestCase {
    private func frame(_ view: NSView, in content: NSView) -> NSRect { view.convert(view.bounds, to: content) }

    private func assertRowsDoNotOverlap(_ rows: [NSView], in content: NSView) {
        content.layoutSubtreeIfNeeded()
        let frames = rows.map { frame($0, in: content) }
        for (view, rowFrame) in zip(rows, frames) {
            XCTAssertGreaterThan(rowFrame.width, 0, "\(view) has a real width")
            XCTAssertGreaterThan(rowFrame.height, 0, "\(view) has a real height")
            XCTAssertTrue(content.bounds.insetBy(dx: -0.5, dy: -0.5).contains(rowFrame), "\(rowFrame) lies inside the sheet \(content.bounds)")
        }
        for (index, rowFrame) in frames.enumerated() {
            for other in frames[(index + 1)...] {
                XCTAssertLessThanOrEqual(rowFrame.intersection(other).height, 0.5, "\(rowFrame) and \(other) do not overlap")
            }
        }
    }

    func testTheSheetsRowsDoNotOverlapAtItsRealWidth() throws {
        let sheet = NewComponentSheet(slide: 5)
        let content = try XCTUnwrap(sheet.contentView)
        assertRowsDoNotOverlap([sheet.nameField, sheet.kindControl, sheet.typeScriptSwitch, sheet.acceptButton, sheet.declineButton], in: content)
    }

    func testTheSheetGrowsToHoldTapsErrorWithoutOverlapping() throws {
        let sheet = NewComponentSheet(slide: 5)
        let content = try XCTUnwrap(sheet.contentView)
        content.layoutSubtreeIfNeeded()
        let heightBefore = sheet.frame.height

        sheet.showError("slides/Counter.jsx already exists")
        content.layoutSubtreeIfNeeded()

        XCTAssertGreaterThan(sheet.frame.height, heightBefore, "the sheet grows to hold the error row")
        assertRowsDoNotOverlap([sheet.nameField, sheet.kindControl, sheet.typeScriptSwitch, sheet.errorLabel, sheet.acceptButton, sheet.declineButton], in: content)
    }
}
