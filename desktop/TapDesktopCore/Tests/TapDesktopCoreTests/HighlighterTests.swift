import XCTest
@testable import TapDesktopCore

final class HighlighterTests: XCTestCase {
    func testStylesMarkdownAndTapSyntax() {
        let lines = [
            "<!-- layout: two-column -->",
            "## What We Knew",
            "::left",
            "- a point",
            "<!-- pause -->",
            "---",
            "#hashtag is text",
        ]
        XCTAssertEqual(Highlighter.styles(for: lines), [.directive, .heading, .slotMarker, .text, .directive, .separator, .text])
    }

    func testCodeInsideAFenceIsCodeEvenWhenItLooksLikeMarkdown() {
        let lines = ["```sql {driver: sqlite}", "# not a heading", "---", "```", "# Heading"]
        XCTAssertEqual(Highlighter.styles(for: lines), [.fence, .code, .code, .fence, .heading])
    }

    func testNotesInAMultiLineCommentAreNotes() {
        let lines = ["<!--", "layout: default", "notes:", "Let the room guess.", "Ask who was paged.", "-->", "After"]
        XCTAssertEqual(Highlighter.styles(for: lines), [.directive, .directive, .notes, .notes, .notes, .directive, .text])
    }

    func testAOneLineNotesComment() {
        XCTAssertEqual(Highlighter.styles(for: ["<!-- notes: say hello -->"]), [.notes])
    }

    func testATildeFenceClosesOnlyWithTildes() {
        XCTAssertEqual(Highlighter.styles(for: ["~~~", "```", "~~~"]), [.fence, .code, .fence])
    }
}
