import XCTest
@testable import TapDesktopCore

final class SparkleInterfaceTests: XCTestCase {
    func testNothingUpMeansNoRefusal() {
        XCTAssertNil(SparkleInterface().playRefusal, "a session that shows nothing never holds Play back")
    }

    func testThePermissionPromptHoldsPlayUntilAnswered() {
        let interface = SparkleInterface()
        interface.permissionPromptShown()
        XCTAssertEqual(interface.playRefusal, SparkleInterface.permissionPromptMessage)
        interface.permissionPromptAnswered()
        XCTAssertNil(interface.playRefusal)
    }

    func testAnUpdateWindowHoldsPlayUntilTheSessionFinishes() {
        let interface = SparkleInterface()
        interface.updateWindowShown()
        XCTAssertEqual(interface.playRefusal, SparkleInterface.updateWindowMessage)
        interface.permissionPromptAnswered()
        XCTAssertEqual(interface.playRefusal, SparkleInterface.updateWindowMessage, "answering a prompt closes no update window")
        interface.sessionFinished()
        XCTAssertNil(interface.playRefusal)
    }

    func testTheEndOfASessionClosesThePromptToo() {
        let interface = SparkleInterface()
        interface.permissionPromptShown()
        interface.updateWindowShown()
        XCTAssertEqual(interface.playRefusal, SparkleInterface.permissionPromptMessage, "the prompt is named first: it is what the person answers")
        interface.sessionFinished()
        XCTAssertFalse(interface.permissionPromptIsUp)
        XCTAssertFalse(interface.updateWindowIsUp)
    }
}
