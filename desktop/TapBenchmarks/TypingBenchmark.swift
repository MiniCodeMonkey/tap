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
                try await sendAndVerify(String(character), code: character == " " ? 49 : 0,
                                         to: window, controller: controller, expectedLengthDelta: 1, recorder: recorder)
            }
            try await sendAndVerify("\r", code: 36, to: window, controller: controller, expectedLengthDelta: 1, recorder: recorder)
            try await sendAndVerify("x", code: 7, to: window, controller: controller, expectedLengthDelta: 1, recorder: recorder)
            try await sendAndVerify("\u{7f}", code: 51, to: window, controller: controller, expectedLengthDelta: -1, recorder: recorder)
            try await Task.sleep(nanoseconds: 250_000_000 - 35_000_000)
        }
        try await Task.sleep(nanoseconds: 400_000_000)
        recorder.uninstall()

        let summary = summarize(recorder.milliseconds)
        write(["slides": controller.editor.boxes.count, "keyEventToFrameCommitted": summary], to: "typing")
        XCTAssertEqual(recorder.milliseconds.count, 280, "every key event has a sample")
        XCTAssertLessThan(summary["p95"] ?? .greatestFiniteMagnitude, 16, "typing latency stays under 16 ms: \(summary)")
    }

    /// Sends one key event and times it with `recorder`, then checks that the
    /// editor's text changed by the expected amount. The sample opens at
    /// the send and closes at the first frame commit after the keystroke
    /// is in the text, so the 35 ms wait before the check is never part of
    /// it. A key event swallowed before it reaches the responder chain
    /// leaves the text unchanged and its sample open, and this fails
    /// loudly instead of reporting nothing for it: the run loop keeps
    /// committing frames on its own even when nothing was typed, so timing
    /// alone cannot tell the two cases apart.
    private func sendAndVerify(_ characters: String, code: UInt16, to window: NSWindow,
                                controller: DeckSessionController, expectedLengthDelta: Int,
                                recorder: KeyLatencyRecorder) async throws {
        let editor = controller.editor
        let lengthBefore = editor.textStorage?.length ?? 0
        let expectedLength = lengthBefore + expectedLengthDelta
        let timestamp = send(key: characters, code: code, to: window)
        recorder.record(eventTimestamp: timestamp) { editor.textStorage?.length == expectedLength }
        try await Task.sleep(nanoseconds: 35_000_000)
        let lengthAfter = editor.textStorage?.length ?? 0
        guard lengthAfter == expectedLength else {
            XCTFail("key event for \(characters.debugDescription) did not reach the editor: "
                     + "expected the text to change by \(expectedLengthDelta) character(s), "
                     + "went from \(lengthBefore) to \(lengthAfter). Key events may not be "
                     + "reaching the responder chain.")
            throw CancellationError()
        }
        guard recorder.openSampleCount == 0 else {
            XCTFail("key event for \(characters.debugDescription) reached the editor, but no frame "
                     + "was committed after it within 35 ms, so its sample never closed.")
            throw CancellationError()
        }
    }
}
