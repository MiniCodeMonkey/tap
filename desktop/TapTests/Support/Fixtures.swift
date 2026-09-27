import Foundation

/// Decks for the hosted tests. Each test works on a fresh copy in a
/// temporary folder, because the app and tap write next to the deck.
enum Fixtures {
    static var repositoryRoot: URL {
        URL(fileURLWithPath: #filePath)
            .deletingLastPathComponent() // Support
            .deletingLastPathComponent() // TapTests
            .deletingLastPathComponent() // desktop
            .deletingLastPathComponent()
    }

    static func temporaryFolder() throws -> URL {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("tap-desktop-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        return folder
    }

    /// P6's fixture: four slides, a live shell block on slide 2, a fragment
    /// on slide 3 and the component slides/Counter.jsx with two steps on slide 4.
    static func copyAppFixture() throws -> URL {
        let copy = try temporaryFolder().appendingPathComponent("app")
        try FileManager.default.copyItem(at: repositoryRoot.appendingPathComponent("internal/cli/testdata/app"), to: copy)
        return copy.appendingPathComponent("talk.md")
    }

    /// A deck from desktop/TapTests/Fixtures: a file, or a folder with a talk.md.
    static func copyDeck(_ name: String) throws -> URL {
        let source = repositoryRoot.appendingPathComponent("desktop/TapTests/Fixtures/\(name)")
        let copy = try temporaryFolder().appendingPathComponent(name)
        try FileManager.default.copyItem(at: source, to: copy)
        var isFolder: ObjCBool = false
        FileManager.default.fileExists(atPath: copy.path, isDirectory: &isFolder)
        return isFolder.boolValue ? copy.appendingPathComponent("talk.md") : copy
    }

    /// The path with every symlink resolved, as usersettings.ResolveDeck
    /// keys a deck: /var/folders is /private/var/folders here, which
    /// URL.resolvingSymlinksInPath() leaves alone.
    static func realPath(of url: URL) -> String {
        var buffer = [Int8](repeating: 0, count: Int(PATH_MAX))
        guard let resolved = realpath(url.path, &buffer) else { return url.path }
        return String(cString: resolved)
    }
}
