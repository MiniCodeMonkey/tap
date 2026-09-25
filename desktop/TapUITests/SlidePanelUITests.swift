import XCTest

final class SlidePanelUITests: UITestCase {
    func testHoveringTheSlidesButtonPeeksAtThePanel() throws {
        let application = launch(withDeck: try copyFixture("seven-slides.md"))
        // The Slides button toggles, so accessibility reports it as a check box.
        let button = application.checkBoxes["slides-button"]
        XCTAssertTrue(button.waitForExistence(timeout: 30))
        // Unpin first: a fresh install starts pinned.
        button.click()
        // The overlay is a group; hidden, it is not in the tree at all.
        let overlay = application.groups["slide-panel-overlay"]
        XCTAssertFalse(overlay.exists)
        // The click left the pointer on the button, and a hover to where the
        // pointer already is posts no new mouseEntered: leave, then come back.
        let away = application.textViews["editor"].coordinate(withNormalizedOffset: CGVector(dx: 0.8, dy: 0.5))
        away.hover()
        button.hover()
        XCTAssertTrue(overlay.waitForExistence(timeout: 3), "hovering the button shows the glass overlay")
        away.hover()
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
        // A gallery cell is a button, as a thumbnail is.
        application.buttons["layout-big-stat"].firstMatch.click()
        let editor = application.textViews["editor"]
        XCTAssertTrue(try XCTUnwrap(editor.value as? String).contains("layout: big-stat"))
    }
}
