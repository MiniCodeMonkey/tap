import AppKit

/// One recent deck: a 96 x 54 thumbnail, the file's name in the display
/// font, its folder in SF Mono with a tilde, and a short date. The texts turn
/// white on the system's accent-colored selection, as native rows do.
final class WelcomeRecentCell: NSTableCellView {
    static let identifier = NSUserInterfaceItemIdentifier("recent-deck-cell")
    static let thumbnailSize = NSSize(width: 96, height: 54)
    static let rowHeight: CGFloat = 70

    let thumbnailView = NSImageView()
    let nameField = NSTextField(labelWithString: "")
    let pathField = NSTextField(labelWithString: "")
    let dateField = NSTextField(labelWithString: "")

    override init(frame frameRect: NSRect) {
        super.init(frame: frameRect)
        identifier = Self.identifier
        thumbnailView.imageScaling = .scaleAxesIndependently
        thumbnailView.wantsLayer = true
        thumbnailView.layer?.cornerRadius = 5
        thumbnailView.layer?.masksToBounds = true
        thumbnailView.layer?.borderWidth = 1
        thumbnailView.setAccessibilityElement(false)
        for field in [nameField, pathField, dateField] {
            field.lineBreakMode = .byTruncatingTail
            field.maximumNumberOfLines = 1
            field.isSelectable = false
        }
        nameField.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
        pathField.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
        dateField.setContentHuggingPriority(.required, for: .horizontal)
        dateField.setContentCompressionResistancePriority(.required, for: .horizontal)
        let texts = NSStackView(views: [nameField, pathField])
        texts.orientation = .vertical
        texts.alignment = .leading
        texts.spacing = 2
        for view in [thumbnailView, texts, dateField] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            addSubview(view)
        }
        NSLayoutConstraint.activate([
            thumbnailView.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 4),
            thumbnailView.centerYAnchor.constraint(equalTo: centerYAnchor),
            thumbnailView.widthAnchor.constraint(equalToConstant: Self.thumbnailSize.width),
            thumbnailView.heightAnchor.constraint(equalToConstant: Self.thumbnailSize.height),
            texts.leadingAnchor.constraint(equalTo: thumbnailView.trailingAnchor, constant: 14),
            texts.centerYAnchor.constraint(equalTo: centerYAnchor),
            dateField.leadingAnchor.constraint(greaterThanOrEqualTo: texts.trailingAnchor, constant: 14),
            dateField.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -4),
            dateField.centerYAnchor.constraint(equalTo: centerYAnchor),
        ])
        applyColors()
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    /// Sets the row's content; `tildePath` is the folder, already abbreviated.
    func configure(name: String, tildePath: String, date: String, thumbnail: NSImage?) {
        nameField.stringValue = name
        pathField.stringValue = tildePath
        dateField.stringValue = date
        thumbnailView.image = thumbnail
        toolTip = tildePath + "/" + name
        applyColors()
    }

    override var backgroundStyle: NSView.BackgroundStyle {
        didSet { applyColors() }
    }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        applyColors()
    }

    private func applyColors() {
        let selected = backgroundStyle == .emphasized
        nameField.font = WelcomeFont.display(size: 14, weight: .semibold)
        pathField.font = WelcomeFont.mono(size: 12)
        dateField.font = .monospacedDigitSystemFont(ofSize: 12, weight: .regular)
        nameField.textColor = selected ? .alternateSelectedControlTextColor : WelcomeColor.ink
        pathField.textColor = selected ? .alternateSelectedControlTextColor : WelcomeColor.text3
        dateField.textColor = selected ? .alternateSelectedControlTextColor : WelcomeColor.text3
        effectiveAppearance.performAsCurrentDrawingAppearance {
            thumbnailView.layer?.borderColor = WelcomeColor.line.cgColor
        }
    }
}

/// The recents list. Typing while it has focus goes to the search field,
/// Return opens the selected deck.
final class WelcomeRecentTable: NSTableView {
    var onTypeToSearch: ((String) -> Void)?
    var onOpenSelection: (() -> Void)?

    override func keyDown(with event: NSEvent) {
        switch event.keyCode {
        case 36, 76:
            onOpenSelection?()
        default:
            let flags = event.modifierFlags.intersection([.command, .control, .option])
            if flags.isEmpty, let characters = event.characters, let first = characters.unicodeScalars.first,
               !CharacterSet.controlCharacters.contains(first), first.value < 0xF700 || first.value > 0xF8FF, first != " " {
                onTypeToSearch?(characters)
            } else {
                super.keyDown(with: event)
            }
        }
    }
}

/// The dotted grid behind the hero, faded out toward its edges by an ellipse
/// centered a little above the middle, as the mockup's mask does.
final class WelcomeDotGridView: NSView {
    private static let pitch: CGFloat = 22

    override var isFlipped: Bool { true }
    override func hitTest(_ point: NSPoint) -> NSView? { nil }
    override var isOpaque: Bool { false }

    override init(frame frameRect: NSRect) {
        super.init(frame: frameRect)
        setAccessibilityElement(false)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func draw(_ dirtyRect: NSRect) {
        effectiveAppearance.performAsCurrentDrawingAppearance {
            let base = WelcomeColor.dot.usingColorSpace(.sRGB) ?? WelcomeColor.dot
            let center = NSPoint(x: bounds.midX, y: bounds.height * 0.42)
            let radiusX = bounds.width * 0.6, radiusY = bounds.height * 0.55
            var y = Self.pitch / 2
            while y < bounds.height {
                var x = Self.pitch / 2
                while x < bounds.width {
                    let distance = hypot((x - center.x) / radiusX, (y - center.y) / radiusY)
                    // Opaque to 20% of the radius, transparent at 75%.
                    let strength = min(max((0.75 - distance) / 0.55, 0), 1)
                    if strength > 0 {
                        base.withAlphaComponent(base.alphaComponent * strength).setFill()
                        NSBezierPath(ovalIn: NSRect(x: x - 1, y: y - 1, width: 2, height: 2)).fill()
                    }
                    x += Self.pitch
                }
                y += Self.pitch
            }
        }
    }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        needsDisplay = true
    }
}
