import Foundation

/// What a drag of slides carries on the pasteboard: which deck they come
/// from, their numbers there, and each slide's text. Within one deck the
/// numbers drive a move; into another deck the texts are inserted.
public struct SlideDragPayload: Codable, Equatable, Sendable {
    public static let pasteboardType = "io.geocod.tap.slides"

    public let deckPath: String
    public let slideNumbers: [Int]
    public let markdowns: [String]

    public init(deckPath: String, slideNumbers: [Int], markdowns: [String]) {
        self.deckPath = deckPath
        self.slideNumbers = slideNumbers
        self.markdowns = markdowns
    }

    public init?(data: Data) {
        guard let decoded = try? JSONDecoder().decode(SlideDragPayload.self, from: data) else { return nil }
        self = decoded
    }

    public func data() throws -> Data {
        try JSONEncoder().encode(self)
    }

    /// Whether the slides come from `deck`, compared through symlinks.
    public func comesFrom(deck: URL) -> Bool {
        FilePaths.same(URL(fileURLWithPath: deckPath), deck)
    }
}
