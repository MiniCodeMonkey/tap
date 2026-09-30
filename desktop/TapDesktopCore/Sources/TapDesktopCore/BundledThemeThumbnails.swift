import Foundation

/// The theme thumbnails the app bundle ships, one `<slug>.png` per theme
/// beside `catalog.json` (`tap theme list --json`), rendered at build time
/// by the bundled tap. Looking one up reads no process and no network.
public struct BundledThemeThumbnails: Sendable {
    public let folder: URL

    public init(folder: URL) { self.folder = folder }

    /// The folder inside an app's Resources folder.
    public static let folderName = "ThemeThumbnails"

    /// The thumbnails folder of the app whose Resources folder this is.
    public init(resourcesFolder: URL) {
        self.init(folder: resourcesFolder.appendingPathComponent(Self.folderName, isDirectory: true))
    }

    /// The catalog the thumbnails were rendered from, nil when the folder has none.
    public func catalog() -> ThemeCatalog? {
        guard let data = try? Data(contentsOf: folder.appendingPathComponent("catalog.json")) else { return nil }
        return try? ThemeCatalog.decode(data)
    }

    /// The PNG for a slug, nil when the bundle has none. A slug that is not
    /// a plain file name (a path, an empty string) has none.
    public func imageURL(forSlug slug: String) -> URL? {
        guard !slug.isEmpty, slug == (slug as NSString).lastPathComponent, !slug.hasPrefix(".") else { return nil }
        let url = folder.appendingPathComponent(slug + ".png")
        return FileManager.default.fileExists(atPath: url.path) ? url : nil
    }
}
