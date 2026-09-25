import XCTest
@testable import Tap

final class SlideMenuTests: HostedTestCase {
    func slideMenu() throws -> NSMenu {
        try XCTUnwrap(NSApp.mainMenu?.items.first { $0.title == "Slide" }?.submenu)
    }

    /// Items are found by action, not title: validation renames Delete and Skip.
    func item(_ menu: NSMenu, _ action: Selector) throws -> NSMenuItem {
        try XCTUnwrap(menu.items.first { $0.action == action }, "no item for \(action)")
    }

    func openOps() async throws -> (DeckDocument, DeckSessionController, DeckWindowController) {
        let document = try await openDeck(try Fixtures.copyDeck("ops.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "the panel") { controller.slidePanel.slides.count == 7 }
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        return (document, controller, try XCTUnwrap(document.windowControllers.first as? DeckWindowController))
    }

    func testSlideMenu() async throws {
        _ = try await openOps()
        await AppEnvironment.shared.layoutCatalog.load()
        let menu = try slideMenu()
        let titles = menu.items.map(\.title)
        for required in ["New Slide", "New Slide from Layout", "Duplicate", "Delete", "Move Up", "Move Down", "Generate Image…", "New Component…", "Go to Slide…"] {
            XCTAssertTrue(titles.contains(required), "\(required) is missing from \(titles)")
        }
        let newSlide = try item(menu, #selector(DeckWindowController.newSlide(_:)))
        XCTAssertEqual(newSlide.keyEquivalent, "n")
        XCTAssertEqual(newSlide.keyEquivalentModifierMask, [.command, .option])
        let layouts = try XCTUnwrap(menu.items.first { $0.title == "New Slide from Layout" }?.submenu)
        layouts.delegate?.menuNeedsUpdate?(layouts)
        XCTAssertEqual(layouts.items.compactMap { $0.representedObject as? String }, AppEnvironment.shared.layoutCatalog.templates.map(\.name), "with layouts, from tap")
        XCTAssertEqual(layouts.items.first?.title, "Title")
        XCTAssertEqual(layouts.items.last?.action, #selector(DeckWindowController.showLayoutGallery(_:)))
        XCTAssertEqual(try item(menu, #selector(DeckWindowController.moveSlidesUp(_:))).keyEquivalentModifierMask, [.command, .option])
        XCTAssertEqual(try item(menu, #selector(DeckWindowController.duplicateSlides(_:))).keyEquivalent, "d")
        let delete = try item(menu, #selector(DeckWindowController.deleteSlides(_:)))
        XCTAssertEqual(delete.keyEquivalent, "\u{8}")
        XCTAssertEqual(delete.keyEquivalentModifierMask, [.command], "Command+Delete, so Backspace stays the editor's")
        XCTAssertNil(menu.items.first { $0.title == "Generate Image…" }?.action, "disabled until the image commands arrive")
        XCTAssertNil(menu.items.first { $0.title == "New Component…" }?.action)
    }

    func testDeleteIsDisabledWhenEverySlideIsSelected() async throws {
        let (_, controller, windowController) = try await openOps()
        let item = NSMenuItem(title: "", action: #selector(DeckWindowController.deleteSlides(_:)), keyEquivalent: "")
        controller.slidePanel.select(numbers: [2, 3], scroll: false)
        XCTAssertTrue(windowController.validateMenuItem(item))
        XCTAssertEqual(item.title, "Delete 2 Slides")
        controller.slidePanel.select(numbers: Array(1...7), scroll: false)
        XCTAssertFalse(windowController.validateMenuItem(item), "a deck keeps at least one slide")
        windowController.deleteSlides(nil)
        XCTAssertEqual(controller.editor.boxes.count, 7)
    }

    func testCommandDeleteDeletesTheSlideTheCaretIsInBeforeTapRenumbers() async throws {
        let (_, controller, windowController) = try await openOps()
        let end = NSMaxRange(controller.editor.boxes[2].range)
        controller.editor.setSelectedRange(NSRange(location: end, length: 0))
        controller.editor.insertText("\n\n---\n\n# Split", replacementRange: NSRange(location: NSNotFound, length: 0))
        XCTAssertNotEqual(controller.editor.string, controller.lastAppliedText, "tap has not answered for the typing")
        windowController.deleteSlides(nil)
        try await waitUntil(timeout: 10, "the queued delete to land and tap to answer") {
            controller.lastAppliedText == controller.editor.string && controller.editor.boxes.count == 7
        }
        let titles = try await TapSlideList.list(text: controller.editor.string).slides.map(\.title)
        XCTAssertEqual(titles, ["One", "Two", "Three", "Four", "Five", "Six", "Seven"], "the caret was in Split, so Split goes")
    }

    func testMoveToTopAndBottomAndSkipFromTheMenu() async throws {
        let (_, controller, windowController) = try await openOps()
        controller.editor.moveCursor(toSlide: 2)
        windowController.moveSlidesToBottom(nil)
        let afterBottom = try await TapSlideList.list(text: controller.editor.string)
        XCTAssertEqual(afterBottom.slides.map(\.title), ["One", "Two", "Four", "Five", "Six", "Seven", "Three"])
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }
        windowController.moveSlidesToTop(nil)
        let afterTop = try await TapSlideList.list(text: controller.editor.string)
        XCTAssertEqual(afterTop.slides.map(\.title), ["Three", "One", "Two", "Four", "Five", "Six", "Seven"])
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }

        let skip = NSMenuItem(title: "", action: #selector(DeckWindowController.toggleSkipSlides(_:)), keyEquivalent: "")
        XCTAssertTrue(windowController.validateMenuItem(skip))
        XCTAssertEqual(skip.title, "Skip Slide")
        windowController.toggleSkipSlides(nil)
        let afterSkip = try await TapSlideList.list(text: controller.editor.string)
        XCTAssertTrue(afterSkip.slides[0].skip)
        try await waitUntil(timeout: 10, "tap's answer") { controller.editor.boxes[0].slide.skip }
        XCTAssertTrue(windowController.validateMenuItem(skip))
        XCTAssertEqual(skip.title, "Unskip Slide")
    }

    func testVoiceOver() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        let windowController = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        try await waitUntil(timeout: 5, "the panel") { controller.slidePanel.slides.count == 7 }
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        controller.editor.layoutSubtreeIfNeeded()

        XCTAssertEqual(controller.slidePanel.item(forSlide: 3)?.view.accessibilityLabel(), "Slide 3, default layout, What We Knew, 1 step")
        let boxElements = try XCTUnwrap(controller.editor.accessibilityChildren()).compactMap { $0 as? NSAccessibilityElement }
        XCTAssertEqual(boxElements.first { $0.accessibilityIdentifier() == "box-3" }?.accessibilityLabel(), "Slide 3, default layout, What We Knew, 1 step")
        XCTAssertEqual(SlideAccessibility.dropLabel(beforeNumber: 3, count: 1), "Drop 1 slide above slide 3")

        // Every slide operation from the keyboard alone: a menu item with a key, or a menu item at all.
        let menu = try slideMenu()
        for action in [#selector(DeckWindowController.newSlide(_:)), #selector(DeckWindowController.duplicateSlides(_:)),
                       #selector(DeckWindowController.deleteSlides(_:)), #selector(DeckWindowController.moveSlidesUp(_:)),
                       #selector(DeckWindowController.moveSlidesDown(_:))] {
            XCTAssertFalse(try item(menu, action).keyEquivalent.isEmpty, "\(action) has a shortcut")
        }
        for action in [#selector(DeckWindowController.toggleSkipSlides(_:)), #selector(DeckWindowController.moveSlidesToTop(_:)),
                       #selector(DeckWindowController.moveSlidesToBottom(_:))] {
            XCTAssertNotNil(try item(menu, action), "\(action) is reachable through the menu bar")
        }
        controller.editor.moveCursor(toSlide: 2)
        windowController.moveSlidesUp(nil)
        XCTAssertEqual(controller.editor.boxes[1].slide.title, "What We Knew", "Cmd+Option+Up moved the slide")
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == controller.editor.string }
        windowController.moveSlidesDown(nil)
        XCTAssertEqual(controller.editor.boxes[2].slide.title, "What We Knew")
    }
}
