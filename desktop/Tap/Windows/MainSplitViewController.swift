import AppKit

/// The editor and the right pane, split 50/50. The app owns the divider: it
/// returns to the middle after every layout change until the user drags it.
final class MainSplitViewController: NSSplitViewController {
    let editorItem: NSSplitViewItem
    let inspectorItem: NSSplitViewItem
    private(set) var dividerPolicy = DividerPolicy()
    private var isBalancing = false

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

    override func viewDidLoad() {
        super.viewDidLoad()
        splitView.dividerStyle = .thin
    }

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

    override func splitViewDidResizeSubviews(_ notification: Notification) {
        super.splitViewDidResizeSubviews(notification)
        if !isBalancing, notification.userInfo?["NSSplitViewDividerIndex"] != nil {
            dividerPolicy.userDragged()
        }
    }
}
