import XCTest
@testable import TapDesktopCore

final class LayoutCatalogTests: XCTestCase {
    func testDecodesTheLayoutListInTapsOrder() throws {
        let json = ##"{"ok": true, "layouts": [{"name": "title", "template": "# Title\n"}, {"name": "big-stat", "template": "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n"}]}"##
        let templates = try LayoutCatalog.decode(Data(json.utf8))
        XCTAssertEqual(templates, [LayoutTemplate(name: "title", markdown: "# Title\n"),
                                   LayoutTemplate(name: "big-stat", markdown: "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n")])
        XCTAssertThrowsError(try LayoutCatalog.decode(Data(#"{"ok": false, "error": {"code": "usage", "message": "no"}}"#.utf8)))
    }

    func testDisplayNames() {
        XCTAssertEqual(LayoutCatalog.displayName("big-stat"), "Big Stat")
        XCTAssertEqual(LayoutCatalog.displayName("two-column"), "Two Column")
        XCTAssertEqual(LayoutCatalog.displayName("title"), "Title")
    }

    func testSchematicsFollowTheTemplate() {
        XCTAssertEqual(LayoutSchematic.elements(for: "## Header\n\n- Point one\n- Point two\n"), [.heading, .line, .line])
        XCTAssertEqual(LayoutSchematic.elements(for: "::left\n\nLeft content\n\n::right\n\nRight content\n"), [.columns(2)])
        XCTAssertEqual(LayoutSchematic.elements(for: "::left\n\nA\n\n::center\n\nB\n\n::right\n\nC\n"), [.columns(3)])
        XCTAssertEqual(LayoutSchematic.elements(for: "<!--\nlayout: code-focus\n-->\n\n```\n// Your code here\n```\n"), [.code])
        XCTAssertEqual(LayoutSchematic.elements(for: "<!--\nlayout: quote\n-->\n\n> \"Your quote here\"\n"), [.quote])
        XCTAssertEqual(LayoutSchematic.elements(for: "<!--\nlayout: big-stat\n-->\n\n# 100%\n\nDescription\n"), [.bigNumber, .line])
        XCTAssertEqual(LayoutSchematic.elements(for: "<!--\nlayout: sidebar\n-->\n\n## Header\n\nMain content\n\n::sidebar\n\n- Note one\n"), [.heading, .line, .sidebar])
        XCTAssertEqual(LayoutSchematic.elements(for: "<!--\nlayout: split-media\n-->\n\n## Header\n\nDescribe the image\n\n::media\n\nImage or video\n"), [.heading, .line, .media])
        XCTAssertEqual(LayoutSchematic.elements(for: "<!--\nlayout: blank\n-->\n\nContent\n"), [.line])
        XCTAssertEqual(LayoutSchematic.elements(for: ""), [])
    }

    func testLastLayoutDefaultsToDefault() throws {
        let defaults = try XCTUnwrap(UserDefaults(suiteName: "LayoutCatalogTests.\(UUID().uuidString)"))
        var last = LastLayout(defaults: defaults)
        XCTAssertEqual(last.name, "default")
        last.name = "two-column"
        XCTAssertEqual(LastLayout(defaults: defaults).name, "two-column")
    }
}
