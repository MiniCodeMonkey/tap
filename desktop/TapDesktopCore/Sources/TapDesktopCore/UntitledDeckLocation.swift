import Foundation

/// Where tap serves a deck that has no file yet.
///
/// tap watches the deck's folder and everything under it, and reloads the
/// preview for a change to any file there. Each untitled deck therefore gets
/// a folder of its own: a deck kept directly in the shared temporary
/// directory would be reloaded for every file any program writes there, and
/// two untitled decks would share one file.
public enum UntitledDeckLocation {
    /// A new, empty folder under `base` holding the path of the deck file.
    public static func make(under base: URL = FileManager.default.temporaryDirectory) throws -> URL {
        let folder = base.appendingPathComponent("tap-untitled-\(UUID().uuidString)", isDirectory: true)
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        return folder.appendingPathComponent("Untitled.md")
    }

    /// Deletes the deck's folder.
    public static func remove(_ deck: URL) {
        try? FileManager.default.removeItem(at: deck.deletingLastPathComponent())
    }
}
