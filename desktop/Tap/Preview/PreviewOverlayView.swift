import AppKit

/// Lies over the preview while tap is down: see-through while tap restarts,
/// so the last good render shows, and opaque with tap's last output once
/// the app stops retrying.
final class PreviewOverlayView: NSView {
    let titleLabel = NSTextField(labelWithString: "")
    let detailLabel = NSTextField(labelWithString: "")
    let outputLabel = NSTextField(wrappingLabelWithString: "")
    /// Restart Preview: tap starts again, with a fresh count.
    let tryAgainButton = NSButton(title: "Restart Preview", target: nil, action: nil)
    /// Opens the raw output under the notice.
    let detailsButton = NSButton(title: "", target: nil, action: nil)
    private let detailsRow = NSStackView()
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
        detailsButton.setButtonType(.pushOnPushOff)
        detailsButton.bezelStyle = .disclosure
        detailsButton.target = self
        detailsButton.action = #selector(detailsPressed(_:))
        detailsButton.setAccessibilityIdentifier("preview-overlay-details")
        detailsButton.setAccessibilityLabel("Details")
        let detailsTitle = NSTextField(labelWithString: "Details")
        detailsTitle.font = .systemFont(ofSize: 11)
        detailsTitle.textColor = .secondaryLabelColor
        detailsRow.setViews([detailsButton, detailsTitle], in: .leading)
        detailsRow.spacing = 2
        let stack = NSStackView(views: [titleLabel, detailLabel, buttons, detailsRow, outputLabel])
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

    /// Runs whenever the overlay shows: the preview then has something to say, so its loading state ends.
    var onShow: (() -> Void)?

    func show(title: String, detail: String, output: [String], opaque: Bool, buttons: Bool) {
        titleLabel.stringValue = title
        detailLabel.stringValue = detail
        outputLabel.stringValue = output.joined(separator: "\n")
        // The raw output is never on the notice itself: it is behind Details.
        outputLabel.isHidden = true
        detailsButton.state = .off
        detailsRow.isHidden = output.isEmpty
        tryAgainButton.isHidden = !buttons
        showLogButton.isHidden = !buttons
        layer?.backgroundColor = NSColor.windowBackgroundColor.withAlphaComponent(opaque ? 0.97 : 0.55).cgColor
        isHidden = false
        onShow?()
    }

    func hide() {
        isHidden = true
        titleLabel.stringValue = ""
    }

    @objc private func detailsPressed(_ sender: NSButton) {
        outputLabel.isHidden = sender.state != .on
    }
}
