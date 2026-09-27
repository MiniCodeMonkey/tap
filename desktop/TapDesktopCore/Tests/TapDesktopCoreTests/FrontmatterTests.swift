import XCTest
@testable import TapDesktopCore

final class FrontmatterTests: XCTestCase {
    let deck = """
    ---
    title: Debugging Production at 3am
    theme: "terminal"
    slideNumbers: false
    # the live code drivers
    drivers:
      sqlite:
        connections:
          incident:
            path: ./incident.db

      shell: {}
    recording:
      output: recordings
    ---

    # One
    """

    func testReadsTheBlockAndItsEntries() throws {
        let frontmatter = Frontmatter(text: deck)
        let text = deck as NSString
        XCTAssertEqual(frontmatter.range, NSRange(location: 0, length: text.range(of: "---\n\n# One").location + 4), "the block runs through the closing line's newline")
        XCTAssertEqual(frontmatter.closingLocation, text.range(of: "---\n\n# One").location)
        XCTAssertEqual(frontmatter.lineEnding, "\n")
        XCTAssertEqual(frontmatter.entries.map(\.key), ["title", "theme", "slideNumbers", "drivers", "recording"])
        XCTAssertEqual(frontmatter.value(at: ["title"]), "Debugging Production at 3am")
        XCTAssertEqual(frontmatter.value(at: ["theme"]), "\"terminal\"")
        XCTAssertEqual(frontmatter.entry(at: ["theme"])?.unquotedValue, "terminal")
        XCTAssertEqual(frontmatter.value(at: ["slideNumbers"]), "false")
        let drivers = try XCTUnwrap(frontmatter.entry(at: ["drivers"]))
        XCTAssertNil(drivers.value, "a block opener has no scalar")
        XCTAssertEqual(drivers.children.map(\.key), ["sqlite", "shell"])
        XCTAssertEqual(drivers.children.map(\.indent), [2, 2])
        XCTAssertEqual(frontmatter.value(at: ["drivers", "shell"]), "{}")
        XCTAssertEqual(frontmatter.value(at: ["drivers", "sqlite", "connections", "incident", "path"]), "./incident.db")
        XCTAssertEqual(frontmatter.text(of: drivers), "drivers:\n  sqlite:\n    connections:\n      incident:\n        path: ./incident.db\n\n  shell: {}\n",
                       "an entry's text runs to its last child, blank lines inside included")
        XCTAssertEqual(frontmatter.value(at: ["recording", "output"]), "recordings")
        XCTAssertNil(frontmatter.value(at: ["author"]))
        let title = try XCTUnwrap(frontmatter.entry(at: ["title"]))
        XCTAssertEqual(text.substring(with: try XCTUnwrap(title.valueRange)), "Debugging Production at 3am")
        XCTAssertEqual(text.substring(with: title.range), "title: Debugging Production at 3am\n")
    }

    func testTheDeclaredDriversComeFromTheDriversMap() {
        XCTAssertEqual(Frontmatter(text: deck).declaredDrivers, ["sqlite", "shell"])
        XCTAssertTrue(Frontmatter(text: deck).declares(driver: "shell"))
        XCTAssertFalse(Frontmatter(text: deck).declares(driver: "mysql"))
        XCTAssertEqual(Frontmatter(text: "---\ntitle: x\n---\n# One\n").declaredDrivers, [])
        XCTAssertEqual(Frontmatter(text: "# One\n").declaredDrivers, [])
        XCTAssertEqual(Frontmatter(text: "---\ndrivers: {shell: {}, sqlite: {connections: {a: {path: x}}}}\n---\n").declaredDrivers, ["shell", "sqlite"],
                       "a flow map declares its keys too")
        XCTAssertEqual(Frontmatter(text: "---\ndrivers: {}\n---\n").declaredDrivers, [])
        XCTAssertEqual(Frontmatter(text: "---\ndrivers:\n---\n").declaredDrivers, [])
        XCTAssertEqual(Frontmatter(text: "---\ndrivers: {sqlite: {connections: {a: {path: x}, b: {path: y}}}}\n---\n").declaredDrivers, ["sqlite"],
                       "a comma nested inside a driver's own map is not a top-level split")
    }

    func testEntryNamesReadABlockOrAFlowMapAtAnyDepth() {
        let frontmatter = Frontmatter(text: deck)
        XCTAssertEqual(frontmatter.entryNames(at: ["drivers"]), ["sqlite", "shell"], "a block map, in the file's order")
        XCTAssertEqual(frontmatter.entryNames(at: ["drivers", "sqlite", "connections"]), ["incident"], "a nested block map")
        XCTAssertEqual(frontmatter.entryNames(at: ["drivers", "shell"]), [], "an empty flow map has no names")
        XCTAssertEqual(frontmatter.entryNames(at: ["title"]), [], "a scalar has no names")
        XCTAssertEqual(frontmatter.entryNames(at: ["author"]), [], "a missing key has no names")
        XCTAssertEqual(frontmatter.entryNames(at: ["drivers", "mysql"]), [], "a missing child has no names")
        let flow = Frontmatter(text: "---\ndrivers: {shell: {}, sqlite: {connections: {a: {path: x}, b: {path: y}}}}\n---\n")
        XCTAssertEqual(flow.entryNames(at: ["drivers"]), ["shell", "sqlite"], "a flow map's top-level keys, nested commas ignored")
        XCTAssertEqual(Frontmatter(text: "---\ndrivers:\n---\n").entryNames(at: ["drivers"]), [], "a key with nothing under it")
        XCTAssertEqual(Frontmatter(text: "# One\n").entryNames(at: ["drivers"]), [], "no frontmatter")
    }

    func testADeckWithoutFrontmatter() {
        let frontmatter = Frontmatter(text: "# One\n\n---\n\n# Two\n")
        XCTAssertNil(frontmatter.range, "a separator later in the deck is not a frontmatter")
        XCTAssertFalse(frontmatter.hasFrontmatter)
        XCTAssertEqual(frontmatter.entries, [])
        XCTAssertNil(Frontmatter(text: "").range)
        XCTAssertNil(Frontmatter(text: "---\ntitle: x\n").range, "a block that never closes is not one")
        XCTAssertNil(Frontmatter(text: "\n---\ntitle: x\n---\n").range, "the first line must be the opener, as tap reads it")
    }

    func testKeepsCarriageReturnLineEndings() {
        let windows = "---\r\ntitle: x\r\ndrivers:\r\n  shell: {}\r\n---\r\n\r\n# One\r\n"
        let frontmatter = Frontmatter(text: windows)
        XCTAssertEqual(frontmatter.lineEnding, "\r\n")
        XCTAssertEqual(frontmatter.value(at: ["title"]), "x")
        XCTAssertEqual(frontmatter.declaredDrivers, ["shell"])
        XCTAssertEqual(frontmatter.range?.length, (windows as NSString).range(of: "\r\n\r\n# One").location + 2)
    }

    func testCommentsBlankLinesAndOddSpacingAreNotEntries() {
        let text = "---\n# a comment\n\ntitle:   spaced   \nkey-with-dash: 1\ndotted.key: 2\ntabbed:\tvalue\nlist:\n  - one\n  - two\n---\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertEqual(frontmatter.entries.map(\.key), ["title", "key-with-dash", "dotted.key", "tabbed", "list"])
        XCTAssertEqual(frontmatter.value(at: ["title"]), "spaced")
        XCTAssertEqual(frontmatter.value(at: ["tabbed"]), "value")
        XCTAssertNil(frontmatter.value(at: ["list"]), "a list opener has no scalar")
        XCTAssertEqual(frontmatter.entry(at: ["list"])?.children, [], "list items are not entries")
        XCTAssertEqual(frontmatter.text(of: frontmatter.entry(at: ["list"])!), "list:\n  - one\n  - two\n")
        XCTAssertTrue(frontmatter.entry(at: ["list"])!.isMultiLine, "a value on more than one line is never edited as a scalar")
        XCTAssertFalse(frontmatter.entry(at: ["title"])!.isMultiLine)
    }

    func testATrailingCommentIsNotPartOfTheValue() throws {
        let text = "---\ntheme: base # dark later\ntitle: \"a # b\"\nquoted: 'x # y' # z\ncommented: # nothing\ndrivers: {shell: {}} # x\nurl: http://x#y\n---\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertEqual(frontmatter.value(at: ["theme"]), "base")
        XCTAssertEqual((text as NSString).substring(with: try XCTUnwrap(frontmatter.entry(at: ["theme"])?.valueRange)), "base", "a write keeps the comment")
        XCTAssertEqual(frontmatter.value(at: ["title"]), "\"a # b\"", "a hash inside quotes is text")
        XCTAssertEqual(frontmatter.value(at: ["quoted"]), "'x # y'")
        XCTAssertNil(frontmatter.value(at: ["commented"]), "a value that is only a comment is none")
        XCTAssertEqual(frontmatter.declaredDrivers, ["shell"], "the comment does not hide the flow map")
        XCTAssertEqual(frontmatter.value(at: ["url"]), "http://x#y", "a hash with no space before it is not a comment")
    }

    func testABlockScalarIsNotAScalar() {
        let text = "---\ntitle: >-\n  Debugging Production\n  at 3am\nauthor: |\n  Me\ntheme: base\n---\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertEqual(frontmatter.entries.map(\.key), ["title", "author", "theme"])
        let title = frontmatter.entry(at: ["title"])!
        XCTAssertEqual(title.value, ">-")
        XCTAssertTrue(title.isMultiLine)
        XCTAssertEqual(frontmatter.text(of: title), "title: >-\n  Debugging Production\n  at 3am\n")
        XCTAssertTrue(frontmatter.entry(at: ["author"])!.isMultiLine)
        XCTAssertFalse(frontmatter.entry(at: ["theme"])!.isMultiLine)
        XCTAssertNil(Frontmatter(text: "---\nnotes: |\n  name: x\n---\n").value(at: ["notes", "name"]), "a block scalar's text holds no keys")
    }

    func testUnquoting() {
        XCTAssertEqual(Frontmatter.unquoted("plain"), "plain")
        XCTAssertEqual(Frontmatter.unquoted("\"a \\\"b\\\" c\""), "a \"b\" c")
        XCTAssertEqual(Frontmatter.unquoted("\"back\\\\slash\""), "back\\slash")
        XCTAssertEqual(Frontmatter.unquoted("'it''s'"), "it's")
        XCTAssertEqual(Frontmatter.unquoted("\"\""), "")
        XCTAssertEqual(Frontmatter.unquoted("\"unterminated"), "\"unterminated")
    }

    /// `replacement` applied to `text`, for reading the result.
    func applied(_ replacement: TextReplacement?, to text: String) throws -> String {
        Frontmatter.applying(try XCTUnwrap(replacement), to: text)
    }

    func testSetsAScalarInPlace() throws {
        let text = "---\ntitle: Old\ntheme: base\n---\n\n# One\n"
        let result = try applied(Frontmatter(text: text).setting(path: ["theme"], to: "midnight"), to: text)
        XCTAssertEqual(result, "---\ntitle: Old\ntheme: midnight\n---\n\n# One\n")
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["title"], to: "\"New: title\""), to: text),
                       "---\ntitle: \"New: title\"\ntheme: base\n---\n\n# One\n")
    }

    func testAddsAMissingKeyBeforeTheClosingLine() throws {
        let text = "---\ntitle: Old\n---\n# One\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["author"], to: "Me"), to: text), "---\ntitle: Old\nauthor: Me\n---\n# One\n")
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["recording", "output"], to: "out"), to: text),
                       "---\ntitle: Old\nrecording:\n  output: out\n---\n# One\n", "a missing parent is made")
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["drivers", "sqlite", "timeout"], to: "5"), to: text),
                       "---\ntitle: Old\ndrivers:\n  sqlite:\n    timeout: 5\n---\n# One\n")
    }

    func testAddsAChildAtTheEndOfItsParentsBlock() throws {
        let text = "---\nrecording:\n  output: out\n\n# a comment between\ntheme: base\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["recording", "audio"], to: "none"), to: text),
                       "---\nrecording:\n  output: out\n  audio: none\n\n# a comment between\ntheme: base\n---\n",
                       "right below the last child, before the gap")
        let deeper = "---\ndrivers:\n  sqlite:\n      timeout: 5\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: deeper).setting(path: ["drivers", "sqlite", "command"], to: "x"), to: deeper),
                       "---\ndrivers:\n  sqlite:\n      timeout: 5\n      command: x\n---\n", "the siblings' indent, whatever it is")
    }

    func testMakesTheFrontmatterWhenThereIsNone() throws {
        let text = "# One\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["title"], to: "T"), to: text), "---\ntitle: T\n---\n\n# One\n")
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["drivers", "shell"], to: "{}"), to: text), "---\ndrivers:\n  shell: {}\n---\n\n# One\n")
        XCTAssertNil(Frontmatter(text: text).setting(path: ["title"], to: nil), "removing from nothing is nothing")
    }

    func testRemovesAKeyWithEverythingUnderIt() throws {
        let text = "---\ntitle: T\ndrivers:\n  sqlite:\n    timeout: 5\n  shell: {}\ntheme: base\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["drivers", "sqlite"], to: nil), to: text),
                       "---\ntitle: T\ndrivers:\n  shell: {}\ntheme: base\n---\n")
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["drivers"], to: nil), to: text), "---\ntitle: T\ntheme: base\n---\n")
        XCTAssertNil(Frontmatter(text: text).setting(path: ["author"], to: nil))
    }

    func testAMultiLineValueIsReplacedWhole() throws {
        let text = "---\ntitle: >-\n  Debugging Production\n  at 3am\ntheme: base\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["title"], to: "Short"), to: text), "---\ntitle: Short\ntheme: base\n---\n",
                       "the continuation lines go with the value, never left behind for tap to choke on")
        let commented = "---\ntheme: base # dark later\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: commented).setting(path: ["theme"], to: "midnight"), to: commented), "---\ntheme: midnight # dark later\n---\n",
                       "a trailing comment stays")
    }

    func testABlockBecomesAScalarAndBack() throws {
        let text = "---\nrecording:\n  output: out\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["recording"], to: "{}"), to: text), "---\nrecording: {}\n---\n")
        let bare = "---\ndrivers:\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: bare).setting(path: ["drivers", "shell"], to: "{}"), to: bare), "---\ndrivers:\n  shell: {}\n---\n",
                       "a bare key opens into a block")
    }

    func testAFlowMapGainsAPair() throws {
        let empty = "---\ndrivers: {}\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: empty).setting(path: ["drivers", "shell"], to: "{}"), to: empty), "---\ndrivers:\n  shell: {}\n---\n",
                       "an empty flow map opens into a block")
        let full = "---\ndrivers: {shell: {}}\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: full).setting(path: ["drivers", "sqlite"], to: "{}"), to: full), "---\ndrivers: {shell: {}, sqlite: {}}\n---\n",
                       "a flow map with pairs keeps its shape")
        XCTAssertNil(Frontmatter(text: full).setting(path: ["drivers", "shell", "timeout"], to: "5"), "two levels into a flow map is not rewritten")
        XCTAssertNil(Frontmatter(text: full).setting(path: ["drivers", "shell"], to: "{timeout: 5}"), "a pair inside a flow map is not rewritten")
    }

    func testAddingADriver() throws {
        let text = "---\ntitle: T\ndrivers:\n  sqlite: {}\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).addingDriver("shell"), to: text), "---\ntitle: T\ndrivers:\n  sqlite: {}\n  shell: {}\n---\n")
        XCTAssertNil(Frontmatter(text: text).addingDriver("sqlite"), "already declared")
        let none = "---\ntitle: T\n---\n\n# One\n"
        XCTAssertEqual(try applied(Frontmatter(text: none).addingDriver("sqlite"), to: none), "---\ntitle: T\ndrivers:\n  sqlite: {}\n---\n\n# One\n")
        XCTAssertEqual(try applied(Frontmatter(text: "# One\n").addingDriver("shell"), to: "# One\n"), "---\ndrivers:\n  shell: {}\n---\n\n# One\n")
    }

    func testCommentsAndBlankLinesStayWhereTheyAre() throws {
        let text = "---\r\n# who\r\ntitle: T\r\n\r\ndrivers:\r\n  # the database\r\n  sqlite: {}\r\n---\r\n\r\n# One\r\n"
        let result = try applied(Frontmatter(text: text).addingDriver("shell"), to: text)
        XCTAssertEqual(result, "---\r\n# who\r\ntitle: T\r\n\r\ndrivers:\r\n  # the database\r\n  sqlite: {}\r\n  shell: {}\r\n---\r\n\r\n# One\r\n",
                       "the file's own line ending, and nothing else moved")
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["author"], to: "Me"), to: text),
                       "---\r\n# who\r\ntitle: T\r\n\r\ndrivers:\r\n  # the database\r\n  sqlite: {}\r\nauthor: Me\r\n---\r\n\r\n# One\r\n")
    }

    func testRawBlocks() throws {
        let text = "---\ndrivers:\n  sqlite:\n    connections:\n      a:\n        path: x\n  shell: {}\n---\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertEqual(frontmatter.rawBlock(at: ["drivers", "sqlite", "connections"]), "    connections:\n      a:\n        path: x\n")
        XCTAssertEqual(try applied(frontmatter.settingRawBlock(at: ["drivers", "sqlite", "connections"], to: "    connections:\n      b:\n        path: y\n"), to: text),
                       "---\ndrivers:\n  sqlite:\n    connections:\n      b:\n        path: y\n  shell: {}\n---\n")
        XCTAssertNil(frontmatter.rawBlock(at: ["drivers", "mysql"]))
    }

    func testScalarsAreQuotedOnlyWhenYAMLWouldReadThemOtherwise() {
        XCTAssertEqual(Frontmatter.scalar(forString: "Debugging Production at 3am"), "Debugging Production at 3am")
        XCTAssertEqual(Frontmatter.scalar(forString: "2026-01-25"), "2026-01-25")
        XCTAssertEqual(Frontmatter.scalar(forString: "true"), "\"true\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "42"), "\"42\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "3.5"), "\"3.5\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "a: b"), "\"a: b\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "a # b"), "\"a # b\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "#hash"), "\"#hash\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "- dash"), "\"- dash\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "say \"hi\""), "\"say \\\"hi\\\"\"")
        XCTAssertEqual(Frontmatter.scalar(forString: " padded"), "\" padded\"")
        XCTAssertEqual(Frontmatter.scalar(forString: ""), "\"\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "null"), "\"null\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "no"), "\"no\"")
        XCTAssertEqual(Frontmatter.unquoted(Frontmatter.scalar(forString: "round \\ trip: \"x\"")), "round \\ trip: \"x\"")
    }
    // MARK: - Lines the key pattern does not know stay out of every entry

    func testKeysOutsideThePlainPatternAreTheirOwnEntries() throws {
        let text = "---\ntitle: T\nårstal: 2026\n\"author\": Me\nog:image: x\ntitle2 : Spaced\ntheme: base\n---\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertEqual(frontmatter.entries.map(\.key), ["title", "årstal", "author", "og:image", "title2", "theme"])
        XCTAssertFalse(try XCTUnwrap(frontmatter.entry(at: ["title"])).isMultiLine, "the line below is not a continuation")
        XCTAssertEqual(try applied(frontmatter.setting(path: ["title"], to: "New"), to: text),
                       "---\ntitle: New\nårstal: 2026\n\"author\": Me\nog:image: x\ntitle2 : Spaced\ntheme: base\n---\n")
        XCTAssertEqual(frontmatter.value(at: ["author"]), "Me")
    }

    func testSettingTheLineAboveAnUnknownKeyKeepsIt() throws {
        for (text, path, expected) in [
            ("---\ntitle: T\nårstal: 2026\ntheme: base\n---\n", ["title"], "---\ntitle: New\nårstal: 2026\ntheme: base\n---\n"),
            ("---\ntitle: T\n\"author\": Me\n---\n", ["title"], "---\ntitle: New\n\"author\": Me\n---\n"),
            ("---\ntheme: base\nog:image: x.png\n---\n", ["theme"], "---\ntheme: New\nog:image: x.png\n---\n"),
            ("---\ntheme: base\ntitle : Spaced\n---\n", ["theme"], "---\ntheme: New\ntitle : Spaced\n---\n"),
        ] {
            XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: path, to: "New"), to: text), expected)
        }
    }

    func testAnUnrecognisedLineIsNeverAbsorbed() throws {
        // A line no key reading accepts, at the entry's own indent, ends the entry and is left alone.
        let text = "---\ntitle: T\n[odd]\ntheme: base\n---\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertFalse(try XCTUnwrap(frontmatter.entry(at: ["title"])).isMultiLine)
        XCTAssertEqual(try applied(frontmatter.setting(path: ["title"], to: nil), to: text), "---\n[odd]\ntheme: base\n---\n")
    }

    func testRemovingADriverKeepsAQuotedSibling() throws {
        let text = "---\ndrivers:\n  sqlite: {}\n  \"my-db\": {}\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).setting(path: ["drivers", "sqlite"], to: nil), to: text),
                       "---\ndrivers:\n  \"my-db\": {}\n---\n")
        XCTAssertEqual(Frontmatter(text: text).declaredDrivers, ["sqlite", "my-db"])
    }

    func testTheFixItOnAnEmptyFlowMapKeepsTheLineBelow() throws {
        let text = "---\ndrivers: {}\nog:image: x.png\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).addingDriver("shell"), to: text), "---\ndrivers:\n  shell: {}\nog:image: x.png\n---\n")
        let theme = "---\ntheme: base\nog:image: x.png\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: theme).addingDriver("shell"), to: theme), "---\ntheme: base\nog:image: x.png\ndrivers:\n  shell: {}\n---\n")
        XCTAssertEqual(try applied(Frontmatter(text: theme).setting(path: ["theme"], to: nil), to: theme), "---\nog:image: x.png\n---\n")
    }

    func testAListUnderAKeyAtTheKeysOwnIndentBelongsToIt() throws {
        let text = "---\nlist:\n- one\n- two\ntheme: base\n---\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertEqual(frontmatter.text(of: try XCTUnwrap(frontmatter.entry(at: ["list"]))), "list:\n- one\n- two\n")
        XCTAssertEqual(try applied(frontmatter.setting(path: ["list"], to: nil), to: text), "---\ntheme: base\n---\n")
        XCTAssertNil(frontmatter.setting(path: ["list", "key"], to: "x"), "a list has no map to add a key to")
    }

    /// Every line of the frontmatter outside the edited entry is in the
    /// result, byte for byte and in order, for a spread of edits on a deck
    /// that mixes every kind of line.
    func testAnEditChangesOnlyItsOwnEntry() throws {
        let lines = [
            "---",                                   // 0
            "# who wrote this",                      // 1
            "title: Let's go # draft",               // 2
            "årstal: 2026",                          // 3
            "\"author\": Me",                        // 4
            "og:image: x.png",                       // 5
            "theme : base",                          // 6
            "notes: >-",                             // 7
            "  first",                               // 8
            "  second",                              // 9
            "list:",                                 // 10
            "- one",                                 // 11
            "  - nested",                            // 12
            "",                                      // 13
            "drivers:",                              // 14
            "  # the database",                      // 15
            "  sqlite:",                             // 16
            "    timeout: 5",                        // 17
            "  \"my-db\": {}",                       // 18
            "  shell: {} # built in",                // 19
            "[odd]",                                 // 20
            "recording:",                            // 21
            "  output: out",                         // 22
            "flow: {a: 1,",                          // 23
            "  b: 2}",                               // 24
            "tabbed:\tvalue\t# note",                // 25
            "---",                                   // 26
        ]
        let text = lines.joined(separator: "\n") + "\n\n# One\n"
        let edits: [(path: [String], value: String?, owned: Set<Int>)] = [
            (["title"], "New", [2]),
            (["title"], nil, [2]),
            (["årstal"], "2027", [3]),
            (["author"], "You", [4]),
            (["author"], nil, [4]),
            (["og:image"], nil, [5]),
            (["theme"], "midnight", [6]),
            (["notes"], "short", [7, 8, 9]),
            (["notes"], nil, [7, 8, 9]),
            (["list"], nil, [10, 11, 12]),
            (["drivers", "sqlite"], nil, [16, 17]),
            (["drivers", "sqlite", "timeout"], "9", [17]),
            (["drivers", "my-db"], nil, [18]),
            (["drivers", "shell"], nil, [19]),
            (["drivers", "mysql"], "{}", []),
            (["drivers"], nil, [14, 15, 16, 17, 18, 19]),
            (["recording", "output"], "elsewhere", [22]),
            (["recording", "audio"], "none", []),
            (["recording"], nil, [21, 22]),
            (["flow"], nil, [23, 24]),
            (["tabbed"], "other", [25]),
            (["author2"], "new", []),
        ]
        for edit in edits {
            let frontmatter = Frontmatter(text: text)
            let replacement = try XCTUnwrap(frontmatter.setting(path: edit.path, to: edit.value), "\(edit.path)")
            let result = Frontmatter.applying(replacement, to: text)
            let resultLines = result.components(separatedBy: "\n")
            var position = 0
            for (index, line) in lines.enumerated() where !edit.owned.contains(index) {
                guard let found = resultLines[position...].firstIndex(of: line) else {
                    XCTFail("\(edit.path) = \(edit.value ?? "nil") lost line \(index): \(line)")
                    continue
                }
                position = found + 1
            }
            XCTAssertTrue(result.hasSuffix("---\n\n# One\n"), "\(edit.path): the body is untouched")
        }
    }
    // MARK: - Comments

    func testAnApostropheInAPlainValueOpensNoQuote() throws {
        let text = "---\ntitle: Let's go # draft title\ntheme: base\n---\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertEqual(frontmatter.value(at: ["title"]), "Let's go")
        XCTAssertEqual(try applied(frontmatter.setting(path: ["title"], to: "New"), to: text), "---\ntitle: New # draft title\ntheme: base\n---\n")
        XCTAssertEqual(Frontmatter(text: "---\ntitle: it's \"x\" # c\n---\n").value(at: ["title"]), "it's \"x\"")
    }

    func testATabBeforeTheHashStartsAComment() throws {
        let text = "---\ntheme: base\t# dark later\n---\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertEqual(frontmatter.value(at: ["theme"]), "base")
        XCTAssertEqual(try applied(frontmatter.setting(path: ["theme"], to: "midnight"), to: text), "---\ntheme: midnight\t# dark later\n---\n")
    }

    func testAnEscapedQuoteDoesNotCloseTheValue() throws {
        let text = "---\ntitle: \"a \\\" # b\" # c\n---\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertEqual(frontmatter.value(at: ["title"]), "\"a \\\" # b\"")
        XCTAssertEqual(frontmatter.entry(at: ["title"])?.unquotedValue, "a \" # b")
        XCTAssertEqual(try applied(frontmatter.setting(path: ["title"], to: "New"), to: text), "---\ntitle: New # c\n---\n")
        XCTAssertEqual(Frontmatter(text: "---\ntitle: 'it''s # x' # y\n---\n").value(at: ["title"]), "'it''s # x'", "'' is a quote inside single quotes")
    }

    // MARK: - Edits that must not corrupt

    func testAFlowMapOverSeveralLinesIsOneEntry() throws {
        let text = "---\ndrivers: {\n  shell: {}\n}\ntheme: base\n---\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertEqual(frontmatter.entries.map(\.key), ["drivers", "theme"])
        XCTAssertEqual(frontmatter.declaredDrivers, ["shell"])
        XCTAssertNil(frontmatter.setting(path: ["drivers", "shell"], to: nil), "nothing is removed from inside a flow map")
        XCTAssertNil(frontmatter.addingDriver("sqlite"), "nor added to it")
        XCTAssertNil(frontmatter.addingDriver("shell"), "already declared")
        XCTAssertEqual(try applied(frontmatter.setting(path: ["drivers"], to: nil), to: text), "---\ntheme: base\n---\n", "the whole map goes, closing brace too")
    }

    func testANullDriversValueOpensIntoABlock() throws {
        for null in ["~", "null", "Null"] {
            let text = "---\ndrivers: \(null)\ntheme: base\n---\n"
            XCTAssertEqual(Frontmatter(text: text).declaredDrivers, [])
            XCTAssertEqual(try applied(Frontmatter(text: text).addingDriver("shell"), to: text), "---\ndrivers:\n  shell: {}\ntheme: base\n---\n")
        }
    }

    func testTheFixItKeepsTheCommentOnAnEmptyFlowMap() throws {
        let text = "---\ndrivers: {} # none yet\n---\n"
        XCTAssertEqual(try applied(Frontmatter(text: text).addingDriver("shell"), to: text), "---\ndrivers: # none yet\n  shell: {}\n---\n")
        let null = "---\r\ndrivers: ~\t# later\r\n---\r\n"
        XCTAssertEqual(try applied(Frontmatter(text: null).addingDriver("shell"), to: null), "---\r\ndrivers:\t# later\r\n  shell: {}\r\n---\r\n")
    }

    func testLineEndingsFollowTheLinesAnEditLandsBeside() throws {
        let mixed = "---\ntitle: T\r\ndrivers:\r\n  sqlite: {}\r\n---\r\n"
        XCTAssertEqual(Frontmatter(text: mixed).lineEnding, "\r\n", "most lines end in CRLF")
        XCTAssertEqual(try applied(Frontmatter(text: mixed).addingDriver("shell"), to: mixed), "---\ntitle: T\r\ndrivers:\r\n  sqlite: {}\r\n  shell: {}\r\n---\r\n")
        let lastLF = "---\r\ntitle: T\r\nnotes: >-\r\n  a\n---\r\n"
        XCTAssertEqual(try applied(Frontmatter(text: lastLF).setting(path: ["notes"], to: "b"), to: lastLF), "---\r\ntitle: T\r\nnotes: b\n---\r\n",
                       "a replaced entry keeps its own last ending")
        let beforeClosing = "---\r\ntitle: T\n---\r\n"
        XCTAssertEqual(try applied(Frontmatter(text: beforeClosing).setting(path: ["theme"], to: "x"), to: beforeClosing), "---\r\ntitle: T\ntheme: x\n---\r\n",
                       "a new line ends like the line above it, not like most lines")
    }

    func testAValueStartingWithANonBreakingSpaceKeepsItsRange() throws {
        let text = "---\ntitle: \u{00A0}x\n---\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertEqual(frontmatter.value(at: ["title"]), "\u{00A0}x", "YAML does not count a no-break space as white space")
        XCTAssertEqual(try applied(frontmatter.setting(path: ["title"], to: "New"), to: text), "---\ntitle: New\n---\n")
    }

    func testAnUnterminatedFrontmatterIsNotEdited() {
        let text = "---\ntitle: x\n\n# One\n"
        let frontmatter = Frontmatter(text: text)
        XCTAssertTrue(frontmatter.isUnterminated)
        XCTAssertNil(frontmatter.addingDriver("shell"))
        XCTAssertNil(frontmatter.setting(path: ["title"], to: "y"))
        XCTAssertFalse(Frontmatter(text: "# One\n").isUnterminated)
        XCTAssertFalse(Frontmatter(text: "").isUnterminated)
        XCTAssertFalse(Frontmatter(text: "---\n---\n").isUnterminated)
    }

    func testAQuotedFlowMapKeyIsDeclared() {
        let text = "---\ndrivers: {\"shell\": {}, 'sqlite': {}}\n---\n"
        XCTAssertEqual(Frontmatter(text: text).declaredDrivers, ["shell", "sqlite"])
        XCTAssertNil(Frontmatter(text: text).addingDriver("shell"), "no duplicate key")
    }

    func testLineBreaksOfEveryKindAreQuoted() {
        XCTAssertEqual(Frontmatter.scalar(forString: "a\r\nb"), "\"a\\r\\nb\"", "\"\\r\\n\" is one Character in Swift")
        XCTAssertEqual(Frontmatter.scalar(forString: "a\rb"), "\"a\\rb\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "a\nb"), "\"a\\nb\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "a\u{2028}b"), "\"a\\Lb\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "a\u{2029}b"), "\"a\\Pb\"")
        XCTAssertEqual(Frontmatter.scalar(forString: "a\u{85}b"), "\"a\\Nb\"")
        for string in ["a\r\nb", "a\rb", "a\u{2028}b", "x\u{2029}\ny"] {
            XCTAssertEqual(Frontmatter.unquoted(Frontmatter.scalar(forString: string)), string)
        }
    }
}
