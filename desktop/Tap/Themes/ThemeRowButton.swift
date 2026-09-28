import AppKit

/// The Deck tab's Theme row: the theme's render and name in one button
/// that opens the theme popover. The render is the loader's PNG, the same
/// one the grid shows, scaled down.
final class ThemeRowButton: NSButton {
    private(set) var slug: String?
    let swatchView = NSImageView()
    let nameLabel = NSTextField(labelWithString: "")

    init() {
        super.init(frame: .zero)
        title = ""
        bezelStyle = .rounded
        swatchView.wantsLayer = true
        swatchView.layer?.cornerRadius = 3
        swatchView.layer?.masksToBounds = true
        swatchView.layer?.backgroundColor = NSColor.quaternaryLabelColor.cgColor
        swatchView.imageScaling = .scaleProportionallyUpOrDown
        nameLabel.font = .systemFont(ofSize: 13)
        let chevron = NSImageView(image: NSImage(systemSymbolName: "chevron.down", accessibilityDescription: nil) ?? NSImage())
        let stack = NSStackView(views: [swatchView, nameLabel, chevron])
        stack.spacing = 8
        stack.edgeInsets = NSEdgeInsets(top: 0, left: 4, bottom: 0, right: 6)
        stack.translatesAutoresizingMaskIntoConstraints = false
        addSubview(stack)
        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: topAnchor), stack.bottomAnchor.constraint(equalTo: bottomAnchor),
            stack.leadingAnchor.constraint(equalTo: leadingAnchor), stack.trailingAnchor.constraint(equalTo: trailingAnchor),
            swatchView.widthAnchor.constraint(equalToConstant: 34), swatchView.heightAnchor.constraint(equalToConstant: 19),
        ])
        setAccessibilityIdentifier("deck-field-theme")
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func hitTest(_ point: NSPoint) -> NSView? {
        bounds.contains(convert(point, from: superview)) ? self : nil
    }

    func show(slug: String?, name: String, image: NSImage?) {
        self.slug = slug
        nameLabel.stringValue = name
        swatchView.image = image
        setAccessibilityLabel("Theme, \(name)")
    }
}
