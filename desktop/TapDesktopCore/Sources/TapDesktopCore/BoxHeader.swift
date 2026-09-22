import Foundation

/// What a slide box's header shows: the number, a meta line with the
/// layout, title and live code blocks, and badges for the reveal count, a
/// skipped slide and each live-code driver.
public struct BoxHeader: Equatable, Sendable {
    public let number: String
    public let meta: String
    public let badges: [String]
    public let errors: [String]

    public init(slide: Slide) {
        number = "\(slide.number)"
        var parts: [String] = []
        if !slide.layout.isEmpty { parts.append(slide.layout) }
        if !slide.title.isEmpty { parts.append(slide.title) }
        let liveBlocks = slide.codeBlocks.filter(\.live)
        if !liveBlocks.isEmpty {
            parts.append(liveBlocks.map { "\($0.language.isEmpty ? "code" : $0.language), live" }.joined(separator: "; "))
        }
        meta = parts.joined(separator: " · ")

        var badges: [String] = []
        let reveals = Self.revealCount(for: slide)
        if reveals > 0 { badges.append(reveals == 1 ? "1 step" : "\(reveals) steps") }
        if slide.skip { badges.append("skipped") }
        var drivers: [String] = []
        for block in liveBlocks where !block.driver.isEmpty && !drivers.contains(block.driver) {
            drivers.append(block.driver)
        }
        badges.append(contentsOf: drivers)
        self.badges = badges
        errors = slide.errors
    }

    /// The number of forward presses the slide takes: its steps, then its fragments.
    public static func revealCount(for slide: Slide) -> Int {
        slide.steps + slide.fragments
    }
}
