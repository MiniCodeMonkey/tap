import XCTest
@testable import Tap

final class RecordingTests: PresentingTestCase {
    func windowController(_ controller: DeckSessionController) throws -> DeckWindowController {
        try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
    }

    func testFirstTalkAsksAboutRecording() async throws {
        try removeRecordingConsent()
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let presentation = controller.presentation
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "tap's consent question") { presentation.pendingQuestion?.kind == "record-consent" }
        let sheet = try XCTUnwrap(deckWindow.questionSheet)
        XCTAssertEqual(sheet.kind, "record-consent")
        XCTAssertTrue(deckWindow.window?.attachedSheet === sheet, "a sheet on the deck window")
        XCTAssertEqual(sheet.titleLabel.stringValue, "Record automatically every time you present?", "the spec's words (05-presenting)")
        XCTAssertTrue(sheet.bodyLabel.stringValue.contains("follows the projector"))
        XCTAssertEqual(sheet.pathLabel.stringValue, settingsFile.path, "tap says where the answer is saved")
        XCTAssertEqual(sheet.declineButton.title, "Don't Record")
        XCTAssertEqual(sheet.acceptButton.title, "Record Automatically")
        XCTAssertEqual(presentation.state, .starting, "the talk waits for the answer")
        // Past the page load and the show-windows fallback: the sheet still has nothing over it.
        try await Task.sleep(nanoseconds: UInt64((PresentationController.showWindowsFallbackInterval + 0.5) * 1_000_000_000))
        XCTAssertEqual(presentation.state, .starting)
        XCTAssertFalse(presentation.windowsShown, "nothing covers the sheet")
        XCTAssertFalse(presentation.audienceWindow?.isVisible ?? false)
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty)
        if let audience = presentation.audienceWindow {
            XCTAssertFalse(onScreenWindowNumbers().contains(audience.windowNumber))
        }

        try XCTUnwrap(sheet.button(titled: "Don't Record")).performClick(nil)
        XCTAssertNil(deckWindow.questionSheet)
        XCTAssertNil(deckWindow.window?.attachedSheet)
        XCTAssertNil(presentation.pendingQuestion)
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }
        try await waitUntil(timeout: 10, "tap to save the answer") {
            (try? String(contentsOf: self.settingsFile, encoding: .utf8))?.contains("record: false") == true
        }
        XCTAssertFalse(presentation.recording.isRecording)
        XCTAssertEqual(presentation.presenterWindow?.presenterToolbar?.recordButton.title, "NOT RECORDING")

        // The answer is the CLI's too: the next talk asks nothing.
        try await stopPresenting(controller)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertNil(deckWindow.questionSheet)
    }

    func testAQuestionDuringTheTalkBringsTheDeckWindowForward() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let deck = try XCTUnwrap(deckWindow.window)
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        presentation.handle(.question(id: "q9", kind: "record-consent", payload: QuestionPayload(settingsPath: "/tmp/settings.yaml")))
        let sheet = try XCTUnwrap(deckWindow.questionSheet)
        XCTAssertTrue(deck.attachedSheet === sheet)
        XCTAssertNil(presentation.windowsGoingDown.first, "the talk windows stay where they are")
        // In full screen the deck window's Space becomes the active one and the audience's leaves the screen;
        // on a host without full screen the deck window simply comes over the audience's plain window.
        try await waitUntil(timeout: 5, "the deck window in front of the talk") {
            let order = onScreenWindowNumbers()
            guard let deckIndex = order.firstIndex(of: deck.windowNumber) else { return false }
            return order.firstIndex(of: audience.windowNumber).map { deckIndex < $0 } ?? true
        }

        // A second question arriving on top waits its turn.
        presentation.handle(.question(id: "q10", kind: "approval", payload: QuestionPayload(deck: "/tmp/ops.md")))
        XCTAssertEqual(presentation.pendingQuestions.count, 2)
        XCTAssertTrue(deckWindow.questionSheet === sheet, "the first sheet is still the one up")

        try XCTUnwrap(sheet.button(titled: "Record Automatically")).performClick(nil)
        XCTAssertNil(presentation.pendingQuestions.first { $0.id == "q9" })
        XCTAssertNil(deckWindow.questionSheet, "the approval is declined with a log line until D5, so no second sheet")
        try await waitUntil(timeout: 5, "the approval answered") { presentation.pendingQuestions.isEmpty }
        XCTAssertTrue(presentation.frontWindow === audience)
        try await waitUntil(timeout: 5, "the talk in front again") {
            let order = onScreenWindowNumbers()
            guard let audienceIndex = order.firstIndex(of: audience.windowNumber) else { return false }
            return order.firstIndex(of: deck.windowNumber).map { audienceIndex < $0 } ?? true
        }
        XCTAssertEqual(presentation.state, .presenting)

        // Two sheets queued: answering the first puts the second up, and the talk stays behind it until it is answered too.
        presentation.handle(.question(id: "q11", kind: "record-consent", payload: QuestionPayload(settingsPath: "/tmp/settings.yaml")))
        let first = try XCTUnwrap(deckWindow.questionSheet)
        presentation.handle(.question(id: "q12", kind: "record-consent", payload: QuestionPayload(settingsPath: "/tmp/settings.yaml")))
        XCTAssertTrue(deckWindow.questionSheet === first)
        try await waitUntil(timeout: 5, "the deck window in front of the talk again") {
            let order = onScreenWindowNumbers()
            guard let deckIndex = order.firstIndex(of: deck.windowNumber) else { return false }
            return order.firstIndex(of: audience.windowNumber).map { deckIndex < $0 } ?? true
        }
        try XCTUnwrap(first.button(titled: "Don't Record")).performClick(nil)
        let second = try XCTUnwrap(deckWindow.questionSheet, "the second question's sheet")
        XCTAssertFalse(second === first)
        try await waitUntil(timeout: 5, "the second sheet on the deck window") { deck.attachedSheet === second }
        // Long enough for a return to the talk to have taken the screen.
        try await Task.sleep(nanoseconds: 1_000_000_000)
        let order = onScreenWindowNumbers()
        let deckIndex = try XCTUnwrap(order.firstIndex(of: deck.windowNumber), "the deck window is still on screen, with the second sheet")
        if let audienceIndex = order.firstIndex(of: audience.windowNumber) {
            XCTAssertLessThan(deckIndex, audienceIndex, "the talk does not cover the second sheet")
        }
        try XCTUnwrap(second.button(titled: "Don't Record")).performClick(nil)
        XCTAssertTrue(presentation.pendingQuestions.isEmpty)
        try await waitUntil(timeout: 5, "the talk in front once every question is answered") {
            let order = onScreenWindowNumbers()
            guard let audienceIndex = order.firstIndex(of: audience.windowNumber) else { return false }
            return order.firstIndex(of: deck.windowNumber).map { audienceIndex < $0 } ?? true
        }
    }

    func testStopDuringTheConsentSheetEndsTheSheetToo() async throws {
        try removeRecordingConsent()
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let presentation = controller.presentation
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "tap's consent question") { deckWindow.questionSheet?.kind == "record-consent" }
        let stale = try XCTUnwrap(deckWindow.questionSheet)
        // Present > Stop is enabled while the talk is starting.
        deckWindow.stopPresenting(nil)
        try await waitUntil(timeout: 30, "the talk to end") { presentation.state == .idle }
        XCTAssertNil(deckWindow.questionSheet, "a talk's sheets end with it")
        XCTAssertNil(deckWindow.window?.attachedSheet)
        XCTAssertTrue(presentation.pendingQuestions.isEmpty)
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty)

        // The next talk asks again, with a sheet of its own, and nothing stale clears it.
        presentation.start(PresentationOptions(mode: .play, startSlide: 1))
        try await waitUntil(timeout: 30, "the new consent question") { deckWindow.questionSheet != nil && deckWindow.questionSheet !== stale }
        let sheet = try XCTUnwrap(deckWindow.questionSheet)
        try XCTUnwrap(sheet.button(titled: "Don't Record")).performClick(nil)
        XCTAssertNil(deckWindow.questionSheet)
        try await waitUntil(timeout: 40, "the talk") { presentation.state == .presenting }
    }

    func testRecordingFollowsTapPresent() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(
            events: [#"{"type":"recording","state":"recording","segment":1,"elapsed":724,"disk":"ok"}"#], recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let toolbar = try XCTUnwrap(presentation.presenterWindow?.presenterToolbar)
        let dot = try XCTUnwrap(presentation.presenterWindow?.recordingDot)
        try await waitUntil(timeout: 10, "tap's recording event") { presentation.recording.isRecording }
        XCTAssertEqual(presentation.recording.segment, 1)
        XCTAssertTrue(toolbar.recordButton.title.hasPrefix("REC 12:0"), "REC 12:04 as tap reports it, counting up: \(toolbar.recordButton.title)")
        XCTAssertFalse(dot.isHidden)
        let shown = presentation.recording.elapsed
        try await waitUntil(timeout: 3, "the clock to count up between events") { presentation.recording.elapsed > shown }
        XCTAssertEqual(toolbar.recordButton.title, "REC " + RecordingStatus.clock(presentation.recording.elapsed))

        // REC stops the recording, and REC again starts a new segment, through tap.
        toolbar.recordButton.performClick(nil)
        try await waitUntil(timeout: 5, "the stop command") {
            (try? String(contentsOf: record, encoding: .utf8))?.contains(#"stdin: {"type":"recording","action":"stop"}"#) == true
        }
        try await waitUntil(timeout: 5, "tap's stopped event") { !presentation.recording.isRecording }
        XCTAssertEqual(toolbar.recordButton.title, "NOT RECORDING")
        XCTAssertTrue(dot.isHidden)
        toolbar.recordButton.performClick(nil)
        try await waitUntil(timeout: 5, "the new segment") { presentation.recording.segment == 2 && presentation.recording.isRecording }
        XCTAssertTrue(toolbar.recordButton.title.hasPrefix("REC 0:0"), "a fresh segment; the 1 s timer may already have ticked: \(toolbar.recordButton.title)")

        // A blocked recording stays NOT RECORDING, with tap's reason kept.
        presentation.handle(.recording(RecordingEvent(state: "stopped", segment: 2, elapsed: 0, disk: "ok")))
        presentation.handle(.error(TapErrorPayload(code: "recording_blocked", message: "Screen Recording is off for Tap in System Settings")))
        XCTAssertEqual(presentation.recording.blockedReason, "Screen Recording is off for Tap in System Settings")
        XCTAssertEqual(toolbar.recordButton.title, "NOT RECORDING")
    }
}
