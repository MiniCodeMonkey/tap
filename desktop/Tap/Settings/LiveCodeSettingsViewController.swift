import AppKit

/// Live Code: the decks tap allows to run code, from tap approval list;
/// Revoke runs tap approval revoke. The list is tap's file, so the CLI
/// and the app see the same approvals. Commands are shown as tap prints
/// them, secrets masked by tap; nothing is expanded here.
final class LiveCodeSettingsViewController: NSViewController, NSTableViewDataSource, NSTableViewDelegate {
    let introLabel = NSTextField(wrappingLabelWithString: "Decks you allowed to run code on this Mac. tap keeps this list in ~/.config/tap/settings.yaml, so tap dev and tap present in Terminal use the same approvals.")
    let table = NSTableView()
    let revokeButton = NSButton(title: "Revoke", target: nil, action: nil)
    let revealButton = NSButton(title: "Show in Finder", target: nil, action: nil)
    /// tap's message when a revoke fails, under the table.
    let errorLabel = NSTextField(wrappingLabelWithString: "")
    private(set) var records: [ApprovalRecord] = []
    var revealInFinder: (URL) -> Void = { url in NSWorkspace.shared.activateFileViewerSelecting([url]) }
    private static let dateFormatter: RelativeDateTimeFormatter = {
        let formatter = RelativeDateTimeFormatter()
        formatter.unitsStyle = .short
        return formatter
    }()

    override init(nibName: NSNib.Name?, bundle: Bundle?) {
        super.init(nibName: nil, bundle: nil)
        title = "Live Code"
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func loadView() {
        introLabel.font = .systemFont(ofSize: 12.5)
        introLabel.textColor = .secondaryLabelColor
        for (identifier, title, width) in [("deck", "Deck", 300.0), ("drivers", "Drivers", 220.0), ("allowed", "Allowed", 90.0)] {
            let column = NSTableColumn(identifier: NSUserInterfaceItemIdentifier(identifier))
            column.title = title
            column.width = width
            table.addTableColumn(column)
        }
        table.dataSource = self
        table.delegate = self
        table.rowHeight = 42
        table.style = .inset
        table.setAccessibilityIdentifier("approvals-table")
        let scroll = NSScrollView()
        scroll.documentView = table
        scroll.hasVerticalScroller = true
        scroll.heightAnchor.constraint(equalToConstant: 300).isActive = true
        for button in [revokeButton, revealButton] {
            button.bezelStyle = .rounded
            button.target = self
            button.isEnabled = false
        }
        revokeButton.action = #selector(revokePressed(_:))
        revokeButton.setAccessibilityIdentifier("approvals-revoke")
        revealButton.action = #selector(revealPressed(_:))
        revealButton.setAccessibilityIdentifier("approvals-reveal")
        let buttons = NSStackView(views: [revokeButton, revealButton])
        buttons.spacing = 8
        errorLabel.font = .systemFont(ofSize: 12)
        errorLabel.textColor = .systemRed
        errorLabel.isHidden = true
        errorLabel.setAccessibilityIdentifier("approvals-error")
        let stack = NSStackView(views: [introLabel, scroll, buttons, errorLabel])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 14
        stack.edgeInsets = NSEdgeInsets(top: 20, left: 28, bottom: 20, right: 28)
        stack.widthAnchor.constraint(equalToConstant: 760).isActive = true
        introLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -56).isActive = true
        scroll.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -56).isActive = true
        view = stack
    }

    override func viewWillAppear() {
        super.viewWillAppear()
        reload()
    }

    /// tap approval list --json, again.
    func reload() {
        Task { @MainActor [weak self] in
            let exit = await TapTool.run(["approval", "list", "--json"], timeout: 30)
            guard let self, let list = try? exit.outcome?.result(ApprovalList.self) else { return }
            self.records = list.approvals
            self.table.reloadData()
            self.selectionChanged()
        }
    }

    private var selectedRecord: ApprovalRecord? {
        let row = table.selectedRow
        return records.indices.contains(row) ? records[row] : nil
    }

    private func selectionChanged() {
        revokeButton.isEnabled = selectedRecord != nil
        revealButton.isEnabled = selectedRecord.map { FileManager.default.fileExists(atPath: $0.deck) } ?? false
    }

    @objc private func revokePressed(_ sender: Any?) {
        guard let record = selectedRecord else { return }
        revokeButton.isEnabled = false
        errorLabel.isHidden = true
        Task { @MainActor [weak self] in
            let exit = await TapTool.run(["approval", "revoke", record.deck, "--json"], timeout: 30)
            if case .failed(_, let message)? = exit.outcome {
                self?.errorLabel.stringValue = message
                self?.errorLabel.isHidden = false
            }
            self?.reload()
        }
    }

    @objc private func revealPressed(_ sender: Any?) {
        guard let record = selectedRecord else { return }
        revealInFinder(URL(fileURLWithPath: record.deck))
    }

    func numberOfRows(in tableView: NSTableView) -> Int { records.count }

    func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
        let record = records[row]
        switch tableColumn?.identifier.rawValue {
        case "deck":
            let name = NSTextField(labelWithString: record.deckName)
            name.font = .systemFont(ofSize: 13, weight: .semibold)
            let folder = NSTextField(labelWithString: (record.folderPath as NSString).abbreviatingWithTildeInPath)
            folder.font = .systemFont(ofSize: 11)
            folder.textColor = .secondaryLabelColor
            let stack = NSStackView(views: [name, folder])
            stack.orientation = .vertical
            stack.alignment = .leading
            stack.spacing = 1
            return stack
        case "drivers":
            let label = NSTextField(labelWithString: record.driverSummary)
            label.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
            label.toolTip = record.commands.map { commands in commands.map { "\($0.key): \($0.value.joined(separator: " "))" }.sorted().joined(separator: "\n") }
            return label
        default:
            let label = NSTextField(labelWithString: record.approvedAtDate.map { Self.dateFormatter.localizedString(for: $0, relativeTo: Date()) } ?? record.approvedAt)
            label.textColor = .secondaryLabelColor
            return label
        }
    }

    func tableViewSelectionDidChange(_ notification: Notification) { selectionChanged() }
}
