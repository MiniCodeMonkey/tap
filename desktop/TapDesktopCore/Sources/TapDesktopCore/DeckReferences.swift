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

    /// The aspect ratios a comment records, tap's deckedit.AIImageAspectRatios.
    static let aspectRatios: Set<String> = ["1:1", "16:9", "9:16", "4:3", "3:4"]

    /// The comment records the person's words, then the choices tap made the
    /// image with (`| aspect: 16:9 | match-theme`); `prompt` is the words
    /// alone. Only a trailing token that is exactly a choice is one, so
    /// "compare the two | aspect: wide" stays the person's words.
    static func words(in commentText: String) -> String {
        var parts = commentText.components(separatedBy: " | ")
        while parts.count > 1, let last = parts.last, isChoice(last) { parts.removeLast() }
        return parts.joined(separator: " | ")
    }

    private static func isChoice(_ token: String) -> Bool {
        token == "match-theme" || (token.hasPrefix("aspect: ") && aspectRatios.contains(String(token.dropFirst("aspect: ".count))))
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
/// ```component ./components/Name.jsx fence). A path always stays inside
/// slides/ or components/: it starts at a word boundary (so
/// `myslides/X.jsx` is not one), and no segment starts with a dot (so
/// `./slides/../../x.jsx` is not one either).
public enum ComponentLink {
    private static let pattern = try! NSRegularExpression(pattern: #"(?<![A-Za-z0-9_./-])(?:\./)?(?:slides|components)(?:/[A-Za-z0-9_-][A-Za-z0-9_.-]*)*/[A-Za-z0-9_-][A-Za-z0-9_.-]*\.(?:jsx|tsx)(?![A-Za-z0-9_])"#)

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
