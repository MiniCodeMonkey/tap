import XCTest
import WebKit
@testable import Tap

/// A test that runs inside Tap.app, driving the real `tap dev --app` the
/// bundle carries. tap gets its own settings folder, so tests never read
/// or write the user's approvals.
@MainActor
class HostedTestCase: XCTestCase {
    private(set) var configHome: URL!
    /// The talk pages' data stores made for earlier tests and not yet
    /// removed. WebKit refuses to remove a store a web view still uses, so
    /// a store that is still in use at one teardown is tried again at the
    /// next.
    private static var dataStoresToRemove: [UUID] = []

    override func setUp() async throws {
        configHome = try Fixtures.temporaryFolder()
        AppEnvironment.shared.extraEnvironment["XDG_CONFIG_HOME"] = configHome.path
        AppEnvironment.shared.recentThumbnailStore = RecentThumbnailStore(directory: try Fixtures.temporaryFolder())
        AppEnvironment.shared.panelState = SlidePanelState(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.\(UUID().uuidString)")))
        AppEnvironment.shared.thumbnailCache = ThumbnailCache(directory: try Fixtures.temporaryFolder())
        AppEnvironment.shared.lastLayout = LastLayout(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.layout.\(UUID().uuidString)")))
        // Copy and paste go to a pasteboard of the test's own, never the person's clipboard.
        AppEnvironment.shared.slidePasteboard = NSPasteboard(name: NSPasteboard.Name("TapTests.copy.\(UUID().uuidString)"))
        let dataStoreIdentifier = UUID()
        Self.dataStoresToRemove.append(dataStoreIdentifier)
        AppEnvironment.shared.presentationDataStore = WKWebsiteDataStore(forIdentifier: dataStoreIdentifier)
        AppEnvironment.shared.displayAssignments = DisplayAssignmentStore(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.displays.\(UUID().uuidString)")))
        AppEnvironment.shared.deckPorts = DeckPortStore(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.ports.\(UUID().uuidString)")))
        AppEnvironment.shared.presentationSettings = PresentationSettingsStore(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.present.\(UUID().uuidString)")))
        AppEnvironment.shared.presentExecutableURL = nil
        // The Focus hint shows before the first talk on a Mac; every test but the hint's own has seen it.
        AppEnvironment.shared.focusHint = FocusHintState(defaults: try XCTUnwrap(UserDefaults(suiteName: "TapTests.focus.\(UUID().uuidString)")))
        AppEnvironment.shared.focusHint.markShown()
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
        await removeDataStores()
    }

    /// Removes the talk pages' data stores of this test and earlier ones,
    /// so the bundle does not leave one on disk, and a session in the
    /// network process, per test. Each removal has 5 s.
    private func removeDataStores() async {
        AppEnvironment.shared.presentationDataStore = .nonPersistent()
        var stillInUse: [UUID] = []
        for identifier in Self.dataStoresToRemove {
            let removed: String = await withCheckedContinuation { continuation in
                var answered = false
                WKWebsiteDataStore.remove(forIdentifier: identifier) { error in
                    MainActor.assumeIsolated {
                        guard !answered else { return }
                        answered = true
                        continuation.resume(returning: error.map { "\($0.localizedDescription)" } ?? "")
                    }
                }
                DispatchQueue.main.asyncAfter(deadline: .now() + 5) {
                    guard !answered else { return }
                    answered = true
                    continuation.resume(returning: "no answer in 5 s")
                }
            }
            if !removed.isEmpty { stillInUse.append(identifier) }
        }
        Self.dataStoresToRemove = stillInUse
        if !stillInUse.isEmpty { print("data stores not removed yet: \(stillInUse.count)") }
    }

    func openDeck(_ url: URL, timeout: TimeInterval = 30) async throws -> DeckDocument {
        // AppKit's completion is given `timeout` seconds, so an open that
        // never completes fails this test rather than hanging the bundle.
        let opened: Result<NSDocument, Error>? = await withCheckedContinuation { continuation in
            var answered = false
            NSDocumentController.shared.openDocument(withContentsOf: url, display: true) { document, _, error in
                guard !answered else { return }
                answered = true
                continuation.resume(returning: document.map { .success($0) } ?? .failure(error ?? CocoaError(.fileReadUnknown)))
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + timeout) {
                guard !answered else { return }
                answered = true
                continuation.resume(returning: nil)
            }
        }
        guard let opened else {
            XCTFail("timed out opening \(url.lastPathComponent) after \(Int(timeout)) s")
            throw CancellationError()
        }
        let deck = try XCTUnwrap(try opened.get() as? DeckDocument)
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
            + "readyMessagesReceived=\(preview.readyMessagesReceived) "
            + "recoveries=\(preview.pageRecoveryCount) "
            + "milestones=[\(preview.navigationMilestoneDescription)]"
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
                message += appSideDiagnostics(preview) + " "
                message += await webProcessDiagnostics(controller)
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
        // A recovery shows up here even in a green run. One is the preview
        // doing its job; more than one for a single open is a defect.
        XCTAssertLessThanOrEqual(preview.pageRecoveryCount, 1, "the preview replaced its web view more than once before its first ready")
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
                let webProcess = await webProcessDiagnostics(controller)
                XCTFail("timed out waiting for the preview on slide \(slide). "
                        + "lastReady=\(String(describing: preview.lastReady)) intent=\(intent) "
                        + "socket=\(controller.socket == nil ? "none" : "open") page=\(inThePage) "
                        + "\(appSideDiagnostics(preview)) "
                        + "\(webProcess) "
                        + "caret=\(controller.editor.selectedRange()) "
                        + "box=\(String(describing: controller.editor.currentBoxIndex)) "
                        + "boxes=\(controller.editor.boxes.map(\.slide.number))")
                throw CancellationError()
            }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
        return try XCTUnwrap(preview.lastReady)
    }

    /// A borderless, opaque window over another window, and a count of the
    /// window server's reports of that window as visible since the cover
    /// took effect.
    @MainActor
    final class Cover {
        let window: NSWindow
        private(set) var visibleReportsWhileCovered = 0
        private var observer: NSObjectProtocol?

        init(window: NSWindow) { self.window = window }

        func startCounting(_ covered: NSWindow) {
            observer = NotificationCenter.default.addObserver(forName: NSWindow.didChangeOcclusionStateNotification, object: covered, queue: nil) { [weak self] _ in
                MainActor.assumeIsolated {
                    guard let self, self.window.isVisible, covered.occlusionState.contains(.visible) else { return }
                    self.visibleReportsWhileCovered += 1
                }
            }
        }

        func remove() {
            if let observer { NotificationCenter.default.removeObserver(observer) }
            observer = nil
            window.orderOut(nil)
            window.close()
        }
    }

    /// Places a borderless, opaque window exactly over `window` once the
    /// window server has made its first report on `window`, and returns
    /// only after `window` has read occluded continuously for half a
    /// second while it stays on screen. A freshly opened window reads no
    /// visible bit before that first report, and the window server reports
    /// a covered window visible for about 0.1 s within its first 0.3 s on
    /// screen, so neither a single occluded reading nor an early one proves
    /// the cover has taken effect.
    func coverWindow(_ window: NSWindow) async throws -> Cover {
        try await waitUntil(timeout: 5, "the window server's first report of the deck window") { window.occlusionState.contains(.visible) }
        let coverWindow = NSWindow(contentRect: window.frame, styleMask: [.borderless], backing: .buffered, defer: false)
        coverWindow.isOpaque = true
        coverWindow.backgroundColor = .black
        coverWindow.level = .floating
        coverWindow.hasShadow = false
        coverWindow.animationBehavior = .none
        // Without this, closing the window also releases it (AppKit's
        // default for a window with no window controller), and the caller
        // still holding it then double-releases it when the test scope
        // ends, which crashes in objc_release during XCTest's post-test
        // deallocation check.
        coverWindow.isReleasedWhenClosed = false
        coverWindow.setFrame(window.frame, display: true)
        let cover = Cover(window: coverWindow)
        coverWindow.orderFrontRegardless()
        do {
            // Any report restarts the half second, so a visible report too
            // brief for a poll to read still counts against the cover.
            var reportCount = 0
            let recorder = NotificationCenter.default.addObserver(forName: NSWindow.didChangeOcclusionStateNotification, object: window, queue: nil) { _ in
                MainActor.assumeIsolated { reportCount += 1 }
            }
            defer { NotificationCenter.default.removeObserver(recorder) }
            var reportsSeen = 0
            var occludedSince: Date?
            let deadline = Date().addingTimeInterval(5)
            while true {
                let now = Date()
                if window.occlusionState.contains(.visible) || reportCount != reportsSeen {
                    reportsSeen = reportCount
                    occludedSince = nil
                }
                if occludedSince == nil, !window.occlusionState.contains(.visible) {
                    occludedSince = now
                }
                if let occludedSince, now.timeIntervalSince(occludedSince) >= 0.5 { break }
                guard now < deadline else {
                    XCTFail("the deck window never read occluded for half a second under the cover. occlusion=\(window.occlusionState.rawValue)")
                    throw CancellationError()
                }
                try await Task.sleep(nanoseconds: 20_000_000)
            }
        } catch {
            cover.remove()
            throw error
        }
        cover.startCounting(window)
        XCTAssertTrue(window.isVisible, "the deck window is still on screen, only covered")
        return cover
    }

    /// What a timed out render wait records: the renderer's loop state
    /// (`ThumbnailRenderer.stateDescription`, including the phase it is
    /// stuck in), its window's occlusion, and the renderer page's own
    /// ready state. The page's state is read with a 3 s limit, so a page
    /// that no longer answers cannot hang the failure itself.
    func rendererDiagnostics(_ renderer: ThumbnailRenderer) async -> String {
        let loop = renderer.stateDescription
        let window = renderer.webView.window
        let windowState = "window=\(window == nil ? "none" : "present") windowVisible=\(window?.isVisible ?? false) "
            + "occlusion=\(window?.occlusionState.rawValue ?? 0) hiddenView=\(renderer.webView.isHiddenOrHasHiddenAncestor)"
        let page: String = await withCheckedContinuation { continuation in
            var answered = false
            renderer.webView.evaluateJavaScript(Self.pageStateScript) { value, error in
                guard !answered else { return }
                answered = true
                continuation.resume(returning: error.map { "script failed: \($0)" } ?? String(describing: value))
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + 3) {
                guard !answered else { return }
                answered = true
                continuation.resume(returning: "no answer in 3 s")
            }
        }
        return "renderer: \(loop) \(windowState) page=\(page)"
    }

    /// What a deck's thumbnail controller last handed on, for a render wait
    /// that times out: whether tap's newest summary ever reached the
    /// renderer, as against the renderer stalling on work it has.
    func thumbnailControllerState(_ controller: DeckSessionController) -> String {
        let summary = controller.thumbnails.lastSummary
        return "controller: summaryRevision=\(summary?.revision ?? "none") summarySlides=\(summary?.slides.count ?? 0) "
            + "previewReady=\(controller.previewViewController.lastReady.map { "slide \($0.slide) revision \($0.revision)" } ?? "none") "
            + "current=\(controller.currentSlideNumber.map(String.init) ?? "none") visible=\(controller.slidePanel.visibleNumbers)"
    }

    /// `waitUntil` for a renderer's progress: a timeout fails with
    /// `rendererDiagnostics`, plus the deck's controller state when a
    /// controller is given, so a stall explains itself.
    func waitForRenderer(_ renderer: ThumbnailRenderer, of controller: DeckSessionController? = nil, timeout: TimeInterval,
                         _ message: String, _ condition: @escaping () -> Bool) async throws {
        let deadline = Date().addingTimeInterval(timeout)
        while !condition() {
            if Date() > deadline {
                let controllerState = controller.map { " " + thumbnailControllerState($0) } ?? ""
                XCTFail("timed out waiting for \(message). \(await rendererDiagnostics(renderer))\(controllerState)")
                throw CancellationError()
            }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
    }

    /// Polls `condition` until it is true. `message` is read when the wait
    /// times out, so a state it names is the state at the timeout.
    //
    // condition is called across await points inside the loop below, which
    // this toolchain only allows a closure parameter to do when it is
    // escaping; a non-escaping parameter fails to build here with "escaping
    // local function captures non-escaping value". Every call site already
    // passes a closure literal, so escaping changes nothing for callers.
    func waitUntil(timeout: TimeInterval = 10, _ message: @autoclosure @escaping () -> String = "condition", _ condition: @escaping () -> Bool) async throws {
        let deadline = Date().addingTimeInterval(timeout)
        while !condition() {
            if Date() > deadline {
                XCTFail("timed out waiting for \(message())")
                throw CancellationError()
            }
            try await Task.sleep(nanoseconds: 20_000_000)
        }
    }
}
