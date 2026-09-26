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

        // Remove it again from its card's header; the card goes. (The deck's body still holds a shell block, so the check is on the drivers.)
        try XCTUnwrap(form.removeButton(for: "drivers.shell")).performClick(nil)
        XCTAssertEqual(Frontmatter(text: editor.string).declaredDrivers, ["sqlite", "fortune"])
        XCTAssertEqual(editor.undoManager?.undoActionName, "Remove shell")
        XCTAssertNil(form.field("drivers.shell.timeout"))
        editor.undoManager?.undo()
        XCTAssertNotNil(form.field("drivers.shell.timeout"), "undo brings the entry and its card back")
    }

    func testRawSettingsAreEditedAsText() async throws {
        let folder = try Fixtures.temporaryFolder()
        let deck = folder.appendingPathComponent("connections.md")
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
        let (_, controller, form) = try await openOnTheDeckTab(deck)
        let editor = controller.editor
        let original = "    connections:\n      incident:\n        path: ./incident.db\n"
        // The rows rebuild whenever a raw block changes, so the editor is fetched again after every edit.
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

    func testUnknownKeysAreListedUnderOtherKeys() async throws {
        let folder = try Fixtures.temporaryFolder()
        let deck = folder.appendingPathComponent("other.md")
        try "---\ntitle: Other\nspeakerNotesFont: 18\nlegacy:\n  a: 1\n---\n\n# One\n".write(to: deck, atomically: true, encoding: .utf8)
        let (_, _, form) = try await openOnTheDeckTab(deck)
        XCTAssertEqual(form.otherKeyLabels.map(\.stringValue), ["speakerNotesFont: 18", "legacy:\n  a: 1"], "as written, read-only")
        XCTAssertNil(form.field("speakerNotesFont"))
    }
}
