import AppKit

/// The Deck tab: a form over the frontmatter, one field per key that
/// tap's schema lists (`tap deck schema --json`), so Swift hard-codes no
/// key. Every change is one edit of the frontmatter through the editor,
/// one undo step named after the field; the form re-reads the text after
/// every change to it, so an undo, a typed edit or a disk load shows here
/// too. A key with fixed nested keys (an object) is a card of fields; a
/// map of named entries (the drivers) is a card per entry (Task 12); keys
/// the schema does not list are read-only rows under Other keys (Task 12).
final class DeckFormViewController: NSViewController, NSTextFieldDelegate {
    /// The deck's text now. The session controller sets it.
    var text: () -> String = { "" }
    /// Applies one edit to the frontmatter as one undo step with the name given.
    var applyEdit: (TextReplacement, String) -> Void = { _, _ in }
    private(set) var keys: [SchemaKey] = []
    private(set) var deckErrors: [String] = []
    /// Every field, by its key path joined with ".", such as "theme",
    /// "recording.output" or "drivers.sqlite.timeout".
    private(set) var fields: [String: NSControl] = [:]
    private(set) var bindings: [(path: [String], control: NSControl)] = []
    let stack = NSStackView()
    let errorTitleLabel = NSTextField(labelWithString: "The deck settings have a problem")
    let errorLabel = NSTextField(wrappingLabelWithString: "")
    let errorHintLabel = NSTextField(labelWithString: "The frontmatter is shown in the editor until it parses.")
    let scrollView = NSScrollView()
    /// What the form's shape was last built from (the entries of each map
    /// key, the raw blocks, the unknown keys), set once the rows exist; a
    /// difference on refresh rebuilds.
    private(set) var builtForEntries: [String: [String]] = [:]
    private var isRefreshing = false
    /// True while `rebuild` runs, so the commit of an edit in progress
    /// (which ends editing, which fires `controlChanged`) applies its edit
    /// once and does not refresh or rebuild from inside the rebuild.
    private var isRebuilding = false

    final class FlippedClipView: NSClipView {
        override var isFlipped: Bool { true }
    }

    override func loadView() {
        stack.orientation = .vertical
        stack.alignment = .leading
        stack.spacing = 14
        stack.edgeInsets = NSEdgeInsets(top: 4, left: 20, bottom: 20, right: 20)
        stack.translatesAutoresizingMaskIntoConstraints = false
        errorTitleLabel.font = .systemFont(ofSize: 13, weight: .semibold)
        errorTitleLabel.textColor = EditorPalette.error
        errorLabel.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
        errorHintLabel.font = .systemFont(ofSize: 12)
        errorHintLabel.textColor = .secondaryLabelColor
        errorLabel.setAccessibilityIdentifier("deck-form-error")
        let clip = FlippedClipView()
        scrollView.contentView = clip
        scrollView.documentView = stack
        scrollView.hasVerticalScroller = true
        scrollView.drawsBackground = false
        NSLayoutConstraint.activate([
            stack.leadingAnchor.constraint(equalTo: clip.leadingAnchor),
            stack.trailingAnchor.constraint(equalTo: clip.trailingAnchor),
            stack.topAnchor.constraint(equalTo: clip.topAnchor),
        ])
        scrollView.setAccessibilityIdentifier("deck-form")
        view = scrollView
        rebuild()
    }

    func setSchema(_ keys: [SchemaKey]) {
        self.keys = keys
        if isViewLoaded { rebuild() }
    }

    func setDeckErrors(_ errors: [String]) {
        guard errors != deckErrors else { return }
        deckErrors = errors
        if isViewLoaded { rebuild() }
    }

    func field(_ path: String) -> NSControl? {
        fields[path]
    }

    /// The popup item that stands for "no key": tap's own default.
    func defaultItemTitle(for key: SchemaKey) -> String {
        key.defaultValue.map { "Default (\($0))" } ?? "Default"
    }

    /// Reads the text again: the fields' values follow it, and the rows
    /// are rebuilt when the frontmatter's shape changed (an entry added
    /// to a map, a raw block changed, a key tap does not know). Nothing
    /// runs while the view is hidden (the Preview tab is up); `showTab`
    /// calls it when the Deck tab comes up. The field being typed in keeps
    /// what was typed.
    func refresh() {
        guard isViewLoaded, !view.isHiddenOrHasHiddenAncestor, deckErrors.isEmpty else { return }
        let frontmatter = Frontmatter(text: text())
        guard entries(in: frontmatter) == builtForEntries else {
            rebuild()
            return
        }
        refreshValues(from: frontmatter)
    }

    /// The field editor of the form's control being typed in, if any.
    private var editingText: NSText? {
        guard let editing = view.window?.firstResponder as? NSText, let editingView = editing as? NSView, editingView.isDescendant(of: view) else { return nil }
        return editing
    }

    private func refreshValues(from frontmatter: Frontmatter) {
        isRefreshing = true
        defer { isRefreshing = false }
        let editing = editingText
        for binding in bindings {
            if let editing, editing.delegate === binding.control { continue }
            guard let key = DeckSchema.key(at: binding.path, in: keys) else { continue }
            show(frontmatter.value(at: binding.path), in: binding.control, for: key)
        }
    }

    private func show(_ value: String?, in control: NSControl, for key: SchemaKey) {
        switch control {
        // NSPopUpButton is an NSButton: its case comes first, or a button case would take it.
        case let popup as NSPopUpButton:
            guard let value else {
                popup.selectItem(withTitle: defaultItemTitle(for: key))
                return
            }
            let text = Frontmatter.unquoted(value)
            if popup.itemTitles.contains(text) { popup.selectItem(withTitle: text) } else { popup.selectItem(at: -1) }
        case let toggle as NSSwitch:
            let text = value ?? key.defaultValue ?? "false"
            toggle.state = ["true", "yes", "on"].contains(text.lowercased()) ? .on : .off
        case let field as NSTextField:
            field.stringValue = value.map(Frontmatter.unquoted) ?? ""
            field.placeholderString = key.defaultValue
        default:
            break
        }
    }

    /// What the form's shape depends on: the entries under each map key
    /// (block or flow style), the text of every raw block a map entry has
    /// (a non-scalar setting that exists, from the schema), and every key
    /// the schema does not list. Read from the frontmatter and the schema
    /// alone, never from the rows, so a build and a refresh compare the
    /// same thing.
    func entries(in frontmatter: Frontmatter) -> [String: [String]] {
        var result: [String: [String]] = [:]
        for key in keys where key.type == "map" {
            let names = frontmatter.entryNames(at: [key.name])
            result[key.name] = names
            for name in names {
                for child in key.keys where !child.isScalar {
                    let path = [key.name, name, child.name]
                    if let raw = frontmatter.rawBlock(at: path) { result["raw:" + path.joined(separator: ".")] = [raw] }
                }
            }
        }
        result["*"] = frontmatter.entries.map(\.key).filter { name in !keys.contains { $0.name == name } }
        return result
    }

    /// Ends an edit in progress in one of the form's fields, so its text
    /// reaches the frontmatter before the rows are torn down or the file
    /// is written. `NSViewController` is an `NSEditor`; the document's save
    /// and `saveForPresenting` call this first.
    override func commitEditing() -> Bool {
        guard isViewLoaded, let window = view.window, editingText != nil else { return true }
        return window.makeFirstResponder(nil)
    }

    /// Writes what is typed so far without ending the edit, for an
    /// autosave: the person keeps typing, and the file has the text so far.
    func commitEditingKeepingFocus() {
        guard isViewLoaded, let editing = editingText else { return }
        if let field = editing.delegate as? NSTextField, bindings.contains(where: { $0.control === field }) {
            controlChanged(field)
        } else if let textView = editing as? NSTextView {
            applyRawEditor(textView)
        }
    }

    func rebuild() {
        guard isViewLoaded, !isRebuilding else { return }
        isRebuilding = true
        defer { isRebuilding = false }
        _ = commitEditing()
        for view in stack.arrangedSubviews {
            stack.removeArrangedSubview(view)
            view.removeFromSuperview()
        }
        fields = [:]
        bindings = []
        clearMapRows()
        guard deckErrors.isEmpty else {
            errorLabel.stringValue = deckErrors[0]
            for label in [errorTitleLabel, errorLabel, errorHintLabel] {
                stack.addArrangedSubview(label)
                label.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -40).isActive = true
            }
            builtForEntries = [:]
            return
        }
        let frontmatter = Frontmatter(text: text())
        let scalars = keys.filter(\.isScalar)
        if !scalars.isEmpty {
            addSection(title: "Deck", rows: scalars.map { row(for: $0, path: [$0.name]) })
        }
        for key in keys where key.type == "object" {
            addSection(title: key.label, rows: key.keys.filter(\.isScalar).map { row(for: $0, path: [key.name, $0.name]) })
        }
        for key in keys where key.type == "map" {
            addMapSection(for: key, in: frontmatter)
        }
        addOtherKeysSection(frontmatter)
        refreshValues(from: frontmatter)
        // Set last: the rows above are what this shape was built for.
        builtForEntries = entries(in: frontmatter)
    }

    /// A map's card per entry, the raw editors and the Other keys rows arrive in Task 12.
    func addMapSection(for key: SchemaKey, in frontmatter: Frontmatter) {}
    func addOtherKeysSection(_ frontmatter: Frontmatter) {}
    func clearMapRows() {}
    func applyRawEditor(_ textView: NSTextView) {}

    func addSection(title: String, rows: [NSView]) {
        let box = NSBox()
        box.title = title
        box.titlePosition = .atTop
        box.titleFont = .systemFont(ofSize: 12, weight: .semibold)
        let column = NSStackView(views: rows)
        column.orientation = .vertical
        column.alignment = .leading
        column.spacing = 8
        column.edgeInsets = NSEdgeInsets(top: 8, left: 8, bottom: 8, right: 8)
        box.contentView = column
        box.setAccessibilityIdentifier("deck-section-\(title)")
        stack.addArrangedSubview(box)
        box.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -40).isActive = true
    }

    func row(for key: SchemaKey, path: [String]) -> NSView {
        let label = NSTextField(labelWithString: key.label)
        label.alignment = .right
        label.textColor = .secondaryLabelColor
        label.font = .systemFont(ofSize: 12)
        label.widthAnchor.constraint(equalToConstant: 130).isActive = true
        let control = makeControl(for: key, path: path)
        let row = NSStackView(views: [label, control])
        row.orientation = .horizontal
        row.alignment = .firstBaseline
        row.spacing = 10
        control.widthAnchor.constraint(greaterThanOrEqualToConstant: 200).isActive = true
        return row
    }

    private func makeControl(for key: SchemaKey, path: [String]) -> NSControl {
        let control: NSControl
        switch key.type {
        case "boolean":
            let toggle = NSSwitch()
            toggle.target = self
            toggle.action = #selector(controlChanged(_:))
            control = toggle
        case "string" where !key.values.isEmpty:
            let popup = NSPopUpButton(frame: .zero, pullsDown: false)
            popup.addItems(withTitles: [defaultItemTitle(for: key)] + key.values)
            popup.target = self
            popup.action = #selector(controlChanged(_:))
            control = popup
        default:
            let field = NSTextField(string: "")
            field.delegate = self
            field.target = self
            field.action = #selector(controlChanged(_:))
            field.font = key.type == "list" ? .monospacedSystemFont(ofSize: 12, weight: .regular) : .systemFont(ofSize: 13)
            control = field
        }
        let identifier = path.joined(separator: ".")
        control.setAccessibilityIdentifier("deck-field-\(identifier)")
        control.toolTip = key.description
        fields[identifier] = control
        bindings.append((path, control))
        return control
    }

    /// A field changed: the key gets the value as YAML would read it back,
    /// or goes when the field is emptied or the popup's Default is chosen.
    /// A value the form cannot write (a pair inside a flow map) beeps and
    /// the field reads the text again.
    @objc func controlChanged(_ sender: NSControl) {
        guard !isRefreshing, let path = bindings.first(where: { $0.control === sender })?.path, let key = DeckSchema.key(at: path, in: keys) else { return }
        let frontmatter = Frontmatter(text: text())
        let raw: String?
        switch sender {
        case let popup as NSPopUpButton:
            raw = popup.titleOfSelectedItem.flatMap { $0 == defaultItemTitle(for: key) ? nil : Frontmatter.scalar(forString: $0) }
        case let toggle as NSSwitch:
            raw = toggle.state == .on ? "true" : "false"
        default:
            let typed = sender.stringValue.trimmingCharacters(in: .whitespaces)
            if typed.isEmpty {
                raw = nil
            } else if key.type == "integer" {
                guard Int(typed) != nil else {
                    NSSound.beep()
                    refresh()
                    return
                }
                raw = typed
            } else if key.type == "list" {
                raw = typed
            } else {
                raw = Frontmatter.scalar(forString: typed)
            }
        }
        guard raw != frontmatter.value(at: path) else { return }
        guard let replacement = frontmatter.setting(path: path, to: raw) else {
            NSSound.beep()
            refresh()
            return
        }
        applyEdit(replacement, "Change \(key.label)")
        if !isRebuilding { refresh() }
    }

    func controlTextDidEndEditing(_ notification: Notification) {
        guard let field = notification.object as? NSTextField else { return }
        controlChanged(field)
    }
}
