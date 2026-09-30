import AppKit
import QuartzCore
import TapDesktopCore

/// One theme in the welcome window's filmstrip: its thumbnail, which lifts
/// when the pointer or keyboard focus is on it and shows the theme's name in
/// a glass pill. Clicking it, or Space or Return on it, sends its action.
final class WelcomeThemeCard: NSButton {
    static let imageSize = NSSize(width: FilmstripLoop.cardWidth, height: FilmstripLoop.cardWidth * 9 / 16)

    let slug: String
    let themeName: String
    /// Called when focus arrives or leaves, so the strip can pause.
    var onFocusChange: (() -> Void)?
    private(set) var isLifted = false
    private(set) var hasKeyboardFocus = false

    private let liftLayer = CALayer()
    private let shadowLayer = CALayer()
    private let imageLayer = CALayer()
    private let ringLayer = CALayer()
    private let pillLayer = CALayer()
    private let pillText = CATextLayer()
    private var hoverArea: NSTrackingArea?

    init(theme: ThemeSummary, image: NSImage?) {
        slug = theme.slug
        themeName = theme.name
        super.init(frame: NSRect(origin: .zero, size: Self.imageSize))
        title = ""
        isBordered = false
        setButtonType(.momentaryChange)
        focusRingType = .none
        wantsLayer = true
        setAccessibilityLabel("\(theme.name) theme")
        setAccessibilityRole(.button)

        liftLayer.anchorPoint = CGPoint(x: 0.5, y: 0.5)
        layer?.addSublayer(liftLayer)
        shadowLayer.shadowOpacity = 0.3
        shadowLayer.shadowRadius = 9
        shadowLayer.shadowOffset = CGSize(width: 0, height: 6)
        shadowLayer.backgroundColor = NSColor.gray.cgColor
        liftLayer.addSublayer(shadowLayer)
        imageLayer.cornerRadius = 9
        imageLayer.masksToBounds = true
        imageLayer.borderWidth = 1
        imageLayer.contentsGravity = .resizeAspectFill
        imageLayer.magnificationFilter = .linear
        imageLayer.minificationFilter = .trilinear
        liftLayer.addSublayer(imageLayer)
        ringLayer.cornerRadius = 12
        ringLayer.borderWidth = 3
        ringLayer.isHidden = true
        liftLayer.addSublayer(ringLayer)
        pillLayer.cornerRadius = 11
        pillLayer.borderWidth = 1
        pillLayer.opacity = 0
        pillText.alignmentMode = .center
        pillText.truncationMode = .none
        pillLayer.addSublayer(pillText)
        liftLayer.addSublayer(pillLayer)
        show(image)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    func show(_ image: NSImage?) {
        imageLayer.contents = image?.cgImage(forProposedRect: nil, context: nil, hints: nil)
        imageLayer.backgroundColor = image == nil ? NSColor.quaternaryLabelColor.cgColor : nil
    }

    // MARK: Layout

    /// Its layers use a top-left origin, like the strip that holds it.
    override var isFlipped: Bool { true }

    override func layout() {
        super.layout()
        CATransaction.begin()
        CATransaction.setDisableActions(true)
        let size = bounds.size
        liftLayer.bounds = CGRect(origin: .zero, size: size)
        liftLayer.position = CGPoint(x: bounds.midX, y: bounds.midY)
        for layer in [shadowLayer, imageLayer] { layer.frame = liftLayer.bounds }
        shadowLayer.cornerRadius = 9
        shadowLayer.shadowPath = CGPath(roundedRect: liftLayer.bounds, cornerWidth: 9, cornerHeight: 9, transform: nil)
        ringLayer.frame = liftLayer.bounds.insetBy(dx: -3, dy: -3)
        let scale = window?.backingScaleFactor ?? 2
        pillText.contentsScale = scale
        let font = WelcomeFont.display(size: 12, weight: .semibold)
        pillText.font = font
        pillText.fontSize = 12
        pillText.string = themeName
        let textWidth = ceil((themeName as NSString).size(withAttributes: [.font: font]).width)
        let pillSize = CGSize(width: textWidth + 18, height: 22)
        // The pill sits 6 points below the image.
        pillLayer.frame = CGRect(x: (size.width - pillSize.width) / 2, y: size.height + 6, width: pillSize.width, height: pillSize.height)
        pillText.frame = CGRect(x: 0, y: (pillSize.height - 15) / 2 + 0.5, width: pillSize.width, height: 15)
        refreshColors()
        CATransaction.commit()
    }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        refreshColors()
    }

    private func refreshColors() {
        effectiveAppearance.performAsCurrentDrawingAppearance {
            imageLayer.borderColor = WelcomeColor.glassLine.cgColor
            ringLayer.borderColor = NSColor.keyboardFocusIndicatorColor.cgColor
            pillLayer.backgroundColor = WelcomeColor.glass.cgColor
            pillLayer.borderColor = WelcomeColor.glassLine.cgColor
            pillText.foregroundColor = WelcomeColor.ink.cgColor
        }
    }

    // MARK: Lift

    private func setLifted(_ lifted: Bool) {
        guard lifted != isLifted else { return }
        isLifted = lifted
        let animated = !WelcomeMotion.reduceMotion()
        CATransaction.begin()
        CATransaction.setDisableActions(!animated)
        CATransaction.setAnimationDuration(0.25)
        CATransaction.setAnimationTimingFunction(CAMediaTimingFunction(controlPoints: 0.2, 0.8, 0.2, 1))
        liftLayer.transform = lifted ? CATransform3DTranslate(CATransform3DMakeScale(1.04, 1.04, 1), 0, -8, 0) : CATransform3DIdentity
        pillLayer.opacity = lifted ? 1 : 0
        CATransaction.commit()
    }

    private func updateLift() { setLifted(hasKeyboardFocus || isPointerInside) }

    private var isPointerInside = false

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let hoverArea { removeTrackingArea(hoverArea) }
        let area = NSTrackingArea(rect: .zero, options: [.mouseEnteredAndExited, .activeInActiveApp, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        hoverArea = area
    }

    override func mouseEntered(with event: NSEvent) { isPointerInside = true; updateLift() }
    override func mouseExited(with event: NSEvent) { isPointerInside = false; updateLift() }

    // MARK: Keyboard

    override func becomeFirstResponder() -> Bool {
        let accepted = super.becomeFirstResponder()
        if accepted {
            hasKeyboardFocus = true
            ringLayer.isHidden = false
            updateLift()
            onFocusChange?()
        }
        return accepted
    }

    override func resignFirstResponder() -> Bool {
        let resigned = super.resignFirstResponder()
        if resigned {
            hasKeyboardFocus = false
            ringLayer.isHidden = true
            updateLift()
            onFocusChange?()
        }
        return resigned
    }

    override func keyDown(with event: NSEvent) {
        // Space is the button's own key; Return and Enter do the same.
        if event.keyCode == 36 || event.keyCode == 76 { performClick(nil) } else { super.keyDown(with: event) }
    }
}

/// The row of every theme along the bottom of the first-launch window. It
/// drifts left, wraps without a seam, and stops while the pointer or focus is
/// on it. With Reduce Motion it holds still and scrolls by hand.
final class WelcomeFilmstrip: FlippedView {
    static let height: CGFloat = 152
    private static let cardTop: CGFloat = 22
    private static let edgeFade: CGFloat = 0.12

    private let scrollView = NSScrollView()
    private let track = FlippedView()
    private let fade = CAGradientLayer()
    /// The first copy of each theme's card: the ones the accessibility tree and Tab reach.
    private(set) var cards: [WelcomeThemeCard] = []
    private var allCards: [WelcomeThemeCard] = []
    private var displayLink: CADisplayLink?
    private var lastTick: CFTimeInterval = 0
    private var hoverArea: NSTrackingArea?
    private var themes: [ThemeSummary] = []
    private var imageProvider: ((String) -> NSImage?)?

    /// Called with the slug of the card that was pressed.
    var onSelect: ((String) -> Void)?
    private(set) var isPointerInside = false
    /// True while the window is on screen; the strip drifts only then.
    private(set) var isActive = false
    private(set) var reduceMotion = false

    private var loop: FilmstripLoop { FilmstripLoop(cardCount: themes.count) }
    /// True while the row is moving.
    var isDrifting: Bool { displayLink != nil }
    /// The scroll offset of the row.
    var offset: CGFloat { scrollView.contentView.bounds.origin.x }

    override init(frame frameRect: NSRect) {
        super.init(frame: frameRect)
        wantsLayer = true
        scrollView.drawsBackground = false
        scrollView.hasHorizontalScroller = false
        scrollView.hasVerticalScroller = false
        scrollView.horizontalScrollElasticity = .allowed
        scrollView.verticalScrollElasticity = .none
        scrollView.autoresizingMask = [.width, .height]
        scrollView.documentView = track
        scrollView.contentView.drawsBackground = false
        addSubview(scrollView)
        fade.startPoint = CGPoint(x: 0, y: 0.5)
        fade.endPoint = CGPoint(x: 1, y: 0.5)
        fade.colors = [NSColor.clear.cgColor, NSColor.black.cgColor, NSColor.black.cgColor, NSColor.clear.cgColor]
        fade.locations = [0, NSNumber(value: Self.edgeFade), NSNumber(value: 1 - Self.edgeFade), 1]
        layer?.mask = fade
        setAccessibilityIdentifier("welcome-themes")
        setAccessibilityRole(.group)
        setAccessibilityLabel("Themes")
        reduceMotion = WelcomeMotion.reduceMotion()
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func layout() {
        super.layout()
        scrollView.frame = bounds
        CATransaction.begin()
        CATransaction.setDisableActions(true)
        fade.frame = bounds
        CATransaction.commit()
        let needed = copyCount()
        if needed != copiesBuilt { rebuild() }
    }

    override func updateTrackingAreas() {
        super.updateTrackingAreas()
        if let hoverArea { removeTrackingArea(hoverArea) }
        let area = NSTrackingArea(rect: .zero, options: [.mouseEnteredAndExited, .activeInActiveApp, .inVisibleRect], owner: self, userInfo: nil)
        addTrackingArea(area)
        hoverArea = area
    }

    override func mouseEntered(with event: NSEvent) { isPointerInside = true; refreshDrift() }
    override func mouseExited(with event: NSEvent) { isPointerInside = false; refreshDrift() }

    // MARK: Cards

    private var copiesBuilt = 0

    private func copyCount() -> Int {
        guard !themes.isEmpty else { return 0 }
        return reduceMotion ? 1 : loop.copiesNeeded(visibleWidth: Double(bounds.width))
    }

    func setThemes(_ themes: [ThemeSummary], image: @escaping (String) -> NSImage?) {
        self.themes = themes
        imageProvider = image
        rebuild()
    }

    func show(_ image: NSImage?, forSlug slug: String) {
        for card in allCards where card.slug == slug { card.show(image) }
    }

    private func rebuild() {
        for card in allCards { card.removeFromSuperview() }
        allCards = []
        cards = []
        let copies = copyCount()
        copiesBuilt = copies
        let pitch = CGFloat(loop.pitch)
        for copy in 0..<copies {
            for (index, theme) in themes.enumerated() {
                let card = WelcomeThemeCard(theme: theme, image: imageProvider?(theme.slug))
                card.frame.origin = NSPoint(x: CGFloat(copy * themes.count + index) * pitch, y: Self.cardTop)
                card.target = self
                card.action = #selector(cardPressed(_:))
                card.onFocusChange = { [weak self] in self?.focusChanged() }
                if copy > 0 {
                    // The repeat exists only to make the loop seamless.
                    card.refusesFirstResponder = true
                    card.setAccessibilityElement(false)
                } else {
                    cards.append(card)
                }
                track.addSubview(card)
                allCards.append(card)
            }
        }
        track.frame = NSRect(x: 0, y: 0, width: max(CGFloat(copies * themes.count) * pitch - CGFloat(FilmstripLoop.gap), bounds.width), height: Self.height)
        scrollView.contentView.scroll(to: NSPoint(x: min(offset, max(0, track.frame.width - bounds.width)), y: 0))
        scrollView.reflectScrolledClipView(scrollView.contentView)
        refreshDrift()
    }

    @objc private func cardPressed(_ sender: WelcomeThemeCard) { onSelect?(sender.slug) }

    private func focusChanged() {
        if let card = cards.first(where: \.hasKeyboardFocus) {
            // A focused card that is off the edge is brought in.
            card.scrollToVisible(card.bounds.insetBy(dx: -24, dy: 0))
        }
        refreshDrift()
    }

    // MARK: Motion

    /// The window is on screen or not.
    func setActive(_ active: Bool) {
        isActive = active
        refreshDrift()
    }

    /// Re-reads Reduce Motion: the row is rebuilt with or without its repeat, and starts or stops.
    func refreshMotionSetting() {
        let now = WelcomeMotion.reduceMotion()
        guard now != reduceMotion else { return refreshDrift() }
        reduceMotion = now
        rebuild()
    }

    private var shouldDrift: Bool {
        isActive && !reduceMotion && !isPointerInside && !cards.contains(where: \.hasKeyboardFocus) && !themes.isEmpty
    }

    private func refreshDrift() {
        if shouldDrift, displayLink == nil {
            let link = displayLink(target: self, selector: #selector(tick(_:)))
            link.preferredFrameRateRange = CAFrameRateRange(minimum: 30, maximum: 60, preferred: 60)
            lastTick = CACurrentMediaTime()
            link.add(to: .main, forMode: .common)
            displayLink = link
        } else if !shouldDrift, let link = displayLink {
            link.invalidate()
            displayLink = nil
        }
    }

    @objc private func tick(_ link: CADisplayLink) {
        let now = CACurrentMediaTime()
        let delta = min(now - lastTick, 0.1)
        lastTick = now
        scrollTo(offset: loop.advance(Double(offset), bySeconds: delta))
    }

    /// Moves the row to `offset`, as a tick does.
    func scrollTo(offset: Double) {
        let clip = scrollView.contentView
        clip.setBoundsOrigin(NSPoint(x: offset, y: 0))
        scrollView.reflectScrolledClipView(clip)
    }

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        if window == nil { setActive(false) }
    }

    deinit { displayLink?.invalidate() }
}
