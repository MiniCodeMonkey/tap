import AppKit
import TapDesktopCore

/// A slide before its picture exists: its heading in bold on the theme's
/// paper colour, two faint text bars, and a slow sheen that sweeps across.
/// It stands in for a thumbnail in the slide panel and, at full size, for
/// the preview until its first paint.
final class SlideCardView: NSView {
    static let sheenDuration: CFTimeInterval = 1.8
    static let sheenAnimationKey = "sheen"

    let headingLabel = NSTextField(labelWithString: "")
    private let firstBar = CALayer()
    private let secondBar = CALayer()
    private let sheen = CAGradientLayer()
    private(set) var card = SlideCard(heading: "")
    private(set) var paper = PaperColour.neutral
    /// True while the sheen runs: it is wanted, Reduce Motion is off and the card is in a window.
    var isSheening: Bool { sheen.animation(forKey: Self.sheenAnimationKey) != nil }
    private var wantsSheen = false

    override init(frame: NSRect) {
        super.init(frame: frame)
        wantsLayer = true
        layer?.masksToBounds = true
        headingLabel.lineBreakMode = .byTruncatingTail
        headingLabel.maximumNumberOfLines = 3
        headingLabel.cell?.wraps = true
        headingLabel.cell?.truncatesLastVisibleLine = true
        addSubview(headingLabel)
        layer?.addSublayer(firstBar)
        layer?.addSublayer(secondBar)
        sheen.startPoint = CGPoint(x: 0, y: 0.5)
        sheen.endPoint = CGPoint(x: 1, y: 0.5)
        sheen.colors = [NSColor.white.withAlphaComponent(0).cgColor, NSColor.white.withAlphaComponent(0.4).cgColor, NSColor.white.withAlphaComponent(0).cgColor]
        sheen.isHidden = true
        layer?.addSublayer(sheen)
        setAccessibilityElement(false)
        setAccessibilityIdentifier("slide-card")
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    func configure(card: SlideCard, paper: PaperColour) {
        self.card = card
        self.paper = paper
        headingLabel.stringValue = card.heading
        layer?.backgroundColor = Self.cgColor(paper)
        let ink = paper.isLight ? NSColor(white: 0.1, alpha: 1) : NSColor.white
        headingLabel.textColor = ink
        for bar in [firstBar, secondBar] { bar.backgroundColor = ink.withAlphaComponent(0.12).cgColor }
        needsLayout = true
    }

    private static func cgColor(_ paper: PaperColour) -> CGColor {
        NSColor(srgbRed: paper.red, green: paper.green, blue: paper.blue, alpha: 1).cgColor
    }

    override func layout() {
        super.layout()
        let width = bounds.width
        let height = bounds.height
        guard width > 0 else { return }
        let inset = width * 0.09
        headingLabel.font = .systemFont(ofSize: max(8, width * 0.075), weight: .bold)
        let textWidth = width - inset * 2
        let fitted = headingLabel.sizeThatFits(NSSize(width: textWidth, height: height))
        headingLabel.preferredMaxLayoutWidth = textWidth
        let headingHeight = min(fitted.height, height * 0.5)
        // Top left: AppKit's y grows upward, so the heading sits at the top edge minus its height.
        headingLabel.frame = NSRect(x: inset, y: height - inset * 0.8 - headingHeight, width: textWidth, height: headingHeight)
        let barHeight = max(3, height * 0.05)
        let barTop = headingLabel.frame.minY - barHeight * 1.6
        CATransaction.begin()
        CATransaction.setDisableActions(true)
        firstBar.frame = CGRect(x: inset, y: barTop, width: textWidth * 0.78, height: barHeight)
        secondBar.frame = CGRect(x: inset, y: barTop - barHeight * 1.9, width: textWidth * 0.5, height: barHeight)
        firstBar.cornerRadius = barHeight / 2
        secondBar.cornerRadius = barHeight / 2
        sheen.frame = CGRect(x: -width * 0.5, y: 0, width: width * 0.5, height: height)
        CATransaction.commit()
        if wantsSheen { restartSheen() }
    }

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        if wantsSheen { restartSheen() }
    }

    /// Runs the sheen unless Reduce Motion is on.
    func startSheen() {
        wantsSheen = true
        restartSheen()
    }

    func stopSheen() {
        wantsSheen = false
        sheen.removeAnimation(forKey: Self.sheenAnimationKey)
        sheen.isHidden = true
    }

    private func restartSheen() {
        sheen.removeAnimation(forKey: Self.sheenAnimationKey)
        guard wantsSheen, window != nil, bounds.width > 0, !WelcomeMotion.reduceMotion() else {
            sheen.isHidden = true
            return
        }
        sheen.isHidden = false
        let sweep = CABasicAnimation(keyPath: "position.x")
        sweep.fromValue = -bounds.width * 0.25
        sweep.toValue = bounds.width * 1.25
        sweep.duration = Self.sheenDuration
        sweep.repeatCount = .infinity
        sweep.timingFunction = CAMediaTimingFunction(name: .easeInEaseOut)
        sheen.add(sweep, forKey: Self.sheenAnimationKey)
    }
}

/// A 2 pt line along the top edge of the preview: a short accent segment
/// that travels across it, over and over. With Reduce Motion on it stays
/// still, as a faint full-width line.
final class IndeterminateProgressLine: NSView {
    static let height: CGFloat = 2
    static let travelAnimationKey = "travel"

    private let segment = CALayer()
    private(set) var isRunning = false
    /// True while the segment travels: the line runs, Reduce Motion is off and it is in a window.
    var isTravelling: Bool { segment.animation(forKey: Self.travelAnimationKey) != nil }

    override init(frame: NSRect) {
        super.init(frame: frame)
        wantsLayer = true
        layer?.masksToBounds = true
        layer?.addSublayer(segment)
        setAccessibilityElement(false)
        setAccessibilityIdentifier("preview-progress-line")
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func updateLayer() {
        segment.backgroundColor = NSColor.controlAccentColor.cgColor
    }

    override var wantsUpdateLayer: Bool { true }

    override func layout() {
        super.layout()
        if isRunning { restart() }
    }

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        if isRunning { restart() }
    }

    func start() {
        isRunning = true
        restart()
    }

    func stop() {
        isRunning = false
        segment.removeAnimation(forKey: Self.travelAnimationKey)
    }

    private func restart() {
        segment.removeAnimation(forKey: Self.travelAnimationKey)
        let width = bounds.width
        CATransaction.begin()
        CATransaction.setDisableActions(true)
        defer { CATransaction.commit() }
        guard !WelcomeMotion.reduceMotion() else {
            segment.frame = CGRect(x: 0, y: 0, width: width, height: bounds.height)
            segment.opacity = 0.35
            return
        }
        segment.opacity = 1
        guard window != nil, width > 0 else { return }
        segment.cornerRadius = bounds.height / 2
        segment.frame = CGRect(x: 0, y: 0, width: width * 0.35, height: bounds.height)
        let travel = CABasicAnimation(keyPath: "position.x")
        travel.fromValue = -width * 0.175
        travel.toValue = width * 1.175
        travel.duration = 1.3
        travel.repeatCount = .infinity
        travel.timingFunction = CAMediaTimingFunction(name: .easeInEaseOut)
        segment.add(travel, forKey: Self.travelAnimationKey)
    }
}

extension PaperColour {
    /// The dominant colour along the edges of a theme's thumbnail, where
    /// the theme's background shows and its content rarely reaches.
    static func sampled(from image: NSImage) -> PaperColour? {
        var rect = CGRect(origin: .zero, size: image.size)
        guard let cgImage = image.cgImage(forProposedRect: &rect, context: nil, hints: nil) else { return nil }
        let width = 48
        let height = 27
        var pixels = [UInt8](repeating: 0, count: width * height * 4)
        let drawn = pixels.withUnsafeMutableBytes { buffer -> Bool in
            guard let context = CGContext(data: buffer.baseAddress, width: width, height: height, bitsPerComponent: 8, bytesPerRow: width * 4,
                                          space: CGColorSpace(name: CGColorSpace.sRGB)!,
                                          bitmapInfo: CGImageAlphaInfo.premultipliedLast.rawValue) else { return false }
            context.draw(cgImage, in: CGRect(x: 0, y: 0, width: width, height: height))
            return true
        }
        guard drawn else { return nil }
        var samples: [PaperColour] = []
        for y in 0..<height {
            for x in 0..<width where x < 3 || x >= width - 3 || y < 3 || y >= height - 3 {
                let offset = (y * width + x) * 4
                guard pixels[offset + 3] == 255 else { continue }
                samples.append(PaperColour(red: Double(pixels[offset]) / 255, green: Double(pixels[offset + 1]) / 255, blue: Double(pixels[offset + 2]) / 255))
            }
        }
        return PaperColour.dominant(of: samples)
    }
}

extension NSView {
    /// Fades a layer-backed view out: its model opacity is 0 at once and a
    /// Core Animation fade from where it was covers the change, so the
    /// state is true the moment this returns. `completion` runs after
    /// `duration`.
    func fadeOut(duration: TimeInterval, completion: @escaping @MainActor () -> Void) {
        if let layer {
            let fade = CABasicAnimation(keyPath: "opacity")
            fade.fromValue = layer.presentation()?.opacity ?? layer.opacity
            fade.toValue = 0
            fade.duration = duration
            layer.add(fade, forKey: "fadeOut")
        }
        alphaValue = 0
        DispatchQueue.main.asyncAfter(deadline: .now() + duration) {
            MainActor.assumeIsolated { completion() }
        }
    }
}
