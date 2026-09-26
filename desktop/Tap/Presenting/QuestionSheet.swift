import AppKit

/// A question tap asked, or the Focus hint, as a sheet on the deck window:
/// a title, a body, an optional path line and two buttons. A sheet belongs
/// to its window; the rest of the app keeps running and nothing is
/// app-modal. Return is the accept button. Escape is the decline button
/// only where declining is harmless; a destructive decline has no key,
/// so a stray Escape after a talk (Escape is how a talk ends) can never
/// choose it.
class QuestionSheet: NSWindow {
    enum EscapeAnswer {
        /// Escape presses the decline button.
        case decline
        /// Escape does nothing; the person clicks.
        case nothing
    }

    /// Which button Return presses. Accept for a question whose yes is
    /// harmless (record consent, keep a recording); decline for the live
    /// code approval, where the safe answer is the default one and Return
    /// must never grant execution (06-live-code-and-trust, "The safe
    /// button is the default").
    enum ReturnAnswer {
        case accept
        case decline
    }

    let kind: String
    let titleLabel = NSTextField(labelWithString: "")
    let bodyLabel = NSTextField(wrappingLabelWithString: "")
    let pathLabel = NSTextField(labelWithString: "")
    let declineButton = NSButton(title: "", target: nil, action: nil)
    let acceptButton = NSButton(title: "", target: nil, action: nil)
    let escape: EscapeAnswer
    let returnAnswer: ReturnAnswer

    init(kind: String, title: String, body: String, path: String?, decline: String, accept: String,
         escape: EscapeAnswer = .decline, returnAnswer: ReturnAnswer = .accept, detail: NSView? = nil) {
        self.kind = kind
        self.escape = escape
        self.returnAnswer = returnAnswer
        super.init(contentRect: NSRect(x: 0, y: 0, width: 460, height: 200), styleMask: [.titled], backing: .buffered, defer: false)
        titleLabel.stringValue = title
        titleLabel.font = .systemFont(ofSize: 15, weight: .bold)
        bodyLabel.stringValue = body
        bodyLabel.font = .systemFont(ofSize: 12.5)
        bodyLabel.textColor = .secondaryLabelColor
        pathLabel.stringValue = path ?? ""
        pathLabel.isHidden = path == nil
        pathLabel.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
        pathLabel.lineBreakMode = .byTruncatingMiddle
        pathLabel.setAccessibilityIdentifier("question-path")
        declineButton.title = decline
        declineButton.bezelStyle = .rounded
        declineButton.target = self
        declineButton.action = #selector(declinePressed(_:))
        acceptButton.title = accept
        acceptButton.bezelStyle = .rounded
        acceptButton.target = self
        acceptButton.action = #selector(acceptPressed(_:))
        switch returnAnswer {
        case .accept:
            acceptButton.keyEquivalent = "\r"
            declineButton.keyEquivalent = escape == .decline ? "\u{1b}" : ""
        case .decline:
            // The safe answer holds Return; Escape, when it declines too, comes through keyDown.
            declineButton.keyEquivalent = "\r"
            acceptButton.keyEquivalent = ""
        }
        let buttons = NSStackView(views: [NSView(), declineButton, acceptButton])
        buttons.orientation = .horizontal
        buttons.spacing = 8
        var views: [NSView] = [titleLabel, bodyLabel]
        if let detail { views.append(detail) }
        views += [pathLabel, buttons]
        let stack = NSStackView(views: views)
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 12
        stack.edgeInsets = NSEdgeInsets(top: 24, left: 24, bottom: 24, right: 24)
        stack.widthAnchor.constraint(equalToConstant: 520).isActive = true
        bodyLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -48).isActive = true
        detail?.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -48).isActive = true
        pathLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -48).isActive = true
        buttons.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -48).isActive = true
        contentView = stack
        setContentSize(stack.fittingSize)
        isReleasedWhenClosed = false
        if returnAnswer == .decline { defaultButtonCell = declineButton.cell as? NSButtonCell }
        setAccessibilityIdentifier("question-\(kind)")
    }

    /// Escape when the decline button already holds Return: a button has
    /// one key equivalent, so the second key arrives here, once no view in
    /// the sheet has taken it (labels and buttons take none), or as
    /// `cancelOperation` when a selectable label holds focus and its field
    /// editor turns Escape into that action. A repeat of a held Escape
    /// (one that ended a talk just as this sheet came up) answers nothing.
    override func keyDown(with event: NSEvent) {
        if event.keyCode == 53, returnAnswer == .decline, escape == .decline {
            if !event.isARepeat { declineButton.performClick(nil) }
            return
        }
        super.keyDown(with: event)
    }

    override func cancelOperation(_ sender: Any?) {
        if returnAnswer == .decline, escape == .decline {
            declineButton.performClick(nil)
            return
        }
        super.cancelOperation(sender)
    }

    func button(titled title: String) -> NSButton? {
        [declineButton, acceptButton].first { $0.title == title }
    }

    @objc private func declinePressed(_ sender: Any?) {
        sheetParent?.endSheet(self, returnCode: .cancel)
    }

    @objc private func acceptPressed(_ sender: Any?) {
        sheetParent?.endSheet(self, returnCode: .OK)
    }

    /// tap's record-consent question. `settingsPath` is where tap saves the answer.
    static func consent(settingsPath: String?) -> QuestionSheet {
        QuestionSheet(kind: "record-consent",
                      title: "Record automatically every time you present?",
                      body: "Tap records the projector screen and your microphone from the start of each talk until you stop, and follows the projector if the cable is swapped. You choose whether to keep each recording at the end. The answer is saved for you, not the deck; tap present in Terminal uses it too.",
                      path: settingsPath,
                      decline: "Don't Record",
                      accept: "Record Automatically")
    }

    /// tap's keep-recording question, asked when Stop ends a run that
    /// recorded. `size` is the run folder's size, formatted. Delete is
    /// destructive: no key reaches it, not even Space with keyboard
    /// navigation on, and a stray Escape after the talk does nothing here.
    static func keepRecording(directory: String, segments: Int, size: String) -> QuestionSheet {
        let sheet = QuestionSheet(kind: "keep-recording",
                                  title: "Keep this recording?",
                                  body: "\(segments) segment\(segments == 1 ? "" : "s"), \(size) on disk.",
                                  path: directory,
                                  decline: "Delete",
                                  accept: "Keep and Show in Finder",
                                  escape: .nothing)
        sheet.declineButton.hasDestructiveAction = true
        // With keyboard navigation on, Space presses the focused button:
        // the focus starts on Keep and never reaches Delete.
        sheet.declineButton.refusesFirstResponder = true
        sheet.initialFirstResponder = sheet.acceptButton
        return sheet
    }

    /// The hint before the first talk. Its accept button opens the setting.
    static func focusHint() -> QuestionSheet {
        QuestionSheet(kind: "focus-hint",
                      title: "Turn on a Focus before your talk?",
                      body: "Notifications can appear on the projector. macOS does not let apps turn on a Focus for you.",
                      path: nil,
                      decline: "Not Now",
                      accept: "Open Focus Settings")
    }
}

/// One block of the approval sheet, as the Approval board draws it: the
/// block's place, and its code right under it, always shown, in a
/// monospaced label the person can select.
final class ApprovalBlockRow: NSView {
    let block: ApprovalBlock
    let placeLabel: NSTextField
    let codeLabel: NSTextField

    init(block: ApprovalBlock) {
        self.block = block
        placeLabel = NSTextField(labelWithString: "Slide \(block.slide), block \(block.block)")
        codeLabel = NSTextField(wrappingLabelWithString: block.code)
        super.init(frame: .zero)
        placeLabel.font = .systemFont(ofSize: 11)
        placeLabel.textColor = .secondaryLabelColor
        codeLabel.font = .monospacedSystemFont(ofSize: 11, weight: .regular)
        codeLabel.isSelectable = true
        codeLabel.setAccessibilityIdentifier("approval-block-\(block.slide)-\(block.block)")
        let stack = NSStackView(views: [placeLabel, codeLabel])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 2
        stack.translatesAutoresizingMaskIntoConstraints = false
        addSubview(stack)
        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: topAnchor),
            stack.leadingAnchor.constraint(equalTo: leadingAnchor, constant: 12),
            stack.trailingAnchor.constraint(equalTo: trailingAnchor),
            stack.bottomAnchor.constraint(equalTo: bottomAnchor),
            codeLabel.widthAnchor.constraint(equalTo: stack.widthAnchor),
        ])
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }
}

/// The live code approval: the drivers a yes allows, each with its
/// blocks and slides (and a custom driver's command), every block's code
/// inline under it as the Approval board draws it, and the safe answer
/// as the default button. The title is the spec's; the second time, when
/// the deck gained a driver, it names only the new ones
/// (internal/cli/approval.go builds the request that way). A driver
/// approved before that tap asks about again, because its command changed
/// or a value in it did, reads as the ApprovalCommandChanged board draws
/// it: a badge on its row, the command before (struck out) and now, and a
/// footer saying where the code goes and what stays allowed.
final class ApprovalSheet: QuestionSheet {
    /// Which request the sheet is for.
    enum Wording: Equatable {
        /// The deck was never approved.
        case first
        /// The deck was approved; these drivers are new to it. A request
        /// that also holds a changed driver reads this way, with the badge
        /// on the changed driver's row.
        case newDrivers
        /// Every driver asked about was approved with another command
        /// line (`previousCommand`); some may have only a changed value.
        case changedCommands
        /// Every driver asked about was approved with the same command as
        /// written, and a value in it changed (`valueChanged`).
        case valueChanged
    }

    /// The rows scroll past this height, so a deck with dozens of blocks
    /// never pushes the buttons off the display.
    static let detailMaximumHeight: CGFloat = 320
    let wording: Wording
    let summaryLabel: NSTextField
    let driverLabels: [NSTextField]
    /// For each changed driver, in order: "Before: <command>", struck out,
    /// when its command changed, and "Now: <command>".
    let commandChangeLabels: [NSTextField]
    /// Where the code goes and which drivers stay allowed, under the rows
    /// of a request with a changed driver; nil otherwise.
    let footerLabel: NSTextField?
    let blockRows: [ApprovalBlockRow]
    let detailScrollView: NSScrollView

    init(payload: QuestionPayload, deckName: String) {
        let drivers = payload.drivers ?? []
        let blocks = payload.blocks ?? []
        let names = Self.joined(drivers.map(\.name))
        let quotedName = "\u{201C}\(deckName)\u{201D}"
        let changed = drivers.filter(Self.isChanged)
        let fresh = drivers.filter { !Self.isChanged($0) }
        let changedNames = Self.joined(changed.map(\.name))
        let title: String
        let body: String
        let accept: String
        if !changed.isEmpty, fresh.isEmpty, changed.allSatisfy({ $0.previousCommand == nil }) {
            wording = .valueChanged
            title = "A value in the command for \(changedNames) changed since it was approved"
            body = "You allowed \(changedNames) for \(quotedName) with the command below. It reads the same, but a value in it changed since, or tap can no longer check that approval. Read it before you allow it."
            accept = "Allow \(changedNames)"
        } else if !changed.isEmpty, fresh.isEmpty {
            wording = .changedCommands
            let plural = changed.count > 1
            title = "The command\(plural ? "s" : "") for \(changedNames) changed"
            body = "You allowed \(changedNames) for \(quotedName) when \(plural ? "they" : "it") ran \(plural ? "different commands" : "a different command"). The deck now runs the command\(plural ? "s" : "") below, for example after a git pull. Read \(plural ? "them" : "it") before you allow \(plural ? "them" : "it")."
            accept = "Allow \(changedNames)"
        } else if payload.isForNewDrivers || !changed.isEmpty {
            wording = .newDrivers
            let freshNames = Self.joined(fresh.map(\.name))
            let allowedBefore = Self.joined((payload.approvedBefore ?? []) + changed.map(\.name))
            title = "This deck now also wants to run \(freshNames)"
            body = "You allowed \(allowedBefore) for \(quotedName) before. The deck now declares \(freshNames) too, for example after a git pull. tap runs only the code written in this deck; read it before you allow it."
            accept = "Allow \(names)"
        } else {
            wording = .first
            title = "This deck can run code on your Mac"
            body = "\(quotedName) declares \(drivers.count) driver\(drivers.count == 1 ? "" : "s") and has \(blocks.count) live code block\(blocks.count == 1 ? "" : "s"). tap runs only the code written in this deck; read it before you allow this deck. A yes is remembered for this file; tap approval revoke undoes it."
            accept = "Allow"
        }
        var footer: String?
        if !changed.isEmpty {
            let approvedBefore = payload.approvedBefore ?? []
            let staysAllowed = approvedBefore.isEmpty ? "" : " \(Self.joined(approvedBefore)) \(approvedBefore.count == 1 ? "stays" : "stay") allowed."
            footer = "The code goes to the command on its standard input." + staysAllowed
        }
        let detail = Self.makeDetail(drivers: drivers, blocks: blocks, summary: payload.approvalSummary, footer: footer)
        summaryLabel = detail.summaryLabel
        driverLabels = detail.driverLabels
        commandChangeLabels = detail.commandChangeLabels
        footerLabel = detail.footerLabel
        blockRows = detail.blockRows
        // The rows live in a scroll view that is as tall as they are, up to the maximum; the buttons stay outside it.
        let scroll = NSScrollView()
        scroll.contentView = FlippedClipView()
        scroll.documentView = detail.stack
        scroll.hasVerticalScroller = true
        scroll.drawsBackground = false
        detail.stack.translatesAutoresizingMaskIntoConstraints = false
        NSLayoutConstraint.activate([
            detail.stack.leadingAnchor.constraint(equalTo: scroll.contentView.leadingAnchor),
            detail.stack.trailingAnchor.constraint(equalTo: scroll.contentView.trailingAnchor),
            detail.stack.topAnchor.constraint(equalTo: scroll.contentView.topAnchor),
        ])
        let fits = scroll.heightAnchor.constraint(equalTo: detail.stack.heightAnchor)
        fits.priority = .defaultHigh
        fits.isActive = true
        scroll.heightAnchor.constraint(lessThanOrEqualToConstant: Self.detailMaximumHeight).isActive = true
        detailScrollView = scroll
        super.init(kind: "approval", title: title, body: body, path: payload.deck, decline: "Don't Allow", accept: accept,
                   escape: .decline, returnAnswer: .decline, detail: scroll)
    }

    /// A clip view that starts its content at the top.
    final class FlippedClipView: NSClipView {
        override var isFlipped: Bool { true }
    }

    /// Whether tap asks about the driver again although it was approved:
    /// its command changed, or a value in it did.
    static func isChanged(_ driver: ApprovalDriver) -> Bool {
        driver.previousCommand != nil || driver.valueChanged
    }

    /// The views inside the sheet's scroll view.
    struct Detail {
        let stack: NSStackView
        let summaryLabel: NSTextField
        let driverLabels: [NSTextField]
        let commandChangeLabels: [NSTextField]
        let footerLabel: NSTextField?
        let blockRows: [ApprovalBlockRow]
    }

    private static func makeDetail(drivers: [ApprovalDriver], blocks: [ApprovalBlock], summary: String, footer: String?) -> Detail {
        let detail = NSStackView()
        detail.orientation = .vertical
        detail.alignment = .leading
        detail.spacing = 6
        let summaryLabel = NSTextField(labelWithString: summary)
        summaryLabel.font = .systemFont(ofSize: 12.5, weight: .semibold)
        summaryLabel.setAccessibilityIdentifier("approval-summary")
        detail.addArrangedSubview(summaryLabel)
        var driverLabels: [NSTextField] = []
        var commandChangeLabels: [NSTextField] = []
        var blockRows: [ApprovalBlockRow] = []
        for driver in drivers {
            // tap's own wording (internal/cli/approval.go, describeDriverBlocks).
            var line = driver.blocks == 0 ? "\(driver.name): no blocks yet" : "\(driver.name): \(driver.blocks) block\(driver.blocks == 1 ? "" : "s")"
            if !driver.slides.isEmpty {
                line += " on slide\(driver.slides.count == 1 ? "" : "s") " + driver.slides.map(String.init).joined(separator: ", ")
            }
            if driver.previousCommand != nil {
                line += ", command changed"
            } else if driver.valueChanged {
                line += ", value changed"
            } else if let command = driver.command {
                line += ", runs: \(command)"
            }
            let label = NSTextField(labelWithString: line)
            label.font = .systemFont(ofSize: 12)
            label.setAccessibilityIdentifier("approval-driver-\(driver.name)")
            driverLabels.append(label)
            detail.addArrangedSubview(label)
            if isChanged(driver) {
                // The board's two lines: the command approved before, struck
                // out, and the one the deck runs now. A changed value has no
                // other command to show, so only the one it runs now.
                if let previous = driver.previousCommand {
                    let before = commandLine("Before: \(previous)", struckOut: true)
                    before.setAccessibilityIdentifier("approval-before-\(driver.name)")
                    commandChangeLabels.append(before)
                    detail.addArrangedSubview(before)
                }
                let now = commandLine("Now: \(driver.command ?? "")", struckOut: false)
                now.setAccessibilityIdentifier("approval-now-\(driver.name)")
                commandChangeLabels.append(now)
                detail.addArrangedSubview(now)
            }
            for block in blocks where block.driver == driver.name {
                let row = ApprovalBlockRow(block: block)
                blockRows.append(row)
                detail.addArrangedSubview(row)
                row.widthAnchor.constraint(equalTo: detail.widthAnchor).isActive = true
            }
        }
        var footerLabel: NSTextField?
        if let footer {
            let label = NSTextField(wrappingLabelWithString: footer)
            label.font = .systemFont(ofSize: 11.5)
            label.textColor = .secondaryLabelColor
            label.setAccessibilityIdentifier("approval-footer")
            detail.addArrangedSubview(label)
            label.widthAnchor.constraint(equalTo: detail.widthAnchor).isActive = true
            footerLabel = label
        }
        return Detail(stack: detail, summaryLabel: summaryLabel, driverLabels: driverLabels,
                      commandChangeLabels: commandChangeLabels, footerLabel: footerLabel, blockRows: blockRows)
    }

    /// A command line of a changed driver's row, monospaced and selectable,
    /// struck out for the command approved before.
    private static func commandLine(_ text: String, struckOut: Bool) -> NSTextField {
        let label = NSTextField(labelWithString: text)
        label.font = .monospacedSystemFont(ofSize: 11.5, weight: .regular)
        label.isSelectable = true
        if struckOut {
            label.textColor = .secondaryLabelColor
            label.attributedStringValue = NSAttributedString(string: text, attributes: [
                .font: NSFont.monospacedSystemFont(ofSize: 11.5, weight: .regular),
                .foregroundColor: NSColor.secondaryLabelColor,
                .strikethroughStyle: NSUnderlineStyle.single.rawValue,
            ])
        }
        return label
    }

    /// "shell", "shell and sqlite", "shell, sqlite and mysql".
    static func joined(_ names: [String]) -> String {
        switch names.count {
        case 0: return ""
        case 1: return names[0]
        default: return names.dropLast().joined(separator: ", ") + " and " + names[names.count - 1]
        }
    }
}
