import Foundation

/// One slide's box: a UTF-16 range from the start of its first line to the
/// end of its last line, and what tap said about the slide.
public struct SlideBox: Equatable, Sendable {
    public var range: NSRange
    public var slide: Slide
    public var end: Int { NSMaxRange(range) }

    public init(range: NSRange, slide: Slide) {
        self.range = range
        self.slide = slide
    }
}

/// Keeps the slide boxes in step with the text. tap's answers set the
/// boxes. Between answers, every edit shifts them, and edits made while a
/// PUT is in flight are replayed onto its answer, so boxes never jump.
public struct SlideRangeTracker: Sendable {
    private struct Edit: Sendable {
        let generation: Int
        let location: Int
        let oldLength: Int
        let newLength: Int
    }

    public private(set) var boxes: [SlideBox] = []
    public private(set) var deckErrors: [String] = []
    public private(set) var generation = 0
    /// The first generation whose answer still describes a document that
    /// exists. `reset` moves it past every send begun before the text was
    /// replaced, so those answers are refused instead of landing on text
    /// they were never computed from.
    private var earliestLiveGeneration = 0
    private var editLog: [Edit] = []

    public init() {}

    public mutating func reset() {
        boxes = []
        deckErrors = []
        editLog = []
        generation += 1
        earliestLiveGeneration = generation
    }

    /// Records that the text as of now goes to tap, and returns its generation.
    public mutating func beginSend() -> Int {
        generation += 1
        return generation
    }

    public mutating func recordEdit(location: Int, oldLength: Int, newLength: Int) {
        for index in boxes.indices {
            boxes[index].range = Self.shift(boxes[index].range, location: location, oldLength: oldLength, newLength: newLength)
        }
        boxes = Self.separated(boxes)
        editLog.append(Edit(generation: generation, location: location, oldLength: oldLength, newLength: newLength))
    }

    /// Applies tap's slide list for the text sent as `sentGeneration`, and
    /// returns the regions whose paragraph roles may have changed. It
    /// returns nil for an answer to a send begun before the text was
    /// replaced: the edits that would place its boxes are gone with the
    /// text they described, so the answer cannot be placed at all.
    public mutating func apply(_ list: SlideList, sentText: String, sentGeneration: Int, currentLength: Int) -> [NSRange]? {
        guard sentGeneration >= earliestLiveGeneration else { return nil }
        var newBoxes = Self.boxes(for: list.slides, in: sentText as NSString)
        for edit in editLog where edit.generation >= sentGeneration {
            for index in newBoxes.indices {
                newBoxes[index].range = Self.shift(newBoxes[index].range, location: edit.location,
                                                   oldLength: edit.oldLength, newLength: edit.newLength)
            }
        }
        newBoxes = Self.separated(newBoxes)
        editLog.removeAll { $0.generation < sentGeneration }
        let dirty = Self.changedRegions(old: boxes, new: newBoxes, length: currentLength)
        boxes = newBoxes
        deckErrors = list.errors
        return dirty
    }

    /// Replaces the boxes with ranges the app built from tap's own ranges,
    /// as a slide operation does when it permutes the slides it was given.
    /// The deck errors stay. Every answer to a send begun before this is
    /// refused, as after `reset`: such an answer describes the text the
    /// operation replaced, and replaying the operation's whole-region edit
    /// onto it would stretch and collapse the boxes. The operation sends
    /// the new text right after, and that answer is the next one applied.
    public mutating func adopt(_ newBoxes: [SlideBox]) {
        boxes = Self.separated(newBoxes)
        editLog = []
        generation += 1
        earliestLiveGeneration = generation
    }

    /// The length of the hidden text before slide 1, the frontmatter. It is 0
    /// while the deck has deck-level errors, so the frontmatter can be fixed.
    public var hiddenPrefixLength: Int {
        deckErrors.isEmpty ? (boxes.first?.range.location ?? 0) : 0
    }

    /// The caret for a slide's box: the end of the box's first heading
    /// line, or the box's start when it has none. It is read from the same
    /// boxes the index names, and `currentBoxIndex` gives that index back
    /// for it, so the caret a move computes and the slide the caret is
    /// attributed to are one answer rather than two that can disagree.
    public func caret(forBoxAt index: Int, in text: NSString) -> Int? {
        guard boxes.indices.contains(index) else { return nil }
        let box = boxes[index]
        let start = min(box.range.location, text.length)
        let scanned = NSRange(location: start, length: min(NSMaxRange(box.range), text.length) - start)
        var offset = 0
        for line in text.substring(with: scanned).components(separatedBy: "\n") {
            if line.hasPrefix("#") { return start + offset + (line as NSString).length }
            offset += (line as NSString).length + 1
        }
        return start
    }

    public func boxIndex(containing position: Int) -> Int? {
        var low = 0
        var high = boxes.count - 1
        while low <= high {
            let middle = (low + high) / 2
            let box = boxes[middle]
            if position < box.range.location {
                high = middle - 1
            } else if position > box.end {
                low = middle + 1
            } else {
                return middle
            }
        }
        return nil
    }

    /// The box around the caret, or else the last box that starts before it.
    public func currentBoxIndex(caret: Int) -> Int? {
        if let exact = boxIndex(containing: caret) { return exact }
        return boxes.lastIndex { $0.range.location <= caret }
    }

    /// Keeps the boxes a sequence in which every position belongs to at
    /// most one box. An edit spanning a slide boundary pulls the boxes on
    /// both sides onto the text that is left, so without this two boxes can
    /// share a position, and a shared position has no answer to which slide
    /// it belongs to: the search for it returns whichever box it reaches
    /// first. The text that survived the edit stays with the earlier box,
    /// and the later one starts after it.
    static func separated(_ boxes: [SlideBox]) -> [SlideBox] {
        var separated = boxes
        for index in separated.indices.dropFirst() {
            let previousEnd = separated[index - 1].end
            guard separated[index].range.location <= previousEnd else { continue }
            let start = previousEnd + 1
            separated[index].range = NSRange(location: start, length: max(0, separated[index].end - start))
        }
        return separated
    }

    /// Shifts a box by an edit so the box keeps covering the same text. Text
    /// typed at either edge of a box belongs to the box.
    public static func shift(_ range: NSRange, location: Int, oldLength: Int, newLength: Int) -> NSRange {
        let delta = newLength - oldLength
        let start = range.location
        let end = NSMaxRange(range)
        let editEnd = location + oldLength
        if location > end { return range }
        if editEnd < start || (editEnd == start && location < start) {
            return NSRange(location: start + delta, length: range.length)
        }
        let newStart = min(start, location)
        let newEnd = max(location + newLength, end + delta)
        return NSRange(location: newStart, length: max(0, newEnd - newStart))
    }

    /// Turns tap's 1-based, inclusive line ranges into UTF-16 ranges of `text`.
    public static func boxes(for slides: [Slide], in text: NSString) -> [SlideBox] {
        var lineStarts = [0]
        let length = text.length
        for index in 0..<length where text.character(at: index) == 10 {
            lineStarts.append(index + 1)
        }
        func lineEnd(_ line: Int) -> Int {
            guard line < lineStarts.count else { return length }
            return lineStarts[line] - 1
        }
        return slides.compactMap { slide in
            guard slide.startLine >= 1, slide.startLine <= lineStarts.count else { return nil }
            let start = lineStarts[slide.startLine - 1]
            let end = max(start, lineEnd(slide.endLine))
            return SlideBox(range: NSRange(location: start, length: end - start), slide: slide)
        }
    }

    /// The regions around every box whose range is in one list and not the other.
    static func changedRegions(old: [SlideBox], new: [SlideBox], length: Int) -> [NSRange] {
        struct Key: Hashable { let location: Int; let length: Int }
        let oldKeys = Set(old.map { Key(location: $0.range.location, length: $0.range.length) })
        let newKeys = Set(new.map { Key(location: $0.range.location, length: $0.range.length) })
        var dirty: [NSRange] = []
        for (list, others) in [(old, newKeys), (new, oldKeys)] {
            for (index, box) in list.enumerated() where !others.contains(Key(location: box.range.location, length: box.range.length)) {
                let start = min(index > 0 ? list[index - 1].end : 0, length)
                let end = min(index + 1 < list.count ? list[index + 1].range.location : length, length)
                dirty.append(NSRange(location: start, length: max(0, end - start)))
            }
        }
        return dirty
    }
}
