import XCTest
@testable import Tap

final class PresentingFailureTests: PresentingTestCase {
    func windowController(_ controller: DeckSessionController) throws -> DeckWindowController {
        try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
    }

    func testATalkThatCannotStartShowsABar() async throws {
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.failing(code: "deck_not_found", message: "deck not found: talk.md")
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        let presentation = controller.presentation
        controller.jumpToSlide(number: 1)
        presentation.start(PresentationOptions(mode: .play, startSlide: 3))
        try await waitUntil(timeout: 30, "the talk to fail") { if case .failed = presentation.state { return true } else { return false } }
        guard case .failed(let message) = presentation.state else { return XCTFail() }
        XCTAssertTrue(message.contains("deck not found: talk.md"), "tap's own reason: \(message)")
        XCTAssertEqual(controller.currentSlideNumber, 1, "no window ever showed, so the cursor did not move to the start slide")
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertFalse(AppEnvironment.shared.isPresenting)
        let bar = try XCTUnwrap(controller.editorViewController.bar(.talkFailed))
        XCTAssertEqual(bar.message, "The talk could not run.")
        XCTAssertNotNil(bar.button(titled: "Show Tap Log"))
        XCTAssertTrue(presentation.canStart, "Play is back")
        XCTAssertTrue(deckWindow.playButton.isEnabled)
        TapLogWindowController.shared.reload()
        XCTAssertEqual(TapLogWindowController.shared.picker.segmentCount, 2, "the failed talk's log stays readable")
        try XCTUnwrap(bar.button(titled: "Dismiss")).performClick(nil)
        XCTAssertNil(controller.editorViewController.bar(.talkFailed))

        // A second failure, whose tap sends no error event: the reason is its own, never the last talk's.
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.failingWithoutAnEvent(stderr: "the second tap had nothing to say")
        presentation.start(PresentationOptions(mode: .play, startSlide: 3))
        try await waitUntil(timeout: 30, "the second failure") { if case .failed = presentation.state { return true } else { return false } }
        guard case .failed(let secondMessage) = presentation.state else { return XCTFail() }
        XCTAssertFalse(secondMessage.contains("deck not found"), "the first talk's error is not this one's reason: \(secondMessage)")
        XCTAssertNotNil(controller.editorViewController.bar(.talkFailed))

        // Play again, with a tap that starts: the old bar goes as the new talk starts.
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.readyAndWaiting()
        deckWindow.startPresenting(PresentationOptions(mode: .play, startSlide: 1))
        XCTAssertEqual(presentation.state, .starting)
        XCTAssertNil(controller.editorViewController.bar(.talkFailed), "a new start clears the failed talk's bar")
    }

    func testATalkThatFailsOnARestartSaysItStopped() async throws {
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.readyAndWaiting()
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        controller.jumpToSlide(number: 1)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 3))
        // The display goes, then tap present dies: the restart finds nowhere to show the talk.
        presentation.screens = { [] }
        let pid = try XCTUnwrap(presentation.session?.processIdentifier)
        kill(pid, SIGKILL)
        try await waitUntil(timeout: 30, "the talk to fail") { if case .failed = presentation.state { return true } else { return false } }
        guard case .failed(let message) = presentation.state else { return XCTFail() }
        XCTAssertEqual(message, "No display is connected.")
        XCTAssertTrue(presentation.failedAfterShowing, "the talk ran, then stopped")
        let bar = try XCTUnwrap(controller.editorViewController.bar(.talkFailed))
        XCTAssertEqual(bar.message, "The talk stopped.")
        XCTAssertEqual(controller.currentSlideNumber, 3, "the cursor is on the last slide presented")
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
    }

    func testATalkThatCannotBeSavedDoesNotStart() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deck = try XCTUnwrap(document.fileURL)
        let presentation = controller.presentation
        // An unsaved edit, and another program's change on disk: the app refuses to save over it.
        let end = (controller.editor.string as NSString).range(of: "# One").upperBound
        controller.editor.setSelectedRange(NSRange(location: end, length: 0))
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))
        try "# Theirs\n".write(to: deck, atomically: true, encoding: .utf8)
        controller.diskChanged()
        XCTAssertTrue(controller.hasDiskConflict)
        controller.jumpToSlide(number: 4)
        presentation.start(PresentationOptions(mode: .play, startSlide: 4))
        try await waitUntil(timeout: 10, "the refusal") { if case .failed = presentation.state { return true } else { return false } }
        guard case .failed(let message) = presentation.state else { return XCTFail() }
        XCTAssertEqual(message, "The deck could not be saved: resolve the change on disk first.", "what to do, not CocoaError's text")
        XCTAssertNil(presentation.session, "tap present never started")
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertFalse(AppEnvironment.shared.isPresenting, "counted out again")
        XCTAssertNotNil(controller.editorViewController.bar(.talkFailed))
        XCTAssertEqual(controller.currentSlideNumber, 4, "no talk ran, so the cursor did not move")
        XCTAssertEqual(try String(contentsOf: deck, encoding: .utf8), "# Theirs\n", "the other program's file is untouched")
    }

    func testPlayIsDisabledWhileADeckHasNoFile() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deckWindow = try windowController(controller)
        XCTAssertTrue(controller.presentation.canStart)
        document.fileWasDeleted()
        XCTAssertFalse(controller.presentation.canStart, "tap present needs a file to read")
        deckWindow.refreshPresentingControls()
        XCTAssertFalse(deckWindow.playButton.isEnabled)
        deckWindow.playButtonClicked(modifiers: [.shift])
        deckWindow.play(nil)
        XCTAssertEqual(controller.presentation.state, .idle)
    }

    func testPlayIsDisabledWhileAnotherDeckPresents() async throws {
        let (_, first) = try await openDeckForPresenting()
        try await startPresenting(first, PresentationOptions(mode: .rehearse, startSlide: 1))
        let (_, second) = try await openDeckForPresenting()
        let secondWindow = try windowController(second)
        XCTAssertFalse(second.presentation.canStart, "one talk at a time, app-wide")
        XCTAssertFalse(secondWindow.playButton.isEnabled)
        secondWindow.play(nil)
        XCTAssertEqual(second.presentation.state, .idle)
        try await stopPresenting(first)
        XCTAssertTrue(second.presentation.canStart)
        XCTAssertTrue(secondWindow.playButton.isEnabled, "the notification reached the other deck")
    }

    func testATalkSurvivesATapPresentRestart() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 2))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        let client = try XCTUnwrap(presentation.client)
        let socket = client.openSocket()
        socket.resume()
        try await Task.sleep(nanoseconds: 300_000_000)
        socket.send(SlideMessage(slideIndex: 2, fragment: -1, step: 0))
        try await waitUntil(timeout: 10, "slide 3") { presentation.lastSlide == 3 }
        socket.close()
        let firstPid = try XCTUnwrap(presentation.session?.processIdentifier)

        kill(firstPid, SIGKILL)
        try await waitUntil(timeout: 30, "tap present back on a new process") {
            if let pid = presentation.session?.processIdentifier, pid != firstPid, case .running = presentation.session?.state { return true }
            return false
        }
        XCTAssertEqual(presentation.state, .presenting, "the talk never ended")
        XCTAssertTrue(presentation.sleepAssertion.isHeld)
        XCTAssertTrue(presentation.audienceWindow === audience, "the same windows")
        XCTAssertTrue(presentation.presenterWindow === presenter)
        try await waitUntil(timeout: 20, "the pages reloaded at the last slide") {
            audience.page.pageLoadCount == 2 && presenter.page.pageLoadCount == 2
        }
        XCTAssertEqual(audience.page.lastLoadedURL?.fragment, "3")
        XCTAssertEqual(audience.page.lastLoadedURL?.port, presentation.client?.ready.port, "the new process's port")
        try await waitUntil(timeout: 20, "the audience page on slide 3 again") { audience.page.lastReady?.slide == 3 }
        XCTAssertTrue(audience.isVisible)
    }

    func testATalkEndsWhenTapPresentKeepsDying() async throws {
        AppEnvironment.shared.presentExecutableURL = try FakeTapScripts.readyAndWaiting()
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        controller.jumpToSlide(number: 1)
        // The talk is on (the windows show after the fallback, since the fake has no pages) when tap starts dying.
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 3))
        // Errors from the first process: one that is not a failure is not kept, and a fatal one is forgotten once a new process is ready.
        presentation.handle(.error(TapErrorPayload(code: "recording_failed", message: "the disk filled up")))
        XCTAssertNil(presentation.lastErrorMessage, "a recording error does not end the command")
        presentation.handle(.error(TapErrorPayload(code: "internal", message: "an earlier process's trouble")))
        XCTAssertEqual(presentation.lastErrorMessage, "an earlier process's trouble")
        for _ in 0..<3 {
            try await waitUntil(timeout: 10, "a ready tap present or the end") {
                if case .failed = presentation.state { return true }
                if case .running = presentation.session?.state { return presentation.session?.processIdentifier != nil }
                return false
            }
            guard let pid = presentation.session?.processIdentifier else { break }
            kill(pid, SIGKILL)
            try await waitUntil(timeout: 10, "the killed process to be gone") { presentation.session?.processIdentifier != pid }
        }
        try await waitUntil(timeout: 30, "the talk to fail") { if case .failed = presentation.state { return true } else { return false } }
        guard case .failed(let message) = presentation.state else { return XCTFail() }
        XCTAssertTrue(message.contains("tap exited 3 times in 30 seconds"), message)
        XCTAssertFalse(message.contains("an earlier process"), "a restarted process is not blamed with the first one's error: \(message)")
        XCTAssertTrue(presentation.failedAfterShowing)
        XCTAssertNil(presentation.audienceWindow)
        XCTAssertNil(presentation.presenterWindow)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        XCTAssertFalse(AppEnvironment.shared.isPresenting)
        try await waitUntil(timeout: 10, "nothing left in full screen") { fullScreenPresentationWindows().isEmpty && presentation.windowsGoingDown.isEmpty }
        let bar = try XCTUnwrap(controller.editorViewController.bar(.talkFailed))
        XCTAssertTrue(bar.message.contains("The talk stopped"))
        XCTAssertEqual(controller.currentSlideNumber, 3, "the cursor is on the last slide presented, not where it was")
    }
}
