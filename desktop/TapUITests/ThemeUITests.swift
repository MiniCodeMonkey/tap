import XCTest

/// T in the preview is the page's own key: it cycles the theme on screen
/// and writes nothing. The toolbar's Theme item, which does write, keeps
/// its title. Runs on CI; locally it would open a window (the person's rule).
final class ThemeUITests: UITestCase {
    func testTryAThemeWithoutSavingIt() throws {
        let deck = try copyFixture("seven-slides.md")
        let application = try launch(withDeck: deck)
        let preview = self.preview(in: application)
        XCTAssertTrue(preview.waitForExistence(timeout: 30), "the preview")
        let themeButton = application.buttons["theme-button"]
        XCTAssertTrue(themeButton.waitForExistence(timeout: 10), "the toolbar's Theme item")
        Thread.sleep(forTimeInterval: 3)
        let fileBefore = try String(contentsOf: deck, encoding: .utf8)
        let titleBefore = themeButton.title

        preview.click()
        Thread.sleep(forTimeInterval: 1)
        let before = preview.screenshot()
        application.typeKey("t", modifierFlags: [])
        Thread.sleep(forTimeInterval: 2)
        let after = preview.screenshot()
        XCTAssertNotEqual(after.pngRepresentation, before.pngRepresentation, "the preview cycled themes")
        Thread.sleep(forTimeInterval: 2)
        XCTAssertEqual(try String(contentsOf: deck, encoding: .utf8), fileBefore, "the file does not change")
        XCTAssertEqual(themeButton.title, titleBefore, "the deck's theme did not change: only the page's did")

        // The grid opens from the item, and closing it without a pick writes nothing either.
        themeButton.click()
        XCTAssertTrue(application.buttons["theme-cell-base"].waitForExistence(timeout: 10), "a cell of the theme grid popover (a stack view is not an accessibility element; its cells are)")
        application.typeKey(.escape, modifierFlags: [])
        Thread.sleep(forTimeInterval: 1)
        XCTAssertEqual(try String(contentsOf: deck, encoding: .utf8), fileBefore)
    }
}
