import Foundation
import XCTest
@testable import Tap

/// The round trip every slide operation is checked with: the buffer is
/// written to a temporary deck and the bundled `tap slide list --json`
/// parses it, so the order and the ranges come from tap, never from Swift.
enum TapSlideList {
    static func list(text: String) async throws -> SlideList {
        let folder = try Fixtures.temporaryFolder()
        let file = folder.appendingPathComponent("roundtrip.md")
        try text.write(to: file, atomically: true, encoding: .utf8)
        let executable = await MainActor.run { AppEnvironment.shared.tapExecutableURL }
        return try await withCheckedThrowingContinuation { continuation in
            DispatchQueue.global().async {
                let process = Process()
                process.executableURL = executable
                process.arguments = ["slide", "list", file.path, "--json"]
                let output = Pipe()
                process.standardOutput = output
                process.standardError = FileHandle.nullDevice
                do {
                    try process.run()
                } catch {
                    return continuation.resume(throwing: error)
                }
                // Read before waiting: a slide list longer than a pipe would block the exit.
                let data = output.fileHandleForReading.readDataToEndOfFile()
                process.waitUntilExit()
                do {
                    continuation.resume(returning: try SlideList.decodeResponse(data))
                } catch {
                    continuation.resume(throwing: error)
                }
            }
        }
    }

    /// Exactly one "---" line in every gap between two slides, and the
    /// frontmatter's two before slide 1. Lines inside a slide's range are
    /// the slide's own (a fence can hold a "---"), so they are never counted.
    static func assertOneSeparatorBetweenSlides(_ text: String, list: SlideList, frontmatterSeparators: Int = 2, file: StaticString = #filePath, line: UInt = #line) {
        let lines = text.components(separatedBy: "\n")
        func separators(fromLine start: Int, toLine end: Int) -> Int {
            guard start <= end, start >= 1 else { return 0 }
            return lines[(start - 1)..<min(end, lines.count)].filter { $0.trimmingCharacters(in: .whitespaces) == "---" }.count
        }
        let slides = list.slides
        guard let first = slides.first else { return }
        XCTAssertEqual(separators(fromLine: 1, toLine: first.startLine - 1), frontmatterSeparators, "only the frontmatter's separators before slide 1", file: file, line: line)
        for (previous, next) in zip(slides, slides.dropFirst()) {
            XCTAssertEqual(separators(fromLine: previous.endLine + 1, toLine: next.startLine - 1), 1,
                           "one --- between slide \(previous.number) and slide \(next.number)", file: file, line: line)
        }
        XCTAssertEqual(separators(fromLine: slides[slides.count - 1].endLine + 1, toLine: lines.count), 0, "nothing after the last slide", file: file, line: line)
    }
}
