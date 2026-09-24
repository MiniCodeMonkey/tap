import AppKit

/// The sidebar split item's controller: an empty view the slide panel's
/// view is placed in while the panel is pinned.
final class SidebarHostViewController: NSViewController {
    override func loadView() {
        view = NSView()
    }

    func host(_ panel: NSView) {
        panel.removeFromSuperview()
        panel.translatesAutoresizingMaskIntoConstraints = false
        view.addSubview(panel)
        NSLayoutConstraint.activate([
            panel.topAnchor.constraint(equalTo: view.safeAreaLayoutGuide.topAnchor),
            panel.leadingAnchor.constraint(equalTo: view.leadingAnchor),
            panel.trailingAnchor.constraint(equalTo: view.trailingAnchor),
            panel.bottomAnchor.constraint(equalTo: view.bottomAnchor),
        ])
    }
}
