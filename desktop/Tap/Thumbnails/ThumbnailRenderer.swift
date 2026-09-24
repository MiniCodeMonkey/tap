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

    static let viewSize = NSSize(width: 960, height: 540)
    /// How long a slide's first attempt may take to report ready before it is requeued.
    static let readyTimeout: TimeInterval = 5

    let webView: WKWebView
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
    private var timeoutWork: DispatchWorkItem?
    /// Per slide: failed attempts in a row, when it may be tried again, and
    /// flat captures in a row for the job currently counted (a job with a
    /// different `ThumbnailKey`, such as edited content, starts its own
    /// count rather than inheriting one left over from the old content).
    private var failures: [Int: Int] = [:]
    private var notBefore: [Int: Date] = [:]
    private var flatCaptures: [Int: (job: Job, count: Int)] = [:]

    private enum Outcome { case rendered, retry }

    override init() {
        let configuration = WKWebViewConfiguration()
        configuration.userContentController = WKUserContentController()
        configuration.websiteDataStore = .nonPersistent()
        webView = WKWebView(frame: NSRect(origin: .zero, size: Self.viewSize), configuration: configuration)
        super.init()
        configuration.userContentController.add(WeakScriptMessageHandler(self), name: "tapReady")
        // Print mode lays a slide out at 1920 by 1080 CSS pixels; half zoom fits it in the view.
        webView.pageZoom = 0.5
        webView.navigationDelegate = self
        webView.setAccessibilityElement(false)
        webView.setAccessibilityIdentifier("thumbnail-renderer")
    }

    var pendingCount: Int { queue.pending.count + (running ? 1 : 0) }

    func isQueued(_ number: Int) -> Bool { queue.pending.contains(number) }

    /// Points the renderer at a running tap, or at nothing while tap is
    /// down: with no base URL the loop stops, and the work waits.
    func configure(client: TapClient?) {
        baseURL = client?.baseURL
        allowedPort = client?.ready.port
        loadedRevision = nil
        lastReady = nil
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
                    return
                }
                if self.isPaused() || !self.canPaint() {
                    try? await Task.sleep(nanoseconds: 150_000_000)
                    continue
                }
                guard let number = self.nextDueSlide() else {
                    if self.queue.isEmpty {
                        self.running = false
                        return
                    }
                    // Every queued job is backing off.
                    try? await Task.sleep(nanoseconds: 100_000_000)
                    continue
                }
                guard let job = self.jobs[number] else { continue }
                switch await self.render(job) {
                case .rendered:
                    self.failures[number] = nil
                    self.notBefore[number] = nil
                case .retry:
                    guard self.jobs[number] == job else { continue }
                    let count = (self.failures[number] ?? 0) + 1
                    self.failures[number] = count
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
            webView.load(URLRequest(url: url))
        } else if lastReady?.slide != number {
            lastReady = nil
            guard let url = printURL(slide: number) else { return .retry }
            navigationCount += 1
            webView.load(URLRequest(url: url))
        }
        let attempt = failures[number] ?? 0
        guard let ready = await waitForReady(slide: number, timeout: readyTimeoutForAttempt(attempt)) else {
            loadedRevision = nil
            return .retry
        }
        loadedRevision = ready.revision
        guard ready.revision == wantedRevision else {
            onPageRevision?(ready.revision)
            return .retry
        }
        let configuration = WKSnapshotConfiguration()
        configuration.snapshotWidth = NSNumber(value: job.key.width)
        guard let image = try? await snapshot(webView, configuration) else { return .retry }
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

    private func waitForReady(slide: Int, timeout: TimeInterval) async -> ReadyPayload? {
        if let lastReady, lastReady.slide == slide { return lastReady }
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
        resumeWaiter(with: nil)
    }

    func webView(_ webView: WKWebView, didFail navigation: WKNavigation!, withError error: Error) {
        resumeWaiter(with: nil)
    }
}
