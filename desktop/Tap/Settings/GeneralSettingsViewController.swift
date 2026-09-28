import AppKit

/// General: the editor's font size and line spacing, the theme New Deck
/// preselects, and the autosave delay, in GeneralSettings. Each group
/// ("Editor", "New decks", "Saving") is a heading over a FormCard, the
/// Deck tab's own card: its rows are pinned by constraints, not held in
/// an NSBox's contentView, so a card is exactly as tall as its rows.
final class GeneralSettingsViewController: NSViewController {
    let fontSizePopup = NSPopUpButton(frame: .zero, pullsDown: false)
    let lineSpacingControl = NSSegmentedControl(labels: ["Tight", "Normal", "Roomy"], trackingMode: .selectOne, target: nil, action: nil)
    let defaultThemePopup = NSPopUpButton(frame: .zero, pullsDown: false)
    let autosavePopup = NSPopUpButton(frame: .zero, pullsDown: false)
    static let noDefaultTheme = "tap's default"
    private var catalogObserver: NSObjectProtocol?

    /// Every card of the pane, top to bottom (a layout test's seam).
    var cards: [FormCard] {
        func cards(in view: NSView) -> [FormCard] {
            view.subviews.flatMap { subview in (subview as? FormCard).map { [$0] } ?? cards(in: subview) }
        }
        return cards(in: view)
    }

    override init(nibName: NSNib.Name?, bundle: Bundle?) {
        super.init(nibName: nil, bundle: nil)
        title = "General"
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func loadView() {
        for size in GeneralSettings.fontSizes { fontSizePopup.addItem(withTitle: "\(Int(size)) pt") }
        fontSizePopup.target = self
        fontSizePopup.action = #selector(changed(_:))
        fontSizePopup.setAccessibilityIdentifier("settings-font-size")
        lineSpacingControl.target = self
        lineSpacingControl.action = #selector(changed(_:))
        lineSpacingControl.setAccessibilityIdentifier("settings-line-spacing")
        defaultThemePopup.target = self
        defaultThemePopup.action = #selector(changed(_:))
        defaultThemePopup.setAccessibilityIdentifier("settings-default-theme")
        for delay in GeneralSettings.autosaveDelays { autosavePopup.addItem(withTitle: Self.title(forDelay: delay)) }
        autosavePopup.target = self
        autosavePopup.action = #selector(changed(_:))
        autosavePopup.setAccessibilityIdentifier("settings-autosave")

        let editorCard = FormCard(rows: [
            Self.row(label: "Font size", control: fontSizePopup),
            Self.row(label: "Line spacing", control: lineSpacingControl),
        ])
        let decksCard = FormCard(rows: [
            Self.row(label: "Default theme", hint: "Preselected in New Deck. Passed to tap new --theme.", control: defaultThemePopup),
        ])
        let savingCard = FormCard(rows: [
            Self.row(label: "Autosave after typing stops", control: autosavePopup),
        ])
        editorCard.setAccessibilityIdentifier("settings-card-Editor")
        decksCard.setAccessibilityIdentifier("settings-card-New decks")
        savingCard.setAccessibilityIdentifier("settings-card-Saving")

        // A section per group, in order; Task 13/D7's Updates card only
        // needs one more entry in this array.
        let sections = [
            FormCard.section(title: "Editor", content: [editorCard]),
            FormCard.section(title: "New decks", content: [decksCard]),
            FormCard.section(title: "Saving", content: [savingCard]),
        ]
        let stack = NSStackView(views: sections)
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 20
        stack.edgeInsets = NSEdgeInsets(top: 20, left: 28, bottom: 20, right: 28)
        for section in sections { section.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -56).isActive = true }
        stack.widthAnchor.constraint(equalToConstant: 760).isActive = true
        view = stack
        rebuildThemes()
        catalogObserver = NotificationCenter.default.addObserver(forName: ThemeImageLoader.didLoadCatalogNotification, object: nil, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated { self?.rebuildThemes() }
        }
        // The Default theme popup needs the catalog alone; the renders
        // (and the export engine's download) wait for a grid that shows them.
        AppEnvironment.shared.themeImages.loadCatalog()
        refresh()
    }

    deinit {
        if let catalogObserver { NotificationCenter.default.removeObserver(catalogObserver) }
    }

    static func title(forDelay delay: TimeInterval) -> String {
        delay == 1 ? "1 second" : delay < 1 ? "\(delay) seconds" : "\(Int(delay)) seconds"
    }

    /// A card's row: the label (and its hint, under it) on the left, the
    /// control on the right, as FormCard.row draws every card's row.
    private static func row(label: String, hint: String? = nil, control: NSView) -> NSView {
        let name = NSTextField(labelWithString: label)
        name.font = .systemFont(ofSize: 13)
        var leading: [NSView] = [name]
        if let hint {
            let hintLabel = NSTextField(wrappingLabelWithString: hint)
            hintLabel.font = .systemFont(ofSize: 11)
            hintLabel.textColor = .secondaryLabelColor
            leading.append(hintLabel)
        }
        let labels = NSStackView(views: leading)
        labels.orientation = .vertical
        labels.alignment = .leading
        labels.spacing = 2
        return FormCard.row(leading: [labels], trailing: [control])
    }

    private func rebuildThemes() {
        defaultThemePopup.removeAllItems()
        defaultThemePopup.addItem(withTitle: Self.noDefaultTheme)
        for theme in AppEnvironment.shared.themeImages.catalog?.themes ?? [] {
            defaultThemePopup.addItem(withTitle: theme.name)
            defaultThemePopup.lastItem?.representedObject = theme.slug
        }
        refresh()
    }

    /// The controls from the settings.
    func refresh() {
        let settings = AppEnvironment.shared.generalSettings
        fontSizePopup.selectItem(withTitle: "\(Int(settings.fontSize)) pt")
        lineSpacingControl.selectedSegment = GeneralSettings.LineSpacing.allCases.firstIndex(of: settings.lineSpacing) ?? 1
        autosavePopup.selectItem(withTitle: Self.title(forDelay: settings.autosaveDelay))
        if let slug = settings.defaultTheme, let index = defaultThemePopup.itemArray.firstIndex(where: { $0.representedObject as? String == slug }) {
            defaultThemePopup.selectItem(at: index)
        } else {
            defaultThemePopup.selectItem(at: 0)
        }
    }

    @objc func changed(_ sender: Any?) {
        let settings = AppEnvironment.shared.generalSettings
        switch sender as AnyObject? {
        case let popup as NSPopUpButton where popup === fontSizePopup:
            settings.fontSize = GeneralSettings.fontSizes[max(0, popup.indexOfSelectedItem)]
        case let control as NSSegmentedControl where control === lineSpacingControl:
            settings.lineSpacing = GeneralSettings.LineSpacing.allCases[max(0, control.selectedSegment)]
        case let popup as NSPopUpButton where popup === defaultThemePopup:
            settings.defaultTheme = popup.selectedItem?.representedObject as? String
        case let popup as NSPopUpButton where popup === autosavePopup:
            settings.autosaveDelay = GeneralSettings.autosaveDelays[max(0, popup.indexOfSelectedItem)]
        default:
            break
        }
    }
}
