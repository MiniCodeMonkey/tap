import AppKit

/// Wires one document: its editor, its tap process, the PUTs that keep the
/// boxes current, and the WebSocket connection that moves the preview.
@MainActor
final class DeckSessionController: NSObject, EditorTextViewDelegate {
    private(set) weak var document: DeckDocument?
    let session: TapSession
    let editorViewController = EditorViewController()
    let inspectorViewController = InspectorViewController()
    private(set) var client: TapClient?
    private(set) var socket: TapSocket?
    private(set) var sourceSync: SourceSync!
    /// Runs after each slide list is applied to the editor.
    var onSlideListApplied: ((SlideList) -> Void)?
    var onHubMessage: ((HubMessage) -> Void)?

    var editor: EditorTextView { editorViewController.textView }

    init(document: DeckDocument) {
        self.document = document
        let deckURL = document.fileURL ?? FileManager.default.temporaryDirectory.appendingPathComponent("Untitled.md")
        session = TapSession(deckURL: deckURL, configuration: AppEnvironment.shared.sessionConfiguration())
        super.init()
        sourceSync = SourceSync(text: { [unowned self] in self.editor.string },
                                beginSend: { [unowned self] in self.editor.beginSend() })
        sourceSync.onAnswer = { [weak self] list, sentText, generation in
            self?.applySlideList(list, sentText: sentText, generation: generation)
        }
        sourceSync.onFailure = { [weak self] error in
            self?.session.log.append("PUT /api/app/source failed: \(error)", source: .app)
        }
        session.onStateChange = { [weak self] state in self?.sessionStateChanged(state) }
        editor.editorDelegate = self
    }

    func start() {
        editor.load(text: document?.text ?? "")
        session.start()
    }

    func stop() {
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
        editor.apply(list, sentText: sentText, sentGeneration: generation)
        onSlideListApplied?(list)
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
        let newSocket = newClient.openSocket()
        newSocket.onMessage = { [weak self] message in self?.onHubMessage?(message) }
        newSocket.resume()
        socket = newSocket
        Task { await sourceSync.sendNow() }
    }

    // MARK: EditorTextViewDelegate

    func editorTextDidChange(_ editor: EditorTextView) {
        sourceSync.textDidChange()
    }

    func editor(_ editor: EditorTextView, currentSlideDidChange index: Int?) {}
}
