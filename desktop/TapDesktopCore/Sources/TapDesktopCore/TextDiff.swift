import Foundation

public struct TextReplacement: Equatable, Sendable {
    public let range: NSRange
    public let replacement: String

    public init(range: NSRange, replacement: String) {
        self.range = range
        self.replacement = replacement
    }
}

/// The single replacement between two texts: everything between their
/// common prefix and their common suffix. Loading a changed file this way is
/// one undoable edit that leaves the caret alone when it is outside the change.
public enum TextDiff {
    public static func replacement(from old: String, to new: String) -> TextReplacement? {
        let oldUnits = Array(old.utf16)
        let newUnits = Array(new.utf16)
        if oldUnits == newUnits { return nil }
        let limit = min(oldUnits.count, newUnits.count)
        var prefix = 0
        while prefix < limit, oldUnits[prefix] == newUnits[prefix] { prefix += 1 }
        if prefix > 0, UTF16.isLeadSurrogate(oldUnits[prefix - 1]) { prefix -= 1 }
        var suffix = 0
        while suffix < limit - prefix,
              oldUnits[oldUnits.count - 1 - suffix] == newUnits[newUnits.count - 1 - suffix] {
            suffix += 1
        }
        if suffix > 0, UTF16.isTrailSurrogate(oldUnits[oldUnits.count - suffix]) { suffix -= 1 }
        let range = NSRange(location: prefix, length: oldUnits.count - prefix - suffix)
        let replacementUnits = Array(newUnits[prefix..<(newUnits.count - suffix)])
        return TextReplacement(range: range, replacement: String(decoding: replacementUnits, as: UTF16.self))
    }
}
