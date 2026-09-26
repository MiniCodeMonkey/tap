import XCTest
@testable import Tap

/// tap asks again when a reload brings a driver it has not approved, by
/// name or by command; the app shows the question whenever it comes.
/// These run the real bundled tap and need the re-ask it gained on
/// feat/approval-asks-again: on a tap without it, every test here times
/// out on "tap asking about shell", which is the signal the dependency
/// is missing from the branch.
final class NewDriverTests: HostedTestCase {
    func testANewDriverAsksAgain() async throws {
        let (document, controller, deckWindow, sheet) = try await openUnapprovedAndWaitForTheQuestion("undeclared-driver.md")
        let deck = try XCTUnwrap(document.fileURL)
        XCTAssertEqual(sheet.summaryLabel.stringValue, "1 sqlite")
        try XCTUnwrap(sheet.button(titled: "Allow")).performClick(nil)
        try await waitUntil(timeout: 10, "the approval") { self.storedApprovals().contains("drivers: [sqlite]") }
        try await waitForPreview(document, slide: 1)
        let pid = try XCTUnwrap(controller.session.processIdentifier)

        // A git pull: the file gains shell with no unsaved edits here, so the app loads it silently and tap renders the new text.
        let pulled = try String(contentsOf: deck, encoding: .utf8).replacingOccurrences(of: "  sqlite: {}\n", with: "  sqlite: {}\n  shell: {}\n")
        try pulled.write(to: deck, atomically: true, encoding: .utf8)
        try await waitUntil(timeout: 15, "the disk version loaded") { controller.editor.string.contains("  shell: {}") }
        XCTAssertFalse(controller.isContentEdited, "loaded, not edited")
        try await waitUntil(timeout: 30, "tap asking about shell") { controller.pendingQuestion?.payload.drivers?.map(\.name) == ["shell"] }
        XCTAssertEqual(controller.session.processIdentifier, pid, "the same tap asks again: nothing restarted")
        XCTAssertEqual(controller.pendingQuestion?.payload.approvedBefore, ["sqlite"])
        try await waitUntil(timeout: 5, "the sheet") { deckWindow.questionSheet is ApprovalSheet }
        let again = try XCTUnwrap(deckWindow.questionSheet as? ApprovalSheet)
        XCTAssertEqual(again.titleLabel.stringValue, "This deck now also wants to run shell")
        XCTAssertEqual(deckWindow.questionSheetSource, .deck)
        try XCTUnwrap(again.button(titled: "Allow shell")).performClick(nil)
        try await waitUntil(timeout: 10, "both drivers stored") { self.storedApprovals().contains("drivers: [shell, sqlite]") }
        try await waitForRunButtons(#"["Run"]"#, in: controller, document: document, slide: 6)

        // Edits to an existing sqlite block never ask again: a code edit, saved, asks nothing.
        let editor = controller.editor
        let query = (editor.string as NSString).range(of: "SELECT 1 AS one;")
        editor.setSelectedRange(NSRange(location: NSMaxRange(query), length: 0))
        editor.insertText(" -- edited", replacementRange: NSRange(location: NSNotFound, length: 0))
        controller.saveNow()
        try await waitUntil(timeout: 10, "the save") { (try? String(contentsOf: deck, encoding: .utf8))?.contains("-- edited") == true }
        try await Task.sleep(nanoseconds: 3_000_000_000)
        XCTAssertNil(controller.pendingQuestion, "no question for a code edit")
        XCTAssertTrue(deckWindow.deckQuestions.isEmpty)
    }

    func testTheFixItAsksAboutTheNewDriver() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("undeclared-driver.md"))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 6)
        try await waitUntil(timeout: 10, "tap's problem") { controller.editor.boxes[5].slide.codeBlocks.first?.problem != nil }
        controller.allowDriver("shell")
        try await waitUntil(timeout: 30, "tap asking about shell from its render of the edit") { controller.pendingQuestion?.payload.drivers?.map(\.name) == ["shell"] }
        XCTAssertEqual(controller.pendingQuestion?.payload.approvedBefore, ["sqlite"], "the test's own approval of the fixture")
        let deckWindow = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        try await waitUntil(timeout: 5, "the sheet") { deckWindow.questionSheet is ApprovalSheet }
        try XCTUnwrap(deckWindow.questionSheet?.button(titled: "Allow shell")).performClick(nil)
        try await waitUntil(timeout: 10, "stored") { self.storedApprovals().contains("drivers: [shell, sqlite]") }
        try await waitForRunButtons(#"["Run"]"#, in: controller, document: document, slide: 6)
    }

    /// A custom driver's approval covers its command: a changed command
    /// asks again, tap names the command it replaces, and the sheet shows
    /// what would run now.
    func testAChangedCustomCommandAsksAgain() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("custom-driver.md"))
        let controller = try XCTUnwrap(document.sessionController)
        let editor = controller.editor
        try await waitForBoxes(document, count: 4)
        let deck = try XCTUnwrap(document.fileURL)
        // The frontmatter is hidden; the edit goes through the editor's programmatic path, as the Deck tab's does.
        let range = (editor.string as NSString).range(of: "command: /bin/cat")
        editor.replaceText(in: range, with: "command: /usr/bin/true", actionName: "Change Command")
        controller.saveNow()
        try await waitUntil(timeout: 10, "the save") { (try? String(contentsOf: deck, encoding: .utf8))?.contains("/usr/bin/true") == true }
        // Not the name alone: the open-time question about fortune would match that; the command is what changed.
        try await waitUntil(timeout: 30, "tap asking about fortune's new command") { controller.pendingQuestion?.payload.drivers?.first?.command == "/usr/bin/true" }
        XCTAssertEqual(controller.pendingQuestion?.payload.drivers?.map(\.name), ["fortune"])
        XCTAssertEqual(controller.pendingQuestion?.payload.drivers?.first?.previousCommand, "/bin/cat", "tap names the command the approval covered")
        XCTAssertEqual(controller.pendingQuestion?.payload.approvedBefore, ["sqlite"])
        let deckWindow = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        try await waitUntil(timeout: 5, "the sheet") { deckWindow.questionSheet is ApprovalSheet }
        let sheet = try XCTUnwrap(deckWindow.questionSheet as? ApprovalSheet)
        // The ApprovalCommandChanged board: the row carries the badge, and the new command is on the Now line.
        XCTAssertEqual(sheet.wording, .changedCommands)
        XCTAssertEqual(sheet.titleLabel.stringValue, "The command for fortune changed")
        XCTAssertEqual(sheet.driverLabels.map(\.stringValue), ["fortune: 1 block on slide 3, command changed"])
        XCTAssertEqual(sheet.commandChangeLabels.map(\.stringValue), ["Before: /bin/cat", "Now: /usr/bin/true"], "what would run now")
        try XCTUnwrap(deckWindow.questionSheet?.button(titled: "Don't Allow")).performClick(nil)
        try await waitForRunButtons(#"["Not approved"]"#, in: controller, document: document, slide: 3)
    }

    func testATapRestartRenewsTheApprovalQuestion() async throws {
        let (_, controller, deckWindow, sheet) = try await openUnapprovedAndWaitForTheQuestion("live-code.md")
        let firstPid = try XCTUnwrap(controller.session.processIdentifier)
        let firstGeneration = controller.questionGeneration
        // tap dies under the sheet; D2's policy restarts it.
        kill(firstPid, SIGKILL)
        try await waitUntil(timeout: 10, "the dead process's question gone") { controller.pendingQuestion == nil }
        XCTAssertNil(deckWindow.questionSheet, "the sheet went with the process that asked")
        XCTAssertNil(deckWindow.window?.attachedSheet)
        XCTAssertGreaterThan(controller.questionGeneration, firstGeneration)
        try await waitUntil(timeout: 30, "the restarted tap's question") {
            controller.pendingQuestion?.kind == "approval" && controller.session.processIdentifier != nil && controller.session.processIdentifier != firstPid
        }
        try await waitUntil(timeout: 5, "a fresh sheet") { deckWindow.questionSheet is ApprovalSheet && deckWindow.questionSheet !== sheet }
        sheet.acceptButton.performClick(nil)
        XCTAssertFalse(storedApprovals().contains("approvals"), "nothing was granted by the old process's sheet")
        XCTAssertEqual(controller.pendingQuestion?.id, "q1", "ids start again per process")
        try XCTUnwrap(deckWindow.questionSheet?.button(titled: "Don't Allow")).performClick(nil)
    }
}
