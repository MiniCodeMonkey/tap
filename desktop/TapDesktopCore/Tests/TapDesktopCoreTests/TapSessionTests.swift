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
        // The restarting state lasts only the policy's base delay, so every
        // state is recorded as it happens rather than polled for.
        var states: [TapSession.State] = []
        tap.onStateChange = { states.append($0) }
        kill(first, SIGKILL)
        try await waitUntil {
            guard let restart = states.firstIndex(where: { if case .restarting = $0 { return true } else { return false } }) else { return false }
            return states[restart...].contains { if case .running = $0 { return true } else { return false } }
        }
        XCTAssertNotEqual(tap.processIdentifier, first)
        tap.stop()
    }

    func testRestartStartsANewTapWithoutCountingAnExit() async throws {
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = session(try FakeTap.ready(recordingTo: record))
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        let first = try XCTUnwrap(tap.processIdentifier)
        var states: [TapSession.State] = []
        tap.onStateChange = { states.append($0) }
        tap.restart()
        try await waitUntil { states.contains { if case .running = $0 { return true } else { return false } } }
        XCTAssertNotEqual(tap.processIdentifier, first)
        XCTAssertFalse(states.contains { if case .restarting = $0 { return true } else { return false } }, "a restart asked for is not an unexpected exit")
        XCTAssertTrue(tap.log.text.contains("restarting tap"))
        tap.stop()
    }

    func testTryAgainRestartsARunningTap() async throws {
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = session(try FakeTap.ready(recordingTo: record))
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        let first = try XCTUnwrap(tap.processIdentifier)
        tap.tryAgain()
        try await waitUntil { if case .running = tap.state, tap.processIdentifier != first { return true } else { return false } }
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

    func testATalkRunsTapPresentWithItsFlagsAndHasItsOwnLogTitle() async throws {
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = TapSession(deckURL: deckURL, configuration: TapSession.Configuration(
            executableURL: try FakeTap.ready(recordingTo: record), environment: { ["PATH": "/usr/bin:/bin"] }),
            command: .present(record: false, presenterPassword: "secret", port: 4242))
        XCTAssertEqual(tap.command.arguments(deck: deckURL), ["present", "--app", "--no-record", "--presenter-password", "secret", "--port", "4242", deckURL.path])
        XCTAssertEqual(tap.command.port, 4242)
        XCTAssertEqual(TapSession.Command.present(record: true, presenterPassword: nil, port: nil).arguments(deck: deckURL), ["present", "--app", deckURL.path])
        XCTAssertEqual(TapSession.Command.present(record: true, presenterPassword: "", port: nil).arguments(deck: deckURL), ["present", "--app", deckURL.path], "an empty password is no password")
        XCTAssertEqual(TapSession.Command.dev.arguments(deck: deckURL), ["dev", "--app", deckURL.path])
        XCTAssertNil(TapSession.Command.dev.port)
        XCTAssertEqual(tap.log.title, "talk, talk")
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertTrue(recorded.contains("arguments: present --app --no-record --presenter-password secret --port 4242 \(deckURL.path)"))
        XCTAssertTrue(tap.log.text.contains("tap present --app --no-record --presenter-password *** --port 4242 talk.md"), "the log hides the password")
        XCTAssertFalse(tap.log.text.contains("secret"))
        tap.stop()
        try await waitUntil { tap.state == .stopped }
    }

    func testQuitSendsTheQuitCommandAndNeverRestarts() async throws {
        // A tap that exits on quit, as tap present does once its recording is finished.
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let script = try TestScripts.make("""
        echo '{"type":"ready","port":4242,"token":"token","launch":"launch","presenter":"p"}'
        while IFS= read -r line; do
          echo "stdin: $line" >> "\(record.path)"
          case "$line" in *'"type":"quit"'*) exit 0 ;; esac
        done
        exit 0
        """)
        let tap = session(script)
        var states: [TapSession.State] = []
        tap.onStateChange = { states.append($0) }
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        tap.quit()
        XCTAssertTrue(tap.quitRequested)
        try await waitUntil { tap.state == .stopped }
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertTrue(recorded.contains(#"stdin: {"type":"quit"}"#))
        XCTAssertFalse(states.contains { if case .restarting = $0 { return true } else { return false } }, "an exit after quit is not a crash")
        XCTAssertTrue(tap.log.text.contains("tap quit"))
    }

    func testQuitClosesStdinWhenTapIgnoresTheCommand() async throws {
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = session(try FakeTap.ready(recordingTo: record))
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        // FakeTap.ready reads stdin until it closes and never acts on quit.
        tap.quit(timeout: 0.3)
        try await waitUntil(timeout: 5) { tap.state == .stopped }
        XCTAssertTrue(tap.log.text.contains("did not quit within 0.3 seconds"))
    }

    func testExtendQuitMovesTheDeadline() async throws {
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = session(try FakeTap.ready(recordingTo: record))
        tap.start()
        try await waitUntil { if case .running = tap.state { return true } else { return false } }
        tap.quit(timeout: 0.3)
        // A keep-recording question would move the deadline past tap's wait; here the test moves it.
        tap.extendQuit(timeout: 1.5)
        try await Task.sleep(nanoseconds: 800_000_000)
        if case .running = tap.state {} else { XCTFail("the first deadline no longer applies") }
        try await waitUntil(timeout: 5) { tap.state == .stopped }
        XCTAssertTrue(tap.log.text.contains("did not quit within 1.5 seconds"))
        XCTAssertFalse(tap.log.text.contains("within 0.3 seconds"))
    }

    func testQuitBeforeTheProcessExistsStopsAtOnce() async throws {
        // The environment closure is still running when quit arrives: no process is ever launched.
        let record = try TestScripts.temporaryFolder().appendingPathComponent("record")
        let tap = TapSession(deckURL: deckURL, configuration: TapSession.Configuration(
            executableURL: try FakeTap.ready(recordingTo: record),
            environment: { try? await Task.sleep(nanoseconds: 500_000_000); return ["PATH": "/usr/bin:/bin"] }))
        tap.start()
        XCTAssertEqual(tap.state, .starting)
        tap.quit()
        XCTAssertEqual(tap.state, .stopped)
        try await Task.sleep(nanoseconds: 800_000_000)
        XCTAssertEqual(tap.state, .stopped, "the environment arriving after quit launches nothing")
        XCTAssertNil(tap.processIdentifier)
        XCTAssertFalse(FileManager.default.fileExists(atPath: record.path), "the fake never ran")
    }
}
