import AppKit

/// The gallery of tap's layouts in a popover: a schematic and a name per
/// layout, in tap's order, and a footer saying where the slide goes. For a
/// slide's layout it also offers Automatic, and a component slide's own
/// component in a section of its own above the rest.
@MainActor
final class LayoutGalleryController: NSObject, NSCollectionViewDataSource, NSCollectionViewDelegate, NSCollectionViewDelegateFlowLayout, NSPopoverDelegate {
    private static let cellIdentifier = NSUserInterfaceItemIdentifier("LayoutCell")
    private static let dividerIdentifier = NSUserInterfaceItemIdentifier("LayoutDivider")
    private static let itemSize = NSSize(width: 124, height: 108)

    /// What a cell stands for: a layout tap renders, Automatic (no
    /// declaration), or the component the slide declares.
    enum Choice: Equatable {
        case automatic
        case layout(String)
        case component(String)
    }

    struct Entry {
        let choice: Choice
        let title: String
        var detail: String?
        var elements: [LayoutSchematic.Element]
    }

    /// The layouts a New Slide gallery inserts from, in tap's order.
    private(set) var templates: [LayoutTemplate] = []
    /// The cells by section: the component, when the slide has one, then the rest.
    private(set) var sections: [[Entry]] = []
    let footerLabel = NSTextField(labelWithString: "")
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
        let componentGlyph = NSTextField(labelWithString: "</>")
        let nameLabel = NSTextField(labelWithString: "")

        override func loadView() {
            let root = NSView()
            schematic.wantsLayer = true
            schematic.layer?.cornerRadius = 7
            componentGlyph.font = .monospacedSystemFont(ofSize: 18, weight: .semibold)
            componentGlyph.textColor = .systemPurple
            nameLabel.font = .systemFont(ofSize: 11.5)
            nameLabel.alignment = .center
            nameLabel.maximumNumberOfLines = 2
            for view in [schematic, nameLabel] as [NSView] {
                view.translatesAutoresizingMaskIntoConstraints = false
                root.addSubview(view)
            }
            componentGlyph.translatesAutoresizingMaskIntoConstraints = false
            schematic.addSubview(componentGlyph)
            NSLayoutConstraint.activate([
                schematic.topAnchor.constraint(equalTo: root.topAnchor, constant: 4),
                schematic.centerXAnchor.constraint(equalTo: root.centerXAnchor),
                schematic.widthAnchor.constraint(equalToConstant: 116),
                schematic.heightAnchor.constraint(equalToConstant: 65),
                componentGlyph.centerXAnchor.constraint(equalTo: schematic.centerXAnchor),
                componentGlyph.centerYAnchor.constraint(equalTo: schematic.centerYAnchor),
                nameLabel.topAnchor.constraint(equalTo: schematic.bottomAnchor, constant: 6),
                nameLabel.centerXAnchor.constraint(equalTo: root.centerXAnchor),
                nameLabel.widthAnchor.constraint(lessThanOrEqualToConstant: 120),
            ])
            // A cell is picked by a click, so it is a button to
            // accessibility, as a thumbnail is; the identifier and the
            // label the data source sets reach the tree through it.
            root.setAccessibilityElement(true)
            root.setAccessibilityRole(.button)
            view = root
        }

        func show(_ entry: Entry) {
            schematic.elements = entry.elements
            if case .component = entry.choice { componentGlyph.isHidden = false } else { componentGlyph.isHidden = true }
            let name = NSMutableAttributedString(string: entry.title, attributes: [.font: NSFont.systemFont(ofSize: 11.5), .foregroundColor: NSColor.labelColor])
            if let detail = entry.detail {
                name.append(NSAttributedString(string: "\n" + detail, attributes: [.font: NSFont.systemFont(ofSize: 10.5), .foregroundColor: NSColor.secondaryLabelColor]))
            }
            let centered = NSMutableParagraphStyle()
            centered.alignment = .center
            centered.lineBreakMode = .byTruncatingMiddle
            name.addAttribute(.paragraphStyle, value: centered, range: NSRange(location: 0, length: name.length))
            nameLabel.attributedStringValue = name
        }

        override var isSelected: Bool {
            didSet {
                schematic.layer?.borderWidth = isSelected ? 3 : 0
                schematic.layer?.borderColor = NSColor.controlAccentColor.cgColor
            }
        }
    }

    /// The hairline between the component's section and tap's layouts.
    final class Divider: NSView, NSCollectionViewElement {
        override func draw(_ dirtyRect: NSRect) {
            NSColor.separatorColor.setFill()
            NSRect(x: 12, y: bounds.midY, width: bounds.width - 24, height: 1).fill()
        }
    }

    override init() {
        collectionView = GalleryCollectionView()
        super.init()
        let layout = NSCollectionViewFlowLayout()
        layout.itemSize = Self.itemSize
        layout.minimumInteritemSpacing = 6
        layout.minimumLineSpacing = 8
        layout.sectionInset = NSEdgeInsets(top: 12, left: 12, bottom: 4, right: 12)
        collectionView.collectionViewLayout = layout
        collectionView.isSelectable = true
        collectionView.register(LayoutCell.self, forItemWithIdentifier: Self.cellIdentifier)
        collectionView.register(Divider.self, forSupplementaryViewOfKind: NSCollectionView.elementKindSectionFooter, withIdentifier: Self.dividerIdentifier)
        collectionView.dataSource = self
        collectionView.delegate = self
        collectionView.backgroundColors = [.clear]
        collectionView.setAccessibilityIdentifier("layout-gallery")
        collectionView.onReturn = { [weak self] in
            guard let self, let choice = self.selectedChoice else { return }
            self.pick(choice)
        }
        footerLabel.font = .systemFont(ofSize: 11)
        footerLabel.textColor = .secondaryLabelColor

        let footer = NSStackView(views: [footerLabel, NSView()])
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
        stack.widthAnchor.constraint(equalToConstant: 4 * Self.itemSize.width + 3 * 6 + 24).isActive = true
        scroll.heightAnchor.constraint(equalToConstant: 3 * Self.itemSize.height + 2 * 8 + 16).isActive = true
        popover.contentViewController = content
        popover.behavior = .transient
        popover.delegate = self
    }

    /// Set while the gallery changes a slide's layout: the pick goes here
    /// instead of `onPick`, with nil for Automatic. Cleared by the next
    /// `show` for a new slide.
    private var changePick: ((String?) -> Void)?

    /// The gallery for one slide's layout. `declared` is the slide's
    /// `layout:` value (nil when it declares none), `rendered` the layout
    /// tap renders it with, and `component` the component's file name when
    /// the declared layout is one. The slide's own choice is selected.
    func show(templates: [LayoutTemplate], relativeTo rect: NSRect, of view: NSView, changingSlide number: Int,
              declared: String?, rendered: String, component: String?, onPick: @escaping (String?) -> Void) {
        let schematic = { (name: String) in templates.first { $0.name == name }.map { LayoutSchematic.elements(for: $0.markdown) } ?? [] }
        var main: [Entry] = []
        // A component slide's Automatic is whatever tap detects once the component is gone, which it has not said yet.
        main.append(component == nil
            ? Entry(choice: .automatic, title: "Automatic", detail: LayoutCatalog.displayName(rendered), elements: schematic(rendered))
            : Entry(choice: .automatic, title: "Automatic", elements: schematic("default")))
        // A declared layout tap no longer lists, or one still loading, keeps its own cell.
        if component == nil, let declared, !templates.contains(where: { $0.name == declared }) {
            main.append(Entry(choice: .layout(declared), title: LayoutCatalog.displayName(declared), elements: []))
        }
        main += templates.map(Self.entry)
        let current: Choice = component.map { .component($0) } ?? declared.map { .layout($0) } ?? .automatic
        let leading = component.map { [[Entry(choice: .component($0), title: $0, detail: "Custom component", elements: [])]] } ?? []
        show(sections: leading + [main], templates: templates, relativeTo: rect, of: view, footer: "Changes the layout of slide \(number)", preselecting: current, changePick: onPick)
    }

    func show(templates: [LayoutTemplate], relativeTo rect: NSRect, of view: NSView, afterSlide number: Int?) {
        show(sections: [templates.map(Self.entry)], templates: templates, relativeTo: rect, of: view,
             footer: number.map { "Inserts after slide \($0)" } ?? "Inserts at the end", preselecting: nil, changePick: nil)
    }

    private static func entry(_ template: LayoutTemplate) -> Entry {
        Entry(choice: .layout(template.name), title: LayoutCatalog.displayName(template.name), elements: LayoutSchematic.elements(for: template.markdown))
    }

    private func show(sections: [[Entry]], templates: [LayoutTemplate], relativeTo rect: NSRect, of view: NSView, footer: String, preselecting choice: Choice?, changePick: ((String?) -> Void)?) {
        self.changePick = changePick
        self.templates = templates
        self.sections = sections
        footerLabel.stringValue = footer
        collectionView.reloadData()
        popover.show(relativeTo: rect, of: view, preferredEdge: .maxY)
        isShown = true
        if let path = indexPath(of: choice) ?? (sections.first?.isEmpty == false ? IndexPath(item: 0, section: 0) : nil) {
            collectionView.selectionIndexPaths = [path]
            collectionView.scrollToItems(at: [path], scrollPosition: .centeredVertically)
        }
        // The popover has its own window; Return and the arrow keys go to the grid only there.
        collectionView.window?.makeFirstResponder(collectionView)
    }

    private func indexPath(of choice: Choice?) -> IndexPath? {
        guard let choice else { return nil }
        for (section, entries) in sections.enumerated() {
            if let item = entries.firstIndex(where: { $0.choice == choice }) { return IndexPath(item: item, section: section) }
        }
        return nil
    }

    private func entry(at path: IndexPath) -> Entry? {
        sections.indices.contains(path.section) && sections[path.section].indices.contains(path.item) ? sections[path.section][path.item] : nil
    }

    /// The cell the grid has selected: what Return would pick.
    var selectedChoice: Choice? {
        collectionView.selectionIndexPaths.first.flatMap(entry(at:))?.choice
    }

    /// The selected cell's layout name, nil for Automatic or a component.
    var selectedLayoutName: String? {
        if case .layout(let name) = selectedChoice { return name }
        return nil
    }

    func close() {
        isShown = false
        popover.performClose(nil)
    }

    func popoverDidClose(_ notification: Notification) {
        isShown = false
    }

    /// Picks a layout by name: the tests' way in.
    func pick(_ name: String) {
        pick(.layout(name))
    }

    /// The click on a cell and Return come here. The component the slide
    /// already declares changes nothing.
    func pick(_ choice: Choice) {
        guard indexPath(of: choice) != nil else { return }
        close()
        switch choice {
        case .automatic: changePick?(nil)
        case .layout(let name): if let changePick { changePick(name) } else { onPick?(name) }
        case .component: break
        }
    }

    func numberOfSections(in collectionView: NSCollectionView) -> Int { sections.count }

    func collectionView(_ collectionView: NSCollectionView, numberOfItemsInSection section: Int) -> Int { sections[section].count }

    func collectionView(_ collectionView: NSCollectionView, itemForRepresentedObjectAt indexPath: IndexPath) -> NSCollectionViewItem {
        let cell = collectionView.makeItem(withIdentifier: Self.cellIdentifier, for: indexPath) as! LayoutCell
        let entry = sections[indexPath.section][indexPath.item]
        cell.show(entry)
        cell.view.setAccessibilityLabel([entry.title, entry.detail].compactMap { $0 }.joined(separator: ", "))
        switch entry.choice {
        case .automatic: cell.view.setAccessibilityIdentifier("layout-automatic")
        case .layout(let name): cell.view.setAccessibilityIdentifier("layout-\(name)")
        case .component: cell.view.setAccessibilityIdentifier("layout-component")
        }
        return cell
    }

    func collectionView(_ collectionView: NSCollectionView, viewForSupplementaryElementOfKind kind: NSCollectionView.SupplementaryElementKind, at indexPath: IndexPath) -> NSView {
        collectionView.makeSupplementaryView(ofKind: kind, withIdentifier: Self.dividerIdentifier, for: indexPath)
    }

    /// Only a section with another after it has the divider under it.
    func collectionView(_ collectionView: NSCollectionView, layout collectionViewLayout: NSCollectionViewLayout, referenceSizeForFooterInSection section: Int) -> NSSize {
        section < sections.count - 1 ? NSSize(width: 0, height: 9) : .zero
    }

    func collectionView(_ collectionView: NSCollectionView, didSelectItemsAt indexPaths: Set<IndexPath>) {
        // A mouse click selects and picks; the keyboard's selection changes only select.
        guard let path = indexPaths.first, let entry = entry(at: path),
              NSApp.currentEvent?.type == .leftMouseUp || NSApp.currentEvent?.type == .leftMouseDown else { return }
        pick(entry.choice)
    }
}
