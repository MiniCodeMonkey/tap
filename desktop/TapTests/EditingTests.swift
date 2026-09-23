import XCTest
@testable import Tap

final class EditingTests: HostedTestCase {
    func testBoxesComeFromTap() async throws {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        let editor = try XCTUnwrap(document.sessionController?.editor)
        XCTAssertEqual(editor.boxes.map(\.slide.number), Array(1...7))
        let text = editor.string as NSString
        XCTAssertEqual(text.substring(with: editor.boxes[0].range), "<!-- layout: title -->\n\n# Debugging Production at 3am\n\nWhat the pager doesn't tell you")
        XCTAssertEqual(editor.header(forBoxAt: 0).number, "1")
        XCTAssertEqual(editor.header(forBoxAt: 0).meta, "title · Debugging Production at 3am")
        XCTAssertEqual(editor.header(forBoxAt: 2).badges, ["1 step"])
        XCTAssertEqual(editor.hiddenLength, text.range(of: "<!-- layout: title -->").location, "slide 1 is the first box")
    }

    func testOpenAFileThatIsNotADeck() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        try await waitForBoxes(document, count: 1)
        let editor = try XCTUnwrap(document.sessionController?.editor)
        XCTAssertEqual(editor.hiddenLength, 0)
        XCTAssertEqual(editor.boxes[0].range.location, 0)
    }

    func testTypingASeparatorSplitsASlide() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("split.md"))
        try await waitForBoxes(document, count: 3)
        let editor = try XCTUnwrap(document.sessionController?.editor)
        let emptyLine = (editor.string as NSString).range(of: "Before\n\n").upperBound
        editor.setSelectedRange(NSRange(location: emptyLine, length: 0))
        for character in "---" {
            editor.insertText(String(character), replacementRange: NSRange(location: NSNotFound, length: 0))
        }
        try await waitForBoxes(document, count: 4)
        let text = editor.string as NSString
        XCTAssertEqual(editor.boxes[2].slide.title, "Three")
        XCTAssertEqual(text.substring(with: editor.boxes[3].range), "After")
    }

    func testParseErrorInOneSlide() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let editor = try XCTUnwrap(document.sessionController?.editor)
        let errors = editor.header(forBoxAt: 1).errors
        XCTAssertFalse(errors.isEmpty)
        XCTAssertTrue(errors.joined().contains("sectoin"), "\(errors)")
        let style = try XCTUnwrap(editor.textStorage?.attribute(.paragraphStyle, at: editor.boxes[1].range.location, effectiveRange: nil) as? NSParagraphStyle)
        XCTAssertGreaterThanOrEqual(style.paragraphSpacingBefore, EditorTextView.headerHeight + EditorTextView.errorLineHeight,
                                    "the message has its own line under the header")
    }

    /// `live` is defined in `internal/slidelist/slidelist.go` (`CodeBlock.Live`),
    /// computed there as `block.Meta.Driver != ""`, and this test reads it
    /// from the bundled tap's own answer.
    func testCodeBlocksWithALiveDriver() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let header = try XCTUnwrap(document.sessionController?.editor.header(forBoxAt: 3))
        XCTAssertTrue(header.meta.hasSuffix("sql, live"), header.meta)
        XCTAssertTrue(header.badges.contains("sqlite"))
    }

    /// The bar is reached by a programmatic edit. `replaceText` sets
    /// `isApplyingProgrammaticEdit`, which is the flag `shouldChangeText`
    /// reads in order to skip the clamp, so this is the path the app itself
    /// uses to load the disk version, not a path a person can take: someone
    /// typing into the hidden frontmatter is beeped and refused.
    ///
    /// The route a person takes to this bar is opening a deck whose
    /// frontmatter is already broken on disk, and that cannot be tested from
    /// here: `tap dev --app` answers such a file with an invalid_deck error
    /// event and exits, so no session ever runs to answer a PUT. What is
    /// asserted below, the bar and the frontmatter coming out of hiding once
    /// the slide list carries an error, is the same end state either route
    /// produces.
    func testDeckErrorsShowABarAndTheFrontmatter() async throws {
        let document = try await openDeck(try Fixtures.copyAppFixture())
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 4)
        // The first match is the frontmatter's title, not the heading below it.
        let title = (controller.editor.string as NSString).range(of: "App Mode Fixture")
        controller.editor.replaceText(in: title, with: "[unclosed", actionName: "Break the frontmatter")
        try await waitUntil(timeout: 30, "the deck errors bar") { controller.editorViewController.bar(.deckErrors) != nil }
        XCTAssertEqual(controller.editor.hiddenLength, 0)
        XCTAssertTrue(controller.editorViewController.bar(.deckErrors)?.message.hasPrefix("The deck settings have a problem") ?? false)
    }
}
