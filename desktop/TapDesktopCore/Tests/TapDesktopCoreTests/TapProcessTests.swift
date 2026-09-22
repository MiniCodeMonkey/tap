import XCTest
@testable import TapDesktopCore

@MainActor
final class TapProcessTests: XCTestCase {
    func configuration(_ script: URL, folder: URL) -> TapProcess.Configuration {
        TapProcess.Configuration(executableURL: script, arguments: ["dev", "--app", folder.appendingPathComponent("talk.md").path],
                                 environment: ["PATH": "/usr/bin:/bin", "MARKER": "from the app"], currentDirectoryURL: folder)
    }

    func testReportsOutputAndErrorLinesAndExitsWhenStdinCloses() async throws {
        let folder = try TestScripts.temporaryFolder()
        let received = folder.appendingPathComponent("stdin")
        let script = try TestScripts.make("""
        echo "$@ $MARKER $(pwd -P)"
        echo '{"type":"ready","port":4242,"token":"t","launch":"l"}'
        echo "listening" >&2
        cat > "\(received.path)"
        exit 0
        """)
        let process = TapProcess(configuration: configuration(script, folder: folder))
        var output: [String] = []
        var errors: [String] = []
        var exit: (Int32, Bool)?
        process.onOutputLine = { output.append($0) }
        process.onStandardErrorLine = { errors.append($0) }
        process.onExit = { exit = ($0, $1) }
        try process.start()
        XCTAssertTrue(process.isRunning)
        try await waitUntil { output.count == 2 && errors == ["listening"] }
        XCTAssertEqual(output[0], "dev --app \(folder.appendingPathComponent("talk.md").path) from the app \(realPath(of: folder))")

        process.send(.saved)
        process.stop()
        try await waitUntil { exit != nil }
        XCTAssertEqual(exit?.0, 0)
        XCTAssertEqual(exit?.1, true, "a stop the app asked for is requested")
        XCTAssertFalse(process.isRunning)
        XCTAssertEqual(try String(contentsOf: received, encoding: .utf8), #"{"type":"saved"}"# + "\n")
    }

    func testAnExitTheAppDidNotAskForIsUnexpected() async throws {
        let folder = try TestScripts.temporaryFolder()
        let script = try TestScripts.make("echo 'panic: boom' >&2\nexit 2")
        let process = TapProcess(configuration: configuration(script, folder: folder))
        var errors: [String] = []
        var exit: (Int32, Bool)?
        process.onStandardErrorLine = { errors.append($0) }
        process.onExit = { exit = ($0, $1) }
        try process.start()
        try await waitUntil { exit != nil }
        XCTAssertEqual(exit?.0, 2)
        XCTAssertEqual(exit?.1, false)
        XCTAssertEqual(errors, ["panic: boom"], "output that arrives just before the exit is not lost")
    }

    func testStopEscalatesWhenTapIgnoresEndOfFile() async throws {
        let folder = try TestScripts.temporaryFolder()
        let script = try TestScripts.make("trap '' TERM\nwhile true; do sleep 0.1; done")
        let process = TapProcess(configuration: configuration(script, folder: folder))
        var exited = false
        process.onExit = { _, _ in exited = true }
        try process.start()
        process.stop(graceSeconds: 0.2)
        try await waitUntil(timeout: 5) { exited }
    }

    func testSendingToAProcessThatExitedDoesNotCrash() async throws {
        let folder = try TestScripts.temporaryFolder()
        let script = try TestScripts.make("exit 0")
        let process = TapProcess(configuration: configuration(script, folder: folder))
        var exited = false
        process.onExit = { _, _ in exited = true }
        try process.start()
        try await waitUntil { exited }
        process.send(.reload)
    }
}
