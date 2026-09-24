import Foundation

/// The buffer cut along tap's slide ranges: the text before slide 1
/// (frontmatter and blank lines), each slide's own text with the gap that
/// follows it (blank lines and the one "---" separator line), and the text
/// after the last slide. Joining the pieces back gives the buffer exactly.
/// Slide boundaries come from tap's boxes; this type never looks for one.
public struct SlideDocument: Equatable, Sendable {
    public struct Entry: Equatable, Sendable {
        /// tap's description of the slide. `number` is the slide's number
        /// before the edit, and 0 for a slide the edit created.
        public var slide: Slide
        public var text: String
        /// The text between this slide and the next; "" after the last.
        public var gapAfter: String

        public init(slide: Slide, text: String, gapAfter: String) {
            self.slide = slide
            self.text = text
            self.gapAfter = gapAfter
        }
    }

    /// The gap written between two slides that were not neighbours before.
    public static let separator = "\n\n---\n\n"

    public var prefix: String
    public var entries: [Entry]
    public var suffix: String

    public init(prefix: String, entries: [Entry], suffix: String) {
        self.prefix = prefix
        self.entries = entries
        self.suffix = suffix
    }

    /// Cuts `text` along `boxes`, tap's boxes for that text, in order.
    public init(text: String, boxes: [SlideBox]) {
        let nsText = text as NSString
        guard let first = boxes.first, let last = boxes.last else {
            prefix = text
            entries = []
            suffix = ""
            return
        }
        prefix = nsText.substring(to: min(first.range.location, nsText.length))
        var cut: [Entry] = []
        for (index, box) in boxes.enumerated() {
            let range = NSIntersectionRange(box.range, NSRange(location: 0, length: nsText.length))
            let nextStart = index + 1 < boxes.count ? boxes[index + 1].range.location : NSMaxRange(range)
            let gap = NSRange(location: NSMaxRange(range), length: max(0, min(nextStart, nsText.length) - NSMaxRange(range)))
            cut.append(Entry(slide: box.slide, text: nsText.substring(with: range), gapAfter: nsText.substring(with: gap)))
        }
        entries = cut
        suffix = nsText.substring(from: min(last.end, nsText.length))
    }

    /// The buffer these pieces make.
    public var text: String {
        prefix + entries.map { $0.text + $0.gapAfter }.joined() + suffix
    }

    /// The boxes of `text`: one per entry, numbered 1 to n, with the line
    /// numbers tap would report for the same text.
    public var boxes: [SlideBox] {
        var location = (prefix as NSString).length
        var line = 1 + Self.newlineCount(prefix)
        var boxes: [SlideBox] = []
        for (index, entry) in entries.enumerated() {
            let length = (entry.text as NSString).length
            let newlines = Self.newlineCount(entry.text)
            let old = entry.slide
            let slide = Slide(number: index + 1, startLine: line, endLine: line + newlines, layout: old.layout, title: old.title,
                              fragments: old.fragments, steps: old.steps, skip: old.skip, errors: old.errors, codeBlocks: old.codeBlocks)
            boxes.append(SlideBox(range: NSRange(location: location, length: length), slide: slide))
            location += length + (entry.gapAfter as NSString).length
            line += newlines + Self.newlineCount(entry.gapAfter)
        }
        return boxes
    }

    /// Gives every boundary between two slides that were not neighbours
    /// before the edit the canonical separator, keeps the gap between two
    /// that were, and leaves nothing after the last slide. Entries carry
    /// their old numbers, so old neighbours are the ones whose numbers are
    /// consecutive; a created slide, numbered 0, was nobody's neighbour.
    public mutating func normalizeGaps() {
        for index in entries.indices {
            guard index + 1 < entries.count else {
                entries[index].gapAfter = ""
                continue
            }
            let current = entries[index].slide.number
            let next = entries[index + 1].slide.number
            let wereNeighbours = current > 0 && next == current + 1
            if !wereNeighbours || !entries[index].gapAfter.contains("---") {
                entries[index].gapAfter = Self.separator
            }
        }
    }

    static func newlineCount(_ string: String) -> Int {
        string.utf16.reduce(0) { $0 + ($1 == 10 ? 1 : 0) }
    }
}
