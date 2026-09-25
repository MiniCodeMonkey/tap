import AppKit

/// The peeked slide panel: a glass overlay at the window's left edge.
/// macOS 26 draws it with the system's glass; 14 and 15 with a popover
/// material inside the window.
final class SlidePanelOverlay: NSView {
    let contentView: NSView
    var onPointerEntered: (() -> Void)?
    var onPointerLeft: (() -> Void)?
    private var trackingArea: NSTrackingArea?

    override init(frame: NSRect) {
        let backing: NSView
        let content: NSView
        if #available(macOS 26.0, *) {
            let glass = NSGlassEffectView()
            glass.cornerRadius = 16
            content = NSView()
            glass.contentView = content
            backing = glass
        } else {
            let material = NSVisualEffectView()
            material.material = .popover
            material.blendingMode = .withinWindow
            material.state = .active
            material.wantsLayer = true
            material.layer?.cornerRadius = 16
            material.layer?.masksToBounds = true
            backing = material
            content = material
        }
        contentView = content
        super.init(frame: frame)
        wantsLayer = true
        shadow = NSShadow()
        layer?.shadowOpacity = 0.22
        layer?.shadowRadius = 25
        layer?.shadowOffset = CGSize(width: 0, height: -18)
        backing.translatesAutoresizingMaskIntoConstraints = false
        addSubview(backing)
        NSLayoutConstraint.activate([
            backing.topAnchor.constraint(equalTo: topAnchor),
            backing.leadingAnchor.constraint(equalTo: leadingAnchor),
            backing.trailingAnchor.constraint(equalTo: trailingAnchor),
            backing.bottomAnchor.constraint(equalTo: bottomAnchor),
        ])
        // An element of its own, a group, so the identifier reaches the
        // accessibility tree; the hosted panel stays inside it.
        setAccessibilityElement(true)
        setAccessibilityRole(.group)
        setAccessibilityIdentifier("slide-panel-overlay")
        isHidden = true
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    func host(_ panel: NSView) {
        panel.removeFromSuperview()
        panel.translatesAutoresizingMaskIntoConstraints = false
        contentView.addSubview(panel)
        NSLayoutConstraint.activate([
            panel.topAnchor.constraint(equalTo: contentView.topAnchor),
            panel.leadingAnchor.constraint(equalTo: contentView.leadingAnchor),
            panel.trailingAnchor.constraint(equalTo: contentView.trailingAnchor),
            panel.bottomAnchor.constraint(equalTo: contentView.bottomAnchor),
        ])
    }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let trackingArea { removeTrackingArea(trackingArea) }
        let area = NSTrackingArea(rect: bounds, options: [.mouseEnteredAndExited, .activeInKeyWindow, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        trackingArea = area
    }

    override func mouseEntered(with event: NSEvent) { onPointerEntered?() }
    override func mouseExited(with event: NSEvent) { onPointerLeft?() }
}
