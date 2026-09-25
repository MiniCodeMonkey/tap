import AppKit

/// The deck window's content: the split view controller's view filling
/// it, and the peek overlay laid over it as a sibling. The overlay must
/// not be inside the split view, which lays out every subview it has as a
/// pane.
final class DeckContentViewController: NSViewController {
    let splitViewController: MainSplitViewController
    let overlay: SlidePanelOverlay

    init(splitViewController: MainSplitViewController, overlay: SlidePanelOverlay) {
        self.splitViewController = splitViewController
        self.overlay = overlay
        super.init(nibName: nil, bundle: nil)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func loadView() {
        let root = NSView()
        addChild(splitViewController)
        for view in [splitViewController.view, overlay] {
            view.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(view)
        }
        NSLayoutConstraint.activate([
            splitViewController.view.topAnchor.constraint(equalTo: root.topAnchor),
            splitViewController.view.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            splitViewController.view.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            splitViewController.view.bottomAnchor.constraint(equalTo: root.bottomAnchor),
            overlay.leadingAnchor.constraint(equalTo: root.leadingAnchor, constant: 12),
            overlay.topAnchor.constraint(equalTo: root.safeAreaLayoutGuide.topAnchor, constant: 8),
            overlay.bottomAnchor.constraint(equalTo: root.bottomAnchor, constant: -12),
            overlay.widthAnchor.constraint(equalToConstant: SlidePanelViewController.width),
        ])
        view = root
    }
}
