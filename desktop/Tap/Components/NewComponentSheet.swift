import AppKit

struct NewComponentRequest: Equatable {
    var name: String
    var inline: Bool
    var typeScript: Bool

    /// tap's own rule for the name; the sheet checks it before tap has to.
    static let namePattern = try! NSRegularExpression(pattern: "^[A-Z][A-Za-z0-9]*$")
    var isValidName: Bool { Self.namePattern.firstMatch(in: name, range: NSRange(location: 0, length: (name as NSString).length)) != nil }

    var fileName: String { "\(inline ? "components" : "slides")/\(name).\(typeScript ? "tsx" : "jsx")" }

    func arguments(deck: URL) -> [String] {
        var arguments = ["component", "new", name, deck.path]
        if inline { arguments.append("--inline") }
        if typeScript { arguments.append("--ts") }
        return arguments + ["--json"]
    }
}

/// Slide > New Component: the name, the kind and TypeScript, then tap
/// component new; the snippet goes in at the caret and the file opens in
/// the default code editor.
final class NewComponentSheet: QuestionSheet, NSTextFieldDelegate {
    let nameField = NSTextField(string: "")
    let kindControl = NSSegmentedControl(labels: ["Whole slide", "Inline block"], trackingMode: .selectOne, target: nil, action: nil)
    let typeScriptSwitch = NSSwitch()
    let errorLabel = NSTextField(wrappingLabelWithString: "")
    private let note = NSTextField(wrappingLabelWithString: "")
    private let slide: Int
    var onCreate: ((NewComponentRequest) -> Void)?
    var createButton: NSButton { acceptButton }
    var cancelButton: NSButton { declineButton }

    var request: NewComponentRequest {
        NewComponentRequest(name: nameField.stringValue.trimmingCharacters(in: .whitespaces), inline: kindControl.selectedSegment == 1, typeScript: typeScriptSwitch.state == .on)
    }

    init(slide: Int) {
        self.slide = slide
        let form = NSView()
        super.init(kind: "new-component", title: "New Component", body: "", path: nil, decline: "Cancel", accept: "Create", escape: .decline, returnAnswer: .accept, detail: form)
        bodyLabel.isHidden = true
        acceptButton.target = self
        acceptButton.action = #selector(createPressed(_:))
        nameField.placeholderString = "PascalCase"
        nameField.delegate = self
        nameField.setAccessibilityIdentifier("new-component-name")
        kindControl.selectedSegment = 0
        kindControl.target = self
        kindControl.action = #selector(nameChanged(_:))
        kindControl.setAccessibilityIdentifier("new-component-kind")
        typeScriptSwitch.target = self
        typeScriptSwitch.action = #selector(nameChanged(_:))
        typeScriptSwitch.setAccessibilityIdentifier("new-component-typescript")
        note.font = .systemFont(ofSize: 11.5)
        note.textColor = .secondaryLabelColor
        errorLabel.textColor = .systemRed
        errorLabel.font = .systemFont(ofSize: 12)
        errorLabel.isHidden = true
        let stack = NSStackView(views: [row("Name", nameField), row("Kind", kindControl), row("TypeScript", typeScriptSwitch), note, errorLabel])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 10
        stack.translatesAutoresizingMaskIntoConstraints = false
        form.addSubview(stack)
        NSLayoutConstraint.activate([
            stack.topAnchor.constraint(equalTo: form.topAnchor), stack.bottomAnchor.constraint(equalTo: form.bottomAnchor),
            stack.leadingAnchor.constraint(equalTo: form.leadingAnchor), stack.trailingAnchor.constraint(equalTo: form.trailingAnchor),
            nameField.widthAnchor.constraint(equalToConstant: 260), note.widthAnchor.constraint(equalTo: stack.widthAnchor), errorLabel.widthAnchor.constraint(equalTo: stack.widthAnchor),
        ])
        setContentSize(contentView?.fittingSize ?? frame.size)
        nameChanged(nameField)
    }

    private func row(_ title: String, _ control: NSView) -> NSView {
        let label = NSTextField(labelWithString: title)
        label.font = .systemFont(ofSize: 13)
        let row = NSStackView(views: [label, NSView(), control])
        row.orientation = .horizontal
        row.spacing = 12
        return row
    }

    @objc func nameChanged(_ sender: Any?) {
        let request = request
        createButton.isEnabled = request.isValidName
        note.stringValue = request.isValidName
            ? "Runs tap component new. The app inserts the snippet into slide \(slide) and opens \(request.fileName) in your code editor."
            : "Runs tap component new. The name is a PascalCase identifier, for example RollingDeploy."
        errorLabel.isHidden = true
        // Hiding a stack view's arranged subview collapses its space; the
        // window shrinks back to its no-error size whenever the name changes.
        setContentSize(contentView?.fittingSize ?? frame.size)
    }

    func controlTextDidChange(_ notification: Notification) { nameChanged(notification.object) }

    @objc private func createPressed(_ sender: Any?) {
        createButton.isEnabled = false
        onCreate?(request)
    }

    /// tap's message, shown on its own row; the sheet grows to hold it,
    /// since a hidden row's space is collapsed by the stack view until now.
    func showError(_ message: String) {
        errorLabel.stringValue = message
        errorLabel.isHidden = false
        createButton.isEnabled = true
        setContentSize(contentView?.fittingSize ?? frame.size)
    }
}
