import XCTest
@testable import TapDesktopCore

final class CommandLineToolTests: XCTestCase {
    func testLocatesEveryTapOnPathInOrder() {
        let existing: Set<String> = ["/opt/homebrew/bin/tap", "/Users/me/.local/bin/tap"]
        let found = CommandLineTool.locate(named: "tap", onPath: "/Users/me/.local/bin:/opt/homebrew/bin:/usr/bin:/bin", fileExists: { existing.contains($0) })
        XCTAssertEqual(found, ["/Users/me/.local/bin/tap", "/opt/homebrew/bin/tap"])
        XCTAssertEqual(CommandLineTool.locate(named: "tap", onPath: "", fileExists: { _ in true }), [])
        XCTAssertEqual(CommandLineTool.locate(named: "tap", onPath: "/a::/b", fileExists: { _ in true }), ["/a/tap", "/b/tap"], "an empty PATH entry is skipped")
    }

    func testKnowsItsOwnLink() {
        XCTAssertTrue(CommandLineTool.isBundledLink(destination: "/Applications/Tap.app/Contents/Resources/tap"))
        XCTAssertTrue(CommandLineTool.isBundledLink(destination: "/Users/me/Desktop/Tap 2.app/Contents/Resources/tap"))
        XCTAssertFalse(CommandLineTool.isBundledLink(destination: "/opt/homebrew/Cellar/tap/2.0.0/bin/tap"))
        XCTAssertFalse(CommandLineTool.isBundledLink(destination: nil), "a plain file has no destination")
    }

    func testTheInstallDecisionNeverTouchesWhatIsNotOurs() {
        XCTAssertEqual(CommandLineTool.installDecision(for: .none), .link)
        XCTAssertEqual(CommandLineTool.installDecision(for: .bundledLink), .replaceOwnLink, "our link to another copy of the app is ours to move")
        XCTAssertEqual(CommandLineTool.installDecision(for: .other), .refuse("a tap that Tap did not install is already there"))
    }

    func testPathOrderDecidesTheHint() {
        let path = "/Users/me/.local/bin:/opt/homebrew/bin:/usr/bin"
        XCTAssertEqual(CommandLineTool.directoryComesFirst("/Users/me/.local/bin", beforeDirectoryOf: "/opt/homebrew/bin/tap", onPath: path), true)
        XCTAssertEqual(CommandLineTool.directoryComesFirst("/Users/me/.local/bin", beforeDirectoryOf: "/opt/homebrew/bin/tap", onPath: "/opt/homebrew/bin:/Users/me/.local/bin"), false)
        XCTAssertNil(CommandLineTool.directoryComesFirst("/Users/me/.local/bin", beforeDirectoryOf: "/opt/homebrew/bin/tap", onPath: "/usr/bin"), "not on PATH at all")
        XCTAssertEqual(CommandLineTool.directoryComesFirst("/Users/me/.local/bin", beforeDirectoryOf: nil, onPath: path), true, "no other tap: first by default")
    }
}
