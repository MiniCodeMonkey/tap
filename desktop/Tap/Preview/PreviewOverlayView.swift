import AppKit

/// Lies over the preview while tap is down: see-through while tap restarts,
/// so the last good render shows, and opaque with tap's last output once
/// the app stops retrying.
final class PreviewOverlayView: NSView {
    let titleLabel = NSTextField(labelWithString: "")
    let detailLabel = NSTextField(labelWithString: "")
    let outputLabel = NSTextField(wrappingLabelWithString: "")
    let tryAgainButton = NSButton(title: "Try Again", target: nil, action: nil)
    let showLogButton = NSButton(title: "Show Tap Log", target: nil, action: nil)

    override init(frame: NSRect) {
        super.init(frame: frame)
        wantsLayer = true
        layer?.cornerRadius = 8
        setAccessibilityIdentifier("preview-overlay")
        titleLabel.font = .systemFont(ofSize: 15, weight: .semibold)
        detailLabel.font = .systemFont(ofSize: 12)
        detailLabel.textColor = .secondaryLabelColor
        outputLabel.font = .monospacedSystemFont(ofSize: 11, weight: .regular)
        outputLabel.textColor = .secondaryLabelColor
        tryAgainButton.bezelStyle = .rounded
        tryAgainButton.keyEquivalent = "\r"
        showLogButton.bezelStyle = .rounded
        let buttons = NSStackView(views: [tryAgainButton, showLogButton])
        buttons.spacing = 8
        let stack = NSStackView(views: [titleLabel, detailLabel, outputLabel, buttons])
        stack.orientation = .vertical
        stack.alignment = .centerX
        stack.spacing = 8
        stack.translatesAutoresizingMaskIntoConstraints = false
        addSubview(stack)
        NSLayoutConstraint.activate([
            stack.centerXAnchor.constraint(equalTo: centerXAnchor),
            stack.centerYAnchor.constraint(equalTo: centerYAnchor),
            stack.leadingAnchor.constraint(greaterThanOrEqualTo: leadingAnchor, constant: 24),
            outputLabel.widthAnchor.constraint(lessThanOrEqualToConstant: 520),
        ])
        isHidden = true
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    func show(title: String, detail: String, output: [String], opaque: Bool, buttons: Bool) {
        titleLabel.stringValue = title
        detailLabel.stringValue = detail
        outputLabel.stringValue = output.joined(separator: "\n")
        outputLabel.isHidden = output.isEmpty
        tryAgainButton.isHidden = !buttons
        showLogButton.isHidden = !buttons
        layer?.backgroundColor = NSColor.windowBackgroundColor.withAlphaComponent(opaque ? 0.97 : 0.55).cgColor
        isHidden = false
    }

    func hide() {
        isHidden = true
        titleLabel.stringValue = ""
    }
}
