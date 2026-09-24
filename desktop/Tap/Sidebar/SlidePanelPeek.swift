import Foundation

/// When the peeked panel shows and hides. It shows a moment after the
/// pointer reaches the toolbar button, stays while the pointer is over the
/// button or the panel, and hides a moment after it has left both.
@MainActor
final class SlidePanelPeek {
    var showDelay: TimeInterval = 0.15
    var hideDelay: TimeInterval = 0.25
    /// False while the panel is pinned: hovering then does nothing.
    var isEnabled = true {
        didSet { if !isEnabled { hideNow() } }
    }
    var onShow: (() -> Void)?
    var onHide: (() -> Void)?
    private(set) var isShowing = false
    private var overButton = false
    private var overPanel = false
    private var pending: DispatchWorkItem?

    func pointerEnteredButton() {
        overButton = true
        guard isEnabled, !isShowing else { return }
        schedule(after: showDelay) { [weak self] in
            guard let self else { return }
            self.isShowing = true
            self.onShow?()
        }
    }

    func pointerLeftButton() {
        overButton = false
        scheduleHide()
    }

    func pointerEnteredPanel() {
        overPanel = true
        pending?.cancel()
    }

    func pointerLeftPanel() {
        overPanel = false
        scheduleHide()
    }

    func hideNow() {
        pending?.cancel()
        pending = nil
        guard isShowing else { return }
        isShowing = false
        onHide?()
    }

    private func scheduleHide() {
        guard !overButton, !overPanel else { return }
        schedule(after: hideDelay) { [weak self] in
            guard let self else { return }
            self.hideNow()
        }
    }

    private func schedule(after delay: TimeInterval, _ work: @escaping () -> Void) {
        pending?.cancel()
        let item = DispatchWorkItem { MainActor.assumeIsolated { work() } }
        pending = item
        DispatchQueue.main.asyncAfter(deadline: .now() + delay, execute: item)
    }
}
