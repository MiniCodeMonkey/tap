import AppKit

/// Shown when no deck is open: New Deck, Open, and recent decks with thumbnails.
/// With no recent decks the two columns give way to one centered invitation:
/// a headline, four theme thumbnails, New Deck, Open and a drop zone.
final class WelcomeWindowController: NSWindowController, NSTableViewDataSource, NSTableViewDelegate {
    static let shared = WelcomeWindowController()

    /// The themes the empty state offers, in order: a varied set from tap's catalog.
    static let emptyStateThemeSlugs = ["poster", "paperback", "riso", "blueprint"]
    static let emptyStateThemeSize = NSSize(width: 112, height: 62)

    let columnNewDeckButton = NSButton(title: "New Deck…", target: nil, action: nil)
    let columnOpenButton = NSButton(title: "Open…", target: nil, action: #selector(NSDocumentController.openDocument(_:)))
    let emptyStateNewDeckButton = NSButton(title: "New Deck…", target: nil, action: nil)
    let emptyStateOpenButton = NSButton(title: "Open…", target: nil, action: #selector(NSDocumentController.openDocument(_:)))
    let emptyStateView = NSView()
    let dropZone = WelcomeDropZone(text: "or drop a Markdown file here")
    let tableView = NSTableView()
    private let versionLabel = NSTextField(labelWithString: "")
    private let columnsView = NSStackView()
    private let recentsScrollView = NSScrollView()
    private let themeRow = NSStackView()
    private(set) var emptyStateThemeCells: [ThemeCell] = []
    private(set) var recentURLs: [URL] = []
    private(set) var showsEmptyState = false
    private var refreshTimer: Timer?
    private var observers: [NSObjectProtocol] = []

    /// The New Deck button that is on screen: the empty state's or the column's.
    var newDeckButton: NSButton { showsEmptyState ? emptyStateNewDeckButton : columnNewDeckButton }
    /// The Open button that is on screen.
    var openButton: NSButton { showsEmptyState ? emptyStateOpenButton : columnOpenButton }

    static func closeIfOpen() {
        shared.window?.orderOut(nil)
    }

    private init() {
        let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 800, height: 460),
                              styleMask: [.titled, .closable, .fullSizeContentView], backing: .buffered, defer: false)
        window.titlebarAppearsTransparent = true
        window.titleVisibility = .hidden
        window.isReleasedWhenClosed = false
        window.center()
        super.init(window: window)

        let icon = NSImageView(image: NSApp.applicationIconImage ?? NSImage())
        icon.imageScaling = .scaleProportionallyUpOrDown
        let name = NSTextField(labelWithString: "Tap")
        name.font = .systemFont(ofSize: 28, weight: .semibold)
        versionLabel.textColor = .secondaryLabelColor
        // nil-targeted, so the responder chain reaches the app delegate.
        for button in [columnNewDeckButton, emptyStateNewDeckButton] {
            button.target = nil
            button.action = #selector(AppDelegate.newDeck(_:))
            button.isEnabled = true
            button.setAccessibilityIdentifier("new-deck")
        }
        for button in [columnOpenButton, emptyStateOpenButton] { button.setAccessibilityIdentifier("open") }
        for button in [columnNewDeckButton, columnOpenButton, emptyStateNewDeckButton, emptyStateOpenButton] {
            button.bezelStyle = .rounded
            button.controlSize = .large
        }
        let left = columnsView
        for view in [icon, name, versionLabel, columnNewDeckButton, columnOpenButton] { left.addArrangedSubview(view) }
        left.orientation = .vertical
        left.spacing = 10
        NSLayoutConstraint.activate([icon.widthAnchor.constraint(equalToConstant: 96), icon.heightAnchor.constraint(equalToConstant: 96)])

        let column = NSTableColumn(identifier: NSUserInterfaceItemIdentifier("recent"))
        tableView.addTableColumn(column)
        tableView.headerView = nil
        tableView.rowHeight = 64
        tableView.style = .inset
        tableView.dataSource = self
        tableView.delegate = self
        tableView.target = self
        tableView.action = #selector(openSelectedRow(_:))
        tableView.setAccessibilityIdentifier("recent-decks")
        let scrollView = recentsScrollView
        scrollView.documentView = tableView
        scrollView.hasVerticalScroller = true

        let root = NSView()
        for view in [left, scrollView] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(view)
        }
        NSLayoutConstraint.activate([
            left.centerYAnchor.constraint(equalTo: root.centerYAnchor),
            left.centerXAnchor.constraint(equalTo: root.leadingAnchor, constant: 170),
            scrollView.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: 340),
            scrollView.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            scrollView.topAnchor.constraint(equalTo: root.topAnchor, constant: 28),
            scrollView.bottomAnchor.constraint(equalTo: root.bottomAnchor),
        ])
        buildEmptyState(in: root)
        window.contentView = root
        dropZone.onDrop = { url in
            NSDocumentController.shared.openDocument(withContentsOf: url, display: true) { _, _, _ in }
        }
        let center = NotificationCenter.default
        let loader = AppEnvironment.shared.themeImages
        observers = [
            center.addObserver(forName: NSWindow.didBecomeKeyNotification, object: window, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated { self?.reload() }
            },
            center.addObserver(forName: ThemeImageLoader.didLoadCatalogNotification, object: loader, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated { self?.rebuildThemeRow() }
            },
            center.addObserver(forName: ThemeImageLoader.didLoadImageNotification, object: loader, queue: .main) { [weak self] notification in
                MainActor.assumeIsolated {
                    guard let slug = notification.userInfo?["slug"] as? String else { return }
                    self?.emptyStateThemeCells.first { $0.slug == slug }?.show(loader.image(for: slug))
                }
            },
        ]
    }

    /// One centered column: icon, headline, subline, theme thumbnails, the
    /// buttons and the drop zone, drawn in system colors.
    private func buildEmptyState(in root: NSView) {
        let icon = NSImageView(image: NSApp.applicationIconImage ?? NSImage())
        icon.imageScaling = .scaleProportionallyUpOrDown
        let headline = NSTextField(labelWithString: "Make your first deck")
        headline.font = .systemFont(ofSize: 22, weight: .semibold)
        let subline = NSTextField(labelWithString: "Write slides in Markdown, run code live, present from here.")
        subline.textColor = .secondaryLabelColor
        themeRow.orientation = .horizontal
        themeRow.spacing = 12
        themeRow.setAccessibilityIdentifier("welcome-themes")
        let buttons = NSStackView(views: [emptyStateNewDeckButton, emptyStateOpenButton])
        buttons.spacing = 10
        let stack = NSStackView(views: [icon, headline, subline, themeRow, buttons, dropZone])
        stack.orientation = .vertical
        stack.alignment = .centerX
        stack.spacing = 8
        stack.setCustomSpacing(12, after: icon)
        stack.setCustomSpacing(18, after: subline)
        stack.setCustomSpacing(18, after: themeRow)
        stack.setCustomSpacing(18, after: buttons)
        stack.translatesAutoresizingMaskIntoConstraints = false
        emptyStateView.translatesAutoresizingMaskIntoConstraints = false
        emptyStateView.addSubview(stack)
        emptyStateView.isHidden = true
        emptyStateView.setAccessibilityIdentifier("welcome-empty-state")
        root.addSubview(emptyStateView)
        NSLayoutConstraint.activate([
            icon.widthAnchor.constraint(equalToConstant: 80), icon.heightAnchor.constraint(equalToConstant: 80),
            themeRow.heightAnchor.constraint(equalToConstant: Self.emptyStateThemeSize.height + 22),
            emptyStateView.leadingAnchor.constraint(equalTo: root.leadingAnchor), emptyStateView.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            emptyStateView.topAnchor.constraint(equalTo: root.topAnchor), emptyStateView.bottomAnchor.constraint(equalTo: root.bottomAnchor),
            stack.centerXAnchor.constraint(equalTo: emptyStateView.centerXAnchor), stack.centerYAnchor.constraint(equalTo: emptyStateView.centerYAnchor, constant: 8),
        ])
    }

    /// The thumbnails for the empty state's themes that tap's catalog has.
    private func rebuildThemeRow() {
        for view in themeRow.arrangedSubviews { view.removeFromSuperview() }
        emptyStateThemeCells = []
        guard let catalog = AppEnvironment.shared.themeImages.catalog else { return }
        for slug in Self.emptyStateThemeSlugs {
            guard let theme = catalog.theme(slug: slug) else { continue }
            let cell = ThemeCell(theme: theme, size: Self.emptyStateThemeSize, nameFontSize: 11.5, cornerRadius: 8)
            cell.highlightsOnHover = true
            cell.show(AppEnvironment.shared.themeImages.image(for: slug))
            cell.target = self
            cell.action = #selector(themePressed(_:))
            themeRow.addArrangedSubview(cell)
            emptyStateThemeCells.append(cell)
        }
    }

    @objc private func themePressed(_ sender: ThemeCell) {
        (NSApp.delegate as? AppDelegate)?.newDeck(on: window, theme: sender.slug)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    // NSWindowController's own showWindow(_:) is the one place this window
    // is brought forward and made key; nothing here calls
    // makeKeyAndOrderFront or NSApp.activate itself, so opening the welcome
    // window never steals focus beyond what that default implementation does.
    override func showWindow(_ sender: Any?) {
        reload()
        super.showWindow(sender)
        startWatchingRecents()
    }

    /// The recents list has no change notification, so a visible window
    /// looks again every second and switches layout when the list changes.
    private func startWatchingRecents() {
        guard refreshTimer == nil else { return }
        refreshTimer = Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { [weak self] _ in
            MainActor.assumeIsolated {
                guard let self else { return }
                guard self.window?.isVisible == true else {
                    self.refreshTimer?.invalidate()
                    self.refreshTimer = nil
                    return
                }
                self.reload()
            }
        }
    }

    func reload() {
        versionLabel.stringValue = "Version \(Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "")"
        let urls = NSDocumentController.shared.recentDocumentURLs.filter { FileManager.default.fileExists(atPath: $0.path) }
        if urls != recentURLs {
            recentURLs = urls
            tableView.reloadData()
        }
        showEmptyState(urls.isEmpty)
    }

    /// Swaps the two layouts; New Deck is the empty state's default button (Return).
    private func showEmptyState(_ shows: Bool) {
        showsEmptyState = shows
        emptyStateView.isHidden = !shows
        columnsView.isHidden = shows
        recentsScrollView.isHidden = shows
        emptyStateNewDeckButton.keyEquivalent = shows ? "\r" : ""
        if shows {
            let loader = AppEnvironment.shared.themeImages
            if emptyStateThemeCells.isEmpty { rebuildThemeRow() }
            loader.loadImages(for: Self.emptyStateThemeSlugs)
        }
    }

    func thumbnail(forRow row: Int) -> NSImage? {
        guard recentURLs.indices.contains(row) else { return nil }
        return AppEnvironment.shared.recentThumbnailStore.imageData(for: recentURLs[row]).flatMap(NSImage.init(data:))
    }

    @objc private func openSelectedRow(_ sender: Any?) {
        guard recentURLs.indices.contains(tableView.clickedRow) else { return }
        NSDocumentController.shared.openDocument(withContentsOf: recentURLs[tableView.clickedRow], display: true) { _, _, _ in }
    }

    // MARK: Table

    func numberOfRows(in tableView: NSTableView) -> Int { recentURLs.count }

    func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
        let url = recentURLs[row]
        let image = NSImageView(image: thumbnail(forRow: row) ?? NSImage(systemSymbolName: "rectangle", accessibilityDescription: nil) ?? NSImage())
        image.imageScaling = .scaleProportionallyUpOrDown
        image.wantsLayer = true
        image.layer?.cornerRadius = 4
        image.layer?.masksToBounds = true
        let name = NSTextField(labelWithString: url.lastPathComponent)
        name.font = .systemFont(ofSize: 13, weight: .medium)
        let folder = NSTextField(labelWithString: (url.deletingLastPathComponent().path as NSString).abbreviatingWithTildeInPath)
        folder.textColor = .secondaryLabelColor
        let modified = (try? FileManager.default.attributesOfItem(atPath: url.path))?[.modificationDate] as? Date
        let date = NSTextField(labelWithString: modified.map { RelativeDateTimeFormatter().localizedString(for: $0, relativeTo: Date()) } ?? "")
        date.textColor = .tertiaryLabelColor
        let text = NSStackView(views: [name, folder, date])
        text.orientation = .vertical
        text.alignment = .leading
        text.spacing = 1
        let row = NSStackView(views: [image, text])
        row.spacing = 12
        NSLayoutConstraint.activate([image.widthAnchor.constraint(equalToConstant: 96), image.heightAnchor.constraint(equalToConstant: 54)])
        return row
    }
}
