import XCTest
import TapDesktopCore
@testable import Tap

/// Permissions that answer from fields and count what was asked, so no test shows a macOS prompt.
@MainActor
final class FakeRecordingPermissions: RecordingPermissions {
    var microphone = MicrophoneAccess.notDetermined
    var screenRecordingAllowed = false
    private(set) var microphoneRequests = 0
    private(set) var microphoneSettingsOpened = 0
    private(set) var screenRequests = 0

    func requestMicrophone() { microphoneRequests += 1 }
    func openMicrophoneSettings() { microphoneSettingsOpened += 1 }
    func requestScreenRecordingAndOpenSettings() { screenRequests += 1 }
}

final class RecordingSetupTests: HostedTestCase {
    var welcome: WelcomeWindowController { WelcomeWindowController.shared }
    var setup: RecordingSetupView { welcome.root.recordingSetup }
    var permissions: FakeRecordingPermissions!

    override func setUp() async throws {
        try await super.setUp()
        permissions = FakeRecordingPermissions()
        AppEnvironment.shared.recordingPermissions = permissions
        AppEnvironment.shared.recordingSetupStore = RecordingSetupStore(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.recording.\(UUID().uuidString)")))
        NSDocumentController.shared.clearRecentDocuments(nil)
    }

    override func tearDown() async throws {
        welcome.root.setShowsRecordingSetup(false)
        WelcomeWindowController.closeIfOpen()
        try await super.tearDown()
    }

    func helpMenuItem() throws -> NSMenuItem {
        try XCTUnwrap(NSApp.helpMenu?.items.first { $0.title == "Set Up Recording…" })
    }

    func testSetUpRecordingBeforeTheFirstTalk() async throws {
        // The Help menu opens the screen in the welcome window, in place of its layouts.
        let item = try helpMenuItem()
        XCTAssertTrue(NSApp.sendAction(try XCTUnwrap(item.action), to: nil, from: item))
        XCTAssertTrue(welcome.window?.isVisible ?? false)
        XCTAssertEqual(welcome.window?.contentView?.frame.size, NSSize(width: 880, height: 560))
        XCTAssertTrue(welcome.root.showsRecordingSetup)
        XCTAssertFalse(setup.isHidden)
        XCTAssertTrue(welcome.root.hero.isHidden && welcome.root.split.isHidden && welcome.root.filmstrip.isHidden)
        XCTAssertEqual(setup.accessibilityIdentifier(), "recording-setup")
        XCTAssertEqual(setup.titleLabel.stringValue, "Get ready to record your talks")
        XCTAssertEqual(setup.metaLabel.stringValue, "About a minute. You only do this once.")

        // Only the microphone has a button; the screen step waits, dimmed.
        XCTAssertEqual(setup.microphoneCard.accessibilityLabel(), "Microphone, not allowed")
        XCTAssertEqual(setup.screenCard.accessibilityLabel(), "Screen Recording, not allowed")
        XCTAssertFalse(setup.microphoneCard.button.isHidden)
        XCTAssertEqual(setup.microphoneCard.button.title, "Allow Microphone")
        XCTAssertTrue(setup.screenCard.button.isHidden)
        XCTAssertEqual(setup.screenCard.alphaValue, 0.55, accuracy: 0.01)
        XCTAssertEqual(setup.continueButton.style, .quiet)
        setup.microphoneCard.button.performClick(nil)
        XCTAssertEqual(permissions.microphoneRequests, 1)

        // A microphone macOS will not ask about again sends the person to Settings.
        permissions.microphone = .denied
        setup.refresh()
        XCTAssertEqual(setup.microphoneCard.button.title, "Open Settings")
        setup.microphoneCard.button.performClick(nil)
        XCTAssertEqual(permissions.microphoneSettingsOpened, 1)

        // Screen Recording: Open Settings asks macOS, then the button gives way to the waiting status and its guide.
        permissions.microphone = .allowed
        setup.refresh()
        XCTAssertEqual(setup.microphoneCard.accessibilityLabel(), "Microphone, allowed")
        XCTAssertEqual(setup.metaLabel.stringValue, "One step left.")
        XCTAssertEqual(setup.screenCard.button.title, "Open Settings")
        setup.screenCard.button.performClick(nil)
        XCTAssertEqual(permissions.screenRequests, 1)
        XCTAssertTrue(setup.screenCard.button.isHidden)
        XCTAssertEqual(setup.continueButton.style, .quiet)

        // Both allowed: the done state, and Continue is the main button.
        permissions.screenRecordingAllowed = true
        welcome.reload()
        XCTAssertEqual(setup.titleLabel.stringValue, "You're ready to record")
        XCTAssertEqual(setup.continueButton.style, .primary)
        XCTAssertTrue(setup.laterButton.superview?.isHidden ?? false, "Set Up Later is gone")
        setup.continueButton.performClick(nil)
        XCTAssertFalse(welcome.root.showsRecordingSetup)
        XCTAssertTrue(setup.isHidden)
    }

    func testSetUpLaterStoresTheDismissalAndShowsTheWelcomeWindow() async throws {
        welcome.showRecordingSetup()
        XCTAssertEqual(setup.laterButton.accessibilityIdentifier(), "recording-setup-later")
        setup.laterButton.performClick(nil)
        XCTAssertTrue(AppEnvironment.shared.recordingSetupStore.dismissed)
        XCTAssertFalse(welcome.root.showsRecordingSetup)
        XCTAssertTrue(setup.isHidden)
        XCTAssertTrue(welcome.window?.isVisible ?? false)
        XCTAssertFalse(welcome.emptyStateView.isHidden, "the normal content is back")

        // The Help menu brings the screen back even after Set Up Later.
        let item = try helpMenuItem()
        XCTAssertTrue(NSApp.sendAction(try XCTUnwrap(item.action), to: nil, from: item))
        XCTAssertTrue(welcome.root.showsRecordingSetup)
        XCTAssertTrue(welcome.root.hero.isHidden)
    }

    func testContinueTurnsPrimaryOnceBothAreAllowed() async throws {
        welcome.showRecordingSetup()
        XCTAssertEqual(setup.continueButton.style, .quiet)
        permissions.microphone = .allowed
        permissions.screenRecordingAllowed = true
        setup.refresh()
        XCTAssertEqual(setup.continueButton.style, .primary)
        XCTAssertEqual(setup.continueButton.accessibilityIdentifier(), "recording-setup-continue")
    }
}
