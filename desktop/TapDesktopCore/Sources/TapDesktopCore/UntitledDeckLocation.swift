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

    /// The folder of tap's recordings next to the deck, where a talk on an
    /// untitled deck records.
    public static func recordings(of deck: URL) -> URL {
        deck.deletingLastPathComponent().appendingPathComponent("recordings", isDirectory: true)
    }

    /// Moves the recordings of a talk on an untitled deck into `destination`,
    /// which is made if needed, so that deleting the deck's folder does not
    /// take them along. A name already taken there gets a number. Returns
    /// false when a recording could not be moved, and the folder is then
    /// left for the caller to keep.
    @discardableResult
    public static func keepRecordings(of deck: URL, in destination: URL) -> Bool {
        let manager = FileManager.default
        let source = recordings(of: deck)
        guard let names = try? manager.contentsOfDirectory(atPath: source.path), !names.isEmpty else { return true }
        do {
            try manager.createDirectory(at: destination, withIntermediateDirectories: true)
            for name in names {
                let base = (name as NSString).deletingPathExtension
                let pathExtension = (name as NSString).pathExtension
                var target = destination.appendingPathComponent(name)
                var number = 2
                while manager.fileExists(atPath: target.path) {
                    let numbered = pathExtension.isEmpty ? "\(base) \(number)" : "\(base) \(number).\(pathExtension)"
                    target = destination.appendingPathComponent(numbered)
                    number += 1
                }
                try manager.moveItem(at: source.appendingPathComponent(name), to: target)
            }
            return true
        } catch {
            return false
        }
    }
}
