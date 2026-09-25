import XCTest
@testable import TapDesktopCore

@MainActor
final class TapLogTests: XCTestCase {
    func testKeepsLinesUpToItsCapacityAndFormatsThem() {
        let log = TapLog(title: "talk", capacity: 3)
        let date = Date(timeIntervalSince1970: 0)
        for index in 1...4 { log.append("line \(index)", source: .standardError, date: date) }
        XCTAssertEqual(log.lines.map(\.text), ["line 2", "line 3", "line 4"])
        XCTAssertTrue(log.lines[0].formatted.hasPrefix("· "))
        XCTAssertTrue(log.lines[0].formatted.hasSuffix("  line 2"))
    }

    func testLastLinesFiltersBySource() {
        let log = TapLog(title: "talk")
        log.append("tap dev --app talk.md", source: .app)
        log.append("panic: boom", source: .standardError)
        log.append("goroutine 1", source: .standardError)
        XCTAssertEqual(log.lastLines(1, from: .standardError), ["goroutine 1"])
        XCTAssertEqual(log.lastLines(5, from: .standardError), ["panic: boom", "goroutine 1"])
    }

    func testPostsANotificationForEachLine() {
        let log = TapLog(title: "talk")
        let posted = expectation(forNotification: TapLog.didAppendNotification, object: log)
        log.append("hello", source: .app)
        wait(for: [posted], timeout: 1)
    }
}
