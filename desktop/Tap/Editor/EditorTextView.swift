import AppKit

protocol EditorTextViewDelegate: AnyObject {
    func editorTextDidChange(_ editor: EditorTextView)
    func editor(_ editor: EditorTextView, currentSlideDidChange index: Int?)
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

    static let font = NSFont.monospacedSystemFont(ofSize: 13, weight: .regular)
    static let boldFont = NSFont.monospacedSystemFont(ofSize: 13, weight: .bold)
    static let smallFont = NSFont.monospacedSystemFont(ofSize: 9, weight: .regular)
    static let lineHeight: CGFloat = 21
    static let headerHeight: CGFloat = 28
    static let errorLineHeight: CGFloat = 18
    static let boxPaddingTop: CGFloat = 6
    static let boxPaddingBottom: CGFloat = 7
    static let horizontalInset: CGFloat = 44
    static let boxOutset: CGFloat = 14

    private enum Role: Hashable {
        case outsideText, separator, blankGap, boxMiddle, boxLast
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
        view.textContentStorage?.delegate = view
        return view
    }

    // MARK: Text and tap's answers

    func load(text: String) {
        tracker.reset()
        textStorage?.setAttributedString(NSAttributedString(string: text, attributes: typingAttributes))
        // Replacing the whole document otherwise leaves the caret at the end,
        // where the text system moved it to follow the insertion.
        setSelectedRange(NSRange(location: 0, length: 0))
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
        let oldErrors = Dictionary(boxes.map { ("\($0.range.location):\($0.range.length)", $0.slide.errors.count) }) { first, _ in first }
        guard var dirty = tracker.apply(list, sentText: sentText, sentGeneration: sentGeneration,
                                        currentLength: (string as NSString).length) else { return false }
        for box in boxes where oldErrors["\(box.range.location):\(box.range.length)"].map({ $0 != box.slide.errors.count }) ?? false {
            dirty.append(box.range)
        }
        if !dirty.isEmpty {
            textStorage?.beginEditing()
            dirty.forEach(restyle)
            textStorage?.endEditing()
        }
        updateHiddenLayout()
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

    func header(forBoxAt index: Int) -> BoxHeader {
        BoxHeader(slide: boxes[index].slide)
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

    private func updateHiddenLayout() {
        let hidden = tracker.hiddenPrefixLength
        guard hidden != layoutHiddenLength, let layoutManager = textLayoutManager else { return }
        layoutHiddenLength = hidden
        layoutManager.invalidateLayout(for: layoutManager.documentRange)
        layoutManager.textViewportLayoutController.layoutViewport()
        setSelectedRanges(selectedRanges, affinity: selectionAffinity, stillSelecting: false)
    }

    // MARK: Styling

    private func role(for paragraph: NSRange, line: String) -> Role {
        if let index = tracker.boxIndex(containing: paragraph.location) {
            let box = boxes[index]
            let errors = box.slide.errors.count
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
        for (index, paragraph) in paragraphs.enumerated() where paragraph.length > 0 {
            storage.setAttributes(attributes(style: styles[index], role: role(for: paragraph, line: lines[index])), range: paragraph)
        }
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
        drawBoxes(in: rect)
    }

    private func drawBoxes(in rect: NSRect) {
        guard let layoutManager = textLayoutManager,
              let contentManager = layoutManager.textContentManager,
              let viewport = layoutManager.textViewportLayoutController.viewportRange,
              !boxes.isEmpty else { return }
        let documentStart = contentManager.documentRange.location
        let viewportStart = contentManager.offset(from: documentStart, to: viewport.location)
        let viewportEnd = contentManager.offset(from: documentStart, to: viewport.endLocation)
        let origin = textContainerOrigin
        let left = origin.x - Self.boxOutset
        let right = bounds.width - origin.x + Self.boxOutset

        // The first box that ends inside or after the viewport.
        var low = 0
        var high = boxes.count
        while low < high {
            let middle = (low + high) / 2
            if boxes[middle].end < viewportStart { low = middle + 1 } else { high = middle }
        }

        var index = low
        while index < boxes.count, boxes[index].range.location <= viewportEnd {
            let box = boxes[index]
            let errorSpace = CGFloat(box.slide.errors.count) * Self.errorLineHeight
            // A box that starts above the viewport has no trustworthy frame there; extend it past the dirty rect.
            var top = rect.minY - 40
            var bottom = rect.maxY + 40
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
            let boxRect = NSRect(x: left, y: top, width: right - left, height: bottom - top)
            if boxRect.intersects(rect) {
                draw(header: BoxHeader(slide: box.slide), skipped: box.slide.skip, in: boxRect, isCurrent: index == currentBoxIndex)
            }
            index += 1
        }
    }

    private static let metaAttributes: [NSAttributedString.Key: Any] = [.font: NSFont.systemFont(ofSize: 11), .foregroundColor: NSColor.secondaryLabelColor]
    private static let errorAttributes: [NSAttributedString.Key: Any] = [.font: NSFont.systemFont(ofSize: 11, weight: .medium), .foregroundColor: EditorPalette.error]

    private func draw(header: BoxHeader, skipped: Bool, in rect: NSRect, isCurrent: Bool) {
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

        var badgeX = rect.maxX - 10
        for badge in header.badges.reversed() {
            let string = NSAttributedString(string: badge, attributes: [.font: NSFont.systemFont(ofSize: 10.5), .foregroundColor: NSColor.secondaryLabelColor])
            let size = string.size()
            badgeX -= size.width + 14
            let pill = NSRect(x: badgeX, y: rect.minY + 6, width: size.width + 14, height: 16)
            NSColor.labelColor.withAlphaComponent(0.06).setFill()
            NSBezierPath(roundedRect: pill, xRadius: 8, yRadius: 8).fill()
            string.draw(at: NSPoint(x: pill.minX + 7, y: pill.minY + 1))
            badgeX -= 6
        }
        NSAttributedString(string: header.meta, attributes: Self.metaAttributes)
            .draw(with: NSRect(x: x, y: baseline, width: max(0, badgeX - x - 8), height: 16), options: [.usesLineFragmentOrigin, .truncatesLastVisibleLine])

        for (line, message) in header.errors.enumerated() {
            let y = headerBottom + 4 + CGFloat(line) * Self.errorLineHeight
            NSAttributedString(string: message, attributes: Self.errorAttributes)
                .draw(with: NSRect(x: rect.minX + 12, y: y, width: rect.width - 24, height: Self.errorLineHeight),
                      options: [.usesLineFragmentOrigin, .truncatesLastVisibleLine])
        }
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
        restyle(editedRange)
        if tracker.hiddenPrefixLength != layoutHiddenLength {
            // Layout cannot change while the text storage is processing an edit.
            DispatchQueue.main.async { [weak self] in self?.updateHiddenLayout() }
        }
    }
}
