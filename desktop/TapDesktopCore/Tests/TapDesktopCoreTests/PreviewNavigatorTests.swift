import XCTest
@testable import TapDesktopCore

final class PreviewNavigatorTests: XCTestCase {
    let plain = Slide(number: 1, startLine: 1, endLine: 1)
    let fragments = Slide(number: 3, startLine: 5, endLine: 9, fragments: 2)
    let stepsAndFragments = Slide(number: 4, startLine: 11, endLine: 12, fragments: 1, steps: 2)

    func testPositionsGoThroughStepsThenFragments() {
        let positions = (0...3).map { RevealPositions.position($0, steps: 2, fragments: 1) }
        XCTAssertEqual(positions, [RevealPosition(step: 0, fragment: -1), RevealPosition(step: 1, fragment: -1),
                                   RevealPosition(step: 2, fragment: -1), RevealPosition(step: 2, fragment: 0)])
        XCTAssertEqual(RevealPositions.position(9, steps: 0, fragments: 2), RevealPosition(step: 0, fragment: 1))
    }

    func testFollowingTheCursorShowsEveryStep() {
        var navigator = PreviewNavigator()
        XCTAssertEqual(navigator.cursorMoved(to: fragments), SlideMessage(slideIndex: 2, fragment: 1, step: 0))
        XCTAssertEqual(navigator.stepLabel, "All steps shown, 2 of 2")
        XCTAssertEqual(navigator.statusLabel, "Slide 3, follows the cursor")
        XCTAssertNil(navigator.cursorMoved(to: fragments), "moving inside the same slide sends nothing")
        XCTAssertEqual(navigator.cursorMoved(to: stepsAndFragments), SlideMessage(slideIndex: 3, fragment: 0, step: 2))
    }

    func testStepControlsWalkThePositions() {
        var navigator = PreviewNavigator()
        _ = navigator.cursorMoved(to: fragments)
        XCTAssertNil(navigator.stepForward(), "already at the last position")
        XCTAssertEqual(navigator.stepBackward(), SlideMessage(slideIndex: 2, fragment: 0, step: 0))
        XCTAssertEqual(navigator.stepLabel, "Step 1 of 2")
        XCTAssertEqual(navigator.stepBackward(), SlideMessage(slideIndex: 2, fragment: -1, step: 0))
        XCTAssertNil(navigator.stepBackward())
        XCTAssertEqual(navigator.stepForward(), SlideMessage(slideIndex: 2, fragment: 0, step: 0))
        XCTAssertEqual(navigator.stepForward(), SlideMessage(slideIndex: 2, fragment: 1, step: 0))
        _ = navigator.cursorMoved(to: plain)
        XCTAssertEqual(navigator.stepLabel, "No steps")
    }

    func testAPinnedSlideIgnoresTheCursorUntilUnpinned() {
        var navigator = PreviewNavigator()
        _ = navigator.cursorMoved(to: fragments)
        navigator.pin()
        XCTAssertTrue(navigator.isPinned)
        XCTAssertEqual(navigator.statusLabel, "Slide 3, pinned")
        XCTAssertNil(navigator.cursorMoved(to: plain))
        XCTAssertEqual(navigator.slideNumber, 3)
        XCTAssertEqual(navigator.unpin(cursorSlide: plain), SlideMessage(slideIndex: 0, fragment: -1, step: 0))
        XCTAssertEqual(navigator.slideNumber, 1)
    }

    func testNewCountsKeepTheLastPositionWhenEverythingWasShown() {
        var navigator = PreviewNavigator()
        _ = navigator.cursorMoved(to: fragments)
        let more = Slide(number: 3, startLine: 5, endLine: 11, fragments: 3)
        XCTAssertEqual(navigator.slidesChanged([plain, more]), SlideMessage(slideIndex: 2, fragment: 2, step: 0))
        XCTAssertNil(navigator.slidesChanged([plain, more]), "the same counts send nothing")
    }

    func testDeletingTheShownSlideFromTheMiddleFallsBackToTheSlideNowAtThatPosition() {
        var navigator = PreviewNavigator()
        _ = navigator.cursorMoved(to: fragments) // number 3, at position index 2
        let remaining = [
            Slide(number: 1, startLine: 1, endLine: 1),
            Slide(number: 2, startLine: 2, endLine: 2),
            Slide(number: 4, startLine: 10, endLine: 12, fragments: 1),
        ]
        XCTAssertEqual(navigator.slidesChanged(remaining), SlideMessage(slideIndex: 3, fragment: 0, step: 0))
        XCTAssertEqual(navigator.slideNumber, 4)
        XCTAssertEqual(navigator.revealCount, 1)
    }

    func testDeletingTheLastShownSlideFallsBackToTheNewLastSlide() {
        var navigator = PreviewNavigator()
        let last = Slide(number: 4, startLine: 20, endLine: 22, fragments: 1)
        _ = navigator.cursorMoved(to: last)
        let shorter = [
            Slide(number: 1, startLine: 1, endLine: 1),
            Slide(number: 2, startLine: 2, endLine: 2),
            Slide(number: 3, startLine: 3, endLine: 9, steps: 2),
        ]
        XCTAssertEqual(navigator.slidesChanged(shorter), SlideMessage(slideIndex: 2, fragment: -1, step: 2))
        XCTAssertEqual(navigator.slideNumber, 3)
        XCTAssertEqual(navigator.revealCount, 2)
    }

    func testDeletingEveryVisibleSlideClearsTheNavigator() {
        var navigator = PreviewNavigator()
        _ = navigator.cursorMoved(to: fragments)
        XCTAssertNil(navigator.slidesChanged([]))
        XCTAssertNil(navigator.slideNumber)
        XCTAssertEqual(navigator.revealCount, 0)
        XCTAssertNil(navigator.message)
    }
}
