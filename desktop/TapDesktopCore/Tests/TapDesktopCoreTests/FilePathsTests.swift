import XCTest
@testable import TapDesktopCore

final class FilePathsTests: XCTestCase {
    func testPathsThroughTheVarSymlinkAreTheSame() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let file = folder.appendingPathComponent("talk.md")
        try "# One\n".write(to: file, atomically: true, encoding: .utf8)
        let throughPrivate = URL(fileURLWithPath: "/private" + file.path)
        XCTAssertTrue(FilePaths.same(file, file.path.hasPrefix("/private") ? file : throughPrivate))
        XCTAssertFalse(FilePaths.same(file, folder.appendingPathComponent("other.md")))
    }

    func testPathsThroughTheVarSymlinkAreTheSameWhenTheFinalComponentDoesNotExist() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent(UUID().uuidString)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let missing = folder.appendingPathComponent("does-not-exist.md")
        let throughPrivate = URL(fileURLWithPath: "/private" + missing.path)
        XCTAssertTrue(FilePaths.same(missing, missing.path.hasPrefix("/private") ? missing : throughPrivate))
    }
}
