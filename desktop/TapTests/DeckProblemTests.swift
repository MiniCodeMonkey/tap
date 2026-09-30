import XCTest
@testable import Tap

/// A mistake in the deck's settings: which setting, in plain words, the
/// fix, and a preview that does not restart tap for a mistake that stays.
final class DeckProblemTests: HostedTestCase {
    func loadSchema() async throws {
        Task { await AppEnvironment.shared.deckSchema.load() }
        try await waitUntil(timeout: 30, "tap deck schema --json") { AppEnvironment.shared.deckSchema.isLoaded }
    }

    /// Opens the deck whose aspect ratio tap refuses, and waits for the app to hold tap stopped on it.
    func openStoppedOnTheAspectRatio() async throws -> (DeckDocument, DeckSessionController) {
        try await loadSchema()
        let document = try await openDeck(try Fixtures.copyDeck("bad-aspect-ratio.md"))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 30, "tap to stop on the aspect ratio") {
            if case .invalidDeck = controller.session.state { return true } else { return false }
        }
        return (document, controller)
    }

    func startCount(_ controller: DeckSessionController) -> Int {
        controller.session.log.text.components(separatedBy: "tap dev --app").count - 1
    }

    func testABadSettingStopsThePreviewWithOneFix() async throws {
        let (document, controller) = try await openStoppedOnTheAspectRatio()
        let preview = controller.previewViewController
        let card = controller.deckCard
        let editor = controller.editor

        // tap started once. A mistake that stays is not a crash to restart from.
        XCTAssertEqual(startCount(controller), 1)
        try await Task.sleep(nanoseconds: 2_000_000_000)
        guard case .invalidDeck(let reported) = controller.session.state else { return XCTFail("tap was started again: \(controller.session.state)") }
        XCTAssertEqual(reported.map(\.key), ["aspectRatio"], "tap named the setting in its deck-problems event")
        XCTAssertEqual(startCount(controller), 1, "no restart loop for a deterministic mistake")
        XCTAssertTrue(preview.overlay.isHidden, "not the stopped notice: the settings are what is wrong")

        // The preview says which setting, in plain words, with the fix.
        XCTAssertFalse(preview.problemCard.isHidden)
        XCTAssertEqual(preview.problemCard.titleLabel.stringValue, DeckProblems.heading(errorCount: 1))
        let message = try XCTUnwrap(preview.problemCard.problemStack.descendantLabels.first { $0.stringValue.contains("16/9") })
        XCTAssertTrue(message.stringValue.contains("16:9, 4:3 or 16:10"), "the allowed values come from tap: \(message.stringValue)")
        let fixButton = try XCTUnwrap(preview.problemCard.fixButtons["aspectRatio"])
        XCTAssertEqual(fixButton.title, "Use 16:9")
        XCTAssertTrue(preview.problemCard.detailsLabel.stringValue.contains("aspectRatio: 16/9"), "the raw line is in Details")
        XCTAssertTrue(preview.problemCard.detailsLabel.isHidden, "and only behind the disclosure")

        // The Deck card opened by itself, is tinted, marks the field, and its closed line carries the chip.
        XCTAssertTrue(card.isOpen)
        XCTAssertEqual(editor.deckCardTint, .error)
        XCTAssertEqual(card.cardView.chipViews.first?.label.stringValue, "1 problem")
        XCTAssertEqual(card.cardView.chipViews.first?.kind, .problem)
        let fieldFix = try XCTUnwrap(card.form.problemFixButtons["aspectRatio"], "the field carries the fix as well")
        XCTAssertEqual(fieldFix.title, "Use 16:9")
        XCTAssertTrue(try XCTUnwrap(card.form.problemLabels["aspectRatio"]).stringValue.contains("16/9"))
        XCTAssertTrue(controller.slidePanel.isDimmed, "the thumbnails stay, dimmed")

        // Show in Editor opens the card on the field.
        card.setOpen(false)
        preview.problemCard.showInEditorButton.performClick(nil)
        XCTAssertTrue(card.isOpen)
        XCTAssertEqual(card.display, .form)

        // The fix is one edit, one undo step; tap starts once and the preview comes back by itself.
        fixButton.performClick(nil)
        XCTAssertTrue(editor.string.contains("aspectRatio: 16:9"), String(editor.string.prefix(80)))
        XCTAssertEqual(editor.undoManager?.undoActionName, "Use 16:9")
        _ = try await waitForRunningTap(document)
        XCTAssertEqual(startCount(controller), 2, "one start for the fix")
        try await waitUntil(timeout: 30, "the preview to render again") { preview.lastReady != nil && preview.problemCard.isHidden }
        XCTAssertFalse(controller.slidePanel.isDimmed)
        XCTAssertEqual(editor.deckCardTint, .none)
        try await waitForTheUndoStepToClose(editor.undoManager)
        editor.undoManager?.undo()
        XCTAssertTrue(editor.string.contains("aspectRatio: 16/9"), "one undo takes the fix back")
    }

    func testThumbnailsDimWhileThePreviewCannotRender() async throws {
        let (document, controller) = try await openStoppedOnTheAspectRatio()
        XCTAssertTrue(controller.slidePanel.isDimmed)
        XCTAssertLessThan(controller.slidePanel.scrollView.alphaValue, 1)
        XCTAssertFalse(controller.slidePanel.cards.isEmpty, "the thumbnails are dimmed, not gone")
        let fix = try XCTUnwrap(controller.previewViewController.problemCard.fixButtons["aspectRatio"])
        fix.performClick(nil)
        _ = try await waitForRunningTap(document)
        try await waitUntil(timeout: 20, "the thumbnails to come back to full strength") { !controller.slidePanel.isDimmed }
        XCTAssertEqual(controller.slidePanel.scrollView.alphaValue, 1)
    }

    func testAnUnknownThemeStillRenders() async throws {
        try await loadSchema()
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("unknown-theme.md"))
        let controller = try XCTUnwrap(document.sessionController)
        let deckWindow = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        let preview = controller.previewViewController
        XCTAssertNotNil(preview.lastReady, "it renders, in Base")
        XCTAssertTrue(preview.problemCard.isHidden, "an unknown theme is not a stop")

        // A quiet band above the preview.
        try await waitUntil(timeout: 10, "the banner") { !preview.themeBanner.isHidden }
        XCTAssertTrue(preview.themeBanner.messageLabel.stringValue.contains("\u{201C}keynot\u{201D} is not a tap theme."))
        XCTAssertTrue(preview.themeBanner.messageLabel.stringValue.contains("The preview uses Base for now."))
        XCTAssertTrue(preview.themeBanner.fixButton.title.hasPrefix("Use "), preview.themeBanner.fixButton.title)
        XCTAssertFalse(preview.themeBanner.fixButton.isHidden)
        XCTAssertEqual(preview.themeBanner.chooseButton.title, "Choose Theme\u{2026}")
        preview.view.layoutSubtreeIfNeeded()
        XCTAssertLessThanOrEqual(preview.pageContainer.frame.maxY, preview.themeBanner.frame.minY + 0.5, "the page sits below the band")

        // The toolbar's Theme button reads Base, with an amber dot and the reason.
        XCTAssertEqual(deckWindow.themeButton.title, "Base")
        XCTAssertFalse(deckWindow.themeWarningDot.isHidden)
        XCTAssertTrue(deckWindow.themeButton.toolTip?.contains("is not a tap theme") == true, deckWindow.themeButton.toolTip ?? "no tooltip")

        // The closed Deck card shows the name as an amber chip.
        let chips = controller.deckCard.cardView.chipViews
        XCTAssertEqual(chips.first?.label.stringValue, "keynot")
        XCTAssertEqual(chips.first?.kind, .warning)
        XCTAssertFalse(controller.deckCard.isOpen, "a warning does not open the card")

        // The nearest theme is one click and one undo step.
        preview.themeBanner.fixButton.performClick(nil)
        XCTAssertTrue(controller.editor.string.contains("theme: keynote\n"))
        XCTAssertTrue(preview.themeBanner.isHidden)
        XCTAssertTrue(deckWindow.themeWarningDot.isHidden)
    }
}

private extension NSView {
    /// Every text field under this view.
    var descendantLabels: [NSTextField] {
        subviews.flatMap { ($0 as? NSTextField).map { [$0] } ?? [] } + subviews.flatMap(\.descendantLabels)
    }
}
