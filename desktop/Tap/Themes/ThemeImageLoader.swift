import AppKit

/// The theme catalog (`tap theme list --json`, once) and every theme's
/// render (`tap theme show <slug> --image --json --progress json`), one
/// at a time in the grid's order, kept for the app's life. tap caches the
/// PNGs by its version, so the next launch renders nothing. The first
/// render on a Mac downloads the export engine; its progress lines are
/// published for the grid to show.
@MainActor
final class ThemeImageLoader {
    static let didLoadCatalogNotification = Notification.Name("TapThemeCatalogDidLoad")
    static let didLoadImageNotification = Notification.Name("TapThemeImageDidLoad")
    static let downloadDidChangeNotification = Notification.Name("TapThemeDownloadDidChange")
    static let maximumAttempts = 3

    private(set) var catalog: ThemeCatalog?
    private var images: [String: NSImage] = [:]
    /// Bytes of totalBytes while the export engine downloads, nil otherwise.
    private(set) var downloadProgress: (bytes: Int64, totalBytes: Int64)?

    /// tap's default theme, from the deck schema's `theme` default (D5
    /// loads it once), "base" until it has: the render the Default cell shows.
    var defaultSlug: String {
        (AppEnvironment.shared.deckSchema.keys.first { $0.name == "theme" }?.defaultValue).flatMap { $0.isEmpty ? nil : $0 } ?? "base"
    }
    private var catalogTask: Task<Void, Never>?
    private var renderTask: Task<Void, Never>?
    private var catalogAttempts = 0

    /// The render for a slug; the Default cell's is the default theme's.
    func image(for slug: String) -> NSImage? { images[slug == ThemeGridViewController.defaultSlug ? defaultSlug : slug] }

    /// Loads the catalog if it is not loaded, then renders every theme
    /// that has no image yet. Safe to call from every grid that opens.
    func loadAll() {
        Task { @MainActor [weak self] in
            await self?.loadCatalog()
            self?.renderMissing()
        }
    }

    func loadCatalog() async {
        if catalog != nil { return }
        if let catalogTask { return await catalogTask.value }
        guard catalogAttempts < Self.maximumAttempts else { return }
        catalogAttempts += 1
        let task = Task { @MainActor [weak self] in
            let exit = await TapTool.run(["theme", "list", "--json"], timeout: 30)
            guard let self, let loaded = try? ThemeCatalog.decode(exit.standardOutput) else { return }
            self.catalog = loaded
            NotificationCenter.default.post(name: Self.didLoadCatalogNotification, object: self)
        }
        catalogTask = task
        await task.value
        catalogTask = nil
    }

    private func renderMissing() {
        guard renderTask == nil, let catalog else { return }
        let missing = (catalog.light + catalog.dark).map(\.slug).filter { images[$0] == nil }
        guard !missing.isEmpty else { return }
        renderTask = Task { @MainActor [weak self] in
            for slug in missing {
                guard let self else { return }
                await self.render(slug)
            }
            // A theme whose render failed is tried again by the next loadAll.
            self?.renderTask = nil
        }
    }

    private func render(_ slug: String) async {
        let exit = await TapTool.run(["theme", "show", slug, "--image", "--json", "--progress", "json"], timeout: 600) { [weak self] progress in
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
        guard let result = try? ThemeImageResult.decode(exit.standardOutput), let image = NSImage(contentsOfFile: result.image) else { return }
        images[slug] = image
        NotificationCenter.default.post(name: Self.didLoadImageNotification, object: self, userInfo: ["slug": slug])
    }
}
