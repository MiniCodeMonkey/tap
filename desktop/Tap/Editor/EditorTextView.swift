import AppKit

protocol EditorTextViewDelegate: AnyObject {
    func editorTextDidChange(_ editor: EditorTextView)
    func editor(_ editor: EditorTextView, currentSlideDidChange index: Int?)
    func editor(_ editor: EditorTextView, payloadForHeaderDragOfBoxAt index: Int) -> SlideDragPayload?
    func editor(_ editor: EditorTextView, dropSlides payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool
    func editor(_ editor: EditorTextView, contextMenuForBoxAt index: Int) -> NSMenu?
    func editor(_ editor: EditorTextView, applyFixItForBoxAt index: Int)
    func editor(_ editor: EditorTextView, showLayoutPickerForBoxAt index: Int, anchor: NSRect)
    func editor(_ editor: EditorTextView, insertImages files: [URL])
    func editor(_ editor: EditorTextView, openComponentLinkAt characterIndex: Int) -> Bool
}

extension EditorTextViewDelegate {
    func editor(_ editor: EditorTextView, payloadForHeaderDragOfBoxAt index: Int) -> SlideDragPayload? { nil }
    func editor(_ editor: EditorTextView, dropSlides payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool { false }
    func editor(_ editor: EditorTextView, contextMenuForBoxAt index: Int) -> NSMenu? { nil }
    func editor(_ editor: EditorTextView, applyFixItForBoxAt index: Int) {}
    func editor(_ editor: EditorTextView, showLayoutPickerForBoxAt index: Int, anchor: NSRect) {}
    func editor(_ editor: EditorTextView, insertImages files: [URL]) {}
    func editor(_ editor: EditorTextView, openComponentLinkAt characterIndex: Int) -> Bool { false }
}

/// A TextKit 2 text view that draws a rounded box behind each slide's lines.
///
/// The ranges come from tap's answers. Between answers the view shifts the
/// last known ranges by each edit, so boxes follow the text without waiting
/// for tap. The frontmatter, the text before slide 1, is hidden: layout skips
/// it, selections are clamped out of it, and no user edit may start in it.
final class EditorTextView: NSTextView {
    weak var editorDelegate: EditorTextViewDelegate?
    private(set) var tracker = SlideRangeTracker()
    private(set) var currentBoxIndex: Int?
    private var layoutHiddenLength = 0
    private var isApplyingProgrammaticEdit = false

    var boxes: [SlideBox] { tracker.boxes }
    var deckErrors: [String] { tracker.deckErrors }
    var hiddenLength: Int { tracker.hiddenPrefixLength }

    static var font: NSFont { EditorTypography.current.font }
    static var boldFont: NSFont { EditorTypography.current.boldFont }
    static var smallFont: NSFont { EditorTypography.current.smallFont }
    static var lineHeight: CGFloat { EditorTypography.current.lineHeight }
    static let headerHeight: CGFloat = 28
    static let errorLineHeight: CGFloat = 18
    static let boxPaddingTop: CGFloat = 6
    static let boxPaddingBottom: CGFloat = 7
    static let horizontalInset: CGFloat = 44
    static let boxOutset: CGFloat = 14

    private enum Role: Hashable {
        case outsideText, separator, blankGap, boxMiddle, boxLast
        /// A line of the frontmatter shown in the Deck card's Text mode, and its closing line.
        case frontmatterLine, frontmatterLast
        case boxFirst(errors: Int)
        case boxOnly(errors: Int)
    }

    private static var paragraphStyles: [Role: NSParagraphStyle] = [:]

    private static func paragraphStyle(for role: Role) -> NSParagraphStyle {
        if let cached = paragraphStyles[role] { return cached }
        let style = NSMutableParagraphStyle()
        var height = lineHeight
        func spacingBefore(_ errors: Int) -> CGFloat { headerHeight + boxPaddingTop + 4 + CGFloat(errors) * errorLineHeight }
        switch role {
        case .outsideText, .boxMiddle: break
        case .separator: height = 10
        case .blankGap: height = 4
        case .boxFirst(let errors): style.paragraphSpacingBefore = spacingBefore(errors)
        case .boxLast: style.paragraphSpacing = boxPaddingBottom + 4
        case .frontmatterLine: break
        case .frontmatterLast: style.paragraphSpacing = deckCardPadding + deckCardGap - firstBoxTopOffset
        case .boxOnly(let errors):
            style.paragraphSpacingBefore = spacingBefore(errors)
            style.paragraphSpacing = boxPaddingBottom + 4
        }
        style.minimumLineHeight = height
        style.maximumLineHeight = height
        paragraphStyles[role] = style
        return style
    }

    static func make() -> EditorTextView {
        let contentStorage = NSTextContentStorage()
        let layoutManager = NSTextLayoutManager()
        contentStorage.addTextLayoutManager(layoutManager)
        let container = NSTextContainer(size: NSSize(width: 0, height: CGFloat.greatestFiniteMagnitude))
        container.widthTracksTextView = true
        container.lineFragmentPadding = 0
        layoutManager.textContainer = container
        let view = EditorTextView(frame: .zero, textContainer: container)
        precondition(view.textLayoutManager != nil, "the editor needs TextKit 2")
        view.isRichText = false
        view.allowsUndo = true
        view.usesFindBar = true
        view.isIncrementalSearchingEnabled = true
        view.isAutomaticQuoteSubstitutionEnabled = false
        view.isAutomaticDashSubstitutionEnabled = false
        view.isAutomaticTextReplacementEnabled = false
        view.isContinuousSpellCheckingEnabled = false
        view.textContainerInset = NSSize(width: horizontalInset, height: 16)
        view.drawsBackground = true
        view.backgroundColor = EditorPalette.editorBackground
        view.font = font
        view.typingAttributes = [.font: font, .foregroundColor: NSColor.labelColor, .paragraphStyle: paragraphStyle(for: .boxMiddle)]
        view.isVerticallyResizable = true
        view.isHorizontallyResizable = false
        view.autoresizingMask = [.width]
        view.minSize = .zero
        view.maxSize = NSSize(width: CGFloat.greatestFiniteMagnitude, height: CGFloat.greatestFiniteMagnitude)
        view.setAccessibilityIdentifier("editor")
        view.textStorage?.delegate = view
        NotificationCenter.default.addObserver(view, selector: #selector(applyTypography), name: EditorTypography.didChangeNotification, object: nil)
        view.textContentStorage?.delegate = view
        view.registerForDraggedTypes(view.registeredDraggedTypes + [NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType)])
        return view
    }

    // MARK: Text and tap's answers

    func load(text: String) {
        tracker.reset()
        textStorage?.setAttributedString(NSAttributedString(string: text, attributes: typingAttributes))
        // Replacing the whole document otherwise leaves the caret at the end,
        // where the text system moved it to follow the insertion.
        setSelectedRange(NSRange(location: 0, length: 0))
        refreshDeclaredDrivers()
        restyle(NSRange(location: 0, length: (text as NSString).length))
        updateHiddenLayout()
    }

    /// Records that the text as of now goes to tap, and returns its generation.
    func beginSend() -> Int { tracker.beginSend() }

    /// Applies tap's slide list for the text sent as `sentGeneration`, and
    /// says whether it was applied. An answer to a send begun before the
    /// text was replaced is refused: it describes a document that is gone.
    @discardableResult
    func apply(_ list: SlideList, sentText: String, sentGeneration: Int) -> Bool {
        let oldErrors = Dictionary(boxes.map { ("\($0.range.location):\($0.range.length)", Self.errorLineCount(for: $0.slide)) }) { first, _ in first }
        guard var dirty = tracker.apply(list, sentText: sentText, sentGeneration: sentGeneration,
                                        currentLength: (string as NSString).length) else { return false }
        for box in boxes where oldErrors["\(box.range.location):\(box.range.length)"].map({ $0 != Self.errorLineCount(for: box.slide) }) ?? false {
            dirty.append(box.range)
        }
        if !dirty.isEmpty {
            textStorage?.beginEditing()
            dirty.forEach(restyle)
            textStorage?.endEditing()
        }
        updateHiddenLayout()
        refreshFrontmatterStyling()
        updateCurrentBox()
        needsDisplay = true
        return true
    }

    /// Adopts boxes the app built by permuting tap's own ranges, right
    /// after a slide operation changed the text, so the boxes and the
    /// sidebar show the new order without waiting for tap's next answer.
    func adoptBoxes(_ newBoxes: [SlideBox]) {
        tracker.adopt(newBoxes)
        textStorage?.beginEditing()
        restyle(NSRange(location: 0, length: (string as NSString).length))
        textStorage?.endEditing()
        updateHiddenLayout()
        updateCurrentBox()
        needsDisplay = true
    }

    /// The error lines a box makes room for: the slide's own and its blocks' problems.
    static func errorLineCount(for slide: Slide) -> Int {
        BoxHeader(slide: slide).errors.count
    }

    /// The drivers the frontmatter declares, read once per change of the
    /// text rather than on every draw. The text storage's delegate
    /// refreshes it after every character edit, so typing, `load`,
    /// `replaceText`, undo and redo all keep it current. It decides only
    /// whether a box offers its fix-it.
    private(set) var declaredDrivers: [String] = []

    private func refreshDeclaredDrivers() {
        let parsed = Frontmatter(text: string)
        frontmatterBlock = parsed
        declaredDrivers = parsed.declaredDrivers
    }

    /// The frontmatter as parsed at the last change of the text.
    private(set) var frontmatterBlock = Frontmatter(text: "")

    /// The header of a box as drawn, hit tested and offered in menus. While
    /// tap reports a problem with the frontmatter it offers no fix-it.
    func header(forBoxAt index: Int) -> BoxHeader {
        BoxHeader(slide: boxes[index].slide, declaredDrivers: declaredDrivers, frontmatterIsBroken: !deckErrors.isEmpty)
    }

    /// An edit the app makes, such as loading the disk version. It may change
    /// the hidden frontmatter, and it is one undo step named `actionName`.
    func replaceText(in range: NSRange, with replacement: String, actionName: String) {
        isApplyingProgrammaticEdit = true
        defer { isApplyingProgrammaticEdit = false }
        breakUndoCoalescing()
        guard shouldChangeText(in: range, replacementString: replacement) else { return }
        textStorage?.replaceCharacters(in: range, with: replacement)
        didChangeText()
        undoManager?.setActionName(actionName)
        breakUndoCoalescing()
    }

    // MARK: Keeping the frontmatter hidden

    override func setSelectedRanges(_ ranges: [NSValue], affinity: NSSelectionAffinity, stillSelecting: Bool) {
        let clamped = ranges.map { NSValue(range: SelectionClamp.clamp($0.rangeValue, hiddenLength: layoutHiddenLength)) }
        super.setSelectedRanges(clamped, affinity: affinity, stillSelecting: stillSelecting)
        updateCurrentBox()
    }

    override func shouldChangeText(in affectedCharRange: NSRange, replacementString: String?) -> Bool {
        shouldChangeText(inRanges: [NSValue(range: affectedCharRange)], replacementStrings: replacementString.map { [$0] })
    }

    override func shouldChangeText(inRanges affectedRanges: [NSValue], replacementStrings: [String]?) -> Bool {
        let hidden = layoutHiddenLength
        let isUndoing = undoManager?.isUndoing == true || undoManager?.isRedoing == true
        let touchesHidden = affectedRanges.contains { SelectionClamp.touchesHidden($0.rangeValue, hiddenLength: hidden) }
        guard touchesHidden, !isApplyingProgrammaticEdit, !isUndoing else {
            return super.shouldChangeText(inRanges: affectedRanges, replacementStrings: replacementStrings)
        }
        // Replace All can match hidden text too. Apply only the visible replacements.
        guard let strings = replacementStrings, strings.count == affectedRanges.count else {
            NSSound.beep()
            return false
        }
        let visible = zip(affectedRanges.map(\.rangeValue), strings).filter { !SelectionClamp.touchesHidden($0.0, hiddenLength: hidden) }
        guard !visible.isEmpty else {
            NSSound.beep()
            return false
        }
        if super.shouldChangeText(inRanges: visible.map { NSValue(range: $0.0) }, replacementStrings: visible.map(\.1)) {
            textStorage?.beginEditing()
            for (range, replacement) in visible.sorted(by: { $0.0.location > $1.0.location }) {
                textStorage?.replaceCharacters(in: range, with: replacement)
            }
            textStorage?.endEditing()
            didChangeText()
        }
        return false
    }

    /// The length of the text layout skips: the frontmatter, unless the
    /// Deck card shows it as text.
    private var effectiveHiddenLength: Int {
        deckCardDisplay == .text ? 0 : tracker.hiddenPrefixLength
    }

    /// Lays every visible fragment out again at the container's current origin.
    private func relayoutText() {
        guard let layoutManager = textLayoutManager else { return }
        layoutManager.invalidateLayout(for: layoutManager.documentRange)
        layoutManager.textViewportLayoutController.layoutViewport()
    }

    private func updateHiddenLayout() {
        let hidden = effectiveHiddenLength
        guard hidden != layoutHiddenLength, let layoutManager = textLayoutManager else { return }
        layoutHiddenLength = hidden
        layoutManager.invalidateLayout(for: layoutManager.documentRange)
        layoutManager.textViewportLayoutController.layoutViewport()
        setSelectedRanges(selectedRanges, affinity: selectionAffinity, stillSelecting: false)
    }

    // MARK: The Deck card

    /// What the Deck card shows under its header.
    enum DeckCardDisplay: Equatable {
        case collapsed
        /// The schema's form, in a body under the header.
        case form
        /// The frontmatter's own lines, in the editor's text, under the header.
        case text
    }

    /// How a problem tints the card: nothing, an amber warning, a red error.
    enum DeckCardTint: Equatable {
        case none, warning, error
    }

    static let deckCardTop: CGFloat = 16
    static let deckCardHeaderHeight: CGFloat = 40
    static let deckCardGap: CGFloat = 12
    static let deckCardPadding: CGFloat = 8
    /// A box's rectangle starts this far below the top of the text
    /// container's inset: the paragraph spacing before its first line, less
    /// the header and padding drawn above the line.
    static let firstBoxTopOffset: CGFloat = 4

    private(set) var deckCardDisplay: DeckCardDisplay = .collapsed
    private(set) var deckCardBodyHeight: CGFloat = 0
    private(set) var deckCardTint: DeckCardTint = .none
    private(set) var deckCardView: NSView?
    /// The 0-based lines of the deck's text that a problem names: the failing
    /// line of frontmatter that does not parse, in red, and the lines of
    /// settings with a problem, tinted red or amber. Drawn in Text mode.
    private(set) var deckCardMarkedLines: [(line: Int, isError: Bool)] = []
    /// Runs when the card's width changes, so its form can lay itself out in one column or two.
    var onDeckCardWidthChange: (() -> Void)?
    private var lastDeckCardWidth: CGFloat = 0
    private var styledFrontmatterLength = 0

    /// Puts the card's view on the editor, above the text it stands over.
    func installDeckCard(_ card: NSView) {
        deckCardView?.removeFromSuperview()
        deckCardView = card
        // A width to start from: the card's own constraints have none to satisfy at zero.
        card.frame = NSRect(x: Self.horizontalInset - Self.boxOutset, y: Self.deckCardTop, width: 600, height: deckCardHeight)
        addSubview(card)
        layoutDeckCard()
    }

    /// Sets what the card shows. The text starts below the card, or in it
    /// in Text mode, and the frontmatter's lines come into view or leave it.
    func setDeckCard(display: DeckCardDisplay, bodyHeight: CGFloat, tint: DeckCardTint, markedLines: [(line: Int, isError: Bool)]) {
        deckCardDisplay = display
        deckCardBodyHeight = display == .form ? bodyHeight : 0
        deckCardTint = tint
        deckCardMarkedLines = markedLines
        let top = Self.deckCardTop + (display == .text ? Self.deckCardHeaderHeight + 6 : deckCardHeight + Self.deckCardGap - Self.firstBoxTopOffset)
        if textContainerInset.height != top {
            textContainerInset = NSSize(width: Self.horizontalInset, height: top)
            // The fragments already laid out keep the origin they were placed at: lay them out again from the new one.
            relayoutText()
        }
        updateHiddenLayout()
        refreshFrontmatterStyling()
        layoutDeckCard()
        needsDisplay = true
    }

    /// The header and, in form mode, the body.
    var deckCardHeight: CGFloat { Self.deckCardHeaderHeight + deckCardBodyHeight }

    /// The card's left and right edges, level with the boxes'.
    private var deckCardHorizontalRange: (left: CGFloat, width: CGFloat) {
        let left = Self.horizontalInset - Self.boxOutset
        return (left, max(0, bounds.width - 2 * left))
    }

    /// Where the card's view sits: over the header, and over the body in form mode.
    func layoutDeckCard() {
        guard let deckCardView, bounds.width > 200 else { return }
        let horizontal = deckCardHorizontalRange
        let frame = NSRect(x: horizontal.left, y: Self.deckCardTop, width: horizontal.width, height: deckCardHeight)
        if deckCardView.frame != frame { deckCardView.frame = frame }
        if abs(horizontal.width - lastDeckCardWidth) > 0.5 {
            lastDeckCardWidth = horizontal.width
            onDeckCardWidthChange?()
        }
    }

    override func setFrameSize(_ newSize: NSSize) {
        super.setFrameSize(newSize)
        layoutDeckCard()
    }

    override func layout() {
        super.layout()
        layoutDeckCard()
    }

    /// The card's surface in view coordinates. In Text mode it reaches down
    /// to the frontmatter's closing line.
    func deckCardRect() -> NSRect {
        let horizontal = deckCardHorizontalRange
        var height = deckCardHeight
        if deckCardDisplay == .text, let bottom = frontmatterBottom() { height = max(height, bottom + Self.deckCardPadding - Self.deckCardTop) }
        return NSRect(x: horizontal.left, y: Self.deckCardTop, width: horizontal.width, height: height)
    }

    /// The lines of the frontmatter shown as text: none unless the card is in Text mode.
    /// With frontmatter tap reads, the layout's hidden text; with frontmatter it does not, the block itself.
    var frontmatterTextRegion: NSRange? {
        guard deckCardDisplay == .text else { return nil }
        if deckErrors.isEmpty {
            let hidden = tracker.hiddenPrefixLength
            return hidden > 0 ? NSRange(location: 0, length: hidden) : nil
        }
        if let range = frontmatterBlock.range { return range }
        guard frontmatterBlock.isUnterminated else { return nil }
        // A frontmatter that never closes runs to the first blank line or heading.
        let text = string as NSString
        var location = 0
        var lineNumber = 0
        while location < text.length {
            let line = text.lineRange(for: NSRange(location: location, length: 0))
            let content = text.substring(with: line).trimmingCharacters(in: .whitespacesAndNewlines)
            if lineNumber > 0, content.isEmpty || content.hasPrefix("#") { break }
            location = NSMaxRange(line)
            lineNumber += 1
        }
        return NSRange(location: 0, length: location)
    }

    /// Restyles the frontmatter's lines when the region they are shown in
    /// changed: they take the text roles when shown, the outside roles when hidden again.
    private func refreshFrontmatterStyling(wrapsEditing: Bool = true) {
        let length = frontmatterTextRegion.map(NSMaxRange) ?? 0
        guard length != styledFrontmatterLength else { return }
        let reach = max(length, styledFrontmatterLength)
        styledFrontmatterLength = length
        // Called from the text storage's own end-of-edit callback, the attributes are set inside that edit.
        if wrapsEditing { textStorage?.beginEditing() }
        restyle(NSRange(location: 0, length: min(reach, (string as NSString).length)))
        if wrapsEditing { textStorage?.endEditing() }
    }

    /// The bottom, in view coordinates, of the frontmatter's closing line.
    private func frontmatterBottom() -> CGFloat? {
        guard let region = frontmatterTextRegion, region.length > 0 else { return nil }
        let closingEnd = frontmatterBlock.range.map(NSMaxRange) ?? NSMaxRange(region)
        return lineRect(atCharacter: max(0, min(closingEnd, region.length) - 1))?.maxY
    }

    /// The rectangle of the line fragment holding a character, across the
    /// card, in view coordinates; nil when layout has no such line.
    private func lineRect(atCharacter index: Int) -> NSRect? {
        guard let contentManager = textContentStorage, let layoutManager = textLayoutManager,
              let location = contentManager.location(contentManager.documentRange.location, offsetBy: index) else { return nil }
        if let through = NSTextRange(location: contentManager.documentRange.location, end: contentManager.location(location, offsetBy: 1) ?? location) {
            layoutManager.ensureLayout(for: through)
        }
        guard let fragment = layoutManager.textLayoutFragment(for: location) else { return nil }
        let offset = contentManager.offset(from: fragment.rangeInElement.location, to: location)
        let lines = fragment.textLineFragments
        guard let line = lines.first(where: { NSLocationInRange(offset, $0.characterRange) }) ?? lines.last else { return nil }
        let horizontal = deckCardHorizontalRange
        return NSRect(x: horizontal.left, y: fragment.layoutFragmentFrame.minY + line.typographicBounds.minY + textContainerOrigin.y,
                      width: horizontal.width, height: line.typographicBounds.height)
    }

    /// The character index where a line of the deck's text starts.
    private func characterIndex(ofLine line: Int) -> Int? {
        let text = string as NSString
        var location = 0
        for _ in 0..<line {
            guard location < text.length else { return nil }
            location = NSMaxRange(text.lineRange(for: NSRange(location: location, length: 0)))
        }
        return location < text.length ? location : nil
    }

    /// Moves the caret to the start of the frontmatter, for Show Deck Settings in Text mode.
    func moveCursorToFrontmatter() {
        setSelectedRange(NSRange(location: 0, length: 0))
        scrollToVisible(NSRect(x: 0, y: 0, width: 1, height: deckCardRect().maxY))
    }

    private func drawDeckCard(in dirtyRect: NSRect) {
        let rect = deckCardRect()
        guard rect.intersects(dirtyRect) else { return }
        let path = NSBezierPath(roundedRect: rect, xRadius: 10, yRadius: 10)
        switch deckCardTint {
        case .none: EditorPalette.boxFill.setFill()
        case .warning: EditorPalette.warningTint.setFill()
        case .error: EditorPalette.errorTint.setFill()
        }
        path.fill()
        if deckCardDisplay == .text {
            for marked in deckCardMarkedLines {
                guard let index = characterIndex(ofLine: marked.line), let line = lineRect(atCharacter: index) else { continue }
                (marked.isError ? EditorPalette.error : EditorPalette.warning).withAlphaComponent(0.16).setFill()
                NSRect(x: line.minX + 1, y: line.minY, width: line.width - 2, height: line.height).fill()
            }
        }
        switch deckCardTint {
        case .none: EditorPalette.boxBorder.setStroke()
        case .warning: EditorPalette.warningTintBorder.setStroke()
        case .error: EditorPalette.errorTintBorder.setStroke()
        }
        path.lineWidth = 1
        path.stroke()
        if deckCardDisplay != .collapsed {
            EditorPalette.boxBorder.setFill()
            NSRect(x: rect.minX + 1, y: rect.minY + Self.deckCardHeaderHeight, width: rect.width - 2, height: 0.5).fill()
        }
    }

    // MARK: Styling

    private func role(for paragraph: NSRange, line: String, region frontmatterRegion: NSRange?) -> Role {
        if let region = frontmatterRegion, paragraph.location < NSMaxRange(region) {
            let closingEnd = frontmatterBlock.range.map(NSMaxRange) ?? NSMaxRange(region)
            if paragraph.location >= closingEnd { return .blankGap }
            return NSMaxRange(paragraph) >= closingEnd ? .frontmatterLast : .frontmatterLine
        }
        if let index = tracker.boxIndex(containing: paragraph.location) {
            let box = boxes[index]
            let errors = Self.errorLineCount(for: box.slide)
            let isFirst = paragraph.location == box.range.location
            let isLast = NSMaxRange(paragraph) >= box.end
            switch (isFirst, isLast) {
            case (true, true): return .boxOnly(errors: errors)
            case (true, false): return .boxFirst(errors: errors)
            case (false, true): return .boxLast
            default: return .boxMiddle
            }
        }
        let trimmed = line.trimmingCharacters(in: .whitespaces)
        if trimmed == "---" { return .separator }
        if trimmed.isEmpty { return .blankGap }
        return .outsideText
    }

    private func attributes(style: LineStyle, role: Role) -> [NSAttributedString.Key: Any] {
        var font = Self.font
        var color = NSColor.labelColor
        var obliqueness: Double = 0
        switch role {
        case .frontmatterLine, .frontmatterLast:
            break
        case .separator, .blankGap:
            font = Self.smallFont
            color = .tertiaryLabelColor
        default:
            switch style {
            case .heading: font = Self.boldFont
            case .fence: color = EditorPalette.fence
            case .directive, .slotMarker: color = EditorPalette.directive
            case .notes:
                color = EditorPalette.notes
                obliqueness = 0.12
            case .separator: color = .tertiaryLabelColor
            case .text, .code: break
            }
        }
        var attributes: [NSAttributedString.Key: Any] = [.font: font, .foregroundColor: color, .paragraphStyle: Self.paragraphStyle(for: role)]
        if obliqueness > 0 { attributes[.obliqueness] = obliqueness }
        return attributes
    }

    /// Sets paragraph roles and highlighting for every paragraph touching
    /// `range`, starting at the slide around it so fences are seen whole.
    private func restyle(_ range: NSRange) {
        guard let storage = textStorage else { return }
        let text = storage.string as NSString
        guard text.length > 0 else { return }
        var start = min(range.location, text.length)
        if let index = tracker.currentBoxIndex(caret: start) {
            start = min(start, boxes[index].range.location)
        } else {
            start = 0
        }
        let end = min(max(NSMaxRange(range), start), text.length)
        let whole = text.paragraphRange(for: NSRange(location: start, length: end - start))
        var paragraphs: [NSRange] = []
        var location = whole.location
        repeat {
            let paragraph = text.paragraphRange(for: NSRange(location: location, length: 0))
            paragraphs.append(paragraph)
            location = NSMaxRange(paragraph)
            if paragraph.length == 0 { break }
        } while location < NSMaxRange(whole)
        let lines = paragraphs.map { text.substring(with: $0).trimmingCharacters(in: .newlines) }
        let styles = Highlighter.styles(for: lines)
        let frontmatterRegion = frontmatterTextRegion
        for (index, paragraph) in paragraphs.enumerated() where paragraph.length > 0 {
            let paragraphRole = role(for: paragraph, line: lines[index], region: frontmatterRegion)
            var paragraphAttributes = attributes(style: styles[index], role: paragraphRole)
            if paragraphRole == .frontmatterLine || paragraphRole == .frontmatterLast {
                // YAML is not Markdown: a comment is not a heading. Only the delimiters are quiet.
                paragraphAttributes[.foregroundColor] = lines[index].trimmingCharacters(in: .whitespaces) == "---" ? NSColor.tertiaryLabelColor : NSColor.labelColor
            }
            storage.setAttributes(paragraphAttributes, range: paragraph)
        }
    }

    /// The settings' font size or line spacing changed: every paragraph is
    /// styled again with the new font and line height.
    @objc func applyTypography() {
        Self.paragraphStyles = [:]
        font = Self.font
        typingAttributes = [.font: Self.font, .foregroundColor: NSColor.labelColor, .paragraphStyle: Self.paragraphStyle(for: .boxMiddle)]
        restyle(NSRange(location: 0, length: (string as NSString).length))
        needsDisplay = true
    }

    // MARK: The current slide

    private func updateCurrentBox() {
        let index = tracker.currentBoxIndex(caret: selectedRange().location)
        guard index != currentBoxIndex else { return }
        currentBoxIndex = index
        needsDisplay = true
        editorDelegate?.editor(self, currentSlideDidChange: index)
    }

    /// Puts the cursor at the end of the slide's first heading, or at its
    /// start.
    ///
    /// The caret comes from the tracker, which resolves it against the same
    /// boxes the index names and gives that index back for it. Working the
    /// caret out here from the box and the text separately made the caret
    /// and the slide it is read back as two answers, and a render or an
    /// edit landing around the move left them naming different slides.
    func moveCursor(toSlide index: Int) {
        guard let caret = tracker.caret(forBoxAt: index, in: string as NSString) else { return }
        setSelectedRange(NSRange(location: caret, length: 0))
        scrollRangeToVisible(boxes[index].range)
    }

    override func didChangeText() {
        super.didChangeText()
        needsDisplay = true
        editorDelegate?.editorTextDidChange(self)
    }

    // MARK: Drawing

    override func drawBackground(in rect: NSRect) {
        super.drawBackground(in: rect)
        drawDeckCard(in: rect)
        drawBoxes(in: rect)
    }

    /// The rectangle of a box whose start or end is inside the viewport,
    /// in view coordinates, for drawing and hit testing, which read the
    /// laid-out viewport only and so stay cheap while typing; nil for a box that is entirely off screen. A
    /// box that starts above the viewport has no trustworthy top, so the
    /// rectangle is extended far above it, as drawing does.
    func onScreenBoxRect(forBoxAt index: Int) -> NSRect? {
        guard boxes.indices.contains(index), let layoutManager = textLayoutManager,
              let contentManager = layoutManager.textContentManager,
              let viewport = layoutManager.textViewportLayoutController.viewportRange else { return nil }
        let documentStart = contentManager.documentRange.location
        let viewportStart = contentManager.offset(from: documentStart, to: viewport.location)
        let viewportEnd = contentManager.offset(from: documentStart, to: viewport.endLocation)
        let box = boxes[index]
        guard box.end >= viewportStart, box.range.location <= viewportEnd else { return nil }
        let origin = textContainerOrigin
        let left = origin.x - Self.boxOutset
        let right = bounds.width - origin.x + Self.boxOutset
        let errorSpace = CGFloat(Self.errorLineCount(for: box.slide)) * Self.errorLineHeight
        var top = visibleRect.minY - 40
        var bottom = visibleRect.maxY + 40
        if box.range.location >= viewportStart,
           let location = contentManager.location(documentStart, offsetBy: box.range.location),
           let fragment = layoutManager.textLayoutFragment(for: location),
           let line = fragment.textLineFragments.first {
            top = fragment.layoutFragmentFrame.minY + line.typographicBounds.minY + origin.y - Self.headerHeight - Self.boxPaddingTop - errorSpace
        }
        if box.end <= viewportEnd,
           let location = contentManager.location(documentStart, offsetBy: max(box.range.location, box.end - 1)),
           let fragment = layoutManager.textLayoutFragment(for: location),
           let line = fragment.textLineFragments.last {
            bottom = fragment.layoutFragmentFrame.minY + line.typographicBounds.maxY + origin.y + Self.boxPaddingBottom
        }
        return NSRect(x: left, y: top, width: right - left, height: max(bottom, top) - top)
    }

    /// The rectangle of a box in view coordinates, from the layout of its
    /// own text. TextKit lays out only the viewport on its own, so the text
    /// up to the box's lines is laid out first, from the start of the
    /// document, which is what gives them a position. A box entirely above or below the
    /// visible area has its true rectangle; nil only when the text has no
    /// layout to measure.
    func boxRect(forBoxAt index: Int) -> NSRect? {
        guard boxes.indices.contains(index), let layoutManager = textLayoutManager,
              let contentManager = layoutManager.textContentManager else { return nil }
        let documentStart = contentManager.documentRange.location
        let box = boxes[index]
        let lastOffset = max(box.range.location, box.end - 1)
        guard let startLocation = contentManager.location(documentStart, offsetBy: box.range.location),
              let lastLocation = contentManager.location(documentStart, offsetBy: lastOffset) else { return nil }
        // Layout up to the end of the box's last paragraph, from the start of the document: a range that ends inside a paragraph leaves it where it was estimated.
        let through = layoutManager.textLayoutFragment(for: lastLocation)?.rangeInElement.endLocation ?? contentManager.documentRange.endLocation
        if let range = NSTextRange(location: documentStart, end: through) { layoutManager.ensureLayout(for: range) }
        // One pass over the box's own fragments, each laid out after the one before it, so the top and the bottom come from the same layout.
        var firstLine: NSTextLineFragment?
        var lastLine: NSTextLineFragment?
        var firstTop: CGFloat = 0
        var lastTop: CGFloat = 0
        layoutManager.enumerateTextLayoutFragments(from: startLocation, options: [.ensuresLayout]) { fragment in
            let fragmentTop = fragment.layoutFragmentFrame.minY + self.textContainerOrigin.y
            if firstLine == nil {
                firstLine = fragment.textLineFragments.first
                firstTop = fragmentTop
            }
            lastLine = fragment.textLineFragments.last
            lastTop = fragmentTop
            return contentManager.offset(from: documentStart, to: fragment.rangeInElement.endLocation) <= lastOffset
        }
        guard let firstLine, let lastLine else { return nil }
        let first = (minY: firstTop + firstLine.typographicBounds.minY, maxY: firstTop + firstLine.typographicBounds.maxY)
        let last = (minY: lastTop + lastLine.typographicBounds.minY, maxY: lastTop + lastLine.typographicBounds.maxY)
        let origin = textContainerOrigin
        let left = origin.x - Self.boxOutset
        let right = bounds.width - origin.x + Self.boxOutset
        let errorSpace = CGFloat(Self.errorLineCount(for: box.slide)) * Self.errorLineHeight
        let top = first.minY - Self.headerHeight - Self.boxPaddingTop - errorSpace
        let bottom = last.maxY + Self.boxPaddingBottom
        return NSRect(x: left, y: top, width: right - left, height: bottom - top)
    }

    func headerRect(forBoxAt index: Int) -> NSRect? {
        onScreenBoxRect(forBoxAt: index).map { NSRect(x: $0.minX, y: $0.minY, width: $0.width, height: Self.headerHeight) }
    }

    /// A point on the character itself, in the view's coordinates: a
    /// quarter of the way into the glyph and halfway down its line, so
    /// `characterIndexForInsertion(at:)` there is `index`. Through TextKit 2
    /// (never `layoutManager`, whose read would turn the editor into a
    /// TextKit 1 view). An edit restyles its whole slide, which leaves those
    /// paragraphs' fragments without lines until the next layout pass, so
    /// the text from the start through the character is laid out first: a
    /// fragment's frame is only true once everything above it is.
    func pointForCharacter(at index: Int) -> NSPoint {
        guard let contentManager = textContentStorage, let layoutManager = textLayoutManager,
              let location = contentManager.location(contentManager.documentRange.location, offsetBy: index) else { return .zero }
        let end = contentManager.location(location, offsetBy: 1) ?? location
        if let through = NSTextRange(location: contentManager.documentRange.location, end: end) {
            layoutManager.ensureLayout(for: through)
        }
        guard let fragment = layoutManager.textLayoutFragment(for: location) else { return .zero }
        let frame = fragment.layoutFragmentFrame
        let offset = contentManager.offset(from: fragment.rangeInElement.location, to: location)
        let lines = fragment.textLineFragments
        guard let line = lines.first(where: { NSLocationInRange(offset, $0.characterRange) }) ?? lines.last else { return .zero }
        let leading = line.locationForCharacter(at: offset).x
        let trailing = NSLocationInRange(offset + 1, line.characterRange) ? line.locationForCharacter(at: offset + 1).x : line.typographicBounds.width
        return NSPoint(x: frame.minX + line.typographicBounds.minX + leading + max(0, trailing - leading) / 4 + textContainerOrigin.x,
                       y: frame.minY + line.typographicBounds.midY + textContainerOrigin.y)
    }

    /// The box whose header is under `point`, in view coordinates.
    func boxIndex(forHeaderAt point: NSPoint) -> Int? {
        for index in visibleBoxIndices() where headerRect(forBoxAt: index)?.contains(point) == true {
            return index
        }
        return nil
    }

    override func menu(for event: NSEvent) -> NSMenu? {
        if let index = boxIndex(forHeaderAt: convert(event.locationInWindow, from: nil)) {
            return editorDelegate?.editor(self, contextMenuForBoxAt: index)
        }
        return super.menu(for: event)
    }

    /// The slide number a drop at `point` lands above: the box under the
    /// point when the point is in its upper half, the next one otherwise;
    /// 1 above every box; nil below the last.
    func dropBoundary(at point: NSPoint) -> Int? {
        guard !boxes.isEmpty else { return nil }
        for index in visibleBoxIndices() {
            guard let rect = onScreenBoxRect(forBoxAt: index) else { continue }
            if point.y < rect.minY { return boxes[index].slide.number }
            if point.y <= rect.maxY {
                return point.y < rect.midY ? boxes[index].slide.number : (index + 1 < boxes.count ? boxes[index + 1].slide.number : nil)
            }
        }
        if let first = visibleBoxIndices().first, let rect = onScreenBoxRect(forBoxAt: first), point.y < rect.minY { return boxes[first].slide.number }
        return nil
    }

    /// The indices of the boxes touching the viewport, in order. This is
    /// the same walk `drawBoxes` makes.
    private func visibleBoxIndices() -> [Int] {
        guard let layoutManager = textLayoutManager, let contentManager = layoutManager.textContentManager,
              let viewport = layoutManager.textViewportLayoutController.viewportRange, !boxes.isEmpty else { return [] }
        let documentStart = contentManager.documentRange.location
        let viewportStart = contentManager.offset(from: documentStart, to: viewport.location)
        let viewportEnd = contentManager.offset(from: documentStart, to: viewport.endLocation)
        var low = 0
        var high = boxes.count
        while low < high {
            let middle = (low + high) / 2
            if boxes[middle].end < viewportStart { low = middle + 1 } else { high = middle }
        }
        var indices: [Int] = []
        var index = low
        while index < boxes.count, boxes[index].range.location <= viewportEnd {
            indices.append(index)
            index += 1
        }
        return indices
    }

    private func drawBoxes(in rect: NSRect) {
        guard !boxes.isEmpty else { return }
        for index in visibleBoxIndices() {
            guard let boxRect = onScreenBoxRect(forBoxAt: index) else { continue }
            if boxRect.intersects(rect) {
                draw(header: header(forBoxAt: index), boxIndex: index, skipped: boxes[index].slide.skip, in: boxRect, isCurrent: index == currentBoxIndex)
            }
        }
        if let before = dropIndicatorBeforeNumber, let y = dropIndicatorY(beforeNumber: before) {
            let origin = textContainerOrigin
            let left = origin.x - Self.boxOutset
            let right = bounds.width - origin.x + Self.boxOutset
            NSColor.controlAccentColor.setFill()
            NSRect(x: left + 8, y: y - 1, width: right - left - 8, height: 2).fill()
            let ring = NSBezierPath(ovalIn: NSRect(x: left + 1, y: y - 4, width: 8, height: 8))
            NSColor.controlAccentColor.setStroke()
            ring.lineWidth = 2
            ring.stroke()
        }
    }

    /// The y of the boundary above slide `beforeNumber`: the top of that
    /// box's rectangle less half the gap, or below the last box for nil.
    private func dropIndicatorY(beforeNumber: Int?) -> CGFloat? {
        if let beforeNumber, let index = boxes.firstIndex(where: { $0.slide.number == beforeNumber }), let rect = boxRect(forBoxAt: index) {
            return rect.minY - 6
        }
        if beforeNumber == nil, let last = boxes.indices.last, let rect = boxRect(forBoxAt: last) {
            return rect.maxY + 6
        }
        return nil
    }

    private static let metaAttributes: [NSAttributedString.Key: Any] = [.font: NSFont.systemFont(ofSize: 11), .foregroundColor: NSColor.secondaryLabelColor]
    private static let errorAttributes: [NSAttributedString.Key: Any] = [.font: NSFont.systemFont(ofSize: 11, weight: .medium), .foregroundColor: EditorPalette.error]
    private static let badgeAttributes: [NSAttributedString.Key: Any] = [.font: NSFont.systemFont(ofSize: 10.5), .foregroundColor: NSColor.secondaryLabelColor]
    private static let fixItAttributes: [NSAttributedString.Key: Any] = [.font: NSFont.systemFont(ofSize: 10.5, weight: .semibold), .foregroundColor: EditorPalette.error]

    /// The badges' pills in the badges' order, laid out right to left from
    /// the header's right edge, and where they end on the left. Drawing and
    /// the fix-it's hit test both come here, so the pill left of them sits
    /// where it was drawn.
    static func badgeLayout(for badges: [String], headerMaxX: CGFloat, headerTop: CGFloat) -> (pills: [NSRect], leftEdge: CGFloat) {
        var badgeX = headerMaxX - 10
        var pills: [NSRect] = []
        for badge in badges.reversed() {
            let width = NSAttributedString(string: badge, attributes: badgeAttributes).size().width + 14
            badgeX -= width
            pills.insert(NSRect(x: badgeX, y: headerTop + 6, width: width, height: 16), at: 0)
            badgeX -= 6
        }
        return (pills, badgeX)
    }

    private static let layoutChipAttributes: [NSAttributedString.Key: Any] = [.font: NSFont.systemFont(ofSize: 11, weight: .semibold), .foregroundColor: NSColor.controlAccentColor]
    private static let layoutChipHoverAttributes: [NSAttributedString.Key: Any] = [.font: NSFont.systemFont(ofSize: 11, weight: .semibold), .foregroundColor: NSColor.labelColor]
    private static let layoutQuietAttributes: [NSAttributedString.Key: Any] = [.font: NSFont.systemFont(ofSize: 11), .foregroundColor: NSColor.secondaryLabelColor]

    /// A layout name, or a component's file name, with its chevron.
    static func layoutChipTitle(_ name: String) -> String { name + " \u{25BE}" }

    /// The layout pop-up in a header: right of the number, 18 points tall,
    /// as wide as the name and its chevron. It is sized for the chip's
    /// semibold text whether it is drawn as the chip or quietly, so the
    /// live segments after it never shift when the chip look comes and goes.
    /// Drawing and the click's hit test both come here.
    static func layoutChipRect(name: String, leftEdge: CGFloat, headerTop: CGFloat) -> NSRect {
        let width = NSAttributedString(string: layoutChipTitle(name), attributes: layoutChipAttributes).size().width + 14
        return NSRect(x: leftEdge, y: headerTop + 5, width: width, height: 18)
    }

    /// The layout pop-up of a box's header, in view coordinates; nil for a box off screen.
    func layoutChipRect(forBoxAt index: Int) -> NSRect? {
        guard boxes.indices.contains(index), let headerRect = headerRect(forBoxAt: index) else { return nil }
        let header = self.header(forBoxAt: index)
        let number = NSAttributedString(string: header.number, attributes: [.font: NSFont.monospacedDigitSystemFont(ofSize: 11, weight: .bold)])
        let chip = Self.layoutChipRect(name: header.layoutName, leftEdge: headerRect.minX + 12 + number.size().width + 7, headerTop: headerRect.minY)
        let badgesLeftEdge = Self.badgeLayout(for: header.badges, headerMaxX: headerRect.maxX, headerTop: headerRect.minY).leftEdge
        let limit = (fixItRect(forBoxAt: index)?.minX ?? badgesLeftEdge) - 8
        return NSRect(x: chip.minX, y: chip.minY, width: min(chip.width, max(0, limit - chip.minX)), height: chip.height)
    }

    /// The fix-it pill: left of the badges, 18 points tall, as wide as its
    /// title. Drawing and the hit test both come here, so a click lands
    /// where the pill was drawn.
    static func fixItRect(title: String, badgesLeftEdge: CGFloat, headerTop: CGFloat) -> NSRect {
        let width = NSAttributedString(string: title, attributes: fixItAttributes).size().width + 16
        return NSRect(x: badgesLeftEdge - width, y: headerTop + 5, width: width, height: 18)
    }

    /// The fix-it pill of a box's header, in view coordinates; nil for a box with none, or off screen.
    func fixItRect(forBoxAt index: Int) -> NSRect? {
        guard boxes.indices.contains(index), let headerRect = headerRect(forBoxAt: index) else { return nil }
        let header = self.header(forBoxAt: index)
        guard let fixIt = header.fixIt else { return nil }
        let badgesLeftEdge = Self.badgeLayout(for: header.badges, headerMaxX: headerRect.maxX, headerTop: headerRect.minY).leftEdge
        return Self.fixItRect(title: fixIt.title, badgesLeftEdge: badgesLeftEdge, headerTop: headerRect.minY)
    }

    private func draw(header: BoxHeader, boxIndex: Int, skipped: Bool, in rect: NSRect, isCurrent: Bool) {
        let hasErrors = !header.errors.isEmpty
        let path = NSBezierPath(roundedRect: rect, xRadius: 10, yRadius: 10)
        if isCurrent {
            NSColor.controlAccentColor.withAlphaComponent(0.2).setStroke()
            let ring = NSBezierPath(roundedRect: rect.insetBy(dx: -2.5, dy: -2.5), xRadius: 12.5, yRadius: 12.5)
            ring.lineWidth = 3.5
            ring.stroke()
        }
        (skipped ? EditorPalette.boxFill.withAlphaComponent(0.5) : EditorPalette.boxFill).setFill()
        path.fill()
        (hasErrors ? EditorPalette.error : (isCurrent ? NSColor.controlAccentColor : EditorPalette.boxBorder)).setStroke()
        path.lineWidth = hasErrors || isCurrent ? 1.5 : 1
        path.stroke()

        let headerBottom = rect.minY + Self.headerHeight
        EditorPalette.boxBorder.setFill()
        NSRect(x: rect.minX + 1, y: headerBottom, width: rect.width - 2, height: 0.5).fill()

        let numberAttributes: [NSAttributedString.Key: Any] = [
            .font: NSFont.monospacedDigitSystemFont(ofSize: 11, weight: .bold),
            .foregroundColor: hasErrors ? EditorPalette.error : (isCurrent ? NSColor.controlAccentColor : NSColor.labelColor),
        ]
        var x = rect.minX + 12
        let baseline = rect.minY + 7
        let number = NSAttributedString(string: header.number, attributes: numberAttributes)
        number.draw(at: NSPoint(x: x, y: baseline))
        x += number.size().width + 7

        let badgeLayout = Self.badgeLayout(for: header.badges, headerMaxX: rect.maxX, headerTop: rect.minY)
        for (badge, pill) in zip(header.badges, badgeLayout.pills) {
            NSColor.labelColor.withAlphaComponent(0.06).setFill()
            NSBezierPath(roundedRect: pill, xRadius: 8, yRadius: 8).fill()
            NSAttributedString(string: badge, attributes: Self.badgeAttributes).draw(at: NSPoint(x: pill.minX + 7, y: pill.minY + 1))
        }
        var metaLimit = badgeLayout.leftEdge
        if let fixIt = header.fixIt {
            let pill = Self.fixItRect(title: fixIt.title, badgesLeftEdge: badgeLayout.leftEdge, headerTop: rect.minY)
            EditorPalette.boxFill.setFill()
            let path = NSBezierPath(roundedRect: pill, xRadius: 9, yRadius: 9)
            path.fill()
            EditorPalette.error.setStroke()
            path.lineWidth = 1
            path.stroke()
            NSAttributedString(string: fixIt.title, attributes: Self.fixItAttributes).draw(at: NSPoint(x: pill.minX + 8, y: pill.minY + 2))
            metaLimit = pill.minX
        }
        let chip = Self.layoutChipRect(name: header.layoutName, leftEdge: x, headerTop: rect.minY)
        let showsChip = isCurrent || hoveredLayoutBoxIndex == boxIndex
        let chipVisibleWidth = min(chip.width, max(0, metaLimit - chip.minX - 8))
        if chipVisibleWidth > 0 {
            let drawnChip = NSRect(x: chip.minX, y: chip.minY, width: chipVisibleWidth, height: chip.height)
            if showsChip {
                NSColor.controlAccentColor.withAlphaComponent(isCurrent ? 0.16 : 0.10).setFill()
                NSBezierPath(roundedRect: drawnChip, xRadius: 9, yRadius: 9).fill()
            }
            NSAttributedString(string: Self.layoutChipTitle(header.layoutName), attributes: showsChip && isCurrent ? Self.layoutChipAttributes : (showsChip ? Self.layoutChipHoverAttributes : Self.layoutQuietAttributes))
                .draw(with: NSRect(x: drawnChip.minX + 7, y: drawnChip.minY + 2, width: max(0, drawnChip.width - 10), height: 14), options: [.usesLineFragmentOrigin, .truncatesLastVisibleLine])
        }
        let metaX = chip.maxX + 6
        NSAttributedString(string: header.meta, attributes: Self.metaAttributes)
            .draw(with: NSRect(x: metaX, y: baseline, width: max(0, metaLimit - metaX - 8), height: 16), options: [.usesLineFragmentOrigin, .truncatesLastVisibleLine])

        for (line, message) in header.errors.enumerated() {
            let y = headerBottom + 4 + CGFloat(line) * Self.errorLineHeight
            NSAttributedString(string: message, attributes: Self.errorAttributes)
                .draw(with: NSRect(x: rect.minX + 12, y: y, width: rect.width - 24, height: Self.errorLineHeight),
                      options: [.usesLineFragmentOrigin, .truncatesLastVisibleLine])
        }
    }

    // MARK: Paste and drop of images

    /// What Paste reads, for images and text alike: the general pasteboard,
    /// unless a test replaces it so a run never touches the person's clipboard.
    var pasteboardForPaste: NSPasteboard = .general

    /// The file URLs on `pasteboard` whose extension tap image add accepts,
    /// in the pasteboard's order. Other files are not images to tap.
    static let imageExtensions: Set<String> = ["png", "jpg", "jpeg", "gif", "webp", "svg", "avif"]

    static func imageFileURLs(on pasteboard: NSPasteboard) -> [URL] {
        let urls = pasteboard.readObjects(forClasses: [NSURL.self], options: [.urlReadingFileURLsOnly: true]) as? [URL] ?? []
        return urls.filter { imageExtensions.contains($0.pathExtension.lowercased()) }
    }

    /// Whether Paste has something to read on `pasteboardForPaste`: an
    /// image file, image data (a screenshot, which carries no string), or
    /// anything NSTextView reads itself. NSTextView's own check reads only
    /// its text types, which would turn Edit > Paste and Command-V off for
    /// a screenshot.
    var canPaste: Bool {
        guard isEditable else { return false }
        return !Self.imageFileURLs(on: pasteboardForPaste).isEmpty
            || pasteboardForPaste.canReadObject(forClasses: [NSImage.self], options: nil)
            || pasteboardForPaste.availableType(from: readablePasteboardTypes) != nil
    }

    /// Validates Paste for menu items too: NSTextView's `validateMenuItem`
    /// asks `validateUserInterfaceItem` and returns its answer.
    override func validateUserInterfaceItem(_ item: NSValidatedUserInterfaceItem) -> Bool {
        if item.action == #selector(paste(_:)) { return canPaste }
        return super.validateUserInterfaceItem(item)
    }

    /// Paste: image files and image data go to tap image add through the
    /// delegate; everything else is NSTextView's own reading of the same
    /// pasteboard (`readSelection(from:)`, so a test's pasteboard is honoured
    /// on the text branch too).
    override func paste(_ sender: Any?) {
        let files = Self.imageFileURLs(on: pasteboardForPaste)
        if !files.isEmpty {
            editorDelegate?.editor(self, insertImages: files)
            return
        }
        if pasteboardForPaste.availableType(from: [.string]) == nil,
           let image = pasteboardForPaste.readObjects(forClasses: [NSImage.self], options: nil)?.first as? NSImage,
           let file = Self.writePastedImage(image) {
            editorDelegate?.editor(self, insertImages: [file])
            return
        }
        _ = readSelection(from: pasteboardForPaste)
    }

    /// Image data (a screenshot) as a PNG file for tap to copy: tap keeps
    /// the name, so a second paste becomes pasted-image-2.png.
    static func writePastedImage(_ image: NSImage) -> URL? {
        guard let tiff = image.tiffRepresentation, let bitmap = NSBitmapImageRep(data: tiff), let png = bitmap.representation(using: .png, properties: [:]) else { return nil }
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("tap-pasted-\(UUID().uuidString)")
        let file = folder.appendingPathComponent("pasted-image.png")
        do {
            try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
            try png.write(to: file)
        } catch {
            return nil
        }
        return file
    }

    // MARK: Layout pop-up

    /// The box whose layout pop-up the pointer is over, drawn as the chip.
    private(set) var hoveredLayoutBoxIndex: Int?
    private var layoutHoverTrackingArea: NSTrackingArea?

    /// Opens the layout gallery under the box's layout chip.
    func showLayoutPicker(forBoxAt index: Int) {
        guard let rect = layoutChipRect(forBoxAt: index) else { return }
        editorDelegate?.editor(self, showLayoutPickerForBoxAt: index, anchor: rect)
    }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let layoutHoverTrackingArea { removeTrackingArea(layoutHoverTrackingArea) }
        let area = NSTrackingArea(rect: .zero, options: [.mouseMoved, .mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        layoutHoverTrackingArea = area
    }

    /// Moves the hover to `index` (nil for none) and redraws the two headers it touches.
    func setHoveredLayoutBox(_ index: Int?) {
        guard index != hoveredLayoutBoxIndex else { return }
        for touched in [hoveredLayoutBoxIndex, index].compactMap({ $0 }) {
            if let rect = headerRect(forBoxAt: touched) { setNeedsDisplay(rect) }
        }
        hoveredLayoutBoxIndex = index
        window?.invalidateCursorRects(for: self)
    }

    /// The layout pop-ups show the arrow, as any pop-up button does, over the text's I-beam.
    override func resetCursorRects() {
        super.resetCursorRects()
        for index in visibleBoxIndices() {
            // AppKit asserts on an empty cursor rect, which a chip scrolled out of view produces.
            guard let chip = layoutChipRect(forBoxAt: index)?.intersection(visibleRect), !chip.isEmpty else { continue }
            addCursorRect(chip, cursor: .arrow)
        }
    }

    override func mouseMoved(with event: NSEvent) {
        super.mouseMoved(with: event)
        let point = convert(event.locationInWindow, from: nil)
        let index = boxIndex(forHeaderAt: point)
        let overChip = index.flatMap { layoutChipRect(forBoxAt: $0)?.contains(point) == true ? $0 : nil }
        setHoveredLayoutBox(overChip)
        if overChip != nil { NSCursor.arrow.set() }
    }

    override func mouseExited(with event: NSEvent) {
        super.mouseExited(with: event)
        setHoveredLayoutBox(nil)
    }

    // MARK: Dragging a header

    private(set) var dropIndicatorBeforeNumber: Int?
    private var dropIndicatorCount = 0
    private static let slideType = NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType)
    static let dragThreshold: CGFloat = 4
    var optionHeld: () -> Bool = { NSEvent.modifierFlags.contains(.option) }
    /// The document's file, so a drop can tell a move within this deck
    /// from one arriving from another. Set by the session controller in
    /// `init` and again on `deckMoved`, as `SlidePanelViewController.deckURL` is.
    var deckURL: URL?
    /// Starts the drag session. A seam: a test replaces it to see what a
    /// header drag carries without a real drag session.
    lazy var headerDragStarter: (SlideDragPayload, NSRect, NSEvent) -> Void = { [weak self] payload, rect, event in
        guard let self, let data = try? payload.data() else { return }
        let item = NSPasteboardItem()
        item.setData(data, forType: Self.slideType)
        let draggingItem = NSDraggingItem(pasteboardWriter: item)
        let index = self.boxes.firstIndex { $0.slide.number == payload.slideNumbers[0] } ?? 0
        draggingItem.setDraggingFrame(rect, contents: self.headerImage(forBoxAt: index, count: payload.slideNumbers.count))
        self.beginDraggingSession(with: [draggingItem], event: event, source: self)
    }

    /// A mouse down on a box header: a drag past the threshold drags the
    /// slide (or the selection it belongs to); a mouse up before that is a
    /// click, handled as any click in the text.
    override func mouseDown(with event: NSEvent) {
        let point = convert(event.locationInWindow, from: nil)
        // A Cmd-click on a component's path opens the file; anywhere else it is a plain click.
        if event.modifierFlags.contains(.command), !event.modifierFlags.contains(.control) {
            let index = characterIndexForInsertion(at: point)
            if index < (string as NSString).length, editorDelegate?.editor(self, openComponentLinkAt: index) == true { return }
        }
        // A click on the layout chip opens the gallery; a Control-click is a context menu click.
        if !event.modifierFlags.contains(.control), isEditable, let index = boxIndex(forHeaderAt: point), layoutChipRect(forBoxAt: index)?.contains(point) == true {
            showLayoutPicker(forBoxAt: index)
            return
        }
        // A Control-click on the pill is a context menu click, as anywhere on the header.
        if !event.modifierFlags.contains(.control), let index = boxIndex(forHeaderAt: point), let pill = fixItRect(forBoxAt: index), pill.contains(point) {
            editorDelegate?.editor(self, applyFixItForBoxAt: index)
            return
        }
        // A Control-click is a context menu click, and a
        // Shift-click extends the selection; neither starts a header drag.
        guard !event.modifierFlags.contains(.control), !event.modifierFlags.contains(.shift),
              let index = boxIndex(forHeaderAt: point), let window,
              let payload = editorDelegate?.editor(self, payloadForHeaderDragOfBoxAt: index) else {
            super.mouseDown(with: event)
            return
        }
        var outcome: NSEvent?
        window.trackEvents(matching: [.leftMouseDragged, .leftMouseUp], timeout: 1.5, mode: .eventTracking) { tracked, stop in
            guard let tracked else {
                stop.pointee = true
                return
            }
            if tracked.type == .leftMouseUp || hypot(tracked.locationInWindow.x - event.locationInWindow.x, tracked.locationInWindow.y - event.locationInWindow.y) >= Self.dragThreshold {
                outcome = tracked
                stop.pointee = true
            }
        }
        if let outcome, outcome.type == .leftMouseDragged {
            let rect = headerRect(forBoxAt: index) ?? NSRect(origin: point, size: NSSize(width: 200, height: Self.headerHeight))
            headerDragStarter(payload, rect, event)
            return
        }
        // A click: the caret goes where a click in the text would put it.
        let caret = characterIndexForInsertion(at: point)
        setSelectedRange(NSRange(location: min(caret, (string as NSString).length), length: 0))
    }

    /// The dragged picture: the header's number and title, with a count.
    private func headerImage(forBoxAt index: Int, count: Int) -> NSImage {
        let slide = boxes[index].slide
        let label = slide.title.isEmpty ? BoxHeader(slide: slide).layoutName : slide.title
        let text = count > 1 ? "\(slide.number) \(label)  +\(count - 1)" : "\(slide.number) \(label)"
        let attributes: [NSAttributedString.Key: Any] = [.font: NSFont.systemFont(ofSize: 12, weight: .semibold), .foregroundColor: NSColor.labelColor]
        let size = (text as NSString).size(withAttributes: attributes)
        return NSImage(size: NSSize(width: size.width + 24, height: Self.headerHeight), flipped: false) { rect in
            EditorPalette.boxFill.setFill()
            NSBezierPath(roundedRect: rect, xRadius: 8, yRadius: 8).fill()
            (text as NSString).draw(at: NSPoint(x: 12, y: (rect.height - size.height) / 2), withAttributes: attributes)
            return true
        }
    }

    override func draggingSession(_ session: NSDraggingSession, sourceOperationMaskFor context: NSDraggingContext) -> NSDragOperation {
        // Only a slide drag is this view's own; a text drag keeps NSTextView's answer.
        guard session.draggingPasteboard.data(forType: Self.slideType) != nil else {
            return super.draggingSession(session, sourceOperationMaskFor: context)
        }
        return [.move, .copy]
    }

    // MARK: Dropping slides

    private func slidePayload(_ sender: NSDraggingInfo) -> SlideDragPayload? {
        sender.draggingPasteboard.data(forType: Self.slideType).flatMap { SlideDragPayload(data: $0) }
    }

    func updateDropIndicator(at point: NSPoint, count: Int) {
        let boundary = dropBoundary(at: point)
        if boundary != dropIndicatorBeforeNumber || count != dropIndicatorCount {
            dropIndicatorBeforeNumber = boundary
            dropIndicatorCount = count
            needsDisplay = true
            NSAccessibility.post(element: self, notification: .announcementRequested,
                                 userInfo: [.announcement: SlideAccessibility.dropLabel(beforeNumber: boundary, count: count),
                                            .priority: NSAccessibilityPriorityLevel.medium.rawValue])
        }
    }

    func clearDropIndicator() {
        guard dropIndicatorBeforeNumber != nil || dropIndicatorCount != 0 else { return }
        dropIndicatorBeforeNumber = nil
        dropIndicatorCount = 0
        needsDisplay = true
    }

    /// A slide drop moves, from this deck or another; Option copies.
    /// A drop inside the dragged block, in this deck, is
    /// refused, as the sidebar refuses it: it would land the block back
    /// where it already is.
    private func slideDropOperation(payload: SlideDragPayload, at point: NSPoint) -> NSDragOperation {
        let sameDeck = deckURL.map { payload.comesFrom(deck: $0) } ?? false
        if sameDeck, let first = payload.slideNumbers.min(), let last = payload.slideNumbers.max() {
            let before = dropBoundary(at: point) ?? (boxes.count + 1)
            if before >= first, before <= last + 1 { return [] }
        }
        return optionHeld() ? .copy : .move
    }

    override func draggingEntered(_ sender: NSDraggingInfo) -> NSDragOperation {
        if !Self.imageFileURLs(on: sender.draggingPasteboard).isEmpty { return .copy }
        guard let payload = slidePayload(sender) else { return super.draggingEntered(sender) }
        let point = convert(sender.draggingLocation, from: nil)
        updateDropIndicator(at: point, count: payload.slideNumbers.count)
        return slideDropOperation(payload: payload, at: point)
    }

    override func draggingUpdated(_ sender: NSDraggingInfo) -> NSDragOperation {
        if !Self.imageFileURLs(on: sender.draggingPasteboard).isEmpty { return .copy }
        guard let payload = slidePayload(sender) else { return super.draggingUpdated(sender) }
        let point = convert(sender.draggingLocation, from: nil)
        updateDropIndicator(at: point, count: payload.slideNumbers.count)
        return slideDropOperation(payload: payload, at: point)
    }

    override func draggingExited(_ sender: NSDraggingInfo?) {
        if sender.flatMap(slidePayload) != nil { clearDropIndicator() } else { super.draggingExited(sender) }
    }

    /// NSTextView accepts a drop only when the pasteboard holds text it can
    /// read, and a slide payload holds none, so its answer would refuse
    /// every slide drop before `performDragOperation` ran. A slide drop is
    /// accepted here exactly when `draggingUpdated` offered an operation for it.
    override func prepareForDragOperation(_ sender: NSDraggingInfo) -> Bool {
        if !Self.imageFileURLs(on: sender.draggingPasteboard).isEmpty { return true }
        guard let payload = slidePayload(sender) else { return super.prepareForDragOperation(sender) }
        return slideDropOperation(payload: payload, at: convert(sender.draggingLocation, from: nil)) != []
    }

    /// A slide drop is finished by `performDragOperation`; NSTextView's
    /// conclusion is for the text drop it would have made.
    override func concludeDragOperation(_ sender: NSDraggingInfo?) {
        guard sender.flatMap(slidePayload) == nil else { return }
        super.concludeDragOperation(sender)
    }

    override func performDragOperation(_ sender: NSDraggingInfo) -> Bool {
        let images = Self.imageFileURLs(on: sender.draggingPasteboard)
        if !images.isEmpty {
            let point = convert(sender.draggingLocation, from: nil)
            setSelectedRange(NSRange(location: min(characterIndexForInsertion(at: point), (string as NSString).length), length: 0))
            editorDelegate?.editor(self, insertImages: images)
            return true
        }
        guard let payload = slidePayload(sender) else { return super.performDragOperation(sender) }
        let point = convert(sender.draggingLocation, from: nil)
        let beforeNumber = dropBoundary(at: point)
        return performSlideDrop(payload: payload, beforeNumber: beforeNumber, isMove: slideDropOperation(payload: payload, at: point) == .move)
    }

    override func draggingEnded(_ sender: NSDraggingInfo) {
        clearDropIndicator()
        super.draggingEnded(sender)
    }

    /// The drop itself, shared with `performDragOperation` so a test can
    /// drive it without a real drag.
    @discardableResult
    func performSlideDrop(payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool {
        clearDropIndicator()
        return editorDelegate?.editor(self, dropSlides: payload, beforeNumber: beforeNumber, isMove: isMove) ?? false
    }

    // MARK: Accessibility

    /// The box elements handed out by the latest `accessibilityChildren`,
    /// by slide number, and the drop indicator's. AppKit's accessibility
    /// does not retain the elements a view returns: an element nothing
    /// else holds is freed as soon as the call returns, and every later
    /// query of it, its role, frame, identifier or parent, fails. So the
    /// view holds each one, and hands out the same object for the same
    /// slide on the next call, until the box leaves the visible range.
    private var boxAccessibilityElements: [Int: NSAccessibilityElement] = [:]
    private var dropIndicatorAccessibilityElement: NSAccessibilityElement?
    private var layoutAccessibilityElements: [Int: LayoutPopUpAccessibilityElement] = [:]

    /// A layout pop-up's element: a pop-up button that opens its menu when pressed.
    private final class LayoutPopUpAccessibilityElement: NSAccessibilityElement {
        var onPress: () -> Void = {}
        override func accessibilityPerformPress() -> Bool {
            onPress()
            return true
        }
    }

    /// The text view's own children, plus one element per visible box and
    /// one for the drop indicator while a drag is over the editor.
    override func accessibilityChildren() -> [Any]? {
        var children = super.accessibilityChildren() ?? []
        var visibleElements: [Int: NSAccessibilityElement] = [:]
        var visiblePopUps: [Int: LayoutPopUpAccessibilityElement] = [:]
        for index in visibleBoxIndices() {
            guard let rect = onScreenBoxRect(forBoxAt: index) else { continue }
            let number = boxes[index].slide.number
            let element = boxAccessibilityElements[number] ?? {
                let element = NSAccessibilityElement()
                element.setAccessibilityRole(.group)
                element.setAccessibilityParent(self)
                element.setAccessibilityIdentifier("box-\(number)")
                return element
            }()
            element.setAccessibilityFrame(convertToScreen(rect))
            element.setAccessibilityLabel(SlideAccessibility.label(for: boxes[index].slide))
            visibleElements[number] = element
            children.append(element)
            if let chip = layoutChipRect(forBoxAt: index) {
                let popUp = layoutAccessibilityElements[number] ?? {
                    let popUp = LayoutPopUpAccessibilityElement()
                    popUp.setAccessibilityRole(.popUpButton)
                    popUp.setAccessibilityParent(self)
                    popUp.setAccessibilityIdentifier("layout-\(number)")
                    popUp.setAccessibilityLabel("Layout")
                    return popUp
                }()
                popUp.setAccessibilityFrame(convertToScreen(chip))
                popUp.setAccessibilityValue(header(forBoxAt: index).layoutName)
                popUp.onPress = { [weak self] in
                    guard let self, let box = self.boxes.firstIndex(where: { $0.slide.number == number }) else { return }
                    self.showLayoutPicker(forBoxAt: box)
                }
                visiblePopUps[number] = popUp
                children.append(popUp)
            }
        }
        boxAccessibilityElements = visibleElements
        layoutAccessibilityElements = visiblePopUps
        // A drag is over the editor exactly while the count is set: both
        // update paths set it and clearDropIndicator zeroes it.
        if dropIndicatorCount > 0 {
            let y = dropIndicatorY(beforeNumber: dropIndicatorBeforeNumber) ?? 0
            let rect = NSRect(x: textContainerOrigin.x - Self.boxOutset, y: y - 4, width: bounds.width, height: 8)
            let element = dropIndicatorAccessibilityElement ?? {
                let element = NSAccessibilityElement()
                element.setAccessibilityRole(.splitter)
                element.setAccessibilityParent(self)
                element.setAccessibilityIdentifier("drop-indicator")
                return element
            }()
            element.setAccessibilityFrame(convertToScreen(rect))
            element.setAccessibilityLabel(SlideAccessibility.dropLabel(beforeNumber: dropIndicatorBeforeNumber, count: dropIndicatorCount))
            dropIndicatorAccessibilityElement = element
            children.append(element)
        } else {
            dropIndicatorAccessibilityElement = nil
        }
        return children
    }

    private func convertToScreen(_ rect: NSRect) -> NSRect {
        guard let window else { return rect }
        return window.convertToScreen(convert(rect, to: nil))
    }
}

extension EditorTextView: NSTextContentStorageDelegate {
    func textContentManager(_ textContentManager: NSTextContentManager, shouldEnumerate textElement: NSTextElement,
                            options: NSTextContentManager.EnumerationOptions = []) -> Bool {
        guard layoutHiddenLength > 0, let range = textElement.elementRange else { return true }
        return textContentManager.offset(from: textContentManager.documentRange.location, to: range.location) >= layoutHiddenLength
    }
}

extension EditorTextView: NSTextStorageDelegate {
    func textStorage(_ textStorage: NSTextStorage, didProcessEditing editedMask: NSTextStorageEditActions,
                     range editedRange: NSRange, changeInLength delta: Int) {
        guard editedMask.contains(.editedCharacters) else { return }
        tracker.recordEdit(location: editedRange.location, oldLength: editedRange.length - delta, newLength: editedRange.length)
        refreshDeclaredDrivers()
        restyle(editedRange)
        refreshFrontmatterStyling(wrapsEditing: false)
        needsDisplay = true
        if deckCardDisplay == .text { DispatchQueue.main.async { [weak self] in self?.layoutDeckCard() } }
        if effectiveHiddenLength != layoutHiddenLength {
            // Layout cannot change while the text storage is processing an edit.
            DispatchQueue.main.async { [weak self] in self?.updateHiddenLayout() }
        }
    }
}
