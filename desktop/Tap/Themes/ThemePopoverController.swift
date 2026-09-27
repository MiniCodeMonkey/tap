import AppKit

/// The theme grid in a popover, for the toolbar's Theme item and the
/// Deck tab's Theme row. A pick closes the popover and reports the slug.
@MainActor
final class ThemePopoverController {
    let grid = ThemeGridViewController(cellSize: ThemeGridViewController.popoverCellSize)
    private let popover = NSPopover()
    var onPick: ((String) -> Void)?

    init() {
        popover.behavior = .transient
        popover.contentViewController = grid
        grid.onPick = { [weak self] slug in
            self?.popover.performClose(nil)
            self?.onPick?(slug)
        }
    }

    var isShown: Bool { popover.isShown }

    func show(relativeTo rect: NSRect, of view: NSView, selected: String?) {
        grid.selectedSlug = selected
        popover.show(relativeTo: rect, of: view, preferredEdge: .maxY)
    }

    func close() { popover.performClose(nil) }
}
