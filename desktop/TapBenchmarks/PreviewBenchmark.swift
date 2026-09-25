import XCTest
import WebKit
@testable import Tap

final class PreviewBenchmark: BenchmarkCase {
    /// 13-performance.feature: the preview shows an edit within 200 ms after I stop typing.
    ///
    /// Each sample runs from the key event to the second animation frame
    /// after the page's DOM first holds the edited text, the frame that
    /// shows it. The page's ready signal comes later: it also waits for
    /// every running animation to finish, and an in-place update replays
    /// the theme's entrance animations on the edited slide (the terminal
    /// theme's run for up to 1.5 s). That time is written to the results
    /// as keyToPreviewSettled, and is not what the target is about.
    func testPreviewUpdate() async throws {
        let document = try await openStressDeck()
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        let editor = controller.editor
        let webView = controller.previewViewController.webView
        window.makeFirstResponder(editor)
        editor.moveCursor(toSlide: 99)
        try await waitUntil(timeout: 60, "the preview on slide 100") { controller.previewViewController.lastReady?.slide == 100 }

        // The caret sits at the end of slide 100's heading, so each key
        // extends the heading's text, which the page then shows.
        let heading = "Slide 100: Probe"
        let caret = editor.selectedRange().location
        let text = editor.string as NSString
        XCTAssertEqual(text.substring(with: NSRange(location: caret - heading.utf16.count, length: heading.utf16.count)), heading,
                       "the caret is at the end of slide 100's heading")
        let installed = try await webView.evaluateJavaScript(Self.installShownProbe) as? Bool
        XCTAssertEqual(installed, true, "the page runs the shown probe")

        var shownDurations: [Double] = []
        var settledDurations: [Double] = []
        let lengthBeforeTyping = editor.textStorage?.length ?? 0
        let letters = "abcdefghijklmnopqrst".map(String.init)
        for index in 0..<letters.count {
            let before = try XCTUnwrap(controller.previewViewController.lastReady)
            let expected = heading + letters[0...index].joined()
            _ = try await webView.evaluateJavaScript("window.__benchmarkShownAt = null; window.__benchmarkExpected = \(Self.javaScriptString(expected)); true")

            let started = Date()
            _ = send(key: letters[index], code: 0, to: window)
            // The page showing the heading with this letter is the proof
            // that the key reached the editor and went out to tap.
            let shownAt = try await waitForShown(expected, in: webView, controller: controller)
            shownDurations.append((shownAt - started.timeIntervalSince1970) * 1000)

            try await waitUntil(timeout: 10, "the page's ready signal for the new revision") {
                controller.previewViewController.lastReady.map { $0.revision != before.revision } ?? false
            }
            settledDurations.append(Date().timeIntervalSince(started) * 1000)
            try await Task.sleep(nanoseconds: 400_000_000)
        }
        XCTAssertEqual(editor.textStorage?.length, lengthBeforeTyping + letters.count,
                       "expected all \(letters.count) typed characters to land in the editor")

        let summary = summarize(shownDurations)
        write(["slides": editor.boxes.count,
               "keyToPreviewShown": summary,
               "keyToPreviewSettled": summarize(settledDurations)], to: "preview-update")
        XCTAssertLessThan(summary["median"] ?? .greatestFiniteMagnitude, 200, "the preview shows the edit within 200 ms: \(summary)")
    }

    /// Watches the page's DOM. Once `window.__benchmarkExpected` is set and
    /// the page's text first contains it, the second animation frame after
    /// that change sets `window.__benchmarkShownAt` to the time in
    /// milliseconds since 1970, the clock `Date` reads in the app.
    private static let installShownProbe = """
    (() => {
        window.__benchmarkShownAt = null;
        window.__benchmarkExpected = null;
        if (!window.__benchmarkObserver) {
            window.__benchmarkObserver = new MutationObserver(() => {
                const expected = window.__benchmarkExpected;
                if (expected === null || !(document.body.textContent || '').includes(expected)) return;
                window.__benchmarkExpected = null;
                requestAnimationFrame(() => requestAnimationFrame(() => {
                    window.__benchmarkShownAt = performance.timeOrigin + performance.now();
                }));
            });
            window.__benchmarkObserver.observe(document.body, { subtree: true, childList: true, characterData: true });
        }
        return true;
    })()
    """

    private static func javaScriptString(_ value: String) -> String {
        let data = try? JSONSerialization.data(withJSONObject: [value])
        let array = data.map { String(decoding: $0, as: UTF8.self) } ?? "[\"\"]"
        return String(array.dropFirst().dropLast())
    }

    /// Waits for the shown probe to report `expected` painted, and returns
    /// when, in seconds since 1970.
    private func waitForShown(_ expected: String, in webView: WKWebView, controller: DeckSessionController) async throws -> TimeInterval {
        let deadline = Date().addingTimeInterval(5)
        while Date() < deadline {
            let shownAt = try await webView.evaluateJavaScript("window.__benchmarkShownAt ?? -1") as? Double ?? -1
            if shownAt >= 0 { return shownAt / 1000 }
            try await Task.sleep(nanoseconds: 10_000_000)
        }
        // probeInstalled false, or a young pageMs, means the page was
        // loaded again after the probe went in, so it saw no mutation.
        let state = try? await webView.evaluateJavaScript("""
            JSON.stringify({hidden: document.hidden, ready: window.__tapReady,
                            hasText: (document.body.textContent || '').includes(\(Self.javaScriptString(expected))),
                            probeInstalled: !!window.__benchmarkObserver, pageMs: Math.round(performance.now()),
                            readyState: window.__tapReadyState})
            """)
        XCTFail("the preview never showed \(expected.debugDescription) within 5 s. page=\(String(describing: state)) "
                + "lastReady=\(String(describing: controller.previewViewController.lastReady)) "
                + "previewPageLoads=\(controller.previewViewController.pageLoadCount) "
                + "renderer: \(controller.thumbnails.renderer.stateDescription)")
        throw CancellationError()
    }
}
