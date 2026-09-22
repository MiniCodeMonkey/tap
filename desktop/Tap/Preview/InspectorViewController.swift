import AppKit

/// The right pane, with Preview and Deck tabs. The Deck tab arrives with
/// `tap deck schema`; until then its segment is disabled.
final class InspectorViewController: NSViewController {
    let tabs = NSSegmentedControl(labels: ["Preview", "Deck"], trackingMode: .selectOne, target: nil, action: nil)
    let contentView = NSView()

    override func loadView() {
        let root = NSView()
        root.wantsLayer = true
        root.layer?.backgroundColor = EditorPalette.dynamic(light: NSColor(red: 0.969, green: 0.969, blue: 0.973, alpha: 1),
                                                            dark: NSColor(white: 0.13, alpha: 1)).cgColor
        tabs.selectedSegment = 0
        tabs.setEnabled(false, forSegment: 1)
        tabs.setWidth(90, forSegment: 0)
        tabs.setWidth(90, forSegment: 1)
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
}
