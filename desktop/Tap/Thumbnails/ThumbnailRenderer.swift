import AppKit
import WebKit

/// Renders slide thumbnails in one print-mode web view that nobody sees.
///
/// The web view sits inside the deck window behind the editor's opaque
/// scroll view (`EditorViewController.hostHiddenView`): WebKit suspends a
/// page in an off-screen window and never paints it, and print mode's
/// ready signal waits for a paint. The page is `/?print=true`, which never
/// joins the WebSocket hub, so walking through slides here never moves the
/// preview; it needs no cookie, because the page and the presentation are
/// audience routes. A slide is chosen with the URL fragment, and it counts
/// as rendered when the page posts `tapReady` for it with the revision the
/// work was set for. The app injects no script into the page.
@MainActor
final class ThumbnailRenderer: NSObject, WKScriptMessageHandler, WKNavigationDelegate {
    struct Job: Equatable {
        let slideNumber: Int
        let key: ThumbnailKey
    }

    /// What the render loop is doing right now, kept for diagnostics: a
    /// stalled renderer is told apart by where it stopped.
    enum Phase: Equatable {
        /// No loop is running.
        case idle
        /// The person is typing.
        case paused
        /// The window cannot paint.
        case waitingToPaint
        /// Every queued job is backing off.
        case backingOff
        /// A page for this slide was requested or is already showing it, and
        /// the loop waits for its ready signal.
        case waitingForReady(slide: Int, attempt: Int)
        case snapshotting(slide: Int)
    }

    static let viewSize = NSSize(width: 960, height: 540)
    /// How long a slide's first attempt may take to report ready before it is requeued.
    static let readyTimeout: TimeInterval = 5
    /// How long a snapshot may take before the attempt counts as failed.
    /// WebKit completes a snapshot only after the web view's next
    /// presentation update, which a web view that stops painting, or whose
    /// content process is gone, may never make.
    static let snapshotTimeout: TimeInterval = 10

    /// The renderer's web view. One whose content process stops answering
    /// is replaced by a new one, so this is read afresh rather than kept.
    private(set) var webView: WKWebView
    /// How long the page has to answer a no-op script after a ready wait
    /// timed out, before its content process counts as stuck.
    static let pageAnswerTimeout: TimeInterval = 2
    /// New web views per tap the renderer is pointed at.
    static let maximumWebViewReplacements = 2
    /// Web views replaced because their content process stopped answering.
    private(set) var webViewReplacementCount = 0
    /// Content processes that ended under the renderer.
    private(set) var processTerminationCount = 0
    private var replacementsForThisClient = 0
    /// True while the person is typing: the loop waits.
    var isPaused: () -> Bool = { false }
    /// True while the web view can paint: its window is on screen and it is not hidden.
    var canPaint: () -> Bool = { true }
    var onImage: ((Job, NSImage, Data) -> Void)?
    /// The page reported a revision other than the one the work was set for.
    var onPageRevision: ((String) -> Void)?
    /// Takes the snapshot. A seam: a test replaces it to hand the renderer a
    /// flat image, which no real page in a visible window produces.
    var snapshot: (WKWebView, WKSnapshotConfiguration) async throws -> NSImage = { webView, configuration in
        try await webView.takeSnapshot(configuration: configuration)
    }
    /// `snapshotTimeout`, as a seam a test shortens.
    var snapshotTimeoutInterval: TimeInterval = ThumbnailRenderer.snapshotTimeout
    /// The ready wait for a job's attempt, 0 on the first try: a seam a
    /// test shortens so it does not wait the real number of seconds the
    /// growing timeout implies. Production doubles `readyTimeout` on each
    /// retry up to a 30 s cap (5 s, 10 s, 20 s, then 30 s), so a slide that
    /// legitimately takes longer than the first window (a map can hold
    /// ready for up to 10 s) gets a longer wait instead of reloading and
    /// timing out at 5 s for as long as the deck is open.
    var readyTimeoutForAttempt: (Int) -> TimeInterval = { attempt in
        min(30, ThumbnailRenderer.readyTimeout * pow(2, Double(attempt)))
    }
    /// The ready payload each slide reported when it was captured.
    private(set) var readyBySlide: [Int: ReadyPayload] = [:]
    /// Images delivered.
    private(set) var renderCount = 0
    /// Loads this renderer asked the web view for.
    private(set) var navigationCount = 0
    private(set) var lastRenderedSlide: Int?
    /// Snapshots given up on because they did not complete in time.
    private(set) var snapshotTimeoutCount = 0
    private(set) var phase: Phase = .idle
    /// The slide whose render is under way, if any.
    private(set) var inFlightSlide: Int?
    /// After this many flat captures in a row, each following a paint-proven ready, a slide counts as blank.
    static let blankAcceptAttempts = 3

    private var baseURL: URL?
    private var allowedPort: Int?
    private var wantedRevision: String?
    private var loadedRevision: String?
    private var loadCount = 0
    private var jobs: [Int: Job] = [:]
    private var queue = ThumbnailQueue()
    private var running = false
    private var lastReady: ReadyPayload?
    private var waitingForSlide: Int?
    private var readyWaiter: CheckedContinuation<ReadyPayload?, Never>?
    /// The load the current ready waiter belongs to. A failure reported
    /// for any other load, such as an earlier one a newer load replaced,
    /// can arrive while this waiter waits; only a failure of this
    /// navigation ends the wait.
    private(set) var currentNavigation: WKNavigation?
    private var timeoutWork: DispatchWorkItem?
    /// Per slide: failed attempts in a row for the job currently counted,
    /// when it may be tried again, and flat captures in a row for the job
    /// currently counted. A job with a different `ThumbnailKey`, such as
    /// edited content, starts its own counts rather than inheriting ones
    /// left over from the old content: `failures` feeds `readyTimeoutForAttempt`,
    /// so a stale count would otherwise hand an edited slide's very first
    /// attempt the old content's already-grown wait.
    private var failures: [Int: (job: Job, count: Int)] = [:]
    private var notBefore: [Int: Date] = [:]
    private var flatCaptures: [Int: (job: Job, count: Int)] = [:]

    private enum Outcome { case rendered, retry }

    override init() {
        webView = Self.makeWebView()
        super.init()
        adopt(webView)
    }

    private static func makeWebView() -> WKWebView {
        let configuration = WKWebViewConfiguration()
        configuration.userContentController = WKUserContentController()
        configuration.websiteDataStore = .nonPersistent()
        return WKWebView(frame: NSRect(origin: .zero, size: Self.viewSize), configuration: configuration)
    }

    private func adopt(_ webView: WKWebView) {
        webView.configuration.userContentController.add(WeakScriptMessageHandler(self), name: "tapReady")
        // Print mode lays a slide out at 1920 by 1080 CSS pixels; half zoom fits it in the view.
        webView.pageZoom = 0.5
        webView.navigationDelegate = self
        webView.setAccessibilityElement(false)
        webView.setAccessibilityIdentifier("thumbnail-renderer")
    }

    /// Puts a new web view, with a new content process, where the old one
    /// is. The next render loads its page there.
    private func replaceWebView() {
        let old = webView
        old.navigationDelegate = nil
        old.configuration.userContentController.removeScriptMessageHandler(forName: "tapReady")
        old.stopLoading()
        let replacement = Self.makeWebView()
        adopt(replacement)
        webView = replacement
        old.replaceInSuperview(with: replacement)
        currentNavigation = nil
        loadedRevision = nil
        lastReady = nil
        webViewReplacementCount += 1
    }

    /// After a ready wait timed out: a page still loading or settling
    /// answers a no-op script at once, so no answer means its content
    /// process is stuck, and a reload would go to that same process. Such
    /// a web view is replaced, at most `maximumWebViewReplacements` times
    /// per tap.
    private func replaceWebViewIfStuck() async {
        guard replacementsForThisClient < Self.maximumWebViewReplacements else { return }
        let checked = webView
        guard case .noAnswer = await checked.evaluate("1", timeout: Self.pageAnswerTimeout), checked === webView else { return }
        replacementsForThisClient += 1
        replaceWebView()
    }

    var pendingCount: Int { queue.pending.count + (running ? 1 : 0) }

    func isQueued(_ number: Int) -> Bool { queue.pending.contains(number) }

    /// Failed attempts in a row for this exact job. A stored count for a
    /// different job at the same slide number, left over from content
    /// since replaced, does not count.
    private func failureCount(for job: Job) -> Int {
        guard let existing = failures[job.slideNumber], existing.job == job else { return 0 }
        return existing.count
    }

    /// Points the renderer at a running tap, or at nothing while tap is
    /// down: with no base URL the loop stops, and the work waits.
    func configure(client: TapClient?) {
        baseURL = client?.baseURL
        allowedPort = client?.ready.port
        loadedRevision = nil
        lastReady = nil
        replacementsForThisClient = 0
        failures = [:]
        notBefore = [:]
        flatCaptures = [:]
        pump()
    }

    /// Replaces the work: the slides without a cached image, and the
    /// revision their keys were computed from. A page loaded for another
    /// revision is reloaded before the next capture.
    func setWork(_ newJobs: [Job], revision: String, visible: [Int], current: Int?) {
        wantedRevision = revision
        jobs = Dictionary(newJobs.map { ($0.slideNumber, $0) }, uniquingKeysWith: { first, _ in first })
        queue.replace(with: Array(jobs.keys), visible: visible, current: current)
        pump()
    }

    private func pump() {
        guard !running, baseURL != nil else { return }
        running = true
        Task { @MainActor [weak self] in
            while true {
                guard let self, self.baseURL != nil else {
                    self?.running = false
                    self?.phase = .idle
                    return
                }
                if self.isPaused() || !self.canPaint() {
                    self.phase = self.isPaused() ? .paused : .waitingToPaint
                    try? await Task.sleep(nanoseconds: 150_000_000)
                    continue
                }
                guard let number = self.nextDueSlide() else {
                    if self.queue.isEmpty {
                        self.running = false
                        self.phase = .idle
                        return
                    }
                    // Every queued job is backing off.
                    self.phase = .backingOff
                    try? await Task.sleep(nanoseconds: 100_000_000)
                    continue
                }
                guard let job = self.jobs[number] else { continue }
                self.inFlightSlide = number
                let outcome = await self.render(job)
                self.inFlightSlide = nil
                switch outcome {
                case .rendered:
                    self.failures[number] = nil
                    self.notBefore[number] = nil
                    // The job just rendered is still the one setWork last set for this
                    // slide: leave it out of the renderer's own work so a hand-off that
                    // arrives during this capture, reporting the same job, does not
                    // requeue and render it again. A changed job for this slide, whose
                    // key no longer equals `job`, is left untouched and renders again.
                    if self.jobs[number] == job {
                        self.jobs[number] = nil
                        self.queue.remove(number)
                    }
                case .retry:
                    guard self.jobs[number] == job else { continue }
                    let count = self.failureCount(for: job) + 1
                    self.failures[number] = (job: job, count: count)
                    // 0.5 s, 1 s, 2 s, then 5 s: a job that keeps failing never spins the page.
                    self.notBefore[number] = Date().addingTimeInterval(min(5, 0.5 * pow(2, Double(count - 1))))
                    self.queue.requeue(number)
                }
            }
        }
    }

    /// The first queued slide whose back-off has passed, removed from the queue.
    private func nextDueSlide() -> Int? {
        let now = Date()
        guard let number = queue.pending.first(where: { (notBefore[$0] ?? .distantPast) <= now }) else { return nil }
        queue.remove(number)
        return number
    }

    private func printURL(slide: Int) -> URL? {
        guard let baseURL else { return nil }
        // A URL that differs only in its fragment is a same-document
        // navigation, so the query carries a count that makes each fresh
        // load a real one.
        return URL(string: "\(baseURL.absoluteString)?print=true&load=\(loadCount)#\(slide)")
    }

    /// Renders one slide. `.retry` when the slide did not report ready in
    /// time, reported another revision, or captured one flat colour fewer
    /// than `blankAcceptAttempts` times in a row; the loop requeues it with
    /// a back-off.
    private func render(_ job: Job) async -> Outcome {
        let number = job.slideNumber
        if loadedRevision == nil || loadedRevision != wantedRevision {
            loadCount += 1
            lastReady = nil
            guard let url = printURL(slide: number) else { return .retry }
            navigationCount += 1
            currentNavigation = webView.load(URLRequest(url: url))
        } else if lastReady?.slide != number {
            lastReady = nil
            guard let url = printURL(slide: number) else { return .retry }
            navigationCount += 1
            currentNavigation = webView.load(URLRequest(url: url))
        }
        let attempt = failureCount(for: job)
        phase = .waitingForReady(slide: number, attempt: attempt)
        guard let ready = await waitForReady(slide: number, timeout: readyTimeoutForAttempt(attempt)) else {
            loadedRevision = nil
            await replaceWebViewIfStuck()
            return .retry
        }
        loadedRevision = ready.revision
        guard ready.revision == wantedRevision else {
            onPageRevision?(ready.revision)
            return .retry
        }
        let configuration = WKSnapshotConfiguration()
        configuration.snapshotWidth = NSNumber(value: job.key.width)
        phase = .snapshotting(slide: number)
        guard let image = await boundedSnapshot(configuration) else { return .retry }
        if FlatImageCheck.isFlat(image) {
            // The page reported ready after a paint, for this slide and revision, and the
            // pixels are still one colour: either the paint came late, or the slide is blank.
            // Only several such captures in a row for this same job settle it as blank; a job
            // with a different key, such as edited content, starts its own count from zero.
            let flat: Int
            if let existing = flatCaptures[number], existing.job == job {
                flat = existing.count + 1
            } else {
                flat = 1
            }
            flatCaptures[number] = (job: job, count: flat)
            guard flat >= Self.blankAcceptAttempts else { return .retry }
        }
        flatCaptures[number] = nil
        guard let tiff = image.tiffRepresentation,
              let png = NSBitmapImageRep(data: tiff)?.representation(using: .png, properties: [:]),
              jobs[number] == job else { return .retry }
        renderCount += 1
        lastRenderedSlide = number
        readyBySlide[number] = ready
        onImage?(job, image, png)
        return .rendered
    }

    /// The snapshot, or nil when it failed or did not complete within
    /// `snapshotTimeoutInterval`. A snapshot left waiting would hold the loop
    /// inside this render, with `running` set, for as long as the deck stays
    /// open; giving up turns it into an ordinary failed attempt that backs
    /// off and retries on a freshly loaded page. A snapshot that completes
    /// after that is dropped.
    private func boundedSnapshot(_ configuration: WKSnapshotConfiguration) async -> NSImage? {
        let snapshot = self.snapshot
        let webView = self.webView
        let timeout = snapshotTimeoutInterval
        let image: NSImage?? = await withCheckedContinuation { continuation in
            let result = FirstResult(continuation)
            Task { @MainActor in
                result.deliver(.some(try? await snapshot(webView, configuration)))
            }
            DispatchQueue.main.asyncAfter(deadline: .now() + timeout) {
                MainActor.assumeIsolated { result.deliver(.none) }
            }
        }
        guard let image else {
            snapshotTimeoutCount += 1
            loadedRevision = nil
            return nil
        }
        return image
    }

    /// `render(_:)`, the only caller, never reaches this with a `lastReady`
    /// for another slide: it resets `lastReady` to nil before loading a
    /// fresh page for any slide or revision change, and falls through with
    /// `lastReady` intact only when `lastReady?.slide` already equals
    /// `slide`. So `lastReady` here, when not nil, always matches `slide`.
    private func waitForReady(slide: Int, timeout: TimeInterval) async -> ReadyPayload? {
        if let lastReady { return lastReady }
        waitingForSlide = slide
        return await withCheckedContinuation { continuation in
            readyWaiter = continuation
            let work = DispatchWorkItem { [weak self] in
                MainActor.assumeIsolated { self?.resumeWaiter(with: nil) }
            }
            timeoutWork = work
            DispatchQueue.main.asyncAfter(deadline: .now() + timeout, execute: work)
        }
    }

    private func resumeWaiter(with payload: ReadyPayload?) {
        timeoutWork?.cancel()
        timeoutWork = nil
        waitingForSlide = nil
        readyWaiter?.resume(returning: payload)
        readyWaiter = nil
    }

    // MARK: WebKit

    func userContentController(_ userContentController: WKUserContentController, didReceive message: WKScriptMessage) {
        guard message.name == "tapReady", let body = message.body as? [String: Any],
              let slide = (body["slide"] as? NSNumber)?.intValue else { return }
        let payload = ReadyPayload(revision: body["revision"] as? String ?? "", slide: slide,
                                   step: (body["step"] as? NSNumber)?.intValue ?? 0)
        lastReady = payload
        if waitingForSlide == slide { resumeWaiter(with: payload) }
    }

    func webView(_ webView: WKWebView, decidePolicyFor navigationAction: WKNavigationAction) async -> WKNavigationActionPolicy {
        guard let url = navigationAction.request.url else { return .cancel }
        if url.host == "127.0.0.1", url.port == allowedPort { return .allow }
        if url.scheme == "about" || url.scheme == "blob" || url.scheme == "data" { return .allow }
        return .cancel
    }

    func webView(_ webView: WKWebView, didFailProvisionalNavigation navigation: WKNavigation!, withError error: Error) {
        navigationFailed(navigation)
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        navigationFailed(navigation)
    }

    /// The page's content process exited or crashed. The attempt under way
    /// fails at once, and the next load starts a new process in the same
    /// web view.
    func webViewWebContentProcessDidTerminate(_ webView: WKWebView) {
        guard webView === self.webView else { return }
        processTerminationCount += 1
        loadedRevision = nil
        lastReady = nil
        resumeWaiter(with: nil)
    }

    /// Only a failure of the load the waiter belongs to ends its wait.
    private func navigationFailed(_ navigation: WKNavigation?) {
        guard let navigation, navigation === currentNavigation else { return }
        resumeWaiter(with: nil)
    }

    // MARK: Diagnostics

    /// The loop's state in one line, for a test that timed out waiting for a render.
    var stateDescription: String {
        let now = Date()
        let failureCounts = failures.keys.sorted().map { "\($0):\(failures[$0]!.count)" }
        let backOff = notBefore.keys.sorted().map { "\($0):\(String(format: "%.1f", notBefore[$0]!.timeIntervalSince(now)))s" }
        let flat = flatCaptures.keys.sorted().map { "\($0):\(flatCaptures[$0]!.count)" }
        return "running=\(running) phase=\(phase) inFlight=\(inFlightSlide.map(String.init) ?? "none") "
            + "queue=\(queue.pending) jobs=\(jobs.keys.sorted()) "
            + "navigationCount=\(navigationCount) renderCount=\(renderCount) "
            + "failures=\(failureCounts) backOff=\(backOff) flatCaptures=\(flat) snapshotTimeouts=\(snapshotTimeoutCount) "
            + "wantedRevision=\(wantedRevision ?? "none") loadedRevision=\(loadedRevision ?? "none") "
            + "lastReady=\(lastReady.map { "slide \($0.slide) revision \($0.revision)" } ?? "none") "
            + "waitingForSlide=\(waitingForSlide.map(String.init) ?? "none") "
            + "client=\(baseURL == nil ? "none" : "set") canPaint=\(canPaint()) isPaused=\(isPaused()) "
            + "webViewReplacements=\(webViewReplacementCount) processTerminations=\(processTerminationCount) "
            + "webViewLoading=\(webView.isLoading) url=\(webView.url?.absoluteString ?? "none")"
    }
}

/// Resumes a continuation with the first value delivered and drops the rest.
@MainActor
private final class FirstResult<Value> {
    private var continuation: CheckedContinuation<Value, Never>?

    init(_ continuation: CheckedContinuation<Value, Never>) {
        self.continuation = continuation
    }

    func deliver(_ value: Value) {
        continuation?.resume(returning: value)
        continuation = nil
    }
}
