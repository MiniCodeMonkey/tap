import XCTest
@testable import TapDesktopCore

final class TalkPageReloadTests: XCTestCase {
    func testTheFragmentIsTheTalksCurrentSlide() {
        let page = URL(string: "http://127.0.0.1:24680/presenter#2")!
        XCTAssertEqual(TalkPageReload.url(reloading: page, atSlide: 5).absoluteString, "http://127.0.0.1:24680/presenter#5")
    }

    func testAPageWithoutAFragmentGetsOne() {
        let page = URL(string: "http://127.0.0.1:24680/")!
        XCTAssertEqual(TalkPageReload.url(reloading: page, atSlide: 3).absoluteString, "http://127.0.0.1:24680/#3")
    }

    func testTheQueryIsKept() {
        let page = URL(string: "http://127.0.0.1:24680/?theme=dark#1")!
        XCTAssertEqual(TalkPageReload.url(reloading: page, atSlide: 4).absoluteString, "http://127.0.0.1:24680/?theme=dark#4")
    }

    func testWithNoSlideKnownThePageURLIsUnchanged() {
        let page = URL(string: "http://127.0.0.1:24680/presenter#2")!
        XCTAssertEqual(TalkPageReload.url(reloading: page, atSlide: nil), page)
    }
}
