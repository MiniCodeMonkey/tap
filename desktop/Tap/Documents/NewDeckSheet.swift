import AppKit

/// What the New Deck sheet asks tap for.
struct NewDeckRequest: Equatable {
    var title: String
    var theme: String?
    var location: URL

    /// tap new makes the folder, the deck and images/, and approves the deck.
    var arguments: [String] {
        var arguments = ["new", "--yes", "--title", title]
        if let theme { arguments += ["--theme", theme] }
        arguments += ["--folder", location.path, "--json"]
        return arguments
    }
}

/// File > New Deck: a title, where to save, and the theme grid. Create
/// runs tap new; tap's error, if any, shows in the sheet and the sheet
/// stays for another try.
final class NewDeckSheet: QuestionSheet, NSTextFieldDelegate {
    let titleField = NSTextField(string: "")
    let locationPopup = NSPopUpButton(frame: .zero, pullsDown: false)
    let locationHint = NSTextField(labelWithString: "Creates a folder named after the title, with the deck and images/")
    let grid = ThemeGridViewController(cellSize: ThemeGridViewController.sheetCellSize)
    let errorLabel = NSTextField(wrappingLabelWithString: "")
    var onCreate: ((NewDeckRequest) -> Void)?
    /// Opens a folder chooser (an NSOpenPanel in production; a test answers at once).
    var chooseFolder: (@escaping (URL?) -> Void) -> Void = { completion in
        let panel = NSOpenPanel()
        panel.canChooseDirectories = true
        panel.canChooseFiles = false
        panel.canCreateDirectories = true
        panel.prompt = "Choose"
        panel.begin { response in completion(response == .OK ? panel.url : nil) }
    }
    private var locations: [URL] = []
    private var chosenLocation: URL

    var createButton: NSButton { acceptButton }
    var cancelButton: NSButton { declineButton }

    /// The Default cell means no --theme: tap new picks its own default.
    var request: NewDeckRequest {
        let theme = grid.selectedSlug.flatMap { $0 == ThemeGridViewController.defaultSlug ? nil : $0 }
        return NewDeckRequest(title: titleField.stringValue.trimmingCharacters(in: .whitespaces), theme: theme, location: chosenLocation)
    }

    init(lastFolder: URL?, defaultTheme: String?) {
        let home = FileManager.default.homeDirectoryForCurrentUser
        let documents = FileManager.default.urls(for: .documentDirectory, in: .userDomainMask).first ?? home.appendingPathComponent("Documents")
        let desktop = FileManager.default.urls(for: .desktopDirectory, in: .userDomainMask).first ?? home.appendingPathComponent("Desktop")
        var locations = [documents, desktop]
        if let lastFolder, !locations.contains(where: { FilePaths.same($0, lastFolder) }) { locations.insert(lastFolder, at: 0) }
        chosenLocation = lastFolder ?? documents
        self.locations = locations
        let form = NSView()
        super.init(kind: "new-deck", title: "New Deck", body: "", path: nil, decline: "Cancel", accept: "Create", escape: .decline, returnAnswer: .accept, detail: form, width: 560)
        bodyLabel.isHidden = true
        // Create is this sheet's own action: the sheet stays up on a failure.
        acceptButton.target = self
        acceptButton.action = #selector(createPressed(_:))
        titleField.placeholderString = "My Presentation"
        titleField.delegate = self
        titleField.setAccessibilityIdentifier("new-deck-title")
        for location in locations { addLocationItem(location, at: locationPopup.numberOfItems) }
        locationPopup.menu?.addItem(.separator())
        locationPopup.addItem(withTitle: "Other…")
        locationPopup.selectItem(at: locations.firstIndex { FilePaths.same($0, chosenLocation) } ?? 0)
        locationPopup.target = self
        locationPopup.action = #selector(locationChanged(_:))
        locationPopup.setAccessibilityIdentifier("new-deck-location")
        locationHint.font = .systemFont(ofSize: 11)
        locationHint.textColor = .secondaryLabelColor
        grid.showsFooter = false
        grid.showsSections = false
        grid.visibleRows = 2.5
        grid.fillsWidth = true
        grid.selectedSlug = defaultTheme
        errorLabel.textColor = .systemRed
        errorLabel.font = .systemFont(ofSize: 12)
        errorLabel.isHidden = true
        errorLabel.setAccessibilityIdentifier("new-deck-error")

        // The NewDeck board: Title and Save in share one card, a hairline between the rows.
        let titleRow = labeledRow("Title", titleField)
        let locationRow = labeledRow("Save in", locationPopup, hint: locationHint)
        let hairline = NSBox()
        hairline.boxType = .separator
        let rows = NSStackView(views: [titleRow, hairline, locationRow])
        rows.orientation = .vertical
        rows.alignment = .leading
        rows.spacing = 10
        rows.edgeInsets = NSEdgeInsets(top: 10, left: 12, bottom: 10, right: 12)
        rows.translatesAutoresizingMaskIntoConstraints = false
        let card = NSBox()
        card.boxType = .custom
        card.titlePosition = .noTitle
        card.fillColor = .controlBackgroundColor
        card.borderColor = .separatorColor
        card.borderWidth = 0.5
        card.cornerRadius = 8
        card.contentViewMargins = .zero
        card.contentView?.addSubview(rows)
        let themeHeading = NSTextField(labelWithString: "Theme")
        themeHeading.font = .systemFont(ofSize: 11, weight: .semibold)
        themeHeading.textColor = .secondaryLabelColor
        let stack = NSStackView(views: [card, themeHeading, grid.view, errorLabel])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 10
        stack.translatesAutoresizingMaskIntoConstraints = false
        form.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: form.topAnchor), stack.bottomAnchor.constraint(equalTo: form.bottomAnchor),
            stack.leadingAnchor.constraint(equalTo: form.leadingAnchor), stack.trailingAnchor.constraint(equalTo: form.trailingAnchor),
            card.widthAnchor.constraint(equalTo: stack.widthAnchor), grid.view.widthAnchor.constraint(equalTo: stack.widthAnchor), titleField.widthAnchor.constraint(equalToConstant: 300),
            rows.topAnchor.constraint(equalTo: card.topAnchor), rows.bottomAnchor.constraint(equalTo: card.bottomAnchor),
            rows.leadingAnchor.constraint(equalTo: card.leadingAnchor), rows.trailingAnchor.constraint(equalTo: card.trailingAnchor),
            titleRow.widthAnchor.constraint(equalTo: rows.widthAnchor, constant: -24), locationRow.widthAnchor.constraint(equalTo: titleRow.widthAnchor),
            hairline.widthAnchor.constraint(equalTo: titleRow.widthAnchor),
            errorLabel.widthAnchor.constraint(equalTo: stack.widthAnchor),
        ])
        setContentSize(contentView?.fittingSize ?? frame.size)
        titleChanged(titleField)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    /// A folder in the Save in popup, with the folder icon the board draws.
    private func addLocationItem(_ location: URL, at index: Int) {
        locationPopup.insertItem(withTitle: location.lastPathComponent, at: index)
        let icon = NSImage(systemSymbolName: "folder", accessibilityDescription: nil)
        icon?.size = NSSize(width: 14, height: 12)
        locationPopup.item(at: index)?.image = icon
    }

    private func labeledRow(_ title: String, _ control: NSView, hint: NSTextField? = nil) -> NSView {
        let label = NSTextField(labelWithString: title)
        label.font = .systemFont(ofSize: 13)
        var left: [NSView] = [label]
        if let hint { left.append(hint) }
        let labels = NSStackView(views: left)
        labels.orientation = .vertical
        labels.alignment = .leading
        labels.spacing = 2
        let row = NSStackView(views: [labels, NSView(), control])
        row.orientation = .horizontal
        row.alignment = .centerY
        row.spacing = 12
        return row
    }

    @objc func titleChanged(_ sender: Any?) {
        createButton.isEnabled = !request.title.isEmpty
    }

    func controlTextDidChange(_ notification: Notification) { titleChanged(notification.object) }

    @objc func locationChanged(_ sender: Any?) {
        let index = locationPopup.indexOfSelectedItem
        if index < locations.count {
            chosenLocation = locations[index]
            return
        }
        chooseFolder { [weak self] chosen in
            guard let self else { return }
            if let chosen {
                if !self.locations.contains(where: { FilePaths.same($0, chosen) }) {
                    self.locations.insert(chosen, at: 0)
                    self.addLocationItem(chosen, at: 0)
                }
                self.chosenLocation = chosen
            }
            self.locationPopup.selectItem(at: self.locations.firstIndex { FilePaths.same($0, self.chosenLocation) } ?? 0)
        }
    }

    @objc private func createPressed(_ sender: Any?) {
        onCreate?(request)
    }

    /// tap's message, in the sheet; Create is enabled again for another try.
    func showError(_ message: String) {
        errorLabel.stringValue = message
        errorLabel.isHidden = false
        createButton.isEnabled = true
        cancelButton.isEnabled = true
    }

    /// While tap new runs, neither Create nor Cancel can be pressed: the
    /// deck tap is writing opens when it lands.
    func beginCreating() {
        errorLabel.isHidden = true
        createButton.isEnabled = false
        cancelButton.isEnabled = false
    }
}
