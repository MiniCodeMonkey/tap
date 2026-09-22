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

    func testContinuousEditsWithoutAPauseStillProduceSendsAtRoughlyTheMaxWaitCadence() async throws {
        let sync = SourceSync(delay: 1.0, maxWait: 0.1, text: { [unowned self] in self.text }, beginSend: { [unowned self] in self.generation += 1; return self.generation })
        var sentCount = 0
        sync.sender = { _ in sentCount += 1; return SlideList(slides: [], errors: []) }
        for index in 0..<20 {
            text = "edit\(index)"
            sync.textDidChange()
            try await Task.sleep(nanoseconds: 20_000_000)
        }
        // Twenty edits, twenty milliseconds apart, never leave a gap as long
        // as the one-second debounce delay, so only the max-wait fallback
        // can be producing sends here.
        try await waitUntil(timeout: 2) { sentCount >= 2 }
        XCTAssertGreaterThanOrEqual(sentCount, 2)
    }

    func testABurstFollowedByAPauseStillProducesExactlyOneSend() async throws {
        let sync = SourceSync(delay: 0.05, maxWait: 0.2, text: { [unowned self] in self.text }, beginSend: { [unowned self] in self.generation += 1; return self.generation })
        var sent: [String] = []
        sync.sender = { source in sent.append(source); return SlideList(slides: [], errors: []) }
        for letter in ["a", "ab", "abc"] {
            text = letter
            sync.textDidChange()
            try await Task.sleep(nanoseconds: 5_000_000)
        }
        try await waitUntil(timeout: 2) { sent.count == 1 }
        // Wait past the max-wait window too, to prove the fallback timer was
        // cancelled by the debounce send rather than left to fire a stray
        // second one later.
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(sent, ["abc"])
    }

    func testMaxWaitAndPauseTogetherNeverOverlapSends() async throws {
        let sync = SourceSync(delay: 0.02, maxWait: 0.05, text: { [unowned self] in self.text }, beginSend: { [unowned self] in self.generation += 1; return self.generation })
        var concurrent = 0
        var maxConcurrent = 0
        var sentCount = 0
        sync.sender = { _ in
            concurrent += 1
            maxConcurrent = max(maxConcurrent, concurrent)
            try await Task.sleep(nanoseconds: 80_000_000)
            concurrent -= 1
            sentCount += 1
            return SlideList(slides: [], errors: [])
        }
        for index in 0..<10 {
            text = "edit\(index)"
            sync.textDidChange()
            try await Task.sleep(nanoseconds: 15_000_000)
        }
        try await waitUntil(timeout: 2) { sentCount >= 1 }
        XCTAssertLessThanOrEqual(maxConcurrent, 1)
    }
}
