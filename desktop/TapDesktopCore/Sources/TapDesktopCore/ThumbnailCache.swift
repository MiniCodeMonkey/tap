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

    /// The PNG for `key`, or nil when there is none. A missing file, such
    /// as a key never rendered or a cache folder removed underneath the
    /// app, is a cache miss: it throws nothing, not even an error swallowed
    /// here, so no Swift error is raised on every refresh for each slide
    /// still waiting for its first render.
    public func data(for key: ThumbnailKey) -> Data? {
        FileManager.default.contents(atPath: url(for: key).path)
    }

    public func save(_ png: Data, for key: ThumbnailKey) throws {
        try FileManager.default.createDirectory(at: directory, withIntermediateDirectories: true)
        try png.write(to: url(for: key), options: .atomic)
    }
}
