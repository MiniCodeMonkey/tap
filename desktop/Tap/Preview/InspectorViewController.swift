import AppKit

/// The right pane, with Preview and Deck tabs. The Deck tab arrives with
/// `tap deck schema`; until then its segment is disabled. One child is
/// shown at a time; the other is hidden, not removed, so the preview's
/// page keeps its state. The tab and the availability are stored state
/// that `loadView` applies, since a deck opened after the schema loaded
/// hears of it before this view exists.
final class InspectorViewController: NSViewController {
    enum Tab: Int {
        case preview = 0
        case deck = 1
    }

    let tabs = NSSegmentedControl(labels: ["Preview", "Deck"], trackingMode: .selectOne, target: nil, action: nil)
    let contentView = NSView()
    private(set) var selectedTab: Tab = .preview
    private(set) var isDeckTabAvailable = false
    private var previewChild: NSViewController?
    private var deckChild: NSViewController?
    /// Runs after the tab changes, whichever way.
    var onTabChange: ((Tab) -> Void)?

    override func loadView() {
        let root = NSView()
        root.wantsLayer = true
        root.layer?.backgroundColor = EditorPalette.dynamic(light: NSColor(red: 0.969, green: 0.969, blue: 0.973, alpha: 1),
                                                            dark: NSColor(white: 0.13, alpha: 1)).cgColor
        tabs.selectedSegment = selectedTab.rawValue
        tabs.setEnabled(isDeckTabAvailable, forSegment: Tab.deck.rawValue)
        tabs.setWidth(90, forSegment: 0)
        tabs.setWidth(90, forSegment: 1)
        tabs.target = self
        tabs.action = #selector(tabChanged(_:))
        tabs.setAccessibilityIdentifier("inspector-tabs")
        for view in [tabs, contentView] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(view)
        }
        NSLayoutConstraint.activate([
            tabs.topAnchor.constraint(equalTo: root.safeAreaLayoutGuide.topAnchor, constant: 12),
            tabs.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: 20),
            contentView.topAnchor.constraint(equalTo: tabs.bottomAnchor, constant: 12),
            contentView.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            contentView.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            contentView.bottomAnchor.constraint(equalTo: root.bottomAnchor),
        ])
        view = root
    }

    /// Shows `child` as the Preview tab's content.
    func embed(_ child: NSViewController) {
        previewChild = child
        place(child)
        child.view.isHidden = selectedTab != .preview
    }

    /// Shows `child` as the Deck tab's content.
    func embedDeck(_ child: NSViewController) {
        deckChild = child
        place(child)
        child.view.isHidden = selectedTab != .deck
    }

    /// The preview moved to its own window (View > Preview in Window): it
    /// is nobody's tab until it docks back, and it shows there whatever
    /// tab is selected here.
    func previewDetached() {
        previewChild?.view.isHidden = false
        previewChild = nil
    }

    private func place(_ child: NSViewController) {
        addChild(child)
        child.view.translatesAutoresizingMaskIntoConstraints = false
        contentView.addSubview(child.view)
        NSLayoutConstraint.activate([
            child.view.topAnchor.constraint(equalTo: contentView.topAnchor),
            child.view.leadingAnchor.constraint(equalTo: contentView.leadingAnchor),
            child.view.trailingAnchor.constraint(equalTo: contentView.trailingAnchor),
            child.view.bottomAnchor.constraint(equalTo: contentView.bottomAnchor),
        ])
    }

    /// The Deck segment is disabled until tap's schema has loaded.
    func setDeckTabAvailable(_ available: Bool) {
        isDeckTabAvailable = available
        if isViewLoaded { tabs.setEnabled(available, forSegment: Tab.deck.rawValue) }
        if !available, selectedTab == .deck { showTab(.preview) }
    }

    func showTab(_ tab: Tab) {
        selectedTab = tab
        if isViewLoaded { tabs.selectedSegment = tab.rawValue }
        // Only a child that is still in this pane follows the tab.
        if let previewChild, previewChild.parent === self { previewChild.view.isHidden = tab != .preview }
        if let deckChild, deckChild.parent === self { deckChild.view.isHidden = tab != .deck }
        onTabChange?(tab)
    }

    @objc private func tabChanged(_ sender: NSSegmentedControl) {
        showTab(Tab(rawValue: sender.selectedSegment) ?? .preview)
    }
}
