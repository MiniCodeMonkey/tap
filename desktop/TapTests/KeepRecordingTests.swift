import XCTest
@testable import Tap

final class KeepRecordingTests: PresentingTestCase {
    var revealed: [URL] = []

    /// A run folder with two segments and a chapters file, 3 kB in all.
    func recordingFolder() throws -> URL {
        let folder = try Fixtures.temporaryFolder().appendingPathComponent("conference-talk-2026-09-24-1932")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        try Data(count: 2048).write(to: folder.appendingPathComponent("segment-1.mov"))
        try Data(count: 1024).write(to: folder.appendingPathComponent("segment-2.mov"))
        try "00:00 Intro\n".write(to: folder.appendingPathComponent("chapters.txt"), atomically: true, encoding: .utf8)
        return folder
    }

    func startTalk(quit: FakeTapScripts.QuitBehavior, record: URL) async throws -> (DeckSessionController, DeckWindowController) {
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], quit: quit, recordingTo: record)
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        deckWindow.revealInFinder = { [weak self] url in self?.revealed.append(url) }
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        return (controller, deckWindow)
    }

    func testKeepTheRecording() async throws {
        let folder = try recordingFolder()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        let (controller, deckWindow) = try await startTalk(quit: .askToKeep(directory: folder, segments: 2), record: record)
        let presentation = controller.presentation
        deckWindow.stopPresenting(nil)
        XCTAssertEqual(presentation.state, .stopping)
        XCTAssertNil(presentation.audienceWindow, "the windows are already down")
        try await waitUntil(timeout: 10, "tap's keep-recording question") { deckWindow.questionSheet?.kind == "keep-recording" }
        let sheet = try XCTUnwrap(deckWindow.questionSheet)
        XCTAssertTrue(deckWindow.window?.attachedSheet === sheet)
        XCTAssertEqual(sheet.titleLabel.stringValue, "Keep this recording?")
        let size = ByteCountFormatter.string(fromByteCount: 3072, countStyle: .file)
        XCTAssertEqual(sheet.bodyLabel.stringValue, "2 segments, \(size) on disk.")
        XCTAssertEqual(sheet.pathLabel.stringValue, folder.path)
        XCTAssertEqual(sheet.declineButton.title, "Delete")
        XCTAssertEqual(sheet.declineButton.keyEquivalent, "", "no key can delete")
        XCTAssertTrue(sheet.declineButton.hasDestructiveAction)
        XCTAssertEqual(sheet.acceptButton.title, "Keep and Show in Finder")
        XCTAssertTrue(presentation.session?.log.text.contains("tap asks a keep-recording question") == true)

        // A second Escape, the one that ended the talk a moment ago, lands on the sheet and does nothing.
        let escape = try XCTUnwrap(NSEvent.keyEvent(with: .keyDown, location: .zero, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                                    windowNumber: sheet.windowNumber, context: nil, characters: "\u{1b}",
                                                    charactersIgnoringModifiers: "\u{1b}", isARepeat: false, keyCode: 53))
        XCTAssertFalse(sheet.performKeyEquivalent(with: escape))
        XCTAssertTrue(deckWindow.window?.attachedSheet === sheet, "the sheet is still up")
        try await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertFalse((try? String(contentsOf: record, encoding: .utf8))?.contains(#""value":false"#) ?? false, "nothing was deleted")
        XCTAssertEqual(presentation.state, .stopping, "tap is still waiting for the answer")

        try XCTUnwrap(sheet.button(titled: "Keep and Show in Finder")).performClick(nil)
        XCTAssertEqual(revealed, [folder], "a kept run is revealed in Finder")
        try await waitUntil(timeout: 5, "the answer to reach tap") {
            (try? String(contentsOf: record, encoding: .utf8))?.contains(#"stdin: {"type":"answer","id":"q1","value":true}"#) == true
        }
        try await waitUntil(timeout: 10, "the talk to end") { presentation.state == .idle }
        XCTAssertNil(deckWindow.questionSheet)
    }

    func testDeletingTheRecordingAnswersNo() async throws {
        let folder = try recordingFolder()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        let (controller, deckWindow) = try await startTalk(quit: .askToKeep(directory: folder, segments: 2), record: record)
        deckWindow.stopPresenting(nil)
        try await waitUntil(timeout: 10, "the question") { deckWindow.questionSheet?.kind == "keep-recording" }
        try XCTUnwrap(deckWindow.questionSheet?.button(titled: "Delete")).performClick(nil)
        XCTAssertEqual(revealed, [], "nothing to show for a deleted run")
        try await waitUntil(timeout: 5, "the answer to reach tap") {
            (try? String(contentsOf: record, encoding: .utf8))?.contains(#"stdin: {"type":"answer","id":"q1","value":false}"#) == true
        }
        try await waitUntil(timeout: 10, "the talk to end") { controller.presentation.state == .idle }
    }

    func testATapThatKeepsWithoutWaitingRevealsTheRun() async throws {
        let folder = try recordingFolder()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        let (controller, deckWindow) = try await startTalk(quit: .askToKeepThenExit(after: 0.5, directory: folder, segments: 1), record: record)
        deckWindow.stopPresenting(nil)
        try await waitUntil(timeout: 10, "the question") { deckWindow.questionSheet?.kind == "keep-recording" }
        // tap's own wait (60 s in tap, 0.5 s in the fake) is over: it kept the recording and exited.
        try await waitUntil(timeout: 10, "the talk to end") { controller.presentation.state == .idle }
        XCTAssertNil(deckWindow.questionSheet, "the sheet goes with the process that asked")
        XCTAssertNil(deckWindow.window?.attachedSheet)
        XCTAssertEqual(revealed, [folder], "kept, so shown")
        XCTAssertTrue(controller.presentation.lastTalkLog?.text.contains("tap kept the recording") == true, "in the talk's log, not tap dev's")
    }

    func testAClosedDeckKeepsItsRecordingWithoutAsking() async throws {
        let folder = try recordingFolder()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record")
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.presenting(events: [], quit: .askToKeep(directory: folder, segments: 2), recordingTo: record)
        let (document, controller) = try await openDeckForPresenting()
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let talk = WeakTalk(controller.presentation)
        // The deck closes mid-talk: nobody is left to answer, so the app answers for tap's own default at once.
        document.close()
        try await waitUntil(timeout: 2, "the keep answer to reach tap") {
            (try? String(contentsOf: record, encoding: .utf8))?.contains(#"stdin: {"type":"answer","id":"q1","value":true}"#) == true
        }
        try await waitUntil(timeout: 5, "the talk counted out") { !AppEnvironment.shared.isPresenting }
        XCTAssertTrue(AppEnvironment.shared.endingTalks.isEmpty)
        XCTAssertEqual(revealed, [], "no window to reveal from")
        XCTAssertTrue(talk.presentation?.lastTalkLog?.text.contains("the deck window has closed; the keep-recording question is answered keep") ?? true)
    }
}
