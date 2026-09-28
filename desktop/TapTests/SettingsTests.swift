import XCTest
@testable import Tap

final class SettingsTests: HostedTestCase {
    var appDelegate: AppDelegate { NSApp.delegate as! AppDelegate }
    var settings: SettingsWindowController { SettingsWindowController.shared }

    /// A pop-up or a segmented control is never performClick'ed (that opens a
    /// modal menu loop): the test selects, then sends the control's action.
    func choose(_ popup: NSPopUpButton, _ title: String) {
        popup.selectItem(withTitle: title)
        popup.sendAction(popup.action, to: popup.target)
    }

    func select(_ control: NSSegmentedControl, _ segment: Int) {
        control.selectedSegment = segment
        control.sendAction(control.action, to: control.target)
    }

    override func tearDown() async throws {
        settings.window?.orderOut(nil)
        try await super.tearDown()
    }

    func testSettingsWindow() async throws {
        let tapMenu = try XCTUnwrap(NSApp.mainMenu?.items.first { $0.title == "Tap" }?.submenu)
        let item = try XCTUnwrap(tapMenu.items.first { $0.title == "Settings…" })
        XCTAssertEqual(item.keyEquivalent, ",")
        XCTAssertEqual(item.action, #selector(AppDelegate.showSettings(_:)))
        appDelegate.showSettings(nil)
        try await waitUntil(timeout: 5, "the window") { self.settings.window?.isVisible ?? false }
        XCTAssertEqual(settings.tabViewController.tabViewItems.map(\.label), ["General", "Live Code", "Image Generation", "Command Line"])
        XCTAssertEqual(settings.tabViewController.tabStyle, .toolbar)

        // Live Code lists the approvals tap keeps, through tap approval list.
        let deck = try Fixtures.copyDeck("live-code.md")
        try approveLiveCode(for: deck)
        settings.show(pane: .liveCode)
        settings.liveCode.reload()
        try await waitUntil(timeout: 20, "tap approval list") { self.settings.liveCode.records.contains { $0.deck == Fixtures.realPath(of: deck) } }
        let row = try XCTUnwrap(settings.liveCode.records.firstIndex { $0.deck == Fixtures.realPath(of: deck) })
        XCTAssertEqual(settings.liveCode.records[row].deckName, "live-code.md")
        XCTAssertEqual(settings.liveCode.records[row].driverSummary, "shell, sqlite")
        XCTAssertEqual(settings.liveCode.table.numberOfRows, settings.liveCode.records.count)
        XCTAssertTrue(settings.liveCode.introLabel.stringValue.contains("~/.config/tap/settings.yaml"))
        XCTAssertFalse(settings.liveCode.revokeButton.isEnabled, "nothing selected")
        settings.liveCode.table.selectRowIndexes([row], byExtendingSelection: false)
        XCTAssertTrue(settings.liveCode.revokeButton.isEnabled)
    }

    /// A Revoke in Settings is what tap sees: the approvals are tap's file,
    /// which the CLI reads too. (The "Shared settings" scenario also names
    /// the recording consent, which Settings does not have; that row is not claimed.)
    func testRevokeInSettingsIsWhatTapSees() async throws {
        let deck = try Fixtures.copyDeck("live-code.md")
        try approveLiveCode(for: deck)
        appDelegate.showSettings(nil)
        settings.show(pane: .liveCode)
        settings.liveCode.reload()
        try await waitUntil(timeout: 20, "the approval") { self.settings.liveCode.records.contains { $0.deck == Fixtures.realPath(of: deck) } }
        var revealed: [URL] = []
        settings.liveCode.revealInFinder = { revealed.append($0) }
        let row = try XCTUnwrap(settings.liveCode.records.firstIndex { $0.deck == Fixtures.realPath(of: deck) })
        settings.liveCode.table.selectRowIndexes([row], byExtendingSelection: false)
        settings.liveCode.revealButton.performClick(nil)
        XCTAssertEqual(revealed.map { Fixtures.realPath(of: $0) }, [Fixtures.realPath(of: deck)])

        settings.liveCode.revokeButton.performClick(nil)
        try await waitUntil(timeout: 20, "the revoke to land") { !self.settings.liveCode.records.contains { $0.deck == Fixtures.realPath(of: deck) } }
        let listed = try await TapApproval.run(["approval", "list", "--json"], configHome: configHome)
        XCTAssertFalse(listed.contains(Fixtures.realPath(of: deck)), "tap approval list agrees: \(listed)")
        XCTAssertFalse(storedApprovals().contains(Fixtures.realPath(of: deck)), "the file tap dev and tap present read")
    }

    /// The General pane's Default theme popup lists the catalog, and
    /// opening it renders no theme: the renders, and the export engine's
    /// download on a Mac that has none, wait for a grid that shows them.
    func testOpeningSettingsStartsNoThemeRender() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.themeShow(recordingTo: record)
        let loader = AppEnvironment.shared.themeImages
        let general = GeneralSettingsViewController()
        _ = general.view
        try await waitUntil(timeout: 30, "the catalog, with the loader idle") { loader.catalog != nil && !loader.isWorking }
        try await waitUntil(timeout: 5, "the Default theme popup") { general.defaultThemePopup.numberOfItems > 1 }
        let runs = try String(contentsOf: record, encoding: .utf8)
        XCTAssertTrue(runs.contains("arguments: theme list"), runs)
        XCTAssertFalse(runs.contains("arguments: theme show"), "no render: \(runs)")
    }

    func testGeneralSettings() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let editor = try XCTUnwrap(document.sessionController?.editor)
        XCTAssertEqual(editor.font?.pointSize, 13)
        appDelegate.showSettings(nil)
        settings.show(pane: .general)
        let general = settings.general
        XCTAssertEqual(general.fontSizePopup.titleOfSelectedItem, "13 pt")
        XCTAssertEqual(general.lineSpacingControl.label(forSegment: general.lineSpacingControl.selectedSegment), "Normal")
        XCTAssertEqual(general.autosavePopup.titleOfSelectedItem, "1 second")
        XCTAssertEqual(general.defaultThemePopup.titleOfSelectedItem, "tap's default")
        try await waitUntil(timeout: 20, "the catalog") { AppEnvironment.shared.themeImages.catalog != nil }
        XCTAssertTrue(general.defaultThemePopup.itemTitles.contains("Terminal"), "every theme from tap: \(general.defaultThemePopup.itemTitles)")

        choose(general.fontSizePopup, "16 pt")
        select(general.lineSpacingControl, 2)
        choose(general.defaultThemePopup, "Terminal")
        choose(general.autosavePopup, "5 seconds")

        let stored = AppEnvironment.shared.generalSettings
        XCTAssertEqual(stored.fontSize, 16)
        XCTAssertEqual(stored.lineSpacing, .roomy)
        XCTAssertEqual(stored.defaultTheme, "terminal", "the slug, for tap new --theme")
        XCTAssertEqual(stored.autosaveDelay, 5)
        XCTAssertEqual(NSDocumentController.shared.autosavingDelay, 5)
        XCTAssertEqual(editor.font?.pointSize, 16, "the open editor follows at once")
        XCTAssertEqual(EditorTypography.current.lineHeight, GeneralSettings.LineSpacing.roomy.lineHeight(forFontSize: 16))
        let style = editor.textStorage?.attribute(.paragraphStyle, at: editor.hiddenLength, effectiveRange: nil) as? NSParagraphStyle
        XCTAssertEqual(style?.minimumLineHeight, EditorTypography.current.lineHeight, "the text was restyled")
        choose(general.fontSizePopup, "13 pt")
        select(general.lineSpacingControl, 1)
        choose(general.autosavePopup, "1 second")
    }

    func testAFailedRevokeShowsTapsMessage() async throws {
        appDelegate.showSettings(nil)
        settings.show(pane: .liveCode)
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.write("""
          "approval list") echo '{"ok": true, "approvals": [{"deck": "/t/gone.md", "drivers": ["shell"], "approvedAt": "2026-09-25T00:00:00Z"}]}'; exit 0 ;;
          "approval revoke") echo '{"ok": false, "error": {"code": "not_approved", "message": "/t/gone.md is not approved to run live code"}}'; exit 1 ;;
        """, recordingTo: record)
        settings.liveCode.reload()
        try await waitUntil(timeout: 10, "the row") { self.settings.liveCode.records.count == 1 }
        settings.liveCode.table.selectRowIndexes([0], byExtendingSelection: false)
        settings.liveCode.revokeButton.performClick(nil)
        try await waitUntil(timeout: 10, "tap's message") { !self.settings.liveCode.errorLabel.isHidden }
        XCTAssertEqual(settings.liveCode.errorLabel.stringValue, "/t/gone.md is not approved to run live code")
    }
}
