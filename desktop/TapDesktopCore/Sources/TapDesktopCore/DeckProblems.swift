import Foundation

/// One thing wrong with the deck's settings, as tap reports it in a
/// `deck-problems` event (internal/config/problems.go) and as the app finds
/// it in the text it holds. `key` is the frontmatter key with dots for
/// nested keys. `suggestions` are allowed values, the closest first;
/// `allowed` is every value the key accepts, empty for free text.
public struct DeckProblem: Equatable, Sendable, Codable {
    public enum Severity: String, Equatable, Sendable, Codable {
        /// tap cannot render the deck until it is fixed.
        case error
        /// tap renders the deck with a fallback.
        case warning
    }

    public let key: String
    public let value: String
    public let message: String
    public let severity: Severity
    public let suggestions: [String]
    public let allowed: [String]

    public init(key: String, value: String, message: String, severity: Severity, suggestions: [String] = [], allowed: [String] = []) {
        self.key = key
        self.value = value
        self.message = message
        self.severity = severity
        self.suggestions = suggestions
        self.allowed = allowed
    }

    private enum CodingKeys: String, CodingKey { case key, value, message, severity, suggestions, allowed }

    /// Reads what tap sends, tolerating what a later tap leaves out: no
    /// suggestions or allowed values, and a severity this app does not know
    /// counts as an error, the one that stops the preview.
    public init(from decoder: Decoder) throws {
        let container = try decoder.container(keyedBy: CodingKeys.self)
        key = try container.decode(String.self, forKey: .key)
        value = try container.decodeIfPresent(String.self, forKey: .value) ?? ""
        message = try container.decodeIfPresent(String.self, forKey: .message) ?? ""
        severity = (try container.decodeIfPresent(String.self, forKey: .severity)).flatMap(Severity.init(rawValue:)) ?? .error
        suggestions = try container.decodeIfPresent([String].self, forKey: .suggestions) ?? []
        allowed = try container.decodeIfPresent([String].self, forKey: .allowed) ?? []
    }

    public var path: [String] { key.split(separator: ".").map(String.init) }
}

/// A one-click repair of a problem: the button's title, the edit, and the
/// undo step's name.
public struct DeckProblemFix: Equatable, Sendable {
    public let title: String
    public let value: String
    public let replacement: TextReplacement
    public let actionName: String
}

/// Reading a deck's settings for problems, offering the fix for each, and
/// summing the deck up on one line. Nothing here knows an allowed value:
/// they come from tap's schema (`tap deck schema --json`), so a value tap
/// adds later is understood without a new app.
public enum DeckProblems {
    /// The problems of the frontmatter's settings: a top-level string key
    /// (or a key inside an object key) whose value is not among the
    /// schema's allowed values. An unknown theme is a warning, because tap
    /// renders with Base; every other key is an error. Sorted as the
    /// schema lists the keys.
    public static func evaluate(_ frontmatter: Frontmatter, schema: [SchemaKey]) -> [DeckProblem] {
        var problems: [DeckProblem] = []
        func visit(_ keys: [SchemaKey], prefix: [String]) {
            for key in keys {
                let path = prefix + [key.name]
                if key.type == "object" {
                    visit(key.keys, prefix: path)
                    continue
                }
                guard key.type == "string", !key.values.isEmpty,
                      let entry = frontmatter.entry(at: path), let written = entry.unquotedValue, !written.isEmpty,
                      !isNull(entry), !key.values.contains(written) else { continue }
                problems.append(problem(for: key, path: path, written: written))
            }
        }
        visit(schema, prefix: [])
        return problems
    }

    /// A value written as YAML null (`~` or `null`, unquoted) is a setting left unset, as tap reads it.
    private static func isNull(_ entry: Frontmatter.Entry) -> Bool {
        guard let raw = entry.value?.trimmingCharacters(in: .whitespaces) else { return false }
        return raw == "~" || raw.lowercased() == "null"
    }

    private static func problem(for key: SchemaKey, path: [String], written: String) -> DeckProblem {
        let suggestions = nearestValues(to: written, in: key.values)
        if path == ["theme"] {
            return DeckProblem(key: "theme", value: written, message: "\u{201C}\(written)\u{201D} is not a tap theme. The preview uses Base for now.",
                               severity: .warning, suggestions: suggestions, allowed: key.values)
        }
        return DeckProblem(key: path.joined(separator: "."), value: written,
                           message: "The \(key.label.lowercased()) \u{201C}\(written)\u{201D} is not one tap supports. Use \(alternatives(key.values)).",
                           severity: .error, suggestions: suggestions, allowed: key.values)
    }

    /// "a, b or c".
    public static func alternatives(_ values: [String]) -> String {
        guard values.count > 1 else { return values.first ?? "" }
        return values.dropLast().joined(separator: ", ") + " or " + values[values.count - 1]
    }

    public static func hasErrors(_ problems: [DeckProblem]) -> Bool {
        problems.contains { $0.severity == .error }
    }

    /// The card's heading for the errors: "This deck's settings need one fix", "need 2 fixes".
    public static func heading(errorCount: Int) -> String {
        errorCount == 1 ? "This deck\u{2019}s settings need one fix" : "This deck\u{2019}s settings need \(errorCount) fixes"
    }

    // MARK: Suggestions

    /// The allowed values closest to `value`, best first, at most three:
    /// the same rule as tap's own (internal/config/problems.go). A value
    /// that matches one after case and separator differences ("16/9" for
    /// "16:9", "Keynote" for "keynote") suggests only that one; otherwise a
    /// value is close when its edit distance is at most a third of the
    /// longer word, and never more than 3.
    public static func nearestValues(to value: String, in allowed: [String]) -> [String] {
        let normalized = normalizedForMatching(value)
        if let exact = allowed.first(where: { normalizedForMatching($0) == normalized && $0 != value }) { return [exact] }
        let scored: [(value: String, distance: Int)] = allowed.compactMap { candidate in
            let candidateText = normalizedForMatching(candidate)
            let limit = min(3, max(1, max(normalized.utf8.count, candidate.utf8.count) / 3))
            let distance = editDistance(Array(normalized.utf8), Array(candidateText.utf8))
            return distance <= limit ? (candidate, distance) : nil
        }
        return Array(scored.enumerated().sorted { ($0.element.distance, $0.offset) < ($1.element.distance, $1.offset) }.prefix(3).map(\.element.value))
    }

    private static func normalizedForMatching(_ value: String) -> String {
        var text = value.trimmingCharacters(in: .whitespaces).lowercased()
        if text.range(of: "^[0-9]+x[0-9]+$", options: .regularExpression) != nil { text = text.replacingOccurrences(of: "x", with: ":") }
        for (separator, replacement) in [("/", ":"), (";", ":"), ("_", "-"), (" ", "-")] { text = text.replacingOccurrences(of: separator, with: replacement) }
        return text
    }

    private static func editDistance(_ first: [UInt8], _ second: [UInt8]) -> Int {
        var previous = Array(0...second.count)
        for row in 1...max(first.count, 1) where !first.isEmpty {
            var current = [row] + Array(repeating: 0, count: second.count)
            for column in 1...max(second.count, 1) where !second.isEmpty {
                current[column] = min(previous[column] + 1, current[column - 1] + 1, previous[column - 1] + (first[row - 1] == second[column - 1] ? 0 : 1))
            }
            previous = current
        }
        return previous[second.count]
    }

    // MARK: Fixes

    /// The one-click fix for a problem: the closest allowed value, or the
    /// schema's default when nothing is close, written over the setting as
    /// one edit. `displayName` gives a value's name for the button ("Use
    /// Keynote" for the slug "keynote"). nil when the frontmatter cannot
    /// take the edit, or when there is no value to offer.
    public static func fix(for problem: DeckProblem, in frontmatter: Frontmatter, schema: [SchemaKey],
                           displayName: (String) -> String = { $0 }) -> DeckProblemFix? {
        guard let value = suggestedValue(for: problem, schema: schema) else { return nil }
        return fix(setting: problem.key, to: value, in: frontmatter, displayName: displayName)
    }

    /// The fix that writes `value` over `key`, for a suggestion the person picked.
    public static func fix(setting key: String, to value: String, in frontmatter: Frontmatter, displayName: (String) -> String = { $0 }) -> DeckProblemFix? {
        let path = key.split(separator: ".").map(String.init)
        guard let replacement = frontmatter.setting(path: path, to: Frontmatter.scalar(forString: value)) else { return nil }
        return DeckProblemFix(title: "Use \(displayName(value))", value: value, replacement: replacement, actionName: "Use \(displayName(value))")
    }

    /// The value a problem's button writes: the closest, else the schema's default when allowed, else the first allowed.
    public static func suggestedValue(for problem: DeckProblem, schema: [SchemaKey]) -> String? {
        if let first = problem.suggestions.first { return first }
        let allowed = problem.allowed.isEmpty ? (DeckSchema.key(at: problem.path, in: schema)?.values ?? []) : problem.allowed
        if let fallback = DeckSchema.key(at: problem.path, in: schema)?.defaultValue, allowed.contains(fallback) { return fallback }
        return allowed.first
    }

    // MARK: Frontmatter that does not parse

    /// The 0-based line of the deck's text that a frontmatter error names:
    /// tap's yaml reader counts lines from the first line after the opening
    /// "---", so "yaml: line 3" is the fourth line of the text. An error
    /// with no line (the frontmatter never closes) names the opening line.
    public static func failingLine(inDeckErrors errors: [String]) -> Int? {
        for error in errors {
            guard error.hasPrefix("frontmatter") else { continue }
            if let range = error.range(of: "line [0-9]+", options: .regularExpression),
               let number = Int(error[range].dropFirst("line ".count)) {
                return number
            }
            return 0
        }
        return nil
    }

    // MARK: The collapsed card

    public struct Chip: Equatable, Sendable {
        public enum Kind: Equatable, Sendable {
            case plain
            /// An error count, in red.
            case problem
            /// An unknown value tap works around, in amber.
            case warning
        }

        public let text: String
        public let kind: Kind

        public init(_ text: String, _ kind: Kind = .plain) {
            self.text = text
            self.kind = kind
        }
    }

    /// What the collapsed card shows after "Deck": the theme's name, the
    /// aspect ratio (the default when the deck sets none) and the author
    /// when set. A setting with a problem is not shown as itself: the
    /// errors are one red chip ("1 problem", "2 problems"), and an unknown
    /// theme is an amber chip with the name as written.
    public static func chips(for frontmatter: Frontmatter, schema: [SchemaKey], problems: [DeckProblem], themeName: (String) -> String) -> [Chip] {
        var chips: [Chip] = []
        let errors = problems.filter { $0.severity == .error }
        if !errors.isEmpty { chips.append(Chip(errors.count == 1 ? "1 problem" : "\(errors.count) problems", .problem)) }
        if let warning = problems.first(where: { $0.key == "theme" && $0.severity == .warning }) {
            chips.append(Chip(warning.value, .warning))
        } else {
            let slug = frontmatter.entry(at: ["theme"])?.unquotedValue.flatMap { $0.isEmpty ? nil : $0 }
            chips.append(Chip(slug.map(themeName) ?? "Default"))
        }
        if !errors.contains(where: { $0.key == "aspectRatio" }) {
            let written = frontmatter.entry(at: ["aspectRatio"])?.unquotedValue.flatMap { $0.isEmpty ? nil : $0 }
            if let ratio = written ?? schema.first(where: { $0.name == "aspectRatio" })?.defaultValue { chips.append(Chip(ratio)) }
        }
        if let author = frontmatter.entry(at: ["author"])?.unquotedValue, !author.isEmpty { chips.append(Chip(author)) }
        return chips
    }
}
