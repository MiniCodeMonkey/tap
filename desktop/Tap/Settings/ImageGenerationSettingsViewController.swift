import AppKit

/// Image Generation: the Gemini key, in the Keychain, passed to tap's
/// image runs as GEMINI_API_KEY. The field is the one place the key is
/// shown, as bullets. With a key in the login shell the field is off:
/// that key wins. Built on FormCard, not an NSBox whose contentView is a
/// stack (Task 12's known layout bug: the box collapses to its title).
final class ImageGenerationSettingsViewController: NSViewController, NSTextFieldDelegate {
    let keyField: NSTextField = NSSecureTextField(string: "")
    /// The hint under "API key", inside the card, as the SettingsImageBullets board draws it.
    let sourceLabel = NSTextField(wrappingLabelWithString: "")
    private(set) var card: FormCard!

    override init(nibName: NSNib.Name?, bundle: Bundle?) {
        super.init(nibName: nil, bundle: nil)
        title = "Image Generation"
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    override func loadView() {
        keyField.placeholderString = "Paste your key"
        keyField.delegate = self
        keyField.target = self
        keyField.action = #selector(keyChanged(_:))
        keyField.setAccessibilityIdentifier("gemini-key")
        keyField.widthAnchor.constraint(equalToConstant: 220).isActive = true
        sourceLabel.font = .systemFont(ofSize: 11)
        sourceLabel.textColor = .secondaryLabelColor
        sourceLabel.setAccessibilityIdentifier("gemini-key-source")
        sourceLabel.widthAnchor.constraint(lessThanOrEqualToConstant: 380).isActive = true

        let name = NSTextField(labelWithString: "API key")
        name.font = .systemFont(ofSize: 13)
        let labels = NSStackView(views: [name, sourceLabel])
        labels.orientation = .vertical
        labels.alignment = .leading
        labels.spacing = 2

        let row = FormCard.row(leading: [labels], trailing: [keyField])
        card = FormCard(rows: [row])
        card.setAccessibilityIdentifier("settings-card-Gemini")

        let section = FormCard.section(title: "Gemini", content: [card])
        let stack = NSStackView(views: [section])
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 20
        stack.edgeInsets = NSEdgeInsets(top: 20, left: 28, bottom: 20, right: 28)
        section.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -56).isActive = true
        stack.widthAnchor.constraint(equalToConstant: 760).isActive = true
        view = stack
    }

    override func viewWillAppear() {
        super.viewWillAppear()
        Task { @MainActor [weak self] in await self?.refresh() }
    }

    /// The field from the store and the label from where tap's key comes
    /// from. This and TapTool's image runs are the two reads of the store.
    func refresh() async {
        let source = await AppEnvironment.shared.geminiKeySource()
        keyField.stringValue = (try? AppEnvironment.shared.geminiKeyStore.read()) ?? ""
        switch source {
        case .shell:
            keyField.isEnabled = false
            sourceLabel.stringValue = "Your shell sets GEMINI_API_KEY, so tap uses that key; the Keychain's is not used."
        case .keychain, .none:
            keyField.isEnabled = true
            sourceLabel.stringValue = "Stored in your Keychain. GEMINI_API_KEY from your shell takes precedence."
        }
    }

    /// The field's value goes to the store; an empty field removes the key.
    @objc func keyChanged(_ sender: Any?) {
        let typed = keyField.stringValue.trimmingCharacters(in: .whitespacesAndNewlines)
        do {
            try AppEnvironment.shared.geminiKeyStore.write(typed.isEmpty ? nil : typed)
        } catch {
            // The error is a KeychainError.status: an OSStatus, never the key. Nothing holding the key may be interpolated here.
            sourceLabel.stringValue = "The Keychain refused the key: \(error)"
        }
    }

    func controlTextDidEndEditing(_ notification: Notification) { keyChanged(notification.object) }
}
