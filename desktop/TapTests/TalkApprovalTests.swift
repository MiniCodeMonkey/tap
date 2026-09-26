import XCTest
@testable import Tap

/// tap present asks the same approval question when the deck is still
/// unapproved at Play; the sheet is the same, on the deck window, before
/// any talk window shows. Its answer reaches only the process that asked.
/// And no talk starts while the deck's own question waits, and no deck
/// question shows while a talk runs.
final class TalkApprovalTests: PresentingTestCase {
    static let approvalQuestion = #"{"type":"question","id":"q1","kind":"approval","payload":{"deck":"/private/tmp/t/ops.md","drivers":[{"name":"shell","slides":[2],"blocks":1}],"blocks":[{"driver":"shell","code":"echo hi","slide":2,"block":1}]}}"#

    /// Whether the window server has `sheet` on screen and in front of every
    /// talk window it has on screen. A talk window in a full screen Space
    /// the person is not looking at is not on screen, and covers nothing.
    @MainActor
    static func isInFrontOfTheTalk(_ sheet: NSWindow, _ presentation: PresentationController) -> Bool {
        let frontToBack = onScreenWindowNumbers()
        guard let sheetPlace = frontToBack.firstIndex(of: sheet.windowNumber) else { return false }
        let talkWindows = [presentation.audienceWindow, presentation.presenterWindow].compactMap { $0 }
        return talkWindows.allSatisfy { talkWindow in
            guard let talkPlace = frontToBack.firstIndex(of: talkWindow.windowNumber) else { return true }
            return talkPlace > sheetPlace
        }
    }

    func testAnApprovalDuringATalkIsASheetOnTheDeckWindow() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [Self.approvalQuestion], exitsOnAnswer: false, recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "the talk's approval question") { presentation.pendingQuestion?.kind == "approval" }
        let sheet = try XCTUnwrap(deckWindow.questionSheet as? ApprovalSheet)
        XCTAssertTrue(deckWindow.window?.attachedSheet === sheet)
        XCTAssertEqual(deckWindow.questionSheetSource, .talk)
        XCTAssertEqual(sheet.titleLabel.stringValue, "This deck can run code on your Mac")
        XCTAssertEqual(presentation.state, .starting, "the windows wait for the answer")
        XCTAssertFalse(presentation.windowsShown)
        try XCTUnwrap(sheet.button(titled: "Allow")).performClick(nil)
        try await waitUntil(timeout: 5, "the answer to reach tap present") {
            (try? String(contentsOf: record, encoding: .utf8))?.contains(#"stdin: {"type":"answer","id":"q1","value":true}"#) == true
        }
        XCTAssertNil(deckWindow.questionSheet)
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }
        try await stopPresenting(controller)
    }

    /// tap present dies under its approval sheet; D2's policy restarts it,
    /// and the new process asks q1 again. The old sheet must be gone, its
    /// Allow must reach nothing, and the new question gets a fresh sheet.
    func testATalkRestartDropsItsApprovalSheet() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        let crash = record.appendingPathExtension("crash")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [Self.approvalQuestion], exitsOnAnswer: false, crashFile: crash, recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "the first process's question") { presentation.pendingQuestion?.id == "q1" }
        try await waitUntil(timeout: 5, "its sheet") { deckWindow.questionSheetQuestionID == "q1" }
        let sheet = try XCTUnwrap(deckWindow.questionSheet as? ApprovalSheet)
        let firstPid = try XCTUnwrap(presentation.session?.processIdentifier)
        // The fake dies when told (a crash under the sheet); the session restarts it.
        try "".write(to: crash, atomically: true, encoding: .utf8)
        try await waitUntil(timeout: 10, "the question gone with its process") { presentation.pendingQuestion == nil }
        XCTAssertNil(deckWindow.questionSheet, "the sheet went with the process that asked")
        XCTAssertNil(deckWindow.window?.attachedSheet)
        try await waitUntil(timeout: 30, "the restarted process's question") {
            presentation.pendingQuestion?.id == "q1" && presentation.session?.processIdentifier != nil && presentation.session?.processIdentifier != firstPid
        }
        try await waitUntil(timeout: 5, "a fresh sheet") { deckWindow.questionSheet is ApprovalSheet && deckWindow.questionSheet !== sheet }
        sheet.acceptButton.performClick(nil)
        try await Task.sleep(nanoseconds: 500_000_000)
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertFalse(recorded.contains(#""value":true"#), "the old sheet's Allow reached nothing: \(recorded)")
        try XCTUnwrap(deckWindow.questionSheet?.button(titled: "Don't Allow")).performClick(nil)
        try await waitUntil(timeout: 5, "the current question's answer") { (try? String(contentsOf: record, encoding: .utf8))?.contains(#""value":false"#) == true }
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }
        try await stopPresenting(controller)
    }

    /// tap present is replaced under its approval sheet by the port
    /// fallback (tap said the remembered port is taken; the session is
    /// stopped, not restarted, and a new one starts on no port). The new
    /// process asks q1 again: the old sheet must be gone, and its Allow
    /// must reach nothing.
    func testAPortFallbackDropsTheTalksApprovalSheet() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [Self.approvalQuestion], exitsOnAnswer: false, recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "the first process's question") { presentation.pendingQuestion?.id == "q1" }
        try await waitUntil(timeout: 5, "its sheet") { deckWindow.questionSheetQuestionID == "q1" }
        let sheet = try XCTUnwrap(deckWindow.questionSheet as? ApprovalSheet)
        let firstPid = try XCTUnwrap(presentation.session?.processIdentifier)
        // tap's own words for a taken port, delivered the way the session delivers its events.
        presentation.handle(.error(TapErrorPayload(code: "failed", message: "port 4321 is already in use (another tap dev may be running); pass --port <other>")))
        try await waitUntil(timeout: 10, "the question gone with its process") { presentation.pendingQuestion == nil }
        XCTAssertNil(deckWindow.questionSheet, "the sheet went with the process that asked")
        XCTAssertNil(deckWindow.window?.attachedSheet)
        try await waitUntil(timeout: 30, "the new process's question") {
            presentation.pendingQuestion?.id == "q1" && presentation.session?.processIdentifier != nil && presentation.session?.processIdentifier != firstPid
        }
        try await waitUntil(timeout: 5, "a fresh sheet") { deckWindow.questionSheet is ApprovalSheet && deckWindow.questionSheet !== sheet }
        sheet.acceptButton.performClick(nil)
        try await Task.sleep(nanoseconds: 500_000_000)
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertFalse(recorded.contains(#""value":true"#), "the old sheet's Allow reached nothing: \(recorded)")
        XCTAssertEqual(recorded.components(separatedBy: "arguments:").count - 1, 2, "a second tap present, the fallback's")
        try XCTUnwrap(deckWindow.questionSheet?.button(titled: "Don't Allow")).performClick(nil)
        try await waitUntil(timeout: 5, "the current question's answer") { (try? String(contentsOf: record, encoding: .utf8))?.contains(#""value":false"#) == true }
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }
        try await stopPresenting(controller)
    }

    /// tap present asks a question this version of the app does not know:
    /// it is declined with a log line, and no sheet shows.
    func testAnUnknownTalkQuestionIsDeclined() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        let mystery = #"{"type":"question","id":"q1","kind":"mystery","payload":{"deck":"/private/tmp/t/ops.md"}}"#
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [mystery], exitsOnAnswer: false, recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "the answer to the unknown question") {
            (try? String(contentsOf: record, encoding: .utf8))?.contains(#""id":"q1""#) == true
        }
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertTrue(recorded.contains(#"stdin: {"type":"answer","id":"q1","value":false}"#), recorded)
        XCTAssertFalse(recorded.contains(#""value":true"#), recorded)
        XCTAssertNil(deckWindow.questionSheet)
        XCTAssertTrue(presentation.lastTalkLog?.text.contains("the mystery question is not answered by this version of the app; declined") == true)
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }
        try await stopPresenting(controller)
    }

    /// tap present withdraws a startup question (a reload changed the
    /// deck) and asks another: the first sheet goes, the second shows, and
    /// the windows wait only for the live one.
    func testATalksWithdrawnQuestionGoes() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        let second = #"{"type":"question","id":"q2","kind":"approval","payload":{"deck":"/private/tmp/t/ops.md","drivers":[{"name":"sqlite","slides":[4],"blocks":1}],"blocks":[]}}"#
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(
            events: [Self.approvalQuestion, #"{"type":"question-closed","id":"q1"}"#, second], exitsOnAnswer: false, recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "q2's sheet") { deckWindow.questionSheetQuestionID == "q2" }
        XCTAssertEqual(presentation.pendingQuestions.map(\.id), ["q2"], "q1 was withdrawn before or after its sheet showed; either way it is gone")
        XCTAssertEqual((deckWindow.questionSheet as? ApprovalSheet)?.summaryLabel.stringValue, "1 sqlite")
        // q1's sheet, if it showed, is off the window: q2's is the one attached, not one queued behind it.
        try await waitUntil(timeout: 5, "q2's sheet on the window") {
            deckWindow.questionSheet != nil && deckWindow.window?.attachedSheet === deckWindow.questionSheet
        }
        try XCTUnwrap(deckWindow.questionSheet?.button(titled: "Allow")).performClick(nil)
        try await waitUntil(timeout: 5, "q2's answer") { (try? String(contentsOf: record, encoding: .utf8))?.contains(#""id":"q2","value":true"#) == true }
        XCTAssertFalse(try String(contentsOf: record, encoding: .utf8).contains(#""id":"q1""#))
        try await waitUntil(timeout: 5, "no sheet left on the window") { deckWindow.window?.attachedSheet == nil }
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }
        try await stopPresenting(controller)
    }

    func testATalkAsksAboutAnUnapprovedDeck() async throws {
        // The deck's own question first: declined, so the deck stays unapproved.
        let (document, controller, deckWindow, deckSheet) = try await openUnapprovedAndWaitForTheQuestion("live-code.md")
        try XCTUnwrap(deckSheet.button(titled: "Don't Allow")).performClick(nil)
        try await waitForPreview(document, slide: 1)
        try await waitForBoxes(document, count: 5)
        let screens = oneScreen()
        let presentation = controller.presentation
        presentation.screens = { screens }
        let available = fullScreenAvailable
        presentation.fullScreenAllowed = { available }
        // The real tap present, which asks after the (pre-answered) consent.
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "tap present's approval question") { presentation.pendingQuestion?.kind == "approval" }
        let sheet = try XCTUnwrap(deckWindow.questionSheet as? ApprovalSheet)
        XCTAssertEqual(deckWindow.questionSheetSource, .talk)
        XCTAssertEqual(sheet.summaryLabel.stringValue, "2 shell, 1 sqlite")
        // tap present asks after its ready line, so its pages may already
        // have put the talk's windows up; the sheet's window then comes
        // forward over them (the step-aside path). Either way nothing covers it.
        try await waitUntil(timeout: 5, "the sheet in front of every talk window on screen") {
            Self.isInFrontOfTheTalk(sheet, presentation)
        }
        try XCTUnwrap(sheet.button(titled: "Allow")).performClick(nil)
        try await waitUntil(timeout: 10, "the talk's yes stored") { self.storedApprovals().contains("drivers: [shell, sqlite]") }
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }
        // The deck's own tap dev learns of the yes from its settings file (and the reload the app sends after the Allow):
        // its preview's blocks offer Run without a second sheet.
        do {
            try await waitForRunButtons(#"["Run"]"#, in: controller, document: document, slide: 2, timeout: 30)
        } catch {
            let modified = (try? FileManager.default.attributesOfItem(atPath: settingsFile.path)[.modificationDate]).map { "\($0)" } ?? "none"
            XCTFail("settings.yaml modified \(modified); the talk's log ends: \(presentation.session?.log.text.suffix(800) ?? "no talk session"); "
                    + "tap dev's log ends: \(controller.session.log.text.suffix(1500))")
            throw error
        }
        XCTAssertNil(controller.pendingQuestion, "already approved: tap dev asks nothing on that reload")
        try await stopPresenting(controller)
    }

    func testPlayWaitsForTheDecksApprovalAnswer() async throws {
        let (document, controller, deckWindow, sheet) = try await openUnapprovedAndWaitForTheQuestion("live-code.md")
        try await waitForPreview(document, slide: 1)
        let presentation = controller.presentation
        XCTAssertTrue(presentation.canStart, "the talk itself could start")
        XCTAssertFalse(deckWindow.canStartATalk, "but not while the person has a question to answer")
        XCTAssertFalse(deckWindow.playButton.isEnabled)
        let play = NSMenuItem(title: "Play", action: #selector(DeckWindowController.play(_:)), keyEquivalent: "")
        XCTAssertFalse(deckWindow.validateMenuItem(play))
        deckWindow.play(nil)
        deckWindow.rehearse(nil)
        deckWindow.playButtonClicked(modifiers: [.shift])
        // The Present popover's Start and Rehearse, which can still be open when the sheet attaches.
        deckWindow.startPresenting(PresentationOptions(mode: .play, startSlide: 1), savingSettings: true)
        deckWindow.startPresenting(PresentationOptions(mode: .rehearse, startSlide: 1), savingSettings: true)
        XCTAssertEqual(presentation.state, .idle, "nothing started")
        try XCTUnwrap(sheet.button(titled: "Don't Allow")).performClick(nil)
        XCTAssertTrue(deckWindow.canStartATalk)
        XCTAssertTrue(deckWindow.validateMenuItem(play))
        // AppKit gives the toolbar back its state from before the sheet once the sheet has ended; Play comes on after that.
        try await waitUntil(timeout: 2, "the Play button on again") { deckWindow.playButton.isEnabled }
    }

    /// A deck question that arrives mid-talk (here: tap dev restarts and asks
    /// again) waits, behind the talk's own sheet and then behind the talk,
    /// and shows when the talk ends.
    func testADeckQuestionWaitsForTheTalkToEnd() async throws {
        let (document, controller, deckWindow, deckSheet) = try await openUnapprovedAndWaitForTheQuestion("live-code.md")
        try XCTUnwrap(deckSheet.button(titled: "Don't Allow")).performClick(nil)
        try await waitForPreview(document, slide: 1)
        try await waitForBoxes(document, count: 5)
        let screens = oneScreen()
        let presentation = controller.presentation
        presentation.screens = { screens }
        let available = fullScreenAvailable
        presentation.fullScreenAllowed = { available }
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "tap present's approval question") { presentation.pendingQuestion?.kind == "approval" }
        let talkSheet = try XCTUnwrap(deckWindow.questionSheet)
        // tap dev dies and comes back asking, while the talk's sheet is up.
        let devPid = try XCTUnwrap(controller.session.processIdentifier)
        kill(devPid, SIGKILL)
        try await waitUntil(timeout: 30, "tap dev's question again") {
            controller.pendingQuestion?.kind == "approval" && controller.session.processIdentifier != nil && controller.session.processIdentifier != devPid
        }
        XCTAssertEqual(deckWindow.deckQuestions.count, 1, "queued")
        XCTAssertTrue(deckWindow.questionSheet === talkSheet, "the talk's sheet is still the one up")
        try XCTUnwrap(talkSheet.button(titled: "Don't Allow")).performClick(nil)
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }
        XCTAssertFalse(storedApprovals().contains("approvals"), "the real tap present stored nothing for the talk's no")
        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertNil(deckWindow.questionSheet, "still waiting: a talk runs")
        XCTAssertEqual(deckWindow.deckQuestions.count, 1)
        try await stopPresenting(controller)
        try await waitUntil(timeout: 5, "the deck's question once the talk has ended") { deckWindow.questionSheet is ApprovalSheet }
        XCTAssertEqual(deckWindow.questionSheetSource, .deck)
        XCTAssertTrue(deckWindow.deckQuestions.isEmpty)
        try XCTUnwrap(deckWindow.questionSheet?.button(titled: "Don't Allow")).performClick(nil)
    }
}
