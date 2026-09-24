import XCTest

final class SlidePanelUITests: UITestCase {
    func testHoveringTheSlidesButtonPeeksAtThePanel() throws {
        let application = launch(withDeck: try copyFixture("seven-slides.md"))
        let button = application.buttons["slides-button"]
        XCTAssertTrue(button.waitForExistence(timeout: 30))
        // Unpin first: a fresh install starts pinned.
        button.click()
        let overlay = application.otherElements["slide-panel-overlay"]
        XCTAssertFalse(overlay.exists)
        button.hover()
        XCTAssertTrue(overlay.waitForExistence(timeout: 3), "hovering the button shows the glass overlay")
        application.textViews["editor"].coordinate(withNormalizedOffset: CGVector(dx: 0.8, dy: 0.5)).hover()
        let gone = NSPredicate(format: "exists == false")
        expectation(for: gone, evaluatedWith: overlay)
        waitForExpectations(timeout: 3)
    }

    func testHoldingNewSlideOpensTheGallery() throws {
        let application = launch(withDeck: try copyFixture("ops.md"))
        let button = application.buttons["new-slide-button"]
        XCTAssertTrue(button.waitForExistence(timeout: 30))
        button.press(forDuration: 0.6)
        XCTAssertTrue(application.collectionViews["layout-gallery"].waitForExistence(timeout: 3))
        application.otherElements["layout-big-stat"].firstMatch.click()
        let editor = application.textViews["editor"]
        XCTAssertTrue(try XCTUnwrap(editor.value as? String).contains("layout: big-stat"))
    }
}
