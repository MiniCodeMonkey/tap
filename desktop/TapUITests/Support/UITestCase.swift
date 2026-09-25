import XCTest

class UITestCase: XCTestCase {
    var repositoryRoot: URL {
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent() // Support
            .deletingLastPathComponent() // TapUITests
            .deletingLastPathComponent() // desktop
            .deletingLastPathComponent()
    }

    func copyFixture(_ name: String) throws -> URL {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("tap-ui-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let copy = folder.appendingPathComponent(name)
        try FileManager.default.copyItem(at: repositoryRoot.appendingPathComponent("desktop/TapTests/Fixtures/\(name)"), to: copy)
        return copy
    }

    /// The preview. The identifier sits on the WKWebView itself, which the
    /// accessibility tree shows as a Group; the WebView element inside it
    /// belongs to the web content process and carries the page's title as
    /// its label, never the identifier.
    func preview(in application: XCUIApplication) -> XCUIElement {
        application.groups["preview"]
    }

    /// The titles of the items in the menu a menu bar item or a menu item
    /// opens, for a failure message that says what the menu held when an
    /// item was missing. A separator shows as an empty title.
    func titles(of menuOwner: XCUIElement) -> [String] {
        menuOwner.menus.firstMatch.children(matching: .menuItem).allElementsBoundByIndex.map(\.title)
    }

    @discardableResult
    func launch(withDeck deck: URL) -> XCUIApplication {
        let application = XCUIApplication()
        application.launchArguments = ["-TapOpenOnLaunch", deck.path, "-ApplePersistenceIgnoreState", "YES"]
        application.launch()
        return application
    }
}
