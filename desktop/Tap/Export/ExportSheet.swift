import AppKit

/// File > Export: the options, then the run's progress, then what tap
/// made. One sheet per export; Cancel sends SIGINT through the controller.
final class ExportSheet: NSWindow {
    typealias State = ExportController.State

    let kind: ExportKind
    let titleLabel = NSTextField(labelWithString: "")
    let contentControl = NSSegmentedControl(labels: ["Slides", "Notes", "Both"], trackingMode: .selectOne, target: nil, action: nil)
    /// The Save as (or Export to) row: the output's name, a click chooses another.
    let outputButton = NSButton(title: "", target: nil, action: nil)
    let statusLabel = NSTextField(labelWithString: "")
    let detailLabel = NSTextField(wrappingLabelWithString: "")
    let bytesLabel = NSTextField(labelWithString: "")
    let progressBar = NSProgressIndicator()
    let pathLabel = NSTextField(labelWithString: "")
    let warningsHeader = NSTextField(labelWithString: "")
    private(set) var warningRows: [NSTextField] = []
    let cancelButton = NSButton(title: "Cancel", target: nil, action: nil)
    let exportButton = NSButton(title: "Export", target: nil, action: nil)
    let revealButton = NSButton(title: "Show in Finder", target: nil, action: nil)
    let previewButton = NSButton(title: "Preview", target: nil, action: nil)
    let doneButton = NSButton(title: "Done", target: nil, action: nil)
    private(set) var state: State = .idle
    private(set) var output: String
    /// Whether the person picked `output` in the save or folder panel,
    /// which asks before it replaces a file. The default path next to the
    /// deck was picked by nobody.
    private(set) var outputWasChosen = false
    private let optionsStack = NSStackView()
    let progressBox = NSStackView()
    private let doneIcon = NSImageView()
    let warningsBox = NSStackView()
    private let warningsList = NSStackView()
    var onExport: ((ExportRequest) -> Void)?
    var onCancel: (() -> Void)?
    var onReveal: ((URL) -> Void)?
    var onPreview: ((URL) -> Void)?
    /// A save or folder panel in production; a test answers at once.
    var chooseOutput: (ExportKind, String, @escaping (String?) -> Void) -> Void = { kind, current, completion in
        if kind.isPDF {
            let panel = NSSavePanel()
            panel.allowedContentTypes = [.pdf]
            panel.nameFieldStringValue = (current as NSString).lastPathComponent
            panel.directoryURL = URL(fileURLWithPath: (current as NSString).deletingLastPathComponent)
            panel.begin { response in completion(response == .OK ? panel.url?.path : nil) }
        } else {
            let panel = NSOpenPanel()
            panel.canChooseDirectories = true
            panel.canChooseFiles = false
            panel.canCreateDirectories = true
            panel.prompt = "Export Here"
            panel.begin { response in completion(response == .OK ? panel.url?.path : nil) }
        }
    }

    var request: ExportRequest {
        let contents = ["slides", "notes", "both"]
        let kind: ExportKind
        switch self.kind {
        case .pdf: kind = .pdf(content: contents[max(0, contentControl.selectedSegment)])
        default: kind = self.kind
        }
        return ExportRequest(kind: kind, output: output)
    }

    init(kind: ExportKind, deck: URL) {
        self.kind = kind
        output = kind.defaultOutput(for: deck).path
        super.init(contentRect: NSRect(x: 0, y: 0, width: 520, height: 260), styleMask: [.titled], backing: .buffered, defer: false)
        titleLabel.stringValue = kind.title
        titleLabel.font = .systemFont(ofSize: 15, weight: .bold)
        contentControl.selectedSegment = 0
        contentControl.setAccessibilityIdentifier("export-content")
        outputButton.bezelStyle = .rounded
        outputButton.target = self
        outputButton.action = #selector(choosePressed(_:))
        outputButton.setAccessibilityIdentifier("export-output")
        for button in [cancelButton, exportButton, revealButton, previewButton, doneButton] { button.bezelStyle = .rounded; button.target = self }
        cancelButton.action = #selector(cancelPressed(_:))
        cancelButton.keyEquivalent = "\u{1b}"
        exportButton.action = #selector(exportPressed(_:))
        exportButton.keyEquivalent = "\r"
        exportButton.setAccessibilityIdentifier("export-run")
        revealButton.action = #selector(revealPressed(_:))
        revealButton.setAccessibilityIdentifier("export-reveal")
        previewButton.action = #selector(previewPressed(_:))
        previewButton.setAccessibilityIdentifier("export-preview")
        doneButton.action = #selector(donePressed(_:))
        doneButton.setAccessibilityIdentifier("export-done")
        statusLabel.font = .systemFont(ofSize: 15, weight: .bold)
        statusLabel.setAccessibilityIdentifier("export-status")
        detailLabel.font = .systemFont(ofSize: 12)
        detailLabel.textColor = .secondaryLabelColor
        bytesLabel.font = .systemFont(ofSize: 11.5)
        bytesLabel.textColor = .secondaryLabelColor
        progressBar.style = .bar
        progressBar.minValue = 0
        progressBar.maxValue = 1
        progressBar.setAccessibilityIdentifier("export-progress")
        pathLabel.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
        pathLabel.lineBreakMode = .byTruncatingMiddle
        doneIcon.image = NSImage(systemSymbolName: kind.isPDF ? "doc" : kind.isWebsite ? "globe" : "photo.on.rectangle", accessibilityDescription: nil)
        doneIcon.contentTintColor = .controlAccentColor
        // The warnings box: a light tint, an orange icon and a heading, then one line per slide in normal text.
        warningsBox.wantsLayer = true
        warningsBox.layer?.backgroundColor = NSColor.systemOrange.withAlphaComponent(0.10).cgColor
        warningsBox.layer?.cornerRadius = 10
        warningsBox.orientation = .vertical
        warningsBox.alignment = .leading
        warningsBox.spacing = 4
        warningsBox.edgeInsets = NSEdgeInsets(top: 6, left: 12, bottom: 6, right: 12)
        let warningIcon = NSImageView(image: NSImage(systemSymbolName: "exclamationmark.triangle", accessibilityDescription: "Warning") ?? NSImage())
        warningIcon.contentTintColor = .systemOrange
        warningsHeader.font = .systemFont(ofSize: 12, weight: .semibold)
        warningsHeader.setAccessibilityIdentifier("export-warnings-header")
        let warningsHead = NSStackView(views: [warningIcon, warningsHeader])
        warningsHead.spacing = 8
        warningsList.orientation = .vertical
        warningsList.alignment = .leading
        warningsList.spacing = 3
        warningsList.edgeInsets = NSEdgeInsets(top: 0, left: 22, bottom: 0, right: 0)
        warningsBox.addArrangedSubview(warningsHead)
        warningsBox.addArrangedSubview(warningsList)
        // Inside the box's insets: the list is the box's width less 12 on each side.
        warningsList.widthAnchor.constraint(equalTo: warningsBox.widthAnchor, constant: -24).isActive = true
        warningsHead.widthAnchor.constraint(lessThanOrEqualTo: warningsBox.widthAnchor, constant: -24).isActive = true
        warningsBox.setAccessibilityIdentifier("export-warnings")

        let outputRow = NSStackView(views: [NSTextField(labelWithString: kind.isPDF ? "Save as" : "Export to"), NSView(), outputButton])
        outputRow.spacing = 8
        var options: [NSView] = []
        if kind.isPDF { options.append(NSStackView(views: [NSTextField(labelWithString: "Content"), NSView(), contentControl])) }
        options.append(outputRow)
        for view in options { optionsStack.addArrangedSubview(view) }
        optionsStack.orientation = .vertical
        optionsStack.alignment = .leading
        optionsStack.spacing = 10
        // The tinted block of the download and the done state: the bar (or the summary) and a line under it.
        progressBox.wantsLayer = true
        progressBox.layer?.backgroundColor = NSColor.labelColor.withAlphaComponent(0.05).cgColor
        progressBox.layer?.cornerRadius = 11
        progressBox.orientation = .vertical
        progressBox.alignment = .leading
        progressBox.spacing = 7
        progressBox.edgeInsets = NSEdgeInsets(top: 12, left: 12, bottom: 12, right: 12)
        for view in [progressBar, detailLabel, bytesLabel] { progressBox.addArrangedSubview(view) }
        let head = NSStackView(views: [doneIcon, statusLabel])
        head.spacing = 10
        let buttons = NSStackView(views: [NSView(), revealButton, previewButton, cancelButton, exportButton, doneButton])
        buttons.spacing = 8
        let stack = NSStackView(views: [titleLabel, optionsStack, head, progressBox, pathLabel, warningsBox, buttons])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 12
        stack.edgeInsets = NSEdgeInsets(top: 22, left: 22, bottom: 22, right: 22)
        stack.widthAnchor.constraint(equalToConstant: 504).isActive = true
        for view in [optionsStack, progressBox, warningsBox, buttons, pathLabel] as [NSView] {
            view.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -44).isActive = true
        }
        // The bar and the line under it fill the tinted block inside its 12 pt insets.
        for view in [progressBar, detailLabel] as [NSView] {
            view.widthAnchor.constraint(equalTo: progressBox.widthAnchor, constant: -24).isActive = true
        }
        bytesLabel.widthAnchor.constraint(lessThanOrEqualTo: progressBox.widthAnchor, constant: -24).isActive = true
        for row in [outputRow] + (kind.isPDF ? [options[0]] : []) { row.widthAnchor.constraint(equalTo: optionsStack.widthAnchor).isActive = true }
        contentView = stack
        isReleasedWhenClosed = false
        setAccessibilityIdentifier("export-sheet")
        refreshOutputButton()
        apply(.idle)
        setContentSize(stack.fittingSize)
    }

    private func refreshOutputButton() {
        outputButton.title = (output as NSString).lastPathComponent
    }

    /// The sheet's face for each state.
    func apply(_ state: State) {
        self.state = state
        let showOptions: Bool, optionsOn: Bool, showProgress: Bool, showDone: Bool
        switch state {
        case .idle: (showOptions, optionsOn, showProgress, showDone) = (true, true, false, false)
        case .running: (showOptions, optionsOn, showProgress, showDone) = (true, false, true, false)
        case .done: (showOptions, optionsOn, showProgress, showDone) = (false, false, true, true)
        case .failed: (showOptions, optionsOn, showProgress, showDone) = (true, true, true, false)
        }
        optionsStack.isHidden = !showOptions
        contentControl.isEnabled = optionsOn
        outputButton.isEnabled = optionsOn
        progressBox.isHidden = !showProgress
        doneIcon.isHidden = !showDone
        statusLabel.isHidden = !showProgress
        pathLabel.isHidden = !showDone
        warningsBox.isHidden = true
        exportButton.isHidden = showDone
        cancelButton.isHidden = showDone
        revealButton.isHidden = !showDone
        previewButton.isHidden = !(showDone && kind.isWebsite)
        doneButton.isHidden = !showDone
        exportButton.keyEquivalent = showDone ? "" : "\r"
        doneButton.keyEquivalent = showDone ? "\r" : ""
        switch state {
        case .idle:
            exportButton.isEnabled = true
            cancelButton.isEnabled = true
            cancelButton.title = "Cancel"
        case .running(let status):
            exportButton.isEnabled = false
            cancelButton.isEnabled = status != "Cancelling…"
            statusLabel.stringValue = status
            statusLabel.font = .systemFont(ofSize: 13, weight: .semibold)
            progressBar.isHidden = false
            if status.hasPrefix("Downloading") {
                detailLabel.stringValue = kind.downloadDetail
            } else {
                detailLabel.stringValue = ""
                bytesLabel.stringValue = ""
            }
            if let (done, total) = Self.counts(in: status), total > 0 {
                progressBar.isIndeterminate = false
                progressBar.doubleValue = Double(done) / Double(total)
            } else if !status.hasPrefix("Downloading") {
                progressBar.isIndeterminate = true
                progressBar.startAnimation(nil)
            }
        case .done(let summary):
            statusLabel.stringValue = kind.doneTitle(warnings: summary.warnings.count)
            statusLabel.font = .systemFont(ofSize: 15, weight: .bold)
            // The ExportWarnings board keeps the bar, full, above the summary.
            progressBar.isHidden = false
            progressBar.stopAnimation(nil)
            progressBar.isIndeterminate = false
            progressBar.doubleValue = progressBar.maxValue
            detailLabel.stringValue = summary.summary
            bytesLabel.stringValue = ""
            pathLabel.stringValue = (summary.output.path as NSString).abbreviatingWithTildeInPath + (kind.isPDF ? "" : "/")
            showWarnings(summary.warnings)
        case .failed(let message):
            exportButton.isEnabled = true
            cancelButton.isEnabled = true
            progressBar.isHidden = true
            statusLabel.stringValue = "Export failed."
            statusLabel.font = .systemFont(ofSize: 13, weight: .semibold)
            detailLabel.stringValue = message
            bytesLabel.stringValue = ""
        }
        setContentSize((contentView as? NSStackView)?.fittingSize ?? frame.size)
    }

    private func showWarnings(_ warnings: [ExportWarning]) {
        for row in warningRows { row.removeFromSuperview() }
        warningRows = []
        guard !warnings.isEmpty else { return }
        warningsHeader.stringValue = kind.warningsHeader
        for warning in warnings {
            // "Slide 2  component Throws.jsx threw": the slide in semibold, tap's words in mono, normal text color.
            let text = NSMutableAttributedString(string: warning.slide > 0 ? "Slide \(warning.slide)  " : "", attributes: [.font: NSFont.systemFont(ofSize: 12, weight: .semibold), .foregroundColor: NSColor.labelColor])
            text.append(NSAttributedString(string: warning.message, attributes: [.font: NSFont.monospacedSystemFont(ofSize: 11.5, weight: .regular), .foregroundColor: NSColor.secondaryLabelColor]))
            let row = NSTextField(labelWithAttributedString: text)
            row.lineBreakMode = .byTruncatingTail
            row.maximumNumberOfLines = 1
            row.usesSingleLineMode = true
            row.setContentCompressionResistancePriority(.defaultLow, for: .horizontal)
            warningRows.append(row)
            warningsList.addArrangedSubview(row)
            // A long message truncates inside the box instead of widening the sheet.
            row.widthAnchor.constraint(lessThanOrEqualTo: warningsList.widthAnchor, constant: -22).isActive = true
        }
        warningsBox.isHidden = false
    }

    func showDownload(_ download: (bytes: Int64, totalBytes: Int64)?) {
        guard let download else { return bytesLabel.stringValue = "" }
        let formatter = ByteCountFormatter()
        formatter.countStyle = .file
        bytesLabel.stringValue = "\(formatter.string(fromByteCount: download.bytes)) of \(formatter.string(fromByteCount: download.totalBytes))"
        progressBar.isIndeterminate = false
        progressBar.doubleValue = download.totalBytes > 0 ? Double(download.bytes) / Double(download.totalBytes) : 0
    }

    /// "Rendering slide 7 of 14" is 7 of 14.
    static func counts(in status: String) -> (Int, Int)? {
        let words = status.split(separator: " ")
        guard words.count >= 3, words[words.count - 2] == "of", let total = Int(words[words.count - 1]), let done = Int(words[words.count - 3]) else { return nil }
        return (done, total)
    }

    @objc private func exportPressed(_ sender: Any?) {
        // Off at once: a second press (Return twice) must not start a second run.
        exportButton.isEnabled = false
        // A PDF already at the default path is a file the person may have
        // made: the save panel asks before replacing it.
        if kind.isPDF, !outputWasChosen, FileManager.default.fileExists(atPath: output) {
            chooseOutput(kind, output) { [weak self] chosen in
                guard let self else { return }
                guard let chosen else {
                    self.exportButton.isEnabled = true
                    return
                }
                self.choose(chosen)
                self.onExport?(self.request)
            }
            return
        }
        onExport?(request)
    }

    private func choose(_ chosen: String) {
        output = chosen
        outputWasChosen = true
        refreshOutputButton()
    }
    @objc private func cancelPressed(_ sender: Any?) {
        if case .running = state { onCancel?() } else { sheetParent?.endSheet(self, returnCode: .cancel) }
    }
    @objc private func revealPressed(_ sender: Any?) { if case .done(let summary) = state { onReveal?(summary.output) } }
    @objc private func previewPressed(_ sender: Any?) { if case .done(let summary) = state { onPreview?(summary.output) } }
    @objc private func donePressed(_ sender: Any?) { sheetParent?.endSheet(self, returnCode: .OK) }
    @objc private func choosePressed(_ sender: Any?) {
        chooseOutput(kind, output) { [weak self] chosen in
            guard let self, let chosen else { return }
            self.choose(chosen)
        }
    }
}
