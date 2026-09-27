import XCTest
@testable import Tap

/// The export sheet's layout at its real width (the ExportPDF and
/// ExportWarnings boards' sheet), in each of its six states: the options,
/// the download, the render, the done state, the done state with
/// warnings, and a failure. No two rows overlap, every row lies inside the
/// sheet, the bar and its lines lie inside the tinted block's insets, and
/// a long warning truncates inside the warnings box instead of widening
/// the sheet.
final class ExportSheetLayoutTests: HostedTestCase {
    let deck = URL(fileURLWithPath: "/tmp/talks/3am/conference-talk.md")
    let output = URL(fileURLWithPath: "/tmp/talks/3am/conference-talk.pdf")

    func makeSheet() throws -> (ExportSheet, NSView) {
        let sheet = ExportSheet(kind: .pdf(content: "slides"), deck: deck)
        return (sheet, try XCTUnwrap(sheet.contentView))
    }

    func assertSheetWidth(_ content: NSView, file: StaticString = #filePath, line: UInt = #line) {
        XCTAssertEqual(content.bounds.width, 504, accuracy: 0.5, "the sheet keeps its width", file: file, line: line)
    }

    func testTheOptions() throws {
        let (sheet, content) = try makeSheet()
        assertRowsDoNotOverlap([[sheet.contentControl], [sheet.outputButton], [sheet.cancelButton, sheet.exportButton]], in: content)
        assertSheetWidth(content)
    }

    func testTheDownload() throws {
        let (sheet, content) = try makeSheet()
        sheet.apply(.running("Downloading the export engine"))
        sheet.showDownload((bytes: 64_000_000, totalBytes: 150_000_000))
        assertRowsDoNotOverlap([[sheet.contentControl], [sheet.outputButton], [sheet.statusLabel], [sheet.progressBar], [sheet.detailLabel], [sheet.bytesLabel],
                                [sheet.cancelButton, sheet.exportButton]], in: content)
        assertCard(sheet.progressBox, holds: [sheet.progressBar, sheet.detailLabel, sheet.bytesLabel], insets: sheet.progressBox.edgeInsets, in: content)
        assertSheetWidth(content)
    }

    func testTheRender() throws {
        let (sheet, content) = try makeSheet()
        sheet.apply(.running("Rendering slide 7 of 14"))
        assertRowsDoNotOverlap([[sheet.contentControl], [sheet.outputButton], [sheet.statusLabel], [sheet.progressBar], [sheet.cancelButton, sheet.exportButton]], in: content)
        assertCard(sheet.progressBox, holds: [sheet.progressBar], insets: sheet.progressBox.edgeInsets, in: content)
        assertSheetWidth(content)
    }

    func testTheDoneState() throws {
        let (sheet, content) = try makeSheet()
        sheet.apply(.done(ExportSummary(output: output, summary: "14 pages, 2.3 MB in 8.4 s. Finder shows the file.", warnings: [])))
        XCTAssertFalse(sheet.progressBar.isHidden, "the board keeps the bar above the summary")
        XCTAssertEqual(sheet.progressBar.doubleValue, sheet.progressBar.maxValue, "full")
        assertRowsDoNotOverlap([[sheet.statusLabel], [sheet.progressBar], [sheet.detailLabel], [sheet.pathLabel], [sheet.revealButton, sheet.doneButton]], in: content)
        assertCard(sheet.progressBox, holds: [sheet.progressBar, sheet.detailLabel], insets: sheet.progressBox.edgeInsets, in: content)
        assertSheetWidth(content)
    }

    func testTheDoneStateWithWarnings() throws {
        let (sheet, content) = try makeSheet()
        let long = String(repeating: "image images/missing-chart.png not found ", count: 6)
        sheet.apply(.done(ExportSummary(output: output, summary: "14 pages, 2.3 MB in 8.4 s. Finder shows the file.",
                                        warnings: [ExportWarning(slide: 2, message: "component Throws.jsx threw"), ExportWarning(slide: 9, message: long)])))
        XCTAssertEqual(sheet.warningRows.count, 2)
        assertRowsDoNotOverlap([[sheet.statusLabel], [sheet.progressBar], [sheet.detailLabel], [sheet.pathLabel], [sheet.warningsHeader]] + sheet.warningRows.map { [$0] }
                               + [[sheet.revealButton, sheet.doneButton]], in: content)
        assertCard(sheet.progressBox, holds: [sheet.progressBar, sheet.detailLabel], insets: sheet.progressBox.edgeInsets, in: content)
        assertCard(sheet.warningsBox, holds: [sheet.warningsHeader] + sheet.warningRows, insets: sheet.warningsBox.edgeInsets, in: content)
        assertSheetWidth(content)
    }

    func testAFailure() throws {
        let (sheet, content) = try makeSheet()
        sheet.apply(.failed(String(repeating: "failed to start browser: no network. ", count: 5)))
        assertRowsDoNotOverlap([[sheet.contentControl], [sheet.outputButton], [sheet.statusLabel], [sheet.detailLabel], [sheet.cancelButton, sheet.exportButton]], in: content)
        assertCard(sheet.progressBox, holds: [sheet.detailLabel], insets: sheet.progressBox.edgeInsets, in: content)
        assertSheetWidth(content)
    }

    /// The website done state adds Preview next to Show in Finder and Done.
    func testTheWebsiteDoneState() throws {
        let sheet = ExportSheet(kind: .website, deck: deck)
        let content = try XCTUnwrap(sheet.contentView)
        let output = URL(fileURLWithPath: "/tmp/talks/3am/dist")
        sheet.apply(.done(ExportSummary(output: output, summary: "7 slides, 12 files, 340 KB in 1.2 s. Live code does not run in a static site.", warnings: [])))
        XCTAssertFalse(sheet.previewButton.isHidden, "the website board offers Preview")
        assertRowsDoNotOverlap([[sheet.statusLabel], [sheet.progressBar], [sheet.detailLabel], [sheet.pathLabel],
                                [sheet.revealButton, sheet.previewButton, sheet.doneButton]], in: content)
        assertCard(sheet.progressBox, holds: [sheet.progressBar, sheet.detailLabel], insets: sheet.progressBox.edgeInsets, in: content)
        assertSheetWidth(content)
    }

    /// The images done state with gaps: the ExportWarnings board's "These slides have no image".
    func testTheImagesDoneStateWithWarnings() throws {
        let sheet = ExportSheet(kind: .images, deck: deck)
        let content = try XCTUnwrap(sheet.contentView)
        let output = URL(fileURLWithPath: "/tmp/talks/3am/conference-talk-slides")
        sheet.apply(.done(ExportSummary(output: output, summary: "Some slides could not be rendered. The others are in the folder.",
                                        warnings: [ExportWarning(slide: 4, message: "an error card")])))
        XCTAssertTrue(sheet.previewButton.isHidden, "nothing to serve")
        XCTAssertEqual(sheet.warningsHeader.stringValue, "These slides have no image")
        assertRowsDoNotOverlap([[sheet.statusLabel], [sheet.progressBar], [sheet.detailLabel], [sheet.pathLabel], [sheet.warningsHeader]] + sheet.warningRows.map { [$0] }
                               + [[sheet.revealButton, sheet.doneButton]], in: content)
        assertCard(sheet.progressBox, holds: [sheet.progressBar, sheet.detailLabel], insets: sheet.progressBox.edgeInsets, in: content)
        assertCard(sheet.warningsBox, holds: [sheet.warningsHeader] + sheet.warningRows, insets: sheet.warningsBox.edgeInsets, in: content)
        assertSheetWidth(content)
    }
}
