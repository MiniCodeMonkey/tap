import XCTest
@testable import TapDesktopCore

final class ThemeCatalogTests: XCTestCase {
    let json = #"{"ok":true,"themes":[{"slug":"base","name":"Base","polarity":"light","pitch":"Clean and quiet."},{"slug":"terminal","name":"Terminal","polarity":"dark","pitch":"A shell at 3am."},{"slug":"product","name":"Product","polarity":"light","pitch":"A launch."},{"slug":"blueprint","name":"Blueprint","polarity":"dark","pitch":"Drafting paper."}]}"#

    func testGroupsThemesByPolarityInTapsOrder() throws {
        let catalog = try ThemeCatalog.decode(Data(json.utf8))
        XCTAssertEqual(catalog.themes.map(\.slug), ["base", "terminal", "product", "blueprint"])
        XCTAssertEqual(catalog.light.map(\.slug), ["base", "product"])
        XCTAssertEqual(catalog.dark.map(\.slug), ["terminal", "blueprint"])
        XCTAssertEqual(catalog.theme(slug: "terminal")?.name, "Terminal")
        XCTAssertEqual(catalog.name(forSlug: "nope"), "nope", "an unknown slug is shown as written")
        XCTAssertNil(catalog.theme(slug: "nope"))
    }

    func testDecodesAnImageResult() throws {
        let result = try ThemeImageResult.decode(Data(#"{"ok":true,"slug":"terminal","image":"/Users/me/Library/Caches/tap/themes/2.1.0/terminal.png","cached":true}"#.utf8))
        XCTAssertEqual(result, ThemeImageResult(slug: "terminal", image: "/Users/me/Library/Caches/tap/themes/2.1.0/terminal.png", cached: true))
    }

    func testRefusesAFailure() {
        XCTAssertThrowsError(try ThemeCatalog.decode(Data(#"{"ok":false,"error":{"code":"internal","message":"x"}}"#.utf8)))
        XCTAssertThrowsError(try ThemeCatalog.decode(Data("SLUG NAME\n".utf8)))
    }
}
