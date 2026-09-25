import XCTest
@testable import TapDesktopCore

final class ThumbnailCacheTests: XCTestCase {
    func testTheKeyNamesOneFilePerSlideThemeAndSize() {
        let key = ThumbnailKey(slideHash: "abc", themeSignature: "terminal|||", width: 320)
        XCTAssertEqual(key.fileName.count, 64 + 4)
        XCTAssertTrue(key.fileName.hasSuffix(".png"))
        XCTAssertNotEqual(key.fileName, ThumbnailKey(slideHash: "abc", themeSignature: "base|||", width: 320).fileName, "the theme is part of the key")
        XCTAssertNotEqual(key.fileName, ThumbnailKey(slideHash: "abc", themeSignature: "terminal|||", width: 640).fileName, "so is the size")
        XCTAssertEqual(key.fileName, ThumbnailKey(slideHash: "abc", themeSignature: "terminal|||", width: 320).fileName)
    }

    func testSavesAndReadsPNGsInItsDirectory() throws {
        let directory = FileManager.default.temporaryDirectory.appendingPathComponent("thumbnails-\(UUID().uuidString)")
        let cache = ThumbnailCache(directory: directory)
        let key = ThumbnailKey(slideHash: "abc", themeSignature: "terminal|||", width: 320)
        XCTAssertFalse(cache.contains(key))
        XCTAssertNil(cache.data(for: key))
        try cache.save(Data([0x89, 0x50, 0x4E, 0x47]), for: key)
        XCTAssertTrue(cache.contains(key))
        XCTAssertEqual(cache.data(for: key), Data([0x89, 0x50, 0x4E, 0x47]))
        XCTAssertEqual(cache.url(for: key).lastPathComponent, key.fileName)
        XCTAssertTrue(ThumbnailCache.defaultDirectory.path.hasSuffix("Application Support/Tap/Thumbnails"))
        try FileManager.default.removeItem(at: directory)
    }
}
