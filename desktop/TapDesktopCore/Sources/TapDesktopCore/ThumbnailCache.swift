import Foundation

/// The thumbnails on disk: one PNG per key, so a reopened deck shows every
/// thumbnail without rendering.
public struct ThumbnailCache: Sendable {
    public let directory: URL

    public static var defaultDirectory: URL {
        FileManager.default.urls(for: .applicationSupportDirectory, in: .userDomainMask)[0]
            .appendingPathComponent("Tap/Thumbnails", isDirectory: true)
    }

    public init(directory: URL = ThumbnailCache.defaultDirectory) {
        self.directory = directory
    }

    public func url(for key: ThumbnailKey) -> URL {
        directory.appendingPathComponent(key.fileName)
    }

    public func contains(_ key: ThumbnailKey) -> Bool {
        FileManager.default.fileExists(atPath: url(for: key).path)
    }

    public func data(for key: ThumbnailKey) -> Data? {
        try? Data(contentsOf: url(for: key))
    }

    public func save(_ png: Data, for key: ThumbnailKey) throws {
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try png.write(to: url(for: key), options: .atomic)
    }
}
