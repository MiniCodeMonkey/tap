import XCTest
import WebKit
@testable import Tap

final class PresentMenuTests: PresentingTestCase {
    func presentMenu() throws -> NSMenu {
        try XCTUnwrap(NSApp.mainMenu?.items.first { $0.submenu?.title == "Present" }?.submenu)
    }

    func item(_ menu: NSMenu, action: Selector) throws -> NSMenuItem {
        try XCTUnwrap(menu.items.first { $0.action == action }, "no item with \(action)")
    }

    func keyEvent(_ type: NSEvent.EventType, characters: String, keyCode: UInt16, modifiers: NSEvent.ModifierFlags = [], in window: NSWindow) throws -> NSEvent {
        try XCTUnwrap(NSEvent.keyEvent(with: type, location: .zero, modifierFlags: modifiers, timestamp: ProcessInfo.processInfo.systemUptime,
                                       windowNumber: window.windowNumber, context: nil, characters: characters,
                                       charactersIgnoringModifiers: characters, isARepeat: false, keyCode: keyCode))
    }

    func testPresentingShortcuts() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let menu = try presentMenu()
        let play = try item(menu, action: #selector(DeckWindowController.play(_:)))
        XCTAssertEqual(play.title, "Play")
        XCTAssertEqual(play.keyEquivalent, "p")
        XCTAssertEqual(play.keyEquivalentModifierMask, [.command, .option])
        let withOptions = try item(menu, action: #selector(DeckWindowController.playWithOptions(_:)))
        XCTAssertEqual(withOptions.title, "Play with Options…")
        XCTAssertEqual(withOptions.keyEquivalent, "", "the popover has no shortcut; the Play button opens it")
        let rehearse = try item(menu, action: #selector(DeckWindowController.rehearse(_:)))
        XCTAssertEqual(rehearse.keyEquivalent, "p")
        XCTAssertEqual(rehearse.keyEquivalentModifierMask, [.command, .option, .shift])
        let stop = try item(menu, action: #selector(DeckWindowController.stopPresenting(_:)))
        XCTAssertEqual(stop.keyEquivalent, ".")
        XCTAssertEqual(stop.keyEquivalentModifierMask, [.command])
        let swap = try item(menu, action: #selector(DeckWindowController.swapDisplays(_:)))

        XCTAssertTrue(deckWindow.validateMenuItem(play))
        XCTAssertTrue(deckWindow.validateMenuItem(withOptions))
        XCTAssertTrue(deckWindow.validateMenuItem(rehearse))
        XCTAssertFalse(deckWindow.validateMenuItem(stop))
        XCTAssertFalse(deckWindow.validateMenuItem(swap), "one display: nothing to swap")

        // Cmd+Option+Shift+P starts rehearsing at once.
        deckWindow.rehearse(nil)
        XCTAssertEqual(controller.presentation.options?.mode, .rehearse)
        XCTAssertFalse(deckWindow.validateMenuItem(play))
        XCTAssertFalse(deckWindow.validateMenuItem(withOptions))
        XCTAssertFalse(deckWindow.validateMenuItem(rehearse))
        XCTAssertTrue(deckWindow.validateMenuItem(stop))
        try await waitUntil(timeout: 40, "the rehearsal") { controller.presentation.state == .presenting }
        deckWindow.stopPresenting(nil)
        try await waitUntil(timeout: 30, "the end") { controller.presentation.state == .idle }

        // Cmd+Option+P starts presenting at once, with the last settings and no popover.
        controller.jumpToSlide(number: 2)
        deckWindow.play(nil)
        XCTAssertFalse(deckWindow.presentPopover.isShown)
        XCTAssertEqual(controller.presentation.options?.mode, .play)
        XCTAssertEqual(controller.presentation.options?.startSlide, 2)
        try await waitUntil(timeout: 40, "the talk") { controller.presentation.state == .presenting }
    }

    func testOneDisplay() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertEqual(audience.targetFrame, NSScreen.screens[0].frame, "the audience page fills the screen")
        XCTAssertTrue(presentation.frontWindow === audience)
        XCTAssertFalse(presenter.isVisible)
        try await waitUntil(timeout: 5, "the audience on screen") { onScreenWindowNumbers().contains(audience.windowNumber) }
        // Option-Tab: the presenter view comes over the audience view, in the same Space, with no animation.
        let optionTab = try keyEvent(.keyDown, characters: "\t", keyCode: 48, modifiers: [.option], in: audience)
        XCTAssertNil(presentation.handleKey(optionTab), "the app takes Option-Tab")
        XCTAssertTrue(presentation.frontWindow === presenter)
        XCTAssertTrue(presentation.presenterIsShownOverAudience)
        XCTAssertTrue(presenter.isAttached)
        try await waitUntil(timeout: 5, "the presenter over the audience") {
            let order = onScreenWindowNumbers()
            guard let a = order.firstIndex(of: audience.windowNumber), let p = order.firstIndex(of: presenter.windowNumber) else { return false }
            return p < a
        }
        let again = try keyEvent(.keyDown, characters: "\t", keyCode: 48, modifiers: [.option], in: presenter)
        XCTAssertNil(presentation.handleKey(again))
        XCTAssertTrue(presentation.frontWindow === audience)
        XCTAssertFalse(presentation.presenterIsShownOverAudience)
        try await waitUntil(timeout: 5, "the presenter off the screen again") { !onScreenWindowNumbers().contains(presenter.windowNumber) }
        let plainTab = try keyEvent(.keyDown, characters: "\t", keyCode: 48, in: audience)
        XCTAssertNotNil(presentation.handleKey(plainTab), "a plain Tab goes to the page")
        let elsewhere = try keyEvent(.keyDown, characters: "\t", keyCode: 48, modifiers: [.option], in: try XCTUnwrap(controller.editor.window))
        XCTAssertNotNil(presentation.handleKey(elsewhere), "Option-Tab in the deck window is not the app's")
    }

    func testOptionTabIsThePagesOnTwoDisplays() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        let screens = halfScreens()
        presentation.screens = { screens }
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        let optionTab = try keyEvent(.keyDown, characters: "\t", keyCode: 48, modifiers: [.option], in: presenter)
        XCTAssertNotNil(presentation.handleKey(optionTab), "with a display each there is nothing to switch, so the page gets the key")
        XCTAssertTrue(presentation.frontWindow === presenter)
    }

    func testEscapeInTheAudienceWindowStopsTheTalk() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        let escapeInPresenter = try keyEvent(.keyDown, characters: "\u{1b}", keyCode: 53, in: presenter)
        XCTAssertNotNil(presentation.handleKey(escapeInPresenter), "Escape in the presenter window is the page's while the audience window exists")
        XCTAssertEqual(presentation.state, .presenting)
        let escape = try keyEvent(.keyDown, characters: "\u{1b}", keyCode: 53, in: audience)
        XCTAssertNil(presentation.handleKey(escape))
        XCTAssertEqual(presentation.state, .stopping)
        XCTAssertFalse(presentation.sleepAssertion.isHeld)
        try await waitUntil(timeout: 30, "the end") { presentation.state == .idle }
        try await waitUntil(timeout: 10, "nothing left in full screen") { fullScreenPresentationWindows().isEmpty && presentation.windowsGoingDown.isEmpty }

        // A rehearsal has no audience window: Escape in the presenter window stops it.
        try await startPresenting(controller, PresentationOptions(mode: .rehearse, startSlide: 1))
        let rehearsal = try XCTUnwrap(presentation.presenterWindow)
        XCTAssertNil(presentation.handleKey(try keyEvent(.keyDown, characters: "\u{1b}", keyCode: 53, in: rehearsal)))
        XCTAssertEqual(presentation.state, .stopping)
        try await waitUntil(timeout: 30, "the end") { presentation.state == .idle }
        try await waitUntil(timeout: 10, "nothing left in full screen") { fullScreenPresentationWindows().isEmpty }
    }

    /// Every key the scenario lists, as the page's handler would see it.
    static let pageKeys: [(characters: String, keyCode: UInt16)] = [
        (String(Character(Unicode.Scalar(UInt16(NSLeftArrowFunctionKey))!)), 123),
        (String(Character(Unicode.Scalar(UInt16(NSRightArrowFunctionKey))!)), 124),
        (String(Character(Unicode.Scalar(UInt16(NSDownArrowFunctionKey))!)), 125),
        (String(Character(Unicode.Scalar(UInt16(NSUpArrowFunctionKey))!)), 126),
        (" ", 49),
        (String(Character(Unicode.Scalar(UInt16(NSHomeFunctionKey))!)), 115),
        (String(Character(Unicode.Scalar(UInt16(NSEndFunctionKey))!)), 119),
        ("o", 31), ("t", 17), ("f", 3), ("?", 44), ("r", 15), ("v", 9),
        ("1", 18), ("2", 19), ("3", 20), ("4", 21), ("5", 23), ("-", 27), ("=", 24),
    ]

    func testEveryTapDevKeyGoesToThePageUnchanged() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 1))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        for window in [audience, presenter] {
            for key in Self.pageKeys {
                for modifiers in [[], [.shift]] as [NSEvent.ModifierFlags] {
                    let event = try keyEvent(.keyDown, characters: key.characters, keyCode: key.keyCode, modifiers: modifiers, in: window)
                    XCTAssertTrue(presentation.handleKey(event) === event, "\(key.characters) with \(modifiers) in the \(window.role) window is the page's")
                }
            }
        }
        XCTAssertEqual(presentation.state, .presenting, "none of them ended the talk")
    }

    func testEveryTapDevPresenterFeatureWorks() async throws {
        let (_, controller) = try await openDeckForPresenting()
        let presentation = controller.presentation
        try await startPresenting(controller, PresentationOptions(mode: .play, startSlide: 2))
        let audience = try XCTUnwrap(presentation.audienceWindow)
        let presenter = try XCTUnwrap(presentation.presenterWindow)
        for page in [audience.page, presenter.page] {
            XCTAssertTrue(page.webView.configuration.preferences.isElementFullscreenEnabled, "F goes full screen")
            XCTAssertTrue(page.webView.configuration.websiteDataStore.isPersistent, "the page's localStorage outlives the process")
        }
        XCTAssertEqual(presenter.page.lastLoadedURL?.port, AppEnvironment.shared.deckPorts.port(for: try XCTUnwrap(controller.document?.fileURL)),
                       "the deck's port: the presenter layout and notes size persist between launches (Task 4 proves the port is reused)")
        try await waitUntil(timeout: 20, "the audience page on slide 2") { audience.page.lastReady?.slide == 2 }
        try await waitUntil(timeout: 20, "the presenter page on slide 2") { presenter.page.lastReady?.slide == 2 }

        // The arrow keys, and every other key, go to tap's page unchanged
        // (testEveryTapDevKeyGoesToThePageUnchanged): the page moves, the
        // hub relays it (the page holds the presenter cookie), and tap
        // reports the new position. The key is pressed in the page itself,
        // so the proof does not depend on which window the host has as key.
        await audience.page.pressKey("ArrowRight")
        try await waitUntil(timeout: 10, "tap's slide event for slide 3") { presentation.lastSlide == 3 }
        try await waitUntil(timeout: 10, "the presenter page following") { presenter.page.lastReady?.slide == 3 }

        // S opens the presenter view in the presenter window, never a browser popup: on one display it comes over the audience view.
        let port = try XCTUnwrap(presentation.client).ready.port
        audience.page.popupRequested(for: URL(string: "http://127.0.0.1:\(port)/presenter#3"), navigationType: .other)
        XCTAssertTrue(presentation.frontWindow === presenter)
        XCTAssertTrue(presentation.presenterIsShownOverAudience)
        try await waitUntil(timeout: 5, "the presenter over the audience") {
            let order = onScreenWindowNumbers()
            guard let a = order.firstIndex(of: audience.windowNumber), let p = order.firstIndex(of: presenter.windowNumber) else { return false }
            return p < a
        }
        audience.page.popupRequested(for: URL(string: "http://127.0.0.1:\(port)/presenter#3"), navigationType: .other)
        XCTAssertTrue(presentation.presenterIsShownOverAudience, "S again keeps it there; Option-Tab is what hides it")
    }
}
