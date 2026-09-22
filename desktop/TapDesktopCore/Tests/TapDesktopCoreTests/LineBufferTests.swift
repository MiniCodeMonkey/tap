import XCTest
@testable import TapDesktopCore

final class LineBufferTests: XCTestCase {
    func testReturnsOnlyCompleteLines() {
        var buffer = LineBuffer()
        XCTAssertEqual(buffer.append(Data("{\"a\":1}\n{\"b\"".utf8)), [#"{"a":1}"#])
        XCTAssertEqual(buffer.append(Data(":2}\r\n\n".utf8)), [#"{"b":2}"#, ""])
        XCTAssertNil(buffer.finish())
    }

    func testFinishReturnsTheTextAfterTheLastNewline() {
        var buffer = LineBuffer()
        XCTAssertEqual(buffer.append(Data("one\ntwo".utf8)), ["one"])
        XCTAssertEqual(buffer.finish(), "two")
        XCTAssertNil(buffer.finish())
    }

    func testKeepsAMultibyteCharacterSplitAcrossChunks() {
        var buffer = LineBuffer()
        let bytes = Array("é\n".utf8)
        XCTAssertEqual(buffer.append(Data(bytes[0..<1])), [])
        XCTAssertEqual(buffer.append(Data(bytes[1...])), ["é"])
    }
}
