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

    /// A session's environment closure does not keep its AppEnvironment
    /// alive, and falls back to the app's own environment once it is gone.
    func testSessionEnvironmentFallsBackOnceTheAppEnvironmentIsGone() async throws {
        var environment: AppEnvironment? = AppEnvironment()
        let configuration = try XCTUnwrap(environment).sessionConfiguration()
        weak var released = environment
        environment = nil
        XCTAssertNil(released, "the configuration does not retain its AppEnvironment")
        let variables = await configuration.environment()
        XCTAssertEqual(variables, ProcessInfo.processInfo.environment)
    }
}
