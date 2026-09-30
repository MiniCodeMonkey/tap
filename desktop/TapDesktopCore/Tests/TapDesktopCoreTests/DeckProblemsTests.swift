import XCTest
@testable import TapDesktopCore

final class DeckProblemsTests: XCTestCase {
    let schema: [SchemaKey] = [
        SchemaKey(name: "author", type: "string"),
        SchemaKey(name: "theme", type: "string", defaultValue: "base", values: ["base", "keynote", "swiss", "terminal"]),
        SchemaKey(name: "aspectRatio", type: "string", defaultValue: "16:9", values: ["16:9", "4:3", "16:10"]),
        SchemaKey(name: "transition", type: "string", defaultValue: "fade", values: ["none", "fade", "slide"]),
        SchemaKey(name: "recording", type: "object", keys: [SchemaKey(name: "audio", type: "string", values: ["default", "none"])]),
    ]

    func problems(_ frontmatter: String) -> [DeckProblem] {
        DeckProblems.evaluate(Frontmatter(text: "---\n\(frontmatter)---\n\n# One\n"), schema: schema)
    }

    // MARK: Evaluating

    func testAValidDeckHasNoProblems() {
        XCTAssertEqual(problems("theme: keynote\naspectRatio: \"16:9\"\nauthor: Ada\n"), [])
        XCTAssertEqual(DeckProblems.evaluate(Frontmatter(text: "# One\n"), schema: schema), [], "no frontmatter is no problem")
    }

    func testAnUnknownThemeIsAWarningNamingTheNearestTheme() {
        let found = problems("theme: Keynot\n")
        XCTAssertEqual(found.count, 1)
        XCTAssertEqual(found[0].severity, .warning)
        XCTAssertEqual(found[0].key, "theme")
        XCTAssertEqual(found[0].value, "Keynot")
        XCTAssertEqual(found[0].suggestions, ["keynote"])
        XCTAssertEqual(found[0].message, "\u{201C}Keynot\u{201D} is not a tap theme. The preview uses Base for now.")
        XCTAssertFalse(DeckProblems.hasErrors(found))
    }

    func testAWrongAspectRatioIsAnErrorWithTheNormalizedValueFirst() {
        for written in ["16/9", "16x9", "\"16/9\""] {
            let found = problems("aspectRatio: \(written)\n")
            XCTAssertEqual(found.first?.severity, .error, written)
            XCTAssertEqual(found.first?.suggestions.first, "16:9", written)
            XCTAssertEqual(found.first?.allowed, ["16:9", "4:3", "16:10"], "the allowed values come from the schema")
            XCTAssertTrue(DeckProblems.hasErrors(found))
        }
        XCTAssertEqual(problems("aspectRatio: 16/9\n").first?.message, "The aspect ratio \u{201C}16/9\u{201D} is not one tap supports. Use 16:9, 4:3 or 16:10.")
    }

    func testTwoProblemsAreListedInSchemaOrderAndNestedKeysAreChecked() {
        let found = problems("aspectRatio: 5:4\ntheme: nope\nrecording:\n  audio: loud\n")
        XCTAssertEqual(found.map(\.key), ["theme", "aspectRatio", "recording.audio"])
        XCTAssertEqual(found.map(\.severity), [.warning, .error, .error])
        XCTAssertEqual(found[1].suggestions, [], "5:4 is not close to anything")
    }

    func testAKeyTapDoesNotLimitIsNeverAProblem() {
        XCTAssertEqual(problems("author: anything at all\ntitle: x\n"), [])
    }

    // MARK: Suggestions

    func testNearestValues() {
        let ratios = ["16:9", "4:3", "16:10"]
        XCTAssertEqual(DeckProblems.nearestValues(to: "16/9", in: ratios), ["16:9"])
        XCTAssertEqual(DeckProblems.nearestValues(to: "4x3", in: ratios), ["4:3"])
        XCTAssertEqual(DeckProblems.nearestValues(to: "16:11", in: ratios), ["16:10"])
        XCTAssertEqual(DeckProblems.nearestValues(to: "banana", in: ratios), [])
        XCTAssertEqual(DeckProblems.nearestValues(to: "fadee", in: ["none", "fade", "slide"]), ["fade"])
        XCTAssertEqual(DeckProblems.nearestValues(to: "", in: ratios), [])
    }

    func testTheClosestComesFirstAndAtMostThreeAreOffered() {
        let names = ["terminals", "terminus", "termite", "terminal", "zzz"]
        let nearest = DeckProblems.nearestValues(to: "termina", in: names)
        XCTAssertEqual(nearest.count, 3)
        XCTAssertEqual(nearest[0], "terminal")
    }

    // MARK: Fixes

    func testTheFixRewritesTheSettingAsOneEdit() throws {
        let text = "---\ntheme: keynote\naspectRatio: 16/9\n---\n\n# One\n"
        let frontmatter = Frontmatter(text: text)
        let problem = try XCTUnwrap(DeckProblems.evaluate(frontmatter, schema: schema).first)
        let fix = try XCTUnwrap(DeckProblems.fix(for: problem, in: frontmatter, schema: schema))
        XCTAssertEqual(fix.title, "Use 16:9")
        XCTAssertEqual(fix.actionName, "Use 16:9")
        let fixed = Frontmatter.applying(fix.replacement, to: text)
        XCTAssertEqual(fixed, "---\ntheme: keynote\naspectRatio: 16:9\n---\n\n# One\n", "only the value changes")
        XCTAssertEqual(DeckProblems.evaluate(Frontmatter(text: fixed), schema: schema), [])
    }

    func testTheFixNamesAThemeByItsDisplayName() throws {
        let text = "---\ntheme: keynot\n---\n\n# One\n"
        let frontmatter = Frontmatter(text: text)
        let problem = try XCTUnwrap(DeckProblems.evaluate(frontmatter, schema: schema).first)
        let fix = try XCTUnwrap(DeckProblems.fix(for: problem, in: frontmatter, schema: schema, displayName: { $0.capitalized }))
        XCTAssertEqual(fix.title, "Use Keynote")
        XCTAssertEqual(Frontmatter.applying(fix.replacement, to: text), "---\ntheme: keynote\n---\n\n# One\n")
    }

    func testWithNothingCloseTheFixUsesTheSchemasDefault() throws {
        let frontmatter = Frontmatter(text: "---\naspectRatio: 5:4\n---\n")
        let problem = try XCTUnwrap(DeckProblems.evaluate(frontmatter, schema: schema).first)
        XCTAssertEqual(DeckProblems.suggestedValue(for: problem, schema: schema), "16:9")
        XCTAssertEqual(DeckProblems.fix(for: problem, in: frontmatter, schema: schema)?.title, "Use 16:9")
    }

    func testAPickedSuggestionIsWrittenTheSameWay() throws {
        let text = "---\ntheme: nope\n---\n"
        let fix = try XCTUnwrap(DeckProblems.fix(setting: "theme", to: "swiss", in: Frontmatter(text: text)))
        XCTAssertEqual(Frontmatter.applying(fix.replacement, to: text), "---\ntheme: swiss\n---\n")
    }

    func testTheFixOfANestedKeyKeepsItsPlace() throws {
        let text = "---\nrecording:\n  audio: loud\n---\n"
        let frontmatter = Frontmatter(text: text)
        let problem = try XCTUnwrap(DeckProblems.evaluate(frontmatter, schema: schema).first)
        let fix = try XCTUnwrap(DeckProblems.fix(for: problem, in: frontmatter, schema: schema))
        XCTAssertEqual(Frontmatter.applying(fix.replacement, to: text), "---\nrecording:\n  audio: default\n---\n")
    }

    // MARK: Headings

    func testTheHeadingCountsTheFixes() {
        XCTAssertEqual(DeckProblems.heading(errorCount: 1), "This deck\u{2019}s settings need one fix")
        XCTAssertEqual(DeckProblems.heading(errorCount: 2), "This deck\u{2019}s settings need 2 fixes")
    }

    // MARK: The collapsed card

    func chips(_ text: String, themeName: @escaping (String) -> String = { $0.capitalized }) -> [DeckProblems.Chip] {
        let frontmatter = Frontmatter(text: text)
        return DeckProblems.chips(for: frontmatter, schema: schema, problems: DeckProblems.evaluate(frontmatter, schema: schema), themeName: themeName)
    }

    func testTheChipsSumUpTheDeck() {
        XCTAssertEqual(chips("---\ntheme: keynote\naspectRatio: 4:3\nauthor: Ada Lovelace\n---\n"),
                       [DeckProblems.Chip("Keynote"), DeckProblems.Chip("4:3"), DeckProblems.Chip("Ada Lovelace")])
    }

    func testADeckWithoutSettingsShowsTheDefaults() {
        XCTAssertEqual(chips("# One\n"), [DeckProblems.Chip("Default"), DeckProblems.Chip("16:9")])
    }

    func testAnErrorIsARedChipAndTheBadSettingIsNotShownAsItself() {
        let shown = chips("---\ntheme: keynote\naspectRatio: 16/9\nauthor: Ada\n---\n")
        XCTAssertEqual(shown, [DeckProblems.Chip("1 problem", .problem), DeckProblems.Chip("Keynote"), DeckProblems.Chip("Ada")])
        XCTAssertEqual(chips("---\naspectRatio: 1/1\ntransition: zip\n---\n").first, DeckProblems.Chip("2 problems", .problem))
    }

    func testAnUnknownThemeIsAnAmberChipWithTheNameAsWritten() {
        XCTAssertEqual(chips("---\ntheme: apple-basic\nauthor: Ada\n---\n"),
                       [DeckProblems.Chip("apple-basic", .warning), DeckProblems.Chip("16:9"), DeckProblems.Chip("Ada")])
    }

    // MARK: Frontmatter that does not parse

    func testTheFailingLineIsTheOneYamlNames() {
        XCTAssertEqual(DeckProblems.failingLine(inDeckErrors: ["frontmatter: failed to parse frontmatter: yaml: line 3: mapping values are not allowed in this context"]), 3)
        XCTAssertEqual(DeckProblems.failingLine(inDeckErrors: ["frontmatter: frontmatter not closed: missing closing ---"]), 0)
        XCTAssertNil(DeckProblems.failingLine(inDeckErrors: ["slide 2: something else"]))
        XCTAssertNil(DeckProblems.failingLine(inDeckErrors: []))
    }
}
