import XCTest
@testable import Tap

/// The approval sheet on its own, on a plain window: its copy, its rows,
/// and which key does what. No tap runs here.
final class ApprovalSheetTests: HostedTestCase {
    var host: NSWindow!

    override func setUp() async throws {
        try await super.setUp()
        host = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 900, height: 700), styleMask: [.titled], backing: .buffered, defer: false)
        host.isReleasedWhenClosed = false
        // A sheet attaches to a window that is on screen; the test host's screen is CI's.
        host.orderFrontRegardless()
    }

    override func tearDown() async throws {
        if let sheet = host.attachedSheet { host.endSheet(sheet, returnCode: .abort) }
        host.orderOut(nil)
        host.close()
        try await super.tearDown()
    }

    func payload(approvedBefore: [String]? = nil) -> QuestionPayload {
        QuestionPayload(deck: "/private/tmp/t/talk.md",
                        drivers: [ApprovalDriver(name: "shell", slides: [2, 5], blocks: 2), ApprovalDriver(name: "sqlite", slides: [4], blocks: 1)],
                        approvedBefore: approvedBefore,
                        blocks: [ApprovalBlock(driver: "shell", code: "echo hello from slide 2", slide: 2, block: 1),
                                 ApprovalBlock(driver: "sqlite", code: "SELECT 1 AS one;", slide: 4, block: 1),
                                 ApprovalBlock(driver: "shell", code: "echo five", slide: 5, block: 1)])
    }

    func key(_ characters: String, code: UInt16, isARepeat: Bool = false, modifiers: NSEvent.ModifierFlags = []) throws -> NSEvent {
        try XCTUnwrap(NSEvent.keyEvent(with: .keyDown, location: .zero, modifierFlags: modifiers, timestamp: 0, windowNumber: host.windowNumber,
                                       context: nil, characters: characters, charactersIgnoringModifiers: characters, isARepeat: isARepeat, keyCode: code))
    }

    /// A key press routed the way AppKit routes one to a window: its key
    /// equivalents first, then the first responder's keyDown, which passes
    /// what it does not handle up to the window.
    func press(_ event: NSEvent, in sheet: NSWindow) {
        if sheet.performKeyEquivalent(with: event) { return }
        (sheet.firstResponder ?? sheet).keyDown(with: event)
    }

    /// Only a click on Allow answers yes. No key a person might press at
    /// the sheet does: each answers Don't Allow or nothing.
    func testNoKeyAllows() throws {
        XCTAssertTrue(ApprovalSheet(payload: payload(), deckName: "talk.md").acceptButton.refusesFirstResponder,
                      "Tab never lands on Allow, with keyboard navigation on or off, so Space never presses it")
        let tab = try key("\t", code: 48)
        let space = try key(" ", code: 49)
        var presses: [(name: String, events: [NSEvent])] = [
            ("Return", [try key("\r", code: 36)]),
            ("Enter on the keypad", [try key("\u{3}", code: 76)]),
            ("Space", [space]),
            ("Escape", [try key("\u{1b}", code: 53)]),
            ("a held Return", [try key("\r", code: 36, isARepeat: true)]),
            ("a held Space", [try key(" ", code: 49, isARepeat: true)]),
            ("Command-Return", [try key("\r", code: 36, modifiers: .command)]),
            ("Option-Return", [try key("\r", code: 36, modifiers: .option)]),
            ("Command-period", [try key(".", code: 47, modifiers: .command)]),
            ("the letter a", [try key("a", code: 0)]),
        ]
        for tabs in 1...6 {
            presses.append(("\(tabs) Tab\(tabs == 1 ? "" : "s") and then Space", Array(repeating: tab, count: tabs) + [space]))
        }
        for (name, events) in presses {
            let sheet = ApprovalSheet(payload: payload(), deckName: "talk.md")
            var answers: [NSApplication.ModalResponse] = []
            host.beginSheet(sheet) { answers.append($0) }
            for event in events where host.attachedSheet === sheet { press(event, in: sheet) }
            XCTAssertFalse(answers.contains(.OK), "\(name) allowed")
            if host.attachedSheet === sheet { host.endSheet(sheet, returnCode: .abort) }
        }

        // A code label holding focus: Escape goes through its field editor,
        // and the field editor sends cancelOperation up to the sheet.
        let focused = ApprovalSheet(payload: payload(), deckName: "talk.md")
        var answers: [NSApplication.ModalResponse] = []
        host.beginSheet(focused) { answers.append($0) }
        focused.makeFirstResponder(focused.blockRows[0].codeLabel)
        press(try key("\u{1b}", code: 53), in: focused)
        if host.attachedSheet === focused { focused.cancelOperation(nil) }
        XCTAssertEqual(answers, [.cancel], "Escape from a focused label is Don't Allow")
    }

    /// A held Escape that ended a talk repeats into the deck's question the
    /// moment it shows; the repeat answers nothing, a fresh press declines.
    func testAHeldEscapeAnswersNothing() throws {
        let sheet = ApprovalSheet(payload: payload(), deckName: "talk.md")
        var answers: [NSApplication.ModalResponse] = []
        host.beginSheet(sheet) { answers.append($0) }
        sheet.keyDown(with: try key("\u{1b}", code: 53, isARepeat: true))
        XCTAssertEqual(answers, [])
        XCTAssertTrue(host.attachedSheet === sheet, "the question is still up")
        sheet.keyDown(with: try key("\u{1b}", code: 53))
        XCTAssertEqual(answers, [.cancel])
    }

    func testTheSafeButtonIsTheDefault() throws {
        let sheet = ApprovalSheet(payload: payload(), deckName: "talk.md")
        XCTAssertEqual(sheet.declineButton.title, "Don't Allow")
        XCTAssertEqual(sheet.acceptButton.title, "Allow")
        XCTAssertEqual(sheet.declineButton.keyEquivalent, "\r", "Return is Don't Allow")
        XCTAssertEqual(sheet.acceptButton.keyEquivalent, "", "no key grants execution")
        XCTAssertTrue(sheet.defaultButtonCell === sheet.declineButton.cell, "drawn as the default button")
        var answers: [NSApplication.ModalResponse] = []
        host.beginSheet(sheet) { answers.append($0) }
        // Return, routed the way AppKit routes a key equivalent: through the sheet's views.
        let handled = sheet.contentView?.performKeyEquivalent(with: try key("\r", code: 36)) ?? false
        XCTAssertTrue(handled, "a view in the sheet took Return")
        XCTAssertEqual(answers, [.cancel])

        // Escape reaches the sheet's own key handling, since Return already holds the one key equivalent.
        let second = ApprovalSheet(payload: payload(), deckName: "talk.md")
        host.beginSheet(second) { answers.append($0) }
        second.keyDown(with: try key("\u{1b}", code: 53))
        XCTAssertEqual(answers, [.cancel, .cancel])

        // Only a click on Allow answers yes.
        let third = ApprovalSheet(payload: payload(), deckName: "talk.md")
        host.beginSheet(third) { answers.append($0) }
        third.acceptButton.performClick(nil)
        XCTAssertEqual(answers, [.cancel, .cancel, .OK])
    }

    func testTheSheetListsTheDriversAndTheirBlocks() throws {
        let sheet = ApprovalSheet(payload: payload(), deckName: "talk.md")
        XCTAssertEqual(sheet.kind, "approval")
        XCTAssertEqual(sheet.accessibilityIdentifier(), "question-approval")
        XCTAssertEqual(sheet.titleLabel.stringValue, "This deck can run code on your Mac", "the spec's words (06-live-code-and-trust)")
        XCTAssertTrue(sheet.bodyLabel.stringValue.contains("\u{201C}talk.md\u{201D} declares 2 drivers and has 3 live code blocks"))
        XCTAssertTrue(sheet.bodyLabel.stringValue.contains("tap runs only the code written in this deck"), "no promise the app cannot keep")
        XCTAssertEqual(sheet.summaryLabel.stringValue, "2 shell, 1 sqlite")
        XCTAssertEqual(sheet.pathLabel.stringValue, "/private/tmp/t/talk.md")
        XCTAssertEqual(sheet.driverLabels.map(\.stringValue), ["shell: 2 blocks on slides 2, 5", "sqlite: 1 block on slide 4"])
        XCTAssertEqual(sheet.blockRows.map(\.placeLabel.stringValue), ["Slide 2, block 1", "Slide 5, block 1", "Slide 4, block 1"],
                       "each driver's blocks under it, in slide order")
        XCTAssertEqual(sheet.blockRows.map(\.codeLabel.stringValue), ["echo hello from slide 2", "echo five", "SELECT 1 AS one;"],
                       "the code inline, as the Approval board draws it: nothing to click before reading it")
        XCTAssertTrue(sheet.blockRows.allSatisfy { !$0.codeLabel.isHidden })
        XCTAssertEqual(sheet.wording, .first)
    }

    func testANewDriverAsksOnlyForItself() {
        let sheet = ApprovalSheet(payload: QuestionPayload(
            deck: "/private/tmp/t/talk.md",
            drivers: [ApprovalDriver(name: "shell", slides: [7], blocks: 1)],
            approvedBefore: ["sqlite"],
            blocks: [ApprovalBlock(driver: "shell", code: "tail -n 20 errors.log", slide: 7, block: 1)]), deckName: "talk.md")
        XCTAssertEqual(sheet.titleLabel.stringValue, "This deck now also wants to run shell")
        XCTAssertTrue(sheet.bodyLabel.stringValue.hasPrefix("You allowed sqlite for \u{201C}talk.md\u{201D} before. The deck now declares shell too"))
        XCTAssertEqual(sheet.acceptButton.title, "Allow shell")
        XCTAssertEqual(sheet.declineButton.title, "Don't Allow")
        XCTAssertEqual(sheet.summaryLabel.stringValue, "1 shell")
        XCTAssertEqual(sheet.blockRows.count, 1)
        XCTAssertEqual(sheet.wording, .newDrivers)
    }

    /// The ApprovalCommandChanged board: the badge on the row, the command
    /// before (struck out) and now, the code inline, and what stays allowed.
    func testAChangedCommandSaysSo() throws {
        let sheet = ApprovalSheet(payload: QuestionPayload(
            deck: "/private/tmp/t/talk.md",
            drivers: [ApprovalDriver(name: "fortune", command: "/usr/bin/true", previousCommand: "/bin/cat", slides: [3], blocks: 1)],
            approvedBefore: ["sqlite"],
            blocks: [ApprovalBlock(driver: "fortune", code: "hello", slide: 3, block: 1)]), deckName: "talk.md")
        XCTAssertEqual(sheet.wording, .changedCommands)
        XCTAssertEqual(sheet.titleLabel.stringValue, "The command for fortune changed")
        XCTAssertEqual(sheet.bodyLabel.stringValue, "You allowed fortune for \u{201C}talk.md\u{201D} when it ran a different command. "
                       + "The deck now runs the command below, for example after a git pull. Read it before you allow it.")
        XCTAssertEqual(sheet.acceptButton.title, "Allow fortune")
        XCTAssertEqual(sheet.declineButton.title, "Don't Allow")
        XCTAssertEqual(sheet.driverLabels.map(\.stringValue), ["fortune: 1 block on slide 3, command changed"])
        XCTAssertEqual(sheet.commandChangeLabels.map(\.stringValue), ["Before: /bin/cat", "Now: /usr/bin/true"])
        let before = sheet.commandChangeLabels[0].attributedStringValue
        XCTAssertEqual(before.attribute(.strikethroughStyle, at: 0, effectiveRange: nil) as? Int, NSUnderlineStyle.single.rawValue,
                       "the command approved before is struck out")
        XCTAssertNil(sheet.commandChangeLabels[1].attributedStringValue.attribute(.strikethroughStyle, at: 0, effectiveRange: nil))
        XCTAssertEqual(sheet.blockRows.map(\.codeLabel.stringValue), ["hello"], "the code inline under the row")
        XCTAssertEqual(sheet.footerLabel?.stringValue, "The code goes to the command on its standard input. sqlite stays allowed.")
        XCTAssertEqual(sheet.declineButton.keyEquivalent, "\r", "Return is still Don't Allow")
        XCTAssertEqual(sheet.acceptButton.keyEquivalent, "")

        // A deck whose only driver changed: nothing else was approved, so the footer names nothing that stays.
        let only = ApprovalSheet(payload: QuestionPayload(
            deck: "/t/talk.md", drivers: [ApprovalDriver(name: "fortune", command: "/usr/bin/true", previousCommand: "/bin/cat", slides: [3], blocks: 1)],
            blocks: []), deckName: "talk.md")
        XCTAssertEqual(only.wording, .changedCommands, "not the first-time wording: the deck was approved before")
        XCTAssertEqual(only.footerLabel?.stringValue, "The code goes to the command on its standard input.")

        // Without the field (a tap that does not send it), the new-driver wording stands.
        let without = ApprovalSheet(payload: QuestionPayload(deck: "/t/talk.md", drivers: [ApprovalDriver(name: "fortune", command: "/usr/bin/true", slides: [3], blocks: 1)],
                                                             approvedBefore: ["sqlite"], blocks: []), deckName: "talk.md")
        XCTAssertEqual(without.wording, .newDrivers)
        XCTAssertEqual(without.commandChangeLabels, [])
        XCTAssertNil(without.footerLabel)
        XCTAssertEqual(without.driverLabels.map(\.stringValue), ["fortune: 1 block on slide 3, runs: /usr/bin/true"])
    }

    /// tap's valueChanged: the command as written is the same, a value in
    /// it changed (or tap lost its approval key). The board's layout, tap's
    /// words, and no struck-out line, since there is no other command.
    func testAChangedValueSaysSo() {
        let sheet = ApprovalSheet(payload: QuestionPayload(
            deck: "/private/tmp/t/env.md",
            drivers: [ApprovalDriver(name: "echoer", command: "/bin/sh -c cat; : ${TAP_TEST_TOKEN}", valueChanged: true, slides: [2], blocks: 1)],
            approvedBefore: ["sqlite", "shell"],
            blocks: [ApprovalBlock(driver: "echoer", code: "hello via env", slide: 2, block: 1)]), deckName: "env.md")
        XCTAssertEqual(sheet.wording, .valueChanged)
        XCTAssertEqual(sheet.titleLabel.stringValue, "A value in the command for echoer changed since it was approved")
        XCTAssertTrue(sheet.bodyLabel.stringValue.hasPrefix("You allowed echoer for \u{201C}env.md\u{201D} with the command below."), sheet.bodyLabel.stringValue)
        XCTAssertTrue(sheet.bodyLabel.stringValue.hasSuffix("Read it before you allow it."))
        XCTAssertEqual(sheet.acceptButton.title, "Allow echoer")
        XCTAssertEqual(sheet.declineButton.title, "Don't Allow")
        XCTAssertEqual(sheet.driverLabels.map(\.stringValue), ["echoer: 1 block on slide 2, value changed"])
        XCTAssertEqual(sheet.commandChangeLabels.map(\.stringValue), ["Now: /bin/sh -c cat; : ${TAP_TEST_TOKEN}"], "no Before line")
        XCTAssertNil(sheet.commandChangeLabels[0].attributedStringValue.attribute(.strikethroughStyle, at: 0, effectiveRange: nil))
        XCTAssertEqual(sheet.blockRows.map(\.codeLabel.stringValue), ["hello via env"])
        XCTAssertEqual(sheet.footerLabel?.stringValue, "The code goes to the command on its standard input. sqlite and shell stay allowed.")
    }

    /// The board's note: a request with a new driver and a changed one
    /// keeps the new-driver title, with the badge on the changed row.
    func testANewDriverAndAChangedCommandTogether() {
        let sheet = ApprovalSheet(payload: QuestionPayload(
            deck: "/private/tmp/t/talk.md",
            drivers: [ApprovalDriver(name: "fortune", command: "/usr/bin/true", previousCommand: "/bin/cat", slides: [3], blocks: 1),
                      ApprovalDriver(name: "shell", slides: [7], blocks: 1)],
            approvedBefore: ["sqlite"],
            blocks: [ApprovalBlock(driver: "fortune", code: "hello", slide: 3, block: 1),
                     ApprovalBlock(driver: "shell", code: "tail -n 20 errors.log", slide: 7, block: 1)]), deckName: "talk.md")
        XCTAssertEqual(sheet.wording, .newDrivers)
        XCTAssertEqual(sheet.titleLabel.stringValue, "This deck now also wants to run shell", "the title names only the new driver")
        XCTAssertTrue(sheet.bodyLabel.stringValue.hasPrefix("You allowed sqlite and fortune for \u{201C}talk.md\u{201D} before. The deck now declares shell too"),
                      sheet.bodyLabel.stringValue)
        XCTAssertEqual(sheet.acceptButton.title, "Allow fortune and shell", "a yes allows both")
        XCTAssertEqual(sheet.driverLabels.map(\.stringValue), ["fortune: 1 block on slide 3, command changed", "shell: 1 block on slide 7"])
        XCTAssertEqual(sheet.commandChangeLabels.map(\.stringValue), ["Before: /bin/cat", "Now: /usr/bin/true"])
        XCTAssertEqual(sheet.footerLabel?.stringValue, "The code goes to the command on its standard input. sqlite stays allowed.")
    }

    func testACustomDriverShowsItsCommand() {
        let sheet = ApprovalSheet(payload: QuestionPayload(
            deck: "/private/tmp/t/talk.md",
            drivers: [ApprovalDriver(name: "fortune", command: "/bin/cat", slides: [3], blocks: 1), ApprovalDriver(name: "sqlite", slides: [2], blocks: 1)],
            blocks: []), deckName: "talk.md")
        XCTAssertEqual(sheet.driverLabels.map(\.stringValue), ["fortune: 1 block on slide 3, runs: /bin/cat", "sqlite: 1 block on slide 2"])
        XCTAssertEqual(ApprovalSheet.joined(["shell", "sqlite", "mysql"]), "shell, sqlite and mysql")
        XCTAssertEqual(ApprovalSheet.joined(["shell"]), "shell")
    }

    func testALongSheetScrollsAndKeepsItsButtonsOnScreen() throws {
        var blocks: [ApprovalBlock] = []
        for slide in 1...40 { blocks.append(ApprovalBlock(driver: "shell", code: String(repeating: "echo line \(slide)\n", count: 8), slide: slide, block: 1)) }
        let sheet = ApprovalSheet(payload: QuestionPayload(deck: "/t/talk.md", drivers: [ApprovalDriver(name: "shell", slides: Array(1...40), blocks: 40)], blocks: blocks),
                                  deckName: "talk.md")
        host.beginSheet(sheet) { _ in }
        let screen = try XCTUnwrap(host.screen ?? NSScreen.screens.first)
        XCTAssertLessThanOrEqual(sheet.frame.height, screen.visibleFrame.height, "the rows scroll; the sheet does not grow past the display")
        XCTAssertLessThanOrEqual(sheet.detailScrollView.frame.height, ApprovalSheet.detailMaximumHeight + 1)
        for button in [sheet.declineButton, sheet.acceptButton] {
            let inWindow = button.convert(button.bounds, to: nil)
            XCTAssertTrue(sheet.contentView!.bounds.contains(inWindow), "\(button.title) is inside the sheet, not scrolled away")
        }
        XCTAssertEqual(sheet.summaryLabel.stringValue, "40 shell")
    }

    func testEscapeReachesTheSheetFromAFocusedLabel() throws {
        // A selectable code label can hold focus; Escape from it still declines, through cancelOperation.
        let sheet = ApprovalSheet(payload: payload(), deckName: "talk.md")
        var answers: [NSApplication.ModalResponse] = []
        host.beginSheet(sheet) { answers.append($0) }
        sheet.cancelOperation(nil)
        XCTAssertEqual(answers, [.cancel])
    }

    func testADriverWithNoBlocksSaysSo() {
        let sheet = ApprovalSheet(payload: QuestionPayload(deck: "/t/talk.md", drivers: [ApprovalDriver(name: "mysql", slides: [], blocks: 0)], blocks: []), deckName: "talk.md")
        XCTAssertEqual(sheet.driverLabels.map(\.stringValue), ["mysql: no blocks yet"], "tap's own words for a declared driver nothing uses")
    }

    func testTheOtherSheetsKeepReturnAsTheirYes() {
        let consent = QuestionSheet.consent(settingsPath: "/tmp/settings.yaml")
        XCTAssertEqual(consent.acceptButton.keyEquivalent, "\r")
        XCTAssertEqual(consent.declineButton.keyEquivalent, "\u{1b}")
        let keep = QuestionSheet.keepRecording(directory: "/tmp/run", segments: 1, size: "1 KB")
        XCTAssertEqual(keep.acceptButton.keyEquivalent, "\r")
        XCTAssertEqual(keep.declineButton.keyEquivalent, "", "D4: no key reaches Delete")
    }
}
