import XCTest
@testable import TapDesktopCore

final class BoxHeaderTests: XCTestCase {
    func testBoxBadges() {
        let twoSteps = BoxHeader(slide: Slide(number: 3, startLine: 1, endLine: 4, layout: "default", title: "What We Knew", fragments: 2))
        XCTAssertEqual(twoSteps.badges, ["2 steps"])
        XCTAssertEqual(BoxHeader(slide: Slide(number: 1, startLine: 1, endLine: 1, steps: 1)).badges, ["1 step"])
        XCTAssertEqual(BoxHeader(slide: Slide(number: 1, startLine: 1, endLine: 1)).badges, [])

        let live = BoxHeader(slide: Slide(number: 4, startLine: 1, endLine: 5, layout: "code-focus", title: "Query",
                                          codeBlocks: [CodeBlock(block: 1, language: "sql", driver: "sqlite", live: true, line: 3),
                                                       CodeBlock(block: 2, language: "sql", driver: "sqlite", live: true, line: 9),
                                                       CodeBlock(block: 3, language: "go", driver: "", live: false, line: 12)]))
        XCTAssertEqual(live.badges, ["sqlite"], "one live-code badge per driver")
    }

    func testTheMetaLineHasTheLayoutTitleAndLiveBlocks() {
        let header = BoxHeader(slide: Slide(number: 4, startLine: 1, endLine: 5, layout: "code-focus", title: "Query",
                                            codeBlocks: [CodeBlock(block: 1, language: "sql", driver: "sqlite", live: true, line: 3)]))
        XCTAssertEqual(header.number, "4")
        XCTAssertEqual(header.meta, "code-focus · Query · sql, live")
        XCTAssertEqual(BoxHeader(slide: Slide(number: 2, startLine: 1, endLine: 1, layout: "section", title: "")).meta, "section")
    }

    func testASkippedSlideAndErrorsShowInTheHeader() {
        let header = BoxHeader(slide: Slide(number: 2, startLine: 1, endLine: 1, skip: true, errors: [#"Unknown layout "sectoin""#]))
        XCTAssertEqual(header.badges, ["skipped"])
        XCTAssertEqual(header.errors, [#"Unknown layout "sectoin""#])
    }
}
