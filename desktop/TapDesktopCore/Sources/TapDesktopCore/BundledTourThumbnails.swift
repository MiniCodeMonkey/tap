import Foundation

/// The theme tour's slide thumbnails, rendered at build time by the bundled
/// tap: `slide-<n>.png` for each slide, beside `theme-tour.md`, the text
/// they were rendered from. An untitled tour that has not been edited shows
/// them at once; once any text differs, every slide renders as usual.
public struct BundledTourThumbnails: Sendable {
    public let folder: URL

    public init(folder: URL) { self.folder = folder }

    /// The folder inside an app's Resources folder.
    public static let folderName = "TourThumbnails"
    public static let sourceFileName = "theme-tour.md"

    public init(resourcesFolder: URL) {
        self.init(folder: resourcesFolder.appendingPathComponent(Self.folderName, isDirectory: true))
    }

    /// The image of each slide, by slide number, when `text` is the tour as
    /// it was rendered and tap reports `slideCount` slides for it; empty
    /// otherwise, and when any image is missing.
    public func imageURLs(forText text: String, slideCount: Int) -> [Int: URL] {
        guard let source = try? String(contentsOf: folder.appendingPathComponent(Self.sourceFileName), encoding: .utf8),
              Self.normalized(source) == Self.normalized(text), slideCount > 0 else { return [:] }
        var urls: [Int: URL] = [:]
        for number in 1...slideCount {
            let url = folder.appendingPathComponent("slide-\(number).png")
            guard FileManager.default.fileExists(atPath: url.path) else { return [:] }
            urls[number] = url
        }
        guard !FileManager.default.fileExists(atPath: folder.appendingPathComponent("slide-\(slideCount + 1).png").path) else { return [:] }
        return urls
    }

    /// Trailing blank space does not change what tap renders.
    private static func normalized(_ text: String) -> String {
        text.trimmingCharacters(in: .whitespacesAndNewlines)
    }
}
