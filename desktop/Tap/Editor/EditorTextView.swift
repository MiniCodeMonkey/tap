import AppKit

protocol EditorTextViewDelegate: AnyObject {
    func editorTextDidChange(_ editor: EditorTextView)
    func editor(_ editor: EditorTextView, currentSlideDidChange index: Int?)
    func editor(_ editor: EditorTextView, payloadForHeaderDragOfBoxAt index: Int) -> SlideDragPayload?
    func editor(_ editor: EditorTextView, dropSlides payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool
    func editor(_ editor: EditorTextView, contextMenuForBoxAt index: Int) -> NSMenu?
}

extension EditorTextViewDelegate {
    func editor(_ editor: EditorTextView, payloadForHeaderDragOfBoxAt index: Int) -> SlideDragPayload? { nil }
    func editor(_ editor: EditorTextView, dropSlides payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool { false }
    func editor(_ editor: EditorTextView, contextMenuForBoxAt index: Int) -> NSMenu? { nil }
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

    /// The rectangle of a box whose start or end is inside the viewport,
    /// in view coordinates; nil for a box that is entirely off screen. A
    /// box that starts above the viewport has no trustworthy top, so the
    /// rectangle is extended far above it, as drawing does.
    func boxRect(forBoxAt index: Int) -> NSRect? {
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
        let errorSpace = CGFloat(box.slide.errors.count) * Self.errorLineHeight
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
        return NSRect(x: left, y: top, width: right - left, height: bottom - top)
    }

    func headerRect(forBoxAt index: Int) -> NSRect? {
        boxRect(forBoxAt: index).map { NSRect(x: $0.minX, y: $0.minY, width: $0.width, height: Self.headerHeight) }
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
            guard let rect = boxRect(forBoxAt: index) else { continue }
            if point.y < rect.minY { return boxes[index].slide.number }
            if point.y <= rect.maxY {
                return point.y < rect.midY ? boxes[index].slide.number : (index + 1 < boxes.count ? boxes[index + 1].slide.number : nil)
            }
        }
        if let first = visibleBoxIndices().first, let rect = boxRect(forBoxAt: first), point.y < rect.minY { return boxes[first].slide.number }
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
            guard let boxRect = boxRect(forBoxAt: index) else { continue }
            if boxRect.intersects(rect) {
                let box = boxes[index]
                draw(header: BoxHeader(slide: box.slide), skipped: box.slide.skip, in: boxRect, isCurrent: index == currentBoxIndex)
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
        // A Control-click is a context menu click (Task 15's), and a
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
        let header = BoxHeader(slide: boxes[index].slide)
        let text = count > 1 ? "\(header.number) \(header.meta)  +\(count - 1)" : "\(header.number) \(header.meta)"
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

    /// A slide drop moves, from this deck or another; Option copies
    /// (decision 2). A drop inside the dragged block, in this deck, is
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
        guard let payload = slidePayload(sender) else { return super.draggingEntered(sender) }
        let point = convert(sender.draggingLocation, from: nil)
        updateDropIndicator(at: point, count: payload.slideNumbers.count)
        return slideDropOperation(payload: payload, at: point)
    }

    override func draggingUpdated(_ sender: NSDraggingInfo) -> NSDragOperation {
        guard let payload = slidePayload(sender) else { return super.draggingUpdated(sender) }
        let point = convert(sender.draggingLocation, from: nil)
        updateDropIndicator(at: point, count: payload.slideNumbers.count)
        return slideDropOperation(payload: payload, at: point)
    }

    override func draggingExited(_ sender: NSDraggingInfo?) {
        if sender.flatMap(slidePayload) != nil { clearDropIndicator() } else { super.draggingExited(sender) }
    }

    override func performDragOperation(_ sender: NSDraggingInfo) -> Bool {
        guard let payload = slidePayload(sender) else { return super.performDragOperation(sender) }
        let point = convert(sender.draggingLocation, from: nil)
        let beforeNumber = dropBoundary(at: point)
        return performSlideDrop(payload: payload, beforeNumber: beforeNumber, isMove: slideDropOperation(payload: payload, at: point) == .move)
    }

    override func draggingEnded(_ sender: NSDraggingInfo) {
        clearDropIndicator()
        super.draggingEnded(sender)
    }

    /// NSTextView, as a drag's own source, deletes the current text
    /// selection when a text drag it built itself ends as a move: the
    /// drop by convention takes the text out of its old place. A header
    /// drag is not that: it is built by hand in `headerDragStarter`, and
    /// the move it performs is `dropSlides`'s replacement of the slide's
    /// own range, never a deletion of whatever the editor happens to have
    /// selected right now. Skipping `super` here keeps that selection
    /// alone; a real text drag still gets NSTextView's own handling.
    override func draggingSession(_ session: NSDraggingSession, endedAt screenPoint: NSPoint, operation: NSDragOperation) {
        guard session.draggingPasteboard.data(forType: Self.slideType) != nil else {
            super.draggingSession(session, endedAt: screenPoint, operation: operation)
            return
        }
    }

    /// The drop itself, shared with `performDragOperation` so a test can
    /// drive it without a real drag.
    @discardableResult
    func performSlideDrop(payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool {
        clearDropIndicator()
        return editorDelegate?.editor(self, dropSlides: payload, beforeNumber: beforeNumber, isMove: isMove) ?? false
    }

    // MARK: Accessibility

    /// The text view's own children, plus one element per visible box and
    /// one for the drop indicator while a drag is over the editor.
    override func accessibilityChildren() -> [Any]? {
        var children = super.accessibilityChildren() ?? []
        for index in visibleBoxIndices() {
            guard let rect = boxRect(forBoxAt: index) else { continue }
            let element = NSAccessibilityElement.element(withRole: .group, frame: convertToScreen(rect), label: SlideAccessibility.label(for: boxes[index].slide), parent: self) as! NSAccessibilityElement
            element.setAccessibilityIdentifier("box-\(boxes[index].slide.number)")
            children.append(element)
        }
        // A drag is over the editor exactly while the count is set: both
        // update paths set it and clearDropIndicator zeroes it.
        if dropIndicatorCount > 0 {
            let y = dropIndicatorY(beforeNumber: dropIndicatorBeforeNumber) ?? 0
            let rect = NSRect(x: textContainerOrigin.x - Self.boxOutset, y: y - 4, width: bounds.width, height: 8)
            let element = NSAccessibilityElement.element(withRole: .splitter, frame: convertToScreen(rect),
                                                         label: SlideAccessibility.dropLabel(beforeNumber: dropIndicatorBeforeNumber, count: dropIndicatorCount),
                                                         parent: self) as! NSAccessibilityElement
            element.setAccessibilityIdentifier("drop-indicator")
            children.append(element)
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
        restyle(editedRange)
        if tracker.hiddenPrefixLength != layoutHiddenLength {
            // Layout cannot change while the text storage is processing an edit.
            DispatchQueue.main.async { [weak self] in self?.updateHiddenLayout() }
        }
    }
}
