import XCTest
@testable import TapDesktopCore

final class SelectionClampTests: XCTestCase {
    func testASelectionInsideTheHiddenTextBecomesACaretAtItsEnd() {
        XCTAssertEqual(SelectionClamp.clamp(NSRange(location: 0, length: 0), hiddenLength: 20), NSRange(location: 20, length: 0))
        XCTAssertEqual(SelectionClamp.clamp(NSRange(location: 3, length: 5), hiddenLength: 20), NSRange(location: 20, length: 0))
    }

    func testASelectionThatStartsInTheHiddenTextStartsAtItsEnd() {
        XCTAssertEqual(SelectionClamp.clamp(NSRange(location: 0, length: 50), hiddenLength: 20), NSRange(location: 20, length: 30))
    }

    func testAVisibleSelectionIsUnchanged() {
        XCTAssertEqual(SelectionClamp.clamp(NSRange(location: 20, length: 4), hiddenLength: 20), NSRange(location: 20, length: 4))
        XCTAssertEqual(SelectionClamp.clamp(NSRange(location: 0, length: 4), hiddenLength: 0), NSRange(location: 0, length: 4))
    }

    func testACaretExactlyAtTheHiddenBoundaryIsUnchanged() {
        XCTAssertEqual(SelectionClamp.clamp(NSRange(location: 20, length: 0), hiddenLength: 20), NSRange(location: 20, length: 0))
    }

    func testAnEditTouchesHiddenTextWhenItStartsBeforeTheEnd() {
        XCTAssertTrue(SelectionClamp.touchesHidden(NSRange(location: 19, length: 1), hiddenLength: 20))
        XCTAssertFalse(SelectionClamp.touchesHidden(NSRange(location: 20, length: 0), hiddenLength: 20))
        XCTAssertFalse(SelectionClamp.touchesHidden(NSRange(location: 0, length: 1), hiddenLength: 0))
    }
}
