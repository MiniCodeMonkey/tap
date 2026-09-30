import XCTest
@testable import TapDesktopCore

final class SlidePanelStateTests: XCTestCase {
    func testPinnedOnFirstLaunchAndRememberedPerDeck() throws {
        let suiteName = "SlidePanelStateTests.\(UUID().uuidString)"
        let defaults = try XCTUnwrap(UserDefaults(suiteName: suiteName))
        let state = SlidePanelState(defaults: defaults)
        let deckA = URL(fileURLWithPath: "/tmp/a.md")
        let deckB = URL(fileURLWithPath: "/tmp/b.md")
        XCTAssertTrue(state.isPinned(deck: deckA), "pinned on first launch, so people find the panel")
        state.setPinned(false, deck: deckA)
        XCTAssertFalse(state.isPinned(deck: deckA))
        XCTAssertFalse(state.isPinned(deck: URL(fileURLWithPath: "/private/tmp/a.md")), "the same deck through a symlink")
        XCTAssertTrue(state.isPinned(deck: deckB), "another deck starts from the default")
        state.moveState(from: deckA, to: deckB)
        XCTAssertFalse(state.isPinned(deck: deckB), "a renamed or saved-as deck keeps its state")
        XCTAssertTrue(state.isPinned(deck: deckA), "and the old key is gone")
        defaults.removePersistentDomain(forName: suiteName)
    }

    func testTheDeckCardIsClosedOnFirstOpenAndRememberedPerDeck() throws {
        let suiteName = "SlidePanelStateTests.\(UUID().uuidString)"
        let defaults = try XCTUnwrap(UserDefaults(suiteName: suiteName))
        defer { defaults.removePersistentDomain(forName: suiteName) }
        let state = SlidePanelState(defaults: defaults)
        let deckA = URL(fileURLWithPath: "/tmp/a.md")
        let deckB = URL(fileURLWithPath: "/tmp/b.md")
        XCTAssertFalse(state.isDeckCardOpen(deck: deckA))
        state.setDeckCardOpen(true, deck: deckA)
        XCTAssertTrue(state.isDeckCardOpen(deck: deckA))
        XCTAssertFalse(state.isDeckCardOpen(deck: deckB))
        XCTAssertTrue(state.isPinned(deck: deckA), "the panel's state is its own")
        state.moveState(from: deckA, to: deckB)
        XCTAssertTrue(state.isDeckCardOpen(deck: deckB), "a saved-as deck keeps its card open")
        XCTAssertFalse(state.isDeckCardOpen(deck: deckA))
    }
}
