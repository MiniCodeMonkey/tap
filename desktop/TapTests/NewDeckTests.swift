import XCTest
@testable import Tap

final class NewDeckTests: HostedTestCase {
    var appDelegate: AppDelegate { NSApp.delegate as! AppDelegate }

    /// Waits for the sheet the New Deck command puts on `window`.
    func newDeckSheet(on window: NSWindow?) async throws -> NewDeckSheet {
        try await waitUntil(timeout: 10, "the New Deck sheet") { window?.attachedSheet is NewDeckSheet }
        return try XCTUnwrap(window?.attachedSheet as? NewDeckSheet)
    }

    func testNewDeck() async throws {
        // With no deck open, File > New Deck goes on the welcome window.
        appDelegate.showWelcomeIfNoDecks()
        let welcome = WelcomeWindowController.shared
        XCTAssertTrue(welcome.newDeckButton.isEnabled)
        let lastUsed = try Fixtures.temporaryFolder()
        AppEnvironment.shared.generalSettings.lastNewDeckFolder = lastUsed
        AppEnvironment.shared.generalSettings.defaultTheme = "terminal"
        welcome.newDeckButton.performClick(nil)
        let sheet = try await newDeckSheet(on: welcome.window)
        XCTAssertEqual(sheet.locationPopup.titleOfSelectedItem, lastUsed.lastPathComponent, "the last used folder is preselected")
        XCTAssertTrue(FilePaths.same(sheet.request.location, lastUsed), "\(sheet.request.location.path) is \(lastUsed.path)")
        // Another folder, through Other…: Create remembers this one.
        let location = try Fixtures.temporaryFolder()
        sheet.chooseFolder = { $0(location) }
        sheet.locationPopup.selectItem(withTitle: "Other…")
        sheet.locationChanged(sheet.locationPopup)
        XCTAssertTrue(FilePaths.same(sheet.request.location, location))
        XCTAssertNotNil(sheet.locationPopup.selectedItem?.image, "the NewDeck board's folder icon")
        try await waitUntil(timeout: 20, "the grid") { !sheet.grid.cells.isEmpty }
        XCTAssertEqual(sheet.grid.selectedSlug, "terminal", "the General default is preselected")
        XCTAssertFalse(sheet.grid.showsFooter, "no deck to set a theme on")
        XCTAssertEqual(sheet.grid.sectionTitles, [], "one flat grid, as the NewDeck board draws it")
        XCTAssertEqual(sheet.grid.cellSize, ThemeGridViewController.sheetCellSize, "the NewDeckHintNoSlug board's 82x46 cells")
        XCTAssertEqual(ThemeGridViewController.sheetCellSize, NSSize(width: 82, height: 46))
        XCTAssertEqual(sheet.grid.cell(for: "terminal")?.nameLabel.font?.pointSize, 10.5, "the sheet's names")
        let themeCount = try XCTUnwrap(AppEnvironment.shared.themeImages.catalog?.themes.count)
        XCTAssertEqual(sheet.scrollHint.stringValue, "Scroll for all \(themeCount) themes", "the count is tap's")
        XCTAssertEqual(sheet.locationHint.stringValue, "Creates a folder named after the title, with the deck and images/")
        XCTAssertGreaterThanOrEqual(sheet.frame.width, 560, "the NewDeck board's sheet, wide enough for five cells")

        sheet.titleField.stringValue = "Debugging Production at 3am"
        sheet.titleChanged(sheet.titleField)
        sheet.grid.cell(for: "default")?.performClick(nil)
        XCTAssertEqual(sheet.request.arguments, ["new", "--yes", "--title", "Debugging Production at 3am", "--folder", location.path, "--json"], "Default: no --theme, tap new's own default")
        sheet.grid.cell(for: "blueprint")?.performClick(nil)
        XCTAssertEqual(sheet.request.arguments, ["new", "--yes", "--title", "Debugging Production at 3am", "--theme", "blueprint", "--folder", location.path, "--json"])
        sheet.createButton.performClick(nil)

        let expected = location.appendingPathComponent("debugging-production-at-3am/debugging-production-at-3am.md")
        try await waitUntil(timeout: 30, "the new deck to open") {
            NSDocumentController.shared.documents.contains { $0.fileURL.map { FilePaths.same($0, expected) } ?? false }
        }
        let document = try XCTUnwrap(NSDocumentController.shared.documents.first { $0.fileURL.map { FilePaths.same($0, expected) } ?? false } as? DeckDocument)
        defer { document.close() }
        XCTAssertNil(welcome.window?.attachedSheet)
        XCTAssertFalse(welcome.window?.isVisible ?? false)
        var isFolder: ObjCBool = false
        XCTAssertTrue(FileManager.default.fileExists(atPath: location.appendingPathComponent("debugging-production-at-3am/images").path, isDirectory: &isFolder) && isFolder.boolValue,
                      "tap made images/")
        let text = try String(contentsOf: expected, encoding: .utf8)
        XCTAssertTrue(text.contains("theme: blueprint"))
        XCTAssertTrue(text.contains("title: \"Debugging Production at 3am\""))
        let remembered = try XCTUnwrap(AppEnvironment.shared.generalSettings.lastNewDeckFolder)
        XCTAssertTrue(FilePaths.same(remembered, location), "the folder Create used is remembered: \(remembered.path)")
        // tap records an approval for the deck it made.
        let listed = try await TapApproval.run(["approval", "list", "--json"], configHome: configHome)
        XCTAssertTrue(listed.contains(Fixtures.realPath(of: expected)), "tap approval list: \(listed)")
        _ = try await waitForRunningTap(document)
    }

    func testTheSheetGoesOnTheDeckWindowAndReportsTapsError() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        // The testable entry names the window; the menu action resolves it from the key or main window.
        appDelegate.newDeck(on: window)
        let sheet = try await newDeckSheet(on: window)
        let location = try Fixtures.temporaryFolder()
        sheet.chooseFolder = { $0(location) }
        sheet.locationPopup.selectItem(withTitle: "Other…")
        sheet.locationChanged(sheet.locationPopup)
        XCTAssertTrue(FilePaths.same(sheet.request.location, location))
        XCTAssertEqual(sheet.locationPopup.titleOfSelectedItem, location.lastPathComponent, "the chosen folder joins the list")
        // A second New Deck on the same window is refused, with a beep and a line in the deck's log.
        appDelegate.newDeck(on: window)
        XCTAssertTrue(window.attachedSheet === sheet)
        XCTAssertTrue(try XCTUnwrap(document.sessionController).session.log.text.contains("New Deck was not shown: a sheet is up on this window"))
        // A location that is gone by the time Create runs: tap's error, in the sheet, the sheet stays.
        try FileManager.default.removeItem(at: location)
        sheet.titleField.stringValue = "Gone"
        sheet.titleChanged(sheet.titleField)
        sheet.createButton.performClick(nil)
        XCTAssertFalse(sheet.cancelButton.isEnabled, "no Cancel while tap new runs: the deck it writes opens")
        try await waitUntil(timeout: 20, "tap's error") { !sheet.errorLabel.isHidden }
        XCTAssertTrue(sheet.cancelButton.isEnabled)
        XCTAssertTrue(sheet.errorLabel.stringValue.contains("does not exist"), sheet.errorLabel.stringValue)
        XCTAssertTrue(window.attachedSheet === sheet, "the sheet stays for another try")
        XCTAssertTrue(sheet.createButton.isEnabled)
        sheet.cancelButton.performClick(nil)
        try await waitUntil(timeout: 5, "the sheet to close") { window.attachedSheet == nil }
    }

    /// With a deck open but a window of another kind in front (Settings, the
    /// Tap Log), the sheet goes on a visible deck window, never on the hidden
    /// welcome window.
    func testTheHostIsAVisibleDeckWindowWhenAnotherWindowIsInFront() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let deckWindow = try XCTUnwrap(document.windowControllers.first?.window)
        let other = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 200, height: 100), styleMask: [.titled], backing: .buffered, defer: false)
        other.isReleasedWhenClosed = false
        defer { other.orderOut(nil) }
        let host = appDelegate.hostWindowForNewDeck(keyWindow: other, mainWindow: nil)
        XCTAssertTrue(host === deckWindow, "the frontmost deck window")
        XCTAssertNil(WelcomeWindowController.shared.window?.attachedSheet)
        document.close()
        let noDeck = appDelegate.hostWindowForNewDeck(keyWindow: other, mainWindow: nil)
        XCTAssertTrue(noDeck === WelcomeWindowController.shared.window, "no deck: the welcome window, shown")
        XCTAssertTrue(WelcomeWindowController.shared.window?.isVisible ?? false)
    }

    func testAnEmptyTitleCannotCreate() async throws {
        appDelegate.showWelcomeIfNoDecks()
        appDelegate.newDeck(on: WelcomeWindowController.shared.window)
        let sheet = try await newDeckSheet(on: WelcomeWindowController.shared.window)
        sheet.titleField.stringValue = ""
        sheet.titleChanged(sheet.titleField)
        XCTAssertFalse(sheet.createButton.isEnabled)
        sheet.titleField.stringValue = "A"
        sheet.titleChanged(sheet.titleField)
        XCTAssertTrue(sheet.createButton.isEnabled)
        sheet.cancelButton.performClick(nil)
    }
}
