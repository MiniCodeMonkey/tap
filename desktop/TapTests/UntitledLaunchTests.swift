import XCTest
@testable import Tap

/// Launching Tap with no document must not ask AppKit to open an untitled
/// document, and must not present an error alert. "New deck" is deliberately
/// inert until its own sheet exists, so there is genuinely no untitled
/// document to offer.
final class UntitledLaunchTests: HostedTestCase {
    func testDoesNotOfferUntitledDocument() throws {
        let delegate = try XCTUnwrap(NSApp.delegate)
        let shouldOpenUntitledFile = delegate.applicationShouldOpenUntitledFile?(NSApp)
        XCTAssertEqual(shouldOpenUntitledFile, false)
    }

    func testLaunchingWithNoDocumentPresentsNoAlert() {
        // Reaching this line at all is part of the proof: an error alert
        // presented with NSAlert.runModal blocks the main thread forever,
        // which would keep every test in the bundle, including this one,
        // from ever starting. The explicit checks below confirm there is no
        // modal session and no alert panel left showing.
        XCTAssertNil(NSApp.modalWindow)
        XCTAssertFalse(NSApp.windows.contains { $0.isVisible && $0.className.contains("Alert") })
    }
}
