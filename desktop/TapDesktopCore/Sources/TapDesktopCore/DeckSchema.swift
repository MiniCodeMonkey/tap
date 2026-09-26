import Foundation

/// One frontmatter key tap understands, from `tap deck schema --json`
/// (internal/config/schema.go). `type` is "string", "boolean", "integer",
/// "list", "object" (fixed nested `keys`) or "map" (entries named by the
/// deck, each with the nested `keys`). `defaultValue` is the default as
/// text, nil for none.
public struct SchemaKey: Equatable, Sendable {
    public let name: String
    public let type: String
    public let defaultValue: String?
    public let values: [String]
    public let description: String
    public let keys: [SchemaKey]

    public init(name: String, type: String, defaultValue: String? = nil, values: [String] = [], description: String = "", keys: [SchemaKey] = []) {
        self.name = name
        self.type = type
        self.defaultValue = defaultValue
        self.values = values
        self.description = description
        self.keys = keys
    }

    /// A key edited on one line: everything but an object or a map.
    public var isScalar: Bool { type != "object" && type != "map" }

    /// "aspectRatio" reads "Aspect ratio".
    public var label: String {
        var words = ""
        for (index, character) in name.enumerated() {
            if character.isUppercase, index > 0 { words += " " }
            words += index == 0 ? character.uppercased() : character.lowercased()
        }
        return words
    }
}

public enum DeckSchema {
    private struct Envelope: Decodable {
        let ok: Bool
        let keys: [Key]?
        let error: TapErrorPayload?
    }

    private struct Key: Decodable {
        let name: String
        let type: String
        let `default`: Default?
        let values: [String]?
        let description: String?
        let keys: [Key]?

        var schemaKey: SchemaKey {
            SchemaKey(name: name, type: type, defaultValue: `default`?.text, values: values ?? [],
                      description: description ?? "", keys: (keys ?? []).map(\.schemaKey))
        }
    }

    /// `default` is null, a string, a boolean or a number.
    private enum Default: Decodable {
        case text(String)

        var text: String {
            switch self {
            case .text(let value): return value
            }
        }

        init(from decoder: Decoder) throws {
            let container = try decoder.singleValueContainer()
            if let value = try? container.decode(Bool.self) { self = .text(value ? "true" : "false"); return }
            if let value = try? container.decode(Int.self) { self = .text(String(value)); return }
            if let value = try? container.decode(Double.self) { self = .text(String(value)); return }
            self = .text(try container.decode(String.self))
        }
    }

    public static func decode(_ data: Data) throws -> [SchemaKey] {
        let envelope = try JSONDecoder().decode(Envelope.self, from: data)
        guard envelope.ok, let keys = envelope.keys else {
            throw envelope.error ?? TapErrorPayload(code: "invalid_response", message: "tap printed no schema")
        }
        return keys.map(\.schemaKey)
    }

    /// The key a frontmatter path names: a map's entry name matches any
    /// name and continues into the map's nested keys.
    public static func key(at path: [String], in keys: [SchemaKey]) -> SchemaKey? {
        var siblings = keys
        var found: SchemaKey?
        var index = 0
        while index < path.count {
            guard let next = siblings.first(where: { $0.name == path[index] }) else { return nil }
            found = next
            siblings = next.keys
            index += 1
            if next.type == "map" {
                // The entry's own name, which the deck picks.
                guard index < path.count else { return found }
                index += 1
            }
        }
        return found
    }
}
