import AppKit

/// One option of Present Settings: a title, a line under it, and a switch,
/// with a help button after the title when the option has one.
final class PresentOptionRow: NSStackView {
    let titleLabel: NSTextField
    let sublineLabel: NSTextField
    let toggle = NSSwitch()
    private(set) var helpButton: NSButton?

    init(title: String, subline: String, hasHelp: Bool) {
        titleLabel = NSTextField(labelWithString: title)
        sublineLabel = NSTextField(wrappingLabelWithString: subline)
        super.init(frame: .zero)
        titleLabel.font = .systemFont(ofSize: 13)
        sublineLabel.font = .systemFont(ofSize: 11.5)
        sublineLabel.textColor = .secondaryLabelColor
        // The switch and the spacing before it take 48 points of the popover's width.
        sublineLabel.preferredMaxLayoutWidth = PresentPopoverController.contentWidth - 48
        var titleViews: [NSView] = [titleLabel]
        if hasHelp {
            let button = NSButton()
            button.bezelStyle = .helpButton
            button.title = ""
            button.setAccessibilityLabel("About \(title)")
            titleViews.append(button)
            helpButton = button
        }
        let titleRow = NSStackView(views: titleViews)
        titleRow.orientation = .horizontal
        titleRow.spacing = 6
        let text = NSStackView(views: [titleRow, sublineLabel])
        text.orientation = .vertical
        text.alignment = .leading
        text.spacing = 1
        toggle.setAccessibilityLabel(title)
        orientation = .horizontal
        alignment = .top
        spacing = 10
        addArrangedSubview(text)
        addArrangedSubview(toggle)
        text.setHuggingPriority(.defaultLow, for: .horizontal)
        toggle.setContentHuggingPriority(.required, for: .horizontal)
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    var isOn: Bool {
        get { toggle.state == .on }
        set { toggle.state = newValue ? .on : .off }
    }
}

/// The small popover a help button opens: a heading and a paragraph or two.
private final class HelpContentViewController: NSViewController {
    static let width: CGFloat = 280

    init(sections: [(heading: String, body: String)], qrNote: String?) {
        super.init(nibName: nil, bundle: nil)
        var views: [NSView] = []
        for section in sections {
            let heading = NSTextField(labelWithString: section.heading)
            heading.font = .systemFont(ofSize: 12.5, weight: .semibold)
            let body = NSTextField(wrappingLabelWithString: section.body)
            body.font = .systemFont(ofSize: 12.5)
            body.textColor = .secondaryLabelColor
            body.preferredMaxLayoutWidth = Self.width - 28
            views.append(contentsOf: [heading, body])
        }
        if let qrNote {
            let image = NSImageView(image: NSImage(systemSymbolName: "qrcode", accessibilityDescription: nil) ?? NSImage())
            image.symbolConfiguration = NSImage.SymbolConfiguration(pointSize: 28, weight: .regular)
            image.contentTintColor = .secondaryLabelColor
            let note = NSTextField(wrappingLabelWithString: qrNote)
            note.font = .systemFont(ofSize: 11)
            note.textColor = .secondaryLabelColor
            note.preferredMaxLayoutWidth = Self.width - 28 - 44
            let row = NSStackView(views: [image, note])
            row.orientation = .horizontal
            row.spacing = 10
            views.append(row)
        }
        let stack = NSStackView(views: views)
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 4
        stack.setCustomSpacing(10, after: views.count > 2 ? views[1] : views[0])
        stack.edgeInsets = NSEdgeInsets(top: 14, left: 14, bottom: 14, right: 14)
        stack.widthAnchor.constraint(equalToConstant: Self.width).isActive = true
        view = stack
        stack.setAccessibilityIdentifier("present-help")
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }
}

/// Present Settings, the popover anchored on the Play button: which display
/// does what (with two or more), recording, the phone remote, and where to
/// start, with Rehearse and Play. It collects `PresentationOptions`; the
/// deck's window controller starts the talk.
@MainActor
final class PresentPopoverController: NSObject, NSPopoverDelegate {
    struct Context {
        let arrangement: DisplayArrangement?
        let cursorSlide: Int
        /// False when two displays share one Space, so the talk windows cannot be full screen.
        let usesFullScreen: Bool
    }

    static let contentWidth: CGFloat = 328

    /// Kept here rather than read from the popover, whose `isShown` can
    /// depend on whether the app is active (D3's gallery found the same).
    private(set) var isShown = false
    /// The help popover the (?) buttons open, and whether it is up.
    private(set) var isHelpShown = false
    let displaysSection = NSStackView()
    let arrangementView = DisplayArrangementView(frame: .zero)
    let displaysCaption = NSTextField(wrappingLabelWithString: "Click a display to make it the audience screen.")
    let spacesNotice = TintedSurfaceView(frame: .zero)
    let spacesNoticeLabel = NSTextField(wrappingLabelWithString: "Each display needs its own Space to play full screen.")
    let spacesSettingsButton = NSButton(title: "Open Settings", target: nil, action: nil)
    let recordRow = PresentOptionRow(title: "Record the talk", subline: "Saves a video of the audience screen and your voice.", hasHelp: false)
    let phoneRemoteRow = PresentOptionRow(title: "Phone remote", subline: "Change slides and read notes from your phone.", hasHelp: true)
    let remoteGroup = TintedSurfaceView(frame: .zero)
    let passwordRow = PresentOptionRow(title: "Require a password", subline: "Asked for once on the phone. Never saved.", hasHelp: false)
    let passwordField = NSSecureTextField()
    let cloudflaredMessage = NSTextField(wrappingLabelWithString: "Needs cloudflared, which is not installed.")
    let copyInstallButton = NSButton(title: "Copy install command", target: nil, action: nil)
    let installCommandLabel = NSTextField(labelWithString: CloudflaredLocator.installCommand)
    let cloudflaredBlock = NSStackView()
    let startFromLabel = NSTextField(labelWithString: "Start from")
    let startFromPopUp = NSPopUpButton(frame: .zero, pullsDown: false)
    let rehearseButton = NSButton(title: "Rehearse", target: nil, action: nil)
    let startButton = NSButton(title: "Play", target: nil, action: nil)
    var recordSwitch: NSSwitch { recordRow.toggle }
    var phoneRemoteSwitch: NSSwitch { phoneRemoteRow.toggle }
    var passwordSwitch: NSSwitch { passwordRow.toggle }
    /// Where Copy install command puts the command. A test gives it a pasteboard of its own.
    var pasteboard: NSPasteboard = .general
    /// False when tap would not find cloudflared: the remote cannot start, and Play starts without it.
    var cloudflaredInstalled = true {
        didSet { if cloudflaredInstalled != oldValue { refresh() } }
    }
    /// A person chose a role for a display, by its menu or by clicking it.
    var onAssign: ((DisplayRole, ScreenInfo) -> Void)?
    /// The displays changed while the popover is up.
    var onScreensChanged: (() -> Void)?
    var onOpenSpacesSettings: (() -> Void)?
    var onStart: ((PresentationOptions) -> Void)?
    var onRehearse: ((PresentationOptions) -> Void)?
    private let popover = NSPopover()
    private let helpPopover = NSPopover()
    private var context = Context(arrangement: nil, cursorSlide: 1, usesFullScreen: true)
    private var screenObserver: NSObjectProtocol?

    override init() {
        super.init()
        let displaysHeader = Self.sectionHeader("Displays")
        displaysCaption.font = .systemFont(ofSize: 11)
        displaysCaption.textColor = .secondaryLabelColor
        displaysCaption.alignment = .center
        displaysCaption.preferredMaxLayoutWidth = Self.contentWidth
        arrangementView.onAssign = { [weak self] role, screen in self?.onAssign?(role, screen) }
        displaysSection.orientation = .vertical
        displaysSection.alignment = .centerX
        displaysSection.spacing = 8
        for view in [displaysHeader, arrangementView, displaysCaption] { displaysSection.addArrangedSubview(view) }
        displaysHeader.leadingAnchor.constraint(equalTo: displaysSection.leadingAnchor).isActive = true
        displaysSection.setAccessibilityIdentifier("displays-section")

        spacesNotice.fillColor = NSColor.systemOrange.withAlphaComponent(0.16)
        spacesNotice.cornerRadius = 9
        spacesNoticeLabel.font = .systemFont(ofSize: 12)
        spacesNoticeLabel.preferredMaxLayoutWidth = Self.contentWidth - 24 - 100
        spacesSettingsButton.bezelStyle = .rounded
        spacesSettingsButton.controlSize = .small
        spacesSettingsButton.target = self
        spacesSettingsButton.action = #selector(openSpacesSettingsPressed(_:))
        spacesSettingsButton.toolTip = "Turn on \"Displays have separate Spaces\" in Desktop & Dock."
        spacesSettingsButton.setAccessibilityIdentifier("open-spaces-settings")
        let noticeRow = NSStackView(views: [spacesNoticeLabel, spacesSettingsButton])
        noticeRow.orientation = .horizontal
        noticeRow.spacing = 10
        noticeRow.alignment = .centerY
        noticeRow.translatesAutoresizingMaskIntoConstraints = false
        spacesNotice.addSubview(noticeRow)
        NSLayoutConstraint.activate([
            noticeRow.topAnchor.constraint(equalTo: spacesNotice.topAnchor, constant: 9),
            noticeRow.bottomAnchor.constraint(equalTo: spacesNotice.bottomAnchor, constant: -9),
            noticeRow.leadingAnchor.constraint(equalTo: spacesNotice.leadingAnchor, constant: 12),
            noticeRow.trailingAnchor.constraint(equalTo: spacesNotice.trailingAnchor, constant: -12),
        ])
        spacesNotice.setAccessibilityIdentifier("spaces-notice")
        spacesNotice.isHidden = true

        recordRow.isOn = true
        phoneRemoteRow.toggle.target = self
        phoneRemoteRow.toggle.action = #selector(phoneRemoteChanged(_:))
        phoneRemoteRow.helpButton?.target = self
        phoneRemoteRow.helpButton?.action = #selector(phoneRemoteHelpPressed(_:))
        phoneRemoteRow.helpButton?.setAccessibilityIdentifier("phone-remote-help")
        recordSwitch.setAccessibilityIdentifier("record-switch")
        phoneRemoteSwitch.setAccessibilityIdentifier("phone-remote-switch")
        passwordSwitch.setAccessibilityIdentifier("password-switch")
        passwordSwitch.target = self
        passwordSwitch.action = #selector(passwordSwitchChanged(_:))
        passwordField.placeholderString = "Password"
        passwordField.setAccessibilityIdentifier("presenter-password")
        passwordField.isHidden = true

        cloudflaredMessage.font = .systemFont(ofSize: 11.5)
        cloudflaredMessage.textColor = .systemRed
        cloudflaredMessage.preferredMaxLayoutWidth = Self.contentWidth - 24
        cloudflaredMessage.setAccessibilityIdentifier("cloudflared-missing")
        copyInstallButton.bezelStyle = .rounded
        copyInstallButton.controlSize = .small
        copyInstallButton.target = self
        copyInstallButton.action = #selector(copyInstallPressed(_:))
        copyInstallButton.setAccessibilityIdentifier("copy-install-command")
        installCommandLabel.font = .monospacedSystemFont(ofSize: 11, weight: .regular)
        installCommandLabel.textColor = .secondaryLabelColor
        let installRow = NSStackView(views: [copyInstallButton, installCommandLabel])
        installRow.orientation = .horizontal
        installRow.spacing = 8
        cloudflaredBlock.orientation = .vertical
        cloudflaredBlock.alignment = .leading
        cloudflaredBlock.spacing = 6
        cloudflaredBlock.addArrangedSubview(cloudflaredMessage)
        cloudflaredBlock.addArrangedSubview(installRow)
        cloudflaredBlock.isHidden = true

        remoteGroup.fillColor = .quaternarySystemFill
        remoteGroup.cornerRadius = 9
        let groupStack = NSStackView(views: [passwordRow, passwordField, cloudflaredBlock])
        groupStack.orientation = .vertical
        groupStack.alignment = .leading
        groupStack.spacing = 10
        groupStack.translatesAutoresizingMaskIntoConstraints = false
        remoteGroup.addSubview(groupStack)
        NSLayoutConstraint.activate([
            groupStack.topAnchor.constraint(equalTo: remoteGroup.topAnchor, constant: 10),
            groupStack.bottomAnchor.constraint(equalTo: remoteGroup.bottomAnchor, constant: -10),
            groupStack.leadingAnchor.constraint(equalTo: remoteGroup.leadingAnchor, constant: 12),
            groupStack.trailingAnchor.constraint(equalTo: remoteGroup.trailingAnchor, constant: -12),
        ])
        passwordRow.widthAnchor.constraint(equalTo: groupStack.widthAnchor).isActive = true
        passwordField.widthAnchor.constraint(equalTo: groupStack.widthAnchor).isActive = true
        passwordRow.sublineLabel.preferredMaxLayoutWidth = Self.contentWidth - 24 - 48 // the group's insets
        remoteGroup.setAccessibilityIdentifier("remote-options")
        remoteGroup.isHidden = true

        startFromPopUp.controlSize = .small
        startFromPopUp.setAccessibilityIdentifier("start-from")
        startFromLabel.font = .systemFont(ofSize: 11)
        startFromLabel.textColor = .secondaryLabelColor
        rehearseButton.bezelStyle = .rounded
        rehearseButton.target = self
        rehearseButton.action = #selector(rehearsePressed(_:))
        rehearseButton.setAccessibilityIdentifier("rehearse")
        startButton.bezelStyle = .rounded
        startButton.keyEquivalent = "\r"
        startButton.target = self
        startButton.action = #selector(startPressed(_:))
        startButton.setAccessibilityIdentifier("start-presenting")
        let spacer = NSView()
        spacer.setContentHuggingPriority(.defaultLow, for: .horizontal)
        let footer = NSStackView(views: [startFromLabel, startFromPopUp, spacer, rehearseButton, startButton])
        footer.orientation = .horizontal
        footer.spacing = 8
        footer.alignment = .centerY
        let divider = NSBox()
        divider.boxType = .separator

        let stack = NSStackView(views: [displaysSection, spacesNotice, Self.sectionHeader("Options"), recordRow, phoneRemoteRow,
                                        remoteGroup, divider, footer])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 12
        stack.edgeInsets = NSEdgeInsets(top: 16, left: 16, bottom: 16, right: 16)
        stack.widthAnchor.constraint(equalToConstant: Self.contentWidth + 32).isActive = true
        for view in [displaysSection, spacesNotice, recordRow, phoneRemoteRow, remoteGroup, divider, footer] {
            view.widthAnchor.constraint(equalToConstant: Self.contentWidth).isActive = true
        }
        stack.setCustomSpacing(8, after: phoneRemoteRow)
        let content = NSViewController()
        content.view = stack
        popover.contentViewController = content
        popover.behavior = .transient
        popover.delegate = self
        helpPopover.behavior = .transient
        helpPopover.delegate = self
        stack.setAccessibilityIdentifier("present-popover")
        refresh()
    }

    private static func sectionHeader(_ title: String) -> NSTextField {
        let label = NSTextField(labelWithString: title)
        label.font = .systemFont(ofSize: 12, weight: .semibold)
        label.textColor = .secondaryLabelColor
        return label
    }

    func show(context: Context, relativeTo rect: NSRect, of view: NSView) {
        resetStartFrom()
        update(context: context)
        popover.show(relativeTo: rect, of: view, preferredEdge: .maxY)
        isShown = true
        observeDisplays()
    }

    /// The displays or the cursor changed, or a role was assigned.
    func update(context: Context) {
        self.context = context
        refresh()
    }

    /// Start from goes back to the cursor's slide: a choice of Beginning belongs to one start.
    private func resetStartFrom() {
        startFromPopUp.selectItem(at: 0)
    }

    func close() {
        closeHelp()
        resetStartFrom()
        isShown = false
        stopObservingDisplays()
        popover.performClose(nil)
    }

    func popoverDidClose(_ notification: Notification) {
        if (notification.object as? NSPopover) === helpPopover {
            isHelpShown = false
            return
        }
        isShown = false
        resetStartFrom()
        stopObservingDisplays()
        closeHelp()
    }

    /// Shows what the context and the switches call for: the displays
    /// only with two or more, the Spaces notice when full screen is out of
    /// reach, the remote's options under the remote, and the fix for a
    /// missing cloudflared.
    private func refresh() {
        let screens = context.arrangement?.screens ?? []
        let showsDisplays = screens.count > 1
        if let arrangement = context.arrangement, showsDisplays { arrangementView.update(arrangement: arrangement) }
        displaysSection.isHidden = !showsDisplays
        spacesNotice.isHidden = !showsDisplays || context.usesFullScreen
        let remoteOn = phoneRemoteSwitch.state == .on
        remoteGroup.isHidden = !remoteOn
        passwordField.isHidden = passwordSwitch.state == .off
        cloudflaredBlock.isHidden = !remoteOn || cloudflaredInstalled
        let cursor = context.cursorSlide
        let previousChoice = startFromPopUp.indexOfSelectedItem
        startFromPopUp.removeAllItems()
        startFromPopUp.addItems(withTitles: ["Slide \(cursor)", "Beginning"])
        startFromPopUp.selectItem(at: previousChoice == 1 ? 1 : 0)
        startFromLabel.isHidden = cursor <= 1
        startFromPopUp.isHidden = cursor <= 1
    }

    private func observeDisplays() {
        guard screenObserver == nil else { return }
        screenObserver = NotificationCenter.default.addObserver(forName: NSApplication.didChangeScreenParametersNotification, object: nil, queue: nil) { [weak self] _ in
            MainActor.assumeIsolated { self?.onScreensChanged?() }
        }
    }

    private func stopObservingDisplays() {
        if let screenObserver { NotificationCenter.default.removeObserver(screenObserver) }
        screenObserver = nil
    }

    /// True when the person chose to start on slide 1 and the cursor is not already there.
    var startsFromBeginning: Bool {
        context.cursorSlide > 1 && startFromPopUp.indexOfSelectedItem == 1
    }

    /// What the controls say now. With cloudflared missing the remote is left out, so Play still starts.
    func options(mode: PresentationMode) -> PresentationOptions {
        var options = settings.options(mode: mode, startSlide: startsFromBeginning ? 1 : context.cursorSlide,
                                       presenterPassword: presenterPassword)
        if !cloudflaredInstalled { options.phoneRemote = false }
        return options
    }

    /// The controls as settings: what a start saves, and what Cmd+Option+P starts with.
    var settings: PresentationSettings {
        PresentationSettings(record: recordSwitch.state == .on, phoneRemote: phoneRemoteSwitch.state == .on)
    }

    /// The field's password, nil when the switch is off or the field empty. It is never saved.
    var presenterPassword: String? {
        guard phoneRemoteSwitch.state == .on, passwordSwitch.state == .on else { return nil }
        let password = passwordField.stringValue.trimmingCharacters(in: .whitespaces)
        return password.isEmpty ? nil : password
    }

    /// Puts saved settings into the controls. Called every time the popover is freshened, not only when it is made.
    func loadSettings(_ settings: PresentationSettings) {
        recordSwitch.state = settings.record ? .on : .off
        phoneRemoteSwitch.state = settings.phoneRemote ? .on : .off
        resetStartFrom()
        refresh()
    }

    // MARK: Help

    /// Opens the help popover on the Phone remote (?) button.
    func showPhoneRemoteHelp() {
        guard let button = phoneRemoteRow.helpButton else { return }
        closeHelp()
        helpPopover.contentViewController = HelpContentViewController(
            sections: [("Phone remote", "Scan a QR code with your phone to turn it into a clicker. It shows the next slide, your notes and a timer."),
                       ("Works away from this Wi\u{2011}Fi", "Venue Wi\u{2011}Fi often blocks devices from seeing each other. The remote goes through a public link, so it works on any network, cellular data included. The link needs cloudflared, a free tool from Cloudflare.")],
            qrNote: "The QR code appears in its own window once the talk starts.")
        helpPopover.show(relativeTo: button.bounds, of: button, preferredEdge: .maxX)
        isHelpShown = true
    }

    private func closeHelp() {
        isHelpShown = false
        if helpPopover.isShown { helpPopover.performClose(nil) }
    }

    /// The text the help popover is showing, for a test.
    var helpText: String {
        guard let stack = helpPopover.contentViewController?.view as? NSStackView else { return "" }
        return stack.arrangedSubviews.compactMap { ($0 as? NSTextField)?.stringValue }.joined(separator: "\n")
    }

    // MARK: Actions

    @objc private func phoneRemoteHelpPressed(_ sender: Any?) {
        showPhoneRemoteHelp()
    }

    @objc private func phoneRemoteChanged(_ sender: Any?) {
        refresh()
    }

    @objc private func passwordSwitchChanged(_ sender: Any?) {
        refresh()
        if passwordSwitch.state == .on { passwordField.window?.makeFirstResponder(passwordField) }
    }

    @objc private func openSpacesSettingsPressed(_ sender: Any?) {
        onOpenSpacesSettings?()
    }

    @objc private func copyInstallPressed(_ sender: Any?) {
        pasteboard.clearContents()
        pasteboard.setString(CloudflaredLocator.installCommand, forType: .string)
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
