import XCTest
@testable import TapDesktopCore

final class SlideDragPayloadTests: XCTestCase {
    func testRoundTripsThroughData() throws {
        let payload = SlideDragPayload(deckPath: "/talks/talk.md", slideNumbers: [5, 6], markdowns: ["# Five", "# Six\n\nBody"])
        let decoded = try XCTUnwrap(SlideDragPayload(data: try payload.data()))
        XCTAssertEqual(decoded, payload)
        XCTAssertNil(SlideDragPayload(data: Data("not json".utf8)))
        XCTAssertEqual(SlideDragPayload.pasteboardType, "io.geocod.tap.slides")
    }

    func testTellsItsOwnDeckFromAnotherThroughSymlinks() throws {
        let payload = SlideDragPayload(deckPath: "/tmp/talks/talk.md", slideNumbers: [1], markdowns: ["# One"])
        XCTAssertTrue(payload.comesFrom(deck: URL(fileURLWithPath: "/private/tmp/talks/talk.md")))
        XCTAssertFalse(payload.comesFrom(deck: URL(fileURLWithPath: "/tmp/talks/other.md")))
    }
}
