import XCTest
@testable import TapDesktopCore

final class ThumbnailQueueTests: XCTestCase {
    func testCurrentFirstThenVisibleThenNearestThenTheRest() {
        var queue = ThumbnailQueue()
        queue.replace(with: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10], visible: [4, 5, 6], current: 5)
        XCTAssertEqual(queue.pending, [5, 4, 6, 3, 7, 2, 8, 1, 9, 10])
    }

    func testWithoutAVisibleRangeTheCurrentSlideAnchorsTheOrder() {
        var queue = ThumbnailQueue()
        queue.replace(with: [1, 2, 3, 4, 5], visible: [], current: 4)
        XCTAssertEqual(queue.pending, [4, 3, 5, 2, 1])
        queue.replace(with: [3, 1, 2], visible: [], current: nil)
        XCTAssertEqual(queue.pending, [1, 2, 3], "nothing to anchor on: deck order")
    }

    func testRequeueAndRemove() {
        var queue = ThumbnailQueue()
        queue.replace(with: [1, 2, 3], visible: [1], current: nil)
        queue.requeue(1)
        XCTAssertEqual(queue.pending, [2, 3, 1])
        queue.remove(2)
        queue.remove(3)
        queue.remove(1)
        XCTAssertTrue(queue.isEmpty)
        queue.replace(with: [4, 5], visible: [], current: nil)
        queue.remove(4)
        XCTAssertEqual(queue.pending, [5])
    }

    func testReplaceDropsNumbersNoLongerWanted() {
        var queue = ThumbnailQueue()
        queue.replace(with: [1, 2, 3], visible: [], current: nil)
        queue.replace(with: [2], visible: [], current: nil)
        XCTAssertEqual(queue.pending, [2])
    }
}
