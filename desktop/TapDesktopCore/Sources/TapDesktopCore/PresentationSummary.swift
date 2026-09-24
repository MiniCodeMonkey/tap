import Foundation

/// One slide of `GET /api/presentation`, reduced to what the thumbnails
/// need: tap's content hash, which changes whenever the slide's text or
/// its component bundle changes, and the fields the panel shows.
public struct SlideSummary: Equatable, Sendable {
    public let hash: String
    public let skip: Bool
    public let steps: Int

    public init(hash: String, skip: Bool, steps: Int) {
        self.hash = hash
        self.skip = skip
        self.steps = steps
    }
}

/// `GET /api/presentation`, reduced to the revision, a signature of the
/// deck-wide settings that change how every slide looks, and each slide's
/// summary. Everything else in the response is the page's business.
public struct PresentationSummary: Equatable, Sendable {
    public let revision: String
    public let themeSignature: String
    public let slides: [SlideSummary]

    public init(revision: String, themeSignature: String, slides: [SlideSummary]) {
        self.revision = revision
        self.themeSignature = themeSignature
        self.slides = slides
    }

    private struct Envelope: Decodable {
        struct Config: Decodable {
            let theme: String?
            let customTheme: Bool?
            let aspectRatio: String?
            let themeColors: [String: String]?
        }
        struct Slide: Decodable {
            let hash: String?
            let skip: Bool?
            let steps: Int?
        }
        let config: Config
        let slides: [Slide]
        let revision: String?
    }

    public static func decode(_ data: Data) throws -> PresentationSummary {
        let envelope = try JSONDecoder().decode(Envelope.self, from: data)
        let colours = (envelope.config.themeColors ?? [:]).sorted { $0.key < $1.key }.map { "\($0.key)=\($0.value)" }.joined(separator: ";")
        let signature = [envelope.config.theme ?? "", envelope.config.customTheme == true ? "custom" : "",
                         envelope.config.aspectRatio ?? "", colours].joined(separator: "|")
        let slides = envelope.slides.map { SlideSummary(hash: $0.hash ?? "", skip: $0.skip ?? false, steps: $0.steps ?? 0) }
        return PresentationSummary(revision: envelope.revision ?? "", themeSignature: signature, slides: slides)
    }
}
