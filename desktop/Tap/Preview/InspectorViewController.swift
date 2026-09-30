import AppKit

/// The right pane: the preview, and nothing else. The deck's settings are
/// the Deck card in the editor.
final class InspectorViewController: NSViewController {
    let contentView = NSView()
    private var previewChild: NSViewController?

    override func loadView() {
        let root = NSView()
        root.wantsLayer = true
        root.layer?.backgroundColor = EditorPalette.dynamic(light: NSColor(red: 0.969, green: 0.969, blue: 0.973, alpha: 1),
                                                            dark: NSColor(white: 0.13, alpha: 1)).cgColor
        contentView.translatesAutoresizingMaskIntoConstraints = false
        root.addSubview(contentView)
        NSLayoutConstraint.activate([
            contentView.topAnchor.constraint(equalTo: root.safeAreaLayoutGuide.topAnchor, constant: 4),
            contentView.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            contentView.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            contentView.bottomAnchor.constraint(equalTo: root.bottomAnchor),
        ])
        view = root
    }

    /// Shows `child` as the pane's content.
    func embed(_ child: NSViewController) {
        previewChild = child
        addChild(child)
        child.view.translatesAutoresizingMaskIntoConstraints = false
        child.view.isHidden = false
        contentView.addSubview(child.view)
        NSLayoutConstraint.activate([
            child.view.topAnchor.constraint(equalTo: contentView.topAnchor),
            child.view.leadingAnchor.constraint(equalTo: contentView.leadingAnchor),
            child.view.trailingAnchor.constraint(equalTo: contentView.trailingAnchor),
            child.view.bottomAnchor.constraint(equalTo: contentView.bottomAnchor),
        ])
    }

    /// The preview moved to its own window (View > Preview in Window): it
    /// is nobody's child until it docks back.
    func previewDetached() {
        previewChild?.view.isHidden = false
        previewChild = nil
    }
}
