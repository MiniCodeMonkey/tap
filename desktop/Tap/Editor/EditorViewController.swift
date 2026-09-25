import AppKit

/// The left pane: bars at the top, and the editor, whose text scrolls under
/// the unified toolbar.
final class EditorViewController: NSViewController {
    let textView = EditorTextView.make()
    let scrollView = NSScrollView()
    let barStack = NSStackView()

    override func loadView() {
        let root = NSView()
        scrollView.documentView = textView
        scrollView.hasVerticalScroller = true
        scrollView.drawsBackground = true
        scrollView.backgroundColor = EditorPalette.editorBackground
        scrollView.automaticallyAdjustsContentInsets = false
        barStack.orientation = .vertical
        barStack.spacing = 0
        barStack.alignment = .leading
        for view in [scrollView, barStack] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            root.addSubview(view)
        }
        NSLayoutConstraint.activate([
            scrollView.topAnchor.constraint(equalTo: root.topAnchor),
            scrollView.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            scrollView.trailingAnchor.constraint(equalTo: root.trailingAnchor),
            scrollView.bottomAnchor.constraint(equalTo: root.bottomAnchor),
            barStack.topAnchor.constraint(equalTo: root.safeAreaLayoutGuide.topAnchor),
            barStack.leadingAnchor.constraint(equalTo: root.leadingAnchor),
            barStack.trailingAnchor.constraint(equalTo: root.trailingAnchor),
        ])
        view = root
    }

    override func viewDidLayout() {
        super.viewDidLayout()
        updateInsets()
    }

    /// The text starts below the toolbar and any bars, and scrolls under them.
    func updateInsets() {
        let barsHeight = barStack.arrangedSubviews.isEmpty ? 0 : barStack.fittingSize.height
        let top = view.safeAreaInsets.top + barsHeight
        if scrollView.contentInsets.top != top {
            scrollView.contentInsets = NSEdgeInsets(top: top, left: 0, bottom: 0, right: 0)
        }
    }
}
