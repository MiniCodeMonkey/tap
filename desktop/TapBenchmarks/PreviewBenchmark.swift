import XCTest
@testable import Tap

final class PreviewBenchmark: BenchmarkCase {
    /// 13-performance.feature: the preview shows an edit within 200 ms after I stop typing.
    func testPreviewUpdate() async throws {
        let document = try await openStressDeck()
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        window.makeFirstResponder(controller.editor)
        controller.editor.moveCursor(toSlide: 99)
        try await waitUntil(timeout: 60, "the preview on slide 100") { controller.previewViewController.lastReady?.slide == 100 }

        var durations: [Double] = []
        let textBeforeTyping = controller.editor.string
        for index in 0..<20 {
            let before = try XCTUnwrap(controller.previewViewController.lastReady)
            let character = "abcdefghijklmnopqrst".map(String.init)[index]
            let textBeforeKey = controller.editor.string
            let started = Date()
            _ = send(key: character, code: 0, to: window)
            try await waitUntil(timeout: 5, "a new revision in the preview") {
                controller.previewViewController.lastReady.map { $0.revision != before.revision } ?? false
            }
            durations.append(Date().timeIntervalSince(started) * 1000)
            // A revision change proves tap re-rendered something, but not that it
            // re-rendered this key: confirm the character actually landed in the
            // editor, the same defect the typing benchmark guards against.
            guard controller.editor.string.count == textBeforeKey.count + 1 else {
                XCTFail("key event for \(character.debugDescription) did not reach the editor even "
                         + "though the preview reported a new revision; the render may have been "
                         + "triggered by something other than this key.")
                throw CancellationError()
            }
            try await Task.sleep(nanoseconds: 400_000_000)
        }
        XCTAssertEqual(controller.editor.string.count, textBeforeTyping.count + 20,
                        "expected all 20 typed characters to land in the editor")

        let summary = summarize(durations)
        write(["slides": controller.editor.boxes.count, "keyToPreviewRendered": summary], to: "preview-update")
        XCTAssertLessThan(summary["median"] ?? .greatestFiniteMagnitude, 200, "the preview updates within 200 ms: \(summary)")
    }
}
