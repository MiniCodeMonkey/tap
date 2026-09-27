import XCTest
@testable import Tap

final class ThemeSetTests: HostedTestCase {
    /// themed.md: ops.md's slides with `theme: terminal` (ops.md names none).
    func openThemed() async throws -> (DeckDocument, DeckSessionController, URL) {
        let deck = try Fixtures.copyDeck("themed.md")
        let document = try await openDeckAndWaitForPreview(deck)
        return (document, try XCTUnwrap(document.sessionController), deck)
    }

    /// The tool path itself: save first, tap on the file, the file back as one undo step.
    func testSetThemeRunsTapOnTheSavedDeck() async throws {
        let (document, controller, deck) = try await openThemed()
        XCTAssertEqual(controller.currentThemeSlug, "terminal", "themed.md names its theme")
        controller.jumpToSlide(number: 2)
        controller.editor.insertText("typed before the pick ", replacementRange: controller.editor.selectedRange())
        XCTAssertTrue(document.isDocumentEdited)
        var themes: [String?] = []
        controller.onThemeChanged = { themes.append($0) }

        controller.setTheme("blueprint")
        try await waitUntil(timeout: 20, "tap theme set to land") { controller.editor.string.contains("theme: blueprint") }
        let onDisk = try String(contentsOf: deck, encoding: .utf8)
        XCTAssertTrue(onDisk.contains("theme: blueprint"), "tap wrote the file")
        XCTAssertTrue(onDisk.contains("typed before the pick"), "the buffer was saved first")
        XCTAssertFalse(document.isDocumentEdited, "the buffer equals the file after the load")
        XCTAssertEqual(controller.currentSlideNumber, 2, "the cursor stays on its slide")
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Change Theme", "one undo step, named")
        XCTAssertEqual(themes, ["blueprint"], "the change is reported once, for the toolbar item")

        controller.editor.undoManager?.undo()
        XCTAssertFalse(controller.editor.string.contains("theme: blueprint"), "undo returns to the text before tap's edit")
        XCTAssertTrue(controller.editor.string.contains("typed before the pick"))
        XCTAssertEqual(themes, ["blueprint", "terminal"], "and reports the way back")

        // The Default cell: tap theme set default removes the line.
        controller.setTheme("default")
        try await waitUntil(timeout: 20, "the theme line to go") { !controller.editor.string.contains("theme:") }
        XCTAssertNil(controller.currentThemeSlug)
        XCTAssertEqual(themes, ["blueprint", "terminal", nil], "reported as no theme")
        XCTAssertFalse(try String(contentsOf: deck, encoding: .utf8).contains("theme:"))
    }

    func testThemeSetIsRefusedWhileADiskConflictShows() async throws {
        let (_, controller, deck) = try await openThemed()
        controller.editor.insertText("mine ", replacementRange: NSRange(location: controller.editor.hiddenLength, length: 0))
        try (try String(contentsOf: deck, encoding: .utf8) + "\n# Theirs\n").write(to: deck, atomically: true, encoding: .utf8)
        try await waitUntil(timeout: 10, "the conflict bar") { controller.editorViewController.bar(.changedOnDisk) != nil }
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.themeSetFailing(recordingTo: record)

        controller.setTheme("blueprint")
        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertFalse(FileManager.default.fileExists(atPath: record.path), "tap was not run: the save was refused")
        let text = controller.editor.string as NSString
        XCTAssertEqual(text.substring(with: NSRange(location: controller.editor.hiddenLength, length: 5)), "mine ", "the edit is kept")
        XCTAssertNotNil(controller.editorViewController.bar(.changedOnDisk))
        XCTAssertTrue(controller.session.log.text.contains("Change Theme was not run: the save was refused"))
    }

    func testTapsErrorShowsOnTheBar() async throws {
        let (_, controller, _) = try await openThemed()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.themeSetFailing(recordingTo: record)
        let textBefore = controller.editor.string

        controller.setTheme("nope")
        try await waitUntil(timeout: 10, "the bar") { controller.editorViewController.bar(.toolFailed) != nil }
        let bar = try XCTUnwrap(controller.editorViewController.bar(.toolFailed))
        XCTAssertEqual(bar.message, "Change Theme failed.")
        XCTAssertTrue(bar.detail.hasPrefix("unknown theme \"nope\""), "tap's own message: \(bar.detail)")
        XCTAssertEqual(controller.editor.string, textBefore, "nothing changed")
        bar.button(titled: "OK")?.performClick(nil)
        XCTAssertNil(controller.editorViewController.bar(.toolFailed))
    }
}
