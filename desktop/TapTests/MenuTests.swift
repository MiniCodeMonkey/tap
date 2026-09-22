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
}
