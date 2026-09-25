import XCTest
@testable import Tap

final class NewSlideTests: HostedTestCase {
    func openOps() async throws -> (DeckDocument, DeckSessionController, DeckWindowController) {
        let document = try await openDeck(try Fixtures.copyDeck("ops.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        await AppEnvironment.shared.layoutCatalog.load()
        return (document, controller, try XCTUnwrap(document.windowControllers.first as? DeckWindowController))
    }

    /// A press on the toolbar's New Slide button, through its real mouse
    /// handling: a short press is a click, a long one a hold.
    func pressNewSlideButton(_ windowController: DeckWindowController, hold: Bool) async throws {
        let button = windowController.newSlideButton
        button.holdDelay = 0.05
        let window = try XCTUnwrap(button.window, "the toolbar has laid the button out")
        let point = button.convert(NSPoint(x: button.bounds.midX, y: button.bounds.midY), to: nil)
        func event(_ type: NSEvent.EventType) throws -> NSEvent {
            try XCTUnwrap(NSEvent.mouseEvent(with: type, location: point, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                             windowNumber: window.windowNumber, context: nil, eventNumber: 0, clickCount: 1, pressure: 1))
        }
        button.mouseDown(with: try event(.leftMouseDown))
        if hold { try await Task.sleep(nanoseconds: 150_000_000) }
        button.mouseUp(with: try event(.leftMouseUp))
    }

    func testInsertASlideWithTheLastLayout() async throws {
        let (document, controller, windowController) = try await openOps()
        AppEnvironment.shared.lastLayout.name = "two-column"
        controller.editor.moveCursor(toSlide: 2)

        try await pressNewSlideButton(windowController, hold: false)
        let template = try await TapTemplate.print(layout: "two-column")
        let text = controller.editor.string as NSString
        XCTAssertEqual(controller.editor.boxes.count, 8)
        XCTAssertEqual(text.substring(with: controller.editor.boxes[3].range), template.trimmingCharacters(in: .whitespacesAndNewlines),
                       "the two-column template is slide 4, after slide 3")
        let list = try await TapSlideList.list(text: controller.editor.string)
        XCTAssertEqual(list.slides.map(\.title), ["One", "Two", "Three", list.slides[3].title, "Four", "Five", "Six", "Seven"])
        TapSlideList.assertOneSeparatorBetweenSlides(controller.editor.string, list: list)
        let selected = controller.editor.selectedRange()
        XCTAssertEqual(text.substring(with: selected), "Left content", "the cursor is in the new slide's first slot, with its placeholder selected")
        XCTAssertEqual(document.undoManager?.undoActionName, "New Slide")
        XCTAssertTrue(windowController.window?.firstResponder === controller.editor, "typing replaces the placeholder")
        XCTAssertFalse(windowController.layoutGallery.isShown, "a click never opens the gallery")

        // The menu item with a layout inserts that layout and makes it the last one.
        let item = NSMenuItem(title: "Quote", action: #selector(DeckWindowController.newSlideFromLayout(_:)), keyEquivalent: "")
        item.representedObject = "quote"
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }
        windowController.newSlideFromLayout(item)
        let quote = try await TapTemplate.print(layout: "quote")
        XCTAssertEqual((controller.editor.string as NSString).substring(with: controller.editor.boxes[4].range), quote.trimmingCharacters(in: .whitespacesAndNewlines))
        XCTAssertEqual(AppEnvironment.shared.lastLayout.name, "quote")
    }

    func testPickALayoutFromTheGallery() async throws {
        let (_, controller, windowController) = try await openOps()
        controller.editor.moveCursor(toSlide: 2)
        try await pressNewSlideButton(windowController, hold: true)
        let gallery = windowController.layoutGallery
        XCTAssertTrue(gallery.isShown, "holding the button opens the gallery")
        XCTAssertEqual(gallery.templates.map(\.name), AppEnvironment.shared.layoutCatalog.templates.map(\.name), "the layouts and their templates come from tap")
        XCTAssertEqual(gallery.templates.count, 12)
        XCTAssertEqual(gallery.templates.first?.name, "title", "in tap's order")
        XCTAssertTrue(gallery.templates.allSatisfy { !LayoutSchematic.elements(for: $0.markdown).isEmpty }, "every cell has a preview")
        XCTAssertEqual(gallery.footerLabel.stringValue, "Inserts after slide 3")

        gallery.pick("big-stat")
        XCTAssertFalse(gallery.isShown)
        let template = try await TapTemplate.print(layout: "big-stat")
        XCTAssertEqual((controller.editor.string as NSString).substring(with: controller.editor.boxes[3].range), template.trimmingCharacters(in: .whitespacesAndNewlines))
        XCTAssertEqual(AppEnvironment.shared.lastLayout.name, "big-stat", "the gallery's pick becomes the last layout")
        XCTAssertEqual((controller.editor.string as NSString).substring(with: controller.editor.selectedRange()), "100%")

        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }
        windowController.showLayoutGallery(nil)
        XCTAssertTrue(gallery.isShown, "Slide > New Slide from Layout > Show Layout Gallery opens it too")
        gallery.close()
        XCTAssertFalse(gallery.isShown)
        try await pressNewSlideButton(windowController, hold: false)
        XCTAssertEqual(controller.editor.boxes.count, 9)
        XCTAssertEqual((controller.editor.string as NSString).substring(with: controller.editor.boxes[4].range), template.trimmingCharacters(in: .whitespacesAndNewlines),
                       "New Slide now inserts big-stat")
    }

    /// A stored last layout that tap no longer offers falls back to one it
    /// does, from every New Slide entry point.
    func testNewSlideFallsBackFromARetiredLastLayout() async throws {
        let (_, controller, windowController) = try await openOps()
        let catalog = AppEnvironment.shared.layoutCatalog
        XCTAssertNil(catalog.template(named: "retired-layout"))
        let fallback = LayoutCatalog.resolvedName("retired-layout", in: catalog.templates)
        XCTAssertNotNil(catalog.template(named: fallback))
        let template = try await TapTemplate.print(layout: fallback).trimmingCharacters(in: .whitespacesAndNewlines)

        AppEnvironment.shared.lastLayout.name = "retired-layout"
        controller.editor.moveCursor(toSlide: 2)
        windowController.newSlide(nil)
        XCTAssertEqual(controller.editor.boxes.count, 8, "New Slide inserts a layout tap offers")
        XCTAssertEqual((controller.editor.string as NSString).substring(with: controller.editor.boxes[3].range), template)
        XCTAssertEqual(AppEnvironment.shared.lastLayout.name, fallback)

        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }
        AppEnvironment.shared.lastLayout.name = "retired-layout"
        controller.editor.moveCursor(toSlide: 0)
        windowController.newSlideAfter(nil)
        XCTAssertEqual(controller.editor.boxes.count, 9, "New Slide After, the context menu's first item, too")
        XCTAssertEqual((controller.editor.string as NSString).substring(with: controller.editor.boxes[1].range), template)
        XCTAssertEqual(AppEnvironment.shared.lastLayout.name, fallback)
    }

    /// A New Slide queued behind typing tap has not confirmed selects
    /// nothing and records nothing until its slide exists, and an insert
    /// that is abandoned (no answer from tap in time) never becomes the
    /// last layout.
    func testAnAbandonedNewSlideRecordsNoLastLayout() async throws {
        let (_, controller, windowController) = try await openOps()
        XCTAssertNotNil(AppEnvironment.shared.layoutCatalog.template(named: "quote"), "the catalog is loaded, so a refusal cannot pass this test for the wrong reason")
        AppEnvironment.shared.lastLayout.name = "two-column"
        controller.sourceSync.sender = nil
        controller.confirmationTimeout = 0.3
        controller.editor.moveCursor(toSlide: 2)
        controller.editor.insertText(" typed", replacementRange: NSRange(location: NSNotFound, length: 0))
        XCTAssertNotEqual(controller.editor.string, controller.lastAppliedText, "tap has not answered for the typing")
        let typed = controller.editor.string
        let selection = controller.editor.selectedRange()

        let item = NSMenuItem(title: "Quote", action: #selector(DeckWindowController.newSlideFromLayout(_:)), keyEquivalent: "")
        item.representedObject = "quote"
        windowController.newSlideFromLayout(item)
        XCTAssertEqual(AppEnvironment.shared.lastLayout.name, "two-column", "no quote slide exists yet")
        XCTAssertEqual(controller.editor.selectedRange(), selection, "no slot is selected in boxes the insert has not reached")

        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertEqual(controller.editor.string, typed, "the insert was abandoned")
        XCTAssertEqual(AppEnvironment.shared.lastLayout.name, "two-column", "an abandoned insert records no last layout")
        XCTAssertEqual(controller.editor.selectedRange(), selection)
    }
}
