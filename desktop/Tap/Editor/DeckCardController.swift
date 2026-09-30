import AppKit

/// The Deck card at the top of the editor, above slide 1: the one
/// foldable element of the editor. Closed, it is a line that sums the deck
/// up (theme, aspect ratio, author) and shows a problem as a chip. Open,
/// it is the deck's settings, as the schema's form or as the frontmatter's
/// own lines in the editor's text. Both change the deck through the
/// editor's edit path, so each change is one undo step in the editor's
/// undo stack.
///
/// A deck with no frontmatter has the card too: the form offers the
/// fields, and the first change writes a frontmatter block.
@MainActor
final class DeckCardController: NSObject {
    let form = DeckFormViewController()
    let cardView = DeckCardView()
    private weak var editor: EditorTextView?

    // MARK: What the session gives the card

    /// The deck's text now.
    var text: () -> String = { "" }
    /// tap's schema keys; empty until they load.
    var schema: () -> [SchemaKey] = { [] }
    /// The deck file, for the open state remembered per deck; nil for a deck with no file yet.
    var deckURL: () -> URL? = { nil }
    /// A theme's display name from tap's catalog, the slug while it loads.
    var themeName: (String) -> String = { $0 }
    /// Applies one edit to the deck as one undo step with the name given.
    var applyEdit: (TextReplacement, String) -> Void = { _, _ in }
    /// The deck's problems changed, so the preview and the toolbar can say so.
    var onProblemsChange: (() -> Void)?

    // MARK: State

    private(set) var isOpen = false
    /// The person chose Text over Form.
    private(set) var prefersText = false
    private(set) var problems: [DeckProblem] = []
    private(set) var deckErrors: [String] = []
    private var knownErrorIdentities: Set<String> = []
    private var knownDeckError: String?
    private var isLayingOut = false
    private var shownChips: [DeckProblems.Chip] = []

    override init() {
        super.init()
        cardView.setBodyContent(form.view)
        form.showsErrorState = false
        cardView.onToggle = { [weak self] in self?.toggle() }
        cardView.onModeChange = { [weak self] isText in self?.setTextMode(isText) }
        form.onLayoutChange = { [weak self] in
            guard let self, !self.isLayingOut else { return }
            self.layout()
        }
        form.problemFix = { [weak self] problem in
            guard let self, let fix = self.fix(for: problem) else { return nil }
            return (fix.title, { [weak self] in self?.apply(fix) })
        }
    }

    /// Puts the card on the editor. The editor tells the card when its width changes.
    func attach(to editor: EditorTextView) {
        self.editor = editor
        editor.installDeckCard(cardView)
        editor.onDeckCardWidthChange = { [weak self] in self?.layout() }
    }

    /// Reads the remembered open state for the deck, once the session can name it.
    func restoreOpenState() {
        isOpen = deckURL().map { AppEnvironment.shared.panelState.isDeckCardOpen(deck: $0) } ?? false
        layout()
    }

    // MARK: What the card shows

    /// The frontmatter as it is now.
    private var frontmatter: Frontmatter { Frontmatter(text: text()) }

    /// Whether the frontmatter can be shown and edited as text: it exists,
    /// or tap says it does not parse and the text is where to fix it.
    var textIsAvailable: Bool { frontmatter.hasFrontmatter || !deckErrors.isEmpty }

    /// What the card shows now. Frontmatter that does not parse is always
    /// shown as text, with the failing line marked.
    var display: EditorTextView.DeckCardDisplay {
        guard isOpen else { return .collapsed }
        if !deckErrors.isEmpty { return .text }
        return prefersText && textIsAvailable ? .text : .form
    }

    /// The problems that stop the preview.
    var errors: [DeckProblem] { problems.filter { $0.severity == .error } }

    func setDeckErrors(_ errors: [String]) {
        guard errors != deckErrors else { return }
        deckErrors = errors
        form.setDeckErrors(errors)
        if let first = errors.first, first != knownDeckError, !isOpen { isOpen = true }
        knownDeckError = errors.first
        refresh()
    }

    /// Reads the text again: the summary, the problems, and the form. Runs
    /// after every change to the text, an undo, a disk load or tap's answer.
    func refresh() {
        let parsed = frontmatter
        let keys = schema()
        let found = keys.isEmpty ? [] : DeckProblems.evaluate(parsed, schema: keys)
        let changed = found != problems
        problems = found
        // A new error opens the card once; a card the person closed stays closed while the same error stands.
        let identities = Set(errors.map { "\($0.key)=\($0.value)" })
        if !identities.subtracting(knownErrorIdentities).isEmpty, !isOpen { isOpen = true }
        knownErrorIdentities = identities
        let chips = DeckProblems.chips(for: parsed, schema: keys, problems: problems, themeName: themeName)
        if chips != shownChips {
            shownChips = chips
            cardView.setChips(chips)
        }
        form.setProblems(problems)
        layout()
        if changed { onProblemsChange?() }
    }

    /// Sizes the card to what it shows and hands the editor the result.
    func layout() {
        guard let editor, !isLayingOut else { return }
        isLayingOut = true
        defer { isLayingOut = false }
        let shown = display
        form.view.isHidden = shown != .form
        var bodyHeight: CGFloat = 0
        if shown == .form {
            // The form reads the text again once it can be seen: it does nothing while hidden.
            form.refresh()
            form.columnCount = cardView.frame.width >= Self.twoColumnWidth ? 2 : 1
            cardView.layoutBodyForMeasuring()
            form.view.layoutSubtreeIfNeeded()
            bodyHeight = min(max(form.contentHeight, 64), Self.maximumBodyHeight)
        }
        cardView.setState(display: shown, bodyHeight: bodyHeight, textIsAvailable: textIsAvailable)
        editor.setDeckCard(display: shown, bodyHeight: bodyHeight, tint: tint, markedLines: markedLines())
        if shown == .form { form.view.layoutSubtreeIfNeeded() }
    }

    /// The card is wide enough for two columns of fields from this width.
    static let twoColumnWidth: CGFloat = 620
    /// A form taller than this scrolls inside the card.
    static let maximumBodyHeight: CGFloat = 420

    private var tint: EditorTextView.DeckCardTint {
        if !deckErrors.isEmpty || !errors.isEmpty { return .error }
        return problems.isEmpty ? .none : .warning
    }

    /// The lines of the text the problems and the parse failure name, for Text mode.
    private func markedLines() -> [(line: Int, isError: Bool)] {
        let parsed = frontmatter
        let whole = text() as NSString
        func line(ofLocation location: Int) -> Int {
            whole.substring(to: min(location, whole.length)).components(separatedBy: "\n").count - 1
        }
        var lines: [(line: Int, isError: Bool)] = []
        if let failing = DeckProblems.failingLine(inDeckErrors: deckErrors) { lines.append((failing, true)) }
        for problem in problems {
            if let entry = parsed.entry(at: problem.path) { lines.append((line(ofLocation: entry.range.location), problem.severity == .error)) }
        }
        return lines
    }

    // MARK: Opening and closing

    func toggle() {
        setOpen(!isOpen)
    }

    /// Opens or closes the card, and remembers it for the deck.
    func setOpen(_ open: Bool) {
        guard open != isOpen || (open && display == .collapsed) else { return }
        isOpen = open
        if let deck = deckURL() { AppEnvironment.shared.panelState.setDeckCardOpen(open, deck: deck) }
        layout()
    }

    func setTextMode(_ isText: Bool) {
        prefersText = isText
        _ = form.commitEditing()
        layout()
        if isText, deckErrors.isEmpty { editor?.window?.makeFirstResponder(editor) }
    }

    /// Show Deck Settings: the card opens, and the focus goes into it: the
    /// field of `key` when the form shows it, otherwise the form's first
    /// field, or the frontmatter's text in Text mode.
    func showAndFocus(key: String? = nil) {
        if !isOpen { setOpen(true) } else { layout() }
        guard let window = editor?.window else { return }
        switch display {
        case .form:
            if let control = key.flatMap({ form.field($0) }) ?? form.bindings.first?.control {
                window.makeFirstResponder(control)
            } else {
                window.makeFirstResponder(cardView.disclosureButton)
            }
        case .text:
            editor?.moveCursorToFrontmatter()
            window.makeFirstResponder(editor)
        case .collapsed:
            window.makeFirstResponder(cardView.disclosureButton)
        }
    }

    // MARK: Fixes

    /// The fix a problem's button offers: its title names a theme by its display name.
    func fix(for problem: DeckProblem) -> DeckProblemFix? {
        let name: (String) -> String = { [themeName] value in problem.key == "theme" ? themeName(value) : value }
        return DeckProblems.fix(for: problem, in: frontmatter, schema: schema(), displayName: name)
    }

    /// Writes a value over a problem's setting: one edit, one undo step.
    func apply(_ fix: DeckProblemFix) {
        applyEdit(fix.replacement, fix.actionName)
    }

    /// The fix that writes a suggestion the person picked.
    func fix(setting key: String, to value: String) -> DeckProblemFix? {
        let name: (String) -> String = { [themeName] value in key == "theme" ? themeName(value) : value }
        return DeckProblems.fix(setting: key, to: value, in: frontmatter, displayName: name)
    }

    // MARK: Editing in progress

    /// Ends an edit in one of the form's fields, so its text reaches the deck.
    @discardableResult
    func commitEditing() -> Bool {
        form.commitEditing()
    }

    /// Writes what is typed in a form field so far, without ending the edit.
    func commitEditingKeepingFocus() {
        form.commitEditingKeepingFocus()
    }
}
