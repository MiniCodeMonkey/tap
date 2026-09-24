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
        AppEnvironment.shared.recentThumbnailStore = RecentThumbnailStore(directory: try Fixtures.temporaryFolder())
    }

    override func tearDown() async throws {
        // WelcomeWindowController.shared is one singleton for the whole
        // hosted process, not a window this test created, so it is ordered
        // out (never closed) here rather than left to whichever test last
        // showed it: a welcome window still on screen would sit over the
        // next test's deck window the same way a stray deck window does.
        WelcomeWindowController.shared.window?.orderOut(nil)
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

    /// What a timed out wait records about the page. `ready` is the page's
    /// own signal (window.__tapReady), null while a slide settles and
    /// "unset" before any cycle has run. `state`
    /// is window.__tapReadyState, which the page's ready logic keeps itself:
    /// the step it is in (`phase`) and for how many milliseconds
    /// (`phaseMs`), the settle round, how many cycles it started and how
    /// many it published, the blockers held and for how long, and why the
    /// last round did not settle. It is null on a page built before the
    /// page kept it. `pageMs` is how long ago this document started
    /// loading, and `navigation` whether it was a reload. `slides` and
    /// `text` say whether the deck ever loaded and drew, and `animations`,
    /// `fonts` and `stylesheets` (link elements with no sheet yet) are what
    /// a settle round's probes look at.
    static let pageStateScript = """
        JSON.stringify({ready: window.__tapReady === undefined ? 'unset' : window.__tapReady, state: window.__tapReadyState ?? null, \
        pageMs: Math.round(performance.now()), \
        navigation: (performance.getEntriesByType('navigation')[0] || {}).type || null, \
        hidden: document.hidden, \
        slides: document.querySelectorAll('.slide').length, \
        text: document.body ? document.body.innerText.slice(0, 80) : null, \
        animations: document.getAnimations().map((animation) => animation.playState), \
        fonts: document.fonts.status, \
        stylesheets: Array.from(document.querySelectorAll('link[rel="stylesheet"]')).filter((link) => link.sheet === null).length})
        """

    /// What a timed out wait records about the app's side of the preview:
    /// how long ago it asked for the page, how many pages it has loaded,
    /// whether WebKit is still loading one, and how many ready messages its
    /// handler has received at all.
    func appSideDiagnostics(_ preview: PreviewViewController) -> String {
        let sinceLoad = preview.lastLoadDate.map { String(format: "%.1fs", Date().timeIntervalSince($0)) } ?? "never"
        return "sinceLoad=\(sinceLoad) pageLoads=\(preview.pageLoadCount) "
            + "webViewLoading=\(preview.webView.isLoading) "
            + "readyMessagesReceived=\(preview.readyMessagesReceived)"
    }

    func openDeckAndWaitForPreview(_ url: URL) async throws -> DeckDocument {
        let document = try await openDeck(url)
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController
        let deadline = Date().addingTimeInterval(30)
        while preview.lastReady == nil {
            if Date() > deadline {
                // A first ready signal that never arrives can be the page
                // itself never painting (the hidden-page stall fixed in
                // pull request 27), or tap never having been asked for
                // anything to paint. This records both sides so a future
                // occurrence does not need a fresh CI run to tell them
                // apart.
                let inThePage = await preview.pageValue(Self.pageStateScript)
                let window = document.windowControllers.first?.window
                let isVisible: Bool = window?.isVisible ?? false
                let isOnScreen: Bool = window?.occlusionState.contains(.visible) ?? false
                let isAppActive: Bool = NSApp.isActive
                let hasSocket: Bool = controller.socket != nil
                var message = "timed out waiting for the preview's first ready signal. "
                message += "page=\(inThePage) "
                message += "windowVisible=\(isVisible) "
                message += "windowOnScreen=\(isOnScreen) "
                message += "appActive=\(isAppActive) "
                message += "socket=\(hasSocket ? "open" : "none") "
                message += appSideDiagnostics(preview)
                XCTFail(message)
                throw CancellationError()
            }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
        // Every first ready is logged with the page's own record of how it
        // got there, so a CI log holds the normal timings to set a stalled
        // one against.
        let readyAfter = Date().timeIntervalSince(deadline.addingTimeInterval(-30))
        let settled = await preview.pageValue("JSON.stringify(window.__tapReadyState ?? null)")
        print("first ready after \(String(format: "%.2f", readyAfter))s: state=\(settled) \(appSideDiagnostics(preview))")
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
                let inThePage = await preview.pageValue(Self.pageStateScript)
                let intent = String(describing: controller.navigator.message)
                XCTFail("timed out waiting for the preview on slide \(slide). "
                        + "lastReady=\(String(describing: preview.lastReady)) intent=\(intent) "
                        + "socket=\(controller.socket == nil ? "none" : "open") page=\(inThePage) "
                        + "\(appSideDiagnostics(preview)) "
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
