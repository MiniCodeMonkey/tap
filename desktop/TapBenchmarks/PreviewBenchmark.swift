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
        for index in 0..<20 {
            let before = try XCTUnwrap(controller.previewViewController.lastReady)
            let started = Date()
            _ = send(key: "abcdefghijklmnopqrst".map(String.init)[index], code: 0, to: window)
            try await waitUntil(timeout: 5, "a new revision in the preview") {
                controller.previewViewController.lastReady.map { $0.revision != before.revision } ?? false
            }
            durations.append(Date().timeIntervalSince(started) * 1000)
            try await Task.sleep(nanoseconds: 400_000_000)
        }

        let summary = summarize(durations)
        write(["slides": controller.editor.boxes.count, "keyToPreviewRendered": summary], to: "preview-update")
        XCTAssertLessThan(summary["median"] ?? .greatestFiniteMagnitude, 200, "the preview updates within 200 ms: \(summary)")
    }
}
