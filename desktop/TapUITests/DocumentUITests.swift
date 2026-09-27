import XCTest

final class DocumentUITests: UITestCase {
    func testBrowseVersions() throws {
        let deck = try copyFixture("seven-slides.md")
        let application = try launch(withDeck: deck)
        let editor = application.textViews["editor"]
        XCTAssertTrue(editor.waitForExistence(timeout: 30))

        // AppKit leaves Revert To out of the File menu while the document
        // has nothing to revert to, as a freshly opened, unchanged deck
        // does. An edit and a save give it a version to go back to.
        editor.click()
        application.typeText("x")
        application.typeKey("s", modifierFlags: [.command])
        Thread.sleep(forTimeInterval: 1)

        let file = application.menuBarItems["File"]
        file.click()
        let revertTo = file.menuItems["Revert To"]
        guard revertTo.waitForExistence(timeout: 5) else {
            XCTFail("the File menu has no Revert To item after an edit and a save; it holds: \(titles(of: file))")
            return
        }
        revertTo.click()
        let browse = revertTo.menuItems["Browse All Versions…"]
        guard browse.waitForExistence(timeout: 5) else {
            XCTFail("Revert To has no Browse All Versions… item; it holds: \(titles(of: revertTo))")
            return
        }
        browse.click()

        // The version browser is a full screen space with Done and Restore.
        XCTAssertTrue(application.buttons["Done"].waitForExistence(timeout: 30), "the macOS version browser opened")
        application.buttons["Done"].click()
    }
}
