import Foundation

/// An AI-generated image in a deck's text: the `ai-prompt` comment and,
/// on the next line, the image it produced, the pair tap's
/// internal/deckedit writes and reads (its aiImagePattern). The app finds
/// them to name a Regenerate menu item; tap edits them.
public struct AIImageReference: Equatable, Sendable {
    public let prompt: String
    public let imagePath: String
    /// The two lines, from the comment's `<` to the link's `)`.
    public let range: NSRange

    private static let pattern = try! NSRegularExpression(pattern: #"<!--\s*ai-prompt:\s*(.+?)\s*-->\n[ \t]*!\[\]\(([^)]+)\)"#)

    /// The comment records the person's words, then the choices tap made the
    /// image with (`| aspect: 16:9 | match-theme`); `prompt` is the words alone.
    static func words(in commentText: String) -> String {
        var parts = commentText.components(separatedBy: " | ")
        while parts.count > 1, let last = parts.last, last == "match-theme" || last.hasPrefix("aspect: ") { parts.removeLast() }
        return parts.joined(separator: " | ")
    }

    public static func find(in text: String) -> [AIImageReference] {
        let whole = text as NSString
        return pattern.matches(in: text, range: NSRange(location: 0, length: whole.length)).map { match in
            AIImageReference(prompt: words(in: whole.substring(with: match.range(at: 1))), imagePath: whole.substring(with: match.range(at: 2)), range: match.range)
        }
    }

    /// The pairs that start inside `slideRange` (a slide's lines from tap).
    public static func find(in text: String, slideRange: NSRange) -> [AIImageReference] {
        find(in: text).filter { NSLocationInRange($0.range.location, slideRange) }
    }
}

/// A deck-supplied component's path in a line of the deck, as tap's
/// snippets write it (`layout: ./slides/Name.jsx`, a
/// ```component ./components/Name.jsx fence).
public enum ComponentLink {
    private static let pattern = try! NSRegularExpression(pattern: #"(?:\./)?(?:slides|components)/[A-Za-z0-9_./-]+\.(?:jsx|tsx)"#)

    /// The path under `column` in `line`, or nil when the column is not on one.
    public static func find(in line: String, at column: Int) -> String? {
        let whole = line as NSString
        guard column >= 0, column < whole.length else { return nil }
        for match in pattern.matches(in: line, range: NSRange(location: 0, length: whole.length)) where NSLocationInRange(column, match.range) {
            return whole.substring(with: match.range)
        }
        return nil
    }
}
