import Foundation

/// What a slide's placeholder card says before its real thumbnail exists:
/// the slide's first heading, or its first line of text, read from the
/// Markdown. Nothing here asks tap, so a deck that has only just opened
/// already has a card for every slide.
public struct SlideCard: Equatable, Sendable {
    public let heading: String

    public init(heading: String) {
        self.heading = heading
    }

    /// The card for one slide's Markdown.
    public init(slideMarkdown: String) {
        self.init(heading: Self.heading(inSlideMarkdown: slideMarkdown))
    }

    /// One card per slide of a deck's Markdown, cut the way tap cuts it:
    /// the frontmatter is skipped, a line that is only `---` outside a code
    /// fence or a comment separates two slides, and a piece with nothing in
    /// it is no slide.
    public static func cards(inDeckMarkdown markdown: String) -> [SlideCard] {
        slideTexts(inDeckMarkdown: markdown).map { SlideCard(slideMarkdown: $0) }
    }

    /// The Markdown of each slide of a deck, in order.
    public static func slideTexts(inDeckMarkdown markdown: String) -> [String] {
        var lines = markdown.components(separatedBy: "\n")
        if lines.first?.trimmingCharacters(in: .whitespaces) == "---",
           let close = lines.dropFirst().firstIndex(where: { isSeparator($0) }) {
            lines.removeSubrange(0...close)
        }
        var slides: [String] = []
        var current: [String] = []
        var fence: String?
        var inComment = false
        func finish() {
            let text = current.joined(separator: "\n")
            if !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty { slides.append(text) }
            current = []
        }
        for line in lines {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if inComment {
                if trimmed.contains("-->") { inComment = false }
            } else if let open = fence {
                if trimmed.hasPrefix(open) { fence = nil }
            } else if trimmed.hasPrefix("```") || trimmed.hasPrefix("~~~") {
                fence = String(trimmed.prefix(3))
            } else if trimmed.hasPrefix("<!--"), !trimmed.contains("-->") {
                inComment = true
            } else if isSeparator(line) {
                finish()
                continue
            }
            current.append(line)
        }
        finish()
        return slides
    }

    private static func isSeparator(_ line: String) -> Bool {
        line.trimmingCharacters(in: .whitespaces) == "---"
    }

    /// The first heading of a slide, or its first line of text when it has
    /// no heading, with the Markdown marks taken off. Comments, directive
    /// lines (`::right`), code fences and blank lines are passed over. ""
    /// for a slide with nothing to say.
    public static func heading(inSlideMarkdown markdown: String) -> String {
        var firstText: String?
        var fence: String?
        var inComment = false
        for line in markdown.components(separatedBy: "\n") {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if inComment {
                if trimmed.contains("-->") { inComment = false }
                continue
            }
            if let open = fence {
                if trimmed.hasPrefix(open) { fence = nil }
                continue
            }
            if trimmed.isEmpty || trimmed.hasPrefix("::") { continue }
            if trimmed.hasPrefix("```") || trimmed.hasPrefix("~~~") {
                fence = String(trimmed.prefix(3))
                continue
            }
            if trimmed.hasPrefix("<!--") {
                if !trimmed.contains("-->") { inComment = true }
                continue
            }
            if let heading = headingText(of: trimmed) {
                let cleaned = plain(heading)
                if !cleaned.isEmpty { return cleaned }
                continue
            }
            if firstText == nil {
                let cleaned = plain(stripBlockMarker(trimmed))
                if !cleaned.isEmpty { firstText = cleaned }
            }
        }
        return firstText ?? ""
    }

    private static func headingText(of line: String) -> String? {
        let hashes = line.prefix { $0 == "#" }
        guard (1...6).contains(hashes.count) else { return nil }
        let rest = line.dropFirst(hashes.count)
        guard rest.first == " " || rest.first == "\t" else { return nil }
        var text = rest.trimmingCharacters(in: .whitespaces)
        while text.hasSuffix("#") { text.removeLast() }
        return text.trimmingCharacters(in: .whitespaces)
    }

    private static func stripBlockMarker(_ line: String) -> String {
        var text = Substring(line)
        while text.first == ">" { text = text.dropFirst().drop { $0 == " " } }
        for marker in ["- ", "* ", "+ "] where text.hasPrefix(marker) {
            return String(text.dropFirst(marker.count))
        }
        let digits = text.prefix { $0.isNumber }
        if !digits.isEmpty, text.dropFirst(digits.count).hasPrefix(". ") {
            return String(text.dropFirst(digits.count + 2))
        }
        return String(text)
    }

    /// Inline Markdown as plain text: a link is its label, and emphasis and
    /// code marks go.
    private static func plain(_ text: String) -> String {
        var result = text
        if let links = try? NSRegularExpression(pattern: "!?\\[([^\\]]*)\\]\\([^)]*\\)") {
            result = links.stringByReplacingMatches(in: result, range: NSRange(result.startIndex..., in: result), withTemplate: "$1")
        }
        result = result.replacingOccurrences(of: "`", with: "")
        if let emphasis = try? NSRegularExpression(pattern: "(\\*\\*|__|\\*|~~)") {
            result = emphasis.stringByReplacingMatches(in: result, range: NSRange(result.startIndex..., in: result), withTemplate: "")
        }
        return result.trimmingCharacters(in: .whitespaces)
    }
}
