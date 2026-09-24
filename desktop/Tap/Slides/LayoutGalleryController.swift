import AppKit

/// The gallery of tap's layouts in a popover: a schematic and a name per
/// layout, in tap's order, and a footer saying where the slide goes.
@MainActor
final class LayoutGalleryController: NSObject, NSCollectionViewDataSource, NSCollectionViewDelegate, NSPopoverDelegate {
    private static let cellIdentifier = NSUserInterfaceItemIdentifier("LayoutCell")
    private(set) var templates: [LayoutTemplate] = []
    let footerLabel = NSTextField(labelWithString: "")
    let sourceLabel = NSTextField(labelWithString: "Templates from tap slide add --print")
    var onPick: ((String) -> Void)?
    /// Kept by this controller rather than read from the popover, whose
    /// `isShown` can depend on whether the app is active, which differs
    /// between this machine and the CI runner.
    private(set) var isShown = false
    private let popover = NSPopover()
    private let collectionView: GalleryCollectionView

    /// Return picks the selected layout; NSCollectionView moves the selection with the arrow keys on its own.
    final class GalleryCollectionView: NSCollectionView {
        var onReturn: (() -> Void)?
        override func insertNewline(_ sender: Any?) { onReturn?() }
        override func keyDown(with event: NSEvent) {
            if event.keyCode == 36 || event.keyCode == 76 { onReturn?() } else { super.keyDown(with: event) }
        }
    }

    final class LayoutCell: NSCollectionViewItem {
        let schematic = LayoutSchematicView()
        let nameLabel = NSTextField(labelWithString: "")

        override func loadView() {
            let root = NSView()
            schematic.wantsLayer = true
            schematic.layer?.cornerRadius = 7
            nameLabel.font = .systemFont(ofSize: 11.5)
            nameLabel.alignment = .center
            for view in [schematic, nameLabel] as [NSView] {
                view.translatesAutoresizingMaskIntoConstraints = false
                root.addSubview(view)
            }
            NSLayoutConstraint.activate([
                schematic.topAnchor.constraint(equalTo: root.topAnchor, constant: 4),
                schematic.centerXAnchor.constraint(equalTo: root.centerXAnchor),
                schematic.widthAnchor.constraint(equalToConstant: 116),
                schematic.heightAnchor.constraint(equalToConstant: 65),
                nameLabel.topAnchor.constraint(equalTo: schematic.bottomAnchor, constant: 6),
                nameLabel.centerXAnchor.constraint(equalTo: root.centerXAnchor),
            ])
            view = root
        }

        override var isSelected: Bool {
            didSet {
                schematic.layer?.borderWidth = isSelected ? 3 : 0
                schematic.layer?.borderColor = NSColor.controlAccentColor.cgColor
            }
        }
    }

    override init() {
        collectionView = GalleryCollectionView()
        super.init()
        let layout = NSCollectionViewFlowLayout()
        layout.itemSize = NSSize(width: 124, height: 96)
        layout.minimumInteritemSpacing = 6
        layout.minimumLineSpacing = 8
        layout.sectionInset = NSEdgeInsets(top: 12, left: 12, bottom: 4, right: 12)
        collectionView.collectionViewLayout = layout
        collectionView.isSelectable = true
        collectionView.register(LayoutCell.self, forItemWithIdentifier: Self.cellIdentifier)
        collectionView.dataSource = self
        collectionView.delegate = self
        collectionView.backgroundColors = [.clear]
        collectionView.setAccessibilityIdentifier("layout-gallery")
        collectionView.onReturn = { [weak self] in
            guard let self, let path = self.collectionView.selectionIndexPaths.first else { return }
            self.pick(self.templates[path.item].name)
        }
        footerLabel.font = .systemFont(ofSize: 11)
        footerLabel.textColor = .secondaryLabelColor
        sourceLabel.font = .systemFont(ofSize: 11)
        sourceLabel.textColor = .tertiaryLabelColor

        let footer = NSStackView(views: [footerLabel, NSView(), sourceLabel])
        footer.orientation = .horizontal
        footer.edgeInsets = NSEdgeInsets(top: 4, left: 16, bottom: 12, right: 16)
        let scroll = NSScrollView()
        scroll.documentView = collectionView
        scroll.drawsBackground = false
        let stack = NSStackView(views: [scroll, footer])
        stack.orientation = .vertical
        stack.spacing = 0
        let content = NSViewController()
        content.view = stack
        stack.widthAnchor.constraint(equalToConstant: 4 * 124 + 3 * 6 + 24).isActive = true
        scroll.heightAnchor.constraint(equalToConstant: 3 * 96 + 2 * 8 + 16).isActive = true
        popover.contentViewController = content
        popover.behavior = .transient
        popover.delegate = self
    }

    func show(templates: [LayoutTemplate], relativeTo rect: NSRect, of view: NSView, afterSlide number: Int?) {
        self.templates = templates
        footerLabel.stringValue = number.map { "Inserts after slide \($0)" } ?? "Inserts at the end"
        collectionView.reloadData()
        popover.show(relativeTo: rect, of: view, preferredEdge: .maxY)
        isShown = true
        if !templates.isEmpty { collectionView.selectionIndexPaths = [IndexPath(item: 0, section: 0)] }
        // The popover has its own window; Return and the arrow keys go to the grid only there.
        collectionView.window?.makeFirstResponder(collectionView)
    }

    func close() {
        isShown = false
        popover.performClose(nil)
    }

    func popoverDidClose(_ notification: Notification) {
        isShown = false
    }

    /// Picks a layout by name: the click on a cell, Return, and the tests all come here.
    func pick(_ name: String) {
        guard templates.contains(where: { $0.name == name }) else { return }
        close()
        onPick?(name)
    }

    func collectionView(_ collectionView: NSCollectionView, numberOfItemsInSection section: Int) -> Int { templates.count }

    func collectionView(_ collectionView: NSCollectionView, itemForRepresentedObjectAt indexPath: IndexPath) -> NSCollectionViewItem {
        let cell = collectionView.makeItem(withIdentifier: Self.cellIdentifier, for: indexPath) as! LayoutCell
        let template = templates[indexPath.item]
        cell.schematic.elements = LayoutSchematic.elements(for: template.markdown)
        cell.nameLabel.stringValue = LayoutCatalog.displayName(template.name)
        cell.view.setAccessibilityLabel(LayoutCatalog.displayName(template.name))
        cell.view.setAccessibilityIdentifier("layout-\(template.name)")
        return cell
    }

    func collectionView(_ collectionView: NSCollectionView, didSelectItemsAt indexPaths: Set<IndexPath>) {
        // A mouse click selects and picks; the keyboard's selection changes only select.
        guard let path = indexPaths.first, NSApp.currentEvent?.type == .leftMouseUp || NSApp.currentEvent?.type == .leftMouseDown else { return }
        pick(templates[path.item].name)
    }
}
