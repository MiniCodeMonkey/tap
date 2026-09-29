import XCTest
@testable import Tap

final class LayoutMenuTests: HostedTestCase {
    func openOps() async throws -> (DeckDocument, DeckSessionController) {
        let document = try await openDeck(try Fixtures.copyDeck("ops.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 5, "tap's answer for the opened text") { controller.lastAppliedText == controller.editor.string }
        await AppEnvironment.shared.layoutCatalog.load()
        let editor = controller.editor
        editor.layoutSubtreeIfNeeded()
        editor.textLayoutManager?.ensureLayout(for: editor.textLayoutManager!.documentRange)
        editor.needsDisplay = true
        editor.displayIfNeeded()
        return (document, controller)
    }

    func slideText(_ editor: EditorTextView, _ number: Int) -> String {
        (editor.string as NSString).substring(with: editor.boxes[number - 1].range)
    }

    func item(_ menu: NSMenu, _ title: String) throws -> NSMenuItem {
        try XCTUnwrap(menu.items.first { $0.title == title }, "the menu has \(title)")
    }

    func testTheMenuListsTapsLayoutsWithTheCurrentOneChecked() async throws {
        let (_, controller) = try await openOps()
        let editor = controller.editor
        let menu = try XCTUnwrap(controller.editor(editor, layoutMenuForBoxAt: 1))
        let layouts = AppEnvironment.shared.layoutCatalog.templates.map { LayoutCatalog.displayName($0.name) }
        XCTAssertEqual(menu.items.prefix(layouts.count).map(\.title), layouts)
        XCTAssertEqual(menu.items.filter { $0.state == .on }.map(\.title), ["Title"], "slide 2 declares the title layout")
        XCTAssertTrue(menu.items[menu.items.count - 2].isSeparatorItem)
        XCTAssertEqual(menu.items.last?.title, "Show All Layouts\u{2026}")
        let undeclared = try XCTUnwrap(controller.editor(editor, layoutMenuForBoxAt: 0))
        XCTAssertEqual(undeclared.items.filter { $0.state == .on }.map(\.title), ["Default"], "a slide that declares none is Default")
    }

    func testChangeASlideSLayoutFromItsHeader() async throws {
        let (document, controller) = try await openOps()
        let editor = controller.editor
        let original = editor.string
        let before = (1...7).map { slideText(editor, $0) }
        let menu = try XCTUnwrap(controller.editor(editor, layoutMenuForBoxAt: 1))
        controller.chooseLayout(try item(menu, "Big Stat"))
        XCTAssertTrue(slideText(editor, 2).contains("layout: big-stat"))
        XCTAssertFalse(slideText(editor, 2).contains("layout: title"))
        for number in [1, 3, 4, 5, 6, 7] { XCTAssertEqual(slideText(editor, number), before[number - 1], "slide \(number) is untouched") }
        XCTAssertEqual(document.undoManager?.undoActionName, "Change Layout")
        let list = try await TapSlideList.list(text: editor.string)
        XCTAssertEqual(list.slides[1].layout, "big-stat")

        document.undoManager?.undo()
        XCTAssertEqual(editor.string, original, "one undo restores the text")
    }

    func testDefaultRemovesTheLayoutDeclaration() async throws {
        let (_, controller) = try await openOps()
        let editor = controller.editor
        let menu = try XCTUnwrap(controller.editor(editor, layoutMenuForBoxAt: 2))
        controller.chooseLayout(try item(menu, "Default"))
        XCTAssertFalse(slideText(editor, 3).contains("layout:"))
        XCTAssertTrue(slideText(editor, 3).contains("# Three"))
    }

    /// The menu names its slide when it opens; a choice made after typing that tap has not answered
    /// waits for the answer and then acts on the slide it named, not on whatever holds the number.
    func testAChoiceQueuedBehindTypingActsOnTheSlideItNamed() async throws {
        let (_, controller) = try await openOps()
        let editor = controller.editor
        let menu = try XCTUnwrap(controller.editor(editor, layoutMenuForBoxAt: 2))
        let unchanged = slideText(editor, 4)
        editor.setSelectedRange(NSRange(location: editor.boxes[0].range.location, length: 0))
        editor.insertText("x", replacementRange: NSRange(location: NSNotFound, length: 0))
        controller.chooseLayout(try item(menu, "Quote"))
        try await waitUntil(timeout: 10, "the choice to land") { self.slideText(editor, 3).contains("layout: quote") }
        XCTAssertEqual(slideText(editor, 4), unchanged)
        XCTAssertTrue(slideText(editor, 1).hasPrefix("x"), "the typed text is kept")
    }

    func testClickingTheLayoutNameOpensTheMenuAndHoverShowsTheChip() async throws {
        let (document, controller) = try await openOps()
        let editor = controller.editor
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        var shown: [NSMenu] = []
        editor.layoutMenuPresenter = { menu, _ in shown.append(menu) }
        let chip = try XCTUnwrap(editor.layoutChipRect(forBoxAt: 4), "box 5 is on screen with a layout name")
        let point = NSPoint(x: chip.midX, y: chip.midY)
        let location = editor.convert(point, to: nil)
        func event(_ type: NSEvent.EventType) throws -> NSEvent {
            try XCTUnwrap(NSEvent.mouseEvent(with: type, location: location, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                             windowNumber: window.windowNumber, context: nil, eventNumber: 0, clickCount: 1, pressure: 1))
        }
        editor.mouseMoved(with: try event(.mouseMoved))
        XCTAssertEqual(editor.hoveredLayoutBoxIndex, 4, "the pointer over a quiet layout name shows the chip look")
        editor.mouseExited(with: try event(.mouseExited))
        XCTAssertNil(editor.hoveredLayoutBoxIndex)

        editor.mouseDown(with: try event(.leftMouseDown))
        XCTAssertEqual(shown.count, 1, "a click on the layout name opens the menu")
        XCTAssertEqual(shown.first?.items.filter { $0.state == .on }.map(\.title), ["Default"])
    }

    func testTheLayoutPopUpIsAccessible() async throws {
        let (_, controller) = try await openOps()
        let popUp = try XCTUnwrap((controller.editor.accessibilityChildren() ?? []).compactMap { $0 as? NSAccessibilityElement }
            .first { $0.accessibilityIdentifier() == "layout-2" })
        XCTAssertEqual(popUp.accessibilityRole(), .popUpButton)
        XCTAssertEqual(popUp.accessibilityLabel(), "Layout")
        XCTAssertEqual(popUp.accessibilityValue() as? String, "Title")
    }
}
