import XCTest
@testable import TapDesktopCore

final class PresentingCoreTests: XCTestCase {
    let builtIn = ScreenInfo(name: "Built-in Retina Display", frame: CGRect(x: 0, y: 0, width: 1728, height: 1117), isBuiltIn: true)
    let projector = ScreenInfo(name: "LG UltraFine", frame: CGRect(x: 1728, y: 0, width: 3840, height: 2160), isBuiltIn: false)

    func freshDefaults() throws -> UserDefaults {
        try XCTUnwrap(UserDefaults(suiteName: "PresentingCoreTests.\(UUID().uuidString)"))
    }

    func testTheExternalDisplayIsTheAudienceByDefaultAndTheBuiltInThePresenter() throws {
        let store = DisplayAssignmentStore(defaults: try freshDefaults())
        let arrangement = try XCTUnwrap(DisplayArrangement.resolve(screens: [builtIn, projector], store: store))
        XCTAssertEqual(arrangement.audience, projector)
        XCTAssertEqual(arrangement.presenter, builtIn)
        XCTAssertFalse(arrangement.isSingleDisplay)
        let reversed = try XCTUnwrap(DisplayArrangement.resolve(screens: [projector, builtIn], store: store))
        XCTAssertEqual(reversed.audience, projector, "the order the system lists screens in does not matter")
    }

    func testOneDisplayIsBothAndNoDisplayIsNothing() throws {
        let store = DisplayAssignmentStore(defaults: try freshDefaults())
        let one = try XCTUnwrap(DisplayArrangement.resolve(screens: [builtIn], store: store))
        XCTAssertEqual(one.audience, builtIn)
        XCTAssertEqual(one.presenter, builtIn)
        XCTAssertTrue(one.isSingleDisplay)
        XCTAssertNil(DisplayArrangement.resolve(screens: [], store: store))
    }

    func testTwoExternalDisplaysUseTheFirstAsTheAudience() throws {
        let store = DisplayAssignmentStore(defaults: try freshDefaults())
        let second = ScreenInfo(name: "Dell", frame: CGRect(x: -1920, y: 0, width: 1920, height: 1080), isBuiltIn: false)
        let arrangement = try XCTUnwrap(DisplayArrangement.resolve(screens: [projector, second], store: store))
        XCTAssertEqual(arrangement.audience, projector)
        XCTAssertEqual(arrangement.presenter, second)
    }

    func testASwapIsRememberedForThePairOfDisplaysWhicheverOrderTheyComeIn() throws {
        let store = DisplayAssignmentStore(defaults: try freshDefaults())
        let arrangement = try XCTUnwrap(DisplayArrangement.resolve(screens: [builtIn, projector], store: store))
        let swapped = arrangement.swapped()
        XCTAssertEqual(swapped.audience, builtIn)
        XCTAssertEqual(swapped.presenter, projector)
        store.setAudienceName(swapped.audience.name, for: [builtIn, projector])
        XCTAssertEqual(DisplayArrangement.resolve(screens: [projector, builtIn], store: store)?.audience, builtIn)
        XCTAssertEqual(DisplayAssignmentStore.key(for: [projector, builtIn]), DisplayAssignmentStore.key(for: [builtIn, projector]))
        // A different pair has no memory of it.
        let other = ScreenInfo(name: "Epson", frame: projector.frame, isBuiltIn: false)
        XCTAssertEqual(DisplayArrangement.resolve(screens: [builtIn, other], store: store)?.audience, other)
        // A remembered name that is no longer connected is ignored.
        store.setAudienceName("Gone", for: [builtIn, projector])
        XCTAssertEqual(DisplayArrangement.resolve(screens: [builtIn, projector], store: store)?.audience, projector)
    }

    func testOptionsBecomeTheTapPresentCommand() {
        let play = PresentationOptions(mode: .play, startSlide: 3)
        XCTAssertEqual(play.command(port: nil), .present(record: true, presenterPassword: nil, port: nil))
        XCTAssertEqual(play.command(port: 4242), .present(record: true, presenterPassword: nil, port: 4242), "the deck's remembered port travels with the command")
        XCTAssertFalse(play.wantsTunnel)
        let quiet = PresentationOptions(mode: .play, startSlide: 1, record: false, phoneRemote: true)
        XCTAssertEqual(quiet.command(port: nil), .present(record: false, presenterPassword: nil, port: nil))
        XCTAssertTrue(quiet.wantsTunnel)
        let rehearse = PresentationOptions(mode: .rehearse, startSlide: 5, record: true, tunnel: true, presenterPassword: "secret")
        XCTAssertEqual(rehearse.command(port: nil), .present(record: false, presenterPassword: "secret", port: nil), "a rehearsal never records")
        XCTAssertTrue(rehearse.wantsTunnel)
    }

    func testThePortIsRememberedPerDeck() throws {
        let store = DeckPortStore(defaults: try freshDefaults())
        let deck = URL(fileURLWithPath: "/talks/ops.md")
        XCTAssertNil(store.port(for: deck))
        store.setPort(4242, for: deck)
        XCTAssertEqual(store.port(for: deck), 4242)
        XCTAssertEqual(store.port(for: URL(fileURLWithPath: "/talks/../talks/ops.md")), 4242, "the same file, however spelled")
        XCTAssertNil(store.port(for: URL(fileURLWithPath: "/talks/other.md")), "another deck has its own")
        store.setPort(4243, for: deck)
        XCTAssertEqual(store.port(for: deck), 4243, "a fallback port replaces the taken one")
        // A deck's first talk asks for a port of its own, outside the ephemeral range every outgoing connection and tap dev draw from.
        let suggested = DeckPortStore.suggestedPort(for: deck)
        XCTAssertTrue((20000...29999).contains(suggested))
        XCTAssertEqual(DeckPortStore.suggestedPort(for: URL(fileURLWithPath: "/talks/../talks/ops.md")), suggested, "the same file, the same port, in every launch")
        XCTAssertNotEqual(DeckPortStore.suggestedPort(for: URL(fileURLWithPath: "/talks/other.md")), suggested)
    }

    func testTheLastSettingsBecomeTheNextOptions() throws {
        let store = PresentationSettingsStore(defaults: try freshDefaults())
        XCTAssertEqual(store.settings, PresentationSettings(), "the defaults: from the cursor, recording on, no remote, no tunnel")
        let chosen = PresentationSettings(startFromSlideOne: true, record: false, phoneRemote: true, tunnel: true)
        store.settings = chosen
        XCTAssertEqual(PresentationSettingsStore(defaults: store.defaults).settings, chosen, "kept across launches")
        XCTAssertEqual(chosen.options(mode: .play, cursorSlide: 4, presenterPassword: nil),
                       PresentationOptions(mode: .play, startSlide: 1, record: false, phoneRemote: true, tunnel: true, presenterPassword: nil))
        XCTAssertEqual(PresentationSettings().options(mode: .play, cursorSlide: 4, presenterPassword: "secret"),
                       PresentationOptions(mode: .play, startSlide: 4, record: true, presenterPassword: "secret"),
                       "the password comes from the popover's field for this launch; it is never kept")
    }

    func testTheRecordingLabelFollowsTapAndCountsUpBetweenEvents() {
        var status = RecordingStatus()
        XCTAssertEqual(status.label, "NOT RECORDING")
        XCTAssertFalse(status.isRecording)
        status.apply(RecordingEvent(state: "recording", segment: 1, elapsed: 724, disk: "ok"))
        XCTAssertEqual(status.label, "REC 12:04")
        XCTAssertTrue(status.isRecording)
        status.tick()
        XCTAssertEqual(status.label, "REC 12:05")
        status.apply(RecordingEvent(state: "paused", segment: 1, elapsed: 725, disk: "ok"))
        XCTAssertEqual(status.label, "REC PAUSED")
        status.tick()
        XCTAssertEqual(status.elapsed, 725, "a pause does not count up")
        status.apply(RecordingEvent(state: "stopped", segment: 1, elapsed: 0, disk: "full"))
        XCTAssertEqual(status.label, "NOT RECORDING")
        XCTAssertEqual(status.disk, "full")
        status.blockedReason = "Screen Recording permission is off"
        XCTAssertEqual(status.label, "NOT RECORDING")
        XCTAssertEqual(RecordingStatus.clock(0), "0:00")
        XCTAssertEqual(RecordingStatus.clock(59), "0:59")
        XCTAssertEqual(RecordingStatus.clock(3661), "1:01:01")
    }

    func testTheFocusHintShowsOnce() throws {
        let hint = FocusHintState(defaults: try freshDefaults())
        XCTAssertFalse(hint.hasBeenShown)
        hint.markShown()
        XCTAssertTrue(hint.hasBeenShown)
    }
}
