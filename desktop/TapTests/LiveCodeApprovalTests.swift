import XCTest
@testable import Tap

/// tap dev asks about live code when the deck opens; the app shows the
/// question as a sheet on the deck window and sends the answer back. The
/// real bundled tap runs here, on a copy of a fixture, with the test's own
/// settings folder; two tests run a scripted tap to see a restart.
final class LiveCodeApprovalTests: HostedTestCase {
    func testADeckWithoutLiveCode() async throws {
        approvesLiveCodeOnOpen = false
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("ops.md"))
        let controller = try XCTUnwrap(document.sessionController)
        // The question, when there is one, comes within milliseconds of ready; a second is plenty.
        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertNil(controller.pendingQuestion, "nothing asks for approval")
        let deckWindow = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        XCTAssertNil(deckWindow.questionSheet)
        XCTAssertNil(deckWindow.window?.attachedSheet)
        XCTAssertTrue(deckWindow.deckQuestions.isEmpty)
        XCTAssertFalse(controller.session.log.text.contains("approval question"))
    }

    func testFirstOpenOfADeckWithLiveCodeInTheApp() async throws {
        let (document, controller, deckWindow, sheet) = try await openUnapprovedAndWaitForTheQuestion("live-code.md")
        let deck = try XCTUnwrap(document.fileURL)
        XCTAssertTrue(controller.session.log.text.contains("tap asks a approval question"), "tap reports that the deck needs approval")
        XCTAssertTrue(deckWindow.window?.attachedSheet === sheet, "a sheet on the deck window")
        XCTAssertEqual(deckWindow.questionSheetSource, .deck)
        XCTAssertEqual(sheet.titleLabel.stringValue, "This deck can run code on your Mac")
        XCTAssertEqual(sheet.summaryLabel.stringValue, "2 shell, 1 sqlite")
        XCTAssertEqual(sheet.pathLabel.stringValue, Fixtures.realPath(of: deck), "tap names the deck by its resolved path")
        XCTAssertEqual(sheet.blockRows.map(\.placeLabel.stringValue), ["Slide 2, block 1", "Slide 5, block 1", "Slide 4, block 1"])
        XCTAssertEqual(sheet.blockRows.map(\.codeLabel.stringValue), ["echo hello from slide 2", "echo five", "SELECT 1 AS one, 'two' AS two;"],
                       "each block's code is there to read, under its driver")
        XCTAssertEqual(sheet.declineButton.title, "Don't Allow")
        XCTAssertEqual(sheet.acceptButton.title, "Allow")
        XCTAssertEqual(controller.pendingQuestion?.payload.drivers?.map(\.name), ["shell", "sqlite"])
        XCTAssertNil(controller.pendingQuestion?.payload.approvedBefore, "the first time")
        XCTAssertEqual(controller.pendingQuestion?.payload.blocks?.count, 3)
    }

    func testAllow() async throws {
        let (document, controller, deckWindow, sheet) = try await openUnapprovedAndWaitForTheQuestion("live-code.md")
        let deck = try XCTUnwrap(document.fileURL)
        try await waitForPreview(document, slide: 1)
        try XCTUnwrap(sheet.button(titled: "Allow")).performClick(nil)
        XCTAssertNil(deckWindow.questionSheet)
        XCTAssertNil(deckWindow.window?.attachedSheet)
        XCTAssertNil(controller.pendingQuestion)
        try await waitUntil(timeout: 10, "tap to store the approval") {
            let stored = self.storedApprovals()
            return stored.contains("deck: \(Fixtures.realPath(of: deck))") && stored.contains("drivers: [shell, sqlite]")
        }
        // tap reloads the page once its policy is set (hub.BroadcastReload); the slide's block then offers Run.
        try await waitForRunButtons(#"["Run"]"#, in: controller, document: document, slide: 2)
        XCTAssertTrue(controller.session.log.text.contains("answered the approval question: allow"))
    }

    /// "Don't allow", as check-scenarios.sh spells the scenario's name.
    func testDonTAllow() async throws {
        let (document, controller, deckWindow, sheet) = try await openUnapprovedAndWaitForTheQuestion("live-code.md")
        let deck = try XCTUnwrap(document.fileURL)
        try XCTUnwrap(sheet.button(titled: "Don't Allow")).performClick(nil)
        XCTAssertNil(deckWindow.questionSheet)
        XCTAssertNil(controller.pendingQuestion)
        // The deck opens and previews normally.
        try await waitForPreview(document, slide: 1)
        try await waitForBoxes(document, count: 5)
        XCTAssertTrue(controller.presentation.canStart, "and presents normally")
        try await waitUntil(timeout: 10, "tap's own words on stderr") {
            controller.session.log.text.contains("Live code is off for shell and sqlite in this run. tap asks again next time.")
        }
        try await waitForRunButtons(#"["Not approved"]"#, in: controller, document: document, slide: 2)
        XCTAssertFalse(storedApprovals().contains("approvals"), "nothing is stored for a no")

        // tap asks again the next time the deck opens.
        document.close()
        try await waitUntil(timeout: 10, "the deck to close") { NSDocumentController.shared.documents.isEmpty }
        let reopened = try await openDeck(deck)
        let again = try XCTUnwrap(reopened.sessionController)
        try await waitUntil(timeout: 30, "the question again") { again.pendingQuestion?.kind == "approval" }
    }

    func testClosingTheDeckWithTheSheetUpAsksAgainNextTime() async throws {
        let (document, controller, _, _) = try await openUnapprovedAndWaitForTheQuestion("live-code.md")
        let deck = try XCTUnwrap(document.fileURL)
        let pid = try XCTUnwrap(controller.session.processIdentifier)
        document.close()
        try await waitUntil(timeout: 10, "tap to exit with its stdin") { kill(pid, 0) != 0 }
        XCTAssertFalse(storedApprovals().contains("approvals"), "no answer was sent")
        let reopened = try await openDeck(deck)
        let again = try XCTUnwrap(reopened.sessionController)
        try await waitUntil(timeout: 30, "the question again") { again.pendingQuestion?.kind == "approval" }
    }

    /// A deck opened by every other test is approved ahead of time, the way
    /// tap new approves a deck the person made, so no sheet sits over the window.
    func testAPreApprovedDeckAsksNothing() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("live-code.md"))
        let controller = try XCTUnwrap(document.sessionController)
        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertNil(controller.pendingQuestion)
        XCTAssertTrue(storedApprovals().contains("drivers: [shell, sqlite]"), "the test wrote tap's record itself")
        // The page read itself, not a description of WebKit's optional: slide 1 has no block.
        try await waitForPreview(document, slide: 1)
        let none = await controller.previewViewController.runButtonLabels()
        XCTAssertEqual(none, "[]")
        try await waitForRunButtons(#"["Run"]"#, in: controller, document: document, slide: 4)
    }

    /// The deck's question moves no window and takes no tab: a question for
    /// a deck behind another deck's tab waits until its tab is chosen, and
    /// the front deck keeps its tab.
    func testADeckQuestionInABackgroundTabTakesNothing() async throws {
        approvesLiveCodeOnOpen = false
        let behind = try await openDeck(try Fixtures.copyDeck("live-code.md"))
        let behindController = try XCTUnwrap(behind.sessionController)
        let behindWindow = try XCTUnwrap(behind.windowControllers.first as? DeckWindowController)
        try await waitUntil(timeout: 30, "the first deck's question") { behindController.pendingQuestion?.kind == "approval" }
        // Its sheet is up; a second deck opens as a tab in front and takes the tab.
        approvesLiveCodeOnOpen = true
        let front = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("ops.md"))
        let frontWindow = try XCTUnwrap(front.windowControllers.first?.window)
        XCTAssertTrue(frontWindow.tabGroup?.selectedWindow === frontWindow, "the deck just opened has the tab")
        // The first deck's tap asks again (a crash and a restart); the new question queues, and nothing changes the tab.
        let pid = try XCTUnwrap(behindController.session.processIdentifier)
        kill(pid, SIGKILL)
        try await waitUntil(timeout: 30, "the restarted tap's question") {
            behindController.pendingQuestion?.kind == "approval" && behindController.session.processIdentifier != nil && behindController.session.processIdentifier != pid
        }
        try await Task.sleep(nanoseconds: UInt64(DeckWindowController.spaceSwitchSettleDelay * 2 * 1_000_000_000))
        XCTAssertTrue(frontWindow.tabGroup?.selectedWindow === frontWindow, "still the front deck's tab")
        XCTAssertNil(behindWindow.questionSheet, "no sheet on a tab the person is not looking at")
        XCTAssertNil(behindWindow.window?.attachedSheet)
        XCTAssertEqual(behindWindow.deckQuestions.count, 1, "the question waits for its tab")
        // The person chooses the tab: the question shows there.
        let behindNSWindow = try XCTUnwrap(behindWindow.window)
        behindNSWindow.tabGroup?.selectedWindow = behindNSWindow
        try await waitUntil(timeout: 5, "the question once its tab is chosen") { behindWindow.questionSheet is ApprovalSheet }
        XCTAssertTrue(behindWindow.deckQuestions.isEmpty)
        XCTAssertEqual(behindWindow.questionSheetSource, .deck)
    }

    /// The test hook answers Allow for a deck a test approved ahead of time,
    /// through the path a click takes: no sheet ever shows, and tap writes
    /// its own full record of the custom driver, its digest included.
    func testAPreApprovedCustomDriverAsksNothing() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("custom-driver.md"))
        let controller = try XCTUnwrap(document.sessionController)
        let deckWindow = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        var sheetsSeen = 0
        let deadline = Date().addingTimeInterval(10)
        while Date() < deadline, !storedApprovals().contains("commandDigests:") {
            if deckWindow.questionSheet != nil || deckWindow.window?.attachedSheet != nil { sheetsSeen += 1 }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
        XCTAssertEqual(sheetsSeen, 0, "no sheet ever showed")
        let stored = storedApprovals()
        XCTAssertTrue(stored.contains("commandDigests:"), "tap stored the command's digest: \(stored)")
        XCTAssertTrue(stored.contains("fortune:") && stored.contains("- /bin/cat"), "and the command as written: \(stored)")
        XCTAssertTrue(controller.session.log.text.contains("the approval question was approved ahead of time by the test"))
        XCTAssertTrue(controller.session.log.text.contains("answered the approval question: allow"), "the click's own path and log")
        XCTAssertNil(controller.pendingQuestion)
        try await waitForRunButtons(#"["Run"]"#, in: controller, document: document, slide: 3)
    }

    /// The hook leaves every other deck to the sheet: one the test did not
    /// approve, and one approved for other drivers than tap asks about.
    func testTheTestHookAnswersOnlyForPreApprovedDecksAndDrivers() async throws {
        XCTAssertNotNil(AppEnvironment.shared.approvalAnswerForTests, "set for every hosted test")
        let listed = try Fixtures.copyDeck("custom-driver.md")
        try approveLiveCode(for: listed, drivers: ["sqlite"])
        let deck = Fixtures.realPath(of: listed)
        let hook = try XCTUnwrap(AppEnvironment.shared.approvalAnswerForTests)
        XCTAssertNil(hook(QuestionPayload(deck: deck, drivers: [ApprovalDriver(name: "fortune")])), "a driver the test did not approve")
        XCTAssertNil(hook(QuestionPayload(deck: deck, drivers: [ApprovalDriver(name: "sqlite"), ApprovalDriver(name: "fortune")])))
        XCTAssertEqual(hook(QuestionPayload(deck: deck, drivers: [ApprovalDriver(name: "sqlite")])), true)
        XCTAssertNil(hook(QuestionPayload(deck: "/private/tmp/elsewhere.md", drivers: [ApprovalDriver(name: "sqlite")])), "another deck")
        XCTAssertNil(hook(QuestionPayload(deck: deck, drivers: [])), "a question with no drivers")
        XCTAssertNil(hook(QuestionPayload(drivers: [ApprovalDriver(name: "sqlite")])), "a question with no deck")

        // A deck opened unapproved still shows its sheet with the hook set.
        let (_, _, deckWindow, sheet) = try await openUnapprovedAndWaitForTheQuestion("custom-driver.md")
        XCTAssertTrue(deckWindow.window?.attachedSheet === sheet)
        try XCTUnwrap(sheet.button(titled: "Don't Allow")).performClick(nil)
    }

    /// tap withdraws a question a reload made stale (question-closed) and
    /// asks a new one; the stale sheet goes, the new one shows, and nothing
    /// is ever sent for the withdrawn id.
    func testAWithdrawnQuestionsSheetGoes() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        let bundled = AppEnvironment.shared.tapExecutableURL
        AppEnvironment.shared.tapExecutableURL = try FakeTapScripts.askingApproval(recordingTo: record, withdrawingAfter: 1.5)
        defer { AppEnvironment.shared.tapExecutableURL = bundled }
        approvesLiveCodeOnOpen = false
        let document = try await openDeck(try Fixtures.copyDeck("live-code.md"))
        let controller = try XCTUnwrap(document.sessionController)
        let deckWindow = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        try await waitUntil(timeout: 30, "q1's sheet") { deckWindow.questionSheetQuestionID == "q1" }
        let first = try XCTUnwrap(deckWindow.questionSheet as? ApprovalSheet)
        XCTAssertEqual(first.summaryLabel.stringValue, "1 shell")
        // The fake withdraws q1 and asks q2 about sqlite.
        try await waitUntil(timeout: 10, "q2's sheet in place of q1's") { deckWindow.questionSheetQuestionID == "q2" }
        XCTAssertFalse(deckWindow.questionSheet === first, "q1's sheet is gone")
        XCTAssertNil(first.sheetParent)
        XCTAssertEqual((deckWindow.questionSheet as? ApprovalSheet)?.summaryLabel.stringValue, "1 sqlite")
        XCTAssertEqual(controller.pendingQuestions.map(\.id), ["q2"])
        XCTAssertTrue(deckWindow.deckQuestions.isEmpty)
        try XCTUnwrap(deckWindow.questionSheet?.button(titled: "Allow")).performClick(nil)
        try await waitUntil(timeout: 5, "q2's answer") { (try? String(contentsOf: record, encoding: .utf8))?.contains(#""id":"q2","value":true"#) == true }
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertFalse(recorded.contains(#""id":"q1""#), "nothing was ever sent for the withdrawn question: \(recorded)")
        XCTAssertTrue(controller.session.log.text.contains("tap withdrew the q1 question"))
    }

    /// tap's ids start at q1 in every process; an answer for the old
    /// process's q1 must never reach the new one. Checked straight on the
    /// controller, with a scripted tap that records its stdin.
    func testAnAnswerForTheOldProcessNeverReachesTheNewOne() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        let bundled = AppEnvironment.shared.tapExecutableURL
        AppEnvironment.shared.tapExecutableURL = try FakeTapScripts.askingApproval(recordingTo: record)
        defer { AppEnvironment.shared.tapExecutableURL = bundled }
        approvesLiveCodeOnOpen = false
        let document = try await openDeck(try Fixtures.copyDeck("live-code.md"))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 30, "the first process's question") { controller.pendingQuestion?.id == "q1" }
        let oldGeneration = controller.questionGeneration
        let firstPid = try XCTUnwrap(controller.session.processIdentifier)
        kill(firstPid, SIGKILL)
        try await waitUntil(timeout: 10, "the question gone with its process") { controller.pendingQuestion == nil }
        try await waitUntil(timeout: 30, "the new process's question, q1 again") {
            controller.pendingQuestion?.id == "q1" && controller.session.processIdentifier != nil && controller.session.processIdentifier != firstPid
        }
        XCTAssertGreaterThan(controller.questionGeneration, oldGeneration)
        controller.answer(id: "q1", value: true, generation: oldGeneration)
        try await Task.sleep(nanoseconds: 1_000_000_000)
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertFalse(recorded.contains(#""type":"answer""#), "the old generation's answer reaches nothing: \(recorded)")
        XCTAssertEqual(controller.pendingQuestion?.id, "q1", "the new question still waits")
        controller.answer(id: "q1", value: false, generation: controller.questionGeneration)
        try await waitUntil(timeout: 5, "the current generation's answer") {
            (try? String(contentsOf: record, encoding: .utf8))?.contains(#"stdin: {"type":"answer","id":"q1","value":false}"#) == true
        }
    }
}
