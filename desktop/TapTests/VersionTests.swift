import XCTest
@testable import Tap

final class VersionTests: HostedTestCase {
    func testVersionMatch() async throws {
        let tap = try XCTUnwrap(Bundle.main.url(forResource: "tap", withExtension: nil), "tap is not in Contents/Resources")
        let tapVersion = await AppEnvironment.readVersion(of: tap)
        let appVersion = Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String
        XCTAssertNotNil(tapVersion)
        XCTAssertEqual(tapVersion, appVersion)
    }
}
