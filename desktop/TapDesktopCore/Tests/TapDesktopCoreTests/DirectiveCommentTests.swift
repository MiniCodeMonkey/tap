import XCTest
@testable import TapDesktopCore

final class DirectiveCommentTests: XCTestCase {
    func testFindsTheLeadingCommentInBothForms() throws {
        let single = try XCTUnwrap(DirectiveComment.leading(in: "<!-- layout: title -->\n\n# Hello"))
        XCTAssertEqual(single.range, NSRange(location: 0, length: 22))
        XCTAssertEqual(single.body, " layout: title ")
        let multi = try XCTUnwrap(DirectiveComment.leading(in: "\n<!--\nlayout: cover\nbackground: images/hero.jpg\n-->\n# Big"))
        XCTAssertEqual(multi.range, NSRange(location: 1, length: 50))
        XCTAssertEqual(multi.body, "\nlayout: cover\nbackground: images/hero.jpg\n")
    }

    func testOnlyACommentAtTheStartIsTheDirectiveComment() {
        XCTAssertNil(DirectiveComment.leading(in: "# Hello\n\n<!-- layout: title -->"))
        XCTAssertNil(DirectiveComment.leading(in: "# Hello\n\n<!-- pause -->"))
        XCTAssertNil(DirectiveComment.leading(in: "<!-- never closed"))
        XCTAssertNil(DirectiveComment.leading(in: ""))
    }

    func testSettingAKeyAddsOneLineAndKeepsTheRest() {
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "<!-- layout: title -->\n\n# Hello", setting: "skip", to: "true"),
                       "<!--\nlayout: title\nskip: true\n-->\n\n# Hello", "a single-line comment becomes the multi-line form")
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "# Hello\n\nBody", setting: "skip", to: "true"),
                       "<!--\nskip: true\n-->\n\n# Hello\n\nBody")
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "<!--\nskip: false\n-->\n# Hello", setting: "skip", to: "true"),
                       "<!--\nskip: true\n-->\n# Hello", "an existing key line is replaced, not doubled")
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "<!--\n  layout: cover\n  background:\n    image: hero.jpg\n-->\n# Big", setting: "skip", to: "true"),
                       "<!--\n  layout: cover\n  background:\n    image: hero.jpg\nskip: true\n-->\n# Big", "every other line stays byte for byte, indentation included")
    }

    func testAKeyGoesBeforeTheNotesBlockAndNeverTouchesIt() {
        let slide = "<!--\nlayout: default\nnotes:\nLet the room guess.\n\nskip: this is a sentence in the notes\n-->\n\n## What We Knew"
        XCTAssertEqual(DirectiveComment.rewrite(slideText: slide, setting: "skip", to: "true"),
                       "<!--\nlayout: default\nskip: true\nnotes:\nLet the room guess.\n\nskip: this is a sentence in the notes\n-->\n\n## What We Knew",
                       "the key goes before notes:, and the notes keep their blank line and their own skip: line")
        let block = "<!--\nnotes: |\n  Line one.\n\n  Line two.\n-->\n# H"
        XCTAssertEqual(DirectiveComment.rewrite(slideText: block, setting: "skip", to: "true"),
                       "<!--\nskip: true\nnotes: |\n  Line one.\n\n  Line two.\n-->\n# H", "a block scalar keeps its indentation")
        XCTAssertEqual(DirectiveComment.rewrite(slideText: DirectiveComment.rewrite(slideText: block, setting: "skip", to: "true"), setting: "skip", to: nil),
                       block, "removing the key gives the original back")
    }

    func testRemovingAKeyDropsAnEmptyComment() {
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "<!--\nskip: true\n-->\n\n# Hello", setting: "skip", to: nil), "# Hello")
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "<!--\nlayout: title\nskip: true\n-->\n\n# Hello", setting: "skip", to: nil),
                       "<!--\nlayout: title\n-->\n\n# Hello")
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "<!-- skip: true -->\n# Hello", setting: "skip", to: nil), "# Hello")
        XCTAssertEqual(DirectiveComment.rewrite(slideText: "# Hello", setting: "skip", to: nil), "# Hello")
    }
}
