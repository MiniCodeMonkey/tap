import AppKit
import XCTest
@testable import Tap

final class DragAndDropTests: HostedTestCase {
    func openOps() async throws -> (DeckDocument, DeckSessionController) {
        let document = try await openDeck(try Fixtures.copyDeck("ops.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "the panel") { controller.slidePanel.slides.count == 7 }
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        return (document, controller)
    }

    func roundTrip(_ controller: DeckSessionController) async throws -> [String] {
        let list = try await TapSlideList.list(text: controller.editor.string)
        TapSlideList.assertOneSeparatorBetweenSlides(controller.editor.string, list: list)
        return list.slides.map(\.title)
    }

    /// A pasteboard written by the panel's real writer for `numbers`, as a drag of them would write it.
    func dragPasteboard(from panel: SlidePanelViewController, numbers: [Int]) throws -> NSPasteboard {
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.drag.\(UUID().uuidString)"))
        pasteboard.clearContents()
        panel.select(numbers: numbers, scroll: false)
        let writer = try XCTUnwrap(panel.collectionView(panel.collectionView, pasteboardWriterForItemAt: IndexPath(item: numbers[0] - 1, section: 0)),
                                   "the real pasteboard writer")
        pasteboard.writeObjects([writer])
        return pasteboard
    }

    /// The real validate and accept path of the panel, fed a fake dragging info.
    func drop(_ pasteboard: NSPasteboard, into panel: SlidePanelViewController, beforeIndex index: Int, source: Any?, window: NSWindow?) -> (NSDragOperation, Bool) {
        let info = FakeDraggingInfo(pasteboard: pasteboard, location: .zero, source: source, window: window)
        var proposed = NSIndexPath(forItem: index, inSection: 0)
        var operation = NSCollectionView.DropOperation.on
        let allowed = panel.collectionView(panel.collectionView, validateDrop: info, proposedIndexPath: &proposed, dropOperation: &operation)
        XCTAssertEqual(operation, .before, "a drop always lands between thumbnails")
        guard allowed != [] else { return (allowed, false) }
        let accepted = panel.collectionView(panel.collectionView, acceptDrop: info, indexPath: IndexPath(item: index, section: 0), dropOperation: .before)
        return (allowed, accepted)
    }

    func testMoveOneSlideByDraggingInTheSidebar() async throws {
        let (document, controller) = try await openOps()
        let panel = controller.slidePanel
        let window = document.windowControllers.first?.window
        let payload = try XCTUnwrap(controller.dragPayload(forSlides: [5]))
        XCTAssertEqual(payload.markdowns, ["# Five"])
        XCTAssertTrue(payload.comesFrom(deck: try XCTUnwrap(document.fileURL)))
        XCTAssertEqual(panel.dropDecision(proposedIndex: 2, payload: payload, deck: document.fileURL, optionHeld: false).beforeNumber, 3)
        XCTAssertEqual(panel.dropDecision(proposedIndex: 4, payload: payload, deck: document.fileURL, optionHeld: false).operation, [],
                       "dropping a slide onto its own place is refused")
        XCTAssertEqual(panel.dropDecision(proposedIndex: 7, payload: payload, deck: document.fileURL, optionHeld: false).beforeNumber, nil, "after the last slide")

        let pasteboard = try dragPasteboard(from: panel, numbers: [5])
        let (operation, accepted) = drop(pasteboard, into: panel, beforeIndex: 2, source: panel.collectionView, window: window)
        XCTAssertEqual(operation, .move)
        XCTAssertTrue(accepted)
        let afterDrop = try await roundTrip(controller)
        XCTAssertEqual(afterDrop, ["One", "Two", "Five", "Three", "Four", "Six", "Seven"], "tap parses the moved text in the new order")
        XCTAssertEqual(panel.selectedNumbers, [3])

        document.undoManager?.undo()
        let afterUndo = try await roundTrip(controller)
        XCTAssertEqual(afterUndo, ["One", "Two", "Three", "Four", "Five", "Six", "Seven"], "Cmd+Z restores the old order")
    }

    func testMoveSeveralSlides() async throws {
        let (document, controller) = try await openOps()
        let panel = controller.slidePanel
        let pasteboard = try dragPasteboard(from: panel, numbers: [5, 6])
        XCTAssertEqual(try XCTUnwrap(SlideDragPayload(data: try XCTUnwrap(pasteboard.data(forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType))))).slideNumbers, [5, 6],
                       "dragging one selected thumbnail drags the whole selection")
        let (_, accepted) = drop(pasteboard, into: panel, beforeIndex: 2, source: panel.collectionView, window: document.windowControllers.first?.window)
        XCTAssertTrue(accepted)
        let afterDrop = try await roundTrip(controller)
        XCTAssertEqual(afterDrop, ["One", "Two", "Five", "Six", "Three", "Four", "Seven"])
        XCTAssertEqual(document.undoManager?.undoActionName, "Move 2 Slides")
        XCTAssertEqual(panel.selectedNumbers, [3, 4], "the moved slides stay selected, in their order")
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }

        XCTAssertTrue(controller.moveSelectedSlides(by: -1), "a keyboard move acts on the whole selection")
        let afterKeyboardMove = try await roundTrip(controller)
        XCTAssertEqual(afterKeyboardMove, ["One", "Five", "Six", "Two", "Three", "Four", "Seven"])
        XCTAssertEqual(panel.selectedNumbers, [2, 3])
        document.undoManager?.undo()
        document.undoManager?.undo()
        let afterBothUndos = try await roundTrip(controller)
        XCTAssertEqual(afterBothUndos, ["One", "Two", "Three", "Four", "Five", "Six", "Seven"], "one undo step each")
    }

    func testFrontmatterNeverMoves() async throws {
        let (document, controller) = try await openOps()
        let panel = controller.slidePanel
        let hidden = controller.editor.hiddenLength
        let pasteboard = try dragPasteboard(from: panel, numbers: [4])
        let (_, accepted) = drop(pasteboard, into: panel, beforeIndex: 0, source: panel.collectionView, window: document.windowControllers.first?.window)
        XCTAssertTrue(accepted)
        XCTAssertTrue(controller.editor.string.hasPrefix("---\ntitle: Ops\n---\n\n# Four"), "inserted after the frontmatter, never above it")
        XCTAssertEqual(controller.editor.hiddenLength, hidden)
        let afterDrop = try await roundTrip(controller)
        XCTAssertEqual(afterDrop, ["Four", "One", "Two", "Three", "Five", "Six", "Seven"])
    }

    func testDragSlidesToAnotherDeck() async throws {
        let (source, sourceController) = try await openOps()
        let target = try await openDeck(try Fixtures.copyAppFixture())
        try await waitForBoxes(target, count: 4)
        let targetController = try XCTUnwrap(target.sessionController)
        try await waitUntil(timeout: 5, "the target's answer") { targetController.lastAppliedText == targetController.editor.string }
        let sourcePanel = sourceController.slidePanel
        let targetPanel = targetController.slidePanel

        let payload = try XCTUnwrap(sourceController.dragPayload(forSlides: [5, 6]))
        XCTAssertEqual(targetPanel.dropDecision(proposedIndex: 4, payload: payload, deck: target.fileURL, optionHeld: false).operation, .move,
                       "into another deck, a drop moves")
        XCTAssertEqual(targetPanel.dropDecision(proposedIndex: 4, payload: payload, deck: target.fileURL, optionHeld: true).operation, .copy,
                       "with Option held, it copies")

        let pasteboard = try dragPasteboard(from: sourcePanel, numbers: [5, 6])
        let (operation, accepted) = drop(pasteboard, into: targetPanel, beforeIndex: 4, source: sourcePanel.collectionView, window: target.windowControllers.first?.window)
        XCTAssertEqual(operation, .move)
        XCTAssertTrue(accepted)
        let targetAfterMove = try await roundTrip(targetController)
        XCTAssertEqual(targetAfterMove, ["App Mode Fixture", "Live Code", "Fragments", "Counter", "Five", "Six"])
        let sourceAfterMove = try await roundTrip(sourceController)
        XCTAssertEqual(sourceAfterMove, ["One", "Two", "Three", "Four", "Seven"], "the source loses them")
        XCTAssertEqual(target.undoManager?.undoActionName, "Insert 2 Slides", "one undo step in the target")
        XCTAssertEqual(source.undoManager?.undoActionName, "Delete 2 Slides", "and one in the source")
        source.undoManager?.undo()
        let sourceAfterUndo = try await roundTrip(sourceController)
        XCTAssertEqual(sourceAfterUndo.count, 7)

        targetPanel.optionHeld = { true }
        let copied = try dragPasteboard(from: sourcePanel, numbers: [1])
        let (copyOperation, copyAccepted) = drop(copied, into: targetPanel, beforeIndex: 0, source: sourcePanel.collectionView, window: target.windowControllers.first?.window)
        XCTAssertEqual(copyOperation, .copy)
        XCTAssertTrue(copyAccepted)
        let targetAfterCopy = try await roundTrip(targetController)
        XCTAssertEqual(targetAfterCopy, ["One", "App Mode Fixture", "Live Code", "Fragments", "Counter", "Five", "Six"])
        let sourceAfterCopy = try await roundTrip(sourceController)
        XCTAssertEqual(sourceAfterCopy.count, 7, "a copy leaves the source alone")
    }

    /// A move into a deck whose typing tap has not confirmed queues the
    /// insert there. The source keeps its slides until the insert lands,
    /// and keeps them for good when the insert is abandoned (no answer
    /// from tap in time, as while it restarts): otherwise the slides would
    /// survive only in the source's undo stack.
    func testAMoveWaitsForTheTargetsInsert() async throws {
        let (_, sourceController) = try await openOps()
        let target = try await openDeck(try Fixtures.copyAppFixture())
        try await waitForBoxes(target, count: 4)
        let targetController = try XCTUnwrap(target.sessionController)
        try await waitUntil(timeout: 5, "the target's answer") { targetController.lastAppliedText == targetController.editor.string }
        let targetWindow = target.windowControllers.first?.window
        func typeInTarget(_ text: String) {
            let end = (targetController.editor.string as NSString).length
            targetController.editor.setSelectedRange(NSRange(location: end, length: 0))
            targetController.editor.insertText(text, replacementRange: NSRange(location: NSNotFound, length: 0))
            XCTAssertNotEqual(targetController.editor.string, targetController.lastAppliedText, "tap has not answered for the typing yet")
        }

        // The insert lands: only then does the source give the slides up.
        typeInTarget("\n\nMore.")
        let sourceText = sourceController.editor.string
        let pasteboard = try dragPasteboard(from: sourceController.slidePanel, numbers: [5, 6])
        let (operation, accepted) = drop(pasteboard, into: targetController.slidePanel, beforeIndex: 4, source: sourceController.slidePanel.collectionView, window: targetWindow)
        XCTAssertEqual(operation, .move)
        XCTAssertTrue(accepted, "the drop is taken, queued behind the target's confirmation")
        XCTAssertEqual(sourceController.editor.string, sourceText, "the source keeps its slides while the target's insert waits")
        try await waitUntil(timeout: 10, "the target's insert to land") { targetController.editor.boxes.count == 6 }
        try await waitUntil(timeout: 10, "the source's delete") { sourceController.editor.boxes.count == 5 }
        let targetAfterInsert = try await roundTrip(targetController)
        XCTAssertEqual(targetAfterInsert, ["App Mode Fixture", "Live Code", "Fragments", "Counter", "Five", "Six"])
        let sourceAfterDelete = try await roundTrip(sourceController)
        XCTAssertEqual(sourceAfterDelete, ["One", "Two", "Three", "Four", "Seven"])
        try await waitUntil(timeout: 10, "both answers") {
            targetController.lastAppliedText == targetController.editor.string && sourceController.lastAppliedText == sourceController.editor.string
        }

        // The insert is abandoned: tap stops answering for the target.
        targetController.sourceSync.sender = nil
        targetController.confirmationTimeout = 0.3
        typeInTarget("\n\nMore again.")
        let sourceBefore = sourceController.editor.string
        let targetBefore = targetController.editor.string
        let again = try dragPasteboard(from: sourceController.slidePanel, numbers: [1])
        let (_, acceptedAgain) = drop(again, into: targetController.slidePanel, beforeIndex: 0, source: sourceController.slidePanel.collectionView, window: targetWindow)
        XCTAssertTrue(acceptedAgain)
        XCTAssertEqual(sourceController.editor.string, sourceBefore, "nothing leaves the source while the insert waits")
        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertEqual(targetController.editor.string, targetBefore, "the target's insert was abandoned")
        XCTAssertEqual(sourceController.editor.string, sourceBefore, "an abandoned insert deletes nothing from the source")
        let sourceAfterAbandoned = try await roundTrip(sourceController)
        XCTAssertEqual(sourceAfterAbandoned, ["One", "Two", "Three", "Four", "Seven"])
    }

    /// A pasteboard built with a payload's raw data, bypassing the panel's
    /// own writer: the tests that pin the staleness guards need a payload
    /// whose slide numbers no longer match what tap now counts, which the
    /// real writer would never produce (it always asks for the current text).
    func rawPasteboard(_ payload: SlideDragPayload, name: String) throws -> NSPasteboard {
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.\(name).\(UUID().uuidString)"))
        pasteboard.clearContents()
        let item = NSPasteboardItem()
        item.setData(try payload.data(), forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType))
        pasteboard.writeObjects([item])
        return pasteboard
    }

    /// The data-loss guard `removeMovedSlides` relies on: a drag's payload
    /// is built from the numbers and text at drag start, but typing since
    /// then, once tap answers, can renumber the slides those numbers now
    /// name. Dropping the stale payload into another deck must still leave
    /// the source with everything it had, never delete whatever now holds
    /// the old numbers.
    func testAStaleCrossDeckMoveKeepsAllOfTheSourcesSlides() async throws {
        let (_, sourceController) = try await openOps()
        let target = try await openDeck(try Fixtures.copyAppFixture())
        try await waitForBoxes(target, count: 4)
        let targetController = try XCTUnwrap(target.sessionController)
        try await waitUntil(timeout: 5, "the target's answer") { targetController.lastAppliedText == targetController.editor.string }

        let payload = try XCTUnwrap(sourceController.dragPayload(forSlides: [5, 6]))
        XCTAssertEqual(payload.markdowns.count, 2)

        // Renumber the source: slide 1 duplicated pushes every later slide
        // up by one, so the numbers [5, 6] the payload was built from now
        // name different slides ("Four", "Five").
        XCTAssertEqual(sourceController.perform(.duplicate(numbers: [1])), .applied)
        try await waitUntil(timeout: 10, "tap's answer for the duplicate") { sourceController.lastAppliedText == sourceController.editor.string }
        XCTAssertNotEqual(sourceController.markdown(forSlides: [5, 6]), payload.markdowns, "the numbers now name different slides")

        let pasteboard = try rawPasteboard(payload, name: "stale-cross-deck")
        let (operation, accepted) = drop(pasteboard, into: targetController.slidePanel, beforeIndex: 4, source: sourceController.slidePanel.collectionView, window: target.windowControllers.first?.window)
        XCTAssertEqual(operation, .move)
        XCTAssertTrue(accepted, "the insert into the target still lands, with the text the payload carried")
        try await waitUntil(timeout: 10, "the target's insert to land") { targetController.editor.boxes.count == 6 }
        let targetAfterInsert = try await roundTrip(targetController)
        XCTAssertEqual(targetAfterInsert, ["App Mode Fixture", "Live Code", "Fragments", "Counter", "Five", "Six"])

        // The source's delete never runs: the numbers no longer hold the
        // dragged text, so the drop acted as a copy, not a move.
        try await Task.sleep(nanoseconds: 500_000_000)
        let sourceAfterDrop = try await roundTrip(sourceController)
        XCTAssertEqual(sourceAfterDrop.count, 8, "the source keeps all its slides")
        XCTAssertEqual(sourceAfterDrop, ["One", "One", "Two", "Three", "Four", "Five", "Six", "Seven"])
    }

}
