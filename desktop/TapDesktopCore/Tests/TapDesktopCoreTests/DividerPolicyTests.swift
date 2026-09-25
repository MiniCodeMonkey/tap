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
}
