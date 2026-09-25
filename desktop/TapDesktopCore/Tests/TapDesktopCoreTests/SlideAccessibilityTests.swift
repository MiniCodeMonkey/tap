import XCTest
@testable import TapDesktopCore

final class SlideAccessibilityTests: XCTestCase {
    func testTheLabelNamesNumberLayoutTitleAndSteps() {
        let slide = Slide(number: 3, startLine: 1, endLine: 2, layout: "default", title: "What We Knew", fragments: 1, steps: 1)
        XCTAssertEqual(SlideAccessibility.label(for: slide), "Slide 3, default layout, What We Knew, 2 steps")
        XCTAssertEqual(SlideAccessibility.label(for: Slide(number: 1, startLine: 1, endLine: 1, layout: "title", title: "Hello")), "Slide 1, title layout, Hello")
        XCTAssertEqual(SlideAccessibility.label(for: Slide(number: 4, startLine: 1, endLine: 1, layout: "code-focus", title: "", fragments: 0, steps: 1)),
                       "Slide 4, code-focus layout, 1 step")
        XCTAssertEqual(SlideAccessibility.label(for: Slide(number: 2, startLine: 1, endLine: 1, layout: "section", title: "The Page", skip: true)),
                       "Slide 2, section layout, The Page, skipped")
    }

    func testTheDropLabelNamesTheBoundary() {
        XCTAssertEqual(SlideAccessibility.dropLabel(beforeNumber: 3, count: 1), "Drop 1 slide above slide 3")
        XCTAssertEqual(SlideAccessibility.dropLabel(beforeNumber: nil, count: 2), "Drop 2 slides at the end")
    }
}
