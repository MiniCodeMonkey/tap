import XCTest
@testable import TapDesktopCore

final class BundledThemeThumbnailsTests: XCTestCase {
    private func makeFolder() throws -> URL {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString, isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        addTeardownBlock { try? FileManager.default.removeItem(at: folder) }
        return folder
    }

    func testFindsAnImageBySlugAndNothingElse() throws {
        let folder = try makeFolder()
        try Data([1]).write(to: folder.appendingPathComponent("base.png"))
        let thumbnails = BundledThemeThumbnails(folder: folder)
        XCTAssertEqual(thumbnails.imageURL(forSlug: "base")?.lastPathComponent, "base.png")
        XCTAssertNil(thumbnails.imageURL(forSlug: "custom"))
        XCTAssertNil(thumbnails.imageURL(forSlug: ""))
        XCTAssertNil(thumbnails.imageURL(forSlug: "../base"))
        XCTAssertNil(thumbnails.imageURL(forSlug: ".hidden"))
    }

    func testResourcesFolderNamesTheThumbnailsFolder() {
        let thumbnails = BundledThemeThumbnails(resourcesFolder: URL(fileURLWithPath: "/App/Contents/Resources"))
        XCTAssertEqual(thumbnails.folder.path, "/App/Contents/Resources/ThemeThumbnails")
    }

    func testReadsTheCatalogAndToleratesItsAbsence() throws {
        let folder = try makeFolder()
        XCTAssertNil(BundledThemeThumbnails(folder: folder).catalog())
        let json = #"{"ok":true,"themes":[{"slug":"base","name":"Base","polarity":"light","pitch":"p"}]}"#
        try Data(json.utf8).write(to: folder.appendingPathComponent("catalog.json"))
        XCTAssertEqual(BundledThemeThumbnails(folder: folder).catalog()?.themes.map(\.slug), ["base"])
    }
}
