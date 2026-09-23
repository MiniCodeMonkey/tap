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
    }

    func start() {
        editor.load(text: document?.text ?? "")
        session.start()
    }

    func stop() {
        stopped = true
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
        sourceSync.textDidChange()
    }

    func editor(_ editor: EditorTextView, currentSlideDidChange index: Int?) {
        guard let index, editor.boxes.indices.contains(index) else { return }
        sendPreviewMessage(navigator.cursorMoved(to: editor.boxes[index].slide))
    }
}
