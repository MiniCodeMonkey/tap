import AppKit

/// A bar at the top of the editor: a message, a detail line and buttons.
final class DocumentBarView: NSView {
    enum Kind: Equatable {
        case changedOnDisk, deleted, deckErrors, environmentNotice
    }

    let kind: Kind
    let message: String
    private var buttons: [NSButton] = []
    private var actions: [() -> Void] = []

    init(kind: Kind, message: String, detail: String, buttons buttonSpecifications: [(String, () -> Void)]) {
        self.kind = kind
        self.message = message
        super.init(frame: .zero)
        wantsLayer = true
        layer?.backgroundColor = NSColor.controlBackgroundColor.withAlphaComponent(0.95).cgColor
        setAccessibilityIdentifier("bar-\(kind)")

        let title = NSTextField(labelWithString: message)
        title.font = .systemFont(ofSize: 12, weight: .semibold)
        let subtitle = NSTextField(labelWithString: detail)
        subtitle.font = .systemFont(ofSize: 11)
        subtitle.textColor = .secondaryLabelColor
        let text = NSStackView(views: [title, subtitle])
        text.orientation = .vertical
        text.alignment = .leading
        text.spacing = 1

        for (index, specification) in buttonSpecifications.enumerated() {
            let button = NSButton(title: specification.0, target: self, action: #selector(buttonPressed(_:)))
            button.tag = index
            button.bezelStyle = .rounded
            buttons.append(button)
            actions.append(specification.1)
        }
        let row = NSStackView(views: [text, NSView()] + buttons)
        row.orientation = .horizontal
        row.spacing = 8
        row.edgeInsets = NSEdgeInsets(top: 8, left: 16, bottom: 8, right: 16)
        row.translatesAutoresizingMaskIntoConstraints = false
        addSubview(row)
        NSLayoutConstraint.activate([
            row.topAnchor.constraint(equalTo: topAnchor),
            row.leadingAnchor.constraint(equalTo: leadingAnchor),
            row.trailingAnchor.constraint(equalTo: trailingAnchor),
            row.bottomAnchor.constraint(equalTo: bottomAnchor),
        ])
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    func button(titled title: String) -> NSButton? {
        buttons.first { $0.title == title }
    }

    @objc private func buttonPressed(_ sender: NSButton) {
        actions[sender.tag]()
    }
}

extension EditorViewController {
    func showBar(_ bar: DocumentBarView) {
        hideBar(bar.kind)
        barStack.addArrangedSubview(bar)
        bar.widthAnchor.constraint(equalTo: barStack.widthAnchor).isActive = true
        view.needsLayout = true
    }

    func hideBar(_ kind: DocumentBarView.Kind) {
        for case let bar as DocumentBarView in barStack.arrangedSubviews where bar.kind == kind {
            barStack.removeArrangedSubview(bar)
            bar.removeFromSuperview()
        }
        view.needsLayout = true
    }

    func bar(_ kind: DocumentBarView.Kind) -> DocumentBarView? {
        barStack.arrangedSubviews.compactMap { $0 as? DocumentBarView }.first { $0.kind == kind }
    }
}
