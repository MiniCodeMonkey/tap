import XCTest
@testable import TapDesktopCore

final class LoginShellEnvironmentTests: XCTestCase {
    func testParsesTheVariablesBetweenTheMarkers() {
        let markers = LoginShellEnvironment.Markers.generate()
        let output = "profile noise\n" + markers.begin + "PATH=/opt/bin:/usr/bin\u{0}MULTI=line one\nline two\u{0}" + markers.end
        XCTAssertEqual(LoginShellEnvironment.parse(Data(output.utf8), markers: markers), ["PATH": "/opt/bin:/usr/bin", "MULTI": "line one\nline two"])
        XCTAssertNil(LoginShellEnvironment.parse(Data("no markers".utf8), markers: markers))
    }

    func testTwoRunsUseDifferentMarkers() {
        let first = LoginShellEnvironment.Markers.generate()
        let second = LoginShellEnvironment.Markers.generate()
        XCTAssertNotEqual(first, second)
        XCTAssertNotEqual(first.begin, second.begin)
        XCTAssertNotEqual(first.end, second.end)
    }

    func testTheTokenIsOnlyLettersAndDigits() {
        let markers = LoginShellEnvironment.Markers.generate()
        XCTAssertFalse(markers.token.isEmpty)
        XCTAssertTrue(markers.token.allSatisfy { $0.isLetter || $0.isNumber })
        // 128 bits of entropy, rendered as lowercase hex: 32 characters.
        XCTAssertEqual(markers.token.count, 32)
    }

    func testFailsCleanlyWhenAnotherMarkerPairAppearsAfterTheRealOne() {
        // A stray marker pair after the real one must not be mistaken for
        // ours just because it uses this run's own token.
        let markers = LoginShellEnvironment.Markers.generate()
        let output = markers.begin + "PATH=/usr/bin" + markers.end + "later noise " + markers.begin + "junk" + markers.end
        XCTAssertNil(LoginShellEnvironment.parse(Data(output.utf8), markers: markers))
    }

    func testFailsCleanlyWhenAnUnmatchedBeginMarkerAppearsAfterTheRealBegin() {
        // A second begin marker with no marker of its own to close it must
        // not be silently absorbed into the real payload.
        let markers = LoginShellEnvironment.Markers.generate()
        let output = markers.begin + "PATH=/usr/bin" + markers.begin + "MORE=1" + markers.end
        XCTAssertNil(LoginShellEnvironment.parse(Data(output.utf8), markers: markers))
    }

    func testFailsCleanlyWhenAVariablesValueContainsMarkerText() {
        // If a variable's own value happens to contain the marker text
        // (here, this run's own markers, the worst realistic case), that is
        // a second occurrence of each marker, and the parse must refuse to
        // guess which pair is real rather than hand back that value as the
        // whole environment.
        let markers = LoginShellEnvironment.Markers.generate()
        let output = markers.begin + "TRAP=" + markers.begin + "fake" + markers.end + "\u{0}PATH=/usr/bin" + markers.end
        XCTAssertNil(LoginShellEnvironment.parse(Data(output.utf8), markers: markers))
    }

    func testReturnsNilWhenTheEndMarkerNeverAppears() {
        let markers = LoginShellEnvironment.Markers.generate()
        let output = "profile noise\n" + markers.begin + "PATH=/usr/bin"
        XCTAssertNil(LoginShellEnvironment.parse(Data(output.utf8), markers: markers))
    }

    func testReturnsNilForARecordThatIsNotKeyEqualsValue() {
        let markers = LoginShellEnvironment.Markers.generate()
        let output = markers.begin + "PATH=/usr/bin\u{0}not-a-record" + markers.end
        XCTAssertNil(LoginShellEnvironment.parse(Data(output.utf8), markers: markers))
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
