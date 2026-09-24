import XCTest
@testable import Tap

final class TypingBenchmark: BenchmarkCase {
    /// 13-performance.feature: typing latency stays under 16 ms.
    func testTyping() async throws {
        let document = try await openStressDeck()
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        window.makeFirstResponder(controller.editor)
        controller.editor.moveCursor(toSlide: 99)
        try await Task.sleep(nanoseconds: 800_000_000)

        let recorder = KeyLatencyRecorder()
        recorder.install()
        for _ in 0..<10 {
            for character in "the quick brown fox jumps" {
                recorder.record(eventTimestamp: send(key: String(character), code: character == " " ? 49 : 0, to: window))
                try await Task.sleep(nanoseconds: 35_000_000)
            }
            recorder.record(eventTimestamp: send(key: "\r", code: 36, to: window))
            try await Task.sleep(nanoseconds: 35_000_000)
            recorder.record(eventTimestamp: send(key: "x", code: 7, to: window))
            try await Task.sleep(nanoseconds: 35_000_000)
            recorder.record(eventTimestamp: send(key: "\u{7f}", code: 51, to: window))
            try await Task.sleep(nanoseconds: 250_000_000)
        }
        try await Task.sleep(nanoseconds: 400_000_000)
        recorder.uninstall()

        let summary = summarize(recorder.milliseconds)
        write(["slides": controller.editor.boxes.count, "keyEventToFrameCommitted": summary], to: "typing")
        XCTAssertGreaterThan(summary["count"] ?? 0, 200)
        XCTAssertLessThan(summary["p95"] ?? .greatestFiniteMagnitude, 16, "typing latency stays under 16 ms: \(summary)")
    }
}
