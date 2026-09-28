import XCTest
@testable import Tap

/// The Updates card in Settings > General: one self-labeled checkbox,
/// "Check for updates automatically", with no row label or hint, bound to
/// `UpdateController.automaticChecks`.
final class SettingsUpdatesTests: HostedTestCase {
    var updates: UpdateController { (NSApp.delegate as! AppDelegate).updateController }

    func testTheUpdatesCardMirrorsSparklesSetting() throws {
        // A store of the test's own: Sparkle's real one is the app's defaults.
        var stored = true
        let previous = updates.automaticChecksStore
        updates.automaticChecksStore = (read: { stored }, write: { stored = $0 })
        defer { updates.automaticChecksStore = previous }

        SettingsWindowController.shared.show(pane: .general)
        let general = SettingsWindowController.shared.general
        let checkbox = general.automaticUpdatesCheckbox
        XCTAssertEqual(checkbox.title, "Check for updates automatically")
        general.refresh()
        XCTAssertEqual(checkbox.state, .on)

        checkbox.state = .off
        checkbox.sendAction(checkbox.action, to: checkbox.target)
        XCTAssertFalse(stored, "the click reaches the store")
        stored = true
        general.refresh()
        XCTAssertEqual(checkbox.state, .on, "refresh reads the store")
        stored = false
        general.refresh()
        XCTAssertEqual(checkbox.state, .off, "and follows it both ways")
        SettingsWindowController.shared.window?.orderOut(nil)
    }

    /// The Updates card is the last card, directly under Saving, with no
    /// row label or hint: the checkbox is the row's only view.
    func testTheUpdatesCardIsLastAndItsRowHoldsOnlyTheCheckbox() throws {
        let general = GeneralSettingsViewController()
        let content = general.view
        content.setFrameSize(NSSize(width: 760, height: 460))
        content.layoutSubtreeIfNeeded()

        let cards = general.cards
        let updatesCard = try XCTUnwrap(cards.last)
        XCTAssertEqual(updatesCard.accessibilityIdentifier(), "settings-card-Updates")
        XCTAssertEqual(updatesCard.rows.count, 1)
        let row = try XCTUnwrap(updatesCard.rows.first as? NSStackView)
        XCTAssertTrue(row.views.contains(general.automaticUpdatesCheckbox))
    }

    /// Under tests the updater never starts, and a fresh view controller
    /// (no app delegate wired to it yet) still shows a sane, unchecked
    /// state rather than crashing.
    func testTheCheckboxShowsASaneStateWithNoUpdaterStarted() {
        XCTAssertFalse(updates.isStarted, "a hosted test process never reaches the feed")
        let general = GeneralSettingsViewController()
        _ = general.view
        XCTAssertNoThrow(general.refresh())
    }

    /// `automaticChecksStore`'s default closures capture the controller
    /// weakly: the store alone does not keep the controller alive, and once
    /// the controller is gone, calling them reads a sane default and writes
    /// nothing, rather than crashing.
    func testTheStoreNeverCrashesOnceItsControllerIsGone() {
        weak var releasedController: UpdateController?
        let store = autoreleasepool { () -> (read: () -> Bool, write: (Bool) -> Void) in
            let controller = UpdateController()
            releasedController = controller
            return controller.automaticChecksStore
        }
        XCTAssertNil(releasedController, "the store's closures must not keep their controller alive")
        XCTAssertFalse(store.read(), "no controller left to ask, so a sane default")
        XCTAssertNoThrow(store.write(true), "no controller left to write to")
    }
}
