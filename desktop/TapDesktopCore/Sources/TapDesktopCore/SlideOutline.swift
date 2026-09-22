import Foundation

public struct OutlineEntry: Equatable, Sendable {
    public let number: Int
    public let title: String
    public let layout: String
}

/// The Go to Slide list: slide titles that contain the query, ignoring case
/// and diacritics, or the slide whose number is the query.
public enum SlideOutline {
    public static func entries(for slides: [Slide], matching query: String) -> [OutlineEntry] {
        let trimmed = query.trimmingCharacters(in: .whitespaces)
        return slides
            .filter { slide in
                trimmed.isEmpty
                    || slide.title.range(of: trimmed, options: [.caseInsensitive, .diacriticInsensitive]) != nil
                    || "\(slide.number)" == trimmed
            }
            .map { OutlineEntry(number: $0.number, title: $0.title, layout: $0.layout) }
    }
}
