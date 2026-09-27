import XCTest
@testable import TapDesktopCore

final class ToolRunTests: XCTestCase {
    func testDecodesTheProgressLines() {
        XCTAssertEqual(ProgressLine.decode(line: #"{"phase":"render","done":7,"total":14}"#), .step(phase: "render", done: 7, total: 14))
        XCTAssertEqual(ProgressLine.decode(line: #"{"phase":"download","bytes":67108864,"totalBytes":157286400}"#), .download(bytes: 67_108_864, totalBytes: 157_286_400))
        let done = ProgressLine.decode(line: #"{"phase":"done","ok":true,"output":"/t/talk.pdf","pages":14,"bytes":120000,"brokenSlides":[{"slide":2,"message":"boom"}]}"#)
        guard case .finished(.ok(let payload))? = done else { return XCTFail("not a finished line: \(String(describing: done))") }
        XCTAssertTrue(String(decoding: payload, as: UTF8.self).contains(#""pages":14"#), "the whole done object is the payload")
        XCTAssertEqual(ProgressLine.decode(line: #"{"phase":"done","ok":false,"error":{"code":"interrupted","message":"interrupted"}}"#),
                       .finished(.failed(code: "interrupted", message: "interrupted")))
        XCTAssertNil(ProgressLine.decode(line: "warning: slide 2 shows an error card: boom"), "a plain stderr line is not progress")
        XCTAssertNil(ProgressLine.decode(line: #"{"phase":"render"}"#), "a step without counts is not one")
        XCTAssertNil(ProgressLine.decode(line: ""))
    }

    func testDecodesAJSONResult() throws {
        struct Result: Decodable, Equatable { let deck: String; let theme: String }
        let ok = ToolOutcome.decode(Data(#"{"ok": true, "deck": "/t/talk.md", "theme": "terminal"}"#.utf8))
        XCTAssertEqual(try ok?.result(Result.self), Result(deck: "/t/talk.md", theme: "terminal"))
        let failed = ToolOutcome.decode(Data(#"{"ok": false, "error": {"code": "unknown_theme", "message": "unknown theme \"x\""}}"#.utf8))
        XCTAssertEqual(failed, .failed(code: "unknown_theme", message: "unknown theme \"x\""))
        XCTAssertThrowsError(try failed?.result(Result.self)) { error in
            XCTAssertEqual(error as? ToolError, .failed(code: "unknown_theme", message: "unknown theme \"x\""))
        }
        XCTAssertNil(ToolOutcome.decode(Data("Theme set to terminal in talk.md\n".utf8)), "text is not an outcome")
        XCTAssertNil(ToolOutcome.decode(Data()))
    }

    @MainActor
    func testRunsAScriptAndReportsItsProgress() async throws {
        let script = try Self.script("""
        #!/bin/sh
        echo '{"phase":"render","done":1,"total":2}' >&2
        echo 'warning: slide 2 shows an error card: boom' >&2
        echo '{"phase":"render","done":2,"total":2}' >&2
        echo '{"phase":"done","ok":true,"files":["a.png"]}' >&2
        printf '{"ok": true, "files": ["a.png"]}\\n'
        exit 0
        """)
        var progress: [ProgressLine] = []
        var otherLines: [String] = []
        let run = ToolRun(configuration: .init(executableURL: script, arguments: [], environment: ["PATH": "/usr/bin:/bin"], currentDirectoryURL: nil, timeout: 10))
        run.onProgress = { progress.append($0) }
        run.onStandardErrorLine = { otherLines.append($0) }
        let exit = await run.run()
        XCTAssertEqual(exit.status, 0)
        XCTAssertFalse(exit.cancelled)
        XCTAssertEqual(progress.count, 3)
        XCTAssertEqual(progress.first, .step(phase: "render", done: 1, total: 2))
        XCTAssertEqual(otherLines, ["warning: slide 2 shows an error card: boom"], "a plain line goes to the log, not to progress")
        struct Files: Decodable { let files: [String] }
        XCTAssertEqual(try exit.outcome?.result(Files.self).files, ["a.png"])
        XCTAssertFalse(run.isRunning)
    }

    @MainActor
    func testCancelSendsSIGINTAndWaitsForTheExit() async throws {
        let script = try Self.script("""
        #!/bin/sh
        trap 'echo "{\\"phase\\":\\"done\\",\\"ok\\":false,\\"error\\":{\\"code\\":\\"interrupted\\",\\"message\\":\\"interrupted\\"}}" >&2; exit 130' INT
        echo '{"phase":"render","done":1,"total":30}' >&2
        i=0; while [ $i -lt 300 ]; do sleep 0.1; i=$((i + 1)); done
        exit 0
        """)
        let run = ToolRun(configuration: .init(executableURL: script, arguments: [], environment: ["PATH": "/usr/bin:/bin"], currentDirectoryURL: nil, timeout: 60))
        var sawFirstLine = false
        run.onProgress = { if case .step = $0 { sawFirstLine = true } }
        var exit: ToolRun.Exit?
        run.onExit = { exit = $0 }
        try run.start()
        try await waitUntil(timeout: 5, "the first progress line") { sawFirstLine }
        run.cancel()
        try await waitUntil(timeout: 10, "the exit") { exit != nil }
        XCTAssertEqual(exit?.status, 130)
        XCTAssertEqual(exit?.cancelled, true)
        XCTAssertEqual(exit?.outcome, .failed(code: "interrupted", message: "interrupted"), "tap's own done line is the outcome")
    }

    /// A deadline is Ctrl-C first, so tap closes its browser, then the escalation.
    @MainActor
    func testATimeoutInterruptsThenKillsTheProcess() async throws {
        let script = try Self.script("#!/bin/sh\ntrap 'echo interrupted >&2; exit 130' INT\ni=0; while [ $i -lt 300 ]; do sleep 0.1; i=$((i + 1)); done\n")
        let run = ToolRun(configuration: .init(executableURL: script, arguments: [], environment: ["PATH": "/usr/bin:/bin"], currentDirectoryURL: nil, timeout: 1))
        var lines: [String] = []
        run.onStandardErrorLine = { lines.append($0) }
        let exit = await run.run()
        XCTAssertTrue(exit.timedOut)
        XCTAssertEqual(exit.status, 130, "SIGINT reached the script")
        XCTAssertEqual(lines, ["interrupted"])
        XCTAssertNil(exit.outcome)
        let deaf = try Self.script("#!/bin/sh\ntrap '' INT TERM\ni=0; while [ $i -lt 300 ]; do sleep 0.1; i=$((i + 1)); done\n")
        let stubborn = ToolRun(configuration: .init(executableURL: deaf, arguments: [], environment: ["PATH": "/usr/bin:/bin"], currentDirectoryURL: nil, timeout: 1))
        let killed = await stubborn.run()
        XCTAssertTrue(killed.timedOut)
        XCTAssertNotEqual(killed.status, 0, "SIGKILL after the graces")
    }

    /// Whether the child is gone: exited and reaped (waitpid says so or has
    /// nothing left to say). `kill(pid, 0)` alone reads a zombie as alive.
    static func hasExited(_ identifier: Int32) -> Bool {
        var status: Int32 = 0
        let result = waitpid(identifier, &status, WNOHANG)
        return result == identifier || result == -1
    }

    /// The escalation belongs to the process, not to the run: a run freed
    /// with its window (an export whose deck closed) still kills a tap
    /// that ignores SIGINT and SIGTERM, and nothing keeps the run alive.
    @MainActor
    func testAFreedRunStillEscalatesToSIGKILL() async throws {
        let script = try Self.script("#!/bin/sh\ntrap '' INT TERM\necho started\ni=0; while [ $i -lt 3000 ]; do sleep 0.1; i=$((i + 1)); done\n")
        var run: ToolRun? = ToolRun(configuration: .init(executableURL: script, arguments: [], environment: ["PATH": "/usr/bin:/bin"], currentDirectoryURL: nil, timeout: nil))
        var started = false
        run?.onStandardOutputLine = { if $0 == "started" { started = true } }
        try run?.start()
        try await waitUntil(timeout: 5, "the script") { started }
        let identifier = try XCTUnwrap(run?.processIdentifier)
        weak var freed = run
        run?.cancel()
        run = nil
        try await waitUntil(timeout: 5, "the run to be freed") { freed == nil }
        XCTAssertFalse(ToolRun.activeRuns.contains { $0.processIdentifier == identifier }, "the registry holds runs weakly")
        try await waitUntil(timeout: 10, "SIGKILL after the two graces, then the reap") { Self.hasExited(identifier) }
    }

    /// A run dropped without a cancel while its process runs kills the process from deinit.
    @MainActor
    func testADroppedRunStopsItsProcess() async throws {
        let script = try Self.script("#!/bin/sh\necho started\ni=0; while [ $i -lt 3000 ]; do sleep 0.1; i=$((i + 1)); done\n")
        var run: ToolRun? = ToolRun(configuration: .init(executableURL: script, arguments: [], environment: ["PATH": "/usr/bin:/bin"], currentDirectoryURL: nil, timeout: nil))
        var started = false
        run?.onStandardOutputLine = { if $0 == "started" { started = true } }
        try run?.start()
        try await waitUntil(timeout: 5, "the script") { started }
        let identifier = try XCTUnwrap(run?.processIdentifier)
        run = nil
        try await waitUntil(timeout: 10, "the orphan to be gone") { Self.hasExited(identifier) }
    }

    @MainActor
    func testStopAllCancelsEveryLiveRun() async throws {
        let script = try Self.script("#!/bin/sh\ntrap 'exit 130' INT\ni=0; while [ $i -lt 3000 ]; do sleep 0.1; i=$((i + 1)); done\n")
        let before = ToolRun.activeRuns.count
        let first = ToolRun(configuration: .init(executableURL: script, arguments: [], environment: ["PATH": "/usr/bin:/bin"], currentDirectoryURL: nil, timeout: nil))
        let second = ToolRun(configuration: .init(executableURL: script, arguments: [], environment: ["PATH": "/usr/bin:/bin"], currentDirectoryURL: nil, timeout: nil))
        try first.start()
        try second.start()
        XCTAssertTrue(ToolRun.activeRuns.contains { $0 === first } && ToolRun.activeRuns.contains { $0 === second })
        XCTAssertEqual(ToolRun.activeRuns.count, before + 2)
        ToolRun.stopAll()
        try await waitUntil(timeout: 10, "both exits") { !first.isRunning && !second.isRunning }
        XCTAssertEqual(ToolRun.activeRuns.count, before)
    }

    /// At quit the main queue dies with the app, so stopAll(waiting:) does
    /// not rely on it: it waits for the exits itself and terminates what
    /// ignored SIGINT before returning.
    @MainActor
    func testStopAllWaitingTerminatesWhatIgnoresSIGINT() async throws {
        let script = try Self.script("#!/bin/sh\ntrap '' INT\necho started\ni=0; while [ $i -lt 3000 ]; do sleep 0.1; i=$((i + 1)); done\n")
        let run = ToolRun(configuration: .init(executableURL: script, arguments: [], environment: ["PATH": "/usr/bin:/bin"], currentDirectoryURL: nil, timeout: nil))
        var started = false
        run.onStandardOutputLine = { if $0 == "started" { started = true } }
        try run.start()
        try await waitUntil(timeout: 5, "the script") { started }
        let identifier = run.processIdentifier
        let began = Date()
        ToolRun.stopAll(waiting: 0.5)
        XCTAssertLessThan(Date().timeIntervalSince(began), 3, "the wait is bounded")
        try await waitUntil(timeout: 5, "SIGTERM to have landed") { Self.hasExited(identifier) }
    }

    /// A run that keeps stdin open (tap serve --json) hands the process a
    /// pipe; a cancel closes it first, which is how tap serve learns the
    /// app is done with it, before any signal.
    @MainActor
    func testAKeptStandardInputClosesOnCancel() async throws {
        let script = try Self.script("#!/bin/sh\ntrap '' INT TERM\ncat >/dev/null\necho 'stdin closed'\nexit 0\n")
        let run = ToolRun(configuration: .init(executableURL: script, arguments: [], environment: ["PATH": "/usr/bin:/bin"], currentDirectoryURL: nil, timeout: nil, keepsStandardInputOpen: true))
        var lines: [String] = []
        run.onStandardOutputLine = { lines.append($0) }
        var exit: ToolRun.Exit?
        run.onExit = { exit = $0 }
        try run.start()
        try await Task.sleep(nanoseconds: 300_000_000)
        XCTAssertEqual(lines, [], "the pipe is held open while the run lives")
        run.cancel()
        try await waitUntil(timeout: 5, "the script to see EOF") { lines == ["stdin closed"] }
        try await waitUntil(timeout: 5, "the exit") { exit != nil }
        XCTAssertEqual(exit?.status, 0, "it left on the EOF, before any signal mattered")
    }

    @MainActor
    func testAnExecutableThatDoesNotStartThrows() {
        let run = ToolRun(configuration: .init(executableURL: URL(fileURLWithPath: "/nonexistent/tap"), arguments: [], environment: [:], currentDirectoryURL: nil, timeout: 1))
        XCTAssertThrowsError(try run.start())
        XCTAssertFalse(run.isRunning)
    }

    static func script(_ text: String) throws -> URL {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("tap-core-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        let url = folder.appendingPathComponent("tool")
        try text.write(to: url, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: url.path)
        return url
    }
}
