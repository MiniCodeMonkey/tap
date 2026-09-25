import AppKit

/// The popover's picture of the displays: two boxes, each the shape of its
/// screen, labelled with its role and its name.
final class DisplayArrangementView: NSView {
    final class ScreenBox: NSView {
        let roleLabel = NSTextField(labelWithString: "")
        let nameLabel = NSTextField(labelWithString: "")
        private var aspect: NSLayoutConstraint?

        init(role: String, accessibilityIdentifier: String) {
            super.init(frame: .zero)
            wantsLayer = true
            layer?.cornerRadius = 8
            layer?.borderWidth = 1
            layer?.borderColor = NSColor.separatorColor.cgColor
            layer?.backgroundColor = NSColor.controlBackgroundColor.cgColor
            roleLabel.stringValue = role
            roleLabel.font = .systemFont(ofSize: 11, weight: .semibold)
            nameLabel.font = .systemFont(ofSize: 11)
            nameLabel.textColor = .secondaryLabelColor
            nameLabel.lineBreakMode = .byTruncatingTail
            let stack = NSStackView(views: [roleLabel, nameLabel])
            stack.orientation = .vertical
            stack.alignment = .centerX
            stack.spacing = 2
            stack.translatesAutoresizingMaskIntoConstraints = false
            addSubview(stack)
            NSLayoutConstraint.activate([
                stack.centerXAnchor.constraint(equalTo: centerXAnchor),
                stack.centerYAnchor.constraint(equalTo: centerYAnchor),
                stack.leadingAnchor.constraint(greaterThanOrEqualTo: leadingAnchor, constant: 6),
                widthAnchor.constraint(equalToConstant: 150),
            ])
            setAccessibilityElement(true)
            setAccessibilityRole(.group)
            setAccessibilityIdentifier(accessibilityIdentifier)
        }

        required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

        func show(_ screen: ScreenInfo) {
            nameLabel.stringValue = screen.name
            setAccessibilityLabel("\(roleLabel.stringValue), \(screen.name)")
            aspect?.isActive = false
            let ratio = screen.frame.width > 0 ? max(screen.frame.height / screen.frame.width, 0.3) : 0.625
            aspect = heightAnchor.constraint(equalTo: widthAnchor, multiplier: ratio)
            aspect?.isActive = true
        }
    }

    let presenterBox = ScreenBox(role: "Presenter view", accessibilityIdentifier: "presenter-screen")
    let audienceBox = ScreenBox(role: "Audience, full screen", accessibilityIdentifier: "audience-screen")

    override init(frame: NSRect) {
        super.init(frame: frame)
        let row = NSStackView(views: [presenterBox, audienceBox])
        row.orientation = .horizontal
        row.alignment = .bottom
        row.spacing = 12
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

    func update(arrangement: DisplayArrangement) {
        presenterBox.show(arrangement.presenter)
        audienceBox.show(arrangement.audience)
    }
}
