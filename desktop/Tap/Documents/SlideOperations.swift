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
        // AppKit opens its own per-event group lazily, on the first undo
        // registration of a run loop turn, and closes it only when that
        // turn was a real NSEvent's. Two operations performed back to back
        // with no real event in between (a menu command driven straight
        // from code, or a hosted test) share that one dangling group
        // unless it is drained first, which would let one undo revert both
        // operations at once. Draining it before opening this operation's
        // own group makes each operation gets its own top-level group
        // regardless of whether a real event closed the previous one.
        while undoManager.groupingLevel > 0 { undoManager.endUndoGrouping() }
        // One group holds the box adoption and the text change. AppKit's
        // event group would hold both on its own; the explicit group makes
        // that a property of this code rather than of the run loop.
        undoManager.beginUndoGrouping()
        registerBoxAdoption(undo: boxes, redo: result.boxes)
        editor.replaceText(in: replacement.range, with: replacement.replacement, actionName: SlideEditing.actionName(for: operation))
        undoManager.endUndoGrouping()
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
}
