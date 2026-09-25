import Foundation

/// VoiceOver labels for boxes, thumbnails and drop indicators.
public enum SlideAccessibility {
    /// "Slide 3, default layout, What We Knew, 2 steps", with the title and
    /// the steps left out when the slide has none, and "skipped" at the end
    /// of a skipped slide.
    public static func label(for slide: Slide) -> String {
        var parts = ["Slide \(slide.number)"]
        if !slide.layout.isEmpty { parts.append("\(slide.layout) layout") }
        if !slide.title.isEmpty { parts.append(slide.title) }
        let reveals = BoxHeader.revealCount(for: slide)
        if reveals == 1 { parts.append("1 step") } else if reveals > 1 { parts.append("\(reveals) steps") }
        if slide.skip { parts.append("skipped") }
        return parts.joined(separator: ", ")
    }

    public static func dropLabel(beforeNumber: Int?, count: Int) -> String {
        let slides = count == 1 ? "1 slide" : "\(count) slides"
        if let beforeNumber { return "Drop \(slides) above slide \(beforeNumber)" }
        return "Drop \(slides) at the end"
    }
}
