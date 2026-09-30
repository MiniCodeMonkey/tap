import XCTest
@testable import TapDesktopCore

final class PreviewLoadStatusTests: XCTestCase {
    func testNothingIsSaidBeforeOneSecond() {
        XCTAssertNil(PreviewLoadStatus.message(for: .startingTap, waited: 0))
        XCTAssertNil(PreviewLoadStatus.message(for: .loadingPage, waited: 0.99))
    }

    func testAfterOneSecondTheStepIsNamed() {
        XCTAssertEqual(PreviewLoadStatus.message(for: .startingTap, waited: 1), "Starting the preview…")
        XCTAssertEqual(PreviewLoadStatus.message(for: .loadingPage, waited: 2), "Loading the slides…")
        XCTAssertEqual(PreviewLoadStatus.message(for: .preparingSlide, waited: 5), "Loading fonts and images…")
    }
}
