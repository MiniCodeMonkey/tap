import XCTest
@testable import TapDesktopCore

final class SlideCardTests: XCTestCase {
    func testTheFirstHeadingIsTheCard() {
        XCTAssertEqual(SlideCard.heading(inSlideMarkdown: "# A Tour of Tap's Themes\n\nOne deck."), "A Tour of Tap's Themes")
        XCTAssertEqual(SlideCard.heading(inSlideMarkdown: "Intro text\n\n## Later heading"), "Later heading", "a heading beats an earlier line of text")
        XCTAssertEqual(SlideCard.heading(inSlideMarkdown: "### Closing ###"), "Closing")
    }

    func testCommentsDirectivesAndFencesArePassedOver() {
        let markdown = "<!-- layout: section -->\n<!--\nlayout: two-column\n-->\n::right\n```js\n# not a heading\n```\n\n## Real one"
        XCTAssertEqual(SlideCard.heading(inSlideMarkdown: markdown), "Real one")
    }

    func testWithoutAHeadingTheFirstLineOfTextIsUsed() {
        XCTAssertEqual(SlideCard.heading(inSlideMarkdown: "\n- First point\n- Second"), "First point")
        XCTAssertEqual(SlideCard.heading(inSlideMarkdown: "> A quote\n\nmore"), "A quote")
        XCTAssertEqual(SlideCard.heading(inSlideMarkdown: "3. Third\n"), "Third")
        XCTAssertEqual(SlideCard.heading(inSlideMarkdown: "<!-- only a comment -->"), "")
        XCTAssertEqual(SlideCard.heading(inSlideMarkdown: ""), "")
    }

    func testInlineMarksComeOff() {
        XCTAssertEqual(SlideCard.heading(inSlideMarkdown: "# **Bold** and `code` and [a link](https://example.com)"), "Bold and code and a link")
        XCTAssertEqual(SlideCard.heading(inSlideMarkdown: "#NotAHeading"), "#NotAHeading", "a hash with no space is text")
    }

    func testADeckIsCutIntoCardsAtSeparators() {
        let deck = """
        ---
        title: Talk
        theme: swiss
        ---

        # One

        ---

        <!-- layout: section -->
        ## Two

        ---
        ```md
        ---
        ```
        ### Three

        ---

        ---
        Four
        """
        XCTAssertEqual(SlideCard.cards(inDeckMarkdown: deck).map(\.heading), ["One", "Two", "Three", "Four"])
    }

    func testADeckWithNoSlidesHasNoCards() {
        XCTAssertEqual(SlideCard.cards(inDeckMarkdown: ""), [])
        XCTAssertEqual(SlideCard.cards(inDeckMarkdown: "---\ntitle: x\n---\n"), [])
    }

    func testASeparatorInsideACommentDoesNotSplit() {
        XCTAssertEqual(SlideCard.cards(inDeckMarkdown: "# A\n<!--\n---\n-->\n\n---\n# B").map(\.heading), ["A", "B"])
    }
}
