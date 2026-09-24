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

    func testNextAndRequeue() {
        var queue = ThumbnailQueue()
        queue.replace(with: [1, 2, 3], visible: [1], current: nil)
        XCTAssertEqual(queue.next(), 1)
        queue.requeue(1)
        XCTAssertEqual(queue.pending, [2, 3, 1])
        XCTAssertEqual(queue.next(), 2)
        XCTAssertEqual(queue.next(), 3)
        XCTAssertEqual(queue.next(), 1)
        XCTAssertNil(queue.next())
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

    func testRequeueBacksOffAfterTheRetryLimit() {
        var queue = ThumbnailQueue()
        queue.replace(with: [1], visible: [], current: nil)
        for _ in 0..<ThumbnailQueue.maxRetries {
            XCTAssertEqual(queue.next(), 1)
            queue.requeue(1)
            XCTAssertEqual(queue.pending, [1], "still under the retry limit")
        }
        XCTAssertEqual(queue.next(), 1)
        queue.requeue(1)
        XCTAssertTrue(queue.pending.isEmpty, "backs off once the retry limit is exceeded")
    }

    func testReplaceDoesNotResurrectANumberThatGaveUp() {
        var queue = ThumbnailQueue()
        queue.replace(with: [1], visible: [], current: nil)
        for _ in 0...ThumbnailQueue.maxRetries {
            XCTAssertEqual(queue.next(), 1)
            queue.requeue(1)
        }
        XCTAssertTrue(queue.pending.isEmpty)
        queue.replace(with: [1], visible: [], current: nil)
        XCTAssertTrue(queue.pending.isEmpty, "a slide that exhausted its retries is not queued again")
    }
}
