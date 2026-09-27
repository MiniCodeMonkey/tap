import XCTest
@testable import Tap

final class GenerateImageTests: HostedTestCase {
    /// A scripted tap image generate: appends tap's pair to the deck file
    /// (the end of the last slide) and prints tap's result. It records
    /// whether GEMINI_API_KEY was set, never its value.
    func fakeGenerate(recordingTo record: URL) throws -> URL {
        try FakeToolScripts.write("""
          "image generate")
            deck="$3"
            printf '\\n<!-- ai-prompt: a fox at dusk -->\\n![](images/generated-1a2b3c4d.png)\\n' >> "$deck"
            printf '{"ok": true, "deck": "%s", "slide": 7, "image": "images/generated-1a2b3c4d.png", "prompt": "a fox at dusk", "markdown": "<!-- ai-prompt: a fox at dusk -->\\\\n![](images/generated-1a2b3c4d.png)"}\\n' "$deck"
            exit 0 ;;
          "image regenerate")
            deck="$3"
            sed -i '' 's/generated-00000000/generated-ffffffff/' "$deck"
            printf '{"ok": true, "deck": "%s", "slide": 2, "image": "images/generated-ffffffff.png", "prompt": "a fox at dusk", "markdown": "m", "replaced": "images/generated-00000000.png"}\\n' "$deck"
            exit 0 ;;
        """, recordingTo: record)
    }

    func testGenerateAnImageWithAI() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try fakeGenerate(recordingTo: record)
        AppEnvironment.shared.geminiKeyStore = MemoryGeminiKeyStore(key: "placeholder-not-a-secret")
        let deck = try Fixtures.copyDeck("seven-slides.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        let deckPath = try XCTUnwrap(document.fileURL?.path)
        controller.jumpToSlide(number: 7)

        window.generateImage(nil)
        try await waitUntil(timeout: 5, "the sheet") { window.window?.attachedSheet is GenerateImageSheet }
        let sheet = try XCTUnwrap(window.window?.attachedSheet as? GenerateImageSheet)
        XCTAssertEqual(sheet.titleLabel.stringValue, "Generate Image for Slide 7")
        XCTAssertFalse(sheet.generateButton.isEnabled, "no prompt yet")
        sheet.promptView.string = "a fox at dusk"
        sheet.textDidChange(Notification(name: NSText.didChangeNotification, object: sheet.promptView))
        XCTAssertTrue(sheet.generateButton.isEnabled)
        XCTAssertEqual(sheet.aspectControl.label(forSegment: sheet.aspectControl.selectedSegment), "16:9")
        XCTAssertEqual(sheet.matchThemeSwitch.state, .on)
        XCTAssertEqual(sheet.request.arguments(deck: URL(fileURLWithPath: deckPath), slide: 7),
                       ["image", "generate", deckPath, "--slide", "7", "--prompt", "a fox at dusk", "--aspect", "16:9", "--match-theme", "--json"])
        sheet.generateButton.performClick(nil)

        try await waitUntil(timeout: 20, "tap's edit to land") { controller.editor.string.contains("generated-1a2b3c4d.png") }
        XCTAssertNil(window.window?.attachedSheet)
        let recorded = try String(contentsOf: record, encoding: .utf8)
        XCTAssertTrue(recorded.contains("arguments: image generate \(deckPath) --slide 7 --prompt a fox at dusk --aspect 16:9 --match-theme --json"), recorded)
        XCTAssertTrue(recorded.contains("gemini: set"), "the Keychain's key reached this run as GEMINI_API_KEY")
        XCTAssertFalse(recorded.contains("placeholder-not-a-secret"), "the fake records that a key was set, never the value")
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Generate Image")
        XCTAssertEqual(controller.currentSlideNumber, 7)
        XCTAssertFalse(document.isDocumentEdited)
        XCTAssertFalse(controller.session.log.text.contains("placeholder-not-a-secret"))
    }

    /// The key goes to the image runs alone: a tap dev or tap present
    /// session, whose shell driver runs blocks with tap's environment,
    /// never sees the Keychain's key, and neither does any other tool run.
    func testATapDevSessionNeverGetsTheKeychainsKey() async throws {
        AppEnvironment.shared.geminiKeyStore = MemoryGeminiKeyStore(key: "placeholder-not-a-secret")
        // HostedTestCase sets the shell value to "", so the session holds "" (or nothing): never the Keychain's.
        let session = await AppEnvironment.shared.sessionConfiguration().environment()
        XCTAssertEqual(session["GEMINI_API_KEY"] ?? "", "", "tap dev's environment has no key from the Keychain")
        XCTAssertNotEqual(session["GEMINI_API_KEY"], "placeholder-not-a-secret")
        let talk = await AppEnvironment.shared.presentSessionConfiguration().environment()
        XCTAssertEqual(talk["GEMINI_API_KEY"] ?? "", "", "tap present's neither")
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.write("""
          "theme list") echo '{"ok": true, "themes": []}'; exit 0 ;;
        """, recordingTo: record)
        _ = await TapTool.run(["theme", "list", "--json"], timeout: 10)
        XCTAssertTrue(try String(contentsOf: record, encoding: .utf8).contains("gemini: \n"), "a tool run without includeGeminiKey has no key")
        _ = await TapTool.run(["theme", "list", "--json"], timeout: 10, includeGeminiKey: true)
        XCTAssertTrue(try String(contentsOf: record, encoding: .utf8).contains("gemini: set\n"), "only a run that asks for it")
        // The deck's tap dev, as it starts: a wrapper records whether the
        // live process has the key (never its value), then runs the real tap.
        let realTap = AppEnvironment.shared.tapExecutableURL
        addTeardownBlock { @MainActor in AppEnvironment.shared.tapExecutableURL = realTap }
        let devRecord = try Fixtures.temporaryFolder().appendingPathComponent("dev.txt")
        let wrapper = try Fixtures.temporaryFolder().appendingPathComponent("tap")
        try """
        #!/bin/sh
        echo "$1 gemini: ${GEMINI_API_KEY:+set}" >> \(FakeToolScripts.shellQuoted(devRecord.path))
        exec \(FakeToolScripts.shellQuoted(realTap.path)) "$@"
        """.write(to: wrapper, atomically: true, encoding: .utf8)
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: wrapper.path)
        AppEnvironment.shared.tapExecutableURL = wrapper
        let document = try await openDeck(try Fixtures.copyDeck("plain.md"))
        let ready = try await waitForRunningTap(document)
        XCTAssertGreaterThan(ready.port, 0)
        let devLines = try String(contentsOf: devRecord, encoding: .utf8).components(separatedBy: "\n").filter { $0.hasPrefix("dev ") }
        XCTAssertFalse(devLines.isEmpty, "the wrapper ran tap dev")
        XCTAssertEqual(Set(devLines), ["dev gemini: "], "the running tap dev has no key")
    }

    func testRegenerateAnAIImage() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try fakeGenerate(recordingTo: record)
        let deck = try Fixtures.copyDeck("ai-image")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 3)
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        let deckPath = try XCTUnwrap(document.fileURL?.path)
        controller.jumpToSlide(number: 2)
        XCTAssertEqual(controller.aiImagesOnCurrentSlide.map(\.imagePath), ["images/generated-00000000.png"])
        controller.jumpToSlide(number: 1)
        XCTAssertEqual(controller.aiImagesOnCurrentSlide, [], "slide 1 has none")

        // The RegenerateMenu board: Insert Image…, Generate Image…, then "Regenerate Image"; with two images, one each named by its prompt.
        controller.jumpToSlide(number: 2)
        let boxTwo = try XCTUnwrap(controller.editor.boxes.firstIndex { $0.slide.number == 2 })
        let menuTwo = try XCTUnwrap(controller.editor(controller.editor, contextMenuForBoxAt: boxTwo))
        let titlesTwo = menuTwo.items.map(\.title)
        let insert = try XCTUnwrap(titlesTwo.firstIndex(of: "Insert Image…"))
        XCTAssertEqual(Array(titlesTwo[insert..<insert + 3]), ["Insert Image…", "Generate Image…", "Regenerate Image"], "the board's order")
        let boxThree = try XCTUnwrap(controller.editor.boxes.firstIndex { $0.slide.number == 3 })
        let menuThree = try XCTUnwrap(controller.editor(controller.editor, contextMenuForBoxAt: boxThree))
        let regenerates = menuThree.items.filter { $0.action == #selector(DeckWindowController.regenerateImage(_:)) }
        XCTAssertEqual(regenerates.map(\.title), ["Regenerate \u{201C}a lighthouse in thick fog\u{201D}", "Regenerate \u{201C}an isometric server room at\u{2026}\u{201D}"])
        XCTAssertEqual(regenerates.map { $0.representedObject as? RegenerateTarget }, [RegenerateTarget(slide: 3, imagePath: "images/generated-11111111.png"),
                                                                                      RegenerateTarget(slide: 3, imagePath: "images/generated-22222222.png")])
        let panelMenu = try XCTUnwrap(controller.slidePanelContextMenu(controller.slidePanel))
        XCTAssertTrue(panelMenu.items.contains { $0.action == #selector(DeckWindowController.regenerateImage(_:)) }, "the thumbnail's menu has the same items")

        // The caret moves to slide 3; slide 2's item still regenerates slide 2's image.
        controller.jumpToSlide(number: 3)
        XCTAssertEqual(controller.currentSlideNumber, 3)
        let item = try XCTUnwrap(menuTwo.items.first { $0.title == "Regenerate Image" })
        XCTAssertEqual(item.representedObject as? RegenerateTarget, RegenerateTarget(slide: 2, imagePath: "images/generated-00000000.png"))
        window.regenerateImage(item)
        try await waitUntil(timeout: 20, "the replacement") { controller.editor.string.contains("generated-ffffffff.png") }
        XCTAssertFalse(controller.editor.string.contains("generated-00000000.png"), "replaced in place")
        XCTAssertTrue(try String(contentsOf: record, encoding: .utf8).contains("arguments: image regenerate \(deckPath) --slide 2 --image images/generated-00000000.png --json"))
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "Regenerate Image")
    }

    func testNoKeyShowsTapsMessageWithASettingsButton() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.write("""
          "image generate")
            printf '{"ok": false, "error": {"code": "no_api_key", "message": "cannot start image generation: GEMINI_API_KEY is not set"}}\\n'
            exit 1 ;;
        """, recordingTo: record)
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        window.generateImage(nil)
        try await waitUntil(timeout: 5, "the sheet") { window.window?.attachedSheet is GenerateImageSheet }
        let sheet = try XCTUnwrap(window.window?.attachedSheet as? GenerateImageSheet)
        sheet.promptView.string = "x"
        sheet.textDidChange(Notification(name: NSText.didChangeNotification, object: sheet.promptView))
        sheet.generateButton.performClick(nil)
        try await waitUntil(timeout: 20, "tap's error") { !sheet.errorLabel.isHidden }
        XCTAssertTrue(sheet.errorLabel.stringValue.contains("GEMINI_API_KEY is not set"))
        XCTAssertFalse(sheet.settingsButton.isHidden, "the way to the Image Generation pane")
        XCTAssertNil(controller.editorViewController.bar(.toolFailed), "the sheet is the one place the error shows")
        XCTAssertTrue(try String(contentsOf: record, encoding: .utf8).contains("gemini: \n"), "no key was set")
        sheet.cancelButton.performClick(nil)
    }

    /// Cancel on the sheet while tap runs closes the sheet; tap's failure
    /// then shows on the bar, since the sheet is gone.
    func testAFailureAfterTheSheetIsCancelledShowsOnTheBar() async throws {
        let record = try Fixtures.temporaryFolder().appendingPathComponent("record.txt")
        AppEnvironment.shared.toolExecutableURL = try FakeToolScripts.write("""
          "image generate")
            sleep 1
            printf '{"ok": false, "error": {"code": "generation_failed", "message": "the model refused the prompt"}}\\n'
            exit 1 ;;
        """, recordingTo: record)
        let document = try await openDeck(try Fixtures.copyDeck("seven-slides.md"))
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        window.generateImage(nil)
        try await waitUntil(timeout: 5, "the sheet") { window.window?.attachedSheet is GenerateImageSheet }
        let sheet = try XCTUnwrap(window.window?.attachedSheet as? GenerateImageSheet)
        sheet.promptView.insertText("a fox", replacementRange: sheet.promptView.selectedRange())
        sheet.generateButton.performClick(nil)
        sheet.cancelButton.performClick(nil)
        try await waitUntil(timeout: 5, "the sheet to close") { window.window?.attachedSheet == nil }
        try await waitUntil(timeout: 20, "tap's failure on the bar") { controller.editorViewController.bar(.toolFailed) != nil }
        XCTAssertEqual(controller.editorViewController.bar(.toolFailed)?.message, "Generate Image failed.")
        XCTAssertEqual(controller.editorViewController.bar(.toolFailed)?.detail, "the model refused the prompt")
    }
}
