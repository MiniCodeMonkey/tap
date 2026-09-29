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
        XCTAssertFalse(arrangement.sharesDisplay)
        let reversed = try XCTUnwrap(DisplayArrangement.resolve(screens: [projector, builtIn], store: store))
        XCTAssertEqual(reversed.audience, projector, "the order the system lists screens in does not matter")
    }

    func testOneDisplayIsBothAndNoDisplayIsNothing() throws {
        let store = DisplayAssignmentStore(defaults: try freshDefaults())
        let one = try XCTUnwrap(DisplayArrangement.resolve(screens: [builtIn], store: store))
        XCTAssertEqual(one.audience, builtIn)
        XCTAssertEqual(one.presenter, builtIn)
        XCTAssertTrue(one.sharesDisplay)
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
        store.remember(swapped)
        XCTAssertEqual(DisplayArrangement.resolve(screens: [projector, builtIn], store: store)?.audience, builtIn)
        XCTAssertEqual(DisplayAssignmentStore.key(for: [projector, builtIn]), DisplayAssignmentStore.key(for: [builtIn, projector]))
        // A different pair has no memory of it.
        let other = ScreenInfo(name: "Epson", frame: projector.frame, isBuiltIn: false)
        XCTAssertEqual(DisplayArrangement.resolve(screens: [builtIn, other], store: store)?.audience, other)
        // A remembered name that is no longer connected is ignored.
        store.setRoles(RememberedDisplayRoles(audience: "Gone", presenter: nil), for: [builtIn, projector])
        XCTAssertEqual(DisplayArrangement.resolve(screens: [builtIn, projector], store: store)?.audience, projector)
    }

    func testAudienceOnlyMemoryFromAnEarlierVersionStillApplies() throws {
        let defaults = try freshDefaults()
        let store = DisplayAssignmentStore(defaults: defaults)
        defaults.set("Built-in Retina Display", forKey: "DisplayAssignment:" + DisplayAssignmentStore.setName(for: [builtIn, projector]))
        let arrangement = try XCTUnwrap(DisplayArrangement.resolve(screens: [builtIn, projector], store: store))
        XCTAssertEqual(arrangement.audience, builtIn)
        XCTAssertEqual(arrangement.presenter, projector)
        XCTAssertFalse(arrangement.sharesDisplay)
    }

    // MARK: Three displays

    let dell = ScreenInfo(name: "Dell U2723", frame: CGRect(x: -2560, y: 0, width: 2560, height: 1440), isBuiltIn: false)

    func threeDisplays() throws -> DisplayArrangement {
        let store = DisplayAssignmentStore(defaults: try freshDefaults())
        return try XCTUnwrap(DisplayArrangement.resolve(screens: [builtIn, projector, dell], store: store))
    }

    func testThreeDisplaysStartWithTheExternalAudienceTheBuiltInPresenterAndTheRestUnused() throws {
        let arrangement = try threeDisplays()
        XCTAssertEqual(arrangement.screens, [builtIn, projector, dell])
        XCTAssertEqual(arrangement.role(of: projector), .audience)
        XCTAssertEqual(arrangement.role(of: builtIn), .presenter)
        XCTAssertEqual(arrangement.role(of: dell), .notUsed)
    }

    func testMakingAnUnusedDisplayTheAudienceLeavesTheOldAudienceUnused() throws {
        let updated = try threeDisplays().assigning(.audience, to: dell)
        XCTAssertEqual(updated.role(of: dell), .audience)
        XCTAssertEqual(updated.role(of: builtIn), .presenter)
        XCTAssertEqual(updated.role(of: projector), .notUsed)
    }

    func testAssigningATakenRoleSwapsWithItsHolder() throws {
        let arrangement = try threeDisplays()
        let audienceToPresenter = arrangement.assigning(.audience, to: builtIn)
        XCTAssertEqual(audienceToPresenter.role(of: builtIn), .audience)
        XCTAssertEqual(audienceToPresenter.role(of: projector), .presenter)
        XCTAssertEqual(audienceToPresenter.role(of: dell), .notUsed)
        let presenterToUnused = arrangement.assigning(.presenter, to: dell)
        XCTAssertEqual(presenterToUnused.role(of: dell), .presenter)
        XCTAssertEqual(presenterToUnused.role(of: builtIn), .notUsed)
        XCTAssertEqual(presenterToUnused.role(of: projector), .audience)
        let audienceAsPresenter = arrangement.assigning(.presenter, to: projector)
        XCTAssertEqual(audienceAsPresenter.role(of: projector), .presenter)
        XCTAssertEqual(audienceAsPresenter.role(of: builtIn), .audience)
    }

    func testThereIsAlwaysOneAudienceAndAtMostOnePresenter() throws {
        let screens = [builtIn, projector, dell]
        let roles: [DisplayRole] = [.audience, .presenter, .notUsed]
        var arrangement = try threeDisplays()
        for screen in screens { for role in roles {
            arrangement = arrangement.assigning(role, to: screen)
            XCTAssertEqual(screens.filter { arrangement.role(of: $0) == .audience }.count, 1, "\(role) to \(screen.name)")
            XCTAssertLessThanOrEqual(screens.filter { arrangement.role(of: $0) == .presenter }.count, 1, "\(role) to \(screen.name)")
        } }
    }

    func testTakingTheRolesOffTheirHolders() throws {
        let arrangement = try threeDisplays()
        let noPresenter = arrangement.assigning(.notUsed, to: builtIn)
        XCTAssertNil(noPresenter.presenterScreen)
        XCTAssertTrue(noPresenter.sharesDisplay, "the presenter page rides over the audience display")
        XCTAssertEqual(noPresenter.presenter, projector)
        let noAudience = arrangement.assigning(.notUsed, to: projector)
        XCTAssertEqual(noAudience.role(of: builtIn), .audience, "the presenter display takes over as the audience")
        XCTAssertNil(noAudience.presenterScreen)
    }

    func testOneDisplayHasNothingToAssign() throws {
        let store = DisplayAssignmentStore(defaults: try freshDefaults())
        let one = try XCTUnwrap(DisplayArrangement.resolve(screens: [builtIn], store: store))
        XCTAssertEqual(one.assigning(.notUsed, to: builtIn), one)
        XCTAssertEqual(one.assigning(.presenter, to: builtIn), one)
    }

    func testThreeDisplayRolesAreRememberedPerSetOfDisplays() throws {
        let store = DisplayAssignmentStore(defaults: try freshDefaults())
        let chosen = try threeDisplays().assigning(.audience, to: dell).assigning(.notUsed, to: builtIn)
        store.remember(chosen)
        let again = try XCTUnwrap(DisplayArrangement.resolve(screens: [dell, builtIn, projector], store: store))
        XCTAssertEqual(again.audience, dell)
        XCTAssertNil(again.presenterScreen, "no presenter display is remembered as none")
        // Two of the three connected is a different set, with no memory of the three.
        XCTAssertEqual(DisplayArrangement.resolve(screens: [builtIn, projector], store: store)?.audience, projector)
    }

    func testADeckRemembersTheDisplaysItWasPlayedOn() throws {
        let store = PresentedDisplaysStore(defaults: try freshDefaults())
        let deck = URL(fileURLWithPath: "/talks/ops.md")
        XCTAssertFalse(store.hasPlayed(deck, on: [builtIn]), "a deck never played")
        store.recordPlay(of: deck, on: [builtIn, projector])
        XCTAssertTrue(store.hasPlayed(URL(fileURLWithPath: "/talks/../talks/ops.md"), on: [projector, builtIn]), "however spelled and listed")
        XCTAssertFalse(store.hasPlayed(deck, on: [builtIn]), "the projector was unplugged")
        XCTAssertFalse(store.hasPlayed(deck, on: [builtIn, projector, dell]), "a display was added")
        XCTAssertFalse(store.hasPlayed(URL(fileURLWithPath: "/talks/other.md"), on: [builtIn, projector]))
    }

    func testOptionsBecomeTheTapPresentCommand() {
        let play = PresentationOptions(mode: .play, startSlide: 3)
        XCTAssertEqual(play.command(port: nil), .present(record: true, presenterPassword: nil, port: nil))
        XCTAssertEqual(play.command(port: 4242), .present(record: true, presenterPassword: nil, port: 4242), "the deck's remembered port travels with the command")
        XCTAssertFalse(play.wantsTunnel)
        let quiet = PresentationOptions(mode: .play, startSlide: 1, record: false, phoneRemote: true)
        XCTAssertEqual(quiet.command(port: nil), .present(record: false, presenterPassword: nil, port: nil))
        XCTAssertTrue(quiet.wantsTunnel)
        let rehearse = PresentationOptions(mode: .rehearse, startSlide: 5, record: true, phoneRemote: true, presenterPassword: "secret")
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
        XCTAssertEqual(suggested, 22933, "FNV-1a of the path, the same in every launch")
    }

    func testTheLastSettingsBecomeTheNextOptions() throws {
        let store = PresentationSettingsStore(defaults: try freshDefaults())
        XCTAssertEqual(store.settings, PresentationSettings(), "the defaults: recording on, no remote")
        let chosen = PresentationSettings(record: false, phoneRemote: true)
        store.settings = chosen
        XCTAssertEqual(PresentationSettingsStore(defaults: store.defaults).settings, chosen, "kept across launches")
        XCTAssertEqual(chosen.options(mode: .play, startSlide: 4, presenterPassword: nil),
                       PresentationOptions(mode: .play, startSlide: 4, record: false, phoneRemote: true, presenterPassword: nil))
        XCTAssertEqual(PresentationSettings().options(mode: .play, startSlide: 4, presenterPassword: "secret"),
                       PresentationOptions(mode: .play, startSlide: 4, record: true, presenterPassword: "secret"),
                       "the password comes from the popover's field for this launch; it is never kept")
    }

    func testATunnelSavedOnItsOwnBecomesThePhoneRemote() throws {
        let defaults = try freshDefaults()
        defaults.set(["startFromSlideOne": true, "record": false, "phoneRemote": false, "tunnel": true], forKey: "PresentationSettings")
        XCTAssertEqual(PresentationSettingsStore(defaults: defaults).settings, PresentationSettings(record: false, phoneRemote: true))
    }

    func testCloudflaredIsFoundOnThePathTapRunsWith() throws {
        let folder = FileManager.default.temporaryDirectory.appendingPathComponent("cloudflared-\(UUID().uuidString)")
        let empty = FileManager.default.temporaryDirectory.appendingPathComponent("empty-\(UUID().uuidString)")
        try FileManager.default.createDirectory(at: folder, withIntermediateDirectories: true)
        try FileManager.default.createDirectory(at: empty, withIntermediateDirectories: true)
        defer { try? FileManager.default.removeItem(at: folder); try? FileManager.default.removeItem(at: empty) }
        let binary = folder.appendingPathComponent("cloudflared")
        try "#!/bin/sh\n".write(to: binary, atomically: true, encoding: .utf8)
        XCTAssertFalse(CloudflaredLocator.isInstalled(searchPath: folder.path), "a file that is not executable is not a tool")
        try FileManager.default.setAttributes([.posixPermissions: 0o755], ofItemAtPath: binary.path)
        XCTAssertTrue(CloudflaredLocator.isInstalled(searchPath: "\(empty.path):\(folder.path)"))
        XCTAssertFalse(CloudflaredLocator.isInstalled(searchPath: empty.path))
        XCTAssertFalse(CloudflaredLocator.isInstalled(searchPath: nil))
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
