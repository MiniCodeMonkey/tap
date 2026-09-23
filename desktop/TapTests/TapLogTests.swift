import XCTest
@testable import Tap

final class TapLogWindowTests: HostedTestCase {
    func testLogsAndVersion() async throws {
        let document = try await openDeck(try Fixtures.copyAppFixture())
        _ = try await waitForRunningTap(document)
        let appDelegate = try XCTUnwrap(NSApp.delegate as? AppDelegate)

        appDelegate.showTapLog(nil)
        let logWindow = TapLogWindowController.shared
        XCTAssertTrue(logWindow.window?.isVisible ?? false)
        try await waitUntil(timeout: 5, "the log text") { logWindow.textView.string.contains("ready on 127.0.0.1:") }
        XCTAssertTrue(logWindow.textView.string.contains("tap dev --app talk.md"))

        // Each open deck has its own log.
        let second = try await openDeck(try Fixtures.copyDeck("plain.md"))
        _ = try await waitForRunningTap(second)
        logWindow.reload()
        XCTAssertEqual(logWindow.picker.segmentCount, 2)

        // The About window shows the bundled tap version.
        try await waitUntil(timeout: 10, "the tap version") { AppEnvironment.shared.bundledTapVersion != nil }
        let credits = try XCTUnwrap(appDelegate.aboutPanelOptions()[.credits] as? NSAttributedString)
        let appVersion = try XCTUnwrap(Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String)
        XCTAssertEqual(credits.string, "Bundled tap \(appVersion)")
        logWindow.close()
    }
}
