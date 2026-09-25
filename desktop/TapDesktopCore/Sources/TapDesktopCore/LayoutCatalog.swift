import Foundation

/// One of tap's layouts and the template `tap slide add --layout <name> --print` writes for it.
public struct LayoutTemplate: Equatable, Sendable {
    public let name: String
    public let markdown: String

    public init(name: String, markdown: String) {
        self.name = name
        self.markdown = markdown
    }
}

public enum LayoutCatalog {
    /// Decodes `tap slide add --print --json`: every layout in the wizard's order.
    public static func decode(_ data: Data) throws -> [LayoutTemplate] {
        struct Envelope: Decodable {
            struct Entry: Decodable { let name: String; let template: String }
            let ok: Bool
            let layouts: [Entry]?
            let error: TapErrorPayload?
        }
        let envelope = try JSONDecoder().decode(Envelope.self, from: data)
        guard envelope.ok, let layouts = envelope.layouts else {
            throw envelope.error ?? TapErrorPayload(code: "invalid_response", message: "tap printed no layouts")
        }
        return layouts.map { LayoutTemplate(name: $0.name, markdown: $0.template) }
    }

    /// "big-stat" reads "Big Stat" in a menu.
    public static func displayName(_ name: String) -> String {
        name.split(separator: "-").map { $0.prefix(1).uppercased() + $0.dropFirst() }.joined(separator: " ")
    }

    /// The name to insert for `requested`: itself when `templates` still
    /// offers it, or the catalog's first when tap has dropped it since it
    /// was last used. An empty catalog (not yet loaded) returns `requested`
    /// unchanged, since there is nothing to fall back to yet.
    public static func resolvedName(_ requested: String, in templates: [LayoutTemplate]) -> String {
        if templates.contains(where: { $0.name == requested }) { return requested }
        return templates.first?.name ?? requested
    }
}

/// The little picture a gallery cell draws for a layout, read off the
/// template's own markdown: headings, lines, slot columns, a code block,
/// a quote, media, a big number. A sketch of the template, not a render.
public enum LayoutSchematic {
    public enum Element: Equatable, Sendable {
        case heading, line, columns(Int), sidebar, code, quote, media, bigNumber
    }

    public static func elements(for markdown: String) -> [Element] {
        var elements: [Element] = []
        var columnMarkers = 0
        var insideSlot = false
        var insideFence = false
        let body = DirectiveComment.leading(in: markdown).map { (markdown as NSString).substring(from: NSMaxRange($0.range)) } ?? markdown
        for rawLine in body.components(separatedBy: "\n") {
            let line = rawLine.trimmingCharacters(in: .whitespaces)
            if line.hasPrefix("```") {
                if !insideFence { elements.append(.code) }
                insideFence.toggle()
                continue
            }
            if insideFence || line.isEmpty { continue }
            if line.hasPrefix("::") {
                insideSlot = true
                switch line {
                case "::left", "::center", "::right": columnMarkers += 1
                case "::sidebar": elements.append(.sidebar)
                case "::media": elements.append(.media)
                default: break
                }
                continue
            }
            if insideSlot { continue }
            if line.hasPrefix("# "), let first = line.dropFirst(2).first, first.isNumber {
                elements.append(.bigNumber)
            } else if line.hasPrefix("#") {
                elements.append(.heading)
            } else if line.hasPrefix(">") {
                if elements.last != .quote { elements.append(.quote) }
            } else if line.hasPrefix("![") {
                elements.append(.media)
            } else {
                elements.append(.line)
            }
        }
        if columnMarkers > 0 { elements.append(.columns(columnMarkers)) }
        return Array(elements.prefix(5))
    }
}

/// The layout New Slide inserts: the one used last, "default" at first.
public struct LastLayout {
    private let defaults: UserDefaults
    static let key = "LastSlideLayout"

    public init(defaults: UserDefaults = .standard) {
        self.defaults = defaults
    }

    public var name: String {
        get { defaults.string(forKey: Self.key) ?? "default" }
        nonmutating set { defaults.set(newValue, forKey: Self.key) }
    }
}
