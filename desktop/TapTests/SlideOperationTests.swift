import XCTest
@testable import Tap

final class SlideOperationTests: HostedTestCase {
    func openOps() async throws -> (DeckDocument, DeckSessionController) {
        let document = try await openDeck(try Fixtures.copyDeck("ops.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "the panel") { controller.slidePanel.slides.count == 7 }
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        return (document, controller)
    }

    func titles(_ controller: DeckSessionController) async throws -> [String] {
        try await TapSlideList.list(text: controller.editor.string).slides.map(\.title)
    }

    /// The buffer as tap parses it, with the separator rule checked.
    func roundTrip(_ controller: DeckSessionController) async throws -> SlideList {
        let list = try await TapSlideList.list(text: controller.editor.string)
        TapSlideList.assertOneSeparatorBetweenSlides(controller.editor.string, list: list)
        return list
    }

    func waitForConfirmation(_ controller: DeckSessionController) async throws {
        try await waitUntil(timeout: 10, "tap's answer for the current text") { controller.lastAppliedText == controller.editor.string }
    }

    func testMoveWithTheKeyboard() async throws {
        let (document, controller) = try await openOps()
        controller.editor.moveCursor(toSlide: 2)
        let offset = controller.editor.selectedRange().location - controller.editor.boxes[2].range.location
        XCTAssertGreaterThan(offset, 0, "the caret sits after the heading, not at the slide's start")

        XCTAssertTrue(controller.moveSelectedSlides(by: -1), "Cmd+Option+Up")
        XCTAssertEqual(controller.editor.boxes.map(\.slide.title), ["One", "Three", "Two", "Four", "Five", "Six", "Seven"], "the boxes follow at once")
        XCTAssertEqual(controller.editor.currentBoxIndex, 1, "slide 3 is now slide 2, and the cursor moved with it")
        XCTAssertEqual(controller.editor.selectedRange().location - controller.editor.boxes[1].range.location, offset)
        XCTAssertEqual(controller.slidePanel.selectedNumbers, [2])

        let list = try await roundTrip(controller)
        XCTAssertEqual(list.slides.map(\.title), ["One", "Three", "Two", "Four", "Five", "Six", "Seven"])
        XCTAssertEqual(list.slides[1].layout, "section")
        try await waitForConfirmation(controller)
        XCTAssertEqual(controller.editor.boxes.map(\.slide.title), list.slides.map(\.title), "tap's answer agrees with the adopted boxes")

        XCTAssertTrue(controller.moveSelectedSlides(by: -1), "slide 2 moves to the top")
        try await waitForConfirmation(controller)
        XCTAssertFalse(controller.moveSelectedSlides(by: -1), "slide 1 cannot move up")
        XCTAssertEqual(document.undoManager?.undoActionName, "Move Slide")
    }

    func testDuplicateAndDelete() async throws {
        let (document, controller) = try await openOps()
        controller.editor.moveCursor(toSlide: 2)
        XCTAssertEqual(controller.perform(.duplicate(numbers: [3])), .applied)
        let afterDuplicate = try await roundTrip(controller)
        XCTAssertEqual(afterDuplicate.slides.map(\.title), ["One", "Two", "Three", "Three", "Four", "Five", "Six", "Seven"])
        XCTAssertEqual(controller.slidePanel.selectedNumbers, [4], "the copy is selected")
        XCTAssertEqual(controller.editor.currentBoxIndex, 3)
        try await waitForConfirmation(controller)

        controller.slidePanel.select(numbers: [6, 7], scroll: false)
        XCTAssertEqual(controller.selectedSlideNumbers, [6, 7])
        XCTAssertEqual(controller.perform(.delete(numbers: controller.selectedSlideNumbers)), .applied)
        let afterDelete = try await roundTrip(controller)
        XCTAssertEqual(afterDelete.slides.map(\.title), ["One", "Two", "Three", "Three", "Four", "Seven"])
        XCTAssertEqual(document.undoManager?.undoActionName, "Delete 2 Slides")
        try await waitForConfirmation(controller)

        document.undoManager?.undo()
        XCTAssertEqual(controller.editor.boxes.count, 8, "one undo step brings both back, and the boxes follow at once")
        let afterUndo = try await roundTrip(controller)
        XCTAssertEqual(afterUndo.slides.map(\.title), ["One", "Two", "Three", "Three", "Four", "Five", "Six", "Seven"])
    }

    func testSkipASlide() async throws {
        let (document, controller) = try await openOps()
        XCTAssertEqual(controller.perform(.setSkip(numbers: [4], skipped: true)), .applied)
        let text = controller.editor.string as NSString
        XCTAssertTrue(text.substring(with: controller.editor.boxes[3].range).hasPrefix("<!--\nskip: true\n-->\n\n# Four"), "skip: true in slide 4's directive comment")
        let list = try await roundTrip(controller)
        XCTAssertTrue(list.slides[3].skip, "tap reads the directive")
        XCTAssertEqual(list.slides.count, 7, "a skipped slide keeps its number")
        try await waitForConfirmation(controller)
        XCTAssertTrue(controller.editor.boxes[3].slide.skip)
        XCTAssertTrue(controller.editor.header(forBoxAt: 3).badges.contains("skipped"), "the box is marked in the editor")
        XCTAssertEqual(controller.slidePanel.item(forSlide: 4)?.thumbnailImageView.alphaValue, 0.45, "and dimmed in the sidebar")

        XCTAssertEqual(controller.perform(.setSkip(numbers: [4], skipped: false)), .applied)
        let afterUnskip = try await roundTrip(controller)
        XCTAssertFalse(afterUnskip.slides[3].skip)
        XCTAssertFalse(controller.editor.string.contains("skip:"))
        _ = document
    }

    func testUndoRestoresBoxesAndResyncsTap() async throws {
        let (document, controller) = try await openOps()
        let original = controller.editor.string
        XCTAssertEqual(controller.perform(.move(numbers: [5], beforeNumber: 3)), .applied)
        let moved = controller.editor.string
        try await waitForConfirmation(controller)
        XCTAssertEqual(controller.lastAppliedText, moved)

        document.undoManager?.undo()
        XCTAssertEqual(controller.editor.string, original, "Cmd+Z restores the text")
        XCTAssertEqual(controller.editor.boxes.map(\.slide.title), ["One", "Two", "Three", "Four", "Five", "Six", "Seven"],
                       "the boxes follow the reverted text the moment the undo group ends, before tap answers")
        try await waitUntil(timeout: 10, "tap to answer for the reverted text") { controller.lastAppliedText == original }
        XCTAssertEqual(controller.sourceSync.lastSentText, original, "the reverted text is what was sent")

        document.undoManager?.redo()
        XCTAssertEqual(controller.editor.string, moved)
        XCTAssertEqual(controller.editor.boxes.map(\.slide.title), ["One", "Two", "Five", "Three", "Four", "Six", "Seven"], "redo adopts the moved boxes again")
        try await waitUntil(timeout: 10, "tap to answer for the redone text") { controller.lastAppliedText == moved }
        XCTAssertEqual(controller.sourceSync.lastSentText, moved)
    }

    /// The defect this pins: a PUT of the old text is in flight when an
    /// operation permutes the boxes. Its answer must be refused, never
    /// applied with the operation's whole-region edit replayed onto it,
    /// which would stretch and collapse the boxes.
    func testAStaleAnswerNeverDisturbsAnOperation() async throws {
        let (_, controller) = try await openOps()
        var ordersSeen: [[String]] = []
        controller.onSlideListApplied = { [weak controller] _ in
            ordersSeen.append(controller?.editor.boxes.map(\.slide.title) ?? [])
        }
        // A send of the unchanged text goes out, and the move must land
        // while it is in flight. tap's own socket on this machine can
        // answer well inside a fixed sleep, which would let the send
        // resolve before the move even starts and prove nothing about
        // staleness, so the first answer is held back briefly here to make
        // the overlap the test needs deterministic rather than a race
        // against however fast tap happens to answer today.
        let originalSender = controller.sourceSync.sender
        var delayedFirstSend = false
        controller.sourceSync.sender = { source in
            if !delayedFirstSend {
                delayedFirstSend = true
                try await Task.sleep(nanoseconds: 200_000_000)
            }
            return try await originalSender!(source)
        }
        defer { controller.sourceSync.sender = originalSender }
        Task { await controller.sourceSync.sendNow() }
        try await Task.sleep(nanoseconds: 20_000_000)
        XCTAssertEqual(controller.perform(.move(numbers: [5], beforeNumber: 3)), .applied)
        try await waitForConfirmation(controller)
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertFalse(ordersSeen.isEmpty)
        for order in ordersSeen {
            XCTAssertEqual(order, ["One", "Two", "Five", "Three", "Four", "Six", "Seven"], "every answer applied after the move describes the moved text; the stale one was refused")
        }
        let finalOrder = try await roundTrip(controller)
        XCTAssertEqual(finalOrder.slides.map(\.title), ["One", "Two", "Five", "Three", "Four", "Six", "Seven"])
    }

    /// Typing that tap has not parsed yet shifts the boxes, and a typed
    /// "---" sits inside the box above it. An operation waits for tap's
    /// answer and then acts on the slides as tap counts them.
    func testAnOperationWaitsForTapToConfirmTypedText() async throws {
        let (_, controller) = try await openOps()
        controller.editor.moveCursor(toSlide: 6)
        let end = NSMaxRange(controller.editor.boxes[6].range)
        controller.editor.setSelectedRange(NSRange(location: end, length: 0))
        controller.editor.insertText("\n\n---\n\n# Eight", replacementRange: NSRange(location: NSNotFound, length: 0))
        XCTAssertNotEqual(controller.editor.string, controller.lastAppliedText, "tap has not answered for the new separator yet")
        XCTAssertTrue(controller.moveSelectedSlides(by: -1), "queued until tap confirms the text")
        try await waitUntil(timeout: 10, "the deferred move to land") { controller.editor.boxes.count == 8 && controller.editor.boxes[6].slide.title == "Eight" }
        try await waitForConfirmation(controller)
        let afterMove = try await roundTrip(controller)
        XCTAssertEqual(afterMove.slides.map(\.title), ["One", "Two", "Three", "Four", "Five", "Six", "Eight", "Seven"],
                       "the cursor was in the new slide 8, so slide 8 moved above slide 7, and slide 7's separator stayed where it was typed")
    }
}
