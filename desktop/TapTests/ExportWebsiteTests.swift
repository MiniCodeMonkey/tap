import XCTest
@testable import Tap

final class ExportWebsiteTests: HostedTestCase {
    /// seven-slides.md keeps D2's misspelled layout, which tap build refuses;
    /// the website tests open seven-slides-site.md, the same deck spelled right.
    func openSevenSlides(_ name: String = "seven-slides.md") async throws -> (DeckDocument, DeckWindowController, URL) {
        let deck = try Fixtures.copyDeck(name)
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        return (document, try XCTUnwrap(document.windowControllers.first as? DeckWindowController), deck)
    }

    func exportSheet(_ window: DeckWindowController) async throws -> ExportSheet {
        try await waitUntil(timeout: 5, "the export sheet") { window.window?.attachedSheet is ExportSheet }
        return try XCTUnwrap(window.window?.attachedSheet as? ExportSheet)
    }

    /// Waits for the done state and fails at once, with tap's message, on the failed state.
    func waitForTheDoneState(_ sheet: ExportSheet, timeout: TimeInterval) async throws -> ExportSummary {
        var failure: String?
        try await waitUntil(timeout: timeout, "the done state") {
            switch sheet.state {
            case .done: return true
            case .failed(let message): failure = message; return true
            default: return false
            }
        }
        if let failure {
            XCTFail("the export failed: \(failure)")
            throw CancellationError()
        }
        guard case .done(let summary) = sheet.state else { throw CancellationError() }
        return summary
    }

    /// The real bundled tap: tap build needs no browser, and tap serve --json gives the port.
    func testExportAStaticSite() async throws {
        let (document, window, deck) = try await openSevenSlides("seven-slides-site.md")
        var revealed: [URL] = []
        var opened: [URL] = []
        window.revealInFinder = { revealed.append($0) }
        window.openURL = { opened.append($0) }

        window.exportWebsite(nil)
        let sheet = try await exportSheet(window)
        XCTAssertEqual(sheet.titleLabel.stringValue, "Export Website")
        XCTAssertEqual(sheet.request.output, deck.deletingLastPathComponent().appendingPathComponent("dist").path)
        XCTAssertEqual(sheet.outputButton.title, "dist", "the Export to row shows the folder's name")
        XCTAssertEqual(sheet.request.arguments(deck: deck), ["build", deck.path, "--output", sheet.request.output, "--progress", "json"])
        sheet.exportButton.performClick(nil)
        let summary = try await waitForTheDoneState(sheet, timeout: 120)
        XCTAssertEqual(sheet.statusLabel.stringValue, "Website exported")
        XCTAssertTrue(summary.summary.hasPrefix("7 slides, "), summary.summary)
        XCTAssertTrue(summary.summary.hasSuffix("Live code does not run in a static site."))
        XCTAssertTrue(FileManager.default.fileExists(atPath: summary.output.appendingPathComponent("index.html").path), "tap build wrote the site")
        XCTAssertTrue(window.window?.attachedSheet === sheet, "the done state stays up for Show in Finder and Preview")
        sheet.revealButton.performClick(nil)
        XCTAssertEqual(revealed, [summary.output])

        sheet.previewButton.performClick(nil)
        try await waitUntil(timeout: 20, "tap serve's ready line") { !opened.isEmpty }
        let url = try XCTUnwrap(opened.first)
        XCTAssertTrue(url.absoluteString.hasPrefix("http://127.0.0.1:"), "the address tap bound: \(url.absoluteString)")
        XCTAssertNotEqual(url.port, 3000, "a free port, not tap serve's default")
        let (data, response) = try await URLSession.shared.data(from: url.appendingPathComponent("index.html"))
        XCTAssertEqual((response as? HTTPURLResponse)?.statusCode, 200)
        XCTAssertTrue(String(decoding: data, as: UTF8.self).contains("<html"), "the built site is what tap serve serves")
        XCTAssertTrue(window.previewServer?.isRunning ?? false)
        XCTAssertFalse(sheet.doneButton.isHidden, "Done closes the sheet (a listed departure from the ExportWebsite board, which draws no way to close)")
        sheet.doneButton.performClick(nil)
        try await waitUntil(timeout: 10, "the server to stop with the sheet") { window.previewServer?.isRunning == false }
        _ = document
    }

    func testExportSlideImages() async throws {
        let (document, window, deck) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportImages(slides: 7, recordingTo: record)
        var revealed: [URL] = []
        window.revealInFinder = { revealed.append($0) }

        window.exportImages(nil)
        let sheet = try await exportSheet(window)
        XCTAssertEqual(sheet.titleLabel.stringValue, "Export Slide Images")
        let folder = deck.deletingLastPathComponent().appendingPathComponent("seven-slides-slides")
        let deckPath = try XCTUnwrap(document.fileURL?.path)
        XCTAssertEqual(sheet.request.output, folder.path)
        XCTAssertEqual(sheet.request.arguments(deck: URL(fileURLWithPath: deckPath)), ["export", "images", deckPath, "--all", "--output", folder.path, "--progress", "json"])
        sheet.exportButton.performClick(nil)
        let summary = try await waitForTheDoneState(sheet, timeout: 20)
        XCTAssertEqual(summary.summary.split(separator: " ").first, "7")
        XCTAssertEqual(summary.output.path, folder.path)
        XCTAssertTrue(summary.output.hasDirectoryPath, "a folder output is a directory URL")
        XCTAssertTrue(sheet.previewButton.isHidden, "nothing to serve")
        XCTAssertTrue(try String(contentsOf: record, encoding: .utf8).contains("arguments: export images \(deckPath) --all --output \(folder.path) --progress json"))
        sheet.revealButton.performClick(nil)
        XCTAssertEqual(revealed.map(\.path), [folder.path])
        sheet.doneButton.performClick(nil)
    }

    func testABrokenSlideInAnImagesExportIsAWarningWithTheFilesThatLanded() async throws {
        let (_, window, deck) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportImages(slides: 3, broken: [2], recordingTo: record)
        window.exportImages(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        let summary = try await waitForTheDoneState(sheet, timeout: 20)
        XCTAssertEqual(summary.warnings, [ExportWarning(slide: 2, message: "an error card")], "tap's per-slide line, as the ExportWarnings board lists it")
        XCTAssertEqual(sheet.statusLabel.stringValue, "Slide images exported with gaps")
        XCTAssertEqual(sheet.warningsHeader.stringValue, "These slides have no image")
        XCTAssertTrue(FileManager.default.fileExists(atPath: deck.deletingLastPathComponent().appendingPathComponent("seven-slides-slides/slide-001.png").path))
        XCTAssertFalse(FileManager.default.fileExists(atPath: deck.deletingLastPathComponent().appendingPathComponent("seven-slides-slides/slide-002.png").path))
        sheet.doneButton.performClick(nil)
    }

    func testClosingTheDeckStopsThePreviewServer() async throws {
        let (document, window, _) = try await openSevenSlides("seven-slides-site.md")
        window.openURL = { _ in }
        window.exportWebsite(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        _ = try await waitForTheDoneState(sheet, timeout: 120)
        sheet.previewButton.performClick(nil)
        try await waitUntil(timeout: 20, "the server") { window.previewServer?.isRunning == true }
        let identifier = try XCTUnwrap(window.previewServer?.processIdentifier)
        document.close()
        try await waitUntil(timeout: 10, "the server process to be gone") { kill(identifier, 0) != 0 }
    }
}
