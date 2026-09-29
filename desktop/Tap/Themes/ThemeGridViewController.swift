import AppKit

/// One cell: the theme's render (its name on a neutral fill until the
/// render lands) and its name. A button, so a click and VoiceOver's press
/// both pick it; every subview is hit-tested as the button itself.
final class ThemeCell: NSButton {
    let slug: String
    let imageView = NSImageView()
    let nameLabel = NSTextField(labelWithString: "")
    private let placeholder = NSTextField(labelWithString: "")

    var isSelected = false {
        didSet {
            updateBorder()
            setAccessibilityValue(isSelected ? "selected" : "")
        }
    }

    /// Draws the selected border while the pointer is over the cell, for a
    /// row of themes that has no selection of its own.
    var highlightsOnHover = false
    private var isHovered = false {
        didSet { updateBorder() }
    }
    private var hoverArea: NSTrackingArea?

    private func updateBorder() {
        let isHighlighted = isSelected || isHovered
        imageView.layer?.borderColor = isHighlighted ? NSColor.controlAccentColor.cgColor : NSColor.separatorColor.cgColor
        imageView.layer?.borderWidth = isHighlighted ? 3 : 0.5
    }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let hoverArea { removeTrackingArea(hoverArea) }
        hoverArea = nil
        guard highlightsOnHover else { return }
        let area = NSTrackingArea(rect: .zero, options: [.mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        hoverArea = area
    }

    override func mouseEntered(with event: NSEvent) { isHovered = highlightsOnHover }

    override func mouseExited(with event: NSEvent) { isHovered = false }

    init(theme: ThemeSummary, size: NSSize, nameFontSize: CGFloat, cornerRadius: CGFloat) {
        slug = theme.slug
        super.init(frame: .zero)
        title = ""
        isBordered = false
        setButtonType(.momentaryChange)
        imageView.wantsLayer = true
        imageView.layer?.cornerRadius = cornerRadius
        imageView.layer?.masksToBounds = true
        imageView.layer?.backgroundColor = NSColor.quaternaryLabelColor.cgColor
        imageView.imageScaling = .scaleProportionallyUpOrDown
        placeholder.stringValue = theme.name
        placeholder.font = .systemFont(ofSize: 11, weight: .semibold)
        placeholder.alignment = .center
        placeholder.textColor = .secondaryLabelColor
        nameLabel.stringValue = theme.name
        nameLabel.font = .systemFont(ofSize: nameFontSize)
        nameLabel.alignment = .center
        nameLabel.lineBreakMode = .byTruncatingTail
        let stack = NSStackView(views: [imageView, nameLabel])
        stack.orientation = .vertical
        stack.spacing = 4
        stack.translatesAutoresizingMaskIntoConstraints = false
        addSubview(stack)
        imageView.addSubview(placeholder)
        placeholder.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: topAnchor), stack.bottomAnchor.constraint(equalTo: bottomAnchor),
            stack.leadingAnchor.constraint(equalTo: leadingAnchor), stack.trailingAnchor.constraint(equalTo: trailingAnchor),
            imageView.widthAnchor.constraint(equalToConstant: size.width), imageView.heightAnchor.constraint(equalToConstant: size.height),
            placeholder.centerXAnchor.constraint(equalTo: imageView.centerXAnchor), placeholder.centerYAnchor.constraint(equalTo: imageView.centerYAnchor),
        ])
        setAccessibilityIdentifier("theme-cell-\(theme.slug)")
        setAccessibilityLabel("\(theme.name), \(theme.polarity) theme")
        isSelected = false
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    /// The render and the name are the button's, so a click on either picks.
    override func hitTest(_ point: NSPoint) -> NSView? {
        bounds.contains(convert(point, from: superview)) ? self : nil
    }

    func show(_ image: NSImage?) {
        imageView.image = image
        placeholder.isHidden = image != nil
    }
}

/// The grid of every theme, the same view inside the New Deck sheet, the
/// toolbar's popover and the Deck tab: Light and Dark sections, five
/// cells per row, in tap's order, with the renders as they land.
final class ThemeGridViewController: NSViewController {
    static let columns = 5
    /// The first cell: tap's default theme, drawn like the others with the
    /// default theme's render. Picking it removes the theme line (tap theme
    /// set default). A deck that names no theme has it selected.
    static let defaultSlug = "default"
    /// The ThemePicker and DeckTabThemeRow boards' cells (names at 11.5
    /// points, corners of 6), and the NewDeckHintNoSlug board's smaller
    /// ones (names at 10.5 points, corners of 5).
    static let popoverCellSize = NSSize(width: 104, height: 58)
    static let sheetCellSize = NSSize(width: 82, height: 46)
    let cellSize: NSSize
    var nameFontSize: CGFloat { cellSize == Self.popoverCellSize ? 11.5 : 10.5 }
    private var cornerRadius: CGFloat { cellSize == Self.popoverCellSize ? 6 : 5 }
    var onPick: ((String) -> Void)?
    private(set) var cells: [ThemeCell] = []
    private(set) var sectionTitles: [String] = []
    let footerLabel = NSTextField(wrappingLabelWithString: "Picking a theme runs tap theme set. Press T in the preview to try one without saving.")
    let downloadLabel = NSTextField(labelWithString: "")
    private let scrollView = NSScrollView()
    private let content = NSStackView()
    private var observers: [NSObjectProtocol] = []

    init(cellSize: NSSize) {
        self.cellSize = cellSize
        super.init(nibName: nil, bundle: nil)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    /// nil (no theme) selects the Default cell.
    var selectedSlug: String? {
        didSet { for cell in cells { cell.isSelected = cell.slug == (selectedSlug ?? Self.defaultSlug) } }
    }

    /// Whether the footer line shows: the toolbar popover and the Deck tab
    /// show it; the New Deck sheet has no deck to set a theme on.
    var showsFooter = true {
        didSet { footerLabel.isHidden = !showsFooter }
    }

    /// Light and Dark headings over their cells (the popover), or one flat
    /// grid in the same order (the New Deck sheet). Set before the view loads.
    var showsSections = true

    /// The rows the scroll view shows at once, nil for the popover's 320
    /// points. The New Deck sheet shows two. Set before the view loads.
    var visibleRows: Int?

    /// The width five cells need, for a sheet that sizes itself to the grid.
    var contentWidth: CGFloat { CGFloat(Self.columns) * cellSize.width + CGFloat(Self.columns - 1) * 12 + 8 }

    func cell(for slug: String) -> ThemeCell? { cells.first { $0.slug == slug } }

    override func loadView() {
        content.orientation = .vertical
        content.alignment = .leading
        content.spacing = 8
        content.edgeInsets = NSEdgeInsets(top: 4, left: 4, bottom: 4, right: 4)
        scrollView.documentView = content
        scrollView.hasVerticalScroller = true
        scrollView.drawsBackground = false
        content.translatesAutoresizingMaskIntoConstraints = false
        content.widthAnchor.constraint(equalTo: scrollView.contentView.widthAnchor).isActive = true
        footerLabel.font = .systemFont(ofSize: 11.5)
        footerLabel.textColor = .secondaryLabelColor
        downloadLabel.font = .systemFont(ofSize: 11.5)
        downloadLabel.textColor = .secondaryLabelColor
        downloadLabel.isHidden = true
        downloadLabel.setAccessibilityIdentifier("theme-download")
        let root = NSStackView(views: [scrollView, downloadLabel, footerLabel])
        root.orientation = .vertical
        root.alignment = .leading
        root.spacing = 8
        scrollView.heightAnchor.constraint(equalToConstant: scrollHeight).isActive = true
        scrollView.widthAnchor.constraint(equalTo: root.widthAnchor).isActive = true
        root.widthAnchor.constraint(equalToConstant: contentWidth).isActive = true
        view = root
        let loader = AppEnvironment.shared.themeImages
        observers = [
            NotificationCenter.default.addObserver(forName: ThemeImageLoader.didLoadCatalogNotification, object: loader, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated { self?.rebuild() }
            },
            NotificationCenter.default.addObserver(forName: ThemeImageLoader.didLoadImageNotification, object: loader, queue: .main) { [weak self] notification in
                MainActor.assumeIsolated {
                    guard let slug = notification.userInfo?["slug"] as? String else { return }
                    self?.cell(for: slug)?.show(loader.image(for: slug))
                    if slug == loader.defaultSlug { self?.cell(for: Self.defaultSlug)?.show(loader.image(for: slug)) }
                }
            },
            NotificationCenter.default.addObserver(forName: ThemeImageLoader.downloadDidChangeNotification, object: loader, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated { self?.refreshDownload() }
            },
        ]
        rebuild()
        refreshDownload()
        loader.loadAll(retryingFailures: true)
    }

    deinit {
        for observer in observers { NotificationCenter.default.removeObserver(observer) }
    }

    private var scrollHeight: CGFloat {
        guard let visibleRows else { return 320 }
        let font = NSFont.systemFont(ofSize: nameFontSize)
        let rowHeight = cellSize.height + 4 + ceil(font.ascender - font.descender + font.leading)
        return CGFloat(visibleRows) * rowHeight + CGFloat(visibleRows - 1) * 12 + 8
    }

    private func rebuild() {
        for view in content.arrangedSubviews { view.removeFromSuperview() }
        cells = []
        sectionTitles = []
        guard let catalog = AppEnvironment.shared.themeImages.catalog else { return }
        let defaultName = catalog.name(forSlug: AppEnvironment.shared.themeImages.defaultSlug)
        let defaultTheme = ThemeSummary(slug: Self.defaultSlug, name: "Default", polarity: "light", pitch: "tap's default theme (\(defaultName))")
        let sections = showsSections ? [("Light", [defaultTheme] + catalog.light), ("Dark", catalog.dark)] : [("", [defaultTheme] + catalog.light + catalog.dark)]
        for (title, themes) in sections where !themes.isEmpty {
            if showsSections {
                sectionTitles.append(title)
                let heading = NSTextField(labelWithString: title)
                heading.font = .systemFont(ofSize: 11, weight: .semibold)
                heading.textColor = .secondaryLabelColor
                content.addArrangedSubview(heading)
            }
            let grid = NSGridView()
            grid.rowSpacing = 12
            grid.columnSpacing = 12
            for row in stride(from: 0, to: themes.count, by: Self.columns) {
                let rowCells = themes[row..<min(row + Self.columns, themes.count)].map { theme -> ThemeCell in
                    let cell = ThemeCell(theme: theme, size: cellSize, nameFontSize: nameFontSize, cornerRadius: cornerRadius)
                    if theme.slug == Self.defaultSlug { cell.setAccessibilityLabel("Default, tap's default theme (\(defaultName))") }
                    cell.show(AppEnvironment.shared.themeImages.image(for: theme.slug))
                    cell.isSelected = theme.slug == (selectedSlug ?? Self.defaultSlug)
                    cell.target = self
                    cell.action = #selector(cellPressed(_:))
                    cells.append(cell)
                    return cell
                }
                // Each empty slot gets a view of its own: NSGridView raises
                // NSInvalidArgumentException for one view in two cells, and
                // an exception raised inside a Swift task's job (the loader's
                // catalog notification, a test's popover) leaves that task
                // current on the main thread and crashes the process later.
                let fillers = (rowCells.count..<Self.columns).map { _ in NSView() }
                grid.addRow(with: rowCells + fillers)
            }
            content.addArrangedSubview(grid)
        }
    }

    private func refreshDownload() {
        guard let progress = AppEnvironment.shared.themeImages.downloadProgress else {
            downloadLabel.isHidden = true
            return
        }
        let formatter = ByteCountFormatter()
        formatter.countStyle = .file
        downloadLabel.stringValue = "Downloading the export engine: \(formatter.string(fromByteCount: progress.bytes)) of \(formatter.string(fromByteCount: progress.totalBytes)). This happens once."
        downloadLabel.isHidden = false
    }

    @objc private func cellPressed(_ sender: ThemeCell) {
        selectedSlug = sender.slug
        onPick?(sender.slug)
    }
}
