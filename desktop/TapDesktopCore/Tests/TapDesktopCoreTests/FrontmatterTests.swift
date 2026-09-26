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
}
