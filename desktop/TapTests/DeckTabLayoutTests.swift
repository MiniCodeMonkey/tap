import XCTest
@testable import Tap

/// The Deck tab's layout at the inspector's width, as the DeckTabFields,
/// DeckTabGroups and DeckTabDrivers boards draw it: each group a card as
/// tall as its rows, the rows one under the other inside it, the cards
/// one under the other, and the form as tall as all of them so it scrolls.
final class DeckTabLayoutTests: HostedTestCase {
    /// A deck with every kind of row: fields, popups and switches, the
    /// object groups, two drivers with raw rows, and keys tap does not know.
    static let deck = """
    ---
    title: Layout
    theme: terminal
    presenterLayout: duo
    themeColors:
      codeBg: "#0f1a16"
    recording:
      showClicks: true
    drivers:
      sqlite:
        timeout: 5
        connections:
          incident:
            path: ./incident.db
      postgres:
        command: psql
        args:
          - -h
          - db
    fragments: false
    codeTheme: github-dark
    ---

    # One
    """

    func testEachCardHoldsItsRowsAtTheInspectorsWidth() async throws {
        Task { await AppEnvironment.shared.deckSchema.load() }
        try await waitUntil(timeout: 30, "tap deck schema --json") { AppEnvironment.shared.deckSchema.isLoaded }
        let form = DeckFormViewController()
        form.text = { Self.deck }
        form.setSchema(AppEnvironment.shared.deckSchema.keys)
        // The inspector is about 600 points wide; the form is laid out in a view of that size, in no window.
        let inspector = NSView(frame: NSRect(x: 0, y: 0, width: 600, height: 700))
        form.view.frame = inspector.bounds
        form.view.autoresizingMask = [.width, .height]
        inspector.addSubview(form.view)
        form.rebuild()
        inspector.layoutSubtreeIfNeeded()

        let cards = form.cards
        XCTAssertEqual(cards.count, 7, "Deck, Theme colors, Recording, a card per driver, the name field with Add, and Other keys")
        func frame(_ view: NSView) -> NSRect { view.convert(view.bounds, to: form.stack) }
        let cardFrames = cards.map(frame)
        for (card, cardFrame) in zip(cards, cardFrames) {
            let rowFrames = card.rows.map(frame)
            XCTAssertFalse(rowFrames.isEmpty)
            XCTAssertGreaterThanOrEqual(cardFrame.height, rowFrames.reduce(0) { $0 + $1.height }, "the card is as tall as its rows: \(cardFrame)")
            for rowFrame in rowFrames {
                XCTAssertGreaterThan(rowFrame.height, 0)
                XCTAssertTrue(cardFrame.insetBy(dx: -0.5, dy: -0.5).contains(rowFrame), "the row \(rowFrame) lies inside its card \(cardFrame)")
            }
            for (index, rowFrame) in rowFrames.enumerated() {
                for other in rowFrames[(index + 1)...] {
                    XCTAssertLessThanOrEqual(rowFrame.intersection(other).height, 0.5, "the rows \(rowFrame) and \(other) do not overlap")
                }
            }
        }
        // The Theme row's button is a row of the Deck card, at its own width.
        let themeButton = try XCTUnwrap(form.themeRowButton)
        let themeFrame = frame(themeButton)
        XCTAssertGreaterThanOrEqual(themeFrame.width, themeButton.fittingSize.width - 0.5, "the theme button \(themeFrame) is not squeezed")
        XCTAssertTrue(cardFrames[0].contains(themeFrame), "the theme button \(themeFrame) lies inside the Deck card \(cardFrames[0])")
        for (index, cardFrame) in cardFrames.enumerated() {
            for other in cardFrames[(index + 1)...] {
                XCTAssertLessThanOrEqual(cardFrame.intersection(other).height, 0.5, "the cards \(cardFrame) and \(other) do not overlap")
            }
        }
        // The form is as tall as its cards, taller than the inspector, so it scrolls to every one.
        let documentFrame = try XCTUnwrap(form.scrollView.documentView).frame
        XCTAssertGreaterThan(documentFrame.height, inspector.bounds.height)
        for cardFrame in cardFrames { XCTAssertTrue(form.stack.bounds.contains(cardFrame), "\(cardFrame) is inside the form") }
    }
}
