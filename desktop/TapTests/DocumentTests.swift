import XCTest
@testable import Tap

// `Fixtures.copyAppFixture()` copies `internal/cli/testdata/app`, which does
// not exist on this branch: it is testdata for the real `tap --app` (Task
// 13, on a parallel branch that has not landed). These tests use
// `desktop/TapTests/Fixtures/app.md` instead, a small deck of our own that
// does not depend on that fixture landing first.
final class DocumentTests: HostedTestCase {
    func commandLine(of processIdentifier: Int32) -> String {
        let process = Process()
        process.executableURL = URL(fileURLWithPath: "/bin/ps")
        process.arguments = ["-o", "args=", "-p", "\(processIdentifier)"]
        let output = Pipe()
        process.standardOutput = output
        try? process.run()
        process.waitUntilExit()
        return String(decoding: output.fileHandleForReading.readDataToEndOfFile(), as: UTF8.self)
    }

    func testOpenADeck() async throws {
        let deck = try Fixtures.copyDeck("app.md")
        let document = try await openDeck(deck)
        let window = try XCTUnwrap(document.windowControllers.first?.window)
        XCTAssertTrue(window.isVisible)
        _ = try await waitForRunningTap(document)
        let processIdentifier = try XCTUnwrap(document.sessionController?.session.processIdentifier)
        let arguments = commandLine(of: processIdentifier)
        XCTAssertTrue(arguments.contains("dev --app \(deck.path)"), arguments)
        XCTAssertEqual(NSDocumentController.shared.documents.count, 1, "one tap process for one document")

        // A second deck opens in a new tab of the open window.
        let second = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let secondWindow = try XCTUnwrap(second.windowControllers.first?.window)
        XCTAssertTrue(window.tabbedWindows?.contains(secondWindow) ?? false)
    }

    func testOpenTheSameDeckTwice() async throws {
        let deck = try Fixtures.copyDeck("app.md")
        let first = try await openDeck(deck)
        let again = try await openDeck(deck)
        XCTAssertTrue(first === again)
        XCTAssertEqual(NSDocumentController.shared.documents.count, 1)
        XCTAssertEqual(first.windowControllers.count, 1)
        XCTAssertTrue(first.windowControllers.first?.window?.isVisible ?? false)
    }

    func testCloseAWindow() async throws {
        let document = try await openDeck(try Fixtures.copyDeck("app.md"))
        _ = try await waitForRunningTap(document)
        let processIdentifier = try XCTUnwrap(document.sessionController?.session.processIdentifier)
        document.windowControllers.first?.window?.performClose(nil)
        try await waitUntil(timeout: 10, "tap to exit") { kill(processIdentifier, 0) != 0 }
        XCTAssertTrue(NSDocumentController.shared.documents.isEmpty)
    }
}
