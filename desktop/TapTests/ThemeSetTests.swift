import XCTest
@testable import Tap

/// A theme pick, from the toolbar's popover or the Deck card's Theme row,
/// rewrites the theme line through the editor's edit path: one undo step,
/// for a deck with a file and one without alike.
final class ThemeSetTests: HostedTestCase {
    func openThemed() async throws -> (DeckDocument, DeckSessionController, DeckWindowController) {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("themed.md"))
        return (document, try XCTUnwrap(document.sessionController), try XCTUnwrap(document.windowControllers.first as? DeckWindowController))
    }

    func openUntitled() async throws -> (DeckDocument, DeckSessionController, DeckWindowController) {
        let text = try String(contentsOf: Fixtures.repositoryRoot.appendingPathComponent("examples/theme-tour.md"), encoding: .utf8)
        let document = try DeckDocument.makeUntitled(text: text)
        let controller = try XCTUnwrap(document.sessionController)
        _ = try await waitForRunningTap(document)
        try await waitUntil(timeout: 30, "the boxes") { !controller.editor.boxes.isEmpty }
        return (document, controller, try XCTUnwrap(document.windowControllers.first as? DeckWindowController))
    }

    /// Picks a theme in one popover, then another, then Default, and undoes each pick.
    func assertThePicksRewriteTheThemeLine(_ controller: DeckSessionController, pick: (String) -> Void, startingWith first: String?, file: StaticString = #filePath, line: UInt = #line) {
        let editor = controller.editor
        var reported: [String?] = []
        controller.onThemeChanged = { reported.append($0) }
        let original = editor.string
        XCTAssertEqual(controller.currentThemeSlug, first, file: file, line: line)

        pick("blueprint")
        XCTAssertEqual(controller.currentThemeSlug, "blueprint", "the theme key is rewritten", file: file, line: line)
        XCTAssertTrue(editor.string.contains("theme: blueprint\n"), file: file, line: line)
        XCTAssertEqual(editor.string.components(separatedBy: "theme:").count, 2, "one theme line", file: file, line: line)
        XCTAssertEqual(editor.undoManager?.undoActionName, "Change Theme", file: file, line: line)
        XCTAssertEqual(reported, ["blueprint"], "reported once, for the toolbar item", file: file, line: line)

        editor.undoManager?.undo()
        XCTAssertEqual(editor.string, original, "one undo takes the pick back", file: file, line: line)
        XCTAssertEqual(controller.currentThemeSlug, first, file: file, line: line)

        pick("default")
        XCTAssertNil(controller.currentThemeSlug, "the Default cell removes the line", file: file, line: line)
        XCTAssertFalse(editor.string.contains("theme:"), file: file, line: line)
        editor.undoManager?.undo()
        XCTAssertEqual(editor.string, original, "and one undo brings it back", file: file, line: line)
    }

    func testThemePicksOnASavedDeck() async throws {
        let (document, controller, deckWindow) = try await openThemed()
        XCTAssertNotNil(document.fileURL)
        assertThePicksRewriteTheThemeLine(controller, pick: { deckWindow.themePopover.onPick?($0) }, startingWith: "terminal")
        assertThePicksRewriteTheThemeLine(controller, pick: { controller.deckForm.themePopover.onPick?($0) }, startingWith: "terminal")
    }

    func testThemePicksOnAnUntitledDeck() async throws {
        let (document, controller, deckWindow) = try await openUntitled()
        XCTAssertNil(document.fileURL, "the deck has no file yet")
        assertThePicksRewriteTheThemeLine(controller, pick: { deckWindow.themePopover.onPick?($0) }, startingWith: "swiss")
        assertThePicksRewriteTheThemeLine(controller, pick: { controller.deckForm.themePopover.onPick?($0) }, startingWith: "swiss")
    }

    func testAThemePickOnADeckWithNoFrontmatterWritesOne() async throws {
        let document = try DeckDocument.makeUntitled(text: "# One\n\n---\n\n# Two\n")
        let controller = try XCTUnwrap(document.sessionController)
        _ = try await waitForRunningTap(document)
        controller.setTheme("blueprint")
        XCTAssertTrue(controller.editor.string.hasPrefix("---\ntheme: blueprint\n---\n"))
        controller.editor.undoManager?.undo()
        XCTAssertEqual(controller.editor.string, "# One\n\n---\n\n# Two\n")
    }
}
