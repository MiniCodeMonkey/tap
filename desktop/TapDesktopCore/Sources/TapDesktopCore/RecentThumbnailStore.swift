import CryptoKit
import Foundation

/// Where the welcome window finds a recent deck's thumbnail: one PNG per
/// deck, named by a hash of the deck's path.
public struct RecentThumbnailStore: Sendable {
    public let directory: URL

    public static var defaultDirectory: URL {
        FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("Tap/RecentThumbnails", isDirectory: true)
    }

    public init(directory: URL = RecentThumbnailStore.defaultDirectory) {
        self.directory = directory
    }

    public func url(for deck: URL) -> URL {
        let digest = SHA256.hash(data: Data(FilePaths.canonical(deck).utf8))
        let name = digest.map { String(format: "%02x", $0) }.joined()
        return directory.appendingPathComponent(name).appendingPathExtension("png")
    }

    public func save(_ png: Data, for deck: URL) throws {
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try png.write(to: url(for: deck), options: .atomic)
    }

    public func imageData(for deck: URL) -> Data? {
        try? Data(contentsOf: url(for: deck))
    }
}
