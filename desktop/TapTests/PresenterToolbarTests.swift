import XCTest
@testable import Tap

final class PresenterToolbarTests: PresentingTestCase {
    /// A pointer move to `point`, in window coordinates, as AppKit's tracking area would deliver it.
    func mouseMove(to point: NSPoint, in window: NSWindow) throws -> NSEvent {
        try XCTUnwrap(NSEvent.mouseEvent(with: .mouseMoved, location: point, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                         windowNumber: window.windowNumber, context: nil, eventNumber: 0, clickCount: 0, pressure: 0))
    }

    func testPresenterControls() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        let toolbar = try XCTUnwrap(presenter.presenterToolbar)
        let dot = try XCTUnwrap(presenter.recordingDot)
        XCTAssertNil(presentation.audienceWindow?.presenterToolbar, "the audience window has no toolbar")
        XCTAssertTrue(toolbar.isHidden, "the toolbar is out of sight until the pointer reaches the bottom edge")
        XCTAssertFalse(toolbar.isShown)
        XCTAssertTrue(dot.isHidden, "no REC dot while nothing records")
        XCTAssertTrue(presenter.container.trackingAreas.contains { $0.options.contains(.mouseMoved) && $0.options.contains(.activeAlways) },
                      "the content view tracks the pointer everywhere in the window")
        XCTAssertTrue(presenter.container.trackingAreas.contains { $0.options.contains(.mouseEnteredAndExited) }, "and hears it leave")

        // The pointer reaches the bottom edge: the toolbar slides up, and the idle cursor timer is armed.
        let height = presenter.container.bounds.height
        presenter.container.mouseMoved(with: try mouseMove(to: NSPoint(x: 400, y: 1), in: presenter))
        XCTAssertTrue(toolbar.isShown)
        XCTAssertFalse(toolbar.isHidden)
        XCTAssertTrue(presentation.isCursorHideArmed, "a move over a talk window arms the cursor hide")
        XCTAssertEqual(toolbar.recordButton.title, "NOT RECORDING")
        XCTAssertEqual(toolbar.reloadButton.title, "Reload Slides")
        XCTAssertEqual(toolbar.swapButton.title, "Swap Displays")
        XCTAssertEqual(toolbar.stopButton.title, "Stop")
        XCTAssertTrue(toolbar.editsLabel.isHidden)
        XCTAssertEqual(toolbar.frame.minY, 0, "it sits along the bottom edge, away from the menu bar full screen drops over the top")
        XCTAssertEqual(toolbar.frame.height, PresenterToolbar.height)

        // The pointer moves up into the page: the toolbar slides away after its delay. The top edge is the menu bar's, not the toolbar's.
        toolbar.hideDelay = 0.1
        presenter.container.mouseMoved(with: try mouseMove(to: NSPoint(x: 400, y: height / 2), in: presenter))
        try await waitUntil(timeout: 2, "the toolbar to slide away") { toolbar.isHidden }
        presenter.container.mouseMoved(with: try mouseMove(to: NSPoint(x: 400, y: height - 1), in: presenter))
        XCTAssertTrue(toolbar.isHidden, "the top edge does nothing")
        // A move within the toolbar's own band keeps it.
        presenter.container.mouseMoved(with: try mouseMove(to: NSPoint(x: 400, y: 1), in: presenter))
        presenter.container.mouseMoved(with: try mouseMove(to: NSPoint(x: 400, y: PresenterToolbar.height / 2), in: presenter))
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertFalse(toolbar.isHidden, "the pointer on the toolbar itself does not send it away")
        presenter.container.mouseMoved(with: try mouseMove(to: NSPoint(x: 400, y: height / 2), in: presenter))
        try await waitUntil(timeout: 2, "the toolbar away") { toolbar.isHidden }
        toolbar.pointerReachedBottomEdge()
        toolbar.pointerLeft()
        toolbar.pointerReachedBottomEdge()
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertFalse(toolbar.isHidden, "coming back cancels the hide")
        // The pointer leaves the window from the toolbar (onto the other display, or the mouse is put down): the toolbar goes too.
        let exit = try XCTUnwrap(NSEvent.enterExitEvent(with: .mouseExited, location: NSPoint(x: 400, y: -1), modifierFlags: [],
                                                        timestamp: ProcessInfo.processInfo.systemUptime, windowNumber: presenter.windowNumber,
                                                        context: nil, eventNumber: 0, trackingNumber: 0, userData: nil))
        presenter.container.mouseExited(with: exit)
        try await waitUntil(timeout: 2, "the toolbar away once the pointer left the window") { toolbar.isHidden }

        // The toolbar's own buttons: Reload Slides saves, then tap reads the file again; Swap has nothing to swap on one display.
        var saves = 0
        let realSave = presentation.saveDeck
        presentation.saveDeck = { completion in
            saves += 1
            realSave(completion)
        }
        toolbar.pointerReachedBottomEdge()
        toolbar.reloadButton.performClick(nil)
        XCTAssertEqual(saves, 1, "the toolbar's Reload Slides saves first")
        presentation.saveDeck = realSave
        toolbar.swapButton.performClick(nil)
        XCTAssertEqual(presentation.state, .presenting)

        // The REC dot stays visible in a corner at all times while recording.
        presentation.handle(.recording(RecordingEvent(state: "recording", segment: 1, elapsed: 5, disk: "ok")))
        XCTAssertFalse(dot.isHidden)
        XCTAssertEqual(toolbar.recordButton.title, "REC 0:05")
        XCTAssertEqual(dot.frame.maxY, presenter.container.bounds.maxY - 16, accuracy: 1)
        XCTAssertEqual(dot.frame.maxX, presenter.container.bounds.maxX - 18, accuracy: 1)
        toolbar.pointerLeft()
        try await waitUntil(timeout: 2, "the toolbar away again") { toolbar.isHidden }
        XCTAssertFalse(dot.isHidden, "the dot does not go with the toolbar")
        presentation.handle(.recording(RecordingEvent(state: "stopped", segment: 1, elapsed: 0, disk: "ok")))
        XCTAssertTrue(dot.isHidden)

        toolbar.pointerReachedBottomEdge()
        toolbar.stopButton.performClick(nil)
        XCTAssertEqual(presentation.state, .stopping)
    }

    func testARehearsalHasNoRecordingOrSwapControls() async throws {
        let (_, controller) = try await openDeckForPresenting()
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let toolbar = try XCTUnwrap(controller.presentation.presenterWindow?.presenterToolbar)
        toolbar.pointerReachedBottomEdge()
        XCTAssertEqual(toolbar.titleLabel.stringValue, "Rehearsal")
        XCTAssertTrue(toolbar.recordButton.isHidden)
        XCTAssertTrue(toolbar.swapButton.isHidden)
        XCTAssertFalse(toolbar.reloadButton.isHidden)
        XCTAssertFalse(toolbar.stopButton.isHidden)
    }

    func testEditWhilePresenting() async throws {
        let (document, controller) = try await openDeckForPresenting()
        let deck = try XCTUnwrap(document.fileURL)
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let presentation = controller.presentation
        controller.jumpToSlide(number: 1)
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let toolbar = try XCTUnwrap(presentation.presenterWindow?.presenterToolbar)
        try await waitUntil(timeout: 20, "the audience page on slide 1") { audience.page.lastReady?.slide == 1 }
        var text = await audience.page.pageText()
        XCTAssertTrue(text.contains("One"))
        XCTAssertEqual(presentation.editsNotShown, 0)

        // An edit reaches tap dev, never tap present.
        let end = (controller.editor.string as NSString).range(of: "# One").upperBound
        controller.editor.setSelectedRange(NSRange(location: end, length: 0))
        controller.editor.insertText(" edited", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitUntil(timeout: 15, "tap dev's answer") { controller.lastAppliedText?.contains("# One edited") == true }
        try await Task.sleep(nanoseconds: 500_000_000)
        text = await audience.page.pageText()
        XCTAssertFalse(text.contains("One edited"), "the audience does not see the change: tap present does not watch files")
        XCTAssertEqual(presentation.editsNotShown, 1)
        toolbar.pointerReachedBottomEdge()
        XCTAssertFalse(toolbar.editsLabel.isHidden)
        XCTAssertEqual(toolbar.editsLabel.stringValue, "1 edit not shown")

        // Another typing pause counts again; undoing back to the presented text counts nothing.
        controller.editor.insertText("!", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitUntil(timeout: 15, "the second answer") { controller.lastAppliedText?.contains("# One edited!") == true }
        XCTAssertEqual(presentation.editsNotShown, 2)
        XCTAssertEqual(toolbar.editsLabel.stringValue, "2 edits not shown")
        // tap dev answering again for the same text (a restart, a component change) is not an edit.
        presentation.deckTextChanged(try XCTUnwrap(controller.lastAppliedText))
        XCTAssertEqual(presentation.editsNotShown, 2, "the same text counts once")

        // Present > Reload Slides saves and reloads, as r does in tap present.
        deckWindow.reloadSlides(nil)
        // waitUntil takes a synchronous condition, and reading the page is async.
        let deadline = Date().addingTimeInterval(20)
        var shown = await audience.page.pageText()
        while !shown.contains("One edited!") {
            if Date() > deadline {
                XCTFail("the audience never showed the edit: \(shown)")
                throw CancellationError()
            }
            try await Task.sleep(nanoseconds: 200_000_000)
            shown = await audience.page.pageText()
        }
        XCTAssertEqual(presentation.editsNotShown, 0)
        XCTAssertTrue(toolbar.editsLabel.isHidden)
        XCTAssertTrue(try String(contentsOf: deck, encoding: .utf8).contains("# One edited!"), "Reload Slides saved the buffer first")
        XCTAssertFalse(document.isDocumentEdited)

        // An edit, then the text back to what tap present read: nothing is unshown.
        let presented = try XCTUnwrap(presentation.presentedText)
        presentation.deckTextChanged(presented + "\nmore")
        XCTAssertEqual(presentation.editsNotShown, 1)
        presentation.deckTextChanged(presented)
        XCTAssertEqual(presentation.editsNotShown, 0, "typing back to the presented text leaves no edit unshown")
    }
}
