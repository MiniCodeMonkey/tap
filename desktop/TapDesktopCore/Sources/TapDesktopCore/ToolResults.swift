import Foundation

/// The `--json` results of the tap commands the app runs, one struct per
/// command, with the field names internal/cli prints. A done line of a
/// `--progress json` run carries the same fields plus `phase`, which these
/// ignore.

public struct NewDeckResult: Decodable, Equatable, Sendable {
    public let deck: String
    public let folder: String
    public init(deck: String, folder: String) { self.deck = deck; self.folder = folder }
}

public struct ThemeSetResult: Decodable, Equatable, Sendable {
    public let deck: String
    public let theme: String
    public init(deck: String, theme: String) { self.deck = deck; self.theme = theme }
}

public struct AddedImageResult: Decodable, Equatable, Sendable {
    public let deck: String
    public let image: String
    public let markdown: String
    public init(deck: String, image: String, markdown: String) { self.deck = deck; self.image = image; self.markdown = markdown }
}

public struct GeneratedImageResult: Decodable, Equatable, Sendable {
    public let deck: String
    public let slide: Int
    public let image: String
    public let prompt: String
    public let markdown: String
    /// The image `tap image regenerate` replaced; nil for `generate`.
    public let replaced: String?
    public init(deck: String, slide: Int, image: String, prompt: String, markdown: String, replaced: String? = nil) {
        self.deck = deck; self.slide = slide; self.image = image; self.prompt = prompt; self.markdown = markdown; self.replaced = replaced
    }
}

public struct ComponentScaffold: Decodable, Equatable, Sendable {
    /// The files tap wrote; the first is the component itself.
    public let files: [String]
    public let snippet: String
    public init(files: [String], snippet: String) { self.files = files; self.snippet = snippet }
}

public struct BrokenSlide: Decodable, Equatable, Sendable {
    public let slide: Int
    public let message: String
    public init(slide: Int, message: String) { self.slide = slide; self.message = message }
}

public struct PDFExportResult: Decodable, Equatable, Sendable {
    public let output: String
    public let pages: Int
    public let bytes: Int64
    public let brokenSlides: [BrokenSlide]
    public init(output: String, pages: Int, bytes: Int64, brokenSlides: [BrokenSlide]) {
        self.output = output; self.pages = pages; self.bytes = bytes; self.brokenSlides = brokenSlides
    }
}

public struct ImagesExportResult: Decodable, Equatable, Sendable {
    public let files: [String]
    public init(files: [String]) { self.files = files }
}

public struct BuildResult: Decodable, Equatable, Sendable {
    public let output: String
    public let files: Int
    public let bytes: Int64
    public init(output: String, files: Int, bytes: Int64) { self.output = output; self.files = files; self.bytes = bytes }
}

public struct ServeReady: Decodable, Equatable, Sendable {
    public let dir: String
    public let port: Int
    public let url: String
    public init(dir: String, port: Int, url: String) { self.dir = dir; self.port = port; self.url = url }
}

/// One deck `tap approval list --json` lists. `commands` is a custom
/// driver's command as tap shows it, secrets already masked by tap; the
/// app shows these strings and expands nothing.
public struct ApprovalRecord: Decodable, Equatable, Sendable {
    public let deck: String
    public let drivers: [String]
    public let commands: [String: [String]]?
    public let approvedAt: String

    public init(deck: String, drivers: [String], commands: [String: [String]]? = nil, approvedAt: String) {
        self.deck = deck; self.drivers = drivers; self.commands = commands; self.approvedAt = approvedAt
    }

    public var deckName: String { (deck as NSString).lastPathComponent }
    public var folderPath: String { (deck as NSString).deletingLastPathComponent }
    /// "shell, kubectl (custom)": a driver with a command of its own is custom.
    public var driverSummary: String {
        drivers.map { name in commands?[name] != nil ? "\(name) (custom)" : name }.joined(separator: ", ")
    }
    /// tap writes RFC 3339 with or without fractional seconds.
    public var approvedAtDate: Date? {
        let withFraction = ISO8601DateFormatter()
        withFraction.formatOptions = [.withInternetDateTime, .withFractionalSeconds]
        return withFraction.date(from: approvedAt) ?? ISO8601DateFormatter().date(from: approvedAt)
    }
}

public struct ApprovalList: Decodable, Equatable, Sendable {
    public let approvals: [ApprovalRecord]
    public init(approvals: [ApprovalRecord]) { self.approvals = approvals }
}
