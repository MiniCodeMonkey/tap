import XCTest
@testable import Tap

final class PlaySplitControlTests: PresentingTestCase {
    /// A mouse event at `point`, in the control's coordinates.
    func event(_ type: NSEvent.EventType, at point: NSPoint, in control: PlaySplitControl) throws -> NSEvent {
        try XCTUnwrap(NSEvent.mouseEvent(with: type, location: control.convert(point, to: nil), modifierFlags: [], timestamp: ProcessInfo.processInfo.systemUptime,
                                         windowNumber: control.window?.windowNumber ?? 0, context: nil, eventNumber: 0, clickCount: 1, pressure: 1))
    }

    func makeControl() async throws -> (PlaySplitControl, plays: () -> Int, menus: () -> Int) {
        let (_, controller) = try await openDeckForPresenting()
        let deckWindow = try XCTUnwrap(controller.editor.window?.windowController as? DeckWindowController)
        let control = deckWindow.playButton
        control.layoutSubtreeIfNeeded()
        var plays = 0
        var menus = 0
        control.onPlay = { _ in plays += 1 }
        control.menuPresenter = { _, _ in menus += 1 }
        control.holdDelay = 0.05
        return (control, { plays }, { menus })
    }

    func testAClickOnThePlayGlyphPlays() async throws {
        let (control, plays, menus) = try await makeControl()
        let glyph = NSPoint(x: control.segmentBoundary / 2, y: control.bounds.midY)
        control.mouseDown(with: try event(.leftMouseDown, at: glyph, in: control))
        control.mouseUp(with: try event(.leftMouseUp, at: glyph, in: control))
        XCTAssertEqual(plays(), 1)
        XCTAssertEqual(menus(), 0)
    }

    func testTheChevronOpensTheMenu() async throws {
        let (control, plays, menus) = try await makeControl()
        let chevron = NSPoint(x: (control.segmentBoundary + control.bounds.maxX) / 2, y: control.bounds.midY)
        XCTAssertEqual(control.segment(at: chevron), 1)
        control.mouseDown(with: try event(.leftMouseDown, at: chevron, in: control))
        control.mouseUp(with: try event(.leftMouseUp, at: chevron, in: control))
        XCTAssertEqual(menus(), 1)
        XCTAssertEqual(plays(), 0)
    }

    func testHoldingThePlayGlyphOpensTheMenuAndDoesNotPlay() async throws {
        let (control, plays, menus) = try await makeControl()
        let glyph = NSPoint(x: control.segmentBoundary / 2, y: control.bounds.midY)
        control.mouseDown(with: try event(.leftMouseDown, at: glyph, in: control))
        try await waitUntil(timeout: 2, "the hold to open the menu") { menus() == 1 }
        control.mouseUp(with: try event(.leftMouseUp, at: glyph, in: control))
        XCTAssertEqual(plays(), 0)
    }

    func testDraggingOffBeforeReleaseDoesNotPlay() async throws {
        let (control, plays, menus) = try await makeControl()
        let glyph = NSPoint(x: control.segmentBoundary / 2, y: control.bounds.midY)
        control.mouseDown(with: try event(.leftMouseDown, at: glyph, in: control))
        control.mouseUp(with: try event(.leftMouseUp, at: NSPoint(x: control.bounds.maxX + 60, y: control.bounds.midY), in: control))
        XCTAssertEqual(plays(), 0)
        XCTAssertEqual(menus(), 0)
    }

    func testADisabledControlDoesNothing() async throws {
        let (control, plays, menus) = try await makeControl()
        control.isEnabled = false
        let glyph = NSPoint(x: control.segmentBoundary / 2, y: control.bounds.midY)
        control.mouseDown(with: try event(.leftMouseDown, at: glyph, in: control))
        control.mouseUp(with: try event(.leftMouseUp, at: glyph, in: control))
        XCTAssertEqual(plays(), 0)
        XCTAssertEqual(menus(), 0)
    }
}
