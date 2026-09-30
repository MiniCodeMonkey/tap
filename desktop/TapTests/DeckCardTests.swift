import XCTest
@testable import Tap

/// The Deck card at the top of the editor: closed it sums the deck up,
/// open it is a form from tap's schema, or the frontmatter's own lines as
/// text. Each change is one undo step through the editor.
final class DeckCardTests: HostedTestCase {
    /// The schema, loaded once per process; the load is a tap run with its own timeout.
    func loadedSchema() async throws -> [SchemaKey] {
        Task { await AppEnvironment.shared.deckSchema.load() }
        try await waitUntil(timeout: 30, "tap deck schema --json") { AppEnvironment.shared.deckSchema.isLoaded }
        return AppEnvironment.shared.deckSchema.keys
    }

    func openOnTheDeckCard(_ name: String = "seven-slides.md", slides: Int = 7) async throws -> (DeckDocument, DeckSessionController, DeckWindowController) {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck(name))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: slides)
        let deckWindow = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        deckWindow.showDeckSettings(nil)
        return (document, controller, deckWindow)
    }

    func chipTexts(_ controller: DeckSessionController) -> [String] {
        controller.deckCard.cardView.chipViews.map(\.label.stringValue)
    }

    func testTheDeckCardIsTheOneThingThatFolds() async throws {
        _ = try await loadedSchema()
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("seven-slides.md"))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 7)
        let editor = controller.editor
        let card = controller.deckCard
        let deck = try XCTUnwrap(document.fileURL)

        // Closed: one line above slide 1, with the summary.
        XCTAssertEqual(card.display, .collapsed)
        XCTAssertTrue(card.cardView.superview === editor, "the card is part of the editor, above slide 1")
        XCTAssertEqual(card.cardView.frame.minY, EditorTextView.deckCardTop, accuracy: 0.5)
        XCTAssertEqual(card.cardView.frame.height, EditorTextView.deckCardHeaderHeight, accuracy: 0.5, "one line")
        XCTAssertEqual(chipTexts(controller), ["Default", "16:9"], "the theme's name and the aspect ratio, from tap's schema")
        XCTAssertTrue(card.cardView.modeControl.isHidden, "the Form | Text switch shows only when open")
        XCTAssertTrue(card.form.view.isHiddenOrHasHiddenAncestor)
        XCTAssertGreaterThan(editor.hiddenLength, 0, "the frontmatter's lines are not in the flow of the editor")
        XCTAssertEqual(editor.boxes[0].range.location, editor.hiddenLength)
        try await waitUntil(timeout: 10, "slide 1 laid out") { editor.boxRect(forBoxAt: 0) != nil }
        XCTAssertGreaterThanOrEqual(try XCTUnwrap(editor.boxRect(forBoxAt: 0)).minY, card.cardView.frame.maxY,
                                    "slide 1 starts below the card")

        // A click opens it, and it is remembered for the deck.
        card.cardView.disclosureButton.performClick(nil)
        XCTAssertEqual(card.display, .form)
        XCTAssertFalse(card.form.view.isHiddenOrHasHiddenAncestor)
        XCTAssertFalse(card.cardView.modeControl.isHidden)
        XCTAssertGreaterThan(card.cardView.frame.height, EditorTextView.deckCardHeaderHeight, "the body is under the header")
        XCTAssertTrue(AppEnvironment.shared.panelState.isDeckCardOpen(deck: deck))
        XCTAssertGreaterThanOrEqual(try XCTUnwrap(editor.boxRect(forBoxAt: 0)).minY, card.cardView.frame.maxY, "slide 1 moves down with the open card")

        // Space, with the disclosure focused, closes it.
        let window = try XCTUnwrap(editor.window)
        XCTAssertTrue(window.makeFirstResponder(card.cardView.disclosureButton))
        let space = try XCTUnwrap(NSEvent.keyEvent(with: .keyDown, location: .zero, modifierFlags: [], timestamp: 0, windowNumber: window.windowNumber,
                                                   context: nil, characters: " ", charactersIgnoringModifiers: " ", isARepeat: false, keyCode: 49))
        card.cardView.disclosureButton.keyDown(with: space)
        XCTAssertEqual(card.display, .collapsed)
        XCTAssertFalse(AppEnvironment.shared.panelState.isDeckCardOpen(deck: deck))
        XCTAssertNil(editor.frontmatterTextRegion, "closed, the frontmatter's lines stay out of the editor")
    }

    func testDeckSettingsLiveInTheDeckCard() async throws {
        let keys = try await loadedSchema()
        let (_, controller, _) = try await openOnTheDeckCard()
        let editor = controller.editor
        XCTAssertGreaterThan(editor.hiddenLength, 0, "the frontmatter text is hidden from the editor")
        XCTAssertEqual(editor.boxes[0].range.location, editor.hiddenLength, "slide 1 is the first box")
        let form = controller.deckForm
        XCTAssertEqual(controller.deckCard.display, .form)
        XCTAssertFalse(form.view.isHiddenOrHasHiddenAncestor)
        XCTAssertFalse(controller.previewViewController.view.isHidden, "the inspector shows the preview only")
        XCTAssertNil(controller.inspectorViewController.view.descendant(identifiedBy: "inspector-tabs"), "no Preview | Deck switch")

        // One field per frontmatter key tap knows; the fields, types and allowed values come from tap deck schema --json.
        for key in keys where key.isScalar { XCTAssertNotNil(form.field(key.name), "a field for \(key.name)") }
        for key in keys where key.type == "object" {
            for child in key.keys where child.isScalar { XCTAssertNotNil(form.field("\(key.name).\(child.name)"), "a field for \(key.name).\(child.name)") }
        }
        XCTAssertTrue(form.field("slideNumbers") is NSSwitch, "a boolean is a switch, as the DeckTabFields board draws it")
        let title = try XCTUnwrap(form.field("title") as? NSTextField)
        XCTAssertEqual(title.stringValue, "Seven Slides")
        // The Theme row is one button, not a popup; it reads "Default"
        // for a deck that names no theme.
        let themeRow = try XCTUnwrap(form.themeRowButton, "the Theme row is a button, not a popup")
        XCTAssertEqual(themeRow.nameLabel.stringValue, "Default", "the deck sets no theme")

        let transition = try XCTUnwrap(form.field("transition") as? NSPopUpButton, "a string with allowed values is a popup")
        let transitionKey = try XCTUnwrap(keys.first { $0.name == "transition" })
        let defaultItem = form.defaultItemTitle(for: transitionKey)
        XCTAssertEqual(defaultItem, "Default (\(transitionKey.defaultValue ?? ""))")
        XCTAssertEqual(transition.itemTitles, [defaultItem] + transitionKey.values, "the allowed values come from tap, behind the way back to tap's default")
        XCTAssertEqual(transition.titleOfSelectedItem, defaultItem, "the deck sets no transition")

        // Changing a field rewrites that key in the frontmatter as one undo step.
        let original = editor.string
        let hiddenBefore = editor.hiddenLength
        let chosen = try XCTUnwrap(transitionKey.values.last)
        transition.selectItem(withTitle: chosen)
        transition.sendAction(transition.action, to: transition.target)
        XCTAssertTrue(editor.string.hasPrefix("---\ntitle: Seven Slides\ndrivers:\n  sqlite: {}\ntransition: \(chosen)\n---\n"), String(editor.string.prefix(80)))
        XCTAssertEqual(editor.undoManager?.undoActionName, "Change Transition")
        XCTAssertTrue(controller.isContentEdited, "the edited flag follows the content")
        try await waitUntil(timeout: 10, "tap's answer for the new frontmatter") { controller.lastAppliedText == editor.string }
        XCTAssertEqual(editor.deckErrors, [], "tap accepts what the form wrote")
        XCTAssertEqual(editor.hiddenLength, hiddenBefore + ("transition: \(chosen)\n" as NSString).length, "the frontmatter stays hidden, one line longer")
        XCTAssertEqual(editor.boxes[0].range.location, editor.hiddenLength, "slide 1 starts where the hidden frontmatter ends")
        try await waitForTheUndoStepToClose(editor.undoManager)
        // The same value again changes nothing, so it is no undo step: the third undo below finds the transition change.
        transition.sendAction(transition.action, to: transition.target)
        XCTAssertEqual(editor.undoManager?.undoActionName, "Change Transition")
        try await waitForTheUndoStepToClose(editor.undoManager)

        title.stringValue = "Deck: renamed"
        title.sendAction(title.action, to: title.target)
        XCTAssertTrue(editor.string.contains("title: \"Deck: renamed\"\n"), "a value YAML would misread is quoted")
        XCTAssertEqual(editor.undoManager?.undoActionName, "Change Title")
        try await waitForTheUndoStepToClose(editor.undoManager)
        let numbers = try XCTUnwrap(form.field("slideNumbers") as? NSSwitch)
        XCTAssertEqual(numbers.state, .on, "absent, so tap's default")
        numbers.state = .off
        numbers.sendAction(numbers.action, to: numbers.target)
        XCTAssertTrue(editor.string.contains("slideNumbers: false\n"))
        try await waitForTheUndoStepToClose(editor.undoManager)

        // Undo, one change at a time; the form follows the text.
        editor.undoManager?.undo()
        XCTAssertFalse(editor.string.contains("slideNumbers"))
        XCTAssertEqual(numbers.state, .on)
        XCTAssertTrue(editor.string.contains("title: \"Deck: renamed\"\n"), "the first undo takes back the switch alone")
        editor.undoManager?.undo()
        XCTAssertEqual(title.stringValue, "Seven Slides")
        XCTAssertTrue(editor.string.contains("transition: \(chosen)\n"), "the second undo takes back the title alone")
        editor.undoManager?.undo()
        XCTAssertEqual(editor.string, original, "the third undo takes back the transition")
        XCTAssertEqual(transition.titleOfSelectedItem, defaultItem, "the popup follows an undo")
        // Choosing the default item removes the key.
        transition.selectItem(withTitle: chosen)
        transition.sendAction(transition.action, to: transition.target)
        XCTAssertTrue(editor.string.contains("transition: \(chosen)\n"))
        transition.selectItem(withTitle: defaultItem)
        transition.sendAction(transition.action, to: transition.target)
        XCTAssertFalse(editor.string.contains("transition:"), "back to tap's default: the key goes")
        XCTAssertEqual(editor.undoManager?.undoActionName, "Change Transition")
        // Not `isContentEdited` here: the autosave may have written the file in between, and the flag is against the file.

        // The theme keeps its own way back to tap's default: the popover's
        // Default cell. The row follows the text, not a popup, once
        // setTheme lands and once an undo takes it back.
        try await waitUntil(timeout: 20, "the theme catalog") { AppEnvironment.shared.themeImages.catalog != nil }
        controller.setTheme("terminal")
        try await waitUntil(timeout: 20, "tap theme set to land") { themeRow.nameLabel.stringValue == "Terminal" }
        editor.undoManager?.undo()
        try await waitUntil(timeout: 5, "the row follows an undo") { themeRow.nameLabel.stringValue == "Default" }

    }

    /// Every text fragment the editor has drawn: the views TextKit places at the fragments' layout positions.
    func renderedTextFrames(_ editor: EditorTextView) -> [NSRect] {
        func collect(_ view: NSView) -> [NSRect] {
            view.subviews.flatMap { (String(describing: type(of: $0)) == "_NSTextViewportElementView" ? [$0.frame] : []) + collect($0) }
        }
        return collect(editor)
    }

    /// The card reserves exactly its own height, the text is drawn where
    /// the layout puts it, and each box wraps the first line of its own slide.
    func assertTheTextFollowsTheCard(_ controller: DeckSessionController, _ context: String, file: StaticString = #filePath, line: UInt = #line) throws {
        let editor = controller.editor
        let card = controller.deckCard
        editor.layoutSubtreeIfNeeded()
        let cardFrame = card.cardView.frame
        if card.display == .form {
            let fitting = EditorTextView.deckCardHeaderHeight + max(card.form.contentHeight, 64)
            XCTAssertEqual(cardFrame.height, fitting, accuracy: 1, "\(context): the card is as tall as its form fits", file: file, line: line)
        }
        XCTAssertEqual(cardFrame.height, editor.deckCardHeight, accuracy: 0.5, "\(context): the editor reserves the card's height", file: file, line: line)
        let rendered = renderedTextFrames(editor)
        if card.display != .text {
            for frame in rendered {
                XCTAssertGreaterThanOrEqual(frame.minY, cardFrame.maxY - 0.5, "\(context): drawn text \(frame) is under the card \(cardFrame)", file: file, line: line)
            }
        }
        let layoutManager = try XCTUnwrap(editor.textLayoutManager, file: file, line: line)
        let contentManager = try XCTUnwrap(layoutManager.textContentManager, file: file, line: line)
        var checked = 0
        var previousMaxY = cardFrame.maxY
        let visible = editor.visibleRect
        for index in editor.boxes.indices {
            // Every box has its rectangle, whether or not it is on screen: none is empty, none starts above the box before it.
            let rect = try XCTUnwrap(editor.boxRect(forBoxAt: index), "\(context): box \(index + 1) has a rectangle", file: file, line: line)
            XCTAssertGreaterThan(rect.size.height, 0, "\(context): box \(index + 1) \(rect) is not empty", file: file, line: line)
            if card.display != .text { XCTAssertGreaterThanOrEqual(rect.minY, previousMaxY - 0.5, "\(context): box \(index + 1) \(rect) starts below the one before it (\(previousMaxY))", file: file, line: line) }
            previousMaxY = rect.maxY
            guard let location = contentManager.location(contentManager.documentRange.location, offsetBy: editor.boxes[index].range.location) else { continue }
            layoutManager.ensureLayout(for: NSTextRange(location: contentManager.documentRange.location, end: contentManager.location(location, offsetBy: 1) ?? contentManager.documentRange.endLocation)!)
            guard let fragment = layoutManager.textLayoutFragment(for: location), let firstLine = fragment.textLineFragments.first else { continue }
            let origin = editor.textContainerOrigin
            let glyphTop = fragment.layoutFragmentFrame.minY + origin.y
            let glyphRect = NSRect(x: rect.minX, y: glyphTop + firstLine.typographicBounds.minY, width: rect.width, height: firstLine.typographicBounds.height)
            XCTAssertTrue(rect.insetBy(dx: -0.5, dy: -0.5).contains(glyphRect), "\(context): box \(index + 1) \(rect) wraps its first line \(glyphRect)", file: file, line: line)
            if glyphRect.intersects(visible) {
                XCTAssertTrue(rendered.contains { abs($0.minY - glyphTop) < 1.5 }, "\(context): the first line of slide \(index + 1) is drawn at \(glyphTop), not elsewhere", file: file, line: line)
            }
            checked += 1
        }
        XCTAssertGreaterThan(checked, 0, "\(context): a box was checked", file: file, line: line)
    }

    /// A box's rectangle is the same wherever the editor is scrolled: a box
    /// with its top above the visible area, one with its bottom below it,
    /// and one wholly below it, are all measured from their own text.
    func assertBoxesKeepTheirRectanglesWhileScrolling(_ controller: DeckSessionController, file: StaticString = #filePath, line: UInt = #line) throws {
        let editor = controller.editor
        let clip = controller.editorViewController.scrollView.contentView
        clip.scroll(to: .zero)
        controller.editorViewController.scrollView.reflectScrolledClipView(clip)
        let reference = try editor.boxes.indices.map { try XCTUnwrap(editor.boxRect(forBoxAt: $0), file: file, line: line) }
        let visibleAtTop = editor.visibleRect
        XCTAssertTrue(reference.contains { $0.minY > visibleAtTop.maxY }, "a box lies wholly below the fold: \(reference) under \(visibleAtTop)", file: file, line: line)
        // Scrolled so that the second box's top is above the visible area and its bottom inside it.
        let second = reference[1]
        clip.scroll(to: NSPoint(x: 0, y: second.minY + 60))
        controller.editorViewController.scrollView.reflectScrolledClipView(clip)
        editor.layoutSubtreeIfNeeded()
        let scrolled = editor.visibleRect
        XCTAssertLessThan(second.minY, scrolled.minY, "the box's top is above the visible area", file: file, line: line)
        XCTAssertGreaterThan(second.maxY, scrolled.minY, "and its bottom is in or below it", file: file, line: line)
        for (index, expected) in reference.enumerated() {
            let rect = try XCTUnwrap(editor.boxRect(forBoxAt: index), file: file, line: line)
            XCTAssertEqual(rect.minY, expected.minY, accuracy: 1, "box \(index + 1) keeps its top when scrolled", file: file, line: line)
            XCTAssertEqual(rect.maxY, expected.maxY, accuracy: 1, "box \(index + 1) keeps its bottom when scrolled", file: file, line: line)
        }
        clip.scroll(to: .zero)
        controller.editorViewController.scrollView.reflectScrolledClipView(clip)
    }

    /// The theme tour, opened untitled as the Welcome window does, with the card open in Form mode.
    func testTheReservedSpaceTracksTheCardsForm() async throws {
        let keys = try await loadedSchema()
        let text = try String(contentsOf: Fixtures.repositoryRoot.appendingPathComponent("examples/theme-tour.md"), encoding: .utf8)
        let document = try DeckDocument.makeUntitled(text: text)
        let controller = try XCTUnwrap(document.sessionController)
        _ = try await waitForRunningTap(document)
        let slideCount = SlideCard.cards(inDeckMarkdown: text).count
        try await waitForBoxes(document, count: slideCount)
        let card = controller.deckCard
        let form = controller.deckForm

        // A short editor: the card, and the boxes under it, reach below the fold.
        let shortWindow = try XCTUnwrap(controller.editorViewController.scrollView.window)
        let tallFrame = shortWindow.frame
        shortWindow.setFrame(NSRect(x: tallFrame.minX, y: tallFrame.minY, width: tallFrame.width, height: 500), display: false)
        controller.editor.layoutSubtreeIfNeeded()

        card.setOpen(true)
        XCTAssertEqual(card.display, .form)
        try await assertTheTextFollowsTheCard(controller, "open")
        try assertBoxesKeepTheirRectanglesWhileScrolling(controller)

        // The schema arrives after the card opens: the form grows from its empty state.
        form.setSchema([])
        card.refresh()
        try await assertTheTextFollowsTheCard(controller, "no schema")
        let emptyHeight = card.cardView.frame.height
        form.setSchema(keys)
        card.refresh()
        try await assertTheTextFollowsTheCard(controller, "schema loaded")
        XCTAssertGreaterThan(card.cardView.frame.height, emptyHeight, "the card grew with the form")
        XCTAssertEqual(card.cardView.frame.height, EditorTextView.deckCardHeaderHeight + form.contentHeight, accuracy: 1, "the card shows the whole form, taller than a short editor")

        card.setTextMode(true)
        try await assertTheTextFollowsTheCard(controller, "text mode")
        card.setTextMode(false)
        try await assertTheTextFollowsTheCard(controller, "form again")
        card.setOpen(false)
        try await assertTheTextFollowsTheCard(controller, "collapsed")
        card.setOpen(true)
        try await assertTheTextFollowsTheCard(controller, "reopened")

        // A width that puts the fields in two columns makes the form shorter.
        let scroll = controller.editorViewController.scrollView
        let window = try XCTUnwrap(scroll.window)
        let original = window.frame
        window.setFrame(NSRect(x: original.minX, y: original.minY, width: original.width + 500, height: original.height), display: false)
        controller.editor.layoutSubtreeIfNeeded()
        card.layout()
        try await assertTheTextFollowsTheCard(controller, "wider")
        window.setFrame(original, display: false)
        controller.editor.layoutSubtreeIfNeeded()
        card.layout()
        try await assertTheTextFollowsTheCard(controller, "narrower")
    }

    func testTheObjectGroupsHaveTheirFields() async throws {
        let keys = try await loadedSchema()
        let (_, controller, _) = try await openOnTheDeckCard()
        let form = controller.deckForm
        let recording = try XCTUnwrap(keys.first { $0.name == "recording" })
        let output = try XCTUnwrap(form.field("recording.output") as? NSTextField)
        XCTAssertEqual(output.stringValue, "")
        XCTAssertEqual(output.placeholderString, recording.keys.first { $0.name == "output" }?.defaultValue, "tap's default, in grey")
        XCTAssertEqual(output.toolTip, recording.keys.first { $0.name == "output" }?.description, "the schema's description")
        output.stringValue = "talks/recordings"
        output.sendAction(output.action, to: output.target)
        XCTAssertTrue(controller.editor.string.contains("recording:\n  output: talks/recordings\n"), "a missing parent is made")
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Change Output")
        XCTAssertNotNil(form.field("themeColors.background"))
    }

    func testTheDeckCardShowsTheFrontmatterAsText() async throws {
        _ = try await loadedSchema()
        let (_, controller, _) = try await openOnTheDeckCard()
        let editor = controller.editor
        let card = controller.deckCard
        let original = editor.string
        let frontmatterLength = editor.hiddenLength

        card.cardView.modeControl.selectedSegment = 1
        card.cardView.modeControl.sendAction(card.cardView.modeControl.action, to: card.cardView.modeControl.target)
        XCTAssertEqual(card.display, .text)
        XCTAssertEqual(editor.deckCardDisplay, .text)
        XCTAssertEqual(editor.frontmatterTextRegion, NSRange(location: 0, length: frontmatterLength), "the frontmatter's own lines are in the editor now")
        XCTAssertTrue(card.form.view.isHiddenOrHasHiddenAncestor, "the form gives way to the text")
        XCTAssertEqual(card.cardView.frame.height, EditorTextView.deckCardHeaderHeight, accuracy: 0.5, "the card's view covers only its header, so the lines can be clicked")
        XCTAssertGreaterThan(editor.deckCardRect().height, EditorTextView.deckCardHeaderHeight + 40, "the card's surface reaches down to the closing line")

        // The caret can be in the frontmatter, and typing there is the editor's own typing.
        let titleEnd = (editor.string as NSString).range(of: "Seven Slides").upperBound
        editor.setSelectedRange(NSRange(location: titleEnd, length: 0))
        XCTAssertEqual(editor.selectedRange().location, titleEnd, "no longer clamped out of the frontmatter")
        editor.insertText("!", replacementRange: NSRange(location: NSNotFound, length: 0))
        XCTAssertTrue(editor.string.hasPrefix("---\ntitle: Seven Slides!\n"))
        editor.undoManager?.undo()
        XCTAssertEqual(editor.string, original, "the same undo stack as any edit in the editor")

        // Back to Form: the frontmatter goes out of the editor's flow again.
        card.cardView.modeControl.selectedSegment = 0
        card.cardView.modeControl.sendAction(card.cardView.modeControl.action, to: card.cardView.modeControl.target)
        XCTAssertEqual(card.display, .form)
        XCTAssertNil(editor.frontmatterTextRegion)
        XCTAssertEqual(editor.hiddenLength, frontmatterLength, "the frontmatter is hidden again")
    }

    func testADeckWithNoFrontmatterHasADeckCard() async throws {
        _ = try await loadedSchema()
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("plain.md"))
        let controller = try XCTUnwrap(document.sessionController)
        let editor = controller.editor
        let card = controller.deckCard
        XCTAssertTrue(card.cardView.superview === editor, "the card is there without frontmatter")
        XCTAssertEqual(card.display, .collapsed)
        XCTAssertEqual(chipTexts(controller), ["Default", "16:9"])
        XCTAssertFalse(card.textIsAvailable, "there is no frontmatter to show as text yet")
        let original = editor.string

        card.setOpen(true)
        XCTAssertEqual(card.display, .form, "it offers the fields")
        let title = try XCTUnwrap(card.form.field("title") as? NSTextField)
        title.stringValue = "Fresh"
        title.sendAction(title.action, to: title.target)
        XCTAssertTrue(editor.string.hasPrefix("---\ntitle: Fresh\n---\n"), "the first change writes a frontmatter block: \(editor.string.prefix(40))")
        XCTAssertTrue(editor.string.hasSuffix(original), "the deck's own text is untouched below it")
        XCTAssertEqual(editor.undoManager?.undoActionName, "Change Title")
        editor.undoManager?.undo()
        XCTAssertEqual(editor.string, original, "one undo step takes the whole block back")
    }

    func testFrontmatterThatDoesNotParseOpensTheDeckCardAsText() async throws {
        _ = try await loadedSchema()
        let document = try await openDeck(try Fixtures.copyDeck("broken-frontmatter.md"))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 2)
        try await waitUntil(timeout: 10, "tap's deck error") { !controller.editor.deckErrors.isEmpty }
        let card = controller.deckCard
        XCTAssertTrue(card.isOpen, "the card opens by itself")
        XCTAssertEqual(card.display, .text, "as text, since a form cannot be built from what does not parse")
        XCTAssertEqual(controller.editor.deckCardTint, .error)
        XCTAssertFalse(controller.editor.deckCardMarkedLines.isEmpty, "the failing line is marked")
        XCTAssertTrue(controller.editor.deckCardMarkedLines.allSatisfy { $0.isError })
        XCTAssertNotNil(controller.editor.frontmatterTextRegion, "the frontmatter's lines are in the editor to fix")
        XCTAssertTrue(card.cardView.modeControl.isHidden == false)
        XCTAssertTrue(card.deckErrors.first?.hasPrefix("frontmatter:") == true, "tap's own message")
    }

    func testDeckSettingsFromTheViewMenu() async throws {
        _ = try await loadedSchema()
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("seven-slides.md"))
        let controller = try XCTUnwrap(document.sessionController)
        let deckWindow = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        let view = MainMenu.viewMenu()
        let item = try XCTUnwrap(view.items.first { $0.title == "Show Deck Settings" })
        XCTAssertEqual(item.keyEquivalent, "2")
        XCTAssertEqual(item.keyEquivalentModifierMask, [.command, .option])
        XCTAssertEqual(item.action, #selector(DeckWindowController.showDeckSettings(_:)))
        let preview = try XCTUnwrap(view.items.first { $0.title == "Show Preview" })
        XCTAssertEqual(preview.keyEquivalent, "1")
        XCTAssertNil(view.items.first { $0.title == "Deck" }, "no Deck tab any more")
        XCTAssertTrue(deckWindow.validateMenuItem(item))

        XCTAssertEqual(controller.deckCard.display, .collapsed)
        deckWindow.showDeckSettings(nil)
        XCTAssertEqual(controller.deckCard.display, .form, "the card opens")
        let responder = try XCTUnwrap(deckWindow.window?.firstResponder as? NSView)
        XCTAssertTrue(responder.isDescendant(of: controller.deckCard.cardView), "and takes the focus")
    }

    func testARefreshNeverClobbersTheFieldBeingEdited() async throws {
        _ = try await loadedSchema()
        let (_, controller, _) = try await openOnTheDeckCard()
        let editor = controller.editor
        let title = try XCTUnwrap(controller.deckForm.field("title") as? NSTextField)
        let window = try XCTUnwrap(title.window)
        XCTAssertTrue(window.makeFirstResponder(title))
        let fieldEditor = try XCTUnwrap(title.currentEditor())
        fieldEditor.string = "Draft"
        // A change to the text elsewhere (a disk load, an undo, a component's answer) refreshes the form.
        let range = (editor.string as NSString).range(of: "# The Page")
        editor.replaceText(in: range, with: "# The Page, edited", actionName: "Edit")
        XCTAssertEqual(fieldEditor.string, "Draft", "the field being typed in keeps what was typed")
        window.makeFirstResponder(nil)
        XCTAssertTrue(editor.string.contains("title: Draft\n"), "ending the edit writes it")
    }

    /// The form rebuilds when the frontmatter's shape changes (a disk load
    /// that adds a driver, here); what was typed and not yet committed
    /// lands in the frontmatter first, never in the bin.
    func testARebuildCommitsTheFieldBeingEdited() async throws {
        _ = try await loadedSchema()
        let (document, controller, _) = try await openOnTheDeckCard()
        let editor = controller.editor
        let deck = try XCTUnwrap(document.fileURL)
        let title = try XCTUnwrap(controller.deckForm.field("title") as? NSTextField)
        let window = try XCTUnwrap(title.window)
        XCTAssertTrue(window.makeFirstResponder(title))
        try XCTUnwrap(title.currentEditor()).string = "Draft"
        // The file gains a driver on disk; the buffer has no edits, so it loads silently and the drivers group rebuilds.
        try approveLiveCode(for: deck, drivers: ["sqlite", "shell"])
        let pulled = try String(contentsOf: deck, encoding: .utf8).replacingOccurrences(of: "  sqlite: {}\n", with: "  sqlite: {}\n  shell: {}\n")
        try pulled.write(to: deck, atomically: true, encoding: .utf8)
        try await waitUntil(timeout: 15, "the disk version loaded and the form rebuilt") { controller.deckForm.builtForEntries["drivers"] == ["sqlite", "shell"] }
        XCTAssertTrue(editor.string.contains("title: Draft\n"), "the typed title was committed before the rows went: \(editor.string.prefix(80))")
        XCTAssertTrue(editor.string.contains("  shell: {}\n"))
        XCTAssertEqual((controller.deckForm.field("title") as? NSTextField)?.stringValue, "Draft")
        XCTAssertNil(window.firstResponder as? NSText, "the edit ended; nothing half typed is left")
    }

    /// Play saves the file; a Deck tab field still being typed in goes into that save.
    func testPlayCommitsTheDeckTabsEdit() async throws {
        _ = try await loadedSchema()
        let (document, controller, _) = try await openOnTheDeckCard()
        let deck = try XCTUnwrap(document.fileURL)
        let title = try XCTUnwrap(controller.deckForm.field("title") as? NSTextField)
        XCTAssertTrue(try XCTUnwrap(title.window).makeFirstResponder(title))
        try XCTUnwrap(title.currentEditor()).string = "Draft"
        // The save Play makes, without the talk, bounded by the test.
        var saved: Error?? = nil
        controller.saveForPresenting { saved = .some($0) }
        try await waitUntil(timeout: 10, "the save") { saved != nil }
        XCTAssertNil(saved ?? nil)
        let onDisk = try String(contentsOf: deck, encoding: .utf8)
        XCTAssertTrue(onDisk.contains("title: Draft\n"), "the file Play reads has the typed title: \(onDisk.prefix(60))")
    }

    /// An autosave writes what is typed so far and leaves the person typing.
    func testAnAutosaveWritesTheFieldWithoutTakingItsFocus() async throws {
        _ = try await loadedSchema()
        let (document, controller, _) = try await openOnTheDeckCard()
        let deck = try XCTUnwrap(document.fileURL)
        let title = try XCTUnwrap(controller.deckForm.field("title") as? NSTextField)
        let window = try XCTUnwrap(title.window)
        XCTAssertTrue(window.makeFirstResponder(title))
        let fieldEditor = try XCTUnwrap(title.currentEditor())
        fieldEditor.string = "Half typ"
        var saved: Error?? = nil
        document.save(to: deck, ofType: document.fileType ?? "net.daringfireball.markdown", for: .autosaveInPlaceOperation) { saved = .some($0) }
        try await waitUntil(timeout: 10, "the autosave") { saved != nil }
        XCTAssertTrue(try String(contentsOf: deck, encoding: .utf8).contains("title: Half typ\n"))
        XCTAssertTrue(window.firstResponder === fieldEditor, "still typing")
        fieldEditor.string = "Half typed"
        window.makeFirstResponder(nil)
        XCTAssertTrue(controller.editor.string.contains("title: Half typed\n"))
    }

    /// Cmd-S, a save the person asks for, ends a Deck tab edit and writes it.
    func testASaveThePersonAsksForCommitsTheDeckField() async throws {
        _ = try await loadedSchema()
        let (document, controller, _) = try await openOnTheDeckCard()
        let deck = try XCTUnwrap(document.fileURL)
        let title = try XCTUnwrap(controller.deckForm.field("title") as? NSTextField)
        let window = try XCTUnwrap(title.window)
        XCTAssertTrue(window.makeFirstResponder(title))
        try XCTUnwrap(title.currentEditor()).string = "Draft"
        var saved: Error?? = nil
        document.save(to: deck, ofType: document.fileType ?? "net.daringfireball.markdown", for: .saveOperation) { saved = .some($0) }
        try await waitUntil(timeout: 10, "the save") { saved != nil }
        XCTAssertNil(saved ?? nil)
        let onDisk = try String(contentsOf: deck, encoding: .utf8)
        XCTAssertTrue(onDisk.contains("title: Draft\n"), "the file has the typed title: \(onDisk.prefix(60))")
        XCTAssertNil(window.firstResponder as? NSText, "the edit ended")
    }

    /// An autosave elsewhere (an untitled deck's) writes what is typed so far and leaves the person typing, as one in place does.
    func testAnAutosaveElsewhereKeepsTheFieldsFocus() async throws {
        _ = try await loadedSchema()
        let (document, controller, _) = try await openOnTheDeckCard()
        let title = try XCTUnwrap(controller.deckForm.field("title") as? NSTextField)
        let window = try XCTUnwrap(title.window)
        XCTAssertTrue(window.makeFirstResponder(title))
        let fieldEditor = try XCTUnwrap(title.currentEditor())
        fieldEditor.string = "Half typ"
        let elsewhere = try Fixtures.temporaryFolder().appendingPathComponent("autosave.md")
        var saved: Error?? = nil
        document.save(to: elsewhere, ofType: document.fileType ?? "net.daringfireball.markdown", for: .autosaveElsewhereOperation) { saved = .some($0) }
        try await waitUntil(timeout: 10, "the autosave") { saved != nil }
        XCTAssertTrue(controller.editor.string.contains("title: Half typ\n"), "the text so far is in the buffer the autosave wrote")
        XCTAssertTrue(window.firstResponder === fieldEditor, "still typing")
        fieldEditor.string = "Half typed"
        window.makeFirstResponder(nil)
        XCTAssertTrue(controller.editor.string.contains("title: Half typed\n"))
    }

    /// When tap reports the frontmatter broken, the form's rebuild ends the
    /// edit in progress without writing it into a frontmatter nobody can read.
    func testARebuildNeverWritesIntoABrokenFrontmatter() async throws {
        _ = try await loadedSchema()
        let (_, controller, _) = try await openOnTheDeckCard()
        let editor = controller.editor
        let title = try XCTUnwrap(controller.deckForm.field("title") as? NSTextField)
        let window = try XCTUnwrap(title.window)
        XCTAssertTrue(window.makeFirstResponder(title))
        try XCTUnwrap(title.currentEditor()).string = "Draft"
        let range = (editor.string as NSString).range(of: "  sqlite: {}")
        editor.replaceText(in: range, with: "  sqlite: [unclosed", actionName: "Edit")
        try await waitUntil(timeout: 15, "tap's deck error") { !editor.deckErrors.isEmpty }
        try await waitUntil(timeout: 5, "the card's text mode") { controller.deckCard.display == .text }
        XCTAssertFalse(editor.string.contains("Draft"), "nothing typed went into the broken frontmatter: \(editor.string.prefix(80))")
        XCTAssertTrue(editor.string.hasPrefix("---\ntitle: Seven Slides\n"))
        XCTAssertNil(window.firstResponder as? NSText, "the edit ended")
    }
}

private extension NSView {
    /// The first view under this one with the accessibility identifier, or nil.
    func descendant(identifiedBy identifier: String) -> NSView? {
        if accessibilityIdentifier() == identifier { return self }
        for subview in subviews {
            if let found = subview.descendant(identifiedBy: identifier) { return found }
        }
        return nil
    }
}
