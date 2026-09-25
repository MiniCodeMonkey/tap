import AppKit

/// The toolbar's New Slide button: a click inserts a slide with the last
/// layout, holding it opens the layout gallery.
final class NewSlideButton: NSButton {
    var onClick: (() -> Void)?
    var onHold: (() -> Void)?
    var holdDelay: TimeInterval = 0.35
    private var holdWork: DispatchWorkItem?
    private var held = false

    override func mouseDown(with event: NSEvent) {
        held = false
        highlight(true)
        let work = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated {
                guard let self else { return }
                self.held = true
                self.highlight(false)
                self.onHold?()
            }
        }
        holdWork = work
        DispatchQueue.main.asyncAfter(deadline: .now() + holdDelay, execute: work)
    }

    override func mouseUp(with event: NSEvent) {
        holdWork?.cancel()
        holdWork = nil
        highlight(false)
        if !held, bounds.contains(convert(event.locationInWindow, from: nil)) { onClick?() }
    }
}
