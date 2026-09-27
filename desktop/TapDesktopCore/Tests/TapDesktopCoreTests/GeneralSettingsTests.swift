import XCTest
@testable import TapDesktopCore

final class GeneralSettingsTests: XCTestCase {
    func fresh() throws -> GeneralSettings {
        let name = "TapDesktopCoreTests.general.\(UUID().uuidString)"
        let defaults = try XCTUnwrap(UserDefaults(suiteName: name))
        addTeardownBlock { defaults.removePersistentDomain(forName: name) }
        return GeneralSettings(defaults: defaults)
    }

    func testDefaultsAreTheSpecs() throws {
        let settings = try fresh()
        XCTAssertEqual(settings.fontSize, 13)
        XCTAssertEqual(settings.lineSpacing, .normal)
        XCTAssertNil(settings.defaultTheme, "no default theme means tap new's own default")
        XCTAssertEqual(settings.autosaveDelay, 1, "the spec's one second")
        XCTAssertNil(settings.lastNewDeckFolder)
    }

    func testChangesPersistAndPostANotification() throws {
        let settings = try fresh()
        var posted = 0
        let observer = NotificationCenter.default.addObserver(forName: GeneralSettings.didChangeNotification, object: settings, queue: nil) { _ in posted += 1 }
        defer { NotificationCenter.default.removeObserver(observer) }
        settings.fontSize = 15
        settings.lineSpacing = .roomy
        settings.defaultTheme = "terminal"
        settings.autosaveDelay = 5
        settings.lastNewDeckFolder = URL(fileURLWithPath: "/tmp/talks")
        XCTAssertEqual(posted, 5)
        let again = GeneralSettings(defaults: settings.defaults)
        XCTAssertEqual(again.fontSize, 15)
        XCTAssertEqual(again.lineSpacing, .roomy)
        XCTAssertEqual(again.defaultTheme, "terminal")
        XCTAssertEqual(again.autosaveDelay, 5)
        XCTAssertEqual(again.lastNewDeckFolder?.path, "/tmp/talks")
        settings.defaultTheme = nil
        XCTAssertNil(GeneralSettings(defaults: settings.defaults).defaultTheme)
    }

    func testAValueOutsideTheChoicesReadsAsTheDefault() throws {
        let settings = try fresh()
        settings.defaults.set(3, forKey: "TapEditorFontSize")
        settings.defaults.set("huge", forKey: "TapEditorLineSpacing")
        settings.defaults.set(-2.0, forKey: "TapAutosaveDelay")
        XCTAssertEqual(settings.fontSize, 13)
        XCTAssertEqual(settings.lineSpacing, .normal)
        XCTAssertEqual(settings.autosaveDelay, 1)
    }

    func testLineHeightsScaleWithTheFont() {
        XCTAssertEqual(GeneralSettings.LineSpacing.normal.lineHeight(forFontSize: 13), 21, "D2's editor line height at the default")
        XCTAssertEqual(GeneralSettings.LineSpacing.tight.lineHeight(forFontSize: 13), 18)
        XCTAssertEqual(GeneralSettings.LineSpacing.roomy.lineHeight(forFontSize: 13), 25)
        XCTAssertGreaterThan(GeneralSettings.LineSpacing.normal.lineHeight(forFontSize: 16), 21)
        XCTAssertEqual(GeneralSettings.fontSizes, [11, 12, 13, 14, 15, 16, 18])
        XCTAssertEqual(GeneralSettings.autosaveDelays, [0.5, 1, 2, 5, 10])
    }
}
