import XCTest
@testable import Tap

final class TapLogWindowTests: HostedTestCase {
    func testLogsAndVersion() async throws {
        let document = try await openDeck(try Fixtures.copyAppFixture())
        _ = try await waitForRunningTap(document)
        let appDelegate = try XCTUnwrap(NSApp.delegate as? AppDelegate)

        appDelegate.showTapLog(nil)
        let logWindow = TapLogWindowController.shared
        XCTAssertTrue(logWindow.window?.isVisible ?? false)
        try await waitUntil(timeout: 5, "the log text") { logWindow.textView.string.contains("ready on 127.0.0.1:") }
        XCTAssertTrue(logWindow.textView.string.contains("tap dev --app talk.md"))

        // The window stays live: a line tap writes after it is already open
        // must appear too, not just whatever was in the log at open time.
        // The desktop app keeps tap rendering its editor buffer (PUT
        // /api/app/source), not the file on disk, so breaking the
        // frontmatter through the editor, the way a person would type it,
        // is what makes tap write a fresh line after the window is open.
        let editor = try XCTUnwrap(document.sessionController?.editor)
        editor.setSelectedRange(NSRange(location: 0, length: (editor.string as NSString).length))
        editor.insertText("---\ntitle: [unclosed\n---\n", replacementRange: NSRange(location: NSNotFound, length: 0))
        // Sends the broken buffer to tap directly rather than waiting on the
        // 100ms debounce, which autosave's own one second timer can now
        // outrace on a loaded runner: the app would still write the same
        // broken text to disk and say "saved" first, which logs its own
        // failure to a different line ("error: saved: ...") and leaves this
        // wait needing a PUT that has not gone out yet.
        await document.sessionController?.sourceSync.sendNow()
        try await waitUntil(timeout: 10, "a line appended after the window was already open") {
            logWindow.textView.string.contains("Not showing the buffer:")
        }

        // Each open deck has its own log.
        let second = try await openDeck(try Fixtures.copyDeck("plain.md"))
        _ = try await waitForRunningTap(second)
        logWindow.reload()
        XCTAssertEqual(logWindow.picker.segmentCount, 2)

        // The About window shows the bundled tap version.
        try await waitUntil(timeout: 10, "the tap version") { AppEnvironment.shared.bundledTapVersion != nil }
        let credits = try XCTUnwrap(appDelegate.aboutPanelOptions()[.credits] as? NSAttributedString)
        let appVersion = try XCTUnwrap(Bundle.main.object(forInfoDictionaryKey: "CFBundleShortVersionString") as? String)
        XCTAssertEqual(credits.string, "Bundled tap \(appVersion)")
        logWindow.close()
    }

    /// AppDelegate.showTapLog(_:) is a one-line hand-off: it reads
    /// NSApp.keyWindow and passes that deck's log straight to
    /// TapLogWindowController.show(log:), which is what actually decides
    /// which log is shown. This hosted test host never becomes the active
    /// application (NSApp.activate(ignoringOtherApps: true) leaves
    /// NSApp.isActive false and NSApp.keyWindow nil even after an explicit
    /// makeKeyAndOrderFront, confirmed while writing this test), so the key
    /// window itself cannot be driven from a test here. show(log:) is
    /// exercised directly instead, which is the same call showTapLog makes
    /// and the same line the coverage gap was in: it must select exactly
    /// the log it is given, not whichever deck is first in the picker.
    func testShowSelectsTheGivenLogNotTheFirstOpened() async throws {
        let first = try await openDeck(try Fixtures.copyAppFixture())
        _ = try await waitForRunningTap(first)
        let second = try await openDeck(try Fixtures.copyDeck("plain.md"))
        _ = try await waitForRunningTap(second)
        let logWindow = TapLogWindowController.shared
        let firstLog = try XCTUnwrap(first.sessionController?.session.log)
        let secondLog = try XCTUnwrap(second.sessionController?.session.log)

        // second is not first in open order, so a fallback to index 0
        // would show first's log instead.
        logWindow.show(log: secondLog)
        XCTAssertTrue(logWindow.selectedLog === secondLog)

        // Asking for first again must move the selection back, not leave
        // it on whatever show(log:) picked before.
        logWindow.show(log: firstLog)
        XCTAssertTrue(logWindow.selectedLog === firstLog)

        logWindow.close()
    }

    /// reload()'s fallback to the first log applies when the selected log
    /// goes stale, such as when its deck closes.
    func testReloadFallsBackWhenTheSelectedLogGoesStale() async throws {
        let first = try await openDeck(try Fixtures.copyDeck("plain.md"))
        _ = try await waitForRunningTap(first)
        let second = try await openDeck(try Fixtures.copyAppFixture())
        _ = try await waitForRunningTap(second)
        let logWindow = TapLogWindowController.shared
        let firstLog = try XCTUnwrap(first.sessionController?.session.log)
        let secondLog = try XCTUnwrap(second.sessionController?.session.log)

        logWindow.show(log: secondLog)
        XCTAssertTrue(logWindow.selectedLog === secondLog)

        second.windowControllers.first?.window?.orderOut(nil)
        second.close()
        try await waitUntil(timeout: 10, "the closed deck to drop out of the document list") {
            !NSDocumentController.shared.documents.contains { $0 === second }
        }

        logWindow.reload()
        XCTAssertFalse(logWindow.selectedLog === secondLog, "the stale log is no longer selected")
        XCTAssertTrue(logWindow.selectedLog === firstLog, "reload() falls back to the remaining log")
        XCTAssertEqual(logWindow.picker.segmentCount, 1)

        logWindow.close()
    }

    /// `AppDelegate.showTapLog` resolves the deck to show through
    /// `AppDelegate.deck(owning:)`, which must find the right deck whether
    /// the key window is the deck window itself or its preview detached
    /// into a window of its own (`DeckWindowController.showPreviewInWindow`).
    /// `NSApp.keyWindow` cannot be driven from this hosted test host (see
    /// `testShowSelectsTheGivenLogNotTheFirstOpened` above), so this drives
    /// the resolution function directly with real windows instead; the one
    /// line this leaves uncovered is `showTapLog`'s own
    /// `NSApp.keyWindow` read.
    func testShowTapLogFindsTheDeckOwningADetachedPreviewWindow() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyAppFixture())
        let deckWindowController = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        try await waitForBoxes(document, count: 4)

        XCTAssertTrue(AppDelegate.deck(owning: deckWindowController.window) === deckWindowController)
        XCTAssertNil(AppDelegate.deck(owning: nil))

        deckWindowController.showPreviewInWindow(nil)
        let previewWindow = try XCTUnwrap(deckWindowController.previewWindowController?.window)
        XCTAssertTrue(AppDelegate.deck(owning: previewWindow) === deckWindowController,
                      "the detached preview window must resolve back to the deck it was detached from")

        deckWindowController.dockPreview()
    }
}
