import AppKit

/// The phone remote: the QR code and URL tap made for the tunnel, over the
/// presenter window, with Turn Off Remote. The QR image comes from tap.
final class RemotePanel: NSPanel {
    let qrImageView = NSImageView()
    let urlLabel = NSTextField(labelWithString: "")
    let noteLabel = NSTextField(wrappingLabelWithString: "")
    let messageLabel = NSTextField(wrappingLabelWithString: "")
    let turnOffButton = NSButton(title: "Turn Off Remote", target: nil, action: nil)
    var onTurnOff: (() -> Void)?

    init() {
        super.init(contentRect: NSRect(x: 0, y: 0, width: 360, height: 460), styleMask: [.titled, .nonactivatingPanel], backing: .buffered, defer: false)
        title = "Phone remote"
        // A floating panel that may show over a full screen Space, and moves
        // to whichever Space is active when it is ordered front.
        level = .floating
        collectionBehavior = [.fullScreenAuxiliary, .moveToActiveSpace]
        isReleasedWhenClosed = false
        isFloatingPanel = true
        setAccessibilityIdentifier("remote-panel")
        let heading = NSTextField(labelWithString: "Scan to control the talk from your phone")
        heading.font = .systemFont(ofSize: 15, weight: .bold)
        qrImageView.imageScaling = .scaleProportionallyUpOrDown
        qrImageView.setAccessibilityIdentifier("remote-qr")
        urlLabel.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
        urlLabel.isSelectable = true
        urlLabel.lineBreakMode = .byTruncatingMiddle
        noteLabel.font = .systemFont(ofSize: 12)
        noteLabel.textColor = .secondaryLabelColor
        messageLabel.font = .systemFont(ofSize: 12)
        messageLabel.textColor = .systemRed
        turnOffButton.bezelStyle = .rounded
        turnOffButton.target = self
        turnOffButton.action = #selector(turnOffPressed(_:))
        let stack = NSStackView(views: [heading, qrImageView, urlLabel, noteLabel, messageLabel, turnOffButton])
        stack.orientation = .vertical
        stack.alignment = .centerX
        stack.spacing = 12
        stack.edgeInsets = NSEdgeInsets(top: 20, left: 20, bottom: 20, right: 20)
        stack.widthAnchor.constraint(equalToConstant: 360).isActive = true
        qrImageView.widthAnchor.constraint(equalToConstant: 256).isActive = true
        qrImageView.heightAnchor.constraint(equalToConstant: 256).isActive = true
        for label in [urlLabel, noteLabel, messageLabel] {
            label.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -40).isActive = true
        }
        contentView = stack
        setContentSize(stack.fittingSize)
    }

    /// Shows tap's tunnel, or the reason there is none, centred on `frame`.
    func show(tunnel: TunnelEvent?, error: String?, ownPassword: Bool, on frame: CGRect) {
        if let tunnel, tunnel.state == "running" {
            urlLabel.stringValue = tunnel.url ?? ""
            qrImageView.image = tunnel.qr.flatMap { Data(base64Encoded: $0) }.flatMap { NSImage(data: $0) }
            messageLabel.isHidden = true
        } else {
            urlLabel.stringValue = tunnel?.state == "starting" ? "Starting the tunnel…" : ""
            qrImageView.image = nil
            messageLabel.stringValue = error ?? ""
            messageLabel.isHidden = error == nil
        }
        noteLabel.stringValue = ownPassword
            ? "Only a phone with your presenter password can control the talk."
            : "tap made a presenter password for this talk, so only this code works."
        setContentSize(contentView?.fittingSize ?? frame.size)
        setFrameOrigin(NSPoint(x: frame.midX - self.frame.width / 2, y: frame.midY - self.frame.height / 2))
        orderFrontRegardless()
    }

    func hide() {
        orderOut(nil)
    }

    @objc private func turnOffPressed(_ sender: Any?) {
        onTurnOff?()
    }
}
