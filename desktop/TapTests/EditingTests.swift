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

    /// Undo and redo send the editor's text to tap on their own, with no
    /// further typing, so tap's answer (here a slide title) follows them.
    func testUndoAndRedoResendTheTextToTap() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let editor = try XCTUnwrap(document.sessionController?.editor)
        let original = editor.string
        XCTAssertEqual(editor.boxes[1].slide.title, "The Page")

        let titleEnd = (editor.string as NSString).range(of: "# The Page").upperBound
        editor.setSelectedRange(NSRange(location: titleEnd, length: 0))
        editor.insertText(" Renamed", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitUntil(timeout: 10, "tap's answer for the typed title") { editor.boxes[1].slide.title == "The Page Renamed" }

        editor.undoManager?.undo()
        XCTAssertEqual(editor.string, original, "one undo reverts the typing")
        try await waitUntil(timeout: 5, "tap's answer for the undone title") { editor.boxes[1].slide.title == "The Page" }

        editor.undoManager?.redo()
        XCTAssertNotEqual(editor.string, original, "redo brings the typing back")
        try await waitUntil(timeout: 5, "tap's answer for the redone title") { editor.boxes[1].slide.title == "The Page Renamed" }
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

    /// The bar the way a person meets it: opening a deck whose frontmatter
    /// is already broken on disk. `tap dev --app` falls back to the default
    /// configuration, starts, and reports the parse failure in the slide
    /// list it answers with, so the app has a session to talk to and an
    /// error to show. The frontmatter comes out of hiding with it, because
    /// a person cannot fix what the editor will not let them see.
    func testOpeningADeckBrokenOnDiskShowsTheBarAndTheFrontmatter() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("broken-frontmatter.md"))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 2)
        try await waitUntil(timeout: 30, "the deck errors bar") { controller.editorViewController.bar(.deckErrors) != nil }
        XCTAssertEqual(controller.editor.hiddenLength, 0, "the frontmatter is shown until it is fixed")
        XCTAssertTrue(controller.editor.string.hasPrefix("---\ntitle: [unclosed"), "including the line that does not parse")
        XCTAssertTrue(controller.editorViewController.bar(.deckErrors)?.message.hasPrefix("The deck settings have a problem") ?? false)
    }
}
