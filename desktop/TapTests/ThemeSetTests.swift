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
        XCTAssertTrue(controller.session.log.text.contains("Change Theme was not run: the save was refused (the deck changed on disk)"),
                      "refused by the conflict guard itself, ahead of the save")
    }

    /// Opens themed.md with autosave held off, and a `tap theme set` that
    /// waits `before` seconds, writes the file, and waits three seconds more before it exits.
    func openThemedWithASlowThemeSet(before: Double = 0) async throws -> (DeckSessionController, URL, URL) {
        let savedDelay = NSDocumentController.shared.autosavingDelay
        NSDocumentController.shared.autosavingDelay = 120
        addTeardownBlock { @MainActor in NSDocumentController.shared.autosavingDelay = savedDelay }
        let (_, controller, deck) = try await openThemed()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.slowThemeSet(before: before, after: 3, recordingTo: record)
        return (controller, deck, record)
    }

    func waitForTheRunToEnd(_ record: URL) async throws {
        try await waitUntil(timeout: 30, "tap theme set to exit") { ((try? String(contentsOf: record, encoding: .utf8)) ?? "").contains("finished") }
        // The exit reaches the app on the main queue after the process ends.
        try await Task.sleep(nanoseconds: 700_000_000)
    }

    /// Typing that lands after tap wrote the file and before the run ends
    /// stays in the buffer, and is never part of the Change Theme step.
    func testTypingWhileTapRunsIsKept() async throws {
        let (controller, deck, record) = try await openThemedWithASlowThemeSet()
        controller.setTheme("blueprint")
        // tap dev reports the write while the run still waits: the unedited buffer loads it.
        try await waitUntil(timeout: 20, "tap dev's report of tap's write") { controller.editor.string.contains("theme: blueprint") }
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Change Theme")
        controller.editor.insertText("typed while tap ran ", replacementRange: NSRange(location: controller.editor.hiddenLength, length: 0))
        try await waitForTheRunToEnd(record)
        XCTAssertTrue(controller.editor.string.contains("typed while tap ran"), "the run's end loads nothing over the typing")
        XCTAssertTrue(controller.editor.string.contains("theme: blueprint"))
        XCTAssertNil(controller.editorViewController.bar(.changedOnDisk), "the disk is the text the buffer already holds, with the typing on top")
        XCTAssertFalse(try String(contentsOf: deck, encoding: .utf8).contains("typed while tap ran"), "not saved yet")
        controller.editor.undoManager?.undo()
        XCTAssertFalse(controller.editor.string.contains("typed while tap ran"), "the typing is its own undo step")
        XCTAssertTrue(controller.editor.string.contains("theme: blueprint"), "and Change Theme is still done")
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Change Theme")
    }

    /// A conflict bar that went up while tap ran stays up when the run ends:
    /// the person still chooses, and Load Disk Version is the Change Theme step.
    func testAConflictDuringTheRunStillAsks() async throws {
        let (controller, deck, record) = try await openThemedWithASlowThemeSet()
        controller.setTheme("blueprint")
        try await waitUntil(timeout: 20, "tap dev's report of tap's write") { controller.editor.string.contains("theme: blueprint") }
        controller.editor.insertText("mine ", replacementRange: NSRange(location: controller.editor.hiddenLength, length: 0))
        try (try String(contentsOf: deck, encoding: .utf8) + "\n# Theirs\n").write(to: deck, atomically: true, encoding: .utf8)
        try await waitUntil(timeout: 10, "the conflict bar") { controller.editorViewController.bar(.changedOnDisk) != nil }
        try await waitForTheRunToEnd(record)
        let bar = try XCTUnwrap(controller.editorViewController.bar(.changedOnDisk), "the run's end leaves the choice to the person")
        XCTAssertNotNil(bar.button(titled: "Keep Mine"))
        XCTAssertTrue(controller.editor.string.contains("mine "), "the edit is kept")
        XCTAssertFalse(controller.editor.string.contains("# Theirs"))
        try XCTUnwrap(bar.button(titled: "Load Disk Version")).performClick(nil)
        XCTAssertTrue(controller.editor.string.contains("# Theirs"))
        XCTAssertNil(controller.editorViewController.bar(.changedOnDisk))
    }

    /// Typing between the save and tap's write: tap's write meets an
    /// edited buffer, the bar goes up and stays past the run's end, and
    /// Load Disk Version carries the action's name.
    func testTypingBeforeTapWritesGetsTheBar() async throws {
        let (controller, deck, record) = try await openThemedWithASlowThemeSet(before: 1.5)
        controller.editor.insertText("mine ", replacementRange: NSRange(location: controller.editor.hiddenLength, length: 0))
        controller.setTheme("blueprint")
        try await waitUntil(timeout: 20, "the save and tap's start") { ((try? String(contentsOf: record, encoding: .utf8)) ?? "").contains("arguments: theme set") }
        XCTAssertTrue(try String(contentsOf: deck, encoding: .utf8).contains("mine "), "saved before tap ran")
        controller.editor.insertText("and more ", replacementRange: NSRange(location: controller.editor.hiddenLength, length: 0))
        try await waitForTheRunToEnd(record)
        let bar = try XCTUnwrap(controller.editorViewController.bar(.changedOnDisk), "an edited buffer is not loaded over")
        XCTAssertTrue(controller.editor.string.contains("and more "))
        try XCTUnwrap(bar.button(titled: "Load Disk Version")).performClick(nil)
        XCTAssertTrue(controller.editor.string.contains("theme: blueprint"))
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Change Theme", "the load is the action's step")
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
