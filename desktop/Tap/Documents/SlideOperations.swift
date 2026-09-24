import AppKit

/// What `perform` did with an operation at the moment it was called.
/// Only `.applied` means the text has changed; a caller that acts on the
/// new slides (selects them, deletes their originals elsewhere, records a
/// layout) does so in the completion, never on `.queued`.
enum SlideOperationOutcome: Equatable {
    /// Applied now. The completion has already run with the result.
    case applied
    /// Waiting for tap to confirm typed text. The completion runs once:
    /// with the result when the operation lands, or with nil when it is
    /// refused then or abandoned (no answer within `confirmationTimeout`).
    case queued
    /// Refused now. The completion has already run with nil.
    case refused

    var isAccepted: Bool { self != .refused }
}

/// The slides a command from a menu, a key or a paste acts on. It is
/// named when the command is invoked and resolved when the command runs,
/// which is after tap's answer when typing is still unconfirmed, and that
/// answer may renumber every slide after the typing. A selection that is
/// the caret's slide follows the caret, which is anchored in the text, so
/// it resolves to the slide the caret is in by then. Any other selection
/// resolves to its numbers only while those slides still hold the text
/// they held when the command was invoked; otherwise the command is
/// refused with a beep, never run on whatever now holds those numbers.
struct SlideSelection {
    fileprivate enum Target {
        case caret
        case slides([Int], markdowns: [String])
    }

    fileprivate let target: Target

    /// The slide the caret is in when the command runs, or none.
    static let caret = SlideSelection(target: .caret)
}

/// Every structural edit on the deck's slides. Each one is one undo step
/// on the buffer, cut along the ranges tap reported: the operation runs
/// only on boxes tap has confirmed; it gives the new text and the boxes
/// of that text; the text goes in as a single replacement through the
/// editor's clamp; the boxes are adopted at once, which makes every
/// answer to an earlier send stale, and tap is sent the text right away.
extension DeckSessionController {
    /// Runs `action` once the editor's text is the text tap last answered
    /// for. Boxes shifted by typing tap has not parsed yet are not tap's
    /// ranges: a "---" typed a moment ago still sits inside the box above
    /// it. If the text is not confirmed, it is sent now and `action` runs
    /// after the answer. If no answer comes within `confirmationTimeout`
    /// (tap restarting, say), `action` never runs: there is a beep and
    /// `abandoned` runs instead, so a caller waiting on the action learns
    /// that it will not happen.
    func whenTextIsConfirmed(_ action: @escaping () -> Void, abandoned: (() -> Void)? = nil) {
        if editor.string == lastAppliedText {
            action()
            return
        }
        Task { @MainActor [weak self] in
            guard let self else {
                abandoned?()
                return
            }
            await self.sourceSync.sendNow()
            let deadline = Date().addingTimeInterval(self.confirmationTimeout)
            while self.editor.string != self.lastAppliedText, Date() < deadline {
                try? await Task.sleep(nanoseconds: 20_000_000)
            }
            guard self.editor.string == self.lastAppliedText else {
                NSSound.beep()
                abandoned?()
                return
            }
            action()
        }
    }

    /// Applies `operation` now when tap has confirmed the text, or queues it
    /// behind the confirmation. `completion` runs exactly once, with the
    /// result when the text has changed and nil when it has not and will
    /// not. Anything that must follow the edit belongs in `completion`.
    @discardableResult
    func perform(_ operation: SlideOperation, completion: ((SlideEditResult?) -> Void)? = nil) -> SlideOperationOutcome {
        guard editor.string == lastAppliedText else {
            whenTextIsConfirmed({ [weak self] in
                // Run before handing the result on: `completion?(...)` with no
                // completion would skip evaluating its argument, the operation too.
                let result = self?.performNow(operation)
                completion?(result)
            }, abandoned: {
                completion?(nil)
            })
            return .queued
        }
        let result = performNow(operation)
        completion?(result)
        return result == nil ? .refused : .applied
    }

    /// Names the selection as it stands, for a command that runs now or
    /// once tap confirms the text. See `SlideSelection`.
    func captureSelection() -> SlideSelection {
        let numbers = selectedSlideNumbers
        if let current = currentSlideNumber, numbers == [current] { return .caret }
        return SlideSelection(target: .slides(numbers, markdowns: markdown(forSlides: numbers)))
    }

    /// The numbers `selection` names now, on tap's ranges, or nil when its
    /// slides no longer hold the text they held when it was named.
    func resolve(_ selection: SlideSelection) -> [Int]? {
        switch selection.target {
        case .caret:
            return currentSlideNumber.map { [$0] } ?? []
        case .slides(let numbers, let markdowns):
            return markdown(forSlides: numbers) == markdowns ? numbers : nil
        }
    }

    /// `perform` for a command on a selection: the operation is built from
    /// the selection's numbers only when it runs, after they are resolved.
    /// A stale selection, or a builder that returns nil, is refused with a
    /// beep, and `completion` gets nil.
    @discardableResult
    func perform(on selection: SlideSelection, _ operation: @escaping ([Int]) -> SlideOperation?,
                 completion: ((SlideEditResult?) -> Void)? = nil) -> SlideOperationOutcome {
        guard editor.string == lastAppliedText else {
            whenTextIsConfirmed({ [weak self] in
                let result = self?.performNow(on: selection, operation)
                completion?(result)
            }, abandoned: {
                completion?(nil)
            })
            return .queued
        }
        let result = performNow(on: selection, operation)
        completion?(result)
        return result == nil ? .refused : .applied
    }

    private func performNow(on selection: SlideSelection, _ operation: ([Int]) -> SlideOperation?) -> SlideEditResult? {
        guard let numbers = resolve(selection), let resolved = operation(numbers) else {
            NSSound.beep()
            return nil
        }
        return performNow(resolved)
    }

    /// The number a slide inserted after `numbers` gets: after the last of
    /// them, or at the end (nil) when they are empty or end the deck.
    func insertionNumber(after numbers: [Int]) -> Int? {
        numbers.max().flatMap { $0 + 1 <= editor.boxes.count ? $0 + 1 : nil }
    }

    /// True when every one of `numbers` is a skipped slide.
    func slidesAreSkipped(_ numbers: [Int]) -> Bool {
        let wanted = Set(numbers)
        let slides = editor.boxes.map(\.slide).filter { wanted.contains($0.number) }
        return !slides.isEmpty && slides.allSatisfy(\.skip)
    }

    @discardableResult
    private func performNow(_ operation: SlideOperation) -> SlideEditResult? {
        let text = editor.string
        let boxes = editor.boxes
        guard let result = SlideEditing.apply(operation, to: text, boxes: boxes, caretOffsetInSlide: caretOffset(for: operation)) else {
            NSSound.beep()
            return nil
        }
        guard let replacement = TextDiff.replacement(from: text, to: result.text), let undoManager = document?.undoManager else { return nil }
        // The operation is one undo step of its own: a top-level group
        // opened and closed here. With no group open, the automatic
        // grouping is paused while the group is open, so the undo manager
        // does not nest an automatic group inside it that would stay open
        // after it and take in the next operation as well. A group that is
        // already open belongs to someone else (the undo manager's own
        // group for this run loop turn, holding typing, or a caller's), so
        // the operation nests inside it and never closes it: the undo
        // manager closes its own group at the end of the turn, and throws
        // when a registration or that close finds its group already gone.
        let pausesAutomaticGrouping = undoManager.groupingLevel == 0 && undoManager.groupsByEvent
        if pausesAutomaticGrouping { undoManager.groupsByEvent = false }
        // One group holds the box adoption and the text change.
        undoManager.beginUndoGrouping()
        registerBoxAdoption(undo: boxes, redo: result.boxes)
        editor.replaceText(in: replacement.range, with: replacement.replacement, actionName: SlideEditing.actionName(for: operation))
        undoManager.endUndoGrouping()
        if pausesAutomaticGrouping { undoManager.groupsByEvent = true }
        // The cursor's own selection sync would collapse a multi-slide
        // selection to the caret's slide; the panel selects the operated
        // slides itself, last.
        withPanelDrivingTheCursor {
            editor.adoptBoxes(result.boxes)
            editor.setSelectedRange(NSRange(location: result.caret, length: 0))
        }
        slidePanel.remapImages(sourceNumbers: result.sourceNumbers)
        slidePanel.setSlides(editor.boxes.map(\.slide))
        slidePanel.select(numbers: result.selectedNumbers, scroll: true)
        if let first = result.selectedNumbers.first, editor.boxes.indices.contains(first - 1) {
            editor.scrollRangeToVisible(editor.boxes[first - 1].range)
        }
        Task { await sourceSync.sendNow() }
        return result
    }

    /// The caret's offset inside the first slide the operation acts on,
    /// when the caret is in it, so the caret moves with the slide.
    private func caretOffset(for operation: SlideOperation) -> Int {
        let numbers: [Int]
        switch operation {
        case .move(let moved, _): numbers = moved
        case .duplicate(let selected), .delete(let selected): numbers = selected
        case .setSkip(let selected, _): numbers = selected
        case .insert: return 0
        }
        guard let first = numbers.min(), let index = editor.currentBoxIndex, editor.boxes[index].slide.number == first else { return 0 }
        return max(0, editor.selectedRange().location - editor.boxes[index].range.location)
    }

    /// Registers, in the current undo group, that after the text is undone
    /// the boxes `undo` belong to it, and after it is redone `redo` do.
    /// AppKit reverts the text through the text storage, which shifts the
    /// boxes by the inverse edit and leaves them wrong. The closure only
    /// notes which boxes to adopt; `undoOrRedoDidChangeText`, which
    /// runs on the undo manager's notification once the whole group has
    /// run, adopts them and then sends the text to tap. The undo manager is
    /// looked up from the target each time, never captured.
    private func registerBoxAdoption(undo: [SlideBox], redo: [SlideBox]) {
        document?.undoManager?.registerUndo(withTarget: self) { target in
            target.registerBoxAdoption(undo: redo, redo: undo)
            target.pendingBoxAdoption = undo
        }
    }

    /// Called by `undoOrRedoDidChangeText` with the boxes the reverted or
    /// redone text has.
    func adoptPendingBoxes(_ boxes: [SlideBox]) {
        guard boxes.map(\.range).allSatisfy({ NSMaxRange($0) <= (editor.string as NSString).length }) else { return }
        editor.adoptBoxes(boxes)
        slidePanel.setSlides(editor.boxes.map(\.slide))
        syncPanelSelectionToCursor()
    }

    /// Moves the selection named now up (-1) or down (+1) by one slide. The
    /// selection is resolved when the move runs, through `perform(on:)`, so
    /// a hand-made selection is checked against tap's ranges rather than
    /// read fresh off the panel, which keeps its selection by number across
    /// a renumbering answer.
    @discardableResult
    func moveSelectedSlides(by offset: Int) -> Bool {
        perform(on: captureSelection()) { [weak self] numbers in
            guard let self, let first = numbers.min(), let last = numbers.max() else { return nil }
            let count = self.editor.boxes.count
            if offset < 0 {
                guard first > 1 else { return nil }
                return .move(numbers: numbers, beforeNumber: first - 1)
            }
            guard last < count else { return nil }
            return .move(numbers: numbers, beforeNumber: last + 2 <= count ? last + 2 : nil)
        }.isAccepted
    }

    @discardableResult
    func moveSelectedSlides(toTop: Bool) -> Bool {
        perform(on: captureSelection()) { numbers in
            guard !numbers.isEmpty else { return nil }
            return .move(numbers: numbers, beforeNumber: toTop ? 1 : nil)
        }.isAccepted
    }

    /// The text of each slide, from tap's ranges.
    func markdown(forSlides numbers: [Int]) -> [String] {
        let text = editor.string as NSString
        return numbers.compactMap { number in
            editor.boxes.first { $0.slide.number == number }.map { text.substring(with: $0.range) }
        }
    }

    @discardableResult
    func insertSlides(markdowns: [String], beforeNumber: Int?, completion: ((SlideEditResult?) -> Void)? = nil) -> SlideOperationOutcome {
        perform(.insert(markdowns: markdowns, beforeNumber: beforeNumber), completion: completion)
    }

    /// Inserts one slide after `selection` (none, or the last slide: at
    /// the end) and selects its first slot, so typing replaces the
    /// placeholder. The selection and the focus wait for the insert to
    /// land: before then, the new slide's number is still an old slide's.
    /// `completion` gets true once the slide exists, false when the insert
    /// is refused or abandoned.
    @discardableResult
    func insertNewSlide(markdown: String, after selection: SlideSelection, completion: ((Bool) -> Void)? = nil) -> SlideOperationOutcome {
        perform(on: selection, { [weak self] numbers in
            .insert(markdowns: [markdown], beforeNumber: self?.insertionNumber(after: numbers))
        }, completion: { [weak self] result in
            guard let self, let newNumber = result?.selectedNumbers.first else {
                completion?(false)
                return
            }
            if self.editor.boxes.indices.contains(newNumber - 1) {
                let box = self.editor.boxes[newNumber - 1]
                let slot = SlideEditing.firstSlotRange(inSlideText: (self.editor.string as NSString).substring(with: box.range))
                self.editor.setSelectedRange(NSRange(location: box.range.location + slot.location, length: slot.length))
                self.editor.window?.makeFirstResponder(self.editor)
            }
            completion?(true)
        })
    }

    /// Copies slides as the app's own type and as plain markdown.
    func copySlides(_ numbers: [Int], to pasteboard: NSPasteboard) {
        guard let payload = dragPayload(forSlides: numbers), let data = try? payload.data() else { return }
        pasteboard.clearContents()
        let item = NSPasteboardItem()
        item.setData(data, forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType))
        item.setString(payload.markdowns.joined(separator: SlideDocument.separator), forType: .string)
        pasteboard.writeObjects([item])
    }

    /// Pastes slides after `selection`: the app's own type when present,
    /// otherwise plain text as one slide. The pasteboard is read now; the
    /// place is resolved when the insert runs.
    @discardableResult
    func pasteSlides(from pasteboard: NSPasteboard, after selection: SlideSelection) -> Bool {
        let markdowns: [String]
        if let data = pasteboard.data(forType: NSPasteboard.PasteboardType(SlideDragPayload.pasteboardType)), let payload = SlideDragPayload(data: data) {
            markdowns = payload.markdowns
        } else if let text = pasteboard.string(forType: .string), !text.trimmingCharacters(in: .whitespacesAndNewlines).isEmpty {
            markdowns = [text]
        } else {
            return false
        }
        return perform(on: selection) { [weak self] numbers in
            .insert(markdowns: markdowns, beforeNumber: self?.insertionNumber(after: numbers))
        }.isAccepted
    }

    func dragPayload(forSlides numbers: [Int]) -> SlideDragPayload? {
        guard let deck = document?.fileURL, !numbers.isEmpty else { return nil }
        let sorted = numbers.sorted()
        return SlideDragPayload(deckPath: deck.path, slideNumbers: sorted, markdowns: markdown(forSlides: sorted))
    }

    /// A drop of slides: a move within this deck; from another deck, an
    /// insert here and, unless the drop is a copy, a delete there. Each
    /// deck registers its own undo step. The source gives the
    /// slides up only in the insert's completion, once they are in this
    /// deck's text: an insert that waits for tap's confirmation and is then
    /// abandoned (no answer while tap restarts) leaves the source as it was,
    /// so the slides are never only in an undo stack. The return value says
    /// whether the drop was taken, applied or queued.
    @discardableResult
    func dropSlides(payload: SlideDragPayload, beforeNumber: Int?, isMove: Bool) -> Bool {
        if let deck = document?.fileURL, payload.comesFrom(deck: deck) {
            // The same staleness the cross-deck move guards against: the
            // numbers were taken at drag start, and typing since then may
            // have renumbered the slides they now name. Refuse rather than
            // move whatever now holds those numbers; one Cmd+Z is not
            // needed because nothing changed. A drop queued behind tap's
            // answer checks again once the answer is in, since that answer
            // is what renumbers the slides.
            guard markdown(forSlides: payload.slideNumbers) == payload.markdowns else {
                NSSound.beep()
                return false
            }
            let move = SlideOperation.move(numbers: payload.slideNumbers, beforeNumber: beforeNumber)
            guard editor.string == lastAppliedText else {
                whenTextIsConfirmed { [weak self] in
                    guard let self else { return }
                    guard self.markdown(forSlides: payload.slideNumbers) == payload.markdowns else {
                        NSSound.beep()
                        return
                    }
                    self.performNow(move)
                }
                return true
            }
            return perform(move).isAccepted
        }
        let source = isMove ? Self.document(forDeckPath: payload.deckPath)?.sessionController : nil
        return perform(.insert(markdowns: payload.markdowns, beforeNumber: beforeNumber)) { [weak self, weak source] result in
            guard result != nil, let source, source !== self else { return }
            source.removeMovedSlides(payload)
        }.isAccepted
    }

    /// Deletes the slides a drop moved out of this deck, once this deck's
    /// own text is confirmed, and only while those slides still hold the
    /// text that was dragged. Slides edited or shifted in the meantime stay,
    /// with a beep: at worst a move becomes a copy, never a loss.
    func removeMovedSlides(_ payload: SlideDragPayload) {
        whenTextIsConfirmed { [weak self] in
            guard let self else { return }
            guard self.markdown(forSlides: payload.slideNumbers) == payload.markdowns else {
                NSSound.beep()
                return
            }
            self.performNow(.delete(numbers: payload.slideNumbers))
        }
    }

    static func document(forDeckPath path: String) -> DeckDocument? {
        let url = URL(fileURLWithPath: path)
        return NSDocumentController.shared.documents.compactMap { $0 as? DeckDocument }
            .first { $0.fileURL.map { FilePaths.same($0, url) } ?? false }
    }
}
