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

    func testABlocksProblemIsAnErrorLineWithAFixIt() {
        let problem = #"This deck does not declare the shell driver. Add "shell: {}" under drivers in the frontmatter."#
        let slide = Slide(number: 6, startLine: 31, endLine: 35, title: "Shell",
                          codeBlocks: [CodeBlock(block: 1, language: "bash", driver: "shell", live: true, line: 33, problem: problem)])
        let header = BoxHeader(slide: slide, declaredDrivers: ["sqlite"])
        XCTAssertEqual(header.errors, ["Line 33: " + problem])
        XCTAssertEqual(header.fixIt, BoxHeader.FixIt(driver: "shell"))
        XCTAssertEqual(header.fixIt?.title, "Allow shell in This Deck")
        XCTAssertEqual(header.badges, ["shell"])
        XCTAssertNil(BoxHeader(slide: slide, declaredDrivers: ["shell"]).fixIt, "declared since tap answered: nothing left to fix")
        XCTAssertNotNil(BoxHeader(slide: slide).fixIt, "with no frontmatter to check, the problem alone offers it")
        let broken = BoxHeader(slide: slide, declaredDrivers: [], frontmatterIsBroken: true)
        XCTAssertNil(broken.fixIt, "a broken frontmatter is the problem to fix, not a missing declaration")
        XCTAssertEqual(broken.errors, header.errors, "the block's problem still shows")
        XCTAssertNil(BoxHeader(slide: Slide(number: 1, startLine: 1, endLine: 2)).fixIt)
        let multiLine = Slide(number: 4, startLine: 17, endLine: 21, codeBlocks: [
            CodeBlock(block: 1, language: "sql", driver: "sqlite", live: true, line: 19,
                      problem: "This deck does not declare the sqlite driver. Add this to the frontmatter:\n\ndrivers:\n  sqlite: {}")])
        XCTAssertEqual(BoxHeader(slide: multiLine).errors, ["Line 19: This deck does not declare the sqlite driver. Add this to the frontmatter: drivers: sqlite: {}"])
        let both = Slide(number: 2, startLine: 1, endLine: 2, errors: [#"Unknown layout "sectoin""#], codeBlocks: slide.codeBlocks)
        XCTAssertEqual(BoxHeader(slide: both).errors.count, 2)
        XCTAssertEqual(BoxHeader(slide: both).errors[0], #"Unknown layout "sectoin""#, "the slide's own errors first")
    }
}
