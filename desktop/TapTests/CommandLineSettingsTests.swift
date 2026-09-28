import XCTest
@testable import Tap

final class CommandLineSettingsTests: HostedTestCase {
    var settings: SettingsWindowController { SettingsWindowController.shared }
    var linkDirectory: URL!
    var pathDirectory: URL!

    override func setUp() async throws {
        try await super.setUp()
        linkDirectory = try Fixtures.temporaryFolder().appendingPathComponent("local-bin")
        pathDirectory = try Fixtures.temporaryFolder()
        AppEnvironment.shared.commandLineInstaller = CommandLineInstaller(linkDirectory: linkDirectory, bundledTap: AppEnvironment.shared.tapExecutableURL)
        AppEnvironment.shared.extraEnvironment["PATH"] = "\(linkDirectory.path):\(pathDirectory.path):/usr/bin:/bin"
    }

    override func tearDown() async throws {
        settings.window?.orderOut(nil)
        try await super.tearDown()
    }

    var shownLinkDirectory: String { (linkDirectory.path as NSString).abbreviatingWithTildeInPath }

    /// A tap of someone else's on PATH: a script that answers --version.
    func installOtherTap(version: String) throws -> URL {
        let other = pathDirectory.appendingPathComponent("tap")
        try "#!/bin/sh\necho 'tap version \(version)'\n".write(to: other, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: other.path)
        return other
    }

    func showPane() async throws -> CommandLineSettingsViewController {
        (NSApp.delegate as! AppDelegate).showSettings(nil)
        settings.show(pane: .commandLine)
        let pane = settings.commandLine
        await pane.refresh()
        return pane
    }

    /// How many sheets the Settings window began while `action` ran.
    /// NSWindow posts willBeginSheet from inside beginSheet, so a count
    /// taken right after a synchronous action is final.
    func sheetsBegun(during action: () -> Void) -> Int {
        var count = 0
        let observer = NotificationCenter.default.addObserver(forName: NSWindow.willBeginSheetNotification, object: settings.window, queue: nil) { _ in count += 1 }
        action()
        NotificationCenter.default.removeObserver(observer)
        return count
    }

    func confirmSheet() async throws -> InstallConfirmSheet {
        try await waitUntil(timeout: 5, "the InstallConfirm sheet") { self.settings.window?.attachedSheet is InstallConfirmSheet }
        return try XCTUnwrap(settings.window?.attachedSheet as? InstallConfirmSheet)
    }

    func testInstallTheTapCommand() async throws {
        let pane = try await showPane()
        XCTAssertTrue(pane.bundledLabel.stringValue.hasPrefix("tap "), pane.bundledLabel.stringValue)
        XCTAssertTrue(pane.bundledPathLabel.stringValue.hasSuffix(".app/Contents/Resources/tap") && !pane.bundledPathLabel.stringValue.hasPrefix("/"),
                      "the path from the bundle, as the SettingsCLI board shows it: \(pane.bundledPathLabel.stringValue)")
        XCTAssertEqual(pane.otherLabel.stringValue, "No other tap is on your PATH.")
        XCTAssertEqual(pane.installButton.title, "Install in \(shownLinkDirectory)…")
        XCTAssertTrue(pane.installButton.isEnabled)
        XCTAssertEqual(pane.installHint.stringValue, "Asks first. \(shownLinkDirectory) is on your PATH.")

        // The InstallConfirm board, frame a: the sheet asks first.
        pane.installButton.performClick(nil)
        let sheet = try await confirmSheet()
        XCTAssertEqual(sheet.titleLabel.stringValue, "Install the tap command?")
        XCTAssertEqual(sheet.bodyLabel.stringValue, "Tap links its bundled tap into \(shownLinkDirectory), so Terminal runs the same tap as the app. Nothing else on your Mac changes.")
        XCTAssertEqual(sheet.pathLabel.stringValue, "\(shownLinkDirectory)/tap \u{2192} \(AppEnvironment.shared.tapExecutableURL.path)")
        XCTAssertEqual(sheet.subLabel.stringValue, "To remove it later, delete the link.")
        XCTAssertTrue(sheet.otherTapCard.isHidden, "no other tap to name")
        XCTAssertEqual(sheet.installButton.keyEquivalent, "\r", "Install is the default button")
        sheet.installButton.performClick(nil)
        try await waitUntil(timeout: 10, "the link") { FileManager.default.fileExists(atPath: self.linkDirectory.appendingPathComponent("tap").path) }
        let destination = try FileManager.default.destinationOfSymbolicLink(atPath: linkDirectory.appendingPathComponent("tap").path)
        XCTAssertEqual(destination, AppEnvironment.shared.tapExecutableURL.path)
        try await waitUntil(timeout: 5, "the pane to notice") { pane.installButton.title == "Installed" }
        XCTAssertFalse(pane.installButton.isEnabled)
        XCTAssertEqual(pane.otherLabel.stringValue, "No other tap is on your PATH.", "our own link is not another tap")
        XCTAssertNil(pane.otherTap)

        // Our link to another copy of the app: not another tap, and Install moves it to this app's tap.
        let copy = try Fixtures.temporaryFolder().appendingPathComponent("Tap 2.app/Contents/Resources/tap")
        try FileManager.default.createDirectory(at: copy.deletingLastPathComponent(), withIntermediateDirectories: true)
        try "#!/bin/sh\necho 'tap version 1.9.0'\n".write(to: copy, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: copy.path)
        try FileManager.default.removeItem(at: linkDirectory.appendingPathComponent("tap"))
        try FileManager.default.createSymbolicLink(at: linkDirectory.appendingPathComponent("tap"), withDestinationURL: copy)
        await pane.refresh()
        XCTAssertEqual(pane.otherLabel.stringValue, "No other tap is on your PATH.", "our link to another copy is ours, not another tap")
        XCTAssertEqual(pane.installButton.title, "Install in \(shownLinkDirectory)…")
        XCTAssertTrue(pane.installButton.isEnabled, "our own link is not refused as a foreign file")
        pane.installButton.performClick(nil)
        let replace = try await confirmSheet()
        replace.installButton.performClick(nil)
        try await waitUntil(timeout: 5, "the pane to notice") { pane.installButton.title == "Installed" }
        XCTAssertEqual(try FileManager.default.destinationOfSymbolicLink(atPath: linkDirectory.appendingPathComponent("tap").path), AppEnvironment.shared.tapExecutableURL.path, "the link now points at this app's tap")
        XCTAssertTrue(FileManager.default.fileExists(atPath: copy.path), "only the link was replaced, never what it pointed at")

        // Cancel installs nothing.
        try FileManager.default.removeItem(at: linkDirectory.appendingPathComponent("tap"))
        await pane.refresh()
        pane.installButton.performClick(nil)
        let second = try await confirmSheet()
        second.cancelButton.performClick(nil)
        try await waitUntil(timeout: 5, "the sheet to close") { self.settings.window?.attachedSheet == nil }
        XCTAssertFalse(FileManager.default.fileExists(atPath: linkDirectory.appendingPathComponent("tap").path))

        // A file that is not ours: frame c of the board. No sheet, the button is off, the pane says why, and the file is never touched.
        let foreign = linkDirectory.appendingPathComponent("tap")
        try "#!/bin/sh\necho someone else's\n".write(to: foreign, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: foreign.path)
        await pane.refresh()
        XCTAssertFalse(pane.installButton.isEnabled, "refused before any click")
        XCTAssertEqual(pane.installHint.stringValue, "\(shownLinkDirectory)/tap is a tap that Tap did not install. Tap never replaces or deletes it.")
        XCTAssertEqual(pane.otherLabel.stringValue, "tap (unknown version)", "the foreign file is the tap on PATH now")
        XCTAssertEqual(pane.otherNoteLabel.stringValue, "Tap did not install it. Tap never replaces or deletes it.")
        XCTAssertEqual(sheetsBegun { pane.installButton.performClick(nil); pane.installPressed(pane.installButton) }, 0, "no sheet for a refused install, clicked or sent")
        XCTAssertNil(settings.window?.attachedSheet)
        XCTAssertThrowsError(try AppEnvironment.shared.commandLineInstaller.install())
        XCTAssertEqual(try String(contentsOf: linkDirectory.appendingPathComponent("tap"), encoding: .utf8), "#!/bin/sh\necho someone else's\n", "untouched")

        // Not on PATH: nothing is in the way, but the state is not ready, so Install does nothing.
        try FileManager.default.removeItem(at: linkDirectory.appendingPathComponent("tap"))
        AppEnvironment.shared.extraEnvironment["PATH"] = "\(pathDirectory.path):/usr/bin:/bin"
        await pane.refresh()
        XCTAssertEqual(pane.installState, .notOnPath)
        XCTAssertFalse(pane.installButton.isEnabled)
        XCTAssertEqual(sheetsBegun { pane.installButton.performClick(nil); pane.installPressed(pane.installButton) }, 0, "no sheet where the state is not ready")
        XCTAssertNil(settings.window?.attachedSheet)
        XCTAssertFalse(FileManager.default.fileExists(atPath: linkDirectory.appendingPathComponent("tap").path), "no link")
    }

    func testAnotherTapIsAlreadyInstalled() async throws {
        let other = try installOtherTap(version: "2.0.0")
        let pane = try await showPane()
        try await waitUntil(timeout: 10, "the other tap's version") { pane.otherLabel.stringValue == "tap 2.0.0" }
        XCTAssertEqual(pane.otherPathLabel.stringValue, other.path)
        XCTAssertEqual(pane.otherNoteLabel.stringValue, "Installed by Homebrew. Tap never replaces or deletes it.", "the SettingsCLI board's note for another tap on PATH")
        XCTAssertEqual(pane.installHint.stringValue, "Asks first. \(shownLinkDirectory) comes before \(pathDirectory.path) on your PATH.")
        XCTAssertTrue(pane.installButton.isEnabled)
        // Frame b of the InstallConfirm board: the other tap named in the sheet, and left exactly as it was.
        pane.installButton.performClick(nil)
        let sheet = try await confirmSheet()
        XCTAssertTrue(sheet.bodyLabel.stringValue.hasSuffix(", which comes before \(pathDirectory.path) on your PATH, so Terminal will run tap \(AppEnvironment.shared.bundledTapVersion ?? "")."), sheet.bodyLabel.stringValue)
        XCTAssertFalse(sheet.otherTapCard.isHidden)
        XCTAssertEqual(sheet.otherTapLabel.stringValue, "tap 2.0.0")
        XCTAssertEqual(sheet.otherTapNote.stringValue, "Stays exactly as it is. Tap never replaces or deletes it.")
        XCTAssertEqual(sheet.otherTapPath.stringValue, other.path)
        sheet.installButton.performClick(nil)
        try await waitUntil(timeout: 10, "the link") { FileManager.default.fileExists(atPath: self.linkDirectory.appendingPathComponent("tap").path) }
        XCTAssertEqual(try String(contentsOf: other, encoding: .utf8), "#!/bin/sh\necho 'tap version 2.0.0'\n")
        // With the other tap first on PATH, Install is off: the spec allows it only where ~/.local/bin comes first.
        try FileManager.default.removeItem(at: linkDirectory.appendingPathComponent("tap"))
        AppEnvironment.shared.extraEnvironment["PATH"] = "\(pathDirectory.path):\(linkDirectory.path):/usr/bin:/bin"
        await pane.refresh()
        XCTAssertFalse(pane.installButton.isEnabled)
        XCTAssertEqual(pane.installHint.stringValue, "\(pathDirectory.path) comes before \(shownLinkDirectory) on your PATH, so Terminal would still run tap 2.0.0.")
        // Not on PATH at all: off too, with the way to fix it.
        AppEnvironment.shared.extraEnvironment["PATH"] = "\(pathDirectory.path):/usr/bin:/bin"
        await pane.refresh()
        XCTAssertFalse(pane.installButton.isEnabled)
        XCTAssertEqual(pane.installHint.stringValue, "\(shownLinkDirectory) is not on your PATH: add it in your shell's profile first.")
    }

    func testTheMenuItemOpensThePane() throws {
        let tapMenu = try XCTUnwrap(NSApp.mainMenu?.items.first { $0.title == "Tap" }?.submenu)
        let item = try XCTUnwrap(tapMenu.items.first { $0.title == "Install Command Line Tool…" })
        XCTAssertEqual(item.action, #selector(AppDelegate.installCommandLineTool(_:)))
        (NSApp.delegate as! AppDelegate).installCommandLineTool(nil)
        XCTAssertEqual(settings.tabViewController.selectedTabViewItemIndex, SettingsWindowController.Pane.commandLine.rawValue)
    }
}
