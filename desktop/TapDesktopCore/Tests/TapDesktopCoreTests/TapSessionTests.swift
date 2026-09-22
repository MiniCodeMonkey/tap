import XCTest
@testable import TapDesktopCore

@MainActor
final class TapSessionTests: XCTestCase {
    var deckURL: URL!

    override func setUp() async throws {
        let folder = try TestScripts.temporaryFolder()
        deckURL = folder.appendingPathComponent("talk.md")
        try "# One\n".write(to: deckURL, atomically: true, encoding: .utf8)
    }

    func session(_ executable: URL, environment: [String: String] = ["PATH": "/usr/bin:/bin"],
                 policy: RestartPolicy = RestartPolicy(baseDelay: 0.05), readyTimeout: TimeInterval = 20) -> TapSession {
        TapSession(deckURL: deckURL, configuration: TapSession.Configuration(
            executableURL: executable, environment: { environment }, readyTimeout: readyTimeout, policy: policy))
    }

    func testStartsTapDevAppAndReadsTheReadyLine() async throws {
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = session(try FakeTap.ready(recordingTo: record))
        var states: [TapSession.State] = []
        tap.onStateChange = { states.append($0) }
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        XCTAssertEqual(states.first, .starting)
        XCTAssertEqual(tap.state, .running(TapReady(port: 4242, token: "token", launch: "launch")))
        XCTAssertTrue(try String(contentsOf: record, encoding: .utf8).contains("arguments: dev --app \(deckURL.path)"))
        XCTAssertTrue(tap.log.text.contains("tap dev --app talk.md"))
        XCTAssertTrue(tap.log.text.contains("ready on 127.0.0.1:4242"))
        XCTAssertTrue(tap.log.text.contains("listening on 127.0.0.1:4242"))

        tap.send(.saved)
        tap.stop()
        try await waitUntil { tap.state == .stopped }
        XCTAssertTrue(try String(contentsOf: record, encoding: .utf8).contains(#"stdin: {"type":"saved"}"#))
    }

    func testLoginShellEnvironment() async throws {
        // The app runs the login shell once, with a timeout...
        let counter = try TestScripts.temporaryFolder().appendingPathComponent("runs")
        let shell = try TestScripts.make("""
        echo run >> "\(counter.path)"
        export FROM_LOGIN_SHELL=yes
        exec /bin/sh -c "$4"
        """)
        let loader = LoginShellEnvironmentLoader(shellPath: shell.path, timeout: 5, fallback: [:])

        // ...and every tap process gets that environment.
        let firstRecord = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let secondRecord = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let sessions = [try FakeTap.ready(recordingTo: firstRecord), try FakeTap.ready(recordingTo: secondRecord)].map { executable in
            TapSession(deckURL: deckURL, configuration: TapSession.Configuration(
                executableURL: executable, environment: { await loader.environment().variables }))
        }
        sessions.forEach { $0.start() }
        try await waitUntil { sessions.allSatisfy { if case .running = $0.state { return true } else { return false } } }
        for record in [firstRecord, secondRecord] {
            XCTAssertTrue(try String(contentsOf: record, encoding: .utf8).contains("FROM_LOGIN_SHELL=yes"))
        }
        XCTAssertEqual(try String(contentsOf: counter, encoding: .utf8), "run\n")
        sessions.forEach { $0.stop() }

        // When the shell hangs, tap runs with the default environment and a notice.
        let hanging = LoginShellEnvironmentLoader(shellPath: try TestScripts.make("sleep 30").path, timeout: 0.3, fallback: ["PATH": "/usr/bin:/bin"])
        let fallback = await hanging.environment()
        XCTAssertEqual(fallback.variables, ["PATH": "/usr/bin:/bin"])
        XCTAssertNotNil(fallback.notice)
    }

    func testRestartsAfterAnUnexpectedExitAndGivesUpAtTheThirdWithinThirtySeconds() async throws {
        let tap = session(try FakeTap.crashing())
        var states: [TapSession.State] = []
        tap.onStateChange = { states.append($0) }
        tap.start()
        try await waitUntil { if case .failed = tap.state { return true } else { return false } }
        XCTAssertEqual(states.filter { if case .restarting = $0 { return true } else { return false } },
                       [.restarting(after: 0.05), .restarting(after: 0.1)])
        guard case .failed(let lastOutput) = tap.state else { return XCTFail() }
        XCTAssertEqual(lastOutput.suffix(2), ["panic: runtime error: index out of range", "internal/parser/slots.go:88"])
        XCTAssertTrue(tap.log.text.contains("tap exited 3 times in 30 seconds"))
    }

    func testTryAgainStartsOver() async throws {
        let tap = session(try FakeTap.crashing())
        tap.start()
        try await waitUntil { if case .failed = tap.state { return true } else { return false } }
        var restarts = 0
        tap.onStateChange = { if case .restarting = $0 { restarts += 1 } }
        tap.tryAgain()
        try await waitUntil { if case .failed = tap.state { return true } else { return false } }
        XCTAssertEqual(restarts, 2, "Try Again resets the three-exit count")
    }

    func testAKilledTapComesBackOnANewProcess() async throws {
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = session(try FakeTap.ready(recordingTo: record))
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        let first = try XCTUnwrap(tap.processIdentifier)
        kill(first, SIGKILL)
        try await waitUntil { if case .restarting = tap.state { return true } else { return false } }
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        XCTAssertNotEqual(tap.processIdentifier, first)
        tap.stop()
    }

    func testATapThatNeverGetsReadyIsKilledAndRetried() async throws {
        let tap = session(try FakeTap.silent(), readyTimeout: 0.2)
        tap.start()
        try await waitUntil(timeout: 5) { if case .failed = tap.state { return true } else { return false } }
        XCTAssertTrue(tap.log.text.contains("did not print its ready line"))
    }

    func testAnErrorEventIsLogged() async throws {
        let tap = session(try FakeTap.failing(code: "deck_not_found", message: "deck not found: talk.md"))
        var events: [TapEvent] = []
        tap.onEvent = { events.append($0) }
        tap.start()
        try await waitUntil { if case .failed = tap.state { return true } else { return false } }
        XCTAssertTrue(events.contains(.error(TapErrorPayload(code: "deck_not_found", message: "deck not found: talk.md"))))
        XCTAssertTrue(tap.log.text.contains("deck not found: talk.md"))
    }

    func testChangingTheDeckRestartsTapOnTheNewPath() async throws {
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = session(try FakeTap.ready(recordingTo: record))
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        let renamed = deckURL.deletingLastPathComponent().appendingPathComponent("renamed.md")
        tap.changeDeck(to: renamed)
        try await waitUntil { (try? String(contentsOf: record, encoding: .utf8))?.contains("arguments: dev --app \(renamed.path)") == true }
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        XCTAssertEqual(tap.log.title, "renamed")
        tap.stop()
    }

    func testNoRestartWhileRestartsArePaused() async throws {
        let tap = session(try FakeTap.crashing())
        tap.restartsWhenExited = false
        tap.start()
        try await waitUntil { tap.state == .stopped }
        XCTAssertTrue(tap.log.text.contains("tap exited with status 2"))
    }
}
