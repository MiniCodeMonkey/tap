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
        for _ in 0..<3 {
            try await waitUntil(timeout: 10, "a running tap present or the end") {
                if case .failed = presentation.state { return true }
                return presentation.session?.processIdentifier != nil
            }
            guard let pid = presentation.session?.processIdentifier else { break }
            kill(pid, SIGKILL)
            try await waitUntil(timeout: 10, "the killed process to be gone") { presentation.session?.processIdentifier != pid }
        }
        try await waitUntil(timeout: 30, "the talk to fail") { if case .failed = presentation.state { return true } else { return false } }
        guard case .failed(let message) = presentation.state else { return XCTFail() }
        XCTAssertTrue(message.contains("tap exited 3 times in 30 seconds"), message)
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
