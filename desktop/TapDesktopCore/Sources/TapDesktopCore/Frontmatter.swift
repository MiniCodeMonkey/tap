import Foundation

/// The deck's frontmatter as text: the YAML block between the "---" lines
/// at the very start of the deck, read as lines rather than parsed as
/// YAML. tap's frontmatter is a map of scalars, one-level maps
/// (themeColors, recording) and the drivers map, and what the app
/// rewrites is one line at a time, so the rest of the block stays byte
/// for byte as the person wrote it. tap reads the same block
/// (internal/config/config.go, parseFrontmatter): the first line, trimmed,
/// must be "---", and the block ends at the next line that is.
///
/// An entry is a line "key: value" or "key:" at some indent; the lines
/// after it that are indented deeper, or blank, or comments, belong to
/// it, and the entries among them at the smallest indent are its
/// children. Anything else (list items, continuation lines) is kept
/// inside its entry's range and never read.
public struct Frontmatter: Equatable, Sendable {
    public struct Entry: Equatable, Sendable {
        public let key: String
        /// The text after "key:", trimmed; nil for "key:" alone, which
        /// opens a block (or is YAML's null).
        public let value: String?
        /// Where `value` sits in the whole text.
        public let valueRange: NSRange?
        public let children: [Entry]
        /// The entry's lines, children included, in the whole text. It
        /// ends with the line ending of its last line, so an insertion at
        /// its end starts a new line right below it.
        public let range: NSRange
        /// The indent of the entry's own line, in spaces.
        public let indent: Int

        public init(key: String, value: String?, valueRange: NSRange?, children: [Entry], range: NSRange, indent: Int, lineCount: Int = 1) {
            self.key = key
            self.value = value
            self.valueRange = valueRange
            self.children = children
            self.range = range
            self.indent = indent
            self.lineCount = lineCount
        }

        /// The value with YAML's quotes removed.
        public var unquotedValue: String? { value.map(Frontmatter.unquoted) }

        /// True for a value that is not one line: a block scalar ("|" or
        /// ">"), a list, or a plain scalar continued on indented lines.
        /// Such an entry is never edited as a scalar: the form shows its
        /// lines as text, and `setting` replaces the whole entry.
        public var isMultiLine: Bool {
            children.isEmpty && lineCount > 1
        }

        /// How many lines the entry's range holds, children included.
        public let lineCount: Int
    }

    /// The whole block from location 0 through the closing line's ending;
    /// nil when the deck has no frontmatter.
    public let range: NSRange?
    public let entries: [Entry]
    /// The line ending the deck uses, so an edit writes the same.
    public let lineEnding: String
    /// Where the closing "---" line starts: a new top-level key goes in front of it.
    public let closingLocation: Int?
    private let source: String

    public var hasFrontmatter: Bool { range != nil }

    private struct Line {
        let range: NSRange
        let content: String
    }

    public init(text: String) {
        source = text
        let nsText = text as NSString
        // The first line's own terminator, not a scan of the whole deck on every change.
        var firstStart = 0, firstLineEnd = 0, firstContentsEnd = 0
        nsText.getLineStart(&firstStart, end: &firstLineEnd, contentsEnd: &firstContentsEnd, for: NSRange(location: 0, length: 0))
        lineEnding = firstLineEnd - firstContentsEnd == 2 ? "\r\n" : "\n"
        var lines: [Line] = []
        var location = 0
        var closingIndex: Int?
        // Read lines only as far as the closing "---": the deck's body is never looked at.
        while location < nsText.length {
            var start = 0, end = 0, contentsEnd = 0
            nsText.getLineStart(&start, end: &end, contentsEnd: &contentsEnd, for: NSRange(location: location, length: 0))
            let content = nsText.substring(with: NSRange(location: start, length: contentsEnd - start))
            lines.append(Line(range: NSRange(location: start, length: end - start), content: content))
            if lines.count == 1, content.trimmingCharacters(in: .whitespaces) != "---" { break }
            if lines.count > 1, content.trimmingCharacters(in: .whitespaces) == "---" {
                closingIndex = lines.count - 1
                break
            }
            guard end > location else { break }
            location = end
        }
        guard let closingIndex else {
            range = nil
            closingLocation = nil
            entries = []
            return
        }
        range = NSRange(location: 0, length: NSMaxRange(lines[closingIndex].range))
        closingLocation = lines[closingIndex].range.location
        entries = Self.parse(lines: Array(lines[1..<closingIndex]))
    }

    private static let keyPattern = try! NSRegularExpression(pattern: #"^( *)([A-Za-z0-9_.-]+):(?:[ \t]+(.*?))?[ \t]*$"#)

    private struct KeyLine {
        let indent: Int
        let key: String
        let value: String?
        let valueRange: NSRange?
    }

    private static func keyLine(_ line: Line) -> KeyLine? {
        let content = line.content as NSString
        guard let match = keyPattern.firstMatch(in: line.content, range: NSRange(location: 0, length: content.length)) else { return nil }
        let indent = match.range(at: 1).length
        let key = content.substring(with: match.range(at: 2))
        var value: String?
        var valueRange: NSRange?
        let valueMatch = match.range(at: 3)
        if valueMatch.location != NSNotFound, valueMatch.length > 0 {
            // A " #" outside quotes starts a comment, which is not part of
            // the value and is kept where it is by a write; a value that is
            // only a comment is none.
            let text = Self.withoutTrailingComment(content.substring(with: valueMatch))
            if !text.isEmpty {
                value = text
                valueRange = NSRange(location: line.range.location + valueMatch.location, length: (text as NSString).length)
            }
        }
        return KeyLine(indent: indent, key: key, value: value, valueRange: valueRange)
    }

    /// `text` up to a "#" that starts a comment: one at the start, or one
    /// after a space, outside single and double quotes. Trailing
    /// whitespace before it goes too.
    static func withoutTrailingComment(_ text: String) -> String {
        var inSingle = false
        var inDouble = false
        var previous: Character = " "
        var kept = ""
        for character in text {
            if character == "\"", !inSingle { inDouble.toggle() }
            if character == "'", !inDouble { inSingle.toggle() }
            if character == "#", !inSingle, !inDouble, previous == " " || kept.isEmpty { break }
            kept.append(character)
            previous = character
        }
        return kept.trimmingCharacters(in: .whitespaces)
    }

    private static func isBlankOrComment(_ line: Line) -> Bool {
        let trimmed = line.content.trimmingCharacters(in: .whitespaces)
        return trimmed.isEmpty || trimmed.hasPrefix("#")
    }

    private static func parse(lines: [Line]) -> [Entry] {
        var entries: [Entry] = []
        var index = 0
        while index < lines.count {
            guard let opener = keyLine(lines[index]) else {
                index += 1
                continue
            }
            // The entry runs until the next key at an indent no deeper than its own.
            var end = index + 1
            while end < lines.count {
                if let next = keyLine(lines[end]), next.indent <= opener.indent { break }
                end += 1
            }
            // Trailing blank and comment lines belong to the gap, not to the
            // entry, so an insertion after the entry lands right below it.
            var last = end - 1
            while last > index, isBlankOrComment(lines[last]) { last -= 1 }
            let children = last > index ? parse(lines: Array(lines[(index + 1)...last])) : []
            let range = NSRange(location: lines[index].range.location, length: NSMaxRange(lines[last].range) - lines[index].range.location)
            entries.append(Entry(key: opener.key, value: opener.value, valueRange: opener.valueRange, children: children, range: range,
                                 indent: opener.indent, lineCount: last - index + 1))
            index = end
        }
        return entries
    }

    public func entry(at path: [String]) -> Entry? {
        var siblings = entries
        var found: Entry?
        for name in path {
            guard let next = siblings.first(where: { $0.key == name }) else { return nil }
            found = next
            siblings = next.children
        }
        return found
    }

    public func value(at path: [String]) -> String? {
        entry(at: path)?.value
    }

    /// An entry's lines, as written.
    public func text(of entry: Entry) -> String {
        (source as NSString).substring(with: entry.range)
    }

    /// The names under `drivers`, in the file's order: what tap's
    /// `Config.DeclaredDrivers()` holds for this text, block or flow style.
    public var declaredDrivers: [String] {
        guard let drivers = entry(at: ["drivers"]) else { return [] }
        if !drivers.children.isEmpty { return drivers.children.map(\.key) }
        if let value = drivers.value { return Self.flowMapKeys(value) }
        return []
    }

    public func declares(driver name: String) -> Bool {
        declaredDrivers.contains(name)
    }

    /// The keys of a flow map, "{shell: {}, sqlite: {path: x}}": the text
    /// before each top-level colon, split on the commas outside any braces.
    static func flowMapKeys(_ flow: String) -> [String] {
        let trimmed = flow.trimmingCharacters(in: .whitespaces)
        guard trimmed.hasPrefix("{"), trimmed.hasSuffix("}") else { return [] }
        var parts: [String] = []
        var current = ""
        var depth = 0
        for character in trimmed.dropFirst().dropLast() {
            switch character {
            case "{", "[":
                depth += 1
                current.append(character)
            case "}", "]":
                depth -= 1
                current.append(character)
            case "," where depth == 0:
                parts.append(current)
                current = ""
            default:
                current.append(character)
            }
        }
        parts.append(current)
        return parts.compactMap { part in
            let name = part.split(separator: ":", maxSplits: 1, omittingEmptySubsequences: false).first.map { $0.trimmingCharacters(in: .whitespaces) } ?? ""
            return name.isEmpty ? nil : name
        }
    }

    /// `value` with YAML's double or single quotes removed and their
    /// escapes undone; anything else as it is.
    public static func unquoted(_ value: String) -> String {
        if value.count >= 2, value.hasPrefix("\""), value.hasSuffix("\"") {
            var result = ""
            var escaped = false
            for character in value.dropFirst().dropLast() {
                if escaped {
                    switch character {
                    case "n": result.append("\n")
                    case "t": result.append("\t")
                    default: result.append(character)
                    }
                    escaped = false
                } else if character == "\\" {
                    escaped = true
                } else {
                    result.append(character)
                }
            }
            return result
        }
        if value.count >= 2, value.hasPrefix("'"), value.hasSuffix("'") {
            return String(value.dropFirst().dropLast()).replacingOccurrences(of: "''", with: "'")
        }
        return value
    }
}
