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
}
