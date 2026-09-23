import XCTest
@testable import Tap

/// A test that runs inside Tap.app, driving the real `tap dev --app` the
/// bundle carries. tap gets its own settings folder, so tests never read
/// or write the user's approvals.
@MainActor
class HostedTestCase: XCTestCase {
    private(set) var configHome: URL!

    override func setUp() async throws {
        configHome = try Fixtures.temporaryFolder()
        AppEnvironment.shared.extraEnvironment["XDG_CONFIG_HOME"] = configHome.path
    }

    override func tearDown() async throws {
        for document in NSDocumentController.shared.documents {
            // The window goes off screen first. document.close() returns
            // before the window server has taken its window down, and a deck
            // window left standing sits over the next test's preview, whose
            // audience page then never paints and never reports a slide
            // ready. Ordering it out is immediate; closing is not.
            for windowController in document.windowControllers {
                windowController.window?.orderOut(nil)
            }
            document.close()
        }
        try await waitUntil(timeout: 10, "every deck window to go away") {
            NSDocumentController.shared.documents.isEmpty
                && !NSApp.windows.contains { $0.isVisible && $0.windowController is DeckWindowController }
        }
    }

    func openDeck(_ url: URL) async throws -> DeckDocument {
        let (document, _) = try await NSDocumentController.shared.openDocument(withContentsOf: url, display: true)
        let deck = try XCTUnwrap(document as? DeckDocument)
        // The audience page reports a slide ready only once it has painted,
        // and a window the window server treats as off screen never paints.
        // A test runner is not a person clicking on the app, so the window
        // is brought forward here rather than left to chance.
        NSApp.activate(ignoringOtherApps: true)
        deck.windowControllers.first?.window?.orderFrontRegardless()
        return deck
    }

    func waitForRunningTap(_ document: DeckDocument) async throws -> TapReady {
        let session = try XCTUnwrap(document.sessionController?.session)
        try await waitUntil(timeout: 30, "tap to be ready") { if case .running = session.state { return true } else { return false } }
        guard case .running(let ready) = session.state else { throw CancellationError() }
        return ready
    }

    func waitForBoxes(_ document: DeckDocument, count: Int) async throws {
        let editor = try XCTUnwrap(document.sessionController?.editor)
        try await waitUntil(timeout: 30, "\(count) boxes") { editor.boxes.count == count }
    }

    func openDeckAndWaitForPreview(_ url: URL) async throws -> DeckDocument {
        let document = try await openDeck(url)
        _ = try await waitForRunningTap(document)
        let preview = try XCTUnwrap(document.sessionController?.previewViewController)
        try await waitUntil(timeout: 30, "the preview's first ready signal") { preview.lastReady != nil }
        return document
    }

    @discardableResult
    func waitForPreview(_ document: DeckDocument, slide: Int, timeout: TimeInterval = 15) async throws -> ReadyPayload {
        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController
        let deadline = Date().addingTimeInterval(timeout)
        while preview.lastReady?.slide != slide {
            if Date() > deadline {
                // Which side stalled is the whole question when this times
                // out: whether the app ever asked for the slide, and whether
                // the page took the request and failed to settle. The page's
                // own signal is window.__tapReady, null while a slide is
                // settling, and the caret and the boxes say whether the
                // cursor is where the test put it.
                let script = "JSON.stringify({ready: window.__tapReady, hidden: document.hidden})"
                let inThePage = await preview.pageValue(script)
                let intent = String(describing: controller.navigator.message)
                XCTFail("timed out waiting for the preview on slide \(slide). "
                        + "lastReady=\(String(describing: preview.lastReady)) intent=\(intent) "
                        + "socket=\(controller.socket == nil ? "none" : "open") page=\(inThePage) "
                        + "caret=\(controller.editor.selectedRange()) "
                        + "box=\(String(describing: controller.editor.currentBoxIndex)) "
                        + "boxes=\(controller.editor.boxes.map(\.slide.number))")
                throw CancellationError()
            }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
        return try XCTUnwrap(preview.lastReady)
    }

    /// Polls `condition` until it is true.
    //
    // condition is called across await points inside the loop below, which
    // this toolchain only allows a closure parameter to do when it is
    // escaping; a non-escaping parameter fails to build here with "escaping
    // local function captures non-escaping value". Every call site already
    // passes a closure literal, so escaping changes nothing for callers.
    func waitUntil(timeout: TimeInterval = 10, _ message: String = "condition", _ condition: @escaping () -> Bool) async throws {
        let deadline = Date().addingTimeInterval(timeout)
        while !condition() {
            if Date() > deadline {
                XCTFail("timed out waiting for \(message)")
                throw CancellationError()
            }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
    }
}
