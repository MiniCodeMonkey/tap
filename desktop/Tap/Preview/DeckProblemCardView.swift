import AppKit

/// Lies over the preview while the deck's settings hold a mistake that
/// keeps tap from rendering it: which setting, in plain words, the fix
/// as a button, and a way to the field in the editor. The raw line of
/// each setting is behind Details. The preview comes back by itself once
/// the settings are fixed.
final class DeckProblemCardView: NSView {
    let glyphView = NSTextField(labelWithString: "!")
    let titleLabel = NSTextField(labelWithString: "")
    let problemStack = NSStackView()
    let showInEditorButton = NSButton(title: "Show in Editor", target: nil, action: nil)
    let noteLabel = NSTextField(wrappingLabelWithString: "The preview comes back by itself once the setting is fixed.")
    let detailsButton = NSButton(title: "Details", target: nil, action: nil)
    let detailsLabel = NSTextField(wrappingLabelWithString: "")
    private(set) var fixButtons: [String: NSButton] = [:]
    private(set) var problems: [DeckProblem] = []
    private var fixActions: [ObjectIdentifier: () -> Void] = [:]
    /// The button for the first problem's fix, or Show in Editor's, was pressed.
    var onShowInEditor: ((DeckProblem) -> Void)?

    override init(frame: NSRect) {
        super.init(frame: frame)
        wantsLayer = true
        layer?.cornerRadius = 8
        setAccessibilityIdentifier("problem-card")
        glyphView.font = .systemFont(ofSize: 15, weight: .bold)
        glyphView.textColor = EditorPalette.error
        glyphView.alignment = .center
        glyphView.setAccessibilityIdentifier("problem-card-glyph")
        titleLabel.font = .systemFont(ofSize: 15, weight: .semibold)
        titleLabel.setAccessibilityIdentifier("problem-card-title")
        problemStack.orientation = .vertical
        problemStack.alignment = .leading
        problemStack.spacing = 8
        showInEditorButton.bezelStyle = .rounded
        showInEditorButton.target = self
        showInEditorButton.action = #selector(showInEditorPressed(_:))
        showInEditorButton.setAccessibilityIdentifier("problem-show-in-editor")
        noteLabel.font = .systemFont(ofSize: 11)
        noteLabel.textColor = .secondaryLabelColor
        noteLabel.alignment = .center
        noteLabel.setAccessibilityIdentifier("problem-card-note")
        detailsButton.setButtonType(.pushOnPushOff)
        detailsButton.bezelStyle = .disclosure
        detailsButton.title = ""
        detailsButton.target = self
        detailsButton.action = #selector(detailsPressed(_:))
        detailsButton.setAccessibilityIdentifier("problem-card-details")
        detailsButton.setAccessibilityLabel("Details")
        let detailsTitle = NSTextField(labelWithString: "Details")
        detailsTitle.font = .systemFont(ofSize: 11)
        detailsTitle.textColor = .secondaryLabelColor
        let detailsRow = NSStackView(views: [detailsButton, detailsTitle])
        detailsRow.spacing = 2
        detailsLabel.font = .monospacedSystemFont(ofSize: 11, weight: .regular)
        detailsLabel.textColor = .secondaryLabelColor
        detailsLabel.isSelectable = true
        detailsLabel.isHidden = true
        detailsLabel.setAccessibilityIdentifier("problem-card-raw")
        let stack = NSStackView(views: [glyphView, titleLabel, problemStack, showInEditorButton, noteLabel, detailsRow, detailsLabel])
        stack.orientation = .vertical
        stack.alignment = .centerX
        stack.spacing = 8
        stack.setCustomSpacing(12, after: problemStack)
        stack.translatesAutoresizingMaskIntoConstraints = false
        addSubview(stack)
        NSLayoutConstraint.activate([
            stack.centerXAnchor.constraint(equalTo: centerXAnchor),
            stack.centerYAnchor.constraint(equalTo: centerYAnchor),
            stack.leadingAnchor.constraint(greaterThanOrEqualTo: leadingAnchor, constant: 20),
            stack.widthAnchor.constraint(lessThanOrEqualToConstant: 420),
            problemStack.widthAnchor.constraint(equalTo: stack.widthAnchor),
            noteLabel.widthAnchor.constraint(lessThanOrEqualTo: stack.widthAnchor),
            detailsLabel.widthAnchor.constraint(lessThanOrEqualTo: stack.widthAnchor),
        ])
        isHidden = true
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { nil }

    override func updateLayer() {
        layer?.backgroundColor = NSColor.windowBackgroundColor.withAlphaComponent(0.97).cgColor
    }

    override var wantsUpdateLayer: Bool { true }

    /// Shows the problems: the heading counts them, each has its words and
    /// its fix (`fixTitle` nil offers none), and Details holds the raw
    /// line of each setting.
    func show(problems newProblems: [DeckProblem], fixTitle: (DeckProblem) -> String?, rawLine: (DeckProblem) -> String?, fix: @escaping (DeckProblem) -> Void) {
        problems = newProblems
        titleLabel.stringValue = DeckProblems.heading(errorCount: newProblems.count)
        for view in problemStack.arrangedSubviews {
            problemStack.removeArrangedSubview(view)
            view.removeFromSuperview()
        }
        fixButtons = [:]
        fixActions = [:]
        let single = newProblems.count == 1
        for problem in newProblems {
            let message = NSTextField(wrappingLabelWithString: problem.message)
            message.font = .systemFont(ofSize: 12)
            message.alignment = single ? .center : .left
            message.setAccessibilityIdentifier("problem-message-\(problem.key)")
            message.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
            var views: [NSView] = [message]
            if let title = fixTitle(problem) {
                let button = NSButton(title: title, target: self, action: #selector(fixPressed(_:)))
                button.bezelStyle = .rounded
                button.setContentHuggingPriority(.defaultHigh, for: .horizontal)
                button.setAccessibilityIdentifier("problem-fix-\(problem.key)")
                if single { button.keyEquivalent = "" }
                fixActions[ObjectIdentifier(button)] = { fix(problem) }
                fixButtons[problem.key] = button
                views.append(button)
            }
            if single {
                let column = NSStackView(views: views)
                column.orientation = .vertical
                column.alignment = .centerX
                column.spacing = 10
                problemStack.addArrangedSubview(column)
                column.widthAnchor.constraint(equalTo: problemStack.widthAnchor).isActive = true
            } else {
                let row = NSStackView(views: views)
                row.orientation = .horizontal
                row.alignment = .firstBaseline
                row.spacing = 10
                problemStack.addArrangedSubview(row)
                row.widthAnchor.constraint(equalTo: problemStack.widthAnchor).isActive = true
            }
        }
        detailsLabel.stringValue = newProblems.compactMap(rawLine).joined(separator: "\n")
        detailsButton.superview?.isHidden = detailsLabel.stringValue.isEmpty
        detailsLabel.isHidden = true
        detailsButton.state = .off
        isHidden = false
    }

    func hide() {
        isHidden = true
    }

    @objc private func fixPressed(_ sender: NSButton) {
        fixActions[ObjectIdentifier(sender)]?()
    }

    @objc private func showInEditorPressed(_ sender: NSButton) {
        guard let first = problems.first else { return }
        onShowInEditor?(first)
    }

    @objc private func detailsPressed(_ sender: NSButton) {
        detailsLabel.isHidden = sender.state != .on
    }
}

/// A quiet amber band above the preview: the deck still renders, with a
/// fallback, and this says which setting is why and how to fix it.
final class PreviewBannerView: NSView {
    let iconLabel = NSTextField(labelWithString: "!")
    let messageLabel = NSTextField(wrappingLabelWithString: "")
    let fixButton = NSButton(title: "", target: nil, action: nil)
    let chooseButton = NSButton(title: "Choose Theme\u{2026}", target: nil, action: nil)
    var onFix: (() -> Void)?
    var onChoose: (() -> Void)?

    override init(frame: NSRect) {
        super.init(frame: frame)
        wantsLayer = true
        layer?.cornerRadius = 8
        layer?.borderWidth = 1
        setAccessibilityIdentifier("theme-banner")
        iconLabel.font = .systemFont(ofSize: 12, weight: .bold)
        iconLabel.textColor = EditorPalette.warning
        iconLabel.setContentHuggingPriority(.defaultHigh, for: .horizontal)
        messageLabel.font = .systemFont(ofSize: 12)
        messageLabel.setAccessibilityIdentifier("theme-banner-message")
        messageLabel.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
        for button in [fixButton, chooseButton] {
            button.bezelStyle = .rounded
            button.controlSize = .small
        }
        fixButton.target = self
        fixButton.action = #selector(fixPressed(_:))
        fixButton.setAccessibilityIdentifier("theme-banner-fix")
        chooseButton.target = self
        chooseButton.action = #selector(choosePressed(_:))
        chooseButton.setAccessibilityIdentifier("theme-banner-choose")
        let buttons = NSStackView(views: [fixButton, chooseButton])
        buttons.spacing = 6
        let text = NSStackView(views: [messageLabel, buttons])
        text.orientation = .vertical
        text.alignment = .leading
        text.spacing = 6
        let row = NSStackView(views: [iconLabel, text])
        row.alignment = .firstBaseline
        row.spacing = 8
        row.edgeInsets = NSEdgeInsets(top: 8, left: 10, bottom: 8, right: 10)
        row.translatesAutoresizingMaskIntoConstraints = false
        addSubview(row)
        NSLayoutConstraint.activate([
            row.topAnchor.constraint(equalTo: topAnchor),
            row.bottomAnchor.constraint(equalTo: bottomAnchor),
            row.leadingAnchor.constraint(equalTo: leadingAnchor),
            row.trailingAnchor.constraint(equalTo: trailingAnchor),
        ])
        isHidden = true
    }

    @available(*, unavailable)
    required init?(coder: NSCoder) { nil }

    override var wantsUpdateLayer: Bool { true }

    override func updateLayer() {
        layer?.backgroundColor = EditorPalette.warningTint.cgColor
        layer?.borderColor = EditorPalette.warningTintBorder.cgColor
    }

    /// Says `name` is not a theme, offering `fixTitle` (nil for none) and the theme picker.
    func show(unknownTheme name: String, fixTitle: String?) {
        let text = NSMutableAttributedString(string: "\u{201C}\(name)\u{201D} is not a tap theme.",
                                             attributes: [.font: NSFont.systemFont(ofSize: 12, weight: .semibold), .foregroundColor: NSColor.labelColor])
        text.append(NSAttributedString(string: " The preview uses Base for now.",
                                       attributes: [.font: NSFont.systemFont(ofSize: 12), .foregroundColor: NSColor.labelColor]))
        messageLabel.attributedStringValue = text
        fixButton.title = fixTitle ?? ""
        fixButton.isHidden = fixTitle == nil
        isHidden = false
    }

    @objc private func fixPressed(_ sender: NSButton) { onFix?() }
    @objc private func choosePressed(_ sender: NSButton) { onChoose?() }
}
