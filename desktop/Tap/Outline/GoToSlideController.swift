import AppKit

/// A search field over the slide titles. Return jumps to the selected
/// slide, Escape closes it.
final class GoToSlideController: NSObject, NSSearchFieldDelegate, NSTableViewDataSource, NSTableViewDelegate {
    let panel: NSPanel
    let searchField = NSSearchField()
    let tableView = NSTableView()
    private(set) var entries: [OutlineEntry] = []
    private let slides: () -> [Slide]
    private let jump: (Int) -> Void

    init(slides: @escaping () -> [Slide], jump: @escaping (Int) -> Void) {
        self.slides = slides
        self.jump = jump
        panel = NSPanel(contentRect: NSRect(x: 0, y: 0, width: 480, height: 320),
                        styleMask: [.titled, .fullSizeContentView], backing: .buffered, defer: false)
        super.init()
        panel.titleVisibility = .hidden
        panel.titlebarAppearsTransparent = true
        panel.isReleasedWhenClosed = false
        panel.title = "Go to slide"

        searchField.placeholderString = "Go to slide"
        searchField.delegate = self
        searchField.setAccessibilityIdentifier("go-to-slide")
        let column = NSTableColumn(identifier: NSUserInterfaceItemIdentifier("slide"))
        tableView.addTableColumn(column)
        tableView.headerView = nil
        tableView.rowHeight = 36
        tableView.dataSource = self
        tableView.delegate = self
        tableView.target = self
        tableView.doubleAction = #selector(confirmFromTable(_:))
        let scrollView = NSScrollView()
        scrollView.documentView = tableView
        scrollView.hasVerticalScroller = true

        let root = NSView()
        for view in [searchField, scrollView] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(view)
        }
        NSLayoutConstraint.activate([
            searchField.topAnchor.constraint(equalTo: root.topAnchor, constant: 28),
            searchField.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: 12),
            searchField.trailingAnchor.constraint(equalTo: root.trailingAnchor, constant: -12),
            scrollView.topAnchor.constraint(equalTo: searchField.bottomAnchor, constant: 8),
            scrollView.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            scrollView.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            scrollView.bottomAnchor.constraint(equalTo: root.bottomAnchor),
        ])
        panel.contentView = root
    }

    func show(over window: NSWindow) {
        searchField.stringValue = ""
        setQuery("")
        let frame = window.frame
        panel.setFrameOrigin(NSPoint(x: frame.midX - panel.frame.width / 2, y: frame.maxY - panel.frame.height - 90))
        window.addChildWindow(panel, ordered: .above)
        panel.makeKeyAndOrderFront(nil)
        panel.makeFirstResponder(searchField)
    }

    func setQuery(_ query: String) {
        entries = SlideOutline.entries(for: slides(), matching: query)
        tableView.reloadData()
        if !entries.isEmpty { tableView.selectRowIndexes(IndexSet(integer: 0), byExtendingSelection: false) }
    }

    func confirm() {
        guard entries.indices.contains(tableView.selectedRow) else { return }
        let number = entries[tableView.selectedRow].number
        cancel()
        jump(number)
    }

    func cancel() {
        panel.parent?.removeChildWindow(panel)
        panel.orderOut(nil)
    }

    @objc private func confirmFromTable(_ sender: Any?) { confirm() }

    // MARK: Search field

    func controlTextDidChange(_ notification: Notification) {
        setQuery(searchField.stringValue)
    }

    func control(_ control: NSControl, textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
        switch commandSelector {
        case #selector(NSResponder.insertNewline(_:)):
            confirm()
        case #selector(NSResponder.cancelOperation(_:)):
            cancel()
        case #selector(NSResponder.moveDown(_:)):
            moveSelection(by: 1)
        case #selector(NSResponder.moveUp(_:)):
            moveSelection(by: -1)
        default:
            return false
        }
        return true
    }

    private func moveSelection(by offset: Int) {
        guard !entries.isEmpty else { return }
        let row = min(max(tableView.selectedRow + offset, 0), entries.count - 1)
        tableView.selectRowIndexes(IndexSet(integer: row), byExtendingSelection: false)
        tableView.scrollRowToVisible(row)
    }

    // MARK: Table

    func numberOfRows(in tableView: NSTableView) -> Int { entries.count }

    func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
        let entry = entries[row]
        let title = NSTextField(labelWithString: entry.title.isEmpty ? "Untitled" : entry.title)
        title.font = .systemFont(ofSize: 13, weight: .medium)
        let detail = NSTextField(labelWithString: "Slide \(entry.number) \u{00b7} \(entry.layout)")
        detail.font = .systemFont(ofSize: 11)
        detail.textColor = .secondaryLabelColor
        let stack = NSStackView(views: [title, detail])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 0
        return stack
    }
}
