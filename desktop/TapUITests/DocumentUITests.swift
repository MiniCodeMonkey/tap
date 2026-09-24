import XCTest

final class DocumentUITests: UITestCase {
    func testBrowseVersions() throws {
        let deck = try copyFixture("seven-slides.md")
        let application = launch(withDeck: deck)
        XCTAssertTrue(application.textViews["editor"].waitForExistence(timeout: 30))

        application.menuBarItems["File"].click()
        application.menuBarItems["File"].menuItems["Revert To"].click()
        application.menuBarItems["File"].menuItems["Revert To"].menuItems["Browse All Versions…"].click()

        // The version browser is a full screen space with Done and Restore.
        XCTAssertTrue(application.buttons["Done"].waitForExistence(timeout: 30), "the macOS version browser opened")
        application.buttons["Done"].click()
    }
}
