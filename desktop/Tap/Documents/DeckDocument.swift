import AppKit

/// A deck: a plain .md file, which stays the only source of truth.
final class DeckDocument: NSDocument {
    /// The text as last read from disk.
    private(set) var text = ""
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
        MainActor.assumeIsolated { Data((self.sessionController?.editor.string ?? self.text).utf8) }
    }

    override nonisolated func close() {
        MainActor.assumeIsolated { self.sessionController?.stop() }
        super.close()
    }
}
