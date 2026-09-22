import Foundation

public struct RevealPosition: Equatable, Sendable {
    public let step: Int
    public let fragment: Int

    public init(step: Int, fragment: Int) {
        self.step = step
        self.fragment = fragment
    }
}

/// A slide's reveal positions in the order the page steps through them:
/// steps first, then fragments. Position 0 reveals nothing, and position
/// `steps + fragments` reveals everything.
public enum RevealPositions {
    public static func position(_ index: Int, steps: Int, fragments: Int) -> RevealPosition {
        let clamped = min(max(index, 0), steps + fragments)
        if clamped <= steps { return RevealPosition(step: clamped, fragment: -1) }
        return RevealPosition(step: steps, fragment: clamped - steps - 1)
    }
}

/// Which slide and reveal position the preview shows. It follows the
/// cursor with every step revealed, stays on a pinned slide, and moves
/// through positions with the step controls. Each change returns the hub
/// `slide` message that shows it.
public struct PreviewNavigator: Equatable, Sendable {
    public private(set) var slideNumber: Int?
    public private(set) var positionIndex = 0
    public private(set) var pinnedSlideNumber: Int?
    private var steps = 0
    private var fragments = 0

    public init() {}

    public var isPinned: Bool { pinnedSlideNumber != nil }
    public var revealCount: Int { steps + fragments }

    public var message: SlideMessage? {
        guard let slideNumber else { return nil }
        let position = RevealPositions.position(positionIndex, steps: steps, fragments: fragments)
        return SlideMessage(slideIndex: slideNumber - 1, fragment: position.fragment, step: position.step)
    }

    public var stepLabel: String {
        if revealCount == 0 { return "No steps" }
        if positionIndex == revealCount { return "All steps shown, \(revealCount) of \(revealCount)" }
        return "Step \(positionIndex) of \(revealCount)"
    }

    public var statusLabel: String {
        guard let slideNumber else { return "" }
        return isPinned ? "Slide \(slideNumber), pinned" : "Slide \(slideNumber), follows the cursor"
    }

    public mutating func cursorMoved(to slide: Slide) -> SlideMessage? {
        guard !isPinned else { return nil }
        if slide.number == slideNumber, slide.steps == steps, slide.fragments == fragments { return nil }
        show(slide)
        return message
    }

    /// tap sent a new slide list. Refreshes the shown slide's counts.
    public mutating func slidesChanged(_ slides: [Slide]) -> SlideMessage? {
        guard let slideNumber, let slide = slides.first(where: { $0.number == slideNumber }) else { return nil }
        guard slide.steps != steps || slide.fragments != fragments else { return nil }
        let wasShowingEverything = positionIndex == revealCount
        steps = slide.steps
        fragments = slide.fragments
        positionIndex = wasShowingEverything ? revealCount : min(positionIndex, revealCount)
        return message
    }

    public mutating func stepForward() -> SlideMessage? {
        guard slideNumber != nil, positionIndex < revealCount else { return nil }
        positionIndex += 1
        return message
    }

    public mutating func stepBackward() -> SlideMessage? {
        guard slideNumber != nil, positionIndex > 0 else { return nil }
        positionIndex -= 1
        return message
    }

    public mutating func pin() {
        pinnedSlideNumber = slideNumber
    }

    public mutating func unpin(cursorSlide: Slide?) -> SlideMessage? {
        pinnedSlideNumber = nil
        guard let cursorSlide else { return nil }
        show(cursorSlide)
        return message
    }

    private mutating func show(_ slide: Slide) {
        slideNumber = slide.number
        steps = slide.steps
        fragments = slide.fragments
        positionIndex = revealCount
    }
}
