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

    /// Launches the app on `deck` with tap's settings in a folder of this
    /// test's own (-TapConfigHome) and the app's own settings in a
    /// defaults suite of its own (-TapDefaultsSuite), so a UI run never
    /// reads or writes the person's ~/.config/tap or the app's defaults.
    /// The deck's declared built-in drivers are approved ahead of time in
    /// that folder, so no approval sheet sits over the window. A custom
    /// driver's approval needs tap's own digest, which a UI test cannot
    /// write: a test of such a deck answers the sheet itself.
    @discardableResult
    func launch(withDeck deck: URL) throws -> XCUIApplication {
        let configHome = try isolatedConfigHome(approving: deck)
        let defaultsSuite = "TapUITests.\(UUID().uuidString)"
        addTeardownBlock { UserDefaults.standard.removePersistentDomain(forName: defaultsSuite) }
        let application = XCUIApplication()
        application.launchArguments = ["-TapOpenOnLaunch", deck.path, "-ApplePersistenceIgnoreState", "YES",
                                       "-TapConfigHome", configHome.path, "-TapDefaultsSuite", defaultsSuite]
        application.launch()
        return application
    }

    /// A tap settings folder for one test, holding `settings` and a
    /// name-only approval of `deck`'s declared drivers (tap's own record
    /// shape, keyed by the deck's real path), which covers built-in ones.
    func isolatedConfigHome(approving deck: URL? = nil, settings: String = "") throws -> URL {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("tap-ui-config-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: folder.appendingPathComponent("tap"), withIntermediateDirectories: true)
        var text = settings
        if let deck {
            let drivers = Self.declaredDrivers(in: (try? String(contentsOf: deck, encoding: .utf8)) ?? "")
            if !drivers.isEmpty {
                if !text.isEmpty, !text.hasSuffix("\n") { text += "\n" }
                text += "approvals:\n    - deck: \(Self.realPath(of: deck))\n      drivers: [\(drivers.joined(separator: ", "))]\n      approvedAt: 2026-09-25T00:00:00Z\n"
            }
        }
        try text.write(to: folder.appendingPathComponent("tap/settings.yaml"), atomically: true, encoding: .utf8)
        return folder
    }

    /// The driver names under `drivers:` in a deck's frontmatter, as the
    /// fixtures write it: one name per line, indented two spaces.
    static func declaredDrivers(in text: String) -> [String] {
        let lines = text.components(separatedBy: "\n")
        guard lines.first == "---", let end = lines.dropFirst().firstIndex(of: "---") else { return [] }
        var names: [String] = []
        var inDrivers = false
        for line in lines[1..<end] {
            if line.hasPrefix("drivers:") { inDrivers = true; continue }
            guard inDrivers else { continue }
            guard line.hasPrefix("  ") else { break }
            if !line.hasPrefix("   "), let colon = line.firstIndex(of: ":") {
                names.append(line[line.index(line.startIndex, offsetBy: 2)..<colon].trimmingCharacters(in: .whitespaces))
            }
        }
        return names
    }

    /// The path with every symlink resolved, as tap keys an approval:
    /// /var/folders is /private/var/folders here.
    static func realPath(of url: URL) -> String {
        var buffer = [Int8](repeating: 0, count: Int(PATH_MAX))
        guard let resolved = realpath(url.path, &buffer) else { return url.path }
        return String(cString: resolved)
    }
}
