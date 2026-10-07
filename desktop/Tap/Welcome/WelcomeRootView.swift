import AppKit
import TapDesktopCore

/// The welcome window's whole view tree, front to back: the drop overlay,
/// the two layouts (first launch: hero and filmstrip; with recents: brand
/// pane and recents list), the dotted grid, the aurora, and the window's
/// background color. It holds no state of its own beyond which layout shows.
final class WelcomeRootView: WelcomeContentView {
    static let windowSize = NSSize(width: 880, height: 560)
    static let brandPaneWidth: CGFloat = 320

    /// The icon, wordmark, tagline and buttons that both layouts show.
    struct BrandBlock {
        let icon: WelcomeIconView
        let wordmark: NSTextField
        let tagline: NSTextField
        let newDeckButton: WelcomeButton
        let openButton: WelcomeButton
    }

    let aurora = AuroraView()
    let dotGrid = WelcomeDotGridView()
    let hero = FlippedView()
    let filmstrip = WelcomeFilmstrip()
    let split = FlippedView()
    let recordingSetup = RecordingSetupView()
    let recentsPane = FlippedView()
    private let hairline = FlippedView()
    let tourLink = WelcomeLinkButton(title: "take the theme tour")

    let emptyStateBrand: BrandBlock
    let columnBrand: BrandBlock
    let recentLabel = welcomeLabel("RECENT", size: 12, weight: .semibold, color: WelcomeColor.text3, tracking: 0.06, alignment: .left)
    let searchField = NSSearchField()
    let tableView = WelcomeRecentTable()
    let recentsScrollView = NSScrollView()
    let noMatchesLabel = welcomeLabel("No decks match", size: 13, weight: .regular, color: WelcomeColor.text3, display: false)

    private var heroEntranceViews: [(NSView, CFTimeInterval)] = []
    private var columnEntranceViews: [(NSView, CFTimeInterval)] = []
    private(set) var showsEmptyState = false
    private(set) var showsRecordingSetup = false
    /// Called with the text of a key press that nothing else in the recents layout handled.
    var onTypeToSearch: ((String) -> Void)?

    static func makeBrand(squircle: CGFloat, wordSize: CGFloat, tagSize: CGFloat) -> BrandBlock {
        let wordmark = NSTextField(labelWithString: "tap")
        wordmark.attributedStringValue = WelcomeFont.attributed("tap", size: wordSize, weight: .bold, color: WelcomeColor.ink, trackingEm: -0.05)
        wordmark.isSelectable = false
        wordmark.setAccessibilityLabel("tap")
        let tagline = NSTextField(wrappingLabelWithString: "")
        tagline.attributedStringValue = WelcomeFont.attributed("Markdown slides, without the markdown limits.", size: tagSize, weight: .regular, color: WelcomeColor.text2)
        tagline.isSelectable = false
        tagline.maximumNumberOfLines = 0
        let newDeck = WelcomeButton(title: "New Deck", hint: "⌘N", style: .primary)
        newDeck.target = nil
        newDeck.action = #selector(AppDelegate.newDeck(_:))
        newDeck.setAccessibilityIdentifier("new-deck")
        let open = WelcomeButton(title: "Open…", hint: "⌘O", style: .glass)
        open.target = nil
        open.action = #selector(NSDocumentController.openDocument(_:))
        open.setAccessibilityIdentifier("open")
        return BrandBlock(icon: WelcomeIconView(squircleSize: squircle), wordmark: wordmark, tagline: tagline, newDeckButton: newDeck, openButton: open)
    }

    init() {
        emptyStateBrand = Self.makeBrand(squircle: 96, wordSize: 58, tagSize: 16)
        columnBrand = Self.makeBrand(squircle: 84, wordSize: 42, tagSize: 14)
        super.init(dropZone: WelcomeDropZone())
        frame = NSRect(origin: .zero, size: Self.windowSize)
        wantsLayer = true
        setAccessibilityElement(false)

        buildHero()
        buildSplit()
        for view in [aurora, dotGrid, hero, filmstrip, split, recordingSetup, dropZone] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            addSubview(view)
            NSLayoutConstraint.activate([
                view.leadingAnchor.constraint(equalTo: leadingAnchor), view.trailingAnchor.constraint(equalTo: trailingAnchor),
            ])
            if view !== filmstrip { view.topAnchor.constraint(equalTo: topAnchor).isActive = true }
            if view === filmstrip {
                view.bottomAnchor.constraint(equalTo: bottomAnchor, constant: -14).isActive = true
                view.heightAnchor.constraint(equalToConstant: WelcomeFilmstrip.height).isActive = true
            } else {
                view.bottomAnchor.constraint(equalTo: bottomAnchor).isActive = true
            }
        }
        aurora.setAccessibilityElement(false)
        setShowsEmptyState(false)
        setShowsRecordingSetup(false)
        refreshBackground()
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    // MARK: Building

    private func buildHero() {
        let brand = emptyStateBrand
        let buttons = NSStackView(views: [brand.newDeckButton, brand.openButton])
        buttons.orientation = .horizontal
        buttons.spacing = 10
        let orLabel = welcomeLabel("or", size: 13, weight: .medium, color: WelcomeColor.text2)
        let tour = NSStackView(views: [orLabel, tourLink])
        tour.orientation = .horizontal
        tour.spacing = 1
        tour.alignment = .firstBaseline
        tourLink.target = nil
        tourLink.action = #selector(AppDelegate.openThemeTour(_:))
        tourLink.setAccessibilityLabel("Take the theme tour")
        tourLink.setAccessibilityIdentifier("theme-tour")

        let stack = NSStackView(views: [brand.icon, brand.wordmark, brand.tagline, buttons, tour])
        stack.orientation = .vertical
        stack.alignment = .centerX
        stack.spacing = 0
        // The icon view is larger than its squircle by the icon grid's margin; the gaps below take it out.
        let margin = (brand.icon.imageSize - brand.icon.squircleSize) / 2
        stack.setCustomSpacing(20 - margin - 8, after: brand.icon)
        stack.setCustomSpacing(10 - 3, after: brand.wordmark)
        stack.setCustomSpacing(27.5, after: brand.tagline)
        stack.setCustomSpacing(16, after: buttons)
        stack.translatesAutoresizingMaskIntoConstraints = false
        hero.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.centerXAnchor.constraint(equalTo: hero.centerXAnchor),
            stack.topAnchor.constraint(equalTo: hero.topAnchor, constant: 58 - margin),
        ])
        hero.setAccessibilityIdentifier("welcome-empty-state")
        hero.setAccessibilityElement(true)
        hero.setAccessibilityRole(.group)
        heroEntranceViews = [(brand.icon, 0), (brand.wordmark, 0.08), (brand.tagline, 0.14), (buttons, 0.2), (tour, 0.2)]
    }

    private func buildSplit() {
        let brand = columnBrand
        brand.tagline.preferredMaxLayoutWidth = 200
        let buttons = NSStackView(views: [brand.newDeckButton, brand.openButton])
        buttons.orientation = .vertical
        buttons.spacing = 8
        buttons.alignment = .centerX
        let stack = NSStackView(views: [brand.icon, brand.wordmark, brand.tagline, buttons])
        stack.orientation = .vertical
        stack.alignment = .centerX
        stack.spacing = 0
        let margin = (brand.icon.imageSize - brand.icon.squircleSize) / 2
        stack.setCustomSpacing(16 - margin - 6, after: brand.icon)
        stack.setCustomSpacing(10 - 6, after: brand.wordmark)
        stack.setCustomSpacing(26, after: brand.tagline)
        let left = FlippedView()
        for view in [stack] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            left.addSubview(view)
        }
        NSLayoutConstraint.activate([
            stack.centerXAnchor.constraint(equalTo: left.centerXAnchor),
            stack.centerYAnchor.constraint(equalTo: left.centerYAnchor),
            brand.newDeckButton.widthAnchor.constraint(equalToConstant: 220),
            brand.openButton.widthAnchor.constraint(equalToConstant: 220),
        ])

        // The right pane: opaque, with a hairline on its left edge.
        recentsPane.wantsLayer = true
        recentsScrollView.drawsBackground = false
        recentsScrollView.hasVerticalScroller = true
        recentsScrollView.autohidesScrollers = true
        recentsScrollView.documentView = tableView
        let column = NSTableColumn(identifier: NSUserInterfaceItemIdentifier("recent"))
        column.resizingMask = .autoresizingMask
        tableView.addTableColumn(column)
        tableView.headerView = nil
        tableView.rowHeight = WelcomeRecentCell.rowHeight
        tableView.style = .inset
        tableView.backgroundColor = .clear
        tableView.intercellSpacing = NSSize(width: 0, height: 2)
        tableView.focusRingType = .none
        tableView.setAccessibilityIdentifier("recent-decks")
        tableView.setAccessibilityLabel("Recent decks")

        searchField.placeholderString = "Search decks"
        searchField.sendsSearchStringImmediately = true
        searchField.sendsWholeSearchString = false
        searchField.font = .systemFont(ofSize: 13)
        searchField.setAccessibilityIdentifier("recent-search")
        searchField.setAccessibilityLabel("Search decks")
        noMatchesLabel.isHidden = true

        hairline.wantsLayer = true
        for view in [hairline, recentLabel, searchField, recentsScrollView, noMatchesLabel] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            recentsPane.addSubview(view)
        }
        NSLayoutConstraint.activate([
            hairline.leadingAnchor.constraint(equalTo: recentsPane.leadingAnchor), hairline.topAnchor.constraint(equalTo: recentsPane.topAnchor),
            hairline.bottomAnchor.constraint(equalTo: recentsPane.bottomAnchor), hairline.widthAnchor.constraint(equalToConstant: 1),
            recentLabel.leadingAnchor.constraint(equalTo: recentsPane.leadingAnchor, constant: 20),
            recentLabel.centerYAnchor.constraint(equalTo: searchField.centerYAnchor),
            searchField.trailingAnchor.constraint(equalTo: recentsPane.trailingAnchor, constant: -20),
            searchField.topAnchor.constraint(equalTo: recentsPane.topAnchor, constant: 16),
            searchField.widthAnchor.constraint(equalToConstant: 180),
            recentsScrollView.leadingAnchor.constraint(equalTo: recentsPane.leadingAnchor),
            recentsScrollView.trailingAnchor.constraint(equalTo: recentsPane.trailingAnchor),
            recentsScrollView.topAnchor.constraint(equalTo: searchField.bottomAnchor, constant: 2),
            recentsScrollView.bottomAnchor.constraint(equalTo: recentsPane.bottomAnchor, constant: -8),
            noMatchesLabel.centerXAnchor.constraint(equalTo: recentsPane.centerXAnchor),
            noMatchesLabel.topAnchor.constraint(equalTo: recentsScrollView.topAnchor, constant: 40),
        ])

        for view in [left, recentsPane] {
            view.translatesAutoresizingMaskIntoConstraints = false
            split.addSubview(view)
        }
        NSLayoutConstraint.activate([
            left.leadingAnchor.constraint(equalTo: split.leadingAnchor), left.topAnchor.constraint(equalTo: split.topAnchor),
            left.bottomAnchor.constraint(equalTo: split.bottomAnchor), left.widthAnchor.constraint(equalToConstant: Self.brandPaneWidth),
            recentsPane.leadingAnchor.constraint(equalTo: left.trailingAnchor), recentsPane.trailingAnchor.constraint(equalTo: split.trailingAnchor),
            recentsPane.topAnchor.constraint(equalTo: split.topAnchor), recentsPane.bottomAnchor.constraint(equalTo: split.bottomAnchor),
        ])
        columnEntranceViews = [(brand.icon, 0), (brand.wordmark, 0.08), (brand.tagline, 0.14), (buttons, 0.2)]
    }

    override func keyDown(with event: NSEvent) {
        let flags = event.modifierFlags.intersection([.command, .control, .option])
        if !showsEmptyState, flags.isEmpty, let characters = event.characters,
           let scalar = characters.unicodeScalars.first, scalar.value >= 0x20, scalar.value != 0x7F, !(0xF700...0xF8FF).contains(scalar.value) {
            onTypeToSearch?(characters)
        } else {
            super.keyDown(with: event)
        }
    }

    // MARK: Layouts

    /// Swaps the two layouts. New Deck is the first-launch layout's default button (Return).
    func setShowsEmptyState(_ shows: Bool) {
        showsEmptyState = shows
        hero.isHidden = !shows || showsRecordingSetup
        filmstrip.isHidden = !shows || showsRecordingSetup
        split.isHidden = shows || showsRecordingSetup
        emptyStateBrand.newDeckButton.keyEquivalent = shows ? "\r" : ""
        columnBrand.newDeckButton.keyEquivalent = ""
        filmstrip.setActive(shows && !showsRecordingSetup && window?.isVisible == true)
    }

    /// Puts the recording setup screen in place of whichever layout shows, or gives the layout back.
    func setShowsRecordingSetup(_ shows: Bool) {
        showsRecordingSetup = shows
        recordingSetup.isHidden = !shows
        setShowsEmptyState(showsEmptyState)
    }

    // MARK: Colors

    override var isFlipped: Bool { true }

    override var wantsUpdateLayer: Bool { true }

    override func updateLayer() { refreshBackground() }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        refreshBackground()
        needsDisplay = true
    }

    private func refreshBackground() {
        effectiveAppearance.performAsCurrentDrawingAppearance {
            layer?.backgroundColor = WelcomeColor.window.cgColor
            recentsPane.layer?.backgroundColor = WelcomeColor.window.cgColor
            hairline.layer?.backgroundColor = WelcomeColor.line.cgColor
        }
    }

    // MARK: Drag state

    /// The window steps back so the drop pill stands alone: everything else fades to 32% and shrinks slightly.
    func setDragActive(_ active: Bool) {
        aurora.setDragBoosted(active)
        let reduce = WelcomeMotion.reduceMotion()
        NSAnimationContext.runAnimationGroup { context in
            context.duration = reduce ? 0 : 0.25
            for view in [hero, filmstrip, split, recordingSetup] { view.animator().alphaValue = active ? 0.32 : 1 }
        }
        for view in [hero, split] { setSublayerScale(of: view, to: active && !reduce ? 0.985 : 1, animated: !reduce) }
    }

    /// Scales a view's sublayers about the view's center, whatever anchor point AppKit gave its layer.
    private func setSublayerScale(of view: NSView, to scale: CGFloat, animated: Bool) {
        guard let layer = view.layer else { return }
        let pivot = CGPoint(x: layer.anchorPoint.x * layer.bounds.width, y: layer.anchorPoint.y * layer.bounds.height)
        let center = CGPoint(x: layer.bounds.midX, y: layer.bounds.midY)
        var transform = CATransform3DMakeTranslation((1 - scale) * (center.x - pivot.x), (1 - scale) * (center.y - pivot.y), 0)
        transform = CATransform3DScale(transform, scale, scale, 1)
        CATransaction.begin()
        CATransaction.setDisableActions(!animated)
        CATransaction.setAnimationDuration(0.25)
        layer.sublayerTransform = transform
        CATransaction.commit()
    }

    // MARK: Entrance

    /// The staggered rise of the icon, wordmark, tagline and buttons, and the strip's fade.
    /// With Reduce Motion nothing moves.
    func playEntrance() {
        guard !WelcomeMotion.reduceMotion() else { return }
        let views = showsEmptyState ? heroEntranceViews : columnEntranceViews
        for (view, delay) in views { rise(view, delay: delay) }
        if showsEmptyState, let layer = filmstrip.layer {
            let fade = CABasicAnimation(keyPath: "opacity")
            fade.fromValue = 0
            fade.toValue = 1
            fade.duration = 1.4
            fade.beginTime = CACurrentMediaTime() + 0.3
            fade.fillMode = .backwards
            fade.timingFunction = CAMediaTimingFunction(name: .easeInEaseOut)
            layer.add(fade, forKey: "entrance")
        }
    }

    private func rise(_ view: NSView, delay: CFTimeInterval) {
        view.wantsLayer = true
        guard let layer = view.layer else { return }
        let scale: CGFloat = 0.985
        let pivot = CGPoint(x: layer.anchorPoint.x * layer.bounds.width, y: layer.anchorPoint.y * layer.bounds.height)
        let center = CGPoint(x: layer.bounds.midX, y: layer.bounds.midY)
        // Starts 10 points lower than it rests: the layer's y axis is flipped when its superlayer is.
        let flipped = view.superview?.layer?.isGeometryFlipped == true || view.superview?.isFlipped == true
        var start = CATransform3DMakeTranslation((1 - scale) * (center.x - pivot.x), (1 - scale) * (center.y - pivot.y) + (flipped ? 10 : -10), 0)
        start = CATransform3DScale(start, scale, scale, 1)
        let move = CABasicAnimation(keyPath: "transform")
        move.fromValue = NSValue(caTransform3D: start)
        move.toValue = NSValue(caTransform3D: CATransform3DIdentity)
        let fade = CABasicAnimation(keyPath: "opacity")
        fade.fromValue = 0
        fade.toValue = 1
        let group = CAAnimationGroup()
        group.animations = [move, fade]
        group.duration = 0.9
        group.beginTime = CACurrentMediaTime() + delay
        group.fillMode = .backwards
        group.timingFunction = CAMediaTimingFunction(controlPoints: 0.2, 0.8, 0.2, 1)
        layer.add(group, forKey: "entrance")
    }
}
