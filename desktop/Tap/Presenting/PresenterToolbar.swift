import AppKit

/// The app's toolbar over the presenter page: REC, the edits the audience
/// has not seen, Reload Slides, Swap Displays, Phone Remote and Stop. It is out of sight
/// while the speaker talks and slides up when the pointer reaches the
/// bottom edge, then slides away once the pointer has left it. The bottom,
/// not the top: in system full screen the menu bar drops over the top
/// edge when the pointer rests there, and would cover a toolbar (the
/// person's decision 7, 2026-09-25).
final class PresenterToolbar: NSView {
    static let height: CGFloat = 52
    let titleLabel = NSTextField(labelWithString: "")
    let recordButton = NSButton(title: "NOT RECORDING", target: nil, action: nil)
    let editsLabel = NSTextField(labelWithString: "")
    let reloadButton = NSButton(title: "Reload Slides", target: nil, action: nil)
    let swapButton = NSButton(title: "Swap Displays", target: nil, action: nil)
    let phoneRemoteButton = NSButton(title: "Phone Remote", target: nil, action: nil)
    let stopButton = NSButton(title: "Stop", target: nil, action: nil)
    /// How long the pointer must be away before the toolbar slides off. Tests shorten it.
    var hideDelay: TimeInterval = 1.5
    private(set) var isShown = false
    var onRecord: (() -> Void)?
    var onReload: (() -> Void)?
    var onSwap: (() -> Void)?
    var onPhoneRemote: (() -> Void)?
    var onStop: (() -> Void)?
    private var hideWork: DispatchWorkItem?

    override init(frame: NSRect) {
        super.init(frame: frame)
        wantsLayer = true
        layer?.backgroundColor = NSColor(white: 0.12, alpha: 0.92).cgColor
        setAccessibilityIdentifier("presenter-toolbar")
        titleLabel.font = .systemFont(ofSize: 13, weight: .semibold)
        titleLabel.textColor = .white
        editsLabel.font = .systemFont(ofSize: 12)
        editsLabel.textColor = .secondaryLabelColor
        for button in [recordButton, reloadButton, swapButton, phoneRemoteButton, stopButton] {
            button.bezelStyle = .rounded
            button.target = self
        }
        recordButton.action = #selector(recordPressed(_:))
        reloadButton.action = #selector(reloadPressed(_:))
        swapButton.action = #selector(swapPressed(_:))
        phoneRemoteButton.action = #selector(phoneRemotePressed(_:))
        phoneRemoteButton.setButtonType(.pushOnPushOff)
        stopButton.action = #selector(stopPressed(_:))
        stopButton.hasDestructiveAction = true
        recordButton.setAccessibilityIdentifier("record-button")
        reloadButton.setAccessibilityIdentifier("reload-slides-button")
        swapButton.setAccessibilityIdentifier("swap-displays-button")
        phoneRemoteButton.setAccessibilityIdentifier("phone-remote-button")
        stopButton.setAccessibilityIdentifier("stop-button")
        let row = NSStackView(views: [titleLabel, recordButton, NSView(), editsLabel, reloadButton, swapButton, phoneRemoteButton, stopButton])
        row.orientation = .horizontal
        row.spacing = 10
        row.edgeInsets = NSEdgeInsets(top: 0, left: 16, bottom: 0, right: 16)
        row.translatesAutoresizingMaskIntoConstraints = false
        addSubview(row)
        NSLayoutConstraint.activate([
            row.topAnchor.constraint(equalTo: topAnchor),
            row.leadingAnchor.constraint(equalTo: leadingAnchor),
            row.trailingAnchor.constraint(equalTo: trailingAnchor),
            row.bottomAnchor.constraint(equalTo: bottomAnchor),
        ])
        isHidden = true
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    /// The pointer touched the bottom edge: the toolbar comes up and stays while the pointer is on it.
    func pointerReachedBottomEdge() {
        hideWork?.cancel()
        hideWork = nil
        isShown = true
        isHidden = false
    }

    /// The pointer left the toolbar: it goes after `hideDelay`, unless the pointer comes back first.
    func pointerLeft() {
        guard isShown else { return }
        hideWork?.cancel()
        let work = DispatchWorkItem { [weak self] in
            MainActor.assumeIsolated {
                guard let self else { return }
                self.isShown = false
                self.isHidden = true
            }
        }
        hideWork = work
        DispatchQueue.main.asyncAfter(deadline: .now() + hideDelay, execute: work)
    }

    /// What the talk is now. A rehearsal has no recording and nothing to swap.
    func update(recording: RecordingStatus, editsNotShown: Int, mode: PresentationMode) {
        titleLabel.stringValue = mode == .rehearse ? "Rehearsal" : "Presenting"
        recordButton.title = recording.label
        recordButton.isHidden = mode == .rehearse
        swapButton.isHidden = mode == .rehearse
        editsLabel.isHidden = editsNotShown == 0
        editsLabel.stringValue = editsNotShown == 1 ? "1 edit not shown" : "\(editsNotShown) edits not shown"
    }

    /// The phone remote's state, drawn as the button's on state, and
    /// whether it can be toggled now (Present > Phone Remote's validation).
    func updateRemote(isOn: Bool, isEnabled: Bool) {
        phoneRemoteButton.state = isOn ? .on : .off
        phoneRemoteButton.isEnabled = isEnabled
    }

    @objc private func recordPressed(_ sender: Any?) { onRecord?() }
    @objc private func reloadPressed(_ sender: Any?) { onReload?() }
    @objc private func swapPressed(_ sender: Any?) { onSwap?() }
    /// The click flips the on state at once; tap's tunnel events set it back to what is true.
    @objc private func phoneRemotePressed(_ sender: Any?) { onPhoneRemote?() }
    @objc private func stopPressed(_ sender: Any?) { onStop?() }
}

/// The small red dot with REC that stays in the top-right corner of the
/// presenter window for as long as tap records.
final class RecordingDot: NSView {
    private let label = NSTextField(labelWithString: "REC")

    override init(frame: NSRect) {
        super.init(frame: frame)
        wantsLayer = true
        layer?.cornerRadius = 12
        layer?.backgroundColor = NSColor(white: 0, alpha: 0.55).cgColor
        setAccessibilityIdentifier("recording-dot")
        let dot = NSView()
        dot.wantsLayer = true
        dot.layer?.cornerRadius = 4
        dot.layer?.backgroundColor = NSColor.systemRed.cgColor
        dot.translatesAutoresizingMaskIntoConstraints = false
        label.font = .systemFont(ofSize: 12, weight: .bold)
        label.textColor = NSColor(red: 1, green: 0.41, blue: 0.38, alpha: 1)
        let row = NSStackView(views: [dot, label])
        row.orientation = .horizontal
        row.spacing = 6
        row.edgeInsets = NSEdgeInsets(top: 4, left: 10, bottom: 4, right: 10)
        row.translatesAutoresizingMaskIntoConstraints = false
        addSubview(row)
        NSLayoutConstraint.activate([
            dot.widthAnchor.constraint(equalToConstant: 8),
            dot.heightAnchor.constraint(equalToConstant: 8),
            row.topAnchor.constraint(equalTo: topAnchor),
            row.leadingAnchor.constraint(equalTo: leadingAnchor),
            row.trailingAnchor.constraint(equalTo: trailingAnchor),
            row.bottomAnchor.constraint(equalTo: bottomAnchor),
        ])
        isHidden = true
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }
}
