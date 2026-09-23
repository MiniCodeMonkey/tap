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
}
