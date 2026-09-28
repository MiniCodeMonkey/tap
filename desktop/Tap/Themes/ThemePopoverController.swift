import AppKit

/// The theme grid in a popover, for the toolbar's Theme item and the
/// Deck tab's Theme row. A pick closes the popover and reports the slug.
@MainActor
final class ThemePopoverController: NSObject, NSPopoverDelegate {
    let grid = ThemeGridViewController(cellSize: ThemeGridViewController.popoverCellSize)
    private let popover = NSPopover()
    var onPick: ((String) -> Void)?
    /// Kept by this controller rather than read from the popover, whose
    /// `isShown` depends on whether the app is active and on the close
    /// animation, which differ between this machine and the CI runner (the
    /// layout gallery and the Present popover keep theirs the same way).
    /// True from `show` until a pick, `close` or AppKit's own close.
    private(set) var isShown = false

    override init() {
        super.init()
        popover.behavior = .transient
        popover.contentViewController = grid
        popover.delegate = self
        grid.onPick = { [weak self] slug in
            self?.close()
            self?.onPick?(slug)
        }
    }

    func show(relativeTo rect: NSRect, of view: NSView, selected: String?) {
        grid.selectedSlug = selected
        popover.show(relativeTo: rect, of: view, preferredEdge: .maxY)
        isShown = true
    }

    func close() {
        isShown = false
        popover.performClose(nil)
    }

    func popoverDidClose(_ notification: Notification) {
        isShown = false
    }
}
