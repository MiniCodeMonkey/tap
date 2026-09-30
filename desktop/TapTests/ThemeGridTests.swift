import XCTest
@testable import Tap

final class ThemeGridTests: HostedTestCase {
    /// A scripted tap whose theme renders are the fixture PNG; `tap theme
    /// list` stays real, so the names and the groups are tap's.
    func useFakeRenders(downloadLines: Int = 0) throws -> URL {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        let png = Fixtures.repositoryRoot.appendingPathComponent("desktop/TapTests/Fixtures/diagram.png")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.themeShow(png: png, downloadLines: downloadLines, recordingTo: record)
        return record
    }

    func testThemePickerListsTapSThemes() async throws {
        let record = try useFakeRenders()
        let grid = ThemeGridViewController(cellSize: ThemeGridViewController.popoverCellSize)
        grid.loadViewIfNeeded()
        try await waitUntil(timeout: 20, "the catalog") { AppEnvironment.shared.themeImages.catalog != nil }
        let catalog = try XCTUnwrap(AppEnvironment.shared.themeImages.catalog)
        XCTAssertEqual(catalog.themes.count, 21, "every theme tap lists")
        XCTAssertEqual(grid.cells.map(\.slug), ["default"] + catalog.light.map(\.slug) + catalog.dark.map(\.slug), "Default first, then light and dark in tap's order")
        XCTAssertEqual(grid.sectionTitles, ["Light", "Dark"])
        XCTAssertEqual(grid.cell(for: "default")?.nameLabel.stringValue, "Default")
        XCTAssertEqual(grid.cell(for: "default")?.accessibilityLabel(), "Default, tap's default theme (Base)")
        XCTAssertEqual(grid.cell(for: "terminal")?.nameLabel.stringValue, "Terminal")
        XCTAssertEqual(grid.cell(for: "terminal")?.accessibilityLabel(), "Terminal, dark theme")
        grid.view.layoutSubtreeIfNeeded()
        XCTAssertEqual(grid.cell(for: "terminal")?.imageView.frame.size, ThemeGridViewController.popoverCellSize)
        XCTAssertEqual(ThemeGridViewController.popoverCellSize, NSSize(width: 104, height: 58), "the ThemePicker and DeckTabThemeRow boards' cells")
        XCTAssertEqual(grid.cell(for: "terminal")?.nameLabel.font?.pointSize, 11.5, "the boards' names in the popover")
        // The renders land one at a time, in the grid's order, from tap theme show --image; Default shows the default theme's render.
        try await waitUntil(timeout: 60, "every render") { grid.cells.allSatisfy { $0.imageView.image != nil } }
        let loader = AppEnvironment.shared.themeImages
        try await waitUntil(timeout: 10, "the loader to finish") { !loader.isWorking }
        let shows = try String(contentsOf: record, encoding: .utf8).components(separatedBy: "\n").filter { $0.hasPrefix("arguments: theme show") }
        XCTAssertEqual(shows.count, 21, "no render for Default: it is the default theme's")
        XCTAssertTrue(shows[0].hasPrefix("arguments: theme show \(catalog.light[0].slug) --image --json --progress json"), shows[0])
        XCTAssertTrue(grid.cell(for: "default")?.imageView.image === grid.cell(for: AppEnvironment.shared.themeImages.defaultSlug)?.imageView.image)
        XCTAssertEqual(grid.footerLabel.stringValue, "Picking a theme runs tap theme set. Press T in the preview to try one without saving.")
        // A second grid shows the renders at once: the images are kept for the app's life.
        let again = ThemeGridViewController(cellSize: ThemeGridViewController.sheetCellSize)
        again.loadViewIfNeeded()
        again.view.layoutSubtreeIfNeeded()
        XCTAssertTrue(again.cells.allSatisfy { $0.imageView.image != nil }, "the renders show at once")
        XCTAssertEqual(again.cell(for: "base")?.imageView.frame.size, ThemeGridViewController.sheetCellSize, "the New Deck sheet's smaller cells")
        // Anything the second grid started would have run by now.
        try await Task.sleep(nanoseconds: 500_000_000)
        try await waitUntil(timeout: 10, "the loader to finish") { !loader.isWorking }
        XCTAssertEqual(try String(contentsOf: record, encoding: .utf8).components(separatedBy: "\n").filter { $0.hasPrefix("arguments: theme show") }.count, 21, "no second render")
    }

    func testTheEngineDownloadShowsUnderTheGrid() async throws {
        _ = try useFakeRenders(downloadLines: 3)
        let grid = ThemeGridViewController(cellSize: ThemeGridViewController.popoverCellSize)
        grid.loadViewIfNeeded()
        try await waitUntil(timeout: 20, "the download label") { !grid.downloadLabel.isHidden }
        XCTAssertTrue(grid.downloadLabel.stringValue.hasPrefix("Downloading the export engine"), grid.downloadLabel.stringValue)
        try await waitUntil(timeout: 60, "every render") { grid.cells.allSatisfy { $0.imageView.image != nil } }
        XCTAssertTrue(grid.downloadLabel.isHidden, "the label goes with the last download line")
    }

    func testAPickReportsTheSlugAndMarksTheCell() async throws {
        _ = try useFakeRenders()
        let grid = ThemeGridViewController(cellSize: ThemeGridViewController.popoverCellSize)
        grid.loadViewIfNeeded()
        try await waitUntil(timeout: 20, "the cells") { !grid.cells.isEmpty }
        var picked: [String] = []
        grid.onPick = { picked.append($0) }
        grid.selectedSlug = "terminal"
        XCTAssertEqual(grid.cell(for: "terminal")?.isSelected, true)
        grid.cell(for: "blueprint")?.performClick(nil)
        XCTAssertEqual(picked, ["blueprint"])
        XCTAssertEqual(grid.selectedSlug, "blueprint")
        XCTAssertEqual(grid.cell(for: "terminal")?.isSelected, false)
        grid.selectedSlug = nil
        XCTAssertEqual(grid.cell(for: "default")?.isSelected, true, "no theme reads as Default")
        grid.view.layoutSubtreeIfNeeded()
        let cell = try XCTUnwrap(grid.cell(for: "base"))
        let center = cell.convert(NSPoint(x: cell.bounds.midX, y: cell.bounds.midY), to: cell.superview)
        XCTAssertTrue(cell.hitTest(center) === cell, "a click on the render or the name is the cell's (hitTest takes the superview's coordinates)")
    }

    /// The loader outlives every grid: a render finishing after its grid
    /// is gone lands in the loader and touches nothing else.
    func testAFreedGridLeavesTheLoaderRunning() async throws {
        _ = try useFakeRenders()
        var grid: ThemeGridViewController? = ThemeGridViewController(cellSize: ThemeGridViewController.popoverCellSize)
        grid?.loadViewIfNeeded()
        try await waitUntil(timeout: 20, "the cells") { !(grid?.cells.isEmpty ?? true) }
        weak var gone = grid
        grid = nil
        try await waitUntil(timeout: 5, "the grid to be freed") { gone == nil }
        try await waitUntil(timeout: 60, "the renders") { AppEnvironment.shared.themeImages.image(for: "blueprint") != nil }
    }

    /// `stop` (every test's teardown, the app's quit) ends the renders:
    /// the run in flight is cancelled and no later render starts.
    func testStopEndsTheRenders() async throws {
        // Each scripted render takes most of a second, so the stop lands mid-run.
        let record = try useFakeRenders(downloadLines: 3)
        let loader = AppEnvironment.shared.themeImages
        let grid = ThemeGridViewController(cellSize: ThemeGridViewController.popoverCellSize)
        grid.loadViewIfNeeded()
        try await waitUntil(timeout: 20, "a render to start") { !grid.downloadLabel.isHidden }
        loader.stop()
        XCTAssertFalse(loader.isWorking)
        XCTAssertTrue(grid.downloadLabel.isHidden, "the download state goes with the stop")
        let started = try String(contentsOf: record, encoding: .utf8).components(separatedBy: "\n").filter { $0.hasPrefix("arguments: theme show") }.count
        try await Task.sleep(nanoseconds: 2_000_000_000)
        let later = try String(contentsOf: record, encoding: .utf8).components(separatedBy: "\n").filter { $0.hasPrefix("arguments: theme show") }.count
        XCTAssertEqual(later, started, "no render starts after the stop")
        XCTAssertLessThan(started, 21)
        try await waitUntil(timeout: 6, "the cancelled run to end") { ToolRun.activeRuns.isEmpty }
    }

    /// The scenario: a pick in the toolbar's pop-up rewrites the theme line, and the preview re-renders.
    func testChangeTheDeckSTheme() async throws {
        _ = try useFakeRenders()
        let deck = try Fixtures.copyDeck("themed.md")
        let document = try await openDeckAndWaitForPreview(deck)
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        try await waitUntil(timeout: 20, "the catalog") { AppEnvironment.shared.themeImages.catalog != nil }
        try await waitUntil(timeout: 5, "the toolbar's title") { window.themeButton.title == "Terminal" }
        controller.jumpToSlide(number: 2)
        let before = try await waitForPreview(document, slide: 2)

        window.showThemePopover(nil)
        XCTAssertTrue(window.themePopover.isShown)
        XCTAssertEqual(window.themePopover.grid.selectedSlug, "terminal", "the deck's theme is marked")
        try await waitUntil(timeout: 20, "the cells") { !window.themePopover.grid.cells.isEmpty }
        window.themePopover.grid.cell(for: "blueprint")?.performClick(nil)
        XCTAssertFalse(window.themePopover.isShown, "a pick closes the popover")

        try await waitUntil(timeout: 20, "the theme line to change") { controller.editor.string.contains("theme: blueprint") }
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Change Theme")
        try await waitUntil(timeout: 10, "the toolbar") { window.themeButton.title == "Blueprint" }
        // The preview re-rendered: tap reloads its pages on the file change, and the app resends the slide.
        try await waitUntil(timeout: 20, "a newer render of slide 2") {
            guard let latest = controller.previewViewController.lastReady else { return false }
            return latest.slide == 2 && latest.revision != before.revision
        }

        // The Default cell removes the theme line; the toolbar reads Default.
        window.showThemePopover(nil)
        window.themePopover.grid.cell(for: "default")?.performClick(nil)
        try await waitUntil(timeout: 20, "the theme line to go") { !controller.editor.string.contains("theme:") }
        try await waitUntil(timeout: 10, "the toolbar") { window.themeButton.title == "Default" }
    }
}
