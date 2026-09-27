import Foundation

/// One built-in theme as `tap theme list --json` lists it (internal/themes).
public struct ThemeSummary: Codable, Equatable, Sendable {
    public let slug: String
    public let name: String
    /// "light" or "dark".
    public let polarity: String
    public let pitch: String

    public init(slug: String, name: String, polarity: String, pitch: String) {
        self.slug = slug; self.name = name; self.polarity = polarity; self.pitch = pitch
    }
}

/// Every theme tap offers, in tap's order, for the grid's two groups.
public struct ThemeCatalog: Equatable, Sendable {
    public let themes: [ThemeSummary]

    public init(themes: [ThemeSummary]) { self.themes = themes }

    private struct Envelope: Decodable { let themes: [ThemeSummary] }

    public static func decode(_ data: Data) throws -> ThemeCatalog {
        guard let outcome = ToolOutcome.decode(data) else { throw ToolError.noResult(status: 0) }
        return ThemeCatalog(themes: try outcome.result(Envelope.self).themes)
    }

    public var light: [ThemeSummary] { themes.filter { $0.polarity != "dark" } }
    public var dark: [ThemeSummary] { themes.filter { $0.polarity == "dark" } }

    public func theme(slug: String) -> ThemeSummary? { themes.first { $0.slug == slug } }

    /// The theme's name for a label, or the slug itself when tap does not know it.
    public func name(forSlug slug: String) -> String { theme(slug: slug)?.name ?? slug }
}

/// The result of `tap theme show <slug> --image --json`: the PNG's path.
public struct ThemeImageResult: Decodable, Equatable, Sendable {
    public let slug: String
    public let image: String
    public let cached: Bool

    public init(slug: String, image: String, cached: Bool) { self.slug = slug; self.image = image; self.cached = cached }

    public static func decode(_ data: Data) throws -> ThemeImageResult {
        guard let outcome = ToolOutcome.decode(data) else { throw ToolError.noResult(status: 0) }
        return try outcome.result(ThemeImageResult.self)
    }
}
