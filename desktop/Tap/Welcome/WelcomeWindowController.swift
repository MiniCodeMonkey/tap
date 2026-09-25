import AppKit

/// Shown when no deck is open: New Deck, Open, and recent decks with thumbnails.
final class WelcomeWindowController: NSWindowController, NSTableViewDataSource, NSTableViewDelegate {
    static let shared = WelcomeWindowController()

    let newDeckButton = NSButton(title: "New Deck…", target: nil, action: nil)
    let openButton = NSButton(title: "Open…", target: nil, action: #selector(NSDocumentController.openDocument(_:)))
    let tableView = NSTableView()
    private let versionLabel = NSTextField(labelWithString: "")
    private(set) var recentURLs: [URL] = []

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
        // New Deck arrives with the New Deck sheet, which runs tap new.
        newDeckButton.isEnabled = false
        newDeckButton.setAccessibilityIdentifier("new-deck")
        openButton.setAccessibilityIdentifier("open")
        for button in [newDeckButton, openButton] {
            button.bezelStyle = .rounded
            button.controlSize = .large
        }
        let left = NSStackView(views: [icon, name, versionLabel, newDeckButton, openButton])
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
        let scrollView = NSScrollView()
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
        window.contentView = root
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    // NSWindowController's own showWindow(_:) is the one place this window
    // is brought forward and made key; nothing here calls
    // makeKeyAndOrderFront or NSApp.activate itself, so opening the welcome
    // window never steals focus beyond what that default implementation does.
    override func showWindow(_ sender: Any?) {
        reload()
        super.showWindow(sender)
    }

    func reload() {
        versionLabel.stringValue = "Version \(Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String ?? "")"
        recentURLs = NSDocumentController.shared.recentDocumentURLs.filter { FileManager.default.fileExists(atPath: $0.path) }
        tableView.reloadData()
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
