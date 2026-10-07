import AppKit
import TapDesktopCore

/// Tokens the recording setup screen adds to the welcome window's.
extension WelcomeColor {
    static let allowed = linkLine
    static let allowedSoft = NSColor.welcomeDynamic(light: NSColor(hex: 0x178a4c, alpha: 0.10), dark: NSColor(hex: 0x7fd18c, alpha: 0.12))
    static let tile = NSColor.welcomeDynamic(light: NSColor(hex: 0xf2f4f7), dark: NSColor(white: 1, alpha: 0.07))
    static let tileInk = NSColor.welcomeDynamic(light: NSColor(hex: 0x344054), dark: NSColor(hex: 0xc9d0da))
    static let switchOff = NSColor.welcomeDynamic(light: NSColor(hex: 0xd0d5dd), dark: NSColor(hex: 0x3a3f48))
    static let switchOn = NSColor.welcomeDynamic(light: NSColor(hex: 0x34c759), dark: NSColor(hex: 0x30d158))
}

/// A small rounded button: `.primary` is filled ink, `.quiet` is a hairline outline.
final class RecordingSetupButton: NSButton {
    enum Style { case primary, quiet }

    var style: Style = .primary { didSet { needsDisplay = true } }

    init(title: String, style: Style) {
        self.style = style
        super.init(frame: .zero)
        self.title = title
        isBordered = false
        setButtonType(.momentaryPushIn)
        focusRingType = .exterior
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    private var titleAttributes: [NSAttributedString.Key: Any] {
        [.font: WelcomeFont.display(size: 13, weight: .semibold), .foregroundColor: style == .primary ? WelcomeColor.primaryText : WelcomeColor.text3]
    }

    override var isFlipped: Bool { true }

    override var intrinsicContentSize: NSSize {
        let size = (title as NSString).size(withAttributes: titleAttributes)
        return NSSize(width: ceil(size.width) + 32, height: ceil(size.height) + 16)
    }

    override func draw(_ dirtyRect: NSRect) {
        effectiveAppearance.performAsCurrentDrawingAppearance {
            let path = NSBezierPath(roundedRect: bounds.insetBy(dx: 0.5, dy: 0.5), xRadius: 9, yRadius: 9)
            switch style {
            case .primary:
                WelcomeColor.primaryFill.withAlphaComponent(isHighlighted ? 0.85 : 1).setFill()
                path.fill()
            case .quiet:
                if isHighlighted {
                    NSColor(white: 0.5, alpha: 0.08).setFill()
                    path.fill()
                }
                WelcomeColor.hairline.setStroke()
                path.lineWidth = 1
                path.stroke()
            }
            let size = (title as NSString).size(withAttributes: titleAttributes)
            (title as NSString).draw(at: NSPoint(x: (bounds.width - size.width) / 2, y: (bounds.height - size.height) / 2), withAttributes: titleAttributes)
        }
    }

    override func drawFocusRingMask() {
        NSBezierPath(roundedRect: bounds, xRadius: 9, yRadius: 9).fill()
    }

    override var focusRingMaskBounds: NSRect { bounds }
}

/// A label in the welcome window's type; `wrapWidth` makes it wrap there.
private func setupLabel(_ text: String, size: CGFloat, weight: NSFont.Weight, color: NSColor, tracking: CGFloat = 0, wrapWidth: CGFloat? = nil) -> NSTextField {
    let field = wrapWidth == nil ? NSTextField(labelWithString: text) : NSTextField(wrappingLabelWithString: text)
    field.isSelectable = false
    if let wrapWidth {
        field.maximumNumberOfLines = 0
        field.preferredMaxLayoutWidth = wrapWidth
    }
    setText(of: field, text, size: size, weight: weight, color: color, tracking: tracking)
    return field
}

private func setText(of field: NSTextField, _ text: String, size: CGFloat, weight: NSFont.Weight, color: NSColor, tracking: CGFloat = 0) {
    field.attributedStringValue = WelcomeFont.attributed(text, size: size, weight: weight, color: color, trackingEm: tracking, alignment: .left)
}

/// A small drawn copy of the System Settings row for Tap, whose switch turns on and off in a loop.
/// With Reduce Motion the switch rests on.
final class SettingsRowView: FlippedView {
    static let height: CGFloat = 32
    private let iconLayer = CAGradientLayer()
    private let trackLayer = CALayer()
    private let knobLayer = CALayer()
    private let nameLabel = NSTextField(labelWithString: "Tap")
    private static let knobTravel: CGFloat = 12

    override init(frame: NSRect) {
        super.init(frame: frame)
        wantsLayer = true
        layer?.cornerRadius = 7
        iconLayer.colors = [NSColor(hex: 0x10b981).cgColor, NSColor(hex: 0x0e7490).cgColor]
        iconLayer.startPoint = CGPoint(x: 0, y: 1)
        iconLayer.endPoint = CGPoint(x: 1, y: 0)
        iconLayer.cornerRadius = 5
        trackLayer.cornerRadius = 9
        knobLayer.cornerRadius = 7
        knobLayer.backgroundColor = NSColor.white.cgColor
        knobLayer.shadowOpacity = 0.25
        knobLayer.shadowRadius = 1
        knobLayer.shadowOffset = CGSize(width: 0, height: -1)
        for sublayer in [iconLayer, trackLayer, knobLayer] { layer?.addSublayer(sublayer) }
        nameLabel.font = .systemFont(ofSize: 12.5, weight: .medium)
        nameLabel.textColor = WelcomeColor.ink
        addSubview(nameLabel)
        setAccessibilityElement(false)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override var intrinsicContentSize: NSSize { NSSize(width: NSView.noIntrinsicMetric, height: Self.height) }

    override func layout() {
        super.layout()
        iconLayer.frame = CGRect(x: 9, y: (bounds.height - 18) / 2, width: 18, height: 18)
        nameLabel.sizeToFit()
        nameLabel.frame.origin = NSPoint(x: 35, y: (bounds.height - nameLabel.frame.height) / 2)
        trackLayer.frame = CGRect(x: bounds.width - 9 - 30, y: (bounds.height - 18) / 2, width: 30, height: 18)
        knobLayer.frame = CGRect(x: trackLayer.frame.minX + 2, y: trackLayer.frame.minY + 2, width: 14, height: 14)
    }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        updateMotion()
    }

    override func viewDidMoveToWindow() {
        super.viewDidMoveToWindow()
        updateMotion()
    }

    /// Sets the switch's colors and starts, or stops, its loop.
    func updateMotion() {
        let reduce = WelcomeMotion.reduceMotion()
        trackLayer.removeAllAnimations()
        knobLayer.removeAllAnimations()
        effectiveAppearance.performAsCurrentDrawingAppearance {
            layer?.backgroundColor = WelcomeColor.window.cgColor
            let off = WelcomeColor.switchOff.cgColor
            let on = WelcomeColor.switchOn.cgColor
            trackLayer.backgroundColor = reduce ? on : off
            knobLayer.transform = CATransform3DMakeTranslation(reduce ? Self.knobTravel : 0, 0, 0)
            guard !reduce, window != nil else { return }
            let keyTimes: [NSNumber] = [0, 0.55, 0.7, 0.9, 1]
            let fill = CAKeyframeAnimation(keyPath: "backgroundColor")
            fill.values = [off, off, on, on, off]
            let slide = CAKeyframeAnimation(keyPath: "transform.translation.x")
            slide.values = [0, 0, Self.knobTravel, Self.knobTravel, 0]
            for (animation, target) in [(fill, trackLayer), (slide, knobLayer)] {
                animation.keyTimes = keyTimes
                animation.duration = 2.4
                animation.repeatCount = .infinity
                animation.timingFunction = CAMediaTimingFunction(name: .easeInEaseOut)
                target.add(animation, forKey: "loop")
            }
        }
    }
}

/// One permission's card: an icon tile, the step's name and reason, and on
/// the right its button, or its status. Screen Recording's card grows a
/// guide to the System Settings switch while it waits.
final class RecordingStepCard: FlippedView {
    let step: RecordingStep
    let button = RecordingSetupButton(title: "", style: .primary)
    /// Called when the button is pressed, with what it was labelled for.
    var onPress: ((RecordingStepAction) -> Void)?

    private var action = RecordingStepAction.none
    private var isDone = false
    private var isCurrent = false
    private let tile = FlippedView()
    private let symbolView = NSImageView()
    private let badge = FlippedView()
    private let checkView = NSImageView()
    private let nameLabel: NSTextField
    private let reasonLabel: NSTextField
    private let spinner = NSProgressIndicator()
    private let statusLabel = setupLabel("", size: 12.5, weight: .semibold, color: WelcomeColor.text3)
    private let statusStack: NSStackView
    private let guide = NSStackView()
    let settingsRow = SettingsRowView()
    private var reason: String { step == .microphone ? "Records your voice with the slides." : "Records the slides." }

    init(step: RecordingStep) {
        self.step = step
        nameLabel = setupLabel(step == .microphone ? "Microphone" : "Screen Recording", size: 15, weight: .semibold, color: WelcomeColor.ink)
        reasonLabel = setupLabel("", size: 12.5, weight: .regular, color: WelcomeColor.text3, wrapWidth: 180)
        statusStack = NSStackView(views: [spinner, statusLabel])
        super.init(frame: .zero)
        wantsLayer = true
        layer?.cornerRadius = 14
        layer?.borderWidth = 1
        setAccessibilityElement(true)
        setAccessibilityRole(.group)

        tile.wantsLayer = true
        tile.layer?.cornerRadius = 11
        symbolView.image = NSImage(systemSymbolName: step == .microphone ? "mic.fill" : "display", accessibilityDescription: nil)?
            .withSymbolConfiguration(NSImage.SymbolConfiguration(pointSize: 18, weight: .regular))
        symbolView.setAccessibilityElement(false)
        badge.wantsLayer = true
        badge.layer?.cornerRadius = 9
        badge.layer?.borderWidth = 2
        checkView.image = NSImage(systemSymbolName: "checkmark", accessibilityDescription: nil)?
            .withSymbolConfiguration(NSImage.SymbolConfiguration(pointSize: 8, weight: .heavy))
        checkView.setAccessibilityElement(false)
        for (parent, child) in [(tile, symbolView), (tile, badge), (badge, checkView)] {
            child.translatesAutoresizingMaskIntoConstraints = false
            parent.addSubview(child)
        }
        tile.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            tile.widthAnchor.constraint(equalToConstant: 40), tile.heightAnchor.constraint(equalToConstant: 40),
            symbolView.centerXAnchor.constraint(equalTo: tile.centerXAnchor), symbolView.centerYAnchor.constraint(equalTo: tile.centerYAnchor),
            badge.widthAnchor.constraint(equalToConstant: 18), badge.heightAnchor.constraint(equalToConstant: 18),
            badge.trailingAnchor.constraint(equalTo: tile.trailingAnchor, constant: 4), badge.bottomAnchor.constraint(equalTo: tile.bottomAnchor, constant: 4),
            checkView.centerXAnchor.constraint(equalTo: badge.centerXAnchor), checkView.centerYAnchor.constraint(equalTo: badge.centerYAnchor),
        ])

        button.target = self
        button.action = #selector(pressed(_:))
        spinner.style = .spinning
        spinner.controlSize = .small
        spinner.isIndeterminate = true
        spinner.setAccessibilityElement(false)
        statusStack.orientation = .horizontal
        statusStack.spacing = 6

        let text = NSStackView(views: [nameLabel, reasonLabel])
        text.orientation = .vertical
        text.alignment = .leading
        text.spacing = 2
        text.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
        text.setHuggingPriority(.defaultLow, for: .horizontal)
        let top = NSStackView(views: [tile, text, button, statusStack])
        top.orientation = .horizontal
        top.alignment = .centerY
        top.spacing = 14

        let breadcrumb = setupLabel("Privacy & Security › Screen & System Audio Recording", size: 11, weight: .regular, color: WelcomeColor.text3, wrapWidth: 168)
        breadcrumb.font = .systemFont(ofSize: 11)
        let preview = NSStackView(views: [breadcrumb, settingsRow])
        preview.orientation = .vertical
        preview.alignment = .width
        preview.spacing = 7
        preview.edgeInsets = NSEdgeInsets(top: 10, left: 11, bottom: 10, right: 11)
        preview.wantsLayer = true
        preview.layer?.cornerRadius = 10
        previewBackground = preview
        let steps = Self.guideSteps()
        guide.setViews([preview, steps], in: .leading)
        guide.orientation = .horizontal
        guide.distribution = .fillEqually
        guide.alignment = .centerY
        guide.spacing = 12
        guide.isHidden = true

        let stack = NSStackView(views: step == .screenRecording ? [top, guide] : [top])
        stack.orientation = .vertical
        stack.alignment = .width
        stack.spacing = 14
        stack.translatesAutoresizingMaskIntoConstraints = false
        addSubview(stack)
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 16), stack.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -16),
            stack.topAnchor.constraint(equalTo: topAnchor, constant: 16), stack.bottomAnchor.constraint(equalTo: bottomAnchor, constant: -16),
        ])
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    private var previewBackground: NSView?

    /// The three numbered lines beside the preview.
    private static func guideSteps() -> NSTextField {
        let lines = ["Switch Tap on in the list.", "Choose Quit & Reopen when macOS asks.", "Tap opens again on this screen."]
        let style = NSMutableParagraphStyle()
        style.tabStops = [NSTextTab(type: .leftTabStopType, location: 16)]
        style.headIndent = 16
        style.paragraphSpacing = 2
        let text = NSMutableAttributedString()
        for (index, line) in lines.enumerated() {
            let paragraph = NSMutableAttributedString(string: "\(index + 1).\t\(line)\(index < lines.count - 1 ? "\n" : "")", attributes: [
                .font: WelcomeFont.display(size: 12.5, weight: .regular), .foregroundColor: WelcomeColor.text2, .paragraphStyle: style,
            ])
            let bold = (paragraph.string as NSString).range(of: "Quit & Reopen")
            if bold.location != NSNotFound { paragraph.addAttribute(.font, value: WelcomeFont.display(size: 12.5, weight: .semibold), range: bold) }
            text.append(paragraph)
        }
        let field = NSTextField(wrappingLabelWithString: "")
        field.attributedStringValue = text
        field.isSelectable = false
        field.maximumNumberOfLines = 0
        field.preferredMaxLayoutWidth = 184
        return field
    }

    @objc private func pressed(_ sender: Any?) { onPress?(action) }

    /// Shows the step as done, current or later, with `action` on its button.
    func update(isDone: Bool, isCurrent: Bool, action: RecordingStepAction) {
        self.isDone = isDone
        self.isCurrent = isCurrent
        self.action = action
        let waiting = action == .waiting
        setText(of: reasonLabel, reason + (step == .screenRecording && !isDone && !isCurrent ? " Tap quits and reopens once." : ""), size: 12.5, weight: .regular, color: WelcomeColor.text3)
        button.isHidden = action != .allow && action != .openSettings
        button.title = action == .allow ? "Allow Microphone" : "Open Settings"
        button.setAccessibilityIdentifier("recording-setup-\(step == .microphone ? "microphone" : "screen")-button")
        button.invalidateIntrinsicContentSize()
        statusStack.isHidden = !isDone && !waiting
        spinner.isHidden = !waiting
        setText(of: statusLabel, isDone ? "Allowed" : "Waiting for System Settings", size: 12.5, weight: .semibold, color: isDone ? WelcomeColor.allowed : WelcomeColor.text3)
        badge.isHidden = !isDone
        guide.isHidden = !waiting
        alphaValue = !isDone && !isCurrent ? 0.55 : 1
        let name = step == .microphone ? "Microphone" : "Screen Recording"
        setAccessibilityLabel("\(name), \(isDone ? "allowed" : "not allowed")")
        updateMotion()
        refreshColors()
    }

    func updateMotion() {
        if action == .waiting, !WelcomeMotion.reduceMotion() { spinner.startAnimation(nil) } else { spinner.stopAnimation(nil) }
        settingsRow.updateMotion()
    }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        refreshColors()
    }

    private func refreshColors() {
        effectiveAppearance.performAsCurrentDrawingAppearance {
            layer?.backgroundColor = WelcomeColor.glass.cgColor
            layer?.borderColor = WelcomeColor.glassLine.cgColor
            layer?.shadowColor = NSColor.black.cgColor
            layer?.shadowOpacity = isCurrent ? 0.10 : 0
            layer?.shadowRadius = 14
            layer?.shadowOffset = .zero
            tile.layer?.backgroundColor = (isDone ? WelcomeColor.allowedSoft : WelcomeColor.tile).cgColor
            symbolView.contentTintColor = isDone ? WelcomeColor.allowed : WelcomeColor.tileInk
            badge.layer?.backgroundColor = WelcomeColor.allowed.cgColor
            badge.layer?.borderColor = WelcomeColor.window.cgColor
            checkView.contentTintColor = WelcomeColor.window
            previewBackground?.layer?.backgroundColor = WelcomeColor.tile.cgColor
        }
    }
}

/// Shown in place of the welcome window's layouts until recording is set
/// up or put off: the microphone and Screen Recording permissions, one card
/// each, with Set Up Later and Continue to Tap along the bottom. It reads
/// its permissions from `AppEnvironment` and looks again whenever
/// `refresh()` is called.
final class RecordingSetupView: FlippedView {
    static let footerHeight: CGFloat = 76

    let icon = WelcomeIconView(squircleSize: 56)
    let microphoneCard = RecordingStepCard(step: .microphone)
    let screenCard = RecordingStepCard(step: .screenRecording)
    let laterButton = WelcomeLinkButton(title: "Set Up Later")
    let continueButton = RecordingSetupButton(title: "Continue to Tap", style: .quiet)
    let titleLabel = setupLabel("", size: 30, weight: .bold, color: WelcomeColor.ink, tracking: -0.03, wrapWidth: 300)
    let bodyLabel = setupLabel("", size: 14, weight: .regular, color: WelcomeColor.text2, wrapWidth: 300)
    let metaLabel = setupLabel("", size: 12.5, weight: .regular, color: WelcomeColor.text3, wrapWidth: 300)
    private let laterGroup = NSStackView()
    private let footer = FlippedView()
    private let footerLine = FlippedView()
    private let segments = [FlippedView(), FlippedView()]
    private(set) var state = RecordingSetup(microphone: .notDetermined, screenRecordingAllowed: false, dismissed: false, awaitingConfirmation: false)
    var onSetUpLater: (() -> Void)?
    var onContinue: (() -> Void)?

    init() {
        super.init(frame: .zero)
        setAccessibilityIdentifier("recording-setup")
        setAccessibilityElement(true)
        setAccessibilityRole(.group)
        setAccessibilityLabel("Recording setup")

        let progress = NSStackView(views: segments)
        progress.orientation = .horizontal
        progress.spacing = 5
        for segment in segments {
            segment.wantsLayer = true
            segment.layer?.cornerRadius = 2
            segment.translatesAutoresizingMaskIntoConstraints = false
            segment.widthAnchor.constraint(equalToConstant: 28).isActive = true
            segment.heightAnchor.constraint(equalToConstant: 4).isActive = true
        }
        let intro = NSStackView(views: [titleLabel, bodyLabel, metaLabel, progress])
        intro.orientation = .vertical
        intro.alignment = .leading
        intro.spacing = 14
        intro.setCustomSpacing(20, after: bodyLabel)
        intro.setCustomSpacing(20, after: metaLabel)
        let steps = NSStackView(views: [microphoneCard, screenCard])
        steps.orientation = .vertical
        steps.alignment = .width
        steps.spacing = 12

        let caption = setupLabel("Find it again in Help › Set Up Recording…", size: 11.5, weight: .regular, color: WelcomeColor.text3)
        laterGroup.setViews([laterButton, caption], in: .leading)
        laterGroup.orientation = .vertical
        laterGroup.alignment = .leading
        laterGroup.spacing = 2
        laterButton.setAccessibilityIdentifier("recording-setup-later")
        laterButton.target = self
        laterButton.action = #selector(setUpLater(_:))
        continueButton.setAccessibilityIdentifier("recording-setup-continue")
        continueButton.target = self
        continueButton.action = #selector(continueToTap(_:))
        microphoneCard.setAccessibilityIdentifier("recording-setup-microphone")
        screenCard.setAccessibilityIdentifier("recording-setup-screen")
        microphoneCard.onPress = { [weak self] in self?.press(.microphone, $0) }
        screenCard.onPress = { [weak self] in self?.press(.screenRecording, $0) }

        footer.wantsLayer = true
        for view in [footerLine, laterGroup, continueButton] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            footer.addSubview(view)
        }
        for view in [icon, intro, steps, footer] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            addSubview(view)
        }
        footerLine.wantsLayer = true
        let margin = (icon.imageSize - icon.squircleSize) / 2
        NSLayoutConstraint.activate([
            icon.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 56 - margin),
            icon.topAnchor.constraint(equalTo: topAnchor, constant: 64 - margin),
            intro.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 56),
            intro.topAnchor.constraint(equalTo: icon.bottomAnchor, constant: 20 - margin),
            intro.widthAnchor.constraint(equalToConstant: 300),
            steps.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 56 + 300 + 44),
            steps.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -56),
            steps.topAnchor.constraint(equalTo: topAnchor, constant: 64),
            footer.leadingAnchor.constraint(equalTo: leadingAnchor), footer.trailingAnchor.constraint(equalTo: trailingAnchor),
            footer.bottomAnchor.constraint(equalTo: bottomAnchor), footer.heightAnchor.constraint(equalToConstant: Self.footerHeight),
            footerLine.leadingAnchor.constraint(equalTo: footer.leadingAnchor), footerLine.trailingAnchor.constraint(equalTo: footer.trailingAnchor),
            footerLine.topAnchor.constraint(equalTo: footer.topAnchor), footerLine.heightAnchor.constraint(equalToConstant: 1),
            laterGroup.leadingAnchor.constraint(equalTo: footer.leadingAnchor, constant: 56), laterGroup.centerYAnchor.constraint(equalTo: footer.centerYAnchor),
            continueButton.trailingAnchor.constraint(equalTo: footer.trailingAnchor, constant: -56), continueButton.centerYAnchor.constraint(equalTo: footer.centerYAnchor),
        ])
        refresh()
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    /// Whether Open Settings for Screen Recording was pressed in this launch.
    private var openedScreenRecordingSettings = false

    /// Reads the permissions and the saved choices again and redraws.
    func refresh() {
        let permissions = AppEnvironment.shared.recordingPermissions
        let store = AppEnvironment.shared.recordingSetupStore
        show(RecordingSetup(microphone: permissions.microphone, screenRecordingAllowed: permissions.screenRecordingAllowed,
                            dismissed: store.dismissed, awaitingConfirmation: store.awaitingConfirmation, screenRecordingSettingsOpened: openedScreenRecordingSettings))
    }

    private func show(_ setup: RecordingSetup) {
        state = setup
        for card in [microphoneCard, screenCard] {
            card.update(isDone: setup.isDone(card.step), isCurrent: setup.currentStep == card.step, action: setup.action(for: card.step))
        }
        setText(of: titleLabel, setup.isComplete ? "You're ready to record" : "Get ready to record your talks", size: 30, weight: .bold, color: WelcomeColor.ink, tracking: -0.03)
        setText(of: bodyLabel, setup.isComplete
                ? "Turn on Record in Present Settings, and Tap saves each talk to a recordings folder next to its deck."
                : "Tap can record your slides and your voice while you present. macOS asks for two permissions first.",
                size: 14, weight: .regular, color: WelcomeColor.text2)
        setText(of: metaLabel, setup.microphone == .allowed ? "One step left." : "About a minute. You only do this once.", size: 12.5, weight: .regular, color: WelcomeColor.text3)
        metaLabel.isHidden = setup.isComplete
        laterGroup.isHidden = setup.isComplete
        continueButton.style = setup.isComplete ? .primary : .quiet
        refreshColors()
    }

    /// Restarts the spinner and the switch under the current Reduce Motion setting.
    func updateMotion() {
        for card in [microphoneCard, screenCard] { card.updateMotion() }
    }

    private func press(_ step: RecordingStep, _ action: RecordingStepAction) {
        let permissions = AppEnvironment.shared.recordingPermissions
        switch (step, action) {
        case (.microphone, .allow): permissions.requestMicrophone()
        case (.microphone, .openSettings): permissions.openMicrophoneSettings()
        case (.screenRecording, .openSettings):
            AppEnvironment.shared.recordingSetupStore.awaitingConfirmation = true
            openedScreenRecordingSettings = true
            permissions.requestScreenRecordingAndOpenSettings()
        default: break
        }
        refresh()
    }

    @objc private func setUpLater(_ sender: Any?) { onSetUpLater?() }
    @objc private func continueToTap(_ sender: Any?) { onContinue?() }

    override func viewDidChangeEffectiveAppearance() {
        super.viewDidChangeEffectiveAppearance()
        refreshColors()
    }

    private func refreshColors() {
        effectiveAppearance.performAsCurrentDrawingAppearance {
            footer.layer?.backgroundColor = WelcomeColor.glass.cgColor
            footerLine.layer?.backgroundColor = WelcomeColor.line.cgColor
            let done = [microphoneCard, screenCard].filter { state.isDone($0.step) }.count
            for (index, segment) in segments.enumerated() {
                segment.layer?.backgroundColor = (index < done ? WelcomeColor.allowed : WelcomeColor.switchOff).cgColor
            }
        }
    }
}
