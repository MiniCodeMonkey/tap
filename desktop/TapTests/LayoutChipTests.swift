import XCTest
@testable import Tap

final class LayoutChipTests: HostedTestCase {
    func openDeck(_ name: String, boxes: Int) async throws -> (DeckDocument, DeckSessionController) {
        let document = try await openDeck(try Fixtures.copyDeck(name))
        try await waitForBoxes(document, count: boxes)
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

    func openOps() async throws -> (DeckDocument, DeckSessionController) {
        try await openDeck("ops.md", boxes: 7)
    }

    func slideText(_ editor: EditorTextView, _ number: Int) -> String {
        (editor.string as NSString).substring(with: editor.boxes[number - 1].range)
    }

    /// The gallery a click on box `index`'s chip opens.
    func openGallery(_ document: DeckDocument, _ controller: DeckSessionController, box index: Int) throws -> LayoutGalleryController {
        controller.editor.showLayoutPicker(forBoxAt: index)
        let gallery = try XCTUnwrap(document.windowControllers.first as? DeckWindowController).layoutGallery
        XCTAssertTrue(gallery.isShown, "the chip of box \(index + 1) opens the gallery")
        return gallery
    }

    func testTheGalleryOffersAutomaticAndTapsLayoutsWithTheDeclaredOneSelected() async throws {
        let (document, controller) = try await openOps()
        let gallery = try openGallery(document, controller, box: 1)
        let templates = AppEnvironment.shared.layoutCatalog.templates
        XCTAssertEqual(gallery.sections.count, 1, "no component, no section of its own")
        let entries = try XCTUnwrap(gallery.sections.first)
        XCTAssertEqual(entries.first?.choice, .automatic)
        XCTAssertEqual(entries.first?.detail, "Title", "tap renders slide 2 with the title layout")
        XCTAssertEqual(entries.dropFirst().map(\.choice), templates.map { .layout($0.name) })
        XCTAssertEqual(gallery.selectedChoice, .layout("title"), "slide 2 declares the title layout")
        XCTAssertEqual(gallery.footerLabel.stringValue, "Changes the layout of slide 2")
        gallery.close()

        let undeclared = try openGallery(document, controller, box: 0)
        XCTAssertEqual(undeclared.selectedChoice, .automatic, "a slide that declares none is Automatic, whatever tap detects")
        undeclared.close()
    }

    func testChangeASlideSLayoutFromItsHeader() async throws {
        let (document, controller) = try await openOps()
        let editor = controller.editor
        let original = editor.string
        let before = (1...7).map { slideText(editor, $0) }
        let gallery = try openGallery(document, controller, box: 1)
        gallery.pick(.layout("big-stat"))
        XCTAssertFalse(gallery.isShown)
        XCTAssertTrue(slideText(editor, 2).contains("layout: big-stat"))
        XCTAssertFalse(slideText(editor, 2).contains("layout: title"))
        for number in [1, 3, 4, 5, 6, 7] { XCTAssertEqual(slideText(editor, number), before[number - 1], "slide \(number) is untouched") }
        XCTAssertEqual(document.undoManager?.undoActionName, "Change Layout")
        let list = try await TapSlideList.list(text: editor.string)
        XCTAssertEqual(list.slides[1].layout, "big-stat")

        document.undoManager?.undo()
        XCTAssertEqual(editor.string, original, "one undo restores the text")
    }

    /// The person's selection stays where it was, shifted only by the edit's length change.
    func testChangingAnotherSlidesLayoutKeepsTheSelection() async throws {
        let (document, controller) = try await openOps()
        let editor = controller.editor
        // A range in slide 5 (after the edited slide), and the caret in slide 1 (before it).
        let inFive = NSRange(location: editor.boxes[4].range.location + 2, length: 1)
        editor.setSelectedRange(inFive)
        let textBefore = editor.string as NSString
        let selectedText = textBefore.substring(with: inFive)
        try openGallery(document, controller, box: 1).pick(.layout("big-stat"))
        XCTAssertEqual((editor.string as NSString).substring(with: editor.selectedRange()), selectedText, "the same characters stay selected")
        XCTAssertEqual(editor.currentBoxIndex, 4)

        let caretInOne = editor.boxes[0].range.location + 2
        editor.setSelectedRange(NSRange(location: caretInOne, length: 0))
        try openGallery(document, controller, box: 2).pick(.layout("quote"))
        XCTAssertEqual(editor.selectedRange(), NSRange(location: caretInOne, length: 0), "an edit after the caret leaves it alone")
    }

    func testAutomaticRemovesTheLayoutDeclaration() async throws {
        let (document, controller) = try await openOps()
        let editor = controller.editor
        let gallery = try openGallery(document, controller, box: 2)
        XCTAssertEqual(gallery.sections.first?.first?.detail, "Section")
        gallery.pick(.automatic)
        XCTAssertFalse(slideText(editor, 3).contains("layout:"))
        XCTAssertTrue(slideText(editor, 3).contains("# Three"))
    }

    func testAutomaticOnASlideThatDeclaresNothingChangesNothing() async throws {
        let (document, controller) = try await openOps()
        let editor = controller.editor
        let original = editor.string
        let gallery = try openGallery(document, controller, box: 3)
        XCTAssertEqual(gallery.sections.first?.first?.detail, "Code Focus")
        gallery.pick(.automatic)
        XCTAssertEqual(editor.string, original)
        XCTAssertNotEqual(document.undoManager?.undoActionName, "Change Layout")
    }

    func testDefaultIsDeclaredLikeAnyOtherLayout() async throws {
        let (document, controller) = try await openOps()
        let editor = controller.editor
        try openGallery(document, controller, box: 3).pick(.layout("default"))
        XCTAssertTrue(slideText(editor, 4).contains("layout: default"))
        let reopened = try openGallery(document, controller, box: 3)
        XCTAssertEqual(reopened.selectedChoice, .layout("default"))
        reopened.close()
    }

    /// The gallery names its slide when it opens; a pick made after typing that tap has not answered
    /// waits for the answer and then acts on the slide it named, not on whatever holds the number.
    func testAPickQueuedBehindTypingActsOnTheSlideItNamed() async throws {
        let (document, controller) = try await openOps()
        let editor = controller.editor
        let gallery = try openGallery(document, controller, box: 2)
        let unchanged = slideText(editor, 4)
        editor.setSelectedRange(NSRange(location: editor.boxes[0].range.location, length: 0))
        editor.insertText("x", replacementRange: NSRange(location: NSNotFound, length: 0))
        gallery.pick(.layout("quote"))
        try await waitUntil(timeout: 10, "the pick to land") { self.slideText(editor, 3).contains("layout: quote") }
        XCTAssertEqual(slideText(editor, 4), unchanged)
        XCTAssertTrue(slideText(editor, 1).hasPrefix("x"), "the typed text is kept")
    }

    func testClickingTheLayoutNameOpensTheGalleryAndHoverShowsTheChip() async throws {
        let (document, controller) = try await openOps()
        let editor = controller.editor
        let windowController = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        let window = try XCTUnwrap(windowController.window)
        let chip = try XCTUnwrap(editor.layoutChipRect(forBoxAt: 4), "box 5 is on screen with a layout name")
        let point = NSPoint(x: chip.midX, y: chip.midY)
        let location = editor.convert(point, to: nil)
        func event(_ type: NSEvent.EventType) throws -> NSEvent {
            try XCTUnwrap(NSEvent.mouseEvent(with: type, location: location, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                             windowNumber: window.windowNumber, context: nil, eventNumber: 0, clickCount: 1, pressure: 1))
        }
        editor.mouseMoved(with: try event(.mouseMoved))
        XCTAssertEqual(editor.hoveredLayoutBoxIndex, 4, "the pointer over a quiet layout name shows the chip look")
        let exit = try XCTUnwrap(NSEvent.enterExitEvent(with: .mouseExited, location: location, modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                                        windowNumber: window.windowNumber, context: nil, eventNumber: 0, trackingNumber: 0, userData: nil))
        editor.mouseExited(with: exit)
        XCTAssertNil(editor.hoveredLayoutBoxIndex)

        editor.mouseDown(with: try event(.leftMouseDown))
        XCTAssertTrue(windowController.layoutGallery.isShown, "a click on the layout name opens the gallery")
        XCTAssertEqual(windowController.layoutGallery.footerLabel.stringValue, "Changes the layout of slide 5")
        windowController.layoutGallery.close()
    }

    func testTheLayoutPopUpIsAccessible() async throws {
        let (_, controller) = try await openOps()
        let popUp = try XCTUnwrap((controller.editor.accessibilityChildren() ?? []).compactMap { $0 as? NSAccessibilityElement }
            .first { $0.accessibilityIdentifier() == "layout-2" })
        XCTAssertEqual(popUp.accessibilityRole(), .popUpButton)
        XCTAssertEqual(popUp.accessibilityLabel(), "Layout")
        XCTAssertEqual(popUp.accessibilityValue() as? String, "Title")
    }

    func testChangeAComponentSlideSLayoutFromItsHeader() async throws {
        let (document, controller) = try await openDeck("stepped", boxes: 2)
        let editor = controller.editor
        let index = try XCTUnwrap(editor.boxes.firstIndex { slideText(editor, $0.slide.number).contains("RollingDeploy.jsx") })
        XCTAssertNotNil(editor.layoutChipRect(forBoxAt: index), "a component slide has a chip too")
        let original = editor.string

        let gallery = try openGallery(document, controller, box: index)
        XCTAssertEqual(gallery.sections.count, 2, "the component, then Automatic and tap's layouts")
        XCTAssertEqual(gallery.sections[0].map(\.choice), [.component("RollingDeploy.jsx")])
        XCTAssertEqual(gallery.sections[1].first?.choice, .automatic)
        XCTAssertEqual(gallery.selectedChoice, .component("RollingDeploy.jsx"))
        gallery.pick(.component("RollingDeploy.jsx"))
        XCTAssertEqual(editor.string, original, "the component the slide already has changes nothing")

        try openGallery(document, controller, box: index).pick(.layout("default"))
        let changed = slideText(editor, index + 1)
        XCTAssertTrue(changed.contains("layout: default"), changed)
        XCTAssertFalse(changed.contains("RollingDeploy.jsx"), changed)
    }
}
