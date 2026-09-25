import XCTest
@testable import Tap

final class MenuTests: HostedTestCase {
    func testMenuBar() throws {
        let menus = try XCTUnwrap(NSApp.mainMenu).items.compactMap(\.submenu)
        XCTAssertEqual(menus.map(\.title), ["Tap", "File", "Edit", "Slide", "View", "Present", "Window", "Help"])
        XCTAssertTrue(NSApp.windowsMenu === menus[6])
        XCTAssertTrue(NSApp.helpMenu === menus[7])
        XCTAssertNotNil(menus[1].items.first { $0.title == "Open…" && $0.keyEquivalent == "o" })
        XCTAssertNotNil(menus[5].items.first { $0.title == "Play" && $0.keyEquivalentModifierMask == [.command, .option] })
    }

    /// AppKit adds Revert To only to a menu bar that is in place when launch
    /// finishes; that item is how a person reaches Browse All Versions.
    func testFileMenuHasRevertTo() throws {
        let file = try XCTUnwrap(NSApp.mainMenu?.items.first { $0.title == "File" }?.submenu)
        let revertTo = try XCTUnwrap(file.items.first { $0.title == "Revert To" }, "File menu: \(file.items.map(\.title))")
        XCTAssertFalse(revertTo.isHidden)
        XCTAssertNotNil(revertTo.submenu)
    }
}
