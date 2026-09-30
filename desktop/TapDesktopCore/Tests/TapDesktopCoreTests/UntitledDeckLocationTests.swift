import XCTest
@testable import TapDesktopCore

final class UntitledDeckLocationTests: XCTestCase {
    private var base: URL!

    override func setUpWithError() throws {
        base = FileManager.default.temporaryDirectory.appendingPathComponent("untitled-location-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: base, withIntermediateDirectories: true)
    }

    override func tearDownWithError() throws {
        try? FileManager.default.removeItem(at: base)
    }

    /// tap watches the deck's folder and everything under it, and reloads
    /// the preview for any change there: a deck sitting directly in a busy
    /// shared folder, such as the temporary directory, reloads without end.
    func testTheDeckHasAFolderOfItsOwn() throws {
        let deck = try UntitledDeckLocation.make(under: base)
        XCTAssertNotEqual(deck.deletingLastPathComponent().standardizedFileURL, base.standardizedFileURL)
        XCTAssertEqual(deck.deletingLastPathComponent().deletingLastPathComponent().standardizedFileURL, base.standardizedFileURL)
        XCTAssertEqual(deck.lastPathComponent, "Untitled.md")
        var isDirectory: ObjCBool = false
        XCTAssertTrue(FileManager.default.fileExists(atPath: deck.deletingLastPathComponent().path, isDirectory: &isDirectory))
        XCTAssertTrue(isDirectory.boolValue)
    }

    func testTwoUntitledDecksDoNotShareAFile() throws {
        let first = try UntitledDeckLocation.make(under: base)
        let second = try UntitledDeckLocation.make(under: base)
        XCTAssertNotEqual(first, second)
    }

    func testRemoveDeletesTheDecksFolder() throws {
        let deck = try UntitledDeckLocation.make(under: base)
        try Data("x".utf8).write(to: deck)
        UntitledDeckLocation.remove(deck)
        XCTAssertFalse(FileManager.default.fileExists(atPath: deck.deletingLastPathComponent().path))
        XCTAssertTrue(FileManager.default.fileExists(atPath: base.path))
    }

    func testKeepRecordingsMovesThemOutOfTheDecksFolder() throws {
        let deck = try UntitledDeckLocation.make(under: base)
        let recordings = UntitledDeckLocation.recordings(of: deck)
        try FileManager.default.createDirectory(at: recordings, withIntermediateDirectories: true)
        try Data("a".utf8).write(to: recordings.appendingPathComponent("talk.mov"))
        let destination = base.appendingPathComponent("Movies/Tap")
        try FileManager.default.createDirectory(at: destination, withIntermediateDirectories: true)
        try Data("old".utf8).write(to: destination.appendingPathComponent("talk.mov"))

        XCTAssertTrue(UntitledDeckLocation.keepRecordings(of: deck, in: destination))
        UntitledDeckLocation.remove(deck)

        XCTAssertEqual(try String(contentsOf: destination.appendingPathComponent("talk.mov")), "old", "an earlier recording is left alone")
        XCTAssertEqual(try String(contentsOf: destination.appendingPathComponent("talk 2.mov")), "a", "the new one takes the next number")
        XCTAssertFalse(FileManager.default.fileExists(atPath: deck.deletingLastPathComponent().path))
    }

    func testKeepRecordingsWithNoneIsANoOp() throws {
        let deck = try UntitledDeckLocation.make(under: base)
        let destination = base.appendingPathComponent("Movies/Tap")
        XCTAssertTrue(UntitledDeckLocation.keepRecordings(of: deck, in: destination))
        XCTAssertFalse(FileManager.default.fileExists(atPath: destination.path), "nothing to keep, so no folder is made")
    }
}
