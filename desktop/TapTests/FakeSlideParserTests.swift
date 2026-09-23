import XCTest

/// `FakeSlideParser` is a stand-in for tap's own slide parsing (see the
/// comment on `FakeTap`), not a specification of it. These two behaviours
/// are the exception: the fake already claims to segment slides and count
/// code blocks, and getting an existing claim wrong is a bug the no-growth
/// ruling on the fake does not protect. Both are checked here against the
/// real rules in `internal/parser/fences.go` and `internal/slidelist/slidelist.go`.
final class FakeSlideParserTests: XCTestCase {
    /// `internal/parser/parser.go`'s `SplitSlidesPreservingCodeBlocks` never
    /// splits on a "---" line inside a fenced code block, and its fence
    /// tracker (`internal/parser/fences.go`) closes a fence only on a run of
    /// the same character at least as long as the opener. This fixture's
    /// fence opens with four backticks, so a three-backtick line inside it
    /// must not close the fence early, and the "---" inside it must not
    /// split the slide.
    func testSeparatorInsideAFencedCodeBlockIsNotASplit() {
        let text = """
        # One

        ````text
        ---
        ```
        still inside
        ````

        ---

        # Two
        """

        let (slides, errors) = FakeSlideParser.parse(text)

        XCTAssertTrue(errors.isEmpty, "\(errors)")
        XCTAssertEqual(slides.count, 2, "the --- inside the fence must not split the slide")
        XCTAssertEqual(slides.first?.title, "One")
        XCTAssertEqual(slides.last?.title, "Two")
    }

    /// `internal/slidelist/slidelist.go`'s `CodeBlock` doc comment: "A
    /// ```component fence is not a code block." `internal/parser/codeblocks.go`
    /// excludes it by matching `component <path>` in the fence's info string.
    func testComponentFenceIsNotACodeBlock() {
        let text = """
        # One

        ```component ./Widget.jsx
        {}
        ```

        ```python {driver: sqlite}
        print(1)
        ```

        ---

        # Two
        """

        let (slides, errors) = FakeSlideParser.parse(text)

        XCTAssertTrue(errors.isEmpty, "\(errors)")
        let one = try? XCTUnwrap(slides.first)
        XCTAssertEqual(one?.codeBlocks.count, 1, "the component fence must not be counted")
        XCTAssertEqual(one?.codeBlocks.first?.language, "python")
    }
}
