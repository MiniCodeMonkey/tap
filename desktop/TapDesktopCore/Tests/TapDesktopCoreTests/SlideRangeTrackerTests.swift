import XCTest
@testable import TapDesktopCore

final class SlideRangeTrackerTests: XCTestCase {
    // "# One" is offsets 0..<5, "# Two" starts at 12.
    let text = "# One\n\n---\n\n# Two\n"
    let list = SlideList(slides: [Slide(number: 1, startLine: 1, endLine: 1, title: "One"),
                                  Slide(number: 2, startLine: 5, endLine: 5, title: "Two")], errors: [])

    func appliedTracker() -> SlideRangeTracker {
        var tracker = SlideRangeTracker()
        let generation = tracker.beginSend()
        _ = tracker.apply(list, sentText: text, sentGeneration: generation, currentLength: (text as NSString).length)
        return tracker
    }

    func testBoxesCoverTheLinesTapReports() {
        let tracker = appliedTracker()
        XCTAssertEqual(tracker.boxes.map(\.range), [NSRange(location: 0, length: 5), NSRange(location: 12, length: 5)])
    }

    func testTypingNeverMakesBoxesJump() {
        var tracker = appliedTracker()
        // Typing before tap answers shifts the boxes at once.
        tracker.recordEdit(location: 5, oldLength: 0, newLength: 3)
        XCTAssertEqual(tracker.boxes.map(\.range), [NSRange(location: 0, length: 8), NSRange(location: 15, length: 5)])

        // The buffer goes to tap, and one more key arrives before the answer.
        let sentText = "# Oneabc\n\n---\n\n# Two\n"
        let generation = tracker.beginSend()
        tracker.recordEdit(location: 8, oldLength: 0, newLength: 1)
        let shifted = tracker.boxes.map(\.range)
        XCTAssertEqual(shifted, [NSRange(location: 0, length: 9), NSRange(location: 16, length: 5)])

        // tap's answer for the sent text replaces the shifted ranges, and the
        // later key is replayed onto it, so nothing moves.
        let answer = SlideList(slides: [Slide(number: 1, startLine: 1, endLine: 1, title: "Oneabc"),
                                        Slide(number: 2, startLine: 5, endLine: 5, title: "Two")], errors: [])
        let dirty = tracker.apply(answer, sentText: sentText, sentGeneration: generation, currentLength: 22)
        XCTAssertEqual(tracker.boxes.map(\.range), shifted)
        XCTAssertEqual(tracker.boxes[0].slide.title, "Oneabc")
        XCTAssertTrue(dirty.isEmpty, "an answer that matches the shifted boxes restyles nothing")
    }

    func testAnAnswerThatMovesABoundaryReturnsTheRegionToRestyle() {
        var tracker = appliedTracker()
        let generation = tracker.beginSend()
        let joined = SlideList(slides: [Slide(number: 1, startLine: 1, endLine: 5)], errors: [])
        let dirty = tracker.apply(joined, sentText: text, sentGeneration: generation, currentLength: 18)
        XCTAssertFalse(dirty.isEmpty)
        XCTAssertEqual(tracker.boxes.count, 1)
    }

    func testShiftKeepsABoxOverTheSameText() {
        let box = NSRange(location: 10, length: 5)
        XCTAssertEqual(SlideRangeTracker.shift(box, location: 20, oldLength: 0, newLength: 4), box, "an edit after the box")
        XCTAssertEqual(SlideRangeTracker.shift(box, location: 2, oldLength: 1, newLength: 3), NSRange(location: 12, length: 5), "an edit before the box")
        XCTAssertEqual(SlideRangeTracker.shift(box, location: 12, oldLength: 2, newLength: 0), NSRange(location: 10, length: 3), "a deletion inside")
        XCTAssertEqual(SlideRangeTracker.shift(box, location: 10, oldLength: 0, newLength: 2), NSRange(location: 10, length: 7), "typing at the start belongs to the box")
        XCTAssertEqual(SlideRangeTracker.shift(box, location: 15, oldLength: 0, newLength: 2), NSRange(location: 10, length: 7), "typing at the end belongs to the box")
    }

    func testTheFrontmatterIsHiddenUntilTheDeckHasErrors() {
        let deck = "---\ntitle: T\n---\n\n# One\n"
        var tracker = SlideRangeTracker()
        var generation = tracker.beginSend()
        _ = tracker.apply(SlideList(slides: [Slide(number: 1, startLine: 5, endLine: 5)], errors: []),
                          sentText: deck, sentGeneration: generation, currentLength: 24)
        XCTAssertEqual(tracker.hiddenPrefixLength, 18)
        generation = tracker.beginSend()
        _ = tracker.apply(SlideList(slides: [Slide(number: 1, startLine: 5, endLine: 5)], errors: ["frontmatter: bad"]),
                          sentText: deck, sentGeneration: generation, currentLength: 24)
        XCTAssertEqual(tracker.hiddenPrefixLength, 0)
        XCTAssertEqual(tracker.deckErrors, ["frontmatter: bad"])
    }

    func testTheCurrentBoxIsTheOneAroundOrBeforeTheCaret() {
        let tracker = appliedTracker()
        XCTAssertEqual(tracker.currentBoxIndex(caret: 3), 0)
        XCTAssertEqual(tracker.currentBoxIndex(caret: 8), 0, "between boxes, the box above")
        XCTAssertEqual(tracker.currentBoxIndex(caret: 17), 1)
        XCTAssertNil(SlideRangeTracker().currentBoxIndex(caret: 0))
    }

    func testASlideOutsideTheTextIsDropped() {
        let boxes = SlideRangeTracker.boxes(for: [Slide(number: 1, startLine: 40, endLine: 41)], in: "one line" as NSString)
        XCTAssertTrue(boxes.isEmpty)
    }
}
