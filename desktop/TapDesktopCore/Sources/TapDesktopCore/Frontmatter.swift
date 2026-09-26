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
/// An entry is a line "key: value" or "key:" at some indent, the key plain
/// (any text up to the first ": ", "og:image" and "årstal" included) or
/// quoted. The lines after it that are indented deeper, blank, or
/// comments belong to it, and so do "- " list items at its own indent
/// under a bare "key:"; the first other line ends it. A flow map or list
/// that opens on the key's line runs to the line that closes it. The
/// entries among a bare "key:"'s lines at the smallest indent are its
/// children. A line that is no entry and belongs to none (a stray "[odd]"
/// at the top) is kept out of every entry's range, so no edit touches it.
public struct Frontmatter: Equatable, Sendable {
    public struct Entry: Equatable, Sendable {
        /// The key as YAML reads it: a quoted key without its quotes.
        public let key: String
        /// The text after "key:", trimmed; nil for "key:" alone, which
        /// opens a block (or is YAML's null). For a flow map or list
        /// over several lines, its lines joined with a space.
        public let value: String?
        /// Where `value` sits in the whole text; nil for a flow
        /// collection over several lines, which is never edited in place.
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
        /// ">"), a list, a flow collection over several lines, or a plain
        /// scalar continued on indented lines. Such an entry is never
        /// edited as a scalar: the form shows its lines as text, and
        /// `setting` replaces the whole entry.
        public var isMultiLine: Bool {
            children.isEmpty && lineCount > 1
        }

        /// How many lines the entry's range holds, children included.
        public let lineCount: Int
    }

    /// The whole block from location 0 through the closing line's ending;
    /// nil when the deck has no frontmatter, or one that never closes.
    public let range: NSRange?
    public let entries: [Entry]
    /// The line ending most of the frontmatter's lines use (the first
    /// line's when there is no frontmatter), for a new frontmatter. An
    /// edit next to existing lines takes the ending of the line it lands
    /// beside instead, so no edited line gains an ending its neighbours lack.
    public let lineEnding: String
    /// Where the closing "---" line starts: a new top-level key goes in front of it.
    public let closingLocation: Int?
    /// True when the deck opens with "---" and no line closes it. tap
    /// refuses such a deck ("frontmatter not closed"), and every edit here
    /// is nil for it: the caller shows tap's error instead.
    public let isUnterminated: Bool
    private let source: String

    public var hasFrontmatter: Bool { range != nil }

    private struct Line {
        let range: NSRange
        let content: String
        /// The line's own terminator: "\r\n", "\n", "\r", or "" for the last line.
        let ending: String
    }

    public init(text: String) {
        source = text
        let nsText = text as NSString
        var lines: [Line] = []
        var location = 0
        var closingIndex: Int?
        // Read lines only as far as the closing "---": the deck's body is never looked at.
        while location < nsText.length {
            var start = 0, end = 0, contentsEnd = 0
            nsText.getLineStart(&start, end: &end, contentsEnd: &contentsEnd, for: NSRange(location: location, length: 0))
            let content = nsText.substring(with: NSRange(location: start, length: contentsEnd - start))
            let ending = nsText.substring(with: NSRange(location: contentsEnd, length: end - contentsEnd))
            lines.append(Line(range: NSRange(location: start, length: end - start), content: content, ending: ending))
            if lines.count == 1, content.trimmingCharacters(in: .whitespaces) != "---" { break }
            if lines.count > 1, content.trimmingCharacters(in: .whitespaces) == "---" {
                closingIndex = lines.count - 1
                break
            }
            guard end > location else { break }
            location = end
        }
        lineEnding = Self.dominantEnding(of: closingIndex.map { Array(lines[0...$0]) } ?? Array(lines.prefix(1)))
        guard let closingIndex else {
            range = nil
            closingLocation = nil
            entries = []
            isUnterminated = lines.first.map { $0.content.trimmingCharacters(in: .whitespaces) == "---" } ?? false
            return
        }
        isUnterminated = false
        range = NSRange(location: 0, length: NSMaxRange(lines[closingIndex].range))
        closingLocation = lines[closingIndex].range.location
        entries = Self.parse(lines: Array(lines[1..<closingIndex]))
    }

    /// The ending most of `lines` end with: the first line's on a tie, "\n" when none has one.
    private static func dominantEnding(of lines: [Line]) -> String {
        guard let first = lines.first(where: { !$0.ending.isEmpty })?.ending else { return "\n" }
        var counts: [String: Int] = [:]
        for line in lines where !line.ending.isEmpty { counts[line.ending, default: 0] += 1 }
        let most = counts.values.max() ?? 0
        return counts[first] == most ? first : counts.first { $0.value == most }?.key ?? first
    }

    private struct KeyLine {
        let indent: Int
        let key: String
        let value: String?
        let valueRange: NSRange?
    }

    private static let space = UInt16(UInt8(ascii: " "))
    private static let tab = UInt16(UInt8(ascii: "\t"))
    private static let colon = UInt16(UInt8(ascii: ":"))
    private static let hash = UInt16(UInt8(ascii: "#"))
    private static let dash = UInt16(UInt8(ascii: "-"))
    private static let doubleQuote = UInt16(UInt8(ascii: "\""))
    private static let singleQuote = UInt16(UInt8(ascii: "'"))
    private static let backslash = UInt16(UInt8(ascii: "\\"))
    private static let flowOpeners: Set<UInt16> = Set("{[".utf16)
    private static let flowClosers: Set<UInt16> = Set("}]".utf16)
    /// Characters that cannot start a plain key: YAML gives each a meaning there.
    private static let notAKeyStart: Set<UInt16> = Set("\t#[]{},?&*!|>%@`".utf16)

    private static func isSpaceOrTab(_ unit: UInt16) -> Bool { unit == space || unit == tab }

    private static func indent(of line: Line) -> Int {
        line.content.utf16.prefix { $0 == space }.count
    }

    /// The indent of a "- " list item (or a lone "-"), nil for any other line.
    private static func listItemIndent(_ line: Line) -> Int? {
        let units = Array(line.content.utf16)
        let index = indent(of: line)
        guard index < units.count, units[index] == dash, index + 1 == units.count || isSpaceOrTab(units[index + 1]) else { return nil }
        return index
    }

    /// The line as "key: value" or "key:", nil for any other line.
    private static func keyLine(_ line: Line) -> KeyLine? {
        let units = Array(line.content.utf16)
        let content = line.content as NSString
        let count = units.count
        let start = indent(of: line)
        guard start < count, !notAKeyStart.contains(units[start]), listItemIndent(line) == nil else { return nil }
        let key: String
        var colonIndex: Int
        if units[start] == doubleQuote || units[start] == singleQuote {
            guard let close = closingQuote(in: units, from: start) else { return nil }
            key = unquoted(content.substring(with: NSRange(location: start, length: close - start + 1)))
            colonIndex = close + 1
            while colonIndex < count, isSpaceOrTab(units[colonIndex]) { colonIndex += 1 }
            guard colonIndex < count, units[colonIndex] == colon, colonIndex + 1 == count || isSpaceOrTab(units[colonIndex + 1]) else { return nil }
        } else {
            // A plain key runs to the first ":" that ends the line or has a space or tab after it.
            colonIndex = start
            while colonIndex < count {
                if units[colonIndex] == colon, colonIndex + 1 == count || isSpaceOrTab(units[colonIndex + 1]) { break }
                if units[colonIndex] == hash, isSpaceOrTab(units[colonIndex - 1]) { return nil }
                colonIndex += 1
            }
            guard colonIndex < count else { return nil }
            var keyEnd = colonIndex
            while keyEnd > start, isSpaceOrTab(units[keyEnd - 1]) { keyEnd -= 1 }
            key = content.substring(with: NSRange(location: start, length: keyEnd - start))
        }
        var valueStart = colonIndex + 1
        while valueStart < count, isSpaceOrTab(units[valueStart]) { valueStart += 1 }
        var value: String?
        var valueRange: NSRange?
        if valueStart < count {
            // A comment is not part of the value and is kept where it is
            // by a write; a value that is only a comment is none.
            let text = withoutTrailingComment(content.substring(from: valueStart))
            if !text.isEmpty {
                value = text
                valueRange = NSRange(location: line.range.location + valueStart, length: (text as NSString).length)
            }
        }
        return KeyLine(indent: start, key: key, value: value, valueRange: valueRange)
    }

    /// The index of the quote that closes the one at `start`: `\` escapes
    /// the next character in double quotes, `''` is a quote in single ones.
    private static func closingQuote(in units: [UInt16], from start: Int) -> Int? {
        let quote = units[start]
        var index = start + 1
        while index < units.count {
            if quote == doubleQuote, units[index] == backslash {
                index += 2
                continue
            }
            if units[index] == quote {
                if quote == singleQuote, index + 1 < units.count, units[index + 1] == singleQuote {
                    index += 2
                    continue
                }
                return index
            }
            index += 1
        }
        return nil
    }

    private static let spacesAndTabs = CharacterSet(charactersIn: " \t")

    /// `text` (a value, from its first character) up to the comment that
    /// ends it, trailing spaces and tabs removed. A comment is a "#" at the
    /// start or after a space or a tab. A value that opens with a quote is
    /// read to its closing quote first, so a "#" inside the quotes is
    /// text; any other value is plain, and an apostrophe in it is only an
    /// apostrophe.
    static func withoutTrailingComment(_ text: String) -> String {
        let units = Array(text.utf16)
        var index = 0
        if let first = units.first, first == doubleQuote || first == singleQuote {
            index = (closingQuote(in: units, from: 0) ?? units.count - 1) + 1
        }
        while index < units.count {
            if units[index] == hash, index == 0 || isSpaceOrTab(units[index - 1]) { break }
            index += 1
        }
        return (text as NSString).substring(to: index).trimmingCharacters(in: spacesAndTabs)
    }

    private static func isBlankOrComment(_ line: Line) -> Bool {
        let trimmed = line.content.trimmingCharacters(in: spacesAndTabs)
        return trimmed.isEmpty || trimmed.hasPrefix("#")
    }

    /// `depth` plus the flow collections `text` opens, minus those it
    /// closes, outside quoted scalars and before a comment.
    private static func flowDepth(_ text: String, from depth: Int = 0) -> Int {
        let units = Array(withoutTrailingComment(text).utf16)
        var depth = depth
        var index = 0
        var previous = space
        while index < units.count {
            let unit = units[index]
            if unit == doubleQuote || unit == singleQuote, isSpaceOrTab(previous) || flowOpeners.contains(previous) || previous == colon
                || previous == UInt16(UInt8(ascii: ",")), let close = closingQuote(in: units, from: index) {
                previous = unit
                index = close + 1
                continue
            }
            if flowOpeners.contains(unit) { depth += 1 }
            if flowClosers.contains(unit) { depth -= 1 }
            previous = unit
            index += 1
        }
        return depth
    }

    private static func parse(lines: [Line]) -> [Entry] {
        var entries: [Entry] = []
        var index = 0
        while index < lines.count {
            guard let opener = keyLine(lines[index]) else {
                index += 1
                continue
            }
            var end = index + 1
            var value = opener.value
            var valueRange = opener.valueRange
            var depth = opener.value.map { flowDepth($0) } ?? 0
            if depth > 0 {
                // A flow collection runs to the line that closes it, whatever that line's indent.
                var joined = [opener.value ?? ""]
                while end < lines.count, depth > 0 {
                    let text = lines[end].content.trimmingCharacters(in: spacesAndTabs)
                    depth = flowDepth(text, from: depth)
                    let kept = withoutTrailingComment(text)
                    if !kept.isEmpty { joined.append(kept) }
                    end += 1
                }
                value = joined.joined(separator: " ")
                valueRange = nil
            } else {
                // The entry ends at the first line that is not blank, not a
                // comment, and not indented deeper than its key, except a
                // list item at the key's own indent under a bare "key:".
                while end < lines.count {
                    let line = lines[end]
                    if !isBlankOrComment(line), indent(of: line) <= opener.indent,
                       opener.value != nil || listItemIndent(line) != opener.indent { break }
                    end += 1
                }
            }
            // Trailing blank and comment lines belong to the gap, not to the
            // entry, so an insertion after the entry lands right below it.
            var last = end - 1
            while last > index, isBlankOrComment(lines[last]) { last -= 1 }
            let children = last > index && childrenAreEntries(opener: opener, lines: lines[(index + 1)...last])
                ? parse(lines: Array(lines[(index + 1)...last])) : []
            let range = NSRange(location: lines[index].range.location, length: NSMaxRange(lines[last].range) - lines[index].range.location)
            entries.append(Entry(key: opener.key, value: value, valueRange: valueRange, children: children, range: range,
                                 indent: opener.indent, lineCount: last - index + 1))
            index = end
        }
        return entries
    }

    /// True when the lines under a key are a map of their own: the key
    /// has no value on its line and the first of them is no list item. A
    /// block scalar's text or a list's items are never read as keys.
    private static func childrenAreEntries(opener: KeyLine, lines: ArraySlice<Line>) -> Bool {
        guard opener.value == nil, let first = lines.first(where: { !isBlankOrComment($0) }) else { return false }
        return listItemIndent(first) == nil
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

    /// The ending of the line that ends right at `location` (a line
    /// start), `lineEnding` when there is none.
    private func ending(before location: Int) -> String {
        let text = source as NSString
        if location >= 2, text.substring(with: NSRange(location: location - 2, length: 2)) == "\r\n" { return "\r\n" }
        guard location >= 1 else { return lineEnding }
        let last = text.substring(with: NSRange(location: location - 1, length: 1))
        return last == "\n" || last == "\r" ? last : lineEnding
    }

    /// The key as written on the entry's line, quotes kept.
    private func writtenKey(of entry: Entry) -> String {
        let text = source as NSString
        let lineStart = entry.range.location + entry.indent
        let units = Array(text.substring(with: NSRange(location: lineStart, length: NSMaxRange(entry.range) - lineStart)).utf16)
        var end = 0
        if let first = units.first, first == Self.doubleQuote || first == Self.singleQuote, let close = Self.closingQuote(in: units, from: 0) {
            end = close + 1
        } else {
            while end < units.count, !(units[end] == Self.colon && (end + 1 == units.count || [Self.space, Self.tab, 10, 13].contains(units[end + 1]))) {
                end += 1
            }
            while end > 0, Self.isSpaceOrTab(units[end - 1]) { end -= 1 }
        }
        return text.substring(with: NSRange(location: lineStart, length: end))
    }

    private static let nullWords: Set<String> = ["~", "null", "Null", "NULL"]

    /// The one edit that gives the key at `path` the raw scalar `value`
    /// (nil removes the key with everything under it), making the
    /// frontmatter, the key's parents and the key as needed: a missing
    /// top-level key goes before the closing line, a missing child right
    /// below its parent's last child at that child's indent (or two spaces
    /// deeper than the parent), a "key:" with nothing under it opens into
    /// a block, "key: {}" and "key: ~" too (a comment after them stays on
    /// the key's line), and a flow map with pairs gains one more. nil when
    /// the edit changes nothing (removing what is not there), when the
    /// frontmatter never closes, or when it would need to rewrite inside
    /// a flow collection, a list or a block scalar, which this type does
    /// not do (the Deck tab then offers the raw text).
    public func setting(path: [String], to value: String?) -> TextReplacement? {
        guard let key = path.last, !isUnterminated else { return nil }
        guard range != nil, let closingLocation else {
            guard let value else { return nil }
            var lines = ["---"]
            for (depth, name) in path.dropLast().enumerated() { lines.append(Self.spaces(depth * 2) + name + ":") }
            lines.append(Self.spaces((path.count - 1) * 2) + key + ": " + value)
            lines.append("---")
            lines.append("")
            return TextReplacement(range: NSRange(location: 0, length: 0), replacement: lines.joined(separator: lineEnding) + lineEnding)
        }
        var parent: Entry?
        var siblings = entries
        var remaining = path[...]
        while let name = remaining.first, let found = siblings.first(where: { $0.key == name }) {
            if remaining.count == 1 {
                guard let value else { return TextReplacement(range: found.range, replacement: "") }
                if let valueRange = found.valueRange, found.children.isEmpty, !found.isMultiLine {
                    return TextReplacement(range: valueRange, replacement: value)
                }
                // A block, a bare "key:", or a value on several lines becomes one scalar line.
                return TextReplacement(range: found.range, replacement: Self.spaces(found.indent) + writtenKey(of: found) + ": " + value
                                       + ending(before: NSMaxRange(found.range)))
            }
            parent = found
            siblings = found.children
            remaining = remaining.dropFirst()
        }
        guard let value else { return nil }
        let baseIndent: Int
        let insertion: Int
        if let parent {
            if let flow = parent.value {
                guard let valueRange = parent.valueRange, parent.lineCount == 1, remaining.count == 1 else { return nil }
                let trimmed = flow.trimmingCharacters(in: .whitespaces)
                let isEmptyFlowMap = trimmed.hasPrefix("{") && trimmed.hasSuffix("}")
                    && trimmed.dropFirst().dropLast().trimmingCharacters(in: .whitespaces).isEmpty
                if isEmptyFlowMap || Self.nullWords.contains(trimmed) {
                    // The value goes, the rest of its line (a comment) stays, and the child goes below it.
                    let text = source as NSString
                    let ending = ending(before: NSMaxRange(parent.range))
                    let lineContentEnd = NSMaxRange(parent.range) - (ending as NSString).length
                    let head = text.substring(with: NSRange(location: parent.range.location, length: valueRange.location - parent.range.location))
                        .trimmingCharacters(in: Self.spacesAndTabs)
                    let tail = text.substring(with: NSRange(location: NSMaxRange(valueRange), length: lineContentEnd - NSMaxRange(valueRange)))
                    let line = Self.spaces(parent.indent) + head + (tail.trimmingCharacters(in: Self.spacesAndTabs).isEmpty ? "" : tail)
                    return TextReplacement(range: parent.range, replacement: line + ending + Self.spaces(parent.indent + 2) + key + ": " + value + ending)
                }
                guard trimmed.hasPrefix("{"), trimmed.hasSuffix("}"),
                      !value.trimmingCharacters(in: .whitespaces).hasPrefix("{") || value.trimmingCharacters(in: .whitespaces) == "{}" else { return nil }
                let inner = trimmed.dropFirst().dropLast().trimmingCharacters(in: .whitespaces)
                return TextReplacement(range: valueRange, replacement: "{" + inner + ", " + key + ": " + value + "}")
            }
            // A bare "key:" over a list has no map to add to.
            guard !parent.isMultiLine else { return nil }
            baseIndent = parent.children.first?.indent ?? parent.indent + 2
            insertion = NSMaxRange(parent.range)
        } else {
            baseIndent = 0
            insertion = closingLocation
        }
        let ending = ending(before: insertion)
        var lines: [String] = []
        let newParents = remaining.dropLast()
        for (depth, name) in newParents.enumerated() { lines.append(Self.spaces(baseIndent + depth * 2) + name + ":") }
        lines.append(Self.spaces(baseIndent + newParents.count * 2) + key + ": " + value)
        return TextReplacement(range: NSRange(location: insertion, length: 0), replacement: lines.joined(separator: ending) + ending)
    }

    /// The fix-it: "<name>: {}" under drivers. nil when it is declared
    /// already, or when the frontmatter cannot take the edit.
    public func addingDriver(_ name: String) -> TextReplacement? {
        guard !declares(driver: name) else { return nil }
        return setting(path: ["drivers", name], to: "{}")
    }

    /// An entry's lines as written, for the Deck tab's raw text field.
    public func rawBlock(at path: [String]) -> String? {
        entry(at: path).map(text(of:))
    }

    /// Replaces an entry's lines with `raw`, which the caller has already
    /// indented and terminated. nil when there is no such entry.
    public func settingRawBlock(at path: [String], to raw: String) -> TextReplacement? {
        entry(at: path).map { TextReplacement(range: $0.range, replacement: raw) }
    }

    /// `replacement` applied to `text`.
    public static func applying(_ replacement: TextReplacement, to text: String) -> String {
        (text as NSString).replacingCharacters(in: replacement.range, with: replacement.replacement)
    }

    private static func spaces(_ count: Int) -> String { String(repeating: " ", count: count) }

    private static let yamlWords: Set<String> = ["true", "false", "null", "~", "yes", "no", "on", "off"]
    private static let numberPattern = try! NSRegularExpression(pattern: #"^[-+]?(\d[\d_]*(\.\d*)?|\.\d+)([eE][-+]?\d+)?$"#)
    private static let unsafeLeading: Set<Character> = ["-", "?", ":", ",", "[", "]", "{", "}", "#", "&", "*", "!", "|", ">", "'", "\"", "%", "@", "`"]

    /// `string` as a YAML scalar: as it is when YAML reads it back as the
    /// same string, double-quoted with escapes otherwise (a number, a
    /// boolean or null in any case, a leading character YAML gives a
    /// meaning to, a `"` anywhere, ": " or " #" inside, leading or
    /// trailing whitespace, any line break (\n, \r, "\r\n", U+0085,
    /// U+2028, U+2029), or nothing at all).
    public static func scalar(forString string: String) -> String {
        let needsQuotes = string.isEmpty
            || string != string.trimmingCharacters(in: .whitespacesAndNewlines)
            || string.rangeOfCharacter(from: .newlines) != nil
            || yamlWords.contains(string.lowercased())
            || numberPattern.firstMatch(in: string, range: NSRange(location: 0, length: (string as NSString).length)) != nil
            || string.first.map { unsafeLeading.contains($0) } == true
            || string.contains(": ")
            || string.contains(" #")
            || string.hasSuffix(":")
            || string.contains("\"")
        guard needsQuotes else { return string }
        var quoted = "\""
        // Unicode scalars, not Characters: "\r\n" is one Character.
        for scalar in string.unicodeScalars {
            switch scalar {
            case "\\": quoted += "\\\\"
            case "\"": quoted += "\\\""
            case "\n": quoted += "\\n"
            case "\r": quoted += "\\r"
            case "\t": quoted += "\\t"
            case "\u{0B}": quoted += "\\v"
            case "\u{0C}": quoted += "\\f"
            case "\u{85}": quoted += "\\N"
            case "\u{2028}": quoted += "\\L"
            case "\u{2029}": quoted += "\\P"
            default: quoted.unicodeScalars.append(scalar)
            }
        }
        return quoted + "\""
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
    /// before each top-level colon, split on the commas outside any
    /// braces, YAML's quotes removed.
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
            return name.isEmpty ? nil : unquoted(name)
        }
    }

    /// `value` with YAML's double or single quotes removed and their
    /// escapes undone; anything else as it is.
    public static func unquoted(_ value: String) -> String {
        if value.count >= 2, value.hasPrefix("\""), value.hasSuffix("\"") {
            var result = ""
            var escaped = false
            for scalar in value.unicodeScalars.dropFirst().dropLast() {
                if escaped {
                    switch scalar {
                    case "n": result.unicodeScalars.append("\n")
                    case "r": result.unicodeScalars.append("\r")
                    case "t": result.unicodeScalars.append("\t")
                    case "v": result.unicodeScalars.append("\u{0B}")
                    case "f": result.unicodeScalars.append("\u{0C}")
                    case "N": result.unicodeScalars.append("\u{85}")
                    case "L": result.unicodeScalars.append("\u{2028}")
                    case "P": result.unicodeScalars.append("\u{2029}")
                    default: result.unicodeScalars.append(scalar)
                    }
                    escaped = false
                } else if scalar == "\\" {
                    escaped = true
                } else {
                    result.unicodeScalars.append(scalar)
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
