import XCTest

/// Real drags, with the real pointer. Local only: they take over the
/// screen and need Xcode's permission to control the computer.
final class SlideDragUITests: UITestCase {
    func testDraggingThumbnailFiveAboveThreeReordersTheDeck() throws {
        let application = launch(withDeck: try copyFixture("ops.md"))
        let panel = application.collectionViews["slide-panel"]
        XCTAssertTrue(panel.waitForExistence(timeout: 30))
        let five = panel.otherElements["thumbnail-5"].firstMatch
        let three = panel.otherElements["thumbnail-3"].firstMatch
        XCTAssertTrue(five.waitForExistence(timeout: 30))
        five.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.5))
            .click(forDuration: 0.4, thenDragTo: three.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.05)))
        let editor = application.textViews["editor"]
        XCTAssertTrue(editor.waitForExistence(timeout: 5))
        let text = try XCTUnwrap(editor.value as? String)
        XCTAssertLessThan(try XCTUnwrap(text.range(of: "# Five")?.lowerBound), try XCTUnwrap(text.range(of: "# Three")?.lowerBound), "Five now comes before Three")
    }

    func testDraggingABoxHeaderReordersTheDeck() throws {
        let application = launch(withDeck: try copyFixture("ops.md"))
        let editor = application.textViews["editor"]
        XCTAssertTrue(editor.waitForExistence(timeout: 30))
        let box5 = application.groups["box-5"].firstMatch
        let box3 = application.groups["box-3"].firstMatch
        XCTAssertTrue(box5.waitForExistence(timeout: 30))
        box5.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.08))
            .click(forDuration: 0.4, thenDragTo: box3.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.1)))
        let text = try XCTUnwrap(editor.value as? String)
        XCTAssertLessThan(try XCTUnwrap(text.range(of: "# Five")?.lowerBound), try XCTUnwrap(text.range(of: "# Three")?.lowerBound))
    }
}
