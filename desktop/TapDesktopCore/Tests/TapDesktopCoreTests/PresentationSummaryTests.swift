import XCTest
@testable import TapDesktopCore

final class PresentationSummaryTests: XCTestCase {
    func testDecodesWhatTheThumbnailsNeed() throws {
        let json = """
        {"config": {"title": "Ops", "theme": "terminal", "customTheme": true, "aspectRatio": "16:9",
                    "themeColors": {"accent": "#ffb84d", "background": "#0f1a16"}},
         "slides": [{"index": 0, "layout": "title", "html": "<h1>x</h1>", "slots": {}, "slotOrder": [], "fragmentCount": 0, "steps": 0, "hash": "aaaaaaaaaaaa"},
                    {"index": 1, "layout": "default", "html": "", "slots": {}, "slotOrder": [], "fragmentCount": 1, "steps": 2, "hash": "bbbbbbbbbbbb", "skip": true}],
         "liveCode": {"drivers": []}, "revision": "7f3c1a2b9d0e"}
        """
        let summary = try PresentationSummary.decode(Data(json.utf8))
        XCTAssertEqual(summary.revision, "7f3c1a2b9d0e")
        XCTAssertEqual(summary.themeSignature, "terminal|custom|16:9|accent=#ffb84d;background=#0f1a16")
        XCTAssertEqual(summary.slides, [SlideSummary(hash: "aaaaaaaaaaaa", skip: false, steps: 0), SlideSummary(hash: "bbbbbbbbbbbb", skip: true, steps: 2)])
    }

    func testDefaultsWhenTheConfigIsBare() throws {
        let summary = try PresentationSummary.decode(Data(#"{"config": {}, "slides": [], "revision": ""}"#.utf8))
        XCTAssertEqual(summary.themeSignature, "|||", "no theme, no custom theme, no aspect ratio, no colours")
        XCTAssertEqual(summary.slides, [])
    }
}
