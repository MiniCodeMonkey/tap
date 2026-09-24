import AppKit

/// Wires one document: its editor, its tap process, the PUTs that keep the
/// boxes current, and the WebSocket connection that moves the preview.
@MainActor
final class DeckSessionController: NSObject, EditorTextViewDelegate {
    private(set) weak var document: DeckDocument?
    let session: TapSession
    let editorViewController = EditorViewController()
    let inspectorViewController = InspectorViewController()
    let previewViewController = PreviewViewController()
    private(set) var navigator = PreviewNavigator()
    private(set) var client: TapClient?
    private(set) var socket: TapSocket?
    private(set) var sourceSync: SourceSync!
    /// True once `stop()` has run. A source-sync PUT started before then can
    /// still resolve after: its answer belongs to a state that no longer
    /// exists and must not reach the editor, the same as a stale render
    /// must not publish over a newer one.
    private var stopped = false
    /// NSTextView replays undo and redo directly against the text storage,
    /// never through didChangeText, so editorTextDidChange never fires for
    /// them (confirmed directly: it fires once for a typed edit and not at
    /// all for the undo that reverts it). The document's edited flag is kept
    /// current by observing the undo manager for undo and redo, and
    /// recomputing the flag from the editor's text on every one of those
    /// notifications and on every forward edit, rather than by counting
    /// changes: one undo can revert many keystrokes at once, so a count that
    /// moves by one per notification falls out of step with the text itself.
    private var undoObserver: NSObjectProtocol?
    private var redoObserver: NSObjectProtocol?
    /// True while the bar offering Load Disk Version and Keep Mine is
    /// showing, between `showDiskConflict` and whichever button resolves it.
    private(set) var hasDiskConflict = false
    /// The slide number the cursor was on when `loadDiskVersion` ran, kept
    /// until the next slide list arrives so the cursor can return to it.
    private var pendingCursorSlideNumber: Int?
    /// Runs after each slide list is applied to the editor.
    var onSlideListApplied: ((SlideList) -> Void)?
    var onHubMessage: ((HubMessage) -> Void)?
    /// Trades the ready line's presenter secret for the hub cookie. It is a
    /// closure so a test can hold the exchange open across a restart, which
    /// is the race the guard after it turns away.
    var exchangePresenterSecret: (TapClient) async throws -> Void = { try await $0.authorizePresenter() }

    var editor: EditorTextView { editorViewController.textView }

    init(document: DeckDocument) {
        self.document = document
        let deckURL = document.fileURL ?? FileManager.default.temporaryDirectory.appendingPathComponent("Untitled.md")
        session = TapSession(deckURL: deckURL, configuration: AppEnvironment.shared.sessionConfiguration())
        super.init()
        sourceSync = SourceSync(text: { [weak self] in self?.editor.string ?? "" },
                                beginSend: { [weak self] in self?.editor.beginSend() ?? 0 })
        sourceSync.onAnswer = { [weak self] list, sentText, generation in
            self?.applySlideList(list, sentText: sentText, generation: generation)
        }
        sourceSync.onFailure = { [weak self] error in
            self?.session.log.append("PUT /api/app/source failed: \(error)", source: .app)
        }
        session.onStateChange = { [weak self] state in self?.sessionStateChanged(state) }
        editor.editorDelegate = self
        inspectorViewController.embed(previewViewController)
        previewViewController.onStepBackward = { [weak self] in self?.sendPreviewMessage(self?.navigator.stepBackward()) }
        previewViewController.onStepForward = { [weak self] in self?.sendPreviewMessage(self?.navigator.stepForward()) }
        previewViewController.onPinToggled = { [weak self] in self?.togglePin() }
        previewViewController.onTryAgain = { [weak self] in self?.session.tryAgain() }
        if let documentUndoManager = document.undoManager {
            undoObserver = NotificationCenter.default.addObserver(forName: .NSUndoManagerDidUndoChange, object: documentUndoManager, queue: nil) { [weak self] _ in
                MainActor.assumeIsolated { self?.refreshEditedState() }
            }
            redoObserver = NotificationCenter.default.addObserver(forName: .NSUndoManagerDidRedoChange, object: documentUndoManager, queue: nil) { [weak self] _ in
                MainActor.assumeIsolated { self?.refreshEditedState() }
            }
        }
        session.onEvent = { [weak self] event in self?.handle(event) }
    }

    /// The document is edited exactly when the editor's text differs from
    /// the deck file's content, as last read from disk (open, revert) or as
    /// last written by a save that wrote the deck's own file
    /// (`DeckDocument.text`). Called on every forward edit and on every undo
    /// and redo, in place of counting changes, so the flag always agrees
    /// with the text regardless of how many keystrokes one undo reverts.
    /// Lengths are compared first so most keystrokes cost almost nothing.
    private func refreshEditedState() {
        guard let document else { return }
        let editorText = editor.string
        let fileText = document.text
        let isEdited = editorText.utf16.count != fileText.utf16.count || editorText != fileText
        document.updateChangeCount(isEdited ? .changeDone : .changeCleared)
    }

    private func handle(_ event: TapEvent) {
        guard case .fileChanged(let path, let list) = event, let fileURL = document?.fileURL else { return }
        if FilePaths.same(URL(fileURLWithPath: path), fileURL) {
            diskChanged()
        } else if let list, let sentText = sourceSync.lastSentText {
            // A component changed, and with it a slide's step count.
            applySlideList(list, sentText: sentText, generation: sourceSync.lastSentGeneration)
        }
    }

    /// The deck file changed on disk.
    func diskChanged() {
        guard let document, let url = document.fileURL,
              let disk = try? String(contentsOf: url, encoding: .utf8) else { return }
        guard disk != editor.string else {
            document.adopt(diskText: disk)
            document.acceptDiskState()
            refreshEditedState()
            return
        }
        if document.isDocumentEdited {
            showDiskConflict(name: url.lastPathComponent)
        } else {
            loadDiskVersion()
        }
    }

    private func showDiskConflict(name: String) {
        hasDiskConflict = true
        editorViewController.showBar(DocumentBarView(
            kind: .changedOnDisk, message: "\(name) changed on disk.", detail: "You have unsaved edits.",
            buttons: [("Load Disk Version", { [weak self] in self?.loadDiskVersion() }),
                      ("Keep Mine", { [weak self] in self?.keepMine() })]))
    }

    /// Replaces the buffer with the file as one undoable edit, keeping the
    /// cursor on its slide. `document.text` is set to the disk text before
    /// the editor's text is replaced, so the document reads as not edited
    /// the moment the replacement lands, rather than staying edited until
    /// the next autosave.
    func loadDiskVersion() {
        hasDiskConflict = false
        editorViewController.hideBar(.changedOnDisk)
        guard let document, let url = document.fileURL,
              let disk = try? String(contentsOf: url, encoding: .utf8) else { return }
        pendingCursorSlideNumber = editor.currentBoxIndex.map { editor.boxes[$0].slide.number }
        document.adopt(diskText: disk)
        if let replacement = TextDiff.replacement(from: editor.string, to: disk) {
            editor.replaceText(in: replacement.range, with: replacement.replacement, actionName: "Load Disk Version")
        }
        refreshEditedState()
        document.acceptDiskState()
        session.log.append("loaded the disk version", source: .app)
    }

    /// Writes the buffer over the changed file.
    func keepMine() {
        hasDiskConflict = false
        editorViewController.hideBar(.changedOnDisk)
        document?.overwriteDisk { [weak self] error in
            if let error { self?.session.log.append("Keep Mine could not save: \(error.localizedDescription)", source: .app) }
        }
    }

    /// Pins the slide the preview shows, or unpins it and follows the cursor again.
    func togglePin() {
        if navigator.isPinned {
            let cursorSlide = editor.currentBoxIndex.map { editor.boxes[$0].slide }
            sendPreviewMessage(navigator.unpin(cursorSlide: cursorSlide))
        } else {
            navigator.pin()
            sendPreviewMessage(nil)
        }
    }

    func start() {
        editor.load(text: document?.text ?? "")
        session.start()
    }

    func stop() {
        stopped = true
        if let undoObserver { NotificationCenter.default.removeObserver(undoObserver) }
        if let redoObserver { NotificationCenter.default.removeObserver(redoObserver) }
        socket?.close()
        socket = nil
        sourceSync.sender = nil
        session.stop()
    }

    /// NSDocument read the file again, for Revert To.
    func documentDidRead(_ text: String) {
        guard text != editor.string else { return }
        editor.load(text: text)
        sourceSync.textDidChange()
        refreshEditedState()
    }

    /// The buffer is on disk. tap drops the buffer it renders and reads the
    /// file. `document.text` already holds the exact text the save wrote, so
    /// the edited flag is recomputed against it right away rather than
    /// waiting for the next keystroke.
    func documentDidSave() {
        session.send(.saved)
        session.log.append("saved the deck", source: .app)
        refreshEditedState()
    }

    private func applySlideList(_ list: SlideList, sentText: String, generation: Int) {
        guard !stopped else { return }
        // An answer the editor refuses was computed from text the editor no
        // longer holds. Nothing it says about this deck is true any more,
        // so none of what follows runs on it.
        guard editor.apply(list, sentText: sentText, sentGeneration: generation) else { return }
        if let first = list.errors.first {
            if editorViewController.bar(.deckErrors)?.message != "The deck settings have a problem: \(first)" {
                editorViewController.showBar(DocumentBarView(kind: .deckErrors, message: "The deck settings have a problem: \(first)",
                                                             detail: "The frontmatter is shown until it is fixed.", buttons: []))
            }
        } else {
            editorViewController.hideBar(.deckErrors)
        }
        if let number = pendingCursorSlideNumber {
            pendingCursorSlideNumber = nil
            if let index = editor.boxes.firstIndex(where: { $0.slide.number == number }), editor.currentBoxIndex != index {
                editor.moveCursor(toSlide: index)
            }
        }
        // New counts for the shown slide, and a new number when slides moved around the cursor.
        sendPreviewMessage(navigator.slidesChanged(list.slides))
        if let index = editor.currentBoxIndex {
            sendPreviewMessage(navigator.cursorMoved(to: editor.boxes[index].slide))
        }
        onSlideListApplied?(list)
    }

    /// Moves the preview through the hub, and updates its labels.
    func sendPreviewMessage(_ message: SlideMessage?) {
        if let message { socket?.send(message) }
        previewViewController.show(navigator)
    }

    private func sessionStateChanged(_ state: TapSession.State) {
        previewViewController.showSessionState(state, restartPolicy: session.restartPolicy)
        socket?.close()
        socket = nil
        guard case .running(let ready) = state else {
            client = nil
            sourceSync.sender = nil
            return
        }
        let newClient = TapClient(ready: ready)
        client = newClient
        sourceSync.sender = { source in try await newClient.putSource(source) }
        previewViewController.load(client: newClient)
        // The presenter secret comes first: a socket opened without the
        // cookie it buys is relayed to nobody, so the preview would never
        // move. A refusal is logged and the socket is opened anyway, because
        // hearing the hub is still worth having.
        let exchange = exchangePresenterSecret
        Task { @MainActor [weak self] in
            do {
                try await exchange(newClient)
            } catch {
                self?.session.log.append("tap refused the presenter secret: \(error)", source: .app)
            }
            guard let self, !self.stopped, self.client === newClient else { return }
            self.openSocket(with: newClient)
        }
        Task { await sourceSync.sendNow() }
    }

    /// Opens the app's own hub connection and sends it the current intent.
    private func openSocket(with newClient: TapClient) {
        socket?.close()
        let newSocket = newClient.openSocket()
        newSocket.onMessage = { [weak self] message in self?.onHubMessage?(message) }
        newSocket.resume()
        socket = newSocket
        if let message = navigator.message { newSocket.send(message) }
    }

    // MARK: EditorTextViewDelegate

    func editorTextDidChange(_ editor: EditorTextView) {
        refreshEditedState()
        sourceSync.textDidChange()
    }

    func editor(_ editor: EditorTextView, currentSlideDidChange index: Int?) {
        guard let index, editor.boxes.indices.contains(index) else { return }
        sendPreviewMessage(navigator.cursorMoved(to: editor.boxes[index].slide))
    }
}
