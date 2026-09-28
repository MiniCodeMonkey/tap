import XCTest
@testable import Tap

final class ExportPDFTests: HostedTestCase {
    override func setUp() async throws {
        try await super.setUp()
        executionTimeAllowance = 600
    }

    func openSevenSlides() async throws -> (DeckDocument, DeckWindowController, URL) {
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        return (document, try XCTUnwrap(document.windowControllers.first as? DeckWindowController), deck)
    }

    func exportSheet(_ window: DeckWindowController) async throws -> ExportSheet {
        try await waitUntil(timeout: 5, "the export sheet") { window.window?.attachedSheet is ExportSheet }
        return try XCTUnwrap(window.window?.attachedSheet as? ExportSheet)
    }

    func isRunning(_ sheet: ExportSheet, statusPrefix: String) -> Bool {
        if case .running(let status) = sheet.state { return status.hasPrefix(statusPrefix) }
        return false
    }

    func isDone(_ sheet: ExportSheet) -> Bool {
        if case .done = sheet.state { return true }
        return false
    }

    /// The real bundled tap, the real export engine (CI caches its download): the PDF matches the CLI's.
    func testExportAPDF() async throws {
        let (document, window, deck) = try await openSevenSlides()
        let controller = try XCTUnwrap(document.sessionController)
        let deckPath = try XCTUnwrap(document.fileURL?.path)
        controller.editor.insertText("edited before the export ", replacementRange: NSRange(location: controller.editor.hiddenLength, length: 0))
        var revealed: [URL] = []
        window.revealInFinder = { revealed.append($0) }

        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        XCTAssertEqual(sheet.titleLabel.stringValue, "Export PDF")
        XCTAssertEqual(sheet.contentControl.label(forSegment: sheet.contentControl.selectedSegment), "Slides")
        let expectedOutput = deck.deletingPathExtension().appendingPathExtension("pdf").path
        XCTAssertEqual(sheet.request.output, expectedOutput, "next to the deck by default")
        XCTAssertEqual(sheet.outputButton.title, "seven-slides.pdf", "the Save as row shows the file's name, as the board draws")
        sheet.contentControl.selectedSegment = 2
        XCTAssertEqual(sheet.request.arguments(deck: URL(fileURLWithPath: deckPath)), ["export", "pdf", deckPath, "--output", expectedOutput, "--content", "both", "--progress", "json"])
        sheet.exportButton.performClick(nil)

        var sawRender = false
        try await waitUntil(timeout: 240, "the export to finish") {
            if self.isRunning(sheet, statusPrefix: "Rendering slide") { sawRender = true }
            return window.window?.attachedSheet == nil
        }
        XCTAssertTrue(sawRender, "real progress showed")
        XCTAssertEqual(revealed.map(\.path), [expectedOutput], "the file is revealed in Finder")
        let attributes = try FileManager.default.attributesOfItem(atPath: expectedOutput)
        XCTAssertGreaterThan((attributes[.size] as? Int) ?? 0, 1000, "a real PDF")
        XCTAssertTrue(try String(contentsOf: deck, encoding: .utf8).contains("edited before the export"), "the buffer was saved first")
        XCTAssertFalse(window.exportController.isRunning)
    }

    /// Every state the controller hands the sheet, in order, from now on.
    func recordStates(_ window: DeckWindowController) -> () -> [ExportController.State] {
        var states: [ExportController.State] = []
        let shown = window.exportController.onStateChange
        window.exportController.onStateChange = { state in
            states.append(state)
            shown?(state)
        }
        return { states }
    }

    func testFirstPDFExport() async throws {
        let (_, window, _) = try await openSevenSlides()
        let folder = try Fixtures.temporaryFolder()
        let record = folder.appendingPathComponent("record.txt")
        // The fake holds its first render line until the test has seen it.
        let gate = folder.appendingPathComponent("gate")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 3, downloadLines: 4, gate: gate, recordingTo: record)
        window.revealInFinder = { _ in }
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 10, "the download state") { self.isRunning(sheet, statusPrefix: "Downloading the export engine") }
        XCTAssertEqual(sheet.statusLabel.stringValue, "Downloading the export engine")
        XCTAssertEqual(sheet.detailLabel.stringValue, "This happens once. tap pdf renders with Chromium, so the PDF matches the CLI exactly.")
        XCTAssertFalse(sheet.contentControl.isEnabled, "the options stay in view, off, as the ExportPDF board draws the download")
        XCTAssertFalse(sheet.contentControl.isHidden)
        XCTAssertTrue(sheet.progressBar.doubleValue > 0 && !sheet.progressBar.isIndeterminate, "bytes of total")
        XCTAssertTrue(sheet.bytesLabel.stringValue.contains(" of "), sheet.bytesLabel.stringValue)
        // The first render line ends the download state and shows at once.
        try await waitUntil(timeout: 20, "the render after the download") { self.isRunning(sheet, statusPrefix: "Rendering slide 1 of 3") }
        XCTAssertEqual(sheet.statusLabel.stringValue, "Rendering slide 1 of 3")
        XCTAssertEqual(sheet.bytesLabel.stringValue, "", "the download's bytes are gone")
        try Data().write(to: gate)
        try await waitUntil(timeout: 20, "the sheet to finish") { window.window?.attachedSheet == nil }
    }

    /// The buffer is saved before tap reads the file: the fake copies the
    /// deck as it starts. Autosave waits long enough that only Export's
    /// own save can have written the edit.
    func testTheBufferIsSavedBeforeTapStarts() async throws {
        let savedDelay = NSDocumentController.shared.autosavingDelay
        NSDocumentController.shared.autosavingDelay = 300
        addTeardownBlock { @MainActor in NSDocumentController.shared.autosavingDelay = savedDelay }
        let (document, window, _) = try await openSevenSlides()
        let controller = try XCTUnwrap(document.sessionController)
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 3, recordingTo: record)
        window.revealInFinder = { _ in }
        controller.editor.insertText("edited before the export ", replacementRange: NSRange(location: controller.editor.hiddenLength, length: 0))
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 20, "the sheet to finish") { window.window?.attachedSheet == nil }
        let read = try String(contentsOf: URL(fileURLWithPath: record.path + ".deck"), encoding: .utf8)
        XCTAssertTrue(read.contains("edited before the export"), "tap read the saved buffer")
    }

    func testASlideFailsDuringExport() async throws {
        let (_, window, _) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 3, broken: [(2, "component Throws.jsx threw")], recordingTo: record)
        var revealed: [URL] = []
        window.revealInFinder = { revealed.append($0) }
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 20, "the done state") { self.isDone(sheet) }
        guard case .done(let summary) = sheet.state else { return XCTFail("not done") }
        XCTAssertEqual(summary.warnings, [ExportWarning(slide: 2, message: "component Throws.jsx threw")], "tap's brokenSlides, as warnings")
        XCTAssertEqual(revealed.count, 1, "the file is still revealed as it finishes: tap wrote it")
        XCTAssertTrue(window.window?.attachedSheet === sheet, "a warning keeps the sheet up so the person reads it")
        // The ExportWarnings board.
        XCTAssertEqual(sheet.statusLabel.stringValue, "PDF exported with 1 warning")
        XCTAssertTrue(sheet.detailLabel.stringValue.hasSuffix("Finder shows the file."), sheet.detailLabel.stringValue)
        XCTAssertEqual(sheet.warningsHeader.stringValue, "These pages show an error card in the PDF")
        XCTAssertEqual(sheet.warningRows.map(\.stringValue), ["Slide 2  component Throws.jsx threw"])
        XCTAssertFalse(sheet.warningRows[0].textColor == .systemOrange, "normal text; the icon carries the color")
        XCTAssertFalse(sheet.doneButton.isHidden)
        XCTAssertEqual(sheet.doneButton.keyEquivalent, "\r", "Done is the default button")
        XCTAssertFalse(sheet.revealButton.isHidden)
        XCTAssertTrue(sheet.previewButton.isHidden)
        sheet.doneButton.performClick(nil)
        try await waitUntil(timeout: 5, "the sheet to close") { window.window?.attachedSheet == nil }
    }

    func testCancelAnExport() async throws {
        let (_, window, _) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        // After SIGINT the fake prints two more render lines, as tap finishes the slide in flight.
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 40, secondsPerSlide: 0.5, renderLinesAfterInterrupt: 2, recordingTo: record)
        var revealed: [URL] = []
        window.revealInFinder = { revealed.append($0) }
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        let states = recordStates(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 10, "rendering") { self.isRunning(sheet, statusPrefix: "Rendering slide") }
        XCTAssertFalse(window.canStartATalk, "no talk while the export reads the file")
        let beforeCancel = states().count
        sheet.cancelButton.performClick(nil)
        XCTAssertEqual(sheet.statusLabel.stringValue, "Cancelling…")
        XCTAssertFalse(sheet.cancelButton.isEnabled)
        try await waitUntil(timeout: 10, "the sheet to close") { window.window?.attachedSheet == nil }
        XCTAssertEqual(Array(states()[beforeCancel...]), [.running("Cancelling…"), .idle], "the render lines tap prints after SIGINT change nothing")
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertTrue(recorded.contains("arguments: export pdf"))
        XCTAssertEqual(revealed, [], "nothing is revealed for a cancelled export")
        XCTAssertFalse(window.exportController.isRunning)
        XCTAssertTrue(window.canStartATalk)
        try await waitUntil(timeout: 5, "tap's exit in the log") { window.sessionController.session.log.text.contains("export cancelled (exit 130)") }
    }

    /// The export lines the fake recorded: one per run of tap export pdf.
    func exportRuns(_ record: URL) -> [String] {
        ((try? String(contentsOf: record, encoding: .utf8)) ?? "").components(separatedBy: "\n").filter { $0.hasPrefix("arguments: export pdf") }
    }

    /// Cancel pressed while the export is still saving or preparing (the
    /// login shell's environment loads on the first run after launch):
    /// the sheet closes, tap never runs, and nothing is revealed.
    func testCancelWhileTheExportStarts() async throws {
        let (_, window, _) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 3, recordingTo: record)
        var revealed: [URL] = []
        window.revealInFinder = { revealed.append($0) }
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        XCTAssertTrue(window.exportController.isStarting, "still saving: tap has not started")
        XCTAssertTrue(sheet.cancelButton.isEnabled, "Cancel is on while the export starts")
        sheet.cancelButton.performClick(nil)
        XCTAssertEqual(sheet.statusLabel.stringValue, "Cancelling…")
        try await waitUntil(timeout: 10, "the sheet to close") { window.window?.attachedSheet == nil }
        XCTAssertFalse(window.exportController.isRunning)
        // Long enough for a start that ignored the cancel to reach tap.
        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertEqual(exportRuns(record), [], "tap never ran")
        XCTAssertEqual(revealed, [], "nothing is revealed")
        XCTAssertTrue(window.sessionController.session.log.text.contains("export cancelled before tap started"))
        XCTAssertTrue(window.canStartATalk)
    }

    /// The deck's window closes while its export is still saving or
    /// preparing: tap never runs.
    func testClosingTheDeckWhileItsExportStartsRunsNothing() async throws {
        let (document, window, _) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 3, recordingTo: record)
        window.revealInFinder = { _ in }
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        let exportController = window.exportController
        sheet.exportButton.performClick(nil)
        XCTAssertTrue(exportController.isStarting, "still saving: tap has not started")
        document.close()
        try await waitUntil(timeout: 10, "the start to end") { !exportController.isRunning }
        try await Task.sleep(nanoseconds: 1_000_000_000)
        XCTAssertEqual(exportRuns(record), [], "tap never ran for a closed deck")
    }

    /// A PDF already where the default output points is not replaced
    /// silently: Export opens the save panel, whose own prompt asks before
    /// replacing. Cancel there runs nothing; a confirmed name runs tap.
    func testAnExistingPDFIsNotReplacedWithoutAsking() async throws {
        let (_, window, deck) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 2, recordingTo: record)
        window.revealInFinder = { _ in }
        let existing = deck.deletingPathExtension().appendingPathExtension("pdf")
        try "a hand-made pdf".write(to: existing, atomically: true, encoding: .utf8)
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        var asked: [String] = []
        var answer: String?
        sheet.chooseOutput = { _, current, completion in
            asked.append(current)
            completion(answer)
        }

        sheet.exportButton.performClick(nil)
        XCTAssertEqual(asked, [existing.path], "the save panel asks about the existing file")
        XCTAssertTrue(sheet.exportButton.isEnabled, "cancelled in the panel: Export is on again")
        try await Task.sleep(nanoseconds: 500_000_000)
        XCTAssertEqual(exportRuns(record), [], "nothing ran")
        XCTAssertEqual(try String(contentsOf: existing, encoding: .utf8), "a hand-made pdf")

        let other = existing.deletingLastPathComponent().appendingPathComponent("talk-export.pdf").path
        answer = other
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 20, "the sheet to finish") { window.window?.attachedSheet == nil }
        XCTAssertEqual(exportRuns(record).count, 1)
        XCTAssertTrue(exportRuns(record)[0].contains("--output \(other)"), "tap wrote the name the panel returned")
        XCTAssertEqual(try String(contentsOf: existing, encoding: .utf8), "a hand-made pdf", "the hand-made file is untouched")
    }

    /// Two starts in a row, before the first has reached tap: one run.
    func testStartingTwiceRunsOnce() async throws {
        let (document, window, deck) = try await openSevenSlides()
        let log = try XCTUnwrap(document.sessionController).session.log
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 2, recordingTo: record)
        let request = ExportRequest(kind: .pdf(content: "slides"), output: deck.deletingPathExtension().appendingPathExtension("pdf").path)
        window.exportController.start(request)
        window.exportController.start(request)
        try await waitUntil(timeout: 20, "the export to finish") {
            if case .done = window.exportController.state { return true } else { return false }
        }
        XCTAssertEqual(exportRuns(record).count, 1, "the second start found the first still starting")
        // The Tap Log names every run the app makes. A second run would be
        // started too, and the first one freed (and interrupted) the moment
        // the second took its place, often before its tap recorded
        // anything, so the record alone cannot tell one start from two.
        let made = log.lines.filter { $0.source == .app && $0.text.hasPrefix("tap export pdf") }
        XCTAssertEqual(made.count, 1, "one run made: \(made.map(\.text))")
    }

    func testASecondExportPressStartsNoSecondRun() async throws {
        let (_, window, _) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 4, secondsPerSlide: 0.3, recordingTo: record)
        window.revealInFinder = { _ in }
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        XCTAssertFalse(sheet.exportButton.isEnabled, "off the moment it is pressed, before the save lands")
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 20, "the sheet to finish") { window.window?.attachedSheet == nil }
        let starts = try String(contentsOf: record, encoding: .utf8).components(separatedBy: "\n").filter { $0.hasPrefix("arguments: export pdf") }
        XCTAssertEqual(starts.count, 1, "one run")
    }

    func testClosingTheDeckStopsItsExport() async throws {
        let (document, window, _) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 40, secondsPerSlide: 0.5, recordingTo: record)
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 10, "rendering") { self.isRunning(sheet, statusPrefix: "Rendering slide") }
        let identifier = try XCTUnwrap(window.exportController.processIdentifier)
        document.close()
        try await waitUntil(timeout: 10, "the export process to be gone") { kill(identifier, 0) != 0 }
    }

    /// A live code question that arrives while the export sheet is up
    /// waits, and gets its turn the moment the sheet closes.
    func testAQuestionArrivingDuringAnExportShowsAfterTheSheet() async throws {
        let (_, window, deck) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 6, secondsPerSlide: 0.3, recordingTo: record)
        window.revealInFinder = { _ in }
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 10, "rendering") { self.isRunning(sheet, statusPrefix: "Rendering slide") }
        let payload = QuestionPayload(deck: Fixtures.realPath(of: deck), drivers: [ApprovalDriver(name: "shell", slides: [2], blocks: 1)],
                                      blocks: [ApprovalBlock(driver: "shell", code: "echo hi", slide: 2, block: 1)])
        window.presentDeckQuestion(DeckSessionController.PendingQuestion(id: "q9", kind: "approval", payload: payload))
        XCTAssertTrue(window.window?.attachedSheet === sheet, "the export sheet keeps its window")
        XCTAssertEqual(window.deckQuestions.count, 1, "the question waits")
        try await waitUntil(timeout: 20, "the export sheet to close") { window.window?.attachedSheet !== sheet }
        try await waitUntil(timeout: 5, "the approval sheet") { window.questionSheet is ApprovalSheet }
        window.endQuestionSheet(as: .cancel)
    }

    /// An export while the deck changed on disk under unsaved edits: the
    /// save is refused, nothing runs, and the sheet says what to do.
    func testAnExportDuringADiskConflictSaysWhatToDo() async throws {
        let (document, window, deck) = try await openSevenSlides()
        _ = try await waitForRunningTap(document)
        let controller = try XCTUnwrap(document.sessionController)
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.exportPDF(slides: 2, recordingTo: record)
        controller.editor.moveCursor(toSlide: 5)
        controller.editor.insertText(" mine", replacementRange: NSRange(location: NSNotFound, length: 0))
        try controller.editor.string.replacingOccurrences(of: " mine", with: " theirs").write(to: deck, atomically: false, encoding: .utf8)
        controller.diskChanged()
        XCTAssertTrue(controller.hasDiskConflict)
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 10, "the failure") { if case .failed = sheet.state { return true } else { return false } }
        XCTAssertEqual(sheet.detailLabel.stringValue, "The deck changed on disk while you have unsaved edits. Choose Load Disk Version or Keep Mine on the bar over the editor, then export again.")
        XCTAssertEqual(exportRuns(record), [], "nothing ran")
        sheet.cancelButton.performClick(nil)
    }

    func testTapsFailureShowsInTheSheet() async throws {
        let (_, window, _) = try await openSevenSlides()
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.write("""
          "export pdf")
            echo '{"phase":"done","ok":false,"error":{"code":"browser","message":"failed to start browser: no network"}}' >&2
            exit 2 ;;
        """, recordingTo: record)
        window.exportPDF(nil)
        let sheet = try await exportSheet(window)
        sheet.exportButton.performClick(nil)
        try await waitUntil(timeout: 10, "the failure") { if case .failed = sheet.state { return true } else { return false } }
        XCTAssertEqual(sheet.statusLabel.stringValue, "Export failed.")
        XCTAssertEqual(sheet.detailLabel.stringValue, "failed to start browser: no network", "tap's own message")
        XCTAssertTrue(sheet.exportButton.isEnabled, "another try")
        sheet.cancelButton.performClick(nil)
    }
}
