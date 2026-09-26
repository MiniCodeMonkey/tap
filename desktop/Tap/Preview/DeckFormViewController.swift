import AppKit

/// The Deck tab: a form over the frontmatter, one field per key that
/// tap's schema lists (`tap deck schema --json`), so Swift hard-codes no
/// key. Every change is one edit of the frontmatter through the editor,
/// one undo step named after the field; the form re-reads the text after
/// every change to it, so an undo, a typed edit or a disk load shows here
/// too. A key with fixed nested keys (an object) is a card of fields; a
/// map of named entries (the drivers) is a card per entry (Task 12); keys
/// the schema does not list are read-only rows under Other keys (Task 12).
final class DeckFormViewController: NSViewController, NSTextFieldDelegate, NSTextViewDelegate {
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
    /// True while `discardEditing` ends an edit: the field's text goes, and
    /// nothing is written.
    private var isDiscarding = false
    private var hintLabels: [String: NSTextField] = [:]
    private var addFields: [String: NSTextField] = [:]
    private var addButtons: [String: NSButton] = [:]
    private var removeButtons: [String: NSButton] = [:]
    private var rawEditors: [(path: [String], textView: NSTextView)] = []
    private(set) var otherKeyLabels: [NSTextField] = []

    func hintLabel(for map: String) -> NSTextField? { hintLabels[map] }
    func addEntryField(for map: String) -> NSTextField? { addFields[map] }
    func addEntryButton(for map: String) -> NSButton? { addButtons[map] }
    func removeButton(for path: String) -> NSButton? { removeButtons[path] }
    func rawEditor(_ path: String) -> NSTextView? { rawEditors.first { $0.path.joined(separator: ".") == path }?.textView }

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
        // Each control class takes one case: none of these is a subclass of another.
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

    /// Ends an edit in progress without writing it: for a rebuild while tap
    /// reports the frontmatter broken, where a typed value would be written
    /// into a frontmatter nobody can read. The frontmatter shows in the
    /// editor, where the person fixes it.
    override func discardEditing() {
        guard isViewLoaded, let window = view.window, editingText != nil else { return }
        isDiscarding = true
        defer { isDiscarding = false }
        window.makeFirstResponder(nil)
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
        // An edit in progress reaches the frontmatter before its row goes,
        // unless tap reports the frontmatter broken.
        if deckErrors.isEmpty { _ = commitEditing() } else { discardEditing() }
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

    func clearMapRows() {
        hintLabels = [:]
        addFields = [:]
        addButtons = [:]
        removeButtons = [:]
        rawEditors = []
        otherKeyLabels = []
    }

    /// A map key (the drivers): a card per entry the frontmatter declares,
    /// its name and Remove in the header, the entry's scalar settings as
    /// fields, its deeper structure (connections, a block-style args list)
    /// as its own lines of text; under the cards a name field with Add for
    /// a new entry, and the hint that keeps secrets out of the deck. The
    /// entry names come from the text, the settings from the schema; the
    /// built-in driver names are tap's and the person types one, so Swift
    /// lists none.
    func addMapSection(for key: SchemaKey, in frontmatter: Frontmatter) {
        var rows: [NSView] = []
        for name in frontmatter.entryNames(at: [key.name]) {
            let entryPath = [key.name, name]
            let remove = NSButton(title: "Remove", target: self, action: #selector(removePressed(_:)))
            remove.bezelStyle = .rounded
            remove.controlSize = .small
            remove.setAccessibilityIdentifier("deck-remove-\(entryPath.joined(separator: "."))")
            removeButtons[entryPath.joined(separator: ".")] = remove
            let header = NSStackView(views: [NSTextField(labelWithString: name), NSView(), remove])
            header.orientation = .horizontal
            var entryRows: [NSView] = [header]
            for child in key.keys {
                let path = entryPath + [child.name]
                if child.isScalar, frontmatter.entry(at: path)?.isMultiLine != true {
                    entryRows.append(row(for: child, path: path))
                } else if frontmatter.entry(at: path) != nil {
                    entryRows.append(rawRow(for: child, path: path, in: frontmatter))
                }
            }
            let card = NSBox()
            card.titlePosition = .noTitle
            let column = NSStackView(views: entryRows)
            column.orientation = .vertical
            column.alignment = .leading
            column.spacing = 8
            column.edgeInsets = NSEdgeInsets(top: 6, left: 6, bottom: 6, right: 6)
            card.contentView = column
            header.widthAnchor.constraint(equalTo: column.widthAnchor, constant: -12).isActive = true
            rows.append(card)
        }
        let nameField = NSTextField(string: "")
        nameField.placeholderString = "shell, sqlite, mysql, postgres, or a custom name"
        nameField.setAccessibilityIdentifier("deck-add-\(key.name)")
        nameField.widthAnchor.constraint(greaterThanOrEqualToConstant: 260).isActive = true
        let add = NSButton(title: "Add", target: self, action: #selector(addPressed(_:)))
        add.bezelStyle = .rounded
        add.setAccessibilityIdentifier("deck-add-button-\(key.name)")
        addFields[key.name] = nameField
        addButtons[key.name] = add
        let addRow = NSStackView(views: [nameField, add])
        addRow.orientation = .horizontal
        addRow.spacing = 8
        rows.append(addRow)
        let hint = NSTextField(wrappingLabelWithString: "Use ${NAME} for passwords and other secrets: tap reads NAME from your login shell's environment when it runs the driver, so the deck is safe to share.")
        hint.font = .systemFont(ofSize: 11)
        hint.textColor = .secondaryLabelColor
        hint.setAccessibilityIdentifier("deck-hint-\(key.name)")
        hintLabels[key.name] = hint
        rows.append(hint)
        addSection(title: key.label, rows: rows)
        for card in rows.compactMap({ $0 as? NSBox }) { card.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -60).isActive = true }
        hint.widthAnchor.constraint(equalTo: stack.widthAnchor, constant: -60).isActive = true
    }

    /// A setting the form has no field for, as its own lines: the entry's
    /// text as written, indented as it is, put back where it was.
    private func rawRow(for key: SchemaKey, path: [String], in frontmatter: Frontmatter) -> NSView {
        let label = NSTextField(labelWithString: key.label)
        label.alignment = .right
        label.textColor = .secondaryLabelColor
        label.font = .systemFont(ofSize: 12)
        label.widthAnchor.constraint(equalToConstant: 130).isActive = true
        let caption = NSTextField(labelWithString: "YAML, as written in the frontmatter")
        caption.font = .systemFont(ofSize: 10)
        caption.textColor = .tertiaryLabelColor
        let textView = NSTextView(frame: NSRect(x: 0, y: 0, width: 300, height: 72))
        textView.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
        textView.isRichText = false
        textView.isAutomaticQuoteSubstitutionEnabled = false
        textView.string = frontmatter.rawBlock(at: path) ?? ""
        textView.delegate = self
        textView.setAccessibilityIdentifier("deck-raw-\(path.joined(separator: "."))")
        let scroll = NSScrollView()
        scroll.documentView = textView
        scroll.hasVerticalScroller = true
        scroll.borderType = .bezelBorder
        scroll.heightAnchor.constraint(equalToConstant: 72).isActive = true
        scroll.widthAnchor.constraint(greaterThanOrEqualToConstant: 300).isActive = true
        textView.autoresizingMask = [.width]
        rawEditors.append((path, textView))
        let column = NSStackView(views: [caption, scroll])
        column.orientation = .vertical
        column.alignment = .leading
        column.spacing = 2
        let row = NSStackView(views: [label, column])
        row.orientation = .horizontal
        row.alignment = .top
        row.spacing = 10
        return row
    }

    /// Keys the schema does not list, as written and read-only.
    func addOtherKeysSection(_ frontmatter: Frontmatter) {
        let unknown = frontmatter.entries.filter { entry in !keys.contains { $0.name == entry.key } }
        guard !unknown.isEmpty else { return }
        var rows: [NSView] = []
        for entry in unknown {
            let label = NSTextField(wrappingLabelWithString: frontmatter.text(of: entry).trimmingCharacters(in: .newlines))
            label.font = .monospacedSystemFont(ofSize: 12, weight: .regular)
            label.isSelectable = true
            label.setAccessibilityIdentifier("deck-other-\(entry.key)")
            otherKeyLabels.append(label)
            rows.append(label)
        }
        addSection(title: "Other keys", rows: rows)
    }

    @objc private func addPressed(_ sender: NSButton) {
        guard let map = addButtons.first(where: { $0.value === sender })?.key, let field = addFields[map] else { return }
        _ = commitEditing()
        let name = field.stringValue.trimmingCharacters(in: .whitespaces)
        guard !name.isEmpty, name.range(of: "^[A-Za-z0-9_-]+$", options: .regularExpression) != nil else {
            NSSound.beep()
            return
        }
        let frontmatter = Frontmatter(text: text())
        guard !frontmatter.entryNames(at: [map]).contains(name), let replacement = frontmatter.setting(path: [map, name], to: "{}") else {
            NSSound.beep()
            return
        }
        field.stringValue = ""
        applyEdit(replacement, "Add \(name)")
        refresh()
    }

    @objc private func removePressed(_ sender: NSButton) {
        guard let joined = removeButtons.first(where: { $0.value === sender })?.key else { return }
        _ = commitEditing()
        let path = joined.split(separator: ".").map(String.init)
        guard let name = path.last, let replacement = Frontmatter(text: text()).setting(path: path, to: nil) else {
            // A pair inside a flow map is not rewritten: said, not swallowed.
            NSSound.beep()
            return
        }
        applyEdit(replacement, "Remove \(name)")
        refresh()
    }

    /// A raw editor lost focus, or an autosave took its text: its lines
    /// replace the entry's, with the file's own line endings and one at
    /// the end. A line that is "---" would close the frontmatter there,
    /// and a first line shallower than the entry's indent would leave its
    /// parent: both are refused with a beep, and the text view reads the
    /// block again.
    func applyRawEditor(_ textView: NSTextView) {
        guard !isDiscarding, let binding = rawEditors.first(where: { $0.textView === textView }), let key = DeckSchema.key(at: binding.path, in: keys) else { return }
        let frontmatter = Frontmatter(text: text())
        var raw = textView.string.replacingOccurrences(of: "\r\n", with: "\n").replacingOccurrences(of: "\n", with: frontmatter.lineEnding)
        if !raw.hasSuffix(frontmatter.lineEnding) { raw += frontmatter.lineEnding }
        guard raw != frontmatter.rawBlock(at: binding.path), let entry = frontmatter.entry(at: binding.path) else { return }
        let lines = raw.components(separatedBy: frontmatter.lineEnding)
        let firstIndent = lines.first.map { $0.prefix { $0 == " " }.count } ?? 0
        guard !lines.contains(where: { $0.trimmingCharacters(in: .whitespaces) == "---" }), firstIndent >= entry.indent,
              let replacement = frontmatter.settingRawBlock(at: binding.path, to: raw) else {
            NSSound.beep()
            textView.string = frontmatter.rawBlock(at: binding.path) ?? ""
            return
        }
        applyEdit(replacement, "Change \(key.label)")
        if !isRebuilding { refresh() }
    }

    func textDidEndEditing(_ notification: Notification) {
        guard let textView = notification.object as? NSTextView else { return }
        applyRawEditor(textView)
    }

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
        guard !isRefreshing, !isDiscarding, let path = bindings.first(where: { $0.control === sender })?.path, let key = DeckSchema.key(at: path, in: keys) else { return }
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
