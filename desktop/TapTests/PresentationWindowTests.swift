import XCTest
import WebKit
@testable import Tap

final class PresentationWindowTests: HostedTestCase {
    override func tearDown() async throws {
        for window in NSApp.windows.compactMap({ $0 as? PresentationWindow }) where !window.isClosed { window.takeDown() }
        try await waitUntil(timeout: 10, "every talk window closed") { fullScreenPresentationWindows().isEmpty }
        try await super.tearDown()
    }

    func testTheSleepAssertionIsHeldOnlyBetweenAcquireAndRelease() {
        let assertion = SleepAssertion()
        XCTAssertFalse(assertion.isHeld)
        XCTAssertFalse(powerAssertionIsListed(named: SleepAssertion.reason))
        assertion.acquire()
        XCTAssertTrue(assertion.isHeld)
        XCTAssertTrue(powerAssertionIsListed(named: SleepAssertion.reason), "the kernel lists the assertion for this process")
        assertion.acquire()
        XCTAssertTrue(assertion.isHeld, "a second acquire changes nothing")
        assertion.release()
        XCTAssertFalse(assertion.isHeld)
        XCTAssertFalse(powerAssertionIsListed(named: SleepAssertion.reason))
        assertion.release()
        XCTAssertFalse(assertion.isHeld)
    }

    func testAWindowEntersItsOwnFullScreenSpaceAndLeavesItOnTakeDown() async throws {
        try await requireFullScreen()
        let screen = NSScreen.screens[0].frame
        let window = PresentationWindow(role: .audience, screenFrame: screen)
        var states: [PresentationWindow.FullScreenState] = []
        window.onFullScreenChange = { states.append($0) }
        XCTAssertEqual(window.fullScreenState, .windowed)
        XCTAssertEqual(window.level, .normal, "no covering level: a full screen Space is what covers the display")
        XCTAssertTrue(window.collectionBehavior.contains(.fullScreenPrimary))
        XCTAssertFalse(window.collectionBehavior.contains(.canJoinAllSpaces), "the window lives in its own Space, so Cmd-Tab to another app leaves it")
        XCTAssertTrue(window.canBecomeKey)
        XCTAssertFalse(window.isVisible)

        var settled = 0
        window.present(on: screen) { settled += 1 }
        XCTAssertEqual(window.targetFrame, screen)
        try await waitUntil(timeout: 10, "the window in full screen (state \(window.fullScreenState))") { window.fullScreenState == .fullScreen }
        XCTAssertTrue(window.styleMask.contains(.fullScreen), "AppKit's own flag, after the enter notification")
        XCTAssertEqual(window.settledFrame, screen)
        XCTAssertEqual(settled, 1)
        XCTAssertEqual(states, [.entering, .fullScreen])
        XCTAssertEqual(window.page.webView.frame.size, window.contentView?.bounds.size, "the page fills the window")
        try await waitUntil(timeout: 5, "the window server to show the window") { onScreenWindowNumbers().contains(window.windowNumber) }

        var closed = false
        window.takeDown { closed = true }
        try await waitUntil(timeout: 10, "the window closed (state \(window.fullScreenState))") { window.isClosed }
        XCTAssertTrue(closed)
        XCTAssertEqual(states, [.entering, .fullScreen, .exiting, .windowed], "it left full screen before it closed")
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty)
        try await waitUntil(timeout: 5, "the window to leave the screen") { !onScreenWindowNumbers().contains(window.windowNumber) }
    }

    func testATakeDownDuringTheEntryStillCloses() async throws {
        try await requireFullScreen()
        let window = PresentationWindow(role: .presenter, screenFrame: NSScreen.screens[0].frame)
        window.present(on: NSScreen.screens[0].frame)
        XCTAssertEqual(window.fullScreenState, .entering)
        window.takeDown()
        try await waitUntil(timeout: 15, "the window closed (state \(window.fullScreenState))") { window.isClosed }
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty, "nothing is left in full screen")
    }

    func testATakeDownClosesEvenWhenTheExitNeverCompletes() async throws {
        try await requireFullScreen()
        let window = PresentationWindow(role: .audience, screenFrame: NSScreen.screens[0].frame)
        window.present(on: NSScreen.screens[0].frame)
        try await waitUntil(timeout: 10, "full screen") { window.fullScreenState == .fullScreen }
        // An AppKit that never answers the exit request: the deadline closes the window anyway.
        window.requestFullScreenToggle = {}
        let asked = Date()
        window.takeDown()
        try await waitUntil(timeout: PresentationWindow.exitTimeout + 5, "the deadline to close the window") { window.isClosed }
        XCTAssertGreaterThanOrEqual(Date().timeIntervalSince(asked), PresentationWindow.exitTimeout - 0.5)
        XCTAssertTrue(fullScreenPresentationWindows().isEmpty)
        try await waitUntil(timeout: 5, "the window off the screen") { !onScreenWindowNumbers().contains(window.windowNumber) }
    }

    func testAMoveToAnotherFrameLeavesAndReentersFullScreen() async throws {
        try await requireFullScreen()
        let screen = NSScreen.screens[0].frame
        let half = CGRect(x: screen.minX, y: screen.minY, width: screen.width / 2, height: screen.height)
        let window = PresentationWindow(role: .audience, screenFrame: screen)
        var states: [PresentationWindow.FullScreenState] = []
        window.onFullScreenChange = { states.append($0) }
        window.present(on: screen)
        try await waitUntil(timeout: 10, "full screen") { window.fullScreenState == .fullScreen }
        var moved = false
        // On one display the half frame is on the same display, but it is not the frame the window settled on, so the window goes out and in again.
        window.present(on: half) { moved = true }
        XCTAssertEqual(window.targetFrame, half)
        try await waitUntil(timeout: 15, "the move to settle (state \(window.fullScreenState))") { moved }
        XCTAssertEqual(window.fullScreenState, .fullScreen)
        XCTAssertEqual(window.settledFrame, half, "what it settled on is what it was asked for, not the display's frame")
        XCTAssertEqual(states, [.entering, .fullScreen, .exiting, .windowed, .entering, .fullScreen])
        var again = false
        window.present(on: half) { again = true }
        XCTAssertTrue(again, "already there: nothing to do, and no exit")
        XCTAssertEqual(states.count, 6)
    }

    /// The state machine without AppKit: the toggle is a seam that records
    /// the asks, and the test plays the delegate's notifications itself.
    /// This is what covers the machine on a host that cannot enter full
    /// screen, and it runs everywhere.
    func testTheStateMachineThroughTheSeams() async throws {
        let screen = NSScreen.screens[0].frame
        let other = CGRect(x: screen.minX, y: screen.minY, width: screen.width / 2, height: screen.height)
        let window = PresentationWindow(role: .audience, screenFrame: screen)
        var toggles = 0
        window.requestFullScreenToggle = { toggles += 1 }
        var states: [PresentationWindow.FullScreenState] = []
        window.onFullScreenChange = { states.append($0) }
        func enter() { window.windowDidEnterFullScreen(Notification(name: NSWindow.didEnterFullScreenNotification, object: window)) }
        func exit() { window.windowDidExitFullScreen(Notification(name: NSWindow.didExitFullScreenNotification, object: window)) }

        var settled = 0
        window.present(on: screen) { settled += 1 }
        XCTAssertEqual(toggles, 1)
        XCTAssertEqual(window.fullScreenState, .entering)
        enter()
        XCTAssertEqual(window.fullScreenState, .fullScreen)
        XCTAssertEqual(window.settledFrame, screen)
        XCTAssertEqual(settled, 1)

        // A move: exit, then enter on the new frame.
        window.present(on: other) { settled += 1 }
        XCTAssertEqual(toggles, 2)
        XCTAssertEqual(window.fullScreenState, .exiting)
        exit()
        XCTAssertEqual(toggles, 3, "out, then in again")
        XCTAssertEqual(window.fullScreenState, .entering)
        XCTAssertEqual(window.frame.size, other.size)
        enter()
        XCTAssertEqual(settled, 2)
        XCTAssertEqual(window.settledFrame, other)

        // The same frame again: nothing.
        window.present(on: other) { settled += 1 }
        XCTAssertEqual(toggles, 3)
        XCTAssertEqual(settled, 3)

        // A failed entry leaves a plain window; a failed exit on take-down closes anyway.
        window.present(on: screen) { settled += 1 }
        exit()
        XCTAssertEqual(window.fullScreenState, .entering)
        window.windowDidFailToEnterFullScreen(window)
        XCTAssertEqual(window.fullScreenState, .windowed)
        XCTAssertEqual(settled, 4, "a refused entry still settles, as a plain window")
        window.present(on: screen) { settled += 1 }
        enter()
        var closed = false
        window.takeDown { closed = true }
        XCTAssertEqual(window.fullScreenState, .exiting)
        window.windowDidFailToExitFullScreen(window)
        XCTAssertTrue(window.isClosed, "closed in whatever state it was in")
        XCTAssertTrue(closed)
        XCTAssertEqual(states, [.entering, .fullScreen, .exiting, .windowed, .entering, .fullScreen,
                                .exiting, .windowed, .entering, .windowed, .entering, .fullScreen, .exiting, .fullScreen])
        XCTAssertEqual(toggles, 7)

        // A plain window: present orders it front and settles at once; take-down closes at once.
        let plain = PresentationWindow(role: .presenter, screenFrame: screen)
        plain.requestFullScreenToggle = { XCTFail("a plain placement never asks for full screen") }
        var plainSettled = false
        plain.present(on: screen, fullScreen: false) { plainSettled = true }
        XCTAssertTrue(plainSettled)
        XCTAssertTrue(plain.isVisible)
        XCTAssertEqual(plain.fullScreenState, .windowed)
        plain.takeDown()
        XCTAssertTrue(plain.isClosed)
    }

    func testAChildWindowRidesInItsParentsSpace() async throws {
        let screen = NSScreen.screens[0].frame
        let audience = PresentationWindow(role: .audience, screenFrame: screen)
        let presenter = PresentationWindow(role: .presenter, screenFrame: screen)
        audience.present(on: screen, fullScreen: false)
        XCTAssertFalse(presenter.isVisible)
        presenter.attach(to: audience)
        XCTAssertTrue(presenter.isAttached)
        XCTAssertTrue(presenter.isVisible, "attaching shows it over the parent")
        XCTAssertTrue(audience.childWindows?.contains(presenter) == true)
        XCTAssertEqual(presenter.frame, audience.frame, "it covers the parent")
        XCTAssertTrue(presenter.collectionBehavior.contains(.fullScreenAuxiliary), "a child may live in a full screen Space")
        XCTAssertFalse(presenter.collectionBehavior.contains(.fullScreenPrimary))
        try await waitUntil(timeout: 5, "both on screen, the child in front") {
            let order = onScreenWindowNumbers()
            guard let parent = order.firstIndex(of: audience.windowNumber), let child = order.firstIndex(of: presenter.windowNumber) else { return false }
            return child < parent
        }
        presenter.detach()
        XCTAssertFalse(presenter.isAttached)
        XCTAssertFalse(presenter.isVisible)
        XCTAssertFalse(audience.childWindows?.contains(presenter) == true)
        XCTAssertTrue(presenter.collectionBehavior.contains(.fullScreenPrimary), "a detached window can have a Space of its own again")
        presenter.attach(to: audience)
        audience.takeDown()
        XCTAssertTrue(audience.isClosed)
        XCTAssertFalse(presenter.isAttached, "a parent going down lets its child go first")
        XCTAssertTrue(presenter.isClosed)
    }

    func testAMenuCannotTakeATalkWindowOutOfFullScreen() {
        let window = PresentationWindow(role: .audience, screenFrame: NSScreen.screens[0].frame)
        let item = NSMenuItem(title: "Enter Full Screen", action: #selector(NSWindow.toggleFullScreen(_:)), keyEquivalent: "")
        XCTAssertFalse(window.validateUserInterfaceItem(item), "View > Enter Full Screen does not reach a talk window; the controller owns its full screen")
    }

    func testThePageHasEverythingTapDevsBrowserWouldGiveIt() {
        let page = PresentationPageController(accessibilityIdentifier: "audience-page", dataStore: AppEnvironment.shared.presentationDataStore)
        _ = page.view
        XCTAssertTrue(page.webView.configuration.preferences.isElementFullscreenEnabled, "the F key's full screen works")
        XCTAssertTrue(page.webView.configuration.websiteDataStore.isPersistent, "localStorage survives the process")
        XCTAssertTrue(page.webView.configuration.websiteDataStore === AppEnvironment.shared.presentationDataStore, "the one store every talk shares")
        XCTAssertFalse(page.webView.configuration.websiteDataStore === WKWebsiteDataStore.default(), "in a test, never the person's own store")
        XCTAssertEqual(page.webView.accessibilityIdentifier(), "audience-page")
    }

    func testTheSKeysPopupBringsThePresenterWindowForwardAndLinksGoToTheBrowser() {
        let page = PresentationPageController(accessibilityIdentifier: "audience-page", dataStore: AppEnvironment.shared.presentationDataStore)
        _ = page.view
        page.load(URL(string: "about:blank")!, allowedPort: 4242)
        var popups = 0
        var opened: [URL] = []
        page.onPresenterPopup = { popups += 1 }
        page.openExternally = { opened.append($0) }
        page.popupRequested(for: URL(string: "http://127.0.0.1:4242/presenter#3"), navigationType: .other)
        XCTAssertEqual(popups, 1)
        page.popupRequested(for: URL(string: "http://127.0.0.1:9999/presenter"), navigationType: .other)
        XCTAssertEqual(popups, 1, "another port is not this talk's presenter view")
        page.popupRequested(for: URL(string: "https://example.com/"), navigationType: .linkActivated)
        XCTAssertEqual(opened, [URL(string: "https://example.com/")!])
        page.popupRequested(for: URL(string: "https://example.com/redirect"), navigationType: .other)
        XCTAssertEqual(opened.count, 1, "a popup the page opened on its own goes nowhere")
        XCTAssertEqual(page.pageLoadCount, 1)
        XCTAssertEqual(page.lastLoadedURL?.absoluteString, "about:blank")
    }
}
