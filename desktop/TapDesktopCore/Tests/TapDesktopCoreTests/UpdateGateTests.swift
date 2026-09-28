import XCTest
@testable import TapDesktopCore

final class UpdateGateTests: XCTestCase {
    func testAChecksWaitsForTheTalk() {
        let gate = UpdateGate()
        XCTAssertTrue(gate.mayCheck(mayInterrupt: true))
        XCTAssertFalse(gate.mayCheck(mayInterrupt: false), "no check during a talk, scheduled or not")
    }

    func testAFoundUpdateIsDroppedWhilePresenting() {
        let gate = UpdateGate()
        XCTAssertTrue(gate.mayProceed(mayInterrupt: true))
        XCTAssertFalse(gate.mayProceed(mayInterrupt: false), "a check that started before Play must not prompt during the talk")
        XCTAssertEqual(UpdateGate.PresentingError().localizedDescription, UpdateGate.presentingMessage)
    }

    func testAPostponedRelaunchRunsOnceWhenTheTalkEnds() {
        let gate = UpdateGate()
        var ran = 0
        XCTAssertFalse(gate.shouldPostponeRelaunch(mayInterrupt: true) { ran += 1 }, "no talk, no postponement")
        XCTAssertNil(gate.postponedRelaunch)
        XCTAssertEqual(ran, 0, "Sparkle relaunches itself when nothing is postponed")

        XCTAssertTrue(gate.shouldPostponeRelaunch(mayInterrupt: false) { ran += 1 })
        XCTAssertNotNil(gate.postponedRelaunch)
        gate.resumePostponedRelaunch()
        XCTAssertEqual(ran, 1)
        XCTAssertNil(gate.postponedRelaunch, "run once, then forgotten")
        gate.resumePostponedRelaunch()
        XCTAssertEqual(ran, 1, "a second talk ending runs nothing")
    }

    func testASecondPostponementKeepsTheNewestBlock() {
        let gate = UpdateGate()
        var order: [String] = []
        _ = gate.shouldPostponeRelaunch(mayInterrupt: false) { order.append("first") }
        _ = gate.shouldPostponeRelaunch(mayInterrupt: false) { order.append("second") }
        gate.resumePostponedRelaunch()
        XCTAssertEqual(order, ["second"], "Sparkle asks again for the same update; the newest handler is the live one")
    }

    func testTheUpdaterStartsOnlyInAReleaseOutsideTests() {
        XCTAssertTrue(UpdateGate.updaterMayStart(bundleVersion: "2.1.0", isHostedByTests: false, hasTestDefaultsSuite: false))
        XCTAssertFalse(UpdateGate.updaterMayStart(bundleVersion: "2.1.0", isHostedByTests: true, hasTestDefaultsSuite: false), "a hosted test process")
        XCTAssertFalse(UpdateGate.updaterMayStart(bundleVersion: "2.1.0", isHostedByTests: false, hasTestDefaultsSuite: true), "a UI test launch")
        XCTAssertFalse(UpdateGate.updaterMayStart(bundleVersion: "0.0.0", isHostedByTests: false, hasTestDefaultsSuite: false), "a Debug build from DerivedData")
        XCTAssertFalse(UpdateGate.updaterMayStart(bundleVersion: "0.0.0-dev", isHostedByTests: false, hasTestDefaultsSuite: false), "a local dry run")
        XCTAssertFalse(UpdateGate.updaterMayStart(bundleVersion: nil, isHostedByTests: false, hasTestDefaultsSuite: false))
        XCTAssertTrue(UpdateGate.updaterMayStart(bundleVersion: "2.1.0-beta.3", isHostedByTests: false, hasTestDefaultsSuite: false), "a pre-release checks the feed like a final")
    }
}
