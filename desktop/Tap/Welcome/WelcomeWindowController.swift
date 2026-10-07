import AppKit
import TapDesktopCore

/// Shown when no deck is open. With no recent decks it is a hero: the app
/// icon, wordmark and tagline over the tap.sh northern lights, New Deck and
/// Open, a link to the theme tour, and a filmstrip of every theme that opens
/// New Deck with the one you click. With recent decks the brand moves to a
/// left pane and an opaque list of decks, with a search field, fills the
/// right. A Markdown file dropped anywhere on the window opens.
final class WelcomeWindowController: NSWindowController, NSTableViewDataSource, NSTableViewDelegate, NSSearchFieldDelegate {
    static let shared = WelcomeWindowController()

    /// The beat the window stays visible before missing theme renders start, so a window that gives way to a deck at once costs no renders.
    static let themeRenderDelay: TimeInterval = 0.3

    let root = WelcomeRootView()
    var columnNewDeckButton: NSButton { root.columnBrand.newDeckButton }
    var columnOpenButton: NSButton { root.columnBrand.openButton }
    var emptyStateNewDeckButton: NSButton { root.emptyStateBrand.newDeckButton }
    var emptyStateOpenButton: NSButton { root.emptyStateBrand.openButton }
    var emptyStateView: NSView { root.hero }
    var dropZone: WelcomeDropZone { root.dropZone }
    var tableView: NSTableView { root.tableView }
    var aurora: AuroraView { root.aurora }
    var filmstrip: WelcomeFilmstrip { root.filmstrip }
    var searchField: NSSearchField { root.searchField }
    /// The theme cards of the filmstrip, one per catalog theme.
    var themeCards: [WelcomeThemeCard] { root.filmstrip.cards }
    /// Every recent deck that still exists, newest first.
    private(set) var recentURLs: [URL] = []
    /// The recent decks the search shows.
    private(set) var visibleRecentURLs: [URL] = []
    private(set) var searchQuery = ""
    private(set) var showsEmptyState = false
    private var refreshTimer: Timer?
    private var themeRenderTimer: Timer?
    private(set) var themeRendersRequested = false
    private var observers: [NSObjectProtocol] = []
    private var workspaceObserver: NSObjectProtocol?

    /// The New Deck button that is on screen: the hero's or the column's.
    var newDeckButton: NSButton { showsEmptyState ? emptyStateNewDeckButton : columnNewDeckButton }
    /// The Open button that is on screen.
    var openButton: NSButton { showsEmptyState ? emptyStateOpenButton : columnOpenButton }

    static func closeIfOpen() {
        shared.themeRenderTimer?.invalidate()
        shared.themeRenderTimer = nil
        shared.themeRendersRequested = false
        AppEnvironment.shared.themeImages.cancelPriorityImages()
        shared.window?.orderOut(nil)
    }

    private init() {
        let window = NSWindow(contentRect: NSRect(origin: .zero, size: WelcomeRootView.windowSize),
                              styleMask: [.titled, .closable, .fullSizeContentView], backing: .buffered, defer: false)
        window.titlebarAppearsTransparent = true
        window.titleVisibility = .hidden
        window.isReleasedWhenClosed = false
        window.isMovableByWindowBackground = true
        window.center()
        super.init(window: window)
        window.contentView = root

        tableView.dataSource = self
        tableView.delegate = self
        tableView.target = self
        tableView.doubleAction = #selector(openClickedRow(_:))
        root.tableView.onOpenSelection = { [weak self] in self?.openSelectedRow() }
        root.tableView.onTypeToSearch = { [weak self] characters in self?.typeToSearch(characters) }
        root.onTypeToSearch = { [weak self] characters in self?.typeToSearch(characters) }
        searchField.delegate = self
        searchField.target = self
        searchField.action = #selector(searchChanged(_:))
        root.filmstrip.onSelect = { [weak self] slug in
            (NSApp.delegate as? AppDelegate)?.newDeck(on: self?.window, theme: slug)
        }
        dropZone.onDrop = { [weak self] url in
            NSDocumentController.shared.openDocument(withContentsOf: url, display: true) { _, _, error in
                guard let error, let window = self?.window else { return }
                // A sheet on the welcome window: nothing app-modal.
                window.presentError(error, modalFor: window, delegate: nil, didPresent: nil, contextInfo: nil)
            }
        }
        root.recordingSetup.onSetUpLater = { [weak self] in
            AppEnvironment.shared.recordingSetupStore.dismissed = true
            self?.finishRecordingSetup()
        }
        root.recordingSetup.onContinue = { [weak self] in
            AppEnvironment.shared.recordingSetupStore.awaitingConfirmation = false
            self?.finishRecordingSetup()
        }
        dropZone.onTargetChange = { [weak self] targeted in self?.root.setDragActive(targeted) }

        let center = NotificationCenter.default
        let loader = AppEnvironment.shared.themeImages
        observers = [
            center.addObserver(forName: NSWindow.didBecomeKeyNotification, object: window, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated {
                    self?.reload()
                    self?.startWatchingRecents()
                }
            },
            center.addObserver(forName: NSWindow.didChangeOcclusionStateNotification, object: window, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated { self?.updateVisibility() }
            },
            center.addObserver(forName: NSApplication.didBecomeActiveNotification, object: nil, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated {
                    guard self?.window?.isVisible == true else { return }
                    self?.startWatchingRecents()
                }
            },
            center.addObserver(forName: ThemeImageLoader.didLoadCatalogNotification, object: loader, queue: .main) { [weak self] _ in
                MainActor.assumeIsolated {
                    self?.rebuildThemeStrip()
                    if self?.themeRendersRequested == true { self?.requestMissingThemeImages() }
                }
            },
            center.addObserver(forName: ThemeImageLoader.didLoadImageNotification, object: loader, queue: .main) { [weak self] notification in
                MainActor.assumeIsolated {
                    guard let slug = notification.userInfo?["slug"] as? String else { return }
                    self?.root.filmstrip.show(loader.image(for: slug), forSlug: slug)
                }
            },
        ]
        workspaceObserver = NSWorkspace.shared.notificationCenter.addObserver(forName: NSWorkspace.accessibilityDisplayOptionsDidChangeNotification, object: nil, queue: .main) { [weak self] _ in
            MainActor.assumeIsolated {
                self?.root.filmstrip.refreshMotionSetting()
                for brand in [self?.root.emptyStateBrand, self?.root.columnBrand] { brand?.icon.updateBlink() }
                self?.root.recordingSetup.icon.updateBlink()
                self?.root.recordingSetup.updateMotion()
            }
        }
        rebuildThemeStrip()
    }

    required init?(coder: NSCoder) { fatalError("init(coder:) is not used") }

    // NSWindowController's own showWindow(_:) is the one place this window
    // is brought forward and made key; nothing here calls
    // makeKeyAndOrderFront or NSApp.activate itself, so opening the welcome
    // window never steals focus beyond what that default implementation does.
    override func showWindow(_ sender: Any?) {
        let wasVisible = window?.isVisible == true
        reload()
        super.showWindow(sender)
        startWatchingRecents()
        updateVisibility()
        guard !wasVisible else { return }
        root.layoutSubtreeIfNeeded()
        root.aurora.restartOpening()
        root.playEntrance()
    }

    /// Starts and stops what only a visible window needs: the aurora reads the window itself, the strip and the caret are told here.
    func updateVisibility() {
        let visible = window.map { $0.isVisible && !$0.isMiniaturized && $0.occlusionState.contains(.visible) } ?? false
        root.filmstrip.setActive(showsEmptyState && visible)
        for brand in [root.emptyStateBrand, root.columnBrand] { brand.icon.setBlinking(visible) }
        root.recordingSetup.icon.setBlinking(visible && root.showsRecordingSetup)
    }

    /// Shows the window with the recording setup screen in place of its layouts.
    func showRecordingSetup() {
        root.recordingSetup.refresh()
        root.setShowsRecordingSetup(true)
        showWindow(nil)
    }

    /// Gives the window its layouts back, or closes it when a deck is open: Set Up Later and Continue to Tap both end here.
    private func finishRecordingSetup() {
        root.setShowsRecordingSetup(false)
        if NSDocumentController.shared.documents.contains(where: { $0 is DeckDocument }) {
            window?.orderOut(nil)
        } else {
            reload()
        }
        updateVisibility()
    }

    /// The recents list has no change notification, so a visible window
    /// looks again every second and switches layout when the list changes.
    private func startWatchingRecents() {
        guard refreshTimer == nil else { return }
        refreshTimer = Timer.scheduledTimer(withTimeInterval: 1, repeats: true) { [weak self] _ in
            MainActor.assumeIsolated {
                guard let self else { return }
                guard self.window?.isVisible == true else {
                    self.refreshTimer?.invalidate()
                    self.refreshTimer = nil
                    return
                }
                self.reload()
            }
        }
    }

    func reload() {
        if root.showsRecordingSetup { root.recordingSetup.refresh() }
        let urls = NSDocumentController.shared.recentDocumentURLs.filter { FileManager.default.fileExists(atPath: $0.path) }
        if urls != recentURLs {
            recentURLs = urls
            applyFilter()
        }
        showEmptyState(urls.isEmpty)
    }

    /// Swaps the two layouts.
    private func showEmptyState(_ shows: Bool) {
        let changed = shows != showsEmptyState
        showsEmptyState = shows
        root.setShowsEmptyState(shows)
        window?.initialFirstResponder = shows ? nil : tableView
        if changed { updateVisibility() }
        if shows {
            if themeCards.isEmpty { rebuildThemeStrip() }
            AppEnvironment.shared.themeImages.loadCatalog()
            requestThemeRendersOnceVisible()
        } else {
            themeRenderTimer?.invalidate()
            themeRenderTimer = nil
            themeRendersRequested = false
        }
    }

    // MARK: Themes

    /// Every catalog theme in the filmstrip; the catalog is in the app bundle, so this needs no tap run.
    private func rebuildThemeStrip() {
        guard let catalog = AppEnvironment.shared.themeImages.catalog else { return }
        let loader = AppEnvironment.shared.themeImages
        guard catalog.themes.map(\.slug) != themeCards.map(\.slug) else { return }
        root.filmstrip.setThemes(catalog.themes) { loader.image(for: $0) }
    }

    /// The renders of themes with no bundled image start once the window has
    /// stayed visible for a beat; a window shown only until a deck opens
    /// never starts them.
    private func requestThemeRendersOnceVisible() {
        guard !themeRendersRequested, themeRenderTimer == nil, window?.isVisible == true else { return }
        themeRenderTimer = Timer.scheduledTimer(withTimeInterval: Self.themeRenderDelay, repeats: false) { [weak self] _ in
            MainActor.assumeIsolated {
                guard let self else { return }
                self.themeRenderTimer = nil
                guard self.showsEmptyState, self.window?.isVisible == true else { return }
                self.themeRendersRequested = true
                self.requestMissingThemeImages()
            }
        }
    }

    private func requestMissingThemeImages() {
        guard let catalog = AppEnvironment.shared.themeImages.catalog else { return }
        AppEnvironment.shared.themeImages.loadImages(for: catalog.themes.map(\.slug))
    }

    // MARK: Recents

    func thumbnail(forRow row: Int) -> NSImage? {
        guard visibleRecentURLs.indices.contains(row) else { return nil }
        return AppEnvironment.shared.recentThumbnailStore.imageData(for: visibleRecentURLs[row]).flatMap(NSImage.init(data:))
    }

    private static func tildePath(of url: URL) -> String {
        (url.deletingLastPathComponent().path as NSString).abbreviatingWithTildeInPath
    }

    /// Shows the decks the search matches, keeping the selected deck selected when it still shows.
    private func applyFilter() {
        let selected = tableView.selectedRow >= 0 && visibleRecentURLs.indices.contains(tableView.selectedRow) ? visibleRecentURLs[tableView.selectedRow] : nil
        visibleRecentURLs = recentURLs.filter { RecentDeckMatcher.matches(name: $0.lastPathComponent, path: Self.tildePath(of: $0), query: searchQuery) }
        tableView.reloadData()
        root.noMatchesLabel.isHidden = !visibleRecentURLs.isEmpty || recentURLs.isEmpty
        let row = selected.flatMap { url in visibleRecentURLs.firstIndex(of: url) } ?? (visibleRecentURLs.isEmpty ? nil : 0)
        if let row { tableView.selectRowIndexes(IndexSet(integer: row), byExtendingSelection: false) }
    }

    /// Filters the recent decks by `query`, as typing in the search field does.
    func search(_ query: String) {
        if searchField.stringValue != query { searchField.stringValue = query }
        // The field's action and its text-change notice both come here for one keystroke: the second finds nothing new.
        guard query != searchQuery else { return }
        searchQuery = query
        applyFilter()
    }

    @objc private func searchChanged(_ sender: NSSearchField) {
        search(sender.stringValue)
    }

    func controlTextDidChange(_ notification: Notification) {
        guard (notification.object as? NSSearchField) === searchField else { return }
        search(searchField.stringValue)
    }

    func control(_ control: NSControl, textView: NSTextView, doCommandBy commandSelector: Selector) -> Bool {
        switch commandSelector {
        case #selector(NSResponder.moveDown(_:)):
            window?.makeFirstResponder(tableView)
            return true
        case #selector(NSResponder.insertNewline(_:)):
            openSelectedRow()
            return true
        default:
            return false
        }
    }

    /// Typing while the window is key and nothing else takes the text goes to the search field.
    private func typeToSearch(_ characters: String) {
        guard !showsEmptyState, window?.isKeyWindow == true else { return }
        window?.makeFirstResponder(searchField)
        // Focusing the field selects its text: the typed characters go after it, not over it.
        let editor = searchField.currentEditor() as? NSTextView
        editor?.setSelectedRange(NSRange(location: (searchField.stringValue as NSString).length, length: 0))
        editor?.insertText(characters, replacementRange: NSRange(location: NSNotFound, length: 0))
        search(searchField.stringValue)
    }

    @objc private func openClickedRow(_ sender: Any?) {
        open(row: tableView.clickedRow)
    }

    private func openSelectedRow() {
        open(row: tableView.selectedRow >= 0 ? tableView.selectedRow : (visibleRecentURLs.isEmpty ? -1 : 0))
    }

    private func open(row: Int) {
        guard visibleRecentURLs.indices.contains(row) else { return }
        NSDocumentController.shared.openDocument(withContentsOf: visibleRecentURLs[row], display: true) { _, _, _ in }
    }

    // MARK: Table

    func numberOfRows(in tableView: NSTableView) -> Int { visibleRecentURLs.count }

    func tableView(_ tableView: NSTableView, viewFor tableColumn: NSTableColumn?, row: Int) -> NSView? {
        let url = visibleRecentURLs[row]
        let cell = (tableView.makeView(withIdentifier: WelcomeRecentCell.identifier, owner: nil) as? WelcomeRecentCell) ?? WelcomeRecentCell()
        let modified = (try? FileManager.default.attributesOfItem(atPath: url.path))?[.modificationDate] as? Date
        cell.configure(name: url.lastPathComponent, tildePath: Self.tildePath(of: url), date: modified.map { RecentDeckDate.label(for: $0) } ?? "",
                       thumbnail: thumbnail(forRow: row) ?? NSImage(systemSymbolName: "rectangle", accessibilityDescription: nil))
        return cell
    }
}
