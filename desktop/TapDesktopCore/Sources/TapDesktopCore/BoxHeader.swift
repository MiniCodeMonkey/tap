import Foundation

/// What a slide box's header shows: the number, the layout's name (a menu
/// in the editor), a meta line with the live code blocks, badges for the reveal count, a
/// skipped slide and each live-code driver, the error lines under the
/// header (the slide's own, then each live block's problem with its line),
/// and the one fix-it the app offers.
public struct BoxHeader: Equatable, Sendable {
    /// Declaring a block's driver, the one problem the app can fix. Offered
    /// when a live block has a problem and the frontmatter the caller holds
    /// does not declare its driver (or the caller holds none), unless the
    /// caller says the frontmatter is broken: tap then renders with its
    /// defaults, every live block reports its driver undeclared, and the
    /// fix is the frontmatter's own problem, not a declaration.
    public struct FixIt: Equatable, Sendable {
        public let driver: String
        public var title: String { "Allow \(driver) in This Deck" }
        public init(driver: String) { self.driver = driver }
    }

    public let number: String
    /// The layout tap renders the slide with, declared or detected; "default" when tap reports none.
    public let layout: String
    /// True when the slide renders as a component: tap then reports the
    /// component's source path in place of a layout name.
    public let layoutIsComponent: Bool
    /// The layout's name as a menu shows it; a component's file name.
    public let layoutName: String
    /// The live code blocks, such as "sql, live"; empty when there are none.
    public let meta: String
    public let badges: [String]
    public let errors: [String]
    public let fixIt: FixIt?

    public init(slide: Slide, declaredDrivers: [String]? = nil, frontmatterIsBroken: Bool = false) {
        number = "\(slide.number)"
        layout = slide.layout.isEmpty ? "default" : slide.layout
        layoutIsComponent = layout.contains("/") || layout.contains(".")
        layoutName = layoutIsComponent ? String(layout.split(separator: "/").last ?? Substring(layout)) : LayoutCatalog.displayName(layout)
        let liveBlocks = slide.codeBlocks.filter(\.live)
        meta = liveBlocks.map { "\($0.language.isEmpty ? "code" : $0.language), live" }.joined(separator: "; ")

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

        var errors = slide.errors
        var fixIt: FixIt?
        for block in slide.codeBlocks {
            guard let problem = block.problem, !problem.isEmpty else { continue }
            // tap's message on one line: the no-drivers form spans several.
            let oneLine = problem.components(separatedBy: .newlines).map { $0.trimmingCharacters(in: .whitespaces) }.filter { !$0.isEmpty }.joined(separator: " ")
            errors.append(block.line > 0 ? "Line \(block.line): \(oneLine)" : oneLine)
            if fixIt == nil, !frontmatterIsBroken, block.live, !block.driver.isEmpty, !(declaredDrivers?.contains(block.driver) ?? false) {
                fixIt = FixIt(driver: block.driver)
            }
        }
        self.errors = errors
        self.fixIt = fixIt
    }

    /// The number of forward presses the slide takes: its steps, then its fragments.
    public static func revealCount(for slide: Slide) -> Int {
        slide.steps + slide.fragments
    }
}
