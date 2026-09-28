import XCTest
@testable import Tap

final class ImageGenerationSettingsTests: HostedTestCase {
    var settings: SettingsWindowController { SettingsWindowController.shared }

    override func tearDown() async throws {
        settings.window?.orderOut(nil)
        try await super.tearDown()
    }

    func testGeminiKey() async throws {
        let store = MemoryGeminiKeyStore()
        AppEnvironment.shared.geminiKeyStore = store
        // A deck and one tool run, so the log has lines to scan for the key.
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let controller = try XCTUnwrap(document.sessionController)
        (NSApp.delegate as! AppDelegate).showSettings(nil)
        settings.show(pane: .imageGeneration)
        let pane = settings.imageGeneration
        try await waitUntil(timeout: 5, "the pane") { !pane.sourceLabel.stringValue.isEmpty }
        XCTAssertTrue(pane.keyField is NSSecureTextField, "bullets only, never the key (the SettingsImageBullets board)")
        XCTAssertEqual(pane.keyField.stringValue, "")
        XCTAssertEqual(pane.sourceLabel.stringValue, "Stored in your Keychain. GEMINI_API_KEY from your shell takes precedence.")
        XCTAssertTrue(pane.sourceLabel.isDescendant(of: pane.card), "the hint sits inside the card, under API key, as the board draws it")
        XCTAssertTrue(pane.keyField.isEnabled)
        XCTAssertNil(pane.view.subviews.first { $0.accessibilityIdentifier() == "settings-card-Model" }, "no Model row: tap names no model")

        pane.keyField.stringValue = "placeholder-not-a-secret"
        pane.keyChanged(pane.keyField)
        XCTAssertEqual(store.writes.count, 1)
        XCTAssertEqual(store.key, "placeholder-not-a-secret", "written to the store, which is the Keychain in the app")
        let source = await AppEnvironment.shared.geminiKeySource()
        XCTAssertEqual(source, .keychain)
        // The key reaches the image runs alone: never tap dev's environment (the tests' shell value is "").
        let session = await AppEnvironment.shared.sessionConfiguration().environment()
        XCTAssertEqual(session["GEMINI_API_KEY"] ?? "", "")
        XCTAssertNotEqual(session["GEMINI_API_KEY"], "placeholder-not-a-secret")
        _ = await TapTool.run(["theme", "list", "--json"], timeout: 30, log: controller.session.log)
        XCTAssertFalse(controller.session.log.text.contains("placeholder-not-a-secret"), "no log line holds the key")

        // The shell's key wins, and the pane says so instead of editing a key nothing reads.
        AppEnvironment.shared.extraEnvironment["GEMINI_API_KEY"] = "shell-placeholder"
        await pane.refresh()
        XCTAssertFalse(pane.keyField.isEnabled)
        XCTAssertEqual(pane.sourceLabel.stringValue, "Your shell sets GEMINI_API_KEY, so tap uses that key; the Keychain's is not used.")
        let shellSource = await AppEnvironment.shared.geminiKeySource()
        XCTAssertEqual(shellSource, .shell)
        AppEnvironment.shared.extraEnvironment["GEMINI_API_KEY"] = ""

        // Clearing the field removes the key.
        await pane.refresh()
        pane.keyField.stringValue = ""
        pane.keyChanged(pane.keyField)
        XCTAssertNil(store.key)
        let cleared = await AppEnvironment.shared.geminiKeySource()
        XCTAssertEqual(cleared, .none)
        XCTAssertFalse((pane.view.accessibilityChildren() ?? []).description.contains("placeholder-not-a-secret"), "no accessibility value holds the key")
    }
}
