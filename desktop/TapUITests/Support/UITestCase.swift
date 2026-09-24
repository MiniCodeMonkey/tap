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

    @discardableResult
    func launch(withDeck deck: URL) -> XCUIApplication {
        let application = XCUIApplication()
        application.launchArguments = ["-TapOpenOnLaunch", deck.path, "-ApplePersistenceIgnoreState", "YES"]
        application.launch()
        return application
    }
}
