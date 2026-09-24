import AppKit

/// The editor and the right pane, split 50/50. The app owns the divider: it
/// returns to the middle after every layout change until the user drags it.
final class MainSplitViewController: NSSplitViewController {
    let sidebarItem: NSSplitViewItem
    let editorItem: NSSplitViewItem
    let inspectorItem: NSSplitViewItem
    private(set) var dividerPolicy = DividerPolicy()
    private var isBalancing = false
    private var isPersonDraggingTheDivider = false
    private var dividerMouseMonitor: Any?
    /// The sidebar's width: the panel plus the stock sidebar's inset.
    static let sidebarWidth = SlidePanelViewController.width + 24

    init(sidebar: NSViewController, editor: NSViewController, inspector: NSViewController) {
        sidebarItem = NSSplitViewItem(sidebarWithViewController: sidebar)
        sidebarItem.minimumThickness = Self.sidebarWidth
        sidebarItem.maximumThickness = Self.sidebarWidth
        sidebarItem.canCollapse = true
        // Collapsing or expanding the sidebar moves the editor and the right
        // pane, never the window: the pinned panel pushes them to the right.
        sidebarItem.collapseBehavior = .preferResizingSiblingsWithFixedSplitView
        editorItem = NSSplitViewItem(viewController: editor)
        editorItem.minimumThickness = 320
        inspectorItem = NSSplitViewItem(viewController: inspector)
        inspectorItem.minimumThickness = 320
        inspectorItem.canCollapse = true
        super.init(nibName: nil, bundle: nil)
        addSplitViewItem(sidebarItem)
        addSplitViewItem(editorItem)
        addSplitViewItem(inspectorItem)
    }

    var isSidebarCollapsed: Bool { sidebarItem.isCollapsed }

    func setSidebarCollapsed(_ collapsed: Bool) {
        guard sidebarItem.isCollapsed != collapsed else { return }
        sidebarItem.isCollapsed = collapsed
        view.needsLayout = true
        view.layoutSubtreeIfNeeded()
        balance()
    }

    /// The divider between the editor and the right pane, which sits after
    /// the sidebar's own divider.
    private var editorDividerIndex: Int { 1 }
    /// How many times `balance()` has actually moved the divider. Internal,
    /// for a test to prove an already-balanced layout does not move it
    /// again, not for anything production code reads.
    private(set) var setPositionCallCount = 0

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
        guard !inspectorItem.isCollapsed else { return }
        // A sidebar-style split item's own column, as arranged by the split
        // view, can be a few points wider than its view controller's own
        // view (AppKit adds a fixed margin around a sidebar's content); the
        // arranged subview's width is what actually crowds the editor, so
        // that is what is read here rather than the view controller's view.
        let arrangedSubviews = splitView.arrangedSubviews
        let leadingWidth = sidebarItem.isCollapsed ? 0 : (arrangedSubviews.first?.frame.width ?? sidebarItem.viewController.view.frame.width)
        // The editor's own view is wrapped by the SDK for safe-area
        // propagation, so its frame is relative to that wrapper, not to the
        // split view: its origin.x reads as 0 whatever the sidebar's width,
        // which would make this guard compare a split-view offset against a
        // value that can never match while the sidebar shows. The editor's
        // arranged subview (the split view's own direct child) is already
        // in the split view's coordinate space, so its maxX is what is
        // compared here instead.
        let editorMaxX = arrangedSubviews.count > editorDividerIndex ? arrangedSubviews[editorDividerIndex].frame.maxX : editorItem.viewController.view.frame.maxX
        guard let position = dividerPolicy.balancedPosition(totalWidth: splitView.bounds.width, dividerThickness: splitView.dividerThickness,
                                                            leadingWidth: leadingWidth),
              abs(editorMaxX - position) > 0.5 else { return }
        isBalancing = true
        splitView.setPosition(position, ofDividerAt: editorDividerIndex)
        setPositionCallCount += 1
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
