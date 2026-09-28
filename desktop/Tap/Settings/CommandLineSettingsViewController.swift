import AppKit

/// Command Line: the bundled tap, any other tap on the login shell's
/// PATH with its version, and Install, which asks first and links the
/// bundled tap into ~/.local/bin, only where that folder comes first on
/// PATH and holds nothing but our own link. Another tap is never replaced
/// or deleted. Built on FormCard, not an NSBox whose contentView is a
/// stack (Task 12's known layout bug: the box collapses to its title).
final class CommandLineSettingsViewController: NSViewController {
    let bundledLabel = NSTextField(labelWithString: "")
    let bundledPathLabel = NSTextField(labelWithString: "")
    let otherLabel = NSTextField(labelWithString: "")
    let otherPathLabel = NSTextField(labelWithString: "")
    let otherNoteLabel = NSTextField(labelWithString: "")
    let installButton = NSButton(title: "Install…", target: nil, action: nil)
    let installHint = NSTextField(wrappingLabelWithString: "")
    private(set) var otherTap: (path: String, version: String?)?

    /// Why Install is on or off.
    enum InstallState: Equatable {
        case ready(hint: String)
        case installed
        case foreignFile
        case notFirstOnPath(otherVersion: String?)
        case notOnPath
    }
    private(set) var installState: InstallState = .notOnPath

    /// Every card of the pane, top to bottom (a layout test's seam).
    var cards: [FormCard] {
        func cards(in view: NSView) -> [FormCard] {
            view.subviews.flatMap { subview in (subview as? FormCard).map { [$0] } ?? cards(in: subview) }
        }
        return cards(in: view)
    }

    override init(nibName: NSNib.Name?, bundle: Bundle?) {
        super.init(nibName: nil, bundle: nil)
        title = "Command Line"
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func loadView() {
        for label in [bundledPathLabel, otherPathLabel] {
            label.font = .monospacedSystemFont(ofSize: 11, weight: .regular)
            label.textColor = .secondaryLabelColor
            label.lineBreakMode = .byTruncatingMiddle
        }
        for label in [bundledLabel, otherLabel] { label.font = .systemFont(ofSize: 13, weight: .semibold) }
        otherNoteLabel.font = .systemFont(ofSize: 11)
        otherNoteLabel.textColor = .secondaryLabelColor
        installButton.bezelStyle = .rounded
        installButton.target = self
        installButton.action = #selector(installPressed(_:))
        installButton.setAccessibilityIdentifier("cli-install")
        installHint.font = .systemFont(ofSize: 11.5)
        installHint.textColor = .secondaryLabelColor
        installHint.setAccessibilityIdentifier("cli-install-hint")

        let otherText = NSStackView(views: [otherLabel, otherNoteLabel])
        otherText.orientation = .vertical
        otherText.alignment = .leading
        otherText.spacing = 2

        let bundledCard = FormCard(rows: [FormCard.row(leading: [bundledLabel], trailing: [bundledPathLabel])])
        let otherCard = FormCard(rows: [FormCard.row(leading: [otherText], trailing: [otherPathLabel])])
        bundledCard.setAccessibilityIdentifier("settings-card-Bundled")
        otherCard.setAccessibilityIdentifier("settings-card-On your PATH")

        let sections = [
            FormCard.section(title: "Bundled", content: [bundledCard]),
            FormCard.section(title: "On your PATH", content: [otherCard]),
        ]
        let stack = NSStackView(views: sections + [installButton, installHint])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 16
        stack.edgeInsets = NSEdgeInsets(top: 20, left: 28, bottom: 20, right: 28)
        for section in sections { section.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -56).isActive = true }
        installHint.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -56).isActive = true
        stack.widthAnchor.constraint(equalToConstant: 760).isActive = true
        view = stack
    }

    override func viewWillAppear() {
        super.viewWillAppear()
        Task { @MainActor [weak self] in await self?.refresh() }
    }

    /// The bundled tap's path from its bundle: "Tap.app/Contents/Resources/tap".
    static func pathFromBundle(_ path: String) -> String {
        let components = path.split(separator: "/").map(String.init)
        guard let app = components.lastIndex(where: { $0.hasSuffix(".app") }) else { return path }
        return components[app...].joined(separator: "/")
    }

    func refresh() async {
        let environment = AppEnvironment.shared
        let installer = environment.commandLineInstaller
        var bundledVersion = environment.bundledTapVersion
        if bundledVersion == nil { bundledVersion = await AppEnvironment.readVersion(of: environment.tapExecutableURL) }
        bundledLabel.stringValue = "tap \(bundledVersion ?? "unknown")"
        bundledPathLabel.stringValue = Self.pathFromBundle(environment.tapExecutableURL.path)
        let path = (await environment.tapEnvironment())["PATH"] ?? ""
        let others = CommandLineTool.locate(named: "tap", onPath: path, fileExists: { FileManager.default.isExecutableFile(atPath: $0) })
            .filter { !CommandLineTool.isBundledLink(destination: try? FileManager.default.destinationOfSymbolicLink(atPath: $0)) }
        if let first = others.first {
            let version = await AppEnvironment.readVersion(of: URL(fileURLWithPath: first))
            otherTap = (first, version)
            otherLabel.stringValue = version.map { "tap \($0)" } ?? "tap (unknown version)"
            otherPathLabel.stringValue = first
            // The boards' words as drawn: SettingsCLI's for another tap on PATH, InstallConfirm's frame c for a foreign file where our link would go.
            otherNoteLabel.stringValue = FilePaths.same(URL(fileURLWithPath: first), installer.linkURL)
                ? "Tap did not install it. Tap never replaces or deletes it." : "Installed by Homebrew. Tap never replaces or deletes it."
        } else {
            otherTap = nil
            otherLabel.stringValue = "No other tap is on your PATH."
            otherPathLabel.stringValue = ""
            otherNoteLabel.stringValue = ""
        }
        installState = Self.installState(installer: installer, otherTap: otherTap, path: path)
        let directory = (installer.linkDirectory.path as NSString).abbreviatingWithTildeInPath
        installButton.title = installState == .installed ? "Installed" : "Install in \(directory)…"
        installButton.isEnabled = { if case .ready = installState { return true } else { return false } }()
        switch installState {
        case .ready(let hint): installHint.stringValue = hint
        case .installed: installHint.stringValue = "Terminal runs the bundled tap. To remove it, delete \(directory)/tap."
        case .foreignFile: installHint.stringValue = "\(directory)/tap is a tap that Tap did not install. Tap never replaces or deletes it."
        case .notFirstOnPath(let otherVersion):
            let otherDirectory = (otherTap?.path as NSString?)?.deletingLastPathComponent ?? ""
            installHint.stringValue = "\(otherDirectory) comes before \(directory) on your PATH, so Terminal would still run tap \(otherVersion ?? "")."
        case .notOnPath: installHint.stringValue = "\(directory) is not on your PATH: add it in your shell's profile first."
        }
    }

    /// The spec's rule: Install only where ~/.local/bin comes first on PATH,
    /// and only over nothing or our own link. A foreign file is the first
    /// thing to say, before any PATH question.
    static func installState(installer: CommandLineInstaller, otherTap: (path: String, version: String?)?, path: String) -> InstallState {
        let directory = (installer.linkDirectory.path as NSString).abbreviatingWithTildeInPath
        switch installer.existingFile() {
        case .other: return .foreignFile
        case .bundledLink where installer.isInstalled: return .installed
        case .bundledLink, .none: break
        }
        switch CommandLineTool.directoryComesFirst(installer.linkDirectory.path, beforeDirectoryOf: otherTap?.path, onPath: path) {
        case true?:
            guard let otherTap else { return .ready(hint: "Asks first. \(directory) is on your PATH.") }
            return .ready(hint: "Asks first. \(directory) comes before \((otherTap.path as NSString).deletingLastPathComponent) on your PATH.")
        case false?: return .notFirstOnPath(otherVersion: otherTap?.version)
        case nil: return .notOnPath
        }
    }

    /// The InstallConfirm sheet, then the link; a refused state never gets a sheet.
    @objc func installPressed(_ sender: Any?) {
        guard case .ready = installState, let window = view.window, window.attachedSheet == nil else { return NSSound.beep() }
        let installer = AppEnvironment.shared.commandLineInstaller
        let sheet = InstallConfirmSheet(installer: installer, bundledVersion: AppEnvironment.shared.bundledTapVersion, otherTap: otherTap)
        window.beginSheet(sheet) { [weak self] response in
            guard response == .OK else { return }
            do {
                try installer.install()
            } catch {
                self?.installHint.stringValue = "Install failed: \(error)"
            }
            Task { @MainActor [weak self] in await self?.refresh() }
        }
    }
}
