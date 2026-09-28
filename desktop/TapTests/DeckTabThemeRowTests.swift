import XCTest
@testable import Tap

final class DeckTabThemeRowTests: HostedTestCase {
    /// The DeckTabThemeRow board: the Theme row is one button, the theme's
    /// render and name, that opens the same popover as the toolbar's item.
    func testTheDeckTabThemeRowOpensTheGrid() async throws {
        let png = Fixtures.repositoryRoot.appendingPathComponent("desktop/TapTests/Fixtures/diagram.png")
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.themeShow(png: png, recordingTo: record)
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("themed.md"))
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        await AppEnvironment.shared.deckSchema.load()
        window.showDeckTab(nil)
        let form = controller.deckForm
        try await waitUntil(timeout: 20, "the catalog") { AppEnvironment.shared.themeImages.catalog != nil }
        let row = try XCTUnwrap(form.themeRowButton, "the Theme row is a button, not a popup")
        XCTAssertNil(form.fields["theme"] as? NSPopUpButton, "the theme is picked in the grid, not a popup")
        try await waitUntil(timeout: 5, "the row's name") { row.nameLabel.stringValue == "Terminal" }
        try await waitUntil(timeout: 60, "the row's render") { row.swatchView.image != nil }

        row.performClick(nil)
        XCTAssertTrue(form.themePopover.isShown)
        XCTAssertEqual(form.themePopover.grid.selectedSlug, "terminal")
        try await waitUntil(timeout: 20, "the cells") { !form.themePopover.grid.cells.isEmpty }
        form.themePopover.grid.cell(for: "blueprint")?.performClick(nil)
        try await waitUntil(timeout: 20, "tap theme set to land") { controller.editor.string.contains("theme: blueprint") }
        try await waitUntil(timeout: 5, "the row follows the frontmatter") { row.nameLabel.stringValue == "Blueprint" }
        controller.editor.undoManager?.undo()
        try await waitUntil(timeout: 5, "and follows an undo") { row.nameLabel.stringValue == "Terminal" }
    }
}
