import XCTest
@testable import TapDesktopCore

final class LoginShellEnvironmentTests: XCTestCase {
    func testParsesTheVariablesBetweenTheMarkers() {
        let output = "profile noise\n" + LoginShellEnvironment.beginMarker + "PATH=/opt/bin:/usr/bin\u{0}MULTI=line one\nline two\u{0}" + LoginShellEnvironment.endMarker
        XCTAssertEqual(LoginShellEnvironment.parse(Data(output.utf8)), ["PATH": "/opt/bin:/usr/bin", "MULTI": "line one\nline two"])
        XCTAssertNil(LoginShellEnvironment.parse(Data("no markers".utf8)))
    }

    func testLoadsFromANoisyShell() async throws {
        let shell = try TestScripts.make("""
        echo "welcome to a noisy profile"
        export FROM_LOGIN_SHELL=yes
        exec /bin/sh -c "$4"
        """)
        let environment = await LoginShellEnvironment.load(shellPath: shell.path, timeout: 5, fallback: ["PATH": "/usr/bin"])
        XCTAssertEqual(environment.outcome, .loaded)
        XCTAssertEqual(environment.variables["FROM_LOGIN_SHELL"], "yes")
        XCTAssertNil(environment.notice)
    }

    func testFallsBackWithANoticeWhenTheShellHangs() async throws {
        let shell = try TestScripts.make("sleep 30")
        let started = Date()
        let environment = await LoginShellEnvironment.load(shellPath: shell.path, timeout: 0.5, fallback: ["PATH": "/usr/bin"])
        XCTAssertLessThan(Date().timeIntervalSince(started), 3)
        XCTAssertEqual(environment.variables, ["PATH": "/usr/bin"])
        XCTAssertNotNil(environment.notice)
    }

    func testFallsBackWhenTheShellFails() async throws {
        let shell = try TestScripts.make("exit 3")
        let environment = await LoginShellEnvironment.load(shellPath: shell.path, timeout: 5, fallback: ["PATH": "/usr/bin"])
        XCTAssertEqual(environment.variables, ["PATH": "/usr/bin"])
        if case .failed = environment.outcome {} else { XCTFail("expected a failure, got \(environment.outcome)") }
    }

    func testTheLoaderRunsTheShellOnce() async throws {
        let counter = try TestScripts.temporaryFolder().appendingPathComponent("runs")
        let shell = try TestScripts.make("""
        echo run >> "\(counter.path)"
        exec /bin/sh -c "$4"
        """)
        let loader = LoginShellEnvironmentLoader(shellPath: shell.path, timeout: 5, fallback: [:])
        async let first = loader.environment()
        async let second = loader.environment()
        _ = await (first, second)
        _ = await loader.environment()
        XCTAssertEqual(try String(contentsOf: counter, encoding: .utf8), "run\n")
    }
}
