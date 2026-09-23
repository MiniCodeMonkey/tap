import XCTest
@testable import Tap

/// `DeckSessionController`'s send lifecycle: the closures it hands to
/// `sourceSync` and the ordering between `stop()` and an in-flight PUT.
/// These build a controller directly, without a document window or a real
/// tap process, because both findings live entirely inside that lifecycle.
final class DeckSessionControllerTests: HostedTestCase {
    /// Lets a test hold a response open until it chooses to release it, and
    /// confirms the response is actually in flight before moving on, so the
    /// ordering (stop before the response resolves) is forced, not hoped for.
    actor ResponseGate {
        private var entered = false
        private var released = false
        private var enteredContinuation: CheckedContinuation<Void, Never>?
        private var releaseContinuation: CheckedContinuation<Void, Never>?

        func markEntered() {
            entered = true
            enteredContinuation?.resume()
            enteredContinuation = nil
        }

        func waitForEntry() async {
            if entered { return }
            await withCheckedContinuation { enteredContinuation = $0 }
        }

        func waitForRelease() async {
            if released { return }
            await withCheckedContinuation { releaseContinuation = $0 }
        }

        func release() {
            released = true
            releaseContinuation?.resume()
            releaseContinuation = nil
        }
    }

    /// The closures `sourceSync` was given must tolerate the controller
    /// deallocating out from under them, the same as every other closure in
    /// the file. Nothing else in the app keeps this controller alive once
    /// the local reference is dropped.
    func testSourceSyncClosuresSurviveTheControllerDeallocating() async {
        var controller: DeckSessionController? = DeckSessionController(document: DeckDocument())
        let sourceSync = controller!.sourceSync!
        sourceSync.sender = { _ in SlideList(slides: [], errors: []) }
        controller = nil

        await sourceSync.sendNow()
    }

    /// A source-sync PUT already in flight when `stop()` runs must not land
    /// on the editor once its response arrives. The gate forces the real
    /// ordering: the request starts, stop() runs while it is still pending,
    /// and only then does the response resolve.
    func testALateSourceSyncResponseAfterStopIsNotApplied() async {
        let controller = DeckSessionController(document: DeckDocument())
        let gate = ResponseGate()
        controller.sourceSync.sender = { _ in
            await gate.markEntered()
            await gate.waitForRelease()
            return SlideList(slides: [], errors: [])
        }
        var applied = false
        controller.onSlideListApplied = { _ in applied = true }

        let send = Task { await controller.sourceSync.sendNow() }
        await gate.waitForEntry() // the PUT is now in flight
        controller.stop() // close the document mid-request
        await gate.release()
        await send.value

        XCTAssertFalse(applied, "a response that arrives after stop() must not reach the editor")
    }

    /// A ready line for a port nothing listens on. These tests never want an
    /// answer from tap; they drive the controller's own restart race, and the
    /// presenter exchange is replaced for each one.
    func readyLine(port: Int) -> TapReady {
        TapReady(port: port, token: String(repeating: "a", count: 64),
                 launch: String(repeating: "b", count: 64), presenter: String(repeating: "c", count: 64))
    }

    /// Hands the controller a running tap the way its session does.
    func sessionBecameRunning(_ controller: DeckSessionController, _ ready: TapReady) {
        controller.session.onStateChange?(.running(ready))
    }

    /// The document can close while the presenter exchange is still in the
    /// air. Its answer belongs to a session that no longer exists, so no
    /// socket may be opened on it.
    func testAPresenterExchangeStillInFlightAtStopOpensNoSocket() async throws {
        let controller = DeckSessionController(document: DeckDocument())
        let gate = ResponseGate()
        controller.exchangePresenterSecret = { _ in
            await gate.markEntered()
            await gate.waitForRelease()
        }

        sessionBecameRunning(controller, readyLine(port: 1))
        await gate.waitForEntry() // the exchange is now in flight
        controller.stop()
        await gate.release()
        try await Task.sleep(nanoseconds: 200_000_000)

        XCTAssertNil(controller.socket, "a stopped controller opens no socket on a finished exchange")
    }

    /// tap restarting swaps the client under an exchange that is still in
    /// the air. The answer belongs to the old client, and the socket that is
    /// already open on the new one must survive it.
    func testAPresenterExchangeForAReplacedClientLeavesTheNewSocketAlone() async throws {
        let controller = DeckSessionController(document: DeckDocument())
        addTeardownBlock { @MainActor in controller.stop() }
        let stale = ResponseGate()
        controller.exchangePresenterSecret = { _ in
            await stale.markEntered()
            await stale.waitForRelease()
        }

        sessionBecameRunning(controller, readyLine(port: 1))
        await stale.waitForEntry() // the first exchange is in flight
        let firstClient = controller.client

        // tap restarts: a second client, whose exchange answers at once.
        controller.exchangePresenterSecret = { _ in }
        sessionBecameRunning(controller, readyLine(port: 2))
        try await waitUntil(timeout: 5, "the new client's socket") { controller.socket != nil }
        let newSocket = try XCTUnwrap(controller.socket)
        XCTAssertFalse(controller.client === firstClient, "the restart replaced the client")

        await stale.release()
        try await Task.sleep(nanoseconds: 200_000_000)

        XCTAssertTrue(controller.socket === newSocket, "the stale exchange left the new client's socket in place")
    }

    /// A refusal is not fatal. The socket still opens, because hearing the
    /// hub is worth having even when the app cannot drive it.
    func testARefusedPresenterSecretStillOpensTheSocket() async throws {
        let controller = DeckSessionController(document: DeckDocument())
        addTeardownBlock { @MainActor in controller.stop() }
        controller.exchangePresenterSecret = { _ in
            throw TapErrorPayload(code: "presenter_refused", message: "tap answered 403 without a presenter cookie")
        }

        sessionBecameRunning(controller, readyLine(port: 1))

        try await waitUntil(timeout: 5, "the socket after the refusal") { controller.socket != nil }
        XCTAssertTrue(controller.session.log.text.contains("tap refused the presenter secret"), controller.session.log.text)
    }
}
