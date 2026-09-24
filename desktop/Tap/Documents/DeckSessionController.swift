import AppKit
import WebKit

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
    private let fileWatcher = DeckFileWatcher()
    /// Shown by the preview in place of tap's own state while the deck's
    /// file is deleted: there is nothing for tap to run against.
    private var pausedMessage: String?
    /// True while the bar offering Load Disk Version and Keep Mine is
    /// showing, between `showDiskConflict` and whichever button resolves it.
    private(set) var hasDiskConflict = false
    /// True once a slide-1 snapshot has been saved as this deck's recent
    /// thumbnail. Set only on a successful capture (see `captureThumbnail`),
    /// so a capture skipped because the preview was hidden or unpainted
    /// leaves this false and a later ready for slide 1 tries again.
    private var recordedRecentThumbnail = false
    /// True while a snapshot is in flight, so two ready signals close
    /// together do not start two overlapping snapshots.
    private var isCapturingRecentThumbnail = false
    /// The slide number the cursor was on when `loadDiskVersion` ran, the
    /// generation that load's own text will be sent as (or the first later
    /// one, if tap is down or busy when it happens), and the text that was
    /// loaded. An answer only restores the cursor to this slide when it
    /// answers that generation or a later one whose text still equals what
    /// was loaded unchanged; any other answer, including one for a later
    /// generation whose text has since moved on (an edit landed while tap
    /// was down, for instance), discards it without moving the cursor. Tied
    /// to the generation and text, not just consumed by whichever
    /// `applySlideList` call happens to run next, so a crash and restart
    /// between the load and tap's answer for it cannot hijack a later,
    /// unrelated cursor move onto the load's slide.
    private struct PendingCursorLoad {
        let slideNumber: Int
        let generation: Int
        let text: String
    }
    private var pendingCursorLoad: PendingCursorLoad?
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
        previewViewController.onReady = { [weak self] payload in self?.previewDidRender(payload) }
        if let documentUndoManager = document.undoManager {
            undoObserver = NotificationCenter.default.addObserver(forName: .NSUndoManagerDidUndoChange, object: documentUndoManager, queue: nil) { [weak self] _ in
                MainActor.assumeIsolated { self?.refreshEditedState() }
            }
            redoObserver = NotificationCenter.default.addObserver(forName: .NSUndoManagerDidRedoChange, object: documentUndoManager, queue: nil) { [weak self] _ in
                MainActor.assumeIsolated { self?.refreshEditedState() }
            }
        }
        session.onEvent = { [weak self] event in self?.handle(event) }
        // A change fires on the deletion or rename itself, which for the
        // app's own safe save is the moment it renames a temporary file
        // over the deck's own path: the file exists again, under the same
        // descriptor's old path, within a moment. Waiting briefly before
        // checking, rather than reacting the instant the event fires,
        // lets that rename land first, so the app's own save is never
        // mistaken for someone deleting the file out from under it.
        fileWatcher.onChange = { [weak self] in
            DispatchQueue.main.asyncAfter(deadline: .now() + 0.2) {
                MainActor.assumeIsolated {
                    guard let self else { return }
                    self.checkFileStillExists()
                    self.fileWatcher.watch(self.document?.fileURL)
                }
            }
        }
        fileWatcher.watch(document.fileURL)
    }

    /// The document is edited exactly when the editor's text differs from
    /// the deck file's content, as last read from disk (open, revert) or as
    /// last written by a save that wrote the deck's own file
    /// (`DeckDocument.text`), or when the deck's file has been deleted:
    /// `deletedName` is checked first, ahead of comparing text, so a
    /// deleted document always reads as edited whatever its text happens to
    /// be, including a deck that was empty and unedited at the moment its
    /// file vanished. This is what `DeckDocument.isDocumentEdited` reads,
    /// computed fresh on every call rather than cached, so an
    /// AppKit-internal side effect that clears its own bookkeeping (as a
    /// refused close does) can never leave the document reading as clean
    /// while the buffer still disagrees with the file. Lengths are compared
    /// first so most keystrokes cost almost nothing.
    var isContentEdited: Bool {
        guard let document else { return false }
        if document.deletedName != nil { return true }
        let editorText = editor.string
        let fileText = document.text
        return editorText.utf16.count != fileText.utf16.count || editorText != fileText
    }

    /// Recomputes the edited state and tells NSDocument. Called on every
    /// forward edit and on every undo and redo, in place of counting
    /// changes, so `updateChangeCount` always agrees with the text
    /// regardless of how many keystrokes one undo reverts.
    /// `DeckDocument.isDocumentEdited` no longer depends on this having run
    /// recently; this call is for the rest of NSDocument's own bookkeeping
    /// (window state, the versions browser) that reads the change count
    /// directly rather than through the overridden getter.
    private func refreshEditedState() {
        document?.updateChangeCount(isContentEdited ? .changeDone : .changeCleared)
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

    /// Checks whether the deck's file is still there, and records a
    /// deletion when it is not. Called after the raw file watcher fires and
    /// after `presentedItemDidChange`, neither of which says on its own
    /// whether what happened was a deletion.
    func checkFileStillExists() {
        guard let url = document?.fileURL else { return }
        if !FileManager.default.fileExists(atPath: url.path) { document?.fileWasDeleted() }
    }

    /// The deck's file was deleted: tap is paused, the buffer stays as an
    /// unsaved document, and a bar offers Save As.
    func deckWasDeleted(name: String) {
        session.restartsWhenExited = false
        pausedMessage = "Save the deck to see the preview."
        fileWatcher.watch(nil)
        // The deletion is the more urgent fact: a disk conflict bar showing
        // for a file that no longer exists no longer describes anything
        // real, and both bars competing for the same space would be worse
        // than either alone.
        if hasDiskConflict {
            hasDiskConflict = false
            editorViewController.hideBar(.changedOnDisk)
        }
        editorViewController.showBar(DocumentBarView(
            kind: .deleted, message: "\(name) was deleted.", detail: "Your text is still here, unsaved.",
            buttons: [("Save As…", { [weak self] in self?.document?.saveAs(nil) })]))
        session.log.append("\(name) was deleted", source: .app)
        refreshEditedState()
    }

    /// The deck has a new path: renamed, moved, or saved after a deletion.
    func deckMoved(to url: URL) {
        session.restartsWhenExited = true
        pausedMessage = nil
        editorViewController.hideBar(.deleted)
        fileWatcher.watch(url)
        session.changeDeck(to: url)
        switch session.state {
        case .stopped, .failed: session.start()
        default: break
        }
    }

    /// The deck file changed on disk.
    ///
    /// A disk text matching the live editor text is a write that has already
    /// converged on what is open, whoever made it: the existing convergence
    /// check. A `file-changed` report can also be tap noticing the app's own
    /// autosave write while a person keeps typing past it: the app writes
    /// the file before it sends "saved", so the report can arrive while
    /// that save is still in flight and the editor has since moved on, which
    /// the convergence check alone cannot catch. For that, this also
    /// compares against `document.text` (the file's text as the app last
    /// read or wrote it) and against `document.savedSnapshot` (the text of a
    /// save of this file still in flight). A disk text matching any of the
    /// three is the app's own write, known or in progress: it is recorded as
    /// the file's text if it is not already, the edited state is refreshed,
    /// and nothing else happens, no load and no bar. Only a disk text
    /// matching none of the three is a real outside change.
    func diskChanged() {
        guard let document, let url = document.fileURL else { return }
        guard FileManager.default.fileExists(atPath: url.path) else {
            document.fileWasDeleted()
            return
        }
        guard let disk = try? String(contentsOf: url, encoding: .utf8) else { return }
        guard disk != editor.string, disk != document.text, disk != document.savedSnapshot else {
            if disk != document.text {
                document.adopt(diskText: disk)
            }
            document.acceptDiskState()
            refreshEditedState()
            // The disk has converged on text the app already knows about:
            // whatever conflict was showing no longer describes reality, and
            // must not survive to block a later autosave.
            if hasDiskConflict {
                hasDiskConflict = false
                editorViewController.hideBar(.changedOnDisk)
            }
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
        let slideNumber = editor.currentBoxIndex.map { editor.boxes[$0].slide.number }
        document.adopt(diskText: disk)
        if let replacement = TextDiff.replacement(from: editor.string, to: disk) {
            editor.replaceText(in: replacement.range, with: replacement.replacement, actionName: "Load Disk Version")
        }
        if let slideNumber {
            // The next send tap answers for this text, whichever generation
            // it lands on, is the one whose answer this load owns.
            pendingCursorLoad = PendingCursorLoad(slideNumber: slideNumber, generation: editor.tracker.generation + 1, text: disk)
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

    /// Moves the cursor to the given slide number, if the editor still has a box for it.
    func jumpToSlide(number: Int) {
        guard let index = editor.boxes.firstIndex(where: { $0.slide.number == number }) else { return }
        editor.moveCursor(toSlide: index)
        editor.window?.makeFirstResponder(editor)
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
        fileWatcher.watch(nil)
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

    /// The first time the preview finishes slide 1 while it is actually
    /// visible and painted, its snapshot becomes the deck's thumbnail on the
    /// welcome window. Ready fires on a settled DOM even for a hidden or
    /// covered page (WebKit runs no animation frames there, so nothing ever
    /// paints), and a snapshot of an unpainted page comes back blank, so
    /// this checks the same native facts WebKit itself uses to decide
    /// `document.hidden`, rather than asking the page (the app drives the
    /// preview only through the WebSocket `slide` message and reads it only
    /// through the `tapReady` handler; it never runs script in the page).
    /// `webView.isHiddenOrHasHiddenAncestor` is true the moment a collapsed
    /// preview pane (`MainSplitViewController.setPreviewHidden(true)`) hides
    /// its split item's view, so this one check covers both the docked and
    /// the detached case (`DeckWindowController.showPreviewInWindow`):
    /// `webView.window` is always whichever window currently holds it.
    /// `occlusionState.contains(.visible)` alone, without a separate
    /// `isVisible` check, is what tells a window on screen apart from one
    /// completely covered by another opaque window (both read `isVisible ==
    /// true`); a window that is not visible at all, such as one ordered out
    /// or miniaturized, is never observed to report the visible occlusion
    /// bit either, so an additional `isVisible` check adds no coverage a
    /// mutation could kill (confirmed directly: dropping it changed no
    /// test's outcome). Failing either of these does not mark this deck as
    /// recorded, so a later ready for slide 1, once the window or pane is
    /// actually shown, gets another chance.
    private func previewDidRender(_ payload: ReadyPayload) {
        guard payload.slide == 1, !recordedRecentThumbnail, !isCapturingRecentThumbnail,
              let deck = document?.fileURL else { return }
        let webView = previewViewController.webView
        guard !webView.isHiddenOrHasHiddenAncestor,
              let previewWindow = webView.window,
              previewWindow.occlusionState.contains(.visible) else { return }
        isCapturingRecentThumbnail = true
        captureThumbnail(for: deck)
    }

    /// Snapshots the preview's web view and, on success only, saves it as
    /// this deck's recent thumbnail and marks the deck recorded.
    private func captureThumbnail(for deck: URL) {
        let configuration = WKSnapshotConfiguration()
        configuration.snapshotWidth = 320
        previewViewController.webView.takeSnapshot(with: configuration) { [weak self] image, _ in
            guard let self else { return }
            self.isCapturingRecentThumbnail = false
            guard let image, let tiff = image.tiffRepresentation,
                  let png = NSBitmapImageRep(data: tiff)?.representation(using: .png, properties: [:]) else { return }
            self.recordedRecentThumbnail = true
            try? AppEnvironment.shared.recentThumbnailStore.save(png, for: deck)
        }
    }

    // internal, not private, so a test can hand it a deliberately stale or
    // refused answer to prove pendingCursorLoad does not outlive it.
    func applySlideList(_ list: SlideList, sentText: String, generation: Int) {
        guard !stopped else { return }
        // Resolved by the first answer that reaches or passes the load's own
        // generation, whether or not its text still matches: reaching that
        // point at all settles the question, so a slide number left over
        // from a load cannot wait indefinitely and then land on some later,
        // unrelated slide list instead. Only restored when the text this
        // answer was computed from still equals what was loaded, unchanged;
        // an edit that landed in between (tap down for a load, for
        // instance) means the answer says nothing true about that slide any
        // more, so the cursor is left where it actually is.
        var pendingNumber: Int?
        if let pendingLoad = pendingCursorLoad, generation >= pendingLoad.generation {
            pendingCursorLoad = nil
            if sentText == pendingLoad.text {
                pendingNumber = pendingLoad.slideNumber
            }
        }
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
        if let number = pendingNumber, let index = editor.boxes.firstIndex(where: { $0.slide.number == number }), editor.currentBoxIndex != index {
            editor.moveCursor(toSlide: index)
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
        previewViewController.showSessionState(state, restartPolicy: session.restartPolicy, pausedMessage: pausedMessage)
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
