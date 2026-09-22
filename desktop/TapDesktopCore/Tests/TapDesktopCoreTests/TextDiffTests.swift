import XCTest
@testable import TapDesktopCore

final class TextDiffTests: XCTestCase {
    func testEqualTextsNeedNoReplacement() {
        XCTAssertNil(TextDiff.replacement(from: "same", to: "same"))
    }

    func testFindsTheChangedMiddle() {
        XCTAssertEqual(TextDiff.replacement(from: "# One\n\n# Two\n", to: "# One\n\n# Deux\n"),
                       TextReplacement(range: NSRange(location: 9, length: 3), replacement: "Deux"))
    }

    func testHandlesInsertionsAndDeletionsAtTheEdges() {
        XCTAssertEqual(TextDiff.replacement(from: "abc", to: "xabc"), TextReplacement(range: NSRange(location: 0, length: 0), replacement: "x"))
        XCTAssertEqual(TextDiff.replacement(from: "abc", to: "ab"), TextReplacement(range: NSRange(location: 2, length: 1), replacement: ""))
        XCTAssertEqual(TextDiff.replacement(from: "", to: "new"), TextReplacement(range: NSRange(location: 0, length: 0), replacement: "new"))
    }

    func testNeverSplitsASurrogatePair() {
        // U+1F600 and U+1F601 share their first UTF-16 unit.
        let replacement = TextDiff.replacement(from: "a\u{1F600}b", to: "a\u{1F601}b")
        XCTAssertEqual(replacement, TextReplacement(range: NSRange(location: 1, length: 2), replacement: "\u{1F601}"))
    }

    func testApplyingTheReplacementGivesTheNewText() {
        let old = "---\ntitle: A\n---\n\n# One\n\n---\n\n# Two\n"
        let new = "---\ntitle: B\n---\n\n# One\n\n---\n\n# Two, again\n"
        let replacement = try! XCTUnwrap(TextDiff.replacement(from: old, to: new))
        XCTAssertEqual((old as NSString).replacingCharacters(in: replacement.range, with: replacement.replacement), new)
    }
}
