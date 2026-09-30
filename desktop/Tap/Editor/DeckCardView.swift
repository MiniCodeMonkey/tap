import AppKit

/// One summary chip of the collapsed Deck card: a pill with a few words.
final class DeckChipView: NSView {
    let label: NSTextField
    let kind: DeckProblems.Chip.Kind

    init(_ chip: DeckProblems.Chip) {
        kind = chip.kind
        label = NSTextField(labelWithString: chip.text)
        super.init(frame: .zero)
        label.font = .systemFont(ofSize: 11.5, weight: chip.kind == .plain ? .regular : .medium)
        label.lineBreakMode = .byTruncatingTail
        label.textColor = switch chip.kind {
        case .plain: .secondaryLabelColor
        case .problem: EditorPalette.error
        case .warning: EditorPalette.warning
        }
        label.translatesAutoresizingMaskIntoConstraints = false
        addSubview(label)
        NSLayoutConstraint.activate([
            label.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 8),
            label.trailingAnchor.constraint(equalTo: trailingAnchor, constant: -8),
            label.centerYAnchor.constraint(equalTo: centerYAnchor),
            heightAnchor.constraint(equalToConstant: 20),
        ])
        setCompressionPriority(.defaultLow)
        setAccessibilityElement(true)
        setAccessibilityRole(.staticText)
        setAccessibilityLabel(chip.text)
    }

    /// How firmly the chip and its text keep their width against a short row.
    func setCompressionPriority(_ priority: NSLayoutConstraint.Priority) {
        label.setContentCompressionResistancePriority(priority, for: .horizontal)
        setContentCompressionResistancePriority(priority, for: .horizontal)
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { nil }

    override func draw(_ dirtyRect: NSRect) {
        let path = NSBezierPath(roundedRect: bounds.insetBy(dx: 0.5, dy: 0.5), xRadius: bounds.height / 2, yRadius: bounds.height / 2)
        switch kind {
        case .plain:
            NSColor.labelColor.withAlphaComponent(0.06).setFill()
            path.fill()
        case .problem:
            EditorPalette.errorTint.setFill()
            path.fill()
            EditorPalette.errorTintBorder.setStroke()
            path.stroke()
        case .warning:
            EditorPalette.warningTint.setFill()
            path.fill()
            EditorPalette.warningTintBorder.setStroke()
            path.stroke()
        }
    }
}

/// The disclosure triangle. Space, with the button focused, opens or closes the card.
final class DeckDisclosureButton: NSButton {
    override func keyDown(with event: NSEvent) {
        if event.charactersIgnoringModifiers == " " {
            performClick(nil)
        } else {
            super.keyDown(with: event)
        }
    }
}

/// The card's header line. A click anywhere on it that no control takes
/// opens or closes the card.
final class DeckCardHeaderView: NSView {
    var onClick: (() -> Void)?

    override func mouseDown(with event: NSEvent) {
        onClick?()
    }

    override var mouseDownCanMoveWindow: Bool { false }
}

/// The Deck card's own views, over the editor's card surface: a header
/// with the disclosure, "Deck", the summary chips and the Form | Text
/// switch, and under it, when open in Form mode, the body that holds the
/// form. The surface (fill, border, tint) is drawn by the editor beneath,
/// so the frontmatter's lines can show through in Text mode.
final class DeckCardView: NSView {
    let header = DeckCardHeaderView()
    let disclosureButton = DeckDisclosureButton()
    let titleLabel = NSTextField(labelWithString: "Deck")
    let chipStack = NSStackView()
    let modeControl = NSSegmentedControl(labels: ["Form", "Text"], trackingMode: .selectOne, target: nil, action: nil)
    let bodyView = NSView()
    private(set) var chipViews: [DeckChipView] = []
    private var bodyHeight: NSLayoutConstraint!
    /// Open or close, from the disclosure or a click on the header.
    var onToggle: (() -> Void)?
    /// The switch: true for Text.
    var onModeChange: ((Bool) -> Void)?

    override init(frame: NSRect) {
        super.init(frame: frame)
        setAccessibilityIdentifier("deck-card")
        setAccessibilityRole(.group)
        setAccessibilityLabel("Deck settings")
        disclosureButton.setButtonType(.pushOnPushOff)
        disclosureButton.bezelStyle = .disclosure
        disclosureButton.title = ""
        disclosureButton.target = self
        disclosureButton.action = #selector(disclosurePressed(_:))
        disclosureButton.setAccessibilityIdentifier("deck-card-toggle")
        disclosureButton.setAccessibilityLabel("Deck settings")
        titleLabel.font = .systemFont(ofSize: 13, weight: .semibold)
        titleLabel.setContentHuggingPriority(.defaultHigh, for: .horizontal)
        chipStack.orientation = .horizontal
        chipStack.spacing = 6
        chipStack.alignment = .centerY
        chipStack.setAccessibilityIdentifier("deck-card-chips")
        chipStack.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
        modeControl.selectedSegment = 0
        modeControl.segmentStyle = .rounded
        modeControl.controlSize = .small
        modeControl.target = self
        modeControl.action = #selector(modeChanged(_:))
        modeControl.setAccessibilityIdentifier("deck-card-mode")
        modeControl.isHidden = true
        let spacer = NSView()
        spacer.setContentHuggingPriority(.init(1), for: .horizontal)
        spacer.setContentCompressionResistancePriority(.init(1), for: .horizontal)
        let row = NSStackView(views: [disclosureButton, titleLabel, chipStack, spacer, modeControl])
        row.orientation = .horizontal
        row.alignment = .centerY
        row.spacing = 8
        row.edgeInsets = NSEdgeInsets(top: 0, left: 10, bottom: 0, right: 12)
        row.translatesAutoresizingMaskIntoConstraints = false
        header.addSubview(row)
        header.onClick = { [weak self] in self?.onToggle?() }
        bodyView.isHidden = true
        for view in [header, bodyView] as [NSView] {
            view.translatesAutoresizingMaskIntoConstraints = false
            addSubview(view)
        }
        bodyHeight = bodyView.heightAnchor.constraint(equalToConstant: 0)
        NSLayoutConstraint.activate([
            header.topAnchor.constraint(equalTo: topAnchor),
            header.leadingAnchor.constraint(equalTo: leadingAnchor),
            header.trailingAnchor.constraint(equalTo: trailingAnchor),
            header.heightAnchor.constraint(equalToConstant: EditorTextView.deckCardHeaderHeight),
            row.topAnchor.constraint(equalTo: header.topAnchor),
            row.bottomAnchor.constraint(equalTo: header.bottomAnchor),
            row.leadingAnchor.constraint(equalTo: header.leadingAnchor),
            row.trailingAnchor.constraint(equalTo: header.trailingAnchor),
            bodyView.topAnchor.constraint(equalTo: header.bottomAnchor),
            bodyView.leadingAnchor.constraint(equalTo: leadingAnchor),
            bodyView.trailingAnchor.constraint(equalTo: trailingAnchor),
            bodyHeight,
        ])
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { nil }

    /// Puts the form in the body, as wide and as tall as it.
    func setBodyContent(_ content: NSView) {
        content.translatesAutoresizingMaskIntoConstraints = false
        bodyView.addSubview(content)
        NSLayoutConstraint.activate([
            content.topAnchor.constraint(equalTo: bodyView.topAnchor),
            content.leadingAnchor.constraint(equalTo: bodyView.leadingAnchor),
            content.trailingAnchor.constraint(equalTo: bodyView.trailingAnchor),
            content.bottomAnchor.constraint(equalTo: bodyView.bottomAnchor),
        ])
    }

    /// Gives the body its width, so the form inside can be measured before the card is sized to it.
    func layoutBodyForMeasuring() {
        bodyView.isHidden = false
        layoutSubtreeIfNeeded()
    }

    func setChips(_ chips: [DeckProblems.Chip]) {
        for view in chipViews {
            chipStack.removeArrangedSubview(view)
            view.removeFromSuperview()
        }
        chipViews = chips.enumerated().map { index, chip in
            let view = DeckChipView(chip)
            view.setAccessibilityIdentifier("deck-chip-\(index)")
            // When the row is short the last chip gives way first, so the first one reads whole.
            view.setCompressionPriority(.init(Float(max(251, 700 - index))))
            return view
        }
        for view in chipViews { chipStack.addArrangedSubview(view) }
    }

    /// What the card shows: the disclosure's state, the switch (only when open), and the body (only in Form mode).
    func setState(display: EditorTextView.DeckCardDisplay, bodyHeight height: CGFloat, textIsAvailable: Bool) {
        disclosureButton.state = display == .collapsed ? .off : .on
        modeControl.isHidden = display == .collapsed
        modeControl.selectedSegment = display == .text ? 1 : 0
        modeControl.setEnabled(textIsAvailable, forSegment: 1)
        bodyView.isHidden = display != .form
        bodyHeight.constant = display == .form ? height : 0
        setAccessibilityValue(display == .collapsed ? "collapsed" : "expanded")
        needsLayout = true
    }

    @objc private func disclosurePressed(_ sender: NSButton) {
        onToggle?()
    }

    @objc private func modeChanged(_ sender: NSSegmentedControl) {
        onModeChange?(sender.selectedSegment == 1)
    }

    /// The card takes only the clicks on its own views: never one meant for the editor's text.
    override func hitTest(_ point: NSPoint) -> NSView? {
        let hit = super.hitTest(point)
        return hit === self ? nil : hit
    }
}
