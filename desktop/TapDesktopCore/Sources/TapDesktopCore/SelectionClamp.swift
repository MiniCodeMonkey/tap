import Foundation

/// The editor hides the frontmatter, the text before slide 1. Hidden text
/// still takes the caret, so every selection is kept out of it, and no edit
/// may start inside it.
public enum SelectionClamp {
    public static func clamp(_ range: NSRange, hiddenLength: Int) -> NSRange {
        guard hiddenLength > 0, range.location < hiddenLength else { return range }
        let end = NSMaxRange(range)
        if end <= hiddenLength { return NSRange(location: hiddenLength, length: 0) }
        return NSRange(location: hiddenLength, length: end - hiddenLength)
    }

    public static func touchesHidden(_ range: NSRange, hiddenLength: Int) -> Bool {
        hiddenLength > 0 && range.location < hiddenLength
    }
}
