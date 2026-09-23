import AppKit

/// The editor and the right pane, split 50/50. The app owns the divider: it
/// returns to the middle after every layout change until the user drags it.
final class MainSplitViewController: NSSplitViewController {
    let editorItem: NSSplitViewItem
    let inspectorItem: NSSplitViewItem
    private(set) var dividerPolicy = DividerPolicy()
    private var isBalancing = false
    private var isPersonDraggingTheDivider = false
    private var dividerMouseMonitor: Any?

    init(editor: NSViewController, inspector: NSViewController) {
        editorItem = NSSplitViewItem(viewController: editor)
        editorItem.minimumThickness = 320
        inspectorItem = NSSplitViewItem(viewController: inspector)
        inspectorItem.minimumThickness = 320
        inspectorItem.canCollapse = true
        super.init(nibName: nil, bundle: nil)
        addSplitViewItem(editorItem)
        addSplitViewItem(inspectorItem)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    deinit {
        if let dividerMouseMonitor {
            NSEvent.removeMonitor(dividerMouseMonitor)
        }
    }

    override func viewDidLoad() {
        super.viewDidLoad()
        splitView.dividerStyle = .thin
        // A person's drag begins with a mouseDown that lands on the divider
        // itself, which the split view draws directly rather than either
        // pane owning it, so hit-testing a mouseDown against the split view
        // tells a real drag apart from any other resize AppKit performs on
        // our behalf, such as enforcing a minimum thickness.
        dividerMouseMonitor = NSEvent.addLocalMonitorForEvents(matching: [.leftMouseDown, .leftMouseUp]) { [weak self] event in
            self?.noteDividerMouseEvent(event)
            return event
        }
    }

    private func noteDividerMouseEvent(_ event: NSEvent) {
        guard event.window === view.window else { return }
        if event.type == .leftMouseUp {
            isPersonDraggingTheDivider = false
            return
        }
        guard let pointInSuperview = splitView.superview?.convert(event.locationInWindow, from: nil) else {
            isPersonDraggingTheDivider = false
            return
        }
        isPersonDraggingTheDivider = splitView.hitTest(pointInSuperview) === splitView
    }

    var isPreviewHidden: Bool { inspectorItem.isCollapsed }

    override func viewDidLayout() {
        super.viewDidLayout()
        balance()
    }

    func balance() {
        guard !inspectorItem.isCollapsed,
              let position = dividerPolicy.balancedPosition(totalWidth: splitView.bounds.width, dividerThickness: splitView.dividerThickness),
              abs(editorItem.viewController.view.frame.width - position) > 0.5 else { return }
        isBalancing = true
        splitView.setPosition(position, ofDividerAt: 0)
        isBalancing = false
    }

    /// Hides the right pane, so the editor takes the full width, or shows it again at 50/50.
    func setPreviewHidden(_ hidden: Bool) {
        inspectorItem.isCollapsed = hidden
        if !hidden {
            dividerPolicy.reset()
            view.needsLayout = true
            view.layoutSubtreeIfNeeded()
            balance()
        }
    }

    /// Records a drag of the divider. After it, the divider stays where the user put it.
    func userDidDragDivider() {
        dividerPolicy.userDragged()
    }

    override func splitViewDidResizeSubviews(_ notification: Notification) {
        super.splitViewDidResizeSubviews(notification)
        if !isBalancing, isPersonDraggingTheDivider, notification.userInfo?["NSSplitViewDividerIndex"] != nil {
            userDidDragDivider()
        }
    }
}
