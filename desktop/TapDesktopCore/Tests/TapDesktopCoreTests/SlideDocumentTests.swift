import XCTest
@testable import TapDesktopCore

final class SlideDocumentTests: XCTestCase {
    /// Frontmatter, three slides, a fence holding a "---" line inside slide 2.
    static let text = """
    ---
    title: Ops
    ---

    # One

    ---

    # Two

    ```text
    ---
    ```

    ---

    # Three

    """

    static func boxes(for text: String) -> [SlideBox] {
        // The ranges tap reports for `text`: each slide from its first line to its last, blank lines outside.
        let nsText = text as NSString
        func range(_ marker: String, to end: String) -> NSRange {
            let start = nsText.range(of: marker).location
            let stop = NSMaxRange(nsText.range(of: end, range: NSRange(location: start, length: nsText.length - start)))
            return NSRange(location: start, length: stop - start)
        }
        return [
            SlideBox(range: range("# One", to: "# One"), slide: Slide(number: 1, startLine: 5, endLine: 5, title: "One")),
            SlideBox(range: range("# Two", to: "```\n"), slide: Slide(number: 2, startLine: 9, endLine: 13, title: "Two")),
            SlideBox(range: range("# Three", to: "# Three"), slide: Slide(number: 3, startLine: 17, endLine: 17, title: "Three")),
        ].map { box in
            // Ranges end at the end of the last line, before its newline.
            var trimmed = box
            while trimmed.range.length > 0, nsText.character(at: NSMaxRange(trimmed.range) - 1) == 10 {
                trimmed.range.length -= 1
            }
            return trimmed
        }
    }

    func testCutsTheTextIntoPrefixSlidesGapsAndSuffix() {
        let document = SlideDocument(text: Self.text, boxes: Self.boxes(for: Self.text))
        XCTAssertEqual(document.prefix, "---\ntitle: Ops\n---\n\n")
        XCTAssertEqual(document.entries.map(\.text), ["# One", "# Two\n\n```text\n---\n```", "# Three"])
        XCTAssertEqual(document.entries.map(\.gapAfter), ["\n\n---\n\n", "\n\n---\n\n", ""])
        XCTAssertEqual(document.suffix, "\n")
        XCTAssertEqual(document.text, Self.text, "cutting and joining is the identity")
    }

    func testBoxesAreRenumberedWithRecomputedLines() {
        var document = SlideDocument(text: Self.text, boxes: Self.boxes(for: Self.text))
        document.entries.swapAt(0, 2)
        document.normalizeGaps()
        let boxes = document.boxes
        XCTAssertEqual(boxes.map(\.slide.number), [1, 2, 3])
        XCTAssertEqual(boxes.map(\.slide.title), ["Three", "Two", "One"])
        XCTAssertEqual(boxes[0].range, NSRange(location: 20, length: 7))
        XCTAssertEqual(boxes.map(\.slide.startLine), [5, 9, 17])
        XCTAssertEqual(boxes.map(\.slide.endLine), [5, 13, 17])
        let text = document.text as NSString
        XCTAssertEqual(text.substring(with: boxes[2].range), "# One")
    }

    func testNormalizeGapsRewritesOnlyNewBoundaries() {
        var document = SlideDocument(text: Self.text, boxes: Self.boxes(for: Self.text))
        document.entries[0].gapAfter = "\n---\n"
        let moved = document.entries.remove(at: 2)
        document.entries.insert(moved, at: 0)
        document.normalizeGaps()
        XCTAssertEqual(document.entries.map(\.slide.title), ["Three", "One", "Two"])
        XCTAssertEqual(document.entries[0].gapAfter, SlideDocument.separator, "a new boundary gets the canonical separator")
        XCTAssertEqual(document.entries[1].gapAfter, "\n---\n", "One and Two were neighbours before, so their gap is kept")
        XCTAssertEqual(document.entries[2].gapAfter, "", "nothing follows the last slide")
    }

    func testAnEmptyDeckIsAllPrefix() {
        let document = SlideDocument(text: "---\ntitle: x\n---\n", boxes: [])
        XCTAssertEqual(document.prefix, "---\ntitle: x\n---\n")
        XCTAssertTrue(document.entries.isEmpty)
        XCTAssertEqual(document.boxes, [])
    }
}
