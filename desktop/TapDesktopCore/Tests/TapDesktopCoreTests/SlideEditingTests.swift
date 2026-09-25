import XCTest
@testable import TapDesktopCore

final class SlideEditingTests: XCTestCase {
    /// Seven one-line slides after a frontmatter. Slide 4 holds a fence with a "---" line.
    static let text = """
    ---
    title: Ops
    ---

    # One

    ---

    # Two

    ---

    <!-- layout: section -->
    # Three

    ---

    # Four

    ```text
    ---
    ```

    ---

    # Five

    ---

    # Six

    ---

    # Seven

    """

    static let boxes: [SlideBox] = {
        let nsText = text as NSString
        let starts = ["# One", "# Two", "<!-- layout: section -->", "# Four", "# Five", "# Six", "# Seven"]
        let ends = ["# One", "# Two", "# Three", "```\n\n---\n\n# Five", "# Five", "# Six", "# Seven"]
        return zip(starts, ends).enumerated().map { index, pair in
            let start = nsText.range(of: pair.0).location
            var end: Int
            if index == 3 {
                end = nsText.range(of: "```\n\n---\n\n# Five").location + 3
            } else {
                end = NSMaxRange(nsText.range(of: pair.1, range: NSRange(location: start, length: nsText.length - start)))
            }
            return SlideBox(range: NSRange(location: start, length: end - start),
                            slide: Slide(number: index + 1, startLine: 0, endLine: 0, title: ["One", "Two", "Three", "Four", "Five", "Six", "Seven"][index]))
        }
    }()

    func titles(_ result: SlideEditResult?) -> [String] {
        (result?.boxes ?? []).map(\.slide.title)
    }

    /// "---" lines outside every slide's range: the frontmatter's two and
    /// one per boundary. A "---" inside a fenced block sits inside a
    /// slide's range and is never counted.
    func separatorLinesOutsideSlides(_ result: SlideEditResult) -> Int {
        let document = SlideDocument(text: result.text, boxes: result.boxes)
        func count(_ string: String) -> Int {
            string.components(separatedBy: "\n").filter { $0.trimmingCharacters(in: .whitespaces) == "---" }.count
        }
        return count(document.prefix) + document.entries.map { count($0.gapAfter) }.reduce(0, +) + count(document.suffix)
    }

    func testMoveOneSlideAboveAnother() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.move(numbers: [5], beforeNumber: 3), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(result), ["One", "Two", "Five", "Three", "Four", "Six", "Seven"])
        XCTAssertEqual(result.selectedNumbers, [3])
        XCTAssertEqual(result.sourceNumbers, [1, 2, 5, 3, 4, 6, 7])
        XCTAssertEqual(separatorLinesOutsideSlides(result), 2 + 7 - 1, "the frontmatter's two and one between each pair; the fence's --- is inside slide 4")
        XCTAssertTrue(result.boxes.map(\.slide.number).allSatisfy { number in
            let document = SlideDocument(text: result.text, boxes: result.boxes)
            return number == 7 || document.entries[number - 1].gapAfter.components(separatedBy: "\n").filter { $0 == "---" }.count == 1
        }, "exactly one --- in every gap")
        XCTAssertEqual(result.caret, result.boxes[2].range.location)
        XCTAssertTrue(result.text.hasPrefix("---\ntitle: Ops\n---\n\n# One"), "the frontmatter never moves")
    }

    func testMoveSeveralSlidesKeepsTheirOrderAsOneBlock() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.move(numbers: [6, 5], beforeNumber: 3), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(result), ["One", "Two", "Five", "Six", "Three", "Four", "Seven"])
        XCTAssertEqual(result.selectedNumbers, [3, 4])
    }

    func testMoveToTheEndAndToTheTop() throws {
        let toEnd = try XCTUnwrap(SlideEditing.apply(.move(numbers: [2], beforeNumber: nil), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(toEnd), ["One", "Three", "Four", "Five", "Six", "Seven", "Two"])
        XCTAssertFalse(toEnd.text.hasSuffix("---\n"), "no separator dangles after the last slide")
        let toTop = try XCTUnwrap(SlideEditing.apply(.move(numbers: [7], beforeNumber: 1), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(toTop), ["Seven", "One", "Two", "Three", "Four", "Five", "Six"])
        XCTAssertTrue(toTop.text.hasPrefix("---\ntitle: Ops\n---\n\n# Seven\n\n---\n\n# One"))
    }

    func testAMoveThatChangesNothingIsRefused() {
        XCTAssertNil(SlideEditing.apply(.move(numbers: [3], beforeNumber: 4), to: Self.text, boxes: Self.boxes))
        XCTAssertNil(SlideEditing.apply(.move(numbers: [3], beforeNumber: 3), to: Self.text, boxes: Self.boxes))
        XCTAssertNil(SlideEditing.apply(.move(numbers: [7], beforeNumber: nil), to: Self.text, boxes: Self.boxes))
        XCTAssertNil(SlideEditing.apply(.move(numbers: [9], beforeNumber: 1), to: Self.text, boxes: Self.boxes))
        XCTAssertNil(SlideEditing.apply(.move(numbers: [], beforeNumber: 1), to: Self.text, boxes: Self.boxes))
    }

    func testAFenceWithASeparatorLineMovesWhole() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.move(numbers: [4], beforeNumber: 1), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(result), ["Four", "One", "Two", "Three", "Five", "Six", "Seven"])
        let four = (result.text as NSString).substring(with: result.boxes[0].range)
        XCTAssertEqual(four, "# Four\n\n```text\n---\n```", "the fence and its --- line stay inside the slide")
    }

    func testMoveCarriesTheCaretOffset() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.move(numbers: [3], beforeNumber: 2), to: Self.text, boxes: Self.boxes, caretOffsetInSlide: 30))
        XCTAssertEqual(result.caret, result.boxes[1].range.location + 30)
        let clamped = try XCTUnwrap(SlideEditing.apply(.move(numbers: [3], beforeNumber: 2), to: Self.text, boxes: Self.boxes, caretOffsetInSlide: 500))
        XCTAssertEqual(clamped.caret, NSMaxRange(clamped.boxes[1].range), "an offset past the slide's end lands at its end")
    }

    func testDuplicatePutsCopiesAfterTheLastSelectedSlide() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.duplicate(numbers: [3]), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(result), ["One", "Two", "Three", "Three", "Four", "Five", "Six", "Seven"])
        XCTAssertEqual(result.selectedNumbers, [4])
        XCTAssertEqual(result.sourceNumbers, [1, 2, 3, nil, 4, 5, 6, 7], "the copy has no image yet")
        XCTAssertEqual((result.text as NSString).substring(with: result.boxes[3].range), "<!-- layout: section -->\n# Three")
        let several = try XCTUnwrap(SlideEditing.apply(.duplicate(numbers: [2, 3]), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(several), ["One", "Two", "Three", "Two", "Three", "Four", "Five", "Six", "Seven"])
        XCTAssertEqual(several.selectedNumbers, [4, 5])
    }

    func testDeleteRemovesSlidesAndTheirSeparators() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.delete(numbers: [5, 6]), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(titles(result), ["One", "Two", "Three", "Four", "Seven"])
        XCTAssertEqual(separatorLinesOutsideSlides(result), 2 + 4)
        XCTAssertEqual(result.selectedNumbers, [5], "the slide now at the first deleted position")
        let last = try XCTUnwrap(SlideEditing.apply(.delete(numbers: [7]), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(last.selectedNumbers, [6])
        XCTAssertFalse(last.text.contains("# Six\n\n---\n\n\n"), "the separator before the deleted last slide goes with it")
    }

    func testDeletingEverySlideIsRefused() {
        XCTAssertNil(SlideEditing.apply(.delete(numbers: Array(1...7)), to: Self.text, boxes: Self.boxes))
    }

    func testInsertAfterTheFrontmatterAndAfterASlide() throws {
        let template = "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n"
        let top = try XCTUnwrap(SlideEditing.apply(.insert(markdowns: [template], beforeNumber: 1), to: Self.text, boxes: Self.boxes))
        XCTAssertTrue(top.text.hasPrefix("---\ntitle: Ops\n---\n\n<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n\n---\n\n# One"))
        XCTAssertEqual(top.selectedNumbers, [1])
        XCTAssertEqual(top.boxes.count, 8)
        let after3 = try XCTUnwrap(SlideEditing.apply(.insert(markdowns: [template], beforeNumber: 4), to: Self.text, boxes: Self.boxes))
        XCTAssertEqual(after3.boxes[3].slide.number, 4)
        XCTAssertEqual((after3.text as NSString).substring(with: after3.boxes[3].range), "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription")
        XCTAssertEqual(SlideEditing.actionName(for: .insert(markdowns: [template], beforeNumber: nil)), "New Slide")
    }

    func testInsertIntoAnEmptyDeck() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.insert(markdowns: ["# First\n"], beforeNumber: nil), to: "---\ntitle: x\n---\n", boxes: []))
        XCTAssertEqual(result.text, "---\ntitle: x\n---\n\n# First")
        XCTAssertEqual(result.boxes.count, 1)
    }

    func testSetSkipRewritesTheDirectiveComment() throws {
        let result = try XCTUnwrap(SlideEditing.apply(.setSkip(numbers: [3, 4], skipped: true), to: Self.text, boxes: Self.boxes))
        let text = result.text as NSString
        XCTAssertEqual(text.substring(with: result.boxes[2].range), "<!--\nlayout: section\nskip: true\n-->\n# Three")
        XCTAssertEqual(text.substring(with: result.boxes[3].range), "<!--\nskip: true\n-->\n\n# Four\n\n```text\n---\n```")
        let back = try XCTUnwrap(SlideEditing.apply(.setSkip(numbers: [3, 4], skipped: false), to: result.text, boxes: result.boxes))
        XCTAssertEqual(back.text, Self.text.replacingOccurrences(of: "<!-- layout: section -->", with: "<!--\nlayout: section\n-->"))
        XCTAssertNil(SlideEditing.apply(.setSkip(numbers: [1], skipped: false), to: Self.text, boxes: Self.boxes), "unskipping a slide that is not skipped changes nothing")
    }

    func testActionNames() {
        XCTAssertEqual(SlideEditing.actionName(for: .move(numbers: [1], beforeNumber: nil)), "Move Slide")
        XCTAssertEqual(SlideEditing.actionName(for: .move(numbers: [1, 2], beforeNumber: nil)), "Move 2 Slides")
        XCTAssertEqual(SlideEditing.actionName(for: .duplicate(numbers: [1])), "Duplicate Slide")
        XCTAssertEqual(SlideEditing.actionName(for: .delete(numbers: [1, 2, 3])), "Delete 3 Slides")
        XCTAssertEqual(SlideEditing.actionName(for: .insert(markdowns: ["a", "b"], beforeNumber: nil)), "Insert 2 Slides")
        XCTAssertEqual(SlideEditing.actionName(for: .setSkip(numbers: [1], skipped: true)), "Skip Slide")
        XCTAssertEqual(SlideEditing.actionName(for: .setSkip(numbers: [1, 2], skipped: false)), "Unskip 2 Slides")
    }

    func testFirstSlotRangeSelectsThePlaceholderAfterTheMarker() {
        XCTAssertEqual(SlideEditing.firstSlotRange(inSlideText: "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription"),
                       NSRange(location: 29, length: 4), "the text after \"# \"")
        XCTAssertEqual(SlideEditing.firstSlotRange(inSlideText: "::left\n\nLeft content\n\n::right\n\nRight content"),
                       NSRange(location: 8, length: 12), "a slot marker is skipped")
        XCTAssertEqual(SlideEditing.firstSlotRange(inSlideText: "<!--\nlayout: code-focus\n-->\n\n```\n// Your code here\n```"),
                       NSRange(location: 33, length: 17), "inside a fence, the first code line")
        XCTAssertEqual(SlideEditing.firstSlotRange(inSlideText: ""), NSRange(location: 0, length: 0))
    }
}
