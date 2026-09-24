import XCTest

final class PreviewKeyTests: UITestCase {
    func testTryAThemeInThePreview() throws {
        let deck = try copyFixture("seven-slides.md")
        let application = launch(withDeck: deck)
        let preview = application.webViews["preview"]
        XCTAssertTrue(preview.waitForExistence(timeout: 30))
        Thread.sleep(forTimeInterval: 3)

        let fileBefore = try String(contentsOf: deck, encoding: .utf8)
        preview.click()
        Thread.sleep(forTimeInterval: 1)
        let before = preview.screenshot().pngRepresentation
        application.typeKey("t", modifierFlags: [])
        Thread.sleep(forTimeInterval: 2)

        XCTAssertNotEqual(preview.screenshot().pngRepresentation, before, "the preview cycled themes")
        XCTAssertEqual(try String(contentsOf: deck, encoding: .utf8), fileBefore, "the file does not change")
    }

    func testShortcutsDoNotClashWithThePresentationKeys() throws {
        let deck = try copyFixture("seven-slides.md")
        let application = launch(withDeck: deck)
        let editor = application.textViews["editor"]
        let preview = application.webViews["preview"]
        XCTAssertTrue(editor.waitForExistence(timeout: 30))
        XCTAssertTrue(preview.waitForExistence(timeout: 30))
        Thread.sleep(forTimeInterval: 3)

        // In the editor, the presentation keys type text.
        editor.click()
        application.typeText("T")
        let afterTyping = try XCTUnwrap(editor.value as? String)
        XCTAssertTrue(afterTyping.contains("T"))

        // With the preview focused, they go to the page.
        preview.click()
        application.typeKey("t", modifierFlags: [])
        application.typeKey(.rightArrow, modifierFlags: [])
        application.typeKey("o", modifierFlags: [])
        Thread.sleep(forTimeInterval: 1)
        XCTAssertEqual(editor.value as? String, afterTyping, "no presentation key reached the editor")
    }
}
