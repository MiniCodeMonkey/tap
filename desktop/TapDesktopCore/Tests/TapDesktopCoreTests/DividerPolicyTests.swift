import XCTest
@testable import TapDesktopCore

final class DividerPolicyTests: XCTestCase {
    func testBalancesUntilTheUserDrags() {
        var policy = DividerPolicy()
        XCTAssertEqual(policy.balancedPosition(totalWidth: 1001, dividerThickness: 1), 500)
        policy.userDragged()
        XCTAssertNil(policy.balancedPosition(totalWidth: 1001, dividerThickness: 1))
        policy.reset()
        XCTAssertEqual(policy.balancedPosition(totalWidth: 801, dividerThickness: 1), 400)
    }

    func testBalancesWhatIsLeftAfterASidebar() {
        let policy = DividerPolicy()
        XCTAssertEqual(policy.balancedPosition(totalWidth: 1000, dividerThickness: 1, leadingWidth: 0), 499)
        XCTAssertEqual(policy.balancedPosition(totalWidth: 1000, dividerThickness: 1, leadingWidth: 224),
                       224 + 1 + ((1000 - 224 - 1 - 1) / 2).rounded(.down), "the position is measured from the split view's left edge")
        var dragged = policy
        dragged.userDragged()
        XCTAssertNil(dragged.balancedPosition(totalWidth: 1000, dividerThickness: 1, leadingWidth: 224))
    }
}
