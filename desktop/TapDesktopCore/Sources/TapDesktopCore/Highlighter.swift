import Foundation

public enum LineStyle: Equatable, Sendable {
    case text, heading, fence, code, directive, notes, slotMarker, separator
}

/// Gives each line of the editor a style: markdown headings and fences,
/// and tap's own syntax (directive comments, `<!-- pause -->`, `::slot`
/// markers, speaker notes). It styles text only; slide boundaries come from tap.
public enum Highlighter {
    public static func styles(for lines: [String]) -> [LineStyle] {
        var styles: [LineStyle] = []
        styles.reserveCapacity(lines.count)
        var fenceMarker: String?
        var inComment = false
        var inNotes = false

        for line in lines {
            let trimmed = line.trimmingCharacters(in: .whitespaces)
            if let marker = fenceMarker {
                if trimmed.hasPrefix(marker) {
                    fenceMarker = nil
                    styles.append(.fence)
                } else {
                    styles.append(.code)
                }
                continue
            }
            if inComment {
                let closes = trimmed.contains("-->")
                if !inNotes, trimmed.hasPrefix("notes:") { inNotes = true }
                if closes {
                    styles.append(inNotes && trimmed != "-->" ? .notes : .directive)
                    inComment = false
                    inNotes = false
                } else {
                    styles.append(inNotes ? .notes : .directive)
                }
                continue
            }
            if trimmed.hasPrefix("```") || trimmed.hasPrefix("~~~") {
                fenceMarker = String(trimmed.prefix(3))
                styles.append(.fence)
                continue
            }
            if trimmed.hasPrefix("<!--") {
                let body = trimmed.dropFirst(4).trimmingCharacters(in: .whitespaces)
                let isNotes = body.hasPrefix("notes:")
                if !trimmed.dropFirst(4).contains("-->") {
                    inComment = true
                    inNotes = isNotes
                }
                styles.append(isNotes ? .notes : .directive)
                continue
            }
            if trimmed == "---" {
                styles.append(.separator)
            } else if trimmed.hasPrefix("::"), trimmed.count > 2 {
                styles.append(.slotMarker)
            } else if isHeading(trimmed) {
                styles.append(.heading)
            } else {
                styles.append(.text)
            }
        }
        return styles
    }

    private static func isHeading(_ trimmed: String) -> Bool {
        let hashes = trimmed.prefix { $0 == "#" }.count
        guard (1...6).contains(hashes) else { return false }
        let rest = trimmed.dropFirst(hashes)
        return rest.isEmpty || rest.first == " "
    }
}
