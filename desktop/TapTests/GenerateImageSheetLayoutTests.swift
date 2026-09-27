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
        content.layoutSubtreeIfNeeded()

        func frame(_ view: NSView) -> NSRect { view.convert(view.bounds, to: content) }
        let rows: [NSView] = [sheet.promptView, sheet.matchThemeSwitch, sheet.aspectControl, sheet.acceptButton, sheet.declineButton]
        let frames = rows.map(frame)
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
}
