import AppKit

/// The toolbar's Play control: a play segment and a chevron segment. A
/// click on Play starts the talk (Shift for the beginning), and holding it
/// or clicking the chevron opens the menu of ways to play.
final class PlaySplitControl: NSSegmentedControl {
    static let playSegmentWidth: CGFloat = 34
    static let chevronSegmentWidth: CGFloat = 20

    /// A click on the play segment, with the modifiers held at the release.
    var onPlay: ((NSEvent.ModifierFlags) -> Void)?
    /// Builds the menu each time it opens, so its titles name the cursor's slide as it is then.
    var makeMenu: (() -> NSMenu)?
    var holdDelay: TimeInterval = 0.35
    private var holdWork: DispatchWorkItem?
    private var held = false

    override init(frame: NSRect) {
        super.init(frame: frame)
        segmentCount = 2
        trackingMode = .momentary
        segmentStyle = .automatic
        setImage(NSImage(systemSymbolName: "play.fill", accessibilityDescription: "Play"), forSegment: 0)
        let chevron = NSImage(systemSymbolName: "chevron.down", accessibilityDescription: "Play options")?
            .withSymbolConfiguration(NSImage.SymbolConfiguration(pointSize: 8, weight: .bold))
        setImage(chevron, forSegment: 1)
        setWidth(Self.playSegmentWidth, forSegment: 0)
        setWidth(Self.chevronSegmentWidth, forSegment: 1)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    /// The segment under a point of this view's own coordinates.
    func segment(at point: NSPoint) -> Int {
        point.x < Self.playSegmentWidth ? 0 : 1
    }

    override func mouseDown(with event: NSEvent) {
        guard isEnabled else { return }
        held = false
        let segment = segment(at: convert(event.locationInWindow, from: nil))
        setSelected(true, forSegment: segment)
        guard segment == 0 else {
            showMenu()
            setSelected(false, forSegment: segment)
            return
        }
        let work = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated {
                guard let self else { return }
                self.held = true
                self.setSelected(false, forSegment: 0)
                self.showMenu()
            }
        }
        holdWork = work
        DispatchQueue.main.asyncAfter(deadline: .now() + holdDelay, execute: work)
    }

    override func mouseUp(with event: NSEvent) {
        holdWork?.cancel()
        holdWork = nil
        setSelected(false, forSegment: 0)
        let point = convert(event.locationInWindow, from: nil)
        if !held, segment(at: point) == 0, bounds.contains(point) { onPlay?(event.modifierFlags) }
    }

    /// Opens the menu under the control.
    func showMenu() {
        guard let menu = makeMenu?() else { return }
        menu.popUp(positioning: nil, at: NSPoint(x: bounds.minX, y: isFlipped ? bounds.maxY + 4 : bounds.minY - 4), in: self)
    }
}
