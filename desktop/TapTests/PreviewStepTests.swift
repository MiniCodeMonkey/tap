import XCTest
@testable import Tap

final class PreviewStepTests: HostedTestCase {
    func testStepThroughFragments() async throws {
        let document = try await openDeckAndWaitForPreview(try Fixtures.copyDeck("steps.md"))
        let controller = try XCTUnwrap(document.sessionController)
        try await waitForBoxes(document, count: 3)
        controller.editor.moveCursor(toSlide: 2)
        try await waitForPreview(document, slide: 3)
        XCTAssertEqual(controller.previewViewController.stepLabel.stringValue, "Step 3 of 3")

        let observer = HubObserver(ready: try await waitForRunningTap(document))
        defer { observer.close() }
        try await Task.sleep(nanoseconds: 300_000_000)

        controller.previewViewController.onStepBackward?()
        controller.previewViewController.onStepBackward?()
        XCTAssertEqual(controller.previewViewController.stepLabel.stringValue, "Step 1 of 3")
        controller.previewViewController.onStepForward?()
        XCTAssertEqual(controller.previewViewController.stepLabel.stringValue, "Step 2 of 3")
        controller.previewViewController.onStepForward?()

        try await waitUntil(timeout: 5, "four slide messages") { observer.slideMessages.count >= 4 }
        XCTAssertEqual(observer.slideMessages.suffix(4).map(\.fragment), [0, -1, 0, 1])
        XCTAssertTrue(observer.slideMessages.allSatisfy { $0.slideIndex == 2 })
        try await waitForPreview(document, slide: 3)
    }
}
