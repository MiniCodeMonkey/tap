import XCTest

/// The real approval sheet on the real screen, with the real keyboard.
/// Runs on CI; locally it would open a window (the person's rule).
final class LiveCodeUITests: UITestCase {
    func testReturnOnTheApprovalSheetIsDontAllow() throws {
        let deck = try copyFixture("live-code.md")
        let home = try isolatedConfigHome()
        let defaultsSuite = "TapUITests.livecode.\(UUID().uuidString)"
        addTeardownBlock { UserDefaults.standard.removePersistentDomain(forName: defaultsSuite) }
        let application = XCUIApplication()
        application.launchArguments = ["-TapOpenOnLaunch", deck.path, "-ApplePersistenceIgnoreState", "YES",
                                       "-TapConfigHome", home.path, "-TapDefaultsSuite", defaultsSuite, "-FocusHintShown", "YES"]
        application.launch()
        let sheet = application.sheets.firstMatch
        XCTAssertTrue(sheet.waitForExistence(timeout: 30), "the approval sheet")
        XCTAssertTrue(sheet.staticTexts["This deck can run code on your Mac"].waitForExistence(timeout: 5))
        Thread.sleep(forTimeInterval: 1)
        application.typeKey(.return, modifierFlags: [])
        let gone = NSPredicate(format: "exists == false")
        expectation(for: gone, evaluatedWith: sheet)
        waitForExpectations(timeout: 10)
        // tap writes a yes within a moment of the answer; three seconds of no approvals is the proof that Return granted nothing.
        let settingsFile = home.appendingPathComponent("tap/settings.yaml")
        for _ in 0..<15 {
            let settings = (try? String(contentsOf: settingsFile, encoding: .utf8)) ?? ""
            XCTAssertFalse(settings.contains("approvals"), "Return granted nothing")
            Thread.sleep(forTimeInterval: 0.2)
        }
        XCTAssertTrue(application.textViews["editor"].waitForExistence(timeout: 10), "the deck is open and usable")
    }
}
