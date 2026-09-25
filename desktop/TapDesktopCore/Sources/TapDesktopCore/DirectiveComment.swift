import Foundation

/// A slide's directive comment: the HTML comment at the very start of the
/// slide's text, which tap reads its per-slide directives from (`layout`,
/// `skip`, `steps`, `transition`, `notes`, ...). This matches what
/// `directivePattern` in internal/parser/parser.go matches: optional
/// whitespace, `<!--`, the body, `-->`. A comment anywhere else, such as
/// `<!-- pause -->`, is not a directive comment.
public struct DirectiveComment: Equatable, Sendable {
    /// The whole comment's range in the slide text, from `<!--` to `-->`.
    public let range: NSRange
    /// The text between `<!--` and `-->`, byte for byte.
    public let body: String

    public init(range: NSRange, body: String) {
        self.range = range
        self.body = body
    }

    public static func leading(in slideText: String) -> DirectiveComment? {
        let text = slideText as NSString
        var start = 0
        while start < text.length, let scalar = Unicode.Scalar(text.character(at: start)), CharacterSet.whitespacesAndNewlines.contains(scalar) {
            start += 1
        }
        guard start + 4 <= text.length, text.substring(with: NSRange(location: start, length: 4)) == "<!--" else { return nil }
        let searchRange = NSRange(location: start + 4, length: text.length - start - 4)
        let close = text.range(of: "-->", range: searchRange)
        guard close.location != NSNotFound else { return nil }
        let body = text.substring(with: NSRange(location: start + 4, length: close.location - start - 4))
        return DirectiveComment(range: NSRange(location: start, length: NSMaxRange(close) - start), body: body)
    }

    /// The slide text with `key` set to `value` in its directive comment,
    /// or removed when `value` is nil. Only the one top-level `key:` line
    /// is added, replaced or removed; every other line of the comment stays
    /// byte for byte, so block scalars, nested maps and the notes' own
    /// lines are untouched. The key goes before a `notes:` line, because
    /// tap's mixed comment rule ends the notes text at the next directive
    /// line, and a line after `notes:` is never treated as the key. A
    /// single-line comment is written back in the multi-line form. A slide
    /// without a comment gets one and a blank line; a comment left empty
    /// is removed with the blank lines after it.
    public static func rewrite(slideText: String, setting key: String, to value: String?) -> String {
        let text = slideText as NSString
        let existing = leading(in: slideText)
        // Lines of the body between the "<!--" line and the "-->" line.
        var lines: [String]
        if let existing {
            if existing.body.contains("\n") {
                var parts = existing.body.components(separatedBy: "\n")
                // The first part is what follows "<!--" on its line and the last what precedes "-->"; both are blank in the multi-line form.
                if parts.first?.trimmingCharacters(in: .whitespaces).isEmpty == true { parts.removeFirst() }
                if parts.last?.trimmingCharacters(in: .whitespaces).isEmpty == true { parts.removeLast() }
                lines = parts
            } else {
                let single = existing.body.trimmingCharacters(in: .whitespaces)
                lines = single.isEmpty ? [] : [single]
            }
        } else {
            lines = []
        }
        let notesIndex = lines.firstIndex { $0.hasPrefix("notes:") }
        let topLevel = notesIndex.map { Array(lines[..<$0]) } ?? lines
        let keyIndex = topLevel.firstIndex { $0.hasPrefix(key + ":") }
        if let keyIndex { lines.remove(at: keyIndex) }
        if let value {
            // The old key's place, else before notes:, else the end.
            let position = keyIndex ?? lines.firstIndex { $0.hasPrefix("notes:") } ?? lines.count
            lines.insert("\(key): \(value)", at: position)
        }
        if lines.allSatisfy({ $0.trimmingCharacters(in: .whitespaces).isEmpty }) {
            guard let existing else { return slideText }
            var end = NSMaxRange(existing.range)
            while end < text.length, text.character(at: end) == 10 { end += 1 }
            return text.replacingCharacters(in: NSRange(location: existing.range.location, length: end - existing.range.location), with: "")
        }
        let comment = "<!--\n" + lines.joined(separator: "\n") + "\n-->"
        if let existing {
            return text.replacingCharacters(in: existing.range, with: comment)
        }
        return comment + "\n\n" + slideText
    }
}
