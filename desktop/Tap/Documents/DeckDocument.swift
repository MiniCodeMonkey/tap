import AppKit

/// A deck: a plain .md file, which stays the only source of truth.
final class DeckDocument: NSDocument {
    /// The deck file's content, as last read from disk (open, revert) or as
    /// last written by a save that wrote the deck's own file. This is what
    /// the session controller compares the editor's text against to decide
    /// whether the document is edited.
    private(set) var text = ""
    /// The exact text handed to `data(ofType:)` for the save now in flight,
    /// taken at the moment the data was produced. A person may keep typing
    /// while the save writes to disk, so this, not the editor's text when
    /// the save finishes, is what `text` becomes on success: it is what
    /// actually reached the file.
    private var savedSnapshot: String?
    private(set) var sessionController: DeckSessionController?

    override class var autosavesInPlace: Bool { true }

    override func makeWindowControllers() {
        let controller = DeckSessionController(document: self)
        sessionController = controller
        addWindowController(DeckWindowController(sessionController: controller))
        controller.start()
    }

    /// Opens the window as a tab of an open deck window.
    override func showWindows() {
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
            self.savedSnapshot = snapshot
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
    /// app loads them as an undoable edit, so NSDocument does not reload.
    override func presentedItemDidChange() {}

    override func checkAutosavingSafety() throws {
        if let known = fileModificationDate, let onDisk = diskModificationDate, onDisk.timeIntervalSince(known) > 0.001 {
            sessionController?.diskChanged()
            throw CocoaError(.userCancelled)
        }
        try super.checkAutosavingSafety()
    }

    // `checkAutosavingSafety()` only runs for the periodic, cancellable
    // autosave (`autosavingIsImplicitlyCancellable == true`); closing a
    // window or quitting with unsaved changes calls this method with
    // `false` directly, since `autosavesInPlace` is true, and never goes
    // through `checkAutosavingSafety()` at all. This guard is that path's
    // only defense against writing over a conflict still on screen, so it
    // must report the cancellation as a failure to save, not as a
    // no-op success: `.userCancelled` is the one error NSDocument treats
    // as a silent refusal, so closing or quitting cancels instead of
    // discarding the person's edits, with no alert shown.
    override func autosave(withImplicitCancellability autosavingIsImplicitlyCancellable: Bool,
                           completionHandler: @escaping (Error?) -> Void) {
        if sessionController?.hasDiskConflict == true { return completionHandler(CocoaError(.userCancelled)) }
        super.autosave(withImplicitCancellability: autosavingIsImplicitlyCancellable, completionHandler: completionHandler)
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
        if saveOperation == .saveOperation, sessionController?.hasDiskConflict == true {
            return completionHandler(CocoaError(.userCancelled))
        }
        super.save(to: url, ofType: typeName, for: saveOperation) { [weak self] error in
            if error == nil, let self, let fileURL = self.fileURL, FilePaths.same(fileURL, url) {
                if let snapshot = self.savedSnapshot {
                    self.text = snapshot
                }
                self.sessionController?.documentDidSave()
            }
            completionHandler(error)
        }
    }
}
