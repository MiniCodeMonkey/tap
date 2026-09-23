import XCTest
@testable import TapDesktopCore

final class RestartPolicyTests: XCTestCase {
    let start = Date(timeIntervalSinceReferenceDate: 1_000)

    func testBacksOffAndGivesUpAtTheThirdExitWithinThirtySeconds() {
        var policy = RestartPolicy()
        XCTAssertEqual(policy.recordExit(at: start), .restart(after: 0.5))
        XCTAssertEqual(policy.recordExit(at: start + 10), .restart(after: 1.0))
        XCTAssertEqual(policy.recordExit(at: start + 20), .giveUp)
    }

    func testExitsOlderThanTheWindowDoNotCount() {
        var policy = RestartPolicy()
        _ = policy.recordExit(at: start)
        _ = policy.recordExit(at: start + 10)
        XCTAssertEqual(policy.recordExit(at: start + 31), .restart(after: 1.0))
        XCTAssertEqual(policy.recentExitCount, 2)
    }

    func testResetStartsOver() {
        var policy = RestartPolicy()
        _ = policy.recordExit(at: start)
        _ = policy.recordExit(at: start + 1)
        policy.reset()
        XCTAssertEqual(policy.recordExit(at: start + 2), .restart(after: 0.5))
    }

    func testTheDelayIsCapped() {
        var policy = RestartPolicy(window: 30, maximumExits: 10, baseDelay: 1, maximumDelay: 3)
        let delays = (0..<5).map { policy.recordExit(at: start + Double($0)) }
        XCTAssertEqual(delays, [.restart(after: 1), .restart(after: 2), .restart(after: 3), .restart(after: 3), .restart(after: 3)])
    }

    /// This sentence feeds both the session's own log line and the preview
    /// overlay's "The preview stopped" text, so it must always name this
    /// policy's real numbers, not a fixed "3 times in 30 seconds".
    func testExitSummaryNamesThisPolicysOwnNumbers() {
        XCTAssertEqual(RestartPolicy().exitSummary, "tap exited 3 times in 30 seconds")
        XCTAssertEqual(RestartPolicy(window: 60, maximumExits: 5).exitSummary, "tap exited 5 times in 60 seconds")
    }
}
