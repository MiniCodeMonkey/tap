import XCTest

/// A real talk on the real screen, driven with the real keyboard and
/// pointer. It runs on CI (the Desktop UI Tests job), never on a person's
/// machine: it covers the screen. tap's settings and the app's own go to
/// folders and suites of the test's own.
final class PresentingUITests: UITestCase {
    /// A settings folder with the recording question answered no, so the
    /// talk asks nothing and records nothing.
    func configHome() throws -> URL {
        try isolatedConfigHome(settings: "present:\n  record: false\n")
    }

    func launchForPresenting() throws -> XCUIApplication {
        let deck = try copyFixture("ops.md")
        let application = XCUIApplication()
        let defaultsSuite = "TapUITests.presenting.\(UUID().uuidString)"
        addTeardownBlock { UserDefaults.standard.removePersistentDomain(forName: defaultsSuite) }
        application.launchArguments = ["-TapOpenOnLaunch", deck.path, "-ApplePersistenceIgnoreState", "YES",
                                       "-TapConfigHome", try configHome().path, "-TapDefaultsSuite", defaultsSuite,
                                       "-FocusHintShown", "YES"]
        application.launch()
        return application
    }

    func testPlayThroughThePopoverAndEscapeStops() throws {
        let application = try launchForPresenting()
        let play = application.buttons["play-button"]
        XCTAssertTrue(play.waitForExistence(timeout: 30))
        Thread.sleep(forTimeInterval: 2)
        play.click()
        let start = application.buttons["start-presenting"]
        XCTAssertTrue(start.waitForExistence(timeout: 5), "the Present popover")
        start.click()
        let audience = application.windows["audience-window"]
        XCTAssertTrue(audience.waitForExistence(timeout: 30), "the audience window covers the screen")
        Thread.sleep(forTimeInterval: 3)
        // The arrow keys go to tap's page.
        application.typeKey(.rightArrow, modifierFlags: [])
        Thread.sleep(forTimeInterval: 1)
        // Option-Tab shows the presenter window over the audience window on one display, and hides it again.
        application.typeKey("\t", modifierFlags: [.option])
        XCTAssertTrue(application.windows["presenter-window"].waitForExistence(timeout: 5))
        Thread.sleep(forTimeInterval: 1)
        application.typeKey("\t", modifierFlags: [.option])
        Thread.sleep(forTimeInterval: 1)
        XCTAssertFalse(application.windows["presenter-window"].exists)
        application.typeKey(.escape, modifierFlags: [])
        let gone = NSPredicate(format: "exists == false")
        expectation(for: gone, evaluatedWith: audience)
        waitForExpectations(timeout: 20)
        XCTAssertTrue(application.textViews["editor"].exists, "back to the editor")
    }

    func testRehearseByShortcutAndStopFromTheToolbar() throws {
        let application = try launchForPresenting()
        let editor = application.textViews["editor"]
        XCTAssertTrue(editor.waitForExistence(timeout: 30))
        Thread.sleep(forTimeInterval: 2)
        application.typeKey("p", modifierFlags: [.command, .option, .shift])
        let presenter = application.windows["presenter-window"]
        XCTAssertTrue(presenter.waitForExistence(timeout: 30), "the presenter view, full screen")
        XCTAssertFalse(application.windows["audience-window"].exists)
        Thread.sleep(forTimeInterval: 2)
        // The toolbar slides up when the pointer reaches the bottom edge.
        // XCUICoordinate's normalized offset is measured from the top, so
        // dy: 1.0 is the bottom edge itself, inside the reveal band (y <= 2).
        presenter.coordinate(withNormalizedOffset: CGVector(dx: 0.5, dy: 1.0)).hover()
        let stop = application.buttons["stop-button"]
        XCTAssertTrue(stop.waitForExistence(timeout: 5))
        stop.click()
        let gone = NSPredicate(format: "exists == false")
        expectation(for: gone, evaluatedWith: presenter)
        waitForExpectations(timeout: 20)
    }
}
