import XCTest
@testable import TapDesktopCore

final class SlideOutlineTests: XCTestCase {
    let slides = [
        Slide(number: 1, startLine: 1, endLine: 1, layout: "title", title: "Debugging Production at 3am"),
        Slide(number: 2, startLine: 3, endLine: 3, layout: "section", title: "The Page"),
        Slide(number: 5, startLine: 5, endLine: 5, layout: "two-column", title: "Root Cause"),
        Slide(number: 6, startLine: 7, endLine: 7, layout: "default", title: ""),
    ]

    func testAnEmptyQueryListsEverySlide() {
        XCTAssertEqual(SlideOutline.entries(for: slides, matching: "  ").map(\.number), [1, 2, 5, 6])
    }

    func testMatchesTitlesWithoutCaseAndByNumber() {
        XCTAssertEqual(SlideOutline.entries(for: slides, matching: "ro").map(\.number), [1, 5])
        XCTAssertEqual(SlideOutline.entries(for: slides, matching: "ROOT").map(\.title), ["Root Cause"])
        XCTAssertEqual(SlideOutline.entries(for: slides, matching: "6").map(\.number), [6])
    }
}
