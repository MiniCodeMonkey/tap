import AppKit

/// The theme catalog (`tap theme list --json`, once) and every theme's
/// render (`tap theme show <slug> --image --json --progress json`), one
/// at a time in the grid's order, kept for the app's life. tap caches the
/// PNGs by its version, so the next launch renders nothing. The first
/// render on a Mac downloads the export engine; its progress lines are
/// published for the grid to show.
///
/// All of it runs in one task at a time, which `stop` cancels: the tap run
/// in flight gets its SIGINT, and no later render starts. The task is
/// detached and made on a main-queue turn of its own, so it inherits
/// nothing from whoever asked (a caller's task-local values, its
/// priority) and is never created inside a caller's task.
@MainActor
final class ThemeImageLoader {
    static let didLoadCatalogNotification = Notification.Name("TapThemeCatalogDidLoad")
    static let didLoadImageNotification = Notification.Name("TapThemeImageDidLoad")
    static let downloadDidChangeNotification = Notification.Name("TapThemeDownloadDidChange")
    static let maximumAttempts = 3
    /// tap's error code for an export engine that could not be downloaded or started.
    static let engineFailureCode = "browser"

    private(set) var catalog: ThemeCatalog?
    private var images: [String: NSImage] = [:]
    /// Bytes of totalBytes while the export engine downloads, nil otherwise.
    private(set) var downloadProgress: (bytes: Int64, totalBytes: Int64)?

    /// tap's default theme, from the deck schema's `theme` default, "base"
    /// until the schema has loaded: the render the Default cell shows.
    var defaultSlug: String {
        (AppEnvironment.shared.deckSchema.keys.first { $0.name == "theme" }?.defaultValue).flatMap { $0.isEmpty ? nil : $0 } ?? "base"
    }

    /// True from a load's request until the catalog has loaded and no render is left to run.
    var isWorking: Bool { work != nil || startIsScheduled }

    private var work: Task<Void, Never>?
    private var workGeneration = 0
    private var startIsScheduled = false
    private var wantsRenders = false
    /// Slugs rendered ahead of everything else, and without `loadAll`: the
    /// welcome window's thumbnails.
    private var prioritySlugs: [String] = []
    private var catalogAttempts = 0
    /// Themes whose render failed; a grid that opens tries them again.
    private var failedSlugs: Set<String> = []
    private var currentRun: ToolRun?

    /// The render for a slug; the Default cell's is the default theme's.
    func image(for slug: String) -> NSImage? { images[slug == ThemeGridViewController.defaultSlug ? defaultSlug : slug] }

    /// Loads the catalog if it is not loaded: a deck window's Theme item
    /// and the Deck tab's row show the theme's name from it.
    func loadCatalog() {
        start()
    }

    /// Loads the catalog, then renders every theme that has no image yet.
    /// Safe to call from every grid that opens and every time the Deck
    /// tab's row shows. `retryingFailures` (a grid opening) tries the
    /// themes whose render failed before too.
    func loadAll(retryingFailures: Bool = false) {
        if retryingFailures { failedSlugs = [] }
        wantsRenders = true
        start()
    }

    /// Loads the catalog and renders just these themes, ahead of any
    /// render `loadAll` asks for.
    func loadImages(for slugs: [String]) {
        prioritySlugs = slugs
        start()
    }

    /// Cancels the work: the tap run in flight gets its SIGINT, and nothing
    /// more starts until the next load.
    func stop() {
        wantsRenders = false
        prioritySlugs = []
        startIsScheduled = false
        work?.cancel()
        work = nil
        currentRun?.cancel()
        currentRun = nil
        if downloadProgress != nil {
            downloadProgress = nil
            NotificationCenter.default.post(name: Self.downloadDidChangeNotification, object: self)
        }
    }

    private func start() {
        guard work == nil, !startIsScheduled, catalogIsWanted || nextSlugToRender != nil else { return }
        startIsScheduled = true
        DispatchQueue.main.async { [weak self] in
            MainActor.assumeIsolated { self?.startWork() }
        }
    }

    private func startWork() {
        guard startIsScheduled else { return }
        startIsScheduled = false
        guard work == nil, catalogIsWanted || nextSlugToRender != nil else { return }
        workGeneration += 1
        let generation = workGeneration
        work = Task.detached { @MainActor [weak self] in
            await self?.runWork()
            guard let self, self.workGeneration == generation else { return }
            self.work = nil
        }
    }

    private var catalogIsWanted: Bool { catalog == nil && catalogAttempts < Self.maximumAttempts }

    private var nextSlugToRender: String? {
        guard let catalog else { return nil }
        let known = Set(catalog.themes.map(\.slug))
        let candidates = prioritySlugs.filter(known.contains) + (wantsRenders ? (catalog.light + catalog.dark).map(\.slug) : [])
        return candidates.first { images[$0] == nil && !failedSlugs.contains($0) }
    }

    private func runWork() async {
        if catalogIsWanted {
            catalogAttempts += 1
            let exit = await runTap(["theme", "list", "--json"], timeout: 30)
            guard !Task.isCancelled else { return }
            if let loaded = try? ThemeCatalog.decode(exit.standardOutput) {
                catalog = loaded
                NotificationCenter.default.post(name: Self.didLoadCatalogNotification, object: self)
            }
        }
        while !Task.isCancelled, let slug = nextSlugToRender {
            await render(slug)
        }
    }

    private func render(_ slug: String) async {
        let exit = await runTap(["theme", "show", slug, "--image", "--json", "--progress", "json"], timeout: 600) { [weak self] progress in
            guard let self else { return }
            switch progress {
            case .download(let bytes, let totalBytes):
                self.downloadProgress = (bytes, totalBytes)
                NotificationCenter.default.post(name: Self.downloadDidChangeNotification, object: self)
            case .step, .finished:
                if self.downloadProgress != nil {
                    self.downloadProgress = nil
                    NotificationCenter.default.post(name: Self.downloadDidChangeNotification, object: self)
                }
            }
        }
        guard !Task.isCancelled else { return }
        guard let result = try? ThemeImageResult.decode(exit.standardOutput), let image = NSImage(contentsOfFile: result.image) else {
            failedSlugs.insert(slug)
            // Without the export engine no theme renders: the rest wait for the next grid.
            if case .failed(let code, _)? = exit.outcome, code == Self.engineFailureCode {
                wantsRenders = false
                prioritySlugs = []
            }
            return
        }
        images[slug] = image
        NotificationCenter.default.post(name: Self.didLoadImageNotification, object: self, userInfo: ["slug": slug])
    }

    /// One tap run that `stop` can cancel. A run whose task was cancelled
    /// while the environment was read never starts.
    private func runTap(_ arguments: [String], timeout: TimeInterval, onProgress: ((ProgressLine) -> Void)? = nil) async -> ToolRun.Exit {
        let run = await TapTool.makeRun(arguments, timeout: timeout)
        guard !Task.isCancelled else { return .cancelledBeforeStart }
        run.onProgress = onProgress
        currentRun = run
        let exit = await run.run()
        if currentRun === run { currentRun = nil }
        return exit
    }
}
