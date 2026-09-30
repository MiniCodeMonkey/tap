import XCTest
@testable import TapDesktopCore

final class PaperColourTests: XCTestCase {
    func testTheMostCommonColourWins() {
        let dark = PaperColour(red: 0.05, green: 0.06, blue: 0.1)
        let nearDark = PaperColour(red: 0.052, green: 0.061, blue: 0.098)
        let accent = PaperColour(red: 0.9, green: 0.2, blue: 0.2)
        let winner = PaperColour.dominant(of: [accent, dark, nearDark, dark])
        XCTAssertEqual(try XCTUnwrap(winner).red, 0.05, accuracy: 0.01)
        XCTAssertFalse(try XCTUnwrap(winner).isLight)
    }

    func testATieGoesToTheFirstColourSeen() {
        let first = PaperColour(red: 1, green: 1, blue: 1)
        let second = PaperColour(red: 0, green: 0, blue: 0)
        XCTAssertEqual(PaperColour.dominant(of: [first, second]), first)
        XCTAssertEqual(PaperColour.dominant(of: [second, first]), second)
    }

    func testNoSamplesHaveNoDominantColour() {
        XCTAssertNil(PaperColour.dominant(of: []))
        XCTAssertTrue(PaperColour.neutral.isLight)
    }
}
