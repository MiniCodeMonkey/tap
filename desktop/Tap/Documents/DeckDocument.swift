import AppKit

/// A deck: a plain .md file, which stays the only source of truth.
final class DeckDocument: NSDocument {
    /// The deck file's content, as last read from disk (open, revert) or as
    /// last written by a save that wrote the deck's own file. This is what
    /// the session controller compares the editor's text against to decide
    /// whether the document is edited.
    private(set) var text = "" { didSet { textRevision += 1 } }
    /// Bumped by every assignment to `text`, whatever the source: opening
    /// the file, a revert, an outside change adopted through
    /// `adopt(diskText:)`, or a save's own completion below. A save's
    /// completion only adopts its snapshot when this still reads the value
    /// it captured when the snapshot was taken, so a save that started
    /// before a newer `adopt(diskText:)` cannot overwrite that newer text
    /// with its own, older, still in-flight snapshot once it completes.
    private(set) var textRevision = 0
    /// The exact text handed to `data(ofType:)` for a save of this
    /// document's own file, taken at the moment the data was produced, for
    /// as long as that save is still in flight. A person may keep typing
    /// while the save writes to disk, so this, not the editor's text when
    /// the save finishes, is what `text` becomes on success: it is what
    /// actually reached the file. Also read by `DeckSessionController.
    /// diskChanged()` to recognize the app's own autosave write before
    /// "saved" is processed: the write reaches disk before that message
    /// does, so a `file-changed` report can arrive while this is still the
    /// save in flight. Exists only for a same-file save: `savingOwnFile`
    /// below keeps `data(ofType:)` from setting it at all for a save known
    /// to land elsewhere (Save To), and
    /// `save(to:ofType:for:completionHandler:)` clears it once its save
    /// completes, successfully or not.
    private(set) var savedSnapshot: String?
    /// `textRevision` as of the moment `savedSnapshot` was taken.
    private var savedSnapshotRevision = 0
    /// Whether the save now in flight through
    /// `save(to:ofType:for:completionHandler:)`, if any, is known to land on
    /// this document's own file. True by default, and while nothing is
    /// saving through that method: a save is presumed to be one of this
    /// deck's own file unless it names a different destination, which is
    /// also what lets a test drive `data(ofType:)` directly to stand in for
    /// a real own-file save without going through that method at all. Only
    /// Save To (and any other save to an explicitly different destination)
    /// sets this false, for the duration of that one save. Save As is an
    /// own-file save: its destination becomes this document's file once the
    /// save completes, and tap should be told about it the same as Save.
    private var savingOwnFile = true
    private(set) var sessionController: DeckSessionController?
    /// The file name of a deck whose file was deleted, until it is saved
    /// again.
    private(set) var deletedName: String?

    override class var autosavesInPlace: Bool { true }

    override nonisolated var fileURL: URL? {
        didSet {
            MainActor.assumeIsolated {
                guard let url = self.fileURL, oldValue.map({ !FilePaths.same($0, url) }) ?? true else { return }
                self.deletedName = nil
                self.sessionController?.deckMoved(to: url)
            }
        }
    }

    /// Keeps the deleted file's name showing until the document is saved
    /// again, in place of NSDocument's own fallback for a document with no
    /// file.
    override var displayName: String! {
        get { deletedName.map { ($0 as NSString).deletingPathExtension } ?? super.displayName }
        set { super.displayName = newValue }
    }

    override func makeWindowControllers() {
        let controller = DeckSessionController(document: self)
        sessionController = controller
        addWindowController(DeckWindowController(sessionController: controller))
        controller.start()
    }

    /// Opens the window as a tab of an open deck window.
    override func showWindows() {
        WelcomeWindowController.closeIfOpen()
        if let window = windowControllers.first?.window, !window.isVisible,
           let existing = NSApp.windows.first(where: { $0 !== window && $0.isVisible && $0.windowController is DeckWindowController }) {
            existing.addTabbedWindow(window, ordered: .above)
        }
        super.showWindows()
    }

    // NSDocument's overrides are not themselves main-actor isolated, but
    // AppKit calls them on the main thread for a local file document, which
    // is the only kind this app opens.
    override nonisolated func read(from data: Data, ofType typeName: String) throws {
        let text = String(decoding: data, as: UTF8.self)
        MainActor.assumeIsolated {
            self.text = text
            self.sessionController?.documentDidRead(text)
        }
    }

    override nonisolated func data(ofType typeName: String) throws -> Data {
        MainActor.assumeIsolated {
            let snapshot = self.sessionController?.editor.string ?? self.text
            if self.savingOwnFile {
                self.savedSnapshot = snapshot
                self.savedSnapshotRevision = self.textRevision
            }
            return Data(snapshot.utf8)
        }
    }

    override nonisolated func close() {
        MainActor.assumeIsolated { self.sessionController?.stop() }
        super.close()
    }

    // MARK: Changes on disk

    private var diskModificationDate: Date? {
        guard let path = fileURL?.path else { return nil }
        return (try? FileManager.default.attributesOfItem(atPath: path))?[.modificationDate] as? Date
    }

    /// Records text read from outside the document, such as a `file-changed`
    /// event, as the deck file's known content, ahead of replacing the
    /// editor's text to match it. Setting this first keeps the edited flag,
    /// which compares the editor's text against this value, from reading
    /// edited between the two steps.
    func adopt(diskText: String) {
        text = diskText
    }

    /// Takes the file on disk as the known version, so the next save does
    /// not report that another program changed it.
    func acceptDiskState() {
        if let date = diskModificationDate { fileModificationDate = date }
    }

    /// The File menu's Save (Cmd-S) reaches NSDocument's default
    /// `saveDocument(_:)`, which calls this method (`saveDocumentWithDelegate:`
    /// in its underlying Objective-C form) directly, never through
    /// `save(to:ofType:for:completionHandler:)` or
    /// `autosave(withImplicitCancellability:completionHandler:)` below, for a
    /// document whose class declares `autosavesInPlace`: confirmed by direct
    /// observation that calling `save(_:)` while a conflict is showing writes
    /// nothing and quietly clears `isDocumentEdited` even before this guard
    /// existed, without ever reaching either of those overrides. Blocking it
    /// here, before calling super at all, is what stops that: nothing runs,
    /// the bar stays up, and the person's edits stay marked edited.
    override func save(withDelegate delegate: Any?, didSave didSaveSelector: Selector?, contextInfo: UnsafeMutableRawPointer?) {
        if sessionController?.hasDiskConflict == true { return }
        super.save(withDelegate: delegate, didSave: didSaveSelector, contextInfo: contextInfo)
    }

    /// Writes the buffer over the file that changed on disk.
    func overwriteDisk(completion: @escaping (Error?) -> Void) {
        guard let url = fileURL else { return completion(nil) }
        acceptDiskState()
        save(to: url, ofType: fileType ?? "net.daringfireball.markdown", for: .saveOperation, completionHandler: completion)
    }

    /// tap reports content changes with its `file-changed` event, and the
    /// app loads them as an undoable edit through `diskChanged()`, so this
    /// does not reload the document itself. A change can also be a
    /// deletion, which is checked for directly here, since a deletion is
    /// not something tap's own `file-changed` event reports.
    override func presentedItemDidChange() {
        DispatchQueue.main.async { [weak self] in self?.sessionController?.checkFileStillExists() }
    }

    override func accommodatePresentedItemDeletion(completionHandler: @escaping (Error?) -> Void) {
        DispatchQueue.main.async { [weak self] in
            self?.fileWasDeleted()
            completionHandler(nil)
        }
    }

    /// Keeps the buffer as an unsaved document with no file, so autosave
    /// does not write the deleted file back.
    func fileWasDeleted() {
        guard deletedName == nil, let url = fileURL else { return }
        deletedName = url.lastPathComponent
        markFileGone()
        fileURL = nil
        sessionController?.deckWasDeleted(name: url.lastPathComponent)
    }

    /// Records that the deck's file no longer exists, distinctly from
    /// `text`'s own value: comparing `text` against the editor's buffer
    /// cannot by itself tell "no file" apart from "file whose content
    /// happens to be empty and unedited". `deletedName` (set by the caller
    /// just before this runs) is what `DeckSessionController.
    /// isContentEdited` checks first, so a deleted document always reads
    /// as edited, whatever `text` holds. `textRevision` is still bumped
    /// here, the same as any assignment to `text`, so a save snapshot
    /// already captured for a write still in flight when the deletion is
    /// noticed is invalidated.
    private func markFileGone() {
        textRevision += 1
    }

    override func checkAutosavingSafety() throws {
        if let known = fileModificationDate, let onDisk = diskModificationDate, onDisk.timeIntervalSince(known) > 0.001 {
            sessionController?.diskChanged()
            throw CocoaError(.userCancelled)
        }
        try super.checkAutosavingSafety()
    }

    // `checkAutosavingSafety()` only runs for the periodic, cancellable
    // autosave (`autosavingIsImplicitlyCancellable == true`); a window close
    // or quit's own final autosave, `canClose` below now refuses before it
    // ever reaches here. This guard stays as this method's own defense for
    // any other caller, direct or future, that drives it with unsaved
    // changes during a shown conflict: `.userCancelled` is the one error
    // NSDocument treats as a silent refusal, so a close or quit driven this
    // way still cancels instead of writing, with no alert shown.
    override func autosave(withImplicitCancellability autosavingIsImplicitlyCancellable: Bool,
                           completionHandler: @escaping (Error?) -> Void) {
        if sessionController?.hasDiskConflict == true { return completionHandler(CocoaError(.userCancelled)) }
        super.autosave(withImplicitCancellability: autosavingIsImplicitlyCancellable, completionHandler: completionHandler)
    }

    // Reads the edited state straight from content on every call, rather
    // than from whatever AppKit's own bookkeeping last landed on, so it can
    // never disagree with the buffer no matter what a refused close does to
    // that bookkeeping behind the scenes.
    override var isDocumentEdited: Bool {
        sessionController?.isContentEdited ?? super.isDocumentEdited
    }

    // Driving the real `canClose(withDelegate:shouldClose:contextInfo:)`
    // during a shown conflict showed NSDocument's default implementation
    // doing more than refuse to close: after the guard above reports the
    // final autosave cancelled, it also silently calls `undo()` on the
    // document's own undo manager, discarding whatever edit was still
    // pending as an uncommitted "Typing" group, even though nothing was
    // ever written to disk. That is real data loss, not just a bookkeeping
    // flag reading wrong, and it happens inside NSDocument's own
    // implementation of this method, so nothing this diff can do to
    // `autosave` or to `isDocumentEdited` reaches it. Refusing here instead,
    // before calling into any of that machinery, avoids it entirely: the
    // delegate's own callback is invoked directly, with the fixed
    // `document:shouldClose:contextInfo:` signature every caller of this
    // method is documented to use, so the file and the buffer are both left
    // exactly as they were.
    override func canClose(withDelegate delegate: Any, shouldClose shouldCloseSelector: Selector?, contextInfo: UnsafeMutableRawPointer?) {
        guard sessionController?.hasDiskConflict == true else {
            super.canClose(withDelegate: delegate, shouldClose: shouldCloseSelector, contextInfo: contextInfo)
            return
        }
        guard let shouldCloseSelector,
              let target = delegate as AnyObject?,
              let implementation = target.method(for: shouldCloseSelector) else { return }
        typealias ShouldCloseFunction = @convention(c) (AnyObject, Selector, NSDocument, Bool, UnsafeMutableRawPointer?) -> Void
        let shouldClose = unsafeBitCast(implementation, to: ShouldCloseFunction.self)
        shouldClose(target, shouldCloseSelector, self, false, contextInfo)
    }

    // A save that lands somewhere other than this document's own file, such
    // as an elsewhere autosave or a Save To, does not change what tap should
    // be showing for this deck, so tap is told only when the URL just
    // written matches fileURL. A Save As does move the document, and by the
    // time this completion handler runs, fileURL already reflects that, so
    // it counts.
    //
    // This is also the method a direct, programmatic `.saveOperation` call
    // goes through (as this app's own tests exercise, and as `overwriteDisk`
    // below does for Keep Mine), so while a conflict is showing,
    // `.saveOperation` is refused here too: `.userCancelled`, no write, no
    // alert. Keep Mine already clears `hasDiskConflict` before it calls this
    // method, so it is never blocked by this guard. Save To and Save As
    // write somewhere other than the conflicting file and are untouched.
    override func save(to url: URL, ofType typeName: String, for saveOperation: NSDocument.SaveOperationType,
                       completionHandler: @escaping (Error?) -> Void) {
        // A Deck tab field still being typed in goes into the file: an autosave,
        // in place or elsewhere (an untitled deck's), takes its text and leaves
        // the person typing; a save the person asked for, Save As or Play's
        // save ends the edit first.
        if saveOperation == .autosaveInPlaceOperation || saveOperation == .autosaveElsewhereOperation {
            sessionController?.deckForm.commitEditingKeepingFocus()
        } else {
            _ = sessionController?.deckForm.commitEditing()
        }
        if saveOperation == .saveOperation, sessionController?.hasDiskConflict == true {
            return completionHandler(CocoaError(.userCancelled))
        }
        // Save To, and any other save explicitly told to land somewhere
        // other than this document's own file, is the one kind of save
        // `data(ofType:)` below must not treat as taking this document's
        // save snapshot: nothing it writes changes what tap should be
        // showing for this deck's own file.
        savingOwnFile = saveOperation != .saveToOperation
        super.save(to: url, ofType: typeName, for: saveOperation) { [weak self] error in
            guard let self else { return completionHandler(error) }
            if self.savingOwnFile {
                if error == nil, let fileURL = self.fileURL, FilePaths.same(fileURL, url) {
                    self.adoptSavedSnapshotIfCurrent()
                    self.sessionController?.documentDidSave(saveOperation)
                }
                // The snapshot only stands for a save of this document's own
                // file while that save is in flight: whatever this save
                // was, it is no longer in flight once this handler runs, so
                // nothing should keep matching against it. Cleared after
                // the adoption above, which is the one thing still allowed
                // to read it. A save known not to land on this document's
                // own file, such as Save To, never sets the snapshot in the
                // first place (`data(ofType:)`'s own `savingOwnFile` guard),
                // so there is nothing to clear here for one of those.
                self.savedSnapshot = nil
            }
            // Resetting this to true leaves a save driven directly against
            // `data(ofType:)`, such as a test standing in for a real save,
            // treated as an own-file save by default.
            self.savingOwnFile = true
            completionHandler(error)
        }
    }

    /// Duplicate (Cmd-Shift-S) never reaches `save(to:ofType:for:
    /// completionHandler:)` above; it seeds the new document's data through
    /// `data(ofType:)` directly, without writing this document's own file at
    /// all. That call still captures whatever the editor holds into
    /// `savedSnapshot`, so it is cleared here once the duplicate is made, the
    /// same way a save that lands elsewhere clears it in its own completion.
    override func duplicate() throws -> NSDocument {
        defer { savedSnapshot = nil }
        return try super.duplicate()
    }

    /// Adopts a completed save's snapshot as the deck's known text, unless a
    /// newer change already replaced it while the save was writing to disk.
    /// Exposed at internal visibility, not private, so a test can drive it
    /// directly around a manufactured race rather than depend on real save
    /// timing.
    func adoptSavedSnapshotIfCurrent() {
        guard let snapshot = savedSnapshot, textRevision == savedSnapshotRevision else { return }
        text = snapshot
    }
}
