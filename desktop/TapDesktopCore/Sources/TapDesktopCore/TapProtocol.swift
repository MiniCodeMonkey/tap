import Foundation

/// The first line `tap dev --app` prints on standard output.
public struct TapReady: Codable, Equatable, Sendable {
    public let port: Int
    public let token: String
    public let launch: String
    /// The presenter secret. It is what separates driving the deck from
    /// watching it: a WebSocket connection relays its slide messages to the
    /// other pages only once it has traded this secret for the presenter
    /// cookie, and the presenter view is served only to a request holding
    /// one of the two. See `AppAuth.PresenterPassword` in
    /// internal/server/app_auth.go.
    public let presenter: String

    public init(port: Int, token: String, launch: String, presenter: String = "") {
        self.port = port
        self.token = token
        self.launch = launch
        self.presenter = presenter
    }
}

/// One fenced code block of a slide, as tap reports it.
public struct CodeBlock: Codable, Equatable, Sendable {
    /// Which fenced block of the slide this is, counting from 1, as
    /// `CodeBlock.Block` in internal/slidelist/slidelist.go defines it.
    public let block: Int
    public let language: String
    public let driver: String
    public let live: Bool
    public let line: Int

    public init(block: Int, language: String, driver: String, live: Bool, line: Int) {
        self.block = block
        self.language = language
        self.driver = driver
        self.live = live
        self.line = line
    }
}

/// One slide as tap reports it. `startLine` and `endLine` are 1-based and
/// inclusive lines of the text tap was given, without leading or trailing
/// blank lines. Separators and the frontmatter belong to no slide.
public struct Slide: Codable, Equatable, Sendable {
    public let number: Int
    public let startLine: Int
    public let endLine: Int
    public let layout: String
    public let title: String
    public let fragments: Int
    public let steps: Int
    public let skip: Bool
    public let errors: [String]
    public let codeBlocks: [CodeBlock]

    public init(number: Int, startLine: Int, endLine: Int, layout: String = "default", title: String = "",
                fragments: Int = 0, steps: Int = 0, skip: Bool = false, errors: [String] = [],
                codeBlocks: [CodeBlock] = []) {
        self.number = number
        self.startLine = startLine
        self.endLine = endLine
        self.layout = layout
        self.title = title
        self.fragments = fragments
        self.steps = steps
        self.skip = skip
        self.errors = errors
        self.codeBlocks = codeBlocks
    }
}

/// The slide list: tap's answer to `PUT /api/app/source`, and the payload of
/// a `file-changed` event for a file other than the deck.
public struct SlideList: Codable, Equatable, Sendable {
    public let slides: [Slide]
    /// Problems with the deck as a whole, such as frontmatter that does not parse.
    public let errors: [String]

    public init(slides: [Slide], errors: [String]) {
        self.slides = slides
        self.errors = errors
    }

    /// Decodes `{"ok": true, "slides", "errors"}`, and throws the
    /// `TapErrorPayload` of `{"ok": false, "error": {...}}`.
    public static func decodeResponse(_ data: Data) throws -> SlideList {
        struct Envelope: Decodable {
            let ok: Bool
            let slides: [Slide]?
            let errors: [String]?
            let error: TapErrorPayload?
        }
        let envelope = try JSONDecoder().decode(Envelope.self, from: data)
        if envelope.ok, let slides = envelope.slides {
            return SlideList(slides: slides, errors: envelope.errors ?? [])
        }
        throw envelope.error ?? TapErrorPayload(code: "invalid_response", message: "tap answered without a slide list")
    }
}

/// An error tap reports, in an `error` event or an error response.
public struct TapErrorPayload: Codable, Equatable, Sendable, Error {
    public let code: String
    public let message: String

    public init(code: String, message: String) {
        self.code = code
        self.message = message
    }
}

/// One JSON line from tap's standard output.
public enum TapEvent: Equatable, Sendable {
    case ready(TapReady)
    case fileChanged(path: String, slideList: SlideList?)
    case question(id: String, kind: String)
    case error(TapErrorPayload)
    case other(type: String)

    private struct Envelope: Decodable {
        let type: String
        let port: Int?
        let token: String?
        let launch: String?
        let presenter: String?
        let path: String?
        let slides: [Slide]?
        let errors: [String]?
        let id: String?
        let kind: String?
        let code: String?
        let message: String?
    }

    /// Returns nil for a line that is not a JSON object with a known shape.
    public static func decode(line: String) -> TapEvent? {
        guard let envelope = try? JSONDecoder().decode(Envelope.self, from: Data(line.utf8)) else { return nil }
        switch envelope.type {
        case "ready":
            guard let port = envelope.port, let token = envelope.token, let launch = envelope.launch else { return nil }
            // presenter is missing only from a tap older than app mode's
            // presenter secret. TapClient refuses to authorize on an empty
            // one rather than opening a socket that cannot drive the deck.
            return .ready(TapReady(port: port, token: token, launch: launch, presenter: envelope.presenter ?? ""))
        case "file-changed":
            guard let path = envelope.path else { return nil }
            let list = envelope.slides.map { SlideList(slides: $0, errors: envelope.errors ?? []) }
            return .fileChanged(path: path, slideList: list)
        case "question":
            guard let id = envelope.id, let kind = envelope.kind else { return nil }
            return .question(id: id, kind: kind)
        case "error":
            return .error(TapErrorPayload(code: envelope.code ?? "unknown", message: envelope.message ?? ""))
        default:
            return .other(type: envelope.type)
        }
    }
}

/// A command the app writes to tap's standard input, one JSON line each.
public enum TapCommand: Equatable, Sendable {
    /// The app saved its buffer to the deck file.
    case saved
    /// Render the deck again and reload every page.
    case reload
    /// Shut down.
    case quit

    /// The command as one JSON line, without the newline.
    public var line: String {
        switch self {
        case .saved: return #"{"type":"saved"}"#
        case .reload: return #"{"type":"reload"}"#
        case .quit: return #"{"type":"quit"}"#
        }
    }
}

/// The hub's `slide` message, which moves every page to a slide position.
/// `slideIndex` is 0-based, as the hub expects; `fragment` -1 reveals none.
public struct SlideMessage: Encodable, Equatable, Sendable {
    public let slideIndex: Int
    public let fragment: Int
    public let step: Int

    public init(slideIndex: Int, fragment: Int, step: Int) {
        self.slideIndex = slideIndex
        self.fragment = fragment
        self.step = step
    }

    private enum CodingKeys: String, CodingKey { case type, slideIndex, fragment, step }

    public func encode(to encoder: Encoder) throws {
        var container = encoder.container(keyedBy: CodingKeys.self)
        try container.encode("slide", forKey: .type)
        try container.encode(slideIndex, forKey: .slideIndex)
        try container.encode(fragment, forKey: .fragment)
        try container.encode(step, forKey: .step)
    }

    public func text() -> String {
        let encoder = JSONEncoder()
        encoder.outputFormatting = .sortedKeys
        let data = (try? encoder.encode(self)) ?? Data()
        return String(decoding: data, as: UTF8.self)
    }
}

/// A message the WebSocket hub sends to the app's own connection.
public enum HubMessage: Equatable, Sendable {
    case update(revision: String, slides: [Int])
    case reload
    case fileChanged(path: String)
    case slide(slideIndex: Int)
    case other(type: String)

    private struct Envelope: Decodable {
        let type: String
        let revision: String?
        let slides: [Int]?
        let path: String?
        let slideIndex: Int?
    }

    public static func decode(_ text: String) -> HubMessage? {
        guard let envelope = try? JSONDecoder().decode(Envelope.self, from: Data(text.utf8)) else { return nil }
        switch envelope.type {
        case "update": return .update(revision: envelope.revision ?? "", slides: envelope.slides ?? [])
        case "reload": return .reload
        case "file-changed": return .fileChanged(path: envelope.path ?? "")
        case "slide":
            guard let index = envelope.slideIndex else { return .other(type: "slide") }
            return .slide(slideIndex: index)
        default: return .other(type: envelope.type)
        }
    }
}
