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

    override func autosave(withImplicitCancellability autosavingIsImplicitlyCancellable: Bool,
                           completionHandler: @escaping (Error?) -> Void) {
        if sessionController?.hasDiskConflict == true { return completionHandler(nil) }
        super.autosave(withImplicitCancellability: autosavingIsImplicitlyCancellable, completionHandler: completionHandler)
    }

    // A save that lands somewhere other than this document's own file, such
    // as an elsewhere autosave or a Save To, does not change what tap should
    // be showing for this deck, so tap is told only when the URL just
    // written matches fileURL. A Save As does move the document, and by the
    // time this completion handler runs, fileURL already reflects that, so
    // it counts.
    override func save(to url: URL, ofType typeName: String, for saveOperation: NSDocument.SaveOperationType,
                       completionHandler: @escaping (Error?) -> Void) {
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
