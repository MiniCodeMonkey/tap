import XCTest
@testable import Tap

final class ComponentTests: HostedTestCase {
    func testCreateAComponent() async throws {
        let deck = try Fixtures.copyDeck("ops.md")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 7)
        let controller = try XCTUnwrap(document.sessionController)
        let window = try XCTUnwrap(document.windowControllers.first as? DeckWindowController)
        var opened: [URL] = []
        controller.openInEditor = { opened.append($0) }
        controller.jumpToSlide(number: 4)
        let caret = controller.editor.selectedRange().location

        window.newComponent(nil)
        try await waitUntil(timeout: 5, "the sheet") { window.window?.attachedSheet is NewComponentSheet }
        let sheet = try XCTUnwrap(window.window?.attachedSheet as? NewComponentSheet)
        XCTAssertFalse(sheet.createButton.isEnabled, "no name yet")
        sheet.nameField.stringValue = "Counter"
        sheet.nameChanged(sheet.nameField)
        XCTAssertTrue(sheet.createButton.isEnabled)
        sheet.nameField.stringValue = "counter"
        sheet.nameChanged(sheet.nameField)
        XCTAssertFalse(sheet.createButton.isEnabled, "tap wants PascalCase; the sheet says so before tap has to")
        sheet.nameField.stringValue = "Counter"
        sheet.nameChanged(sheet.nameField)
        XCTAssertEqual(sheet.request.arguments(deck: deck), ["component", "new", "Counter", deck.path, "--json"])
        sheet.createButton.performClick(nil)

        try await waitUntil(timeout: 20, "tap's snippet") { controller.editor.string.contains("layout: ./slides/Counter.jsx") }
        XCTAssertNil(window.window?.attachedSheet)
        let file = deck.deletingLastPathComponent().appendingPathComponent("slides/Counter.jsx")
        XCTAssertTrue(FileManager.default.fileExists(atPath: file.path), "tap wrote the component")
        XCTAssertEqual(opened.map(\.path), [file.path], "opened in the default code editor")
        let text = controller.editor.string as NSString
        XCTAssertEqual(text.range(of: "<!--\nlayout: ./slides/Counter.jsx\n-->").location, caret, "tap's snippet, at the caret")
        XCTAssertEqual(controller.editor.undoManager?.undoActionName, "New Component")
        XCTAssertEqual(controller.currentSlideNumber, 4)

        // Inline and TypeScript: the flags, and tap's snippet for a block component.
        window.newComponent(nil)
        try await waitUntil(timeout: 5, "the second sheet") { window.window?.attachedSheet is NewComponentSheet }
        let second = try XCTUnwrap(window.window?.attachedSheet as? NewComponentSheet)
        second.nameField.stringValue = "LatencyDrop"
        second.nameChanged(second.nameField)
        second.kindControl.selectedSegment = 1
        second.typeScriptSwitch.state = .on
        XCTAssertEqual(second.request.arguments(deck: deck), ["component", "new", "LatencyDrop", deck.path, "--inline", "--ts", "--json"])
        second.createButton.performClick(nil)
        try await waitUntil(timeout: 20, "the inline snippet") { controller.editor.string.contains("```component ./components/LatencyDrop.tsx") }
        XCTAssertTrue(FileManager.default.fileExists(atPath: deck.deletingLastPathComponent().appendingPathComponent("components/LatencyDrop.tsx").path))
        XCTAssertEqual(opened.count, 2)

        // A name tap refuses (the file exists now): tap's message in the sheet.
        window.newComponent(nil)
        try await waitUntil(timeout: 5, "the third sheet") { window.window?.attachedSheet is NewComponentSheet }
        let third = try XCTUnwrap(window.window?.attachedSheet as? NewComponentSheet)
        third.nameField.stringValue = "Counter"
        third.nameChanged(third.nameField)
        third.createButton.performClick(nil)
        try await waitUntil(timeout: 20, "tap's error") { !third.errorLabel.isHidden }
        XCTAssertTrue(third.errorLabel.stringValue.contains("already exists"), third.errorLabel.stringValue)
        third.cancelButton.performClick(nil)
    }

    func testOpenAComponent() async throws {
        let deck = try Fixtures.copyDeck("stepped")
        let document = try await openDeck(deck)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 30, "tap's boxes for the fixture") { !controller.editor.boxes.isEmpty && controller.lastAppliedText == controller.editor.string }
        var opened: [URL] = []
        controller.openInEditor = { opened.append($0) }
        let text = controller.editor.string as NSString
        let pathRange = text.range(of: "./slides/RollingDeploy.jsx")
        XCTAssertNotEqual(pathRange.location, NSNotFound, "the fixture names its component")

        // A Cmd-click on the path opens the file; a plain click moves the caret.
        XCTAssertTrue(controller.openComponentLink(at: pathRange.location + 5))
        XCTAssertEqual(opened.map { Fixtures.realPath(of: $0) }, [Fixtures.realPath(of: deck.deletingLastPathComponent().appendingPathComponent("slides/RollingDeploy.jsx"))])
        XCTAssertFalse(controller.openComponentLink(at: text.range(of: "# ").location), "not on a path")
        XCTAssertEqual(opened.count, 1)

        // From a downloaded deck: a link that leads out of the deck's folder, and a bundle named like a component, open nothing.
        let slides = deck.deletingLastPathComponent().appendingPathComponent("slides")
        let outside = try Fixtures.temporaryFolder().appendingPathComponent("Secret.jsx")
        try "export default 1".write(to: outside, atomically: true, encoding: .utf8)
        try FileManager.default.createSymbolicLink(at: slides.appendingPathComponent("Linked.jsx"), withDestinationURL: outside)
        try FileManager.default.createDirectory(at: slides.appendingPathComponent("Bundle.jsx"), withIntermediateDirectories: true)
        let end = (controller.editor.string as NSString).length
        controller.editor.replaceText(in: NSRange(location: end, length: 0), with: "\n./slides/Linked.jsx ./slides/Bundle.jsx\n", actionName: "Typing")
        let edited = controller.editor.string as NSString
        XCTAssertFalse(controller.openComponentLink(at: edited.range(of: "./slides/Linked.jsx").location + 3), "a link out of the deck's folder")
        XCTAssertFalse(controller.openComponentLink(at: edited.range(of: "./slides/Bundle.jsx").location + 3), "a folder, not a file")
        XCTAssertEqual(opened.count, 1, "nothing more opened")
        let point = controller.editor.pointForCharacter(at: pathRange.location + 5)
        let clicked = controller.editor.characterIndexForInsertion(at: point)
        guard NSLocationInRange(clicked, pathRange) else {
            return XCTFail("the click's point \(point) reads character \(clicked), not one of the path's \(pathRange)")
        }
        // A leftMouseUp is queued behind the click: the Cmd-click branch
        // returns without reading it, but a click that reached NSTextView's
        // own mouseDown would otherwise wait forever in its tracking loop for
        // a mouse up that never comes, hanging the test instead of failing it.
        let window = try XCTUnwrap(controller.editor.window)
        func event(_ type: NSEvent.EventType) throws -> NSEvent {
            try XCTUnwrap(NSEvent.mouseEvent(with: type, location: controller.editor.convert(point, to: nil), modifierFlags: [.command], timestamp: ProcessInfo.processInfo.systemUptime,
                                             windowNumber: window.windowNumber, context: nil, eventNumber: 0, clickCount: 1, pressure: 1))
        }
        NSApp.postEvent(try event(.leftMouseUp), atStart: false)
        controller.editor.mouseDown(with: try event(.leftMouseDown))
        NSApp.discardEvents(matching: .leftMouseUp, before: nil)
        XCTAssertEqual(opened.count, 2, "the Cmd-click reached the same path")
    }

    func testComponentErrors() async throws {
        let deck = try Fixtures.copyDeck("broken-component")
        let document = try await openDeck(deck)
        try await waitForBoxes(document, count: 2)
        let controller = try XCTUnwrap(document.sessionController)
        try await waitUntil(timeout: 10, "tap's component error on slide 2") {
            controller.editor.boxes.count == 2 && controller.editor.header(forBoxAt: 1).errors.contains { $0.contains("failed to build") }
        }
        let errors = controller.editor.header(forBoxAt: 1).errors
        XCTAssertTrue(errors.contains { $0.contains("Broken.jsx") }, "tap names the file: \(errors)")
        XCTAssertEqual(controller.editor.header(forBoxAt: 0).errors, [], "slide 1 is fine")
        XCTAssertGreaterThan(EditorTextView.errorLineCount(for: controller.editor.boxes[1].slide), 0, "the box makes room for the line")
    }
}
