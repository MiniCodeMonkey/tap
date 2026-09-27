import AppKit

struct GenerateImageRequest: Equatable {
    var prompt: String
    var aspect: String?
    var matchTheme: Bool

    func arguments(deck: URL, slide: Int) -> [String] {
        var arguments = ["image", "generate", deck.path, "--slide", String(slide), "--prompt", prompt]
        if let aspect { arguments += ["--aspect", aspect] }
        if matchTheme { arguments.append("--match-theme") }
        return arguments + ["--json"]
    }
}

/// Slide > Generate Image: the prompt, Match theme and the aspect, then
/// tap image generate on the saved deck. tap's error shows in the sheet,
/// with a way to Settings for a missing key.
final class GenerateImageSheet: QuestionSheet, NSTextViewDelegate {
    let promptView = NSTextView()
    let matchThemeSwitch = NSSwitch()
    let aspectControl = NSSegmentedControl(labels: ["16:9", "1:1", "4:3"], trackingMode: .selectOne, target: nil, action: nil)
    let errorLabel = NSTextField(wrappingLabelWithString: "")
    let settingsButton = NSButton(title: "Settings…", target: nil, action: #selector(AppDelegate.showSettings(_:)))
    var onGenerate: ((GenerateImageRequest) -> Void)?
    var generateButton: NSButton { acceptButton }
    var cancelButton: NSButton { declineButton }

    var request: GenerateImageRequest {
        GenerateImageRequest(prompt: promptView.string.trimmingCharacters(in: .whitespacesAndNewlines),
                             aspect: aspectControl.label(forSegment: aspectControl.selectedSegment), matchTheme: matchThemeSwitch.state == .on)
    }

    init(slide: Int, themeName: String) {
        let form = NSView()
        super.init(kind: "generate-image", title: "Generate Image for Slide \(slide)", body: "", path: nil, decline: "Cancel", accept: "Generate", escape: .decline, returnAnswer: .accept, detail: form)
        bodyLabel.isHidden = true
        acceptButton.target = self
        acceptButton.action = #selector(generatePressed(_:))
        let scroll = NSScrollView()
        scroll.documentView = promptView
        scroll.hasVerticalScroller = true
        scroll.borderType = .bezelBorder
        promptView.isRichText = false
        promptView.font = .systemFont(ofSize: 13)
        promptView.delegate = self
        promptView.setAccessibilityIdentifier("generate-image-prompt")
        scroll.heightAnchor.constraint(equalToConstant: 72).isActive = true
        matchThemeSwitch.state = .on
        matchThemeSwitch.setAccessibilityIdentifier("generate-image-match-theme")
        aspectControl.selectedSegment = 0
        aspectControl.setAccessibilityIdentifier("generate-image-aspect")
        errorLabel.textColor = .systemRed
        errorLabel.font = .systemFont(ofSize: 12)
        errorLabel.isHidden = true
        settingsButton.isHidden = true
        settingsButton.bezelStyle = .rounded
        let describe = NSTextField(labelWithString: "Describe the image")
        describe.font = .systemFont(ofSize: 11, weight: .semibold)
        describe.textColor = .secondaryLabelColor
        let styleHint = NSTextField(labelWithString: "Uses tap theme show --prompt for the \(themeName) style.")
        styleHint.font = .systemFont(ofSize: 11)
        styleHint.textColor = .secondaryLabelColor
        let note = NSTextField(wrappingLabelWithString: "Runs tap image generate. The image is saved to images/ and added to the end of slide \(slide) with its prompt.")
        note.font = .systemFont(ofSize: 11.5)
        note.textColor = .secondaryLabelColor
        let stack = NSStackView(views: [describe, scroll, row("Style", "Match theme", matchThemeSwitch, hint: styleHint), row("Aspect", nil, aspectControl), note, errorLabel, settingsButton])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 10
        stack.translatesAutoresizingMaskIntoConstraints = false
        form.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: form.topAnchor), stack.bottomAnchor.constraint(equalTo: form.bottomAnchor),
            stack.leadingAnchor.constraint(equalTo: form.leadingAnchor), stack.trailingAnchor.constraint(equalTo: form.trailingAnchor),
            scroll.widthAnchor.constraint(equalTo: stack.widthAnchor), note.widthAnchor.constraint(equalTo: stack.widthAnchor),
            errorLabel.widthAnchor.constraint(equalTo: stack.widthAnchor),
        ])
        setContentSize(contentView?.fittingSize ?? frame.size)
        generateButton.isEnabled = false
    }

    private func row(_ title: String, _ controlTitle: String?, _ control: NSView, hint: NSTextField? = nil) -> NSView {
        let label = NSTextField(labelWithString: title)
        label.font = .systemFont(ofSize: 11, weight: .semibold)
        label.textColor = .secondaryLabelColor
        var right: [NSView] = []
        if let controlTitle { right.append(NSTextField(labelWithString: controlTitle)) }
        right.append(control)
        let controls = NSStackView(views: right)
        controls.spacing = 8
        let line = NSStackView(views: [label, NSView(), controls])
        line.orientation = .horizontal
        var lines: [NSView] = [line]
        if let hint { lines.append(hint) }
        let stack = NSStackView(views: lines)
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 2
        return stack
    }

    func textDidChange(_ notification: Notification) {
        generateButton.isEnabled = !request.prompt.isEmpty
        errorLabel.isHidden = true
        settingsButton.isHidden = true
    }

    @objc private func generatePressed(_ sender: Any?) {
        generateButton.isEnabled = false
        onGenerate?(request)
    }

    /// tap's message; the Settings button for a missing key.
    func showError(_ message: String, code: String) {
        errorLabel.stringValue = message
        errorLabel.isHidden = false
        settingsButton.isHidden = code != "no_api_key"
        generateButton.isEnabled = true
    }
}
