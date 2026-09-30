import XCTest
@testable import Tap

/// What a deck shows while it gets ready: text cards in the slide panel and
/// the preview, a status line that names the wait, a Play glyph that
/// breathes, and the theme tour's thumbnails ready from the bundle.
final class LoadingStatesTests: HostedTestCase {
    private let standIn = TapReady(port: 1, token: "token", launch: "launch")
    private var savedReduceMotion: (() -> Bool)?

    override func setUp() async throws {
        try await super.setUp()
        savedReduceMotion = WelcomeMotion.reduceMotion
        WelcomeMotion.reduceMotion = { false }
    }

    override func tearDown() async throws {
        if let savedReduceMotion { WelcomeMotion.reduceMotion = savedReduceMotion }
        try await super.tearDown()
    }

    /// A panel of its own, sized so its items exist, with no tap behind it.
    private var offscreenWindows: [NSWindow] = []

    private func makePanel(headings: [String]) -> SlidePanelViewController {
        let panel = SlidePanelViewController()
        // A window that is never shown gives the collection view what it needs to make its items.
        let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: SlidePanelViewController.width, height: 700),
                              styleMask: [.titled], backing: .buffered, defer: true)
        window.isReleasedWhenClosed = false
        window.contentView = panel.view
        offscreenWindows.append(window)
        panel.setCards(headings.map { SlideCard(heading: $0) }, paper: PaperColour(red: 0.1, green: 0.12, blue: 0.2))
        window.layoutIfNeeded()
        panel.view.layoutSubtreeIfNeeded()
        panel.collectionView.layoutSubtreeIfNeeded()
        return panel
    }

    private func fixtureImage() throws -> NSImage {
        try XCTUnwrap(NSImage(contentsOf: Fixtures.repositoryRoot.appendingPathComponent("desktop/TapTests/Fixtures/diagram.png")))
    }

    // MARK: Slide cards

    func testSlideCardsFillThePanelAtOnce() async throws {
        let panel = makePanel(headings: ["A Tour", "Same Content", "Why It Matters"])
        XCTAssertTrue(panel.slides.isEmpty, "tap has not answered")
        XCTAssertEqual(panel.collectionView.numberOfItems(inSection: 0), 3, "one placeholder per slide of the Markdown")
        for (index, heading) in ["A Tour", "Same Content", "Why It Matters"].enumerated() {
            let item = try XCTUnwrap(panel.item(forSlide: index + 1), "slide \(index + 1) has an item")
            XCTAssertTrue(item.showsCard)
            XCTAssertEqual(item.cardView.headingLabel.stringValue, heading)
            XCTAssertEqual(item.cardView.paper, PaperColour(red: 0.1, green: 0.12, blue: 0.2), "drawn on the theme's paper")
            XCTAssertNil(item.thumbnailImageView.image)
        }
        XCTAssertEqual(panel.item(forSlide: 1)?.numberLabel.stringValue, "1")

        // tap's slide list replaces the placeholders, and the cards stay until pictures arrive.
        panel.setSlides((1...3).map { Slide(number: $0, startLine: $0, endLine: $0) })
        panel.view.layoutSubtreeIfNeeded()
        XCTAssertEqual(panel.item(forSlide: 2)?.cardView.headingLabel.stringValue, "Same Content")
        XCTAssertTrue(panel.item(forSlide: 2)?.showsCard ?? false)
    }

    func testTheCardsOfAnOpenedDeckComeFromItsMarkdown() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        let controller = try XCTUnwrap(document.sessionController)
        let headings = controller.slidePanel.cards.map(\.heading)
        XCTAssertEqual(headings.count, 7)
        XCTAssertEqual(headings[0], "Debugging Production at 3am")
        XCTAssertEqual(headings[1], "The Page")
        XCTAssertEqual(headings[2], "What We Knew")
        XCTAssertEqual(Array(headings.suffix(3)), ["Root Cause", "Eleven Minutes", "Tail the Logs"])
    }

    func testRealThumbnailsFadeInOverTheCards() async throws {
        let panel = makePanel(headings: ["One", "Two"])
        let image = try fixtureImage()
        let first = try XCTUnwrap(panel.item(forSlide: 1))
        panel.setImage(image, forSlide: 1)
        XCTAssertNotNil(first.thumbnailImageView.image, "the picture is under the card")
        XCTAssertTrue(first.showsCard, "the card is still there while it fades")
        XCTAssertEqual(first.cardView.alphaValue, 0, "and fades to 0 over \(ThumbnailItem.crossfadeDuration) s")
        XCTAssertNotNil(first.cardView.layer?.animation(forKey: "fadeOut"), "the fade runs")
        XCTAssertFalse(first.cardView.isSheening, "the sheen stops with the fade")
        try await waitUntil(timeout: 3, "the card to leave") { !first.showsCard }

        // Reduce Motion: no fade, the picture replaces the card.
        WelcomeMotion.reduceMotion = { true }
        let second = try XCTUnwrap(panel.item(forSlide: 2))
        panel.setImage(image, forSlide: 2)
        XCTAssertFalse(second.showsCard)
        XCTAssertNil(second.cardView.layer?.animation(forKey: "fadeOut"))
    }

    func testTheSheenRunsOnlyWithMotion() throws {
        let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 300, height: 170), styleMask: [.titled], backing: .buffered, defer: true)
        let card = SlideCardView(frame: NSRect(x: 0, y: 0, width: 300, height: 170))
        window.contentView = card
        card.configure(card: SlideCard(heading: "Sheen"), paper: .neutral)
        card.startSheen()
        XCTAssertTrue(card.isSheening)
        card.stopSheen()
        XCTAssertFalse(card.isSheening)
        WelcomeMotion.reduceMotion = { true }
        card.startSheen()
        XCTAssertFalse(card.isSheening, "Reduce Motion: no sheen")
    }

    // MARK: Preview

    private func makePreview(clock: @escaping () -> Date = Date.init) -> PreviewViewController {
        let preview = PreviewViewController()
        preview.now = clock
        preview.placeholder = { (card: SlideCard(heading: "Placeholder heading"), paper: PaperColour.neutral) }
        preview.view.frame = NSRect(x: 0, y: 0, width: 400, height: 400)
        return preview
    }

    func testThePreviewShowsATextCardUntilItsFirstPaint() async throws {
        let preview = makePreview()
        XCTAssertTrue(preview.isLoadingFirstPaint)
        XCTAssertFalse(preview.placeholderCard.isHidden)
        XCTAssertEqual(preview.placeholderCard.headingLabel.stringValue, "Placeholder heading")
        XCTAssertFalse(preview.progressLine.isHidden, "the progress line shows along the top edge")
        XCTAssertEqual(IndeterminateProgressLine.height, 2)

        preview.pageReportedReady(ReadyPayload(revision: "r", slide: 1, step: 0))
        XCTAssertFalse(preview.isLoadingFirstPaint)
        XCTAssertTrue(preview.progressLine.isHidden)
        try await waitUntil(timeout: 3, "the card to go") { preview.placeholderCard.isHidden }

        // A later ready does not bring it back.
        preview.pageReportedReady(ReadyPayload(revision: "r", slide: 2, step: 0))
        XCTAssertTrue(preview.placeholderCard.isHidden)
    }

    func testTheCardOfARealPreviewGoesAtItsFirstPaint() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("seven-slides.md"))
        let preview = try XCTUnwrap(document.sessionController?.previewViewController)
        XCTAssertFalse(preview.isLoadingFirstPaint)
        try await waitUntil(timeout: 3, "the card to go") { preview.placeholderCard.isHidden }
        XCTAssertTrue(preview.progressLine.isHidden)
        XCTAssertEqual(preview.statusLabel.alphaValue, 0)
    }

    func testThePreviewNamesTheStepAfterOneSecond() async throws {
        var now = Date()
        let preview = makePreview(clock: { now })
        preview.refreshStatus()
        XCTAssertEqual(preview.statusLabel.alphaValue, 0, "no words at first")

        now += 0.9
        preview.refreshStatus()
        XCTAssertEqual(preview.statusLabel.alphaValue, 0, "nothing before one second")

        now += 0.2
        preview.refreshStatus()
        XCTAssertEqual(preview.statusLabel.alphaValue, 1)
        XCTAssertEqual(preview.statusLabel.stringValue, "Starting the preview…")

        // tap runs and the page loads: the step changes.
        preview.showSessionState(.running(standIn), restartPolicy: RestartPolicy())
        preview.refreshStatus()
        XCTAssertEqual(preview.statusLabel.stringValue, "Loading the slides…")

        preview.pageReportedReady(ReadyPayload(revision: "r", slide: 1, step: 0))
        XCTAssertEqual(preview.statusLabel.alphaValue, 0, "the line fades out at the paint")
        XCTAssertNotNil(preview.statusLabel.layer?.animation(forKey: "fadeOut"))
        now += 5
        preview.refreshStatus()
        XCTAssertEqual(preview.statusLabel.alphaValue, 0, "and stays gone")
    }

    func testThePreviewStopsLoadingWhenItFails() async throws {
        var now = Date()
        let preview = makePreview(clock: { now })
        now += 3
        preview.refreshStatus()
        XCTAssertEqual(preview.statusLabel.alphaValue, 1, "the wait has a status line")

        preview.showSessionState(.failed(lastOutput: ["boom"]), restartPolicy: RestartPolicy())
        XCTAssertFalse(preview.isLoadingFirstPaint, "an error state ends the loading state")
        XCTAssertTrue(preview.placeholderCard.isHidden)
        XCTAssertTrue(preview.progressLine.isHidden)
        XCTAssertFalse(preview.progressLine.isRunning)
        XCTAssertEqual(preview.statusLabel.alphaValue, 0, "the status line does not keep spinning")
        now += 10
        preview.refreshStatus()
        XCTAssertEqual(preview.statusLabel.alphaValue, 0)
        XCTAssertFalse(preview.overlay.isHidden, "the error is what shows")
    }

    func testTheProgressLineStandsStillWithReduceMotion() throws {
        WelcomeMotion.reduceMotion = { true }
        let window = NSWindow(contentRect: NSRect(x: 0, y: 0, width: 300, height: 100), styleMask: [.titled], backing: .buffered, defer: true)
        let line = IndeterminateProgressLine(frame: NSRect(x: 0, y: 0, width: 300, height: 2))
        window.contentView = line
        line.start()
        XCTAssertFalse(line.isTravelling)
        WelcomeMotion.reduceMotion = { false }
        line.start()
        XCTAssertTrue(line.isTravelling)
        line.stop()
        XCTAssertFalse(line.isTravelling)
    }

    // MARK: Play

    func testPlayIsDimmedWhileTapGetsReady() async throws {
        let control = PlaySplitControl()
        control.readyToolTip = "Play from the current slide"
        XCTAssertFalse(control.isGettingReady)
        XCTAssertEqual(control.currentToolTip, "Play from the current slide")

        control.isGettingReady = true
        XCTAssertEqual(control.toolTip, "Getting the slides ready…")
        XCTAssertTrue(control.isBreathing, "the glyph breathes")
        let breathe = try XCTUnwrap(control.layer?.animation(forKey: PlaySplitControl.breatheAnimationKey) as? CABasicAnimation)
        XCTAssertEqual(breathe.duration * 2, 1.6, accuracy: 0.001, "a slow 1.6 s cycle")
        XCTAssertLessThan(try XCTUnwrap(breathe.fromValue as? Float), 0.3)
        XCTAssertGreaterThan(try XCTUnwrap(breathe.toValue as? Float), 0.5)

        control.isGettingReady = false
        XCTAssertFalse(control.isBreathing)
        XCTAssertEqual(control.layer?.opacity, 1, "solid when ready")
        XCTAssertEqual(control.toolTip, "Play from the current slide")

        WelcomeMotion.reduceMotion = { true }
        control.isGettingReady = true
        XCTAssertFalse(control.isBreathing, "Reduce Motion: dimmed and still")
        XCTAssertLessThan(control.layer?.opacity ?? 1, 0.6)
    }

    func testPlayFollowsTapsReadiness() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        let controller = try XCTUnwrap(document.sessionController)
        let deckWindow = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        XCTAssertEqual(deckWindow.playButton.isGettingReady, !controller.isTapReady)
        _ = try await waitForRunningTap(document)
        try await waitUntil(timeout: 5, "Play to turn solid") { !deckWindow.playButton.isGettingReady }
        XCTAssertFalse(deckWindow.playButton.isBreathing)
        XCTAssertEqual(deckWindow.playButton.toolTip, deckWindow.playButton.readyToolTip)
    }

    // MARK: The theme tour

    private func makeTourFolder(source: String, slides: Int) throws -> BundledTourThumbnails {
        let folder = try Fixtures.temporaryFolder()
        try source.write(to: folder.appendingPathComponent(BundledTourThumbnails.sourceFileName), atomically: true, encoding: .utf8)
        let png = Fixtures.repositoryRoot.appendingPathComponent("desktop/TapTests/Fixtures/diagram.png")
        for number in 1...slides {
            try FileManager.default.copyItem(at: png, to: folder.appendingPathComponent("slide-\(number).png"))
        }
        return BundledTourThumbnails(folder: folder)
    }

    func testTheThemeTourOpensWithItsThumbnails() async throws {
        let text = try String(contentsOf: Fixtures.repositoryRoot.appendingPathComponent("examples/theme-tour.md"), encoding: .utf8)
        let slideCount = SlideCard.cards(inDeckMarkdown: text).count
        AppEnvironment.shared.tourThumbnails = try makeTourFolder(source: text, slides: slideCount)

        let document = try DeckDocument.makeUntitled(text: text)
        NSApp.activate(ignoringOtherApps: true)
        document.windowControllers.first?.window?.orderFrontRegardless()
        let controller = try XCTUnwrap(document.sessionController)
        _ = try await waitForRunningTap(document)
        try await waitUntil(timeout: 20, "every tour slide to have its picture") {
            controller.slidePanel.slides.count == slideCount && (1...slideCount).allSatisfy { controller.slidePanel.image(forSlide: $0) != nil }
        }
        let renderer = controller.thumbnails.renderer
        XCTAssertEqual(renderer.renderCount, 0, "every picture came from the bundle")
        XCTAssertEqual(renderer.navigationCount, 0, "and no render started")
        XCTAssertEqual(renderer.pendingCount, 0)
        for number in 1...slideCount {
            let key = try XCTUnwrap(controller.thumbnails.key(forSlide: number))
            XCTAssertTrue(AppEnvironment.shared.thumbnailCache.contains(key), "slide \(number) is in the cache under its own key")
        }

        // An edit to one slide renders that slide as usual; the others keep the bundle's pictures.
        let editor = controller.editor
        let heading = (editor.string as NSString).range(of: "# Same Content, Different Room").upperBound
        editor.setSelectedRange(NSRange(location: heading, length: 0))
        editor.insertText(" now edited", replacementRange: NSRange(location: NSNotFound, length: 0))
        try await waitForRenderer(renderer, of: controller, timeout: 40, "the edited slide to render") { renderer.renderCount >= 1 }
        XCTAssertEqual(renderer.lastRenderedSlide, 2)
        XCTAssertEqual(renderer.renderCount, 1, "only the edited slide rendered")
    }

    func testAnEditedTourIsNotSeeded() throws {
        let tour = try makeTourFolder(source: "# One\n\n---\n\n# Two\n", slides: 2)
        XCTAssertEqual(tour.imageURLs(forText: "# One\n\n---\n\n# Two\n", slideCount: 2).count, 2)
        XCTAssertTrue(tour.imageURLs(forText: "# One\n\n---\n\n# Two edited\n", slideCount: 2).isEmpty)
    }

    func testTheBundleHoldsTheTourThumbnails() throws {
        let bundled = BundledTourThumbnails(resourcesFolder: Bundle.main.resourceURL!)
        let text = try String(contentsOf: Fixtures.repositoryRoot.appendingPathComponent("examples/theme-tour.md"), encoding: .utf8)
        let urls = bundled.imageURLs(forText: text, slideCount: SlideCard.cards(inDeckMarkdown: text).count)
        XCTAssertFalse(urls.isEmpty, "the build renders the tour's slides into the bundle")
        for url in urls.values { XCTAssertNotNil(NSImage(contentsOf: url)) }
    }
}
