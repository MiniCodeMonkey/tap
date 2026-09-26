import XCTest

/// Real drags, with the real pointer. Local only: they take over the
/// screen and need Xcode's permission to control the computer.
final class SlideDragUITests: UITestCase {
    func testDraggingThumbnailFiveAboveThreeReordersTheDeck() throws {
        let application = try launch(withDeck: try copyFixture("ops.md"))
        let panel = application.collectionViews["slide-panel"]
        XCTAssertTrue(panel.waitForExistence(timeout: 30))
        // A thumbnail is an accessibility element with the button role.
        let five = panel.buttons["thumbnail-5"].firstMatch
        let three = panel.buttons["thumbnail-3"].firstMatch
        XCTAssertTrue(five.waitForExistence(timeout: 30))
        five.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.5))
            .click(forDuration: 0.4, thenDragTo: three.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.05)))
        let editor = application.textViews["editor"]
        XCTAssertTrue(editor.waitForExistence(timeout: 5))
        let text = try XCTUnwrap(editor.value as? String)
        XCTAssertLessThan(try XCTUnwrap(text.range(of: "# Five")?.lowerBound), try XCTUnwrap(text.range(of: "# Three")?.lowerBound), "Five now comes before Three")
    }

    func testDraggingABoxHeaderReordersTheDeck() throws {
        let application = try launch(withDeck: try copyFixture("ops.md"))
        let editor = application.textViews["editor"]
        XCTAssertTrue(editor.waitForExistence(timeout: 30))
        let box5 = application.groups["box-5"].firstMatch
        let box3 = application.groups["box-3"].firstMatch
        XCTAssertTrue(box5.waitForExistence(timeout: 30))
        box5.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.08))
            .click(forDuration: 0.4, thenDragTo: box3.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.1)))
        // AppKit delivers the drop after the mouse goes up, so the move
        // lands a moment after the drag call returns.
        let fiveBeforeThree = NSPredicate { _, _ in
            guard let text = editor.value as? String, let five = text.range(of: "# Five"), let three = text.range(of: "# Three") else { return false }
            return five.lowerBound < three.lowerBound
        }
        let reordered = XCTNSPredicateExpectation(predicate: fiveBeforeThree, object: nil)
        XCTAssertEqual(XCTWaiter().wait(for: [reordered], timeout: 5), .completed,
                       "Five now comes before Three; the editor holds: \(String(describing: editor.value))")
    }

    /// A header drag is the editor's own drag session, so the text view's
    /// end-of-move handling must leave a text selection elsewhere alone.
    func testDraggingABoxHeaderLeavesTheSelectedTextInPlace() throws {
        let application = try launch(withDeck: try copyFixture("ops.md"))
        let editor = application.textViews["editor"]
        XCTAssertTrue(editor.waitForExistence(timeout: 30))
        let box1 = application.groups["box-1"].firstMatch
        let box5 = application.groups["box-5"].firstMatch
        let box3 = application.groups["box-3"].firstMatch
        XCTAssertTrue(box5.waitForExistence(timeout: 30))
        // Select one whole line of slide 1 ("# One" or "First.").
        box1.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.35)).click()
        editor.typeKey(.leftArrow, modifierFlags: .command)
        editor.typeKey(.rightArrow, modifierFlags: [.command, .shift])
        box5.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.08))
            .click(forDuration: 0.4, thenDragTo: box3.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 0.1)))
        let fiveBeforeThree = NSPredicate { _, _ in
            guard let text = editor.value as? String, let five = text.range(of: "# Five"), let three = text.range(of: "# Three") else { return false }
            return five.lowerBound < three.lowerBound
        }
        let reordered = XCTNSPredicateExpectation(predicate: fiveBeforeThree, object: nil)
        XCTAssertEqual(XCTWaiter().wait(for: [reordered], timeout: 5), .completed,
                       "Five now comes before Three; the editor holds: \(String(describing: editor.value))")
        let text = try XCTUnwrap(editor.value as? String)
        XCTAssertTrue(text.contains("# One\n\nFirst."), "the selected line survives the move; the editor holds: \(text)")
        let titles = text.components(separatedBy: "\n").filter { $0.hasPrefix("# ") }
        XCTAssertEqual(titles, ["# One", "# Two", "# Five", "# Three", "# Four", "# Six", "# Seven"])
    }
}
