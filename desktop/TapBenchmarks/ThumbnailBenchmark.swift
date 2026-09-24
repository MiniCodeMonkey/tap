import XCTest
@testable import Tap

/// A cold pass over every thumbnail of the 200-slide deck, and a reopen
/// that must render nothing. The person runs this with `make -C desktop bench`.
final class ThumbnailBenchmark: BenchmarkCase {
    func testColdThumbnailPassAndReopenFromTheCache() async throws {
        AppEnvironment.shared.thumbnailCache = ThumbnailCache(directory: FileManager.default.temporaryDirectory.appendingPathComponent("tap-bench-thumbnails-\(UUID().uuidString)"))
        let deck = try await openStressDeck()
        let controller = try XCTUnwrap(deck.sessionController)
        let started = Date()
        try await waitUntil(timeout: 180, "200 thumbnails") { (1...200).allSatisfy { controller.slidePanel.image(forSlide: $0) != nil } }
        let seconds = Date().timeIntervalSince(started)
        XCTAssertEqual(controller.thumbnails.renderer.renderCount, 200)
        XCTAssertLessThan(seconds / 200, 0.1, "under 100 ms per slide; the prototype measured about 24 ms")
        let deckURL = try XCTUnwrap(deck.fileURL)
        deck.close()
        try await Task.sleep(nanoseconds: 500_000_000)

        let (document, _) = try await NSDocumentController.shared.openDocument(withContentsOf: deckURL, display: true)
        let reopened = try XCTUnwrap(document as? DeckDocument)
        let again = try XCTUnwrap(reopened.sessionController)
        try await waitUntil(timeout: 60, "200 thumbnails from the cache") { (1...200).allSatisfy { again.slidePanel.image(forSlide: $0) != nil } }
        XCTAssertEqual(again.thumbnails.renderer.renderCount, 0, "reopening loads every thumbnail from the cache without rendering")
    }
}
