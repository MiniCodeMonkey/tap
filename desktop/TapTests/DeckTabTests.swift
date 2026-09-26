import XCTest
@testable import Tap

/// The Deck tab: a form from tap's schema over the frontmatter, each
/// change one undo step through the editor.
final class DeckTabTests: HostedTestCase {
    /// The schema, loaded once per process; the load is a tap run with its own timeout.
    func loadedSchema() async throws -> [SchemaKey] {
        Task { await AppEnvironment.shared.deckSchema.load() }
        try await waitUntil(timeout: 30, "tap deck schema --json") { AppEnvironment.shared.deckSchema.isLoaded }
        return AppEnvironment.shared.deckSchema.keys
    }

    func openOnTheDeckTab(_ name: String = "seven-slides.md", slides: Int = 7) async throws -> (DeckDocument, DeckSessionController, DeckWindowController) {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck(name))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: slides)
        let deckWindow = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        deckWindow.showDeckTab(nil)
        return (document, controller, deckWindow)
    }

    func testTheDeckTabEnablesOnceTheSchemaHasLoaded() async throws {
        _ = try await loadedSchema()
        // The schema loaded before this deck opened: the tab is on from the start.
        let (_, controller, deckWindow) = try await openOnTheDeckTab()
        let inspector = controller.inspectorViewController
        XCTAssertTrue(inspector.isDeckTabAvailable)
        XCTAssertTrue(inspector.tabs.isEnabled(forSegment: 1), "the segment shows what the pane knew before its view loaded")
        XCTAssertEqual(inspector.selectedTab, .deck)
        XCTAssertEqual(inspector.tabs.selectedSegment, 1)
        let item = NSMenuItem(title: "Deck", action: #selector(DeckWindowController.showDeckTab(_:)), keyEquivalent: "")
        XCTAssertTrue(deckWindow.validateMenuItem(item))
        deckWindow.showPreviewTab(nil)
        XCTAssertEqual(inspector.selectedTab, .preview)
        XCTAssertFalse(controller.previewViewController.view.isHidden)
        // Cmd+Option+0 hides the pane; View > Deck brings it back with the tab.
        deckWindow.togglePreview(nil)
        XCTAssertTrue(deckWindow.splitViewController.isPreviewHidden)
        deckWindow.showDeckTab(nil)
        XCTAssertFalse(deckWindow.splitViewController.isPreviewHidden, "a tab nobody can see is no tab")
        XCTAssertEqual(inspector.selectedTab, .deck)
    }

    func testDeckSettingsLiveInTheInspector() async throws {
        let keys = try await loadedSchema()
        let (_, controller, deckWindow) = try await openOnTheDeckTab()
        let editor = controller.editor
        XCTAssertGreaterThan(editor.hiddenLength, 0, "the frontmatter text is hidden from the editor")
        XCTAssertEqual(editor.boxes[0].range.location, editor.hiddenLength, "slide 1 is the first box")
        let form = controller.deckForm
        XCTAssertFalse(form.view.isHiddenOrHasHiddenAncestor)
        XCTAssertTrue(controller.previewViewController.view.isHidden)

        // One field per frontmatter key tap knows; the fields, types and allowed values come from tap deck schema --json.
        for key in keys where key.isScalar { XCTAssertNotNil(form.field(key.name), "a field for \(key.name)") }
        for key in keys where key.type == "object" {
            for child in key.keys where child.isScalar { XCTAssertNotNil(form.field("\(key.name).\(child.name)"), "a field for \(key.name).\(child.name)") }
        }
        XCTAssertTrue(form.field("slideNumbers") is NSSwitch, "a boolean is a switch, as the DeckTabFields board draws it")
        let title = try XCTUnwrap(form.field("title") as? NSTextField)
        XCTAssertEqual(title.stringValue, "Seven Slides")
        let theme = try XCTUnwrap(form.field("theme") as? NSPopUpButton, "a string with allowed values is a popup")
        let themeKey = try XCTUnwrap(keys.first { $0.name == "theme" })
        let defaultItem = form.defaultItemTitle(for: themeKey)
        XCTAssertEqual(defaultItem, "Default (\(themeKey.defaultValue ?? ""))")
        XCTAssertEqual(theme.itemTitles, [defaultItem] + themeKey.values, "the allowed values come from tap, behind the way back to tap's default")
        XCTAssertEqual(theme.titleOfSelectedItem, defaultItem, "the deck sets no theme")

        // Changing a field rewrites that key in the frontmatter as one undo step.
        let original = editor.string
        let chosen = try XCTUnwrap(themeKey.values.last)
        theme.selectItem(withTitle: chosen)
        theme.sendAction(theme.action, to: theme.target)
        XCTAssertTrue(editor.string.hasPrefix("---\ntitle: Seven Slides\ndrivers:\n  sqlite: {}\ntheme: \(chosen)\n---\n"), String(editor.string.prefix(80)))
        XCTAssertEqual(editor.undoManager?.undoActionName, "Change Theme")
        XCTAssertTrue(controller.isContentEdited, "the edited flag follows the content")
        theme.sendAction(theme.action, to: theme.target)
        XCTAssertEqual(editor.undoManager?.undoActionName, "Change Theme", "the same value again registers no second step")
        try await waitUntil(timeout: 10, "tap's answer for the new frontmatter") { controller.lastAppliedText == editor.string }
        XCTAssertEqual(editor.deckErrors, [], "tap accepts what the form wrote")
        XCTAssertGreaterThan(editor.hiddenLength, (original as NSString).range(of: "# Debugging").location, "the frontmatter stays hidden, one line longer")

        title.stringValue = "Deck: renamed"
        title.sendAction(title.action, to: title.target)
        XCTAssertTrue(editor.string.contains("title: \"Deck: renamed\"\n"), "a value YAML would misread is quoted")
        XCTAssertEqual(editor.undoManager?.undoActionName, "Change Title")
        let numbers = try XCTUnwrap(form.field("slideNumbers") as? NSSwitch)
        XCTAssertEqual(numbers.state, .on, "absent, so tap's default")
        numbers.state = .off
        numbers.sendAction(numbers.action, to: numbers.target)
        XCTAssertTrue(editor.string.contains("slideNumbers: false\n"))

        // Undo, one change at a time; the form follows the text.
        editor.undoManager?.undo()
        XCTAssertFalse(editor.string.contains("slideNumbers"))
        XCTAssertEqual(numbers.state, .on)
        editor.undoManager?.undo()
        XCTAssertEqual(title.stringValue, "Seven Slides")
        editor.undoManager?.undo()
        XCTAssertEqual(editor.string, original)
        XCTAssertEqual(theme.titleOfSelectedItem, defaultItem, "the popup follows an undo")
        // Choosing the default item removes the key.
        theme.selectItem(withTitle: chosen)
        theme.sendAction(theme.action, to: theme.target)
        XCTAssertTrue(editor.string.contains("theme: \(chosen)\n"))
        theme.selectItem(withTitle: defaultItem)
        theme.sendAction(theme.action, to: theme.target)
        XCTAssertFalse(editor.string.contains("theme:"), "back to tap's default: the key goes")
        XCTAssertEqual(editor.undoManager?.undoActionName, "Change Theme")
        // Not `isContentEdited` here: the autosave may have written the file in between, and the flag is against the file.
        deckWindow.showPreviewTab(nil)
        XCTAssertEqual(controller.inspectorViewController.selectedTab, .preview)
    }

    func testTheObjectGroupsHaveTheirFields() async throws {
        let keys = try await loadedSchema()
        let (_, controller, _) = try await openOnTheDeckTab()
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

    func testTheDeckTabRefusesWhileTheFrontmatterIsBroken() async throws {
        _ = try await loadedSchema()
        let document = try await openDeck(try Fixtures.copyDeck("broken-frontmatter.md"))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 2)
        try await waitUntil(timeout: 10, "tap's deck error") { !controller.editor.deckErrors.isEmpty }
        let deckWindow = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        deckWindow.showDeckTab(nil)
        let form = controller.deckForm
        XCTAssertNil(form.field("title"), "no field to edit a frontmatter tap cannot read")
        XCTAssertTrue(form.stack.arrangedSubviews.contains(form.errorTitleLabel))
        XCTAssertEqual(form.errorTitleLabel.stringValue, "The deck settings have a problem")
        XCTAssertTrue(form.errorLabel.stringValue.hasPrefix("frontmatter:"), "tap's own message")
        XCTAssertEqual(form.errorHintLabel.stringValue, "The frontmatter is shown in the editor until it parses.")
    }

    func testARefreshNeverClobbersTheFieldBeingEdited() async throws {
        _ = try await loadedSchema()
        let (_, controller, _) = try await openOnTheDeckTab()
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
        let (document, controller, _) = try await openOnTheDeckTab()
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
        let (document, controller, _) = try await openOnTheDeckTab()
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
        let (document, controller, _) = try await openOnTheDeckTab()
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
        let (document, controller, _) = try await openOnTheDeckTab()
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
        let (document, controller, _) = try await openOnTheDeckTab()
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
        let (_, controller, _) = try await openOnTheDeckTab()
        let editor = controller.editor
        let title = try XCTUnwrap(controller.deckForm.field("title") as? NSTextField)
        let window = try XCTUnwrap(title.window)
        XCTAssertTrue(window.makeFirstResponder(title))
        try XCTUnwrap(title.currentEditor()).string = "Draft"
        let range = (editor.string as NSString).range(of: "  sqlite: {}")
        editor.replaceText(in: range, with: "  sqlite: [unclosed", actionName: "Edit")
        try await waitUntil(timeout: 15, "tap's deck error") { !editor.deckErrors.isEmpty }
        try await waitUntil(timeout: 5, "the form's error state") { controller.deckForm.stack.arrangedSubviews.contains(controller.deckForm.errorTitleLabel) }
        XCTAssertFalse(editor.string.contains("Draft"), "nothing typed went into the broken frontmatter: \(editor.string.prefix(80))")
        XCTAssertTrue(editor.string.hasPrefix("---\ntitle: Seven Slides\n"))
        XCTAssertNil(window.firstResponder as? NSText, "the edit ended")
    }
}
