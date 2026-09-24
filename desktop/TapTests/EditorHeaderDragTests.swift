import XCTest
@testable import Tap

final class EditorHeaderDragTests: HostedTestCase {
    func openOpsLaidOut() async throws -> (DeckDocument, DeckSessionController, EditorTextView) {
        let document = try await openDeck(try Fixtures.copyDeck("ops.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        let editor = controller.editor
        editor.layoutSubtreeIfNeeded()
        editor.textLayoutManager?.ensureLayout(for: editor.textLayoutManager!.documentRange)
        editor.needsDisplay = true
        editor.displayIfNeeded()
        return (document, controller, editor)
    }

    func mouseEvent(_ type: NSEvent.EventType, at point: NSPoint, in editor: EditorTextView) throws -> NSEvent {
        let window = try XCTUnwrap(editor.window)
        return try XCTUnwrap(NSEvent.mouseEvent(with: type, location: editor.convert(point, to: nil), modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                                windowNumber: window.windowNumber, context: nil, eventNumber: 0, clickCount: 1, pressure: 1))
    }

    func testMoveByDraggingABoxHeaderInTheEditor() async throws {
        let (document, controller, editor) = try await openOpsLaidOut()
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        let header5 = try XCTUnwrap(editor.headerRect(forBoxAt: 4), "box 5 is on screen with a header")
        XCTAssertEqual(editor.boxIndex(forHeaderAt: NSPoint(x: header5.midX, y: header5.midY)), 4)
        XCTAssertNil(editor.boxIndex(forHeaderAt: NSPoint(x: header5.midX, y: header5.maxY + EditorTextView.lineHeight)), "the body of a box is not its header")

        // A mouse down on the header followed by a drag past the threshold starts a slide drag.
        // A leftMouseUp is queued right behind the drag: production code
        // consumes only the drag and never reaches it, but if mouseDown
        // were to call super unconditionally, NSTextView's own tracking
        // loop would otherwise wait forever for a mouse up that never
        // comes, hanging the test instead of failing it.
        var started: [SlideDragPayload] = []
        editor.headerDragStarter = { payload, _, _ in started.append(payload) }
        let down = NSPoint(x: header5.midX, y: header5.midY)
        NSApp.postEvent(try mouseEvent(.leftMouseDragged, at: NSPoint(x: down.x + 12, y: down.y), in: editor), atStart: false)
        NSApp.postEvent(try mouseEvent(.leftMouseUp, at: down, in: editor), atStart: false)
        editor.mouseDown(with: try mouseEvent(.leftMouseDown, at: down, in: editor))
        XCTAssertEqual(started.map(\.slideNumbers), [[5]], "the drag carries slide 5")
        NSApp.discardEvents(matching: .leftMouseUp, before: nil)

        // A mouse down and up on a header with no drag is a click, which places the caret in that slide.
        editor.setSelectedRange(NSRange(location: 0, length: 0))
        NSApp.postEvent(try mouseEvent(.leftMouseUp, at: down, in: editor), atStart: false)
        editor.mouseDown(with: try mouseEvent(.leftMouseDown, at: down, in: editor))
        XCTAssertEqual(started.count, 1, "no second drag")
        XCTAssertEqual(editor.currentBoxIndex, 4, "the click put the caret in slide 5")

        // The drop, through the real drag destination methods.
        let box3 = try XCTUnwrap(editor.boxRect(forBoxAt: 2))
        let above3 = NSPoint(x: box3.midX, y: box3.minY + 4)
        XCTAssertEqual(editor.dropBoundary(at: above3), 3, "the upper half of box 3 means above slide 3")
        XCTAssertEqual(editor.dropBoundary(at: NSPoint(x: box3.midX, y: box3.maxY - 4)), 4, "the lower half means below it")
        XCTAssertEqual(editor.dropBoundary(at: NSPoint(x: box3.midX, y: -100)), 1, "above everything is above slide 1")

        let payload = try XCTUnwrap(controller.dragPayload(forSlides: [5]))
        let pasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.headerdrag.\(UUID().uuidString)"))
        pasteboard.clearContents()
        let item = NSPasteboardItem()
        item.setData(try payload.data(), forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType))
        pasteboard.writeObjects([item])
        let info = FakeDraggingInfo(pasteboard: pasteboard, location: editor.convert(above3, to: nil), source: editor, window: window)
        XCTAssertEqual(editor.draggingUpdated(info), .move)
        XCTAssertEqual(editor.dropIndicatorBeforeNumber, 3, "the indicator follows the pointer")
        XCTAssertTrue(editor.performDragOperation(info))
        editor.draggingEnded(info)
        XCTAssertNil(editor.dropIndicatorBeforeNumber, "the indicator is gone after the drop")
        let list = try await TapSlideList.list(text: editor.string)
        XCTAssertEqual(list.slides.map(\.title), ["One", "Two", "Five", "Three", "Four", "Six", "Seven"], "the same result as dragging in the sidebar")
        TapSlideList.assertOneSeparatorBetweenSlides(editor.string, list: list)

        // From another deck, a drop moves the slides there too, and Option copies.
        editor.optionHeld = { true }
        let other = SlideDragPayload(deckPath: "/tmp/elsewhere/talk.md", slideNumbers: [1], markdowns: ["# Elsewhere"])
        let otherPasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.headerdrag.\(UUID().uuidString)"))
        otherPasteboard.clearContents()
        let otherItem = NSPasteboardItem()
        otherItem.setData(try other.data(), forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType))
        otherPasteboard.writeObjects([otherItem])
        let otherInfo = FakeDraggingInfo(pasteboard: otherPasteboard, location: editor.convert(NSPoint(x: box3.midX, y: -100), to: nil), source: nil, window: window)
        XCTAssertEqual(editor.draggingUpdated(otherInfo), .copy, "Option held: a copy")
        editor.optionHeld = { false }
        XCTAssertEqual(editor.draggingUpdated(otherInfo), .move, "otherwise a move, as in the sidebar")
        editor.draggingExited(otherInfo)
        XCTAssertNil(editor.dropIndicatorBeforeNumber)
    }

    func testTheDropIndicatorHasALabel() async throws {
        let (_, _, editor) = try await openOpsLaidOut()
        let box2 = try XCTUnwrap(editor.boxRect(forBoxAt: 1))
        editor.updateDropIndicator(at: NSPoint(x: box2.midX, y: box2.minY + 2), count: 2)
        XCTAssertEqual(editor.dropIndicatorBeforeNumber, 2)
        let element = try XCTUnwrap(editor.accessibilityChildren()?.compactMap { $0 as? NSAccessibilityElement }.first { $0.accessibilityIdentifier() == "drop-indicator" })
        XCTAssertEqual(element.accessibilityLabel(), "Drop 2 slides above slide 2")
        editor.clearDropIndicator()
        XCTAssertNil(editor.dropIndicatorBeforeNumber)
    }
}
