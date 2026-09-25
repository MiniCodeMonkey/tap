import AppKit

/// Keeps the panel's thumbnails current. After every render tap answers,
/// it reads the presentation summary, works out each slide's key, shows
/// the images it has in memory or on disk, and hands the rest to the
/// renderer with the current slide first.
@MainActor
final class ThumbnailController {
    let renderer = ThumbnailRenderer()
    let cache: ThumbnailCache
    weak var panel: SlidePanelViewController?
    var currentSlideNumber: () -> Int? = { nil }
    var client: TapClient? {
        didSet {
            renderer.configure(client: client)
            if client != nil { deckChanged() }
        }
    }
    private(set) var lastSummary: PresentationSummary?
    private var keys: [ThumbnailKey] = []
    private var images: [ThumbnailKey: NSImage] = [:]
    private var jobs: [ThumbnailRenderer.Job] = []
    private var fetching = false
    private var fetchAgain = false
    /// The pending job's key per slide, the revision, the visible range and
    /// the current slide, as last handed to the renderer. tap can answer
    /// two summaries back to back for one deck change, and a cursor move or
    /// a scroll can report the same visible range and current slide the
    /// renderer already has; handing the renderer the same work again would
    /// requeue a slide it is already in the middle of capturing, rendering
    /// it twice. Comparing against what was last sent, rather than sending
    /// on every refresh or every reprioritize, keeps the renderer's queue
    /// untouched when nothing actually changed.
    private var lastHandedToRenderer: [Int: ThumbnailKey] = [:]
    private var lastHandedRevision: String?
    private var lastHandedVisible: [Int] = []
    private var lastHandedCurrent: Int?
    /// Consecutive failed summary fetches since the last one that succeeded.
    private var summaryFetchFailures = 0
    private static let maxSummaryFetchRetries = 1

    init(cache: ThumbnailCache, panel: SlidePanelViewController) {
        self.cache = cache
        self.panel = panel
        renderer.onImage = { [weak self] job, image, png in self?.rendered(job, image: image, png: png) }
        renderer.onPageRevision = { [weak self] _ in self?.deckChanged() }
        panel.onVisibleRangeChanged = { [weak self] in self?.reprioritize() }
    }

    func key(forSlide number: Int) -> ThumbnailKey? {
        guard number >= 1, number <= keys.count else { return nil }
        return keys[number - 1]
    }

    /// tap rendered again: fetch the summary and refresh. One fetch runs
    /// at a time; a call during a fetch runs another when it returns. A
    /// fetch that fails retries once after a short back-off, so a deck
    /// opened and only read after a failed fetch still gets its thumbnails
    /// without waiting for the next edit.
    func deckChanged() {
        guard let client else { return }
        if fetching {
            fetchAgain = true
            return
        }
        fetching = true
        Task { @MainActor [weak self] in
            defer { self?.fetching = false }
            repeat {
                self?.fetchAgain = false
                if let summary = try? await client.presentation() {
                    self?.summaryFetchFailures = 0
                    self?.refresh(with: summary)
                } else if let self, self.summaryFetchFailures < Self.maxSummaryFetchRetries {
                    self.summaryFetchFailures += 1
                    try? await Task.sleep(nanoseconds: 500_000_000)
                    self.fetchAgain = true
                }
            } while self?.fetchAgain == true
        }
    }

    /// Reorders the renderer's work around what is visible now.
    func reprioritize() {
        guard let summary = lastSummary else { return }
        handToRenderer(jobs, revision: summary.revision, visible: panel?.visibleNumbers ?? [], current: currentSlideNumber())
    }

    /// Hands work to the renderer only when the job set, the revision, the
    /// visible range or the current slide actually changed since the last
    /// hand-off. A redundant call, such as a cursor move that lands back on
    /// the slide the renderer already favors, would otherwise requeue an
    /// in-flight job and render it twice.
    private func handToRenderer(_ jobs: [ThumbnailRenderer.Job], revision: String, visible: [Int], current: Int?) {
        let bySlide = Dictionary(uniqueKeysWithValues: jobs.map { ($0.slideNumber, $0.key) })
        guard bySlide != lastHandedToRenderer || revision != lastHandedRevision
                || visible != lastHandedVisible || current != lastHandedCurrent else { return }
        lastHandedToRenderer = bySlide
        lastHandedRevision = revision
        lastHandedVisible = visible
        lastHandedCurrent = current
        renderer.setWork(jobs, revision: revision, visible: visible, current: current)
    }

    private func refresh(with summary: PresentationSummary) {
        lastSummary = summary
        // The bundled tap's version is part of the signature: an app update
        // renders every slide again rather than showing yesterday's pixels.
        let signature = summary.themeSignature + "|tap=" + (AppEnvironment.shared.bundledTapVersion ?? "")
        keys = summary.slides.map { ThumbnailKey(slideHash: $0.hash, themeSignature: signature) }
        var pending: [ThumbnailRenderer.Job] = []
        var updating: Set<Int> = []
        for (index, key) in keys.enumerated() {
            let number = index + 1
            if let image = images[key] {
                panel?.setImage(image, forSlide: number)
                continue
            }
            if let data = cache.data(for: key), let image = NSImage(data: data) {
                images[key] = image
                panel?.setImage(image, forSlide: number)
                continue
            }
            guard !key.slideHash.isEmpty else { continue }
            updating.insert(number)
            if !pending.contains(where: { $0.key == key }) {
                pending.append(ThumbnailRenderer.Job(slideNumber: number, key: key))
            }
        }
        jobs = pending
        panel?.setUpdating(updating)
        handToRenderer(jobs, revision: summary.revision, visible: panel?.visibleNumbers ?? [], current: nil)
    }

    private func rendered(_ job: ThumbnailRenderer.Job, image: NSImage, png: Data) {
        images[job.key] = image
        try? cache.save(png, for: job.key)
        jobs.removeAll { $0.key == job.key }
        for (index, key) in keys.enumerated() where key == job.key {
            panel?.setImage(image, forSlide: index + 1)
        }
    }
}
