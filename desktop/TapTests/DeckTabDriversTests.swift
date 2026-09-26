import XCTest
@testable import Tap

/// The Deck tab's drivers cards: one per declared driver with its settings
/// and Remove, the hint about secrets, the name field with Add, raw text
/// for the settings the form has no field for, and Other keys for what
/// tap does not know.
final class DeckTabDriversTests: HostedTestCase {
    func openOnTheDeckTab(_ deck: URL) async throws -> (DeckDocument, DeckSessionController, DeckFormViewController) {
        Task { await AppEnvironment.shared.deckSchema.load() }
        try await waitUntil(timeout: 30, "the schema") { AppEnvironment.shared.deckSchema.isLoaded }
        let document = try await openDeckAndWaitForPreview(deck)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 10, "tap's first answer") { controller.lastAppliedText != nil }
        let deckWindow = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        deckWindow.showDeckTab(nil)
        return (document, controller, controller.deckForm)
    }

    func testTheDriversCardsListEachDriverWithItsSettings() async throws {
        let (_, controller, form) = try await openOnTheDeckTab(try Fixtures.copyDeck("custom-driver.md"))
        let editor = controller.editor
        XCTAssertEqual(form.builtForEntries["drivers"], ["sqlite", "fortune"])
        let command = try XCTUnwrap(form.field("drivers.fortune.command") as? NSTextField)
        XCTAssertEqual(command.stringValue, "/bin/cat")
        let timeout = try XCTUnwrap(form.field("drivers.sqlite.timeout") as? NSTextField)
        XCTAssertEqual(timeout.stringValue, "")
        XCTAssertNotNil(timeout.placeholderString, "tap's default, from the schema")
        let hint = try XCTUnwrap(form.hintLabel(for: "drivers"))
        XCTAssertTrue(hint.stringValue.contains("${NAME}"), "the hint to keep secrets out of the deck")

        // A driver's setting, as one undo step.
        timeout.stringValue = "5"
        timeout.sendAction(timeout.action, to: timeout.target)
        XCTAssertTrue(editor.string.hasPrefix("---\ntitle: Custom Driver\ndrivers:\n  sqlite:\n    timeout: 5\n  fortune:\n    command: /bin/cat\n---\n"), String(editor.string.prefix(90)))
        XCTAssertEqual(editor.undoManager?.undoActionName, "Change Timeout")
        let timeoutAgain = try XCTUnwrap(form.field("drivers.sqlite.timeout") as? NSTextField, "the rows were not rebuilt for a scalar change")
        timeoutAgain.stringValue = "x"
        timeoutAgain.sendAction(timeoutAgain.action, to: timeoutAgain.target)
        XCTAssertTrue(editor.string.contains("    timeout: 5\n"), "a value that is not an integer is refused")
        XCTAssertEqual(timeoutAgain.stringValue, "5", "and the field reads the text again")
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == editor.string }
        XCTAssertEqual(editor.deckErrors, [])

        // Add a driver by name; the cards rebuild with a new one.
        let addField = try XCTUnwrap(form.addEntryField(for: "drivers"))
        addField.stringValue = "shell"
        try XCTUnwrap(form.addEntryButton(for: "drivers")).performClick(nil)
        XCTAssertTrue(editor.string.contains("    command: /bin/cat\n  shell: {}\n---\n"), String(editor.string.prefix(120)))
        XCTAssertEqual(editor.undoManager?.undoActionName, "Add shell")
        XCTAssertNotNil(form.field("drivers.shell.timeout"), "the form rebuilt for the new entry")
        XCTAssertEqual(form.addEntryField(for: "drivers")?.stringValue, "", "ready for the next name")
        try await waitForTheUndoStepToClose(editor.undoManager)

        // Remove it again from its card's header; the card goes. (The deck's body still holds a shell block, so the check is on the drivers.)
        try XCTUnwrap(form.removeButton(for: "drivers.shell")).performClick(nil)
        XCTAssertEqual(Frontmatter(text: editor.string).declaredDrivers, ["sqlite", "fortune"])
        XCTAssertEqual(editor.undoManager?.undoActionName, "Remove shell")
        XCTAssertNil(form.field("drivers.shell.timeout"))
        try await waitForTheUndoStepToClose(editor.undoManager)
        editor.undoManager?.undo()
        XCTAssertNotNil(form.field("drivers.shell.timeout"), "undo brings the entry and its card back")
        XCTAssertTrue(editor.string.contains("    command: /bin/cat\n  shell: {}\n---\n"), "only the Remove was undone")
    }

    /// A deck whose sqlite driver has a connections map, in a folder of its own.
    func connectionsDeck() throws -> URL {
        let deck = try Fixtures.temporaryFolder().appendingPathComponent("connections.md")
        try """
        ---
        title: Connections
        drivers:
          sqlite:
            connections:
              incident:
                path: ./incident.db
        ---

        # One

        ```sql {driver: sqlite, connection: incident}
        SELECT 1;
        ```
        """.write(to: deck, atomically: true, encoding: .utf8)
        return deck
    }

    func testRawSettingsAreEditedAsText() async throws {
        let (_, controller, form) = try await openOnTheDeckTab(try connectionsDeck())
        let editor = controller.editor
        let original = "    connections:\n      incident:\n        path: ./incident.db\n"
        // The editor is fetched again after every edit, so the checks hold whether or not the rows were rebuilt.
        var raw = try XCTUnwrap(form.rawEditor("drivers.sqlite.connections"), "a map inside a driver is edited as its own lines")
        XCTAssertEqual(raw.string, original)
        raw.string = "    connections:\n      incident:\n        path: ${INCIDENT_DB}\n"
        form.textDidEndEditing(Notification(name: NSText.didEndEditingNotification, object: raw))
        XCTAssertTrue(editor.string.contains("        path: ${INCIDENT_DB}\n"))
        XCTAssertEqual(editor.undoManager?.undoActionName, "Change Connections")
        try await waitUntil(timeout: 10, "tap's answer") { controller.lastAppliedText == editor.string }
        XCTAssertEqual(editor.deckErrors, [], "tap reads the block as written")
        let accepted = editor.string
        // Text that would end the frontmatter early, or step out of the entry's indent, is refused.
        raw = try XCTUnwrap(form.rawEditor("drivers.sqlite.connections"))
        XCTAssertEqual(raw.string, "    connections:\n      incident:\n        path: ${INCIDENT_DB}\n")
        raw.string = "    connections:\n---\n"
        form.textDidEndEditing(Notification(name: NSText.didEndEditingNotification, object: raw))
        XCTAssertEqual(editor.string, accepted, "a --- line would close the frontmatter on the wrong line")
        raw = try XCTUnwrap(form.rawEditor("drivers.sqlite.connections"))
        raw.string = "connections:\n  b:\n    path: y.db\n"
        form.textDidEndEditing(Notification(name: NSText.didEndEditingNotification, object: raw))
        XCTAssertEqual(editor.string, accepted, "a block that lost its indent would leave its driver")
        raw = try XCTUnwrap(form.rawEditor("drivers.sqlite.connections"))
        raw.string = "    connections:\n  shell: {}\n"
        form.textDidEndEditing(Notification(name: NSText.didEndEditingNotification, object: raw))
        XCTAssertEqual(editor.string, accepted, "a later line at the drivers' indent would declare a driver from the Connections box")
        raw = try XCTUnwrap(form.rawEditor("drivers.sqlite.connections"))
        raw.string = "    connections:\n      incident:\n        path: x.db\ntitle: Twice\n"
        form.textDidEndEditing(Notification(name: NSText.didEndEditingNotification, object: raw))
        XCTAssertEqual(editor.string, accepted, "a later line at the top level would repeat a key of the frontmatter")
        raw = try XCTUnwrap(form.rawEditor("drivers.sqlite.connections"))
        XCTAssertEqual(raw.string, "    connections:\n      incident:\n        path: ${INCIDENT_DB}\n", "the editor reads the text again")
        editor.undoManager?.undo()
        raw = try XCTUnwrap(form.rawEditor("drivers.sqlite.connections"))
        XCTAssertEqual(raw.string, original, "and follows an undo")
        // A scalar edit elsewhere does not rebuild the rows: the raw editor stays the same view.
        let title = try XCTUnwrap(form.field("title") as? NSTextField)
        title.stringValue = "Renamed"
        title.sendAction(title.action, to: title.target)
        XCTAssertTrue(form.rawEditor("drivers.sqlite.connections") === raw)
    }

    /// An args list written as a block is its own lines of text, as the
    /// DeckTabDrivers board draws it; the row follows the text (an undo, a
    /// disk load) and never writes stale lines back.
    func testABlockStyleListIsEditedAsTextAndFollowsTheText() async throws {
        let deck = try Fixtures.temporaryFolder().appendingPathComponent("args.md")
        try "---\ntitle: Args\ndrivers:\n  incidents:\n    command: /bin/echo\n    args:\n    - one\n    - two\n---\n\n# One\n".write(to: deck, atomically: true, encoding: .utf8)
        let (_, controller, form) = try await openOnTheDeckTab(deck)
        let editor = controller.editor
        XCTAssertEqual((form.field("drivers.incidents.command") as? NSTextField)?.stringValue, "/bin/echo")
        XCTAssertNil(form.field("drivers.incidents.args"), "no one-line field for a list on several lines")
        let original = "    args:\n    - one\n    - two\n"
        var raw = try XCTUnwrap(form.rawEditor("drivers.incidents.args"))
        XCTAssertEqual(raw.string, original)

        // An item at the entry's own indent stays in the entry.
        raw.string = "    args:\n    - one\n    - three"
        form.textDidEndEditing(Notification(name: NSText.didEndEditingNotification, object: raw))
        XCTAssertTrue(editor.string.contains("    - one\n    - three\n---\n"), String(editor.string.prefix(120)))
        XCTAssertEqual(editor.undoManager?.undoActionName, "Change Args")
        try await waitForTheUndoStepToClose(editor.undoManager)

        // An undo: the row shows the lines back, and leaving it writes nothing.
        editor.undoManager?.undo()
        XCTAssertTrue(editor.string.contains(original))
        raw = try XCTUnwrap(form.rawEditor("drivers.incidents.args"))
        XCTAssertEqual(raw.string, original, "the row follows an undo")
        let undone = editor.string
        form.textDidEndEditing(Notification(name: NSText.didEndEditingNotification, object: raw))
        XCTAssertEqual(editor.string, undone, "a focus in and out writes no stale lines back")

        // A change from outside the form (a disk load goes through the same edit) that keeps the list on several lines.
        var range = (editor.string as NSString).range(of: "    - two\n")
        editor.replaceText(in: range, with: "    - pulled\n", actionName: "Edit")
        raw = try XCTUnwrap(form.rawEditor("drivers.incidents.args"))
        XCTAssertEqual(raw.string, "    args:\n    - one\n    - pulled\n", "the row shows the pulled lines")

        // One that puts the list on one line: the row becomes a field.
        range = (editor.string as NSString).range(of: "args:\n    - one\n    - pulled")
        editor.replaceText(in: range, with: "args: [one]", actionName: "Edit")
        XCTAssertNil(form.rawEditor("drivers.incidents.args"))
        XCTAssertEqual((form.field("drivers.incidents.args") as? NSTextField)?.stringValue, "[one]")
    }

    /// An autosave writes what is typed in a raw row so far and leaves the
    /// person typing in it, as it does for a field.
    func testAnAutosaveWritesARawRowWithoutTakingItsFocus() async throws {
        let (document, controller, form) = try await openOnTheDeckTab(try connectionsDeck())
        let deck = try XCTUnwrap(document.fileURL)
        let raw = try XCTUnwrap(form.rawEditor("drivers.sqlite.connections"))
        let window = try XCTUnwrap(raw.window)
        XCTAssertTrue(window.makeFirstResponder(raw))
        let typed = "    connections:\n      incident:\n        path: ./other.db"
        raw.string = typed
        var saved: Error?? = nil
        document.save(to: deck, ofType: document.fileType ?? "net.daringfireball.markdown", for: .autosaveInPlaceOperation) { saved = .some($0) }
        try await waitUntil(timeout: 10, "the autosave") { saved != nil }
        XCTAssertTrue(try String(contentsOf: deck, encoding: .utf8).contains("        path: ./other.db\n---\n"))
        XCTAssertTrue(form.rawEditor("drivers.sqlite.connections") === raw, "the rows were not rebuilt")
        XCTAssertTrue(window.firstResponder === raw, "still typing")
        XCTAssertEqual(raw.string, typed, "what is typed stays as typed")
        raw.string = typed + "x"
        window.makeFirstResponder(nil)
        XCTAssertTrue(controller.editor.string.contains("        path: ./other.dbx\n"))
    }

    /// A driver name with a dot in it is one name: its card's Remove removes it.
    func testADriverWhoseNameHasADotIsRemoved() async throws {
        let deck = try Fixtures.temporaryFolder().appendingPathComponent("dotted.md")
        try "---\ntitle: Dotted\ndrivers:\n  sqlite: {}\n  my.db:\n    command: /bin/cat\n---\n\n# One\n".write(to: deck, atomically: true, encoding: .utf8)
        let (_, controller, form) = try await openOnTheDeckTab(deck)
        XCTAssertEqual(form.builtForEntries["drivers"], ["sqlite", "my.db"])
        try XCTUnwrap(form.removeButton(for: "drivers.my.db")).performClick(nil)
        XCTAssertEqual(Frontmatter(text: controller.editor.string).declaredDrivers, ["sqlite"])
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Remove my.db")
    }

    func testUnknownKeysAreListedUnderOtherKeys() async throws {
        let folder = try Fixtures.temporaryFolder()
        let deck = folder.appendingPathComponent("other.md")
        try "---\ntitle: Other\nspeakerNotesFont: 18\nlegacy:\n  a: 1\n---\n\n# One\n".write(to: deck, atomically: true, encoding: .utf8)
        let (_, _, form) = try await openOnTheDeckTab(deck)
        XCTAssertEqual(form.otherKeyLabels.map(\.stringValue), ["speakerNotesFont: 18", "legacy:\n  a: 1"], "as written, read-only")
        XCTAssertNil(form.field("speakerNotesFont"))
    }
}
