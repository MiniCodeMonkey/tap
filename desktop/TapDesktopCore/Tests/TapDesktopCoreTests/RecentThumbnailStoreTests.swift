import XCTest
@testable import TapDesktopCore

final class RecentThumbnailStoreTests: XCTestCase {
    private func temporaryFolder() throws -> URL {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        return folder
    }

    func testStoresOneThumbnailPerDeck() throws {
        let store = RecentThumbnailStore(directory: try temporaryFolder())
        let folder = try temporaryFolder()
        let first = folder.appendingPathComponent("first.md")
        let second = folder.appendingPathComponent("second.md")
        XCTAssertNotEqual(store.url(for: first), store.url(for: second))
        XCTAssertEqual(store.url(for: first), store.url(for: URL(fileURLWithPath: first.path + "/../first.md")))
        XCTAssertNil(store.imageData(for: first))
        try store.save(Data([1, 2, 3]), for: first)
        XCTAssertEqual(store.imageData(for: first), Data([1, 2, 3]))
        XCTAssertEqual(store.url(for: first).pathExtension, "png")
    }
}
