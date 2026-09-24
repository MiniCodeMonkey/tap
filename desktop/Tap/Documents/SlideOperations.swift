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
                completion?(self?.performNow(operation))
            }, abandoned: {
                completion?(nil)
            })
            return .queued
        }
        let result = performNow(operation)
        completion?(result)
        return result == nil ? .refused : .applied
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
    /// notes which boxes to adopt; D2's `undoOrRedoDidChangeText`, which
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

    /// Moves the selection up (-1) or down (+1) by one slide. The selection
    /// is read once the text is confirmed, so the slides that move are the
    /// ones tap counts, not the ones a shifted box suggests.
    @discardableResult
    func moveSelectedSlides(by offset: Int) -> Bool {
        guard editor.string == lastAppliedText else {
            whenTextIsConfirmed { [weak self] in _ = self?.moveSelectedSlides(by: offset) }
            return true
        }
        let numbers = selectedSlideNumbers
        guard let first = numbers.min(), let last = numbers.max() else { return false }
        let count = editor.boxes.count
        if offset < 0 {
            guard first > 1 else { return false }
            return performNow(.move(numbers: numbers, beforeNumber: first - 1)) != nil
        }
        guard last < count else { return false }
        return performNow(.move(numbers: numbers, beforeNumber: last + 2 <= count ? last + 2 : nil)) != nil
    }

    @discardableResult
    func moveSelectedSlides(toTop: Bool) -> Bool {
        guard editor.string == lastAppliedText else {
            whenTextIsConfirmed { [weak self] in _ = self?.moveSelectedSlides(toTop: toTop) }
            return true
        }
        let numbers = selectedSlideNumbers
        guard !numbers.isEmpty else { return false }
        return performNow(.move(numbers: numbers, beforeNumber: toTop ? 1 : nil)) != nil
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

    func dragPayload(forSlides numbers: [Int]) -> SlideDragPayload? {
        guard let deck = document?.fileURL, !numbers.isEmpty else { return nil }
        let sorted = numbers.sorted()
        return SlideDragPayload(deckPath: deck.path, slideNumbers: sorted, markdowns: markdown(forSlides: sorted))
    }

    /// A drop of slides: a move within this deck; from another deck, an
    /// insert here and, unless the drop is a copy, a delete there. Each
    /// deck registers its own undo step (decision 2). The source gives the
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
            // needed because nothing changed.
            guard markdown(forSlides: payload.slideNumbers) == payload.markdowns else {
                NSSound.beep()
                return false
            }
            return perform(.move(numbers: payload.slideNumbers, beforeNumber: beforeNumber)).isAccepted
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
