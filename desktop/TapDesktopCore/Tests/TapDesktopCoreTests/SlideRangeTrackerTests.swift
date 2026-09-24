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
        XCTAssertEqual(dirty, [], "an answer that matches the shifted boxes restyles nothing")
    }

    func testAnAnswerThatMovesABoundaryReturnsTheRegionToRestyle() {
        var tracker = appliedTracker()
        let generation = tracker.beginSend()
        let joined = SlideList(slides: [Slide(number: 1, startLine: 1, endLine: 5)], errors: [])
        let dirty = tracker.apply(joined, sentText: text, sentGeneration: generation, currentLength: 18)
        XCTAssertFalse(dirty?.isEmpty ?? true)
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

    // Three slides: "# One" at 0..<5, "# Two" at 12..<17, "# Three" at 24..<31.
    let threeSlideText = "# One\n\n---\n\n# Two\n\n---\n\n# Three\n"
    let threeSlideList = SlideList(slides: [Slide(number: 1, startLine: 1, endLine: 1, title: "One"),
                                            Slide(number: 2, startLine: 5, endLine: 5, title: "Two"),
                                            Slide(number: 3, startLine: 9, endLine: 9, title: "Three")], errors: [])

    func threeSlideTracker() -> SlideRangeTracker {
        var tracker = SlideRangeTracker()
        let generation = tracker.beginSend()
        _ = tracker.apply(threeSlideList, sentText: threeSlideText, sentGeneration: generation,
                          currentLength: (threeSlideText as NSString).length)
        return tracker
    }

    func testADeletionThatCrossesASlideBoundaryLeavesValidRanges() {
        var tracker = threeSlideTracker()
        XCTAssertEqual(tracker.boxes.map(\.range), [NSRange(location: 0, length: 5), NSRange(location: 12, length: 5),
                                                     NSRange(location: 24, length: 7)])
        // Delete from inside slide 1, through the gap, into slide 2.
        tracker.recordEdit(location: 3, oldLength: 11, newLength: 0)
        let newLength = (threeSlideText as NSString).length - 11
        for box in tracker.boxes {
            XCTAssertGreaterThanOrEqual(box.range.length, 0, "no range has a negative length")
            XCTAssertGreaterThanOrEqual(NSMaxRange(box.range), box.range.location, "no range ends before it starts")
            XCTAssertLessThanOrEqual(NSMaxRange(box.range), newLength, "no range extends past the new end of the text")
        }
        // Both slides are pulled onto the text the deletion left. Slide 1
        // keeps it and slide 2 starts after it, so no position is in two
        // boxes and the caret at the join has one slide to belong to.
        XCTAssertEqual(tracker.boxes.map(\.range), [NSRange(location: 0, length: 3), NSRange(location: 4, length: 2),
                                                     NSRange(location: 13, length: 7)])
    }

    func testNoPositionIsInTwoBoxesAfterAnEditAcrossABoundary() {
        var tracker = threeSlideTracker()
        tracker.recordEdit(location: 3, oldLength: 11, newLength: 0)
        for (index, box) in tracker.boxes.enumerated().dropFirst() {
            XCTAssertGreaterThan(box.range.location, tracker.boxes[index - 1].end,
                                 "box \(index) starts inside the box before it")
        }
    }

    func testTheCaretForABoxIsReadBackAsThatBox() {
        var tracker = threeSlideTracker()
        func check(_ note: String) {
            for index in tracker.boxes.indices {
                let caret = tracker.caret(forBoxAt: index, in: threeSlideText as NSString)
                XCTAssertEqual(tracker.currentBoxIndex(caret: caret ?? -1), index,
                               "\(note): the caret for box \(index) belongs to another box")
            }
        }
        check("as tap answered")
        tracker.recordEdit(location: 3, oldLength: 11, newLength: 0)
        check("after a deletion across a slide boundary")
        tracker.recordEdit(location: 3, oldLength: 0, newLength: 6)
        check("after typing at the join")
    }

    func testASlideListWhoseLinesOverlapStillGivesEachPositionOneSlide() {
        var tracker = SlideRangeTracker()
        let generation = tracker.beginSend()
        // Two slides tap reports over the same lines.
        _ = tracker.apply(SlideList(slides: [Slide(number: 1, startLine: 1, endLine: 5),
                                             Slide(number: 2, startLine: 3, endLine: 5)], errors: []),
                          sentText: threeSlideText, sentGeneration: generation,
                          currentLength: (threeSlideText as NSString).length)
        XCTAssertGreaterThan(tracker.boxes[1].range.location, tracker.boxes[0].end,
                             "the second slide starts after the first ends")
        for index in tracker.boxes.indices {
            let caret = tracker.caret(forBoxAt: index, in: threeSlideText as NSString)
            XCTAssertEqual(tracker.currentBoxIndex(caret: caret ?? -1), index)
        }
    }

    func testAnAnswerForTextThatIsGoneIsRefused() {
        var tracker = threeSlideTracker()
        // A PUT goes out, and the whole document is replaced before it
        // answers, the way Revert To replaces it.
        let generation = tracker.beginSend()
        tracker.reset()
        tracker.recordEdit(location: 0, oldLength: 0, newLength: 31)
        XCTAssertNil(tracker.apply(threeSlideList, sentText: threeSlideText, sentGeneration: generation, currentLength: 31),
                     "the answer describes text the editor no longer holds")
        XCTAssertTrue(tracker.boxes.isEmpty, "and it leaves no boxes behind that name the wrong lines")
        let next = tracker.beginSend()
        XCTAssertNotNil(tracker.apply(threeSlideList, sentText: threeSlideText, sentGeneration: next, currentLength: 31),
                        "the answer for the text that replaced it is taken")
    }

    func testADeletionThatExactlyEmptiesASlideZeroesItAndShiftsLaterSlidesUp() {
        var tracker = threeSlideTracker()
        // Delete precisely slide 2's range.
        tracker.recordEdit(location: 12, oldLength: 5, newLength: 0)
        let ranges = tracker.boxes.map(\.range)
        XCTAssertEqual(ranges[0], NSRange(location: 0, length: 5), "slide 1 is untouched")
        XCTAssertEqual(ranges[1], NSRange(location: 12, length: 0), "slide 2 becomes zero length, not inverted")
        XCTAssertEqual(ranges[2], NSRange(location: 19, length: 7), "slide 3 shifts up by exactly the deleted length")
    }

    func testEditsRecordedInflightReplayInTheOrderTheyWereRecorded() {
        var tracker = appliedTracker()
        let generation = tracker.beginSend()
        // Two edits whose combined effect depends on replay order: first removes
        // the gap after slide 1, then inserts inside slide 1's remaining text.
        // Replayed backwards or skipped, slide 1 ends up a different length.
        tracker.recordEdit(location: 5, oldLength: 7, newLength: 0)
        tracker.recordEdit(location: 3, oldLength: 0, newLength: 4)
        _ = tracker.apply(list, sentText: text, sentGeneration: generation, currentLength: 15)
        XCTAssertEqual(tracker.boxes.map(\.range), [NSRange(location: 0, length: 9), NSRange(location: 10, length: 4)],
                       "replaying the log out of order would give slide 1 a different length")
    }

    func testAdoptRefusesAnswersToEarlierSends() {
        var tracker = SlideRangeTracker()
        let text = "# A\n\n---\n\n# B"
        _ = tracker.beginSend()
        _ = tracker.apply(SlideList(slides: [Slide(number: 1, startLine: 1, endLine: 1, title: "A"), Slide(number: 2, startLine: 5, endLine: 5, title: "B")], errors: ["x"]),
                          sentText: text, sentGeneration: tracker.generation, currentLength: (text as NSString).length)
        // A PUT of the old text is in flight when the operation permutes the boxes.
        let inFlight = tracker.beginSend()
        tracker.recordEdit(location: 0, oldLength: 13, newLength: 13)
        tracker.adopt([SlideBox(range: NSRange(location: 0, length: 3), slide: Slide(number: 1, startLine: 1, endLine: 1, title: "B")),
                       SlideBox(range: NSRange(location: 10, length: 3), slide: Slide(number: 2, startLine: 5, endLine: 5, title: "A"))])
        XCTAssertEqual(tracker.boxes.map(\.slide.title), ["B", "A"])
        XCTAssertEqual(tracker.deckErrors, ["x"], "adopting boxes says nothing about the deck's errors")

        let stale = tracker.apply(SlideList(slides: [Slide(number: 1, startLine: 1, endLine: 1, title: "A"), Slide(number: 2, startLine: 5, endLine: 5, title: "B")], errors: []),
                                  sentText: text, sentGeneration: inFlight, currentLength: 13)
        XCTAssertNil(stale, "the answer to a send begun before the adoption describes text the operation replaced")
        XCTAssertEqual(tracker.boxes.map(\.slide.title), ["B", "A"], "the adopted order survives it")

        // The operation's own send, begun after, is applied.
        let own = tracker.beginSend()
        let applied = tracker.apply(SlideList(slides: [Slide(number: 1, startLine: 1, endLine: 1, title: "B"), Slide(number: 2, startLine: 5, endLine: 5, title: "A")], errors: []),
                                    sentText: "# B\n\n---\n\n# A", sentGeneration: own, currentLength: 13)
        XCTAssertNotNil(applied)
        XCTAssertEqual(tracker.boxes.map(\.slide.title), ["B", "A"])
        XCTAssertEqual(tracker.boxes[1].range, NSRange(location: 10, length: 3))
    }

    func testAdoptSeparatesOverlappingBoxes() {
        var tracker = SlideRangeTracker()
        tracker.adopt([SlideBox(range: NSRange(location: 0, length: 10), slide: Slide(number: 1, startLine: 1, endLine: 1)),
                       SlideBox(range: NSRange(location: 8, length: 10), slide: Slide(number: 2, startLine: 2, endLine: 2))])
        XCTAssertEqual(tracker.boxes[1].range.location, 11)
    }
}
