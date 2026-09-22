import XCTest
@testable import TapDesktopCore

@MainActor
final class SourceSyncTests: XCTestCase {
    var text = ""
    var generation = 0

    func makeSync() -> SourceSync {
        SourceSync(delay: 0.05, text: { [unowned self] in self.text }, beginSend: { [unowned self] in self.generation += 1; return self.generation })
    }

    func testSendsOnceAfterThePauseWithTheLatestText() async throws {
        let sync = makeSync()
        var sent: [String] = []
        sync.sender = { source in sent.append(source); return SlideList(slides: [], errors: []) }
        var answers: [(String, Int)] = []
        sync.onAnswer = { _, sentText, sentGeneration in answers.append((sentText, sentGeneration)) }
        for letter in ["a", "ab", "abc"] {
            text = letter
            sync.textDidChange()
        }
        try await waitUntil { answers.count == 1 }
        XCTAssertEqual(sent, ["abc"])
        XCTAssertEqual(answers.first?.0, "abc")
        XCTAssertEqual(answers.first?.1, 1)
        XCTAssertEqual(sync.lastSentText, "abc")
    }

    func testAnEditWhileAPutIsInFlightSendsAgainAfterIt() async throws {
        let sync = makeSync()
        var sent: [String] = []
        sync.sender = { [unowned self] source in
            sent.append(source)
            if sent.count == 1 { self.text = "second" }
            try await Task.sleep(nanoseconds: 50_000_000)
            return SlideList(slides: [], errors: [])
        }
        text = "first"
        let first = Task { await sync.sendNow() }
        try await Task.sleep(nanoseconds: 10_000_000)
        await sync.sendNow()
        await first.value
        try await waitUntil { sent.count == 2 }
        XCTAssertEqual(sent, ["first", "second"])
    }

    func testNothingIsSentWithoutASenderAndTheTextGoesOutWhenOneArrives() async throws {
        let sync = makeSync()
        text = "waiting"
        await sync.sendNow()
        var sent: [String] = []
        sync.sender = { source in sent.append(source); return SlideList(slides: [], errors: []) }
        await sync.sendNow()
        XCTAssertEqual(sent, ["waiting"])
    }

    func testAFailureIsReported() async throws {
        let sync = makeSync()
        var failures = 0
        sync.sender = { _ in throw TapErrorPayload(code: "http_500", message: "boom") }
        sync.onFailure = { _ in failures += 1 }
        await sync.sendNow()
        XCTAssertEqual(failures, 1)
    }
}
