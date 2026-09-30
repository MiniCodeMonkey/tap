import AppKit

/// The app icon at the top of the welcome window: the icon without its caret
/// (the `WelcomeIconBase` image, drawn from the same script as the app icon)
/// with a caret layer on top that blinks, unless Reduce Motion is on. Where
/// that image is missing it shows `NSApp.applicationIconImage`, caret and all.
final class WelcomeIconView: NSView {
    /// The squircle's edge in points; the image is `squircleSize / (824/1024)` wide, the icon grid's margin included.
    let squircleSize: CGFloat
    private let imageLayer = CALayer()
    private let glowLayer = CALayer()
    private let caretLayer = CALayer()
    private let hasBaseImage: Bool
    private(set) var blinkIsRunning = false

    /// The caret's place in the 96 point design: 4 x 17 points, 11 points into a 69 x 39 slide at the center.
    private static let designSize: CGFloat = 96
    private static let caretRect = CGRect(x: 48 - 34.5 + 11, y: 48 - 8.5, width: 4, height: 17)

    init(squircleSize: CGFloat) {
        self.squircleSize = squircleSize
        let base = NSImage(named: "WelcomeIconBase")
        hasBaseImage = base != nil
        super.init(frame: .zero)
        wantsLayer = true
        setAccessibilityElement(true)
        setAccessibilityRole(.image)
        setAccessibilityLabel("Tap")

        glowLayer.cornerRadius = squircleSize * 0.23
        glowLayer.shadowOffset = CGSize(width: 0, height: -squircleSize * 0.19)
        glowLayer.shadowRadius = squircleSize * 0.18
        glowLayer.shadowOpacity = 1
        layer?.addSublayer(glowLayer)

        let image = base ?? NSApp.applicationIconImage
        imageLayer.contents = image?.cgImage(forProposedRect: nil, context: nil, hints: nil)
        imageLayer.contentsGravity = .resizeAspect
        imageLayer.magnificationFilter = .linear
        layer?.addSublayer(imageLayer)

        caretLayer.backgroundColor = WelcomeColor.caret.cgColor
        caretLayer.shadowColor = NSColor(hex: 0x7fd18c).cgColor
        caretLayer.shadowOpacity = 0.9
        caretLayer.shadowOffset = .zero
        caretLayer.isHidden = !hasBaseImage
        layer?.addSublayer(caretLayer)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    /// The icon grid puts the squircle 824 of every 1024 points, so the image is larger than the squircle.
    var imageSize: CGFloat { squircleSize * 1024 / 824 }

    override var intrinsicContentSize: NSSize { NSSize(width: imageSize, height: imageSize) }

    override func layout() {
        super.layout()
        let squircle = CGRect(x: (bounds.width - squircleSize) / 2, y: (bounds.height - squircleSize) / 2, width: squircleSize, height: squircleSize)
        imageLayer.frame = CGRect(x: (bounds.width - imageSize) / 2, y: (bounds.height - imageSize) / 2, width: imageSize, height: imageSize)
        glowLayer.frame = squircle
        glowLayer.shadowPath = CGPath(roundedRect: CGRect(origin: .zero, size: squircle.size).insetBy(dx: squircleSize * 0.06, dy: squircleSize * 0.06), cornerWidth: squircleSize * 0.2, cornerHeight: squircleSize * 0.2, transform: nil)
        let unit = squircleSize / Self.designSize
        // The layer's y axis points up; the design's points down.
        caretLayer.frame = CGRect(x: squircle.minX + Self.caretRect.minX * unit,
                                  y: squircle.minY + (Self.designSize - Self.caretRect.maxY) * unit,
                                  width: Self.caretRect.width * unit, height: Self.caretRect.height * unit)
        caretLayer.cornerRadius = 2 * unit
        caretLayer.shadowRadius = 6 * unit
        refreshColors()
        updateBlink()
    }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        refreshColors()
    }

    private func refreshColors() {
        effectiveAppearance.performAsCurrentDrawingAppearance {
            glowLayer.shadowColor = WelcomeColor.iconGlow.cgColor
            glowLayer.backgroundColor = NSColor.clear.cgColor
        }
    }

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        updateBlink()
    }

    /// Blinks the caret (a 1.1 s step, as the mockup does) unless Reduce Motion is on.
    func updateBlink() {
        caretLayer.removeAnimation(forKey: "blink")
        blinkIsRunning = false
        guard hasBaseImage, window != nil, !WelcomeMotion.reduceMotion() else { return }
        let blink = CAKeyframeAnimation(keyPath: "opacity")
        blink.values = [1, 0]
        blink.keyTimes = [0, 0.5]
        blink.calculationMode = .discrete
        blink.duration = 1.1
        blink.repeatCount = .infinity
        blink.isRemovedOnCompletion = false
        caretLayer.add(blink, forKey: "blink")
        blinkIsRunning = true
    }

    /// Stops the blink while the window cannot be seen.
    func setBlinking(_ isVisible: Bool) {
        if isVisible { updateBlink() } else {
            caretLayer.removeAnimation(forKey: "blink")
            blinkIsRunning = false
        }
    }
}
