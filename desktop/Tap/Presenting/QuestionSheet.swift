import AppKit

/// A question tap asked, or the Focus hint, as a sheet on the deck window:
/// a title, a body, an optional path line and two buttons. A sheet belongs
/// to its window; the rest of the app keeps running and nothing is
/// app-modal. Return is the accept button. Escape is the decline button
/// only where declining is harmless; a destructive decline has no key,
/// so a stray Escape after a talk (Escape is how a talk ends) can never
/// choose it.
final class QuestionSheet: NSWindow {
    enum EscapeAnswer {
        /// Escape presses the decline button.
        case decline
        /// Escape does nothing; the person clicks.
        case nothing
    }

    let kind: String
    let titleLabel = NSTextField(labelWithString: "")
    let bodyLabel = NSTextField(wrappingLabelWithString: "")
    let pathLabel = NSTextField(labelWithString: "")
    let declineButton = NSButton(title: "", target: nil, action: nil)
    let acceptButton = NSButton(title: "", target: nil, action: nil)

    init(kind: String, title: String, body: String, path: String?, decline: String, accept: String, escape: EscapeAnswer = .decline) {
        self.kind = kind
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
        declineButton.keyEquivalent = escape == .decline ? "\u{1b}" : ""
        declineButton.target = self
        declineButton.action = #selector(declinePressed(_:))
        acceptButton.title = accept
        acceptButton.bezelStyle = .rounded
        acceptButton.keyEquivalent = "\r"
        acceptButton.target = self
        acceptButton.action = #selector(acceptPressed(_:))
        let buttons = NSStackView(views: [NSView(), declineButton, acceptButton])
        buttons.orientation = .horizontal
        buttons.spacing = 8
        let stack = NSStackView(views: [titleLabel, bodyLabel, pathLabel, buttons])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 12
        stack.edgeInsets = NSEdgeInsets(top: 24, left: 24, bottom: 24, right: 24)
        stack.widthAnchor.constraint(equalToConstant: 460).isActive = true
        bodyLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -48).isActive = true
        pathLabel.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -48).isActive = true
        buttons.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -48).isActive = true
        contentView = stack
        setContentSize(stack.fittingSize)
        isReleasedWhenClosed = false
        setAccessibilityIdentifier("question-\(kind)")
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
    /// destructive: no key reaches it, and a stray Escape after the talk
    /// does nothing here.
    static func keepRecording(directory: String, segments: Int, size: String) -> QuestionSheet {
        let sheet = QuestionSheet(kind: "keep-recording",
                                  title: "Keep this recording?",
                                  body: "\(segments) segment\(segments == 1 ? "" : "s"), \(size) on disk.",
                                  path: directory,
                                  decline: "Delete",
                                  accept: "Keep and Show in Finder",
                                  escape: .nothing)
        sheet.declineButton.hasDestructiveAction = true
        return sheet
    }
}
