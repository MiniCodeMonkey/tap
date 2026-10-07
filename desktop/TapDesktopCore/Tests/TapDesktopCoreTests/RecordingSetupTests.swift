import XCTest
@testable import TapDesktopCore

final class RecordingSetupTests: XCTestCase {
    private func setup(_ microphone: MicrophoneAccess = .notDetermined, screen: Bool = false, dismissed: Bool = false) -> RecordingSetup {
        RecordingSetup(microphone: microphone, screenRecordingAllowed: screen, dismissed: dismissed)
    }

    func testTheMicrophoneComesFirst() {
        let fresh = setup()
        XCTAssertEqual(fresh.currentStep, .microphone)
        XCTAssertEqual(fresh.action(for: .microphone), .allow)
        XCTAssertEqual(fresh.action(for: .screenRecording), .none, "later steps have no button")
        XCTAssertEqual(setup(.denied).action(for: .microphone), .openSettings, "macOS asks only once")
        XCTAssertEqual(setup(.denied, screen: true).currentStep, .microphone)
    }

    func testScreenRecordingFollowsAndWaitsInSettings() {
        let ready = setup(.allowed)
        XCTAssertEqual(ready.currentStep, .screenRecording)
        XCTAssertEqual(ready.action(for: .microphone), .none)
        XCTAssertEqual(ready.action(for: .screenRecording), .openSettings)
        var opened = setup(.allowed)
        XCTAssertEqual(opened.action(for: .screenRecording), .openSettings, "a fresh launch offers the button")
        opened.screenRecordingSettingsOpened = true
        XCTAssertEqual(opened.action(for: .screenRecording), .waiting)
        XCTAssertTrue(ready.isDone(.microphone))
        XCTAssertFalse(ready.isDone(.screenRecording))
    }

    func testBothAllowedIsComplete() {
        let done = setup(.allowed, screen: true)
        XCTAssertTrue(done.isComplete)
        XCTAssertNil(done.currentStep)
        XCTAssertEqual(done.action(for: .screenRecording), .none)
    }

    func testShowsAtLaunchUntilSetUpOrPutOff() {
        XCTAssertTrue(setup().showsAtLaunch)
        XCTAssertTrue(setup(.allowed).showsAtLaunch, "one permission is still missing")
        XCTAssertFalse(setup(dismissed: true).showsAtLaunch)
        XCTAssertFalse(setup(.allowed, screen: true).showsAtLaunch, "with both allowed it never opens by itself")
    }

    func testStoreKeepsTheDismissal() throws {
        let defaults = try XCTUnwrap(UserDefaults(suiteName: "RecordingSetupTests.\(UUID().uuidString)"))
        let store = RecordingSetupStore(defaults: defaults)
        XCTAssertFalse(store.dismissed)
        store.dismissed = true
        XCTAssertTrue(RecordingSetupStore(defaults: defaults).dismissed)
    }
}
