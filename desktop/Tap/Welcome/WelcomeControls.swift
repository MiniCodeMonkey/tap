import AppKit

/// One of the welcome window's two big buttons: a rounded rectangle with the
/// title on the left and its shortcut on the right. `.primary` is solid
/// (black in light mode, white in dark, as on tap.sh) and `.glass` is the
/// translucent secondary button. It is a real NSButton, so Return fires the
/// default one and the focus ring is the system's.
final class WelcomeButton: NSButton {
    enum Style { case primary, glass }

    static let cornerRadius: CGFloat = 10
    private static let horizontalPadding: CGFloat = 18
    private static let verticalPadding: CGFloat = 10
    private static let gapBeforeHint: CGFloat = 12

    let style: Style
    let hint: String
    private var isHovered = false
    private var hoverArea: NSTrackingArea?

    init(title: String, hint: String, style: Style) {
        self.style = style
        self.hint = hint
        super.init(frame: .zero)
        self.title = title
        isBordered = false
        setButtonType(.momentaryPushIn)
        focusRingType = .exterior
        wantsLayer = true
        setAccessibilityHelp("Shortcut \(hint)")
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    private var titleAttributes: [NSAttributedString.Key: Any] {
        [.font: WelcomeFont.display(size: 14, weight: .semibold), .foregroundColor: style == .primary ? WelcomeColor.primaryText : WelcomeColor.ink]
    }

    private var hintAttributes: [NSAttributedString.Key: Any] {
        [.font: WelcomeFont.mono(size: 11, weight: .medium), .foregroundColor: style == .primary ? WelcomeColor.primaryHint : WelcomeColor.hint]
    }

    override var intrinsicContentSize: NSSize {
        let titleSize = (title as NSString).size(withAttributes: titleAttributes)
        let hintSize = (hint as NSString).size(withAttributes: hintAttributes)
        return NSSize(width: ceil(Self.horizontalPadding * 2 + titleSize.width + Self.gapBeforeHint + hintSize.width),
                      height: ceil(titleSize.height + Self.verticalPadding * 2 + 2))
    }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let hoverArea { removeTrackingArea(hoverArea) }
        let area = NSTrackingArea(rect: .zero, options: [.mouseEnteredAndExited, .activeInActiveApp, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        hoverArea = area
    }

    override func mouseEntered(with event: NSEvent) { isHovered = true; needsDisplay = true }
    override func mouseExited(with event: NSEvent) { isHovered = false; needsDisplay = true }

    override var isFlipped: Bool { true }

    override func draw(_ dirtyRect: NSRect) {
        let outline = bounds.insetBy(dx: 0.5, dy: 0.5)
        let path = NSBezierPath(roundedRect: outline, xRadius: Self.cornerRadius, yRadius: Self.cornerRadius)
        effectiveAppearance.performAsCurrentDrawingAppearance {
            let pressed = isHighlighted
            switch style {
            case .primary:
                WelcomeColor.primaryFill.withAlphaComponent(pressed ? 0.85 : 1).setFill()
                path.fill()
            case .glass:
                WelcomeColor.glassButtonFill.setFill()
                path.fill()
                if pressed || isHovered {
                    (pressed ? NSColor(white: 0, alpha: 0.06) : NSColor(white: 0.5, alpha: 0.06)).setFill()
                    path.fill()
                }
                WelcomeColor.glassButtonLine.setStroke()
                path.lineWidth = 1
                path.stroke()
            }
            let titleSize = (title as NSString).size(withAttributes: titleAttributes)
            let hintSize = (hint as NSString).size(withAttributes: hintAttributes)
            (title as NSString).draw(at: NSPoint(x: Self.horizontalPadding, y: (bounds.height - titleSize.height) / 2), withAttributes: titleAttributes)
            (hint as NSString).draw(at: NSPoint(x: bounds.width - Self.horizontalPadding - hintSize.width, y: (bounds.height - hintSize.height) / 2), withAttributes: hintAttributes)
        }
    }

    override func drawFocusRingMask() {
        NSBezierPath(roundedRect: bounds, xRadius: Self.cornerRadius, yRadius: Self.cornerRadius).fill()
    }

    override var focusRingMaskBounds: NSRect { bounds }
}

/// "take the theme tour": ink text with a green underline that turns ink on hover.
final class WelcomeLinkButton: NSButton {
    private var isHovered = false
    private var hoverArea: NSTrackingArea?

    init(title: String) {
        super.init(frame: .zero)
        self.title = title
        isBordered = false
        setButtonType(.momentaryPushIn)
        focusRingType = .exterior
        setAccessibilityRole(.link)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    private var attributes: [NSAttributedString.Key: Any] {
        [.font: WelcomeFont.display(size: 13, weight: .medium), .foregroundColor: WelcomeColor.ink]
    }

    override var isFlipped: Bool { true }

    override var intrinsicContentSize: NSSize {
        let size = (title as NSString).size(withAttributes: attributes)
        return NSSize(width: ceil(size.width) + 2, height: ceil(size.height) + 6)
    }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let hoverArea { removeTrackingArea(hoverArea) }
        let area = NSTrackingArea(rect: .zero, options: [.mouseEnteredAndExited, .activeInActiveApp, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        hoverArea = area
    }

    override func mouseEntered(with event: NSEvent) { isHovered = true; needsDisplay = true }
    override func mouseExited(with event: NSEvent) { isHovered = false; needsDisplay = true }

    override func draw(_ dirtyRect: NSRect) {
        effectiveAppearance.performAsCurrentDrawingAppearance {
            let size = (title as NSString).size(withAttributes: attributes)
            let origin = NSPoint(x: 1, y: 0)
            (title as NSString).draw(at: origin, withAttributes: attributes)
            (isHovered ? WelcomeColor.ink : WelcomeColor.linkLine).setFill()
            NSRect(x: origin.x, y: origin.y + size.height + 1, width: size.width, height: 1.5).fill()
        }
    }

    override func drawFocusRingMask() {
        NSBezierPath(roundedRect: bounds, xRadius: 3, yRadius: 3).fill()
    }

    override var focusRingMaskBounds: NSRect { bounds }
}

/// Text that is plain label text in the mockup's type.
func welcomeLabel(_ text: String, size: CGFloat, weight: NSFont.Weight, color: NSColor, display: Bool = true, tracking: CGFloat = 0, alignment: NSTextAlignment = .center) -> NSTextField {
    let field = NSTextField(labelWithString: text)
    field.attributedStringValue = display
        ? WelcomeFont.attributed(text, size: size, weight: weight, color: color, trackingEm: tracking, alignment: alignment)
        : NSAttributedString(string: text, attributes: [.font: NSFont.systemFont(ofSize: size, weight: weight), .foregroundColor: color])
    field.isSelectable = false
    return field
}
