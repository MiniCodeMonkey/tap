import AppKit

/// The Present popover: which display is which, Swap Displays, where to
/// start, recording, the phone remote, and the Advanced options, with
/// Rehearse and Start Presenting. It collects `PresentationOptions`; the
/// deck's window controller starts the talk.
@MainActor
final class PresentPopoverController: NSObject, NSPopoverDelegate {
    struct Context {
        let arrangement: DisplayArrangement?
        let cursorSlide: Int
        /// False when two displays share one Space, so the talk windows cannot be full screen.
        let usesFullScreen: Bool
    }

    /// Kept here rather than read from the popover, whose `isShown` can
    /// depend on whether the app is active (D3's gallery found the same).
    private(set) var isShown = false
    let arrangementView = DisplayArrangementView(frame: .zero)
    let singleDisplayLabel = NSTextField(wrappingLabelWithString: "One display: the audience fills the screen, and Option-Tab shows the presenter view.")
    let separateSpacesLabel = NSTextField(wrappingLabelWithString: "Turn on \"Displays have separate Spaces\" in System Settings > Desktop & Dock so each display gets its own full screen window. Until then the talk windows are plain windows over their displays.")
    let swapButton = NSButton(title: "Swap Displays", target: nil, action: nil)
    let startFromControl = NSSegmentedControl(labels: ["Slide 1", "Slide 1"], trackingMode: .selectOne, target: nil, action: nil)
    let recordCheckbox = NSButton(checkboxWithTitle: "Record the talk", target: nil, action: nil)
    let recordHint = NSTextField(wrappingLabelWithString: "Records from the start until you stop, when recording is on for you.")
    let phoneRemoteCheckbox = NSButton(checkboxWithTitle: "Phone remote", target: nil, action: nil)
    let advancedButton = NSButton(title: "Advanced", target: nil, action: nil)
    let advancedStack = NSStackView()
    let passwordField = NSSecureTextField()
    let tunnelCheckbox = NSButton(checkboxWithTitle: "Public tunnel", target: nil, action: nil)
    let tunnelHint = NSTextField(labelWithString: "Needs cloudflared.")
    let rehearseButton = NSButton(title: "Rehearse", target: nil, action: nil)
    let startButton = NSButton(title: "Start Presenting", target: nil, action: nil)
    var onSwap: (() -> Void)?
    var onStart: ((PresentationOptions) -> Void)?
    var onRehearse: ((PresentationOptions) -> Void)?
    private let popover = NSPopover()
    private var context = Context(arrangement: nil, cursorSlide: 1, usesFullScreen: true)

    override init() {
        super.init()
        singleDisplayLabel.font = .systemFont(ofSize: 12)
        singleDisplayLabel.textColor = .secondaryLabelColor
        separateSpacesLabel.font = .systemFont(ofSize: 12)
        separateSpacesLabel.textColor = .systemOrange
        separateSpacesLabel.isHidden = true
        swapButton.bezelStyle = .rounded
        swapButton.target = self
        swapButton.action = #selector(swapPressed(_:))
        startFromControl.selectedSegment = 0
        recordCheckbox.state = .on
        recordHint.font = .systemFont(ofSize: 11)
        recordHint.textColor = .secondaryLabelColor
        phoneRemoteCheckbox.target = self
        phoneRemoteCheckbox.action = #selector(phoneRemoteChanged(_:))
        // A disclosure button draws its triangle and no title; the label beside it says Advanced.
        advancedButton.bezelStyle = .disclosure
        advancedButton.setButtonType(.pushOnPushOff)
        advancedButton.title = ""
        advancedButton.target = self
        advancedButton.action = #selector(advancedPressed(_:))
        advancedButton.setAccessibilityLabel("Advanced")
        let advancedRow = NSStackView(views: [advancedButton, NSTextField(labelWithString: "Advanced")])
        advancedRow.orientation = .horizontal
        advancedRow.spacing = 4
        passwordField.placeholderString = "None"
        passwordField.setAccessibilityIdentifier("presenter-password")
        tunnelHint.font = .systemFont(ofSize: 11)
        tunnelHint.textColor = .secondaryLabelColor
        rehearseButton.bezelStyle = .rounded
        rehearseButton.target = self
        rehearseButton.action = #selector(rehearsePressed(_:))
        startButton.bezelStyle = .rounded
        startButton.keyEquivalent = "\r"
        startButton.target = self
        startButton.action = #selector(startPressed(_:))
        startButton.setAccessibilityIdentifier("start-presenting")

        let passwordRow = NSStackView(views: [NSTextField(labelWithString: "Presenter password"), passwordField])
        passwordRow.orientation = .horizontal
        passwordField.widthAnchor.constraint(equalToConstant: 160).isActive = true
        let tunnelRow = NSStackView(views: [tunnelCheckbox, tunnelHint])
        tunnelRow.orientation = .horizontal
        advancedStack.orientation = .vertical
        advancedStack.alignment = .leading
        advancedStack.spacing = 6
        advancedStack.addArrangedSubview(passwordRow)
        advancedStack.addArrangedSubview(tunnelRow)
        advancedStack.isHidden = true

        let startRow = NSStackView(views: [NSTextField(labelWithString: "Start from"), startFromControl])
        startRow.orientation = .horizontal
        let buttons = NSStackView(views: [NSView(), rehearseButton, startButton])
        buttons.orientation = .horizontal
        let stack = NSStackView(views: [arrangementView, singleDisplayLabel, separateSpacesLabel, swapButton, startRow, recordCheckbox, recordHint,
                                        phoneRemoteCheckbox, advancedRow, advancedStack, buttons])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 10
        stack.edgeInsets = NSEdgeInsets(top: 16, left: 16, bottom: 16, right: 16)
        stack.widthAnchor.constraint(equalToConstant: 360).isActive = true
        buttons.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -32).isActive = true
        singleDisplayLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -32).isActive = true
        separateSpacesLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -32).isActive = true
        recordHint.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -32).isActive = true
        let content = NSViewController()
        content.view = stack
        popover.contentViewController = content
        popover.behavior = .transient
        popover.delegate = self
        stack.setAccessibilityIdentifier("present-popover")
    }

    func show(context: Context, relativeTo rect: NSRect, of view: NSView) {
        update(context: context)
        popover.show(relativeTo: rect, of: view, preferredEdge: .maxY)
        isShown = true
    }

    /// The displays or the cursor changed, or a swap happened.
    func update(context: Context) {
        self.context = context
        startFromControl.setLabel("Slide \(context.cursorSlide)", forSegment: 0)
        startFromControl.setLabel("Slide 1", forSegment: 1)
        if let arrangement = context.arrangement, !arrangement.isSingleDisplay {
            arrangementView.update(arrangement: arrangement)
            arrangementView.isHidden = false
            singleDisplayLabel.isHidden = true
            separateSpacesLabel.isHidden = context.usesFullScreen
            swapButton.isEnabled = true
        } else {
            arrangementView.isHidden = true
            singleDisplayLabel.isHidden = false
            separateSpacesLabel.isHidden = true
            swapButton.isEnabled = false
        }
    }

    func close() {
        isShown = false
        popover.performClose(nil)
    }

    func popoverDidClose(_ notification: Notification) {
        isShown = false
    }

    /// What the controls say now.
    func options(mode: PresentationMode) -> PresentationOptions {
        settings.options(mode: mode, cursorSlide: context.cursorSlide, presenterPassword: presenterPassword)
    }

    /// The controls as settings: what a start saves, and what Cmd+Option+P starts with.
    var settings: PresentationSettings {
        PresentationSettings(startFromSlideOne: startFromControl.selectedSegment == 1,
                             record: recordCheckbox.state == .on,
                             phoneRemote: phoneRemoteCheckbox.state == .on,
                             tunnel: tunnelCheckbox.state == .on)
    }

    /// The field's password, nil when empty. It is never saved.
    var presenterPassword: String? {
        let password = passwordField.stringValue.trimmingCharacters(in: .whitespaces)
        return password.isEmpty ? nil : password
    }

    /// Puts saved settings into the controls, once, when the popover is made.
    func loadSettings(_ settings: PresentationSettings) {
        startFromControl.selectedSegment = settings.startFromSlideOne ? 1 : 0
        recordCheckbox.state = settings.record ? .on : .off
        phoneRemoteCheckbox.state = settings.phoneRemote ? .on : .off
        tunnelCheckbox.state = settings.tunnel || settings.phoneRemote ? .on : .off
        tunnelCheckbox.isEnabled = !settings.phoneRemote
    }

    @objc private func swapPressed(_ sender: Any?) {
        onSwap?()
    }

    @objc private func advancedPressed(_ sender: Any?) {
        advancedStack.isHidden = advancedButton.state == .off
    }

    /// The phone remote is the tunnel with a QR code, so the tunnel switch follows it.
    @objc private func phoneRemoteChanged(_ sender: Any?) {
        let on = phoneRemoteCheckbox.state == .on
        if on { tunnelCheckbox.state = .on }
        tunnelCheckbox.isEnabled = !on
    }

    @objc private func startPressed(_ sender: Any?) {
        let options = options(mode: .play)
        close()
        onStart?(options)
    }

    @objc private func rehearsePressed(_ sender: Any?) {
        let options = options(mode: .rehearse)
        close()
        onRehearse?(options)
    }
}
