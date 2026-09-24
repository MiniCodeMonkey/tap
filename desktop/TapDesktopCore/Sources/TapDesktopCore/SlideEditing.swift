import Foundation

/// A structural edit on the deck's slides. Numbers are the 1-based
/// numbers of the boxes the edit is applied to; `beforeNumber` names the
/// slide the moved or inserted slides go above, or the end when nil.
public enum SlideOperation: Equatable, Sendable {
    case move(numbers: [Int], beforeNumber: Int?)
    case duplicate(numbers: [Int])
    case delete(numbers: [Int])
    case insert(markdowns: [String], beforeNumber: Int?)
    case setSkip(numbers: [Int], skipped: Bool)
}

/// What an operation produces: the new text, the boxes of that text, the
/// new numbers of the slides the operation acted on, and where the caret
/// goes (inside the first of them).
public struct SlideEditResult: Equatable, Sendable {
    public let text: String
    public let boxes: [SlideBox]
    public let selectedNumbers: [Int]
    public let caret: Int
    /// For each slide of the new text, its number before the edit, or nil
    /// for a slide the edit created. The panel moves its images by this.
    public let sourceNumbers: [Int?]

    public init(text: String, boxes: [SlideBox], selectedNumbers: [Int], caret: Int, sourceNumbers: [Int?]) {
        self.text = text
        self.boxes = boxes
        self.selectedNumbers = selectedNumbers
        self.caret = caret
        self.sourceNumbers = sourceNumbers
    }
}

public enum SlideEditing {
    /// Applies `operation` to `text` cut along `boxes`. Returns nil when the
    /// operation is invalid (a number outside the deck, an empty selection,
    /// deleting every slide) or changes nothing, so the caller registers no
    /// undo step for it. `caretOffsetInSlide` is where the caret sat inside
    /// the first operated slide; it lands at the same offset afterwards.
    public static func apply(_ operation: SlideOperation, to text: String, boxes: [SlideBox], caretOffsetInSlide: Int = 0) -> SlideEditResult? {
        var document = SlideDocument(text: text, boxes: boxes)
        let count = document.entries.count
        func validated(_ numbers: [Int]) -> [Int]? {
            let sorted = Array(Set(numbers)).sorted()
            guard count > 0, !sorted.isEmpty, sorted.allSatisfy({ $0 >= 1 && $0 <= count }) else { return nil }
            return sorted
        }
        var selected: [Int] = []

        switch operation {
        case .move(let numbers, let beforeNumber):
            guard let numbers = validated(numbers) else { return nil }
            if let beforeNumber, !(1...count).contains(beforeNumber) || numbers.contains(beforeNumber) { return nil }
            let moving = numbers.map { document.entries[$0 - 1] }
            var remaining = document.entries.enumerated().filter { !numbers.contains($0.offset + 1) }.map(\.element)
            let insertionIndex = beforeNumber.flatMap { before in remaining.firstIndex { $0.slide.number == before } } ?? remaining.count
            remaining.insert(contentsOf: moving, at: insertionIndex)
            guard remaining.map(\.slide.number) != document.entries.map(\.slide.number) else { return nil }
            document.entries = remaining
            selected = Array(insertionIndex + 1 ... insertionIndex + moving.count)

        case .duplicate(let numbers):
            guard let numbers = validated(numbers) else { return nil }
            let copies = numbers.map { number -> SlideDocument.Entry in
                var copy = document.entries[number - 1]
                copy.slide = created(from: copy.slide)
                copy.gapAfter = SlideDocument.separator
                return copy
            }
            let insertionIndex = numbers[numbers.count - 1]
            document.entries.insert(contentsOf: copies, at: insertionIndex)
            selected = Array(insertionIndex + 1 ... insertionIndex + copies.count)

        case .delete(let numbers):
            guard let numbers = validated(numbers), numbers.count < count else { return nil }
            document.entries = document.entries.enumerated().filter { !numbers.contains($0.offset + 1) }.map(\.element)
            selected = [min(numbers[0], document.entries.count)]

        case .insert(let markdowns, let beforeNumber):
            guard !markdowns.isEmpty else { return nil }
            if let beforeNumber, beforeNumber < 1 || beforeNumber > count { return nil }
            let created = markdowns.map { markdown in
                SlideDocument.Entry(slide: Slide(number: 0, startLine: 0, endLine: 0, layout: "", title: ""),
                                    text: markdown.trimmingCharacters(in: .whitespacesAndNewlines),
                                    gapAfter: SlideDocument.separator)
            }
            let insertionIndex = beforeNumber.map { $0 - 1 } ?? count
            document.entries.insert(contentsOf: created, at: insertionIndex)
            if count == 0, !document.prefix.isEmpty, !document.prefix.hasSuffix("\n\n") {
                document.prefix += document.prefix.hasSuffix("\n") ? "\n" : "\n\n"
            }
            selected = Array(insertionIndex + 1 ... insertionIndex + created.count)

        case .setSkip(let numbers, let skipped):
            guard let numbers = validated(numbers) else { return nil }
            for number in numbers {
                let entry = document.entries[number - 1]
                document.entries[number - 1].text = DirectiveComment.rewrite(slideText: entry.text, setting: "skip", to: skipped ? "true" : nil)
            }
            selected = numbers
        }

        let sourceNumbers = document.entries.map { $0.slide.number > 0 ? Optional($0.slide.number) : nil }
        document.normalizeGaps()
        let newText = document.text
        guard newText != text else { return nil }
        let newBoxes = document.boxes
        let target = newBoxes[selected[0] - 1]
        let caret = target.range.location + min(max(0, caretOffsetInSlide), target.range.length)
        return SlideEditResult(text: newText, boxes: newBoxes, selectedNumbers: selected, caret: caret, sourceNumbers: sourceNumbers)
    }

    /// The undo menu's name for an operation.
    public static func actionName(for operation: SlideOperation) -> String {
        func plural(_ verb: String, _ count: Int) -> String { count == 1 ? "\(verb) Slide" : "\(verb) \(count) Slides" }
        switch operation {
        case .move(let numbers, _): return plural("Move", Set(numbers).count)
        case .duplicate(let numbers): return plural("Duplicate", Set(numbers).count)
        case .delete(let numbers): return plural("Delete", Set(numbers).count)
        case .insert(let markdowns, _): return markdowns.count == 1 ? "New Slide" : "Insert \(markdowns.count) Slides"
        case .setSkip(let numbers, let skipped): return plural(skipped ? "Skip" : "Unskip", Set(numbers).count)
        }
    }

    /// The range to select in a freshly inserted slide so typing replaces
    /// its first placeholder: the first content line after the directive
    /// comment and any slot marker, without its markdown prefix; inside a
    /// fence, the first code line. An empty slide gives an empty range at 0.
    public static func firstSlotRange(inSlideText slideText: String) -> NSRange {
        let text = slideText as NSString
        var location = 0
        if let comment = DirectiveComment.leading(in: slideText) {
            // The line that holds "-->" is the comment's, not content.
            location = NSMaxRange(text.lineRange(for: NSRange(location: NSMaxRange(comment.range), length: 0)))
        }
        var insideFence = false
        while location < text.length {
            let lineRange = text.lineRange(for: NSRange(location: location, length: 0))
            let line = text.substring(with: lineRange).trimmingCharacters(in: .newlines)
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            location = NSMaxRange(lineRange)
            if trimmed.hasPrefix("```") {
                insideFence.toggle()
                continue
            }
            if trimmed.isEmpty || (!insideFence && trimmed.hasPrefix("::")) { continue }
            let prefixLength = insideFence ? 0 : markdownPrefixLength(of: line)
            let contentLength = (line as NSString).length - prefixLength
            return NSRange(location: lineRange.location + prefixLength, length: contentLength)
        }
        return NSRange(location: 0, length: 0)
    }

    private static let markdownPrefix = try! NSRegularExpression(pattern: #"^(#{1,6} |> |- |\* |\d+\. |> "|")"#)

    static func markdownPrefixLength(of line: String) -> Int {
        let nsLine = line as NSString
        guard let match = markdownPrefix.firstMatch(in: line, range: NSRange(location: 0, length: nsLine.length)) else { return 0 }
        return match.range.length
    }

    /// A copy's metadata: the same slide, numbered 0 as a slide the edit created.
    private static func created(from slide: Slide) -> Slide {
        Slide(number: 0, startLine: 0, endLine: 0, layout: slide.layout, title: slide.title, fragments: slide.fragments,
              steps: slide.steps, skip: slide.skip, errors: slide.errors, codeBlocks: slide.codeBlocks)
    }
}
