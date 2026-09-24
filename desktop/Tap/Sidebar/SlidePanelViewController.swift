import AppKit

protocol SlidePanelDelegate: AnyObject {
    /// A click on a thumbnail. `selection` is the panel's selection after the click.
    func slidePanel(_ panel: SlidePanelViewController, didClickSlide number: Int, selection: [Int])
    func slidePanelSelectionDidChange(_ panel: SlidePanelViewController)
    func slidePanel(_ panel: SlidePanelViewController, payloadForSlides numbers: [Int]) -> SlideDragPayload?
    func slidePanel(_ panel: SlidePanelViewController, acceptDrop payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool
    func slidePanelContextMenu(_ panel: SlidePanelViewController) -> NSMenu?
    func slidePanelDeleteSelection(_ panel: SlidePanelViewController)
    func slidePanelCopySelection(_ panel: SlidePanelViewController)
    func slidePanelPaste(_ panel: SlidePanelViewController)
}

/// The collection view, which remembers which item a click landed on, so
/// a Shift-click reports the slide clicked last rather than the range's end.
final class SlidePanelCollectionView: NSCollectionView {
    private(set) var clickedIndexPath: IndexPath?
    var onDelete: (() -> Void)?
    var onCopy: (() -> Void)?
    var onPaste: (() -> Void)?
    var contextMenuProvider: ((Int?) -> NSMenu?)?

    override func mouseDown(with event: NSEvent) {
        clickedIndexPath = indexPathForItem(at: convert(event.locationInWindow, from: nil))
        super.mouseDown(with: event)
    }

    override var acceptsFirstResponder: Bool { true }

    @objc func copy(_ sender: Any?) { onCopy?() }
    @objc func paste(_ sender: Any?) { onPaste?() }

    /// Delete (51) and Forward Delete (117) delete the selected slides. Only
    /// here, with the sidebar focused: in the editor these keys edit text.
    override func keyDown(with event: NSEvent) {
        if event.keyCode == 51 || event.keyCode == 117 {
            onDelete?()
        } else {
            super.keyDown(with: event)
        }
    }

    override func menu(for event: NSEvent) -> NSMenu? {
        let number = indexPathForItem(at: convert(event.locationInWindow, from: nil)).map { $0.item + 1 }
        return contextMenuProvider?(number)
    }
}

/// The thumbnails, in a virtualized collection view. The panel is linked
/// to the cursor both ways by its delegate: a click moves the cursor, and
/// a cursor move selects the thumbnail.
final class SlidePanelViewController: NSViewController, NSCollectionViewDataSource, NSCollectionViewDelegateFlowLayout {
    static let width: CGFloat = 200

    weak var delegate: SlidePanelDelegate?
    let collectionView = SlidePanelCollectionView()
    let scrollView = NSScrollView()
    let titleLabel = NSTextField(labelWithString: "Slides")
    /// Runs when the visible thumbnails change, so the renderer can reorder its queue.
    var onVisibleRangeChanged: (() -> Void)?
    private(set) var slides: [Slide] = []
    private var images: [Int: NSImage] = [:]
    private var updating: Set<Int> = []
    private var isSyncingSelection = false
    /// The slide last clicked without Shift: a Shift-click extends from it.
    private var selectionAnchor: Int?
    /// The document's file, so a drop can tell a move within this deck from
    /// one arriving from another. Set by the session controller in `init`
    /// and again on `deckMoved`.
    var deckURL: URL?
    /// Whether Option is held, for a drop into another deck: a test replaces this.
    var optionHeld: () -> Bool = { NSEvent.modifierFlags.contains(.option) }

    override func loadView() {
        let root = NSView()
        titleLabel.font = .systemFont(ofSize: 12, weight: .semibold)
        titleLabel.textColor = .secondaryLabelColor

        let layout = NSCollectionViewFlowLayout()
        layout.itemSize = NSSize(width: ThumbnailItem.numberWidth + ThumbnailItem.gap + ThumbnailItem.imageSize.width, height: ThumbnailItem.imageSize.height)
        layout.minimumLineSpacing = 10
        layout.sectionInset = NSEdgeInsets(top: 4, left: 6, bottom: 12, right: 12)
        collectionView.collectionViewLayout = layout
        collectionView.isSelectable = true
        collectionView.allowsMultipleSelection = true
        collectionView.allowsEmptySelection = true
        collectionView.backgroundColors = [.clear]
        collectionView.register(ThumbnailItem.self, forItemWithIdentifier: ThumbnailItem.identifier)
        collectionView.dataSource = self
        collectionView.delegate = self
        collectionView.setAccessibilityIdentifier("slide-panel")
        collectionView.setAccessibilityLabel("Slides")
        collectionView.registerForDraggedTypes([NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType)])
        collectionView.setDraggingSourceOperationMask([.move, .copy], forLocal: true)
        collectionView.setDraggingSourceOperationMask([.move, .copy], forLocal: false)
        collectionView.onDelete = { [weak self] in self.map { $0.delegate?.slidePanelDeleteSelection($0) } }
        collectionView.onCopy = { [weak self] in self.map { $0.delegate?.slidePanelCopySelection($0) } }
        collectionView.onPaste = { [weak self] in self.map { $0.delegate?.slidePanelPaste($0) } }
        collectionView.contextMenuProvider = { [weak self] number in number.flatMap { self?.contextMenu(forSlide: $0) } }
        scrollView.documentView = collectionView
        scrollView.drawsBackground = false
        scrollView.hasVerticalScroller = true
        scrollView.automaticallyAdjustsContentInsets = false
        scrollView.contentView.postsBoundsChangedNotifications = true
        NotificationCenter.default.addObserver(self, selector: #selector(boundsChanged(_:)), name: NSView.boundsDidChangeNotification, object: scrollView.contentView)

        for view in [titleLabel, scrollView] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(view)
        }
        NSLayoutConstraint.activate([
            titleLabel.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: 14),
            titleLabel.centerYAnchor.constraint(equalTo: root.topAnchor, constant: 20),
            scrollView.topAnchor.constraint(equalTo: root.topAnchor, constant: 40),
            scrollView.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            scrollView.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            scrollView.bottomAnchor.constraint(equalTo: root.bottomAnchor),
        ])
        view = root
    }

    deinit {
        NotificationCenter.default.removeObserver(self)
    }

    @objc private func boundsChanged(_ notification: Notification) {
        onVisibleRangeChanged?()
    }

    // MARK: Content

    /// Replaces the slides. The selection is kept by number where the
    /// numbers still exist.
    func setSlides(_ newSlides: [Slide]) {
        let previousSelection = selectedNumbers
        let countChanged = newSlides.count != slides.count
        slides = newSlides
        images = images.filter { $0.key <= newSlides.count }
        updating = updating.filter { $0 <= newSlides.count }
        if countChanged {
            collectionView.reloadData()
        } else {
            for (index, slide) in newSlides.enumerated() {
                (collectionView.item(at: IndexPath(item: index, section: 0)) as? ThumbnailItem)?
                    .configure(slide: slide, image: images[slide.number], isUpdating: updating.contains(slide.number))
            }
        }
        let kept = previousSelection.filter { $0 <= newSlides.count }
        if kept != selectedNumbers { select(numbers: kept, scroll: false) }
    }

    func setImage(_ image: NSImage?, forSlide number: Int) {
        images[number] = image
        updating.remove(number)
        refreshItem(forSlide: number)
    }

    func image(forSlide number: Int) -> NSImage? {
        images[number]
    }

    /// Moves the images with their slides after an operation: the new slide
    /// `index + 1` shows the image the old slide `sourceNumbers[index]` had,
    /// and a created slide has none until the renderer delivers one.
    func remapImages(sourceNumbers: [Int?]) {
        let old = images
        images = [:]
        for (index, source) in sourceNumbers.enumerated() {
            if let source, let image = old[source] { images[index + 1] = image }
        }
        for number in 1...max(1, slides.count) { refreshItem(forSlide: number) }
    }

    /// Marks the slides whose thumbnails are being rendered again. Their
    /// old images stay until the new ones replace them.
    func setUpdating(_ numbers: Set<Int>) {
        let changed = updating.symmetricDifference(numbers)
        updating = numbers
        changed.forEach(refreshItem(forSlide:))
    }

    private func refreshItem(forSlide number: Int) {
        guard number >= 1, number <= slides.count,
              let item = collectionView.item(at: IndexPath(item: number - 1, section: 0)) as? ThumbnailItem else { return }
        item.configure(slide: slides[number - 1], image: images[number], isUpdating: updating.contains(number))
    }

    func item(forSlide number: Int) -> ThumbnailItem? {
        guard number >= 1, number <= slides.count else { return nil }
        return collectionView.item(at: IndexPath(item: number - 1, section: 0)) as? ThumbnailItem
    }

    /// The context menu for a thumbnail. A right-click on a thumbnail
    /// outside the selection selects it alone first, as Finder does.
    func contextMenu(forSlide number: Int) -> NSMenu? {
        if !selectedNumbers.contains(number) {
            select(numbers: [number], scroll: false)
            delegate?.slidePanel(self, didClickSlide: number, selection: [number])
        }
        return delegate?.slidePanelContextMenu(self)
    }

    // MARK: Selection

    var selectedNumbers: [Int] {
        collectionView.selectionIndexPaths.map { $0.item + 1 }.sorted()
    }

    var visibleNumbers: [Int] {
        collectionView.indexPathsForVisibleItems().map { $0.item + 1 }.sorted()
    }

    /// Selects thumbnails without telling the delegate: the cursor moved,
    /// or an operation chose the slides it made.
    func select(numbers: [Int], scroll: Bool) {
        let paths = Set(numbers.filter { $0 >= 1 && $0 <= slides.count }.map { IndexPath(item: $0 - 1, section: 0) })
        guard collectionView.selectionIndexPaths != paths else { return }
        isSyncingSelection = true
        collectionView.selectionIndexPaths = paths
        isSyncingSelection = false
        if scroll, let first = paths.min() {
            collectionView.scrollToItems(at: [first], scrollPosition: .nearestHorizontalEdge)
        }
    }

    /// A click on a thumbnail, with or without Shift: the same path the
    /// mouse takes through `didSelectItemsAt`. With Shift, the selection
    /// extends from its anchor to the clicked slide; the cursor goes to the
    /// clicked slide either way.
    func click(slide number: Int, extendingSelection: Bool) {
        guard number >= 1, number <= slides.count else { return }
        var numbers = [number]
        if extendingSelection, let anchor = selectionAnchor ?? selectedNumbers.first {
            numbers = Array(min(anchor, number)...max(anchor, number))
        } else {
            selectionAnchor = number
        }
        select(numbers: numbers, scroll: false)
        delegate?.slidePanel(self, didClickSlide: number, selection: selectedNumbers)
    }

    // MARK: NSCollectionViewDataSource and delegate

    func collectionView(_ collectionView: NSCollectionView, numberOfItemsInSection section: Int) -> Int {
        slides.count
    }

    func collectionView(_ collectionView: NSCollectionView, itemForRepresentedObjectAt indexPath: IndexPath) -> NSCollectionViewItem {
        let item = collectionView.makeItem(withIdentifier: ThumbnailItem.identifier, for: indexPath) as! ThumbnailItem
        let slide = slides[indexPath.item]
        item.configure(slide: slide, image: images[slide.number], isUpdating: updating.contains(slide.number))
        return item
    }

    func collectionView(_ collectionView: NSCollectionView, didSelectItemsAt indexPaths: Set<IndexPath>) {
        guard !isSyncingSelection else { return }
        let clicked = self.collectionView.clickedIndexPath.map { $0.item + 1 } ?? (indexPaths.map { $0.item + 1 }.max() ?? 0)
        guard clicked > 0 else { return }
        if NSApp.currentEvent?.modifierFlags.contains(.shift) != true { selectionAnchor = clicked }
        delegate?.slidePanel(self, didClickSlide: clicked, selection: selectedNumbers)
    }

    func collectionView(_ collectionView: NSCollectionView, didDeselectItemsAt indexPaths: Set<IndexPath>) {
        guard !isSyncingSelection else { return }
        delegate?.slidePanelSelectionDidChange(self)
    }

    // MARK: Drag and drop

    /// Where a drop lands and what it does. `proposedIndex` is the gap
    /// before that item (`count` means after the last). A slide dropped
    /// onto its own place, inside the dragged block, is refused. A drop
    /// moves, within the deck and from another deck alike; with Option
    /// held it copies, as the Finder does.
    func dropDecision(proposedIndex: Int, payload: SlideDragPayload, deck: URL?, optionHeld: Bool) -> (beforeNumber: Int?, operation: NSDragOperation) {
        let beforeNumber: Int? = proposedIndex < slides.count ? proposedIndex + 1 : nil
        let sameDeck = deck.map { payload.comesFrom(deck: $0) } ?? false
        if sameDeck {
            let block = payload.slideNumbers.sorted()
            let before = beforeNumber ?? (slides.count + 1)
            if let first = block.first, let last = block.last, before >= first, before <= last + 1 {
                return (beforeNumber, [])
            }
            return (beforeNumber, .move)
        }
        return (beforeNumber, optionHeld ? .copy : .move)
    }

    func performDrop(payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool {
        delegate?.slidePanel(self, acceptDrop: payload, beforeNumber: beforeNumber, isMove: isMove) ?? false
    }

    private var lastAnnouncedDrop: Int??

    func collectionView(_ collectionView: NSCollectionView, canDragItemsAt indexPaths: Set<IndexPath>, with event: NSEvent) -> Bool {
        true
    }

    func collectionView(_ collectionView: NSCollectionView, pasteboardWriterForItemAt indexPath: IndexPath) -> NSPasteboardWriting? {
        // Every dragged item writes the whole selection's payload; the drop reads the first.
        let numbers = selectedNumbers.contains(indexPath.item + 1) ? selectedNumbers : [indexPath.item + 1]
        guard let payload = delegate?.slidePanel(self, payloadForSlides: numbers), let data = try? payload.data() else { return nil }
        let item = NSPasteboardItem()
        item.setData(data, forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType))
        return item
    }

    func collectionView(_ collectionView: NSCollectionView, draggingSession session: NSDraggingSession, willBeginAt screenPoint: NSPoint,
                        forItemsAt indexPaths: Set<IndexPath>) {
        let numbers = selectedNumbers.isEmpty ? indexPaths.map { $0.item + 1 } : selectedNumbers
        guard let first = numbers.min(), let image = images[first] else { return }
        let count = numbers.count
        session.enumerateDraggingItems(options: [], for: collectionView, classes: [NSPasteboardItem.self], searchOptions: [:]) { item, index, stop in
            item.draggingFrame = NSRect(origin: item.draggingFrame.origin, size: NSSize(width: 164, height: 92))
            item.imageComponentsProvider = {
                let picture = NSDraggingImageComponent(key: .icon)
                picture.contents = image
                picture.frame = NSRect(x: 0, y: 0, width: 164, height: 92)
                guard count > 1 else { return [picture] }
                let badge = NSDraggingImageComponent(key: .label)
                badge.contents = Self.countBadge("\(count)")
                badge.frame = NSRect(x: 164 - 13, y: 92 - 13, width: 22, height: 22)
                return [picture, badge]
            }
            // Only the first item carries an image; the others ride along invisibly.
            if index > 0 { item.imageComponentsProvider = { [] } }
        }
    }

    static func countBadge(_ text: String) -> NSImage {
        NSImage(size: NSSize(width: 22, height: 22), flipped: false) { rect in
            NSColor.systemRed.setFill()
            NSBezierPath(ovalIn: rect).fill()
            let attributes: [NSAttributedString.Key: Any] = [.font: NSFont.systemFont(ofSize: 12, weight: .bold), .foregroundColor: NSColor.white]
            let size = (text as NSString).size(withAttributes: attributes)
            (text as NSString).draw(at: NSPoint(x: (rect.width - size.width) / 2, y: (rect.height - size.height) / 2), withAttributes: attributes)
            return true
        }
    }

    private func payload(on draggingInfo: NSDraggingInfo) -> SlideDragPayload? {
        guard let data = draggingInfo.draggingPasteboard.data(forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType)) else { return nil }
        return SlideDragPayload(data: data)
    }

    func collectionView(_ collectionView: NSCollectionView, validateDrop draggingInfo: NSDraggingInfo,
                        proposedIndexPath: AutoreleasingUnsafeMutablePointer<NSIndexPath>,
                        dropOperation: UnsafeMutablePointer<NSCollectionView.DropOperation>) -> NSDragOperation {
        guard let payload = payload(on: draggingInfo) else { return [] }
        dropOperation.pointee = .before
        let decision = dropDecision(proposedIndex: proposedIndexPath.pointee.item, payload: payload, deck: deckURL, optionHeld: optionHeld())
        if decision.operation != [], lastAnnouncedDrop != .some(decision.beforeNumber) {
            lastAnnouncedDrop = .some(decision.beforeNumber)
            NSAccessibility.post(element: collectionView, notification: .announcementRequested,
                                 userInfo: [.announcement: SlideAccessibility.dropLabel(beforeNumber: decision.beforeNumber, count: payload.slideNumbers.count),
                                            .priority: NSAccessibilityPriorityLevel.medium.rawValue])
        }
        return decision.operation
    }

    func collectionView(_ collectionView: NSCollectionView, acceptDrop draggingInfo: NSDraggingInfo, indexPath: IndexPath,
                        dropOperation: NSCollectionView.DropOperation) -> Bool {
        guard let payload = payload(on: draggingInfo) else { return false }
        lastAnnouncedDrop = nil
        let decision = dropDecision(proposedIndex: indexPath.item, payload: payload, deck: deckURL, optionHeld: optionHeld())
        guard decision.operation != [] else { return false }
        return performDrop(payload: payload, beforeNumber: decision.beforeNumber, isMove: decision.operation == .move)
    }
}
